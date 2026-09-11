# Tests and verification

## Standard pre-commit gate

```sh
make check
```

[Makefile](../../Makefile) runs formatting (`golangci-lint fmt` with gofumpt/goimports), `go vet ./...`, `golangci-lint run`, and `go test ./...`. [CI](../../.github/workflows/check.yml) pins Go 1.26.8 and golangci-lint 2.12.2. `make test`, `make lint` and `make vet` are individual entry points. Run this gate before every commit.

## Artwork and material verification

Math/color/noise tests defend numerical properties. Sketch tests defend determinism, seed variation, geometry/plan bounds, override isolation and golden image output. Regenerate goldens only when the visual change is intended:

```sh
go test ./internal/sketch/... -run TestGolden -update
```

Open changed goldens and useful-size previews. Sweep fixed seeds/options and compare contact sheets to understand typical behavior. Flame requires a 1000×1000 quality-80 first review. Sampler tests can examine equal-aspect planned coordinates and allocation behavior before whole-frame encoding. [Performance](../performance.md) contains benchmark commands and memory implications.

## Public application checks

```sh
go test -race ./internal/artwork ./internal/explore ./internal/publish ./internal/renderjob ./internal/studio ./internal/web ./cmd/staticart
```

These tests cover recipe canonicalization, public policy, fresh definitions, bounded/coalesced admission, cancellation, cache reconciliation, renderer failure limits, revision/replay/ownership and HTTP protections. Fake renderer implementations make queue failure paths controllable; real renderer tests cover process/protocol behavior. Race checks are separate from `make check`.

Browser journeys use Playwright 1.63.0 from the lockfile:

```sh
cd web/browser
npm ci --ignore-scripts --registry=https://registry.npmjs.org
npx playwright install chromium
npm test
```

[Playwright configuration](../../web/browser/playwright.config.js) starts the local test server via the checked-in script. [Journeys](../../web/browser/journey.spec.js) exercise gallery/exploration, favourites, downloads, no-JavaScript behavior and failure recovery. Browser dependencies are test tooling, not Go module dependencies.

## Deployment checks

| Command | Coverage / requirements |
| --- | --- |
| `make check-compose` | Real Linux-container isolation, health and rendering; Docker/Compose |
| `python3 deploy/tests/test_activation.py` | Activation/drain/rollback failure paths |
| `python3 deploy/tests/test_provision.py` | Local provisioning input/release verification |
| `deploy/scripts/verify-cloud-init.sh` | Cloud-init, SSH access and installation in disposable Ubuntu containers |
| `terraform -chdir=deploy/terraform fmt -check` | Terraform formatting |
| `terraform -chdir=deploy/terraform init -backend=false -input=false` then `validate` | Provider/schema validation without cloud provisioning |
| `govulncheck ./...` | Go vulnerability analysis with current vulnerability data |

[verify-local.sh](../../deploy/scripts/verify-local.sh) combines several of these checks; [.github/workflows/check.yml](../../.github/workflows/check.yml) is the authoritative CI sequence and includes additional provisioning/cloud-init checks. Docker tests run disposable local projects; Terraform validation does not apply infrastructure. `make check` alone does not establish browser/deployment health.

## Documentation checks

`make docs-check` validates every Markdown link and local fragment plus documentation tooling tests. `make docs` builds every page, copies referenced assets/source and validates generated local links. Browser inspection is still needed to confirm Mermaid rendering and layout quality. See [documentation tooling](documentation.md).
