#!/usr/bin/env python3
"""Create database secrets once; never rotate or overwrite existing credentials."""
import argparse
import json
import os
from pathlib import Path
import secrets

parser = argparse.ArgumentParser()
parser.add_argument("directory", type=Path)
parser.add_argument("--local", action="store_true")
args = parser.parse_args()
args.directory.mkdir(parents=True, exist_ok=True, mode=0o700)

def write(name, content, mode):
    path = args.directory / name
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    except FileExistsError:
        return path.read_text().strip()
    with os.fdopen(fd, "w") as stream:
        stream.write(content + "\n")
    return content

owner = write("postgres-password", secrets.token_hex(32), 0o600)
write("admin-database", f"postgres://art_owner:{owner}@postgres:5432/art?sslmode=disable", 0o600)
for role in ("web", "renderer"):
    password = write(f"{role}-password", secrets.token_hex(32), 0o600)
    path = args.directory / f"{role}-database"
    write(path.name, f"postgres://art_{role}:{password}@postgres:5432/art?sslmode=disable", 0o440)
    if os.geteuid() == 0:
        os.chown(path, 0, 10000)
if args.local:
    for role in ("web", "renderer"):
        path = args.directory / f"{role}-objects"
        write(path.name, json.dumps({"access_key": "disposable-local", "secret_key": "disposable-local-password"}), 0o440)
        if os.geteuid() == 0:
            os.chown(path, 0, 10000)
