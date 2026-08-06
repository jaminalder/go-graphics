# Warp Folded Depth Design

## Purpose

Revise the existing `warp` experiment so its strongest seeds read as folded,
layered material rather than a flat scalar gradient. Preserve the broad motion
and calm/turbulent contrast already visible in seeds 1, 2, 5, and 8 while
adding finer nested filaments, dark recesses, pale ridges, and coherent depth.

The reference demonstrates useful mechanisms but carries a restrictive source
license. This implementation may use the general published ideas of nested
domain warping, octave-domain rotation, nonlinear field shaping, and normals
derived from a scalar field. It must not copy shader source or constants
verbatim. The result remains an original artwork in this repository's Perlin,
palette, coordinate, color, and rendering model.

## Diagnosis

The current image is flat for three related reasons:

- every octave samples the same axes, so fine detail accumulates as soft grain
  rather than a directional tangle;
- `q`, `r`, and the final value use one octave vocabulary, so nested levels are
  separated by displacement but not by scale or texture;
- final color is almost entirely a one-dimensional value ramp. Dark, middle,
  and light colors describe scalar level, but nothing makes adjacent levels
  read as a recess, wall, or ridge under one coherent light.

The reference's apparent depth is not palette alone. Its final field is sampled
nearby to derive a normal and then lit. Palette hierarchy and nonlinear value
shaping strengthen that surface cue.

## Chosen Approach

Treat the nested scalar as a folded material height field. Improve the field
first, then apply subtle physically coherent shading from that same field.

This is preferred over palette-only tuning because value structure alone
cannot reliably tell the eye which side of a fold rises. It is preferred over
pseudo-lighting from `q` or `r` because those vectors displace the domain but
are not the gradient of the visible surface; highlights based on them can
disagree with the actual folds.

## Field Construction

Keep the private immutable `plan` and existing plain/single/nested development
axis. Replace axis-aligned octave stepping with a fixed orientation-preserving
rotation applied between octaves. Frequency still grows by lacunarity and
amplitude falls by gain. The rotation is an internal constant, not a CLI knob.

The three field roles differ deliberately:

- `q` remains broad and controls large bends;
- `r` samples a somewhat finer domain and creates nested folds;
- the final value samples the folded coordinate with enough high-frequency
  energy to produce narrow veins without making quiet regions uniformly busy.

The existing low-frequency activity envelope continues to suppress both warp
strength and fine-detail emphasis in calm passages. The fine structure must be
a consequence of the same nested field, not a separate line, hatch, particle,
or cell layer.

## Material Signal

Field evaluation returns a raw final value and a shaped material height. The
height is built from original sketch-local shaping, with two jobs:

- asymmetric contrast opens low values into dark cavities while retaining
  broad mid-value bodies;
- a restrained ridge term derived from the high-frequency portion narrows
  selected transitions into pale filaments.

The shaping must remain continuous enough for stable finite differences. It
must not quantize into terraces or draw explicit contour lines. Plain and
single modes remain useful diagnostic baselines and may receive the same
appearance treatment, but nested mode is the artistic target.

## Folded Appearance

Replace `structure` with, or evolve it into, a folded-material appearance. Keep
`gradient` as an unlit diagnostic baseline unless visual comparison shows no
value in retaining it. Do not add more than one new public appearance name; a
minimal result may keep exactly `gradient|folded`.

Palette roles are selected by luminance and constrained hue relationships:

- darkest color: cavities and deep seams;
- one or two middle colors: the material body;
- lightest suitable color: raised narrow ridges;
- optional restrained palette member: sparse warm or cool inflection driven by
  a nested internal field, never a rainbow spectrum.

Value structure leads color. Typical output should retain genuine near-black
or dark recesses, a broad middle-value mass, and small pale highlights. The
palette pass may desaturate or mix repository palette colors, but provenance
remains with the selected palette.

## Surface Lighting

Derive the normal from central differences of the shaped material height at a
fixed epsilon in normalized canvas units:

```text
dx = h(u + eps, v) - h(u - eps, v)
dy = h(u, v + eps) - h(u, v - eps)
normal = normalize(-depth*dx, -depth*dy, 2*eps)
```

Use one fixed oblique light direction, soft ambient fill, restrained diffuse
contrast, and at most a very small broad highlight. Apply light in linear-light
RGB so darkening and illumination do not distort sRGB incorrectly. Lighting
modulates the palette color; it does not replace the field's value structure.

`eps` is a canvas length, never one output pixel, so equal-aspect preview and
print renders describe the same surface. Nearby height samples are pure and
allocation-free. They may make folded appearance several times slower than the
gradient baseline; preview feedback remains the performance target.

## Scope And Controls

Keep changes inside `internal/sketch/warp`, its tests/golden, sketch spec,
experiment records, and generic inventory text if needed. Do not alter shared
noise behavior or create a field, material, normal, or lighting framework.

Retain the six numeric field controls. Prefer internal tuned constants for
rotation, role-specific scale ratios, ridge shaping, finite-difference epsilon,
light direction, ambient, and depth. Add at most one public control only if a
fixed value prevents meaningful comparison; the preferred first version adds
none.

## Testing

Focused tests must establish:

- rotated fBM remains deterministic, finite, pure, and allocation-free;
- equal-aspect plans produce equal material samples and colors at identical
  normalized coordinates;
- the shaped height and central-difference normal remain finite and bounded on
  all fixed seeds and modes;
- a constant or locally flat height receives only ambient/front-facing light,
  while opposite slopes shade monotonically under the fixed light;
- folded appearance has materially greater luminance range than the diagnostic
  gradient across representative nested samples without clipping most pixels;
- explicit options still resolve independently;
- final render determinism and the deliberate golden.

## Visual Comparison

Use the existing eight fixed seeds and preserve the current artifacts as the
baseline. Render candidate sheets under a new ignored directory so comparison
does not overwrite them.

Required comparisons:

- baseline versus candidate for seeds `1,2,3,5,8,13,21,34`;
- focused full-size review of the user's preferred tiles 1, 2, 4, and 5, which
  correspond to seeds 1, 2, 5, and 8 in the existing contact sheet;
- gradient versus folded appearance at the same seeds;
- plain, single, and nested modes at one representative seed;
- strongest and weakest candidate at full preview size.

One or two calibrated constant sweeps are acceptable for depth/ridge strength,
but do not broaden the public parameter surface or add unrelated effects.

## Acceptance Criteria

- Preferred seeds retain their existing broad directional composition.
- Dark cavities, middle-value bodies, and pale raised ridges create an
  immediate folded-material read without looking like embossed contour bands.
- Fine filaments are visibly denser and narrower than the broad forms, but calm
  areas remain calm.
- Highlights and shadows agree across the image under one light direction.
- Depth remains legible when desaturated; hue is not carrying the illusion.
- The candidate does not reproduce the reference image, source constants, or
  exact color treatment.
- The hot path remains pure, deterministic, resolution-independent, and free
  of ordinary allocations.
- `make check` passes, fixed-seed imagery is inspected, and the follow-up result
  report records gains, regressions, render cost, strongest/weakest seeds, and
  whether this revision is worth integrating.
