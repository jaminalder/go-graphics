# Sketch catalogue

`artwork.Registry` creates 17 definitions: 16 artworks and Hatchbook. The public catalogue separately admits Pools, Foam and Iris with fixed styles, palettes and resource policy. Availability in `staticart list` does not imply web publication.

| Reference | CLI ID | Availability |
| --- | --- | --- |
| [Contour](../sketches/001-contour-noise.md) | `contour` | Local |
| [Tapestry](../sketches/002-tapestry.md) | `tapestry` | Local |
| [Circles](../sketches/003-circles.md) | `circles` | Local |
| [Drift](../sketches/004-drift.md) | `drift` | Local |
| [Rounds](../sketches/005-rounds.md) | `rounds` | Local |
| [Shoal](../sketches/006-shoal.md) | `shoal` | Local |
| [QQL](../sketches/007-qql.md) | `qql` | Local |
| [Pools](../sketches/008-pools.md) | `pools` | Public + local |
| [Foam](../sketches/009-foam.md) | `foam` | Public + local |
| [Scree](../sketches/010-scree.md) | `scree` | Local |
| [Riffle](../sketches/011-riffle.md) | `riffle` | Local |
| [Shallows](../sketches/012-shallows.md) | `shallows` | Local |
| [Warp](../sketches/013-warp.md) | `warp` | Local |
| [Iris](../sketches/014-iris.md) | `iris` | Public + local |
| [Glaze](../sketches/015-glaze.md) | `glaze` | Local |
| [Flame](../sketches/016-flame.md) | `flame` | Local |
| [Hatchbook](../sketches/017-hatchbook.md) | `hatchbook` | Local specimen |

Each page explains the implemented algorithm, trait/control vocabulary, image path, source and tests, then includes the current CLI help generated directly from its sketch definition. [CLI guide](../guides/cli.md) covers the shared commands and [materials](materials.md) distinguishes sampling, sequential painting and histogram development.

Trait schemas are implemented by QQL, Pools, Foam, Scree, Riffle, Shallows, Iris, Glaze and Flame. A weight of zero means an explicitly selected local choice rather than an automatically sampled one. Public trait spaces can apply a stricter allowlist. Source: [registry](../../internal/artwork/registry.go), [public policy](../../internal/publish/catalog.go).
