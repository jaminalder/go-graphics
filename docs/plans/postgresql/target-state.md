# PostgreSQL target state

**Approved design baseline; implementation is recorded in [implementation.md](implementation.md).** Illustrative normalized schema and provisional settings below are planning history where superseded by that record. Current behavior is in [data](../../reference/data.md) and [operations](../../operations/persistence.md).

## Why persistence

Before this change, the [studio store](../../../internal/studio/studio.go) and [render manager](../../../internal/renderjob/manager.go) kept navigation, queues and cache indexes in process memory. Web restart lost admitted work/navigation, and sharing a cache directory did not share eviction bookkeeping. Those types remain as domain/test support; production now uses the persistent adapter and River.

PostgreSQL becomes the source of truth for navigation, durable River jobs and artifact pointers. Its transactions replace cross-component in-memory critical sections. Use River OSS rather than implement a second queue engine. Pinned River/pgx/S3 client dependencies explicitly extend the current standard-library-only policy; the artwork CLI remains independent of these runtime services.

## Runtime ownership

**Agreed:** web produces jobs, renderer consumes jobs and publishes finished results, and communication between them is exclusively through PostgreSQL. River handles its own maintenance election inside renderer clients. There is no additional coordinator container and no render loop in web.

Proposed single-host deployment (boxes are processes/services or storage, arrows are communication):

```text
Browser --HTTPS--> Caddy --HTTP--> artweb
                                    |
                                    | SQL: studio + River enqueue; LISTEN results
                                    v
                            PostgreSQL + durable volume
                                    ^
                                    | River worker + maintenance; result commits
                                 artrender
                                    |
                                    | exec + bounded stdin/stdout
                              disposable render child

artrender --HTTPS PUT/LIST/DELETE--> private image bucket
artweb    --HTTPS GET-------------> private image bucket
artweb    --SSE (proposed)---------> browser
```

- **artweb** owns public HTTP, sessions, validation, studio use cases, transactional admission and result presentation. Its River client is insert-only (not started as a worker), so it neither renders nor participates in maintenance election. One dedicated PostgreSQL listener per web process fans result changes out locally; notifications are not mandatory business-processing callbacks.
- **artrender** runs a River worker client and the bounded render supervisor in one container. It claims work, invokes its fixed executable's child mode, validates PNGs, uploads them and commits artifact pointers with job completion. It has PostgreSQL/bucket access but no public port or Docker socket. River chooses one renderer client for internal maintenance; every renderer can execute jobs independently of that leader.
- **Render children** take validated recipes over stdin and return bounded PNGs over stdout. Preserve subprocess kill/reap behavior, 15/30-second render deadlines and non-root/container resource limits. Initially allow one child at a time. Children share the container network namespace; this is accepted. Sanitize environment and inherited descriptors; credential file mounts and the shared namespace mean this is not a security sandbox against arbitrary hostile child code. Keep the fixed trusted executable and no user-supplied code.
- **PostgreSQL** owns durable studio records, River tables, application interests/controls and artifact pointers. Constraints and transactions enforce integrity. `LISTEN`/`NOTIFY` signals committed changes. Time passing does not fire a trigger: River's in-process maintenance handles retry scheduling and abandoned-job rescue. No PostgreSQL scheduler extension or custom leader election is needed. No PNG bytes are stored in database rows or large objects.
- **Image bucket** owns PNG bytes under immutable, application-generated object keys. It is private; browsers use the existing web image/download routes, not arbitrary bucket URLs. Object storage is separate from the VPS in production.
- **Local CLI artwork commands** remain database-independent. The web studio's supported runtime becomes database-backed; an in-memory production fallback would mask persistence failures and is not proposed.

Keep `internal/studio` responsible for domain behavior and `internal/renderjob` for recipe/result validation, the River render handler and bounded child supervision. Introduce focused SQL persistence and S3 adapters; exact package boundaries follow real callers. Use River's `InsertTx` in the **same pgx transaction** as a studio command, and its transactional completion API alongside artifact publication. Remove the web-to-renderer Unix-socket transport once the new runtime is wired; the stdin/stdout child protocol remains useful. Do not reproduce the old nested `Store`/`Manager` ownership or build a universal repository/queue abstraction.

