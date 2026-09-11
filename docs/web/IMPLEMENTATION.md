> State setup update (2026-09-11): local Terraform state for one operator
> supersedes the remote-state recommendations and lock-test gates below.
> See [the current setup](../../deploy/terraform/README.md).

> Current runtime update (2026-09-10): Docker Compose on `master` supersedes the
> initial host-unit deployment below. See ADR 0004 and deploy/README.md. Earlier
> worktree and systemd verification entries are historical evidence only.

# Implementation checkpoint

Branch: `exp/public-art-app`, based on planning commit `7807154`.

The owner approved implementation of the complete plan on 2026-09-09.
The coordinator checkout remains on master; all implementation lives in this
dedicated sibling worktree. Do not integrate or publish without approval.

## Working assumptions

- Launch candidates: pools, foam, iris; no public QQL port.
- Image-file sharing, up to 1200 px long edge; durable links deferred.
- Go SSR, self-hosted htmx, no application database or authentication.
- Typed configurations and canonical recipes; fresh sketch instances.
- Pure exploration planner, bounded transient workspaces and jobs.
- Separate renderer supervisor, hard process deadlines, bounded artifacts.
- Caddy + Docker Compose + Terraform deployment artifacts; no paid provisioning yet.
- Test seams agreed in the approved plan: recipes/configuration, exploration,
  HTTP journeys, admission/artifact lifecycle and renderer protocol.

## Operational logging (2026-09-10)

Both services now use service-tagged stdlib `slog` records on stderr. Incoming
HTTP completions report route patterns, method, status, elapsed time and bytes;
successful probes/polling/assets/previews are debug-only. `ART_LOG_LEVEL=debug`
reveals them when needed. Errors remain visible without recording raw queries,
request bodies, headers or workspace/sample capabilities.

Renderer calls report operation, returned status, duration and failure, with
the canonical job key shared by queue lifecycle and child execution logs.
Render start/completion includes artwork and tier; failures distinguish missing
services, build mismatch, truncated output, deadlines, cancellation and bounded
output overflow. Child stderr remains discarded. Logging occurs outside the
manager mutex, and the child PNG stdout protocol remains unchanged.

Focused tests cover final HTTP status (including implicit 200, 303, 304, 4xx,
5xx and informational headers), streaming, aborted handlers, redaction/noise
levels, real Unix-socket client outcomes and healthy/unavailable probes.
The local/deployment runbook documents log levels and `journalctl` use.

Verification: `make check` passed with zero lint issues; targeted logging and
renderjob race checks passed, including the real-socket outcome tests. An
isolated Chromium/services smoke generated four real Iris previews and verified
600 px PNG delivery and matching job keys in both service logs. Stopping only
that smoke renderer produced an outgoing health failure and incoming readiness
503. A hostile URL/query was absent from logs, and healthy probes stayed quiet.
Example logs and the verified PNG remain under `out/observability/`; smoke
services were shut down without touching manual preview services.

## Site identity (2026-09-10)

The owner selected **Singular Seed** and **`singularseed.art`**. The site
wordmark, HTML titles/description, About copy, shared/downloaded filenames,
service descriptions and deployment examples use that identity. Terraform
resource labels/default names and the example state key use `singular-seed`;
no existing infrastructure was changed. Local listeners and test hosts retain
their original defaults. Name/domain selection is complete; operator contact,
licences, DNS, deployment and public launch remain separate pending work.

Verification: `make check`, Terraform format/validate and Caddy validation
with `ART_DOMAIN=singularseed.art` passed; validation did not start a public
server or change DNS. Loaded Chromium
screenshots at 1440, 390 and 320 px confirm the exact name/title and a readable
header with no overlap or horizontal overflow; desktop and mobile PNGs were
visually inspected under `out/singular-seed-*.png`. Standard local binaries
were rebuilt so the existing run commands pick up the identity after restart.

## Current interaction revision (2026-09-10)

The owner's revised brief supersedes the original multi-parent, recovery-heavy
interface recorded in the historical slices below. Entering from the direction
chooser now admits four images immediately. The grid retains small illustrated
direction labels and only Open image / Favourite actions. Global Favourites
collects images across art forms; one-image view owns download, share and
Generate similar ones. The latter accepts one unfavourited image and keeps
its full traits/palette for all four new composition seeds. Repeated entry
preserves favourites; failed admission restores previous choices.

