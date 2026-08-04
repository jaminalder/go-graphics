# Architecture review

Review of the repository at `bdcae5c3020ecba495720e59109c67013306dd52`
on 2026-08-04. This document describes the implementation as it existed at
that revision. It is evidence for, not a replacement for, the target design in
`pipeline-design.md`.

## Baseline

The worktree was clean on `master` before the review. The baseline commands
were:

```text
git status --short --branch
git rev-parse HEAD
git log --oneline -10
make check
go test -race ./...
go test -run '^$' -bench . -benchmem ./...
shasum -a 256 internal/sketch/*/testdata/*.png

go run ./cmd/staticart render contour --profile preview --seed 42 --out out/architecture-baseline/previews
go run ./cmd/staticart render qql --profile preview-tall --seed 42 --out out/architecture-baseline/previews
go run ./cmd/staticart render foam --profile preview --seed 42 --out out/architecture-baseline/previews
go run ./cmd/staticart render scree --profile preview --seed 42 --out out/architecture-baseline/previews
go run ./cmd/staticart render riffle --profile preview --seed 42 --out out/architecture-baseline/previews
go run ./cmd/staticart render shallows --profile preview --seed 12 --water-seed 42 --out out/architecture-baseline/previews
go run ./cmd/staticart render pools --profile preview --seed 42 --out out/architecture-baseline/previews
go run ./cmd/staticart render drift --profile preview --seed 42 --out out/architecture-baseline/previews

go run ./cmd/staticart sweep contour --seeds 1-4 --profile preview --jobs 2 --out out/architecture-baseline/sweeps/contour
go run ./cmd/staticart sweep qql --seeds 1-4 --profile preview-tall --jobs 2 --out out/architecture-baseline/sweeps/qql
go run ./cmd/staticart sweep foam --seeds 1-4 --profile preview --jobs 2 --out out/architecture-baseline/sweeps/foam
go run ./cmd/staticart sweep scree --seeds 1-4 --profile preview --jobs 2 --out out/architecture-baseline/sweeps/scree
go run ./cmd/staticart sweep riffle --seeds 1-4 --profile preview --jobs 2 --out out/architecture-baseline/sweeps/riffle
go run ./cmd/staticart sweep shallows --seeds 10-13 --water-seed 42 --profile preview --jobs 2 --out out/architecture-baseline/sweeps/shallows
go run ./cmd/staticart sweep pools --seeds 1-4 --profile preview --jobs 2 --out out/architecture-baseline/sweeps/pools
shasum -a 256 out/architecture-baseline/previews/*.png out/architecture-baseline/sweeps/*/sheet.png
```

`make check` and `go test -race ./...` passed. `golangci-lint` reported zero
issues. The benchmark command found no benchmarks: every package only reported
`PASS`. That absence is itself part of the baseline.

All seven contact sheets were inspected. They showed healthy seed variation
and the expected identities: contour bands, QQL structures, foam regions,
faceted scree, directional riffle water, integrated shallows, and painted
pools. The individual drift preview was also inspected as a bristle-canvas
example.

### Golden identifiers

