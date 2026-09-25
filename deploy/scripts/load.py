#!/usr/bin/env python3
"""Run pinned k6 against an existing local Compose studio or an explicit remote origin."""
import argparse
from datetime import datetime, timezone
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import threading
import uuid
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[2]
K6_IMAGE = "grafana/k6:1.6.1@sha256:a5ad6bc089a08d77c3ec49f3db8c6fa7a148e4073efcac44c675dbaf3568d8e1"


def duration(value):
    match = re.fullmatch(r"([1-9][0-9]*)(s|m|h)", value)
    if not match:
        raise argparse.ArgumentTypeError("use a duration such as 30s, 5m or 1h")
    seconds = int(match[1]) * {"s": 1, "m": 60, "h": 3600}[match[2]]
    if not 5 <= seconds <= 86400:
        raise argparse.ArgumentTypeError("duration must be between 5 seconds and 24 hours")
    return seconds


def bounded(low, high):
    def parse(value):
        try:
            number = int(value)
        except ValueError as error:
            raise argparse.ArgumentTypeError("expected an integer") from error
        if not low <= number <= high:
            raise argparse.ArgumentTypeError(f"expected {low}..{high}")
        return number
    return parse


def origin(value):
    parsed = urlsplit(value)
    try:
        port = parsed.port
    except ValueError as error:
        raise argparse.ArgumentTypeError("invalid target port") from error
    if parsed.scheme not in ("http", "https") or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment or parsed.path not in ("", "/"):
        raise argparse.ArgumentTypeError("target must be an HTTP(S) origin without credentials, path or query")
    if port is not None and not 1 <= port <= 65535:
        raise argparse.ArgumentTypeError("invalid target port")
    return value.rstrip("/")


def local_origin(value):
    host = urlsplit(value).hostname
    if host == "localhost":
        return True
    try:
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True, stderr=subprocess.PIPE, timeout=15)


def discover(project):
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]*", project):
        raise ValueError("invalid Compose project")
    ids = docker("ps", "-q", "--filter", f"label=com.docker.compose.project={project}").split()
    if not ids:
        raise ValueError(f"no running containers for {project}; start browser-server.sh first")
    containers = json.loads(docker("inspect", *ids))
    services = {}
    for container in containers:
        labels = container["Config"].get("Labels") or {}
        if labels.get("com.docker.compose.oneoff", "false").lower() == "true":
            continue
        services.setdefault(labels.get("com.docker.compose.service"), container)
    if "web" not in services or "caddy" not in services:
        raise ValueError("load testing needs a running web and Caddy in the selected project")
    env = dict(item.split("=", 1) for item in services["web"]["Config"]["Env"] if "=" in item)
    target = origin(env.get("ART_ORIGIN", ""))
    network = project + "_edge"
    address = services["caddy"]["NetworkSettings"]["Networks"].get(network, {}).get("IPAddress")
    if not address:
        raise ValueError("Caddy's expected edge network is unavailable")
    parsed = urlsplit(target)
    # k6 changes the dial address only, preserving URL Host, cookie domain and POST Origin.
    hosts = {parsed.netloc: address + (":443" if parsed.scheme == "https" else ":80")}
    return target, network, hosts


def private_command(project, *command):
    ids = docker("ps", "-q", "--filter", f"label=com.docker.compose.project={project}", "--filter", "label=com.docker.compose.service=web").split()
    for identifier in ids:
        try:
            return json.loads(docker("exec", identifier, "/app/artctl", *command))
        except (subprocess.SubprocessError, ValueError):
            continue
    raise ValueError("no reachable web private diagnostics (server evidence unavailable)")