Removed browser storage/recovery UI and script access; server favourites remain
bounded. Initial creation shares rendering quotas with detail actions, with a
burst of three for create → download → similar. Native share uses a prepared
existing file and a fresh click. Enhanced downloads begin automatically after
preparation; ordinary forms and automatic no-JavaScript progress work too.
Poll fragments preserve focused image actions. Desktop sizing keeps the main
image and its actions together at typical display heights.

Pools/Tide, Foam/After dark and Iris/Earth now provide varied real hero images.
Regenerated catalogue manifests match their PNG hashes and immutable recipes.
Style comparison examples and artwork algorithms/goldens remain unchanged.
The updated product brief and cited `ux-simplification.md` record the rationale.

Verification for this revision:

- `make check` passed: formatting, vet, lint (zero issues), all Go tests.
- Race tests passed for studio, web, explore and publish.
- Three Chromium journeys passed: automatic four-image entry, selected direction
  miniatures, focus through polling, independent favourites, one-image similarity,
  real 1200 px download, delayed prepared native sharing with active user gesture,
  blocked localStorage, no-JavaScript progress/download, and mobile cross-artwork
  favourites with no horizontal overflow.
- Final review also covered an all-four renderer failure during similarity:
  the failure-only retry now carries its batch identity and retains the original
  parent, with owned-batch validation and idempotent replay. Ready batches are
  rejected as failure retries.
- Studio tests defend one-batch admission on replay, one-parent trait/palette
  preservation, repeated play beyond four entries, retained favourites, ownership,
  and failed-admission rollback; HTTP tests defend the shared generation quota.
- Visually inspected loaded desktop gallery/detail and mobile grid screenshots
  under `out/browser-*.png`; coordinator independently checked the full journey,
  single-click download, native share activation and no-JavaScript interaction.
- Browser runner uses isolated configurable ports (8280/8281 by default) and
  cleans up both owned services on interruption. Manual preview servers are untouched.

Final human usability/accessibility, curation and public launch gates are still
separate. No human usability study, merge, deployment or publication is claimed.

## Progress

- [x] Dedicated implementation branch and worktree.
- [x] Provisional visual direction and inspected example assets (owner approval pending).
- [x] Typed recipes and promoted-sketch configuration.
- [x] Shared exploration and publication catalogue.
- [x] Complete SSR/htmx studio and browser recovery/share.
- [x] Isolated renderer, queue, artifact cache and abuse controls.
- [x] Deployment tooling, Terraform, CI and operational runbooks.
- [x] Local browser, race, security, visual and recovery verification.

Human-dependent launch gates remain separate from local implementation:
source/output licences, operator identity/contact, target-host benchmarks,
infrastructure credentials and public launch approval.

## Recipe and exploration slice

Implemented immutable canonical edition-1 recipes, complete resolved traits,
explicit pointer-valued numeric overrides, fresh local factory registry, and
writer encoders. Existing artistic stream IDs and algorithms are unchanged.
Public restoration rejects numeric controls and unpublished trait values.
The CLI delegates breeding to the pure planner with its original 50/50 ratio;
the public planner uses two parent candidates, one boosted and one base draw.
Pins override preferences and parent representation rotates each round.

Provisional direction: an off-white editorial gallery with full uncropped art,
three visual directions per art form and four illustrated colourways (Tide,
Earth, After dark, Meadow). The catalogue owns all mappings. Final artist and
unfamiliar-user review remain launch gates. Phase 0 is not owner approval.

Validation: `make check GOLANGCI_LINT_CACHE=/private/tmp/public-art-worker-lint`
passed after the typed recipe, public choice matrix and exploration tests.
The existing local CLI and all sketch golden/determinism tests pass unchanged.

## Studio and isolated runtime slice

The complete SSR studio is implemented with optional htmx 2.0.10 enhancement:
gallery, illustrated styles/colours, four-image batches, favourites, refinement,
result, preview/download rendition, recovery/export and file-share fallback.
The HTTP app validates Host/Origin/CSRF and exact form keys; separate bounded
request/generation token buckets protect lazy transient workspaces.

`artrender` supervises one fixed child executable over a private Unix socket.
`renderjob` owns the sole bounded queue, resource admission, cancellation and
atomic PNG cache publication. Downloads hold leases while the cache is read.
Execution budgets are 15 seconds preview / 30 seconds download; encoded images
are capped at 16 MiB and named tiers at 600 / 1200 square pixels. Startup image
cache reconciliation removes partial temporary artifacts. HTTP never renders.

