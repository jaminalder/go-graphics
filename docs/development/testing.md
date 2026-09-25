# Tests and verification

## Persistent runtime

Load tooling: `node --test web/load/config.test.js` and `python3 deploy/tests/test_load.py` validate inputs/routing. `make check-load` executes actual k6 browse, studio/download/similarity and burst scenarios against its own Compose stack. See [load testing](../operations/load-testing.md); these checks do not assert production capacity.

Monitoring coverage includes version-1-to-2 migration with retained sessions, producer/claimant attribution, distinct restart boots, stale/stopped presence, old outstanding jobs and legacy metadata. `python3 deploy/tests/test_watch.py` tests local/hosted naming and discovery changes. Compose tests execute the actual helper and scale renderer replicas 1 → 2 → 1, verify unique identities/full names, and reject public `/monitor` access.

`make test-persistence` starts disposable PostgreSQL/S3 and runs race-enabled tests for transactional admission/replay, actual PNG publication, stale-attempt rejection, River leader handover/rescue, listener reconnect, retention/cleanup and database error semantics. `make check-compose` verifies runtime roles, network/resource boundaries, rendering through Caddy and service recreation. Browser tests exercise SSE and no-JavaScript journeys on the same topology. See [implementation evidence](../plans/postgresql/implementation.md).

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

These unit tests cover recipes, public policy, legacy queue/cache fixtures, child failure limits, domain replay/revision and HTTP guards. They do not exercise production SQL/S3 unless `make test-persistence` supplies its dedicated fixture. That suite refuses arbitrary database URLs because it drops its disposable schema. Race checks are separate from `make check`.

Browser journeys use Playwright 1.63.0 from the lockfile:

```sh
cd web/browser
npm ci --ignore-scripts --registry=https://registry.npmjs.org
npx playwright install chromium
npm test
```

[Playwright configuration](../../web/browser/playwright.config.js) starts disposable Docker PostgreSQL/S3 and application services. The three [journeys](../../web/browser/journey.spec.js) cover desktop/mobile exploration, favourites, downloads, sharing, similarity and no-JavaScript use. Failure/reconnect/rescue evidence comes from the Go integration and Compose tests rather than those three browser journeys. Run one browser fixture at a time; teardown is manifest-scoped.

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
