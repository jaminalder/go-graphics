#!/usr/bin/env bash
# Candidate database and local bucket never use production state or credentials.
set -euo pipefail
candidate=${1:?absolute candidate release directory}
[[ "$candidate" = /* && -f "$candidate/images.env" ]] || exit 1
export ART_APP_IMAGE ART_EDGE_IMAGE ART_REVISION
ART_APP_IMAGE=$(sed -n 's/^ART_APP_IMAGE=//p' "$candidate/images.env")
ART_EDGE_IMAGE=$(sed -n 's/^ART_EDGE_IMAGE=//p' "$candidate/images.env")
ART_REVISION=$(sed -n 's/^ART_REVISION=//p' "$candidate/images.env")
export ART_COMPOSE_PROJECT="art-smoke-$$" ART_HTTP_PORT=0 ART_HTTPS_PORT=0 ART_PROXY_NET=172.30.81
export ART_LOCAL_ROOT="$candidate"
source "$candidate/deploy/scripts/local-persistence.sh"
trap cleanup_persistence EXIT
start_persistence
compose exec -T web /app/artctl ready
compose exec -T web /app/artctl live
