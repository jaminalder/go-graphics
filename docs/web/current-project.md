# Current project and refactoring assessment

Inspected on 2026-09-09 at `63c622a`. This is a system overview and readiness
assessment, not a claim of an exhaustive security audit of every algorithm.

## Inventory

The repository has one Go module, **178 tracked Go files**, **16 artworks plus
the `hatchbook` specimen generator**, and no third-party Go modules in
`go.mod`. The local toolchain reports Go 1.26.5. The registry in
[`cmd/staticart/main.go`](../../cmd/staticart/main.go) is the source of truth;
the README's earlier twelve-sketch count was stale at the start of this review.

| Layer | Current responsibility | Assessment |
|---|---|---|
| `cmd/staticart` | Registry construction; render, traits, sweep, flock; flags, files, metadata | Useful CLI, but some reusable application logic is trapped here |
| `internal/sketch` | `Sketch`, `Configurable`, `Traited`, render context and registry | Small artwork contract; configuration lifecycle assumes CLI ownership |
| Per-sketch packages | Artistic choices, defaults, trait ranges, plans, painting/sampling | Preserve these responsibilities |
| `trait`, `rnd`, `opt` | Weighted choices, seed-derived traits, CLI overrides and ranges | Strong foundation; `opt` is a CLI adapter, not a web form schema |
| `geom`, `cells`, `noise`, `mathx` | Geometry, partitions, immutable fields, arithmetic | Already meaningful mechanisms; no web-driven redesign needed |
| `palette`, `gradient`, `scheme` | Provenance, colour operations and arrangement | Preserve; public colour choices need artwork-specific interpretation |
| `hatch`, `paint`, `flame` | Mark fields, sequential painting, chaos-game histogram | Distinct execution models with different resource costs |
| `render` | Parallel raster loops, AA, profiles, encoding, metadata, sheets | Reuse; add writer-based encoding when HTTP becomes a second caller |
| `tools` | Palette generation, specimen/review sheets, local experiments | Keep developer-only |
| `docs` | Sketch specs, rationale, references, worktree practice, performance evidence | Strong artistic memory; add product/runtime documentation alongside it |
| `out` | Ignored local renders | Never expose this directory as a public file server |

No web application, Terraform, Caddy configuration, deployment pipeline, or
`.github` directory was present in the inspected checkout.

## Artworks and publication readiness

Each row links its existing specification. Publication recommendations are
provisional: none of these rows constitutes artistic or legal approval.

| Sketch | Mechanism and identity | Public relevance |
|---|---|---|
| [contour](../sketches/001-contour-noise.md) | Noise-driven colour contours; simple sampler | Cheap technical pilot; needs editorial style choices for guided exploration |
| [tapestry](../sketches/002-tapestry.md) | Layered contour bands, grain, relief | Rich visual range; many interdependent CLI overrides need deliberate public presets |
| [circles](../sketches/003-circles.md) | Packed circles with patterned interiors | Clear motif; no existing trait-led public journey |
| [drift](../sketches/004-drift.md) | Painted marks along a flow field | Preserve as local artwork/paint study; profile painting costs before publication |
| [rounds](../sketches/005-rounds.md) | Bristle-brush circle study | Primarily a mechanism study; not an automatic gallery entry |
| [shoal](../sketches/006-shoal.md) | Chains of small painted dots | Distinct motif; evaluate legibility at download size |
| [qql](../sketches/007-qql.md) | Ring-dot packing, 4:5, trait schema | Strong UX reference and typed-config test case; port provenance is a publication gate |
| [pools](../sketches/008-pools.md) | Overlapping watercolour discs/rings | Launch candidate: direct visual language and existing arrangement/colourway traits |
| [foam](../sketches/009-foam.md) | Curved cells with wash, hatch, mosaic or relief | Launch candidate: strong contrast between material choices; narrowly curate the combinations |
| [scree](../sketches/010-scree.md) | Faceted, lit stone bed | Candidate after first set; useful structural configuration reference |
| [riffle](../sketches/011-riffle.md) | Water via depth, velocity, upstream sampling | Expensive sampling; postpone until VPS measurements justify interactive budgets |
| [shallows](../sketches/012-shallows.md) | Scree bed and riffle surface sampled together | Keep legitimate sketch composition; inherited bed traits do not cover all water controls |
| [warp](../sketches/013-warp.md) | Nested fBM folded material | Candidate for future style curation; not currently `Traited` |
| [iris](../sketches/014-iris.md) | Radial fibre field, pupil and rim | Launch candidate: recognisable subject and meaningful visual trait choices |
| [glaze](../sketches/015-glaze.md) | Painted warped veil over stone | Material-led long-form space; validate cost and coverage choices |
| [flame](../sketches/016-flame.md) | IFS orbit density and tone mapping | Compelling but expensive; must be judged at 1000², quality 80; separate later admission gate |
| `hatchbook` | Deterministic mechanism specimen sheets | Internal catalogue, not public artwork |

`Traited` is currently implemented by qql, pools, foam, scree, riffle,
shallows, iris, glaze, and flame. Iris currently declares **eight** dimensions,
including `rim`; descriptions saying seven are historical.
Foam's current schema also differs from older guide text: it exposes `scheme`
but no `water` trait. Build public choices from the checked-in configuration,
not from historical flag descriptions alone.

Existing pools and foam four-seed review sheets and one 1000² flame rendition
were visually inspected for this assessment. They support visual-first
selection: large-scale arrangement/material differences are legible without
exposing the underlying numeric controls. These are prior local renders, not
fresh performance measurements or final launch curation. Their presence under
`out/` is not a portable content source; production examples need an explicit
reviewed asset manifest.

