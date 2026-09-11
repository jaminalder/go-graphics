# Pools

## Implemented algorithm

A planned arrangement of overlapping watercolour circles, annuli and nested rings is painted on paper. Arrangement can scatter or follow currents/orbits; fills resolve circle-size/coverage ranges and the pigment/edge behavior used for each planned mark.

## Controls and output space

Traits: arrange, colourway, size-variety, flow and fill. Numeric controls override trait-resolved ranges. `--colourway from-flag` honors the global palette. Concrete config supports canonical public recipes; the web publishes selected styles/colors only.

## Rendering and review

Sequential paint path; ignores Context.AA/Deep. Review overlap, pigment mixing, paper space and broad arrangement over a sweep.

```sh
go run ./cmd/staticart render pools --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/pools/pools.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/pools/options.go) own local knobs. [Tests](../../internal/sketch/pools/pools_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -alpha float
        pool strength; below 1 keeps crossings readable (default 0.74)
  -arrange string
        how the marks are laid out on the sheet: orbital|formation|shadows|scatter (default: from seed)
  -band-overlap float
        how far neighbouring rings cross, x pitch (default 0.4)
  -band-width float
        ring pitch of a banded circle, canvas units (default 0.022)
  -banded float
        share filled with concentric rings (default 0.3)
  -base float
        smallest circle radius, canvas units (default 0.06909957938022503)
  -colourway string
        which palette the pigments come from: tchelitchew-hide-and-seek|botero-seated-nude|diebenkorn-seawall|davis-anthracite-minuet|bruegel-icarus|hopper-night-windows|cezanne-bathers|beckmann-dancing-bar|avery-bicycle-rider|munch-scream-oil|dix-nun|dechirico-red-tower|chagall-mariee|klee-destruction-hope|botticelli-venus|cassatt-letter|albers-tehuana|duchamp-landscape|from-flag (default: from seed)
  -count int
        anchor circles before satellites (default 26)
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -fill string
        how much of the frame the discs take up: sparse|open|medium|busy|packed (default: from seed)
  -flow string
        the field a run of marks follows: horizontal|vertical|diagonal|spiral|circular|explosive (default: from seed)
  -format string
        output format: png|jpg (default "png")
  -gap float
        clearance between circles, x radius; negative lets them cross (default 0.052933429872933695)
  -glaze float
        share carrying a second pigment on top (default 0.16)
  -ground float
        strength of the painted ground wash; 0 is bare paper (default 0.5)
  -ground-blotch float
        wavelength of the ground's unevenness, canvas units (default 0.34)
  -height int
        override height in px (requires --width)
  -margin float
        clear paper at the edge (default 0.03478246198305265)
  -max-bands int
        most rings a banded circle may have (default 5)
  -open float
        share painted as annuli rather than discs (default 0.28)
  -out string
        output directory (default "out")
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -pigments int
        palette colours in play (default 4)
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -ragged float
        wash edge deviation; 0 is a true circle, 0.22 a blob (default 0.055)
  -ratio float
        size ladder step ratio (default 1.5828784856410427)
  -rings float
        share of circles carrying inner rings (default 0.34)
  -rungs int
        steps on the size ladder (default 4)
  -satellites float
        share of circles given an overlapping companion (default 0.6715109651657202)
  -seed uint
        random seed (same seed → same image) (default 42)
  -size-variety string
        whether every run is the same size: varied|constant (default: from seed)
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
