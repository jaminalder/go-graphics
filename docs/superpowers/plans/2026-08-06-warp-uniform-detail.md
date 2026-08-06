# Warp Uniform Detail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a default uniform-detail mode that keeps nested warp turbulence and sharp ridges across the whole canvas while preserving the current activity-varying render as `--detail varied`.

**Architecture:** Resolve one private detail enum into immutable plan settings. Uniform mode returns a fixed high activity for displacement and material shaping; varied mode executes the existing envelope formula unchanged. All field, material, normal, palette, and raster mechanisms remain sketch-local and otherwise intact.

**Tech Stack:** Go standard library, existing `internal/noise`, `internal/opt`, `internal/palette`, `internal/sketch`, sweep/contact-sheet CLI, 2000px PNG inspection.

---

### Task 1: Add And Pin The Detail Axis

**Files:**
- Modify: `internal/sketch/warp/warp.go`
- Modify: `internal/sketch/warp/options.go`
- Modify: `internal/sketch/warp/warp_test.go`

- [ ] **Step 1: Preserve the current varied render before changing defaults**

Add a test fixture hash or exact pixel comparison around a 64x64 folded render
for seed 13 with the current defaults. The test should identify this as the
future `varied` baseline and initially call the current render path.

- [ ] **Step 2: Write failing choice and activity tests**

Add tests asserting the required behavior:

```go
func TestUniformDetailIsDefault(t *testing.T) {}
func TestVariedDetailUsesExistingActivityEnvelope(t *testing.T) {}
func TestUniformDetailUsesOneHighActivityEverywhere(t *testing.T) {}
func TestDetailOptionChangesOnlyDetailSetting(t *testing.T) {}
```

For varied mode, duplicate the old formula only in the test and compare exact
activity values at representative coordinates. For uniform, sample a grid and
require one constant activity value high enough to fully enable ridges.

- [ ] **Step 3: Run tests and verify RED**

```sh
go test ./internal/sketch/warp -run 'Test(UniformDetailIsDefault|VariedDetailUsesExistingActivityEnvelope|UniformDetailUsesOneHighActivityEverywhere|DetailOptionChangesOnlyDetailSetting)' -count=1
```

Expected: FAIL because the detail axis does not exist.

- [ ] **Step 4: Implement minimal detail policy**

Add private `detail` values `detailUniform` and `detailVaried`, include one in
`settings`, set `uniform` as `New()` default, and declare:

```go
o.Choice("detail", "distribution of nested field detail", "dt",
	[]string{"uniform", "varied"}, &s.detailName,
	func(value int) { s.detail = detail(value) })
```

Move activity resolution into `plan.activityAt(u,v)`. Varied returns the exact
old expression. Uniform returns one named internal constant. Use it for warp
strength and ridge shaping. Do not add a numeric flag.

- [ ] **Step 5: Prove varied byte preservation**

Update the baseline test to render `--detail varied` and require exact equality
with the captured current pixels/hash. If it differs, fix the implementation;
do not update the baseline.

- [ ] **Step 6: Add a distribution test**

On fixed seeds and grid cells, calculate local variation of `fine` or `ridge`
energy. Require uniform mode to have a high mean and materially lower
between-cell coefficient of variation than varied mode. Keep thresholds broad
enough to specify the visual policy without pinning incidental noise values.

- [ ] **Step 7: Run focused, package, and benchmark checks**

```sh
go test ./internal/sketch/warp -count=1
go test ./internal/sketch/warp -run '^$' -bench BenchmarkAt -benchmem
```

Expected: PASS and `0 B/op`, `0 allocs/op`.

- [ ] **Step 8: Run full gate and commit**

```sh
make check
git add internal/sketch/warp
git commit -m "feat: add uniform warp detail"
```

### Task 2: Update Specification And Golden

**Files:**
- Modify: `docs/sketches/013-warp.md`
- Modify: `internal/sketch/warp/testdata/warp_seed13_64.png`

- [ ] **Step 1: Document detail behavior**

Describe `uniform|varied`, uniform default, exact varied preservation, and that
uniform removes focus variation rather than applying post-process sharpening.
Update acceptance criteria to distinguish uniformly turbulent and varied
calm/turbulent expressions.

