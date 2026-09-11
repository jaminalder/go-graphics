# Container view: Singular Seed

Scope: applications and data stores inside the Singular Seed software system. Audience: developers and operators. C4 containers are logical runtime units; Docker placement is in [deployment](deployment.md).

```mermaid
---
title: "Container view: Singular Seed"
---
flowchart TB
    artist["Artist · Person<br/>Explores and downloads artwork"]
    operator["Operator · Person<br/>Controls private service availability"]
    developer["Developer · Person<br/>Creates local images"]
    subgraph system["Singular Seed · System"]
        web["Public studio · Container / Go HTTP + HTML + JavaScript<br/>Owns navigation, admission and image serving"]
        supervisor["Renderer supervisor · Container / Go<br/>Validates requests and bounds one child"]
        child["Render child · Container / Go process<br/>Computes one public rendition and exits"]
        cache["Image cache · Container: data store / PNG filesystem<br/>Disposable completed public images"]
        ctl["Operator control · Container / Go artctl<br/>Probes health and controls admission"]
        cli["Local renderer · Container / Go CLI<br/>Renders, sweeps and breeds all local sketches"]
        files["Local artwork files · Container: data store / filesystem<br/>Images, recovery JSON and flock records"]
    end
    artist -->|Browses and submits forms / HTTPS in hosted environment| web
    web -->|Requests rendition / HTTP JSON over Unix socket| supervisor
    supervisor -->|Starts child and sends one request / process exec + JSON stdin| child
    child -->|Returns one image / PNG stdout| supervisor
    supervisor -->|Returns rendition / HTTP PNG over Unix socket| web
    web -->|Publishes and opens images / filesystem calls| cache
    web -->|Delivers download or recovery record via browser / HTTP PNG or JSON| files
    operator -->|Runs private operations / command arguments| ctl
    ctl -->|Probes and controls services / HTTP loopback| web
    ctl -->|Checks build readiness / HTTP over Unix socket| supervisor
    developer -->|Selects artwork and settings / command arguments| cli
    cli -->|Writes images, manifests and flock records / filesystem calls| files
    cli -->|Reads parent flock / JSONL filesystem read| files
```

Key: typed boxes are persons, applications or stores. The enclosing box is the software system boundary. Each arrow names a directional interaction and transport. Box size and color carry no extra meaning. The browser mediates downloads into the artist's filesystem; the server does not write the user's disk directly.

## Responsibility and state

| Container | Entry point or ownership | State and lifetime |
| --- | --- | --- |
| Public studio | [artweb](../../cmd/artweb/main.go) | Workspaces, revisions, favourites, rate buckets and sole render queue in memory; one process |
| Renderer supervisor | [artrender](../../cmd/artrender/main.go) | Unix listener and one active-child flag; no queue or image cache |
| Render child | [Child](../../internal/renderjob/protocol.go) via `artrender --child` | One bounded request, fresh sketch and image; exits after output |
| Image cache | [Manager](../../internal/renderjob/manager.go) | Bounded PNG directory, reconciled on web startup |
| Operator control | [artctl](../../cmd/artctl/main.go) | One private command, run inside the appropriate service container |
| Local renderer | [staticart](../../cmd/staticart/main.go) | One command; directly compiles and calls artwork code |
| Local artwork files | CLI output and browser downloads | User-retained files; no automatic service backup or account association |

The studio's server-rendered pages and browser enhancement code form one application, following the [model convention](model.md). No browser local-storage database exists. Recovery is explicit JSON export/import. `artctl` is a separate operator command used against private listeners, including Compose health checks; its process runs inside the appropriate service container.

Both the local renderer and the render child compile artwork libraries. The local renderer does not call the public supervisor. The web process resolves public recipes but never executes their rasterization. No containers communicate with a database or broker.

See [components](components.md) for internal ownership, [data](../reference/data.md) for persistence and [runtime](runtime.md) for ordered interactions.
