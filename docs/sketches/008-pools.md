# 008 — pools

Watercolour circles on paper: a scatter of near-exact discs and open rings
laid as transparent pools, at a few discrete sizes, deliberately allowed to
overlap so the pigment mixes where they cross.

## Why this sketch

`internal/paint`'s wash model was built inside sketch 006 (shoal), where it
paints a field of small chaotic dots strung along a flow. That hides most
of what the model can do: at dot size a pool is a smudge, the rim never
resolves, and no two pools cross cleanly enough to show the mixing that is
the whole point of transparent paint.

This sketch is the opposite exposure. Few marks, large, close to circular,
and overlapping on purpose — so the wash has to hold up as a *shape*: a
clean boundary, a rim where the water dried, granulation in the middle, and
a believable third colour wherever two discs cross.

## Algorithm

1. **Paper, then ground.** The canvas starts as bare paper — the palette's
   lightest colour taken almost to white — and a single tint is washed over
   the whole sheet with `paint.Wash.Ground`. A flat wash dries with the
   unevenness of its own laying in it: blotchy at a broad scale where the
   pigment pooled, speckled at the paper's scale where the tooth caught it.
   Both come from a per-pixel field, not from shapes. The first version
   covered the sheet with a grid of overlapping pools, and no matter how
   soft their edges were made, every pool boundary stayed legible as a fine
   arc — more circles, on a picture already full of them. A field has
   nothing to have an edge. `Ground` is the wash's strength and
   `GroundBlotch` the wavelength of its unevenness; at `Ground` 0 the sheet
   is bare paper.

   Ground and marks granulate at one shared `paperTooth`, because they are
   on one piece of paper — two grain scales in a picture are two papers.

   The tint is the palette's lightest colour, softened. A few palettes have
   a near-white as their lightest — a paper colour rather than a paint —
   and it cannot tint anything: the ground comes out as bare paper with a
   rumour of colour on it, and none of the wash's structure shows. Those
   borrow from the next colour down until the tint has body. The threshold
   sits above every ordinary palette's lightest (0.88 against a typical
   0.61-0.82), so it rescues the handful that need it rather than
   correcting the rest.
2. **Pigments.** `Pigments` colours drawn from the palette by luminance,
   darkest kept as the ink for interior rings. A weighted bag makes one
   pigment dominant and the rest progressively rarer, so the sheet reads as
   chosen rather than sampled.
3. **Anchors.** `Count` positions by best-candidate sampling (`Candidates`
   darts each, keep the one furthest from what is already placed), inside a
   `Margin`. Few darts on purpose: many darts maximise the spacing, and
   perfectly spaced marks are as inert as a grid. Each anchor takes a radius
   from a **geometric ladder** of `Rungs` steps at ratio `Ratio`, weighted
   toward the small end but only gently — a steep falloff never reaches the
   top rungs and leaves a field of same-sized specks.
4. **Satellites.** A `Satellites` share of anchors spawn one companion from
   the rung immediately below, centred 0.42–0.74 of the two radii apart, so
   it always crosses its parent. This is where the overlaps come from —
   left to chance they either never happen or arrive as a pile-up, and a
   speck crossing a large disc shows nothing, so the companion has to be
   worth looking at in its own right.
5. **Structure.** Every circle draws one of four:
   - `plain` — a single pool of one pigment
   - `nested` — a pool with 2–3 concentric rings glazed inside it
   - `open` — an annulus, bare paper in the middle, with 0–2 inner rings
   - `glaze` — a pool with a smaller, offset pool of a second pigment on it
   - `banded` — a disc *made of* fine concentric rings, rim to centre
6. **Paint.** Largest first, so small marks settle on top the way later
   touches of a brush do. Each circle is `paint.Wash.Pool` / `.Ring` at low
   `Ragged` — deformed enough to be hand-laid, not enough to stop being a
   circle.

## Traits

| Flag | Values (weight) |
|---|---|
| `--arrange` | **orbital 3** · **shadows 3** · formation 2 · scatter 1 |
| `--flow` | horizontal 3 · vertical 2 · diagonal 2 · **spiral 4** · circular 2 · explosive 1 |
| `--fill` | sparse 1 · open 2 · **medium 3** · **busy 3** · packed 2 |

### `--arrange` and `--flow`: where the marks go

Borrowed from QQL, and all three of its moving parts. A **structure** seeds
start points; a **flow field** says which way to walk from them; and marks
are laid along that walk, shoulder to shoulder, until something already on
the sheet is in the way. The field proposes and the spacing rule disposes.

The first version of this took only the structure, on QQL's own reasoning
that the seeding matters more than the field. That is true of the
*composition* and false of the *surface*. What makes a QQL piece
recognisable at a glance is that its marks **touch**: they run in
contiguous strands that curve, and a strand holds one size and one colour
for its whole length. Scattering marks over a structured grid gives the
same large-scale arrangement with none of that, and reads as a scatter.

Three things are load-bearing, and all three come from the walk:

- **Marks advance by their own diameter**, so consecutive marks touch. A
  fixed grid of candidate positions cannot do this — the step has to follow
  the size of the mark being laid, which changes with every run.
- **A run holds one whole colour scheme**, not just a hue: the pigment, its
  partner, and the direction a banded mark graduates. Holding only the base
  lets the other two redraw at every mark, and a strand whose marks share a
  colour but differ in everything else about their colour does not read as
  one thing. QQL's runs are identical marks repeated.
- **A run holds one size.** Per mark, sizes come out as salt and pepper and
  no strand reads as a strand.

