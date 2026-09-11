# Rounds

## Implemented algorithm

A grid of large circular forms cycles through donuts, groove discs and targets. Each is laid down by sweeping a bristle brush along concentric rings, with dry arcs cutting back into the coat. Row colors sit on a dark ground.

## Controls and output space

`--human` controls departure from machine-perfect marks toward dry/streaky painting. It is the only sketch-owned CLI flag; geometry/brush construction is internal Go configuration. There is no weighted trait schema.

## Rendering and review

Sequential paint path; ignores Context.AA/Deep. Review the concentric brush record and frayed silhouettes against the retained circular-mark reference.

```sh
go run ./cmd/staticart render rounds --seed 42 --profile preview --out out
```

Visual source: [retained reference](../reference/target-rounds.png).

## Implementation and tests

[Artwork implementation](../../internal/sketch/rounds/rounds.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/rounds/options.go) own local knobs. [Tests](../../internal/sketch/rounds/rounds_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

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
  -human float
        humanization, 0 machine-perfect to 1 full dry-brush (default 0.8)
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
