# go-graphics

Generative static 2D art in Go — deterministic sketches rendered as
high-resolution images for print, plus web and preview sizes, from the same
seed.

Successor in spirit to [staticart](https://github.com/jaminalder/staticart)
(Clojure/quil). Raster work uses the Go standard `image` library; future
vector work will use [tdewolff/canvas](https://github.com/tdewolff/canvas).
Color palettes are grounded in artist palettes from
[ColorLisa](https://colorlisa.com/).

**Status:** groundwork — architecture and specs are in place, implementation
of the first sketch is next.

## Quick start

Requires Go ≥ 1.26 and (for linting) golangci-lint + gofumpt.

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

Same seed ⇒ same composition at every size.

## Project layout

```
cmd/staticart/     CLI
internal/          palette, gradient, noise, render, sketch packages
docs/              ARCHITECTURE.md, per-sketch specs, ColorLisa reference data
out/               rendered images (gitignored)
```

- **Design & invariants:** [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **First sketch spec:** [docs/sketches/001-contour-noise.md](docs/sketches/001-contour-noise.md)
- **Agent/contributor guide:** [CLAUDE.md](CLAUDE.md)

## Credits & inspiration

- [jaminalder/staticart](https://github.com/jaminalder/staticart) — reference
  artwork and algorithms
- [ColorLisa](https://colorlisa.com/) — artist color palettes
- Iñigo Quílez — [cosine gradient palettes](https://iquilezles.org/articles/palettes/)
