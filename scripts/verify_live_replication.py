#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
import urllib.request

LOCAL = 'http://127.0.0.1:51818/api/live/public/live-time'
REMOTE = 'http://192.168.102.74:51818/api/live/public/live-time'


def fetch(url: str) -> dict:
    with urllib.request.urlopen(url, timeout=8) as resp:
        return json.load(resp)


def summarize(name: str, data: dict) -> None:
    events = data.get('events') or []
    latest = events[-1] if events else {}
    payload = latest.get('payload') or {}
    print(f'[{name}] visible={data.get("visible_event_count")} total={data.get("total_event_count")}')
    print(f'[{name}] latest_ts={latest.get("timestamp", "none")}')
    print(f'[{name}] latest_content={payload.get("content", "")}')


def main() -> int:
    local = fetch(LOCAL)
    remote = fetch(REMOTE)
    summarize('local', local)
    summarize('remote', remote)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
