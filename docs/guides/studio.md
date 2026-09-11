# Using the public studio

Singular Seed presents three curated artworks: Pools (overlapping pigment circles), Foam (inked cells) and Iris (radial fibres). Start from the gallery, open an artwork, choose an optional style and colour, and request images. Each successful request admits four samples; they become available individually while the remaining images are made.

## Choices and similarity

| Artwork | Styles |
| --- | --- |
| Pools | Spacious circles, Flowing strands, Gathered colour |
| Foam | Ink and paper, Painted cells, Open cells |
| Iris | Fine fibres, Winding fibres, Layered rings |

Every artwork offers Tide, Earth, After dark and Meadow. An unpinned style keeps its public trait space open; an unpinned colour lets the studio choose. These visual choices map to explicit trait/palette pins in the [publication catalogue](../../internal/publish/catalog.go). Local command-line numerical controls are not exposed on the public site.

Open a sample to keep it as a favourite, request similar images or prepare a download. Similarity preserves the selected sample's complete traits and palette while new seeds vary its composition. Changing current choices does not change an already kept sample.

## Download and share

Previews are 600×600 PNG. Downloads are 1200×1200 PNG of the same recipe. Preparing a download is explicit work; with JavaScript the browser begins downloading once ready. Without JavaScript, refresh the page and follow the available download link. Native sharing is offered where the browser supports sharing image files; otherwise download the image and share it from local files.

Downloaded PNG metadata contains the canonical recipe and renderer build information. Public image URLs contain a content-derived rendition key and are served without session authentication while the artifact is available. Treat a copied URL as access to that image, not as a durable saved favourite.

## Favourites and recovery

The session keeps up to 24 favourites across at most four explorations. The server keeps navigation in memory, expiring after 30 minutes idle or 24 hours total, and losing it on a web restart. A browser cookie identifies the workspace; it is not an account. This implementation does not persist favourites to browser local storage.

While a session is active, manually visiting `/export` downloads `art-favourites.json`. The current page templates do not expose export/restore controls or a recovery form. The implemented `/restore` form endpoint accepts that JSON through a technical HTTP client in a fresh session without existing explorations. It validates every recipe against currently available editions and rebuilds kept samples without rendering. See [HTTP interfaces](../reference/http-api.md) for the protocol. Download image files through the normal interface to retain the artwork itself.

## Busy, stale and failed pages

An active batch must finish before another is generated in that exploration. The HTTP API supports cancellation of unfinished interests, but the current page templates do not expose a cancel control. A failed/unavailable latest batch can be retried explicitly from the interface. Refreshing or polling never retries a render automatically.

A stale form can report a conflict because another tab changed the exploration revision. Refresh the page to use its current state. Expired sessions and images report expiry. Rate or capacity limits show a pause message; wait and submit again. JavaScript enhances polling, navigation and sharing; ordinary forms remain the base interaction.

Source: [templates](../../internal/web/templates/page.html), [browser enhancement](../../internal/web/assets/studio.js), [studio state](../../internal/studio/studio.go), [HTTP behavior](../reference/http-api.md), [data lifetimes](../reference/data.md).
