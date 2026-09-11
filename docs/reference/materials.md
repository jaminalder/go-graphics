# Rendering and material mechanisms

## Deterministic sampling

A `sketch.Context` carries width, height, seed, palette, AA and deep-output request. `RNG(stream)` returns PCG seeded by composition seed and a named stream ID. Pure per-pixel materials use immutable noise/hash lookups instead of drawing mutable random numbers. `render.PixelFunc` is called concurrently, so planned state is immutable during sampling.

Raster coordinates are `(x+offset)/height`, `(y+offset)/height`. For AA factor N, ordinary samplers evaluate N×N subpixels and average in linear light. Eight-bit output uses deterministic interleaved-gradient-noise quantization; deep output uses 16-bit NRGBA without dither. Straight-alpha layer sampling averages premultiplied linear color/alpha then writes unassociated color, avoiding dark translucent fringes. Sources: [context](../../internal/sketch/sketch.go), [raster loops](../../internal/render/render.go), [color conversions](../../internal/palette/color.go).

## Actual image paths

| Path | Sketches | AA / deep behavior |
| --- | --- | --- |
| Pure or planned raster sampler | Contour, Tapestry, Circles, Foam, Scree, Riffle, Shallows, Warp, Iris, Glaze, Hatchbook | Uses context raster/layer adapters for supersampling and 8/16-bit output |
| Sequential paint canvas | Drift, Rounds, Shoal, QQL, Pools | Brush/analytic painting into `paint.Canvas`; `Image()` is 8-bit; global AA/deep are not used as ordinary raster settings |
| Chaos-game histogram | Flame | AA multiplies orbit budget, oversample enlarges histogram; honors deep output after development |

A CLI `--deep` request is accepted for PNG but does not upgrade the painted canvas paths to 16-bit. Painting uses soft/analytic mark edges as implemented by each material; it is not equivalent to sampler N×N AA. See [flame](../sketches/016-flame.md) for its separate density development.

## Color, gradients and schemes

`palette.Color` uses float64 sRGB channels. Palette data keeps artist/artwork provenance; QQL's original swatches retain HSB bounds and sequences. The Zander spindle palette is a separately attributed flame reference palette. [Source assets](source-assets.md) identifies these inputs.

`gradient` supplies cosine, HSL/RGB interpolation, sampled/shuffled discrete gradients and terracing. `scheme` chooses hue and value per region ahead of sampling: passage, gradient, sequence, inherit, dominance, complement, analogous, triad, quiet, notan, terrace, anchor, weather, duet, by-size and by-darkness. A scheme decides region appearance; it does not paint pixels itself. Source: [schemes](../../internal/scheme/strategies.go), [ramp](../../internal/scheme/ramp.go).

## Geometry and fill queries

`geom` supplies circle collision/containment queries through a spatial index. `cells` makes an Apollonius partition, comparing distance minus site weight, and can merge sites into one region. `Hit` reports a cell, neighbor, wall distance and junction information; a fixed-grid plan derives approximate region area, centroid and inradius. These values let fills and lighting depend on the same region structure. Scree's facets are secondary partitions; each facet receives a shade at its centroid. Source: [cells](../../internal/cells/cells.go), [Scree facets](../../internal/sketch/scree/facets.go).

## Paint and water

`paint.Canvas` stores sequential floating-point RGB working pixels and interprets stamps, bristle sweeps, washes and rings in drawing order. A brush is a mechanism for placing paint; sketches own mark placement and artistic parameters. The canvas costs approximately 24 bytes per pixel before its output image and encoding buffers. [Performance](../performance.md) explains how that affects parallel large renders.

`FlatWash` is a separate pure field material: it evaluates pigment/paper behavior at coordinates inside a caller-supplied region. It can be combined with structural sampling without pre-rendering a full image. `hatch` similarly returns only scalar ink coverage; callers assign pigment and compose it with other layers. See [hatching](../hatching.md).

## Composition before rasterization

Shallows samples Scree's `Bed` with Riffle's `Surface` at each coordinate; refraction, shadow and highlights affect the bed before a final pixel is written. Glaze samples the same bed under a nested-noise painted absorption veil in linear light. These are typed component collaborations, not composited full-frame image files. Source: [Shallows](../../internal/sketch/shallows/shallows.go), [Glaze water](../../internal/sketch/glaze/water.go), [Riffle surface](../../internal/sketch/riffle/surface.go).
