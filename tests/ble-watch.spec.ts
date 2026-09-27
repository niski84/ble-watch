import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

export const suite = {
  id: "ble-watch",
  summary: "Read-only health and ingestion contracts, live dashboard, registry, event log, and theme persistence; no observation requirement or private artifacts.",
  tags: ["api", "ui", "smoke", "privacy"],
} as const;

const BASE = (process.env.TEST_BASE_URL || 'http://localhost:8128').replace(/\/+$/, '');

// Playwright also writes an error-context.md DOM snapshot independently of trace.
// Disable it in this worker before any page is opened; never attach live payloads.
process.env.PLAYWRIGHT_NO_COPY_PROMPT = '1';
test.use({ screenshot: 'off', trace: 'off', video: 'off' });

async function readJSON(request: APIRequestContext, url: string): Promise<Record<string, unknown>> {
  // Fixed diagnostics prevent a proxy/error body or JSON parse error leaking data.
  const response = await request.get(url, { maxRedirects: 0 }).catch(() => {
    throw new Error('BLE Watch API request failed (details suppressed for privacy)');
  });
  expect(response.status(), 'Read-only API returns HTTP 200').toBe(200);
  expect(response.headers()['content-type']?.includes('application/json') === true,
    'Read-only API returns JSON').toBe(true);
  const body: unknown = await response.json().catch(() => {
    throw new Error('BLE Watch API returned invalid JSON (body suppressed for privacy)');
  });
  expect(body !== null && typeof body === 'object' && !Array.isArray(body),
    'API response is an object').toBe(true);
  return body as Record<string, unknown>;
}

async function openPage(page: Page, url: string) {
  const response = await page.goto(url, { waitUntil: 'domcontentloaded' }).catch(() => {
    throw new Error('BLE Watch page navigation failed (details suppressed for privacy)');
  });
  expect(response?.status(), 'Live page returns HTTP 200').toBe(200);
}

// Poll booleans, not locator assertions that can print matching private DOM nodes.
async function visible(page: Page, selector: string, label: string) {
  await expect.poll(() => page.locator(selector).isVisible(), { message: label }).toBe(true);
}

test.beforeEach(async ({ context }) => {
  // Guard against accidental writes even if future frontend code changes.
  await context.route('**/*', async route => {
    if (['GET', 'HEAD', 'OPTIONS'].includes(route.request().method())) {
      await route.continue();
    } else {
      await route.abort();
    }
  });
});

test('health identifies the running service', async ({ request }) => {
  const body = await readJSON(request, `${BASE}/api/health`);
  expect(body.status === 'ok', 'Health status is ok').toBe(true);
  expect(body.service === 'ble-watch', 'Health identifies BLE Watch').toBe(true);
});

test('ingestion exposes nonnegative integer counters, including an idle scanner', async ({ request }) => {
  const body = await readJSON(request, `${BASE}/api/ingestion`);
  for (const key of ['submitted', 'rejected', 'processed']) {
    const value = body[key];
    expect(typeof value === 'number' && Number.isInteger(value) && value >= 0,
      `Ingestion ${key} is a nonnegative integer`).toBe(true);
  }
  // Zero is valid: this smoke suite does not claim hardware/observation evidence.
});

test('live dashboard provides monitoring structure and read-only filters', async ({ page }) => {
  await openPage(page, `${BASE}/`);
  await expect.poll(async () => (await page.title()) === 'Dashboard · ble-watch',
    { message: 'Dashboard document title' }).toBe(true);
  await visible(page, 'main h1', 'Dashboard heading is visible');
  await visible(page, '#live-devices', 'Live device region is visible');
  await expect.poll(() => page.locator('#live-events').count(),
    { message: 'Live event region exists even when empty' }).toBe(1);
  for (const heading of ['Anomaly feed', 'Detection modes']) {
    await expect.poll(() => page.getByRole('heading', { name: heading, exact: true }).isVisible(),
      { message: `${heading} heading is visible` }).toBe(true);
  }
  for (const filter of ['all', 'recognized', 'present']) {
    const button = page.locator(`button[data-filter="${filter}"]`);
    await button.click();
    await expect.poll(async () => (await button.getAttribute('class'))?.split(/\s+/).includes('btn-primary') === true,
      { message: `${filter} filter becomes selected without requiring devices` }).toBe(true);
  }
  // Navigation exists on both layouts (the mobile drawer starts collapsed).
  for (const href of ['/devices', '/events']) {
    await expect.poll(() => page.locator(`.drawer-side a[href="${href}"]`).count(),
      { message: 'Primary navigation destination exists' }).toBe(1);
  }
});

test('registry and event log render without requiring stored observations', async ({ page }) => {
  await openPage(page, `${BASE}/devices`);
  await expect.poll(() => page.getByRole('heading', { name: 'Device registry', exact: true }).isVisible(),
    { message: 'Registry heading is visible' }).toBe(true);
  for (const name of [/Recognized devices/, /Unknown devices/]) {
    await expect.poll(() => page.getByRole('heading', { name }).isVisible(),
      { message: 'Registry classification section is visible' }).toBe(true);
  }
  await openPage(page, `${BASE}/events`);
  await expect.poll(() => page.getByRole('heading', { name: 'Event log', exact: true }).isVisible(),
    { message: 'Event log heading is visible' }).toBe(true);
});

test('theme follows system preference and persists an explicit toggle', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await openPage(page, `${BASE}/`);
  const isTheme = (theme: string) => page.locator('html').getAttribute('data-theme').then(value => value === theme);
  await expect.poll(() => isTheme('dark'), { message: 'Dark system preference applies' }).toBe(true);
  await page.getByRole('button', { name: 'Toggle theme', exact: true }).click();
  await expect.poll(() => isTheme('light'), { message: 'Theme toggle applies light mode' }).toBe(true);
  await openPage(page, `${BASE}/`);
  await expect.poll(() => isTheme('light'), { message: 'Explicit theme survives navigation' }).toBe(true);
});
