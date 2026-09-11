# Flame

## Implemented algorithm

A small nonlinear IFS iterates a deterministic chaos game into an integer density/color histogram. A fixed 48000-sample framing pass uses a separate stream from the quality-dependent main orbit. The developed image uses logarithmic density, optional flam3-style density estimation and oversampling/downsampling; wash medium develops pigment on paper.

## Controls and output space

Traits: structure, weave, reach, aperture, tint, ground, cast, medium and manner. `--quality` is samples per output pixel; AA multiplies the budget (minimum 1024 orbit samples). Oversample defaults to 2 and estimator maximum radius to 9 output pixels. `--cast from-flag` uses the supplied palette.

## Rendering and review

Histogram path; honors Deep. Never assess filament/grain quality at the generic 600-pixel preview: use 1000×1000 quality 80 for first review. Web/print profiles are for display/paper-size checks.

```sh
go run ./cmd/staticart render flame --seed 42 --width 1000 --height 1000 --quality 80 --palette zander-spindle --cast from-flag --structure spindle --tint split --ground void --out out
```

Visual source: [retained reference](../reference/apophysis-flame.jpg).

## Implementation and tests

[Artwork implementation](../../internal/sketch/flame/flame.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/flame/options.go) own local knobs. [Trait schema](../../internal/sketch/flame/traits.go) owns levels, weights and pins. [Tests](../../internal/sketch/flame/flame_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -aperture string
        how tightly the camera sits: tight|frame|wide (default: from seed)
  -brightness float
        mid-density lift before the log (default 1.05)
  -cast string
        which palette casts the flame: zander-spindle|diebenkorn-seawall|hopper-night-windows|cezanne-bathers|davis-anthracite-minuet|bruegel-icarus|from-flag (default: from seed)
  -de-curve float
        DE radius ~ 1/n^curve (default 0.4)
  -de-min float
        minimum DE radius (cores)
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -estimator float
        density-estimation radius in output pixels; 0 disables (default 9)
  -filter float
        Gaussian spatial-filter radius in output pixels (default 0.5)
  -format string
        output format: png|jpg (default "png")
  -gamma float
        lifts faint filaments; higher is noisier (default 3)
  -gleam float
        how far the densest cores move toward white (default 0.4)
  -ground string
        what the flame sits on: void|dusk|paper (default: from seed)
  -height int
        override height in px (requires --width)
  -manner string
        wash character when medium is wash: stain (default: from seed)
  -medium string
        how the attractor is developed: ember|wash (default: from seed)
  -out string
        output directory (default "out")
  -oversample int
        histogram resolution multiplier; 1 disables spatial AA (default 2)
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -quality float
        samples per output pixel (default 40)
  -reach string
        how wild the nonlinearities are: gentle|vivid|wild (default: from seed)
  -scale float
        multiplies the auto-framed camera (default 1)
  -seed uint
        random seed (same seed → same image) (default 42)
  -structure string
        the body of the flame: chaos|spindle|filament|bloom|spiral|fold|julia (default: from seed)
  -tint string
        how colour is assigned to maps: split|walk|stain (default: from seed)
  -vibrancy float
        1 keeps colour, 0 gammas each channel (default 0.88)
  -wash-body float
        FlatWash body for wash medium; higher is inkier, lower is a glaze (default from wash-sat) (default -1)
  -wash-pow float
        wash load curve; higher prints only dense cores, lower lets filaments land (default 0.55)
  -wash-sat float
        pigment saturation multiplier for wash medium (1 = raw mean colour) (default 2.5)
  -weave string
        how many maps speak: sparse|chorus|dense (default: from seed)
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
