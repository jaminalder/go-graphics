# 011 — riffle

A small river seen from directly above. Riffles breaking where the bed
shallows, glassy tongues over the deep, foam lines and bubble trails carried
downstream, eddies curling behind boulders, sunlight refracted into a caustic
net on the gravel, and the dark of a pool against the bright of a run.

## Why this sketch

Everything before it draws marks on a sheet or divides a sheet into regions.
This one draws a **material**. There is no plan, no scatter, no partition —
the whole picture is one pure function of position, and every cue in it comes
out of three scalar fields (depth, velocity, surface) composed per pixel.

That is the repo's oldest idea taken as far as it will go, and water is the
right subject for it: a river seen from above has no objects in it. It has a
bed, a flow, and a surface, and everything the eye reads — the foam line, the
eddy, the tongue, the caustic — is one of those three fields showing through
another.

It is also the first sketch that needs a **streamline**. A flow field has
been in the repo since sketch 004, but drift *walks* it and stamps dots along
it; the walk is part of the plan. Here the walk happens inside the pixel
function: every pixel asks where its water came from and looks upstream. That
is line integral convolution, and it is the single most legible "this is
moving water" cue there is.

## The reference

A trout stream in summer, photographed from a bridge. Three or four metres
across, gravel bed, half a dozen boulders, the sun high enough to put
caustics on the bottom. What makes such a photograph readable at a glance,
in order of how much work each does:

1. **Tone follows depth.** A pool is a dark shape; a run over gravel is a
   bright one. The picture is composed of those shapes before it is composed
   of anything else.
2. **White water is directional.** Foam is not scattered; it is drawn out
   into lines that all agree about which way is downstream.
3. **Boulders have wakes.** A rock with a white pillow upstream and a curl of
   foam trailing behind it is a river; the same rock in still water is a
   pond.
4. **The bed is visible where it is shallow and gone where it is deep**, and
   the crossover is what the eye reads as depth.

Anything that does not serve one of those four is decoration. The caustic net
and the sun glints are decoration, and they are wonderful, but they go on
last.

## This is not a simulation

No shallow-water solver, no particles, no time. Every field below is a closed
form evaluated at a point, chosen because it *reads* as the thing it stands
for. Where the accurate answer and the legible one differ, this sketch takes
the legible one, and each of those places is called out under **The
approximations** below.

## Algorithm

### 1. The channel

A centreline `axis(v)` wandering down the frame — a sine of a drawn amplitude
and wavelength plus a low-frequency fBm wobble so no two bends are the same —
and a half-width `halfWidth(v)` that may taper. Across-channel position is
`x = (u − axis(v)) / halfWidth(v)`: 0 on the deepest line, ±1 at the bank.

The centreline's *slope* is used twice more, so it is returned with it: it is
the flow direction, and it skews the cross-section.

### 2. The bed, and therefore the depth

```
depth = deep · cross(x) · grade(v) + dunes − rocks
```

- **`cross(x) = 1 − (x − skew)²`** — a parabolic section. Negative outside the
  banks, which is what makes dry gravel possible without a second field: land
  is water of negative depth. `skew` displaces the deep line toward the
  *outside* of a bend, proportional to the centreline slope, because that is
  where a real channel scours its pool.
- **`grade(v)`** — the pool–riffle sequence: a sine down the length of the
  frame, warped by noise so the crossings are not evenly spaced. This is the
  single most important structural term in the sketch. It is what puts a dark
  band across the picture and a bright one below it, and it is what makes the
  water *break* somewhere in particular rather than everywhere.
- **`dunes`** — two octaves of fBm at a fraction of a channel width, so the
  bed is irregular and the riffle crest is a ragged line rather than a
  contour of a sine.
- **`rocks`** — each boulder lifts the bed by a smooth bump. A bump taller
  than the local depth breaks the surface and the rock is dry.

Depth is in extinction units, not metres: 1 is "deep enough that the bed is
nearly gone". That is the only unit the picture cares about.

### 3. The flow

A velocity vector, not an angle. Built from four terms that are each about
one visible thing:

- **The current.** Direction is the channel tangent `(axis′(v), 1)`
  normalised. Speed is `speed · profile(x) / grade(v)`: fastest in mid
  channel, dying to nothing at the banks, and **faster where the pool–riffle
  term shallows**. That last is what makes a riffle a riffle.
- **Boulders deflect.** Each rock contributes the exact potential-flow dipole
  of a cylinder in a uniform stream, rotated into the local flow direction:
  the water slows to a stagnation point on the upstream face and accelerates
  past the shoulders. Two lines of arithmetic for the single most convincing
  thing in the picture.
- **Boulders shed.** Potential flow has no wake — d'Alembert's paradox — so
  the eddy is put in by hand: a counter-rotating pair of point vortices a rock
  radius apart, planted a radius and a half downstream, with a Gaussian decay.
  Between them the flow runs *back upstream*, which is exactly what the water
  behind a rock does and what makes the foam sit there instead of leaving.
- **Turbulence.** Curl noise at two wavelengths, its amplitude scaled by the
  local Froude number, so the glassy water stays glassy and the broken water
  is shredded. Curl rather than an angle field because it is divergence-free:
  an angle field has sinks, and streamlines walked into a sink all arrive at
  the same place, which reads as hair rather than as water.

### 4. The walk

The heart of the sketch. From each pixel, step **upstream** a fixed number of
times, each step `−v(q)·dt`, so a step is long in fast water and short in
slow. Along the way accumulate two things:

- **the streak** — a fine noise field, averaged over the whole walk. This is
  line integral convolution: an isotropic texture averaged along a streamline
  comes out as streaks that are noise-width across and walk-length along, and
  it is what makes the surface read as moving at all. Because the step is
  `velocity × dt`, streaks are *long on the tongues and short in the slack
  water* for free. The step has to stay well under the texture's own
  wavelength — at a step near the wavelength the samples are independent and
  the average is white noise.
- **the foam** — the foam source at each visited point, weighted by
  `exp(−age/life)`, so foam is born upstream and fades as it travels. Where
  the flow stalls — in an eddy, against a rock — the walk stalls with it and
  the same source is counted many times, which is why foam piles up in
  exactly the places foam piles up.

The walk is one-sided (upstream only) on purpose. It halves the cost, and it
is the honest direction: every property of a piece of water surface is a fact
about where that water has been.

**Chop is a separate field, and that cost a rewrite.** The surface slope was
originally taken as the difference between the walk's first two samples —
free, and wrong. Those two samples are a *streak* wavelength apart, so the
ripples came out at the streak's scale: long dashes, laid in rows by the
convolution, which read as basketry. Ripples and streaks are two scales of
one surface and they need two fields. The chop is its own fine noise,
differenced along the flow at the pixel (two extra samples, not two per
step), and gated by the Froude number so a glide is glass and a riffle is
broken.

### 5. Foam

Foam is born where water breaks, and water breaks at high **Froude number**:
`F = speed / √depth`. That one expression does most of the work in this
sketch — it *automatically* makes the riffle break on its crest and the pool
stay smooth, with no threshold on depth or speed alone. A second source sits
in a short tongue behind each boulder, because the whitewater there comes
from a plunging jet that no surface field knows about.

The threshold on F is **relative to the reach's own nominal Froude number**,
not absolute, and that is the single correction this part of the sketch
needed most. Depth and speed are in arbitrary units, so an absolute threshold
that left a pool clean turned a riffle entirely white and one that suited the
riffle left the rapid untouched. Measured against the reach's own average,
"breaking" means what it should — locally much faster or much shallower than
the water around it — and how much of that a level tolerates is what the
`foam` knob says. `TestFoamStaysOffMostOfTheSheet` fences the regression.

The source is multiplied by a patchy low-frequency field (so foam runs in
lines rather than covering the surface evenly) and by a **lace**; advected,
the lace comes out as trails, which is what it looks like from a bridge. The
lace is noise, not a Worley dot field: a regular lattice pushed through the
shear behind a rock comes out as a set of nested arcs — a comb, not a bubble
trail.

### 6. The bed's colour, and the water over it

Composed as a stack, bottom up:

1. **Gravel.** Worley cells at pebble scale, each pebble taking a tone from a
   hash of its cell id, with the cell boundary darkened — plus a broad fBm
   mottle so there are patches of coarse and fine. Two palette colours.

   The weights are low on everything cell-shaped and high on the mottle, and
   that is a correction rather than a taste. Gravel is genuinely cellular,
   but a Worley diagram with its walls drawn *and* its cells shaded is very
   cellular, and seen through half a metre of water the first version came
   out as tooled leather over the whole sheet — the strongest texture in the
   picture, at a scale belonging to nothing in it.
2. **Caustics.** A Worley `f2−f1` net evaluated at a domain-warped
   coordinate: the warp is what turns a cell diagram into the folded,
   pinched, brightest-at-the-cusps net that light actually makes. The net's
   cell size grows with depth and its contrast falls, so it is sharp in the
   shallows and gone in the pool without being masked.

   The threshold on `f2−f1` is the net's **line width**, and it is the
   parameter this sketch got wrong for longest. Wide, it lights a third of
   the sheet — the same tooled-leather failure as the gravel, from the other
   direction. A caustic is a *filament*: bright, narrow and mostly absent,
   with a gate that closes it entirely below the shallows, because a caustic
   still legible in a pool is the tell that it was painted on rather than
   cast.
3. **Refraction.** The bed is sampled at a point displaced along the flow by
   `depth × surface slope` — the deeper the water, the more the bed wobbles
   under the surface waves. One multiply, and it is the difference between a
   bed lying under glass and a bed lying under water.

   And the bed goes **soft** with depth, because light scattered on the way
   down and back has been spread sideways. Mixing toward the gravel's own
   mean rather than blurring costs one lerp and does the same job: crisp
   stones in the shallows, a smooth wash in the pool, and the crossover
   between the two is a large part of what reads as depth.
4. **The water column.** Per-channel Beer–Lambert in linear light:
   `out = bed·e^(−k·depth) + body·(1 − e^(−k·depth))`. `k` differs per channel,
   which is the actual reason water has a colour, and `body` is what is left
   when the bed is gone — the pool's own colour. Absorption is radiometric,
   so it happens in linear light (decision 13).
5. **The surface**, on the normal `(−slope·direction, 1)`, in four terms:
   the ripples shaded by the sun (a smooth signed term — the specular alone
   is a threshold, and a threshold on a noise field is salt and pepper); the
   streaks as a tonal veil along the flow; a small tilt-dependent reflection
   of the sky; and a glint.

   The reflection is deliberately tiny. Seen from straight above, water
   reflects a couple of percent — the first version's flat 20% lerp toward a
   pale sky turned every pool into milk.

   And the glint is an **angular window**, not a power of a cosine. Raised to
   a power, a flat surface under a sun 60° up still returns a third of full
   brightness, because the half vector is only 15° off vertical: the whole
   river lit up. A gaussian in the angle between the normal and the half
   vector fires only where a facet actually tips far enough to throw the sun
   into the lens, which is what sun glitter is.
6. **Foam**, laid over everything, near white.

Dry gravel is the same stack with the water skipped, lightened and
desaturated — dry stones are pale — with a **damp band** just above the
waterline where the gravel is at its darkest. That band is most of what makes
an exposed bar read as an exposed bar.

## The approximations

Each of these is somewhere the physically right answer was tried, or was
obviously available, and the cheaper one reads better.