```text
1d7d49b3e5e3c9740428fd76e39d6506a05bc078697a36623332678caf38cbc5  circles_seed42_64.png
71b62c916c61cd66395c4bb0c959625097d0e8047511e7a4f1975ff39ee6e62c  contour_seed42_64.png
82475bd8880b72e06433e7a41eb2f25053874f6cda01b72f2acb8f0aa89c2e9b  drift_seed42_64.png
966bf6e531cd9df0d781aa50cb6e82d1b847c9b42e30e18fe69d51cdaf553d83  foam_seed42_96.png
8402e9c7ca6bc6645d6ea09dbc659a395063a21ddcdb5f814b53d425b56daaf4  hatchbook_colour_96.png
65d12062ebd8488d6ec65a6ff9fdfbe0c9ede6751e50071fdfa0e38248aca737  hatchbook_parameters_96.png
e8eefdd26eee5df5dc8777517c422da333c964102c8a00e5365979f633f25963  hatchbook_shapes_96.png
df263443f6f1a97d443ac3e90816537aefd341181196572c52818bd4d8c3f2f4  hatchbook_structures_96.png
96529627c88abe4bb14f7b9bf7f5e39b8c459c1100be0c177c0df7e719b34ca1  hatchbook_variation_96.png
f713b5c80b01c4af28349da5d83b12e90b0b9d63a310f6267fa37d9e7ee24c98  pools_seed42_96.png
5dc48272b5bbfbd3578a366d5a518504f5ab1647a85c4532e902626a8e116874  qql_seed42_64.png
b668976a67038cb00a35f8dfaacc49225b75a1710dafcb343974826b5bb05399  riffle_seed42_96.png
0b287e0a9cbdcaae9c6ffae3272c348add3a7d60ed3a90105e3726d0aec22d12  rounds_seed42_64.png
ebba70be58344e8639729f3d115199d72e7790562358d1775c31dcbe2dae6f80  scree_seed42_96.png
b7c469841be28e5cdadcfce0d95a013657103924e7591dd1700a53b0055d79b7  shallows_seed12_96.png
27188561ffe8e792ee2f95dc89de9a3a2bdf7797a98776410cd114fd5294f74c  shoal_seed42_64.png
ec5893d290173add3fabaed2c43cab47c5f7859d12d9a6d85f8de90bc1bc58cf  tapestry_seed42_64.png
```

### Preview and sweep identifiers

```text
1f0d768cbabed9862dd4f0d02c06e32c41a07b67d2c673d8a9fa4e042133abad  contour preview
3a7bf75a24ef178cb5c5e0b2545e9c7279b2ead04a07b0a5ee632b8b7efa53cf  drift preview
4ed5322d3252bc97145b18266b3d916b85a2f95712e26b96d77c511752d750e7  foam preview
8e0221d0a03ee648d285266b77f8157fdaba4b371685619327a54256c8780c28  pools preview
5cdfb8b9162f5efac30e19744bad415574e6738d1e2d7f21b044eaf494951225  qql preview
139890ece011013757bbdd99afad6792ccfa140bb0cc0ab9ac5dc6a7815cb0d4  riffle preview
5a08f29d8c538eaa02a8fc8f868004528452fc9f6e3882e094173de4f6ecdc13  scree preview
a50ce9df30a5e1a6ae5be553dbc1851bff444463e604db8bcbec0feffc5ebc26  shallows preview
aed5989e44616d1c5c9ee696cb9a609afd657273ff991d5c95159d1a2f598c41  contour sweep sheet
f1b35b156707bb841fb2be7925a2ac5e484bf0474260b5d4c133fa3a71bf3f2c  foam sweep sheet
59bc2afd30d90a9e8502a6558bfa094c4ffefddb7768d66ad08d4e4aa5ac6953  pools sweep sheet
089ba77c9bbf3023e3e6c7ae131af2a49a8764390ff0dd3ad5280f381eb748fe  qql sweep sheet
26842b621962d66bf886235b05b15cd953a0bcd2f81eceaf52f6e41350b937a6  riffle sweep sheet
000e21b8348e795e1dd2b2ee8aca4e5bf895158be19d7590db080800e498783d  scree sweep sheet
820fc6553cfbe8b6b1f57d5b89bc9b8c9dfb86311365fd78523cdb5edcd851b7  shallows sweep sheet
```

The hashes include encoded metadata. Pixel comparisons in golden tests remain
the stronger compatibility check when build-revision metadata changes.

## Actual execution path

### Command and lookup

`cmd/staticart/main.go` dispatches `list`, `palettes`, `render`, `sweep`, and
`traits`. `registry()` explicitly constructs one new instance of every sketch
for each lookup. There is no `init` registration and no long-lived global
sketch. This matters because sketches are mutable CLI adapters: a parallel
sweep is safe because each `renderOne` call receives fresh instances, not
because a configured sketch is immutable.

`runSweep` strips only sweep-owned flags, expands the seed/flag Cartesian
product, and runs `renderOne` in a bounded worker pool. It then writes each
render, a manifest, and a downsampled contact sheet. The sweep knows no
sketch-specific flags.

### Configuration and traits

`renderOne` creates one `flag.FlagSet` containing common output flags and the
selected sketch's flags. If the sketch implements `Configurable`, `Flags`
registers sketch options and `Configure` validates and mutates the sketch
after parsing. `opt.Set` records which flags were explicitly visited, which is
how trait-derived values remain distinguishable from overrides. `trait.Options`
does the equivalent job for discrete dimensions.

