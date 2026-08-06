# 015 — glaze

A bed of worn, faceted stones seen through water that is not simulated but
*painted*: a translucent blue veil whose whole structure comes from a nested
fBM domain warp. Where the veil thins the stones come up sharp; where it
thickens they sink into colour. It is a glaze, a marbling, a stack of flat
washes or a net of bright filaments depending on the manner asked for.

## Why a new sketch and not a `--medium` on 012

`shallows` has one identity and it is *clear river water*: a riffle surface
with a flow direction, a Froude number, ripple crests that cast paired shadow
and light, and refraction restrained so hard stone joints do not fold into
moire. Every one of its knobs is a fact about that model. The water here is a
different material with a different theory — it has no flow, no crests and no
physical slope; its light and dark come from a warped scalar field and its
"refraction" is a domain displacement large enough to smear a stone, which
012's spec explicitly rejects. Bolting it on as a second medium would give
`shallows` two mutually exclusive water models, two disjoint halves of its
control surface, and an acceptance checklist that contradicts itself.

The *bed*, on the other hand, is exactly the same object, so it is reused
rather than reinvented: `scree.Bed` is the documented pre-raster model the
architecture already blesses a second consumer for (decision 51). All of
scree's traits and material knobs come through untouched, so a bed found with
`shallows` renders identically under `glaze`.

Like `shallows`, the output-space identity stays the bed's: `Schema()`,
`Traits()` and `TraitSuffix()` delegate to scree. The water is a deliberate
choice of material, not a dimension a seed samples — the same argument
decision 21 makes for QQL's wash medium.

## The veil

One field, evaluated per pixel, pure and allocation-free:

```text
p  = scale * (u, v)
q  = (fbm(qx, p),            fbm(qy, p))
a  = p + warp * q
r  = (fbm(rx, a·rRole),      fbm(ry, a·rRole))
b  = (ax + nest*rx, ay + nest*ry) stretched anisotropically
raw, fine = fbm(body, b), its top two octaves
```

Each field is an independently seeded Perlin, and the running domain is
rotated by a fixed angle and scaled by the lacunarity after every octave so
that no two scales accumulate along the same axes. This is 013's vocabulary,
re-stated here rather than imported: `warp` exports an artwork, not a field,
and the constants here are this sketch's aesthetic policy (its role scales,
its shaping curves, its anisotropy) rather than 013's.

The sample the water hands back is small on purpose:

- `load` — how much pigment is over this point, in `[0,1]`;
- `filament` — a narrow response to the top-octave residual, `[0,1]`;
- `dx, dy` — the nested displacement as a unit-ish vector, so the caller can
  displace the bed lookup by a distance measured in canvas units.

`stretch` is applied to the *base* coordinate, before either warp, so every
later stage inherits it; the frame is also turned 22 degrees off the axes
first, because bands running exactly across the picture read as a machine's
output however good the ripple on them is. At 1 the veil is isotropic; above
1 its forms draw out into long parallel bands with the nested warp rippling
across them, which is what watered silk looks like.

The drift vector is read at its **own** frequency (`--drift-scale`, a
multiple of the veil scale) rather than from the warp fields that shape the
load. A displacement that varies only over the whole canvas *translates* the
bed instead of stretching it: at four times the drift the marbling was the
same picture, moved. Smearing a stone needs the offset to change appreciably
across one stone.

## Painting the water

The water is a **coloured medium**, not a sheet of tinted acetate. A straight
`Lerp` toward blue at alpha `load` averages the bed away; absorption in
linear light takes the bed's own colour *through* the water, so a thin
passage keeps its stone hue and its facets while a thick one loses the red
end first and sinks toward the water's colour. It also means the bed is never
fully hidden at `--body 0` however dense the veil gets, which is the
difference between a glaze and a lid. This is the pigment model of `paint`'s
washes (decision 39) with the layer count handed to it analytically.

The one deliberate untruth is where the extinction comes from. Taken straight
from the pigment, a pale swatch absorbs almost nothing and the veil vanishes
— the water would be as strong as the palette happened to be *dark*, which
has nothing to do with what colour the water is. So the pigment's strongest
channel is normalised to pass freely and only the ratios between its channels
carry hue; how much light the water takes overall is a separate neutral term
every channel pays alike, scaled by `--opacity`. `--body` then adds what the
medium scatters back toward the eye, which is what turns a deep glaze milky.

The load also carries a fine multiplicative speckle (`--grain-water`) at a
wavelength in canvas units. It multiplies rather than adds, so it disappears
where the water does instead of dusting the dry passages.

Order per sample:

1. read the veil at `(u, v)`;
2. sample the bed at `(u − drift·dx, v − drift·dy)`;
3. glaze the water colour over it at the veil's load;
4. mix the glint colour in where the filament response is high.

There is no intermediate image and no alpha channel; bed and water become one
colour before the pixel is written.

## The five manners

