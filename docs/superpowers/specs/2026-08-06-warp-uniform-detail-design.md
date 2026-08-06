# Warp Uniform Detail Design

## Purpose

Add a detail-distribution axis to `warp`. The new default should render the
entire canvas with one consistently turbulent, sharp material vocabulary rather
than alternating finely folded zones with soft low-activity zones. Preserve the
current calm-versus-turbulent behavior as an explicit option for future uses.

## Diagnosis

The apparent blur is not output resolution or filtering. The broad activity
field currently controls two separate mechanisms:

- it scales both nested domain displacements, so low-activity areas remain much
  closer to broad unwarped fBM;
- it gates the fine ridge signal, so the same areas lose narrow filaments.

Those areas are mathematically smooth by design. Rendering them at 2000 pixels
cannot reveal detail that the field suppressed.

## Approaches Considered

1. **Constant full activity (chosen):** replace the envelope with one tuned
   high activity value for both displacement and ridge shaping. This directly
   removes the source of spatially varying sharpness and keeps one field model.
2. **Keep varied displacement, force ridges everywhere:** preserves broad calm
   geometry but overlays fine relief on it. This risks a disconnected etched
   texture rather than uniformly turbulent folds.
3. **Local contrast sharpening:** amplify normals or color contrast in soft
   zones. This treats the visible symptom but cannot restore missing nested
   geometry and may create halos.

## Detail Axis

Add `--detail uniform|varied` as a sketch-owned choice.

- `uniform` is the new default. Every coordinate uses one fixed high activity
  value for first displacement, nested displacement, and ridge shaping.
- `varied` evaluates the existing low-frequency activity field and preserves
  its current mapping exactly.

The activity field may remain constructed in the immutable plan because varied
mode needs it and plan construction is outside the hot path. Uniform sampling
must not query it. Plain mode remains geometrically unwarped, but uniform detail
may fully enable its fine material residual so appearance sharpness follows the
selected detail mode consistently.

Do not add a numeric sharpness/activity flag. The two modes are coherent visual
policies, not a public collection of internal constants.

## Architecture

Keep the private plan and pure sampler. Resolve the choice into a private enum
stored in `settings`. A small `activity(u,v)` method returns either the fixed
uniform value or the existing varied envelope. Field construction, material
shaping, central-difference normals, palette roles, and raster rendering remain
unchanged.

The ordinary point path remains deterministic, mutation-free, resolution-
independent, and allocation-free.

## Compatibility

This experiment is not integrated, so the new default may intentionally change
the golden and default renders. `--detail varied` must reproduce the current
folded output byte-for-byte for the same seed, palette, options, and dimensions.
That exact preservation is tested before accepting the change.

## Testing

Focused tests establish:

- default resolution selects uniform detail and explicit varied selects the old
  behavior;
- varied activity samples exactly match the previous envelope formula;
- uniform activity is constant across coordinates and does not sample the
  activity field in the repeated path;
- uniform nested samples show materially less spatial variation in local
  fine-detail energy across a fixed grid than varied samples, while retaining a
  high mean detail level;
- each option changes only its own resolved setting;
- field/color finiteness, determinism, resolution independence, zero-allocation
  `At`, and the deliberate new golden.

## 2000-Pixel Review

Render web-size originals at 2000x2000 for seeds `1,5,8`, the user's strongest
current examples, using these palette families:

- `hokusai-great-wave`: deep navy, muted earth, pale cream;
- `okeeffe-abstraction-blue`: narrow cool monochrome value ladder;
- `durer-turf`: muted green/brown mineral earth;
- `rembrandt-night-watch`: deep near-black, ochre, and warm light.

Generate all twelve full-size PNGs and a contact sheet under
`out/warp-nested-fbm/uniform-detail/web-palettes/`. Also render a preview
comparison of `uniform|varied` across the fixed eight seeds and inspect full-size
strongest and weakest examples. Existing baseline/folded-depth artifacts remain
untouched.

## Acceptance Criteria

- Uniform mode has no broad area that reads defocused relative to another.
- Fine nested folds and ridge sharpness cover the whole canvas at one visual
  scale; variation comes from form and color, not focus.
- Uniform mode remains organized rather than becoming pixel-scale noise or a
  uniformly embossed texture.
- Varied mode preserves the current calm/turbulent folded artwork exactly.
- Seeds 1, 5, and 8 remain compelling at 2000 pixels across multiple palettes.
- Palette changes preserve value-led depth and do not flatten the material.
- No new shared abstraction or numeric calibration surface is introduced.
- `make check`, direct image inspection, report completion, and clean worktree
  status are required before review.
