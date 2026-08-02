# Architecture

Design document for `go-graphics` — a Go project for generative static 2D art.
Read this before changing package structure or adding cross-cutting features.
For day-to-day agent workflow (commands, conventions, how to add things), see
[../CLAUDE.md](../CLAUDE.md).

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
| `Swatch` | A colour with room to move: an HSB base plus a per-channel spread and a clamp box it may never leave. Drawing from one repeatedly gives a family; stepping from the previous draw walks that family. | `internal/palette` |
| Hatch | A region filled with repeated marks: a coverage function of a point *and* the region containing it. The arranging rule (parallel, contour, radial, flow, …) is a parameter, not a type; colour is the caller's. | `internal/hatch` |
| Partition / foam | The canvas divided into curved-walled cells, each addressable: which cell a point is in, its distance to the nearest wall, how crowded it is with further cells. Per cell: area, centroid, inscribed radius. A distance *field* (Worley) cannot be filled; a partition can. | `internal/cells` |
| Colour scheme | An arrangement of colour over many discrete regions: which region gets which colour, and how dark. Fifteen strategies, each answering hue *and* value. | `internal/scheme` |
| Trait / output space | A sketch's space of outcomes as orthogonal, weighted, discrete dimensions derived from the seed and overridable per render. The idea behind QQL; the machinery is sketch-agnostic. | `internal/trait` |
| Render | Pixel-loop execution (parallel), size profiles, PNG/JPEG encoding. | `internal/render` |

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
    shoal/, qql/, pools/, foam/             the sketches     → all of the above
    hatchbook/            specimen sheet for hatch (a
                          catalogue, not an artwork)         → hatch, palette
docs/                     this file, sketch specs, idea backlog, reference data
tools/                    code generators (palette data)
out/                      rendered images (gitignored)
```

Dependency direction (arrows = "may import"):

```
cmd → sketch (registry) → {gradient, noise, render, trait} → palette → mathx → stdlib
```

Rules:

- `mathx`, `geom`, `noise`, `trait`, `rnd`, `opt` and `cells` are leaf
  packages: stdlib imports only. `hatch` sits in the same tier but imports
  `mathx` and `noise` (see decision 38).
- Sketches live in subpackages of `internal/sketch`; `cmd` discovers them
  only through the registry. Sketch-specific CLI options are owned by the
  sketch via the `Configurable` interface — `cmd` stays sketch-agnostic.
- Nothing imports `cmd`. No package keeps global mutable state.
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

## 5. Rendering pipeline

```
Sketch.Render(ctx)
  └─ builds its plan/scene from ctx.Palette + ctx.RNG (seed)
  └─ sketch.Raster(ctx, func(u, v float64) palette.Color)
       └─ render.RasterSS / RasterDeep: parallel rows, ctx.AA supersamples
          per pixel averaged in linear light, dithered 8-bit (or 16-bit
          with ctx.Deep)
cmd/staticart
  └─ render.WritePNGMeta / WriteJPEGMeta: sRGB tag, 300 DPI, full render
     recipe + code revision embedded (render.Meta)
  └─ filename: <sketch><option-suffix>_<palette>_<seed>_<WxH>.<ext>
```

Per-pixel work must be pure (no shared mutable state) so the row-parallel loop
is race-free by construction.

**Second rendering model:** `paint.Canvas` (used by stroke-based sketches
like drift) is stamp-based and sequential — soft dabs blended source-over,
anti-aliasing from the dab edges (Context.AA unused), same dithered
quantization on output. Compositions stay resolution-independent; stroke
texture varies subtly with resolution, which is part of the medium.

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

- Sketch-specific tunables are fields on the sketch struct with defaults in a
  `New()` constructor — not a generic params map. Add CLI flags per sketch
  only when actually needed for exploration.
- Sketches whose interesting choices are *discrete and orthogonal* should
  declare a `trait.Schema` instead of hand-rolling flags: it gives seed
  derivation, per-dimension overrides, filename and metadata plumbing, and
  the `traits` command for free. `qql` is the first user; see
  [sketches/007-qql.md](sketches/007-qql.md).
- Each sketch subpackage registers itself via `sketch.Register(New())` from
  the registry wiring in `cmd` (explicit imports, no `init()` magic beyond a
  single registration call).

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
6. **Exploration tooling for curation.** Done: `staticart sweep`. What it
   still lacks is a way to record a verdict — a sweep produces twenty
   images and the judgement about them lives only in the conversation.
7. Possible later: fractals, tilings, SVG export, gallery index generator.
