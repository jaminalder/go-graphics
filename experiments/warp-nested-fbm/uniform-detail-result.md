# Uniform detail follow-up result

- Name: `warp-uniform-detail`
- Parent experiment: `warp-nested-fbm`
- Branch: `exp/warp-nested-fbm`
- Fixed review seeds: `1,5,8`
- Status: `DONE`

## Summary And Diagnosis

The apparent blur in the folded artwork was field policy, not raster resolution
or filtering. The existing low-frequency activity envelope reduced both nested
domain displacement and fine-ridge shaping in the same broad regions. Those
regions were therefore mathematically smooth; a 2000px render could not reveal
detail that the sampler had removed.

`--detail uniform|varied` now makes that policy explicit. `uniform` is the
default and returns one fixed high activity for displacement and ridge shaping
at every coordinate. `varied` executes the previous envelope expression
unchanged. There is no numeric sharpness control, post-process sharpening,
explicit line layer, shared framework, or change to field scales, light,
palette mapping, or rasterization.

The uniform result keeps organized nested folds across all reviewed frames.
Variation now comes from form, value, and color rather than locally changing
focus. The previous calm-versus-turbulent folded artwork remains available
exactly as `--detail varied`.

## TDD And Preservation Evidence

The old behavior was pinned before any production or default change. A first
fixture run against the untouched default intentionally failed:

```text
current folded baseline hash
952ae7ef551db600e4c25bfe5dfc49c30afc63e34588254852157c4394c9cd46,
want capture-current-default
```

The hash is SHA-256 over raw 64x64 NRGBA pixels for seed 13, not encoded PNG
bytes. After fixing that value as the baseline, the same untouched render
passed. Only then were the new detail tests written.

- Detail-axis RED:
  `go test ./internal/sketch/warp -run 'Test(UniformDetailIsDefault|VariedDetailUsesExistingActivityEnvelope|UniformDetailUsesOneHighActivityEverywhere|DetailOptionChangesOnlyDetailSetting)' -count=1`
  failed to compile because `settings.detail`, `detailUniform`,
  `detailVaried`, and `plan.activityAt` did not exist.
- Detail-axis GREEN: the same tests passed after adding the private policy,
  sketch-owned choice, and constant uniform activity.
- Varied preservation GREEN:
  `TestVariedDetailPreservesFoldedBaseline` explicitly configures
  `--detail varied` and still produces raw-pixel SHA-256
  `952ae7ef551db600e4c25bfe5dfc49c30afc63e34588254852157c4394c9cd46`.
  The fixture was not updated after the default changed.
- Exact envelope evidence: `TestVariedDetailUsesExistingActivityEnvelope`
  duplicates the preceding expression only in the test and requires exact
  floating-point equality at representative coordinates.
- Uniform policy evidence: `TestUniformDetailUsesOneHighActivityEverywhere`
  requires one high value over a 9x9 grid.
  `TestUniformDetailDoesNotSampleActivityField` sets the varied-only Perlin
  pointer to nil and successfully samples uniform plans, proving the repeated
  uniform path does not query that field.
- Distribution evidence:
  `TestUniformDetailDistributesFineEnergyAcrossCanvas` measures mean ridge
  energy and between-cell coefficient of variation over seeds 1, 5, and 8.
  Uniform must exceed varied mean by at least 15 percent and remain below 85
  percent of varied's coefficient of variation; it passes.
- Option isolation: complete resolved-setting snapshots prove `--detail
  varied` changes only detail. Public option acceptance and invalid choice
  rejection include both new values.
- Deliberate golden RED:
  `go test ./internal/sketch/warp -run TestGolden -count=1` failed with the
  expected pixel mismatch after uniform became the default.
- Golden GREEN:
  `go test ./internal/sketch/warp -run TestGolden -update -count=1` regenerated
  `internal/sketch/warp/testdata/warp_seed13_64.png`. It was read directly;
  full-frame detail was present. The 600px sweep was then used to judge
  organization at a meaningful visual size.
- Final package check before the implementation commit:
  `go test ./internal/sketch/warp -count=1` passed.

The implementation, sketch specification, tests, and deliberate golden change
were committed together because the repository requires a passing `make check`
before every commit; splitting them would have committed a stale failing
golden.

## Uniform Constant And Calibration

The retained private constant is `uniformActivity = 1.28`. It is high enough to
fully enable ridge shaping and lies within the high end of the existing varied
envelope's range.

The initial candidate was reviewed with:

```sh
go run ./cmd/staticart sweep warp --seeds 1,2,3,5,8,13,21,34 \
  --appearance folded --vary detail=uniform,varied --profile preview \
  --out out/warp-nested-fbm/uniform-detail/comparison
```

Its sheet and the full 600px uniform originals for seeds 1, 5, and 8 were read
directly. Uniform removed broad soft zones while preserving nested, flowing
forms rather than pixel noise or a disconnected embossed texture. No
calibration pass was needed, so the allowed activity-only calibration count was
zero.

## Preview Artifacts

- Comparison sheet:
  `out/warp-nested-fbm/uniform-detail/comparison/sheet.png`
- Manifest:
  `out/warp-nested-fbm/uniform-detail/comparison/manifest.txt`
- Full uniform seed 1 preview:
  `out/warp-nested-fbm/uniform-detail/comparison/01_warp-apfolded-dtuniform_kandinsky-soft-pressure_1_600x600.png`
- Full uniform seed 5 preview:
  `out/warp-nested-fbm/uniform-detail/comparison/07_warp-apfolded-dtuniform_kandinsky-soft-pressure_5_600x600.png`
