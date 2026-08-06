# Warp Folded Depth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Revise `warp` so nested fields render as finely folded material with coherent dark cavities, pale ridges, and subtle resolution-independent surface lighting.

**Architecture:** The existing immutable plan gains a rotated sketch-local fBM evaluator, role-specific field scales, and a continuous material-height function. A folded appearance maps palette roles from that material signal, derives a normal through canvas-unit central differences, and applies fixed lighting in linear RGB without introducing shared field or lighting abstractions.

**Tech Stack:** Go standard library, existing `internal/noise`, `internal/palette`, `internal/mathx`, `internal/sketch`, CLI sweep/contact sheets, PNG visual inspection.

---

## File Map

- Modify `internal/sketch/warp/warp.go`: rotated octave evaluation, material signal, folded palette roles, central-difference normals, linear-light shading.
- Modify `internal/sketch/warp/options.go`: replace `structure` choice with `folded` if that is the minimal final appearance axis; add no numeric knobs.
- Modify `internal/sketch/warp/warp_test.go`: TDD tests for rotation, field roles, material finiteness, normals, lighting order, luminance structure, options, determinism, resolution, benchmark, and golden.
- Modify `internal/sketch/warp/testdata/warp_seed13_64.png`: deliberate candidate golden after visual inspection.
- Modify `docs/sketches/013-warp.md`: folded material algorithm, final appearance names, lighting, cost, acceptance checklist.
- Add `experiments/warp-nested-fbm/folded-depth-result.md`: follow-up commands, artifacts, visual findings, speed, commits, strongest/weakest seeds, recommendation.
- Keep old baseline imagery intact under `out/warp-nested-fbm/{seeds,modes,appearances}`.
- Generate candidate imagery only under ignored `out/warp-nested-fbm/folded-depth/`.

### Task 1: Pin Rotated Role-Specific Field Behavior

**Files:**
- Modify: `internal/sketch/warp/warp_test.go`
- Modify: `internal/sketch/warp/warp.go`

- [ ] **Step 1: Write failing tests for rotated fBM and role separation**

Add package-local tests with clear claims:

```go
func TestOctaveRotationChangesDirectionWithoutChangingBounds(t *testing.T) {
	// Compare the existing axis-aligned helper against the required rotated
	// evaluator at asymmetric coordinates. Require a changed value and finite
	// normalized output within the field's established practical bounds.
}

func TestNestedRolesUseDifferentSpatialScales(t *testing.T) {
	// A nested sample exposes q, r, raw value, and material height. Assert all
	// are finite, and compare nearby-coordinate variation over a grid so r/final
	// carry more local variation than q for representative seeds.
}
```

Do not compare exact reference-shader numbers. Tests describe repository-owned
properties only.

- [ ] **Step 2: Run focused tests and verify RED**

```sh
go test ./internal/sketch/warp -run 'Test(OctaveRotationChangesDirectionWithoutChangingBounds|NestedRolesUseDifferentSpatialScales)' -count=1
```

Expected: FAIL because rotated evaluation, role scales, and material height do
not exist.

- [ ] **Step 3: Implement rotated octave stepping**

Keep the evaluator concrete in `warp.go`. Use a fixed angle not copied from the
reference and update coordinates after every octave:

```go
func rotateScale(x, y, frequency float64) (float64, float64) {
	const c = 0.8191520442889918 // cos(35 degrees)
	const s = 0.5735764363510460 // sin(35 degrees)
	return (c*x - s*y) * frequency, (s*x + c*y) * frequency
}
```

The exact angle may be adjusted once during visual calibration, but stays an
internal named constant. Refactor `fbm` to rotate and scale the running domain
between octaves while retaining gain normalization and zero allocations.

- [ ] **Step 4: Separate field-role scales minimally**

Keep `q` at the current base scale. Evaluate `r` and final detail at fixed
sketch-local ratios near, but not equal to, one. Preserve the current activity
envelope and broad warped coordinate so preferred seeds retain recognizable
composition. Extend `fieldSample` with `raw` or `height` only as needed by the
next task; do not expose a public field type.

- [ ] **Step 5: Run focused and existing package tests**

```sh
go test ./internal/sketch/warp -run 'Test(OctaveRotationChangesDirectionWithoutChangingBounds|NestedRolesUseDifferentSpatialScales|FieldSamplesStayFinite|ModesAreDistinct|ModesLeaveUnusedFieldsZero)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run the repository gate and commit**

```sh
make check
git add internal/sketch/warp/warp.go internal/sketch/warp/warp_test.go
git commit -m "feat: refine nested warp field detail"
```

### Task 2: Build A Continuous Folded Material Signal

**Files:**
- Modify: `internal/sketch/warp/warp.go`
- Modify: `internal/sketch/warp/warp_test.go`

- [ ] **Step 1: Write failing material-signal tests**

Add a table over fixed seeds `1,2,3,5,8,13,21,34`, all modes, and a coordinate
grid. Assert raw and shaped heights are finite and bounded. Add distribution
claims over nested samples:

```go
func TestMaterialHeightCreatesCavitiesBodiesAndRidges(t *testing.T) {
	// Gather a dense deterministic sample grid over seeds 1,2,5,8.
	// Require nontrivial low, middle, and high quantile separation rather than
	// exact percentages, proving the mapping is not flat or mostly clipped.
}

