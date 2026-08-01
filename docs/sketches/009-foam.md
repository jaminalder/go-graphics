# 009 — foam

A sheet divided into curved-walled cells by a heavy ink line, each cell
filled with something else: a watercolour wash, coloured-pencil hatching,
concentric bands, or nothing at all.

## Why this sketch

Every sketch up to here draws *marks* — discs, strands, contours — onto an
undivided sheet. This one draws a **structure** first and treats what goes
inside it as a separate question. The structure is a planar partition:
every point on the canvas belongs to exactly one cell, and every cell knows
its own area, centre and inscribed radius. That is the thing the earlier
sketches never had, and it is what makes "fill this region with a wash" or
"hatch this region at 40°" expressible at all.

`internal/noise.Worley` is a distance *field* — it answers "how far to the
nearest site". It cannot answer "which cell am I in, and how big is that
cell", so it cannot be filled. `internal/cells` answers both.

## The reference

Hand-drawn neurographic art: wandering lines, every crossing rounded off,
the enclosed cells filled with coloured pencil. Structurally the same thing
as a **2D wet foam** — a soap-bubble cluster obeying Plateau's laws. Three
walls meet at each junction at roughly 120°; the walls are arcs, not
straight lines; and the ink swells at the junctions and tapers thin
mid-wall, because a real foam collects liquid in its Plateau borders.

All three of those fall out of a weighted distance field. None of them
needs curve intersection, half-edge arrangements or face extraction — the
route this sketch deliberately does not take.

## Algorithm

1. **Sites.** Circles packed by best-candidate sampling over a geometric
   size ladder, the same vocabulary sketch 008 uses, but placed with an
   **overscan** — the pack covers a rectangle larger than the canvas, so
   cells at the border are cut by the frame rather than the frame being one
   enormous cell. Each circle contributes a site at its centre carrying an
   additive weight proportional to its radius.

2. **Cells.** For a point *p*, every site has distance
   `d_i = |p − c_i| − w_i`. The nearest site's cell owns the point. This is
   the **Apollonius** (additively weighted) diagram: its walls are
   hyperbolic arcs rather than straight bisectors, and a heavier site
   claims more ground. That single change is what separates this from a
   crystalline Voronoi — with all weights equal the sheet comes out as a
   shattered pane of glass, and no amount of ink weight rescues it.

   The **`weight`** knob is how strongly a radius becomes a weight. At 0
   the walls are straight; pushed too far a heavy site swallows a light
   neighbour whole and the cell count silently drops.

3. **Lobes.** A share of neighbouring site pairs are **merged** into one
   cell: the wall between them is never computed, so the union of two
   convex bubbles becomes a single concave lobe. This is where the
   crescents and kidney shapes come from. A convex-only foam reads as
   stained glass; the reference's character is in the shapes that bend
   around their neighbours.

   Merging is done on the *cell* id, not by erasing a drawn line: the two
   sites' shared wall does not exist, so nothing inside the lobe knows it
   was ever two cells — the fill, the rim and the band spacing all treat it
   as one region. Only a genuine neighbour will do: joining the nearest
   site that happens to be free lets a crowded sheet pair a site with
   something across the frame, and a cell in two disconnected pieces is
   something neither the metric nor any fill can express.

4. **The warp.** The whole partition is looked up through a smooth
   displacement of the plane — curl noise, wavelength many cells long,
   displacement a fraction of one cell.

   This is not decoration. The weighted metric curves a wall only where the
   two sites either side of it differ sharply in weight, and in a packed
   sheet most neighbours are much the same size, so without a warp the
   typical wall comes out straight and the sheet reads as a cracked pane
   however many lobes it has. Bending the plane curves every wall at once,
   for two noise samples.

   Curl rather than a plain noise gradient because it is divergence-free:
   it shears the plane without compressing it, so a cell comes out bent
   rather than squeezed to nothing. Its length is unbounded, so the
   displacement is held inside the pack's overscan by a tanh limiter — a
   hard clamp would crease the field exactly where it bites, and a crease
   in the displacement is a straight edge cutting across the cells.

   Cells are measured *before* the warp. The field neither creates nor
   destroys area, so a cell's share of the sheet survives it.

