# Warp

## Implemented algorithm

A point-sampled fBm field is evaluated as plain, single-domain-warp or nested-domain-warp. Independent displacement fields bend coordinates before color mapping. Uniform detail uses one activity level; varied detail uses a broad activity field to create quiet and concentrated passages. Folded coloring maps field behavior differently from a continuous gradient.

## Controls and output space

Local controls select warp mode, gradient/folded mapping, uniform/varied detail and field/color parameters. There is no weighted trait schema.

## Rendering and review

Sampler; honors raster AA/deep. Compare the same seed across modes/details to isolate coordinate deformation from palette changes.

```sh
go run ./cmd/staticart render warp --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/warp/warp.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/warp/options.go) own local knobs. [Tests](../../internal/sketch/warp/warp_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -appearance string
        colour mapping: gradient|folded (default "folded")
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -detail string
        distribution of nested field detail: uniform|varied (default "uniform")
  -format string
        output format: png|jpg (default "png")
  -gain float
        amplitude multiplier per octave (default 0.5)
  -height int
        override height in px (requires --width)
  -lacunarity float
        frequency multiplier per octave (default 2)
  -nested-strength float
        second domain displacement (default 4)
  -octaves int
        number of fBM components (default 5)
  -out string
        output directory (default "out")
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -scale float
        base field cycles per canvas unit (default 1.65)
  -seed uint
        random seed (same seed → same image) (default 42)
  -warp string
        field development mode: plain|single|nested (default "nested")
  -warp-strength float
        first domain displacement (default 3)
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
