> Interaction revision, 2026-09-10: the owner’s simpler [current product brief](product-and-ux.md)
> supersedes the multi-parent refinement and visible browser-recovery UI below.
> Implementation and verification are recorded in [IMPLEMENTATION.md](IMPLEMENTATION.md).

# Implementation plan and future-session handoff

Status: implemented locally on `exp/public-art-app`, with external launch gates
pending. The original plan below remains the acceptance specification. See
[the implementation evidence](IMPLEMENTATION.md), [launch gates](launch-gates.md)
and [operations runbook](../../deploy/README.md) for actual results and remaining
owner/target-host work. The original planning baseline was `63c622a`.

## Order and working rules

Build a narrow complete experience before expanding the catalogue. Validate
the central visual interaction early; finish resource isolation and security
before making generation publicly reachable. Preserve offline development
throughout. No foundational step requires converting all sixteen artworks.

Read [the overview](README.md), [domain vocabulary](../../CONTEXT.md), the
relevant artwork specifications, and the current `AGENTS.md` before starting.
Use dedicated branches/worktrees for implementation according to
[the existing workflow](../WORKTREE-WORKFLOW.md); the user's exception placing
this analysis on `master` is not a blanket change to future implementation.
Run `make check` before commits. Artwork changes require fixed-seed render
comparison and visual review, even when tests pass.

Each phase should be a coherent reviewable change or a small set of changes.
Update this plan with actual commands/results, accepted choices and any changed
assumptions so later sessions do not infer completion from prose. Do not repeat
the older completed artwork lifecycle migration.

## Phase 0 — settle the public contract and visual direction

Scope: documentation/content exploration and a disposable UI prototype.

- Public name/domain settled on 2026-09-10: Singular Seed, `singularseed.art`.
  Confirm the final launch shortlist; image-file sharing remains the baseline.
  Choose source/output licences after provenance inventory.
- Curate provisional examples for `pools`, `foam`, `iris`; the artist may choose
  another set. Include multiple seeds and relevant colour/style comparisons.
- Prototype gallery, visual choice cards, four-image studio, favourites and
  result page using real artwork. Compare mobile/desktop layouts and run the
  unfamiliar-user exercise from the UX brief.
- Write the chosen public style/colour mappings and acceptance criteria before
  implementing handlers. Keep numeric rendering quality private.

Exit: one approved art form and journey for a vertical slice; unresolved rights
exclude a candidate from publication, not from local development. `prototype`
is useful here; focused `grilling` may resolve the few remaining decisions.

## Phase 1 — recipe/configuration seam with two actual consumers

Scope: `internal/artwork`, factory wiring, typed config for the chosen pilot,
and only the necessary existing sketch/CLI adapters.

- Use `contour` as a cheap technical fixture if useful; use a traited launch
  artwork for the first real public path. Cover one raster and one painted
  sketch before declaring the shared interface settled.
- Define versioned canonical recipe, rendition identity and seed-as-string
  serialization. Distinguish seed, resolved choices, pins and quality settings.
- Give promoted sketches typed override/configuration validation. Adapt
  `Flags`/`Configure` without changing default/explicit-override semantics.
- Centralize fresh factories. Keep unpublished sketches registered for the CLI.
- Add writer-based metadata encoding while preserving file-writing wrappers;
  preserve local filenames and existing recipe metadata where compatible.

Exit tests: same recipe matches the prior CLI at fixed seeds; two differently
configured concurrent jobs cannot contaminate each other; canonical recipe
round trips; malformed/duplicate/unknown inputs fail; explicit default-valued
overrides survive; dimensions/quality change artifact identity. Use TDD for
these behavioural contracts. No HTTP-to-CLI argument bridge is accepted as
the final seam.

## Phase 2 — reusable exploration and publication policy

Scope: `internal/explore`, `internal/publish`, and flock CLI adaptation.

- Extract pure candidate planning from `flock.go`; keep JSONL and output files
  in the CLI. Preserve current local sampling ratios and seed streams.
- Add the small-batch public policy with explicit pins, selected-parent
  sampling, preference boost and a base-distribution candidate.
- Add bounded de-duplication/history, parent tracking, zero-weight handling
  and recipe completion. Public policy excludes unpublished/expensive controls.
- Add a one-artwork catalogue with validated example manifest, display copy,
  styles, colourways, allowed renditions and resource class.

Exit tests: deterministic multi-round suggestions; pins never loosen silently;
no duplicate recipes; empty/conflicting choices have defined outcomes; weight-0
wash stays selected only when explicit; base diversity remains; existing CLI
flock still works. Visually judge the generated space and relationship between
refinement batches. Do not implement image embeddings or genetic crossover.

## Phase 3 — complete local web vertical slice

Scope: `cmd/artweb`, `internal/studio`, `internal/web`, embedded assets.

- Build gallery → choices → four samples → favourites → refinement → result
  → download for the first artwork, with server-rendered full pages.
- Add bounded opaque-cookie workspaces, CSRF checks, revision/idempotency
  handling and explicit expired/retry states. Use a development renderer
  adapter while the hard-isolation slice is built, bound to localhost only.
- Add htmx enhancement after forms work. Pin the chosen major/minor and test
  its actual CSP/error/history behaviour. Add minimal browser recovery/share
  code, handling disabled storage and unsupported native sharing.
- Pre-render catalogue examples; do not generate on gallery requests.

