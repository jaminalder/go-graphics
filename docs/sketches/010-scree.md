# Scree

## Implemented algorithm

A weighted partition plans worn stones, each with a secondary facet partition. Wall distance determines relief and each facet receives one shade computed at its centroid against the chosen lamp. The exported concrete `Bed` combines immutable geometry and material sampling; it is reused by Shallows and Glaze.

## Controls and output space

Traits: colourway, bed, stones, facets, light, wet, joint and scheme. `--facets smooth` is the continuous-shading control; facet scale can vary between a common bed-wide grain and stone-proportional detail. Many numeric material/geometry overrides are local flags.

## Rendering and review

Sampler; honors raster AA/deep. Preserve flat-per-facet shading when reviewing changes; the smooth control demonstrates the distinction. Inspect facet size across large and small stones.

```sh
go run ./cmd/staticart render scree --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/scree/scree.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/scree/options.go) own local knobs. [Trait schema](../../internal/sketch/scree/traits.go) owns levels, weights and pins. [Tests](../../internal/sketch/scree/scree_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -accent float
        share of stones taking a colour from outside their passage (default 0.2)
  -ambient float
        how much light reaches a face turned away (default 0.4)
  -base float
        smallest stone radius, canvas units (default 0.04118962144220252)
  -bearing float
        the lamp's bearing in degrees; 90 is from the top (default 135)
  -bed string
        how coarse the bed is: boulders|cobbles|shingle|gravel|grit (default: from seed)
  -colourway string
        which palette the bed is drawn from: tchelitchew-hide-and-seek|kandinsky-apple-tree|cezanne-bathers|seurat-grande-jatte|gauguin-siesta|monet-water-lilies|sargent-carnation-lily|diebenkorn-seawall|redon-green-vase|matisse-collioure|hopper-night-windows|bruegel-icarus|klee-fire-evening|vangogh-arles|avery-bicycle-rider|varo-harmony|delaunay-bleriot|chagall-mariee|from-flag (default: from seed)
  -coolness float
        how far the shadowed side leans toward the sky's (default 0.34)
  -count int
        stones the pack aims for (default 110)
  -crease float
        how far a face darkens toward its own edge (default 0.14410641829733997)
  -cut float
        random tilt on each face; 1 is 45 degrees (default 0.09177422348798718)
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -depth float
        how far a stone's colour goes toward the water's own (default 0.12)
  -elevation float
        how high the lamp stands; low is dramatic (default 0.62)
  -facet float
        facet size, x the smallest stone (default 0.2961091206959952)
  -facet-scale float
        how far the grain follows the stone; 0 is one fineness for the bed
  -faceted float
        share of stones cut into facets (default 1)
  -facets string
        how finely each stone is cut into facets: plates|cut|crazed|shattered|smooth (default: from seed)
  -flake float
        random scaling on each face's shade (default 0.04646902410326698)
  -format string
        output format: png|jpg (default "png")
  -gap float
        clearance between stones, x radius (default 0.06940014480940054)
  -gloss float
        strength of the specular (default 0.16)
  -gold
        reserve yellow for two or three rare gold nuggets
  -grain float
        paper tooth (default 0.05)
  -height int
        override height in px (requires --width)
  -ink float
        the joint's thickness, canvas units (default 0.004191069917110888)
  -joint string
        the weight of the water between the stones: fine|drawn|bold (default: from seed)
  -light string
        how the bed is lit: raking|morning|noon|overcast (default: from seed)
  -load float
        pigment in a stone at full tone (default 0.95)
  -max-lobe int
        most sites one lobe may absorb (default 2)
  -merge float
        share of stones merged into a neighbouring lobe (default 0.2348122564901623)
  -node float
        distance over which a third stone counts as near (default 0.010953345210541253)
  -out string
        output directory (default "out")
  -over float
        how far the pack reaches past the frame, canvas units (default 0.07772266222614387)
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -passage float
        wavelength of the colour field, canvas units (default 0.8)
  -pool float
        wavelength of the pigment's pooling, canvas units (default 0.09)
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -ratio float
        size ladder step ratio (default 1.5174691227179813)
  -rise float
        how proud a stone stands, x its own inradius (default 0.58)
  -round float
        radius a stone's corner is worn over, canvas units (default 0.010001597763217377)
  -rungs int
        steps on the stone size ladder (default 5)
  -saturate float
        lift on every pigment's saturation
  -scheme string
        how colour is organised over the bed: dominance|passage|quiet|analogous|notan|anchor|inherit|by-size|weather|complement|by-darkness|duet|triad|sequence|gradient|terrace (default: from seed)
  -seed uint
        random seed (same seed → same image) (default 42)
  -shades float
        how far a stone wanders from its palette swatch (default 0.75)
  -sharp float
        how tight the specular is (default 26)
  -sheen float
        how much the water polishes it, x gloss (default 1)
  -soak float
        how much the water darkens a stone (default 0.5)
  -stones string
        how worn the stones are: worn|rolled|broken|jumbled (default: from seed)
  -swell float
        extra thickness where three stones meet, x ink (default 1.5471810748789447)
  -swirl float
        wavelength of that bending, x smallest stone (default 25.064750692464862)
  -uneven float
        how strongly the pigment pools (default 0.6)
  -warmth float
        how far the lit side leans toward the lamp's colour (default 0.3)
  -warp float
        how far the whole bed is bent, x smallest stone (default 0.9159488057063172)
  -weight float
        how strongly a stone's size bends its walls; 0 is straight (default 1.05)
  -wet string
        how much water is standing over the bed: dry|damp|wet|sunk (default: from seed)
  -width int
        override width in px (requires --height)
  -wobble float
        hand wander of the joint, x its width (default 0.24)
```
<!-- sketch-help:end -->
