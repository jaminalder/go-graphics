#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
compose=(docker compose -f "$root/deploy/compose.test.yaml")
cleanup(){ "${compose[@]}" down --volumes >/dev/null; }
trap cleanup EXIT
"${compose[@]}" up -d --wait --wait-timeout 90
export ART_TEST_DATABASE_URL='postgres://art:disposable-test-password@127.0.0.1:15439/art_test?sslmode=disable'
go test -race -v -count=1 ./internal/persistence
go test -race -count=1 ./internal/studio ./internal/web ./internal/objectstore
