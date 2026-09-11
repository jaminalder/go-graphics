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

Go version and application dependencies are declared in [go.mod](go.mod); the Go application currently uses only the standard library. Documentation uses Python, Pandoc and pinned Mermaid. [Development workflow](docs/development/workflow.md) and [worktree rules](docs/WORKTREE-WORKFLOW.md) explain contribution and review.
