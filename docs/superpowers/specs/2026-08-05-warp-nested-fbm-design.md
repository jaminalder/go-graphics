# Nested fBM Domain Warp Design

## Purpose

Add a new static raster sketch, `warp`, inspired by nested fBM domain warping
without copying the reference artwork. The first version prioritizes quick
visual feedback and a readable comparison between plain fBM, one domain warp,
and two nested domain warps.

The visual goal is broad organic movement with detail at several scales,
meaningful seed variation, and quiet passages beside turbulent ones. It must
avoid generic cloud noise, uniform agitation, rainbow colour, and unrelated
graphic devices.

## Chosen Approach

Use independent deterministic Perlin fields for the components of the first
warp `q`, second warp `r`, final scalar value, and a broad activity envelope.
The envelope modulates warp strength spatially so some passages remain calm
while others fold strongly. This is sketch-local artistic policy; existing
`internal/noise.Perlin` remains the mechanism.

This is preferred over sampling one field at fixed offsets because independent
fields reduce visible correlation. It is preferred over rotated octaves because
the simpler octave sum makes the three development modes easier to interpret
in a first experiment.

## Architecture

`internal/sketch/warp` owns a small immutable private plan built once per
render. The sketch resolves and validates CLI settings, creates all seeded
Perlin fields, derives any fixed offsets and palette colors, and stores only
immutable values in the plan.

A sketch-local fBM evaluator sums a configurable octave count with explicit
gain and lacunarity. It is a concrete function rather than a shared interface
or package because this is the only current caller requiring both controls.

The sampling flow is:

```text
Sketch fields + Context
  -> resolved settings and seeded Perlin fields
  -> q(p): first vector fBM field
  -> r(p): q-warped vector fBM field when nested mode is active
  -> value(p): plain, q-warped, or r-warped scalar fBM
  -> appearance(value, q, r, activity)
  -> sketch.Raster
```

The normal point path draws no random values, mutates no state, allocates no
ordinary heap objects, and never rebuilds field configuration.

## Modes And Controls

The independent development axis is:

- `--warp plain`: evaluate final fBM directly.
- `--warp single`: displace the final lookup by `q * warp-strength`.
- `--warp nested`: build `r` from q-warped fBM and displace the final lookup by
  `r * nested-strength`.

The independent appearance axis is:

- `--appearance gradient`: restrained palette-derived gradient, the default.
- `--appearance structure`: simple palette-derived mapping using internal
  values such as final scalar, one component or magnitude of `q`, and one
  component of `r` to make nested structure visible without becoming rainbow.

The complete public parameter set is:

- `--scale`
- `--octaves`
- `--gain`
- `--lacunarity`
- `--warp-strength`
- `--nested-strength`
- `--warp plain|single|nested`
- `--appearance gradient|structure`

The first six are the requested numeric controls; the final two are discrete
comparison and appearance controls. Fixed offsets, activity scale, activity
range, field seeds, and color-mapping constants stay internal.

## Appearance

Both mappings start from the selected repository palette and require enough
colors to form a useful ramp. The gradient mapping normalizes the practical
fBM range, adds restrained contrast shaping, and interpolates through a small
ordered selection of palette colors.

The structure mapping uses the same palette family but allows `q`, `r`, and
the final value to influence different mixes or tonal weights. It must remain
one coherent mapping, not several overlaid effects. Plain and single modes use
zero or absent deeper components consistently so the option remains valid for
all three modes.

## Determinism And Errors

Every Perlin field receives a seed derived deterministically from
`Context.Seed` and a named stream or fixed seed transform. Sampling depends
only on plan values and normalized coordinates. Invalid CLI ranges and unknown
choice names fail through `internal/opt`; an undersized palette returns a clear
render error.

Scalar evaluation must remain finite for all valid options and representative
coordinates. Option bounds prevent octave counts, gain, lacunarity, and warp
strengths that would overflow or make preview iteration impractical.

## Testing

Focused tests cover:

- identical renders for the same seed and different renders for another seed;
- finite scalar and vector samples across all three warp modes, both
  appearances, representative seeds, and a coordinate grid;
- pure repeated point samples;
- equal-aspect plans agreeing at identical normalized coordinates regardless
  of pixel dimensions;
- option validation and registry visibility where existing tests require it;
- one deliberately generated and visually inspected 64-pixel golden.

`make check` is the final engineering gate. Visual approval comes from reading
the generated PNGs, not from tests.

## Visual Review

Use preview resolution and fixed seeds `1,2,3,5,8,13,21,34`.

Produce:

- an eight-seed nested-warp contact sheet;
- a three-mode comparison at one representative seed;
- individual previews for the strongest and weakest seeds;
- both appearance mappings where needed to judge whether structure mode adds
  information rather than decorative colour.

The result report records visible gains and failure modes, strongest and
weakest seeds, render-speed observation, verification commands, commits, and a
recommendation. Generated files remain ignored under `out/warp-nested-fbm/`.

## Acceptance Criteria

- Large-scale directional movement reads before fine texture.
- At least one calm area and one visibly folded area coexist in typical seeds.
- Nested mode is structurally distinguishable from plain and single modes.
- Detail appears at multiple scales without covering the whole image uniformly.
- The gradient is restrained and structure mode reveals field organization.
- Fixed seeds differ meaningfully while remaining recognizably one artwork.
- No unrelated marks, rainbow mapping, mutable sampling, or new framework.
