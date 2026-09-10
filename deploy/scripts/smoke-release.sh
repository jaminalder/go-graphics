#!/usr/bin/env bash
# A separate, generation-disabled candidate; no public ports or shared volumes.
set -euo pipefail
candidate=${1:?absolute candidate release directory}
[[ "$candidate" = /* && -f "$candidate/images.env" ]] || exit 1
project="art-smoke-$$"
compose(){
 env -u ART_APP_IMAGE -u ART_EDGE_IMAGE ART_DOMAIN=http://localhost ART_ORIGIN=http://localhost \
 ART_BIND=127.0.0.1 ART_HTTP_PORT=0 ART_HTTPS_PORT=0 ART_PROXY_NET=172.30.81 ART_GENERATION=off \
 docker compose --project-name "$project" --env-file "$candidate/images.env" -f "$candidate/deploy/compose.yaml" "$@"
}
cleanup(){ compose down --volumes >/dev/null 2>&1 || true; }
trap cleanup EXIT
compose config --quiet
compose run --rm --no-deps caddy caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
compose up -d --no-build --pull never --wait --wait-timeout 60 renderer web
compose exec -T web /app/artctl ready
compose exec -T web /app/artctl live
