# Sketch 005 — `rounds` (brush prototype: a grid of painted circles)

A regular grid of large circles — donuts, groove discs, targets — one
colour per row on a dark ground. The circles are only a test bed: the
point of this sketch is the **bristle brush** it introduces in
`internal/paint`, which is meant to be reused for other shapes and fills.

Reference for the look: `docs/reference/target-rounds.png`.

## The problem this replaces

The first draft built texture by drawing extra marks *on top* of a
finished shape — closed wobbly loops at random radii, each with its own
jitter and centre offset. Those loops crossed one another, so the result
read as **someone scribbling over a circle** rather than as paint. Two
things were wrong, and only one of them was the amount of randomness:

1. **Direction.** The marks did not share a direction with each other or
   with the shape. Crossing marks read as a second hand drawing on top;
   parallel marks read as one brush passing over a surface.
2. **Geometry vs. texture.** Disorder was injected by warping the
   *geometry* (heavy radial wobble on every path), which made the
   silhouettes lumpy. In the reference the silhouettes are nearly
   circular and all of the life is in the *surface*.

## The brush model

`paint.Brush` is a bundle of bristles. Each bristle has a fixed lateral
offset across the ferrule, and `Brush.Stroke` paints every bristle as its
own copy of the path, displaced along the path normal. Because the
offsets are fixed, the tracks stay parallel through curves and never
cross — the streaks a brush leaves always run *along* the stroke. That
property is the whole design, and `TestDryStreaksRunAlongTheStroke` pins
it by asserting that gaps in a dry stroke are ≥3× longer along the
stroke than across it.

Two parameters shape a brush:

- **`dry`** (0–1) — the single character knob. At 0 the bundle is even
  and fully charged: uniform spacing, full load, no lift-off, so it lays
  a flat coat with a crisp edge. As it rises the bristles splay, the
  outer ones lose load (which frays the silhouette), and each starts
  lifting off the surface. Tying all three to one number means there is
  no separate "make it flat" path — `dry = 0` *is* machine-perfect.
- **`grain`** — how far the brush travels between lifting off and
  settling back down, in canvas units. This is a property of the hand,
  not of the ferrule: a fine liner dragged around a whole shape needs a
  grain far longer than its own width. Getting this wrong is what turns
  smears into stitching, so `Grain` is settable and defaults to eight
  ferrule widths.

Lift-off is a two-harmonic wave in arc length, thresholded with a wide
smoothstep so a bristle thins out and returns gradually rather than
switching on and off into hard-ended dashes.

## Fills are sweeps

A shape is filled by dragging a brush along a family of paths that follow
the shape's own flow. For anything round that family is concentric
rings, and `paint.SweepRing` walks it: overlapping passes half a ferrule
inside each rim, the brush `Reload`ed between passes so bristle gaps
don't line up, each pass just over one turn with a random start angle so
the lift-off seams scatter instead of stacking on one radius.

This is the part meant to generalise. Adding a shape means adding a path
family (parallel lines for a band, offset outlines for a polygon); the
brush, its texture and its resolution independence come along unchanged.

Concentric rings rather than one continuous spiral is deliberate: the
draft used `Spiral` for fills, and its outer end overshot into a visible
comma-shaped tail on every disc.

## Layers of a shape

1. **Coat** — a wide brush swept over the band. Nearly flat; the body of
   a shape wants to be solid, and a coat that leaves gaps everywhere
   reads as grit rather than as paint.
2. **Drawn lines** — for groove discs and targets, the concentric rings
   that are part of the design, laid with a fine, wetter brush.
3. **Scuff** — a handful of arcs from a fine *dry* liner in the ground
   colour, at random radii, biased toward the rims where a real brush
   loads up and drags as it turns. This carries most of the visible
   texture. It is applied on top of a finished coat, but unlike the
   draft's scribble every arc belongs to the same family of concentric
   circles the coat was swept along, so it reads as part of the paint.

## Tunables

| Knob | Default | Effect |
|---|---|---|
| `Rows`, `Cols` | 5, 4 | grid size |
| `Human` (`--human`) | 0.8 | 0 = machine-perfect flat shapes, 1 = dry and streaky |

`Human` scales brush dryness, scuff count, path wobble and the drift of
centres off the grid. Wobble stays small at every setting (≤1.2% of
radius) on purpose — the disorder is meant to come from the brush, not
from warping the circles.

## Acceptance checklist

- [ ] Streaks are concentric arcs that never cross each other.
- [ ] Silhouettes read as circles; no lumpy or wandering outlines.
- [ ] Each shape has flat, untextured areas — texture is not uniform
      grit or wood grain across the whole disc.
- [ ] Streaks are long smears that fade in and out, not dashes.
- [ ] No spiral tail or comma sticking out of any disc.
- [ ] `--human 0` gives flat, hard-edged shapes with no texture at all.
- [ ] Preview and print of the same seed show the same composition.