class ServerEvidence:
    """Read-only private diagnostics from the operator host, never exposed to k6/browser."""
    def __init__(self, project, directory):
        self.project = project
        self.directory = directory
        self.stop = threading.Event()
        self.thread = None
        self.start_snapshot = None
        self.samples = []
        self.errors = 0

    def sample(self):
        try:
            snapshot = private_command(self.project, "status", "--json")
            if self.start_snapshot is None:
                self.start_snapshot = snapshot
            sample = {"at": snapshot["at"], "queue": snapshot["queue"], "limits": snapshot.get("limits"),
                      "instances": snapshot["instances"]}
            self.samples.append(sample)
            return snapshot
        except (ValueError, OSError, subprocess.SubprocessError):
            self.errors += 1
            return None

    def begin(self):
        if not self.project:
            return
        self.sample()
        def loop():
            while not self.stop.wait(2):
                self.sample()
        self.thread = threading.Thread(target=loop, daemon=True)
        self.thread.start()

    def finish(self):
        self.stop.set()
        if self.thread:
            self.thread.join()
        report = {"available": False, "scope": "all render jobs enqueued in the server-clock run window, including unrelated clients", "sample_errors": self.errors}
        if not self.project:
            report["reason"] = "remote target: no operator-local Docker access; collect artctl load-stats on the server separately"
        else:
            end = self.sample()
            if self.start_snapshot and end:
                since, until = self.start_snapshot["at"], end["at"]
                report.update(since=since, until=until, start_limits=self.start_snapshot.get("limits"), end_limits=end.get("limits"),
                              samples=self.samples, sample_errors=self.errors)
                try:
                    report["timings"] = private_command(self.project, "load-stats", "--since", since, "--until", until)
                    report["available"] = True
                except (ValueError, OSError, subprocess.SubprocessError):
                    report["reason"] = "job timing query unavailable; recorded snapshots retained"
            else:
                report["reason"] = "private diagnostics unavailable; no reliable server window"
        report["sample_errors"] = self.errors
        (self.directory / "server.json").write_text(json.dumps(report, indent=2) + "\n")
        if report["available"]:
            rows = report["timings"]
            jobs = sum(r["jobs"] for r in rows)
            completed = sum(r["jobs"] for r in rows if r["state"] == "completed")
            maximum = max((s["queue"]["running"] for s in self.samples), default=0)
            text = f"SERVER WINDOW {report['since']} → {report['until']}\nJobs enqueued: {jobs}; completed by snapshot: {completed}; max sampled running: {maximum}\n"
            for row in rows:
                text += f"{row['tier']} {row['last_renderer']} {row['state']}: {row['jobs']} jobs; queue avg={row['enqueue_to_last_attempt_avg_seconds']}s; execution avg={row['last_attempt_to_final_avg_seconds']}s; total p95={row['enqueue_to_final_p95_seconds']}s\n"
            text += "Window includes all clients; sampled running is not CPU utilization. Retried-job timings describe last attempt, not total CPU. Unfinished jobs remain unfinished, not failures.\n"
        else:
            text = "SERVER EVIDENCE UNAVAILABLE: " + report["reason"] + "\n"
        (self.directory / "server.txt").write_text(text)
        print(text, flush=True)


def parser():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--scenario", choices=("browse", "studio", "burst"), default="browse")
    p.add_argument("--users", type=bounded(1, 2000), help="concurrent VUs, or burst peak, 1–2000 (default browse=5, studio/burst=3)")
    p.add_argument("--duration", type=duration, default=120, help="load phase; graceful completion can extend runtime (default 2m)")
    p.add_argument("--project", help="local Compose project (default browser-runtime.json)")
    p.add_argument("--target", type=origin, help="canonical origin; explicit remote target requires --allow-remote")
    p.add_argument("--allow-remote", action="store_true", help="explicitly authorize load against a non-loopback origin")
    p.add_argument("--iterations", type=bounded(1, 10000), default=0, help="finite iterations per VU for smoke checks, instead of time/ramp profile")
    p.add_argument("--completion-timeout", type=bounded(5, 600), default=120, metavar="SECONDS")
    p.add_argument("--poll", type=bounded(2, 30), default=3, metavar="SECONDS")
    p.add_argument("--think", type=bounded(0, 60), default=2, metavar="SECONDS")
    p.add_argument("--seed", type=bounded(0, 2147483647), default=1)
    p.add_argument("--artworks", default="pools,foam,iris", help="comma-separated workload mix; repeat names for weighting")
    p.add_argument("--download-every", type=bounded(0, 10000), default=5, help="studio iteration frequency; 0 disables")
    p.add_argument("--similar-every", type=bounded(0, 10000), default=2)
    p.add_argument("--favourite-every", type=bounded(0, 10000), default=3)
    p.add_argument("--image-keys", default="", help="browse-only existing rendition keys, comma-separated; never creates them")
    p.add_argument("--dry-run", action="store_true", help="resolve target and print safe configuration without starting k6")
    return p