Exit: keyboard/touch/no-JS core path works; back/refresh/stale tabs do not lose
favourites or duplicate batches; same image is downloaded and shared; error
states remain useful. Use `httptest` for HTTP contracts and a small real-browser
suite for critical journeys. Do not make this development adapter public.

## Phase 4 — bounded production renderer and artifact lifecycle

Scope: `internal/renderjob`, `cmd/artrender`, private protocol and limits.

- Implement atomic admission/reservations, one job owner, finite queue,
  per-workspace/IP limits, fair scheduling, deduplication and cancellation.
- Implement private supervisor + fixed child executable; validate recipes at
  both sides, bound input/output/stderr, enforce deadline and kill/reap.
- Give artifacts complete-write/atomic-ready semantics, byte/count/age limits,
  open-download/eviction coordination, ETags and explicit cache-miss behaviour.
- Add graceful shutdown, generation kill switch, health/degraded status and
  structured resource/error metrics.

Exit: timeout/hang/panic/oversized output and worker loss cannot kill the web
service; cancellation releases resources; partial/late results are not served;
cache-full and quota states are visible; GET/HEAD never enqueue work; maps and
temporary files stay bounded. Test actual subprocess failure as well as fast
deterministic test adapters.

## Phase 5 — infrastructure, CI and staging

Scope: `deploy/`, CI configuration, operational documentation.

- Select/pin a supported Linux image, architecture, Go and tool versions.
- Implement Terraform for server/firewall/IP/key resources; select/bootstrap
  remote state, then prove locking/version recovery from two clients.
- Add nonsecret cloud-init, separate systemd web/renderer cgroups, Caddy config,
  private endpoints and restricted administrative access.
- Add immutable release build/deploy/rollback scripts and checksummed manifests.
  CI runs `make check`, race tests, vulnerability checks and required browser/
  protocol checks. Keep production secrets out of test output/state.
- Add external uptime alerting, bounded logs, resource dashboards or simple
  private metrics, and backup/restore runbooks.

Exit: staged deploy, rollback, reboot, hard renderer OOM isolation, state-lock
failure and clean-host restore are demonstrated. Cloud creation, DNS changes
and credentials require owner action/authorisation; this plan is not an apply.
Use `wizard` only for genuinely human-only credential/dashboard steps later.

## Phase 6 — benchmark, expand the launch set and release

Scope: actual target VPS measurements and final product quality.

- Measure CPU-seconds, p50/p95/p99 wall time, RSS and output bytes over at least
  100 representative seeds per candidate public configuration class; cover
  worst-known valid combinations and saturated traffic.
- Validate browse/status latency while rendering, slow downloads, burst load,
  request/body limits, proxy trust and cache exhaustion. Record hardware,
  compiler/release, command, output sizes and warm/cold cache conditions.
- Add the remaining approved artworks through the same publication contract.
  Judge each style's output space and download size. Admit flame only with its
  required 1000²/quality-80 review and a measured runtime budget.
- Complete visual/accessibility review, provenance/usage terms, privacy and
  contact information. Repeat the visitor exercise with the real application.
- Verify recovery from a second machine and obtain final publication approval.

Exit: all [launch gates](security-and-operations.md#public-launch-gates) pass
with recorded evidence; owner approves the final catalogue and operation cost.
If measurements fail, reduce the admitted catalogue/load or change host sizing;
do not remove quotas or silently lower artwork quality to satisfy a benchmark.

## Optional extension — public links

Only include if selected by the owner. Implement the
[recipe-link contract](architecture.md#optional-public-artwork-links), key
rotation, edition-retention/retirement rules, cheap cache-miss pages, static
social fallback and public-link browser tests. Explicitly budget any durable
shared-image storage. This extension must not change the guarantee that GET
does not start expensive generation.

## Refactors deliberately deferred

No universal `Plan`/`Scene`/field interface, mass sketch renaming, full renderer
cancellation migration, paint tiling, all-sketch trait conversion, microservices,
database, user accounts, uploads, payments, live collaboration, infinite
generation, public REST API, WASM engine, Kubernetes or container platform.
Each has a possible later motivation; none is required for this first product.

Revisit cooperative cancellation for efficiency if process overhead is material;
revisit paint memory if approved public renditions need it; revisit durable
state/coordination only when jobs or multiple replicas must survive independently.

## Documentation-task verification

The analysis inspected registry/configuration, exploration, rendering/encoding,
artwork lifecycle examples, the sketch specification inventory, prior
architecture decisions, local performance evidence and primary web sources.
Existing pools/foam sheets and a 1000² flame image were viewed. `go test ./...`
passed on the inspected baseline (cached results). No artwork algorithms,
generated goldens, dependencies or infrastructure were changed.

The full pre-commit gate also passed with zero lint issues:
`make check GOLANGCI_LINT_CACHE=/private/tmp/go-graphics-docs-lint.x7EWfm`.
The first sandboxed run could not load packages; an unrestricted run exposed
stale cached diagnostics referring to a removed sibling worktree. Using a new
temporary lint cache resolved that without code or linter-configuration changes.
The path above records this run, not a path required on another machine.
All 59 local Markdown links in the changed/new documentation were checked,
including internal anchors, and code fences were balanced.

Future implementation claims require fresh phase-specific evidence. Tests
passing in this documentation session say nothing about a web application that
does not yet exist.
