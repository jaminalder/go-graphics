---
name: colouring
description: How to colour a generative composition made of many discrete regions — cells, tiles, marks, strokes. Covers arrangement strategies (how colour is distributed across a composition), value structure, harmony schemes and proportion. Use when choosing or implementing how a sketch assigns colour, when a piece "looks like confetti", when a palette reads as a sampler rather than a picture, or when adding a colour-scheme axis to a sketch's output space.
---

# Colouring a composition

This is about **arrangement** — how colour is distributed across many regions —
not about picking a palette. The palettes in `internal/palette` are already
chosen by painters; the question is what to do with them.

## The one rule that matters most

**A hue arrangement with no value structure has nothing to look at from across
the room.** Squint at the render, or convert it to greyscale. If it goes flat
grey, the piece has no composition regardless of how good the colours are.

Every strategy below must therefore answer *two* questions per region: which
hue, and how dark. Answering only the first is the single most common failure,
and it is why a sheet of correctly-harmonised colour can still look like a
swatch card. In painting this is *notan* — the abstract light/dark skeleton
underneath the colour.

Give hue and value **different spatial rules**. A sheet whose hue and weight
both run along the same axis is a gradient, and a gradient has no composition
in it. Field for one, direction for the other; or field for one, region size
for the other.

## Arrangement strategies

After Tyler Hobbs' *Color Arrangement in Generative Art*, plus the classical
painting devices. Each is a different picture from the same structure.

| strategy | rule | what it gives | fails when |
| --- | --- | --- | --- |
| **clump / passage** | ramp position from a low-frequency noise field at the region's centroid | passages of related colour — a green corner, a brown corner | the field's swing is small; sampled raw everything lands mid-ramp and you get two colours out of nine. Stretch it (×1.5) before reading |
| **gradient** | ramp position from position along a direction | large-scale structure, a landscape read | with no jitter the boundary between ends is a visible straight line |
| **sequence** | order the regions, then walk a sorted ramp with probabilistic deviation | bands, structured variety | the walk step is too large and it becomes a shuffle |
| **inherit** | take a neighbour's colour with high probability; mutate rarely | large unified chunks with organic edges | the mutation rate is too high — it degenerates to confetti |
| **dominance** | weighted probability over 3 palette members, roughly 70/20/10 | hierarchy; the eye gets somewhere to rest | equal weights. A uniform draw gives every colour equal presence, which reads as a sampler |
| **complement** | a muted dominant over most of the area, small saturated accents of its opposite | vibrancy without noise | 50/50. A complementary scheme is 80% muted green and 20% intense red, not half each |
| **analogous** | restrict hue to a neighbouring arc of the wheel; value does the work | coherence, atmosphere | no value range — it turns into a single flat field |
| **triad** | three hues spaced round the wheel, in *unequal* proportion | liveliness | equal proportion, which is a flag rather than a picture |
| **monochrome** | one pigment at every dilution, plus one or two cells allowed to shout | the strongest compositional read of all of them | no spark. The accent is what makes the restraint legible as a choice |
| **notan** | two or three values only, hue nearly constant | maximum carrying power | mid-values creeping in and softening the poster read |
| **temperature** | warm-to-cool along a direction, value on its own separate field | depth; warm advances, cool recedes | value follows the same axis (see the rule above) |
| **duet** | two pigments, every colour on the sheet a mix of them | the coherence a limited palette always has | both pigments drawn from a palette's near-neutral ends — two greys mix to grey |
| **by-size** | colour follows the region's *area*, not its position | makes the structure itself legible; a gradation with no spatial gradient | nothing — this one is reliable, and it is underused |

## Two rules that only show up once you build it

**Keep the value out of the colour.** Return the hue and the value as separate
answers. Bake the dilution into the fill and a "near-monochrome" arrangement
reports a hundred distinct pigments, and any caller that wanted to *use* the
value — a wash reading it as pigment load, a relief reading it as height — has
nothing left to read.

**Give each strategy its own field.** Two strategies that sample the same noise
field at the same scale and offset produce the same picture with a different
caption. Offset and rescale per strategy, or you will ship one idea twice.

## Proportion

Harmony is as much about *area* as about hue. The classical ratios — 70/20/10
or 60/30/10 for dominant / secondary / accent — exist because a colour that
occupies half the picture is doing a different job from one that occupies a
twentieth. Implement proportion as a **weighted probability distribution over
the palette**, not as a uniform draw. This is also Hobbs' habit: "70% chance of
white, 20% blue, 10% red".

Where regions differ wildly in size (a foam, a packing), weight by *area* and
not by count, or the twenty slivers outvote the one big lobe.

## Working in HSL/HSB, not RGB

Hold hue constant and vary saturation and brightness to get families that feel
intentional. Small targeted jitter on one component — hue ±3°, or lightness
±0.05 — reads as organic; jitter on all three at once reads as noise.

`internal/palette` has `HSL()`, `FromHSL`, `LerpHSL`, `Lighten`, `Desaturate`
and an `HSB` `Swatch` type with a clamp box, which is the right tool for "this
colour, with room to move".

## Ordering a palette

Most strategies need the palette as a *ramp*, and which ramp matters:

- **by luminance** (`palette.ByLuminance`) — for value structure and for any
  strategy where adjacent entries should agree about lightness.
- **by chroma** — for picking pigments that are actually pigments. Most
  ColorLisa palettes have a near-neutral at each end.
- **by warmth** — hue projected onto the orange axis, for temperature runs.
  A desaturated colour has no temperature; put it in the middle.

Beware: a luminance ramp's neighbours agree about lightness and can disagree
wildly about hue, so a "walk two steps along the ramp" can span the whole
palette. Cut the spread until it reads as a family.

## Checking the result

- **Squint / desaturate.** If it goes flat, add value structure before anything
  else.
- **Sweep, don't stare.** One seed says almost nothing about a colour rule.
  `staticart sweep <sketch> --vary scheme=a,b,c` and look at the sheet.
- **Count the presence.** If the commonest colour holds less than ~25% of the
  marks, the colour is scattering rather than arranging.
- **Look at the smallest regions.** A rule tuned on big shapes often turns the
  slivers into noise.

## Sources

- [Tyler Hobbs — Color Arrangement in Generative Art](https://www.tylerxhobbs.com/words/color-arrangment-in-generative-art)
- [Tyler Hobbs — Working with Color in Generative Art](https://www.tylerxhobbs.com/words/working-with-color-in-generative-art)
- [Notan: creating powerful notans](https://www.virtualartacademy.com/notan/)
- [Colour harmony schemes and proportion](https://www.beyondeveryart.com/color-harmony-schemes/)
