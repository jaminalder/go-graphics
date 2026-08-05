# Experiment brief

- Name: `warp-nested-fbm`
- Branch: `exp/warp-nested-fbm`
- Worktree: `../go-graphics-worktrees/warp-nested-fbm`
- Base commit: `80ad6d7`
- Stage: new sketch, scalar field and appearance mapping
- Profile: `preview`
- Fixed seeds: `1,2,3,5,8,13,21,34`

## Hypothesis

A nested pair of vector-valued fBM domain warps, with broad spatial variation
in warp activity, will turn plain fBM into flowing structures with distinct
quiet and turbulent passages while preserving multiscale detail and meaningful
variation across seeds.

## Artistic purpose

The experiment should produce an original field-based artwork whose movement
is legible at thumbnail size. Nested warping should organize noise into broad
currents rather than merely making cloud texture more complicated. Restrained
appearance mappings should reveal that structure without rainbow colour or
unrelated marks.

## Stage

Add one self-contained point-sampled sketch: resolved parameters, immutable
warp-field plan, scalar field evaluation, appearance mapping, and raster
rendering. Existing sketches and shared noise behavior remain unchanged.

## Preserve

- Deterministic output for a fixed sketch, options, seed, size, and palette.
- Resolution-independent normalized coordinates.
- A pure, allocation-free ordinary per-pixel path with no random draws.
- The existing command registry and sketch-owned option conventions.
- Zero third-party dependencies.

## Scope

- `docs/sketches/013-warp.md`
- `docs/superpowers/specs/2026-08-05-warp-nested-fbm-design.md`
- `internal/sketch/warp/` implementation, options, focused tests, and golden.
- `cmd/staticart/main.go` registry wiring and its affected tests.
- `experiments/warp-nested-fbm/brief.md` and `result.md`.
- Ignored previews and comparison sheets under `out/warp-nested-fbm/`.

## Out of scope

- Shared field interfaces, node graphs, pipelines, or new shared packages.
- Changes to `internal/noise` unless an existing boundary demonstrably blocks
  the sketch-local implementation.
- Animation, hatching, particles, Voronoi cells, subdivision, or extra layers.
- More than two appearance mappings or the seven requested numeric controls.
- Exhaustive trait design, broad parameter exploration, or production tuning.

## Baseline

There is no pre-existing `warp` sketch on `master`. The required baseline is
the plain fBM mode implemented in the same sketch and compared with one-level
and nested warping at fixed seeds.

```sh
go run ./cmd/staticart sweep warp --seeds 1 --vary warp=plain,single,nested \
  --profile preview --out out/warp-nested-fbm/modes
```

Render the nested candidate seed space with:

```sh
go run ./cmd/staticart sweep warp --seeds 1,2,3,5,8,13,21,34 \
  --warp nested --profile preview --out out/warp-nested-fbm/seeds
```

Keep generated renders under this worktree's ignored `out/` directory.

## Required deliverables

- Working registered `warp` sketch and its specification.
- Plain fBM, one-level warp, and nested warp modes.
- Gradient and structure appearance mappings.
- Focused determinism and finite-value tests.
- Representative previews, an eight-seed contact sheet, and a mode comparison.
- `make check` passing and a brief render-speed observation.
- Coherent commits and a completed result report.
- Stop without merging or removing the worktree.

## Worker restrictions

Operate only inside `/Users/taastbeb/learning_projects/go-graphics-worktrees/warp-nested-fbm`
on `exp/warp-nested-fbm`. Do not switch branches, create or remove worktrees,
merge, rebase, or modify `master`. Do not work outside the assigned scope or
modify another experiment's files.
