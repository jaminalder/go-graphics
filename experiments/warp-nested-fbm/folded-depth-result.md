# Folded depth follow-up result

- Name: `warp-folded-depth`
- Parent experiment: `warp-nested-fbm`
- Branch: `exp/warp-nested-fbm`
- Profile: `preview`
- Fixed seeds: `1,2,3,5,8,13,21,34`
- Status: `DONE_WITH_CONCERNS`

## Summary

The follow-up replaces the unintegrated `structure` appearance with a default
`folded` material appearance while retaining `gradient` as an unlit diagnostic.
Rotated fBM octaves and role-specific scales give the nested field finer
directional structure. A continuous material function opens low values into
cavities and derives restrained ridges from the final field's high-octave
residual. Fixed canvas-unit central differences of that exact visible height
produce one surface normal, and one fixed oblique light modulates palette color
in linear light.

The result is a substantial visual improvement over the original scalar ramp:
dark recesses, broad middle-value bodies, and pale ridges remain legible when
desaturated. Broad composition is recognizable in the preferred seeds, most
clearly seed 2. The status retains concerns because seed 1 is intricate across
most of the frame and seed 3 remains a diffuse quiet extreme; the fixed-seed
space does not always achieve the strongest calm-versus-folded balance.

The implementation was independently designed with repository Perlin fields,
palette operations, normalized coordinates, and original sketch-local
constants/formulas from the approved design. It does not copy source
expressions, constant or color sequences, or formulas from the supplied
restrictive-license shader.

## TDD Evidence

- Rotation/role RED:
  `go test ./internal/sketch/warp -run 'Test(OctaveRotationChangesDirectionWithoutChangingBounds|NestedRolesUseDifferentSpatialScales)' -count=1`
  failed because all three rotated fBM samples exactly equaled the old
  axis-aligned evaluator. The aggregate role test already passed through
  independent-field variation, so it was retained as a behavioral guard while
  explicit sketch-local role scales were implemented.
- Rotation/role GREEN:
  `go test ./internal/sketch/warp -run 'Test(OctaveRotationChangesDirectionWithoutChangingBounds|NestedRolesUseDifferentSpatialScales|FieldSamplesStayFinite|ModesAreDistinct|ModesLeaveUnusedFieldsZero)' -count=1`
  passed after rotated octave stepping and q/r/final scale separation.
- Material RED:
  `go test ./internal/sketch/warp -run 'Test(MaterialHeightCreatesCavitiesBodiesAndRidges|QuietActivitySuppressesFineRidges|MaterialSamplesStayFinite)' -count=1`
  failed to compile because `fieldSample` had no fine residual, height, or
  ridge signal.
- Material GREEN:
  `go test ./internal/sketch/warp -run 'Test(MaterialHeightCreatesCavitiesBodiesAndRidges|QuietActivitySuppressesFineRidges|MaterialSamplesStayFinite|SamplingIsPure|PlanIgnoresPixelDimensions)' -count=1`
  passed after continuous asymmetric body and activity-gated ridge shaping.
- Normal/lighting RED:
  `go test ./internal/sketch/warp -run 'Test(MaterialNormalsAreFiniteAndNormalized|LightOrdersOppositeSlopes|NormalStepUsesCanvasUnits|FoldedShadingPreservesColorBounds)' -count=1`
  failed to compile because material-normal, vector, fixed-light, and
  linear-shading helpers did not exist.
- Normal/lighting GREEN: the same command passed after fixed canvas-unit
  central differences and bounded linear-light illumination were implemented.
- Deliberate golden RED:
  `go test ./internal/sketch/warp -run TestGolden -count=1` failed with the
  expected pixel mismatch after the artwork changed.
- Golden generation:
  `go test ./internal/sketch/warp -run TestGolden -update -count=1` passed. The
  64px golden was read after initial implementation and again after final
  tuning; the retained image has continuous folds, cavities, and pale ridges.
- Final palette-role RED during calibration: the existing
  `TestFoldedAppearanceHasGreaterLuminanceRange` measured folded 1st-99th
  percentile luminance span `0.250` against gradient `0.381`. After continuous
  linear-light cavity attenuation and a stronger sparse pale-ridge role, the
  test passed without changing field composition or adding a control.
- Final package test: `go test ./internal/sketch/warp -count=1` passed.

The first implementation invalidated the old golden before the plan's later
golden task. Because `AGENTS.md` requires `make check` before every commit, the
field, material, lighting, spec, tests, and candidate golden were kept as one
coherent first commit instead of committing an intentionally failing tree.

## Calibration

Exactly two small internal-constant sweeps were used, both over preferred seeds
`1,2,5,8`. No public control was added.

- Initial candidate:
  `out/warp-nested-fbm/folded-depth/seeds/sheet.png` was visibly over-embossed;
  high-frequency normals dominated broad bodies in seeds 1, 5, and 8.
- Calibration 1:
  `out/warp-nested-fbm/folded-depth/calibration-1/sheet.png` reduced material
  depth and ridge height/color. It restored broad regions but retained more
  uniform fine relief than desired in seeds 1 and 8.
- Calibration 2:
  `out/warp-nested-fbm/folded-depth/calibration-2/sheet.png` reduced depth and
  ridge height once more. It retained visible folds while softening edge
  dominance and became the final constant set.

