#!/usr/bin/env bash
# Disposable measured renderer load, retaining deadlines and per-visitor bounds.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
export ART_HTTP_PORT=18090 ART_HTTPS_PORT=18446 ART_PROXY_NET=172.30.85
export ART_ORIGIN=http://127.0.0.1:18090 ART_LIMITS_PROFILE=local-capacity
source "$root/deploy/scripts/local-persistence.sh"
trap cleanup_persistence EXIT
compose build web caddy
start_persistence
for replicas in 1 2; do
 compose up -d --no-deps --no-build --scale "renderer=$replicas" --wait renderer
 python3 "$root/deploy/scripts/load.py" --project "$ART_COMPOSE_PROJECT" --scenario studio --users 20 --duration 30s \
   --artworks iris --download-every 0 --similar-every 0 --favourite-every 0 --think 0
done
