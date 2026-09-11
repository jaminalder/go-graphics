# Contour

## Implemented algorithm

Three cosine gradients color a seeded fBm field. Low and high bands are discretized and independently shuffled, making sharp contour rings; the middle mapping keeps its 50 sampled colors in gradient order rather than shuffling them. The private plan constructs noise and gradients once; `At` selects the mapped band at a normalized coordinate.

## Controls and output space

No sketch-owned CLI flags or trait schema. Go defaults: frequency 6, octave index 2 (three components), 50 bands, gain 1, thresholds −0.15/0.15 and mapped range −0.6/0.6. Palette colors 0–2 form the gradient endpoints.

## Rendering and review

Sampler; honors raster AA and deep output. Compare ringed outer regions and smooth middle passages against the retained target.

```sh
go run ./cmd/staticart render contour --seed 42 --profile preview --out out
```

Visual source: [retained reference](../reference/target-sketch7.jpg).

## Implementation and tests

[Artwork implementation](../../internal/sketch/contour/contour.go) owns the mechanism and defaults. [Tests](../../internal/sketch/contour/contour_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -format string
        output format: png|jpg (default "png")
  -height int
        override height in px (requires --width)
  -out string
        output directory (default "out")
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -seed uint
        random seed (same seed → same image) (default 42)
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
