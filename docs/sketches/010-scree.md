# 010 — scree

A river bed seen from directly above: worn stones packed edge to edge in
dark water, each one cut into a mosaic of flat facets, and each facet given
one shade of the stone's own colour by a single light standing over the
whole sheet.

## Why this sketch

Sketch 009 ends on a sheet that already reads as stones — the reference for
this one is a `foam` render, `--hatching spike --fills watercolour` on a
`fine` bed. What makes it read as stones is the hatching: the marks thin
toward the light and crowd away from it, so a flat tile comes up off the
page. That is an *engraver's* trick, and it has an engraver's limit — the
tone is carried by marks, so the stone is a drawing of a stone.

This sketch answers the same question with a surface instead of with marks.
Each stone is a dome; the dome is cut into facets; each facet takes **one**
flat shade from its own normal against one light. Nothing is hatched,
nothing is stroked, and the third dimension comes out of the same field the
walls do. The facets are the point: a smooth dome shaded continuously reads
as an airbrushed blob, and a facet with a hard step at every edge reads as
stone, because stone is what breaks into flat faces.

So the reference's spikes are gone. What replaces them is not another mark —
it is the thing the marks were imitating.

## The reference

`out/foam-spike/foam-htspike-…-d-fine-l-few-f-watercolour-n-drawn-s-dominance_…_23_2000x2000.png`.

What is kept: the packed bed with an order of magnitude of size between the
biggest stone and the smallest, the heavy dark joint that swells where three
stones meet, the polychrome palette held together by a colour scheme, and
the watercolour surface — paper tooth and an uneven wash, so the sheet reads
as a painted study rather than a render.

What is dropped: the hatching, and with it the whole `fills` axis. Every
stone here is painted and then lit. A bare cell is load-bearing on a drawn
sheet and meaningless on a river bed — there is no such thing as a stone the
water missed.

**A bed, not a wall.** The stones are rolled, not laid: worn round rather
than dressed, sizes graded rather than coursed, no shared straight edges and
no repeated shape. The corner rounding and the low merge share are both in
service of that — a stone with a corner reads as masonry, and two stones
that meet along a straight seam read as a joint someone cut.

## Algorithm

1. **The bed.** Circles packed by best-candidate darts over a geometric size
   ladder with an overscan, exactly as 009 packs its sites, and for the same
   reason: a stone at the border must be *cut* by the frame. Each circle
   contributes an additively weighted site — the Apollonius metric, so walls
   are hyperbolic arcs and a big stone claims more ground than the bisector
   would give it.

2. **The stones.** `internal/cells` over those sites, corners rounded hard.
   Rounding is what turns a polygon into a pebble, and this sketch leans on
   it further than 009 does: `worn` rounds by nearly a third of a stone.
   A small share of neighbouring pairs is merged into a lobe, and the whole
   partition is looked up through a curl-noise warp so that no wall is
   straight. All three are 009's, unchanged in kind.

3. **The facets.** A *second* partition over the whole sheet — one
   `cells.Foam`, not one per stone — with **weight 0** sites and corners
   barely rounded, so the inner metric is an ordinary Voronoi: straight
   bisectors, sharp corners, flat faces. Crystal inside organic.

   **The grain is one fineness for the whole bed**, sized off the *smallest*
   stone: a boulder is cut as finely as a chip, so it comes out with many
   facets and the chip with a handful. That is what rock does — the grain of
   a stone is a property of the rock it broke off, not of how big the piece
   is. It is also what the picture needs. The facets are here to describe a
   *surface*; scaled with the stone, facet size doubles as a second reading of
   stone size, the two cues confound each other, and a boulder ends up looking
   like a chip photographed from closer.

   `--facet-scale` recovers the proportional behaviour, which is what
   sketch 009's mosaic does — and rightly, because there the tiles carry a
   *colour walk* that needs a comparable number of steps in every cell. Same
   mechanism, different end. It runs as a power rather than a mix, because
   what is being interpolated is a ratio of lengths and the halfway point
   between "one fineness" and "proportional" is the geometric mean.

   Two partitions read at one warped point, the stone's ink drawn over the
   top afterwards: the heavy joint clips the fine facet net for free, and
   there is no clipping code anywhere. That is 009's mosaic, and it is
   reused here rather than reinvented because it was already the right
   answer.

