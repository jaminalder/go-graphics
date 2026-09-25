# Component views

Each diagram decomposes one application/process from the [container view](containers.md). Audience: developers. Components are in-process responsibilities, not additional Docker services.

## Public studio components

Scope: `artweb`.

```mermaid
---
title: "Component view: artweb"
---
flowchart TB
    artist["Artist · Person / browser"]
    subgraph app["artweb · Go application"]
        http["Presentation · Go web, HTML, JavaScript<br/>Forms, CSRF, images and SSE"]
        domain["Studio domain · Go studio.Store<br/>Recipes, revisions, replay and favourites"]
        persistence["Aggregate persistence · Go studio.Persistent<br/>Loads/saves one workspace transaction"]
        queue["Admission · Go persistence.QueueTx + River<br/>Coalesces interests and inserts jobs"]
        listener["Result listener · Go persistence.Events<br/>One LISTEN connection, local fan-out"]
        images["Image reader · Go objectstore<br/>Bounded HTTPS read and digest check"]
    end
    db[("PostgreSQL · Data store<br/>Studio aggregates, River jobs and pointers")]
    bucket[("Private S3 bucket · Data store<br/>Immutable PNG objects")]
    artist -->|HTTP forms and reads| http
    http -->|Domain commands / Go calls| persistence
    persistence -->|Apply bounded domain behavior / Go calls| domain
    domain -->|Transaction-bound job operations / Go calls| queue
    persistence -->|Workspace and pin transactions / SQL| db
    queue -->|River InsertTx and status / SQL| db
    db -->|Committed result hints / NOTIFY| listener
    listener -->|Wake local authorized streams / Go channels| http
    http -->|Resolve image pointer / SQL| db
    http -->|Fetch retained image / Go call| images
    images -->|Read PNG / HTTPS S3 GET| bucket
    http -->|SSE hints and HTML/images / HTTP| artist
```

Key: the enclosing box is one application; inside boxes are components, outside boxes are a person and stores. Arrows identify directional calls/transports. Web does not call a renderer or execute jobs. River's web client is insert-only.

[web](../../internal/web/web.go) binds request context to [studio.Persistent](../../internal/studio/persistent.go). It loads one bounded aggregate and invokes the existing [studio domain](../../internal/studio/studio.go) with a [QueueTx](../../internal/persistence/queue.go) sharing that transaction. Navigation, replay and job admission commit together. The legacy in-memory Manager and socket client remain for existing unit tests; production commands do not instantiate them.

Private admin HTTP also exposes a read-only [monitor snapshot](../../internal/persistence/monitor.go), consumed by artctl rather than the public browser. Web and renderer write boot-specific presence records; enqueue stores producer metadata. The [host-side terminal helper](../../deploy/scripts/watch.py) resolves Docker names, without adding a monitoring container or application access to Docker.

Both runtime services load the validated [limit policy](../../internal/limits/policy.go) at startup. Private [load timing queries](../../internal/persistence/loadstats.go) let the operator's k6 launcher collect server evidence separately from public HTTP requests; no load generator gets database credentials. Image delivery uses a [bounded byte/reader reservation](../../internal/web/image_budget.go) before S3 fetch and through response writing.

## Renderer service components

Scope: the `artrender` parent process.

```mermaid
---
title: "Component view: artrender parent"
---
flowchart TB
    db[("PostgreSQL · Data store<br/>River and application records")]
    bucket[("Private S3 bucket · Data store<br/>PNG bytes")]
    subgraph app["artrender · Go application"]
        river["River client · Go library<br/>Claims, retries and internal maintenance election"]
        worker["Render handler · Go persistence.Worker<br/>Attempt guard, validation and publication"]
        supervisor["Supervisor · Go renderjob.Supervisor<br/>Fixed executable, deadlines and output bounds"]
        cleanup["Application maintenance · Go persistence.Maintainer<br/>Expiry, intents and deletion reconciliation"]
        objects["Object client · Go objectstore<br/>S3 PUT, LIST, DELETE and health"]
    end
    child["Render child · Go process<br/>Computes one PNG"]
    river -->|Claim, retry and schedule / SQL| db
    db -->|Work and control hints / NOTIFY| river
    river -->|Job context / Go call| worker
    river -->|Periodic maintenance job / Go call| cleanup
    worker -->|Render request / Go call| supervisor
    supervisor -->|exec and JSON stdin| child
    child -->|PNG stdout| supervisor
    worker -->|Upload image / Go call| objects
    objects -->|HTTPS S3 operations| bucket
    worker -->|Serializable pointer, outcome and completion / SQL| db
    cleanup -->|Ownership and lifecycle transactions / SQL| db
    cleanup -->|List/delete unreferenced objects / Go calls| objects
```

Key: the enclosing box is the parent process. River maintenance is library code inside that process, not another service. The separate child shares its Docker network/resource boundary. All renderers consume independently; one River client is elected for internal queue maintenance.

[Worker](../../internal/persistence/worker.go) verifies canonical identity and fences publication using River attempt state and application generation/epoch. [Supervisor](../../internal/renderjob/protocol.go) kills/reaps child processes on deadline/output overflow. [Maintainer](../../internal/persistence/maintenance.go) performs application cleanup as River jobs, separately from River's own scheduler/rescuer/cleaner.

## Render child components

Scope: one `artrender --child` process.

```mermaid
---
title: "Component view: disposable render child"
---
flowchart LR
    parent["Renderer parent · Go application<br/>Owns child lifecycle"]
    subgraph child["Render child · Go process"]
        input["Input validation · Go renderjob.Child<br/>Bounded recipe/build/tier decoding"]
        recipe["Recipe execution · Go artwork/publish<br/>Reconstructs validated edition"]
        algorithm["Artwork · Go sketch packages<br/>Deterministic image computation"]
        encoding["PNG encoding · Go render<br/>Pixels and recipe metadata"]
    end
    parent -->|JSON stdin| input
    input -->|Go calls| recipe
    recipe -->|Go calls| algorithm
    recipe -->|Encode rendered image / Go call| encoding
    encoding -->|PNG stdout| parent
```

Key: internal boxes are components of one process; parent is an external application. Pipes carry requests/results. The child does not use River or S3 credentials in its input/environment, but same-container execution is not a sandbox for hostile code. Source: [child](../../internal/renderjob/protocol.go), [recipe](../../internal/artwork/recipe.go), [render](../../internal/render/meta.go).

## Local renderer components

Scope: `staticart`, independent of PostgreSQL and S3.

```mermaid
---
title: "Component view: staticart"
---
flowchart TB
    developer["Developer · Person"]
    subgraph app["staticart · Go application"]
        command["Commands · Go cmd/staticart<br/>Render, sweep and flock control"]
        registry["Definitions · Go artwork/sketch/trait<br/>Fresh factories and resolved controls"]
        exploration["Exploration · Go explore<br/>Candidate seeds and traits"]
        algorithms["Artwork · Go sketch packages<br/>Image computation"]
        output["Output · Go render<br/>Images, metadata and contact sheets"]
    end
    files[("Local artwork files · Filesystem store")]
    developer -->|Command arguments| command
    command -->|Go calls| registry
    command -->|Go calls| exploration
    command -->|Go calls| algorithms
    command -->|Go calls| output
    command -->|Read/write flock records / filesystem| files
    output -->|Write artwork and sheets / filesystem| files
```

Key: components are enclosed in one CLI application; person/store are outside. Arrows identify calls/file operations. Sources: [CLI](../../cmd/staticart/main.go), [flock](../../cmd/staticart/flock.go), [explore](../../internal/explore/explore.go), [package map](../reference/packages.md).
