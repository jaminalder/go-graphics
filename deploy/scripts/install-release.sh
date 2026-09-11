#!/usr/bin/env bash
# Root entry point installed by cloud-init; consumes an SSH-uploaded release.
set -Eeuo pipefail
archive=${1:?archive path}
digest=${2:?archive SHA256}
revision=${3:?release commit}
origin=${4:?site origin}
environment=${5:?environment}
[[ $EUID -eq 0 ]]
[[ "$digest" =~ ^[0-9a-f]{64}$ && "$revision" =~ ^[0-9a-f]{40}$ ]]
[[ "$origin" =~ ^https?://[a-z0-9.-]+$ ]]
case "$environment" in
 staging) approval=staging-approved ;;
 production) [[ "$origin" == https://* && -f /etc/art/launch-approved ]]; approval=launch-approved ;;
 *) exit 1 ;;
esac
exec 8>/run/lock/art-install.lock
flock -n 8 || { echo 'Another release installation is running' >&2; exit 1; }
printf '%s  %s\n' "$digest" "$archive" | sha256sum -c -
install -d -m 0755 /opt/art/releases /etc/art
incoming=$(mktemp -d /opt/art/releases/.incoming.XXXXXXXX)
previous_env=$(mktemp /etc/art/.previous-env.XXXXXXXX)
had_env=no
if [[ -f /etc/art/operator.env ]]; then
 cp /etc/art/operator.env "$previous_env"
 had_env=yes
fi
cleanup(){ rm -rf "$incoming"; rm -f "$previous_env"; }
trap cleanup EXIT
tar --no-same-owner -xf "$archive" -C "$incoming"
(cd "$incoming"; sha256sum -c SHA256SUMS)
grep -qx "source=$revision" "$incoming/manifest.txt"
grep -qx 'platform=linux/amd64' "$incoming/manifest.txt"
next="/opt/art/releases/$revision"
if [[ -d "$next" ]]; then
 cmp "$incoming/SHA256SUMS" "$next/SHA256SUMS"
 (cd "$next"; sha256sum -c SHA256SUMS)
else
 mv "$incoming" "$next"
fi
operator_env=$(mktemp /etc/art/.operator-env.XXXXXXXX)
cat > "$operator_env" <<EOF
ART_ENVIRONMENT=$environment
ART_DOMAIN=$origin
ART_ORIGIN=$origin
ART_BIND=0.0.0.0
ART_BIND6=[::]
ART_HTTP_PORT=80
ART_HTTPS_PORT=443
ART_PROXY_NET=172.30.80
ART_LOG_LEVEL=info
EOF
chmod 0600 "$operator_env"
mv "$operator_env" /etc/art/operator.env
if [[ "$environment" == staging ]]; then
 printf 'Terraform apply selected %s at %s with release %s\n' "$environment" "$origin" "$revision" > "/etc/art/$approval"
 chmod 0600 "/etc/art/$approval"
fi
if "$next/deploy/scripts/activate-release.sh" "$revision"; then
 printf 'Application ready at %s\n' "$origin"
else
 status=$?
 if [[ "$had_env" == yes ]]; then
  cp "$previous_env" /etc/art/operator.env
  if [[ -L /opt/art/current ]]; then
   /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current up -d --no-build --pull never --wait --wait-timeout 60 || true
  fi
 else
  rm -f /etc/art/operator.env
 fi
 exit "$status"
fi