| instead of | this sketch does | why |
| --- | --- | --- |
| continuity (`speed ∝ 1/depth`) | speed follows the *along-channel* term only, with a separate cross-channel profile | Continuity across the section makes the shallow margins the fastest water on the sheet, and a river that runs fastest up its own banks reads as a mistake before it reads as anything else. Splitting the two lets the riffle run fast *and* the banks run slow, which is what a river actually does — for reasons (friction, slope) that are not worth modelling. |
| a wake solver | a hand-planted counter-rotating vortex pair | Potential flow gives a beautiful deflection and no wake at all; the wake is the whole point of a boulder. Two vortices and a Gaussian give the recirculation, the reversal, and the shear line, at four multiplies. |
| divergence of the flow as the foam source | Froude number plus a wake cone | Divergence needs four extra field evaluations *per step of the walk*, which quadruples the sketch's cost, and it puts foam in the convergence lines only — the riffle crest, where most white water actually is, has no particular divergence. `speed/√depth` is one square root and is the textbook criterion for breaking. |
| integrating streamlines and stroking them | line integral convolution per pixel | A stroked streamline is a shape, with a start, an end and a width, and a field of them reads as hair. LIC has no shapes in it: it is the flow's own texture, exact at any resolution, and it keeps the sketch a pure per-pixel function (invariant 2 for free). |
| refracting light through the surface for caustics | a domain-warped Worley `f2−f1` net | Real caustics are the folds of a light-map, which needs the surface's full gradient and a projection. A warped cell diagram *is* a fold pattern — the warp is what makes the cusps — and it lands within a hair of the right look for a Worley lookup and two noise samples. |
| the full surface gradient | the along-flow derivative only | A riffle's standing waves have their crests **across** the flow, so nearly all the slope is in the along-flow direction. The walk has that derivative already, as the difference between its first two samples. The cross-flow component would cost two more walks. |
| a scattering solve for the water colour | per-channel Beer–Lambert to a body colour | The same maths the wash model already uses (decision 39), and it gives both cues at once: the bed fading out with depth, and what remains being the water's own colour. |
| blurring the bed under deep water | mixing it toward the gravel's own mean | A blur needs neighbours, and this sketch has no neighbours — it is one pure function of a point. One lerp toward a constant gives the same reading of "detail dies with depth", which is the only part of a blur the picture uses. |
| equal weight on the two turbulence scales | 86% broad, 14% fine | Curl noise returns a gradient in *noise-space* units, so both scales come back at much the same amplitude while the fine one has four times the spatial frequency and therefore four times the shear. Given equal weight it curls every streamline into a closed eddy a thirtieth of the frame across, and a convolution along closed eddies is a field of cells. |

## What did not read as water

Kept here because each was tried, looked at, and thrown away, and because
every one of them is a plausible thing to try again.

- **A caustic net at a comfortable line width.** The most expensive mistake
  in the sketch. A `f2−f1` threshold wide enough to light a third of the
  sheet gives a pale reticulation over everything at one contrast, and the
  whole picture reads as **tooled leather**. It survived four rounds of
  looking because it was blamed on the flow field, then on the convolution,
  then on the refraction; a two-by-two sweep varying `caustic` and `pebble`
  found it in one render. Cell-shaped textures are dangerous exactly because
  they are convincing individually.
- **The same failure from the gravel**, for the same reason: Worley cells
  with their walls drawn and their interiors shaded.
- **Taking the ripple slope from the convolution's own samples.** Free, and
  it produces ripples at the streak's wavelength, laid in rows — basketry.
- **Absolute Froude thresholds for foam.** Either a clean rapid or a river
  under a white sheet, with nothing in between across the reach axis.
- **Speed from continuity (`∝ 1/depth`).** The banks come out as the fastest
  water in the frame.
- **A rock plume as wide as its rock and seven radii long.** Advected, it
  fills a tenth of the frame: a boulder trailing a comet.
- **Blinn–Phong with a hard exponent for the glints.** From directly above
  with a high sun, a flat surface is already close to the specular peak, so
  the exponent lights the entire river rather than picking out facets.

## Traits

| dimension | key | values (weight) |
| --- | --- | --- |
| `colourway` | c | 17 curated palettes, **hokusai-great-wave 3**; `from-flag` 0 |
| `reach` | r | pool 2 · glide 3 · **run 3** · **riffle 3** · rapid 2 · cascade 0 |
| `channel` | n | straight 3 · **bend 4** · chute 2 · bar 2 · braid 1 |
| `boulders` | b | clear 1 · few 3 · **scattered 3** · field 2 · ledge 1 |
| `water` | w | **clear 3** · green 3 · peat 2 · glacial 1 · silt 1 |
| `light` | l | **high 4** · low 2 · overcast 2 · dappled 2 |

`reach` is the energy of the stretch, and it is one dimension because it is
one decision: depth, current speed, the amplitude of the pool–riffle
sequence, the turbulence, the chop and the foam thresholds all move together.
Setting them separately gives water that is deep and slow and covered in
white — which is not a river, it is five knobs. Like 008's `fill` and 009's
`density`, a level resolves to *ranges*, so two `riffle` seeds are two
different riffles.

