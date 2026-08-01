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

   - `wall = (d₂ − d₁)/2` — the distance to the nearest wall, and the
     field every fill uses to know how deep inside its cell it is. Halved
     because `d₂ − d₁` closes at twice the rate the point approaches the
     wall.
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

## Traits

| dimension | key | values |
| --- | --- | --- |
| `colourway` | c | which palette the sheet is drawn from |
| `density` | d | sparse, open, medium, busy, packed |
| `lobes` | l | tidy, few, many, most |
| `fills` | f | washed, mixed, drawn, airy |
| `line` | n | fine, drawn, bold |

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
| `--wobble` | hand wander of the line and the strokes |
| `--rim`, `--rim-width`, `--mottle`, `--blotch`, `--grain` | the wash's character inside a cell |
| `--wash`, `--pencil`, `--bands`, `--hatch`, `--empty` | style weights, overriding `fills` |
| `--accent`, `--passage`, `--stroke` | colour spread, colour wavelength, stroke pitch |

The pack overrides are applied *before* the line is drawn, because the ink
width is a fraction of the smallest site: applied afterwards, `--base`
gives a hand-set cell size under a line scaled for the one the seed drew.

## Acceptance checklist

- Three walls meet at each junction, and the junction is visibly swollen
  while the wall is at its thinnest between two junctions.
- Walls are curved. If they read as straight segments, `weight` is too low
  or the size ladder is too flat.
- Some cells are concave lobes wrapping around a neighbour.
- Cell sizes span at least an order of magnitude in area on a `busy` sheet
  — big lobes next to slivers, as in the reference.
- Fills stop cleanly at the ink and never leak into a neighbour.
- Bands and rims follow the cell's own outline, including in a lobe.
- Some cells are bare paper.
- Preview and print of one seed are the same picture, and no spikes radiate
  from the junctions at print size.
