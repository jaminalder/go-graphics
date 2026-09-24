#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
source "$root/deploy/scripts/local-persistence.sh"
trap cleanup_persistence EXIT
compose config --quiet
compose build web caddy
start_persistence
python3 "$root/deploy/tests/test_compose.py"