`channel` is the plan form: where the water is and where it is not. `bar` and
`braid` are the two that expose dry gravel, and they are what make the frame
read as *a* river rather than as water.

`boulders` is count and size together. `ledge` is the outlier worth having: a
line of rock across the channel, so the whole frame is divided by one white
horizontal — the strongest composition the sketch makes.

`water` is turbidity: how fast light dies in the column, and what colour is
left when it has. `clear` shows the bed nearly everywhere; `peat` is a
brown-black hill stream where only the shallowest gravel shows; `glacial` is
milky, so the *surface* carries the picture and the bed is gone.

`light` decides whether there are caustics at all. `overcast` has none, and
the sheet is carried by tone and foam alone — it is the honest test of
whether the depth field is doing its job. `dappled` puts a low-frequency
shade mask over the sun, so caustics come and go in patches, which is what a
tree-lined stream does.

`cascade` (weight 0) is past what this vocabulary is for: everything shallow,
fast and white. It is one flag away and it is the only way to see what the
foam model does at saturation.

```sh
staticart sweep riffle --seeds 1-12 --profile web
staticart sweep riffle --seeds 1-6 --vary reach=pool,riffle,rapid --profile web
staticart render riffle --boulders ledge --light dappled --seed 7
```

## Tunables

Every one of these is an *override* on what a trait drew: a knob left alone
is the seed's, not the table's, which is what `opt.Set.WasSet` is for.

| flag | what it does |
| --- | --- |
| `--depth`, `--riffle`, `--riffle-wave`, `--dune` | the bed, overriding what `reach` drew |
| `--speed`, `--turbulence`, `--chop` | the current, its noise, and the surface wave height |
| `--rocks`, `--rock-size` | the boulder pack, overriding `boulders` |
| `--wake`, `--eddy` | how much foam a rock sheds and how hard it spins the water behind it |
| `--channel-width`, `--bend`, `--meander`, `--taper` | the plan form, overriding `channel` |
| `--steps`, `--step` | length of the upstream walk: how far a streak is smeared |
| `--foam`, `--foam-life`, `--bubbles` | how easily water goes white, how long it stays white, lace scale |
| `--extinction`, `--milk` | how fast light dies with depth and how far the water's own colour is lifted, overriding `water` |
| `--caustic`, `--caustic-scale`, `--caustic-warp` | the net on the bed |
| `--sun`, `--sun-height`, `--glint`, `--sheen`, `--dapple` | where the light comes from, how tight the glints are, and how much sky the surface returns |
| `--pebble` | gravel scale, canvas units |

The sketch's own flags share one FlagSet with the render command's, so none
of them may be named `profile`, `width`, `height`, `seed`, `aa`, `deep`,
`palette`, `format` or `out`. That is why the channel's half width is
`--channel-width` and the fluvial depth is `--depth` —
`TestFlagsDoNotShadowTheRenderFlags` keeps it that way.

## Acceptance checklist

- [ ] The picture reads as **water seen from above** before any detail is
      examined: large dark and light shapes that are depth, not marks.
- [ ] Every streak, foam line and bubble trail on the sheet agrees about
      which way is downstream.
- [ ] A riffle **breaks where it shallows** — the white is on the crest of
      the bed, not scattered evenly.
- [ ] Every boulder has a wake, and the foam sits *behind* it rather than
      passing over it.
- [ ] Foam is drawn out into lines and trails, never a uniform speckle.
- [ ] A glassy tongue exists somewhere: smooth, long streaks, no foam, bed
      visible through it.
- [ ] Caustics appear only where the water is shallow, and fade out — they
      are never cut off by an edge.
- [ ] On a `bar` or `braid` seed, dry gravel reads as dry: paler, no
      caustics, no streaks, with a dark damp band at the waterline.
- [ ] The bed under deep water is *gone*, not merely dimmed.
- [ ] Nothing tiles, and no Worley cell boundary is legible as a straight
      line anywhere.
- [ ] Preview and print of one seed are the same picture — the same streak
      lengths, the same pebbles, the same caustic cells.
