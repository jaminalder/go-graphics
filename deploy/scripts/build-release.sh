#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
revision=$(git rev-parse HEAD)
release_dir="out/releases/$revision"
if [[ -e "$release_dir" ]]; then echo "Release already exists: $release_dir" >&2; exit 1; fi
if [[ -n "$(git status --porcelain)" ]]; then echo "Commit changes before building an immutable release." >&2; exit 1; fi
mkdir -p "$release_dir"
for command in artweb artrender staticart; do
 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.build=$revision" -o "$release_dir/$command" "./cmd/$command"
done
git archive HEAD deploy | tar -x -C "$release_dir"
cp web/catalog/manifest.json "$release_dir/catalogue.json"
{
 printf 'source=%s\n' "$revision"
 go version
 printf 'platform=linux/amd64\nrecipe_version=1\neditions=pools:1,foam:1,iris:1\nhtmx=2.0.10\ncaddy=2.11.4\n'
} > "$release_dir/manifest.txt"
(cd "$release_dir"; find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 shasum -a 256 > SHA256SUMS)
printf '%s\n' "$release_dir"
