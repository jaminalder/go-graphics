# Scree Gold Review Set Design

## Goal

Render a coherent review set of 15 genuinely different `scree --gold` images
at 2000x2000. The set varies only seed, colourway, and stone size. Every image
retains the approved wet, worn, fully faceted river-bed character.

## Matrix

The set is a five-by-three matrix. Rows are colourways and columns are stone
sizes. Every cell uses a unique seed so no two images share a composition.

Colourways:

1. `avery-bicycle-rider`: warm brown, taupe, red-gray.
2. `rothko-white-black-rust`: graphite, rust, cool gray.
3. `picasso-demoiselles`: rose, mauve, slate.
4. `hokusai-great-wave`: blue-gray, warm neutral.
5. `magritte-menaced-assassin`: cold gray, blue-gray, umber.

Stone-size bands:

| band | `--base` | `--count` | purpose |
| --- | ---: | ---: | --- |
| current | 0.0215 | 240 | approved gravel scale |
| smaller | 0.017 | 340 | clearly finer stones without becoming texture |
| fine | 0.013 | 500 | smallest review scale that still carries visible facets at 2000px |

Seeds are assigned sequentially across the matrix, 11 through 25, with no
reuse. The exact seed mapping is recorded in the output manifest.

## Fixed Treatment

Every render uses:

```text
--profile web
--gold
--facet 0.34
--bed gravel
--stones worn
--facets cut
--light noon
--wet wet
--scheme passage
```

The curated Avery row uses `--colourway avery-bicycle-rider --palette
kandinsky-soft-pressure`. The other four ColorLisa palettes are outside
scree's curated seed space and use the supported deliberate override path:
`--colourway from-flag --palette <slug>`.

`--facet 0.34`, not `--faceted 0.34`, is mandatory. `facet` controls Voronoi
grain while preserving full faceting; `faceted` would randomly make stones
smooth and repeat the washed-out nugget failure.

## Output

Write the set under `out/scree-gold-set/`:

- `01_...png` through `15_...png`, each 2000x2000;
- `sheet.png`, a five-row by three-column review sheet;
- `manifest.txt`, with image number, palette, seed, base, count, and filename.

The render files remain gitignored. No source change is required unless the
set exposes an actual renderer defect.

## Review Criteria

- All 15 images are visually distinct in both composition and colour family.
- The three columns read as current, smaller, and fine stone scales.
- Every stone and every nugget has clear, consistent Voronoi facets.
- Every image has two or three visible gold nuggets, subject only to the
  renderer's documented sparse-bed fallback.
- Ordinary stones contain no competing perceptible yellow.
- Fine-scale images still read as individual river stones rather than noise.
- No image has obvious packing holes, dominant border slivers, or a nugget
  clipped into irrelevance.

If a seed fails visual review, replace only that seed and update the manifest;
do not change the fixed treatment or size matrix.
