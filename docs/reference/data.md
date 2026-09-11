# Data, identities and persistence

## Domain vocabulary

| Term | Meaning and owner |
| --- | --- |
| Sketch | Local artwork algorithm implementing `sketch.Sketch` |
| Trait schema / set | Weighted discrete output dimensions / one resolved choice per dimension |
| Public entry | Explicitly admitted artwork, styles and colours in `publish` |
| Recipe | Canonical, validated complete artistic choices for one edition |
| Rendition | Pixel dimensions, AA and encoding for a recipe |
| Workspace | Temporary capability-owned studio navigation |
| Exploration | One revisioned artwork direction and its samples/batches |
| Sample | Immutable recipe with preview/download interests and favourite state |
| Batch | Four sample identifiers, optionally retaining a similarity parent |
| Job | Coalesced rendition work with subscribing workspaces |
| Artifact | Completed PNG identified by the rendition key |

## Recipe format and identity

A canonical record has `recipe_version: 1`, `artwork_id` (`pools`, `foam` or `iris`), `edition: "1"`, a decimal **string** `seed`, a palette slug, and a concrete `config` object. The string preserves the full unsigned 64-bit seed across JavaScript/JSON consumers. Config types and JSON fields are defined in [Pools](../../internal/sketch/pools/config.go), [Foam](../../internal/sketch/foam/config.go) and [Iris](../../internal/sketch/iris/config.go); they include resolved traits and optional pointer-valued numeric overrides. Public canonical records serialize complete traits while omitting numeric overrides; the edition implementation and seed resolve numeric/material ranges during rendering. Obtain actual canonical examples from [catalogue manifest](../../web/catalog/manifest.json), an exported favourite, or a PNG's metadata.

`artwork.Decode` rejects recipes larger than 16384 bytes, unsupported versions/editions, invalid seeds/configs and unknown palettes. Strict JSON rejects duplicate keys, unknown fields, trailing data, over-deep nesting and oversized collections. Decoding resolves traits and re-encodes a canonical record. Public validation permits only the published spaces and four curated palettes, and rejects numeric overrides by reconstructing the allowed record.

Recipe digest is SHA-256 of canonical recipe bytes. Rendition key is SHA-256 of canonical recipe bytes followed by JSON encoding of the rendition and the immutable renderer build string. An identical recipe at another size/build has a different key. The PNG's byte digest is separate and is used as its ETag. Recipe edition and build are distinct: edition identifies supported artistic configuration; build binds execution/cache output to a release.

## Public state lifetimes and bounds

| State | Storage | Bounds / restart behavior |
| --- | --- | --- |
| Workspaces | Web memory | At most 1000; 30-minute idle, 24-hour absolute lifetime; lost on restart |
| Explorations | Workspace memory | At most four per workspace; independently revisioned |
| Batches/action replay | Exploration memory | Latest eight retained; samples retained for these batches, parents and favourites |
| Favourites | Workspace memory | At most 24; export needed for durable recovery |
| Canonical recipe memory | Web memory | Admission reserves within a 32 MiB budget |
| Queue | Web memory | Eight waiting jobs by default plus one running; at most four active interests per owner |
| Job subscribers | Web memory | Up to 24 per coalesced identity |
| Job records | Web memory | Bounded to 5000; old completed state pruned after 30 minutes |
| PNG cache | Web-owned directory | Default 2 GiB, 5000 artifacts, maximum age 24 hours; reconciled on startup |
| Open image readers | Web process | Bounded to 64 globally and 32 per artifact; active references protect artifacts from eviction |
| Browser cookie | Browser cookie jar | Opaque workspace token, maximum age 24 hours; server expiry still applies |
| Recovery JSON / image downloads | User filesystem | User-retained; not synchronized or automatically backed up by the app |

Queued work expires when it has waited over one minute. The serial worker prefers an owner different from the previous job when one is available, reducing domination by one workspace. Completed/failed state is only retried by an explicit command, never a status read.

The cache is disposable. Startup reconciles eligible filenames, size/age and file digests; it does not decode all cached PNGs. New publication validates PNG content/dimensions, writes a temporary file, syncs and renames it atomically. Free disk space is checked at publication and must cover the image plus a 64 MiB reserve; queue capacity is checked at admission. A cache surviving restart does not restore lost workspaces, queue subscriptions or favourite state. Recovery reconstructs recipes, and explicit rendering can reuse a matching surviving cache entry.

## Security identity versus artistic identity

Workspace, CSRF, exploration, action and sample identifiers are cryptographic capabilities or navigation tokens generated from 24 random bytes. They are unrelated to deterministic PCG artwork streams. New web explorations choose seed entropy cryptographically, then record concrete seeds so rendering is deterministic. Session cookies use `HttpOnly`, `SameSite=Lax`, path `/`, and `Secure` with `__Host-art-studio` on HTTPS; local HTTP uses `art-studio`.

Artifact URLs intentionally require no session and expose an image while its key is known and cached. Recovery records contain recipes, not session/CSRF secrets. Copying a recipe does not transfer a workspace.

## Local command records

CLI images embed their invocation/traits and software revision, with PNG/JPEG metadata in [render/meta.go](../../internal/render/meta.go). Flock JSONL records store seed, trait set, image filename, mode and parent. Sweep manifests map tile numbers to filenames and varied values; flock manifests include seeds, filenames, modes and traits. Full CLI commands are embedded in image metadata. These are local workflow formats, distinct from the public edition envelope.

Source: [recipes](../../internal/artwork/recipe.go), [publication](../../internal/publish/catalog.go), [studio state](../../internal/studio/studio.go), [queue/cache](../../internal/renderjob/manager.go), [CLI flock](../../cmd/staticart/flock.go).
