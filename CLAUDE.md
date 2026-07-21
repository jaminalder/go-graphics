# CLAUDE.md — agent guide for go-graphics

Generative static 2D art in Go. Deterministic sketches → PNG/JPEG at preview,
web, and print resolution. Stdlib raster now; `tdewolff/canvas` later for
vector work. Zero third-party dependencies at the moment — keep it that way
unless a documented decision says otherwise.

## Commands

```sh
make check      # fmt + vet + lint + test — MUST pass before any commit
make test       # go test ./...
make lint       # golangci-lint (v1.64 local; config .golangci.yml)
make preview    # render sketch "contour" at preview size into out/
go run ./cmd/staticart render <sketch> --profile preview|web|print --seed N --palette <name> --out out
go run ./cmd/staticart list
```

Rendered files land in `out/` (gitignored), named
`<sketch>_<palette>_<seed>_<WxH>.<ext>`.

## Verify visually — always

Tests prove determinism, not beauty. After changing any sketch or color code:
render a preview (`make preview` or the render command) and **Read the output
PNG** to look at it. For sketch 001 compare against the target image at
`docs/reference/target-sketch7.jpg` using its spec's acceptance checklist.

## Docs map (read before working on the related area)

- `docs/ARCHITECTURE.md` — package layout, dependency rules, **core
  invariants**, testing strategy, decision log. Update the decision log when
  making a non-obvious choice.
- `docs/sketches/NNN-<name>.md` — one spec per sketch: algorithm, tunables,
  acceptance checklist. Write the spec before implementing a new sketch.
- `docs/reference/colorlisa-palettes.md` — full ColorLisa palette dataset
  (source data for `internal/palette`; don't re-fetch the website).

## Invariants (breaking these is a bug even if output looks fine)

1. **Determinism**: all randomness from `Context.Seed` via `math/rand/v2`
   PCG or seed-derived tables. Never `time.Now()`, never global rand.
   Same (sketch, params, seed, size) → byte-identical image.
2. **Resolution independence**: sample in normalized coords
   (`v ∈ [0,1]`, `u ∈ [0, aspect]`); frequencies are cycles-per-canvas-unit,
   never per-pixel. Preview and print of the same seed must match.
3. **Color**: `float64` sRGB in `[0,1]` everywhere internally; clamp at 8-bit
   conversion. Palettes originate from the ColorLisa data and keep
   artist/artwork provenance.
4. **Dependency direction**: `cmd → sketch → {gradient, noise, render} →
   palette → stdlib`. `palette` and `noise` import stdlib only. No art logic
   in `cmd`.

## Layout

```
cmd/staticart/        CLI (wiring only)
internal/palette/     Color type + ops, ColorLisa palette data
internal/gradient/    cosine / sampled / shuffled gradients
internal/noise/       Perlin + fBm (in-repo, no deps)
internal/render/      parallel pixel loop, size profiles, PNG/JPEG encode
internal/sketch/      Sketch interface, Context, registry
internal/sketch/<x>/  one package per sketch + its testdata/ goldens
```

## How to add a sketch

1. Write `docs/sketches/NNN-<name>.md` (algorithm, tunables, acceptance
   checklist).
2. Create `internal/sketch/<name>/` implementing the `sketch.Sketch`
   interface; tunables are struct fields with defaults in `New()`.
3. Register it in the registry wiring used by `cmd`.
4. Tests: determinism (64×64, same seed twice → identical) + golden PNG in
   `testdata/` (regenerate with `-update`, eyeball before committing).
5. Render previews, check against the spec's acceptance list, iterate.

## Engineering standards

- TDD for math-heavy code (color, gradient, noise, mapping): table-driven
  tests first. Don't overengineer elsewhere — no abstractions for
  single-implementation concepts.
- `gofumpt` + `goimports` formatting (local prefix
  `github.com/jaminalder/go-graphics`) — `make fmt`.
- Exported identifiers get doc comments; sketch packages start with a short
  package comment describing the artwork.
- Commit `testdata/` goldens; never commit `out/`.