- [ ] **Step 2: Verify old default golden fails**

```sh
go test ./internal/sketch/warp -run TestGolden -count=1
```

Expected: FAIL because the default is intentionally now uniform.

- [ ] **Step 3: Regenerate and inspect default golden**

```sh
go test ./internal/sketch/warp -run TestGolden -update -count=1
```

Read the PNG. Confirm detail spans the frame and remains continuous rather than
pixel-noisy or uniformly embossed.

- [ ] **Step 4: Run gate and commit**

```sh
make check
git add docs/sketches/013-warp.md internal/sketch/warp/testdata/warp_seed13_64.png
git commit -m "docs: specify uniform warp detail"
```

### Task 3: Preview-Calibrate Uniform Sharpness

**Files:**
- Modify: `internal/sketch/warp/warp.go` only if one uniform activity constant needs calibration
- Modify: golden/spec after intentional tracked changes
- Generate ignored files: `out/warp-nested-fbm/uniform-detail/**`

- [ ] **Step 1: Render fixed-seed detail comparison**

```sh
go run ./cmd/staticart sweep warp --seeds 1,2,3,5,8,13,21,34 \
  --appearance folded --vary detail=uniform,varied --profile preview \
  --out out/warp-nested-fbm/uniform-detail/comparison
```

Read the sheet and full uniform previews for seeds 1, 5, and 8.

- [ ] **Step 2: Make at most one constant calibration pass**

If uniform is still locally soft or becomes indiscriminate noise, adjust only
the fixed uniform activity constant and rerender the exact comparison. Do not
change varied behavior, field scales, lighting, palette mapping, or controls.

- [ ] **Step 3: Regenerate final preview artifacts and golden if needed**

Repeat Step 1 and update/read the golden after an intentional constant change.

- [ ] **Step 4: Run gate and commit tuning if tracked behavior changed**

```sh
make check
git add internal/sketch/warp docs/sketches/013-warp.md
git commit -m "art: tune uniform warp detail"
```

Skip the commit when no calibration was required.

### Task 4: Render 2000-Pixel Palette Examples

**Files:**
- Generate ignored files: `out/warp-nested-fbm/uniform-detail/web-palettes/**`

- [ ] **Step 1: Render the twelve web-size originals**

Use the sweep cartesian product:

```sh
go run ./cmd/staticart sweep warp --seeds 1,5,8 \
  --vary palette=hokusai-great-wave,okeeffe-abstraction-blue,durer-turf,rembrandt-night-watch \
  --detail uniform --appearance folded --profile web --cell 500 --jobs 2 \
  --out out/warp-nested-fbm/uniform-detail/web-palettes
```

Expected: twelve 2000x2000 PNGs, manifest, and contact sheet. If sweep does not
forward `palette` through `--vary`, run four sweeps with explicit `--palette`
into subdirectories and create comparison sheets using existing CLI tooling;
do not build a new tool.

- [ ] **Step 2: Inspect contact sheet and full originals**

Read the sheet, then at least one 2000px original per palette and every seed's
strongest palette. Check edge sharpness at full resolution, value-led depth,
palette role suitability, clipping, and whether any broad region remains soft.

- [ ] **Step 3: Identify strongest/weakest combinations**

Record seed/palette pairs and concrete visual reasons. Preserve every generated
original for human review under ignored `out/`.

### Task 5: Report, Review, And Preserve

**Files:**
- Add: `experiments/warp-nested-fbm/uniform-detail-result.md`

- [ ] **Step 1: Write exact result report**

Record TDD RED/GREEN evidence, varied hash preservation, uniform calibration,
commands, artifacts, 2000px findings by palette, strongest/weakest pairs,
performance, regressions, and recommendation.

- [ ] **Step 2: Run final verification**

```sh
make check
go test ./internal/sketch/warp -run '^$' -bench BenchmarkAt -benchmem
git diff --check master...HEAD
```

- [ ] **Step 3: Commit report**

```sh
git add experiments/warp-nested-fbm/uniform-detail-result.md
git commit -m "docs: report uniform warp detail"
```

- [ ] **Step 4: Confirm clean preserved worktree**

```sh
git status --short --branch
git log --oneline master..HEAD
```

Do not merge, push, remove the worktree, or delete the branch.
