# Sketch 001 — `contour` (shuffled-gradient contour noise)

Reproduce the look of
[shuffled_grad_palette_scale_2.jpg](https://github.com/jaminalder/staticart/blob/master/shuffled_grad_palette_scale_2.jpg)
from the Clojure staticart project. Local copy of the target:
[../reference/target-sketch7.jpg](../reference/target-sketch7.jpg). Original
source: [sketch_7.clj](https://github.com/jaminalder/staticart/blob/master/src/staticart/sketch_7.clj).

**Goal is visual equivalence, not bit-exactness** (decision #5 in
ARCHITECTURE.md): same character of contour bands, cloud areas, and color
feel; exact pixel values will differ because noise and shuffle differ.

## Why the image looks the way it does (the key insight)

A smooth 2-octave noise field is colored through three gradients selected by
noise value. Two of the three gradients are **shuffled** — their 50 discrete
colors are put in random order — so as the smooth noise passes through those
ranges, adjacent noise values jump between unrelated colors, producing sharp
**contour rings** (like a topographic map / marbled agate). The middle
gradient is **not** shuffled, so the mid-range renders as smooth cream↔teal
clouds. The tension between smooth clouds and hard contour bands *is* the
artwork.

## Original algorithm (from sketch_7.clj), exactly

- Canvas: 2000 × 2000 (the `scale_2` output file is 2500 × 2500 — a different
  run of the same sketch).
- Palette (5 RGBA colors, *not* from ColorLisa):
  `#ED6A5A` coral, `#F4F1BB` cream, `#9BC1BC` sage-teal, `#5CA4A9` teal,
  `#E6EBE0` off-white. (Only the first three are used by the gradients; keep
  all five as the sketch's "original" palette.)
- Three cosine gradients, each sampled to **50 discrete colors**:
  - `grad1` = cosine(coral → cream), **shuffled**
  - `grad2` = cosine(cream → sage-teal), smooth (not shuffled)
  - `grad3` = cosine(sage-teal → coral), **shuffled**
- Noise: thi.ng simplex `noise2`, fBm with max octave index 2, i.e. three
  components: `noise(f)/1 + noise(2f)/2 + noise(4f)/4`, base scale
  `0.006` per pixel on the 2000px canvas.
- Per pixel: `n = fbm(x, y)`, then band selection with clamped linear mapping
  of `n` into a gradient index:
  - `n < -0.15` → grad1, `n` mapped over `[-1, -0.15]` → index `[0, 49]`
  - `-0.15 ≤ n < 0.15` → grad2, mapped over `[-0.15, 0.15]`
  - `n ≥ 0.15` → grad3, mapped over `[0.15, 1]`
- No turbulence (identity warp), background white, saved as JPG.

## Port design

### Cosine gradients (Iñigo Quílez form, as thi.ng computes it)

`color(t) = a + b · cos(2π(c·t + d))` per RGB channel, with coefficients
derived from two endpoint colors `c1, c2`:

```
a = (c1 + c2) / 2      (offset)
b = (c1 − c2) / 2      (amplitude)
c = −0.5               (frequency)
d = 0                  (phase)
```

so `t=0 → c1`, `t=1 → c2` along a half-cosine ease. Implement as
`gradient.CosineBetween(c1, c2)`; then `gradient.Sample(g, 50)` → discrete
gradient; then `.Shuffled(rng)` for the banded ones.

### Mapping to our invariants

- **Normalized coordinates**: original frequency `0.006/px × 2000 px = 12
  cycles per canvas unit`. Sketch parameter is `Frequency = 12.0` (per-unit),
  so preview/web/print all show the same composition.
- **Determinism**: one seed drives (a) the noise permutation table and (b) the
  two gradient shuffles, via seed-derived sub-RNGs so adding a consumer later
  doesn't reshuffle everything.
- **Palette**: default palette should come from ColorLisa (pick one with a
  warm accent + pale neutral + cool mid — e.g. Hokusai's Great Wave or
  Cézanne's Bathers; choose by rendering candidates). Keep the original
  5-color staticart palette available as `staticart-seven` for direct
  comparison with the reference image. Gradient endpoints = palette colors
  0→1, 1→2, 2→0, as in the original.

### Tunables (fields on the sketch struct, with these defaults)

| Field | Default | Note |
|---|---|---|
| `Frequency` | 12.0 | cycles per canvas unit |
| `Octaves` | 2 | max octave index → 3 fBm components, persistence 0.5 |
| `Bands` | 50 | discrete colors per gradient |
| `LowThreshold` / `HighThreshold` | −0.15 / 0.15 | band split points |
| `NoiseMin` / `NoiseMax` | −1.0 / 1.0 | outer mapping range |

Our Perlin fBm has a different value distribution than thi.ng's simplex
(2D Perlin single octave stays within ≈ ±0.71, sum ≈ ±1.24), so expect to
tune thresholds and possibly rescale `n` so the three bands get similar area
coverage to the reference. Tune by rendering previews side by side with the
target image.

## Acceptance checklist (visual, at `web` size vs the reference)

- [ ] Organic blob regions with concentric sharp contour rings (many distinct
      bands, ~coral/cream tones in one family of regions).
- [ ] Smooth, cloudy cream↔teal areas between blob regions — no banding there.
- [ ] Ring colors look *non-monotonic* (shuffled): adjacent rings are not a
      smooth ramp.
- [ ] Feature size: roughly 5–15 major blob clusters across the canvas
      (frequency ≈ 12 at aspect 1).
- [ ] Same seed at preview and print size → same composition.
- [ ] Determinism + golden tests pass (see ARCHITECTURE.md §7).
