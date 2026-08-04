# Performance

Representative measurements after the architecture migrations, at revision
`affaca5` plus the benchmark changes described here. These numbers are local
engineering evidence, not cross-machine performance promises.

## Environment

```text
OS/architecture: darwin/arm64
CPU: Apple M1 Pro
Logical CPUs: 10
Memory: 34359738368 bytes (32 GiB)
Go: go1.26.5
```

## Microbenchmarks

Command:

```sh
go test -run '^$' -bench . -benchmem \
  ./internal/sketch/contour \
  ./internal/sketch/foam \
  ./internal/sketch/qql \
  ./internal/sketch/shallows
```

Results:

| Workload | Time/op | Bytes/op | Allocations/op |
|---|---:|---:|---:|
| contour plan | 5.787 us | 7,248 | 12 |
| contour sample | 33.62 ns | 0 | 0 |
| contour render 128x128 | 450.3 us | 74,184 | 38 |
| foam plan | 39.77 ms | 17,760 | 191 |
| foam sample | 595.3 ns | 0 | 0 |
| foam render 96x96 | 40.11 ms | 62,389 | 223 |
| QQL plan | 417.7 ms | 7,160,698 | 3,060 |
| QQL paint existing plan at 64x80 | 10.80 ms | 143,440 | 4 |
| shallows stone-bed plan | 230.0 ms | 1,829,720 | 22,781 |
| shallows surface sample | 11.82 us | 0 | 0 |
| shallows render 96x96 | 252.0 ms | 1,886,352 | 22,859 |
| shallows render 192x192 | 311.2 ms | 1,992,108 | 22,858 |

The direct sample paths allocate nothing. That is the important architectural
result: plans own their construction cost, while parallel raster workers only
evaluate immutable data. Riffle's water surface is intentionally expensive
per coordinate because every sample performs an upstream line-integral walk;
shallows therefore scales much more steeply with pixels than contour or foam.

Planning has different dominant costs:

- contour builds one Perlin table and three short gradients;
- foam measures a weighted partition on a fixed normalized grid;
- QQL constructs its flow grid, traces candidate lines, and collision-packs
  a large mark set;
- shallows builds and measures the stone and facet partitions, including
  precomputed flat facet light.

QQL's painting benchmark reuses one plan. It demonstrates that the expensive
composition is not repeated by the painter.

## Profile scaling

The CLI was built once with `make build`. Each workload was rendered at AA1
under `/usr/bin/time -l`; the table reports elapsed wall time and maximum
resident set size.

### Commands

```sh
/usr/bin/time -l ./bin/staticart render contour --profile preview|web|print --aa 1 --seed 42 --out <dir>
/usr/bin/time -l ./bin/staticart render foam --profile preview|web|print --aa 1 --seed 42 --out <dir>
/usr/bin/time -l ./bin/staticart render qql --profile preview-tall|web-tall|print-tall --aa 1 --seed 42 --out <dir>
/usr/bin/time -l ./bin/staticart render shallows --profile preview|web|print --aa 1 --seed 12 --water-seed 42 --out <dir>
```

### Results

| Artwork | Profile | Dimensions | Elapsed | Max RSS |
|---|---|---:|---:|---:|
| contour | preview | 600x600 | 0.12 s | 11.5 MB |
| contour | web | 2000x2000 | 1.37 s | 38.2 MB |
| contour | print | 6000x6000 | 12.39 s | 258.6 MB |
| foam | preview | 600x600 | 0.20 s | 12.2 MB |
| foam | web | 2000x2000 | 1.77 s | 39.5 MB |
| foam | print | 6000x6000 | 15.34 s | 260.4 MB |
| QQL | preview-tall | 480x600 | 0.56 s | 21.8 MB |
| QQL | web-tall | 1600x2000 | 1.21 s | 106.3 MB |
| QQL | print-tall | 4800x6000 | 6.25 s | 874.4 MB |
| shallows | preview | 600x600 | 1.11 s | 12.6 MB |
| shallows | web | 2000x2000 | 10.87 s | 40.2 MB |
| shallows | print | 6000x6000 | 95.76 s | 263.7 MB |

All print runs completed.

## Interpretation

Contour and foam scale approximately with pixel count once their small or
fixed planning costs are amortized. Foam's partition measurement does not
grow with output resolution; this is both a determinism and a performance
property.

Shallows is compute-bound. Its upstream walk occurs per output sample and is
the material evaluation, so replacing it with a pre-rendered water image would
be faster but would lose refraction and bed-integrated light. The architecture
keeps that trade visible rather than hiding it behind a generic sampler chain.

QQL is the memory outlier. `paint.Canvas` stores one `palette.Color` of three
`float64` channels per pixel before producing the final NRGBA image. A
4800x6000 canvas therefore needs about 691 MB for its color buffer alone, and
the measured process reached 874 MB. Direct raster sketches allocate the
output image without that full-frame float intermediate and remained near
260 MB at 6000x6000.

This is not a new regression from the refactor; it is the explicit cost of the
sequential painting model. A future paint-memory project could tile compatible
marks or store a narrower linear representation, but it must preserve
order-dependent blending and is outside this architecture migration.

## Guardrails

- Keep direct `At`/sample methods allocation-free.
- Build partitions, spatial indices, schemes, and per-facet appearance once.
- Do not move RNG, trait resolution, or geometry construction into raster
  workers.
- Benchmark planning separately from painting or sampling when changing a
  structural sketch.
- Treat a full-frame `paint.Canvas` as a deliberate material cost; do not use
  one for a composition that can remain a compact point sampler.
- Use representative print measurements before adding interface chains or
  extra per-coordinate field work.
