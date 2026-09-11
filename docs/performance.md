# Performance and resource behavior

## Measure the boundary that changed

List benchmark entry points from source with `rg '^func Benchmark' internal cmd tools`, then run the relevant package with `go test -run '^$' -bench . -benchmem ./internal/<package>`. Planning, point sampling, sequential painting, histogram iteration and encoding have different costs; compare the same seeds/settings and record dimensions, AA, Go version and machine.

For a whole public rendition, `tools/artbench` runs cold isolated children and emits per-render JSONL with wall/CPU/RSS/byte/error measurements, plus wall-time p50/p95/p99 and failure-count summaries:

```sh
go build -o out/artrender ./cmd/artrender
go run ./tools/artbench --renderer out/artrender --build development --count 100 --tier preview
```

Count is bounded 1–1000. Tier is the public preview/download policy. The build must match the renderer executable. These measurements use actual public styles and separate processes, with a 35-second command deadline. They are machine-specific measurements, not permanent performance promises. Source: [benchmark utility](../tools/artbench/main.go).

## Memory and parallelism

`paint.Canvas` stores three float64 channels, approximately 24 bytes per pixel. At 6000×6000 that working canvas alone is 864,000,000 bytes (about 824 MiB), before the 8-bit image, temporary marks and encoding. Multiple sweep/flock workers multiply such memory pressure. Samplers keep compact planned geometry but still allocate the final image; deep NRGBA64 uses eight bytes per pixel rather than four.

Raster AA factor N costs N×N samples per output pixel. Pure row workers use Go runtime parallelism. Flame instead budgets approximately `quality × width × height × max(AA,1)` orbit samples, and oversampling increases histogram storage with the square of the factor. Density estimation adds development work after accumulation. [Materials](reference/materials.md) explains path-specific quality semantics.

## Public bounds

The public catalogue fixes square 600/1200-pixel PNG renditions at AA1, restricts expensive trait choices and forbids numeric overrides. One web worker renders serially through one supervisor child, bounded by 15-second preview and 30-second download deadlines. Compose provides an additional renderer 2 GiB memory / 1.5 CPU limit with `GOMAXPROCS=2`. These controls cap resource use; they do not guarantee every admitted candidate finishes on every machine.

See [cache/queue limits](reference/data.md), [runtime isolation](architecture/deployment.md), [queue implementation](../internal/renderjob/manager.go) and [supervisor](../internal/renderjob/protocol.go). Performance tuning requires both measurements and fixed-seed visual inspection when sampling/development changes.
