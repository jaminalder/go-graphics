#!/usr/bin/env bash
# Target-host candidate smoke; no public listener or generation.
set -euo pipefail
candidate=${1:?absolute candidate release directory}
[[ "$candidate" =~ ^/opt/art/releases/[0-9a-f]{40}$ ]] || exit 1
cleanup(){ systemctl stop art-smoke-web art-smoke-render >/dev/null 2>&1 || true; }
trap cleanup EXIT
systemd-run --unit=art-smoke-render --collect --property=User=artrender --property=Group=art-render --property=MemoryMax=2G --property=RuntimeDirectory=art-smoke --property=RuntimeDirectoryMode=0750 --setenv=ART_SOCKET=/run/art-smoke/render.sock --setenv=GOMAXPROCS=2 "$candidate/artrender"
systemd-run --unit=art-smoke-web --collect --property=User=artweb --property=Group=artweb --property=SupplementaryGroups=art-render --property=MemoryMax=384M --property=CacheDirectory=art-smoke --property=CacheDirectoryMode=0700 --setenv=ART_SOCKET=/run/art-smoke/render.sock --setenv=ART_CACHE=/var/cache/art-smoke --setenv=ART_ADDR=127.0.0.1:8180 --setenv=ART_ADMIN_ADDR=127.0.0.1:8181 --setenv=ART_ORIGIN=http://127.0.0.1:8180 --setenv=ART_GENERATION=off "$candidate/artweb"
for attempt in $(seq 1 10); do
 if curl -fsS http://127.0.0.1:8181/ready; then break; fi
 sleep 1
done
curl -fsS http://127.0.0.1:8181/ready
curl -fsS http://127.0.0.1:8180/ >/dev/null
