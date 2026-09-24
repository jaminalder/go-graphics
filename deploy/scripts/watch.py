#!/usr/bin/env python3
"""Read-only terminal monitor for local/hosted Compose, with actual container names.

Docker access remains on the operator host. No daemon or socket mount is required.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True, stderr=subprocess.PIPE, timeout=10)


def containers(project):
    ids = docker("ps", "-aq", "--filter", f"label=com.docker.compose.project={project}").split()
    return json.loads(docker("inspect", *ids)) if ids else []


def refresh(project, names, as_json=False):
    candidates = []
    # Also discovers replicas added after this command started. Names retained in
    # this watch session remain readable after the corresponding container is removed.
    for container in containers(project):
        config = container["Config"]
        labels = config.get("Labels") or {}
        if labels.get("com.docker.compose.service") not in ("web", "renderer"):
            continue
        if labels.get("com.docker.compose.oneoff", "false").lower() == "true":
            continue
        names[config["Hostname"]] = container["Name"].lstrip("/")
        if labels.get("com.docker.compose.service") == "web" and container["State"]["Running"]:
            candidates.append(container["Id"])
    if not candidates:
        raise RuntimeError("no running web container; database status cannot be fetched through its private endpoint")
    # Bound the optional display cache; IDs are still shown if an old name was lost.
    while len(names) > 1000:
        del names[next(iter(names))]
    last = None
    for identifier in candidates:
        try:
            return docker("exec", "-e", "ART_MONITOR_NAMES=" + json.dumps(names),
                          identifier, "/app/artctl", "status", *(["--json"] if as_json else []))
        except subprocess.CalledProcessError as error:
            last = error
    raise RuntimeError("web monitor unavailable (restarting, database down, or older image); retrying") from last


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", help="Compose project; defaults to local browser manifest, otherwise singular-seed")
    parser.add_argument("--interval", type=float, default=2, help="refresh interval in seconds (1–60)")
    parser.add_argument("--once", action="store_true", help="print one snapshot")
    parser.add_argument("--json", action="store_true", help="one raw JSON snapshot (implies --once)")
    args = parser.parse_args()
    if not 1 <= args.interval <= 60:
        parser.error("interval must be between 1 and 60 seconds")
    project = args.project
    if not project:
        manifest = Path(__file__).resolve().parents[2] / "out/browser-runtime.json"
        project = json.loads(manifest.read_text())["project"] if manifest.exists() else "singular-seed"
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]*", project):
        parser.error("invalid Compose project")
    names = {}
    terminal = sys.stdout.isatty() and os.environ.get("TERM") != "dumb"
    while True:
        failed = False
        try:
            output = refresh(project, names, args.json)
        except (RuntimeError, subprocess.SubprocessError, OSError, ValueError):
            failed = True
            output = "MONITOR UNAVAILABLE — no current snapshot. Check Docker, web and PostgreSQL.\n"
        if terminal and not args.once and not args.json:
            print("\033[H\033[2J", end="")
        print(output, end="", flush=True)
        if args.once or args.json:
            return int(failed)
        time.sleep(args.interval)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        pass