The CLI then resolves dimensions through `Traited.Traits(ctx)` to construct
the filename and metadata. The sketch resolves the same traits again while
planning. This is deterministic because both calls start a fresh PCG stream,
but it is a duplicated lifecycle step and the interface does not prove that
the reported traits are the traits the render consumed.

### Seed and streams

`sketch.Context.RNG(stream)` returns a fresh
`rand.New(rand.NewPCG(ctx.Seed, stream))`. Sketches use package-local numeric
stream constants for independently evolving concerns. Hash and noise fields
derive directly from the seed plus named numeric salts. No random generator
is used in the parallel raster loops.

The policy is strong but convention-based. Stream ownership is documented in
comments and sketch specs, not represented by a type. Adding draws within one
stream intentionally changes later draws in that stage; adding another stream
does not. `foam` currently gives `streamHatch` and `streamTiles` the same value
(`5`). They each create a fresh generator, so ordering is stable, but their
draws are correlated and the comments incorrectly imply independent streams.
Changing this would change existing images, so it is migration debt rather
than a behavior-preserving cleanup.

### Resolution, planning, and evaluation

The common context carries width, height, seed, palette, supersampling, and
bit depth. Composition code converts dimensions to `aspect = width/height` and
uses canvas coordinates where height is one. Fixed analysis grids, not output
pixels, measure partitions and choose visible stones. The output dimensions
otherwise enter only raster execution and the stamp canvas.

There are three real execution shapes:

```text
point sampled
resolved parameters -> compact fields/model -> pure At(u,v) -> sketch.Raster

planned and painted
resolved parameters -> marks -> sequential paint.Canvas operations -> Image

composed material
bed model + surface model -> one pure combined sample -> sketch.Raster
```

`render.RasterSS` and `RasterDeep` own normalized sample positions, parallel
rows, linear-light supersample averaging, dither, and quantization.
`RasterLayerSS` and `RasterLayerDeep` add premultiplied-alpha averaging.
`paint.Canvas` owns a sequential float color buffer and quantizes it through
`render.ImageFromColors` at the end.

### Output

The sketch returns an `image.Image`; it does not choose filenames or encode
files. The CLI builds a canonical recipe, filename, DPI, software revision,
and trait metadata. `render.WritePNGMeta` or `WriteJPEGMeta` performs encoding
and embeds the metadata. This is a clean outer boundary.

## Package inventory

The classifications below distinguish mechanism from artistic policy and
state whether useful data survives before rasterization.

| Package | Responsibility and main types | Input -> output | Kind and stage | Configuration, randomness, tests |
|---|---|---|---|---|
| `mathx` | Scalar interval operations | numbers -> numbers | mechanism; evaluation | no config or RNG; table tests |
| `rnd` | weighted draws, Gaussian draws, winnowing, bags, ladders | explicit `*rand.Rand`, tables -> values/slices | mechanism; resolution/planning | caller owns stream; no direct tests at baseline |
| `opt` | declarative sketch flag registration, validation, suffixes, `WasSet` | destinations + `FlagSet` -> mutated destinations and suffix | CLI mechanism | owns no art policy or RNG; unit tested |
| `trait` | weighted output-space schemas and overrides | schema + RNG + pins -> `trait.Set` | resolution mechanism | schema order owns RNG consumption; unit tested |
| `palette` | sRGB `Color`, provenance-bearing `Palette`, HSL/HSB and `Swatch` | color data -> colors/families | color mechanism plus source data | explicit RNG only for swatch draws; thoroughly tested |
| `gradient` | continuous and discrete color maps, terracing | scalar -> color | appearance mechanism/evaluation | RNG only when explicitly shuffling; tested |
| `noise` | Perlin/fBm, curl, Worley, coordinate hash | seed + coordinate -> scalar/vector/cell identity | field mechanism/evaluation | immutable seed-built tables or hashes; tested |
| `geom` | circles and uniform collision/containment index | circles -> queryable index | geometry mechanism/planning | no RNG; tested |
| `cells` | weighted, mergeable partition with wall/junction fields and measured regions | sites/groups/aspect -> `Foam`; coordinate -> `Hit` | geometry, partition, measurement, evaluation | caller provides merge RNG; fixed-resolution measurement; strong invariant tests |
| `hatch` | immutable repeated-mark coverage and composition | `Spec` -> `Hatch`; region `Sample` -> coverage | appearance mechanism/evaluation | seed hashes and Perlin, no mutable RNG in evaluation; structural tests and specimen sketch |
| `scheme` | hue and tone assignment over discrete regions | `Spec` + `[]Region` -> immutable `Mixer` | appearance mechanism/planning | private seed-derived RNG; strategy tests |
| `paint` | sequential float canvas, bristle brushes, analytic rings, washes, flat wash/glaze | paths/marks/colors -> mutable `Canvas` or point color | painting mechanism, with `FlatWash` also an evaluator | RNG always explicit for mark construction; extensive material tests |
| `render` | profiles, raster loops, alpha layers, quantization, encoding, metadata, contact sheets | pure sampler or image -> raster/file | output rendering | no artistic RNG; tested |
| `sketch` | stable artwork boundary, context, registry, raster adapters | configured artwork + context -> `image.Image` | outer orchestration | stream factory in context; registry itself has no tests |
| `sketchtest` | determinism and golden helpers | sketch + contexts -> assertions | testing support | no production policy |

