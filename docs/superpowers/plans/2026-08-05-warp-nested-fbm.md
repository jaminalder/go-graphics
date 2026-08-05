# Nested fBM Domain Warp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and visually evaluate a registered `warp` raster sketch with plain fBM, one-level domain warp, nested domain warp, and two restrained appearance mappings.

**Architecture:** A sketch-local immutable plan owns independently seeded Perlin fields and resolved fBM settings. Its pure sampler evaluates `q`, optional `r`, and a final scalar before a separate appearance function maps those values through palette-derived colors; `sketch.Raster` owns pixel traversal.

**Tech Stack:** Go standard library, `internal/noise`, `internal/opt`, `internal/gradient`, `internal/palette`, `internal/sketch`, existing CLI sweep/contact-sheet tooling.

---

## File Map

- Create `docs/sketches/013-warp.md`: artwork intent, algorithm, controls, and acceptance checklist.
- Create `internal/sketch/warp/warp.go`: sketch defaults, immutable plan, fBM and nested field evaluation, appearance mapping, rendering.
- Create `internal/sketch/warp/options.go`: declarative CLI choices and numeric bounds.
- Create `internal/sketch/warp/warp_test.go`: field finiteness, determinism, purity, resolution independence, option behavior, and golden.
- Create `internal/sketch/warp/testdata/warp_seed13_64.png`: visually inspected golden.
- Modify `cmd/staticart/main.go`: import and register `warp.New()` only.
- Modify affected `cmd/staticart` tests only if their expected registry listing is explicit.
- Modify `experiments/warp-nested-fbm/result.md`: verification, artifacts, visual findings, speed, commits, and recommendation.
- Keep all generated review imagery under ignored `out/warp-nested-fbm/`.

### Task 1: Write The Sketch Specification

**Files:**
- Create: `docs/sketches/013-warp.md`

- [ ] **Step 1: Write the artwork contract**

Document the purpose, three modes, equations, activity envelope, two appearances, public controls, deterministic seed ownership, and this acceptance checklist:

```markdown
## Acceptance checklist

- Large-scale directional movement reads before fine texture.
- Typical nested renders contain both calm and strongly folded passages.
- Plain, single, and nested modes are visibly distinct at one fixed seed.
- Detail exists at several scales without uniform agitation.
- Gradient colour is restrained; structure colour clarifies q/r organization.
- Fixed seeds vary meaningfully while retaining one visual identity.
- The hot path is pure, resolution-independent, and allocation-free in normal sampling.
```

- [ ] **Step 2: Check the spec against the experiment design**

Run:

```sh
git diff --check -- docs/sketches/013-warp.md
```

Expected: no output.

- [ ] **Step 3: Commit the specification**

```sh
git add docs/sketches/013-warp.md
git commit -m "docs: specify nested warp sketch"
```

### Task 2: Drive Scalar Field Evaluation With Tests

**Files:**
- Create: `internal/sketch/warp/warp_test.go`
- Create: `internal/sketch/warp/warp.go`

- [ ] **Step 1: Write failing finite-field and mode tests**

Create package-local tests that construct `New()`, build a plan for each of
`plain`, `single`, and `nested`, and sample a grid for seeds
`1,2,3,5,8,13,21,34`. Assert every `value`, `q.x`, `q.y`, `r.x`, `r.y`, and
`activity` is finite with `math.IsNaN` and `math.IsInf`. Also assert repeated
samples are exactly equal and that the three modes differ at at least one grid
point for seed 13.

Use this private result shape in the test:

```go
type fieldSample struct {
	value, qx, qy, rx, ry, activity float64
}
```

- [ ] **Step 2: Run the tests to verify the package is absent**

```sh
go test ./internal/sketch/warp -run 'Test(FieldSamplesStayFinite|SamplingIsPure|ModesAreDistinct)' -count=1
```

Expected: FAIL because `internal/sketch/warp` has no implementation.

- [ ] **Step 3: Implement the minimal immutable scalar plan**

In `warp.go`, define:

```go
type mode uint8

const (
	modePlain mode = iota
	modeSingle
	modeNested
)

type settings struct {
	scale, gain, lacunarity       float64
	warpStrength, nestedStrength  float64
	octaves                       int
	mode                          mode
}

type plan struct {
	set settings
	q       [2]*noise.Perlin
	r       [2]*noise.Perlin
	value   *noise.Perlin
	activity *noise.Perlin
}
```

Construct each field once from `ctx.Seed` plus unique fixed stream constants.
Implement `fbm(field, x, y)` as a loop of exactly `octaves` components, with
`frequency *= lacunarity`, `amplitude *= gain`, and normalization by the sum of
amplitudes. Implement `sample(u,v)` in this order:

```text
p = (u*scale, v*scale)
activity = smooth low-frequency field mapped to a bounded multiplier
q = two normalized fBM calls at p
r = two normalized fBM calls at p + q*warpStrength*activity
plain value  = final fBM(p)
single value = final fBM(p + q*warpStrength*activity)
nested value = final fBM(p + r*nestedStrength*activity)
```

Do not allocate slices, use RNGs, mutate state, or create fields in `sample`.

