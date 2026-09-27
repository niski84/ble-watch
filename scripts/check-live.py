#!/usr/bin/env python3
"""Check live ingestion without printing device identities or database rows."""

import argparse
import json
from pathlib import Path
import sqlite3
import subprocess
import sys
import time
from urllib.request import urlopen


def snapshot(base, db):
    with urlopen(base.rstrip("/") + "/api/ingestion", timeout=5) as response:
        counters = json.load(response)
    with sqlite3.connect(db.resolve().as_uri() + "?mode=ro", uri=True) as conn:
        # MAX(id) avoids full-table scans and is independent of retention deletes.
        latest_id = conn.execute("SELECT coalesce(max(id),0) FROM observations").fetchone()[0]
        latest_ts = conn.execute("SELECT ts FROM observations ORDER BY id DESC LIMIT 1").fetchone()
    return counters, latest_id, latest_ts[0] if latest_ts else 0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8128")
    parser.add_argument("--db", type=Path, default=Path("data/ble-watch.db"))
    parser.add_argument("--adapter", default="hci0")
    parser.add_argument("--seconds", type=int, default=45)
    args = parser.parse_args()
    if not 1 <= args.seconds <= 300 or not args.adapter.isalnum():
        parser.error("use 1..300 seconds and an alphanumeric adapter name")
    try:
        discovery = subprocess.run(
            ["busctl", "get-property", "org.bluez", "/org/bluez/" + args.adapter,
             "org.bluez.Adapter1", "Discovering"], capture_output=True, text=True, timeout=5,
        )
        scanning = discovery.returncode == 0 and discovery.stdout.strip() == "b true"
        before, before_id, _ = snapshot(args.base_url, args.db)
        time.sleep(args.seconds)
        after, after_id, after_ts = snapshot(args.base_url, args.db)
        processed = after["processed"] - before["processed"]
        advanced = after_id > before_id
        passed = scanning and processed > 0 and advanced and after_ts >= int(time.time()) - args.seconds - 5
        print(json.dumps({
            "status": "passed" if passed else "blocked",
            "bluez_discovering": scanning,
            "sample_seconds": args.seconds,
            "processed_delta": processed,
            "observation_id_advanced": advanced,
            "newest_observation_age_seconds": max(0, int(time.time()) - after_ts) if after_ts else None,
        }))
        return 0 if passed else 2
    except Exception as exc:
        # Exception text can contain local paths, URLs, or source data.
        print(json.dumps({"status": "error", "error_type": type(exc).__name__}))
        return 1


if __name__ == "__main__":
    sys.exit(main())