4. **The surface.** The height of the sheet at a point is

   `h(p) = rise · span(p) · dome(wall(p), span(p))`

   where `wall` is the point's distance to its stone's boundary and `span`
   is that stone's inradius. `dome(t) = √(t(2−t))` — 0 at the wall, 1 at the
   deepest point, with the steep shoulder near the edge that makes a rolled
   thing look rolled. The rise is a *multiple of the stone's own span*, not
   a length on the page: pebbles are roughly self-similar, so a big stone
   stands proportionally as proud as a small one. A fixed rise makes the
   small stones domes and the large ones puddles.

   Nothing is modelled. The surface is a function of the same wall-distance
   field the walls are drawn from, so the stones' boundaries are the creases
   of the surface for free.

5. **The light.** One direction for the whole sheet, near enough to
   up-and-left that the eye reads bumps rather than holes. The normal of a
   height field is `(−∂h/∂x, −∂h/∂y, 1)` normalised, and the shade is
   `amb + (1−amb)·clamp(gain · N·L)`.

   Inconsistent lighting is what makes fake relief look fake. The eye
   forgives an impossible surface long before it forgives two shadows
   pointing different ways — which is exactly why 009's `spike` hatching
   takes one angle for the whole sheet, and the same rule is why there is
   one light here.

   **The gain is not a fudge.** Seen from above, a *horizontal* face receives
   only `lz` of the light, so under a raking lamp nothing anywhere on the
   sheet is brighter than half and the bed renders as a dark, low-contrast
   slab whatever the stones are painted. The diffuse is scaled so a flat face
   lands at a fixed 0.70 and faces tilted into the lamp clamp above it. The
   headroom matters as much as the lift: at 0.9 every facet on a dome's
   near-flat top clamps to the same value and each stone comes out with a
   bald plateau on it.

   The specular is taken against the **half-vector** between the lamp and the
   viewer, not against the lamp. From directly overhead, a face pointing at a
   low lamp is a face pointing away from the camera, and its gleam would
   never be seen — the highlight belongs where a face bisects the two.

   The gradient is capped. The dome's shoulder is vertical where it meets the
   wall, and a difference taken *across* a wall compares two different
   stones, so the raw slope is unbounded exactly at the boundary. Uncapped,
   the Lambert clamps to nothing all the way round the shadow side and every
   stone loses its colour into the joint before the joint is even drawn.

6. **Flat shading.** Here is the whole sketch. The shade is computed **once
   per facet**, at the facet's centroid, and held constant across it. Every
   facet edge is therefore a hard step in value, and the stone is a faceted
   solid rather than a gradient.

   Computed per pixel instead — which is one line's difference — the same
   surface, the same light and the same colours give a sheet of soft blobs.
   Flat shading is not an optimisation of smooth shading; it is a different
   picture, and it is the one that reads as rock.

   Each facet's normal is nudged by a small random tilt (`cut`), so the
   faces are cut rather than moulded, and its shade by a small random
   scaling (`flake`), so two facets on the same slope still differ. Both are
   small on purpose: pushed up they stop describing a surface and become
   noise, and the stone flattens out again.

7. **The joint.** The 009 line, unchanged: ink laid where
   `wall < ink/2 · (1 + swell · node)`, so the joint is thinnest halfway
   between two junctions and swells where three stones meet, with the edge
   softened by a fraction of the unswollen width. Graphite rather than a
   palette colour — a joint drawn in a pigment joins the composition and the
   bed stops reading as stones in water.

8. **Raster.** One pure pixel function, no stamps: preview and print of one
   seed are the same picture.

## What a facet straddling a joint does

The facets are a global partition and the stones are another, so a facet can
sit across a stone's wall. Its centroid is in one stone, and that stone's
normal is what its precomputed shade describes.