## Studio behavior and HTTP contracts

Preserve the existing anonymous capability-cookie model, canonical Host/Origin checks, CSRF, publication validation and status codes. A restart is no longer an expiry event.

- Legacy expiry was 30 minutes idle / 24 hours absolute with a one-day cookie. The accepted replacement is 90-day idle expiry and renewal only on meaningful visits, not background result/image traffic. Expired identities are not revived; GETs never create identities/jobs.
- Persist navigation IDs, exploration revisions, generation round, concrete recipes, ordered batches, parent links, favourites, cancellation flags and action replay results. Persist concrete random choices; retries must not silently generate another batch.
- Store a hash of the high-entropy workspace cookie for lookup, with a separate internal workspace ID for joins. Do not log cookies/CSRF or use raw cookies as database ownership identifiers. CSRF must remain recoverable for rendering forms; retain its value in protected session data and compare in constant time.
- Keep exact replay within the current bounded history: identical action ID/payload returns its recorded result, and different payload conflicts. Bound replay history to eight entry records per workspace and eight generation records per exploration, as today. Check retained replay **before** stale-revision checks.
- Replay is scoped to an established workspace. If the first response that sets a new cookie is lost, the browser may not have that capability; do not claim cross-session exactly-once entry. A retry can create another bounded, expiring workspace. Restores into fresh sessions have the same bootstrap limitation.
- Use a workspace row lock for mutations so sibling explorations cannot race the workspace-wide favourite/interest limits. Use revisions to reject stale forms. Lock multiple job rows in stable key order.
- Keep finite admission/storage bounds, with current four-exploration, eight-batch and 24-favourite limits as the baseline. Longer-lived anonymous identities require reviewing the 1000-workspace and 32 MiB global recipe budgets so inactive visitors do not exhaust live capacity; do not silently evict retained favourites to accept new visitors. Final identity/favourite capacity policy depends on the retention decision below.
- Session lookup failure because PostgreSQL is down is **503**, not an expired session or permission failure. Never clear/replace a cookie solely because storage is unavailable. Audit currently ignored store errors in HTTP handlers, including favourites, clear and image serving.
- Preserve unauthenticated artifact reads by rendition key, PNG ETags and missing-artifact 410 behavior. A database outage is 503, not 410. Static gallery/assets can still render without a database query.
- Preserve explicit export/restore and clear semantics. Restore validates the whole bounded input before committing. Clear atomically removes workspace-owned state/interests without deleting a coalesced job's other subscribers.

## Returning without a login: accepted decision

The capability remains a random 24-byte token encoded as 48 hexadecimal characters. HTTPS uses `__Host-art-studio` with `Secure`, `HttpOnly`, `SameSite=Lax` and path `/`; no login/account. The mechanism supports returning on the same browser profile while cookie and stored identity survive. The old one-day cookie/short workspace expiry has been replaced.

**Accepted:** anonymous visitor identity/cookie expires after 90 days of inactivity, refreshed on meaningful visits, not background result/image traffic. Retain bounded favourite recipes/previews. Implementation keeps a bounded durable workspace aggregate rather than separately expiring exploration rows, so pruning batch navigation cannot cascade away favourites. Unfavourited images and downloads retain short cache lifetimes.

The 2 GiB/5000 budget includes favourite pins and pending/deleting uploads. A full budget rejects new publication instead of deleting retained favourites. The 24-hour age is only for unpinned disposable images. See the implementation record for the 1000-identity and aggregate payload bounds.

There is no cross-device discovery or recovery from a deleted cookie without an additional mechanism. Anyone possessing the cookie has access to that anonymous workspace; no personal identity is established. Keep the existing recipe export/import as an explicit user-controlled transfer path. Do not add fingerprinting, accounts, email flows or recovery-link features to this scope.

## Logical schema

