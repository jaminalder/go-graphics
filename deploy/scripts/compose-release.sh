#!/usr/bin/env bash
# Operations against the one host project, with immutable release image IDs.
set -euo pipefail
release=${1:?usage: compose-release.sh RELEASE_DIRECTORY COMPOSE_ARGS...}
shift
exec env -u ART_APP_IMAGE -u ART_EDGE_IMAGE -u ART_REVISION docker compose \
 --project-name singular-seed --env-file /etc/art/operator.env \
 --env-file "$release/images.env" -f "$release/deploy/compose.yaml" "$@"
