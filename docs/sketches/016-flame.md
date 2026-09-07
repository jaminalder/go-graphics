# 016 — flame

A fractal flame: a small iterated function system whose attractor is
drawn by the chaos game, shown with log-density so both the white-hot
cores and the hairline filaments survive, and coloured by the path that
produced each point rather than by density.

The first target is the look of Jonathan Zander's 2007 Apophysis render
(`docs/reference/apophysis-flame.jpg`): a vertical spindle of golden
filaments, a cyan nest at the crossing, pointed at both ends, on a void.
That file is a look, not a genome. We do not reproduce it pixel-for-pixel.

Theory, sources and the Wikipedia pitfall: [reference/fractal-flames.md](../reference/fractal-flames.md).

## Why a new lifecycle, not a new `At`

Every existing artwork is either a pure function of a coordinate or a
sequence of stamps on a canvas. A flame is neither. The interesting
object is a *measure on the plane*, estimated by iterating maps, and the
picture is a tone-mapped histogram of that measure. Asking `At(u, v)`
would mean a different algorithm (escape time, distance estimation) and
would not be a fractal flame.

So this sketch does not call `sketch.Raster`. It plans an IFS, iterates,
develops the histogram, and hands a colour buffer to
`render.ImageFromColors` — the same quantization path painters already
use. `--aa` is a samples-per-pixel multiplier, not a grid of function
evaluations. `--deep` still means a 16-bit master.

## What lives where

`internal/flame` is the mechanism: variations, xforms, xaos, the chaos
game, the four-channel histogram, log-density, density estimation, gamma,
vibrancy. Independently meaningful (it is a named published algorithm),
independently testable, and the performance story belongs there.

`internal/sketch/flame` is the policy: the trait schema, the genomes that
resolve those traits into an IFS, how a ColorLisa palette becomes a
1-D gradient, auto-framing taste, the void/dusk/paper grounds.

This split is decision 56. It is not a generic `HistogramRenderer` and it
is not a plugin variation registry. Adding a variation is a code change
in `internal/flame`. Adding a look is a code change in the sketch.

## Algorithm

```text
traits + overrides
  -> genome: weighted xforms (affine + variation blend + colour index)
  -> optional symmetry xforms (weight-balanced, colour-inert)
  -> xaos: per-predecessor relative weights (nil = independent picks)
  -> optional final transform (non-linear camera, not in the loop)
  -> Frame: short walk, percentile bbox -> Camera in math units
  -> Accumulate at oversample× resolution: 8 parallel orbits,
     bilinear splat into a shared uint64 histogram (r, g, b, α)
  -> Develop: log-density, optional density estimation (radius × ss),
     gamma, vibrancy, gleam, composite on ground
  -> Downsample: flam3 Gaussian spatial filter → output pixels
  -> render.ImageFromColors / ImageFromColorsDeep
```

Quality is samples per *output* pixel, scaled by `max(1, ctx.AA)`.
`--oversample` (default 2) only raises histogram resolution; it does
not multiply the sample budget. Preview and print of the same seed show
the same attractor; the print is the same measure with less Monte Carlo
grain. The camera is computed in the mathematical plane, never in
pixels, so invariant 2 holds even though the hot path is not `At`.

The worker count is a fixed 8, not `GOMAXPROCS`. A partition that
followed the machine would make the Monte Carlo realisation
machine-dependent and break invariant 1. Addition into `uint64` bins is
commutative, so merge order is free; atomics make the shared buffer
safe. Histogram memory is one 32-byte-per-pixel buffer at
`oversample²` the output area — at print (6000²) with oversample 2
that is about 4.6 GB, plus a matching float scratch for DE. Cap is 3;
use `--oversample 1` if the machine cannot hold it. Do not size the
histogram by `--aa` (a 3× print buffer is ~10 GB).

## The output space

Knobs that only make sense together are one trait. A pointed-both-ends
spindle *is* a 2-fold symmetry plus a teardrop variation plus a vertical
stretch; those are not three flags.

