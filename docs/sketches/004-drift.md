# Drift

## Implemented algorithm

Dots are planned along streamlines of a Perlin flow field and interpreted through the stamp-based paint canvas. Layout and mark style are separate decisions; fixed placement budgets and named random streams make a repeatable composition.

## Controls and output space

`--style` chooses mix, rings, scribble or gouache; it is the only sketch-owned CLI flag. Other structural settings are Go fields. There is no weighted trait schema.

## Rendering and review

Sequential paint path; ignores Context.AA/Deep and writes 8-bit canvas output. Inspect the marks at useful scale, not just the placement geometry.

```sh
go run ./cmd/staticart render drift --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/drift/drift.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/drift/options.go) own local knobs. [Tests](../../internal/sketch/drift/drift_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

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
  -style string
        painting style: mix|rings|scribble|gouache (default "mix")
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
