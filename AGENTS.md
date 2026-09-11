# AGENTS.md — agent guide for go-graphics

## Branch and worktree rules

- Keep the coordinator in `master/` on `master`. The container above it
  (`go-graphics/master` plus `go-graphics/worktrees`) is not a repository.
- Give every writing worker one dedicated branch and sibling worktree under
  `../worktrees/<name>`; never share a writing checkout.
- Create, inspect, integrate, and remove worktrees with ordinary Git commands
  from `docs/WORKTREE-WORKFLOW.md`.
- Project skills live on `master` (`.claude/skills/`, optionally
  `.cursor/skills/`). Open Cursor on `master/` so every worker sees them.
  Do not copy skills into experiment worktrees.
- Workers operate only in their assigned worktree, do not switch its branch,
  and do not merge or remove worktrees.
- Keep generated renders under `out/` in the assigned worktree. Commit only a
  deliberately selected review sheet when useful.
- Commit coherent changes before review. For artwork changes, compare fixed
  seeds and visually inspect the output; passing tests is not artistic approval.
- Integrate or discard work only after explicit user approval.
- Do not use hard resets, cleans, force deletion, or forced worktree removal as
  routine workflow commands. Inspect and resolve the condition Git reports.

Generative static 2D art in Go. Deterministic sketches → PNG/JPEG at preview,
web, and print resolution. Stdlib raster now; `tdewolff/canvas` later for
vector work. Zero third-party dependencies at the moment — keep it that way
unless a documented decision says otherwise. The public studio and its
deployment/tooling are documented in `docs/README.md`.

## Commands

```sh
make check      # fmt + vet + lint + test — MUST pass before any commit
make test       # go test ./...
make lint       # golangci-lint (v2.12 local; config .golangci.yml)
make preview    # render sketch "contour" at preview size into out/
make preview-qql # render sketch "qql" at its native 4:5 preview size
make preview-pools # render sketch "pools" (watercolour circles)
make preview-foam # render sketch "foam" (inked cells with pluggable fills)
make preview-scree # render sketch "scree" (a river bed of faceted, lit stones)
make preview-riffle # render sketch "riffle" (a small river seen from above)
make preview-iris # render sketch "iris" (an abstract iris)
make preview-glaze # render sketch "glaze" (a stone bed under a painted warped veil)
make preview-flame # first-look flame: 1000² at quality 80 (600 is too small)
make hatchbook  # render the internal/hatch specimen sheets + manifest
go run ./cmd/staticart render <sketch> --profile preview|web|print --seed N --palette <name> --out out
go run ./cmd/staticart list
go run ./cmd/staticart traits <sketch> --seed N  # the output-space point a seed lands on
go run ./cmd/staticart sweep <sketch> --seeds 1-12 [--vary flag=a,b]  # batch + contact sheet
go run ./cmd/staticart flock <sketch> --count 48 [--from dir --likes a,b]  # Traited sample / breed
```

Sketches: contour, tapestry, circles, drift, rounds, shoal, qql, pools, foam,
scree, riffle, shallows, warp, iris, glaze, flame,
hatchbook (a specimen sheet for `internal/hatch`, not an artwork — `make hatchbook`).

`flame` is a chaos-game histogram, not a point sampler: it iterates a
small IFS and tone-maps the log-density of the orbit. `--quality` is
samples per output pixel (preview and print share a camera); `--aa`
multiplies that budget; `--oversample` (default 2) is the histogram
resolution used for spatial AA; `--estimator` (default 9) is flam3
density estimation after the log. Colour is a `cast` trait inside the
output space. `--medium wash` develops the same measure as pigment on
paper (`--manner stain` via FlatWash); weight 0 so ember stays the
default. Curate with `staticart flock` (like→boost traits +
neighbor seeds), not genome crossover — decision 60. **Never judge a
flame at 600px.** First-look review renders are 1000×1000 at
`--quality 80` (`make preview-flame`). Filaments and grain do not read
smaller than that. Use `--profile web` / `print` only when checking a
candidate at display or paper size. The first look is a spindle of
luminous filaments on a void, after the Apophysis render at
`docs/reference/apophysis-flame.jpg`, with cast/palette `zander-spindle`
(cyan nest, gold mass — not a ColorLisa extract). See
`docs/sketches/016-flame.md`.

`foam` has a watercolour layer: `--fills watercolour` paints every cell,
`--water` says what the paint did (blooms, bleeding, glazing, …) and
`--scheme` how colour is spread over the sheet — see
`docs/sketches/009-foam.md`. The `watercolour` fill level carries weight 0,
so a painted sheet is always a deliberate act.

