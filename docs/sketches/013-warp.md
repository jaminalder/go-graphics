# 013 - Folded Nested fBM Warp

## Purpose

`warp` is a point-sampled field artwork whose nested domain warps form a folded
material. Broad currents and calm passages remain the composition; rotated
octaves add narrow nested filaments, while one coherent light turns the same
visible height into dark cavities and pale raised ridges.

The sketch deliberately has no explicit lines, contours, marks, particles,
cells, or post-processing layers. Its fine veins emerge continuously from the
nested field itself.

## Field

Canvas coordinates are normalized by the rasterizer: `v` spans `[0,1]` and `u`
spans `[0,aspect]`. For `p = scale * (u,v)`, each independently seeded Perlin
field is evaluated as normalized fBM. After every octave, the running domain is
rotated by a fixed sketch-local angle and scaled by `lacunarity`; amplitude is
multiplied by `gain`. Rotation prevents every scale from accumulating along the
same axes without adding randomness or another field type.

A low-frequency Perlin envelope supplies activity `a(p)`. It suppresses both
domain displacement and ridge emphasis in quiet passages, while active regions
carry stronger folds. The nested field roles use fixed internal scale ratios:

- `q` stays broad and bends the large composition;
- `r` evaluates the q-warped coordinate at a somewhat finer scale;
- the final field evaluates the selected folded coordinate at the finest role
  scale, retaining its top two octave contributions as a fine residual.

The `warp` mode chooses how far field development proceeds:

- `plain`: final fBM at the unwarped coordinate;
- `single`: final fBM displaced by `q * warp-strength * activity`;
- `nested`: final fBM displaced by q-warped `r * nested-strength * activity`.

Shallower modes do not evaluate unused deeper fields. Each field has its own
deterministic seed and is built once in the private immutable plan.

## Material

The final raw scalar is shaped continuously into a material height in `[0,1]`.
An asymmetric smooth curve opens low values into cavities while preserving a
broad middle-value body. A narrow smooth ridge response comes from the final
field's high-octave residual and is multiplied by eased activity before being
added to the body. It is not quantized, terraced, or sampled as an explicit
contour, so finite differences remain stable and fine ridges belong to the same
surface as the broad forms.

## Appearance

Both mappings require at least three palette colors and order them by
luminance.

- `gradient` is an unlit diagnostic. It contrast-shapes the raw final scalar
  through a dark-middle-light HSL ramp.
- `folded` is the default artwork. Material height maps darkest palette color
  to cavities, a middle color to the body, and the lightest suitable color to
  raised ridges. Low height continuously attenuates cavity color in linear
  light, while ridge color is mixed sparingly, so value structure leads hue.

`structure` was an unintegrated experiment name and is removed rather than kept
as an alias.

## Surface Light

Folded appearance derives its normal from central differences of the exact
material height:

```text
dx = h(u + eps, v) - h(u - eps, v)
dy = h(u, v + eps) - h(u, v - eps)
normal = normalize(-depth*dx, -depth*dy, 2*eps)
```

`eps` and depth are fixed sketch-local canvas values, never pixel dimensions.
One fixed oblique light combines ambient fill with bounded diffuse response.
The palette color is converted channel-by-channel from sRGB to linear light,
multiplied by the light factor, and converted back to sRGB. The same visible
height therefore controls both color role and illumination.

Folded rendering evaluates nearby material samples for each output sample, so
it is intentionally several times more expensive than the diagnostic gradient.
Both paths remain pure and allocation-free in ordinary point sampling.

## Controls

- `--scale` in `[0.25,12]`: base field cycles per canvas unit.
- `--octaves` in `[1,8]`: number of fBM components.
- `--gain` in `[0.2,0.85]`: amplitude multiplier per octave.
- `--lacunarity` in `[1.2,3.5]`: frequency multiplier per octave.
- `--warp-strength` in `[0,8]`: first domain displacement.
- `--nested-strength` in `[0,8]`: second domain displacement.
- `--warp plain|single|nested`: field development mode.
- `--appearance gradient|folded`: palette mapping and material treatment.

The defaults target nested folded material with a low base scale, five octaves,
and strong displacement modulated from near-calm to turbulent by the activity
envelope. Rotation, role scales, material shaping, normal step, depth, and light
remain internal constants rather than public calibration controls.

## Determinism And Resolution

All Perlin seeds derive from `Context.Seed` and fixed unique transforms. The
immutable plan contains no pixel dimensions. Frequencies, displacement, ridge
shaping, and the normal step are in normalized canvas units, so equal-aspect
renders sample the same field, height, normal, and color at identical `(u,v)`
coordinates regardless of output resolution.

The mechanisms and constants are independently designed for this repository.
No source expression, constant sequence, color sequence, or formula is copied
from the restrictive-license shader that motivated the follow-up study.

## Acceptance checklist

- Preferred seeds 1, 2, 5, and 8 retain recognizable broad directional composition.
- Dark cavities, middle-value bodies, and pale ridges read as folded material rather than embossed contours.
- Fine filaments are narrower and denser than broad forms while calm areas remain calm.
- Highlights and shadows agree under one fixed light direction.
- Depth remains legible when desaturated; hue does not carry the illusion.
- Plain, single, and nested remain useful and visibly distinct diagnostics.
- Fixed seeds vary meaningfully while retaining one visual identity.
- The hot path is pure, deterministic, resolution-independent, and allocation-free in normal sampling.
