#!/usr/bin/env bash
# Build a portable, checksummed image archive from a clean committed checkout.
set -euo pipefail
cd "$(dirname "$0")/../.."
revision=$(git rev-parse HEAD)
release_dir="out/releases/$revision"
[[ ! -e "$release_dir" ]] || { echo "Release already exists: $release_dir" >&2; exit 1; }
[[ -z "$(git status --porcelain)" ]] || { echo 'Commit changes before building an immutable release.' >&2; exit 1; }
mkdir -p "$release_dir"
for target in app edge; do
 docker build --platform linux/arm64 --target "$target" --build-arg "REVISION=$revision" -f deploy/Dockerfile -t "singular-seed-$target:$revision" .
done
docker image save -o "$release_dir/images.tar" "singular-seed-app:$revision" "singular-seed-edge:$revision"
{
 printf 'ART_APP_IMAGE=%s\n' "$(docker image inspect --format '{{.Id}}' "singular-seed-app:$revision")"
 printf 'ART_EDGE_IMAGE=%s\n' "$(docker image inspect --format '{{.Id}}' "singular-seed-edge:$revision")"
 printf 'ART_REVISION=%s\n' "$revision"
} > "$release_dir/images.env"
git archive HEAD deploy | tar -x -C "$release_dir"
cp web/catalog/manifest.json "$release_dir/catalogue.json"
{
 printf 'source=%s\nplatform=linux/arm64\ngo=1.26.8\nrecipe_version=1\neditions=pools:1,foam:1,iris:1\n' "$revision"
 docker version --format 'docker_client={{.Client.Version}} docker_server={{.Server.Version}}'
 docker compose version
 cat "$release_dir/images.env"
} > "$release_dir/manifest.txt"
(cd "$release_dir"; find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 shasum -a 256 > SHA256SUMS)
printf '%s\n' "$release_dir"