These packages are mostly cohesive. `paint` is the broadest: it contains both
sequential canvas operations and `FlatWash`, a pure point evaluator. They
still share one precise material domain and pigment math, so splitting them
would not currently improve dependencies.

## Representative sketches

### `contour`

`Sketch.Render` validates the palette, constructs three discrete gradients
and one Perlin field, then closes over them in one pure pixel function. The
gradient shuffles own streams 1 and 2; noise derives directly from the seed.
There is no useful object layout and no reason to invent one. The reusable
intermediate is simply a scalar field plus a color mapping. The code is small
and independently deterministic, but planning and sampling are only separated
by local variables, so there is no direct sampler-purity or planning test.

### `qql`

QQL has the clearest planned/painted structure. `plan` stores resolved traits,
the frame, color scheme, stack offset, and fully specified dots. Planning
resolves numeric specs, a direction grid, grouped starts, line decisions,
color sequences, and collision-packed dots through eight documented RNG
streams. `Render` then creates `paint.Canvas` and interprets the plan with an
independent paint stream. Ink and wash share composition and differ only at
the material operation. Tests inspect dot bounds and prove that medium changes
only paint. The plan is private, correctly: no second consumer needs QQL's
particular dot vocabulary.

### `foam`

`sheet` is the real plan/scene even though the documentation uses both words.
It owns the outer `cells.Foam`, per-cell dressing, resolved levels, field,
flat-wash evaluator, paper/ink/ramp, hatches, and optional inner tiling.
`plan` performs site packing, merging, partition measurement, color scheme
resolution, material assignment, hatching construction, and subdivision.
The raster closure then performs warp -> cell lookup -> fill -> tile -> hatch
-> relief -> ink.

The files correspond to concepts, and most concepts also have types or
functions (`dress`, `tiling`, `hatching`, `reliefLevels`). The remaining
problem is that evaluation order exists only in the long closure and methods
on mutable `Sketch`. `sheet` cannot answer a sample itself, so the planned
result is not independently reusable. Subdivision is independently testable,
but planning, appearance, and evaluation still share the large `Sketch`
configuration object. `streamHatch`/`streamTiles` collision is noted above.

### `scree`

`sheet` owns a measured stone partition, optional facet partition with
precomputed per-facet lighting, per-stone material, levels, warp field, wash,
and ink roles. `NewBed` exposes the exact planned output as `Bed.At`, and
`Render` is already the ideal thin wrapper: plan once, pass `Bed.At` to the
rasterizer. Expensive packing, measurement, facet construction, color scheme,
and facet lighting happen outside the pixel loop. The hot path is allocation
free and RNG free.

`Bed` is a meaningful reusable intermediate and has a real second consumer,
`shallows`. Its weakness is package ownership: constructing it requires a
complete mutable `scree.Sketch`, so a reusable stone-bed material is trapped
inside the concrete artwork package together with CLI traits and identity.
The implementation is also highly artistic; moving all of it to a generic
geometry package would be worse than the present coupling.