def main(argv=None):
    p = parser()
    args = p.parse_args(argv)
    if any(x not in ("pools", "foam", "iris") for x in args.artworks.split(",")):
        p.error("artworks must contain only pools,foam,iris")
    if args.image_keys and any(not re.fullmatch(r"[a-f0-9]{64}", x) for x in args.image_keys.split(",")):
        p.error("image keys must be 64 lowercase hexadecimal characters")
    if args.scenario != "browse" and args.image_keys:
        p.error("image keys are only used by browse")
    project = args.project
    network, hosts = None, {}
    if not args.target or local_origin(args.target) or project:
        if not project:
            manifest = ROOT / "out/browser-runtime.json"
            if not manifest.exists():
                p.error("start browser-server.sh or select --project; an external target needs --target and --allow-remote")
            project = json.loads(manifest.read_text())["project"]
        target, network, hosts = discover(project)
        if args.target and args.target != target:
            p.error("target differs from the selected web's canonical ART_ORIGIN")
    else:
        target = args.target
    if not local_origin(target) and not args.allow_remote:
        p.error("non-loopback target requires explicit --allow-remote")
    if args.scenario == "burst" and args.duration < 10 and not args.iterations:
        p.error("burst profile requires at least 10s")
    users = args.users or (5 if args.scenario == "browse" else 3)
    env = {
        "LOAD_SCENARIO": args.scenario, "LOAD_TARGET": target, "LOAD_HOSTS": json.dumps(hosts),
        "LOAD_USERS": str(users), "LOAD_SECONDS": str(args.duration), "LOAD_ITERATIONS": str(args.iterations),
        "LOAD_COMPLETION_TIMEOUT": str(args.completion_timeout), "LOAD_POLL_SECONDS": str(args.poll),
        "LOAD_THINK_SECONDS": str(args.think), "LOAD_SEED": str(args.seed), "LOAD_ARTWORKS": args.artworks,
        "LOAD_DOWNLOAD_EVERY": str(args.download_every), "LOAD_SIMILAR_EVERY": str(args.similar_every),
        "LOAD_FAVOURITE_EVERY": str(args.favourite_every), "LOAD_IMAGE_KEYS": args.image_keys,
        "K6_NO_USAGE_REPORT": "true",
    }
    config = {"scenario": args.scenario, "target": target, "project": project, "users": users,
              "duration_seconds": args.duration, "k6_image": K6_IMAGE, "environment": env,
              "script_sha256": {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in (ROOT / "web/load").glob("*.js") if not path.name.endswith(".test.js")},
              "policies": "deployment-configured limits unchanged by generator; HTTP status polling, no SSE"}
    if project:
        try:
            config["effective_limits"] = private_command(project, "status", "--json").get("limits")
        except (ValueError, OSError, subprocess.SubprocessError):
            config["effective_limits"] = None
    if args.dry_run:
        print(json.dumps(config, indent=2))
        return 0
    directory = ROOT / "out/load" / (datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + args.scenario + "-" + uuid.uuid4().hex[:8])
    directory.mkdir(parents=True)
    (directory / "config.json").write_text(json.dumps(config, indent=2) + "\n")
    name = "art-load-" + uuid.uuid4().hex[:12]
    command = ["docker", "run", "--rm", "--name", name, "--user", f"{os.getuid()}:{os.getgid()}",
               "--cap-drop=ALL", "--security-opt=no-new-privileges:true", "--read-only", "--tmpfs", "/tmp:size=32m",
               "--mount", f"type=bind,src={ROOT / 'web/load'},dst=/scripts,readonly",
               "--mount", f"type=bind,src={directory},dst=/results"]
    if network:
        command += ["--network", network]
    for key, value in env.items():
        command += ["-e", key + "=" + value]
    command += [K6_IMAGE, "run", "--no-color", "/scripts/scenarios.js"]
    print(f"Load target: {target} | scenario: {args.scenario} | users: {users}\nReports: {directory}", flush=True)
    exit_code = 1
    interrupted = False
    process = None
    evidence = ServerEvidence(project, directory)
    try:
        evidence.begin()
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        with (directory / "console.log").open("w") as log:
            for line in process.stdout:
                print(line, end="", flush=True)
                log.write(line)
        exit_code = process.wait()
    except KeyboardInterrupt:
        interrupted = True
        subprocess.run(["docker", "stop", "--signal=SIGINT", "--time=15", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
        if process:
            process.wait(timeout=20)
        exit_code = 130
    finally:
        # Only this invocation's random load-container name is removed, never application services.
        subprocess.run(["docker", "rm", "-f", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False, timeout=20)
        evidence.finish()
        (directory / "result.json").write_text(json.dumps({"exit_code": exit_code, "interrupted": interrupted,
            "summary_available": (directory / "summary.json").exists()}, indent=2) + "\n")
    print(f"\nReports: {directory}")
    return exit_code


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        sys.exit(f"Load runner failed: {error}")
