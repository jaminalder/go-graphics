#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
make check
go test -race ./internal/artwork ./internal/explore ./internal/publish ./internal/renderjob ./internal/studio ./internal/web ./cmd/staticart
govulncheck ./...
python3 deploy/tests/test_activation.py
deploy/scripts/verify-compose.sh
terraform -chdir=deploy/terraform fmt -check
terraform -chdir=deploy/terraform init -backend=false -input=false
terraform -chdir=deploy/terraform validate
for script in deploy/scripts/*.sh; do bash -n "$script"; done
(cd web/browser; npm ci --ignore-scripts --registry=https://registry.npmjs.org; npx playwright test)