5. **Walls and junctions.** With `d₁ ≤ d₂ ≤ …` the nearest *distinct*
   cells:

   - `wall = (d₂ − d₁)/2 − (Round/2)·log Σ_{k≥2} exp(−(dₖ − d₂)/Round)` —
     the distance to the nearest wall, and the field every fill uses to
     know how deep inside its cell it is. Halved because `d₂ − d₁` closes
     at twice the rate the point approaches the wall.

     The second term **rounds the corners**. A cell of a foam meets its
     neighbours at 120°, so measured against a hard minimum it is a polygon
     with curved sides — and at a glance a polygon is what it reads as,
     however well the sides curve. Replacing that minimum with a soft one
     pulls the wall in wherever two other cells are close at once, which is
     at a corner and nowhere else: mid-wall the sum has a single term, the
     soft minimum equals the hard one, and a straight run of wall is
     untouched to within a hundredth of `Round`.

     `Round` has to stay well under half a cell. Past that the corners eat
     the walls between them and the cells come apart into discs floating in
     ink; a foam whose cells do not touch is not a foam. It also does the
     swelling's job at the junctions, so `swell` is half what it was before
     rounding existed — run together they gave every node a blot.
   - `node = 1 − exp(−Σ_{k≥3} exp(−(dₖ − d₂)/Node))` — a smooth count of
     how crowded the point is with cells beyond the two whose wall it is
     on. Near 0 halfway along a wall, rising toward 1 at a junction, and
     higher at a four-way junction than a three-way one.

   `node` is a *sum*, not "how near is the third cell", and that is the
   whole of it. Ranking is not smooth in position: the identity of the
   third-nearest cell swaps along rays running out of every junction, and a
   measure built on it creases along those rays. Fed into the line width
   the creases come out as sharp spikes radiating from every node —
   invisible at 600px, unmissable at 6000.

   The ink is laid where `wall < halfwidth`, with
   `halfwidth = ink/2 · (1 + swell · node)`. The taper and the swollen
   junction are therefore the *same* rule, not a special case: the line is
   at its thinnest exactly halfway between two junctions, and the fillets
   between three cells are concave because `node` falls off smoothly. The
   softness of the ink's edge is a fraction of the *unswollen* line — tied
   to the local half-width it grows with the swelling, and the junctions
   dissolve into a halo exactly where they most need to read.

   `ink` itself is a fraction of the smallest site radius, not a distance
   on the page: a line is heavy or fine relative to the cells it divides,
   and the same absolute width that reads as confident on an open sheet
   closes a packed one into a black net with pebbles in it.

6. **Dressing.** Each cell draws a pigment and a fill style before anything
   is rasterised. Pigment comes from a low-frequency Perlin field sampled
   at the cell's centroid, so the sheet has *passages* of related colour —
   a green corner, a brown corner — rather than confetti; a small share of
   cells take an accent from elsewhere on the ramp. Style is drawn from the
   weights the `fills` trait resolves to.

7. **Raster.** One pure pixel function: look up the cell, ask its dressing
   for a colour, then lay the ink over the top. No stamps, so the sketch is
   resolution-independent by construction and `--profile print` is the same
   picture as `--profile preview`.

## Fill styles

The point of the sketch. Every style is a function of the cell's dressing
and the `wall` field, which is what lets a fill know where its own edge is
without ever seeing a polygon.

- **wash** — flat pigment with a watercolour rim: the pigment concentrates
  in the last stretch before the wall, the way a drying pool deposits at
  its edge. Mottled at a broad scale and granulated at the paper's tooth.
- **pencil** — directional strokes at a per-cell angle, warped slightly so
  the hatching is not mechanical, laid over paper so the tooth shows
  through. The reference's dominant treatment.
- **bands** — concentric rings *following the wall distance*, so they run
  parallel to the cell's own boundary whatever shape it is. In a lobe they
  bend around the waist. Restricted to cells that are roughly round: the
  wall-distance field ridges along a cell's medial axis, and in a long or
  forked one it is those ridges the rings trace, which fills the cell with
  a tangle of pinched loops rather than with rings. Roundness — the cell's
  area against the disc of its own inscribed radius — is the test, and it
  is the reason a measured cell carries an inradius at all.
- **hatch** — thin parallel lines on bare paper, the reference's patterned
  cells.
- **empty** — bare paper. Load-bearing: a sheet with every cell filled has
  nowhere to rest, and the reference leaves perhaps one cell in six white.

`--fills net` is every cell empty: the structure with nothing in it. It
carries weight 0, so no seed draws it — an unfilled sheet is a drawing of
the algorithm rather than a picture — but it is one flag away, and it is
the only way to actually look at what the walls are doing.

```sh
staticart render foam --fills net --density packed --line drawn --seed 7
```

## The mosaic — subdividing a cell

