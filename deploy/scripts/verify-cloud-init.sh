#!/usr/bin/env bash
# Test the real Ubuntu account setup without touching a host or cloud resources.
set -euo pipefail
cd "$(dirname "$0")/../.."
docker build -t singular-seed-cloud-init-check:local -f deploy/tests/cloud-init.Dockerfile deploy/tests
docker run --rm --network none --mount "type=bind,src=$PWD,dst=/repo,readonly" \
  singular-seed-cloud-init-check:local
