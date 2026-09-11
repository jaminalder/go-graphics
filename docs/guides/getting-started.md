# Getting started

## Requirements

Use the Go version declared in [go.mod](../../go.mod): 1.26.8. The Go application has no third-party module dependencies. Formatting/linting uses `golangci-lint`; browser, container and documentation checks have separate tool requirements in [testing](../development/testing.md).

Run commands from the repository root. `out/` and `bin/` are generated, ignored directories.

## Render a local image

```sh
go run ./cmd/staticart list
go run ./cmd/staticart palettes
go run ./cmd/staticart render contour --seed 42 --profile preview --out out
```

The command prints the image path. Open the PNG to inspect it. The default preview is 600×600. The default palette is `kandinsky-soft-pressure`. Repeating the same settings and seed preserves the composition; embedded command/trait metadata records the choices.

```sh
make build
./bin/staticart render qql --seed 42 --profile preview-tall --out out
./bin/staticart sweep pools --seeds 1-12 --out out/pools-review
```

The sweep writes a contact sheet and manifest. [CLI guide](cli.md) explains flags and output. [Sketch catalogue](../reference/sketches.md) links every algorithm and its current controls. Flame's first visual review needs more detail than the generic preview: use `make preview-flame` for a 1000×1000 image at quality 80.

## Run the local public studio

Build matching web and renderer binaries, then run the renderer and web process in separate terminals, both from the repository root:

```sh
mkdir -p out
go build -o out/artweb ./cmd/artweb
go build -o out/artrender ./cmd/artrender
```

Terminal one:

```sh
./out/artrender
```

Terminal two:

```sh
./out/artweb
```

Open `http://127.0.0.1:8080` exactly: the server checks the canonical Host. Both binaries default to build identity `development` and socket `out/artrender.sock`. A parent socket directory must already exist. Stop both with Ctrl-C. This direct mode provides process deadlines and output bounds, but the Linux container limits are exercised by [Compose](../operations/running.md).

The public catalogue has Pools, Foam and Iris. Local CLI availability does not automatically publish a sketch. The [studio guide](studio.md) explains the artist's workflow; [operations](../operations/running.md) covers readiness and diagnostics.

## Read the documentation in a browser

Install Pandoc and Python 3, then run `make docs`. The builder caches pinned Mermaid for diagrams, requiring network access on the first uncached build. Open `out/docs/index.html`. [Documentation tooling](../development/documentation.md) explains offline builds and validation.
