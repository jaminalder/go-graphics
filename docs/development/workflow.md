# Development workflow

## Repository boundaries

Applications live in `cmd/`; reusable mechanisms and artwork implementations live in `internal/`; embedded web assets and templates live in `internal/web`; catalogue provenance and browser tests live in `web/`; runnable infrastructure lives in `deploy/`. The [package map](../reference/packages.md) and [C4 components](../architecture/components.md) identify ownership.

Use a dedicated branch/worktree for writing work, keeping the coordinator on master. [Worktree workflow](../WORKTREE-WORKFLOW.md) gives exact commands and integration rules. Generated renders, benchmark output and HTML belong under that worktree's `out/`. Committed golden files remain under sketch `testdata/`.

## Artwork and configuration changes

`artwork.Registry` constructs fresh sketches. Local flags are declared by each sketch through `sketch.Configurable`; `Configure` validates/applies them after parsing. The current code has both handwritten older flags and declarative `opt` usage. Weighted discrete traits use `trait.Schema` and `sketch.Traited`. Publicly supported artwork configs are concrete types reconstructed by `artwork.Decode` and restricted by `publish.Validate`.

Changing a local registry definition does not publish it. Public choices, resource policy and fixed preview/download tiers are explicit in `internal/publish`. Embedded catalogue images must correspond to recorded recipes/digests, verified at web startup. [Data contracts](../reference/data.md) describes edition and build identity.

Planning resolves random choices before repeated sampling. Point samplers keep immutable fields/region appearances and no ordinary RNG draws in the pixel loop. Painted paths plan marks and apply them in deterministic order. Named random streams isolate independent effects; changing a stream can intentionally change a seed's output and requires reviewing goldens.

## Checks and review

Run `make check` before every commit. The target formats, vets, lints and tests Go source. Run targeted tests appropriate to the changed responsibility; [testing](testing.md) lists broader public-runtime checks. For artwork/color changes, render fixed seeds and visually inspect outputs and a sweep; tests establish determinism and bounds, not artistic approval.

Update the relevant current-state documentation with behavior changes. Retain the C4 abstraction boundaries: a new Go package is not automatically a container, and deployment changes are not component changes. [Documentation maintenance](documentation.md) covers source links, option snapshots and generated HTML.