`scree` is a river bed of worn stones seen from above, each stone cut into
flat Voronoi facets and each facet given *one* shade from its own normal
against a single lamp — so the third dimension comes out of the same
wall-distance field the walls do. `--bed` is how coarse the bed is, `--stones`
how worn, `--facets` how finely they are cut, `--light` the weather and
`--wet` how much water is standing over it; see `docs/sketches/010-scree.md`.
Two things there are easy to undo by accident: the shade is flat *per facet*
(computed at its centroid, not per pixel — that one line is the whole sketch,
and `--facets smooth` is the control that shows it), and the facet grain is
one fineness for the whole bed rather than proportional to each stone
(`--facet-scale` restores the proportional behaviour, which is what 009's
mosaic wants and this does not).

`riffle` is a river seen from directly above — one pure per-pixel function
over a depth field, a velocity field and an upstream walk. `--reach` is the
energy of the stretch, `--channel` its plan form, `--water` its turbidity;
`--medium overlay` reduces it to a translucent all-over water layer with no
bed or banks. See `docs/sketches/011-riffle.md`, and its implemented
algorithm before changing textures.

`shallows` combines 010's planned faceted stone bed with 011's point-sampled
surface in one raster function. Ripple shadows, highlights and refraction act
on the bed before the final pixel is written; it is not a composite of two
rendered images. See `docs/sketches/012-shallows.md`.

`warp` is a point-sampled nested fBM field with plain, single-warp and
nested-warp comparison modes. Uniform detail is the default; `--detail varied`
uses a broad activity field to separate quiet passages from concentrated folds.
See `docs/sketches/013-warp.md`.

`iris` aims 013's warp vocabulary at a subject. Its one mechanism is a radial
remap of the plane — `P = direction * (ringBase + stretch*s)` — which makes an
ordinary 2D Perlin field run as fibres from pupil to limbus *and* be seamless
around the disc by construction; warping then happens in the local
tangential/radial frame, so a strong warp bends fibres instead of dissolving
them. There is no lighting model at all: value is the field. Eight trait
dimensions (`structure`, `weave`, `grain`, `reach`, `aperture`, `tint`,
`rim`, `ground`) carry the output space — see `docs/sketches/014-iris.md`, and read
the implemented algorithm there before touching any coordinate.

`glaze` lays a *painted* water over the same faceted stone bed: a translucent
veil whose entire structure is a nested fBM domain warp rather than a physical
surface. `--veil glaze|marble|terrace|filament|silk` is the material (one
knob, five materials — every other water flag is an override on it), `--cast`
which pigment the palette supplies. The water is absorption in linear light
with the extinction normalised off the pigment's *hue*, which is what lets a
pale palette swatch still read as blue. How much of the sheet is wet at all is
`--coverage`, a spatial envelope rather than a global dial, so one frame can
hold broad dry stone against flooded passages; `--opacity` and `--body` say
how dense the water is where it covers, and `--density` and `--gather` how
many threads the filament draws and how tightly they pack. See
`docs/sketches/015-glaze.md`, and
its implemented algorithm before retuning any field constant — the veil
wants forms much broader than 013's defaults, the drift needs its own finer
frequency or it translates the bed instead of smearing it, and neither
coverage nor density is the global multiplier it first looks like.

`qql` is 4:5 — render it with `--profile preview-tall|web-tall|print-tall`.
It also has `--medium wash` (watercolour instead of ink); it needs room, so
pair it with `--ring-size large` and/or `--spacing sparse`.

Rendered files land in `out/` (gitignored), named
`<sketch>_<palette>_<seed>_<WxH>.<ext>`.

## Verify visually — always

Tests prove determinism, not beauty. After changing any sketch or color code:
render a preview (`make preview` or the render command) and **Read the output
PNG** to look at it. For sketch 001 compare against the target image at
`docs/reference/target-sketch7.jpg` using its current sketch reference.

## Judge the space, not the render

One seed says almost nothing about what a change did. Use `sweep`: it
renders a seed range or a parameter grid in parallel and writes one
`sheet.png` you can look at in a single Read, plus a `manifest.txt` naming
each tile.

```sh
staticart sweep pools --seeds 1-12 --vary fill=busy,packed --profile web
staticart sweep qql --seeds 1-16 --profile preview-tall --out out/qql-sweep
```

`sweep` passes every flag it does not own straight through to `render`, so
any sketch knob is sweepable. Calibrating a constant is the same move:
`--vary band-width=0.008,0.022,0.03` and look at the sheet. Prefer this to
rendering one image and squinting — it is faster, and it is the only way to
see whether a change helped the *typical* seed or just the one in front of
you.

## Docs map (read before working on the related area)

- `docs/WORKTREE-WORKFLOW.md` — container layout (`master/` + `worktrees/`),
  native Git commands, skills, review, integration, and cleanup.
