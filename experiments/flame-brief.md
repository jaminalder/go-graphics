# Experiment brief

- Name: flame
- Branch: `exp/flame`
- Worktree: `../worktrees/flame`
- Base commit: 013bc15
- Stage: new sketch 016
- Profile: `1000×1000` at `--quality 80` (first-look review; never 600)
- Fixed seeds: `1,2,3,5,8,13`

## Hypothesis

A fractal-flame chaos game, with log-density colouring and a spindle
genome, can sit in this repo as a fourth artwork lifecycle and produce
images in the family of Zander's 2007 Apophysis render.

## Artistic purpose

Give the system a genuinely different kind of picture — luminous
filaments on a void, coloured by recursive path — without forcing the
algorithm through a point sampler.

## Stage

New mechanism `internal/flame` and new sketch `internal/sketch/flame`.

## Preserve

Determinism, resolution-independent camera, ColorLisa or a documented
non-ColorLisa ramp (`zander-spindle`), zero third-party dependencies.

## Scope

Engine, sketch, spec, architecture decisions 56–59, preview path.
Xaos, flam3 density estimation, and oversample+Gaussian downsample
are in.

## Out of scope

Motion blur, XML genome import, GPU, flam3's 16384-wide xaos LUT and
full spatial-filter menagerie (Gaussian only).

## Baseline

None: this is a new artwork.

```sh
make preview-flame
go run ./cmd/staticart sweep flame --seeds 1-12 --width 1000 --height 1000 --quality 80 --palette zander-spindle --structure spindle --out out/flame-sweep
```
