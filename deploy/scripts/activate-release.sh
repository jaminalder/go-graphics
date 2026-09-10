#!/usr/bin/env bash
# Run on the target host as root after an operator uploads a checksummed release.
set -euo pipefail
release=${1:?usage: activate-release.sh COMMIT}
[[ "$release" =~ ^[0-9a-f]{40}$ ]] || { echo 'Expected a full commit id' >&2; exit 1; }
[[ $EUID -eq 0 ]] || { echo 'Run as root on the target host' >&2; exit 1; }
next="/opt/art/releases/$release"
[[ -f /etc/art/launch-approved ]] || { echo 'Record owner launch approval and launch gate evidence in /etc/art/launch-approved first.' >&2; exit 1; }
(cd "$next"; sha256sum -c SHA256SUMS)
[[ -x "$next/artweb" && -x "$next/artrender" ]]
previous=
if [[ -L /opt/art/current ]]; then
 previous=$(readlink -e /opt/art/current)
 [[ -x "$previous/artweb" && -x "$previous/artrender" ]] || { echo 'Current release is incomplete' >&2; exit 1; }
elif [[ -e /opt/art/current ]]; then
 echo 'Current release must be a symlink' >&2; exit 1
fi
# Validate the versioned configuration before switching any active service.
ART_DOMAIN=$(sed -n 's/^ART_DOMAIN=//p' /etc/art/domain.env) caddy validate --config "$next/deploy/caddy/Caddyfile" --adapter caddyfile
verification_dir=$(mktemp -d /tmp/art-units.XXXXXX)
for unit in artweb artrender; do
 sed "s#/opt/art/current/#$next/#g" "$next/deploy/systemd/$unit.service" > "$verification_dir/$unit.service"
done
systemd-analyze verify "$verification_dir/artweb.service" "$verification_dir/artrender.service"
rm "$verification_dir/artweb.service" "$verification_dir/artrender.service"
rmdir "$verification_dir"
"$next/deploy/scripts/smoke-release.sh" "$next"
caddy_backup=$(mktemp /tmp/art-caddy.XXXXXX)
trap 'rm -f "$caddy_backup"' EXIT
had_caddy=no
if [[ -f /etc/caddy/Caddyfile ]]; then cp /etc/caddy/Caddyfile "$caddy_backup"; had_caddy=yes; fi
rollback(){
 if [[ -n "$previous" ]]; then
  ln -sfn "$previous" /opt/art/current
  install -m 0644 "$previous/deploy/systemd/artweb.service" "$previous/deploy/systemd/artrender.service" /etc/systemd/system/
  systemctl daemon-reload
  systemctl restart artrender artweb
 else
  systemctl stop artweb artrender || true
  systemctl disable artweb artrender || true
 fi
 if [[ "$had_caddy" == yes ]]; then
  cp "$caddy_backup" /etc/caddy/Caddyfile
  systemctl reload caddy || true
 fi
}
trap rollback ERR
curl -fsS -X POST http://127.0.0.1:8081/generation/off || true
for attempt in $(seq 1 40); do
 metrics=$(curl -fsS http://127.0.0.1:8081/metrics || true)
 if ! printf '%s' "$metrics" | grep -Eq 'art_jobs_(running|queued) [1-9]'; then break; fi
 sleep 1
done
ln -sfn "$next" /opt/art/current
install -m 0644 "$next/deploy/systemd/artweb.service" "$next/deploy/systemd/artrender.service" /etc/systemd/system/
systemctl daemon-reload
systemctl restart artrender artweb
for attempt in $(seq 1 10); do
 if curl -fsS http://127.0.0.1:8081/ready; then break; fi
 sleep 1
done
curl -fsS http://127.0.0.1:8081/ready
origin=$(sed -n 's/^ART_ORIGIN=//p' /etc/art/web.env)
canonical_host=${origin#https://}
curl -fsS -H "Host: $canonical_host" http://127.0.0.1:8080/ >/dev/null
# Caddy uses its EnvironmentFile containing ART_DOMAIN, configured at provisioning.
install -m 0644 "$next/deploy/caddy/Caddyfile" /etc/caddy/Caddyfile
systemctl reload caddy
systemctl enable artrender artweb
trap - ERR
rm "$caddy_backup"
printf 'Activated %s; previous=%s\n' "$release" "$previous"
