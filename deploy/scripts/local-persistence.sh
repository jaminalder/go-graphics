#!/usr/bin/env bash
# Sourced by disposable local validation entry points; never production secrets.
set -euo pipefail
root=${ART_LOCAL_ROOT:-$(git rev-parse --show-toplevel)}
export ART_COMPOSE_PROJECT=${ART_COMPOSE_PROJECT:-art-persistent-$$}
export ART_DATA_PROJECT="${ART_COMPOSE_PROJECT}-data"
export ART_DATABASE_NETWORK="${ART_DATA_PROJECT}_default"
export ART_SECRETS_DIR
mkdir -p "$root/out"
ART_SECRETS_DIR=$(mktemp -d "$root/out/persistence-secrets.XXXXXXXX")
export ART_S3_ENDPOINT=http://objects:9000 ART_S3_REGION=us-east-1 ART_S3_BUCKET=art-local ART_S3_LOCAL=true ART_S3_PREFIX=artifacts
export ART_LIMITS_PROFILE=${ART_LIMITS_PROFILE:-local-capacity}
export ART_LIMITS_JSON=${ART_LIMITS_JSON:-}
export ART_ORIGIN=${ART_ORIGIN:-http://localhost:18088}
export ART_DOMAIN
ART_DOMAIN="http://$(python3 -c 'import os,urllib.parse; print(urllib.parse.urlparse(os.environ["ART_ORIGIN"]).hostname)')"
export ART_BIND=127.0.0.1 ART_BIND6='[::1]' ART_HTTP_PORT=${ART_HTTP_PORT:-18088} ART_HTTPS_PORT=${ART_HTTPS_PORT:-18443} ART_PROXY_NET=${ART_PROXY_NET:-172.30.82}
python3 "$root/deploy/scripts/init-secrets.py" "$ART_SECRETS_DIR" --local
# Disposable secrets must be readable by container UIDs; parent directory is 0700.
chmod 0444 "$ART_SECRETS_DIR"/*-database "$ART_SECRETS_DIR"/*-objects
data(){ docker compose -p "$ART_DATA_PROJECT" -f "$root/deploy/compose.data.yaml" -f "$root/deploy/compose.objects-local.yaml" "$@"; }
compose(){ docker compose -p "$ART_COMPOSE_PROJECT" -f "$root/deploy/compose.yaml" "$@"; }
admin(){ docker compose -p "${ART_COMPOSE_PROJECT}-admin" -f "$root/deploy/compose.admin.yaml" run --rm --no-deps admin "$@"; }
cleanup_persistence(){
 compose down --volumes >/dev/null 2>&1 || true
 data down --volumes >/dev/null 2>&1 || true
 docker compose -p "${ART_COMPOSE_PROJECT}-admin" -f "$root/deploy/compose.admin.yaml" down >/dev/null 2>&1 || true
 rm -rf "$ART_SECRETS_DIR"
}
start_persistence(){
 data up -d --wait --wait-timeout 90
 admin migrate
 python3 "$root/deploy/scripts/database-roles.py" "$ART_SECRETS_DIR" docker compose -p "$ART_DATA_PROJECT" -f "$root/deploy/compose.data.yaml"
 for attempt in $(seq 1 30); do if admin create-local-bucket; then break; fi; sleep 1; done
 admin bind-bucket
 admin activate
 admin enable
 compose up -d --no-build --wait --wait-timeout 90
 for attempt in $(seq 1 30); do if compose exec -T web /app/artctl ready; then return; fi; sleep 1; done
 return 1
}
