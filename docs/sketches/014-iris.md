# 014 - Iris

## Purpose

`iris` is an abstract iris: one disc on a plain ground, filled with as much
structure as the field can carry. It takes 013's nested domain warping and
aims it at a subject, on the theory that a warp becomes a picture the moment
its coordinate system means something.

It is deliberately **not** a rendering of an eye. There is no cornea, no
specular highlight, no sphere, no lighting model of any kind — nothing in the
image is shaded by a light direction. Every value is the field itself, read
through a colour ramp. The composition is plain (a circle, a pupil, a rim) so
that all the interest has to come from the inside.

## The radial remap

The whole sketch rests on one coordinate change. For a canvas point `p`
measured from the centre, with unit direction `d = p/|p|` and normalized
annulus position `s` (0 at the pupil edge, 1 at the limbus), the fields are
evaluated not at `p` but at

```text
P(d, s) = d * scale * (ringBase + stretch * s)
```

That is: the plane is squashed radially onto a narrow band of noise space.
Two consequences make the sketch work.

1. **The field runs as fibres.** Moving outward along a ray changes the noise
   coordinate by only `stretch`, while moving around the disc traverses the
   full circumference `2*pi*rho`. The field therefore varies fast tangentially
   and slowly radially, which is what a fibre is. `stretch` is the single
   control over how far one thread holds together; `ringBase` fixes how many
   threads fit around the pupil before octaves subdivide them.
2. **There is no angular seam.** `P` traces a closed circle in noise space, so
   an ordinary 2D Perlin field evaluated there is continuous all the way round
   by construction. Nothing has to be wrapped, blended, or hidden — which is
   the trap in the obvious `fbm(angle, radius)` formulation.

`twist` rotates `d` by `twist * s` before the remap, so the fibres leave the
pupil as a vortex. It bends the field, never the geometry: the iris stays a
circle.

## Warping

Displacement is applied in the local frame, not in canvas axes. With tangent
`t = perp(d)`:

```text
W = P + t*(q1 * fiber) + d*(q2 * radial)
```

Tangential displacement bends a fibre sideways; radial displacement breaks it
along its length. Separating them is what stops a strong warp from dissolving
the radial reading into ordinary fBm mush. A second field pair displaces `W`
again by `nested`, in the same frame.

The `comb` is a third coordinate whose radius is fixed, independent of `s`, so
a field read there is constant along every ray: straight spokes converging on
the pupil, bent outward by the same tangential displacement the stroma uses.
Only `fiber` and `filament` use it.

## Structures

Four readings of the same warped point. They are traits, not modes: a seed
draws one.

- `fiber` — a broad body crossed by the comb, plus **strands taken from the
  zero set** of the field (`1 - |value|`) rather than from its peaks, which is
  what gives long meandering threads instead of blobs. The broad warp field
  doubles as mottling, keeping soft dark passages between the fibre bundles.
- `filament` — the level sets of the warped field, drawn as contour lines.
  Lines crowd where the field is steep and vanish where it is calm, so the
  stroma comes out as intricate whorls separated by plain passages. The one
  structure with no radial term at all: only the warp makes the contours run
  outward.
- `strata` — the radial coordinate itself, displaced by the field and
  terraced. Alternate rings sit high and low so neighbours always contrast,
  and the step edge is a drawn line. Reads as growth rings, or as a contour
  map of an eye.
- `crypt` — Worley cells in the warped space. The space is already stretched
  radially, so the cells come out as elongated lacunae; each takes one flat
  tone from its own identity, and the gaps draw the web.

## Colour

Both mappings need at least four palette colours.

Value carries the structure and hue follows it — radius only bends the result.
The palette is laid out on a **value ramp** whose stops sit at half their own
relative luminance and half an equal share. Pure luminance spacing starves a
palette whose colours cluster at one end; pure equal spacing blows the sheet
out to whichever single colour is lightest. The blend keeps mid-key structure
legible on every ColorLisa palette.

On top of that ramp:

- `depth` is a gamma on the value axis — it sinks the midtones without moving
  the crests, so a bright strand stays a strand;
- `warmth` pulls the pupillary zone toward an **accent** colour, chosen as the
  most saturated warm colour that is neither the lightest nor the darkest;
- `gleam` mixes the lightest colour into narrow crests, `shadow` mixes the
  darkest into crypts, risers and cell webs;
- a soft outer zone sinks into a crisp limbal ring, and a collar of shadow
  marks where the stroma meets the pupil. These are the only places radius
  overrules the field.

The pupil is dark but not dead: the same field the stroma is made of survives
inside it as a barely visible turbulence, and the aperture deepens toward its
own centre. A flat black disc reads as a hole cut in the picture rather than as
part of it.

The ground is never neutral. Every level carries the palette's own hue — a
tinted paper, a deep tone, or a receding mid — because a disc this saturated
sitting on undifferentiated white reads as a cut-out rather than as a picture.

`tint sector` adds a slow field read on a small circle — a function of angle
only — that slides whole wedges of the stroma along the value ramp, the way a
real sectoral heterochromia does.

## Output space

Seven weighted dimensions (`internal/trait`), resolved from the seed before
any number is drawn:

| dimension | values |
| --- | --- |
| `structure` | fiber, filament, strata, crypt |
| `weave` | radial, swirl, vortex |
| `grain` | broad, fine, dense |
| `reach` | long, broken |
| `aperture` | narrow, even, wide |
| `tint` | plain, sector |
| `ground` | light, dark, shade |

Each level resolves to *ranges*, not numbers, so two seeds sharing a level
still draw different irises. Every numeric knob (`--scale`, `--twist`,
`--fiber`, …) is an override that replaces exactly one drawn value and leaves
the rest of the draw alone.

## Determinism and resolution

All Perlin seeds derive from `Context.Seed` and fixed transforms; traits come
from stream 1 and the numeric recipe from stream 2, so adding a numeric range
later cannot move an existing seed's traits. The plan holds no pixel
dimensions — only the aspect ratio, which fixes the centre. Frequencies,
displacements and edge softness are in normalized canvas units, so equal-aspect
renders sample the same field at identical `(u,v)`.

The hot path is pure and allocation-free: no RNG draws, no option resolution,
no geometry construction. There are no normals and no derivative samples, so a
pixel costs a single pass through the field regardless of structure.

## Provenance

The subject was motivated by a well-known ShaderToy iris study, and the polar
framing is the obvious one for the subject. The mechanisms here are otherwise
independently designed for this repository: the radial remap and its seamless
consequence, the local-frame anisotropic warp, the fixed-radius comb, the four
structures, the luminance-blended value ramp and the trait space. No source
expression, constant sequence, or colour sequence is copied.

## Acceptance checklist

- The disc reads as an iris at a glance and as an abstract field up close.
- No highlight, no sphere, no light direction anywhere in the image.
- Fibres run pupil-to-limbus without an angular seam on the negative x axis.
- Detail survives at print resolution and does not alias at preview.
- Twist bends the field only: the disc stays circular and the rim stays crisp.
- All four structures are distinct enough to name from across a room.
- Sixteen consecutive seeds show visibly different irises, not one iris with
  different noise.
- Pinning one numeric flag does not disturb the rest of the draw.
- Palettes with clustered luminance still produce mid-key structure.
