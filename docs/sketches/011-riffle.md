# Riffle

## Implemented algorithm

A pure point sample combines depth/bed, velocity and an upstream walk to model a river from above. Surface slopes, dapple and ripple patterns are planned from immutable fields. An exported concrete `Surface` exposes coordinated water behavior for Shallows.

## Controls and output space

Traits: colourway, reach, channel, boulders, water and light. `--medium overlay` returns a translucent all-over water layer without bed or banks. Numeric overrides control the sampled fields.

## Rendering and review

Raster or straight-alpha layer path; honors AA/deep. Review water flow, bed visibility and opacity together; overlays need alpha-aware viewing.

```sh
go run ./cmd/staticart render riffle --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/riffle/riffle.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/riffle/options.go) own local knobs. [Trait schema](../../internal/sketch/riffle/traits.go) owns levels, weights and pins. [Tests](../../internal/sketch/riffle/riffle_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -bend float
        lateral swing of the centreline, canvas units (default 0.1)
  -boulders string
        how much rock breaks the surface: clear|few|scattered|field|ledge (default: from seed)
  -bubbles float
        bubble lattice scale, canvas units (default 0.014)
  -caustic float
        strength of the net of light on the bed (default 1)
  -caustic-scale float
        caustic cell size in the shallows, canvas units (default 0.03)
  -caustic-warp float
        how hard the net is folded (default 0.55)
  -channel string
        the plan form: where the water is and where it is not: straight|bend|chute|bar|braid (default: from seed)
  -channel-width float
        half width of the channel, canvas units (default 0.72)
  -chop float
        surface wave height, canvas units (default 0.0016)
  -colourway string
        which palette the water and its bed are drawn from: hokusai-great-wave|monet-parasol|cezanne-bathers|bruegel-icarus|seurat-grande-jatte|manet-boating|durer-turf|diebenkorn-seawall|turner-val-daosta|sargent-villa-marlia|delaunay-air-iron-water|kandinsky-apple-tree|homer-clams|quidor-leatherstocking|hopper-night-windows|masaccio-tax-collector|vangogh-starry-night|from-flag (default: from seed)
  -dapple float
        patchy shade over the sun
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -depth float
        water on the thalweg in a pool, in extinction units (default 1.1)
  -dune float
        irregularity of the bed (default 0.3)
  -eddy float
        circulation of the vortex pair behind a boulder (default 0.5)
  -extinction float
        how fast light dies with depth (default 2.2)
  -foam float
        how easily water goes white (default 0.5)
  -foam-life float
        steps a bubble survives (default 7)
  -format string
        output format: png|jpg (default "png")
  -glint float
        angular width of a sun glint, radians (default 0.11)
  -ground string
        overlay preview ground: transparent|gray-light|gray-mid|gray-dark (default "transparent")
  -height int
        override height in px (requires --width)
  -light string
        the sun on the water: high|low|overcast|dappled (default: from seed)
  -meander float
        swings down the height of the frame (default 0.9)
  -medium string
        rendering medium: river|overlay (default "river")
  -milk float
        how far the water's own colour is lifted toward light
  -out string
        output directory (default "out")
  -overlay-alpha float
        overall opacity of the water layer (default 0.28)
  -overlay-dots float
        strength of washed-out boulder dots (default 0.22)
  -overlay-ripples float
        strength of current streaks and wave facets (default 0.8)
  -overlay-shadows float
        strength of broad shadow patches (default 0.35)
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -pebble float
        gravel scale, canvas units (default 0.006)
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -reach string
        the energy of the stretch of river: pool|glide|run|riffle|rapid|cascade (default: from seed)
  -riffle float
        amplitude of the pool-riffle sequence (default 0.5)
  -riffle-wave float
        pool-riffle sequences down the frame (default 1.4)
  -rock-size float
        typical boulder radius, canvas units (default 0.035)
  -rocks int
        boulders in the channel (default 7)
  -seed uint
        random seed (same seed → same image) (default 42)
  -sheen float
        broad reflected sky on the surface (default 0.08)
  -speed float
        mid-channel current (default 0.85)
  -step float
        time per step: a step is speed x this (default 0.006)
  -steps int
        steps of the upstream walk (default 20)
  -sun float
        sun azimuth, degrees (default 130)
  -sun-height float
        sun altitude, degrees (default 60)
  -taper float
        narrowing (negative) or opening (positive) downstream
  -turbulence float
        curl noise on the current (default 0.3)
  -wake float
        white water a boulder sheds (default 0.7)
  -water string
        how far light gets into the column, and what colour is left: clear|green|peat|glacial|silt (default: from seed)
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
