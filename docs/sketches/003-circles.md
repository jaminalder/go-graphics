# Sketch 003 — `circles` (packed circles with patterned fills)

A square (or any aspect) canvas filled with non-overlapping circles of
varied sizes on a quiet background. Each circle is filled with shades of
**one** palette color arranged in a random structure: stripes (horizontal /
vertical / diagonal), a dot raster, or Voronoi tiles. Anti-aliasing comes
from the standard pipeline (Context.AA).

## Architecture: scene, then pixels

The plan is a pure **scene model** — a list of `circleSpec` (center,
radius, fill kind, angle, scale, phase, shades, per-circle noise salt) plus
a background color. The per-pixel function only *evaluates* that data via a
uniform spatial grid (pixel → circle in O(1)). Rationale: if we ever want
vector output, a tdewolff/canvas backend can consume the same scene without
touching the art logic (see ARCHITECTURE decision log #4 — the dependency
stays deferred; output remains PNG/JPG for now, per user).

Layout code (packing, grid) lives inside the sketch package until a second
geometric sketch justifies an `internal/geom` package.

## Layers / stages

1. **Packing** (layout RNG stream) — largest-first-ish rejection sampling:
   radii drawn from a shrinking-cap power distribution (few large, many
   small), fixed position tries per attempt, fixed attempt budget (so the
   loop is deterministic), a small gap between circles, all fully inside
   the canvas margin.
2. **Fill assignment** (fill RNG stream) — per circle: base palette color,
   3–5 shades built as an HSL lightness ladder of that color, fill kind
   (stripes ~45%, dots ~25%, Voronoi ~30%), angle (stripes quantized to
   0°/90°/±45° with a small jitter), feature scale relative to the radius,
   phase, and a unique salt so every circle's pattern is independent
   (the per-terrace-salt lesson from crackle).
3. **Evaluation** — patterns are computed in circle-local coordinates
   (translated, rotated, radius-normalized):
   - *Stripes*: band index along the rotated axis; each band picks a shade
     by hash (random order, stable per circle).
   - *Dots*: square cells with a disc SDF; dot and ground shades hashed
     per cell.
   - *Voronoi*: `noise.WorleyCell` (new: returns the nearest feature
     cell's identity) → shade per cell by hash, plus a subtle tone-on-tone
     border where F2−F1 is small (crackle's low-contrast rule).
4. **Background** — lightest palette color, lightened and slightly
   desaturated, so circles carry the color.

## Per-seed draws (bounded ranges)

| Draw | Range | Purpose |
|---|---|---|
| Radii | 0.008 – 0.16 canvas units, power-skewed small | size hierarchy |
| Gap | 0.004 | breathing room between circles |
| Shades per circle | 3 – 5 | tonal richness |
| Stripe width | 0.12 – 0.35 × r | per-circle rhythm |
| Dot cell | 0.18 – 0.35 × r, dot radius 0.55 – 0.8 of cell | raster density |
| Voronoi cell | 0.15 – 0.3 × r | tile size |

Guardrails learned from the first renders: the feature size is floored at
0.008 canvas units so small circles show a few bold elements instead of
sub-pixel confetti, and the shade ladder's lightness is capped at 0.85 so
even the palest circle keeps a visible edge against the background.

## Reuse

`sketch.Raster` (AA, linear averaging, dither, 16-bit), palette HSL
machinery, `noise.Hash01` / `noise.WorleyCell`, RNG stream discipline,
metadata pipeline, golden/determinism/bounds test patterns.

## Acceptance checklist (visual)

- [ ] Clear size hierarchy: a few large anchors, many small fillers; no
      overlaps, no canvas-edge clipping.
- [ ] Each circle reads as one color family; patterns crisp with AA.
- [ ] All three fill kinds present and distinguishable at web size.
- [ ] Canvas cohesive under one palette; background recessive.
- [ ] Different seeds → clearly different layouts, consistently balanced.
