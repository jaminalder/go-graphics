# Architecture Refactoring Implementation Plan

> **Execution constraint:** Work directly on `master`, inline in this session.
> Do not use subagents, worktrees, experiment branches, or branch-management
> tooling.

**Goal:** Make resolution, planning, pure evaluation, sequential painting, and
output rendering explicit and testable without imposing one pipeline contract
on every artwork.

**Architecture:** Keep `sketch.Sketch.Render` as the outer boundary. Use
private concrete plans for single-artwork data, typed domain models for proven
pre-raster composition, and distinct raster-sampling and canvas-painting
flows. Preserve every existing seed and golden unless a task explicitly says
otherwise; none of the tasks below is expected to change pixels.

**Tech Stack:** Go 1.26.5, standard library raster/image packages,
`math/rand/v2`, existing in-repository graphics packages, Make, golangci-lint.

---

## File map

The implementation is intentionally narrow.

- `internal/sketch/contour/contour.go`: make the smallest point-sampled
  artwork an explicit resolved sampler.
- `internal/sketch/contour/contour_test.go`: sampler and resolution tests.
- `internal/sketch/qql/qql.go`: keep the existing private mark plan and make
  the plan-to-canvas boundary explicit.
- `internal/sketch/qql/qql_test.go`: structural plan and resolution tests.
- `internal/sketch/foam/foam.go`: name the complete planned-sheet evaluation
  operation instead of leaving it only in `Render`'s closure.
- `internal/sketch/foam/foam_test.go`: sampler purity and stage tests.
- `internal/sketch/riffle/field.go`: replace the hard-coded reusable surface
  constructor with an explicit concrete configuration and default.
- `internal/sketch/riffle/field_test.go`: surface configuration and purity.
- `internal/sketch/shallows/shallows.go`: construct the typed water component
  explicitly while retaining the legitimate complete-scree composition.
- `internal/sketch/shallows/shallows_test.go`: verify independent bed and
  water seeds.
- `internal/sketch/*/*_test.go`: representative planning, evaluation, and
  render benchmarks.
- `docs/performance.md`: commands, results, allocation observations, and
  profile-scaling tradeoffs.
- `docs/ARCHITECTURE.md`: replace the implied universal plan with the actual
  optional lifecycle and component rules.
- `CLAUDE.md`: practical rules for agents modifying stages and composing
  artwork systems.

No new generic package or third-party dependency is planned.

### Task 1: Clean simple point-sampled pilot

**Files:**
- Modify: `internal/sketch/contour/contour.go`
- Modify: `internal/sketch/contour/contour_test.go`

- [ ] **Step 1: Write a failing structural test**

Add a same-package test that calls a new `plan(ctx)` method, samples fixed
coordinates twice, and asserts exact equality. Add a second assertion that
plans built for `96x64` and `960x640` agree at the same canvas coordinates.
The test should fail to compile because `plan` does not yet exist.

- [ ] **Step 2: Verify the test fails for the missing boundary**

Run:

```sh
go test ./internal/sketch/contour -run 'TestSamplerIsPure|TestPlanIsResolutionIndependent'
```

Expected: compile failure reporting that `(*Sketch).plan` is undefined.

- [ ] **Step 3: Extract the minimal concrete sampler**

Introduce a private concrete type containing the Perlin field, three discrete
gradients, and resolved numeric contour values. Give it:

```go
func (p plan) At(u, v float64) palette.Color
```

Move palette validation and immutable field/gradient construction to:

```go
func (s *Sketch) plan(ctx sketch.Context) (plan, error)
```

Make `Render` only call `plan` and `sketch.Raster(ctx, p.At)`. Keep stream IDs,
draw order, math, and receiver field values unchanged.

- [ ] **Step 4: Verify focused and golden compatibility**

Run:

```sh
go test ./internal/sketch/contour
make check
go test -race ./...
shasum -a 256 internal/sketch/contour/testdata/contour_seed42_64.png
```

Expected: all pass; golden hash remains
`71b62c916c61cd66395c4bb0c959625097d0e8047511e7a4f1975ff39ee6e62c`.

- [ ] **Step 5: Commit the pilot**

```sh
git add internal/sketch/contour/contour.go internal/sketch/contour/contour_test.go
git commit -m "refactor: separate contour planning from sampling"
```

### Task 2: Make the planned painter boundary explicit

**Files:**
- Modify: `internal/sketch/qql/qql.go`
- Modify: `internal/sketch/qql/qql_test.go`

- [ ] **Step 1: Write failing plan claims**

Add tests asserting that one seed at `64x80` and `640x800` produces identical
traits, frame aspect, stack offset, scheme, and dots, and that calling the new
private `paint(ctx, plan)` operation twice yields byte-identical pixels. The
paint test should fail to compile before the method exists.

