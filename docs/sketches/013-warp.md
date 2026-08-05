# 013 - Nested fBM Warp

## Purpose

`warp` is a point-sampled field artwork built to test whether two nested domain
warps can organize ordinary fBM into broad currents. Its large forms should read
before its grain: calm passages sit beside strongly folded ones, with detail at
several scales rather than cloud texture spread uniformly over the sheet.

The sketch deliberately has no marks, particles, cells, or other devices. The
field and its restrained palette mapping are the complete image.

## Field

Canvas coordinates are normalized by the rasterizer: `v` spans `[0,1]` and `u`
spans `[0,aspect]`. For `p = scale * (u,v)`, `F_i(p)` is normalized fBM from an
independently seeded Perlin field:

```text
F_i(p) = sum(o=0..octaves-1, gain^o * N_i(lacunarity^o * p))
         / sum(o=0..octaves-1, gain^o)
```

A low-frequency Perlin field is eased and mapped to a bounded activity
multiplier `a(p)`. This broad envelope keeps some regions quiet while allowing
other regions to fold strongly. Its frequency, range, and shaping remain
internal artistic constants.

Two independent vector fields are then evaluated:

```text
q(p) = (F_qx(p), F_qy(p))
r(p) = (F_rx(p + q(p) * warp-strength * a(p)),
        F_ry(p + q(p) * warp-strength * a(p)))
```

The `warp` mode chooses the final scalar lookup:

- `plain`: `F_value(p)`
- `single`: `F_value(p + q(p) * warp-strength * a(p))`
- `nested`: `F_value(p + r(p) * nested-strength * a(p))`

Each scalar or vector component has its own deterministic Perlin seed. Fields
are built once in an immutable plan. Sampling consumes no random values,
allocates no ordinary heap objects, and mutates no state.

## Appearance

Both mappings require at least three colors and order selected palette colors
by luminance before building their ramps.

- `gradient` is the default. It contrast-shapes the normalized final scalar and
  moves through a small restrained HSL ramp from dark through middle to light.
- `structure` keeps final value as the tonal foundation while bounded `q` and
  `r` components shift adjacent palette mixes. Plain mode omits both structural
  influences and single mode omits `r`, so the mapping remains meaningful for
  all three modes without inventing decorative rainbow color.

## Controls

- `--scale` in `[0.25,12]`: base field cycles per canvas unit.
- `--octaves` in `[1,8]`: number of fBM components.
- `--gain` in `[0.2,0.85]`: amplitude multiplier per octave.
- `--lacunarity` in `[1.2,3.5]`: frequency multiplier per octave.
- `--warp-strength` in `[0,8]`: first domain displacement.
- `--nested-strength` in `[0,8]`: second domain displacement.
- `--warp plain|single|nested`: field development mode.
- `--appearance gradient|structure`: palette mapping.

The defaults target useful preview iteration: nested mode with gradient
appearance, moderate base scale, five octaves, and enough displacement for the
broad activity envelope to separate calm and folded passages.

## Determinism And Resolution

All Perlin seeds derive from `Context.Seed` and fixed, unique stream constants.
The immutable plan contains no pixel dimensions. Frequencies and displacement
are in normalized canvas units, so equal-aspect renders sample the same field at
the same `(u,v)` coordinates regardless of output resolution.

## Acceptance checklist

- Large-scale directional movement reads before fine texture.
- Typical nested renders contain both calm and strongly folded passages.
- Plain, single, and nested modes are visibly distinct at one fixed seed.
- Detail exists at several scales without uniform agitation.
- Gradient colour is restrained; structure colour clarifies q/r organization.
- Fixed seeds vary meaningfully while retaining one visual identity.
- The hot path is pure, resolution-independent, and allocation-free in normal sampling.