A second, much finer partition is laid over the *whole canvas*, so a point's
identity becomes the pair (outer cell, inner tile). Both are looked up at
the same warped coordinate, the tile decides the colour, and the outer ink
goes on top afterwards — so the heavy line clips the fine net for free, with
no clipping code anywhere. Per-cell site sets would need a foam per cell
(forty measuring passes instead of one) and would still have to solve the
same clipping problem at every border.

The one thing a single global foam apparently cannot do is give a big lobe
and a sliver comparable tile counts, since one site spacing serves the whole
sheet. It can: the inner pack is a **variable-radius dart throw**, and each
dart's radius is a fixed fraction of the inradius of whichever outer cell it
lands in. The spacing follows the outer structure while the partition stays
global — and that is the second thing `cells.Cell.Inradius` is for.

The inner sites carry **weight 0**, so the inner metric is an ordinary
Voronoi: straight bisectors, sharp corners, bent only by the shared warp.
That is deliberate contrast. A second bubble cluster inside the first gives
one texture at two scales and the sheet reads as a blur; crystal inside
organic reads as two things.

Only a *share* of cells are subdivided (`--tiled`), so a sheet has plain
passages — a wash, a hatched cell, bare paper — next to busy ones. That
contrast is most of the point; at share 1 the mosaic is a wallpaper.

### Where a tile gets its colour — the `mosaic` trait

| level | the rule |
| --- | --- |
| `plain` | not subdivided at all; the sketch as it was |
| `family` | the tile's centroid projected on a per-cell axis walks two or three steps along the ramp, so a cell reads as one family of related pigments |
| `strata` | hue from where the tile is on the *sheet* (a field that crosses the walls), value from the tile's area against the median |
| `tonal` | one hue per outer cell; the tiles differ only in lightness |
| `soloist` | one cell — large, and near the middle — in full colour, everything else drained to near-neutral |
| `neighbour` | a tile wears the pigment of the cell across its nearest *outer* wall, in proportion to how near that wall it is: a cell's rim belongs to what it touches, its core keeps its own |

`neighbour` is what `cells.Hit.Near` was added for. Its reach is measured
against the containing cell's **own inradius**; a fixed reach is longer than
a small cell is wide, and then every tile is a half-and-half mix of two
pigments, which reads as mud.

## The relief — lighting one foam field

`Wall` is a real signed distance field, not a mask, so it can be
differenced: a height built from it has a slope, a slope is a normal, and a
normal can be lit. The sheet gets a surface without anything being
modelled, and the partition's own creases become the surface's edges.

Height is in **canvas units of rise** and the difference step is a canvas
length, which is what keeps the lighting resolution-independent — a step of
"one pixel" gives a chamfer that hardens as the render grows. The outer
cells carry the large form and the tiles carry the facets on top of it, at
half the rise: one foam field with relief at two scales.

One light, one direction, drawn once per sheet within twenty degrees of
up-and-left. Inconsistent lighting is the single thing that makes fake depth
read as fake.

| level | what it does |
| --- | --- |
| `flat` | no surface |
| `bevel` | a chamfer at every wall — the sheet as cut and inlaid tiles |
| `cushion` | a dome per cell and per tile, height `√(t(2−t))` of the wall distance over the cell's inradius: inflated |
| `occlude` | no light at all, only the darkening that collects in a crease. One extra lookup instead of ten, and it can never blow a pale pigment out |
| `terrace` | cells at one of four depths, casting soft shadows on each other, higher cards catching more light |
| `glass` | lit from behind: pigment at its most saturated where the glass is thin, going to the lead where it thickens |

`terrace` needs both cues. Lambert alone says nothing: the top of a slab is
flat, so every slab is lit identically however high it stands. The cast
shadow says which cell is *behind* another and the tone says which is
*above*. The shadow march is as long as the tallest step can reach
(`3·depth / light slope`) and its taps are **averaged**, not maximised — a
max switches each tap's whole contribution on at once and the shadow's edge
comes out as a staircase.

## Traits

| dimension | key | values |
| --- | --- | --- |
| `colourway` | c | which palette the sheet is drawn from |
| `density` | d | sparse, open, medium, busy, packed |
| `lobes` | l | tidy, few, many, most |
| `fills` | f | washed, mixed, drawn, airy, net (weight 0) |
| `line` | n | fine, drawn, bold |
| `mosaic` | m | plain, family, strata, tonal, soloist, neighbour (all but `plain` weight 0) |
| `relief` | r | flat, bevel, cushion, occlude, terrace, glass (all but `flat` weight 0) |

