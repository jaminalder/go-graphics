# Hatching — `internal/hatch`

Filling a region with repeated marks, as a general facility rather than as
one sketch's fill. The companion specimen sheet is the `hatchbook` sketch;
`make hatchbook` renders it and writes the manifest that names every square.

---

## 1. The one idea

A hatch is a **coverage function**:

```go
type Func func(Sample) float64      // → how much ink is here, in [0,1]
```

That is all. Colour is the caller's business, which is what makes the
package general: a caller lerps toward one ink, or two, or uses the coverage
as a mask for a wash, or shifts hue along the marks — and none of that has
to be anticipated here. A hatch that returned a colour would be usable by
exactly one sketch.

It fits the repo's rendering model exactly. `render.PixelFunc` is a pure
per-pixel function, so a hatch is evaluated where the pixel is and nothing
has to be rasterised, buffered or stroked.

## 2. What a hatch is allowed to know

Several structures need more than a position: contour hatching needs the
distance to the boundary, radial and concentric need a centre,
boundary-to-boundary needs the region's extent, a density gradient needs
something to grade against. All of it arrives in one small struct.

```go
type Sample struct {
    U, V   float64 // the point, canvas units
    CX, CY float64 // the region's centre
    Axis   float64 // the region's own direction, radians (0 if it has none)
    Wall   float64 // distance to the nearest boundary (+Inf = unknown)
    Reach  float64 // the region's half-size: inradius, or half its width
    Tone   float64 // the value to encode, [0,1]
}
```

It is a bundle of numbers rather than a shape interface on purpose. A cell
of `internal/cells` fills it in directly — `Hit.Wall`, `Cell.CX/CY`,
`Cell.Inradius` — and so can a circle, a polygon, a quarter of a square, or
the whole canvas. `internal/hatch` does not import `cells` and must not: a
region is whatever the caller can describe this much of.

Only `U, V` are always required. A structure that needs a field it did not
get either falls back (`Reach` 0 is taken as half the canvas) or draws
nothing (`Contour` with `Wall` at +Inf).

## 3. The parameter vocabulary

One table of parameters serves every structure. This is the point of the
design: the knobs are the words a person uses about hatching, not one set
per structure.

| Parameter | Meaning |
|---|---|
| `Angle` | direction of the marks, radians (for `Chord`, the turn each chord makes around the boundary) |
| `Spacing` | pitch between marks, canvas units |
| `Fit` | *n* marks across the region instead of a fixed pitch — the alignment knob |
| `Thickness` | mark width **as a fraction of the spacing** |
| `Softness` | edge falloff, as a fraction of the half-width |
| `Curvature` | constant bend, in 1/canvas-units: arc radius is `1/Curvature` |
| `Waveform` | `Straight` \| `Sine` \| `Zigzag` |
| `Amplitude` | displacement across the marks, × spacing (also the strength of the flow field) |
| `Wavelength` | period of that displacement along the mark, canvas units (also the scale of the field) |
| `Continuity` | share of a mark actually drawn: 1 unbroken, 0.4 dashed |
| `Dash` | length of one dash-plus-gap, canvas units |
| `Jitter` | hand wander: per-mark displacement and width variation, × spacing |
| `ToneDensity` | how many times `Sample.Tone` may halve the density |
| `ToneWidth` | how strongly `Sample.Tone` drives the width |
| `Align` | `AlignCanvas` (one screen over everything) \| `AlignRegion` (each region its own) |
| `Seed` | drives jitter, dash phases and the noise fields |

Two of these carry a decision worth stating.

**Thickness is dimensionless.** It is a fraction of the spacing, which is
how an engraver thinks — the line-to-gap ratio *is* the tone — and it means
a hatch fitted to a small region gets proportionally finer marks instead of
turning solid as the region shrinks.

**A density gradient thins by halving, not by stretching.** A lattice whose
pitch varies with position has to split or merge its marks somewhere, and
both are visible. `ToneDensity` drops every other mark, then every other
survivor, fading the ones on the boundary between levels. Every surviving
mark stays exactly where it was at full tone, so a graded hatch does not
appear to slide as it lightens.

## 4. The structures

A structure is a **change of coordinates**, not a different kind of
hatching. Each maps a point to the same pair of numbers — how far *across*
the family it is and how far *along* its own mark — and everything after
that (width, waveform, dashes, thinning, dots) is shared.

