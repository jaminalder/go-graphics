#!/usr/bin/env bash
# Disposable local Linux-container checks. Only loopback ports; no cloud or TLS.
set -euo pipefail
cd "$(dirname "$0")/../.."
export ART_DOMAIN=http://localhost ART_ORIGIN=http://localhost:18088
export ART_BIND=127.0.0.1 ART_BIND6='[::1]' ART_HTTP_PORT=18088 ART_HTTPS_PORT=18443 ART_PROXY_NET=172.30.82
export ART_COMPOSE_PROJECT="art-check-$$"
compose(){ docker compose -p "$ART_COMPOSE_PROJECT" --env-file deploy/local.env -f deploy/compose.yaml "$@"; }
cleanup(){ compose down --volumes; }
trap cleanup EXIT
compose config --quiet
compose build web caddy
compose up -d --no-build --wait --wait-timeout 60
python3 deploy/tests/test_compose.py
