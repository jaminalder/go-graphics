# Foam

## Implemented algorithm

A weighted curved cell partition is planned with merging/subdivision and per-region appearance. Sampling combines base cell fills, mosaic subdivision, hatch coverage, relief and ink edges. Watercolour is a field material evaluated inside the same region geometry; color schemes plan both hue and value before pixels.

## Controls and output space

Traits: colourway, density, lobes, fills, line, mosaic, relief and scheme. `--water` selects paint behavior and is a separate option. Watercolour fill is pin-only in the local weighted schema but explicitly admitted by the public Painted cells style.

## Rendering and review

Sampler; honors raster AA/deep. Verify clear cell boundaries, distinct fills and coherent color distribution; dense public-excluded choices remain local controls.

```sh
go run ./cmd/staticart render foam --seed 42 --profile preview --out out
```

## Implementation and tests

[Artwork implementation](../../internal/sketch/foam/foam.go) owns the mechanism and defaults. [Option declarations](../../internal/sketch/foam/options.go) own local knobs. [Trait schema](../../internal/sketch/foam/traits.go) owns levels, weights and pins. [Tests](../../internal/sketch/foam/foam_test.go) defend deterministic behavior and algorithm-specific claims. See [materials](../reference/materials.md) for shared rendering behavior and [CLI guide](../guides/cli.md) for profiles, metadata and batch review.

## Current command help

This block is generated from the checked-in application with `make docs-sketch-help`. It includes global flags because their interaction with each sketch matters.

<!-- sketch-help:start -->
```text
Usage of render:
  -aa int
        anti-aliasing samples (1 = off; use 3 for print); point samplers supersample per axis, flame multiplies the orbit budget (default 2)
  -accent float
        share of cells taking a colour from outside their passage (default 0.22)
  -bands float
        weight of the concentric band fill (default 2)
  -base float
        smallest site radius, canvas units (default 0.05309957938022502)
  -bevel float
        run that rise happens over, x the smallest cell (default 0.2)
  -blotch float
        strength of that unevenness (default 0.24)
  -colourway string
        which palette the sheet is drawn from: tchelitchew-hide-and-seek|kandinsky-apple-tree|matisse-collioure|klee-fire-evening|chagall-mariee|miro-woman-dog-moon|hockney-bigger-splash|delaunay-bleriot|gauguin-siesta|redon-green-vase|varo-harmony|seurat-grande-jatte|bruegel-icarus|cezanne-bathers|diebenkorn-seawall|hopper-night-windows|avery-bicycle-rider|sargent-carnation-lily|monet-water-lilies|vangogh-arles|from-flag (default: from seed)
  -count int
        sites the pack aims for (default 64)
  -deep
        render a 16-bit PNG master (archival/print; png only)
  -density string
        how finely the sheet is divided: sparse|open|medium|busy|packed|fine (default: from seed)
  -depth float
        rise of the relief, x the smallest cell (default 0.12)
  -dry float
        extra pigment gathered at a cell's edge as it dried
  -empty float
        weight of leaving a cell as bare paper (default 3)
  -fills string
        what the cells are filled with: washed|mixed|drawn|airy|net|watercolour|flat (default: from seed)
  -fine float
        inner net width, x the outer ink (default 0.33685544581524596)
  -flat float
        how far a flat fill deepens at full tone; 1 is no deepening (default 0.76)
  -format string
        output format: png|jpg (default "png")
  -gap float
        clearance between sites, x radius (default 0.1258668597458674)
  -grain float
        paper tooth (default 0.05)
  -hatch float
        weight of the hatched fill (default 1.5)
  -hatch-fit int
        marks across a cell, whatever its size (default 7)
  -hatch-pitch float
        mark spacing in canvas units, for looks that keep one fineness (default 0.0075)
  -hatch-press float
        how hard the marks are pressed (default 0.62)
  -hatch-vary float
        how irregular the marks are in spacing and in length (default 0.7)
  -hatch-weight float
        mark thickness, as a share of the spacing (default 0.34)
  -hatching string
        family of marks laid over each cell: none|parallel|cross|contour|wave|shade|dome|sphere|engrave|stipple|hollow|flow|weave|fan|spike (default "none")
  -height int
        override height in px (requires --width)
  -ink float
        wall thickness, canvas units (default 0.0063370804613122325)
  -light float
        the light's bearing in degrees; 90 is from the top (default 135)
  -line string
        the weight of the ink line: fine|drawn|bold (default: from seed)
  -load float
        pigment in a cell at full tone (default 1)
  -lobes string
        how many cells are lobes rather than bubbles: tidy|few|many|most|molten (default: from seed)
  -max-lobe int
        most sites one lobe may absorb (default 2)
  -merge float
        share of sites merged into a neighbouring lobe (default 0.2543406434438134)
  -mosaic string
        how the cells are subdivided into tiles, and where a tile's colour comes from: plain|family|strata|tonal|soloist|neighbour (default: from seed)
  -mottle float
        wavelength of the wash's unevenness, canvas units (default 0.22)
  -node float
        distance over which a third cell counts as near (default 0.01577070881418603)
  -out string
        output directory (default "out")
  -over float
        how far the pack reaches past the frame, canvas units (default 0.10019613019048078)
  -palette string
        palette slug (see: staticart palettes) (default "kandinsky-soft-pressure")
  -passage float
        wavelength of the colour field, canvas units (default 0.75)
  -pencil float
        weight of the pencil fill (default 5)
  -pool float
        wavelength of the pigment's pooling, canvas units (default 0.07)
  -profile string
        size profile: preview|preview-tall|print|print-tall|web|web-tall (default "preview")
  -ratio float
        size ladder step ratio (default 1.5657569712820856)
  -relief string
        how the sheet is lit: flat|bevel|cushion|occlude|terrace|glass (default: from seed)
  -rim float
        strength of the wash's dried rim (default 0.9)
  -rim-width float
        how far in from the wall the rim reaches (default 0.014)
  -round float
        radius a cell corner is rounded over, canvas units (default 0.011656247394238239)
  -rungs int
        steps on the site size ladder (default 4)
  -saturate float
        lift on every pigment saturation; 0 is the palette as mixed
  -scheme string
        how colour is organised over the sheet: passage|gradient|sequence|inherit|dominance|complement|analogous|triad|quiet|notan|terrace|anchor|weather|duet|by-size|by-darkness (default: from seed)
  -seed uint
        random seed (same seed → same image) (default 42)
  -shades float
        how far a cell wanders from its palette swatch; 0 is the bare palette (default 0.7)
  -spread float
        ramp steps a cell's family of tiles walks (default 2.289678257663641)
  -stroke float
        pencil stroke pitch, canvas units (default 0.0045)
  -swell float
        extra wall thickness at a junction, x ink (default 1.4954437903937836)
  -swirl float
        wavelength of that bending, x smallest cell (default 29.422759228019377)
  -tile float
        inner tile size, x the outer cell's inradius (default 0.2479624408652437)
  -tiled float
        share of cells subdivided into tiles (default 0.7393682823489838)
  -uneven float
        how strongly the pigment pools (default 0.75)
  -warp float
        how far the whole partition is bent, x smallest cell (default 0.8659488057063173)
  -wash float
        weight of the wash fill (default 6)
  -weight float
        how strongly a site radius bends the walls; 0 is straight (default 1.1)
  -width int
        override width in px (requires --height)
  -wobble float
        hand wander of the line and the strokes, x width (default 0.28)
```
<!-- sketch-help:end -->
