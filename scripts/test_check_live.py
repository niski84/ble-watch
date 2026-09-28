"""Hardware-independent tests for the bounded live verification contract."""

import contextlib
import copy
import importlib.util
import io
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest
from unittest.mock import patch
from urllib.error import HTTPError, URLError


spec = importlib.util.spec_from_file_location("check_live", Path(__file__).with_name("check-live.py"))
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class LiveCheckTests(unittest.TestCase):
    def setUp(self):
        self.before = {"scanner": {"ready": True, "started_at": "2026-09-27T12:00:00Z",
                                   "live_updates": 10, "seeded": 50, "activity": "active"},
                       "processed": 60, "observation_id": 100, "observation_ts": 990}
        self.after = copy.deepcopy(self.before)
        self.after.update(processed=65, observation_id=105, observation_ts=1000)
        self.after["scanner"]["live_updates"] = 15

    def verdict(self, error=None):
        result = check.evaluate(self.before, self.after, now=1000, seconds=10)
        self.assertEqual(result["status"], "blocked" if error else "passed")
        self.assertEqual(result.get("error_type"), error)
        return result

    def test_live_good(self):
        self.assertEqual(self.verdict()["live_updates_delta"], 5)

    def test_cached_only(self):
        self.after["scanner"].update(live_updates=10, seeded=500)
        self.verdict("SourceSilence")

    def test_restart(self):
        self.after["scanner"]["started_at"] = "2026-09-27T12:00:01Z"
        self.verdict("ScannerRestarted")

    def test_unavailable_at_either_boundary(self):
        for side in ("before", "after"):
            with self.subTest(side=side):
                self.setUp()
                setattr(self, side, {"scanner": {"ready": False}})
                self.verdict("NoReady")

    def test_zero_or_invalid_start(self):
        for start in (None, "", 0, "0001-01-01T00:00:00Z", "invalid"):
            with self.subTest(start=start):
                self.before["scanner"]["started_at"] = start
                self.after["scanner"]["started_at"] = start
                self.verdict("NoReady")

    def test_stale_or_future_database(self):
        for ts in (0, 984, 1001):
            self.after["observation_ts"] = ts
            self.verdict("StaleDatabase")

    def test_counter_regression(self):
        for field in ("live_updates", "processed", "observation_id"):
            with self.subTest(field=field):
                self.setUp()
                target = self.after["scanner"] if field == "live_updates" else self.after
                target[field] = 1
                self.verdict("CounterRegression")

    def test_stalled_processing_or_database(self):
        self.after["processed"] = self.before["processed"]
        self.verdict("ProcessingStalled")
        self.setUp()
        self.after["observation_id"] = self.before["observation_id"]
        self.verdict("DatabaseStalled")

    def test_endpoint_silence_is_authoritative(self):
        self.after["scanner"]["activity"] = "silent"
        self.verdict("SourceSilence")

    def test_invalid_counters_cannot_pass(self):
        for value in (None, True, "15", -1):
            self.after["scanner"]["live_updates"] = value
            self.verdict("InvalidSample")

    def test_snapshot_uses_highest_id_and_its_timestamp(self):
        with tempfile.TemporaryDirectory(prefix="ble-live-test-") as directory:
            db = Path(directory) / "sample.db"
            with sqlite3.connect(db) as conn:
                conn.execute("CREATE TABLE observations(id INTEGER PRIMARY KEY, ts INTEGER)")
                conn.executemany("INSERT INTO observations VALUES (?, ?)", [(1, 1000), (2, 990)])
            with patch.object(check, "scanner_health", return_value=self.before["scanner"]), \
                    patch.object(check, "urlopen", return_value=io.BytesIO(b'{"processed": 60}')):
                sample = check.snapshot("http://localhost", db)
            self.assertEqual(sample["observation_id"], 2)
            self.assertEqual(sample["observation_ts"], 990)
            self.assertEqual(sample["processed"], 60)

    def test_passed_output_is_allowlisted(self):
        self.after["scanner"].update(error="private-device", adapter="private-adapter")
        self.after["identity"] = "private-identity"
        self.assertNotIn("private", json.dumps(self.verdict()))

    def test_503_body_is_read_and_private(self):
        payload = {"ready": False, "error": "private-device-identity"}
        body = io.BytesIO(json.dumps(payload).encode())
        error = HTTPError("http://private-host/api/scanner", 503, "private", {}, body)
        with patch.object(check, "urlopen", side_effect=error) as request:
            sample = check.snapshot("http://private-host", Path("missing.db"))
        self.assertEqual(sample["scanner"], payload)
        request.assert_called_once_with("http://private-host/api/scanner", timeout=5)
        result = check.evaluate(sample, sample, 1000, 10)
        self.assertNotIn("private", json.dumps(result))
        self.assertEqual(result["error_type"], "NoReady")

    def test_main_exit_codes_and_private_errors(self):
        for samples, code in (([self.before, self.after], 0),
                              ([{"scanner": {"ready": False}}] * 2, 2)):
            output = io.StringIO()
            with patch("sys.argv", ["check-live.py", "--seconds", "10"]), \
                    patch.object(check, "snapshot", side_effect=samples), \
                    patch.object(check.time, "sleep"), patch.object(check.time, "time", return_value=1000), \
                    contextlib.redirect_stdout(output):
                self.assertEqual(check.main(), code)
            self.assertEqual(json.loads(output.getvalue())["status"], "passed" if code == 0 else "blocked")
        output = io.StringIO()
        with patch("sys.argv", ["check-live.py"]), \
                patch.object(check, "snapshot", side_effect=URLError("private-host")), \
                contextlib.redirect_stdout(output):
            self.assertEqual(check.main(), 2)
        self.assertEqual(json.loads(output.getvalue()), {"status": "blocked", "error_type": "URLError"})


if __name__ == "__main__":
    unittest.main()
