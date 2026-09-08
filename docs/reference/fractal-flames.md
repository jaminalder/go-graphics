# Fractal flames

Research note for sketch 016. Primary sources: Draves & Reckase, *The
Fractal Flame Algorithm* (2003, revised 2008) at
[flam3.com/flame_draves.pdf](https://flam3.com/flame_draves.pdf); the
`flam3` implementation; Wikipedia's algorithm summary, which disagrees
with the paper on a load-bearing colour detail and should not be copied.

The first visual target is Jonathan Zander's 2007 Apophysis render,
`docs/reference/apophysis-flame.jpg` (Commons:
[File:Flame_Apophysis_Fractal_Flame.jpg](https://commons.wikimedia.org/wiki/File:Flame_Apophysis_Fractal_Flame.jpg),
CC BY-SA). It is a look, not a genome we possess.

## What a flame is

A two-dimensional iterated function system: a finite set of maps
`F_i : ℝ² → ℝ²`. The picture is the attractor of the *chaos game* —
start at a random point in the bi-unit square, repeatedly pick a map by
weight, apply it, and plot. After ~20 iterations the point is on the
attractor for any system that is contractive on average. Sample count is
not a composition knob; it is how close the histogram is to the exact
measure.

Textbook IFS uses affine maps and paints binary membership. Flames
change three things, and Draves' stated design rule is the reason they
look like they do: **expose and preserve as much of the attractor's
information as possible**. Preserving information maximised aesthetics.

## The three innovations

### 1. Nonlinear variations

Each `F_i` is an affine transform, then a blend of named nonlinear maps
`V_j` called *variations*, then an optional second affine *post*
transform:

```
F_i(x, y) = P_i( Σ_j v_ij V_j(A_i · (x, y) + t_i) )
```

Variation 0 is the identity. Spherical inverts through the origin.
Swirl rotates by `r²`. Heart, polar, horseshoe, disc, spiral each stamp
a recognisable character on the attractor. JulianN (flam3 plugin 42) is
the filigree engine: a power-`n` Julia with a random branch, so one
point fans into `n` images and contractive copies reprint that nest at
smaller scales. A *final transform* is a non-linear camera: it is
applied after every `F_i` and is not fed back into the loop.

JulianN uses the plugin convention `θ = atan2(y, x)`, not the paper's
`atan2(x, y)`. The paper's polar angle still applies to the catalog
variations 1–20.

Useful systems are only contractive *on average*. Some genomes diverge.
A robust iterator resets NaN/Inf/huge points into the bi-unit square
and continues.

### 2. Log-density display

The attractor is a measure, not a set. Binary plotting throws away
every revisit. A linear histogram of those revisits is almost as bad:
IFS densities follow a power law, so the bright cores (thousands of
hits) force the filaments (tens of hits) to black.

The logarithm of the hit count is an ad-hoc HDR tone map. It is also
the source of the 3D illusion: where a dense branch crosses a thin one,
the thin contribution is lost in the sum and the dense branch appears
to occlude. Nothing in the algorithm is three-dimensional.

Wikipedia's renderer uses `log(freq) / log(freq_max)` as an alpha and
multiplies the stored colour by `alpha^(1/gamma)`. The paper is more
careful, and this is the version that matters:

- Accumulate **four** channels: add the current colour into RGB and `1`
  into α.
- After sampling, scale RGB by `log(α) / α`. Taking the log of each
  colour channel independently washes the palette out.

Gamma then lifts the filaments. Values around 2.2 are "correct" for
sRGB display; values around 4, which Apophysis artists routinely used,
are a visibility choice and they amplify Monte Carlo noise, so they
cost samples. *Vibrancy* (2001) blends two gammas: per-channel (pastel,
ghostly) against a single scale from α (saturated). The Zander image
is high-vibrancy.

### 3. Colour by structure

Colouring by density shows the measure and hides the functions. Colouring
by which `F_i` just fired makes the recursive construction visible, and
it is what keeps a region the same colour if the genome is later
animated.

Each map carries a colour coordinate `c_i ∈ [0, 1]`. The iterator carries
a third coordinate and blends `c ← (c + c_i) / 2` (flam3 later
generalises this to a colour speed). The most recent map dominates both
position and colour, so colour is continuous on the attractor. The
coordinate indexes a palette. In this repo the first-look ramp is
`zander-spindle`, sampled from the Zander render, not a ColorLisa
extract. ColorLisa palettes still run through the same mapper.

The mapper is RGB, not HSL. HSL between a cool swatch and a warm one
takes the magenta (violet→orange) or green (cyan→gold) trench; that is
why the first `klee-fire-evening` spindles came out magenta. White-hot
cores are gleam on the log-density, not a white palette stop — a white
stop paints the filaments silver. Flam3's 256-entry byte palettes are
the same RGB idea at a finer grain.

Symmetry maps must **not** touch the colour coordinate. They send a
point back onto itself; blending colour on those visits averages the
palette to mud. (Visible in the paper's Figure 5d.)

## Density estimation and quality

The chaos game is a Monte Carlo process. Quality is *samples per output
pixel*, so a print of the same genome is the same picture with finer
grain, not a different composition. Spatial anti-aliasing is cheap:
increase histogram resolution without increasing samples, then filter
down — or splat each hit with a bilinear kernel into the output-resolution
histogram, which buys sub-pixel coverage without a 4× buffer.

Low-density regions stay noisy. Flam3's answer is a variable-width blur
inversely proportional to local density (Suykens & Willems, WSCG 2000):
filaments soften, cores stay sharp. This repo implements that as a
Gaussian scatter after log-density and before gamma (`--estimator`,
default 9). Details and the flam3 formula:
[flame-xaos-de.md](flame-xaos-de.md).

Spatial anti-aliasing is a separate tool: accumulate at
`--oversample` (default 2) times the output resolution, then reduce
with flam3's separable Gaussian (`--filter`, default 0.5). `--aa` only
multiplies the sample budget; it does not grow the histogram.

Xaos (relative weights) is the other sequential structure: P(j|i) is
proportional to `weight[j] * xaos[i][j]`. Independent picks cannot make
"this filament only appears after that map". Spindle punches
julian-after-julian nearly to zero so contractive copies reprint the
nest.

## Aesthetics of the Apophysis era

The Zander image, and the mid-2000s Apophysis corpus it stands for:

- A **void ground**. The attractor is a luminous object in a black
  field, not a texture filling the rectangle. Paper or dusk grounds are
  possible and they are a different picture.
- **Filamentary structure.** Hair-like orbits, not filled regions. That
  comes from mixing spherical/swirl/heart with modestly contractive
  affines, and from log-density making the thin visits visible.
- **White-hot cores.** The densest folds clip toward white; colour lives
  in the mid-density ribbons. Gamma plus a small gleam toward white on
  the top of the density range.
- **Warm/cool split.** Orange-gold mass, electric cyan in the nest.
  Structural colour, not a spatial gradient: one map is assigned the
  palette's coolest swatch, another the warmest.
- **Pointed spindle, 2-fold.** A teardrop plus a 180° symmetry map is a
  spindle pointed at both ends. The paper's §7 is load-bearing here:
  the symmetry maps need weight equal to the sum of the others or one
  half is a ghost.
- **Fine arcs off the core.** Secondary loops from spherical inversion
  and a little post-transform scale, not extra objects.

What this is not: escape-time fractals (Mandelbrot, Julia sets as
pixel functions), nor a flow-field painting. Those are different
families and they do not share this pipeline.

## What this repo should not copy

- Electric Sheep XML genomes and the 256-entry byte palettes. Our
  output space is a trait schema; our colour is a named ramp (ColorLisa,
  or `zander-spindle` sampled from the reference).
- Bit-exact flam3. Same argument as QQL (decision 16): we want the
  vocabulary and the look, on our RNG, our seeds, our canvas units.
- A plugin registry of variations. One adapter is a hypothetical seam.
  Adding a variation is a code change in `internal/flame`.
- Per-pixel evaluation of the attractor. There is no closed form to
  sample. Forcing flames through `sketch.Raster` would mean a different
  algorithm.