Generated 24 catalogue examples with full recipes and content hashes. Hero
images were visually inspected at 600px. At this checkpoint, matched-seed style
sweeps and browser tests were the next step; their completed results follow below. Assets are provisional review
content; copying them into an embedded release is not publication approval.

Validation: complete `make check` passed, including painted/raster CLI pixel
parity, explicit override round-trips, concurrent recipe isolation, identity,
workspace ownership/revisions, atomic admission/cancellation and real hung,
panicking and oversized child process rejection. Browser/recovery tests and operational artifacts were outstanding at this
checkpoint; the final local verification is recorded below.

## Reliability, browser and operational slice

Fixed review findings around atomic admission of mixed cached/new batches,
subscription limits and release, finite queue storage, open-download leases,
late-result rejection and graceful process shutdown. Cancellation now validates
both exploration revision and batch identity, preserving a visible cancelled
state. Polling preserves open choice panels, unsent radio/checkbox selections
and focus. Previous completed samples stay visible during the next batch.
Restored favourites display completed download renditions when previews expire.
Browser backups are updated only by explicit favourite/restore actions; starting
a new exploration cannot replace an old backup with an empty set. Sharing
prepares the file first, then uses a separate user click for native sharing.

Browser checks: four Chromium tests passed, covering the complete enhanced
journey, live polling choice preservation, favourites/recipe recovery, PNG
file downloads, prepared-share fallback, desktop/mobile layouts, ordinary
no-JavaScript forms, blocked storage and backup preservation when starting over.
Independent coordinator browsing also exercised the no-JavaScript 1200px
result. Root review screenshots and browser-run screenshots remain under out/.
Embedded catalogue examples are checked against manifest hashes, dimensions,
recipe allowlists and provenance at startup; all static assets use content-
hashed immutable URLs. Static asset rate accounting is separate from navigation.

Visual review: 36 matched renders at 600px, seeds 1,2,3,42, across all nine
styles. Inspected all three contact sheets. Pools shows sparse, flowing and
crowded families; the flowing seed3 has few large dominant circles, so examples
must not promise a strand in every result. Foam painted cells consistently
replace mark patterns with pigment; airy can still be substantially filled.
Iris preserves the disc/fibre identity while winding and layered structures
remain distinguishable. The owner must make the final artistic judgement.

Operational artifacts include independently bootstrapped locked/versioned
remote-state configuration, pinned Terraform/provider checksums for Linux and
Darwin, separate systemd renderer/web cgroups, bounded Caddy proxy configuration,
immutable tracked-source releases/checksums, private candidate smoke and
activation/rollback scripts, CI and a recovery/monitoring runbook. Local
Terraform init/validate and Caddy validation pass; shell syntax is checked.
No cloud apply, DNS change, domain publication or external message occurred.

Go security: adding HTTP/templates made six Go1.26.5 standard-library advisories
reachable. The module/release pin is now Go1.26.8 (same supported minor line).
All existing art goldens and fixed-pixel CLI parity tests pass on the patch.
The runtime also bounds HTTP concurrency to128 and open cache leases to64;
job metadata is capped at5000 with terminal expiry, independently of eight
queued jobs and one running renderer. No third-party Go dependency was added.

The initial local benchmark smoke rendered72 preview recipes (eight seeds per
style) with zero failures on Apple M1 Pro, Darwin arm64, Go1.26.5 renderer.
Observed style p95 wall time was89–607ms, maximum RSS31.3MiB, maximum image
711463bytes. These small-sample workstation figures do not prove VPS capacity.
The benchmark tool defaults to100 seeds per style and records CPU/wall/RSS/bytes,
renderer/tool Go versions and cache conditions. Full workstation preview and
sample download measurements completed with zero failures: 900 previews
(100 per style, Go1.26.5) had style p95 wall times153–753ms, maximum RSS31.7MiB
and maximum image742865bytes; 72 downloads (eight per style, Go1.26.8) had
style p95 wall times541–2388ms, maximum RSS77.8MiB and maximum image2217822bytes.
Raw JSONL remains under out/artbench-local-100-preview.jsonl and
out/artbench-local-download-smoke.jsonl; these runs predate separate renderer/tool
version fields and identify the actual build versions above. These are cold
isolated children, without artifact-cache hits, on the same workstation.

Remaining launch work is explicitly listed in `launch-gates.md`: operator
identity/contact and licence/provenance decisions, final visual/visitor/assistive-tech review,
actual target100-seed/class/saturation measurements, staged deploy/rollback,
OOM/reboot isolation, remote-state two-client lock proof, TLS/second-machine
recovery and final publication approval. These need an owner or target host;
the local implementation does not fabricate evidence for them.

