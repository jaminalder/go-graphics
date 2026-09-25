#!/usr/bin/env bash
# Exercise the actual k6 journeys against an isolated, disposable Compose stack.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
export ART_HTTP_PORT=18089 ART_HTTPS_PORT=18445 ART_PROXY_NET=172.30.84
export ART_ORIGIN=http://127.0.0.1:18089
source "$root/deploy/scripts/local-persistence.sh"
trap cleanup_persistence EXIT
compose build web caddy
start_persistence
python3 "$root/deploy/scripts/load.py" --project "$ART_COMPOSE_PROJECT" --scenario browse --users 1 --iterations 1 --duration 30s --think 0
python3 "$root/deploy/scripts/load.py" --project "$ART_COMPOSE_PROJECT" --scenario studio --users 1 --iterations 1 --duration 3m --artworks iris --download-every 1 --similar-every 1 --favourite-every 1 --think 0
image=$(data exec -T postgres psql -U art_owner -d art -Atc 'SELECT request FROM art_artifacts ORDER BY created DESC LIMIT 1')
python3 "$root/deploy/scripts/load.py" --project "$ART_COMPOSE_PROJECT" --scenario browse --users 1 --iterations 1 --duration 30s --image-keys "$image" --think 0
python3 "$root/deploy/scripts/load.py" --project "$ART_COMPOSE_PROJECT" --scenario burst --users 3 --duration 20s --artworks pools,foam,iris --think 0
