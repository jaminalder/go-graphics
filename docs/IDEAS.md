# Idea backlog

Effects brainstormed but not (all) built. Each hooks into the tapestry
terrace system: every pixel knows its stratum (band + level index), levels
have known widths, and per-terrace draws on dedicated RNG streams keep
everything deterministic and toggleable — the pattern established by
terrace grain.

## Per-terrace effects (brainstorm 2026-07-22)

Surface / material:

- **Crackle** — Voronoi crack network on some wide terraces (dried mud,
  ceramic crazing, old varnish). → implemented, see sketch 002 spec.
- **Per-terrace gloss** — per-stratum specular strength/tightness overrides
  in the relief pass: wet-polished vs bone-dry levels; rare "metallic"
  terrace as a foil/mineral seam.
- **Hatching / engraving** — parallel or contour-following line fills at a
  per-terrace angle; woodcut / copperplate character.
- **Grain styles** — per-terrace grain cell size (sand vs gravel), streak
  direction, blue-noise stippling (today only amplitude varies).
- **Mottling** — soft large blotches confined to a terrace; watercolor
  pooling, stone clouding.

Geometry / relief:

- **Per-terrace step height** — irregular stair heights in the paper-cut
  shading; thick slabs vs near-flush levels (analog of the variable widths,
  but in the third dimension).
- **Boundary erosion** — domain-warp the contours of selected terraces so
  soft strata get ragged edges while resistant ones stay clean.
- **Sub-terracing** — faint micro-steps inside some wide flats, visible
  mostly through relief shading.

Edges:

- **Inked outlines** — thin dark/light contour line on selected terrace
  boundaries only.
- **Selective softening** — blend a few terrace transitions smooth while
  the rest stay hard steps.
- **Stitched / dashed edges** — dashes following the contour; textile echo.

Color (treat cautiously — the hill-level accent recolor was rejected):

- **Vein accents** — only *thin* terraces recolored toward a saturated
  accent (gold seams).
- **Within-terrace ombré** — darker at a level's lower edge via bandFrac.
- **Aging** — per-terrace exposure shift (sun-bleached vs freshly cut).

Ranking at time of brainstorm: crackle and step height highest payoff,
then boundary erosion, then gloss.