| Dimension   | What a viewer names        | Levels (weights)                                      |
|-------------|----------------------------|-------------------------------------------------------|
| `structure` | the body of the flame      | spindle (4), filament (3), bloom (2), spiral (2), fold (1.5), julia (1) |
| `weave`     | how many maps speak        | sparse (3), chorus (4), dense (2)                     |
| `reach`     | how wild the nonlinearities| gentle (2), vivid (4), wild (2)                       |
| `aperture`  | how tightly the camera sits| tight (2), frame (4), wide (2)                        |
| `tint`      | how colour is assigned     | split (4), walk (3), stain (1.5)                      |
| `ground`    | what it sits on            | void (6), dusk (1.5), paper (0.8)                     |

`spindle` is the Zander-like default mass: 180° symmetry, a low-opacity
JulianN nest (cyan), three anisotropic linear copies that draw strands
rather than packing area (gold), a vertical final post, and xaos that
keeps body→body low so black can show between hairlines. Mild isotropic
contraction plus DE was what made the sponge fill. At `weave=chorus` a
faint spherical adds equatorial arcs; at `dense` a second Julian branch
and a low-weight heart needle join. Heart is never the primary map. A
spherical *final* was tried and rejected. For open silk on a given
seed, prefer `--estimator 2–4` and a modest gamma (~2.3); the default
estimator 9 smooths cores but fills the gaps. `filament` drops the
required symmetry. `bloom` / `spiral` / `fold` / `julia` are the other
families.

`split` on spindle pins Julian maps to the cool end and body maps to the
warm end. Other structures still use order-based split (coolest on the
first creative map, warmest on a later one). The ramp itself is **RGB**,
not HSL between the two extremes: HSL from a cool swatch to a warm one
takes the magenta or green trench, which is why `klee-fire-evening`
rendered magenta. `walk` spreads colour indices along the palette order.
`stain` keeps them close, so the piece is almost monochrome with density
doing the work.

The first-look palette is `zander-spindle`, a five-stop ramp sampled
from the Zander render (cyan, pale cyan, gold, amber, burnt orange).
White-hot cores come from gleam on the log-density, not from a white
swatch: a white stop in the ramp paints the filaments silver. It is
not a ColorLisa extract; see the non-ColorLisa table in
[reference/colorlisa-palettes.md](../reference/colorlisa-palettes.md).
ColorLisa palettes still work — split sorts them cool→warm, RGB-lerps,
and no longer takes the HSL magenta trench. They still will not look
like the reference, because they do not contain cyan and gold.

`void` is black and is the Apophysis look. `dusk` is a very dark mix of
the palette's darkest swatch. `paper` is a light ground; the flame is
then a drawing, not a nebula.

## Overrides

`--quality` samples per pixel (default 40). `--gamma` (default 3.0, high
enough to see filaments). `--vibrancy` (default 0.88). `--brightness`
lifts mid-densities before the log. `--gleam` how far the densest cores
move toward white. `--scale` multiplies the auto-framed camera.
`--estimator` is the flam3 density-estimation radius in *output* pixels
(default 9; `0` disables). `--de-min` and `--de-curve` are the clamp
and the `1/n^curve` exponent (defaults 0 and 0.4). DE is a Gaussian
scatter *after* log-density and *before* gamma; see
[reference/flame-xaos-de.md](../reference/flame-xaos-de.md).
`--oversample` (default 2) is the histogram resolution multiplier;
`--filter` (default 0.5) is the Gaussian spatial-filter radius used
when downsampling to the output. `--aa` still only multiplies samples.

## Acceptance

A seed at `structure=spindle`, `tint=split`, `ground=void`, palette
`zander-spindle` should read as:

1. A luminous object on a black field, not an all-over texture.
2. Fine filaments, not filled blobs or a dotted spray.
3. A denser, brighter core where branches cross, with colour living in
   the mid-density ribbons and the core clipping toward white.
4. A cool (blue/cyan) region against a warm (gold/orange/red) mass —
   structural, not a spatial wash.
5. Some pointed or spindle-like termination, not a round puff.
6. Preview and print of the same seed show the same body; the print is
   not a crop and not a different arrangement.
7. Two runs of the same recipe are byte-identical, including across
   different `GOMAXPROCS`.

