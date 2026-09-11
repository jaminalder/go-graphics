#!/usr/bin/env python3
"""Terraform's local deployment task: verify, wait for cloud-init, upload, activate.

SSH handles the local identity. No private key or cloud credential is read here
or sent to the server. A failed task is retried independently of server creation.
"""
import hashlib
import ipaddress
import os
from pathlib import Path
import re
import shlex
import subprocess
import tarfile
import tempfile
import time
from urllib.parse import urlsplit
from urllib.request import ProxyHandler, build_opener


def checksum(path):
    with path.open("rb") as stream:
        digest = hashlib.sha256()
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
        return digest.hexdigest()


def verify_release(path, expected_digest):
    """Reject changed or incomplete artifacts before making an SSH connection."""
    manifest_path = path / "SHA256SUMS"
    if checksum(manifest_path) != expected_digest:
        raise ValueError("Release checksum manifest changed since terraform plan")
    covered = set()
    for line in manifest_path.read_text().splitlines():
        digest, filename = line.split("  ", 1)
        relative = Path(filename)
        if relative.is_absolute() or ".." in relative.parts:
            raise ValueError("Release contains an unsafe path")
        target = path / relative
        if target.is_symlink() or not target.is_file() or checksum(target) != digest:
            raise ValueError(f"Release checksum mismatch: {filename}")
        covered.add(relative.as_posix())
    actual = set()
    for item in path.rglob("*"):
        if item.is_symlink():
            raise ValueError("Release must not contain symlinks")
        if item.is_file():
            actual.add(item.relative_to(path).as_posix())
    if actual != covered | {"SHA256SUMS"}:
        raise ValueError("Release contains files not listed in SHA256SUMS")
    required = {"images.tar", "images.env", "manifest.txt", "deploy/scripts/activate-release.sh", "deploy/compose.yaml"}
    if not required <= covered:
        raise ValueError("Release is missing required files")
    manifest = dict(line.split("=", 1) for line in (path / "manifest.txt").read_text().splitlines() if "=" in line)
    revision = manifest.get("source", "")
    if not re.fullmatch("[0-9a-f]{40}", revision) or manifest.get("platform") != "linux/amd64":
        raise ValueError("Expected a committed Linux amd64 release")
    return revision


def wait_for_ssh(ssh, timeout=300):
    deadline = time.monotonic() + timeout
    last_error = ""
    print("Waiting for the operator account and SSH (up to five minutes)...", flush=True)
    while time.monotonic() < deadline:
        try:
            result = subprocess.run(ssh + ["true"], capture_output=True, text=True, timeout=15)
            if result.returncode == 0:
                return
            last_error = result.stderr.strip()
        except subprocess.TimeoutExpired:
            last_error = "SSH connection timed out"
        time.sleep(3)
    raise RuntimeError(f"SSH did not become ready. Check the identity/agent and cloud-init logs. Last error: {last_error}")


def provision(settings):
    address = str(ipaddress.IPv4Address(settings["ART_SERVER_IP"]))
    server_id = settings["ART_SERVER_ID"]
    if not server_id.isdecimal():
        raise ValueError("Invalid server ID")
    origin = settings["ART_SITE_ORIGIN"]
    site = urlsplit(origin)
    if site.scheme not in {"http", "https"} or not re.fullmatch("[a-z0-9.-]+", site.netloc) or site.path or site.query or site.fragment:
        raise ValueError("Expected an HTTP(S) origin without port, credentials or path")
    if site.scheme == "http" and site.netloc != address:
        raise ValueError("HTTP origin must use the assigned server IPv4")
    release = Path(settings["ART_RELEASE_PATH"]).resolve()
    revision = verify_release(release, settings["ART_RELEASE_DIGEST"])
    known_hosts = Path("out/provision/known_hosts").resolve()
    known_hosts.parent.mkdir(parents=True, exist_ok=True)
    options = ["-F", "/dev/null", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
               "-o", "ConnectTimeout=5", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=4",
               "-o", "StrictHostKeyChecking=accept-new", "-o", f"HostKeyAlias=singular-seed-{server_id}",
               "-o", f"UserKnownHostsFile={known_hosts}", "-i", str(Path(settings["ART_SSH_IDENTITY"]).expanduser())]
    destination = f"operator@{address}"
    ssh = ["ssh", *options, destination]
    wait_for_ssh(ssh)
    print("Waiting for cloud-init to install Docker and the host firewall...", flush=True)
    subprocess.run(ssh + ["sudo cloud-init status --wait --long"], check=True, timeout=1200)
    remote = subprocess.check_output(ssh + ["mktemp -d /tmp/art-upload.XXXXXXXX"], text=True, timeout=30).strip()
    if not re.fullmatch(r"/tmp/art-upload\.[a-zA-Z0-9]+", remote):
        raise ValueError("Unexpected upload directory from server")
    try:
        with tempfile.TemporaryDirectory(prefix="art-upload-") as directory:
            archive = Path(directory) / "release.tar"
            with tarfile.open(archive, "w") as bundle:
                for entry in sorted(release.iterdir()):
                    bundle.add(entry, arcname=entry.name)
            digest = checksum(archive)
            print(f"Uploading release {revision}...", flush=True)
            subprocess.run(["scp", *options, str(archive), f"{destination}:{remote}/release.tar"], check=True, timeout=1200)
            command = shlex.join(["sudo", "/usr/local/sbin/art-install-release", f"{remote}/release.tar", digest, revision, origin])
            subprocess.run(ssh + [command], check=True, timeout=1200)
    finally:
        try:
            subprocess.run(ssh + [shlex.join(["rm", "-rf", "--", remote])], check=False, timeout=30)
        except (OSError, subprocess.TimeoutExpired) as error:
            print(f"Upload cleanup failed: {error}", flush=True)
    # Check from the operator machine as well as the server. Bypass local HTTP
    # proxies so this actually exercises the deployed origin.
    with build_opener(ProxyHandler({})).open(origin + "/", timeout=30) as response:
        if response.status != 200:
            raise RuntimeError(f"Unexpected site status: {response.status}")
    print(f"Application ready: {origin}", flush=True)


if __name__ == "__main__":
    provision(os.environ)
