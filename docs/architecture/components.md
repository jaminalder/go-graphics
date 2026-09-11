# Component views

Each diagram below decomposes one container from the [container view](containers.md). Audience: developers. Components are coherent interfaces inside a process; source package names locate their implementations.

## Public studio components

Scope: the `artweb` container.

```mermaid
---
title: "Component views — Public studio components"
---
flowchart TB
    person["Artist · Person<br/>Browses and submits choices"]
    subgraph app["artweb · Go container"]
        http["Presentation · Component / Go web + HTML + JS<br/>Validates forms and renders pages"]
        state["Studio state · Component / Go studio.Store<br/>Owns workspaces, revisions and samples"]
        policy["Publication and exploration · Component / Go publish + explore<br/>Resolves allowed traits into complete recipes"]
        jobs["Job admission and cache · Component / Go renderjob.Manager<br/>Coalesces requests, schedules one worker and publishes images"]
        client["Renderer client · Component / Go renderjob.Client<br/>Checks build-bound private transport"]
    end
    supervisor["Renderer supervisor · Container / Go<br/>Executes one bounded child"]
    cache["Image cache · Container: data store / PNG filesystem<br/>Stores completed renditions"]
    person -->|Submits forms and polls pages / HTTP| http
    http -->|Reads snapshots and applies commands / Go calls| state
    http -->|Opens images / Go calls| jobs
    state -->|Completes permitted recipes / Go calls| policy
    state -->|Admits or cancels jobs and reads status / Go calls| jobs
    jobs -->|Requests rendition / Renderer interface| client
    client -->|Renders and checks health / HTTP over Unix socket| supervisor
    jobs -->|Publishes and opens completed PNGs / filesystem calls| cache
```

Key: the enclosing box is one container. Inside boxes are Go components; outside boxes are a person, another container and a store. Arrows are directional calls/requests, labeled with their implementation mechanism. Layout has no semantic meaning.

[cmd/artweb](../../cmd/artweb/main.go) constructs dependencies, configures listeners, adds structured HTTP logging and handles shutdown. [web](../../internal/web/web.go) owns host, origin, form and CSRF checks; [studio](../../internal/studio/studio.go) owns navigation and command semantics; [publish](../../internal/publish/catalog.go) owns the public allowlist; [explore](../../internal/explore/explore.go) proposes deterministic candidates; [Manager](../../internal/renderjob/manager.go) is the only queue. Embedded templates and assets are presentation implementation, not independent backend services.

## Renderer supervisor components

Scope: the `artrender` supervisor container.

```mermaid
---
title: "Component views — Renderer supervisor components"
---
flowchart LR
    web["Public studio · Container / Go<br/>Owns request admission"]
    subgraph renderer["Supervisor · Go container"]
        transport["Private HTTP handler · Component / Go net/http<br/>Parses bounded requests and reports build health"]
        execution["Child supervision · Component / Go os/exec<br/>Allows one child; enforces timeout and output limits"]
        validation["Publication validation · Component / Go publish + artwork<br/>Checks release, recipe and rendition policy"]
    end
    child["Render child · Container / Go process<br/>Renders one recipe"]
    web -->|Requests PNG / HTTP JSON over Unix socket| transport
    transport -->|Validates request / Go calls| validation
    transport -->|Runs accepted rendition / Go calls| execution
    execution -->|Starts process and writes request / exec + JSON stdin| child
    child -->|Returns image / PNG stdout| execution
```

Key: the enclosing box is the supervisor process. Typed internal boxes are components; external boxes are containers. Arrows name directional interactions and protocols. The child is outside the supervisor boundary because it is a separate process.

[cmd/artrender](../../cmd/artrender/main.go) creates the private socket and handles signals. [protocol.go](../../internal/renderjob/protocol.go) implements handler, validation and supervision. It also contains the child entry function and client code compiled into other containers. A shared source file does not imply shared runtime memory.

## Render child components

Scope: one `artrender --child` process.

```mermaid
---
title: "Component views — Render child components"
---
flowchart LR
    supervisor["Renderer supervisor · Container / Go<br/>Owns child lifecycle"]
    subgraph child["Child · Go container"]
        input["Request decoder · Component / Go renderjob.Child<br/>Reads and validates one request"]
        recipe["Recipe execution · Component / Go artwork + publish<br/>Reconstructs concrete artwork and fixed rendition"]
        artwork["Artwork implementation · Component / Go pools, foam or iris<br/>Plans and paints or samples an image"]
        encode["Image encoding · Component / Go render<br/>Writes PNG and deterministic metadata"]
    end
    supervisor -->|Provides one request / JSON stdin| input
    input -->|Executes validated recipe / Go calls| recipe
    recipe -->|Creates and renders fresh sketch / Go calls| artwork
    recipe -->|Encodes completed image / Go calls| encode
    encode -->|Returns completed rendition / PNG stdout| supervisor
```

Key: one process boundary encloses Go components. The external box is the supervising container. Arrows name call or pipe direction; placement is layout only. [Child and transport](../../internal/renderjob/protocol.go), [recipe rendering](../../internal/artwork/recipe.go) and [materials](../reference/materials.md) establish these responsibilities.

## Local renderer components

Scope: the `staticart` container.

```mermaid
---
title: "Component views — Local renderer components"
---
flowchart TB
    developer["Developer · Person<br/>Renders and reviews artwork"]
    subgraph cli["staticart · Go container"]
        command["Command and batch control · Component / Go cmd/staticart<br/>Parses flags and schedules render, sweep or flock"]
        registry["Definitions and traits · Component / Go artwork.Registry + sketch + trait<br/>Creates fresh definitions and resolves options"]
        algorithms["Artwork implementations · Component / Go sketch packages<br/>Plan and render local artworks"]
        exploration["Candidate planning · Component / Go explore<br/>Breeds trait and seed candidates"]
        output["Image and sheet output · Component / Go render<br/>Encodes images, metadata and contact sheets"]
    end
    files["Local artwork files · Container: data store / filesystem<br/>Images and flock records"]
    developer -->|Invokes command / command arguments| command
    command -->|Creates and configures artwork / Go calls| registry
    command -->|Plans flock candidates / Go calls| exploration
    command -->|Renders configured sketch / Go calls| algorithms
    command -->|Encodes results / Go calls| output
    command -->|Reads and writes flock records / JSONL filesystem calls| files
    output -->|Writes images and sheets / filesystem calls| files
```

Key: one CLI process encloses Go components. Outside boxes identify the person and local store. Arrows describe calls or file operations. Source: [command](../../cmd/staticart/main.go), [sweep](../../cmd/staticart/sweep.go), [flock](../../cmd/staticart/flock.go), [exploration](../../internal/explore/explore.go). The [package map](../reference/packages.md) details shared mechanisms without giving each leaf its own C4 box.
