# Sketch 006 — `shoal` (dot field along a noise flow)

An all-over field of small brush-painted dots, strung in chains along the
streamlines of a noise flow field on an off-white ground. Every dot is
laid with the bristle brush from `internal/paint`, so a large dot carries
the concentric smear of the brush that made it and a small one reads as a
flat spot.

Reference for the look: `docs/reference/target-shoal.png` — a dot field
organised around a spiral. Here the organising structure is a noise flow
field instead, which gives eddies and drifts rather than one centre.

## What makes a dot field read as designed

Four things, in rough order of how much they matter:

1. **Chains must read as threads.** This is Gestalt proximity, and it is
   controlled entirely by the ratio of the gap *along* a chain to the gap
   *across* to the next chain. Dots advance by `2r + 0.2r` along the
   flow, so consecutive dots nearly touch; neighbouring chains only ever
   meet at the collision test. As the two gaps approach each other the
   flow structure disappears and the field collapses into isotropic
   circle packing.
2. **Colour runs along a chain, not per dot.** A chain takes one colour
   and keeps it, turning over with probability `ColorFlip` (0.06) at each
   dot — runs of roughly a dozen dots. Choosing per dot scatters the
   palette into confetti and destroys the drifts of colour that carry the
   large-scale structure.
3. **The fields must be independent and at different frequencies.** Flow
   (1.8), size (1.6) and colour (1.9) come from separately salted noise.
   Sharing one field, or matching their frequencies, gives mush.
4. **A long tail beats uniform variety.** Colours are drawn from a
   weighted bag — after a per-seed shuffle one colour dominates, one
   supports it and the rest are progressively rarer.

## Layout

Seeds sit on a jittered grid (`Starts` per axis). Each seed grows a chain
in **both** directions along the flow, so a chain is a whole streamline
through its seed rather than a tail hanging off one.

A blocked step is skipped rather than fatal: the chain threads on through
crowded ground and only gives up after `MaxMiss` (34) consecutive
failures. Chains shorter than `MinChain` are dropped as stubs.

**Nothing enters the collision index until its chain survives that cull.**
A culled stub that had already been inserted goes on blocking ground it
no longer occupies; enough of them starve the field, and the symptom is
that adding seeds stops adding dots.
`TestCulledChainsLeaveNoGhosts` pins this by requiring the index and the
returned dots to hold the same circles.

At the default `Starts` the layout is at saturation (~2900 dots on a
square), which is the intent — the field should be full.

## Fields

| `--field` | Character |
|---|---|
| `flow` (default) | angle straight from fBm. Has sources, sinks and saddles, so chains gather into dense drifts and leave quiet ground — density variation for free. |
| `curl` | the divergence-free `(∂ψ/∂y, −∂ψ/∂x)`. Genuine closed eddies, evenly spread; needs its variety from the size and colour fields instead. |
| `ridge` | steered by `1 − |fBm|`. Chains snap into filaments along the folds; graphic rather than painterly. |

The plain angle field is the default even though the curl is the
"correct" one, precisely because perfectly even spread is the same
problem as wallpaper.

All three get one level of domain warp (`Warp`, 0.7) — enough to fold
broad sweeps into eddies, where a second level would just be turbulence.

## Grading and detail

`--grade vortex` (default) shrinks dots toward an off-centre focus, so
the field tightens into a dense eye; `patches` takes radius from the size
field alone. Only the variable part of the radius is squeezed, so the eye
fills with dots at `MinR` instead of shrinking to specks.

Interior rings go on `Detail` (0.09) of dots, clustered by a noise field
and gated to `r > 2.2·MinR` — scattered evenly the detail reads as a
rendering fault, and on the smallest dots the bands cannot resolve.
`Open` (0.05) of dots are painted as rings rather than discs.

## Mark, ground and layering

Placement and painting are deliberately separate. The packing decides
where paint may go; what is actually painted there is a second question,
and the answer changes the picture far more than any layout parameter.

- **`--mark disc`** paints each dot on its own: the chain reads as beads.
- **`--mark ribbon`** paints a whole run as one brush stroke through its
  dot centres. The layout is identical — only the interpretation
  changes — but the brush is finally used the way it was built to be
  used, dragged along a path, so the flow itself becomes the subject and
  the bristle smear runs the length of it.
- **`--mark wash`** paints each dot as a pool of watercolour instead of
  opaque paint (`paint.Wash`). Pools are transparent, so where two cross
  the pigments mix into a third colour rather than one hiding the other.
  That only shows if the marks are allowed to touch, so pair it with
  `--overlap` and a larger `--maxr` than the disc default.
- **`--mark mixed`** makes the mark type a *spatial* variable: coarse
  chains become strokes, fine ones stay beads, so brushwork erupts out of
  a stippled field wherever the size field runs large. The split sits at
  28% of the radius range rather than the midpoint, because the size
  field is biased quadratically toward small and a midpoint threshold
  leaves almost nothing on the ribbon side.

A run is a stretch of one chain sharing one colour — the unit a ribbon is
painted as, so a stroke never jumps between chains or changes colour
part way.

**`--ground dark`** sinks the darkest palette colour to L≈0.09, keeping a
little of its hue so it reads as painted ground rather than as a hole,
and hands the marks the lightest colour. The same layout reads as lit
rather than printed.

**`--overlap`** relaxes the collision gap into negative territory so marks
crowd into one another, and switches the painting order to coarsest
first, so small marks settle on top of large the way later touches of a
brush do.

**`--mono`** replaces the palette with a ladder of tints and shades of one
hue plus a single rare accent. The field stops being about which colour
each mark is and becomes about the structure.

**`--margin 0`** bleeds the field off every edge instead of framing it,
and the `-tall` render profiles give a 4:5 portrait frame, which reads
very differently from a square for all-over work.

## Tunables

| Knob | Flag | Default |
|---|---|---|
| Field | `--field` | `flow` (default), `curl`, `ridge` |
| Grading | `--grade` | `vortex` (default), `patches` |
| Mark | `--mark` | `disc` (default), `ribbon`, `mixed`, `wash` |
| Ground | `--ground` | `light` (default), `dark` |
| Single-hue inks | `--mono` | off |
| Crowding | `--overlap` | 0 |
| Ring detail share | `--detail` | 0.09 |
| Open (ring) dots | `--open` | 0.05 |
| Colour off-field share | `--confetti` | 0.4 |
| Dot radius range | `--minr` / `--maxr` | 0.0035 / 0.0135 |
| Framing | `--margin` | 0.045; 0 bleeds off the edge |

## Acceptance checklist

- [ ] Chains read as threads, not as isotropic packing.
- [ ] Neighbouring dots usually share a colour; no confetti.
- [ ] One colour dominates; accents are rare.
- [ ] The field is full — no large unexplained bald patches.
- [ ] Blurred heavily, the piece resolves into a few masses rather than
      a flat field.
- [ ] Dots are solid, with no pinhole at the centre and no unintended
      concentric ring in a plain dot.
- [ ] Preview and print of the same seed show the same composition.
