#!/usr/bin/env bash
# Explicit one-time bootstrap. Existing secrets and data volume are preserved.
set -euo pipefail
release=${1:?release directory}
[[ $EUID -eq 0 ]] || { echo 'Run on the target host as root' >&2; exit 1; }
[[ -f /etc/art/storage.env ]] || { echo 'Configure /etc/art/storage.env and image credentials first' >&2; exit 1; }
(cd "$release"; sha256sum -c SHA256SUMS)
docker image load -i "$release/images.tar"
python3 "$release/deploy/scripts/init-secrets.py" /etc/art/secrets
for role in web renderer; do
 [[ -f /etc/art/secrets/$role-objects ]] || { echo "Missing $role object credentials" >&2; exit 1; }
 chown 0:10000 "/etc/art/secrets/$role-objects"
 chmod 0440 "/etc/art/secrets/$role-objects"
done
docker compose -p singular-seed-data -f "$release/deploy/compose.data.yaml" up -d --wait --wait-timeout 90
bash "$release/deploy/scripts/admin-release.sh" "$release" migrate
python3 "$release/deploy/scripts/database-roles.py" /etc/art/secrets docker compose -p singular-seed-data -f "$release/deploy/compose.data.yaml"
bash "$release/deploy/scripts/admin-release.sh" "$release" bind-bucket