| Structure | across | along | needs |
|---|---|---|---|
| `Parallel` | distance perpendicular to `Angle` | distance along it | — |
| `Contour` | `Wall` | angle round the centre | `Wall` |
| `Concentric` | radius from the centre | arc length | `CX, CY` |
| `Radial` | angle × `Reach` | radius | `CX, CY, Reach` |
| `Fan` | the angle subtended between two poles | log of the distance ratio | `CX, CY, Reach` |
| `Flow` | parallel coordinate + a Perlin stream function | parallel coordinate | — |
| `Scribble` | the stream function alone | parallel coordinate | — |
| `Stipple` | as `Parallel`, but dots on the (across, along) lattice | | — |
| `Chord` | *(special)* min distance to a ring of chords | | `CX, CY, Reach` |

Notes on the ones with real content in them:

- **Curved hatching** is not a structure — it is `Curvature` on `Parallel`.
  The across coordinate becomes `R − |P − C|` for a circle centre one radius
  off to the side, which expands to `w − a²/2R`: the marks keep their
  spacing and pick up a sagitta. As curvature goes to zero it returns the
  straight coordinate exactly.
- **Radial and fan wrap.** Their across coordinate is an angle, so unless
  the pitch divides the turn a whole number of times there is a seam where
  the last mark meets the first — a crack running out of the centre of every
  region. The pitch is quantised to the nearest exact fit.
- **Fan** puts two poles one `Reach` either side of the centre. The angle a
  point subtends between two fixed poles is constant along a circular arc
  through both, so the level sets really are a fan of arcs from pole to pole.
- **Flow** adds a Perlin field to the parallel coordinate and takes the
  level sets. The level sets of a stream function are exactly the
  streamlines of the divergence-free field `∇⊥ψ`, so these follow a field by
  construction rather than being straight lines wobbled to look as if they
  did: they never cross and never stop. **Scribble** is the same thing with
  the mean direction removed.
- **Chord** is boundary-to-boundary hatching: chord *i* leaves the boundary
  at angle `2πi/n` and arrives at `2πi/n + Angle`, so every mark runs edge
  to edge, the family closes, and its envelope is the circle of radius
  `Reach·|cos(Angle/2)|`. It is the one structure that needs the region to
  be roughly round — the honest cost of describing a region by a centre and
  a scale rather than by its outline.

### The field-steepness correction

`Flow` and `Scribble` need one extra step. Their across coordinate is a
noise field, not a distance, so the gap between consecutive level sets is
the pitch divided by the local gradient. Left alone, a mark whose width is a
fraction of *that* gap comes out as a blob where the field is slack and
vanishes where it is steep — the first render of the specimen sheet showed
exactly this. Dividing the distance by the gradient and leaving the width
alone gives marks of one width that converge and diverge, which is what
following a field looks like.

## 5. Composition

The remaining named kinds of hatching are combinations, and get combinators
rather than structures.

```go
func Over(fs ...Func) Func                      // layer, as transparency
func Cross(s Spec, angles ...float64) Func      // cross-hatching
func Weave(a, b Spec) func(Sample) (ca, cb float64)
func Nested(sub func(Sample) (Sample, int), inner ...Func) Func
func Mask(f Func, m func(Sample) float64) Func
```

- **`Over`** composites as transparency (`1 − ∏(1 − cᵢ)`), not by addition,
  so two families crossing give a crossing rather than a burnt-out patch.
- **`Cross`** gives each family its own seed; two families that jitter in
  step read as one hatch drawn twice.
- **`Weave`** returns what is *visible* of each family, so a caller can put
  them in different inks. Which thread is on top alternates with the parity
  of the two mark indices — the rule a plain weave follows. Anything else
  reads as two hatchings stacked, not as cloth.
- **`Nested`** takes a subdivision that **re-describes** the sample for the
  sub-region — a new centre, reach, wall and axis — and says which inner
  hatch fills it. Re-describing rather than merely choosing is the point: an
  inner hatch fitted to its sub-region has to be told what that region is,
  and only the caller knows.
- **`Mask`** confines a hatch to an outline the package cannot see.

Beyond the coverage itself:

```go
func (h *Hatch) Cover(s Sample) float64
func (h *Hatch) CoverLine(s Sample) (cover float64, line int)
func (h *Hatch) Coords(s Sample) (across, along float64, ok bool)
```

`CoverLine` gives the index of the mark, so a caller can put every third
line in a second ink. `Coords` gives the hatch's own frame, so a caller can
shift colour along the stroke direction. Both exist so that colouring stays
outside the package without being crippled.

