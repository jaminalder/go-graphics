# Sketch 004 — `drift` (painted dots along a flow field)

Inspired by Tyler Hobbs' spiral dot arrangements, but the dots follow the
streamlines of a Perlin flow field instead of a spiral. Every dot is
*painted* with a human-like brush mark rather than filled analytically —
this sketch introduces the `internal/paint` package.

## The paint model (new, reusable)

`paint.Canvas` is a float-RGBA buffer painted by **stamping**: soft-edged
dabs blended source-over, strokes as dab trains along paths. This is a
second rendering model beside `render.Raster`'s pure per-pixel functions —
painting is sequential and order-dependent, anti-aliasing comes from the
soft dab edges (Context.AA is ignored), and the final buffer is quantized
with the same dithering as the raster path. Composition stays
resolution-independent (paths and sizes in canvas units); exact stroke
texture varies subtly with resolution, which is part of the medium.

Humanity comes from three ingredients, all seeded: **wobble** (radial
harmonics with random amplitude/phase on every path), **drift** (slow
center wander along a stroke), and **misregistration** (under-layers offset
from the strokes above them).

## Disc styles (paint package marks)

- `RingsDisc` — a solid under-disc, slightly offset, under a hand-drawn
  spiral of concentric ink rings plus a center dot (the classic Hobbs
  ringed dot).
- `ScribbleDisc` — an under-disc under many overlapping wobbly loops at
  random radii; translucent strokes build up where they cross (the
  sequin/crayon look).
- `GouacheDisc` — an opaque blob with an irregular wobbled edge over an
  offset shadow blob (flat hand-painted dot).

## Layout

1. **Paper** — lightest palette color, lightened and desaturated, plus a
   sparse speckle of tiny ink dots.
2. **Flow field** — angle = k·fBm; streamlines start on a jittered grid
   and step along the field, placing dots of locally-coherent size (a
   second low-frequency size field) wherever they fit (geom collision
   index, small gap). Fixed streamline/step/miss budgets keep the loop
   deterministic.
3. **Painting** — dots painted in placement order. Per dot: main color
   from a low-frequency color field (coherent patches) with jitter; ink =
   darkest palette color (or lightest when the main is the darkest);
   style per dot (mix of the three, weighted) or forced via `--style`.

## CLI

`--style rings|scribble|gouache|mix` (default mix) — same composition,
different painting; the style is part of the filename suffix.

## Acceptance checklist (visual)

- [ ] Dots visibly follow flowing currents; sizes cluster regionally.
- [ ] Marks read as hand-made: wobbly rings, misregistered under-discs,
      scribble buildup — no two dots identical.
- [ ] The three forced styles give clearly different moods on the same
      composition.
- [ ] Paper + speckle reads as a quiet ground; palette cohesive.
