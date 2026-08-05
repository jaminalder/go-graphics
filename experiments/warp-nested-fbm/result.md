# Experiment result

- Name: `warp-nested-fbm`
- Branch: `exp/warp-nested-fbm`
- Base commit: `80ad6d7`
- Profile: `preview`
- Fixed seeds: `1,2,3,5,8,13,21,34`
- Status: `DONE_WITH_CONCERNS`

## Summary

The experiment produced a registered `warp` sketch with plain fBM, one domain
warp, nested domain warping, and gradient/structure appearances. The nested
mode is visibly distinct from both shallower modes and typical seeds combine
broad quiet areas with concentrated folded detail. One focused tuning pass
lowered the base scale, increased displacement, and widened the activity range;
this improved separation between calm and turbulent regions.

The technical hypothesis is supported. The artistic hypothesis is supported
with concerns: several seeds read as directional marbling, but seed 3 remains
mostly cloud-like, and structure appearance usually sharpens local separation
rather than exposing a substantially new organization.

Read-only reviews after the initial report prompted robustness and performance
fixes, not another artistic pass. Ranged float options now reject non-finite
values generically, and shallower warp modes no longer evaluate fields that
cannot affect their output. Field constants, palette mapping, the golden, and
all visual artifacts remain unchanged from the one permitted tuning pass.

## Verification

- Baseline `go test ./...`: PASS before implementation.
- TDD field red run: `go test ./internal/sketch/warp -run 'Test(FieldSamplesStayFinite|SamplingIsPure|ModesAreDistinct)' -count=1` failed because the package implementation was absent; the same command passed after the immutable field plan was added.
- TDD appearance/options red run: `go test ./internal/sketch/warp -run 'Test(RenderIsDeterministic|PlanIgnoresPixelDimensions|BothAppearancesProduceFiniteColors|Options|RejectsTooSmallPalette)' -count=1` failed on the missing sketch, options, render, appearance, and palette contracts; it passed after implementation.
- Golden red run: `go test ./cmd/staticart ./internal/sketch/warp -run 'Test(Golden|List|Registry)' -count=1` failed because `testdata/warp_seed13_64.png` did not exist.
- Golden generation: `go test ./internal/sketch/warp -run TestGolden -update -count=1` passed; the generated PNG was read and retained after confirming folded broad movement. It was regenerated and read again after the one tuning pass.
- Review TDD red run: `go test ./internal/opt -run TestNonFiniteFloatsAreRejected -count=1` failed because `--alpha NaN` was accepted. `+Inf` and `-Inf` were already rejected by range comparisons, but explicit finite validation now rejects all three.
- Review TDD green run: `go test ./internal/opt -run 'Test(NonFiniteFloatsAreRejected|OutOfRangeIsRejected|SetKnobsApplyAndName|NegativeRangesAreExpressible)' -count=1` PASS.
- Lazy-mode TDD red run: `go test ./internal/sketch/warp -run 'Test(ModesLeaveUnusedFieldsZero|ModesAreDistinct|FieldSamplesStayFinite)' -count=1` failed because plain populated activity/q/r and single populated r.
- Lazy-mode TDD green run: `go test ./internal/sketch/warp -run 'Test(ModesLeaveUnusedFieldsZero|ModesAreDistinct|FieldSamplesStayFinite|Golden)' -count=1` PASS; the nested golden remained unchanged.
- Expanded matrix: finite in-range colors pass for all eight fixed seeds, all three modes, both appearances, and a 9x9 coordinate grid.
- Resolved options: `TestOptionsAlterResolvedSettings` proves all six numeric controls and both choices propagate into immutable plan settings.
- Review affected packages: `go test ./internal/opt ./internal/sketch/warp -count=1` PASS.
- Package tests: `go test ./internal/sketch/warp -count=1` PASS.
- Command/package tests: `go test ./cmd/staticart ./internal/sketch/warp -count=1` PASS.
- Registry: `go run ./cmd/staticart list` includes `warp` with `flowing organic fields from nested fBM domain warps`.
- Sampling benchmark after tuning: `BenchmarkSample-10 5448380 219.7 ns/op 0 B/op 0 allocs/op` on Apple M1 Pro.
- Sampling benchmark after review fixes: `BenchmarkSample-10 5366755 221.6 ns/op 0 B/op 0 allocs/op` on Apple M1 Pro. Nested-mode benchmark behavior remains allocation-free; lazy evaluation reduces work only in plain and single modes.
- `make check` passed before every worker commit: formatter, vet, golangci-lint (`0 issues`), and `go test ./...` all passed.
- Final pre-report `make check`: PASS with the same four gates.
- `git diff --check master...HEAD`: PASS with no output before this report.

## Commits