func TestQuietActivitySuppressesFineRidges(t *testing.T) {
	// Find low- and high-activity samples on fixed grids and compare the local
	// high-frequency/ridge contribution. High activity must carry more detail.
}
```

- [ ] **Step 2: Run tests and verify RED**

```sh
go test ./internal/sketch/warp -run 'Test(MaterialHeightCreatesCavitiesBodiesAndRidges|QuietActivitySuppressesFineRidges|MaterialSamplesStayFinite)' -count=1
```

Expected: FAIL because the current scalar has no separate material shaping.

- [ ] **Step 3: Implement original continuous shaping**

Implement a small `material(raw, fine, activity) materialSample` function in
`warp.go`. It should:

- map raw field asymmetrically so low values open into cavities;
- derive a narrow smooth ridge response from a folded high-frequency residual;
- multiply ridge strength by eased activity;
- combine and clamp to a continuous `[0,1]` height/tone;
- retain the raw/fine terms only if appearance or tests genuinely use them.

Use `mathx.Smoothstep`, `math.Abs`, and simple powers. Do not transcribe the
reference's nonlinear expression or periodic grid term.

- [ ] **Step 4: Run material and regression tests**

```sh
go test ./internal/sketch/warp -run 'Test(MaterialHeightCreatesCavitiesBodiesAndRidges|QuietActivitySuppressesFineRidges|MaterialSamplesStayFinite|SamplingIsPure|PlanIgnoresPixelDimensions)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run the repository gate and commit**

```sh
make check
git add internal/sketch/warp
git commit -m "feat: shape warp as folded material"
```

### Task 3: Add Field-Normal Lighting And Folded Palette Roles

**Files:**
- Modify: `internal/sketch/warp/warp.go`
- Modify: `internal/sketch/warp/options.go`
- Modify: `internal/sketch/warp/warp_test.go`

- [ ] **Step 1: Write failing normal and lighting tests**

Define private vector/lighting helpers through tests of the required behavior:

```go
func TestMaterialNormalsAreFiniteAndNormalized(t *testing.T) {
	// All fixed seeds/modes, representative coordinate grid. Require finite
	// components and length approximately one.
}

func TestLightOrdersOppositeSlopes(t *testing.T) {
	// Feed synthetic normalized normals facing toward, flat to, and away from
	// the fixed light into the pure light-factor helper. Require toward > flat >
	// away and a nonzero ambient floor.
}

func TestNormalStepUsesCanvasUnits(t *testing.T) {
	// Equal-aspect contexts of different pixel dimensions produce identical
	// normals and folded colors at identical normalized coordinates.
}
```

Add a color test proving shading a mid color preserves finite `[0,1]` channels
and changes linear luminance monotonically with light factor.

- [ ] **Step 2: Run focused tests and verify RED**

```sh
go test ./internal/sketch/warp -run 'Test(MaterialNormalsAreFiniteAndNormalized|LightOrdersOppositeSlopes|NormalStepUsesCanvasUnits|FoldedShadingPreservesColorBounds)' -count=1
```

Expected: FAIL because normal and folded shading helpers do not exist.

- [ ] **Step 3: Implement canvas-unit central differences**

Add a fixed `normalEpsilon` in canvas units and a `materialHeight(u,v)` pure
query. Compute central differences and normalize `(-depth*dx, -depth*dy,
2*epsilon)`. Avoid allocations and do not derive epsilon from image dimensions.

- [ ] **Step 4: Implement linear-light fixed illumination**

Create a private fixed normalized oblique light direction and a pure helper
returning ambient-plus-diffuse light. Convert each palette channel with
`palette.SRGBToLinear`, multiply by the bounded light factor, then convert back
with `palette.LinearToSRGB`. A small broad ridge highlight may mix toward the
selected pale ridge color, but no second light, vignette, or post-processing
layer is allowed.

- [ ] **Step 5: Implement folded palette roles**

At plan construction, sort palette colors by luminance and assign dark cavity,
middle body, and pale ridge roles. Build at most two smooth HSL ramps. Folded
appearance maps material height into those roles, optionally using one bounded
nested component for sparse palette-derived temperature variation. Lighting is
applied after base material color.

Replace `appearanceStructure` and CLI `structure` with `appearanceFolded` and
`folded`; set folded as the default. Keep `gradient` as the unlit diagnostic.
Do not retain aliases or backwards compatibility because the experiment has
not been integrated and no persisted external behavior requires it.

- [ ] **Step 6: Run focused, package, and allocation tests**

```sh
go test ./internal/sketch/warp -count=1
go test ./internal/sketch/warp -run '^$' -bench 'Benchmark(Sample|Render)' -benchmem
```

Expected: tests PASS and direct sampling reports `0 B/op`, `0 allocs/op`.

- [ ] **Step 7: Run the repository gate and commit**

```sh
make check
git add internal/sketch/warp
git commit -m "feat: light folded warp material"
```