- [ ] **Step 2: Run the focused tests red**

```sh
go test ./internal/sketch/qql -run 'TestPlanIsResolutionIndependent|TestPaintingAPlanIsDeterministic'
```

Expected: compile failure for the missing `paint` method.

- [ ] **Step 3: Extract only plan interpretation**

Keep the existing private `plan` unchanged. Move canvas construction, optional
wash construction, and `paintDots` from `Render` into:

```go
func (s *Sketch) paint(ctx sketch.Context, p plan) image.Image
```

`Render` becomes `plan -> paint`. Do not export the QQL plan: there is no
second consumer of its dot vocabulary.

- [ ] **Step 4: Verify pixels and races**

```sh
go test ./internal/sketch/qql
make check
go test -race ./...
shasum -a 256 internal/sketch/qql/testdata/qql_seed42_64.png
```

Expected golden hash:
`5dc48272b5bbfbd3578a366d5a518504f5ab1647a85c4532e902626a8e116874`.

- [ ] **Step 5: Commit the painter boundary**

```sh
git add internal/sketch/qql/qql.go internal/sketch/qql/qql_test.go
git commit -m "refactor: separate qql planning from painting"
```

### Task 3: Name foam's complete evaluator

**Files:**
- Modify: `internal/sketch/foam/foam.go`
- Modify: `internal/sketch/foam/foam_test.go`

- [ ] **Step 1: Write failing sampler tests**

Add a test that plans one sheet, calls a new private
`sample(sheet, seed, u, v)` method repeatedly at a fixed grid, and asserts
exact colors and unchanged structural counts. Add a test comparing samples
from plans at the same aspect but different pixel dimensions.

- [ ] **Step 2: Run focused tests red**

```sh
go test ./internal/sketch/foam -run 'TestPlannedSheetSamplesPurely|TestPlannedSheetIgnoresPixelDimensions'
```

Expected: compile failure for the missing `sample` method.

- [ ] **Step 3: Extract the existing evaluation sequence verbatim**

Move the body of `Render`'s pixel closure to:

```go
func (s *Sketch) sample(sh *sheet, seed uint64, u, v float64) palette.Color
```

Preserve this exact order:

```text
warp -> outer cell -> fill -> tile -> hatch -> relief -> ink
```

Make `Render` call `sketch.Raster(ctx, func(u, v float64) palette.Color {
return s.sample(sh, ctx.Seed, u, v) })`. Do not export `sheet`; no second
artwork consumes the foam composition.

- [ ] **Step 4: Verify structural and visual compatibility**

```sh
go test ./internal/sketch/foam
make check
go test -race ./...
shasum -a 256 internal/sketch/foam/testdata/foam_seed42_96.png
```

Expected golden hash:
`966bf6e531cd9df0d781aa50cb6e82d1b847c9b42e30e18fe69d51cdaf553d83`.

- [ ] **Step 5: Commit the evaluator boundary**

```sh
git add internal/sketch/foam/foam.go internal/sketch/foam/foam_test.go
git commit -m "refactor: isolate foam sheet evaluation"
```

### Task 4: Clarify the reusable water component

**Files:**
- Modify: `internal/sketch/riffle/field.go`
- Modify: `internal/sketch/riffle/field_test.go`
- Modify: `internal/sketch/shallows/shallows.go`
- Modify: `internal/sketch/shallows/shallows_test.go`

- [ ] **Step 1: Write failing component-configuration tests**

Define tests against a proposed exported concrete value:

```go
type SurfaceConfig struct {
    Seed     uint64
    Reach    string
    Channel  string
    Boulders string
    Water    string
    Light    string
}

func DefaultSurfaceConfig(seed uint64) SurfaceConfig
func NewSurface(ctx sketch.Context, cfg SurfaceConfig) (*Surface, error)
```

Test that defaults preserve the current pool/bend/field/clear/dappled surface,
invalid names return errors, and repeated `At` calls are exact. In shallows,
test that changing `WaterSeed` changes surface samples while two beds planned
from the unchanged main context return identical colors at fixed points.

- [ ] **Step 2: Run the component tests red**

```sh
go test ./internal/sketch/riffle ./internal/sketch/shallows -run 'TestSurfaceConfig|TestWaterSeedDoesNotChangeTheBed'
```

Expected: compile failures because `SurfaceConfig` and the new constructor do
not exist.

- [ ] **Step 3: Implement explicit surface resolution**

Add `SurfaceConfig`, `DefaultSurfaceConfig`, and validation through the
existing trait level functions. Keep the default's draw sequence and stream
exactly unchanged. Change shallows to:

