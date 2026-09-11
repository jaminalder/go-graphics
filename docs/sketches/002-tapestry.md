# Tapestry

## Implemented algorithm

A seeded fBm terrain chooses five color bands, with HSL gradients, shuffled/terraced mappings and terrain-owned coloring. Vertical stripes modify the mapped color after the terrain band is selected. Optional relief shades height gradients and band edges; crackle and grain add material variation. A per-seed plan resolves stripe placement and gradients before sampling.

## Controls and output space

Local flags control relief on/off and presets, terraces, stripes, crackle and grain. Relief numeric parameters are internal; `--smooth` controls fBm persistence. It uses handwritten configuration rather than a weighted trait schema. The exact help below is generated from its implementation.

## Rendering and review

Sampler; honors raster AA/deep. Relief comparison should preserve seed/palette so shading changes can be distinguished from composition.

```sh
go run ./cmd/staticart render tapestry --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/tapestry/tapestry.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/tapestry/options.go) own local knobs. [Tests](../../internal/sketch/tapestry/tapestry_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -crackle
        crack network on some of the wide terraces
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -format string
        output format: png|jpg (default "png")
  -grain
        boost grain strongly on some of the wide terraces
  -grain-seed uint
        seed for the grain/crackle assignment (0 = terrace seed, implies --grain); vary for different effect layouts on the same image
  -height int
        override height in px (requires --width)
  -no-stripes
        render without the vertical stripe layer
  -out string
        output directory (default "out")
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -relief
        3D relief shading (hillshade + paper-cut edges)
  -relief-preset string
        named relief look (implies --relief): baseline|low-sun-west|noon-soft|southeast-light|deep-carve|gentle-emboss|papercut-max|smooth-marble|glossy-enamel|hard-metallic
  -seed uint
        random seed (same seed → same image) (default 42)
  -smooth float
        fBm persistence override, e.g. 0.35 for smoother terrace lines (0 = default 0.5)
  -terrace-seed uint
        seed for the terrace layout (0 = main seed); vary for different terracings of the same composition
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