### Task 4: Update Spec And Establish Candidate Golden

**Files:**
- Modify: `docs/sketches/013-warp.md`
- Modify: `internal/sketch/warp/warp_test.go`
- Modify: `internal/sketch/warp/testdata/warp_seed13_64.png`

- [ ] **Step 1: Update the artwork specification**

Document rotated octaves, role-specific scales, continuous cavity/ridge
shaping, `gradient|folded`, palette roles, canvas-unit normals, linear-light
shading, and the expected performance trade-off. Replace obsolete structure
appearance text and add acceptance checks from the approved design.

- [ ] **Step 2: Make the old golden fail deliberately**

```sh
go test ./internal/sketch/warp -run TestGolden -count=1
```

Expected: FAIL with a pixel mismatch because the artwork intentionally changed.

- [ ] **Step 3: Generate and inspect the candidate golden**

```sh
go test ./internal/sketch/warp -run TestGolden -update -count=1
```

Read `internal/sketch/warp/testdata/warp_seed13_64.png`. Retain it only if dark
cavities and pale ridges remain visible at 64 pixels without turning into hard
terraces or an embossed outline filter.

- [ ] **Step 4: Run tests and commit**

```sh
make check
git add docs/sketches/013-warp.md internal/sketch/warp/warp_test.go internal/sketch/warp/testdata/warp_seed13_64.png
git commit -m "docs: specify folded warp material"
```

### Task 5: Calibrate The Visual Space

**Files:**
- Modify: `internal/sketch/warp/warp.go` only for justified internal-constant tuning
- Modify: `docs/sketches/013-warp.md` if final behavior changes
- Modify: golden after intentional visual changes
- Generate ignored files: `out/warp-nested-fbm/folded-depth/**`

- [ ] **Step 1: Render the fixed-seed folded candidate**

```sh
go run ./cmd/staticart sweep warp --seeds 1,2,3,5,8,13,21,34 \
  --warp nested --appearance folded --profile preview \
  --out out/warp-nested-fbm/folded-depth/seeds
```

Read the sheet and full previews for seeds 1, 2, 5, and 8 first, then inspect
the strongest and weakest remaining seeds.

- [ ] **Step 2: Render diagnostic appearance comparison**

```sh
go run ./cmd/staticart sweep warp --seeds 1,2,3,5,8,13,21,34 \
  --warp nested --vary appearance=gradient,folded --profile preview \
  --out out/warp-nested-fbm/folded-depth/appearances
```

Folded must add depth visible in value, not merely stronger saturation.

- [ ] **Step 3: Render mode comparison**

```sh
go run ./cmd/staticart sweep warp --seeds 2 \
  --appearance folded --vary warp=plain,single,nested --profile preview \
  --out out/warp-nested-fbm/folded-depth/modes
```

Confirm nested mode adds finer organized folds while plain and single remain
legible development baselines.

- [ ] **Step 4: Run at most two small internal-constant sweeps**

If depth is weak or emboss-like, temporarily calibrate only named internal
constants for material depth and ridge strength. Use two or three values each,
render fixed seeds 1, 2, 5, and 8, and retain only the selected constants in
code. Do not expose flags solely for calibration and do not add another effect.

- [ ] **Step 5: Re-render final sheets and golden after tuning**

Repeat Steps 1-3 exactly. Regenerate and read the golden if tracked output
changed. Record which candidate was chosen and why.

- [ ] **Step 6: Run the repository gate and commit visual tuning**

```sh
make check
git add internal/sketch/warp docs/sketches/013-warp.md
git commit -m "art: tune folded warp depth"
```

Skip this commit if no tracked tuning was needed.

### Task 6: Report And Stop For Visual Review

**Files:**
- Add: `experiments/warp-nested-fbm/folded-depth-result.md`

- [ ] **Step 1: Measure candidate preview cost**

```sh
/usr/bin/time -p go run ./cmd/staticart render warp --seed 2 --warp nested \
  --appearance folded --profile preview \
  --out out/warp-nested-fbm/folded-depth/strongest
```

Record wall time as an iteration observation and compare it with the prior
`0.64s` observation without claiming a controlled benchmark.

- [ ] **Step 2: Run final verification**

```sh
make check
git diff --check master...HEAD
```

Expected: all gates PASS and diff check has no output.

- [ ] **Step 3: Write the follow-up result report**

Record exact verification commands, commit hashes, artifact paths, render cost,
baseline/candidate differences, visual gains/regressions, preferred seeds 1,
2, 5, and 8, strongest/weakest candidate, whether depth survives
desaturation, and a recommendation. Explicitly state that the implementation
uses independent constants/formulas and does not copy the supplied shader.

- [ ] **Step 4: Commit the report**

```sh
git add experiments/warp-nested-fbm/folded-depth-result.md
git commit -m "docs: report folded warp depth"
```

- [ ] **Step 5: Confirm clean preserved experiment**

```sh
git status --short --branch
git log --oneline master..HEAD
```

Expected: clean `exp/warp-nested-fbm` with ignored candidate imagery retained.
Do not merge, cherry-pick, remove the worktree, or delete the branch.
