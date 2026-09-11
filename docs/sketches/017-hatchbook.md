# Hatchbook

## Implemented algorithm

A specimen-sheet sketch constructs repeatable examples of hatch structures, parameter variation, color interpretation and region shapes. It demonstrates the color-free hatch API through concrete rendered examples and a matching manifest.

## Controls and output space

`--page` selects structures, parameters, variation, colour or shapes. `make hatchbook` renders all five at their intended specimen dimensions and writes the manifest.

## Rendering and review

Sampler; honors raster AA/deep. This is an internal reference sheet, not one of the sixteen artworks or a public catalogue entry.

```sh
go run ./cmd/staticart render hatchbook --seed 42 --page structures --width 2000 --height 2000 --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/hatchbook/hatchbook.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/hatchbook/options.go) own local knobs. [Tests](../../internal/sketch/hatchbook/hatchbook_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

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
  -gutter float
        space between squares, canvas units (default 0.014)
  -height int
        override height in px (requires --width)
  -margin float
        border around the grid, canvas units (default 0.03)
  -out string
        output directory (default "out")
  -page string
        which specimen sheet to draw: structures|parameters|variation|colour|shapes (default "structures")
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