## Existing strengths to preserve

Deterministic RNG streams, normalized coordinates, optional private plans,
allocation-free point sampling, palette provenance, and explicit typed
composition are already the right foundations. The preceding architecture
refactor is substantially reflected in the code: contour and foam have planned
samplers, QQL separates mark planning from painting, and shallows combines a
typed bed and water surface before rasterisation.

Do not repeat that refactor because its old checklist still has unchecked
boxes. Do not impose a universal scene/stage system, replace Go with a browser
renderer, or make every experimental sketch satisfy a public product contract.

## Gaps that matter for the web application

| Evidence | Consequence | Necessary change |
|---|---|---|
| `Registry` stores sketch instances; `registry()` currently constructs fresh instances for each CLI lookup | Reusing one registry's configured objects across HTTP requests would introduce shared mutable options | Central factory catalogue; a fresh configured sketch per job |
| `trait.Options` holds flag pointers and overrides; numeric pins depend on `opt.Set.WasSet` | Assigning struct fields from HTTP can silently fail to apply intended pins | Typed validated configuration for promoted sketches, shared by CLI and public adapters |
| `renderOne` parses flags, resolves traits, renders and builds filenames | HTTP cannot import `cmd`; turning request values into command lines would couple it to local capabilities | Separate recipe resolution/render invocation from CLI parsing and local output naming |
| `planBreed` in `flock.go` loads files and produces flag argument slices | Favourite-guided planning is not independently reusable | Extract pure candidate planning into `internal/explore`; keep JSONL/files in CLI |
| `flock.jsonl` records seed, traits, filename, mode and parent | It is not a full recipe: numeric overrides, artwork identity/version and quality are missing | New complete recipe value; preserve existing local manifest compatibility |
| Recipe metadata is a reconstructed command plus VCS revision | Defaults and algorithms can change; a seed or revision label cannot itself replay an old implementation | Edition-scoped canonical recipe and explicit replay/retirement policy |
| `Sketch.Render` has no standard `context.Context`; render, paint and orbit loops have no cancellation | Returning a timed-out HTTP response does not release rendering CPU/RAM | Hard child-process cancellation for public jobs; optional cooperative cancellation later |
| Raster loops use `GOMAXPROCS`; flame uses eight fixed deterministic orbits | Concurrent requests multiply resource pressure; changing orbit count can change output | Global admission plus renderer-specific process CPU limits; preserve numerical partitioning |
| Images and canvases allocate from caller-supplied sizes; CLI assumes trusted local inputs | Exposing width/AA/quality/oversample directly permits enormous allocations/work | Fixed public renditions and checked arithmetic before construction |
| `WritePNGMeta`/`WriteJPEGMeta` accept paths and buffer encoded output | HTTP/cache callers need safe writers and must budget temporary encoding memory | Add encoder methods over `io.Writer`; retain path wrappers for CLI |
| No HTTP state, quota, content catalogue or operations layer | These are new capabilities, not evidence the mathematical packages are wrong | Add focused modules around the art engine |

`trait.Set` is a legitimate map of known discrete dimensions; this is different
from a generic `map[string]any` for every sketch parameter. Keep that distinction
when adding recipe validation.

## Exploration semantics already in the code

`trait.Schema.Boost` raises liked values' weights without removing the base
distribution; zero-weight choices stay zero. `planBreed` allocates about half
its children to that boosted distribution and half to parent traits with a new
seed. It is trait-guided sampling, not image similarity or continuous genome
interpolation. `neighborStride = 100003` does not create perceptual neighbours.

Public refinement should build on these mechanisms but use deliberate rules
for pins, diversity, duplicates, and explicitly enabled zero-weight materials.
The current flock tests mainly cover parsing, pin argument construction and
manifest round trips; meaningful multi-round exploration tests are still needed.

## Resource evidence

[Existing local measurements](../performance.md) were taken on an Apple M1 Pro,
not on Hetzner, at AA1. Representative values:

| Artwork | Preview elapsed / RSS | 2000 px long-edge elapsed / RSS |
|---|---|---|
| contour | 0.12 s / 11.5 MB | 1.37 s / 38.2 MB |
| foam | 0.20 s / 12.2 MB | 1.77 s / 39.5 MB |
| qql | 0.56 s / 21.8 MB | 1.21 s / 106.3 MB |
| shallows | 1.11 s / 12.6 MB | 10.87 s / 40.2 MB |

QQL's 4800×6000 print render reached about 874 MB because `paint.Canvas` holds
three float64 channels per pixel. Flame at 1000² and oversample 2 has **128 MB
of raw histogram channels alone** (`1000² × 2² × 4 × 8` bytes), before density
estimation, colour buffers, output and encoding. A low-resolution label is not
a sufficient cost estimate.

Budget actual peak RSS and CPU-seconds over many seeds and every allowed style,
including failure cases. Treat wall time, total CPU and peak RSS as separate
measurements. Existing workstation timings establish priorities, not production
throughput or safe concurrency.

## Documentation and governance follow-ups

The architecture decision log contains duplicate historical numbers; new web
decisions start above its highest existing number and link to uniquely named
ADRs. Historical decisions remain intact. Decision 3's local-only CI policy
must be superseded for public deployment. Decision 52's private artwork
lifecycles remain applicable; decision 60's trait-led exploration is extended.

No repository licence file was found in tracked content. The QQL spec says
tables/formulas/layouts were copied from `qqlrs`; website terms do not establish
a licence for that source port. Inventory code, palettes, fonts, references and
public examples before publication. A free site still needs clear permissions
for what it serves; this assessment does not resolve those permissions.
