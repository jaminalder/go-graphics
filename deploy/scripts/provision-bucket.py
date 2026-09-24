#!/usr/bin/env python3
"""Explicit private S3 bucket bootstrap via AWS CLI; credentials stay in protected files.

This command makes billable provider changes only when explicitly invoked by an operator.
It does not alter the VPS Terraform state or configure destructive expiry rules.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
from urllib.parse import urlsplit

p = argparse.ArgumentParser()
p.add_argument("--endpoint", required=True)
p.add_argument("--region", required=True)
p.add_argument("--bucket", required=True)
p.add_argument("--credentials", required=True, type=Path)
a = p.parse_args()
u = urlsplit(a.endpoint)
if u.scheme != "https" or not u.hostname or u.username or u.query or u.fragment:
    raise SystemExit("verified HTTPS endpoint required")
credentials = json.loads(a.credentials.read_text())
for field in ("access_key", "secret_key"):
    if not credentials.get(field) or any(c in credentials[field] for c in "\r\n"):
        raise SystemExit("invalid credentials file")
with tempfile.TemporaryDirectory(prefix="art-bucket-") as directory:
    path = Path(directory) / "credentials"
    path.write_text(f'[default]\naws_access_key_id={credentials["access_key"]}\naws_secret_access_key={credentials["secret_key"]}\n')
    path.chmod(0o600)
    env = {k: v for k, v in os.environ.items() if not k.startswith("AWS_")}
    env.update(AWS_SHARED_CREDENTIALS_FILE=str(path), AWS_EC2_METADATA_DISABLED="true", AWS_DEFAULT_REGION=a.region)
    base = ["aws", "--endpoint-url", a.endpoint, "--region", a.region, "s3api"]
    # ListBuckets proves account authentication first: do not treat a permission failure as absence.
    result = subprocess.run(base + ["list-buckets"], env=env, check=True, capture_output=True, text=True)
    owned = {b["Name"] for b in json.loads(result.stdout)["Buckets"]}
    if a.bucket not in owned:
        subprocess.run(base + ["create-bucket", "--bucket", a.bucket, "--acl", "private",
                               "--create-bucket-configuration", f"LocationConstraint={a.region}"], env=env, check=True)
    result = subprocess.run(base + ["get-bucket-acl", "--bucket", a.bucket], env=env, check=True, capture_output=True, text=True)
    if any(g["Grantee"].get("Type") == "Group" for g in json.loads(result.stdout)["Grants"]):
        raise SystemExit("bucket contains a group grant; review access before application use")
    print("Private owner ACL verified. Verify bucket policy/credentials with the provider contract test before deployment.")
