# Architecture

Design document for `go-graphics` — a Go project for generative static 2D art.
Read this before changing package structure or adding cross-cutting features.
For day-to-day agent workflow (commands, conventions, how to add things), see
[../CLAUDE.md](../CLAUDE.md).

The public application extension is documented in
[web/README.md](web/README.md) (2026-09-09). It preserves the local laboratory
and the artwork internals described here. Its product/runtime architecture is
implemented locally with pending public launch gates; shared domain terms are in [../CONTEXT.md](../CONTEXT.md).

## 1. Vision & scope

Produce aesthetic, print-quality static 2D images in Go:

- **Raster/pixel work** (fractals, noise fields, per-pixel logic) uses the Go
  standard library (`image`, `image/color`, `image/png`, `image/jpeg`).
- **Vector work** (shapes, curves, typography — future) will use
  [`github.com/tdewolff/canvas`](https://github.com/tdewolff/canvas). It is
  **not** a dependency yet; add it only when the first vector sketch lands.
- Every artwork must render at multiple sizes: small previews for fast
  iteration, web-size, and high-resolution print output — from the *same*
  deterministic definition.
- Colors are grounded in proven artist palettes from
  [ColorLisa](https://colorlisa.com/) (see
  [reference/colorlisa-palettes.md](reference/colorlisa-palettes.md)); sketches
  may desaturate/manipulate them but start from that data.

Reference for the kind of output we want:
[jaminalder/staticart](https://github.com/jaminalder/staticart) (Clojure/quil).
The first sketch reproduces
[shuffled_grad_palette_scale_2.jpg](https://github.com/jaminalder/staticart/blob/master/shuffled_grad_palette_scale_2.jpg)
— spec in [sketches/001-contour-noise.md](sketches/001-contour-noise.md).

## 2. Domain concepts

| Concept | Meaning | Package |
|---|---|---|
| `Color` | One color, `float64` R/G/B in `[0,1]`, sRGB. Manipulations: desaturate, lighten, mix, conversion to/from `image/color` and hex. | `internal/palette` |
| `Palette` | Named, ordered set of colors with provenance (artist, artwork). The ColorLisa data lives here as Go data. | `internal/palette` |
| `Gradient` | A function `t ∈ [0,1] → Color`. Implementations: cosine (Iñigo Quílez form), discrete sampled, shuffled-discrete, multi-band. | `internal/gradient` |
| Noise / field | Deterministic scalar fields `f(x, y) → float64`, e.g. Perlin + fBm octaves. Seedable, no global state. | `internal/noise` |
| `Sketch` | One artwork algorithm: deterministic function of (params, seed, size) → `image.Image`. | `internal/sketch`, one subpackage per sketch |
| Resolved values | The concrete numbers one seed's traits, ranges and explicit overrides become before geometry is built. Usually a private `levels`, `settings` or spec value. | sketch package |
| Plan / model | Optional seed-dependent data built once and reused while sampling or painting: a mark set, partition, stone bed, channel or field bundle. Plans are concrete and normally private, not one shared interface. | sketch package, or a precise domain component with a real second consumer |
| Sample / evaluation | A pure coordinate query over an immutable model. The hot path has no RNG draws, option resolution, geometry construction or ordinary allocations. | sketch or domain component |
| `Swatch` | A colour with room to move: an HSB base plus a per-channel spread and a clamp box it may never leave. Drawing from one repeatedly gives a family; stepping from the previous draw walks that family. | `internal/palette` |
| Hatch | A region filled with repeated marks: a coverage function of a point *and* the region containing it. The arranging rule (parallel, contour, radial, flow, …) is a parameter, not a type; colour is the caller's. | `internal/hatch` |
| Flame IFS | A weighted set of affine+variation maps iterated as a chaos game into a four-channel histogram, then shown with log-density and optional density estimation. Xaos (relative weights) can make the next map depend on the last. The attractor is a measure, not a function of a pixel. | `internal/flame` |
| Partition / foam | The canvas divided into curved-walled cells, each addressable: which cell a point is in, its distance to the nearest wall, how crowded it is with further cells. Per cell: area, centroid, inscribed radius. A distance *field* (Worley) cannot be filled; a partition can. | `internal/cells` |
| Colour scheme | An arrangement of colour over many discrete regions: which region gets which colour, and how dark. Fifteen strategies, each answering hue *and* value. | `internal/scheme` |
| Trait / output space | A sketch's space of outcomes as orthogonal, weighted, discrete dimensions derived from the seed and overridable per render. The idea behind QQL; the machinery is sketch-agnostic. | `internal/trait` |
| Render | Pixel-loop execution (parallel), size profiles, PNG/JPEG encoding. | `internal/render` |
| Paint | Order-dependent mutation of a float canvas by dabs, bristles, analytic rings and washes. A separate execution model from point sampling. | `internal/paint` |

## 3. Package layout & dependency rules

```
cmd/staticart/            CLI: generic flag parsing + wiring only, no art logic
internal/
  mathx/                  Clamp01, Remap, Rescale,
                          Smoothstep                         → (stdlib only)
  rnd/                    the sampling vocabulary: weighted
                          choice, gaussians, winnow, bag     → (stdlib only)
  opt/                    declarative CLI knobs: flags,
                          ranges, filename suffix            → (stdlib only)
  palette/                Color type, manipulation, data     → mathx
  gradient/               Gradient implementations           → palette, mathx
  geom/                   circles + spatial index            → (stdlib only)
  cells/                  weighted partition of the canvas
                          into fillable cells (foam)         → (stdlib only)
  scheme/                 colour arrangement over regions:
                          15 strategies, hue and value       → palette, mathx, noise, rnd
  hatch/                  filling a region with repeated
                          marks: coverage functions          → mathx, noise
  flame/                  fractal-flame IFS: variations, xaos,
                          chaos game, log-density, DE        → palette, mathx
  noise/                  Perlin, fBm, Worley, Hash01        → (stdlib only)
  trait/                  weighted output-space dimensions,
                          seed derivation, CLI overrides     → (stdlib only)
  paint/                  stamp canvas, wobbly paths, rings,
                          disc marks, watercolour washes,
                          per-pixel pigment glaze            → palette, render, mathx
  render/                 pixel loop (AA, dither), profiles,
                          contact sheets, encoding+metadata  → palette
  sketch/                 Sketch/Configurable/Traited,
                          Context, registry, Raster helper   → palette, render, trait
    sketchtest/           shared test helpers (goldens etc.) → sketch
    contour/, tapestry/, circles/, drift/, rounds/,
    shoal/, qql/, pools/, foam/, scree/,
    riffle/, shallows/, warp/, iris/, glaze/,
    flame/                the sketches                       → all of the above
    hatchbook/            specimen sheet for hatch (a
                          catalogue, not an artwork)         → hatch, palette
docs/                     this file, sketch specs, idea backlog, reference data
tools/                    code generators (palette data)
out/                      rendered images (gitignored)
```

Typical dependency direction (arrows = "may import"):

```
cmd → sketch wrappers/compositions → domain mechanisms → palette/mathx → stdlib
```

Rules:

- `mathx`, `geom`, `noise`, `trait`, `rnd`, `opt` and `cells` are leaf
  packages: stdlib imports only. `hatch` sits in the same tier but imports
  `mathx` and `noise` (see decision 38).
- Sketches live in subpackages of `internal/sketch`; `cmd` discovers them
  only through the registry. Sketch-specific CLI options are owned by the
  sketch via the `Configurable` interface — `cmd` stays sketch-agnostic.
- Nothing imports `cmd`. No package keeps global mutable state.
- A sketch may import another sketch when it genuinely composes that complete
  artwork or its documented pre-raster model. If the dependency exists only
  because a reusable mechanism is trapped behind the artwork wrapper, extract
  the narrow domain component instead. Do not ban or introduce sketch imports
  mechanically.
- New third-party dependencies require a documented decision (§8). Current
  count: **zero**.

### What changes where (knob taxonomy)

When making variants of a sketch, changes fall into four tiers — keep each
knob in its tier:

0. **Traits** (CLI, per render): for sketches with a `trait.Schema`, the
   discrete orthogonal choices a seed resolves to (`--structure shadows`).
   Pinning one leaves every other dimension exactly as that seed drew it,
   so this is the cheapest way to steer without losing a composition.
1. **Seeds** (CLI, per render): composition (`--seed`) plus sketch-owned
   sub-seeds (`--terrace-seed`, `--grain-seed`) that re-deal one aspect on
   a fixed composition. Cheap exploration; embedded in the recipe.
2. **Optional layers/effects** (CLI flags): `--relief`, `--crackle`,
   `--no-stripes`, `--smooth`, … Toggles must not disturb the base
   composition (dedicated RNG streams).
3. **Aesthetic ranges** (code): the bounded per-seed draw ranges and
   structural constants in each sketch. Changing them changes every seed —
   do it deliberately, update the sketch spec, regenerate goldens.
4. **Quality/output** (CLI, composition-neutral): `--aa`, `--deep`,
   `--format`, profiles.

### Working the output space

The unit of work is not an image, it is a space. A change to a sketch is
judged by sweeping it, not by looking at one render — one seed says almost
nothing about what a change did.

```sh
staticart sweep pools --seeds 1-12 --vary fill=busy,packed --profile web
staticart sweep tapestry --seeds 1-20 --vary relief-preset=baseline,deep-carve
```

`sweep` strips its own flags (`--seeds`, `--vary`, `--out`, `--cols`,
`--cell`, `--jobs`) and passes everything else to `render` untouched, so a
knob added to a sketch today is sweepable without touching the command. It
writes the renders, a `manifest.txt` naming each tile, and one `sheet.png`
— box-downsampled, because point sampling turns fine rings and dithered
gradients into moiré and a thumbnail that lies is worse than none.

## 4. Core invariants

These are load-bearing; breaking them is a bug even if output "looks fine".

1. **Determinism.** Same (sketch, params, seed, width, height) → identical
   image, on every run and platform. All randomness flows from the seed via
   `math/rand/v2` (`rand.New(rand.NewPCG(seed, …))`) or seed-derived tables.
   Never `time.Now()`, never package-global `rand`.
2. **Resolution independence.** Sketches sample in *normalized* coordinates:
   `v = (y+0.5)/H ∈ [0,1]`, `u = (x+0.5)/W · aspect` with `aspect = W/H`.
   Frequencies are "cycles per canvas unit", never per pixel. A preview render
   and a print render of the same seed must show the same composition, just at
   different pixel densities.
3. **Color space.** Internal color math is `float64` sRGB components in
   `[0,1]`; clamp on conversion to 8-bit. (Good enough for this art style;
   revisit with a decision entry if we ever need linear-light blending.)
4. **Palettes carry provenance.** Every palette knows its artist/artwork
   source. Derived palettes (desaturated etc.) are computed from the originals
   at use-time, not stored as mutated copies.

## 5. Artwork lifecycle

`Sketch.Render(ctx)` is the stable outer boundary. Behind it, an artwork uses
only the stages that make its algorithm clearer. There is no shared `Plan`,
`Scene`, `Stage`, scalar-field or color-sampler interface.

### Simple point sampler

`contour` is the minimal example:

```text
definition fields + Context
  -> plan: validate palette, build shuffled gradients and Perlin field once
  -> plan.At(u, v): field value -> gradient colour
  -> sketch.Raster(ctx, plan.At)
```

The private plan is useful because its `At` operation is independently
testable. A still simpler sketch may keep those immutable values local to
`Render`; a plan is not mandatory.

### Structural point sampler

`foam` and `scree` resolve ranges, build measured partitions and assign
appearance before sampling:

```text
traits + explicit overrides
  -> concrete levels
  -> sites / partitions / facets / per-region material
  -> immutable planned sheet or bed
  -> pure sample: fill -> subdivision -> hatch/light -> joint
  -> sketch.Raster
```

Expensive packing, measurement, scheme resolution and per-facet light belong
before the pixel loop. Coordinate-dependent material behavior remains in the
sample when that is the mechanism, not avoidable setup.

### Planned sequential painter

`qql`, `pools`, `drift` and `shoal` use the other execution model:

```text
resolved values -> planned marks -> paint.Canvas in explicit order -> Image
```

Painting is sequential and order-dependent. It may consume a dedicated paint
RNG stream. Soft dab edges provide anti-aliasing, so `Context.AA` is unused.
Composition remains in canvas units; exact stroke texture may vary subtly with
resolution as a property of the medium.

### Composed material

`shallows` plans a `scree.Bed` and a `riffle.Surface`, then combines their
typed samples before rasterization:

```text
planned faceted bed + planned all-water surface
  -> refracted bed lookup
  -> water tint, ripple shadow and focused light applied to bed material
  -> sketch.Raster
```

There is no intermediate image. `riffle.SurfaceSample` keeps water-specific
direction, slope, ripple and dapple rather than reducing the component to a
weak generic vector field. `scree.Bed.At` exposes the complete generated bed
because that exact model has a real second consumer.

### Chaos-game histogram

`flame` is the first of these. The interesting object is a measure on the
plane, not a colour at a coordinate and not a sequence of stamps:

```text
traits + overrides
  -> IFS (weighted xforms + variations + optional symmetry)
  -> camera in the mathematical plane
  -> chaos game into a four-channel histogram
  -> log-density, gamma, vibrancy
  -> render.ImageFromColors
```

`--aa` multiplies samples per pixel rather than evaluating a function on a
subpixel grid. Spatial anti-aliasing is histogram oversample plus a
Gaussian downsample (`--oversample`). See decisions 56 and 59 and
[sketches/016-flame.md](sketches/016-flame.md).

### Output rendering

All paths return `image.Image` to the command. `sketch.Raster` and
`RasterLayer` delegate normalized coordinates, parallel rows, supersampling,
linear-light averaging and quantization to `internal/render`.
`paint.Canvas.Image` shares final dithered quantization through
`render.ImageFromColors`. The CLI alone chooses filenames and calls
`WritePNGMeta` or `WriteJPEGMeta` with DPI, recipe, traits and code revision.

Per-coordinate samplers must be pure so row-parallel rasterization is
race-free by construction. Their normal path should allocate nothing and must
not resolve options, consume mutable RNGs or rebuild geometry.

### Size profiles

| Profile | Pixels (square default) | Purpose |
|---|---|---|
| `preview` | 600 × 600 | fast iteration, visual checks by agents |
| `web` | 2000 × 2000 | matches the original staticart canvas |
| `print` | 6000 × 6000 | ≈ 50 × 50 cm at 300 DPI |

`--width/--height` override for custom/non-square sizes. Non-square must work
for every sketch (invariant 2 handles composition).

## 6. Sketch contract

```go
// internal/sketch
type Context struct {
    Width, Height int
    Seed          uint64
    Palette       palette.Palette
    AA            int  // supersampling per axis; composition-neutral
    Deep          bool // 16-bit output
}

// RNG returns a generator derived from (Seed, stream). Sketches use distinct
// stream ids per consumer (shuffle A, shuffle B, …) so adding a consumer
// never disturbs the values existing streams produce.
func (c Context) RNG(stream uint64) *rand.Rand

type Sketch interface {
    Name() string        // CLI id, kebab-case, e.g. "contour"
    Describe() string    // one line for `staticart list`
    Render(ctx Context) (image.Image, error)
}

// Optional: sketches with CLI options implement Configurable —
// Flags(fs) registers them, Configure() applies them and returns the
// output-filename suffix. cmd never type-asserts concrete sketches.

// Optional: sketches whose output space is a trait.Schema implement
// Traited — Schema(), Traits(ctx) and TraitSuffix(set). cmd uses it for
// `staticart traits <sketch>`, for the trait part of the filename, and to
// record the resolved set in the file's metadata. Deriving must be cheap:
// it happens before rendering.
type Traited interface {
    Sketch
    Schema() trait.Schema
    Traits(ctx Context) trait.Set
    TraitSuffix(set trait.Set) string
}
```

- Sketch-specific tunables may remain fields on the sketch struct with
  defaults in `New()` when the sketch is the only consumer. Use an explicit
  concrete configuration value when another artwork constructs the component,
  tests otherwise need to simulate CLI parsing, or a planned value would
  retain mutable CLI state. Never use a generic params map, and do not convert
  every sketch for uniformity.
- Sketches whose interesting choices are *discrete and orthogonal* should
  declare a `trait.Schema` instead of hand-rolling flags: it gives seed
  derivation, per-dimension overrides, filename and metadata plumbing, and
  the `traits` command for free. `qql` is the first user; see
  [sketches/007-qql.md](sketches/007-qql.md).
- `cmd/staticart` constructs the registry explicitly from each sketch's
  `New()` value. There is no `init()` registration or global mutable sketch;
  each command/render lookup gets fresh CLI adapter instances.

### What belongs in a shared package

Keep code in the sketch package when it chooses aesthetic ranges,
probabilities, ordering, palette interpretation, composition or deliberate
exceptions. Extract a mechanism only when it has a coherent domain name and
responsibility, is independently testable, and has at least two real consumers
or an existing composition crossing artwork boundaries.

Plans remain private by default. Promotion does not require making a component
generic: a planned faceted stone bed or an all-water surface is a valid domain
component. Do not create `utils`, `helpers`, `common`, `pipeline`, `stages` or
`models` packages.

### RNG ownership by stage

Every random consumer belongs to a named lifecycle stage. A constructor
receives `ctx.RNG(stream)` or a derived seed explicitly; resolved values and
planned models do not retain generators unless a sequential painter itself is
the consumer. Optional effects use dedicated streams. Per-coordinate
variation uses seed-keyed fields or hashes, never random draws.

Existing numeric stream IDs are part of seed behavior. Adding a new stream
does not disturb another; adding draws inside one stream intentionally changes
later values in that stage. Stream IDs within a sketch must be unique unless
correlation is deliberate and documented.

## 7. Testing strategy

- **Unit tests** (table-driven) for all math: color conversions and
  manipulation, gradient sampling endpoints/monotonicity, noise value range,
  interval mapping. Write these first (TDD) — the math is where silent bugs
  live.
- **Determinism tests**: render a sketch twice at 64×64 with the same seed,
  require byte-identical pixels; different seeds must differ.
- **Golden-image tests**: 64×64 PNG per sketch committed under
  `internal/sketch/<name>/testdata/`; compare pixel-exact (helpers in
  `internal/sketch/sketchtest`). Regenerate deliberately with
  `make golden` when a sketch intentionally changes, and eyeball the new
  goldens before committing. Goldens render at AA 1.
- **Visual verification is part of done**: render a preview and *look at it*
  (agents: `Read` the PNG). Tests prove determinism, not beauty.
- **Resolution tests**: build plans at two pixel sizes with the same aspect and
  compare structural values or samples at identical canvas coordinates.
- **Intermediate tests**: assert geometry bounds, coverage, adjacency,
  material assignment, facet-flatness, field ranges and sampler purity before
  relying on a golden. This distinguishes a changed composition from changed
  rasterization.
- **Benchmarks**: separate planning from repeated sampling or painting. Direct
  `At` methods should report zero allocations. Representative results and
  print-memory implications are in [performance.md](performance.md).

## 8. Decision log

| # | Decision | Rationale |
|---|---|---|
| 1 | Module path `github.com/jaminalder/go-graphics` | Matches owner's GitHub handle + directory name (user-confirmed). |
| 2 | Implement Perlin/fBm noise in-repo | ~150 LoC, zero deps, full control over seeding/determinism; good base for turbulence/curl later (user-confirmed). |
| 3 | No CI for now; local `make check` is the gate | Repo is local-first (user-confirmed). Add GitHub Actions when pushed. |
| 4 | Stdlib raster now, `tdewolff/canvas` deferred | First sketches are pure pixel work; don't take the dependency before the first vector sketch. |
| 5 | Reproduce the *look* of sketch_7, not bit-exact output | Original uses thi.ng simplex noise + JVM shuffle; we use our own Perlin. Thresholds/scales are tuned visually against the reference image. |
| 6 | `float64` sRGB color math, no linear-light pipeline | The source material (thi.ng gradients) also interpolates in sRGB; simpler, matches reference. Revisit if blending artifacts appear. |
| 7 | golangci-lint v2 config format | Migrated from v1 on 2026-07-29 alongside the Go 1.26.5 bump; local toolchain is v2.12.x. `make fmt` now runs `golangci-lint fmt`, so formatting and linting share one config (`formatters:` section) instead of invoking gofumpt/goimports separately. |
| 8 | Gradient interpolation in HSL space for new work | RGB-space blends between distant hues gray out; HSL (shortest hue arc) stays vivid and closer to the palette (user request 2026-07-22). `palette.LerpHSL` + `gradient.HSLBetween`. The contour sketch keeps RGB cosine gradients for fidelity to the original staticart port. |
| 9 | Output pipeline: linear-light AA averaging, dithered 8-bit quantization (IGN), optional 16-bit masters, embedded metadata | Averaging supersamples in sRGB darkens high-contrast edges — AA averages in linear light (partially supersedes #6). 8-bit quantization dithers deterministically to prevent banding in slow gradients; `--deep` renders 16-bit PNG masters. Every written file embeds sRGB tag, 300 DPI, and the full render recipe + code revision (`render.Meta`) so artworks stay self-describing. |
| 10 | Brush texture comes from bristle geometry, not from marks drawn on top | The first `rounds` draft textured shapes with wobbly closed loops at random radii; because they crossed each other it read as scribbling over a circle rather than as paint. `paint.Brush` instead sweeps a bundle of laterally offset bristles along a path, so streaks are parallel to the stroke by construction and any path picks up the same character. Disorder lives in the surface; path wobble stays ≤1.2% of radius so silhouettes stay circular (user feedback 2026-07-29, spec: sketches/005-rounds.md). |
| 11 | Brush lift-off length (`grain`) is independent of ferrule width | Tying the dry-out wave to the brush's own width breaks a fine liner's marks into stitched dashes, since a thin brush still smears over long distances. Grain is a property of the hand, in canvas units, defaulting to 8× width. `TestDryStreaksRunAlongTheStroke` pins both the direction and the length of the gaps. |
| 12 | A swept ring must not be shorter than its brush's grain | `SweepRing` seals the centre of a disc with a dab and caps the grain to a few cycles per revolution. On a ring shorter than its grain every bristle holds one constant coverage the whole way round, so a lifted bristle drops a *complete* concentric ring instead of a streak — small painted dots came out looking like targets with a pinhole in the middle (found building sketch 006). |
| 13 | Watercolour composites as absorption in linear light, with a back-scattering floor | `paint.Wash` stacks transmittances (`T = exp(−n·α)`) rather than interpolating toward the new colour, which is why two pigments crossing give a believable third instead of whichever was painted second. Absorption is radiometric, so it happens in linear light; multiplying sRGB darkens at the wrong rate and turns crossings to mud. Pure absorption marches to black, so a fraction of the pigment's own masstone is scattered back (`R = R·T + Rf·(1−T)`) — the difference between glazed layers staying luminous and palette-mixed ones going dead. |
| 14 | Wash detail is shared by every deposit; only broad wobble varies | A pool is ~40 near-transparent deposits (≈4% each, matching Hobbs). The fine harmonics of the outline are computed **once** and shared: detail that differs per deposit averages out across the stack into a fuzzy halo, which is the classic tell of procedural watercolour. Deposits differ only in low-frequency wobble and in how far they reach, which spreads the boundary into a soft band without blurring it away. Amplitudes fall as 1/k^1.4 — shallower serrates the outline into a starburst. |
| 15 | Pools are painted one at a time, not interleaved | The usual recipe interleaves layers between blobs so neither ends up wholly on top. Absorption commutes exactly, and the scattering floor breaks that by only a couple of levels out of 255 (`TestWashOrderBarelyMatters`), so interleaving would buy nothing visible and pools stay independent. |
| 16 | QQL is ported natively, not entropy-exactly | `qqlrs` reproduces the original JavaScript bit for bit — murmur2 seeding, a PCG variant with a JS bit-drop anomaly, table sin/cos, a Newton sqrt, a cached gaussian deviate, deliberate unused draws. All of it exists to make *minted* seeds reproduce, which this project has no use for, and porting it would mean a foreign RNG, a foreign seed type and a foreign coordinate model sitting outside every invariant here. Sketch 007 copies the weight tables, spec formulas and layouts verbatim and runs them on `ctx.RNG(stream)`, `uint64` seeds and canvas units. A seed is not a minted piece; the vocabulary and the distributions are (user decision 2026-07-29, spec: sketches/007-qql.md). |
| 17 | Traits live in `internal/trait`, and their weights are honoured | An output space of orthogonal, weighted, discrete dimensions is QQL's central idea and the thing worth keeping — so it is a stdlib-only leaf package rather than sketch-private, ready for tapestry and the watercolour work to declare a schema. `qqlrs` derives traits by masking seed bits and **discards the declared weights**, an artifact of the on-chain mint: it makes flow fields uniform where the table says spirals should be four times as common as explosive ones. Drawing by weight restores the quality-floor-with-outliers shape the essays argue for. One table needed a real number: `Turbulence::None` carries weight 0 yet the bit mechanism produced it ~50% of the time, so it is given weight 2 (≈33%) — common enough to keep QQL's clean fields, damped so `low` stays the default. |
| 18 | Ring bands are analytic annuli, not stroked polygons | QQL strokes each of a band's many concentric rounds as a tessellated polygon; with zero third-party dependencies that would mean writing a path stroker, and a dab train (`Canvas.Stroke`) costs roughly 7× more pixel work per ring. `paint.Ring` solves each row's spans directly, which is what makes 100k+ ring-dots affordable at print size. The cost is the polygon faceting that makes `qqlrs`' smallest dots visibly octagonal at `--min-circle-steps 8`; it is invisible below a few hundred pixels. The band-level jitter that gives the motif its worked edge is kept in full — that, not the faceting, is where the texture comes from. |
| 19 | `geom.Index` gained explicit bounds and an explicit cell size | Circle packing with an overscan needs both: the old constructor covered `[0,max]` and *silently dropped* a circle whose cell range was entirely negative, and it sized cells by the largest radius — fine when radii are uniform, quadratic when they span two orders of magnitude as QQL's do. Cell size is now documented as a performance knob only (`TestCellSizeDoesNotChangeResults` pins that), and circles outside the bounds are pinned to the nearest edge cells, which costs extra distance checks but never hides a neighbour from a circle inside the bounds. |

| 20 | A watercolour ring is its own primitive, not a pool with the ground repainted over it | Both boundaries of a brushed circle of water are wet edges, so both dry with a rim and the pigment banks toward the middle of the band from each side. Covering a pool with a disc of paper colour gets none of that, and worse, it is opaque — it would erase whatever the ring was laid over, in a model whose whole point is that marks stack transparently. `Wash.Ring` gives each deposit an inner radius table as well as an outer one, with its own outline and softness (a hole that echoes the outside reads as an extruded shape). `Wash.Pool` is the `rInner = 0` case and its output is unchanged, byte for byte. |

| 21 | The wash medium is a weight-0 trait appended to QQL's schema, not a plain flag | It is a discrete, orthogonal choice, so the trait system already owns the flag, the validation, the filename fragment and the metadata line — writing a second mechanism beside it would be the overengineering the standards warn about. Appending is safe because `Derive` draws in schema order: the twelve dimensions above it are untouched, and at weight 0 the wash is unreachable from a seed, so no existing seed lands on a different piece (`TestSchemaIsValid` pins both the order and the weight). The medium is a fact about the material, not about the work, which is exactly why it may not be in the space a seed samples. |
| 22 | A wash deposit is pulled back by a fraction of the band's *width*, not of its radius | Found by putting the annulus under QQL's thin bands. Deposits are held back from the boundary so that only the last group reaches it, which is what banks pigment at the rim; the pull-back was a tenth of the radius, which on a narrow ring is wider than the whole band — every deposit but the last landed inside-out and contributed nothing, so the ring dried at a fraction of the strength it was asked for and had no rim. For a filled pool the band and the radius are the same thing, so `Pool` is unchanged byte for byte. The same fix visibly strengthened sketch 008's inner rings. |
| 23 | A ring's two rims combine by taking the stronger, not by adding | A point in the band belongs to whichever wet edge the water left it at. Summed, a narrow ring — where both rim tails span the whole width — is lifted by twice the rim everywhere and dries as one solid dark stripe, which is a drawn circle rather than a dried one. |
| 24 | `Wash` gained `Body` alongside `Scatter` | Scatter alone cannot put light on dark: a pale pigment barely absorbs, so however much it scatters, the stack still transmits most of the ground and a white mark on a dark ground stays a ghost — measured at 211,193,168 against a 209,188,157 ground before the fix. Hiding has to come off the transmittance, so `Body` subtracts from each deposit's absorbance and raises the back-scatter floor toward the pigment's own colour. At `Body` 0 (the default, and shoal's) nothing changes and `Pool` stays byte-identical; at 1 the mark dries as flat pigment colour. It is the difference between watercolour and gouache, and it is what makes the medium usable on QQL's dark grounds. |

| 25 | The wash medium is laid below full strength, and that is the setting it turns on | Everything that makes a wash look like one — the rim, the granulation, the pooling — works by varying how much pigment reached a pixel. Laid at full strength the mark saturates: the variation lands on a stack that is already opaque, and every cue is crushed flat, which is what the first version did at alpha 0.9. Held at 0.75 the body of a band still transmits some ground while the rim can go all the way to the pigment, and the difference between the two is the rim. Body then does the opposite job — it keeps the colour the pigment's rather than the ground's — so the two are tuned against each other, not together. |

| 26 | `Wash` caps a ring's raggedness against its own width | Raggedness is an edge deviation measured against the radius, which is right for a pool's silhouette and wrong for a narrow band: the outer and inner boundaries wander independently, so once the deviation approaches the width they cross and the ring dries as a string of beads. The cap lives in `lay` rather than in each caller because it is a property of the primitive, and because it is what makes a run of fine concentric rings — sketch 008's banded circle — possible at all. A filled pool is as wide as it is round, so the cap never binds and `Pool` is untouched. |

| 27 | The ground is a per-pixel field, not a wash of shapes | A flat wash dries with the unevenness of its own laying in it, so sketch 008's ground has to vary — but the first version covered the sheet with a grid of overlapping `Pool`s, and a pool is a shape. However soft its edge was made (raggedness up, rim almost off, generous bleed past the frame), every boundary that fell inside the canvas stayed legible as a fine arc, which on a picture full of circles reads as more circles. `Wash.Ground` evaluates the pigment density per pixel instead — broad-scale fBm for the pooling, cell noise for the tooth — so there is nothing that *can* have an edge, and the only structure in the ground is the structure asked for. It reuses the pool's pigment maths exactly (linear-light absorption, back-scatter floor, `Body`), so a ground and a mark in one colour agree, and it costs a single pass instead of the two-plus coverage a grid needs. |

| 28 | Past `MaxBands` the banded mark widens its rings instead of adding them | The ring pitch is a width, so the ring texture keeps its weight as a mark grows — which is right until the mark is large, at which point it goes on accumulating rings until it reads as a target. What makes the mark is a ring wide enough to be a band of colour in its own right, with its own wet edge and rim, and that is exactly what a rising count destroys. So the rule inverts at the cap: the count stops and the pitch grows. It is the same trade a painter makes, and it means the size ladder can reach much further up — a disc of a quarter of the canvas is still five bands — without the largest marks turning into a different kind of object from the small ones. |

| 29 | The sampling vocabulary and the CLI-knob boilerplate became leaf packages (`rnd`, `opt`) | Both had been written for one sketch and then reinvented, more weakly, in others: `pools` and `shoal` each grew a private weighted-bag, and every sketch grew the same forty lines of range checks. `rnd` also names the idea rather than just sharing it — a parameter is a *weighted choice among a few hand-picked options, softened by a gaussian*, which is what gives an output space outliers instead of uniform noise. `opt` fixed a bug class while it was at it: knobs are registered with the sketch's real defaults and "was it set" is read from the FlagSet, where the hand-written versions used a `-1` sentinel that hid the true default from `--help`, made a negative range unexpressible (`--gap -0.05`), and silently swallowed `--count -2` as "unset". |
| 30 | Sketch 008 has a `fill` trait; its composition knobs are overrides on it | Filling a frame takes the count up, the ladder down, the clearance negative and the margin closed, *together* — raising the count alone gives the same picture with more specks in it. Those six numbers were being set together by hand on every render, which is a single decision wearing six hats. `fill` is that decision, and like QQL's traits it resolves to *ranges* rather than numbers, so two seeds at the same level differ in how many marks, how large and how crowded while both still read as busy. The individual flags remain, now as overrides that only apply when actually given — which is what `opt.Set.WasSet` is for. |
| 31 | tapestry's options were left hand-written | `opt` earns its place where a sketch has a list of ranged knobs. Tapestry's `Configure` is not that: it is flag *implications* (`--relief-preset` implies `--relief`; `--grain-seed` implies `--grain` unless `--crackle`), which is domain logic and reads better as the code it already is. Converting it would have been churn for uniformity's sake and would have risked the recipes of finished pieces. Extract where there is duplication, not everywhere there is a pattern. |

| 32 | Sketch 008 borrows QQL's structures *and* its walk | The first attempt took only the seeding, on QQL's own note that the structure matters more than the field. That is true of the composition and false of the surface: what makes a QQL piece recognisable is that its marks touch — contiguous strands that curve, each holding one size and one whole colour scheme for its length. A structured grid of scattered marks has the same large-scale arrangement and reads as a scatter, which is what a review caught. Marks now advance by their own diameter along a field, which a fixed candidate grid cannot do, since the step has to follow the size of the mark being laid. Reimplemented in the sketch rather than extracted from `qql`: QQL's versions are entangled with its `frame`, its spec machinery and its tracer, so sharing would have meant retrofitting the port and risking its output for a third caller that does not exist. |

| 33 | Sketch 008's palette is a trait, not just the `--palette` flag | A sketch whose colours sit outside its output space cannot be swept: every seed comes out in whatever `--palette` said, and varying the palette instead gives the cartesian product — one composition repeated once per colour, which is one picture shown five ways rather than five pictures. QQL always had this (`--qql-palette`); 008 was the odd one out, and a review of a 45-piece sweep is what made it obvious. The list is curated rather than the whole ColorLisa set, because a transparent wash on tinted paper needs pigments dark enough to read against their own ground; `from-flag` carries weight 0 and hands colour duty back to `--palette` for anything outside it. |
| 34 | A partition (`internal/cells`), not another distance field | Sketch 009 needed regions that could be *filled* — a wash in one, hatching in the next — and `noise.Worley` answers only "how far to the nearest site". The addition is small (which cell won the argmin, plus one measuring pass for area/centroid/inradius) and it is what turns cell noise into a structure a sketch can address. It is a leaf package rather than sketch-private because filling a partition is a whole family of sketches, not one. The metric is additively weighted (Apollonius) so the walls are arcs; sites may be merged so a cell can be a concave lobe. |
| 35 | The junction measure is a smooth crowding count, not "how near is the third cell" | The ink swells where cells meet, which needs a number that is 1 at a junction and 0 mid-wall. Ranking gives one for free — but ranking is not smooth in position: the identity of the third-nearest cell swaps along rays running out of every junction, and the measure creases along them. Fed into the line width the creases came out as sharp spikes radiating from every node, invisible at 600px and unmissable at 6000. Summing `exp(−(dₖ−d₂)/σ)` over *every* further cell depends on no ordering at all, and the same sum makes a four-way junction swell more than a three-way one, which is correct. |
| 36 | Sketch 009 bends the plane rather than only the metric | A weighted diagram curves a wall only where the two sites either side of it differ sharply in weight, and in a packed sheet most neighbours are the same size — the result reads as a cracked pane. A curl-noise domain warp, wavelength many cells long and displacement a fraction of one, curves every wall for two noise samples. Curl rather than a plain gradient because it is divergence-free: it shears the plane without compressing it, so cells come out bent rather than squeezed. It is bounded by a tanh limiter and by widening the pack's overscan, so a displaced lookup still lands among sites. |
| 38 | Hatching is a coverage function of a point *and* its region (`internal/hatch`) | The repo renders by evaluating a pure function per pixel, so the natural primitive for "fill this region with marks" is `Sample → [0,1]`, not a stroker or a path list. Colour stays outside, which is what makes it general: one function serves ink on paper, two-colour cross-hatching, a tonal screen and a mask for a wash. The structures — parallel, contour, concentric, radial, fan, flow, scribble, stipple, chord — are *changes of coordinates* feeding one shared mark-maker, so a parameter added to the mark-maker (dashes, jitter, tonal thinning) applies to all nine at once; cross-hatching, weave and nesting are combinators over coverage functions rather than structures of their own. `Sample` is a bundle of numbers (centre, axis, wall distance, reach, tone) rather than a shape interface, so `internal/cells` satisfies it exactly without `hatch` depending on it, and so can a circle or a quarter of a square. It imports `mathx` and `noise` — noise because flow hatching is the level sets of a Perlin stream function and the jitter and dash phases are hash lookups; both are stdlib-only leaves, so the leaf tier is intact. |
| 39 | Mark thickness is a fraction of the spacing, and a density gradient thins by halving | Two decisions that make hatching *fit* a region. An absolute width turns a hatch fitted to a small cell solid as the cell shrinks; the line-to-gap ratio is what an engraver actually controls, and it scales for free. And a tonal gradient cannot stretch the pitch: a lattice whose pitch varies with position has to split or merge marks somewhere and both are visible, so tone instead drops every other mark and then every other survivor, which leaves every surviving mark exactly where it was at full tone. |
| 40 | Flow and scribble divide the coverage distance by the field's gradient | Their across coordinate is a noise field rather than a distance, so consecutive level sets are pitch/|∇ψ| apart. Widths taken as a fraction of *that* gap come out as blobs where the field is slack and vanish where it is steep — the first specimen sheet showed both. Dividing the distance by the gradient and leaving the width alone gives marks of one width that converge and diverge, which is what a flow field looks like. It costs two extra field evaluations per sample and is applied to those two structures only. |
| 41 | The specimen sheet is a registered sketch (`hatchbook`), with its manifest generated by `tools/hatchbook` | A catalogue is not an artwork, but making it a sketch gets the size profiles, palettes, supersampling and embedded recipe for nothing, and — the part that matters — it drives `internal/hatch` through exactly the pipeline a real sketch uses, so `TestASheetIsIdenticalAtAnyResolution` is evidence about the invariant rather than about a bespoke harness. It is the one sketch that deliberately ignores its seed: every square is pinned, because a specimen that redrew itself per seed could not be cited. The squares carry no labels (no fonts, no third-party dependencies), so the row/column key is printed by a tool reading the same tables the sketch draws from. |
| 37 | The wall distance is measured against a *soft* minimum | The partition's cells meet at 120°, so against a hard minimum every cell is a polygon with curved sides — and at a glance a polygon is what it reads as, however well the sides curve. A soft minimum over the other cells (`−σ·log Σ exp(−dₖ/σ)`) pulls the wall in wherever two of them are close at once, which is at a corner and nowhere else: mid-wall the sum has one term and the two minima agree, so straight runs of wall are untouched while the corners they run into are rounded over σ. It costs one more accumulation in the same loop that already computes the crowding, and it does enough of the junction swelling's job that `swell` halved. |
| 38 | Sketch 009's watercolour is analytic per pixel, not `paint.Wash` | Reusing 008's wash was the obvious move and it is the wrong one, for two reasons that are both about shape. `Wash` is *stamp-based* — it writes pixels into a `paint.Canvas` sequentially, in pixel coordinates — and 009 is one pure per-pixel function, which is what makes it the same picture at preview and at print; adopting `Wash` would mean giving that up for the whole sketch. And `Wash` is *radial*: a pool is a star-shaped blob described by one radius per angle, which cannot express a concave lobe, and that silhouette is most of what `Wash` is. Meanwhile the thing `Wash` has to synthesise, 009 already has exactly: `cells.Hit.Wall` is a real signed distance to the cell's own boundary whatever its shape, so the rim, the overshoot, the bleed and the backrun front are all one-line functions of it *and they work inside a crescent*. What is genuinely shared is the pigment maths, and that was extracted rather than reimplemented — see 39. |
| 39 | `paint.Glaze`: the wash's pigment model taken continuous | `Wash` builds a mark from ~40 near-transparent deposits and asks how many reached a pixel; the answer is a layer *count*, and the colour follows from stacking that many transmittances. A caller that knows analytically how much pigment reached a point wants the same physics with the count handed to it. Taking the deposit thickness to zero at fixed total gives Beer–Lambert, `T = exp(−load·(1−L))` in linear light, with `Wash`'s back-scatter floor unchanged — so two glazes of load 1 are exactly one of load 2, glazing commutes, and a foam wash and a pools wash of one pigment agree about that pigment. It is additive: `water.go` is untouched and sketch 008 is byte-identical. |
| 40 | `cells.Hit` gained `Next`, the cell across the wall | `Wall` says how far a point is from a boundary; nothing said *whose*. Two of the most characteristic things watercolour does need that — paint failing to register with the drawn line, and two cells painted while both are wet mixing across it — and both turn out to be the same mechanism: the neighbour's dressing evaluated at the mirrored wall distance, since a point is as far outside its neighbour as it is inside its own cell. Two lines in the lookup that already had the answer in hand. |
| 41 | Sketch 009's watercolour is two appended trait dimensions, and its fill level carries weight 0 | The manner weights, the pigment load, the registration error, the granulation and the wall wetness are set *together* to get a named look, which by the standards in CLAUDE.md is one knob, not five — so `water` is a dimension resolving to ranges, with the individual numbers left as overrides. `scheme` is genuinely orthogonal to it (how one cell is painted versus how colour is spread over a hundred and fifty of them) so it is a second dimension rather than a product baked into the first. Both are appended, and `watercolour` is appended to `fills` at weight 0, so that no existing seed of 009 is moved: `Derive` draws once per dimension in schema order, and a weight-0 value does not change a dimension's total. Same argument as 21. |

| 38 | Sketch 009 subdivides with **one** global inner partition, not a site set per cell | A point's identity becomes the pair (outer cell, inner tile), both read at the same warped coordinate, and the outer ink is laid afterwards — so the heavy line clips the fine net for free and there is no clipping code anywhere. A foam per cell would mean forty measuring passes instead of one and would still have to solve the same border problem. The objection to a global foam is that one site spacing cannot give a big lobe and a sliver comparable tile counts; the answer is a *variable-radius* dart throw whose radius at a point is a fraction of the inradius of the outer cell that owns it. The spacing follows the outer structure while the partition stays global. Inner sites carry weight 0, so the tiles are an ordinary Voronoi — crystal inside organic reads as two things, where a second bubble cluster inside the first reads as a blur. |
| 39 | Relief is differenced out of the wall field, and its height is in canvas units | `cells.Hit.Wall` is a real signed distance field, so a height built from it can be central-differenced into a normal and lit — the sheet gets a surface with nothing modelled, and the partition's creases become the surface's edges. The height is a *rise in canvas units* and the difference step is a canvas length, which is the whole of invariant 2 here: a step of "one pixel" gives a chamfer that hardens as the render grows. The outer cells carry the large form and the mosaic's tiles the facets on top at half the rise, so one foam field carries relief at two scales. `cells.Hit` also gained `Near` (the cell across the nearest wall) — the companion of `Wall`, and what a fill needs to know whose neighbour it is standing next to. |
| 38 | Colour arrangement is a package (`internal/scheme`), and every strategy answers *value* as well as hue | Sketch 009's colour organisation started as seven schemes private to the sketch, but arranging colour over a set of regions is not a fact about foams — a packing, a mosaic or a set of brush marks all want the same vocabulary, exactly as `internal/hatch` does for marks. The load-bearing part is the `Tone`: an arrangement of hue with no value structure goes flat grey when you squint at it, which is the commonest way a correctly-harmonised palette still comes out looking like a swatch card. A test asserts a real tone spread for every strategy. Two further rules earned their way in from failures: hue and value get *different* spatial fields (sharing one made `by-darkness` agree with `passage` on 85% of cells — one idea shown twice), and the dilution belongs in the Tone rather than baked into the Fill (baked in, a near-monochrome sheet reported 112 distinct pigments and a caller laying a wash had no load left to read). |
| 39 | Schemes paint with the palette they were given, rather than synthesising hues | The first cut of `analogous`, `triad` and `notan` did the textbook thing — `FromHSL(h+120, …)` — which is correct colour theory and wrong for this repo: the palettes carry an artist and an artwork, and a strategy that invents its own hues has stopped painting with them, leaving the provenance in `internal/palette` as decoration. They now *select* from the palette — an arc of the hue wheel, three members greedily spread round it, the darkest/middle/lightest — with lightening, desaturating and mixing allowed. A test bounds every fill's hue to 40° of something in the palette. |
| 40 | A region is painted with a *field*, not with shapes | Sketch 009 needed watercolour in cells that are not round, and both obvious routes failed. An analytic per-pixel watercolour came out harder-edged and darker than `paint/water.go`, whose pool is luminous precisely because it is a stack of forty transparent deposits. Covering a cell with those pools instead — several round touches, as a painter has no crescent-shaped brush — filled the shape but left every pool's boundary legible inside it, which is the *same* failure sketch 008's ground wash already documents: "a pool is a shape, and a shape has a boundary… here there is nothing to have an edge." `paint.FlatWash` is `Ground` evaluated at a point rather than over a canvas, so the caller decides where the paint stops and the fill is exact to any region — a crescent included — while the wash contributes only its character: absorption in linear light, broad pooling, and the paper's tooth. It is a simpler thing than a pool and deliberately so; what it keeps is the part that reads as watercolour at a glance. Two scales are cell-relative rather than page-relative, because a rim or a pooling wavelength that reads on an open sheet swallows a packed one whole. |

| 41 | Sketch 010 shades **per facet**, not per pixel | The whole sketch is one line's difference. The same dome, the same lamp and the same palette shaded continuously give a bed of airbrushed blobs; the shade computed once at each facet's centroid and held constant across it gives cut stone, because a hard step in value at every facet edge is what a broken face *is*. Flat shading is therefore not an optimisation of smooth shading here, it is the picture — and the per-pixel version is kept as the weight-0 `smooth` level precisely so the difference can be looked at. The per-facet tilt and shade jitter that make the faces read as broken have to stay well under the dome's own gradient: at parity they stop describing a surface, the faces stop agreeing about where the light is, and the stone goes flat again. |
| 42 | A low lamp needs a gain, or it costs the picture its top end | Lambert on a bed seen from above gives a *horizontal* face only `lz` of the light, so at a raking elevation nothing anywhere on the sheet is brighter than half and the bed renders as a dark low-contrast slab whatever the stones are painted. The diffuse is therefore scaled so a flat face lands at a fixed 0.70 and faces tilted into the lamp clamp above it. The headroom is load-bearing: at 0.9 every facet on a dome's near-flat top clamps to the same value and each stone comes out with a bald plateau on it. The specular uses the half-vector between the lamp and the viewer rather than the lamp itself — from directly overhead, a face pointing at a low lamp is a face pointing away from the camera, and its gleam would never be seen. |
| 43 | A colour lean must be value-matched, or it darkens twice | Leaning the shadowed side toward a sky colour is what stops fake relief looking like a greyscale multiply. But the point has already been multiplied by the diffuse, so mixing in a colour that is dark in its own right takes the light away a second time — the shadowed half of every stone sank into the joint it was lying in, and the bed lost its boundaries. The sky colour is rescaled to the luminance the point already has, so only its hue is borrowed. The warm lean is left to brighten, since that is what a light arriving *is*, and it is squared so that only faces genuinely square to the lamp take the colour rather than the whole sheet drifting warm. |
| 44 | The darkest thing on the sheet is derived, not chosen | Sketch 010's joint is the water between the stones and it has to stay darker than any stone, because it is what tells the eye where one stone ends. A fixed fraction of the palette's darkest swatch cannot promise that: a bed painted in the palette's own darks, seen at the ambient, goes below any joint mixed from the same handful of colours — caught by a test on the composed colour rather than on the shading, since the ambient floor makes the shading alone impossible to fail. So the joint is taken down until it clears the deepest shadow *this* bed can actually throw. Both colours still come from the palette, which is what keeps the provenance (invariant 3); what is computed is only how far down. |
| 45 | Sketch 010's facet grain is one fineness for the whole bed, where 009's mosaic scales with the cell | Both use the same variable-radius dart throw, and they want opposite things from it. 009's tiles carry a *colour walk*, which needs a comparable number of steps in every cell however big it is, so the dart radius is read from the containing cell's inradius. 010's facets describe a *surface*, and there the same rule actively hurts: facet size becomes a second reading of stone size, the two cues confound each other, and a boulder reads as a chip photographed from closer. It is also untrue of the material — the grain of a stone is a property of the rock it broke off, not of how big the piece is. So the radius is sized off the smallest stone, `--facet-scale` interpolates back to the proportional behaviour as a *power* (the quantity is a ratio of lengths, so the halfway point is the geometric mean), and the facet count is now set by the finest bed rather than the coarsest — which needs both a raised cap and an absolute floor on a facet's radius, because a cap that binds stops the darts before the frame is covered and leaves a few enormous faces in one corner. |
| 46 | Sketch 011 walks the flow field **inside** the pixel function | Sketch 004 has had a flow field since the beginning, but it *walks* it in the plan and stamps dots along the result, so the streamline is an object with a start, an end and a width — and a field of those reads as hair. 011 needs the streamline as a *texture*, so the walk moved into `Raster`'s pure per-pixel function: every pixel integrates its own short path upstream and averages a noise field along it. That is line integral convolution, and it buys resolution independence for free (invariant 2 is a property of the function, not of a stroker) at a cost of roughly fifteen noise evaluations per step. Two things fall out of it that a stamped walk cannot give: the step is `velocity × dt` rather than a fixed arc length, so streaks are long on a fast tongue and short in slack water; and where the flow stalls — in an eddy, against a rock — the walk stalls with it, so the same foam source is counted many times and foam piles up exactly where foam piles up. |
| 47 | 011's foam thresholds are relative to the reach's own Froude number | Water breaks at high `F = speed/√depth`, and that one expression is what makes a riffle break on its crest and a pool stay smooth without a threshold on depth or on speed alone. But depth and speed here are in *arbitrary units* — depth is measured in extinction lengths, speed in canvas units per notional second — so an absolute threshold on `F` is meaningless across the reach axis: the value that left a pool clean turned a riffle entirely white, and the one that suited the riffle left a rapid untouched. Measured against the reach's own nominal `F`, "breaking" means locally much faster or much shallower than the water around it, which is what the word means. The `foam` level then says only how much of that a stretch of river tolerates. |
| 48 | A cell-shaped texture is the most dangerous thing this sketch can draw | Twice, and from opposite directions: the gravel is a Worley diagram with its walls drawn and its interiors shaded, and the caustic net is a Worley `f2−f1` field. Individually both are convincing. Laid over a whole frame at one contrast and one scale, each turns the picture into tooled leather — a reticulation that belongs to nothing in the composition and buries every other cue. The gravel's fix is to weight the cell-shaped terms down and the broad mottle up, and to mix the bed toward its own mean as depth rises; the caustic's is that the threshold on `f2−f1` is a *line width*, and a caustic is a filament: bright, narrow and mostly absent, gated off entirely below the shallows. Worth recording because the symptom was blamed on the flow field, then the convolution, then the refraction, before a two-by-two `sweep --vary` found it in one render. Sweeping *isolations* is as useful as sweeping seeds. |
| 49 | Chop and current are two fields, not two readings of one | The surface slope was first taken as the difference between the convolution walk's first two samples — free, since both had been fetched anyway. It is also wrong: those samples are a streak wavelength apart, so the ripples came out at the streak's scale and were laid in rows by the convolution, which reads as basketry. Ripples and streaks are two scales of one surface and they need two fields; the chop is its own fine noise differenced along the flow at the pixel (two extra samples per pixel, not two per step) and gated by the Froude number, so a glide is glass and a riffle is broken. The related saving: a glint must be an angular *window* rather than a power of a cosine, because seen from straight above with a high sun a flat surface is already near the specular peak and the exponent lights the whole river. |
| 50 | A reusable translucent raster is averaged premultiplied, then stored straight-alpha | Straight RGB averaging lets a fully transparent sample contribute hidden colour, which becomes a dark or coloured fringe when the layer is later composited. `RasterLayerSS` accumulates linear-light RGB multiplied by alpha, averages alpha separately, and unassociates only when writing `image.NRGBA`. The layer remains conventional straight-alpha PNG data while AA behaves as compositing maths requires. Sketch 011's overlay uses the same surface field as the river, but moves banks off-frame and omits the bed, caustics, foam and physical rock response instead of trying to erase those cues from the finished colour. |
| 51 | Sketch 012 combines point-sampled materials before rasterisation | A finished water PNG placed over a finished stone PNG can change opacity and colour, but it cannot move the bed lookup through a sloping surface or let a ripple's dark and light faces modulate the stone material. Scree therefore exposes the exact pure point sampler used by its own `Render`, and riffle exposes an all-water surface sample containing flow, slope, ripple and dapple. `shallows` plans both, then calls them inside one `Raster` pixel function: refract the bed coordinate, tint its colour, shadow the trough and light the crest. Existing scree goldens prove extracting `Bed.At` did not change 010. Refraction stays subordinate because large offsets fold hard stone joints into moire; paired value changes are the primary water cue. |
| 53 | Sketch 015's water is a coloured *medium*, and its extinction is normalised off the pigment's hue | Two failures, one cause. Lerping the bed toward blue at the veil's alpha reads as a sheet of acetate laid on a photograph — the bed's own colour is averaged away, so a thin passage and a thick one differ only in how grey they are. Absorption in linear light instead takes the stone's colour *through* the water, which is why a shallow passage keeps its facets and a deep one loses the red end first. But absorption taken straight from the palette swatch makes the water as strong as the palette happens to be *dark*: hokusai's own navy filters hard while kandinsky's pale sky blue does nothing at all, and 012's channel-based `coolest` made it worse by picking the great wave's *cream*. So the pigment's brightest channel is normalised to pass freely, only the ratios between channels carry hue, and how much light the water takes is a separate neutral term. Colour and depth become independent, which is the way round a painter would expect. |
| 54 | 015's marbling reads its displacement from a *finer* field than the one that shapes its load | The obvious economy is to reuse the warp vectors already computed for the veil — they are right there and they are a displacement. They are also broad by construction, and a displacement that varies only over the whole canvas translates the bed rather than stretching it: at four times the drift the picture was the same picture, moved a little. Smearing a stone needs the offset to change appreciably *across one stone*, so the drift has its own frequency multiplier (`--drift-scale`, 8× for marble against 3× elsewhere). The same lesson in reverse is why the veil's own structure had to be made much broader than 013's defaults: at 013's scale and warp strengths nearly all the field's energy sits at a fraction of a stone and the water comes out as an even tint with a swirl inside every pebble. |
| 52 | The repository has a shared lifecycle, not a universal pipeline interface | The actual cross-section has simple field samplers, structural models, sequential painters and composed materials. Their useful intermediates have different operations. `Sketch.Render` remains the common outer boundary; private concrete plans name build-once work, and only models with real pre-raster consumers are exposed as typed domain components. This keeps hot paths allocation-free and preserves explicit Go composition without `[]Stage`, generic field hierarchies or `Process(any) any`. Review and alternatives: `architecture-review.md` and `pipeline-design.md`. |
| 55 | Coordinator and worktrees share one container; skills stay on `master` | A bare clone adds fetch/push machinery without a benefit: the coordinator is a privileged `master` checkout, so the container is `go-graphics/master` (ordinary clone) plus `go-graphics/worktrees/<name>`. Project skills are edited only on `master` and applied by opening Cursor there; workers change cwd, they do not get a second skills tree. |
| 56 | Fractal flames are a chaos-game histogram in `internal/flame`, not a point sampler | A flame is a measure estimated by iterating maps. There is no closed form to put behind `At(u,v)`, and stretching `sketch.Raster` around it would be a different algorithm. The published mechanism — variations, four-channel accumulation, log-density, vibrancy — is independently meaningful and the performance story (fixed 8 orbits, atomic uint64 bins, samples-per-pixel, bilinear splat into the histogram) belongs there. Spatial AA is histogram oversample plus Gaussian downsample (decision 59), not a larger output buffer. The sketch owns genomes, the colour ramp, auto-framing taste and the void/dusk/paper ground. Quality is samples per pixel so preview and print share a camera; `--aa` multiplies that budget rather than allocating an `aa²` histogram, which at print would be several gigabytes. Worker count is a constant, not `GOMAXPROCS`, because a machine-dependent partition would make the Monte Carlo realisation non-deterministic. |
| 57 | Flame colour is an RGB ramp, and the first-look palette is sampled from the Zander render | ColorLisa 5-swatches interpolated in HSL cannot make a cyan nest against a gold mass. The short path from Klee's violet to its fire-orange is magenta; the short path from cyan to gold is green. Flam3 palettes are 256-entry RGB LUTs. `zander-spindle` is five stops sampled from `docs/reference/apophysis-flame.jpg` (cyan, pale cyan, gold, amber, burnt orange), listed cool→warm, with the same non-ColorLisa provenance as `staticart-seven`. White-hot cores are gleam on log-density: putting white in the ramp paints the filaments silver. Split still works on ColorLisa palettes (sort by warmth, RGB-lerp). Heart as the primary spindle map was the matching mistake on the structure side — it fills a teardrop of fuzzy hair; JulianN plus contractive linear copies is what reprints a branching nest at smaller scales. |
| 58 | Flame xaos is a per-row CDF, and density estimation is a log-then-Gaussian scatter | Independent weighted picks cannot nest "this map only after that one". flam3's xaos is P(j\|i) ∝ weight[j]·chaos[i][j]; a 16384-wide LUT is overkill for a handful of xforms, so each predecessor gets a short CDF and the first pick (no predecessor) uses the raw weights. Spindle punches julian→julian to 0.05 so copies reprint the nest; symmetry rows stay 1. Density estimation is the other half of Apophysis quality: radius = estimator / n^curve after log-density, before gamma (Suykens & Willems; flam3 defaults 9 / 0 / 0.4). Blurring the linear histogram then logging is a different picture. The scatter is a sequential float64 post-pass so GOMAXPROCS cannot change it; `--estimator 0` keeps the old per-pixel develop path. |
| 59 | Flame spatial AA is histogram oversample + Gaussian downsample, not `--aa` | Thin filaments alias when the chaos game splats into an output-resolution histogram. flam3 accumulates at `spatial_oversample` and reduces with a separable Gaussian (`spatial_filter_radius`, support 1.5). `--oversample` (default 2, cap 3) raises hist resolution; `--filter` (default 0.5) is the radius in output pixels; estimator radii scale by ss so DE stays consistent across oversample levels. `--aa` remains a sample-budget multiplier only — sizing the hist by aa at print is the gigabyte trap already refused in decision 56. Downsample averages in linear light and is sequential for determinism. |
| 60 | Flame long-form curation is trait-space flock breeding, not genome GA | QQL’s lesson is an orthogonal weighted output space plus a curator; Electric Sheep’s is like→reproduce with local exploration. Combining them here means `staticart flock`: sample Traited seeds into a sheet + `flock.jsonl`, then `--likes` boosts schema weights (`trait.Schema.Boost`) and half the next generation keeps a parent’s trait pins on a neighboring seed. Continuous flam3 crossover would fight decisions 56–58 (policy stays in genomes; no XML DNA). Colour is a `cast` trait (pools’ colourway pattern) so likes can steer the ramp without a cartesian `--vary palette`. |
| 61 | Flame wash is a second Develop on the same Measure, not a new IFS | The attractor is already a pigment-load field once log-density is stripped of gleam and void composite. `Hist.Measure` exposes that field; ember `Develop` and wash `developWash` share it so genome, xaos, camera and histogram stay one path. Wash character (`FlatWash` mottling/tooth, forced paper ground) is sketch policy — no `paint.Wash` stamps along orbit points, no blur-of-PNG, no plugin HistogramRenderer. `--medium wash` is weight 0 so ember seeds stay byte-stable. |
| 62 | Flame structure space needs a chaos genome, not only named recipes | Electric Sheep variety is random IFS DNA — many variations, map counts, finals, symmetries, xaos — not jitter around six fixed templates. Named families (spindle, filament, …) stay as recognisable characters with pooled primaries; `chaos` draws 2–8 maps from the full catalog and carries the highest weight so flocks explore widely. Spindle remains the Zander pin. |

### Proposed public-application decisions (2026-09-09)

These records capture the recommended design for later implementation. They
are **proposed**, not claims that the code or deployment already follows them.
Their identifiers continue above the highest historical number without
renumbering earlier duplicate entries.

| # | Proposal | Rationale |
|---|---|---|
| 63 | [Curated publication beside the local laboratory](adr/0001-curated-publication.md) | One Go module with separate CLI/public adapters; publish explicit editions, not every registered sketch. Preserve decision 52's artwork lifecycles. |
| 64 | [Recipes and edition-scoped reproduction](adr/0002-published-recipe-identity.md) | A seed or cache key alone cannot preserve a visitor's result across algorithm changes; file sharing is the baseline and durable links require a retention policy. |
| 65 | [Isolated public rendering on one VPS](adr/0003-isolated-public-rendering.md) | Anonymous expensive computation needs hard deadlines and memory isolation; private renderer supervision and bounded ephemeral state avoid a database/queue platform in v1. Public release requires CI, superseding decision 3 when implemented. |

## 9. Roadmap

1. **Sketch 001 "contour"** — shuffled-gradient contour noise (spec:
   [sketches/001-contour-noise.md](sketches/001-contour-noise.md)).
2. Palette library: full ColorLisa dataset + manipulation ops.
3. More noise-family sketches (turbulence, domain warping).
4. First vector sketch → introduce `tdewolff/canvas`.
5. **Give the other sketches an output space.** Done for 008 (`fill`); the
   sampling vocabulary is now `internal/rnd`. Tapestry is the open case and
   the hard one: its interesting choices are toggles and continuous ranges
   rather than orthogonal discrete axes, so a schema would have to be
   *designed* for it (a "terrace character" axis, say) rather than
   transcribed from the knobs it already has. Do it when a tapestry sweep
   is wanted, not before.
6. **Exploration tooling for curation.** Done: `staticart sweep` for grids;
   `staticart flock` for Traited like→breed (decision 60).
7. Possible later: more IFS looks on the same `internal/flame` mechanism,
   tilings, SVG export, gallery index generator. Sketch 016 is the first
   fractal flame.

### 61. Published recipes and bounded exploration (2026-09-09)

The public application uses explicit factory definitions in `internal/artwork`,
immutable canonical edition recipes, and concrete pointer-valued configuration
owned by pools, foam and iris. CLI and typed adapters share the existing opt
and trait validators; omitted overrides retain their meaning. Numeric defaults
and named artistic streams are fixed by an edition's retained renderer release.
The serialized recipe records resolved traits, effective palette and override
presence; no public preset name is needed to execute it. `internal/publish`
restricts public recipes further, forbidding numeric overrides and experimental
materials. `internal/explore` owns pure local/public candidate planning.
No third-party Go dependency is introduced. Published edition 1 is provisional
until Linux compatibility, provenance and owner review pass.

### 62. Patch the published runtime toolchain (2026-09-09)

The HTTP/template path makes six existing Go 1.26.5 standard-library advisories
reachable according to govulncheck. Pin Go 1.26.8, the current supported patch
in the existing minor line, for the module and release CI. Do not suppress
vulnerability results to preserve the workstation's older toolchain. Keep
artistic edition compatibility guarded by fixed-pixel and existing golden tests.
Source: https://go.dev/dl/ and the Go vulnerability database, including
https://pkg.go.dev/vuln/GO-2026-6091 .

### 63. One image guides public similarity (2026-09-10)

The owner simplified the public journey to direction choice, four images, and
one image with download/share/similarity actions. The chosen sample supplies
its complete permitted traits and palette to all four fresh composition seeds.
Favouriting is independent; there is no multi-parent selection or unrelated
base candidate in this action. This makes “Generate similar ones” predictable
while preserving the CLI flock policy and shared planner's existing defaults.
Samples retain their original direction labels. Re-entering an art form reuses
bounded navigation and preserves favourites; initial admission is atomic and
idempotent, and failures leave prior directions unchanged. The interface uses
server-side favourites without browser recovery controls. Full rationale:
[web/ux-simplification.md](web/ux-simplification.md).

### 64. Docker Compose owns the public runtime (2026-09-10)

The owner chose three containers on one VPS for portable image releases and
container-first infrastructure learning. [ADR 0004](adr/0004-compose-runtime.md)
supersedes ADR 0003's host application units while preserving one queue, private
renderer supervision and separate resource limits. Terraform/cloud-init still
own the host; Compose owns processes/networks/volumes. Image archives and release
scripts retain smoke, drain and rollback; host/TLS/recovery proof remains pending.

### 65. Match the deployment target to CAX11 ARM64 (2026-09-10)

The owner selected Hetzner CAX11 in nbg1 and the existing MacBook SSH public
key. CAX is ARM64, so retained release builds, the Ubuntu Docker repository
and bootstrap architecture check, and the CI runner all target ARM64 together.
The Dockerfile already supports target-platform Go compilation and its pinned
base images support ARM64. Local Compose remains native-platform development;
it does not force an architecture or change the three-service boundary.
Terraform reads one expanded public-key path for both its SSH-key resource and
cloud-init; the sensitive API-token variable comes from the operator environment.
No host creation or existing-resource ownership change is authorized by this
configuration update. Previous amd64 verification remains historical evidence.

Sources: [Hetzner CAX architecture](https://www.hetzner.com/pressroom/arm64-cloud/),
[GitHub ARM64 runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
