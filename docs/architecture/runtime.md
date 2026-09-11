# Dynamic views: rendering and recovery

## Creating a four-image preview batch

Scope: one successful studio entry with a cache miss. Audience: developers and operators. Participants are containers or the artist from the [static model](containers.md).

```mermaid
---
title: "Dynamic views: rendering and recovery — Creating a four-image preview batch"
---
sequenceDiagram
    actor Artist as Artist · Person
    participant Web as Public studio · Go container
    participant Supervisor as Renderer supervisor · Go container
    participant Child as Render child · Go container
    participant Cache as Image cache · PNG store
    Artist->>Web: 1. POST choices and action token / HTTP form
    Web->>Web: 2. Validate form, resolve recipes, admit all four
    Web-->>Artist: 3. Redirect to exploration / HTTP 303
    loop Each admitted cache miss, serially
        Web->>Supervisor: 4. Request rendition / HTTP JSON over Unix socket
        Supervisor->>Child: 5. Start child and send request / exec + JSON stdin
        Child-->>Supervisor: 6. Complete image and exit / PNG stdout
        Supervisor-->>Web: 7. Return image and build / HTTP PNG
        Web->>Cache: 8. Validate and publish completed PNG / filesystem
    end
    Artist->>Web: 9. Poll fragment or reload / HTTP GET
    Web-->>Artist: 10. HTML status and image URLs
    Artist->>Web: 11. Request ready image / HTTP GET
    Web->>Cache: 12. Open published image / filesystem
    Web-->>Artist: 13. Return PNG with immutable cache headers
```

Key: participant labels state person/container/store types and technology. Solid arrows are requests or actions; dashed arrows are replies. Numbers establish ordering; the loop is serial work. HTML polling does not create jobs.

Admission may reuse completed or in-flight identities, so a hit skips execution. The queue accepts all four interests or none. The supervisor has no second queue. Each child has a deadline and bounded output; a failed child produces a failed job, never a partial published PNG. Cancellation releases one owner's interest and stops shared work only when no subscribers remain. See [Manager](../../internal/renderjob/manager.go), [Supervisor](../../internal/renderjob/protocol.go), [studio admission](../../internal/studio/studio.go).

## Similar images and downloads

A similarity request selects one existing sample. The studio pins its complete traits and palette and chooses fresh seeds for four new compositions. This is stricter than the generic candidate planner's mixed family policy: all four public results retain the selected visual family. Favourites remain immutable recipes when current choices change.

A download is a separate explicit POST. It admits the same recipe at the server-owned 1200×1200 rendition; a ready download does not need another render. The browser enhancement clicks the ready download link after polling. Without JavaScript the artist prepares the image, refreshes and uses the available link. A GET of an expired image returns 410; it never regenerates that image.

## Manual API export and restore

The HTTP routes support the sequence below, but the current templates do not provide an export/restore form. Export is a manually visited URL; restore requires a form-capable technical client.

```mermaid
---
title: "Dynamic views: rendering and recovery — Manual API export and restore"
---
sequenceDiagram
    actor Artist as Artist · Person
    participant Web as Public studio · Go container
    participant Files as Local artwork files · JSON store
    Artist->>Web: 1. Export favourites / HTTP GET
    Web-->>Artist: 2. Versioned recipe JSON / HTTP attachment
    Artist->>Files: 3. Retain downloaded recovery file / browser file save
    Artist->>Files: 4. Read recovery JSON / local file access
    Artist->>Web: 5. Submit recovery JSON in fresh session / HTTP form POST
    Web->>Web: 6. Validate every record and rebuild favourites
    Web-->>Artist: 7. Redirect to restored exploration / HTTP 303
    Artist->>Web: 8. Explicitly request an image download / HTTP form POST
```

Key: participants are the artist, web container and user-owned file store. Solid arrows are requests/actions; dashed arrows are replies. Numbers show sequence. Restore allocates navigation and recipes only; step 8 is what requests rendering. Source: [Recovery and Restore](../../internal/studio/studio.go), [web export/restore routes](../../internal/web/web.go).
