#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
export ART_ORIGIN="http://${ART_BROWSER_ADDR:-127.0.0.1:8280}"
export ART_HTTP_PORT="${ART_BROWSER_ADDR:-127.0.0.1:8280}"
ART_HTTP_PORT=${ART_HTTP_PORT##*:}
export ART_HTTPS_PORT=18444 ART_PROXY_NET=172.30.83
source "$root/deploy/scripts/local-persistence.sh"
python3 -c 'import json,os,pathlib; pathlib.Path("out/browser-runtime.json").write_text(json.dumps({"project":os.environ["ART_COMPOSE_PROJECT"],"secrets":os.environ["ART_SECRETS_DIR"]}))'
trap cleanup_persistence EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
compose build web caddy
start_persistence
while sleep 5; do compose exec -T web /app/artctl live; done