## Final local verification

- Go1.26.8: `make check` passed (format, vet, lint with zero issues, all tests).
  Existing artwork goldens remain unchanged.
- Race tests passed for artwork, explore, publish, renderjob, studio, web and CLI.
- `govulncheck ./...`: no vulnerabilities found.
- Four Chromium journeys passed, including a six-second delayed image preparation
  followed by a fresh-click native-share stub with active user activation, actual
  cross-tab storage notice, backup preservation, and the ordinary no-JS path.
  Browser tests honour the real IP admission limit and Retry-After response.
- Caddy2.11.4 validation, Terraform1.14.9 format/init/validate and deployment shell
  syntax passed without provisioning infrastructure.
- Four isolated activation control-flow tests passed: success enables boot startup;
  candidate smoke failure leaves old admission untouched; first-deploy failure stops
  and disables candidate services without a self-link; failed upgrade restores the
  previous release. These temporary-filesystem tests fake host commands and do not
  replace target systemd/TLS/rollback exercises.
- The final standards/spec review found and resolved those three deployment defects.
  Earlier recipe, HTTP, browser and runtime findings are covered by regression tests.
- Idle cache retention now runs periodically even when generation is disabled;
  its regression first failed with an expired artifact still occupying the cache,
  then passed with periodic maintenance. Open-download leases still protect files.

The complete implementation is committed on the dedicated branch for owner review.
Master remains unchanged; no integration or public deployment has occurred.

## Compose runtime implementation (2026-09-10)

Owner requested Docker Compose and documentation/learning updates on master.
ADR 0004 supersedes host application units. Added pinned multi-stage image
builds, static app image, Caddy image, private bridge with explicit proxy trust,
Unix socket volume, distinct users/resource limits, persistent TLS volumes,
bounded Docker logs, and private artctl health/admin operations. Only Caddy
publishes ports; renderer networking is disabled and no container receives the
Docker socket. Native loopback development remains available.

Release artifacts now contain a checksummed Linux amd64 image archive, exact
image IDs, Compose configuration and source/edition metadata. Activation has a
deploy lock, generation-disabled smoke,
confirmed queue drain, replacement and rollback. First-install failure removes
the candidate pointer without deleting volumes. Added pinned Docker host
bootstrap and Terraform configuration; OS automatic reboot is disabled.
CI runs real Compose checks and activation failure-path tests. The learning
track now starts with local images, networking, volumes and resource failure,
then state/host/TLS/recovery before later scaling and Kubernetes experiments.

Local evidence: Go proxy/listener tests and focused race tests passed; seven
activation tests passed; Compose generated four real PNGs, checked memory/swap/
CPU/task/rootfs/capability/mount/port settings, rejected private paths and wrong
Host, and kept browsing alive while renderer was stopped, then recovered after
recreation. Rehearsal used Docker 29.7.1 / Compose 5.4.0 in Colima Linux arm64.
Terraform format/validate passed without backend/cloud access. Full `make check` passed with fresh Go/linter caches (the old cache referenced
a removed worktree). Release archive/smoke results are recorded below.

Target Ubuntu amd64 installation, real OOM/reboot, dual-stack forwarding policy,
public TLS/renewal, state-lock/restore and measured second-machine recovery
remain pending. No cloud resources, DNS or public release were changed.

Release evidence: commit `995b48e763c5677d7f0ca6b088cb02ad0ab23619` built a
Linux amd64 image archive; all SHA256 checksums passed. Its exact recorded image
IDs passed `smoke-release.sh` (Caddy validation plus private web liveness and
renderer readiness) in Colima under amd64 emulation. This is packaging/protocol
evidence, not native VPS performance or TLS evidence. The pinned Go vulnerability
checker reported no vulnerabilities. The `art-local` Compose project remains
available at http://localhost:8088 for the first learning exercise.

### Production environment clarification (2026-09-11)

There is one production VPS, initially accessible over HTTP at its assigned IP.
Terraform and scripts no longer select staging versus production; installation
records one operator-selected deployment. The current site responds over HTTP;
full destroy/apply verification of this cleanup precedes domain/DNS setup.
The older staging/public-approval descriptions above are historical. See
`deploy/terraform/README.md` for preserving the existing backend state key and
updating ignored inputs before the rebuild.