### `riffle`

The private `plan` combines resolved channel settings, channel geometry,
rocks, color roles, independent noise fields, foam/light constants, and
overlay data. `pixel` composes bed, velocity, upstream LIC walk, caustics,
water column, surface, and foam. All per-pixel work is pure and allocation
free, but deliberately expensive: each pixel walks the field for a configured
number of steps.

`Surface` exposes an all-water point sampler for composition. It plans a fixed
pool/bend/field/clear/dappled vocabulary and returns a typed `SurfaceSample`
containing direction, slope, streak, ripple, and dapple. This is independently
tested and is a useful intermediate representation. Like `Bed`, it remains in
a concrete artwork package. Unlike a general vector-field interface, its
sample is deliberately water-specific and therefore useful.

### `shallows`

`shallows` is the repository's strongest evidence about composition. It plans
`scree.Bed` and `riffle.Surface`, samples both inside one raster function, and
changes the bed lookup and bed color before quantization. There is no
full-frame intermediate. This is the desired composition style: typed domain
components, explicit math, one final sampler.

The package imports two concrete sketch packages and embeds a complete scree
sketch to inherit flags and traits. This is convenient but conflates "scree
the CLI artwork" with "the planned stone bed". It also means the water
component is fixed to one hard-coded riffle preset rather than constructed
from an explicit reusable surface configuration.

### Brush and wash sketches

`drift` separates dot placement from painting through `[]dot`; `shoal` uses a
stateful planner and then groups dots into runs before painting; `pools`
constructs fully specified circles before sequentially laying ground and
washes; `rounds` intentionally collapses planning and painting because its
regular grid has no useful independent scene. These are appropriate variants,
not inconsistencies to erase.

`pools` is the clearest wash-based example: layout and mark material are
settled before any canvas mutation, then marks are painted largest first.
`paint.Canvas.Scale` is consulted only to skip wash details that cannot
resolve; composition lengths remain in canvas units. Exact brush texture is
resolution-sensitive by documented design, while composition is not.

## Problems found

### 1. The documented `plan/scene` is a pattern, not a shared concept

There is no repository-wide plan type or contract. That is mostly correct.
`circles`, `tapestry`, `qql`, `riffle`, `foam`, and `scree` all have private
planned values with different meanings; `contour` needs only local immutable
fields; `rounds` gains nothing from a plan. The architecture document implies
a more uniform stage than the code actually has.

The real common rule is weaker and more useful: perform seed-dependent,
expensive, or mutable work once; make repeated evaluation pure where the
rendering model allows it. A common plan interface would add no operation that
all these values meaningfully share.

### 2. Configuration, resolution, and execution are coupled through mutable sketches

Most `Sketch` structs simultaneously hold defaults, raw CLI destinations,
configured values, override bookkeeping, trait resolvers, artistic policy,
and methods used by the hot evaluator. Fresh construction makes the CLI and
sweeps safe, but the types do not make that lifecycle obvious. Reusing one
configured instance for concurrent renders is not a stated guarantee.

Universal conversion to `Config` structs would be churn. It is justified
where a sketch is composed by another sketch or where planning tests currently
need to simulate CLI parsing. The scree embedding in shallows is the clearest
case. Small fixed sketches gain little.

### 3. Reported and rendered trait resolution are separate calls

The CLI resolves traits for names/metadata and each sketch resolves them again
for planning. Fresh deterministic streams make the values equal today. The
contract does not pass the resolved set into rendering, so equality depends
on convention. It also makes "resolution" less visible than the desired
lifecycle suggests.

Changing the stable `Sketch.Render` boundary to carry a generic resolved value
would force every sketch into a common pipeline. A smaller remedy is to keep
trait derivation pure, test that reporting and planning agree, and let
composition-facing components accept explicit resolved configuration when a
second consumer requires it.

### 4. Reusable bed and water models live in concrete sketch packages

Only `shallows` imports another concrete artwork, and it does so for genuine
pre-raster composition. The dependency is not a raster-compositing error; the
models are exactly the right shape. The ownership is the issue. `scree.Bed`
and `riffle.Surface` are reusable domain components with real consumers, but
their constructors and options remain tied to complete artwork wrappers.

