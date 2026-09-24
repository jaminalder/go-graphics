# Getting started

## Requirements

Use Go 1.26.8 from [go.mod](../../go.mod). The studio uses pinned River, pgx and AWS S3 modules; the artwork CLI needs no database or bucket. Formatting/linting uses `golangci-lint`; see [testing](../development/testing.md) for other tools.

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

With Docker/Compose running:

```sh
bash deploy/scripts/browser-server.sh
```

Open `http://127.0.0.1:8280` exactly. The helper builds matching services, starts disposable PostgreSQL/S3 infrastructure, migrates, creates local credentials and starts Caddy/web/renderer. Ctrl-C removes only that helper's projects/volumes. It is a disposable development session, not permanent personal storage. For retained production data and direct-binary configuration see [persistence operations](../operations/persistence.md).

The public catalogue has Pools, Foam and Iris. Local CLI availability does not automatically publish a sketch. The [studio guide](studio.md) explains the artist's workflow; [operations](../operations/running.md) covers readiness and diagnostics.

## Read the documentation in a browser

Install Pandoc and Python 3, then run `make docs`. The builder caches pinned Mermaid for diagrams, requiring network access on the first uncached build. Open `out/docs/index.html`. [Documentation tooling](../development/documentation.md) explains offline builds and validation.