- Full uniform seed 8 preview:
  `out/warp-nested-fbm/uniform-detail/comparison/09_warp-apfolded-dtuniform_kandinsky-soft-pressure_8_600x600.png`

The comparison sheet makes the policy change clear: each odd-numbered uniform
tile carries folds across the whole frame, while the adjacent varied tile keeps
the preceding broad calm and active regions. All prior artifact directories
remain untouched; every new render is below `uniform-detail/`.

## 2000px Palette Review

The requested generic palette sweep worked without fallback:

```sh
go run ./cmd/staticart sweep warp --seeds 1,5,8 \
  --vary palette=hokusai-great-wave,okeeffe-abstraction-blue,durer-turf,rembrandt-night-watch \
  --detail uniform --appearance folded --profile web --cell 500 --jobs 2 \
  --out out/warp-nested-fbm/uniform-detail/web-palettes
```

`sips -g pixelWidth -g pixelHeight` reported `2000` by `2000` for every
numbered original. The sheet, all twelve tiles through the contact sheet, and
four full-size originals were inspected directly. The full-size set covered
one original per palette and the selected strongest palette for each seed.
There are no broad defocused regions, edge drop-offs, or visible channel
clipping in the reviewed originals.

Contact sheet and manifest:

- `out/warp-nested-fbm/uniform-detail/web-palettes/sheet.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/manifest.txt`

Exact 2000x2000 originals:

- `out/warp-nested-fbm/uniform-detail/web-palettes/01_warp-apfolded-dtuniform_hokusai-great-wave_1_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/02_warp-apfolded-dtuniform_okeeffe-abstraction-blue_1_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/03_warp-apfolded-dtuniform_durer-turf_1_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/04_warp-apfolded-dtuniform_rembrandt-night-watch_1_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/05_warp-apfolded-dtuniform_hokusai-great-wave_5_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/06_warp-apfolded-dtuniform_okeeffe-abstraction-blue_5_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/07_warp-apfolded-dtuniform_durer-turf_5_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/08_warp-apfolded-dtuniform_rembrandt-night-watch_5_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/09_warp-apfolded-dtuniform_hokusai-great-wave_8_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/10_warp-apfolded-dtuniform_okeeffe-abstraction-blue_8_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/11_warp-apfolded-dtuniform_durer-turf_8_2000x2000.png`
- `out/warp-nested-fbm/uniform-detail/web-palettes/12_warp-apfolded-dtuniform_rembrandt-night-watch_8_2000x2000.png`

Palette assessments:

- `hokusai-great-wave` produces cream ridges over plum/navy cavities. It has a
  mineral, marbled identity and good warm/cool separation; broad light bodies
  remain shaped by fine internal folds rather than reading soft.
- `okeeffe-abstraction-blue` gives the cleanest cool depth ladder. Near-black
  cavities, slate bodies, and restrained mint highlights separate structure
  without hue noise. It is the most consistently legible palette.
- `durer-turf` reads as pale sage stone. Fine geometry remains visible, but its
  close middle/high values compress depth and make large bodies comparatively
  flat. It is usable but weakest for this material treatment.
- `rembrandt-night-watch` gives the strongest dramatic value hierarchy:
  near-black cavities, olive bodies, and warm ochre ridges. It preserves detail
  without clipping highlights and best emphasizes folded volume.

Seed assessments:

- Seed 1 retains a large central diagonal current and nested eddies throughout.
  O'Keeffe is strongest: its dark voids and cool highlights separate both broad
  flow and fine folds without any region falling out of focus.
- Seed 5 has the clearest alternating cavities and raised islands, especially
  through the center and lower right. Rembrandt is the strongest overall
  combination because deep black-green recesses and warm ridges establish the
  most immediate three-dimensional hierarchy.
- Seed 8 is dense but remains organized around sweeping upper and lower flows.
  Hokusai is strongest for this seed: cream/plum contrast reveals fine nested
  structure while retaining larger directional movement.

Strongest and weakest combinations:

- Strongest: seed 5 with `rembrandt-night-watch`
  (`08_..._2000x2000.png`). It has the broadest usable value range, coherent
  warm highlights, deep cavities, and no loss of fine detail across the frame.
- Strong cool alternative: seed 1 with `okeeffe-abstraction-blue`
  (`02_..._2000x2000.png`). It offers the clearest restrained value-led depth.
- Weakest: seed 8 with `durer-turf` (`11_..._2000x2000.png`). The composition
  and edge detail remain sound, but the pale green middle/high roles merge more
  than in the other palettes and reduce perceived relief.

## Performance And Verification

Pre-commit verification passed:

- Focused detail, varied preservation, and golden tests: passed.
- `go test ./internal/sketch/warp -count=1`: passed.
- `make check`: formatter, vet, golangci-lint (`0 issues`), and all Go tests
  passed.
- Complete default uniform point hot path on Apple M1 Pro:
  `BenchmarkAt-10 549867 2168 ns/op 0 B/op 0 allocs/op`.

The benchmark varies coordinates and captures `plan.At`, including field and
material sampling, central-difference normals, palette mapping, and linear-light
shading. Uniform therefore retains the required allocation-free ordinary point
path.

## Commits

- `5113745` `docs: design uniform warp detail`
- `b4eab67` `docs: plan uniform warp detail`
- `f2e61e8` `feat: add uniform warp detail`

## Recommendation

Recommend uniform detail as the default and retain varied as the explicit
calm-versus-turbulent alternative. Uniform directly fixes the diagnosed field
suppression instead of masking it, meets the visual target across the fixed
seed/palette matrix, preserves the previous artwork byte-for-byte, and does not
widen the numeric control surface. O'Keeffe and Rembrandt are the strongest
palette families for review; Durer remains valid but less dimensional.
