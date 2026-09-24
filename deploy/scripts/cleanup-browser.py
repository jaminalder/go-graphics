#!/usr/bin/env python3
"""Clean only the disposable projects recorded by the browser test runner."""
import json
from pathlib import Path
import re
import shutil
import subprocess

root = Path(__file__).resolve().parents[2]
manifest = root / "out/browser-runtime.json"
if not manifest.exists():
    raise SystemExit(0)
state = json.loads(manifest.read_text())
project = state["project"]
if not re.fullmatch(r"art-persistent-[0-9]+", project):
    raise SystemExit("refusing unexpected browser project")
for name in (project, project + "-admin", project + "-data"):
    label = f"label=com.docker.compose.project={name}"
    for kind, listing, removal in (
        ("container", ["ps", "-aq"], ["rm", "-f"]),
        ("network", ["network", "ls", "-q"], ["network", "rm"]),
        ("volume", ["volume", "ls", "-q"], ["volume", "rm"]),
    ):
        ids = subprocess.check_output(["docker", *listing, "--filter", label], text=True).split()
        if ids:
            subprocess.run(["docker", *removal, *ids], check=True, stdout=subprocess.DEVNULL)
secret = Path(state["secrets"])
if secret.parent == root / "out" and secret.name.startswith("persistence-secrets.") and secret.is_dir():
    shutil.rmtree(secret)
manifest.unlink()
