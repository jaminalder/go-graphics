# Singular Seed documentation

Singular Seed creates deterministic static artwork with Go. The local `staticart` command exposes 16 artworks and a hatch specimen book. The public studio offers curated editions of Pools, Foam and Iris through HTML forms, with rendering isolated in a separate process. The repository also contains the deployment and documentation tooling.

This documentation describes the checked-in implementation. It does not assert that a particular release is currently deployed. Every architectural view links to the source that establishes its claims.

## Start here

- [Run a first image or the local studio](guides/getting-started.md).
- [Use the browser studio](guides/studio.md).
- [Render, sweep and breed from the command line](guides/cli.md).
- [Browse all artworks](reference/sketches.md).

## Understand the system

The [architecture index](ARCHITECTURE.md) follows C4 from the whole system to selected code: [context](architecture/context.md), [containers](architecture/containers.md), [components](architecture/components.md) and [code](architecture/code.md). [Runtime interactions](architecture/runtime.md) and [deployment](architecture/deployment.md) explain execution and hosting. [Model conventions](architecture/model.md) explain the scope and notation.

## Reference

- [HTTP interfaces](reference/http-api.md), [data and identities](reference/data.md), [configuration](reference/configuration.md).
- [Packages and tools](reference/packages.md), [rendering and materials](reference/materials.md), [hatching](hatching.md).
- [Palette and visual source assets](reference/source-assets.md).

## Operate and develop

- [Run and diagnose services](operations/running.md), [build and activate releases](operations/releases.md), [provision a host](operations/provisioning.md), [security boundaries](operations/security.md).
- [Development workflow](development/workflow.md), [tests](development/testing.md), [performance](performance.md), [branches and worktrees](WORKTREE-WORKFLOW.md).
- [Build and maintain this documentation](development/documentation.md).

Run `make docs` to generate the complete browser edition at `out/docs/index.html`. Every Markdown page participates in the same navigation and reading layouts. Generated output remains outside this folder.