`--arrange` picks the structure:

- `orbital` — concentric bands about a usually off-canvas centre, cut into
  arcs; walked along a circular field these come out as record grooves
- `shadows` — non-overlapping blobs, each filling in against the field, so
  they read as separate objects
- `formation` — a grid of rectangles, some dropped; the gaps are what keep
  it from reading as wallpaper
- `scatter` — darts rather than walks, the only one with no direction in it

`--flow` picks the field: `horizontal`, `vertical`, `diagonal` for strands
that run straight, `spiral`, `circular`, `explosive` for strands that curve.

The seeds are spaced off the **smallest** rung, not the mean. A walk fills
in along its own length, so seeding at mark spacing lays every strand on
the last one; seeding at the mean — which a ladder reaching a quarter of
the canvas drags upward — puts a handful of walks on the whole sheet and
leaves most of it bare.

```sh
staticart sweep pools --seeds 1-12 --vary arrange=orbital,formation,shadows \
  --vary fill=busy,packed --profile web
```

## Tunables

The composition ones live on the `fill` trait above; these are what a
mark is made of and painted with.

| field | default | what it does |
|---|---|---|
| `Ragged` | 0.055 | wash edge deviation; 0.22 is shoal's blob |
| `Rings` | 0.34 | share of circles carrying inner rings |
| `Open` | 0.28 | share painted as annuli rather than discs |
| `Glaze` | 0.16 | share carrying a second pigment on top |
| `Banded` | 0.3 | share filled with fine concentric rings |
| `BandWidth` | 0.022 | ring pitch of a banded circle, canvas units |
| `BandOverlap` | 0.4 | how far neighbouring rings cross, ×pitch |
| `MaxBands` | 5 | most rings a banded circle may be built from |
| `Alpha` | 0.74 | pool strength; below 1 so crossings stay readable |
| `Pigments` | 4 | palette colours in play |
| `Ground` | 0.5 | strength of the painted ground wash; 0 is bare paper |
| `GroundBlotch` | 0.34 | wavelength of the ground's unevenness, canvas units |
| `Gap` | 0.12 | clearance between anchors, ×radius |

## The banded circle

Not a pool with rings drawn inside it: a disc that is *made of* rings, the
way a section through a tree is. Two things do the work, and both are
properties of transparent paint rather than anything drawn.

**The overlap.** Neighbouring bands cross by `BandOverlap` of the pitch, so
each seam is two glazes passing through one another and darkens on its own
— a fine contour line at every ring boundary that nobody drew and that no
opaque medium would produce. Butt the bands together instead and the mark
is a flat gradient with hairline gaps in it. The innermost band is widened
until it has no hole left, so the disc always closes over its centre; past
that point the wash draws it as the pool it has become.

**The colour ramp.** Bands graduate between `rampAnchors` pigments taken as
*consecutive entries of the luminance-ordered set, walked in one
direction*. Free draws from the whole palette put the ramp's midpoint in
the muddy space between two unrelated colours and the mark comes out grey
whatever its endpoints were; neighbours graduate cleanly, and walking one
way means the mark also darkens or lightens as it goes inward rather than
doubling back.

The pitch is a **width, not a count**, so a mark twice the radius gets twice
the rings rather than rings twice as fat and the ring texture weighs the
same across the size ladder — up to `MaxBands`, where the rule inverts.

That inversion is what the mark lives on. Past the cap a large disc would go
on accumulating rings until it read as a *target*, and the thing that makes
it — a ring wide enough to be a band of colour in its own right, with its own
wet edge and rim — would be lost to a count. So the biggest discs keep the
ring count and **widen the rings instead**, which is the trade a painter
makes for the same reason. `TestBandPitchHoldsUntilTheCapBinds` covers both
halves; `TestBigCircleKeepsFewWideRings` checks the top of the ladder comes
out with few rings *and* that they are genuinely wider, not merely fewer.

The dials: `--max-bands` is how busy a large disc may get; `--band-width
0.008` gives a fine grain on the small ones, `0.03` a bold half-dozen;
`--band-overlap 0.2` leaves the seams open, `0.9` merges them into a
graduated fill.

Prototype — a handful of big ones, some crossing:

```sh
staticart render pools --profile web --seed 3 --banded 1 --count 7 \
  --base 0.115 --rungs 2 --ratio 1.45 --satellites 0.7 --pigments 5 \
  --palette tchelitchew-hide-and-seek
```

## Acceptance checklist

- [ ] Discs read as **circles**, not blobs — the silhouette wobbles but never
      lobes.
- [ ] Every disc has a visible rim on at least part of its perimeter, and a
      lighter middle.
- [ ] Where two discs cross, the crossing is a **third colour**, darker than
      either parent, and both parents stay identifiable through it.
- [ ] Open rings show bare paper inside, with a rim on the inner boundary
      as well as the outer.
- [ ] A clear size hierarchy: a few anchors, many small marks, nothing in
      between that muddles the two.
- [ ] Paper grain is visible inside the pools and absent outside them.
- [ ] The ground reads as a laid wash — uneven, granulated, paper showing
      through — with **no line, arc or edge of any kind** legible in it.
- [ ] The ground and the marks granulate at the same scale: one sheet of
      paper, not two.
- [ ] A banded circle is filled edge to centre with no pinhole, its seams
      read as fine contour lines, and its colour graduates rather than
      jumping band to band.
- [ ] Where two banded circles cross, the two ring systems interfere and
      both stay legible.
- [ ] Preview and print of the same seed are the same composition.
