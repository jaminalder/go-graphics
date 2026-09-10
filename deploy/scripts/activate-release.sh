#!/usr/bin/env bash
# Run on the target host as root after uploading a checksummed release.
set -Eeuo pipefail
release=${1:?usage: activate-release.sh COMMIT}
[[ "$release" =~ ^[0-9a-f]{40}$ ]] || { echo 'Expected a full commit id' >&2; exit 1; }
[[ $EUID -eq 0 ]] || { echo 'Run as root on the target host' >&2; exit 1; }
exec 9>/run/lock/art-deploy.lock
flock -n 9 || { echo 'Another activation is running' >&2; exit 1; }
next="/opt/art/releases/$release"
case "$(sed -n 's/^ART_ENVIRONMENT=//p' /etc/art/operator.env)" in
 production) approval=/etc/art/launch-approved ;;
 staging) approval=/etc/art/staging-approved ;;
 *) echo 'Set ART_ENVIRONMENT=staging or production' >&2; exit 1 ;;
esac
[[ -f "$approval" ]] || { echo "Record owner approval in $approval first" >&2; exit 1; }
(cd "$next"; sha256sum -c SHA256SUMS)
# Only IDs from the checked archive may be activated, even if a tag has moved.
[[ $(grep -Ec '^ART_(APP|EDGE)_IMAGE=sha256:[0-9a-f]{64}$' "$next/images.env") == 2 ]]
docker image load -i "$next/images.tar"
compose(){ "$next/deploy/scripts/compose-release.sh" "$1" "${@:2}"; }
compose "$next" config --quiet
previous=
if [[ -L /opt/art/current ]]; then
 previous=$(readlink -e /opt/art/current)
 [[ -f "$previous/images.env" ]] || { echo 'Current release is incomplete' >&2; exit 1; }
elif [[ -e /opt/art/current ]]; then
 echo 'Current release must be a symlink' >&2; exit 1
fi
"$next/deploy/scripts/smoke-release.sh" "$next"
changed=no
rollback(){
 status=$?
 trap - ERR
 set +e
 if [[ "$changed" == yes ]]; then
  if [[ -n "$previous" ]]; then
   compose "$next" stop web renderer caddy
   ln -sfn "$previous" /opt/art/current
   compose "$previous" up -d --no-build --pull never --wait --wait-timeout 60
   compose "$previous" exec -T web /app/artctl ready
  else
   compose "$next" down
   if [[ -L /opt/art/current ]]; then unlink /opt/art/current; fi
  fi
 elif [[ -n "$previous" ]]; then
  compose "$previous" exec -T web /app/artctl generation-on
 fi
 echo "Activation failed; previous=$previous. Verify recovery before resuming operation." >&2
 exit "$status"
}
trap rollback ERR
if [[ -n "$previous" ]]; then
 compose "$previous" exec -T web /app/artctl generation-off
 drained=no
 for attempt in $(seq 1 40); do
  metrics=$(compose "$previous" exec -T web /app/artctl metrics)
  if printf '%s\n' "$metrics" | grep -qx 'art_jobs_running 0' && printf '%s\n' "$metrics" | grep -qx 'art_jobs_queued 0'; then drained=yes; break; fi
  sleep 1
 done
 [[ "$drained" == yes ]] || { echo 'Drain timed out; aborting release' >&2; false; }
fi
changed=yes
# Stop first: never run two independent admission queues during replacement.
if [[ -n "$previous" ]]; then compose "$previous" stop web renderer; fi
ln -sfn "$next" /opt/art/current
compose "$next" up -d --no-build --pull never --wait --wait-timeout 60
compose "$next" exec -T web /app/artctl ready
compose "$next" exec -T web /app/artctl live
origin=$(sed -n 's/^ART_ORIGIN=//p' /etc/art/operator.env)
curl --fail --silent --show-error --max-time 15 "$origin/" >/dev/null
trap - ERR
printf 'Activated %s; previous=%s\n' "$release" "$previous"