A wholesale move of scree or riffle into generic packages would move artistic
policy away from the artworks and create a large migration. The useful seam
is narrower: explicit component configuration and construction, with sketch
wrappers retaining traits, CLI adaptation, defaults, and identity. That seam
should be proven incrementally rather than by moving files first.

### 5. Some stage boundaries are file boundaries only

Foam's `fills.go`, `subdivide.go`, `hatching.go`, and `shade.go` are strong
conceptual files, but the planned `sheet` delegates all evaluation back to the
mutable `Sketch`. Tapestry's large `Render` closure similarly owns layering
order. This makes the stages readable but less replaceable and less directly
testable than their file layout suggests.

The fix is not exported interfaces. A private `sample` or `At` method on a
planned value is enough to name and test evaluation. Separate concrete
configuration values are warranted only when another component needs to
construct or replace a stage.

### 6. Performance evidence is missing

The code already applies important performance practices: spatial indices,
fixed-size lookup state, precomputed facet lighting, no RNG in raster loops,
no intermediate full-frame composition, analytic rings, and reused tracing
buffers. There were no benchmarks to protect those properties or separate
planning cost from repeated evaluation. The most expensive known paths are
cell measurement/facet planning, stamp/wash painting, and riffle/shallows
per-pixel upstream walks.

### 7. Naming is locally clear but globally overloaded

`Render` correctly means the stable artwork-to-image boundary and output
raster execution, while sketch methods also use `paint`, `surface`, `plan`,
`sheet`, and `scene` in local senses. The ambiguity is mostly documentation,
not code failure. Use:

- **resolve** for traits/ranges/overrides becoming concrete parameters;
- **plan** for seed-dependent data built once;
- **sample/evaluate/At** for pure coordinate queries;
- **paint** for order-dependent canvas mutation;
- **rasterize** for executing a sampler over pixels;
- **encode** for writing a file.

`surface` should name a geometric/material field, not a generic pipeline
stage. `scene` should be avoided unless the value actually contains several
spatial objects or materials.

## Non-problems and rejected diagnoses

- The CLI contains no sketch-specific artistic logic. Explicit imports in
  `registry()` are wiring, not coupling that needs reflection or `init`.
- No full-frame intermediate is used for shallows. Its composition already
  happens at material-sample level.
- Random work does not occur in raster hot loops. Coordinate hashes and noise
  evaluations are deterministic field evaluation, not uncontrolled draws.
- Interface dispatch is not a demonstrated bottleneck. Pixel functions are
  concrete function values, and domain interfaces should not be introduced
  merely to label stages.
- Duplicated `pt`, `paint.Pt`, `geom.Circle`, `hatch.Sample`, `cells.Site`, and
  riffle readings represent different domain promises. A universal point or
  region model would erase useful distinctions for little gain.
- `paint.Canvas` and point-sampled rendering are not variants of one weak
  interface. Their common boundary is the final `image.Image` plus shared
  coordinates and color math.
- `Sketch.Render` remains a good stable outer boundary. The diversity is
  behind it, where it belongs.

## Recommendation

Keep `Sketch.Render` and private concrete plans. Standardize the internal
shape by example and vocabulary, not by a mandatory interface:

```text
definition/defaults + overrides
    -> resolved concrete values
    -> optional private plan/model
    -> either pure sample/rasterize or sequential paint
    -> image.Image
```

Plans remain private unless a real pre-raster consumer exists. Where one does
exist, expose a narrowly typed domain model such as a stone-bed color sampler
or water `SurfaceSample`, with explicit immutable construction inputs. Compose
those models rather than complete rendered sketches. Do not add a universal
`Plan`, `Stage`, `ScalarField`, `VectorField`, or `ColorSampler` contract until
two consumers need substitution through that exact operation.

Pilot the pattern on:

1. `contour`, to show that a simple sampler needs no artificial plan;
2. `qql`, to document and benchmark a planned sequential painter;
3. `foam`, to make its planned sheet own the evaluation boundary;
4. scree/riffle/shallows, to clarify the existing reusable domain models and
   remove concrete artwork coupling only through proven, typed seams.

The selected target and alternatives are evaluated in `pipeline-design.md`.