`mosaic` and `relief` are **appended** to the schema, and everything but
their neutral value carries weight 0. `Derive` draws in schema order, so the
five dimensions above them are untouched and every existing seed still lands
on exactly the sheet it drew before — the same argument decision 21 makes
for QQL's wash medium. They are two dimensions rather than one because they
are genuinely orthogonal: a plain foam can be lit, and a mosaic can be flat.

Subdivision and colouring, on the other hand, are one dimension, because
they are one decision: a walk along the ramp needs enough tiles in a cell
for the walk to be a walk, and colouring by tile area needs tiles whose
areas differ.

`lobes` carries both ways the sheet can bend — how many cells are merged
*and* how hard the plane is warped. They belong on one axis because they
are one decision: a sheet that bends its walls and never merges a cell is
as inconsistent as the reverse.

`density` resolves to *ranges* — count, size ladder, clearance and overscan
together — for the reason set out in 008: a level is a kind of sheet, not
one particular sheet.

## Tunables

| flag | what it does |
| --- | --- |
| `--count`, `--rungs`, `--base`, `--ratio`, `--gap`, `--over` | the site pack, overriding what `density` drew |
| `--weight` | how strongly a site's radius becomes an additive weight; 0 gives straight walls |
| `--merge`, `--max-lobe` | the merging, overriding `lobes` |
| `--warp`, `--swirl` | the plane's bending and its wavelength, × smallest cell |
| `--ink` | wall thickness in canvas units |
| `--swell` | extra thickness at a junction, × `ink` |
| `--node` | distance over which a further cell counts as near |
| `--round` | radius a cell's corners are rounded over |
| `--wobble` | hand wander of the line and the strokes |
| `--rim`, `--rim-width`, `--mottle`, `--blotch`, `--grain` | the wash's character inside a cell |
| `--wash`, `--pencil`, `--bands`, `--hatch`, `--empty` | style weights, overriding `fills` |
| `--accent`, `--passage`, `--stroke` | colour spread, colour wavelength, stroke pitch |
| `--tile` | inner tile size, × the outer cell's inradius; smaller means more tiles per cell |
| `--tiled` | share of cells subdivided at all; below 1 the sheet has plain and busy passages |
| `--fine` | inner net width, × the outer ink |
| `--spread` | ramp steps a cell's family of tiles walks |
| `--depth`, `--bevel` | the relief's rise and the run it happens over, × smallest cell |
| `--light` | the light's bearing in degrees; 90 is from the top, 135 the default up-and-left |

The pack overrides are applied *before* the line is drawn, because the ink
width is a fraction of the smallest site: applied afterwards, `--base`
gives a hand-set cell size under a line scaled for the one the seed drew.

## Acceptance checklist

- Three walls meet at each junction, and the junction is visibly swollen
  while the wall is at its thinnest between two junctions.
- Walls are curved and the corners between them are rounded. If the cells
  read as polygons, `round` is too low; if they float apart into discs with
  ink between them rather than sharing walls, it is too high.
- Some cells are concave lobes wrapping around a neighbour.
- Cell sizes span at least an order of magnitude in area on a `busy` sheet
  — big lobes next to slivers, as in the reference.
- Fills stop cleanly at the ink and never leak into a neighbour.
- Bands and rims follow the cell's own outline, including in a lobe.
- Some cells are bare paper.
- Preview and print of one seed are the same picture, and no spikes radiate
  from the junctions at print size.

For the mosaic and the relief:

- A quarter-canvas lobe and a cell an order of magnitude smaller hold tile
  counts within a factor of five of each other.
- The outer ink runs unbroken over the inner net, and the inner net is
  visibly the lighter line of the two.
- With `--tiled` below 1 the sheet has plain cells — a wash, hatching, bare
  paper — sitting against tiled ones.
- Every highlight on the sheet is on the same side of its bump, and every
  cast shadow falls the same way.
- `--profile preview` and `--profile print` of one seed have the same
  chamfer width relative to a cell.

### Where to start looking

```sh
staticart render foam --density open --lobes few --line drawn \
  --fills washed --mosaic tonal --relief cushion --seed 7
staticart render foam --density open --lobes few --line drawn \
  --fills net --mosaic soloist --relief bevel --seed 12
staticart sweep foam --seeds 5,7,12 \
  --vary mosaic=plain,family,strata,tonal,soloist,neighbour \
  --density open --lobes few --line drawn --fills washed --relief cushion
```