No field scale, light direction, public option, unrelated effect, explicit
line, or extra layer was introduced during calibration.

## Verification And Cost

- Baseline `go test ./...`: passed before follow-up implementation.
- `make check`: passed before both implementation commits; formatter, vet,
  golangci-lint (`0 issues`), and `go test ./...` all passed.
- Final pre-report `make check`: passed with the same gates.
- `git diff --check master...HEAD`: passed with no output before this report.
- Direct folded sample benchmark on Apple M1 Pro:
  `BenchmarkSample-10 3509832 342.7 ns/op 0 B/op 0 allocs/op`.
- 64px folded render benchmark:
  `BenchmarkRender64-10 658 1801393 ns/op 21400 B/op 42 allocs/op`.
  Render allocations belong to raster/image construction; direct point
  sampling remains allocation-free.
- Timed seed-2 preview command:

  ```sh
  /usr/bin/time -p go run ./cmd/staticart render warp --seed 2 \
    --warp nested --appearance folded --profile preview \
    --out out/warp-nested-fbm/folded-depth/strongest
  ```

  Observed wall time was `real 0.87s` (`user 3.94s`, `sys 0.22s`), compared
  with the prior experiment's `0.64s`. Both include `go run` startup and are
  iteration observations, not a controlled render benchmark.

## Artifacts

- Golden: `internal/sketch/warp/testdata/warp_seed13_64.png`
- Final eight-seed folded sheet:
  `out/warp-nested-fbm/folded-depth/seeds/sheet.png`
- Final seed manifest and full renders:
  `out/warp-nested-fbm/folded-depth/seeds/manifest.txt` and numbered PNGs in
  that directory
- Gradient/folded comparison:
  `out/warp-nested-fbm/folded-depth/appearances/sheet.png`
- Plain/single/nested seed-2 comparison:
  `out/warp-nested-fbm/folded-depth/modes/sheet.png`
- Strongest full render:
  `out/warp-nested-fbm/folded-depth/strongest/warp-wnested-apfolded_kandinsky-soft-pressure_2_600x600.png`
- Desaturated strongest review:
  `out/warp-nested-fbm/folded-depth/strongest/seed2-grayscale.png`
- Calibration sheets:
  `out/warp-nested-fbm/folded-depth/calibration-1/sheet.png` and
  `out/warp-nested-fbm/folded-depth/calibration-2/sheet.png`

The original baseline directories `out/warp-nested-fbm/seeds`, `modes`, and
`appearances` remain intact; all follow-up renders are under `folded-depth/`.

## Preferred Seeds

- Seed 1 preserves its broad looping currents and lower-left quiet edge. Fine
  folds now cover most of the frame, making it the busiest preferred seed and
  the clearest residual concern about uniform intricacy.
- Seed 2 is strongest. Its detailed left/upper current, central dark seam, and
  broad quiet lower-right body preserve the preferred composition while adding
  the clearest coherent depth hierarchy.
- Seed 5 preserves the broad warm upper-left mass and concentrates pale nested
  folds through the center/right. The transition now reads as material folding
  into a recess rather than merely changing scalar color.
- Seed 8 preserves its broad left-edge passage and highly folded central field.
  Pale/cyan inflections remain sparse enough that the structure also reads in
  grayscale, though its active area is nearly as dense as seed 1.

## Strongest And Weakest

Seed 2 is the strongest candidate for the reasons above. The grayscale review
retains its bright folded left side, dark central cavity, and quiet middle-gray
right side, proving hue is not carrying the illusion.

Seed 3 remains weakest. Its broad diffuse body dominates most of the frame and
organized folds stay near the top-right edge. Folded appearance gives it real
cavities and volume, but it remains the least directional and least immediate
composition in the fixed set.

## Visual Consequences

Gains:

- Rotated octaves replace soft same-axis grain with narrower directional
  filaments that emerge from the nested final field.
- Cavities, bodies, and ridges establish a substantially clearer value
  hierarchy than the old gradient/structure pair.
- Fixed-light highlights and shadows agree with the visible material height and
  survive desaturation.
- Plain, single, and nested remain legible development baselines; nested adds
  the densest organized folds without a decorative line layer.
- Preferred broad masses remain recognizable despite deliberate field-detail
  changes.

Regressions and concerns:

- Folded appearance is more subdued and burgundy/pink than the vivid original
  gradient; cyan and red palette extremes are less dominant.
- Seed 1 is busy across most of its surface, while seed 3 remains too diffuse.
- Fine folds can approach engraved or topographic texture in the busiest zones,
  although the final depth calibration removed the initial harsh embossing.
- Folded preview iteration is slower because every output sample queries nearby
  material heights for its normal.

## Commits

- `5f9d909` `docs: design folded warp depth`
- `2326e38` `docs: plan folded warp depth`
- `ce6da26` `feat: render folded warp material`
- `734d657` `art: tune folded warp depth`

## Recommendation

Recommend the folded revision for visual review and likely integration. It
directly addresses the original flatness concern, preserves the strongest broad
compositions, satisfies deterministic/resolution/allocation invariants, and
does so without copied shader code or a wider public API. Keep the
`DONE_WITH_CONCERNS` label until the user decides whether seed-space variation
and the more subdued palette treatment fit the intended artwork identity.