`--veil` is one knob because these are not five independent numbers, they are
five materials. Each resolves the field scale, both warp strengths, the
anisotropy, the opacity, the drift, the terrace count and the glint together;
every individual flag remains available as an override on what the manner
chose (`opt.Set.WasSet`).

- **`glaze`** — a smooth ramp from the warped field to pigment density. The
  water thins to nothing over some stones and banks up over others; drift is
  small, so the bed stays legible and the veil is pure value and hue. The
  default.
- **`marble`** — the same field, but a much finer displacement is taken up to
  several stone widths and applied to the *bed lookup*. Stone shapes stretch, swim
  and fold; joints become currents. Opacity drops, because the drawing is
  already doing the work. This is refraction as domain warp, deliberately far
  past what 012 permits.
- **`terrace`** — the density is quantised into a handful of plateaus with
  antialiased risers, so the water reads as flat overlapping sheets of blue,
  like layered glass or a screen print. The bed shows through each sheet at
  its own fixed strength.
- **`filament`** — a broad, thin body of blue with the top-octave ridged
  residual drawn over it as bright threads. The bed stays sharp between them;
  the glints are narrow and mostly absent, which is the only way a highlight
  reads as a highlight.
- **`silk`** — the anisotropic case: long drawn-out bands with the nested
  warp rippling across them. Watered silk rather than water.

## Colour

`--cast` picks the pigment out of the palette rather than inventing it
(decision 39). The base is the member with the most chroma around a marine
hue; `deep` mixes it toward the palette's darkest member, `pale` lightens it,
and `smoke` desaturates it *and* darkens it — desaturating alone all but
switches the water off, since the extinction lives in the channel ratios and
a neutral has none, so smoke gives up in hue what it takes back in depth. The
glint is the cast lightened, so highlight and body agree about the colour of
the water.

012 scores this on the raw channels, which is right for a clear film that
only has to be cool and bright, and wrong here: a near-white with a faint
blue bias scored well and then filtered nothing. Hokusai's cream beat its own
navy and *The Great Wave* came out bone dry.

## Controls

Every scree trait and knob (`--bed`, `--stones`, `--facets`, `--light`,
`--wet`, `--scheme`, `--gold`, …) plus:

- `--veil glaze|marble|terrace|filament|silk` — the material.
- `--cast cool|deep|pale|smoke` — which pigment the palette supplies.
- `--water-seed` — the veil's fields, independent of the bed.
- `--water-scale` in `[0.2,10]` — veil cycles per canvas unit.
- `--veil-warp`, `--veil-nest` in `[0,8]` — the two displacements.
- `--stretch` in `[0.25,6]` — anisotropy of the veil's forms.
- `--opacity` in `[0,1.2]` — the densest the water gets.
- `--body` in `[0,1]` — pigment opacity; 0 is a pure glaze.
- `--drift` in `[0,0.25]` — bed displacement by the veil, canvas units.
- `--drift-scale` in `[0.5,24]` — how finely that displacement varies,
  as a multiple of the veil scale.
- `--terraces` in `[2,12]` — plateaus when the manner terraces.
- `--glint` in `[0,1.2]` — strength of the filament highlight.
- `--grain-water` in `[0,0.5]` — paper tooth in the veil.

## Determinism and resolution

Every Perlin seed derives from `--water-seed` and a fixed transform, so the
bed and the veil are independently re-dealable and neither disturbs the
other. Scale, drift, stretch and the wash's tooth are all in canvas units;
the plan holds no pixel dimensions, so a preview and a print of one recipe
sample the same field.

## What did not work

**Taking the veil's structure straight from 013's defaults.** A base scale of
1.7 with warp strengths of 3 and 4 puts almost all of the field's energy at a
fraction of a stone, so the water came out as an even teal cast with a fine
swirl inside every stone and no composition at all. The water wants *large*
forms — scale below 1, displacements under 2.5 — and only then does the sheet
divide into deep passages and dry ones.

**Terracing a five-octave field.** The fine octaves cross every riser, so the
plateaus break back up into the gradient they were quantised out of. The
terrace manner runs three octaves; that one number is what makes it read as
flat sheets of glass.

**An ungated ridge for the filaments.** The residual is everywhere, so drawn
everywhere it covers the whole frame in an even reticulation and reads as
etched glass or frost — the same failure 011 records for its caustic net.
The threads are gated by the water's own depth so they gather in the deep
passages and leave the shallows alone.

**Refraction from the warp fields that shape the load.** Those are broad by
construction, and a broad displacement translates the bed rather than
stretching it. Four times the drift gave the same picture in a slightly
different place.

## Acceptance checklist

- [ ] The first read is stone under coloured water, not a stone image with a
      blue rectangle over it.
- [ ] The veil has visible *structure* — currents, folds, bands — rather than
      an even tint.
- [ ] Stones, joints and facets stay identifiable where the veil is thin.
- [ ] The five manners are different materials at thumbnail size, not five
      settings of one.
- [ ] `--water-seed` moves only the water; `--seed` re-plans the bed.
- [ ] Preview and print of one recipe show the same composition.
