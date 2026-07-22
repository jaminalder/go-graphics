# Sketch 002 — `tapestry` (striped, grained contour layers)

A richer composition built on the contour system of sketch 001, inspired by
woven-carpet-like generative pieces: contour noise overlaid with vertical
stripes, large tinted regions, and grain texture.

## Layers (applied per pixel, in order)

1. **Contour base with terrain-owned colorways (v4)** — one fBm field,
   its value range split into **five bands**: deep-basin / basin / cloud /
   peak / high-peak. Each band has its own colorway gradient (outer four
   shuffled, cloud smooth), so every hill or basin is uniformly one
   coloring and all color-area boundaries are contour lines of the terrain
   itself. History: v1 multiply-tint region read as "shadows over the
   image"; v3's independent zone field cut across hills and still read as
   overlay (user feedback 2026-07-22: "one hill area one coloring"). The
   reference works exactly this way — its color areas are the field's own
   basins and peaks. Deliberately chunky rings (few bands, moderate
   frequency) per the earlier scale-hierarchy feedback.
2. **Coloring** — every gradient endpoint is an actual palette color
   (chosen by luminance role: darkest two, mid, lightest two), and
   interpolation happens in **HSL space** (`gradient.HSLBetween`,
   `palette.LerpHSL`, shortest hue arc) so blends stay saturated instead
   of graying out as RGB interpolation does. Note: distant-hue pairs pass
   through intermediate hues (e.g. red→navy passes violet) — harmonious,
   but constrain pairs to nearby hues if stricter palette fidelity is
   wanted.
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
   **Height grain (optional, `--grain` / `Sketch.HeightGrain`)** — one
   height window of the terrain (per-seed: 15–30% of the folded noise
   range, centered anywhere within it) gets the grain boosted 2.5–5.5×,
   with soft edges. Because it keys off the same noise value as the color
   bands, the textured band aligns with the visible terraces and appears
   at that elevation across the whole image — like scree at one altitude.
   (A spatial-region version was tried first and rejected: the user wants
   height-dependent, not location-dependent.) Draws live on a dedicated
   RNG stream; the composition is pixel-identical with the flag off.

## Per-seed randomization ("art director" draws)

All draws come from Context.RNG streams within bounded ranges chosen for
aesthetics; the tunable struct fields hold the *ranges*, not the values:

| Draw | Range | Purpose |
|---|---|---|
| Contour frequency | 4.0 – 6.0 | feature scale variation |
| Bands per gradient | 20 – 40 | narrowest-terrace width (1/bands of the band range) |
| Terrace widths | 1/bands × (1 + Exp·spread), spread 1.5 – 4.0 | irregular level spacing: narrowest ≈ old uniform width, many levels much wider (v6, user feedback 2026-07-22). Seeded by `TerraceSeed`/`--terrace-seed` (default = main seed) so layouts can vary on a fixed composition |
| Band thresholds ±T | 0.10 – 0.18 | ring-area vs cloud-area balance |
| Noise mapping range | ±0.55 – ±0.70 | how much of each gradient shows |
| Region frequency | 2.2 – 3.2, 2 fBm octaves | size and edge complexity of tinted areas |
| Band cuts ±t1 / ±t2 | t1 0.08 – 0.14, t2 0.28 – 0.38 | cloud width and deep-band coverage |
| Cloud partner color | mid or second-lightest (50/50) | cloud tint variation |
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