**Never judge a flame at 600px.** Filaments, grain and the core/haze
split are invisible at preview size and lie about quality. First-look
review renders — including agent Reads — are **1000×1000 at `--quality 80`**
(~10 s). Use `--profile web` (2000²) or `print` (6000²) only when checking
a candidate at display or paper size. Sweeps of a new genome start at
1000² / q80 as well.

Sweep 12 seeds before calling a genome change good. One pretty seed is
not evidence.

## Against the Zander reference

The 2007 Apophysis image is a *look*, not a genome we possess. The
first `klee-fire-evening` spindles missed it in two independent ways:

1. **Colour.** `klee-fire-evening` is violet, purple, fire-orange, red,
   maroon. Split took the coolest and warmest and HSL-lerped them. The
   short path from hue ~240° to hue ~15° is magenta. The reference is
   cyan nest, white-hot core, gold/amber mass. That ramp has to pass
   through white (saturation 0) in **RGB**, or any interpolation of
   cyan and gold goes green.
2. **Filigree.** Heart as the primary map, 180°-copied, is a filled
   teardrop of fuzzy hair — a beginner's flame. The reference's
   hairline structure is recursive: a branching nest (Julian with
   power 3–8) reprinted at smaller scales by contractive linear copies.
   Post-transform stretch makes the needle; it cannot be faked by
   stretching a blob after the fact.

`spindle` now starts from JulianN + two contractive linears, with xaos
forbidding nest-after-nest and favouring body↔nest recursion. Colour
roles are pinned under `--tint split`: nest cyan, body gold. A spherical
final was tried and rejected (central eye on every seed); the final stays
a vertical post on a linear camera. Heart is a low-weight extra at
`weave=dense`. This is still a sketch of that vocabulary, not Zander's
unknown `.flame` file.

## What is still missing for Apophysis-level quality

These are the remaining gaps between this engine and the reference's
*quality*, as opposed to matching one picture:

| Gap | What it buys | Status |
|---|---|---|
| **Xaos** (relative weights) | Nested "this filament only appears after that map". Independent picks cannot make sequential structure. | built: `System.Xaos`; spindle punches julian→julian to 0.05 |
| **Density estimation** | Variable-width kernel, inversely proportional to local density. Hairlines go smooth without `--quality` in the hundreds. | built: `--estimator` default 9, after log, before gamma |
| **Supersample then downsample** | Thin lines at 1000px still alias with bilinear splat at output resolution. | built: `--oversample` default 2, flam3 Gaussian spatial filter |
| **256-entry LUTs** | Flam3 palettes wiggle hue along the ramp. A 5-stop RGB ramp is the right *shape* and a coarser grain. | 5-stop `zander-spindle` |
| **Final nonlinear** | A spherical or Julian final is a second camera, not a stretch. | tried spherical; rejected (black-hole eye). Linear + vertical post |
| **More maps** | Typical Apophysis genomes run 6–12 creative xforms. We run 3–5 plus symmetry. | spindle: 3–6 creative (nest + copies + arcs) |
| **More variations** | blob, pdj, ngon, bipolar, perspective, … each a recognisable family. | add as a code change in `internal/flame` |

`--quality 80` at 1000² with oversample 2 and estimator 9 is the
first-look recipe. Grain on the faintest hairlines is still a
sample-budget question, not a genome one.

## What not to do

- Do not evaluate the attractor per pixel.
- Do not take the logarithm of R, G, B independently.
- Do not let symmetry maps blend the colour coordinate.
- Do not scale iteration count as a fixed N; quality is per pixel.
- Do not size the histogram by `--aa` at print (a 3× buffer is ~10 GB).
  Use `--oversample` (cap 3; print usually 2).
- Do not blur the linear histogram and then take the log; DE is after
  log-density, before gamma.
- JulianN uses the plugin `θ = atan2(y, x)`, not the paper's
  `atan2(x, y)`. Catalog variations 1–20 still follow the paper.
- Do not retune log/gamma/vibrancy from one seed. Sweep, then look at
  the faintest filaments *and* the cores.
