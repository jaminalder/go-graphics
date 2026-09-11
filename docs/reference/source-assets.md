# Palette and visual source assets

These retained references are source inputs or useful visual acceptance material for implemented artworks. They are not roadmap or research backlog documents.

| Asset | Current purpose |
| --- | --- |
| [colorlisa-palettes.md](colorlisa-palettes.md) | Source dataset read by `tools/genpalettes`; generated Go data keeps artist/work provenance |
| [qql-colordata.json](qql-colordata.json) | QQL HSB swatches and city palette definitions read by `tools/genqqlpalettes` |
| [target-sketch7.jpg](target-sketch7.jpg) | Visual comparison reference for Contour |
| [target-rounds.png](target-rounds.png) | Reference for brush-painted circular marks in Rounds |
| [target-shoal.png](target-shoal.png) | Reference for Shoal's field of small painted marks |
| [apophysis-flame.jpg](apophysis-flame.jpg) | Visual spindle reference for the implemented flame family/cast |

The two palette datasets are preserved byte-for-byte; their generators depend on their current formats and paths. Regenerate Go data with `go run ./tools/genpalettes` and `go run ./tools/genqqlpalettes` from the repository root. Do not hand-edit generated palette files.

The web catalogue is separate: [web/catalog/manifest.json](../../web/catalog/manifest.json) records canonical recipes/digests for image assets embedded by [web catalogue validation](../../internal/web/catalog.go). `go run ./tools/webcatalog` makes candidates in `out/catalog`; selection and copying into embedded assets is deliberate. Runtime rendering does not fetch palettes or reference images from the internet.

[Palette implementation](../../internal/palette/palette.go), [generated palette data](../../internal/palette/data.go), [QQL color sets](../../internal/sketch/qql/colorset.go) and [flame cast](../../internal/sketch/flame/cast.go) own the actual color behavior.
