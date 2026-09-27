#!/usr/bin/env python3
"""Audit tracked publication content. Findings report locations, never values."""

import re
import subprocess
import sys
from pathlib import Path


RULES = {
    "private-path": re.compile(r"/(?:home|Users)/[A-Za-z0-9_-]+"),
    "private-network": re.compile(r"\b(?:192\.168\.\d+\.\d+|10\.\d+\.\d+\.\d+|172\.(?:1[6-9]|2\d|3[01])\.\d+\.\d+)\b"),
    "email": re.compile(r"[\w.+-]+@[\w.-]+\.[a-zA-Z]{2,}"),
    "credential": re.compile(r"(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[A-Z0-9]{16}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|https?://[^\s/:]+:[^\s/@]+@)"),
    "em-dash": re.compile(r"\u2014"),
    "emoji": re.compile(r"[\U0001F300-\U0001FAFF\u2600-\u27BF]"),
}
MAC = re.compile(r"\b(?:[0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}\b")
EXAMPLES = {"AA:BB:CC:DD:EE:FF", "02:00:00:00:00:01"}


def git(*args):
    return subprocess.check_output(["git", *args])


def main():
    findings = []
    paths = git("ls-files", "-z").decode().split("\0")
    for name in filter(None, paths):
        path = Path(name)
        if (name == "data/known-devices.json" or name.startswith(("test-results/", "playwright-report/"))
                or path.suffix in {".db", ".sqlite", ".sqlite3", ".pid", ".log", ".png", ".jpg", ".zip"}
                or path.name == ".env"):
            findings.append((name, 0, "runtime-or-private-artifact"))
        # Read the index so ignored private local files cannot enter the scan.
        raw = git("show", ":" + name)
        try:
            content = raw.decode("utf-8")
        except UnicodeDecodeError:
            findings.append((name, 0, "binary-needs-review"))
            continue
        for number, line in enumerate(content.splitlines(), 1):
            for kind, pattern in RULES.items():
                if pattern.search(line):
                    findings.append((name, number, kind))
            if any(match.upper() not in EXAMPLES for match in MAC.findall(line)):
                findings.append((name, number, "non-fixture-device-address"))
    for name, number, kind in findings:
        print(f"{name}:{number}: {kind}")
    print(f"Tracked content audit: {len(findings)} finding(s). Manual review is still required.")
    return bool(findings)


if __name__ == "__main__":
    sys.exit(main())