- [ ] **Step 4: Run focused tests until they pass**

```sh
go test ./internal/sketch/warp -run 'Test(FieldSamplesStayFinite|SamplingIsPure|ModesAreDistinct)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit scalar field behavior**

```sh
git add internal/sketch/warp/warp.go internal/sketch/warp/warp_test.go
git commit -m "feat: add nested warp scalar field"
```

### Task 3: Add Appearance, Options, And Raster Rendering

**Files:**
- Modify: `internal/sketch/warp/warp.go`
- Create: `internal/sketch/warp/options.go`
- Modify: `internal/sketch/warp/warp_test.go`

- [ ] **Step 1: Write failing appearance and validation tests**

Add tests asserting:

```go
func TestRenderIsDeterministic(t *testing.T) {
	sketchtest.AssertDeterministic(t, New(), testCtx(t, 13), testCtx(t, 21))
}

func TestPlanIgnoresPixelDimensions(t *testing.T) {
	// Build equal-aspect 96x64 and 960x640 contexts and compare field samples
	// and colors at the same normalized coordinates.
}

func TestBothAppearancesProduceFiniteColors(t *testing.T) {
	// For gradient and structure, assert each RGB component is finite and in [0,1].
}
```

Test flag parsing through a `flag.FlagSet`: valid numeric boundaries and both
choices pass `Configure`; invalid `--octaves 0`, excessive gain/lacunarity or
strengths, and unknown choices return errors. Test that a palette with fewer
than three colors makes `Render` return an error.

- [ ] **Step 2: Run tests to verify missing behavior fails**

```sh
go test ./internal/sketch/warp -run 'Test(RenderIsDeterministic|PlanIgnoresPixelDimensions|BothAppearancesProduceFiniteColors|Options|RejectsTooSmallPalette)' -count=1
```

Expected: FAIL because appearance, options, and rendering are incomplete.

- [ ] **Step 3: Implement appearance mapping and `Sketch`**

Add `Sketch` fields with defaults approximately centered on a useful preview:

```go
type Sketch struct {
	Scale, Gain, Lacunarity      float64
	WarpStrength, NestedStrength float64
	Octaves                      int
	warpName, appearanceName     string
	knobs                        *opt.Set
}
```

`New()` sets `warpName: "nested"` and `appearanceName: "gradient"`, calls
`declare`, and returns the sketch. Implement `Name`, `Describe`, `plan`, and
`Render`. Require three palette colors. Build a restrained HSL or cosine
gradient once in the plan.

Implement two pure mappings:

```text
gradient: contrast-shaped normalized final value -> ordered palette ramp
structure: final value supplies tone; bounded q/r components shift two
           palette mixes, with nested influence absent in shallower modes
