# Iris

## Implemented algorithm

A radial remap sends direction around the disc to a seamless 2D noise field, stretching the field into fibres from pupil to limbus. Nested warping occurs in the local tangential/radial frame so strong displacement bends fibres. Value/color derive from the field; there is no lighting model.

## Controls and output space

Traits: structure, weave, grain, reach, aperture, tint, rim and ground. Concrete configs support public edition recipes. The public Fine/Winding/Layered styles pin subsets while numerical overrides remain local.

## Rendering and review

Sampler; honors raster AA/deep. Inspect angular seam continuity, pupil/limbus structure and fibre integrity. Geometry changes need equal-aspect comparisons.

```sh
go run ./cmd/staticart render iris --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/iris/iris.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/iris/options.go) own local knobs. [Trait schema](../../internal/sketch/iris/traits.go) owns levels, weights and pins. [Tests](../../internal/sketch/iris/iris_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -aperture string
        how open the pupil is: narrow|even|wide (default: from seed)
  -bands float
        strata terraces across the stroma, or filament contour levels (default 14)
  -cells float
        crypt: cell density across the stroma (default 12)
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -depth float
        gamma on the value axis: how far the midtones sink (default 0.45)
  -fiber float
        tangential warp: sideways meander of the fibres (default 0.55)
  -format string
        output format: png|jpg (default "png")
  -gain float
        amplitude multiplier per octave (default 0.5)
  -gleam float
        how far sharp crests brighten toward the light colour (default 0.16)
  -grain string
        how finely the stroma is divided: broad|fine|dense (default: from seed)
  -ground string
        what surrounds the iris: light|dark|shade|ink|black (default: from seed)
  -height int
        override height in px (requires --width)
  -lacunarity float
        frequency multiplier per octave (default 2.05)
  -limbus float
        iris radius as a fraction of canvas height (default 0.42)
  -nested float
        second warp displacement (default 0.5)
  -octaves int
        number of fBM components (default 5)
  -out string
        output directory (default "out")
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -pupil float
        pupil radius as a fraction of the iris (default 0.28)
  -radial float
        radial warp: interruption along a fibre (default 0.2)
  -reach string
        how far one thread holds together from pupil to limbus: long|broken (default: from seed)
  -rim string
        how the disc ends: soft|ring|halo|frayed|band (default: from seed)
  -rim-width float
        how far the disc's edge treatment reaches inward (default 0.3)
  -scale float
        field cycles around the stroma (default 1.9)
  -seed uint
        random seed (same seed → same image) (default 42)
  -stretch float
        how fast the field changes from pupil to limbus (default 0.5)
  -structure string
        what the warped field is read as: fiber|filament|strata|crypt (default: from seed)
  -tint string
        how hue is spread over the stroma: plain|sector (default: from seed)
  -twist float
        radians the fibres rotate between pupil and limbus
  -warmth float
        how far the pupillary zone pulls toward the accent colour (default 0.35)
  -weave string
        which way the stroma runs out of the pupil: radial|swirl|vortex (default: from seed)
  -width int
        override width in px (requires --height)
```
<!-- sketch-help:end -->