## 6. Using it

```go
spec := hatch.Defaults()
spec.Structure = hatch.Contour
spec.Fit, spec.Align = 6, hatch.AlignRegion   // six marks per cell, however big
spec.Jitter, spec.Seed = 0.15, ctx.Seed

ink := hatch.New(spec)                         // resolve once, outside the loop

sketch.Raster(ctx, func(u, v float64) palette.Color {
    hit := foam.At(u, v)
    cell := foam.Cells()[hit.Cell]
    c := ink.Cover(hatch.Sample{
        U: u, V: v,
        CX: cell.CX, CY: cell.CY,
        Reach: cell.Inradius,
        Wall:  hit.Wall,
        Tone:  tone(u, v),
    })
    return palette.Lerp(paper, pigment, c)
})
```

`New` is not free (it builds a Perlin table), and a `*Hatch` is immutable
and its methods are pure, so build it once and let the whole parallel pixel
loop share it.

## 7. Invariants

- **Determinism.** Randomness is hash and Perlin lookups keyed on
  `Spec.Seed` and position — never a generator — so a `*Hatch` is pure and
  safe to call concurrently.
- **Resolution independence.** Every length is in canvas units, and
  `Thickness` is dimensionless. `hatchbook`'s
  `TestASheetIsIdenticalAtAnyResolution` renders each sheet at 240 px and at
  720 px, averages both to a common grid in linear light and requires them
  to agree to under a level out of 255 — the dithering and nothing else.
- **Dependencies.** `hatch → mathx, noise → stdlib`. `noise` is needed for
  the flow and scribble stream functions and for the jitter and dash-phase
  hashes; `mathx` for `Clamp01`/`Smoothstep`. Both are stdlib-only leaves
  themselves, so `hatch` sits alongside `geom` and `cells` in the leaf tier.

## 8. Known weaknesses

Honest list, in order of how much they bother me.

1. **Scribble reads as a contour map, not as a scribble.** Level sets of
   noise are nested closed loops that never cross, and a real scribble is
   one line crossing itself. It is a good marbled texture and it is not what
   the name promises. Doing it properly needs a *path* — a wandering line
   that revisits its own neighbourhood — which a coverage function evaluated
   at an isolated point cannot express without carrying the path with it.
2. **Chord hatching assumes a round region.** It spans the disc of radius
   `Reach`, so in a long or forked region the marks stop short of the real
   boundary at the ends and overshoot at the waist. Fixing it needs a
   boundary parameterisation in `Sample`, which no current caller could
   supply.
3. **Contour hatching pinches on the medial axis.** Where a lobed region's
   wall distance ridges, the contours crowd into a knot — visible in the
   `shapes` sheet. It is inherent to level sets of a distance field, and
   `foam` already works around the same thing by refusing its band fill to
   elongated cells.
4. **Jitter above about 0.3 merges neighbouring marks.** The lookup finds
   them (there is a test for exactly that), but two marks displaced toward
   each other genuinely overlap. It is a property of jittering a lattice,
   not a bug, and the specimen sheet shows where it starts.
5. **Thin marks below one pixel thin out rather than grey out.** The
   coverage is analytic with no notion of pixel size, so a mark narrower
   than the sampling grid is caught only by supersampling. This is the same
   trade the rest of the repo makes (`--aa 3` for print); a pixel-aware
   falloff would fix the look and break resolution independence.
6. **The steepness correction costs two extra field evaluations** per
   sample, on `Flow` and `Scribble` only. It is the largest per-pixel cost
   in the package.

## 9. How `foam` would adopt this

Not done — `internal/sketch/foam` is being worked on elsewhere — but the
shape of it is short. `foam`'s `pencil` and `hatch` styles are both its
private `strokes()`: one wobbled parallel family, angle quantised to a
twelfth of a turn, phase in canvas space, width and falloff hand-tuned per
style. That is `Spec{Structure: Parallel, Angle: …, Spacing: d.stroke,
Thickness: …, Jitter: …, Align: AlignRegion}` with `Cover` in place of
`strokes`, and the two styles become two specs rather than two functions.
`foam`'s `bands` style is `Structure: Contour` with `Fit` off the cell's
inradius, which is what `d.pitch` already computes by hand. The dressing
code keeps deciding pigment, style and angle per cell; only the mark-making
moves. The gain is not lines of code, it is that a foam cell could then be
filled with any of the other eight structures, and with tone, for free.
