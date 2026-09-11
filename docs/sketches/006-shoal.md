# Shoal

## Implemented algorithm

Collision-aware chains of painted dots follow a noise flow/curl/ridge field. Color and radius vary along planned chains. Dots can be interpreted as discs, ribbons, mixed marks or washes; abandoned short chains are removed from collision occupancy.

## Controls and output space

Field/grade/mark/ground and placement controls are local flags, not a weighted trait schema. `--grade` includes vortex/patches, `--mark` disc/ribbon/mixed/wash, and `--ground` light/dark.

## Rendering and review

Sequential paint path; ignores Context.AA/Deep. Review continuity, crowding and brush character together; geometry tests defend chain separation and dropped-chain cleanup.

```sh
go run ./cmd/staticart render shoal --seed 42 --profile preview --out out
```

Visual source: [retained reference](../reference/target-shoal.png).

## Implementation and tests

[Artwork implementation](../../internal/sketch/shoal/shoal.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/shoal/options.go) own local knobs. [Tests](../../internal/sketch/shoal/shoal_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -confetti float
        share of chains whose colour ignores the field (default 0.4)
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -detail float
        share of dots with concentric ring detail (default 0.09)
  -field string
        flow field: flow|curl|ridge (default "flow")
  -format string
        output format: png|jpg (default "png")
  -grade string
        size grading: vortex|patches (default "vortex")
  -ground string
        canvas ground: light|dark (default "light")
  -height int
        override height in px (requires --width)
  -margin float
        clear ground around the field; 0 bleeds off the edge (default 0.045)
  -mark string
        what a chain paints as: disc|ribbon|mixed|wash (default "disc")
  -maxr float
        largest dot radius in canvas units (default 0.0135)
  -minr float
        smallest dot radius in canvas units (default 0.0035)
  -mono
        build the ink set from one hue plus a rare accent
  -open float
        share of dots painted as rings not discs (default 0.05)
  -out string
        output directory (default "out")
  -overlap float
        how far marks may crowd into each other, x radius
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
