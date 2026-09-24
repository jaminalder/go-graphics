#!/usr/bin/env bash
set -euo pipefail
release=${1:?release directory}; shift
exec env -u ART_APP_IMAGE -u ART_EDGE_IMAGE -u ART_REVISION docker compose \
 --project-name singular-seed-admin --env-file /etc/art/operator.env --env-file /etc/art/storage.env \
 --env-file "$release/images.env" -f "$release/deploy/compose.admin.yaml" run --rm --no-deps admin "$@"