```

Clamp all mapping inputs and final RGB values through existing palette/mathx
operations. Keep appearance code in `warp.go` unless it makes that file hard to
scan; do not create a shared abstraction.

- [ ] **Step 4: Declare exactly the requested CLI options**

In `options.go`, use `internal/opt`:

```go
o.Float("scale", "base field cycles per canvas unit", "sc", 0.25, 12, &s.Scale)
o.Int("octaves", "number of fBM components", "oc", 1, 8, &s.Octaves)
o.Float("gain", "amplitude multiplier per octave", "gn", 0.2, 0.85, &s.Gain)
o.Float("lacunarity", "frequency multiplier per octave", "la", 1.2, 3.5, &s.Lacunarity)
o.Float("warp-strength", "first domain displacement", "w1", 0, 8, &s.WarpStrength)
o.Float("nested-strength", "second domain displacement", "w2", 0, 8, &s.NestedStrength)
o.Choice("warp", "field development mode", "w", []string{"plain", "single", "nested"}, &s.warpName, applyMode)
o.Choice("appearance", "colour mapping", "ap", []string{"gradient", "structure"}, &s.appearanceName, applyAppearance)
```

Implement `Flags` and `Configure` as the same thin adapter used by existing
sketches. It is acceptable to adjust numeric upper bounds downward if focused
tests or preview cost justify it; document final bounds in the sketch spec.

- [ ] **Step 5: Run focused tests and an allocation benchmark**

```sh
go test ./internal/sketch/warp -count=1
go test ./internal/sketch/warp -run '^$' -bench BenchmarkSample -benchmem
```

Expected: tests PASS; `BenchmarkSample` reports `0 B/op` and `0 allocs/op`.
Add the benchmark to `warp_test.go` if it was not part of Step 1.

- [ ] **Step 6: Commit the complete sketch package**

```sh
git add internal/sketch/warp
git commit -m "feat: render nested warp artwork"
```

### Task 4: Register The Sketch And Pin Its Golden

**Files:**
- Modify: `cmd/staticart/main.go`
- Modify: affected `cmd/staticart/*_test.go` only if registry expectations fail
- Modify: `internal/sketch/warp/warp_test.go`
- Create: `internal/sketch/warp/testdata/warp_seed13_64.png`

- [ ] **Step 1: Write the failing golden and registry expectations**

Add:

```go
func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 13))
	sketchtest.Golden(t, got, "testdata/warp_seed13_64.png", *update)
}
```

Declare the package's `-update` flag consistently with existing sketch tests.
If command tests enumerate names, add `warp` to the expected sorted list.

- [ ] **Step 2: Run tests to verify registration/golden failure**

```sh
go test ./cmd/staticart ./internal/sketch/warp -run 'Test(Golden|List|Registry)' -count=1
```

Expected: FAIL because `warp` is not registered and the golden does not exist.

- [ ] **Step 3: Register only generic wiring**

Import `internal/sketch/warp` in `cmd/staticart/main.go` and add `warp.New()` to
`registry()`. Do not add any sketch-specific flags or logic to `cmd`.

- [ ] **Step 4: Generate and inspect the 64-pixel golden**

```sh
go test ./internal/sketch/warp -run TestGolden -update -count=1
```

Read `internal/sketch/warp/testdata/warp_seed13_64.png`. Confirm it contains
broad movement rather than only cloud texture before retaining it.

- [ ] **Step 5: Run package and command tests**

```sh
go test ./cmd/staticart ./internal/sketch/warp -count=1
go run ./cmd/staticart list
```

Expected: tests PASS; list includes `warp` with its description.

- [ ] **Step 6: Commit registration and golden**

```sh
git add cmd/staticart internal/sketch/warp/warp_test.go internal/sketch/warp/testdata/warp_seed13_64.png
git commit -m "feat: register warp sketch"
```

### Task 5: Render And Iterate On The Output Space

**Files:**
- Modify: `internal/sketch/warp/warp.go` only for visually justified constant/default tuning
- Modify: `docs/sketches/013-warp.md` when final behavior differs
- Modify: golden if intentional appearance changes occur
- Generate ignored files: `out/warp-nested-fbm/**`

- [ ] **Step 1: Render the eight fixed nested seeds**

```sh
go run ./cmd/staticart sweep warp --seeds 1,2,3,5,8,13,21,34 \
  --warp nested --profile preview --out out/warp-nested-fbm/seeds
```

Expected: eight PNGs, `manifest.txt`, and `sheet.png`.

- [ ] **Step 2: Read the contact sheet and representative individual PNGs**

Inspect `out/warp-nested-fbm/seeds/sheet.png`, then the apparent strongest and
weakest tiles at full preview size. Judge every acceptance item from the spec.

- [ ] **Step 3: Render the three-mode comparison**

Use the most representative fixed seed found in Step 2:

```sh
go run ./cmd/staticart sweep warp --seeds 13 \
  --vary warp=plain,single,nested --profile preview \
  --out out/warp-nested-fbm/modes
```

Expected: three tiles and a sheet that clearly shows increasing organization.

- [ ] **Step 4: Compare appearances**

```sh
go run ./cmd/staticart sweep warp --seeds 1,2,3,5,8,13,21,34 \
  --warp nested --vary appearance=gradient,structure --profile preview \
  --out out/warp-nested-fbm/appearances
```

Expected: a 16-tile sheet showing whether structure mode reveals q/r without
rainbow color or destroying value organization.

- [ ] **Step 5: Make at most one focused tuning pass if needed**

If typical seeds are uniformly cloudy, uniformly turbulent, or mode differences
are weak, alter only sketch-local defaults/constants for scale, activity
envelope, strengths, or contrast. Re-run Steps 1-4 with exactly the same seeds.
Do not add parameters or effects. Update the golden deliberately after reading
it if output changes.

- [ ] **Step 6: Commit visual tuning coherently**

```sh
git add internal/sketch/warp docs/sketches/013-warp.md
git commit -m "art: tune nested warp movement"
```

Skip this commit if no tracked tuning was needed.

### Task 6: Verify, Report, And Stop For Review

**Files:**
- Modify: `experiments/warp-nested-fbm/result.md`

- [ ] **Step 1: Measure a brief preview render observation**

```sh
time go run ./cmd/staticart render warp --seed 13 --warp nested \
  --profile preview --out out/warp-nested-fbm/strongest
```

Record elapsed time as an observation, not a formal benchmark claim.

- [ ] **Step 2: Run the final gate**

```sh
make check
```

Expected: formatter, vet, lint, and all tests PASS.

- [ ] **Step 3: Complete the result report**

Replace every provisional section in `experiments/warp-nested-fbm/result.md`
with exact commands/outcomes, contact-sheet paths, strongest and weakest seed
numbers with visual reasons, render-speed observation, visible gains/failures,
behavior to keep/reject, and a recommendation. Include all commit hashes that
already exist; note that the final report commit follows.

- [ ] **Step 4: Commit the result report**

```sh
git add experiments/warp-nested-fbm/result.md
git commit -m "docs: report nested warp experiment"
```

- [ ] **Step 5: Confirm branch and worktree state**

```sh
git status --short --branch
git log --oneline master..HEAD
```

Expected: `## exp/warp-nested-fbm`, no modified or untracked tracked files, and
the coherent experiment commits listed. Leave ignored `out/` renders in place.
Do not merge, cherry-pick, remove the worktree, or delete the branch.
