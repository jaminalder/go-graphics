# Hatching reference

`internal/hatch` turns a point and region description into scalar ink coverage. It owns the arrangement of marks, not color or artistic layer order. Callers construct a `Spec` once with `hatch.New`, then call `Cover(Sample)` from the pure pixel path. Source: [core API](../internal/hatch/hatch.go), [field evaluation](../internal/hatch/field.go), [combinations](../internal/hatch/compose.go).

## Structures

| Structure | Rule and required region information |
| --- | --- |
| Parallel | Constant angle/spacing; optional curvature or waveform |
| Contour | Level sets of wall distance; requires `Wall` |
| Concentric | Rings around `CX`, `CY` |
| Radial | Seamless quantized rays; centre and `Reach` |
| Fan | Circular arcs between two poles; centre and reach |
| Flow | Level sets of a Perlin stream function with a mean direction |
| Scribble | Noise level sets without mean direction |
| Stipple | Dots on the same two-dimensional lattice |
| Chord | Chords through a roughly circular region; centre and reach |

Cross-hatching, weave and nesting are compositions of primitives rather than additional primitive structures. The composition API combines coverage before the caller chooses color.

## Coordinates and controls

`Sample` provides normalized `U,V`, optional region centre `CX,CY`, region `Axis`, boundary distance `Wall`, half-size `Reach`, and desired darkness `Tone`. Unknown wall distance may be positive infinity; boundary-dependent structures then draw nothing. Unknown zero reach is treated as 0.5 canvas unit.

Lengths such as spacing and wavelength use canvas units. Thickness is dimensionless, a fraction of spacing. Alignment can use the canvas or region's centre/axis. Fit expresses region-relative scale. Tone has an effect only when tone-to-density or tone-to-width controls are enabled. Seed-keyed noise/hash variation makes repeated samples deterministic and concurrency-safe.

The full `Spec` groups structure, spacing/thickness, angle/phase, alignment/fit, waveform/curvature, dash/variation and tone effects. Its field comments describe exact units and defaults; `New` normalizes the specification. Avoid treating mark thickness as pixels when the caller changes output size.

## Specimen book and verification

```sh
make hatchbook
```

This writes five sheets plus a manifest under `out/agent-hatch` by default: structures, parameters, variation, colour and shapes. `HATCHBOOK_OUT` overrides the directory. The [Hatchbook sketch](sketches/017-hatchbook.md) is an internal specimen book, not a public artwork.

Coverage tests defend geometry/parameter behavior and composition. Inspect the generated sheets to see the effect of mark families at useful scale; a scalar test cannot assess artistic suitability. Foam is an existing consumer that combines cell measurements, hatch coverage and color, in [hatching.go](../internal/sketch/foam/hatching.go).
