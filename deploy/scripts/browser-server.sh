#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
mkdir -p out
# Separate test endpoints/cache from any manual preview session.
go build -o out/browser-artrender ./cmd/artrender
go build -o out/browser-artweb ./cmd/artweb
ART_SOCKET=out/browser-render.sock out/browser-artrender &
renderer_pid=$!
web_pid=""
cleanup() {
  if [[ -n "$web_pid" ]]; then kill "$web_pid" 2>/dev/null || true; fi
  kill "$renderer_pid" 2>/dev/null || true
  if [[ -n "$web_pid" ]]; then wait "$web_pid" 2>/dev/null || true; fi
  wait "$renderer_pid" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
ART_ADDR="${ART_BROWSER_ADDR:-127.0.0.1:8280}" ART_ADMIN_ADDR="${ART_BROWSER_ADMIN_ADDR:-127.0.0.1:8281}" ART_ORIGIN="http://${ART_BROWSER_ADDR:-127.0.0.1:8280}" ART_SOCKET=out/browser-render.sock ART_CACHE=out/browser-cache out/browser-artweb &
web_pid=$!
wait "$web_pid"