The table below is the original **illustrative normalized design**, not the implemented SQL. The implementation instead uses a bounded workspace aggregate and `art_uploads` as both upload ledger and deletion outbox. Use the [actual schema reference](../../reference/data.md#implemented-postgresql-schema); do not create the extra planned tables merely to match this historical sketch.

| Record | Essential fields and invariants |
| --- | --- |
| Application migration records + River migration records | Version both schemas explicitly; use River's supported migrator for its tables, not handwritten edits to River migrations |
| River-owned tables (`river_job`, leadership/queue/migration metadata as required by pinned version) | Execution state, attempts, scheduling, claims, cancellation, retry/rescue, retention and internal maintenance ownership; access through supported River APIs |
| `runtime_control` | Admission flag, active renderer build/queue and monotonically increasing deployment epoch; River queue pause/resume controls dispatch; release activation updates these explicitly, never on web startup |
| `capacity` | Bounded admission and artifact reservations; serialize admissions to enforce outstanding-job/recipe/storage budgets; do not mirror River's running-slot accounting |
| `workspaces` | Internal ID, unique token hash, CSRF, creation/touch/absolute expiry; index expiry |
| `explorations` | Workspace FK, artwork/choices, revision, round; workspace-scoped ownership |
| `samples` | Exploration FK, stable ID, style/colour, canonical recipe bytes/digest, favourite/cancel flags, rendition references |
| `batches`, `batch_samples` | Ordered retained batches/samples and similarity parent; do not delete a referenced parent or favourite when pruning old batches |
| `actions` | Workspace/exploration scope, action ID, request digest, result ID and order; uniqueness in its scope; bounded replay retention |
| `render_requests` | Stable rendition identity, exact canonical recipe, tier/build, current River job ID and execution generation, admission/first-start deadlines and durable user-visible outcome; no independent queue scheduler or renewable lease engine |
| `job_interests` | Workspace-to-render-request subscription; unique pair, at most 24 owners per coalesced request; cap active interests at four per workspace |
| Renderer health record, only if River exposes insufficient health/build information | Bounded boot/build/last-seen data for readiness; never a second leader election or job claim authority |
| `artifacts` | Rendition key, storage location identifier, immutable object key (and provider version ID if used), SHA-256/ETag, size, media type, publication/expiry timestamps and lifecycle state; no image bytes or expiring signed URLs |
| `artifact_uploads` | Unique execution-specific object key, River job/attempt identity, application generation/epoch, expected digest/size, reservation, pending/published/deleting state and deadline; intent before upload allows crash cleanup |
| `object_deletions` | Durable deletion tasks for exact object keys, retry state and not-before time; metadata tombstone and deletion intent committed together |
| `rate_buckets` | Bounded expiring buckets for starts and generation, keyed by workspace or client identity; atomic token consumption |

Canonical recipe identity must be preserved **byte for byte**. Store canonical bytes as `bytea` (or an equivalently exact representation), not a round-trip through JSONB that can reorder keys. River's JSON args contain a render-request ID and required build, not the hash source. Derive rendition keys using the existing recipe + rendition + renderer-build contract. Seeds remain decimal strings in exported recipes.

An explicit retry retains the rendition's artistic identity but creates a new supported execution generation/job as appropriate; previous attempts cannot publish into it. A sample's artistic identity remains independent of the build-specific execution key. After a release, an explicit download uses the current build when the old artifact is unavailable; it must not dispatch an old-build job to a new renderer. Application records/artifacts outlive River's prunable job history; avoid foreign keys that make its cleaner fail or cascade away product data.

## Transaction and concurrency boundaries

### Admit a four-image batch

One short transaction must:

1. Lock admission/capacity state, then workspace, then affected jobs in stable order; all mutation/cleanup paths follow the same ordering when taking these locks.
2. Verify session expiry, retained replay, revision, choices, existing active batch and global/workspace/subscriber capacities.
3. Resolve and persist concrete recipes for this action; reuse ready artifacts or coalesce eligible requests by rendition key. Call River `InsertTx` for each new execution within this transaction.
4. Write the complete batch, samples, interests, revision/round and replay result; reserve capacity for all new work, or roll everything back.
5. Commit before redirecting. An ambiguous commit/HTTP disconnect is retried using the action ID, not by creating new work blindly.

Rendering, PNG encoding/validation and external calls happen outside this transaction. Generation planning is bounded; if moved before the lock, recheck revision and replay before committing its candidate recipes. An internal transaction retry reuses prepared entropy/IDs and has a small bounded retry budget.

Admission and state-changing maintenance serialize briefly on the capacity row at this scale. This is deliberately simpler than distributed counters. Read-only snapshots and session touches do not acquire that global lock. Measure contention before replacing it.

### River claims, capacity and fairness

River owns claiming, execution attempts, scheduling and rescue. Do not add a custom `SKIP LOCKED` consumer, lease-renewal loop or independent rescuer beside it. Use a build-specific queue, for example a bounded name derived from the build hash, and validate the full build/epoch in the handler before launching a child. A renderer consumes only its supported build queue. Maintenance can observe all River queues without executing another build's artwork.

**Simplification proposed for this revision:** cap outstanding render jobs (available + running + scheduled/retryable) at nine globally, with initially `MaxWorkers=1` for the render queue in the one production renderer. Cap each workspace's active interests at four as before. Admission serializes through application capacity control and checks authoritative River state for active linked executions; admission must not depend on a local completion callback releasing a counter. Automatic retries retain an outstanding slot; explicit retries must recheck admission. Queries must treat missing history through retained application outcomes rather than assume missing means successful.

This replaces the old exact eight-waiting/one-running split and alternating-owner scheduler with a small total-work budget and River's ordering. Do not claim River OSS provides that custom fairness or a cluster-global execution limit. `MaxWorkers` is local: two renderer containers with one slot each can render two images. Test anti-monopolization via workspace quotas, bound waits, and document this change. If exact owner alternation is required, bring that requirement back for review rather than fork River silently.

### Attempts, failure and completion

Preserve 15/30-second child deadlines. Proposed River settings for validation: 90-second total job timeout (including upload/SQL completion), three maximum attempts, stuck-job rescue eligibility after two minutes (strictly beyond total timeout), 30-second fallback fetch interval, and short bounded retry backoff for classified infrastructure failures. These replace the old custom 60-second lease/10-second renewal plan. Rescue scan cadence adds latency beyond eligibility; measure the actual pinned-version bound rather than promise recovery exactly at two minutes. One-minute never-started expiry and a five-minute total retry lifetime remain proposed application policies; verify they are compatible with rescue cadence and record an expiry outcome without rendering stale work.

- Use River's retry/cancel APIs for abandoned and explicitly classified transient failures. Invalid recipe, deterministic render failure and output-limit violation are terminal for that execution; do not inherit the library's default attempt count blindly. Bound SDK retries within the same job timeout, not a second unbounded retry policy.
- River execution is at least once. On each handler entry, establish an application publication generation/unique attempt token tied to the River job/attempt and current deployment epoch. This guards our external result side effect; it is not a second renewable job lease. Once a new attempt starts, old attempts cannot publish. Clear/cancel/build switch also invalidates publication authority.
- Validate PNG size, dimensions, build and digest; reserve upload intent before network upload. Final publication must verify the execution generation, active epoch, remaining interests and River execution eligibility, then atomically commit artifact pointer, durable request outcome, River `JobCompleteTx` and a result notification.
- **Early verification gate:** test the pinned River version's transactional completion, rescue and cancellation behavior. We must reject stale completion after rescue/cancel, not infer that from a state name or marketing guarantee. Identify a supported atomic check/locking strategy during WP1/WP2, follow compatible lock ordering, and bring any need for unsupported River internals or a fork back for review. Do not implement a competing queue state machine to hide a mismatch.
- Cancellation removes an interest. If others remain, keep the job. Last-interest cancellation invalidates publication authority and requests River cancellation through the supported transactional API. Child cancellation follows its job context; publication checks protect against a lost cancellation notification. Persist durable terminal outcomes before relevant River history is pruned, using an idempotent application reconciliation task where library failure paths do not permit our own transaction.
- Graceful shutdown stops new claims and finishes bounded work or cancels children; River can rescue interrupted work. Web shutdown leaves committed rendering jobs alone. A PostgreSQL outage prevents publication; do not assume a custom lease-heartbeat mechanism exists in River or that lost connectivity immediately cancels CPU work. Local deadlines bound it.

### Notifications and maintenance

- River supplies new-job wake-ups through PostgreSQL `LISTEN`/`NOTIFY`. Keep a fallback fetch interval (proposed 30 seconds) and library maintenance; this is notification-driven, not polling-free. Research found a one-second default fetch fallback and a five-second scheduler in current River documentation/source. Verify the pinned release, total idle queries and available supported configuration before fixing budgets.
- River elects one started renderer client for its internal scheduler/rescuer/cleaner. No custom election table, leader process or extra container. With one renderer it is the maintenance leader; after leader loss another running client takes over. If all renderers stop, durable jobs wait until a renderer starts and resumes maintenance.
- PostgreSQL does not emit notifications merely because a timestamp passed. Do not add `pg_cron` or hand-built database timers; River handles queue time-based transitions. Domain cleanup (expired identities, upload intents, object deletions) uses bounded idempotent maintenance jobs in a separate low-concurrency River queue, with durable eligibility records and database locking. River's periodic enqueueing/uniqueness may reduce duplication, but is not an exactly-once cleanup guarantee. A bounded startup sweep catches missed maintenance periods. Pausing the render queue must not pause this maintenance queue.
- Result notifications are application-owned, emitted in the same SQL transaction as the outcome. River `Client.Subscribe` is local to jobs worked by that client, not cross-container delivery. Use small IDs/revisions, no cookies or recipes; notifications are visible to database listeners and are not an authorization boundary.
- Each web process has one dedicated listener connection: commit `LISTEN`, then read current state; repeat on reconnect. It fans out to local browser subscribers. Notifications can coalesce or be missed while disconnected, so durable snapshots and a bounded reconciliation of actively awaited results are mandatory. Idle pages need no per-browser database polling.
- **Browser recommendation for final review:** SSE for result updates, snapshot resync on connect/reconnect and ordinary HTML/manual refresh fallback. Authorize every subscription against its cookie/session; stream IDs do not grant access. Recheck expiry, bound stream counts/buffers and close slow clients. Heartbeat writes need not query PostgreSQL or extend identity lifetime. A race-safe local subscribe-then-snapshot sequence handles completion during attachment; do not build a durable event replay log just to show latest state.
- Web receiving a notification only updates presentation. A missed event cannot leave a successfully rendered job requiring an essential web-side mutation to become usable.

## Artifact and retention policy

PNG bytes live only in a private S3-compatible bucket. PostgreSQL stores stable location/object identifiers and metadata; never store credentials, public URLs or expiring signed URLs in artifact records. The old web cache volume is not authoritative or imported at cutover.

### Upload then publish a pointer

There is no distributed transaction between PostgreSQL and S3. The invariant is: **only publish a ready pointer after a successful complete upload; tolerate and reclaim unreferenced objects.**

1. Validate the complete PNG and compute its SHA-256 locally. Create a durable upload intent with a unique key such as `artifacts/<build>/<rendition-key>/<attempt-id>.png`; never overwrite another attempt's key. Reserve bytes/count before sending data.
2. Upload outside the SQL transaction using the River job context and finite request/retry deadlines. Use single-object PUT for the bounded 16 MiB images. Confirm the provider's complete-object/read-after-write semantics and verify length/checksum metadata; do not treat an S3 ETag as the PNG SHA-256.
3. Commit pointer + durable outcome + upload-intent transition + River completion + result notification in one fenced SQL transaction. The intent must still be pending and owned by the current eligible execution/generation/epoch. A timeout with ambiguous commit is resolved by querying the intent/result, not by deleting the object blindly.
4. If upload succeeded but pointer commit fails, retain intent for reconciliation. An upload timeout may also have succeeded remotely; retry only the same intent/key under the same eligible execution, or abandon it for cleanup. Bound transient storage retries within the River job/attempt budget.
5. Maintenance locks the same intent before marking it deleting. Publication rejects deleting intents. Do not delete an object associated with a current eligible execution; allow the bounded upload timeout/grace period before cleanup. A late PUT may still finish after deletion: retain tombstones and periodically rescan old keys so it is found and deleted again.

A stale worker can upload bytes, but cannot overwrite a winner's object or publish a ready pointer. Detect out-of-band object loss during reads/reconciliation; no design can transactionally prevent an external administrator from deleting a referenced object.

### Reads and missing objects

Keep `/images/<key>` and `/downloads/<key>` unchanged. Web resolves an unexpired ready pointer, releases its SQL connection and retrieves the object over verified HTTPS. Serve the application SHA-256 ETag, content type, cache policy and download filename. Do not expose bucket credentials or turn persisted keys into arbitrary URLs/SSRF targets.

Initially buffer and verify each bounded object before sending headers, so truncated/corrupt reads produce an error rather than a partial successful PNG. Bound aggregate buffered bytes as well as request count; 64 × 16 MiB would exceed the current web memory limit. Proposed starting buffer budget is 64 MiB per web process, to be verified under its 384 MiB cap. Preserve request/per-artifact concurrency bounds where they fit that budget. No database transaction stays open for a browser download. Bytes already fetched survive later eviction.

Bucket timeout, access denial or service failure yields bounded 503 and an operational signal, not false expiry. A confirmed absent object or expired pointer yields 410 and schedules metadata reconciliation; a corrupt object yields 503 and quarantine/inspection. GET never recreates it. An explicit later action can request rendering using the supported build after the missing artifact has been invalidated.

### Eviction, reservations and reconciliation

Keep the logical 2 GiB / 5000 ready artifact / 24-hour limits. Expire/evict database visibility first and enqueue durable deletion tasks in the same transaction. Physical deletion is asynchronous and idempotent; confirm deletion before releasing physical-byte accounting. Ready-artifact eviction must not destroy favourite recipes or another subscriber's job state.

Track pending uploads and deletion backlog separately from ready artifacts. Bound in-flight reservations (initially 16 MiB per configured execution slot) and stop fresh publication/admission under sustained cleanup failure or exhausted storage budget. Reconcile provider listings against intents, pointers and tombstones in bounded pages. Respect active uploads and a configurable orphan grace period, initially one hour; listing alone never decides that a recently uploaded key is safe to delete. Retry deletes after transient failures. Logical limits cannot be claimed as an instantaneous physical bucket quota because late uploads and provider accounting may lag; report actual storage/backlog and enforce a separate operational ceiling.

River job history is operational, not the product record. Proposed retention: 24 hours for completed/cancelled jobs and seven days for discarded jobs, verified/configured on the pinned version; retain application outcomes until no user relationship needs them. Object references, upload intents and deletion records remain long enough to finish/reconcile their lifecycles. A provider lifecycle rule may be a delayed safety net, but must not preempt live artifact/favourite retention. Confirm noncurrent-version behavior so deleted versions cannot silently accumulate.

PostgreSQL still needs WAL/disk headroom for metadata churn, but no image traffic enters its WAL. See [operations](operations.md) for bucket ownership, lifecycle, monitoring and runtime reconciliation.

## Rate limits and operational controls

- Move new-session and generation token buckets to shared transactional storage so adding web replicas does not multiply expensive-work allowance. Keep the current rates, bursts, bounded key count and idle pruning. Bound client-identity retention; exclude it from logs.
- Read/asset rate limits and HTTP concurrency stay per web process initially; document the aggregate multiplier in the two-instance rehearsal. An edge/global read limiter is future production-replication work.
- Admission pause is durable and global. Distinguish it from dispatch pause: normal draining stops new admissions while workers finish existing jobs. Database restart or web restart must not unexpectedly re-enable generation.
- Expose bounded counters for River queue states/age, retries/rescues, terminal failures, renderer health, maintenance leadership, notification reconnects, idle SQL activity, artifact usage, upload/deletion backlog, object errors/latency, pool saturation and database latency. Keep labels free of tokens and unbounded job IDs.
- Liveness tests the process. Serving readiness tests database connectivity/schema compatibility; generation readiness additionally checks admission policy, a fresh matching worker/renderer and recent bucket-access health. Report object-read health separately so a bucket outage need not remove otherwise usable studio navigation from routing. Probe object access at a bounded interval; do not create images on health requests. Generation-off is an intentional mode, not a reason to restart web. All probes are private; static browsing may continue during a database outage.

## Scope boundary

Production remains one active application build and one renderer container with one render slot initially. Multi-web/two-renderer tests demonstrate shared state and River failover, but do not authorize mixed-build rolling deployment or claim database/host HA. Compatibility and the initial session reset are specified in [operations](operations.md). Database backup/restore work is explicitly excluded by owner instruction.
