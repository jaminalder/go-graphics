# Architecture

Design document for `go-graphics` — a Go project for generative static 2D art.
Read this before changing package structure or adding cross-cutting features.
For day-to-day agent workflow (commands, conventions, how to add things), see
[../CLAUDE.md](../CLAUDE.md).

## 1. Vision & scope

Produce aesthetic, print-quality static 2D images in Go:

- **Raster/pixel work** (fractals, noise fields, per-pixel logic) uses the Go
  standard library (`image`, `image/color`, `image/png`, `image/jpeg`).
- **Vector work** (shapes, curves, typography — future) will use
  [`github.com/tdewolff/canvas`](https://github.com/tdewolff/canvas). It is
  **not** a dependency yet; add it only when the first vector sketch lands.
- Every artwork must render at multiple sizes: small previews for fast
  iteration, web-size, and high-resolution print output — from the *same*
  deterministic definition.
- Colors are grounded in proven artist palettes from
  [ColorLisa](https://colorlisa.com/) (see
  [reference/colorlisa-palettes.md](reference/colorlisa-palettes.md)); sketches
  may desaturate/manipulate them but start from that data.

Reference for the kind of output we want:
[jaminalder/staticart](https://github.com/jaminalder/staticart) (Clojure/quil).
The first sketch reproduces
[shuffled_grad_palette_scale_2.jpg](https://github.com/jaminalder/staticart/blob/master/shuffled_grad_palette_scale_2.jpg)
— spec in [sketches/001-contour-noise.md](sketches/001-contour-noise.md).

## 2. Domain concepts

| Concept | Meaning | Package |
|---|---|---|
| `Color` | One color, `float64` R/G/B in `[0,1]`, sRGB. Manipulations: desaturate, lighten, mix, conversion to/from `image/color` and hex. | `internal/palette` |
| `Palette` | Named, ordered set of colors with provenance (artist, artwork). The ColorLisa data lives here as Go data. | `internal/palette` |
| `Gradient` | A function `t ∈ [0,1] → Color`. Implementations: cosine (Iñigo Quílez form), discrete sampled, shuffled-discrete, multi-band. | `internal/gradient` |
| Noise / field | Deterministic scalar fields `f(x, y) → float64`, e.g. Perlin + fBm octaves. Seedable, no global state. | `internal/noise` |
| `Sketch` | One artwork algorithm: deterministic function of (params, seed, size) → `image.Image`. | `internal/sketch`, one subpackage per sketch |
| Render | Pixel-loop execution (parallel), size profiles, PNG/JPEG encoding. | `internal/render` |

## 3. Package layout & dependency rules

```
cmd/staticart/            CLI: generic flag parsing + wiring only, no art logic
internal/
  mathx/                  Clamp01, Remap, Smoothstep         → (stdlib only)
  palette/                Color type, manipulation, data     → mathx
  gradient/               Gradient implementations           → palette, mathx
  geom/                   circles + spatial index            → (stdlib only)
  noise/                  Perlin, fBm, Worley, Hash01        → (stdlib only)
  paint/                  stamp canvas, wobbly paths, disc
                          marks (rings/scribble/gouache)     → palette, render, mathx
  render/                 pixel loop (AA, dither), profiles,
                          encoding + metadata                → palette
  sketch/                 Sketch/Configurable, Context,
                          registry, Raster helper            → palette, render
    sketchtest/           shared test helpers (goldens etc.) → sketch
    contour/, tapestry/, circles/, drift/   the sketches     → all of the above
docs/                     this file, sketch specs, idea backlog, reference data
out/                      rendered images (gitignored)
```

Dependency direction (arrows = "may import"):

```
cmd → sketch (registry) → {gradient, noise, render} → palette → mathx → stdlib
```

Rules:

- `mathx`, `geom`, and `noise` are leaf packages: stdlib imports only.
- Sketches live in subpackages of `internal/sketch`; `cmd` discovers them
  only through the registry. Sketch-specific CLI options are owned by the
  sketch via the `Configurable` interface — `cmd` stays sketch-agnostic.
- Nothing imports `cmd`. No package keeps global mutable state.
- New third-party dependencies require a documented decision (§8). Current
  count: **zero**.

### What changes where (knob taxonomy)

When making variants of a sketch, changes fall into four tiers — keep each
knob in its tier:

1. **Seeds** (CLI, per render): composition (`--seed`) plus sketch-owned
   sub-seeds (`--terrace-seed`, `--grain-seed`) that re-deal one aspect on
   a fixed composition. Cheap exploration; embedded in the recipe.
2. **Optional layers/effects** (CLI flags): `--relief`, `--crackle`,
   `--no-stripes`, `--smooth`, … Toggles must not disturb the base
   composition (dedicated RNG streams).
3. **Aesthetic ranges** (code): the bounded per-seed draw ranges and
   structural constants in each sketch. Changing them changes every seed —
   do it deliberately, update the sketch spec, regenerate goldens.
4. **Quality/output** (CLI, composition-neutral): `--aa`, `--deep`,
   `--format`, profiles.

## 4. Core invariants

These are load-bearing; breaking them is a bug even if output "looks fine".

1. **Determinism.** Same (sketch, params, seed, width, height) → identical
   image, on every run and platform. All randomness flows from the seed via
   `math/rand/v2` (`rand.New(rand.NewPCG(seed, …))`) or seed-derived tables.
   Never `time.Now()`, never package-global `rand`.
2. **Resolution independence.** Sketches sample in *normalized* coordinates:
   `v = (y+0.5)/H ∈ [0,1]`, `u = (x+0.5)/W · aspect` with `aspect = W/H`.
   Frequencies are "cycles per canvas unit", never per pixel. A preview render
   and a print render of the same seed must show the same composition, just at
   different pixel densities.
3. **Color space.** Internal color math is `float64` sRGB components in
   `[0,1]`; clamp on conversion to 8-bit. (Good enough for this art style;
   revisit with a decision entry if we ever need linear-light blending.)
4. **Palettes carry provenance.** Every palette knows its artist/artwork
   source. Derived palettes (desaturated etc.) are computed from the originals
   at use-time, not stored as mutated copies.

## 5. Rendering pipeline

```
Sketch.Render(ctx)
  └─ builds its plan/scene from ctx.Palette + ctx.RNG (seed)
  └─ sketch.Raster(ctx, func(u, v float64) palette.Color)
       └─ render.RasterSS / RasterDeep: parallel rows, ctx.AA supersamples
          per pixel averaged in linear light, dithered 8-bit (or 16-bit
          with ctx.Deep)
cmd/staticart
  └─ render.WritePNGMeta / WriteJPEGMeta: sRGB tag, 300 DPI, full render
     recipe + code revision embedded (render.Meta)
  └─ filename: <sketch><option-suffix>_<palette>_<seed>_<WxH>.<ext>
```

Per-pixel work must be pure (no shared mutable state) so the row-parallel loop
is race-free by construction.

**Second rendering model:** `paint.Canvas` (used by stroke-based sketches
like drift) is stamp-based and sequential — soft dabs blended source-over,
anti-aliasing from the dab edges (Context.AA unused), same dithered
quantization on output. Compositions stay resolution-independent; stroke
texture varies subtly with resolution, which is part of the medium.

### Size profiles

| Profile | Pixels (square default) | Purpose |
|---|---|---|
| `preview` | 600 × 600 | fast iteration, visual checks by agents |
| `web` | 2000 × 2000 | matches the original staticart canvas |
| `print` | 6000 × 6000 | ≈ 50 × 50 cm at 300 DPI |

`--width/--height` override for custom/non-square sizes. Non-square must work
for every sketch (invariant 2 handles composition).

## 6. Sketch contract

```go
// internal/sketch
type Context struct {
    Width, Height int
    Seed          uint64
    Palette       palette.Palette
    AA            int  // supersampling per axis; composition-neutral
    Deep          bool // 16-bit output
}

// RNG returns a generator derived from (Seed, stream). Sketches use distinct
// stream ids per consumer (shuffle A, shuffle B, …) so adding a consumer
// never disturbs the values existing streams produce.
func (c Context) RNG(stream uint64) *rand.Rand

type Sketch interface {
    Name() string        // CLI id, kebab-case, e.g. "contour"
    Describe() string    // one line for `staticart list`
    Render(ctx Context) (image.Image, error)
}

// Optional: sketches with CLI options implement Configurable —
// Flags(fs) registers them, Configure() applies them and returns the
// output-filename suffix. cmd never type-asserts concrete sketches.
```

- Sketch-specific tunables are fields on the sketch struct with defaults in a
  `New()` constructor — not a generic params map. Add CLI flags per sketch
  only when actually needed for exploration.
- Each sketch subpackage registers itself via `sketch.Register(New())` from
  the registry wiring in `cmd` (explicit imports, no `init()` magic beyond a
  single registration call).

## 7. Testing strategy

- **Unit tests** (table-driven) for all math: color conversions and
  manipulation, gradient sampling endpoints/monotonicity, noise value range,
  interval mapping. Write these first (TDD) — the math is where silent bugs
  live.
- **Determinism tests**: render a sketch twice at 64×64 with the same seed,
  require byte-identical pixels; different seeds must differ.
- **Golden-image tests**: 64×64 PNG per sketch committed under
  `internal/sketch/<name>/testdata/`; compare pixel-exact (helpers in
  `internal/sketch/sketchtest`). Regenerate deliberately with
  `make golden` when a sketch intentionally changes, and eyeball the new
  goldens before committing. Goldens render at AA 1.
- **Visual verification is part of done**: render a preview and *look at it*
  (agents: `Read` the PNG). Tests prove determinism, not beauty.

## 8. Decision log

| # | Decision | Rationale |
|---|---|---|
| 1 | Module path `github.com/jaminalder/go-graphics` | Matches owner's GitHub handle + directory name (user-confirmed). |
| 2 | Implement Perlin/fBm noise in-repo | ~150 LoC, zero deps, full control over seeding/determinism; good base for turbulence/curl later (user-confirmed). |
| 3 | No CI for now; local `make check` is the gate | Repo is local-first (user-confirmed). Add GitHub Actions when pushed. |
| 4 | Stdlib raster now, `tdewolff/canvas` deferred | First sketches are pure pixel work; don't take the dependency before the first vector sketch. |
| 5 | Reproduce the *look* of sketch_7, not bit-exact output | Original uses thi.ng simplex noise + JVM shuffle; we use our own Perlin. Thresholds/scales are tuned visually against the reference image. |
| 6 | `float64` sRGB color math, no linear-light pipeline | The source material (thi.ng gradients) also interpolates in sRGB; simpler, matches reference. Revisit if blending artifacts appear. |
| 7 | golangci-lint v2 config format | Migrated from v1 on 2026-07-29 alongside the Go 1.26.5 bump; local toolchain is v2.12.x. `make fmt` now runs `golangci-lint fmt`, so formatting and linting share one config (`formatters:` section) instead of invoking gofumpt/goimports separately. |
| 8 | Gradient interpolation in HSL space for new work | RGB-space blends between distant hues gray out; HSL (shortest hue arc) stays vivid and closer to the palette (user request 2026-07-22). `palette.LerpHSL` + `gradient.HSLBetween`. The contour sketch keeps RGB cosine gradients for fidelity to the original staticart port. |
| 9 | Output pipeline: linear-light AA averaging, dithered 8-bit quantization (IGN), optional 16-bit masters, embedded metadata | Averaging supersamples in sRGB darkens high-contrast edges — AA averages in linear light (partially supersedes #6). 8-bit quantization dithers deterministically to prevent banding in slow gradients; `--deep` renders 16-bit PNG masters. Every written file embeds sRGB tag, 300 DPI, and the full render recipe + code revision (`render.Meta`) so artworks stay self-describing. |

## 9. Roadmap

1. **Sketch 001 "contour"** — shuffled-gradient contour noise (spec:
   [sketches/001-contour-noise.md](sketches/001-contour-noise.md)).
2. Palette library: full ColorLisa dataset + manipulation ops.
3. More noise-family sketches (turbulence, domain warping).
4. First vector sketch → introduce `tdewolff/canvas`.
5. Possible later: fractals, tilings, SVG export, gallery index generator.
