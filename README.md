# go-graphics

Generative static 2D art in Go — deterministic sketches rendered as
high-resolution images for print, plus web and preview sizes, from the same
seed.

Successor in spirit to [staticart](https://github.com/jaminalder/staticart)
(Clojure/quil). Raster work uses the Go standard `image` library; future
vector work will use [tdewolff/canvas](https://github.com/tdewolff/canvas).
Color palettes are grounded in artist palettes from
[ColorLisa](https://colorlisa.com/).

**Status:** twelve artwork sketches, from contour fields and packed forms to
watercolour, faceted river stones, flowing water, and `shallows`, which
combines its generated stone bed and rippled surface in one material render.
All 133 ColorLisa palettes are built in (`staticart palettes`; `staticart
list` shows the complete set).

## Quick start

Requires Go ≥ 1.26.5 and, for `make fmt`/`make lint`, golangci-lint ≥ 2.12.

```sh
make help                 # list targets
make check                # fmt + vet + lint + test
make preview              # render the contour sketch at preview size → out/

# General form (once cmd/staticart exists):
go run ./cmd/staticart render contour --profile print --seed 42 \
    --palette hokusai-great-wave --out out
```

| Profile | Size | Use |
|---|---|---|
| `preview` | 600² px | fast iteration |
| `web` | 2000² px | web/social |
| `print` | 6000² px | ≈ 50×50 cm at 300 DPI |

Same seed ⇒ same composition at every size. **For final print renders use
`--aa 3`** (9× supersampling; ~2–3× render time) and optionally `--deep`
for a 16-bit PNG master. All files embed sRGB + 300 DPI metadata and the
full render recipe (view with `strings file.png | grep staticart`).

## Project layout

```
cmd/staticart/     CLI
internal/          mathx, rnd, opt, palette, gradient, noise, geom,
                   trait, paint, render, sketch packages
docs/              ARCHITECTURE.md, per-sketch specs, ColorLisa reference data
out/               rendered images (gitignored)
```

- **Design & invariants:** [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **First sketch spec:** [docs/sketches/001-contour-noise.md](docs/sketches/001-contour-noise.md)
- **Agent/contributor guide:** [AGENTS.md](AGENTS.md)
- **Branch/worktree workflow:** [docs/WORKTREE-WORKFLOW.md](docs/WORKTREE-WORKFLOW.md)

## Credits & inspiration

- [jaminalder/staticart](https://github.com/jaminalder/staticart) — reference
  artwork and algorithms
- [ColorLisa](https://colorlisa.com/) — artist color palettes
- Iñigo Quílez — [cosine gradient palettes](https://iquilezles.org/articles/palettes/)
