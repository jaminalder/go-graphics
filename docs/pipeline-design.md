# Pipeline design

Target architecture derived from `architecture-review.md`. The word
"pipeline" here describes a lifecycle and a vocabulary. It does not imply a
runtime stage list or one interface every artwork must implement.

## Goals

The design must preserve the repository's strongest existing choices:

- `sketch.Sketch.Render` is the stable artwork-to-image boundary.
- The CLI remains generic and owns output concerns, not art logic.
- Randomness is explicit, deterministic, and divided into stable streams.
- Composition uses normalized canvas coordinates and is independent of output
  resolution.
- Point-sampled rasterization and sequential painting remain visibly
  different execution models.
- Reusable packages implement real graphics mechanisms; sketch packages keep
  artistic policy.
- Typed Go composition is preferred over dynamic stage machinery.

The design must improve the weak points found in the review:

- distinguish defaults, overrides, resolved values, planned models, and
  repeated evaluation;
- make expensive planning independently testable and benchmarkable;
- make reusable pre-raster models usable without treating a complete artwork
  wrapper as the component;
- make RNG ownership and hot-path rules easy to verify;
- keep simple sketches simple.

## Option A: evolutionary private plans

Keep all existing public contracts. Standardize a recommended internal shape,
without adding a shared plan type:

```go
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
    model, err := s.plan(ctx)
    if err != nil {
        return nil, err
    }
    return sketch.Raster(ctx, model.At), nil
}
```

Painted work uses a different final step:

```go
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
    marks, err := s.plan(ctx)
    if err != nil {
        return nil, err
    }
    canvas := paint.NewCanvas(ctx.Width, ctx.Height, marks.Ground)
    marks.Paint(canvas, ctx.RNG(streamPaint))
    return canvas.Image(), nil
}
```

`model`, `marks`, `plan`, and `At` remain private unless another real package
needs them. A simple sketch may collapse `resolve`, `plan`, and model
construction into a few local immutable values. A regular painted study may
paint directly when no inspectable mark set exists.

When a second artwork already consumes a model before rasterization, extract
or expose a precise domain component. Examples are a planned stone bed that
answers color at a coordinate and a water surface that answers direction,
slope, ripple, and dapple. These are concrete types, not generic sampler
interfaces.

### Evaluation

| Criterion | Result |
|---|---|
| Simplicity | Best. No new framework or outer contract. |
| Idiomatic Go | Concrete private types and ordinary functions. |
| Type safety | Strong within each artwork; domain components have exact outputs. |
| Composability | Explicit where compatible, as shallows already demonstrates. |
| Testability | Plans and `At` methods can be tested directly. |
| Determinism | Existing stream policy remains visible at constructors. |
| Performance | No required interface dispatch or allocations in hot loops. |
| Raster and paint compatibility | Preserves their different control flow. |
| New artwork cost | Low; use only the stages the artwork needs. |
| AI-assisted changes | Good if files and methods follow the lifecycle vocabulary. |
| Premature abstraction risk | Lowest. |

The limitation is intentional inconsistency: private model names and exact
shapes differ. Documentation and examples, not a compiler-enforced universal
contract, carry the pattern.

## Option B: optional shared planning and sampling contracts

Keep `Sketch.Render`, but add optional interfaces in `internal/sketch` or a
new package:

```go
type Planner[P any] interface {
    Plan(Context) (P, error)
}

type ColorSampler interface {
    At(u, v float64) palette.Color
}

type ScalarField interface {
    At(u, v float64) float64
}

type VectorField interface {
    At(u, v float64) geom.Vec2
}
```

Raster sketches could return a `ColorSampler`; benchmarks and test helpers
could accept it. Reusable fields could be substituted through common
interfaces. Painted plans would require a separate contract such as
`Paint(*paint.Canvas)`.

### Evaluation

| Criterion | Result |
|---|---|
| Simplicity | Moderate. Generic planner types add vocabulary and wiring. |
| Idiomatic Go | Small consumer-owned interfaces are idiomatic; central ones are not automatically so. |
| Type safety | Better than dynamic stages, but a color sampler loses material-specific information. |
| Composability | Useful only where consumers need substitution through the same operation. |
| Testability | Shared helpers become possible, but private concrete methods already permit the same assertions. |
| Determinism | Neutral; interfaces do not express RNG ownership. |
| Performance | Usually small, but interface calls in millions of samples need evidence. |
| Raster and paint compatibility | Requires two parallel contracts immediately. |
| New artwork cost | Higher if contributors feel obliged to implement optional concepts. |
| AI-assisted changes | Names may help, but weak interfaces invite inappropriate adapters. |
| Premature abstraction risk | Significant: no current need to substitute arbitrary scalar/vector/color fields. |