- `docs/ARCHITECTURE.md` — C4 architecture index and core invariants;
  follow its links to components, deployment, packages and testing. Update the
  relevant current-state view when responsibilities change.
- `docs/sketches/NNN-<name>.md` — implemented algorithms, tunables,
  rendering behavior and generated CLI help for every sketch. Write the spec
  before implementing a new sketch.
- `docs/reference/colorlisa-palettes.md` — full ColorLisa palette dataset
  (source data for `internal/palette`; don't re-fetch the website).
- `docs/reference/qql-colordata.json` — QQL's 153 HSB swatches and 7 city
  palettes (source data for `internal/sketch/qql/palettes.go`, generated by
  `go run ./tools/genqqlpalettes`; don't hand-edit either file).
- `docs/hatching.md` — the hatch package: structures, parameter vocabulary,
  API, and its known weaknesses. Read before filling anything with marks.
- `docs/README.md` — full current documentation, including studio, API,
  deployment and operator procedures.

## Invariants (breaking these is a bug even if output looks fine)

1. **Determinism**: all randomness from `Context.Seed` via `math/rand/v2`
   PCG or seed-derived tables. Never `time.Now()`, never global rand.
   Same (sketch, params, seed, size) → byte-identical image.
2. **Resolution independence**: sample in normalized coords
   (`v ∈ [0,1]`, `u ∈ [0, aspect]`); frequencies are cycles-per-canvas-unit,
   never per-pixel. Preview and print of the same seed must match.
3. **Color**: `float64` sRGB in `[0,1]` everywhere internally; clamp at 8-bit
   conversion. Palettes originate from the ColorLisa data and keep
   artist/artwork provenance.
4. **Dependency direction**: `cmd → sketch → {gradient, noise, render} →
   palette → mathx → stdlib`. `mathx` and `noise` import stdlib only. No
   art logic and no sketch-specific code in `cmd` (sketch options live in
   the sketch via `sketch.Configurable`).

## Layout

```
cmd/staticart/          CLI (generic wiring only — no sketch specifics)
internal/mathx/         Clamp01 / Remap / Rescale / Smoothstep (leaf)
internal/rnd/           sampling vocabulary: weighted choice, gaussians,
                        winnow, weighted bag (leaf)
internal/opt/           declarative CLI knobs → flags, ranges, name suffix (leaf)
internal/geom/          circles + spatial collision/containment index (leaf)
internal/cells/         weighted partition of the canvas into addressable,
                        fillable cells: a foam (leaf)
internal/hatch/         filling a region with repeated marks: structures,
                        parameters and coverage functions (mathx + noise)
internal/flame/         fractal-flame IFS: variations, xaos, chaos game,
                        log-density, density estimation, oversample
internal/scheme/        colour arrangement over a set of regions: 16 strategies,
                        each answering hue *and* value (leaf)
internal/trait/         weighted output-space dimensions, seed → traits, CLI overrides (leaf)
internal/palette/       Color type + ops, HSB Swatch (clamp box + walk), ColorLisa data
internal/gradient/      cosine / HSL / discrete / terraced gradients
internal/noise/         Perlin + fBm, Worley cell noise, Hash01 (leaf)
internal/render/        pixel loop (AA, dither, 16-bit), profiles, encode+metadata
internal/paint/         stamp-based brush canvas, bristle brush, washes, analytic
                        rings; FlatWash fills any region as a field (flatwash.go)
internal/sketch/        Sketch + Configurable + Traited interfaces, Context, registry
internal/sketch/sketchtest/  shared test helpers (goldens, determinism)
internal/sketch/<x>/    one package per sketch + its testdata/ goldens
```

## How to add a sketch

1. Write `docs/sketches/NNN-<name>.md` (algorithm, tunables, acceptance
   checklist).
2. Create `internal/sketch/<name>/` implementing the `sketch.Sketch`
   interface; tunables are struct fields with defaults in `New()`.
3. Register it in the registry wiring used by `cmd`. Sketch-specific CLI
   options: implement `sketch.Configurable` and declare the knobs with
   `internal/opt` (see pools/options.go) — never add sketch flags to `cmd`,
   and never hand-roll range checks. If the interesting choices are
   discrete and orthogonal, declare a `trait.Schema` and implement
   `sketch.Traited` too (see qql, and pools' `fill`) — you get seed
   derivation, per-dimension flags, filename/metadata plumbing and the
   `traits` command for free.

   **Knobs that only make sense together are one trait, not several
   flags.** If you find yourself setting five values in lockstep to get a
   named look, that look is the knob; give it a trait dimension resolving
   to *ranges*, and keep the five as overrides.
4. Tests via `sketchtest`: determinism, plan-bounds over many seeds, and a
   golden PNG in `testdata/` (regenerate with `make golden`, eyeball
   before committing).
5. Render previews, check against the spec's acceptance list, iterate.

## Artwork lifecycle

Use the smallest internal shape that makes the algorithm clear. These are
patterns, not interfaces every sketch implements:

```text
simple sampler
  defaults/overrides -> immutable fields and colour mapping -> At -> Raster

structural sampler
  traits/overrides -> concrete levels -> geometry/material plan -> At -> Raster

painted artwork
  traits/overrides -> planned marks -> sequential paint.Canvas -> Image

composed material
  typed planned components -> combine samples before rasterisation -> Raster

chaos-game histogram
  traits/overrides -> IFS + camera -> iterate into a density histogram -> Image
```

`contour` is the simple reference: its private plan builds gradients and noise
once, then `plan.At` is pure. `qql` is the planned painter: its private plan is
fully specified dots, interpreted later through a paint stream. `foam` is the
structural reference: plan the partition and appearance once, then sample in
the documented fill/mosaic/hatch/relief/ink order. `shallows` is the composed
reference: it combines `scree.Bed` and `riffle.Surface` before pixels are
written, with no intermediate image. `flame` is the chaos-game reference: the
attractor is a measure, not a function of a pixel.

Not every sketch needs a plan. Keep one private unless another real consumer
needs its pre-raster result. Do not add universal `Plan`, `Scene`, `Stage`,
`ScalarField`, `VectorField` or `ColorSampler` interfaces just to label the
lifecycle.

### Mechanism vs policy

Shared packages own mechanisms: spatial queries, partitions, noise, clipping,
hatch coverage, colour interpolation, region schemes, lighting arithmetic,
rasterization, brushes and washes. Sketch packages own artistic policy:
parameter ranges, weights, ordering, composition, palette roles and aesthetic
exceptions.

Extract only when there are two real consumers, an existing composition
crosses artwork boundaries, or the responsibility is independently meaningful
and testable. A reusable component may remain domain-specific: an all-water
surface is better than a generic vector-field adapter if consumers need
surface slope, ripple and dapple together. Never create vague `utils`,
`helpers`, `common`, `pipeline`, `stages` or `models` packages.

Concrete sketch-to-sketch imports are not automatically wrong. `shallows`
legitimately inherits the complete scree bed vocabulary. If an import exists
only because a mechanism is trapped behind an artwork wrapper, extract the
narrow component instead of banning the dependency.

### Configuration and RNG

Sketch fields plus `New()` defaults are fine for a standalone artwork. Prefer
an explicit concrete config when another artwork constructs the component or
when a planned value would otherwise retain mutable CLI state. Keep the
existing CLI interfaces as adapters; never use a generic parameter map.

Every random consumer owns a named stream. Resolve traits and numeric ranges
before geometry. Pass an RNG or derived seed explicitly into planning; do not
silently create shared generators. The repeated pixel path has no RNG draws:
use immutable seed-keyed noise and hashes there. Optional effects get dedicated
streams and must not consume the base composition's stream. Preserve existing
numeric stream IDs unless a seed change is intentional and documented.

### Hot path and tests

A point sampler must avoid mutation, option resolution, geometry construction
and ordinary allocations. Build partitions, indices, schemes and constant
per-region/per-facet appearance once. Do not pre-render a full frame when a
compact model can answer coordinates.

Test the earliest meaningful boundary:

- resolution and override isolation;
- geometry bounds, coverage and adjacency;
- deterministic material assignment and shading ranges;
- exact repeated samples and equal-aspect plans at different pixel sizes;
- paint interpretation independently of mark planning;
- final determinism and goldens;
- preview and sweep inspection.

Benchmark planning separately from sampling or painting for expensive work.
Direct `At` methods should remain allocation-free. See `docs/performance.md`
for measurement commands and the print-memory cost of
`paint.Canvas`.

## Engineering standards

- TDD for math-heavy code (color, gradient, noise, mapping): table-driven
  tests first. Don't overengineer elsewhere — no abstractions for
  single-implementation concepts. Extract on the *second* copy, and only
  where the copies are actually the same thing.
- Name a test after the claim it defends, not the function it calls
  (`TestWashesMixWhereTheyCross`), and say in its comment what breaks if it
  fails. These tests are the spec; goldens only catch that *something*
  moved.
- Pixel-exact reproducibility across versions is **not** a goal. If a
  cleaner design changes what a seed draws, take the cleaner design,
  regenerate the goldens and eyeball them.
- `gofumpt` + `goimports` formatting (local prefix
  `github.com/jaminalder/go-graphics`) — `make fmt`.
- Exported identifiers get doc comments; sketch packages start with a short
  package comment describing the artwork.
- Commit `testdata/` goldens; never commit `out/`.
