# Experiment brief — flame wash develop

- Name: flame-wash
- Parent: `exp/flame` (sketch 016), to be continued on a fresh worktree after
  integration
- Branch (next): `exp/flame-wash`
- Worktree (next): `../worktrees/flame-wash`
- Stage: second Develop path for the existing histogram — not a new IFS
- Profile: `1000×1000` at `--quality 80` (same first-look rule as 016)
- Fixed seeds: hero set from flock curation — `26`, `100029`, plus
  `1,2,3,5,8,13` for regression against ember

## Hypothesis

The same attractor measure, interpreted as pigment load on paper instead of
emissive log-density on a void, can read as abstract hand-painted fractals:
wet edges, mottled pooling, paper tooth — without stamping radial pools along
orbit points.

## Artistic purpose

Flame today is a nebula. The interesting next picture is not a softer nebula;
it is the spindle/nest *drawn in watercolour* — translucent crossings, pigment
banking at density falloffs, structural colour (cyan nest / gold mass) meeting
as two pigments rather than as HDR gleam.

## Stage

`internal/flame` Accumulate stays. Add a wash Develop beside ember Develop.
`internal/sketch/flame` owns the medium trait and paper defaults.

## Preserve

- Genome, xaos, camera, histogram, oversample/downsample
- Colour-by-xform (structure colour), cast / tint
- Determinism; byte-identical double render
- Ember path (`--medium ember` or default) byte-stable for existing seeds

## Scope

1. **`--medium wash`** (weight 0 on a new or appended dimension), default
   remains ember/void so no existing seed moves.
2. **Wash Develop**: map log-density α → pigment strength; map accumulated
   colour → palette pigment; composite with `paint.FlatWash` / `paint.Glaze`
   onto paper. No gleam, no void-nebula defaults.
3. **One manner trait** resolving to ranges (not five flags), starting with:
   - **stain** — FlatWash over density (fastest abstract read)
   - later: **rimmed** / **pooled** / **charged** if stain holds
4. Density-edge cue for rimmed: approximate a wall from α (iso-α band or
   soft |∇α|), reuse foam’s pigment maths only — not `cells.Hit`.
5. Spec note under 016 or a short `017` only if it becomes its own artwork;
   preference is medium on 016.
6. Sweep stain vs ember on the fixed seeds; Read the sheets.

## Out of scope

- Stamping `paint.Wash` pools along chaos-game points
- Changing Accumulate, variations, or flock
- Blurring the developed PNG and calling it watercolour
- A generic HistogramRenderer or paint plugin registry

## Architecture

```text
genome → Accumulate → Hist
                    ├─ Develop(ember)  → nebula (today)
                    └─ Develop(wash)   → load + FlatWash/Glaze on paper
```

Mechanism stays in `internal/flame` only where the tone map is shared math;
wash character (tooth, blotch, rim from density) is sketch policy or a thin
call into existing `paint` — extract nothing until a second consumer appears.

## Baseline (after next worktree exists)

```sh
go run ./cmd/staticart sweep flame --seeds 26,100029,1,2,3,5,8,13 \
  --profile preview --quality 80 --out out/flame-wash-baseline
```

## Candidate

Same command with `--medium wash` (and later `--vary manner=stain,rimmed`).

## Acceptance

On paper ground, a wash spindle should:

1. Read as translucent pigment, not as a glowing object on black.
2. Keep structural cool/warm roles without magenta trenches.
3. Show uneven pooling / tooth; not a uniform soft blur of the ember render.
4. Crossings mix toward a believable third via absorption, not overwrite.
5. Ember of the same seed remains unchanged.

## Recommendation fork

Ship **stain** first. Only invest in density-as-wall rims if stain already
reads painted rather than “flame with paper background.”