For colour that would not matter much — 009's mosaic keys a tile's colour on
the *pair* (stone, tile) for exactly this reason. For light it matters a
great deal, and in the worst way: the rim of one stone and the rim of its
neighbour, either side of one wall, have nearly **opposite** normals, so a
straddling facet would carry a lit sliver into the shadowed rim next door.

So a pixel whose stone is not the facet's own stone is shaded *smoothly*, at
that pixel, from its own stone's surface. It costs four extra field lookups
on a small share of the sheet, it is continuous where it meets the flat
shading, and it is the same function the `smooth` facet level uses for the
whole sheet.

## The water

`wet` is how much water is standing over the bed, and it is one axis because
a wet stone changes in four ways at once: it darkens, its colour saturates,
its highlight tightens and brightens, and the joints between stones go
darker still because that is where the water is deepest. Set as four flags
these drift apart and the sheet reads as a colour mistake rather than as
water.

| level | what it is |
| --- | --- |
| `dry` | a bar above the waterline: pale, matte, wide soft highlight |
| `damp` | the tideline; colour just beginning to come up |
| `wet` | under an inch of water — saturated, dark, a tight specular |
| `sunk` | deep and still, colour going toward the water's own |

## Colour

`internal/scheme` decides every stone's pigment and its **tone**, from the
region's centroid and area. The whole sixteen-strategy vocabulary is
available, and unlike 009 they are weighted properly rather than pinned to
`passage` — this sketch has no seeds to keep faith with.

Tone is not thrown away: it is how heavily the stone was painted, so it
carries the sheet's value structure *underneath* the light. A stone that the
scheme made dark and a stone the light turned away from are different facts,
and a picture needs both — light alone gives a sheet of one colour in
relief, and tone alone gives the flat sampler that lighting was supposed to
fix.

The light is applied on top of the paint, not mixed into it:

- the shade multiplies the painted colour, so a dark pigment stays dark;
- what the light *adds* leans toward the palette's warmest member, squared so
  that only faces genuinely square to the lamp take it — linear, the whole
  sheet drifts warm at once and reads as a colour cast rather than as a lit
  surface;
- what the shadow takes away leans toward the palette's coolest, because a
  face lit only by the sky is bluer than the thing turning away from the sun.
  **That lean is value-matched**, and it has to be: the point has already been
  multiplied by the diffuse, so mixing in a colour that is dark in its own
  right takes the light away twice and the shadowed half of every stone sinks
  into the joint it is lying in. Only the hue is borrowed;
- the specular is added last, in the lamp's colour, and is the one cue that
  says the surface is smooth and wet rather than matte. Added rather than
  mixed, so a dark stone — the kind a gleam actually reads on — can have one.

Both the lamp's colour and the sky's come from the palette, not from a
constant: an invented warm white reads as a filter laid over the picture, and
it throws away the provenance every palette here carries (invariant 3).

### Gold nuggets

`--gold` reserves yellow for a rare accent. Before the colour scheme runs,
yellow and amber members are removed from the selected colourway; after the
ordinary bed has been dressed, two or three random stones from the smaller
two-thirds of the visible bed are repainted with the gold from Milton Avery's
*Bicycle Rider By The Loire* (`#F3C937`). The gold is saturated after the
water treatment so it remains richer than the muted bed.

The selection has its own deterministic RNG stream. Nugget placement may
cluster or spread according to the seed, but it cannot change the layout,
facets, ordinary colour arrangement, or any render made without `--gold`.

**The water is the darkest thing on the sheet, and that is derived rather than
chosen.** A fixed fraction of the palette's darkest swatch cannot promise it —
a bed painted in the palette's own darks, seen at the ambient, goes below any
joint mixed from the same handful of colours, and a stone that sinks below the
water stops being a stone. So the joint is taken down until it clears the
deepest shadow *this* bed can actually throw.

## Traits

| dimension | key | values |
| --- | --- | --- |
| `colourway` | c | which palette the bed is drawn from |
| `bed` | b | boulders, cobbles, shingle, gravel, grit (weight 0) |
| `stones` | t | worn, rolled, broken, jumbled (weight 0) |
| `facets` | f | plates, cut, crazed, shattered, smooth (weight 0) |
| `light` | g | raking, morning, noon, overcast |
| `wet` | w | dry, damp, wet, sunk |
| `joint` | n | fine, drawn, bold |
| `scheme` | s | the sixteen `internal/scheme` strategies |