```go
surface, err := riffle.NewSurface(ctx, riffle.DefaultSurfaceConfig(s.WaterSeed))
if err != nil {
    return nil, err
}
```

Do not extract a general vector-field interface. Do not move the complete
scree artwork: shallows deliberately inherits all scree traits and options,
so that concrete composition remains legitimate and documented. The reusable
boundary is `scree.Bed`, not an arbitrary color sampler.

- [ ] **Step 4: Verify all three composed sketches**

```sh
go test ./internal/sketch/riffle ./internal/sketch/scree ./internal/sketch/shallows
make check
go test -race ./...
```

Expected: no golden changes for riffle, scree, or shallows.

- [ ] **Step 5: Commit the typed surface configuration**

```sh
git add internal/sketch/riffle/field.go internal/sketch/riffle/field_test.go internal/sketch/shallows/shallows.go internal/sketch/shallows/shallows_test.go
git commit -m "refactor: make riffle surface construction explicit"
```

### Task 5: Add planning, sampling, and painting benchmarks

**Files:**
- Modify: `internal/sketch/contour/contour_test.go`
- Modify: `internal/sketch/foam/foam_test.go`
- Modify: `internal/sketch/qql/qql_test.go`
- Modify: `internal/sketch/shallows/shallows_test.go`
- Create: `docs/performance.md`

- [ ] **Step 1: Add benchmark functions with fixed inputs**

Add `BenchmarkPlan`, `BenchmarkSample`, and/or `BenchmarkRender` as applicable:

- contour: plan, one `At`, and `128x128` raster;
- foam: plan and repeated planned-sheet samples;
- QQL: plan and painting the same plan to a modest canvas;
- shallows: complete render at `96x96` and `192x192`.

Every benchmark must construct deterministic palette/context inputs outside
the timed loop where that input is not the subject, call `b.ReportAllocs()`,
and retain outputs in package-level sinks where needed.

- [ ] **Step 2: Run benchmarks and inspect allocation shape**

```sh
go test -run '^$' -bench . -benchmem ./internal/sketch/contour ./internal/sketch/foam ./internal/sketch/qql ./internal/sketch/shallows
```

Expected: named benchmark rows with `ns/op`, `B/op`, and `allocs/op`. Point
sampling benchmarks should report zero allocations per direct `At` call.

- [ ] **Step 3: Measure profile scaling explicitly**

Build once, then time representative AA1 renders at preview, web, and print:

```sh
make build
/usr/bin/time -l ./bin/staticart render contour --profile preview --aa 1 --seed 42 --out out/architecture-final/perf/contour-preview
/usr/bin/time -l ./bin/staticart render contour --profile web --aa 1 --seed 42 --out out/architecture-final/perf/contour-web
/usr/bin/time -l ./bin/staticart render contour --profile print --aa 1 --seed 42 --out out/architecture-final/perf/contour-print
/usr/bin/time -l ./bin/staticart render foam --profile preview --aa 1 --seed 42 --out out/architecture-final/perf/foam-preview
/usr/bin/time -l ./bin/staticart render foam --profile web --aa 1 --seed 42 --out out/architecture-final/perf/foam-web
/usr/bin/time -l ./bin/staticart render foam --profile print --aa 1 --seed 42 --out out/architecture-final/perf/foam-print
/usr/bin/time -l ./bin/staticart render qql --profile preview-tall --aa 1 --seed 42 --out out/architecture-final/perf/qql-preview
/usr/bin/time -l ./bin/staticart render qql --profile web-tall --aa 1 --seed 42 --out out/architecture-final/perf/qql-web
/usr/bin/time -l ./bin/staticart render qql --profile print-tall --aa 1 --seed 42 --out out/architecture-final/perf/qql-print
/usr/bin/time -l ./bin/staticart render shallows --profile preview --aa 1 --seed 12 --water-seed 42 --out out/architecture-final/perf/shallows-preview
/usr/bin/time -l ./bin/staticart render shallows --profile web --aa 1 --seed 12 --water-seed 42 --out out/architecture-final/perf/shallows-web
/usr/bin/time -l ./bin/staticart render shallows --profile print --aa 1 --seed 12 --water-seed 42 --out out/architecture-final/perf/shallows-print
```

If a print render exceeds available time or memory, stop that command, record
the exact failure and the largest completed profile, and do not claim a print
measurement.

- [ ] **Step 4: Document measured results**

Write `docs/performance.md` with the revision, hardware/runtime context from
`go version` and `sysctl -n hw.logicalcpu hw.memsize`, exact commands,
benchmark table, elapsed/max-resident-memory data from `/usr/bin/time -l`, and
the distinction between one-time planning and per-pixel scaling. Record that
stamp canvases retain a float color buffer and therefore have larger peak
memory than direct NRGBA raster output.

