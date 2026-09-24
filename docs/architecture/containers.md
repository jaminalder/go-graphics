# Container view: Singular Seed

Scope: logical applications and stores. Audience: developers/operators. Docker placement is in [deployment](deployment.md).

```mermaid
---
title: "Container view: persistent Singular Seed"
---
flowchart TB
    artist["Artist · Person"]
    developer["Developer · Person"]
    operator["Operator · Person"]
    subgraph system["Singular Seed · Software system"]
        web["Public studio · Go HTTP/HTML/SSE application"]
        database[("PostgreSQL · Studio state, River jobs and artifact pointers")]
        renderer["Renderer service · Go/River worker and child supervisor"]
        child["Render child · Disposable Go process"]
        bucket[("Private S3 bucket · PNG objects")]
        cli["Local renderer · Go staticart application"]
        ctl["Private probes · Go artctl application"]
        admin["Database administration · Go artdb application"]
        files[("Local files · User-retained artwork/recipes")]
    end
    artist -->|HTTPS forms and images| web
    developer -->|Command arguments| cli
    operator -->|Command arguments| ctl
    operator -->|Explicit migration and control commands| admin
    ctl -->|Loopback health and admission HTTP| web
    ctl -->|Loopback process/storage health HTTP| renderer
    admin -->|SQL migrations, activation and queue controls| database
    admin -->|Explicit prefix binding verification / HTTPS| bucket
    web -->|SQL transactions and River enqueue| database
    database -->|Committed change notifications| web
    renderer -->|River claims, completion and maintenance SQL| database
    database -->|Work notifications| renderer
    renderer -->|exec and recipe stdin| child
    child -->|PNG stdout| renderer
    renderer -->|HTTPS upload and cleanup| bucket
    web -->|HTTPS reads| bucket
    cli -->|Filesystem writes| files
    web -->|Browser-mediated downloads| files
```

Key: person, process/application and store types are explicit. Arrows name directional interactions. No direct web-renderer communication exists. The child is a separate process inside the renderer Docker container, sharing its network/resources.

- [artweb](../../cmd/artweb/main.go) owns presentation and insert-only River use; [persistent studio](../../internal/studio/persistent.go) saves domain state and jobs atomically.
- [artrender](../../cmd/artrender/main.go) consumes River, supervises one child and uploads results. River internally elects one started client for queue maintenance; no coordinator container.
- [PostgreSQL](../../internal/persistence/schema.sql) owns anonymous identities, navigation aggregates, requests/interests, pointers, upload intents, controls and rate buckets. River owns its own execution/history/leadership tables.
- [Object store](../../internal/objectstore/store.go) holds private PNG bytes. SQL contains keys/digests/sizes, never images.
- [artctl](../../cmd/artctl/main.go) probes private HTTP; [artdb](../../cmd/artdb/main.go) performs explicit migrations and release/queue control.
- [staticart](../../cmd/staticart/main.go) compiles artwork libraries and requires neither database nor bucket.

See [components](components.md), [runtime](runtime.md), [data](../reference/data.md) and [persistence operations](../operations/persistence.md).
