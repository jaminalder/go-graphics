#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
mkdir -p out
# Separate test endpoints/cache from any manual preview session.
go build -o out/browser-artrender ./cmd/artrender
go build -o out/browser-artweb ./cmd/artweb
ART_SOCKET=out/browser-render.sock out/browser-artrender &
renderer_pid=$!
trap 'kill "$renderer_pid" 2>/dev/null || true; wait "$renderer_pid" 2>/dev/null || true' EXIT
ART_SOCKET=out/browser-render.sock ART_CACHE=out/browser-cache out/browser-artweb
