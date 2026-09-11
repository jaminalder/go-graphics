# Fractal flame implementation

The implemented flame engine is an iterated function system (IFS) that accumulates orbit density. It is not a per-pixel point sampler. A transform chooses an affine mapping and weighted nonlinear variations; weighted transitions select subsequent transforms. Iterating this small system produces a histogram of spatial hits and color coordinates.

[internal/flame](../../internal/flame/system.go) owns orbit iteration, histogram accumulation and development. [Variations](../../internal/flame/variation.go) implement the nonlinear transforms; [the flame sketch](../sketches/016-flame.md) chooses genomes, ranges, camera and palette policy. Its deterministic framing pass uses a separate stream from the quality-dependent main sample budget.

The output is developed through logarithmic density, optional density estimation and oversampling/downsampling. [Xaos and density estimation](flame-xaos-de.md) describes those concrete mechanisms. The source's behavior is the implementation contract; this page is not a reproduction of an external flame renderer specification.

For visual review use the [retained Apophysis reference](apophysis-flame.jpg) and render at least 1000×1000 at quality 80 (`make preview-flame`). A smaller generic preview obscures filament/grain structure. Color/geometry quality requires visual inspection across fixed seeds in addition to numeric tests.
