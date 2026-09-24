# Data, identities and persistence

The production studio uses PostgreSQL for state and River jobs, and a private S3 bucket for PNGs. [Persistence operations](../operations/persistence.md) gives bootstrap and lifecycle details; [implementation evidence](../plans/postgresql/implementation.md) records the reviewed changes from the original proposal.

## Domain vocabulary

| Term | Meaning and owner |
| --- | --- |
| Sketch | Local artwork algorithm implementing `sketch.Sketch` |
| Trait schema / set | Weighted discrete output dimensions / one resolved choice per dimension |
| Public entry | Explicitly admitted artwork, styles and colours in `publish` |
| Recipe | Canonical, validated complete artistic choices for one edition |
| Rendition | Pixel dimensions, AA and encoding for a recipe |
| Workspace | Durable, idle-expiring capability-owned studio navigation |
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
| Workspaces | PostgreSQL bounded aggregate | At most 1000; 90-day sliding idle lifetime; survive service restart |
| Explorations | Workspace aggregate | At most four; independently revisioned; no separate expiry cascading favourites away |
| Batches/action replay | Workspace aggregate | Latest eight retained; samples retained for batches, parents and favourites |
| Favourites | Aggregate + SQL pins | At most 24; recipe and preview retained with unexpired identity |
| Workspace payloads | PostgreSQL bytea | At most 2 MiB each and 64 MiB globally; byte-exact recipes in render requests |
| Queue | PostgreSQL / River | Nine outstanding render jobs globally; four active interests per owner; one local worker per renderer |
| Job subscribers | SQL interests | Up to 24 per coalesced rendition |
| River history | PostgreSQL | Completed/cancelled 24 hours, discarded seven days; application references survive history pruning |
| PNGs | Private bucket | 2 GiB / 5000 accounted objects including pending/deleting; ordinary age 24 hours, live favourite pins exempt |
| Open image readers | Web buffers | Four bounded 16 MiB reads, at most 64 MiB encoded buffers per process |
| Browser cookie | Browser cookie jar | Opaque token, 90-day age refreshed on meaningful visits; fragments/SSE/images do not refresh |
| Recovery JSON / image downloads | User filesystem | User-retained; not synchronized or automatically backed up by the app |

Never-started work expires after one minute; already-started retries have a five-minute lifetime. River handles ordering, retries and maintenance election; the previous alternating-owner scheduler is replaced by per-owner quotas and a global outstanding budget. Deterministic render failures require explicit retry. Abandoned/infrastructure failures can retry within three attempts. No status read creates work.

Publication validates PNG content/dimensions, records an upload intent, writes an immutable bucket key, then atomically commits its pointer/outcome and River completion. External upload and SQL cannot be one transaction; old intents and deletion tombstones enable bounded reconciliation. Pins prevent favourite previews from normal expiry; storage saturation rejects new publication rather than evicting favourites. Database volume loss loses metadata even if image objects survive; database backups are outside this task.

## Implemented PostgreSQL schema

This is the current [application schema](../../internal/persistence/schema.sql), not the more normalized illustrative schema in the design proposal. River owns its own migration, job, queue, client and leadership tables.

| Table | Stored state and relationships |
| --- | --- |
| `art_schema` | Ordered application migration versions and checksums; separate from River migration history |
| `art_control` | Singleton admission flag, active build/epoch, bound bucket location and listing cursor; serializes application admission/publication |
| `art_workspaces` | SHA-256 cookie lookup ID, CSRF, touch/expiry timestamps and bounded JSON-encoded aggregate stored as `bytea` |
| `art_requests` | Canonical recipe bytes, rendition/build, current River job ID, publication generation/epoch and durable outcome/timestamps |
| `art_interests` | Unique workspace/request pair; deletion cascades when either owner/request is removed |
| `art_artifacts` | One current immutable object key, digest, size and publication/expiry timestamps per request |
| `art_pins` | Workspace/sample-to-request references preventing ordinary expiry of favourite previews |
| `art_uploads` | Immutable key, request/generation, bytes, pending/published/deleting state, deletion eligibility and first successful deletion timestamp; also serves as deletion outbox/tombstone |
| `art_rate_buckets` | Hashed expensive-action quota keys, tokens and last update |
| `art_renderers` | Boot ID, build, heartbeat time and bucket-health result for readiness |
| `art_instances` | Migration 2: web/renderer boot ID, role, Docker hostname/optional name, build, heartbeat/stopped timestamps and storage probe |

The workspace aggregate contains navigation, actions/replay, sample recipes and favourite flags. SQL pins/interests are maintained in the same transaction; no separate exploration/favourite table exists. `art_requests.job_id` deliberately has no foreign key to River history, so its cleaner cannot remove product state or be blocked by it. Object upload/deletion records are likewise retained independently for cleanup.

New River execution metadata stores original producer boot/name/hostname/build; coalescing another interest does not overwrite it. `attempted_by` records River claimant IDs, now identical to renderer presence IDs. Instance records persist eight days and mark clean shutdown, enabling live/stale/stopped monitoring without confusing restarted processes. See [terminal monitoring](../operations/monitoring.md).

Normal image expiry removes the database pointer first and schedules physical deletion one hour later. Pending uploads older than one hour without a published pointer become deletable. Tombstones are retried hourly and removed only after seven days since first successful deletion and eight days since creation. Listing reconciliation skips objects younger than one hour, visits at most 100 per page, and requires the explicitly bound bucket location. [Maintenance](../../internal/persistence/maintenance.go) also reaps expired identities and unreferenced requests. These are application accounting limits, not an instantaneous provider-enforced bucket quota.

## Security identity versus artistic identity

Workspace, CSRF, exploration, action and sample identifiers are cryptographic capabilities or navigation tokens generated from 24 random bytes. They are unrelated to deterministic PCG artwork streams. New web explorations choose seed entropy cryptographically, then record concrete seeds so rendering is deterministic. Session cookies use `HttpOnly`, `SameSite=Lax`, path `/`, and `Secure` with `__Host-art-studio` on HTTPS; local HTTP uses `art-studio`.

Artifact URLs intentionally require no session and expose an image while its key is known and cached. Recovery records contain recipes, not session/CSRF secrets. Copying a recipe does not transfer a workspace.

## Local command records

CLI images embed their invocation/traits and software revision, with PNG/JPEG metadata in [render/meta.go](../../internal/render/meta.go). Flock JSONL records store seed, trait set, image filename, mode and parent. Sweep manifests map tile numbers to filenames and varied values; flock manifests include seeds, filenames, modes and traits. Full CLI commands are embedded in image metadata. These are local workflow formats, distinct from the public edition envelope.

Source: [recipes](../../internal/artwork/recipe.go), [publication](../../internal/publish/catalog.go), [persistent studio](../../internal/studio/persistent.go), [schema](../../internal/persistence/schema.sql), [worker](../../internal/persistence/worker.go), [CLI flock](../../cmd/staticart/flock.go).
