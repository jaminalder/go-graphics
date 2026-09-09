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
- Caddy + systemd + Terraform deployment artifacts; no paid provisioning yet.
- Test seams agreed in the approved plan: recipes/configuration, exploration,
  HTTP journeys, admission/artifact lifecycle and renderer protocol.

## Progress

- [x] Dedicated implementation branch and worktree.
- [ ] Visual direction and reviewed example assets.
- [x] Typed recipes and promoted-sketch configuration.
- [x] Shared exploration and publication catalogue.
- [ ] Complete SSR/htmx studio and browser recovery/share.
- [ ] Isolated renderer, queue, artifact cache and abuse controls.
- [ ] Deployment tooling, Terraform, CI and operational runbooks.
- [ ] Browser, race, security, visual and recovery verification.

Human-dependent launch gates remain separate from local implementation:
source/output licences, domain/operator details, target-host benchmarks,
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
images were visually inspected at 600px; matched-seed style sweeps and actual
browser tests remain the next verification step. Assets are provisional review
content; copying them into an embedded release is not publication approval.

Validation: complete `make check` passed, including painted/raster CLI pixel
parity, explicit override round-trips, concurrent recipe isolation, identity,
workspace ownership/revisions, atomic admission/cancellation and real hung,
panicking and oversized child process rejection. Browser/load/recovery tests,
operational artifacts and launch gates remain outstanding at this checkpoint.
