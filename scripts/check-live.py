#!/usr/bin/env python3
"""Check live ingestion without printing device identities or database rows."""

import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import sqlite3
import sys
import time
from urllib.request import urlopen
from urllib.error import HTTPError


def scanner_health(base):
    """503 is a health result: retain its JSON, never its raw error text."""
    try:
        response = urlopen(base.rstrip("/") + "/api/scanner", timeout=5)
    except HTTPError as exc:
        if exc.code != 503:
            raise
        response = exc
    with response:
        health = json.load(response)
        if response.code == 503:
            health["ready"] = False
        return health


def snapshot(base, db):
    health = scanner_health(base)
    if health.get("ready") is not True:
        return {"scanner": health}
    with urlopen(base.rstrip("/") + "/api/ingestion", timeout=5) as response:
        counters = json.load(response)
    with sqlite3.connect(db.resolve().as_uri() + "?mode=ro", uri=True) as conn:
        # Read the maximum ID and its timestamp together via the primary key.
        latest = conn.execute("SELECT id, ts FROM observations ORDER BY id DESC LIMIT 1").fetchone()
    return {"scanner": health, "processed": counters["processed"],
            "observation_id": latest[0] if latest else 0,
            "observation_ts": latest[1] if latest else 0}


def valid_start(value):
    if not isinstance(value, str):
        return False
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        return parsed.tzinfo is not None and parsed > datetime.min.replace(tzinfo=timezone.utc)
    except (ValueError, OverflowError):
        return False


def evaluate(before, after, now, seconds):
    """Pure bounded-sample verdict; only allowlisted aggregate data leaves here."""
    first, last = before["scanner"], after["scanner"]
    result = {"status": "blocked", "sample_seconds": seconds,
              "scanner_ready_before": first.get("ready") is True,
              "scanner_ready_after": last.get("ready") is True}

    def blocked(error_type):
        return dict(result, error_type=error_type)

    if not result["scanner_ready_before"] or not result["scanner_ready_after"]:
        return blocked("NoReady")
    starts = (first.get("started_at"), last.get("started_at"))
    if not all(valid_start(value) for value in starts):
        return blocked("NoReady")
    result["scanner_start_unchanged"] = starts[0] == starts[1]
    if not result["scanner_start_unchanged"]:
        return blocked("ScannerRestarted")
    values = (first.get("live_updates"), last.get("live_updates"),
              before.get("processed"), after.get("processed"),
              before.get("observation_id"), after.get("observation_id"),
              after.get("observation_ts"))
    if any(type(value) is not int or value < 0 for value in values):
        return blocked("InvalidSample")
    live_before, live_after, processed_before, processed_after, id_before, id_after, ts = values
    result.update(live_updates_delta=live_after - live_before,
                  processed_delta=processed_after - processed_before,
                  observation_id_advanced=id_after > id_before,
                  newest_observation_age_seconds=now - ts if ts else None)
    if live_after < live_before or processed_after < processed_before or id_after < id_before:
        return blocked("CounterRegression")
    if live_after == live_before or last.get("activity") == "silent":
        return blocked("SourceSilence")
    if processed_after == processed_before:
        return blocked("ProcessingStalled")
    if id_after == id_before:
        return blocked("DatabaseStalled")
    # Reject future timestamps as well as old cached observations.
    if not ts or not 0 <= now - ts <= seconds + 5:
        return blocked("StaleDatabase")
    result["status"] = "passed"
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8128")
    parser.add_argument("--db", type=Path, default=Path("data/ble-watch.db"))
    parser.add_argument("--adapter", default="hci0", help="legacy option; scanner endpoint is authoritative")
    parser.add_argument("--seconds", type=int, default=45)
    args = parser.parse_args()
    if not 1 <= args.seconds <= 300 or not args.adapter.isalnum():
        parser.error("use 1..300 seconds and an alphanumeric adapter name")
    try:
        before = snapshot(args.base_url, args.db)
        time.sleep(args.seconds)
        after = snapshot(args.base_url, args.db)
        result = evaluate(before, after, int(time.time()), args.seconds)
        print(json.dumps(result))
        return 0 if result["status"] == "passed" else 2
    except Exception as exc:
        # Exception text can contain local paths, URLs, or source data.
        print(json.dumps({"status": "blocked", "error_type": type(exc).__name__}))
        return 2


if __name__ == "__main__":
    sys.exit(main())