This is plausible later. It is not justified now. `riffle.SurfaceSample` is
more useful than reducing the water to a `VectorField`; `scree.Bed.At` being a
color sampler does not mean every color sampler can replace a stone bed in
shallows.

## Option C: universal staged pipeline

Represent every artwork as an ordered list of stages with a common process
operation, dynamic value, or generic scene:

```go
type Stage interface {
    Process(any) (any, error)
}
```

or:

```go
type Stage[I, O any] interface {
    Run(I) (O, error)
}
```

The apparent benefit is uniform orchestration, inspection, and replacement.
The actual repository would need adapters for scalar fields, partitions,
fully planned marks, mutable canvases, images, and material samples. The
sequence would either become weakly typed or grow a large hierarchy.

### Evaluation

| Criterion | Result |
|---|---|
| Simplicity | Poor. |
| Idiomatic Go | Poor for this fixed, explicit set of compositions. |
| Type safety | Poor with `any`; cumbersome with generic chains. |
| Composability | Broad in theory, mostly adapters in practice. |
| Testability | Stages can be tested, but meaningful domain functions already can be. |
| Determinism | RNG ownership becomes easier to hide behind stage objects. |
| Performance | Risks allocations, interface chains, and lost inlining. |
| Raster and paint compatibility | Hides their essential execution difference. |
| New artwork cost | High ceremony. |
| AI-assisted changes | Superficially discoverable, semantically weak and easy to misuse. |
| Premature abstraction risk | Certain. |

This option is rejected.

## Selected design

Select Option A, with one limited principle borrowed from Option B:
interfaces may be introduced at a consumer boundary only after two concrete
implementations need substitution through the same operation. No such new
interface is required by the current migration.

The stable outer flow remains:

```text
CLI lookup
-> register and parse common + sketch-owned flags
-> validate overrides
-> construct Context
-> resolve traits and concrete ranges from named RNG streams
-> build a private plan or reusable domain model once
-> evaluate through either a pure sampler or sequential painter
-> return image.Image
-> CLI encodes with metadata
```

### Definition and configuration

An artwork package owns:

- stable name and description;
- default artistic values;
- trait schema and weights;
- CLI declarations and implications;
- the composition of mechanisms;
- aesthetic constraints and exceptions.

The current `Sketch` struct may continue to be the definition and CLI adapter
for sketches that are not composed elsewhere. Fresh construction per CLI
invocation remains required.

Use an explicit concrete configuration value when at least one is true:

- another artwork constructs the component;
- planning tests should not need a synthetic `flag.FlagSet`;
- the component has more than one preset or material interpretation;
- mutable CLI state is otherwise retained by a planned object.

Do not convert every sketch for uniformity. Do not introduce a generic
parameter map.

### Resolution

Resolution is the deterministic conversion from traits and explicit
overrides to concrete values for one seed. It happens before geometry or
painting. The result should be a concrete value (`levels`, `settings`,
`Specs`, or a domain-named `Config`) and should not retain an RNG.

Trait derivation may remain exposed through `sketch.Traited` for the CLI. A
planning test must prove that the set used for reporting is the set used by
the plan when a sketch has nontrivial trait behavior. A future change may
cache one resolved set in command orchestration, but the present migration
will not alter the stable `Render` signature merely to remove a cheap
deterministic second call.

### Plans and models

A plan is seed-dependent data built once and reused during painting or
sampling. It is not mandatory.

Keep a plan private when:

- only one sketch consumes it;
- exposing it would reveal many artwork-specific details;
- another output target is only hypothetical.

Promote a component when:

- a second real artwork consumes it before rasterization;
- its responsibility is independently meaningful;
- it can expose a small typed operation;
- promotion improves dependency direction without moving generic policy into
  a mechanism package.

Promotion does not require making the component aesthetically neutral. A
"planned faceted river-stone bed" is a coherent reusable art system even
though it is not a generic mesh library. Its low-level partition, palette,
wash, and lighting mechanisms remain in their existing shared packages.

Planned values should be treated as immutable after construction. Their
repeated methods must be safe for concurrent raster rows.

### Geometry, fields, and transformations

Continue using precise existing representations:

- `geom.Circle` and `geom.Index` for packed circles;
- `cells.Foam`, `cells.Cell`, and `cells.Hit` for addressable partitions;
- `noise.Perlin` and coordinate hashes for deterministic fields;
- sketch-private structs for channels, rocks, facets, dots, and layouts.

Do not unify `paint.Pt`, QQL's tracing point, hatch samples, cells sites, and
river readings. They carry different guarantees.

Transformations belong with the domain object they transform unless proven
general. Foam's warp and stone faceting are artistic policy using shared
mechanisms, not candidates for a generic transformation pipeline.

### Appearance and materials

`scheme` remains the shared mechanism for assigning hue and value to discrete
regions. `hatch` remains a color-free coverage mechanism. `paint` remains the
material mechanism for brushes and washes. Sketches decide which strategy,
parameter ranges, order, palette interpretation, and exceptions to use.

Appearance that is constant per region or facet should be resolved during
planning. Coordinate-dependent material behavior remains in the evaluator.
For example, scree precomputes flat facet light, while riffle computes a
surface walk per coordinate because the walk is the texture itself.

### Sampling and painting

Point-sampled work ends in a pure function or method with this practical
contract:

- no allocation in the normal path;
- no mutation;
- no mutable RNG;
- no option or trait resolution;
- no geometry construction;
- safe concurrent calls;
- coordinates in canvas units.

Sequential work plans marks before creating a `paint.Canvas` where useful,
then applies them in a documented order. The painter may use an explicit RNG
stream because order is part of the medium. `Context.AA` remains irrelevant to
stamp-painted work; soft edges and material resolution rules remain explicit.

### Output rendering

No change. `sketch.Raster` and `RasterLayer` remain convenience adapters over
`render`. Sketches do not encode files, choose output paths, or construct
metadata. `paint.Canvas.Image` continues to share final quantization through
`render.ImageFromColors`.

## RNG ownership

Every random consumer belongs to one lifecycle stage. Define stream constants
beside the constructor that consumes them and document their owner:

```go
const (
    streamTraits = 1
    streamResolve = 2
    streamLayout = 3
    streamAppearance = 4
    streamPaint = 5
)
```

The names are illustrative; existing numeric IDs remain stable. Rules:

- a plan constructor receives `ctx.RNG(stream)` or a seed-derived immutable
  field explicitly;
- a reusable component does not silently read global or parent RNG state;
- per-coordinate variation uses seed-keyed hashes/fields, not random draws;
- optional effects get dedicated streams and do not consume base streams;
- changing draw order within one stream is an intentional seed change;
- stream numbers must be unique within a sketch unless correlation is
  deliberate and documented.

The current duplicate foam stream ID is documented debt. Correcting it would
change mosaic output and therefore requires an explicitly approved visual
change, not an architectural refactor.

## Testing model

Tests should locate change at the earliest meaningful boundary:

- resolution tests: pins affect only named values;
- plan tests: bounds, coverage, counts, adjacency, stable material assignment;
- evaluator tests: range, purity, resolution independence, flat-per-facet or
  other structural claims;
- painter tests: same marks under alternate media, order, wash mixing;
- raster tests: supersampling, alpha, quantization;
- golden tests: final compatibility;
- sweeps: artistic output-space review.

Plans should expose private query methods to same-package tests instead of
exporting internals for tests. Reusable domain models should have focused tests
in their own package and thin artwork wrappers should retain golden tests.

## Performance model

Measure planning and repeated evaluation separately where possible. Add
benchmarks for:

- contour: pure point evaluation and full raster;
- foam or scree: partition/facet planning and full raster;
- QQL or pools: planning and sequential painting;
- shallows: bed planning, surface samples, and combined raster.

Benchmarks use fixed seeds, palettes, and modest dimensions by default so
they run locally. A separate profile-scaling benchmark may use preview and web
sizes; print-size memory/time should be documented from explicit runs rather
than imposed on every `go test -bench` invocation.

No abstraction is accepted if it introduces allocations in a point sampler
or a material regression large enough to obscure the clarity benefit.

## Rejected abstractions

The following are deliberately not part of the target:

- universal `Plan` or `Scene` interface;
- `[]Stage`, `Process(any) any`, or generic pipeline builder;
- universal scalar/vector/color field interfaces;
- one point or region type for all packages;
- one rendering interface pretending raster sampling and canvas painting are
  the same operation;
- generic immutable parameter maps;
- service locators, plugin registries, reflection registration, or runtime
  dependency injection.

The repository remains a collection of explicit artwork systems sharing
proven graphics mechanisms, not a general graphics framework.