- `5c037bb` `docs: define nested warp experiment` (pre-existing experiment brief commit)
- `6f42c45` `docs: plan nested warp implementation` (pre-existing design and plan commit)
- `4d7628e` `docs: specify nested warp sketch`
- `1a51a76` `feat: add nested warp scalar field`
- `2d78058` `feat: render nested warp artwork`
- `461b85d` `feat: register warp sketch`
- `86fa56b` `art: tune nested warp movement`
- `1d0481c` `docs: report nested warp experiment`
- `db09a8b` `fix: harden warp field evaluation` (review-driven robustness, lazy evaluation, tests, and inventory corrections)
- Final review report commit follows this list: `docs: update warp review results`.

## Artifacts

- Golden: `internal/sketch/warp/testdata/warp_seed13_64.png`
- Eight-seed nested contact sheet: `out/warp-nested-fbm/seeds/sheet.png`
- Eight-seed manifest and full previews: `out/warp-nested-fbm/seeds/manifest.txt` and numbered PNGs in that directory
- Three-mode seed-13 comparison: `out/warp-nested-fbm/modes/sheet.png`
- Three-mode manifest and full previews: `out/warp-nested-fbm/modes/manifest.txt` and numbered PNGs in that directory
- Two-appearance, eight-seed comparison: `out/warp-nested-fbm/appearances/sheet.png`
- Appearance manifest and full previews: `out/warp-nested-fbm/appearances/manifest.txt` and numbered PNGs in that directory
- Timed strongest render: `out/warp-nested-fbm/strongest/warp-wnested_kandinsky-soft-pressure_2_600x600.png`

The timed strongest render command was:

```sh
/usr/bin/time -p go run ./cmd/staticart render warp --seed 2 --warp nested \
  --profile preview --out out/warp-nested-fbm/strongest
```

Observed wall time was `real 0.64s` (`user 0.94s`, `sys 0.17s`). This includes
`go run` startup/compilation and is an iteration observation, not a formal
render benchmark.

## Visual Consequences

- Nested mode replaces plain fBM's soft isolated masses with folded channels,
  pinched boundaries, and directional marbling.
- The activity envelope creates real scale separation in seeds 2, 5, 8, and
  34: broad calm fields meet concentrated turbulent zones.
- Five fBM components preserve fine detail inside folds while the lower base
  scale leaves large forms legible at contact-sheet size.
- The palette ramp stays coherent and uses only luminance-ordered colors from
  the selected artist palette. It is vivid but not rainbow-generated.
- Structure appearance remains in the same palette family and exposes q/r
  influence as local tonal shifts. It is informative in busy boundaries but
  often only modestly different from gradient appearance.
- Fixed seeds vary meaningfully while retaining one field-and-ramp identity.

## Strongest Seed

Seed 2 is strongest. Its left and upper areas contain nested, directional folds
and bright narrow currents, while the lower-right half opens into a broad quiet
passage. Large movement reads first, and detail survives at multiple scales
without filling the whole image uniformly.

- Gradient preview: `out/warp-nested-fbm/seeds/02_warp-wnested_kandinsky-soft-pressure_2_600x600.png`
- Structure preview: `out/warp-nested-fbm/appearances/04_warp-wnested-apstructure_kandinsky-soft-pressure_2_600x600.png`

## Weakest Seed

Seed 3 is weakest. Most of the sheet is a broad soft violet mass with isolated
cyan patches; folds are concentrated near the top-right edge rather than
organizing the composition. It demonstrates the intended quiet extreme, but it
falls closest to generic cloud output and has weak directional movement.

- Preview: `out/warp-nested-fbm/seeds/03_warp-wnested_kandinsky-soft-pressure_3_600x600.png`

## Keep

- Independent Perlin fields for q, r, final value, and activity.
- The broad activity envelope and tuned near-calm-to-turbulent range.
- Plain/single/nested as a direct development comparison.
- The immutable private plan and allocation-free point path.
- Gradient appearance as the default.
- Structure appearance as an explicit diagnostic/alternative, with its modest
  visual benefit documented rather than overstated.

## Reject

- Uniform activity or higher base scale; the first preview set was more evenly
  marbled and separated calm/turbulent passages less clearly.
- Additional marks, layers, field frameworks, or public controls.
- A second tuning pass aimed only at rescuing seed 3; that would overfit one
  outlier and violate the experiment limit.

## Recommendation

Recommend retaining the implementation as a successful self-contained field
experiment, but revising its artistic output space before treating it as a
finished sketch or integrating it on visual merit. Seed 3 does not meet the
directional-movement target, and structure appearance remains a subtle
alternative rather than a decisive analytical view. No further tuning belongs
in this experiment because its one permitted pass is already complete; a later
approved revision should start from these documented limitations. The user
should make the final artistic and integration decision.
