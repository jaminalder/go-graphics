# go-graphics / Singular Seed

Deterministic static artwork in Go, with a local renderer and a curated public browser studio. The local registry contains sixteen artworks and a hatch specimen book; the studio publishes Pools, Foam and Iris. The repository includes isolated rendering, deployment tooling and a complete C4 architecture reference.

```sh
go run ./cmd/staticart render contour --seed 42 --profile preview --out out
```

[Getting started](docs/guides/getting-started.md) covers local image generation and the studio. [Documentation home](docs/README.md) links user guides, the [C4 architecture](docs/ARCHITECTURE.md), all sketches, API/data references, operations and development checks.

```sh
make check   # Required before commits
make docs    # Complete browser edition at out/docs/index.html
```

Go version and pinned River/pgx/S3 dependencies are in [go.mod](go.mod). The CLI needs no database; the studio uses PostgreSQL and a private image bucket. See [persistence operations](docs/operations/persistence.md). Documentation uses Python, Pandoc and pinned Mermaid.

Product and infrastructure development is trunk-based on `master`; branches/worktrees are reserved for artistic experiments. The [implementation record](docs/plans/postgresql/implementation.md) records decisions, local evidence and external validation boundaries.