`bed` resolves to *ranges* — count, ladder, clearance and overscan together
— for the reason set out in 008: a level is a kind of bed, not one
particular bed.

`stones` is how worn they are, and it carries the rounding, the merge share
and the warp at once. They are one decision: a stone rounded like a pebble
that is also a four-site lobe bent double is not a stone anyone has seen.
`jumbled` is past that on purpose, and carries weight 0.

`facets` carries the facet size, the random tilt, the shade jitter and the
crease together. The tilt and the jitter have to stay well under the dome's
own gradient: at parity they stop describing a surface, the faces stop
agreeing about where the light is, and the stone goes flat again.

Every stone is faceted at every level, and `--faceted` is what the share is
for. A stone left smooth among faceted ones does not read as variety, it
reads as one the sheet forgot — the facets are how a stone is described here,
so a stone without them is an airbrushed blob with pebbles round it.

`smooth` is weight 0 and is the control: the same surface and the same light
with no facets at all, which is the only way to see what the flat shading is
actually doing.

`light` sets the lamp's height and the ambient together, because they are
one decision about weather rather than two numbers — a low sun with a high
ambient is a contradiction, and it renders as a flat sheet with long
shadows.

## Tunables

| flag | what it does |
| --- | --- |
| `--count`, `--rungs`, `--base`, `--ratio`, `--gap`, `--over` | the stone pack, overriding what `bed` drew |
| `--weight` | how strongly a stone's size bends its walls; 0 gives straight ones |
| `--merge`, `--max-lobe`, `--warp`, `--swirl`, `--round` | the stones' shape, overriding `stones` |
| `--ink`, `--swell`, `--node`, `--wobble` | the joint |
| `--facet`, `--facet-scale`, `--faceted`, `--cut`, `--flake`, `--crease` | the facets, overriding `facets` |
| `--rise`, `--bearing`, `--elevation`, `--ambient`, `--gloss`, `--sharp` | the surface and the lamp (`--light` is the trait, which sets all of them) |
| `--warmth`, `--coolness` | how far the lit and shadowed sides lean in colour |
| `--soak`, `--sheen`, `--depth` | the water, overriding `wet` |
| `--load`, `--pool`, `--uneven`, `--grain` | the paint and the paper |
| `--accent`, `--passage`, `--shades`, `--saturate` | the colour scheme's spread |
| `--gold` | reserve yellow for two or three rare, saturated gold nuggets |

## Acceptance checklist

- The bed reads as stones **under water**, not as a wall: no straight seams
  between two stones, no repeated shape, and sizes graded across an order of
  magnitude on a `shingle` bed.
- Every stone is convex-ish and worn. If any reads as a polygon, `round` is
  too low; if the stones float apart into discs with joint between them
  rather than sharing walls, it is too high.
- The facets are flat: hard steps in value at every facet edge, no gradient
  inside a facet.
- The facets are the same fineness on the biggest stone as on the smallest.
  If the big ones look coarsely cut, the grain is following the stone.
- The facets describe the stone rather than confetti it — walking across one
  stone from the lit side to the shadowed side, the facet shades go
  monotonically darker. If they do not, `cut` or `flake` is too high.
- One light: every stone on the sheet is bright on the same side.
- The stone still reads as its own colour at both ends of the light. A facet
  in shadow must not be black, and a facet in the highlight must not be
  white. In particular, the shadowed half of a stone must stay clearly
  lighter than the water it is lying in — check it on a `sunk`, `raking` seed
  with a dark palette, which is where it fails first.
- The joint swells at the junctions and is thinnest mid-wall.
- With `--gold`, only two or three small-to-medium stones read yellow or gold;
  the ordinary bed remains muted and contains no competing yellow passage.
- Preview and print of one seed are the same picture, and no facet edge
  aliases into a stair at print size.
- `--facets smooth` gives the same picture soft, and it should look
  distinctly worse. If it does not, the facets are not earning their place.
