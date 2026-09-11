# Source packages and tools

This is a source navigation map, not a diagram of deployable services. The [C4 component views](../architecture/components.md) group these packages by runtime responsibility. Go source has no third-party module dependencies; web JavaScript, documentation and deployment tools have their own dependencies.

## Applications and orchestration

| Source | Responsibility |
| --- | --- |
| [cmd/staticart](../../cmd/staticart/main.go) | Generic local render/traits/sweep/flock command wiring |
| [cmd/artweb](../../cmd/artweb/main.go) | Public web and private admin listeners, dependency construction, shutdown |
| [cmd/artrender](../../cmd/artrender/main.go) | Private Unix socket supervisor and child process entry point |
| [cmd/artctl](../../cmd/artctl/main.go) | Private health, generation and metrics command |
| [artwork](../../internal/artwork/registry.go) | Fresh local factories; canonical public edition recipes in [recipe.go](../../internal/artwork/recipe.go) |
| [publish](../../internal/publish/catalog.go) | Curated public artwork/style/colour allowlist and fixed renditions |
| [explore](../../internal/explore/explore.go) | Deterministic candidate planning, parent traits and seed policies |
| [studio](../../internal/studio/studio.go) | Bounded transient workspaces, revisioned actions, favourites and recovery |
| [renderjob manager](../../internal/renderjob/manager.go) | Single queue, coalescing, cancellation, PNG validation and cache |
| [renderjob protocol](../../internal/renderjob/protocol.go) | Client, private transport, supervisor and child boundaries |
| [web](../../internal/web/web.go) | Routes, guards, embedded HTML/assets and public presentation |
| [logging](../../internal/logging/logging.go) | Structured text service logger and bounded HTTP outcome logging |

## Artwork mechanisms

| Package | Responsibility and boundary |
| --- | --- |
| [sketch](../../internal/sketch/sketch.go) | Context, interfaces, raster adapters and registry mechanism |
| [trait](../../internal/trait/trait.go) | Weighted discrete dimensions, derivation, pins and boosts |
| [opt](../../internal/opt/opt.go) | Declarative sketch flags, validation and filename suffixes |
| [rnd](../../internal/rnd/rnd.go) | Explicit-RNG weighted choice, Gaussian draws, winnowing and bags |
| [mathx](../../internal/mathx/mathx.go) | Scalar mapping, clamp and smoothstep helpers |
| [noise](../../internal/noise/perlin.go) | Seeded Perlin/fBm, curl/ridged fields, Worley and hashes |
| [palette](../../internal/palette/color.go) | Floating sRGB, HSL/HSB conversions, provenance palettes and swatches |
| [gradient](../../internal/gradient/gradient.go) | Continuous/discrete gradients, shuffles and terraces |
| [geom](../../internal/geom/geom.go) | Circles and collision/containment spatial queries |
| [cells](../../internal/cells/cells.go) | Weighted curved partitions, merged regions and boundary queries |
| [scheme](../../internal/scheme/scheme.go) | Planned per-region hue/value arrangement |
| [hatch](../../internal/hatch/hatch.go) | Pure color-free mark coverage and composition |
| [paint](../../internal/paint/paint.go) | Sequential brush canvas, bristle marks, washes and analytic rings; pure FlatWash material |
| [render](../../internal/render/render.go) | Parallel normalized raster loops, alpha-aware AA, encoding and metadata; sheets/profiles |
| [flame](../../internal/flame/system.go) | Nonlinear IFS variations, chaos-game histogram, tone mapping and density estimation |
| [sketchtest](../../internal/sketch/sketchtest/sketchtest.go) | Shared determinism and golden helpers |

[Sketch catalogue](sketches.md) covers all artwork policies. The deepest leaf packages own mechanisms, while sketches choose ranges, stream IDs, material order and composition. Existing concrete cross-sketch composition is deliberate: Shallows and Glaze construct Scree's bed; Shallows also constructs Riffle's surface. The dependency structure is broader than a single historical `cmd → sketch → render` chain because the public studio adds recipes, publication and execution boundaries.

## Repository tools

| Source | Implemented use |
| --- | --- |
| [tools/genpalettes](../../tools/genpalettes/main.go) | Generate internal ColorLisa data from retained Markdown dataset |
| [tools/genqqlpalettes](../../tools/genqqlpalettes/main.go) | Generate QQL swatches/cities from retained JSON dataset |
| [tools/hatchbook](../../tools/hatchbook/main.go) | Print specimen sheet manifest used by `make hatchbook` |
| [tools/flocksheet](../../tools/flocksheet/main.go) | Rebuild a sheet from an existing directory's numbered PNGs |
| [tools/reliefexp](../../tools/reliefexp/main.go) | Existing fixed tapestry relief render comparison utility |
| [tools/webcatalog](../../tools/webcatalog/main.go) | Produce canonical hero/choice image candidates and digest manifest in `out/catalog` |
| [tools/artbench](../../tools/artbench/main.go) | Measure isolated public renders and emit JSONL/quantile summaries |
| [tools/docs-prototype](../../tools/docs-prototype/build.py) | Build/validate the full Markdown browser edition using adopted formatting |

`web/catalog/manifest.json` records the committed embedded catalogue assets. Candidate image generation does not automatically promote files into `internal/web/assets`. [Testing](../development/testing.md) and [performance](../performance.md) explain appropriate checks. `experiments/` contains local artwork experiment inputs outside the current documentation tree; these are not the public application contract.
