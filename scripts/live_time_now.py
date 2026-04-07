#!/usr/bin/env python3
from __future__ import annotations

import os
import signal
import subprocess
import sys
import time
from datetime import datetime
from pathlib import Path


def env(name: str, default: str) -> str:
    value = os.environ.get(name, "").strip()
    return value or default


AIP2P_BIN = env("AIP2P_BIN", "/Users/haoniu/go/bin/aip2p")
MASTER_IDENTITY = env("AIP2P_MASTER_IDENTITY", "/Users/haoniu/.aip2p/identities/pc75.json")
CHILD_IDENTITY = env("AIP2P_IDENTITY", "/Users/haoniu/.aip2p/identities/pc75-now-time.json")
AUTHOR = env("AIP2P_AUTHOR", "agent://pc75/now-time")
STORE = env("AIP2P_STORE", "/Users/haoniu/.aip2p/aip2p/.aip2p")
BASE_NET = env("AIP2P_BASE_NET", "/Users/haoniu/.aip2p/aip2p_live_net.inf")
NET = env("AIP2P_NET", "/Users/haoniu/.aip2p/aip2p_live_sender_net.inf")
ROOM_ID = env("AIP2P_ROOM_ID", "public-live-time")
ROOM_TITLE = env("AIP2P_ROOM_TITLE", "Live-Time")
CHANNEL = env("AIP2P_CHANNEL", "aip2p/live/public")
INTERVAL_SECONDS = int(env("AIP2P_INTERVAL_SECONDS", "60"))
AUTO_ARCHIVE = env("AIP2P_ARCHIVE_ON_EXIT", "false").lower() in {"1", "true", "yes", "on"}

DEFAULT_SENDER_TCP_PORT = "51585"
DEFAULT_SENDER_QUIC_PORT = "51585"


def ensure_child_identity() -> None:
    child = Path(CHILD_IDENTITY)
    if child.exists():
        return
    child.parent.mkdir(parents=True, exist_ok=True)
    cmd = [
        AIP2P_BIN,
        "identity",
        "derive",
        "--identity-file",
        MASTER_IDENTITY,
        "--author",
        AUTHOR,
        "--out",
        CHILD_IDENTITY,
    ]
    print(f"[live-time] derive child identity -> {CHILD_IDENTITY}", file=sys.stderr)
    subprocess.run(cmd, check=True)


def ensure_sender_net() -> None:
    net_path = Path(NET)
    if net_path.exists():
        return
    base_path = Path(BASE_NET)
    if not base_path.exists():
        raise FileNotFoundError(f"base live net config not found: {BASE_NET}")

    lines = base_path.read_text(encoding="utf-8").splitlines()
    rendered: list[str] = []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("libp2p_listen=/ip4/0.0.0.0/tcp/"):
            rendered.append(f"libp2p_listen=/ip4/0.0.0.0/tcp/{DEFAULT_SENDER_TCP_PORT}")
            continue
        if stripped.startswith("libp2p_listen=/ip4/0.0.0.0/udp/") and stripped.endswith("/quic-v1"):
            rendered.append(f"libp2p_listen=/ip4/0.0.0.0/udp/{DEFAULT_SENDER_QUIC_PORT}/quic-v1")
            continue
        rendered.append(line)

    net_path.parent.mkdir(parents=True, exist_ok=True)
    net_path.write_text("\n".join(rendered).rstrip() + "\n", encoding="utf-8")
    print(f"[live-time] created sender net -> {NET}", file=sys.stderr)


def build_host_cmd() -> list[str]:
    return [
        AIP2P_BIN,
        "live",
        "host",
        "--store",
        STORE,
        "--net",
        NET,
        "--identity-file",
        CHILD_IDENTITY,
        "--author",
        AUTHOR,
        "--room-id",
        ROOM_ID,
        "--title",
        ROOM_TITLE,
        "--channel",
        CHANNEL,
        "--archive-on-exit=" + ("true" if AUTO_ARCHIVE else "false"),
    ]


def format_message() -> str:
    now = datetime.now().astimezone()
    return (
        "当前时间："
        + now.strftime("%Y-%m-%d %H:%M:%S %Z")
        + " | ISO="
        + now.isoformat(timespec="seconds")
    )


def write_line(proc: subprocess.Popen[str], line: str) -> None:
    if proc.stdin is None:
        raise RuntimeError("live host stdin unavailable")
    proc.stdin.write(line + "\n")
    proc.stdin.flush()
    print(f"[live-time] sent: {line}", file=sys.stderr)


def terminate_process(proc: subprocess.Popen[str]) -> int:
    if proc.poll() is not None:
        return proc.returncode or 0
    try:
        proc.send_signal(signal.SIGINT)
        return proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.terminate()
        try:
            return proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            return proc.wait(timeout=5)


def main() -> int:
    ensure_child_identity()
    ensure_sender_net()
    cmd = build_host_cmd()
    print("[live-time] room page: http://127.0.0.1:51818/live/public/live-time", file=sys.stderr)
    print("[live-time] starting:", " ".join(cmd), file=sys.stderr)
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        text=True,
    )

    stop = False

    def handle_signal(signum: int, _frame) -> None:
        nonlocal stop
        stop = True
        print(f"[live-time] signal {signum}, stopping", file=sys.stderr)

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    try:
        time.sleep(2)
        write_line(proc, format_message())
        while not stop:
            for _ in range(INTERVAL_SECONDS):
                if stop:
                    break
                if proc.poll() is not None:
                    return proc.returncode or 0
                time.sleep(1)
            if stop:
                break
            if proc.poll() is not None:
                return proc.returncode or 0
            write_line(proc, format_message())
        return terminate_process(proc)
    finally:
        if proc.poll() is None:
            terminate_process(proc)


if __name__ == "__main__":
    raise SystemExit(main())
