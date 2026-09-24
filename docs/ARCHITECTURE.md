# Architecture

Singular Seed is one artwork creation system with two user workflows: a developer CLI and a curated public studio. Go artwork packages are shared compiled code, not independently deployed services. Web persists studio commands and River jobs in PostgreSQL. The renderer consumes River, supervises a disposable child and publishes images to a private S3 bucket; PostgreSQL contains image pointers only.

The [persistence implementation record](plans/postgresql/implementation.md) documents decisions, local evidence and external validation boundaries. [Persistence operations](operations/persistence.md) is the current runbook.

## C4 views

| View | Scope | Read it to understand |
| --- | --- | --- |
| [System context](architecture/context.md) | Singular Seed and its users | Purpose and system boundary |
| [Containers](architecture/containers.md) | Applications and stores inside Singular Seed | Runtime responsibility and communication |
| [Components](architecture/components.md) | One application at a time | Interfaces and source ownership |
| [Code](architecture/code.md) | Recipe and renderer admission contracts | Types, validation and identity |
| [Dynamic views](architecture/runtime.md) | Preview, download and recovery | Ordered interactions and failure handling |
| [Deployment](architecture/deployment.md) | Local and supplied Compose environments | Processes, networks and volumes |

[Model conventions](architecture/model.md) state how the official C4 guidance is applied. [Package reference](reference/packages.md) covers source that does not need its own architecture diagram.

## Core invariants

Artistic randomness derives from a composition seed, using `Context.RNG(stream)` (PCG) or immutable seed-keyed fields. Authentication tokens and the initial entropy used to choose web exploration seeds use cryptographic randomness; they are outside the deterministic artwork function.

Point samplers use normalized coordinates: `v` spans `[0,1]` and `u` spans `[0,width/height]`. Both axes divide by height. Rendering the same aspect and seed at another resolution preserves composition, while sampling and quantization change the actual pixels. Pixel functions run concurrently and must be pure.

Internal palette colors are floating-point sRGB. Supersampling and translucent compositing convert to linear light where implemented; encoding clamps channels. Some paint-based sketches have their own image conversion and do not honor every raster quality flag. See [materials](reference/materials.md).

A public recipe contains complete concrete artistic choices. Its rendition is server-owned. Publication validation restricts artwork, edition, palette, traits and numeric overrides independently of the unrestricted local registry. Cache identity includes recipe, rendition and renderer build.

## Source of truth

The structure follows [entry points](../cmd/artweb/main.go), [recipes](../internal/artwork/recipe.go), [publication](../internal/publish/catalog.go), [transactional admission](../internal/persistence/queue.go) and [renderer worker](../internal/persistence/worker.go). No separate broker or account service is deployed. Anonymous browser capabilities identify persistent visitors.
