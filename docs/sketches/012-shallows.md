# 012 — shallows

A clear small river seen from directly above: broad, broken ripple crests cast
alternating light and shadow over a bed of worn, faceted stones. The stones,
their joints and rare gold remain visible through the water.

## Why this is a sketch, not a composite

The source studies are 010's convincing stone bed and 011's moving surface.
Rendering each to a PNG and laying one over the other loses the important
interaction: water does not merely tint a stone. Its sloping surface changes
which point on the bed is visible, shadows one side of a ripple and focuses
light on the other.

`shallows` therefore plans both fields and evaluates them in one pixel
function. No intermediate raster exists. For every output sample it:

1. reads the riffle surface's flow direction, slope, signed ripple and broad
   dapple;
2. offsets the lookup coordinate of the generated scree bed by the surface
   slope, producing restrained refraction;
3. borrows the coolest swatch from the selected ColorLisa palette and mixes
   a value-matched share into the bed;
4. multiplies the visible stone colour by the ripple's shadow face, then
   mixes its light face toward a pale version of that same cool swatch.

The shadow and highlight change the *bed colour itself*. That is what makes
the ripples survive over the bed's already strong facet structure.

## The surface

The reusable part of 011 remains its all-water pool/bend/field flow plan and
per-pixel upstream walk. Fine chop and the line-integral current texture are
retained as subordinate irregularity. A longer standing-wave field is added
for 012 because the original fine facets disappear against 010's many stone
facets.

The standing crests cross the downstream direction, wander through broad
flow noise and fade through a second broad field, so they form interrupted
river ripples instead of uninterrupted parallel rules. Their wavelength is
in canvas units and is independent of output resolution.

Refraction is intentionally small. Large displacement repeats and tears the
high-contrast stone edges into contour-like lines; at the default it shifts
the bed by roughly one or two preview pixels, leaving shadow and light to do
most of the surface work.

## Controls

All of scree's trait dimensions and material controls remain available,
including `--bed`, `--stones`, `--facets`, `--light`, `--wet`, `--scheme` and
`--gold`. Water adds:

- `--water-seed` — independent current and surface composition;
- `--water-depth` and `--water-tint` — optical depth and cool colour share;
- `--refraction` — maximum bed displacement response;
- `--ripple-strength`, `--ripple-shadow`, `--ripple-light` — definition and
  contrast of the surface facets;
- `--dapple-shadow` — broad shade over the complete water surface.

The Avery calibration requested for this sketch is:

```sh
staticart render shallows --profile preview --aa 2 \
  --seed 12 --water-seed 42 --palette kandinsky-soft-pressure \
  --colourway avery-bicycle-rider --bed gravel --stones worn \
  --facets cut --light noon --wet wet --scheme passage \
  --count 340 --base 0.017 --facet 0.34 --gold
```

## Acceptance checklist

- [ ] The first read is clear water over stones, not a translucent graphic
      layer and not dry scree with texture on top.
- [ ] Ripple crests and their darker trough faces are visible at thumbnail
      size, broken and gently curved rather than fine parallel hatching.
- [ ] Stones, joints, flat facet steps and gold remain identifiable below the
      surface.
- [ ] No bank, shore line, foam or second set of boulders enters the frame.
- [ ] Changing `--water-seed` moves only the surface; changing `--seed` plans
      a different bed and its corresponding surface context.
- [ ] Preview, web and print preserve the same composition and wavelengths.

## What did not work

**A separately rendered alpha overlay.** It could tint an existing image but
could not refract or cast light and shadow into the stone material. Strong
enough to read, it looked pasted on; weak enough to integrate, it disappeared.

**Promoting 011's fine chop directly.** The detailed scree bed overwhelmed it
at normal strength. Raising it produced dense horizontal engraving rather
than water.

**Large refraction.** A several-stone displacement folded every hard joint
many times and turned the image into moire. Refraction is now restrained and
the legibility comes primarily from paired shadow and light.
