# Sketch 002 — `tapestry` (striped, grained contour layers)

A richer composition built on the contour system of sketch 001, inspired by
woven-carpet-like generative pieces: contour noise overlaid with vertical
stripes, large tinted regions, and grain texture.

## Layers (applied per pixel, in order)

1. **Contour base** — as sketch 001: fBm noise → three-band gradient
   coloring, outer bands shuffled, middle smooth. Deliberately chunky rings
   (few bands, moderate frequency): user feedback 2026-07-22 was that the
   gap between finest ripples and biggest shadows read as too large, so the
   smallest structures were made bigger and the regions smaller/more complex.
2. **Region tint** — a second, low-frequency fBm field; where it exceeds a
   threshold the base color is **multiply-blended** with a saturated,
   lightened version of the palette's darkest color (smoothstepped near the
   edge). Multiply acts like dye: hue shifts and deepens while the rings'
   contrast survives. (Plain lerp-to-dark was tried first and went muddy.)
3. **Vertical stripes** — full-height stripes of random width covering the
   canvas: a mix of wide bands and thin lines. Colored stripes multiply with
   a lightened palette color (warp-thread dye); others nudge toward white or
   black or pass through. Each stripe carries its own grain multiplier and
   grain style.
3b. **Relief (optional, `--relief` / `Sketch.Relief`)** — 3D shading pass
   over the assembled surface, treating the contour noise as a height
   field: Lambertian hillshade from finite-difference normals, a paper-cut
   shadow just below every band edge with a lit rim just above it, and a
   subtle Blinn-Phong specular. A pure shading pass (composition identical
   to the unshaded seed); also reveals terracing inside the smooth middle
   gradient, giving cloud areas a carved-lacquer look. Constants are in
   tapestry.go (`relief*`, `edge*`, `spec*`); promote to fields when they
   need per-image variation.
4. **Grain** — deterministic white-noise displacement of the final color,
   sampled on a fixed lattice in normalized coordinates (resolution
   independent). Two styles per stripe: `speckle` (square cells) and
   `streak` (tall thin cells → vertical fiber look).

## Per-seed randomization ("art director" draws)

All draws come from Context.RNG streams within bounded ranges chosen for
aesthetics; the tunable struct fields hold the *ranges*, not the values:

| Draw | Range | Purpose |
|---|---|---|
| Contour frequency | 4.0 – 6.0 | feature scale variation |
| Bands per gradient | 20 – 40 | ring density |
| Band thresholds ±T | 0.10 – 0.18 | ring-area vs cloud-area balance |
| Noise mapping range | ±0.55 – ±0.70 | how much of each gradient shows |
| Region frequency | 2.2 – 3.2, 2 fBm octaves | size and edge complexity of tinted areas |
| Region threshold / strength | 0.10 – 0.25 / 0.50 – 0.85 | tint coverage and depth (multiply) |
| Stripe widths | thin 0.003 – 0.010 (45%), wide 0.02 – 0.09 | warp rhythm |
| Stripe effect | tint 60% / lighten 12% / darken 13% / none 15%, amount 0.05 – 0.30 | color variety without mud |
| Stripe grain | multiplier 0.3 – 1.4, streak style 20% | woven texture |
| Grain amount | 0.03 – 0.06 | overall texture strength |

**Palette roles are assigned by luminance**, not position, so any ColorLisa
palette works: lightest color → smooth middle gradient anchor, darkest →
region tint, two random picks of the remaining three → the shuffled gradient
anchors. This is the main aesthetic guardrail.

## Determinism & resolution notes

- RNG streams: 1 shuffle-low, 2 shuffle-high, 3 stripe layout, 4 parameter
  draws. Region noise uses a seed derived from Context.Seed so it is
  independent of the contour field.
- Stripes are generated to cover [0, aspect]; a non-square canvas draws more
  (or fewer) stripes but the same seed at the same aspect is identical.
- Grain lattices are fixed-resolution in normalized coordinates, so grain is
  the *same pattern* at every output size (invariant 2 holds fully). At
  print size one grain cell spans a few pixels — intentional, matches the
  chunky stipple of the reference.

## Acceptance checklist (visual)

- [ ] Contour-ring regions and smooth clouds still read clearly under the
      overlays.
- [ ] Stripes: mixed widths, no two adjacent stripes with jarring identical
      effects; thin dark lines present but not dominant.
- [ ] Large tinted regions with rings showing through the tint.
- [ ] Visible grain; some stripes with vertical streak texture.
- [ ] 10 different seeds → clearly different but consistently presentable
      images across several palettes.
