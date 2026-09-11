# Circles

## Implemented algorithm

A fixed-budget packing plan places non-overlapping circles of varied sizes using a spatial collision index. Each circle owns one palette color and a planned stripe, dot-raster or Voronoi tile fill. Sampling queries the circle/appearance plan without building geometry.

## Controls and output space

No sketch-owned CLI flags or trait schema. Go defaults: maximum radius 0.16, minimum 0.008, gap 0.004, 2400 packing attempts and 30 position tries per attempt. Radii/gaps use canvas units.

## Rendering and review

Sampler; honors raster AA/deep. Tests and review inspect containment, separation and diverse fill structure.

```sh
go run ./cmd/staticart render circles --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/circles/circles.go) owns the mechanism and defaults. [Tests](../../internal/sketch/circles/circles_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

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
