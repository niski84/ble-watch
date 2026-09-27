# Skitchcraft / BLE Watch demo

This is a working software demonstration, not a radio capture or a simulation
of a proprietary Palantir product. The observations are synthetic. Witchcraft,
the detector, SQLite, HTTP handlers, and rendered dashboard are real.

## Start

From the repository root:

```sh
go run ./cmd/ble-demo
```

Open the URL printed in the terminal. The listener uses an available loopback
port. To choose a stable port, pass `-port 8138`, provided that port is free.
An occupied port fails without stopping any other process.

The command does not load `.env` or the live receiver's configuration. It does
not connect to BlueZ or create a webhook sink. Each launch makes its own
temporary database and prints that path. The live database is not opened.

Stop with Ctrl+C. The synthetic database is retained for inspection and may be
removed manually afterward. Restart the command to repeat the scenario from a
fresh database. Baseline readings continue after the scenario, so stop a demo
you are no longer using; this command does not run a retention job.

## Walkthrough

1. Open the dashboard during the five-second startup pause. The banner identifies
   synthetic input on every page. Bluetooth readiness deliberately returns 503
   with `disabled_for_demo`; HTTP liveness remains 200.
2. Watch the synthetic workshop sensor appear. The scenario explicitly marks
   this fixture recognized and supplies six baseline readings after its first
   sighting. Recognition here is a fixture choice, not AI classification.
3. Watch accepted work advance before processing during the two-second delayed
   submission. The stronger reading moves from about -72 dBm to -38 dBm and
   triggers the existing baseline rule. The event's details include the z-score,
   signal delta, and baseline statistics. This is not proof of a security threat.
4. During silence, the demo waits beyond its four-second presence threshold and
   invokes the existing disappearance check. It submits the return three
   seconds later. The live event feed updates without a page reload.
5. Open the sensor's detail page to inspect stored signal history. Compare the
   processing counters with stored samples. They represent different facts:
   queue acceptance, detector returns, rejections, and rows currently in SQLite.

The scripted portion accepts nine observations and creates nine stored samples
and four events: appearance, signal surge, disappearance, return. Afterward,
one baseline observation every two seconds keeps the sensor present. Event
times follow the wall clock. This is not yet the speed-independent replay
benchmark described in the intelligence plan.

## What to say while showing it

"I wanted to follow Bluetooth observations all the way through processing and
storage. I kept the Smash Deck web patterns I already use, then put Witchcraft
behind the observation boundary. This demo uses fake input so I can show the
real queue, detector, database, and dashboard without exposing nearby devices.
I also contributed a way for callers to detect rejected submissions during
cancellation or shutdown."

The practical reasons are:

- One worker keeps the stateful detector's processing serialized.
- Observable immediate and delayed submission make acceptance failures explicit.
- A shared path from input to stored events makes integration bugs reproducible.

Do not claim durable delivery, persistence retries, automatic ownership
inference, or a relationship graph. They are not implemented. The delayed
reading is a demonstration of the scheduling API, not a reason to delay real
Bluetooth observations. A plain Go worker could handle the current workload;
the library becomes more valuable as lifecycle and scheduling needs grow.

## Verify

```sh
go test -race ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
TEST_BASE_URL=http://localhost:8138 BLE_DEMO_TEST=1 npm test
```

For the browser command, first start the demo on port 8138, or substitute the
printed URL. The scenario integration test checks real database rows and event
order. The browser suite checks both desktop and mobile, with no mocked API.
Only set `BLE_DEMO_TEST=1` against the isolated demo.

The frontend currently loads libraries and fonts from CDNs, so browser rendering
requires internet access. Bundling assets for a fully offline demo is future work.