- [ ] **Step 5: Verify and commit benchmarks**

```sh
make check
go test -race ./...
git add internal/sketch/contour/contour_test.go internal/sketch/foam/foam_test.go internal/sketch/qql/qql_test.go internal/sketch/shallows/shallows_test.go docs/performance.md
git commit -m "bench: measure planning and sampling costs"
```

### Task 6: Update architecture and agent guidance

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Replace the implied universal plan**

Update the rendering section to show the three supported flows: simple
sampler, planned sampler, and planned painter. State that plans are private and
optional, `Render` is stable, and pre-raster components are promoted only for
real consumers.

- [ ] **Step 2: Document package and RNG ownership rules**

Add the selected rules from `pipeline-design.md`: mechanism versus artistic
policy, immutable planned values, explicit RNG stream ownership, no RNG or
allocation in hot samplers, and no concrete sketch-to-sketch dependency ban.
Document why shallows' complete scree composition is legitimate and why the
water `Surface` has a typed sample rather than a generic vector field.

- [ ] **Step 3: Add practical artwork templates to `CLAUDE.md`**

Include concise preferred structures for:

```text
simple: resolve local immutable fields -> At -> Raster
structural: resolve -> plan concrete model -> At -> Raster
painted: resolve -> plan marks -> sequential Canvas -> Image
composed: plan typed domain components -> combine samples -> Raster
```

Add rules for intermediate tests and when code earns a shared package.

- [ ] **Step 4: Add one complete example**

Use contour as the minimal example and shallows as the composition note. Show
configuration, planning, sampling, rasterization, and encoding ownership
without introducing pseudointerfaces.

- [ ] **Step 5: Verify and commit documentation**

```sh
git diff --check
make check
go test -race ./...
git add docs/ARCHITECTURE.md CLAUDE.md
git commit -m "docs: describe artwork lifecycle and component boundaries"
```

### Task 7: Final compatibility and visual verification

**Files:**
- No committed output files; all renders stay under ignored `out/`.

- [ ] **Step 1: Run the complete engineering gate**

```sh
make check
go test -race ./...
```

Expected: all packages pass and lint reports zero issues.

- [ ] **Step 2: Compare every golden hash with the baseline**

```sh
shasum -a 256 internal/sketch/*/testdata/*.png
```

Expected: every hash listed in `architecture-review.md` is unchanged. Do not
run `make golden`.

- [ ] **Step 3: Re-render representative previews**

Repeat the eight baseline preview commands with output under
`out/architecture-final/previews`, hash the PNGs, and compare their pixels or
full hashes to the baseline. Full hashes should remain equal because the
build revision embedded by `go run` may stay `dev`; if metadata differs, decode
and compare pixels.

- [ ] **Step 4: Re-render and inspect representative sweeps**

Repeat the seven baseline sweep commands under `out/architecture-final/sweeps`.
Read every `sheet.png` and check the relevant spec acceptance lists. Record
any visual difference; expected result is none.

- [ ] **Step 5: Verify non-square and independent effects**

Render contour and foam at `720x480`; render QQL at `480x600`; render shallows
twice with only `--water-seed` changed. Confirm dimensions, composition scale,
and that the bed remains fixed while the surface moves.

- [ ] **Step 6: Inspect repository state**

```sh
git status --short --branch
git log --oneline -10
git diff HEAD~7..HEAD --stat
```

Expected: clean `master`; no `out/` or regenerated golden files tracked.

## Expected visual compatibility

| Sketch | Expected result |
|---|---|
| contour | byte-identical pixels and unchanged golden |
| qql | byte-identical pixels and unchanged golden |
| foam | byte-identical pixels and unchanged golden |
| riffle | byte-identical pixels and unchanged golden |
| scree | no code-path change; unchanged golden |
| shallows | byte-identical pixels and unchanged golden |
| pools/drift | benchmark/verification only; unchanged goldens |

No task intentionally changes visual identity, seed meaning, CLI options, or
goldens. Any unexpected pixel change stops that migration step for diagnosis.

## Deferred debt

- Foam's hatch and tile streams both use numeric stream ID 5. Correcting it
  would change mosaic images and needs explicit approval.
- Trait reporting and planning derive the same set twice. It is cheap and
  deterministic; changing `Sketch.Render` to pass a generic resolved value
  would cost more architecture than it saves.
- Scree and foam share substantial artistic construction patterns, but their
  facet/tile grain policies differ deliberately. No extraction is planned
  until a truly identical mechanism has a second implementation.
- Broader immutable `Config` conversions remain case-by-case. Universal
  conversion would be repository-wide churn.
