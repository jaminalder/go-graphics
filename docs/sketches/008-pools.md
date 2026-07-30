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

1. **Paper.** The palette's lightest colour, lightened and desaturated to an
   off-white. Everything else is transparent pigment over it.
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
6. **Paint.** Largest first, so small marks settle on top the way later
   touches of a brush do. Each circle is `paint.Wash.Pool` / `.Ring` at low
   `Ragged` — deformed enough to be hand-laid, not enough to stop being a
   circle.

## Tunables

| field | default | what it does |
|---|---|---|
| `Count` | 22 | anchor circles before satellites |
| `Rungs` / `Ratio` / `Base` | 5 / 1.55 / 0.030 | the size ladder, canvas units |
| `Satellites` | 0.45 | share of anchors that get an overlapping companion |
| `Ragged` | 0.055 | wash edge deviation; 0.22 is shoal's blob |
| `Rings` | 0.34 | share of circles carrying inner rings |
| `Open` | 0.28 | share painted as annuli rather than discs |
| `Glaze` | 0.16 | share carrying a second pigment on top |
| `Alpha` | 0.74 | pool strength; below 1 so crossings stay readable |
| `Pigments` | 4 | palette colours in play |
| `Margin` | 0.06 | clear paper at the edge |
| `Gap` | 0.12 | clearance between anchors, ×radius |
| `Candidates` | 7 | darts per anchor; few on purpose, see below |

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
- [ ] Preview and print of the same seed are the same composition.
