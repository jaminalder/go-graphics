# PostgreSQL implementation work packages

**Status: approved local execution; evidence and per-package outcome are in [implementation.md](implementation.md).** The following is the original acceptance checklist, not a claim that live-provider/production exercises were performed. Work is directly on `master`, one writer at a time. See [target](target-state.md), [operations](operations.md) and [review](review.md).

## Sequence and integration strategy

The agreed topology is web (River insert-only client), PostgreSQL and renderer (River worker client plus child supervisor/uploader). River owns queue mechanics and maintenance election. No separate coordinator container, custom lease scheduler or direct web-to-renderer connection is planned. Final retention, queue-policy and browser-delivery recommendations were accepted in the [overview](README.md#final-recommendations-accepted-for-implementation).

```text
WP1 database, River and bucket foundations / compatibility proof
  -> WP2 application schema and transactional River admission
    -> WP3 River renderer and artifact publication
      -> WP4 persistent studio and HTTP integration
        -> WP5 runtime, controls and Compose integration
          -> WP6 release/upgrade/cutover integration
            -> WP7 restart, reconciliation and failure evidence
              -> WP8 final documentation and release readiness
```

Tests and documentation belong to each package, not only WP8. New primitives/commands may land before production wiring changes, but do not introduce a production mode that persists sessions while keeping their admitted jobs in memory. The old runtime stays deployable until the complete transactional path and deployment integration are ready. Buildability on trunk does not imply permission to deploy intermediate commits. No dual-write migration or indefinite pair of production backends is planned.

The later production rollout is an explicit operator action after WP8, not an implicit last implementation step.

## WP1 — Database, River and bucket foundations

**Dependencies:** final owner review. **Deliverable:** reproducible local PostgreSQL/S3 services, pinned clients/migrations and an early River integration proof, without changing the running studio backend.

- Select and pin the supported PostgreSQL 17 image; verify architecture, entrypoint/user, volume layout, writable paths and health check.
- Add data Compose/bootstrap tooling with unique disposable project support and stable production identity; no public database port.
- Pin compatible River OSS, `riverpgxv5`, `pgx/v5` and AWS SDK for Go v2 S3 dependencies. Verify Go/PostgreSQL support, migration compatibility and runtime permissions. Keep CLI artwork use independent of database/bucket startup.
- Prove `InsertTx` rollback, supported transactional completion with application updates, abandoned-job rescue and stale-attempt/cancel races against real PostgreSQL. Identify the supported atomic publication guard before committing to the application schema; bring unsupported-internals/fork requirements back for review.
- Verify River insert-only web configuration, worker configuration, build-specific queue routing, local `MaxWorkers`, pause/resume, leader handover and graceful shutdown. Configure proposed timeout/retry/rescue/fetch defaults from the target; measure idle SQL and actual rescue latency, including scheduler cadence. Do not use default 25 attempts/one-hour rescue without review.
- Verify managed Hetzner Object Storage suitability and provisioning support; propose exact private-bucket/policy/lifecycle definitions in a separate storage lifecycle. Verify credentials stay out of Terraform state. Select/pin a maintained local S3-compatible service and bootstrap unique disposable buckets for tests.
- Add a narrow object adapter (PUT, GET/metadata, LIST, DELETE), immutable-key validation, verified TLS/CA support, request deadlines and bounded retries. Test against both local S3 and an explicitly disposable provider bucket before production approval; local emulation alone is insufficient.
- Add `artdb`, checksummed application migrations and River's supported migrator in explicit dependency order. Establish compatibility metadata for both schemas; inspect nontransactional upstream migrations rather than assume they share application rollback semantics.
- Add ignored development secret generation and examples; never ship real credentials in release files.
- Provide one command to start/test/clean disposable PostgreSQL and object-storage instances/buckets. Add integration support to CI; tests must fail clearly rather than silently skip when the dedicated persistence job is missing a required service.

**Touchpoints:** `go.mod`/new `go.sum`, persistence/object-store/admin code, `deploy/compose.data.yaml`, separate bucket provisioning definitions, bootstrap/test tooling, CI and local getting-started instructions.

**Acceptance:** fresh initialization, reconnect, retained-volume recreation, interrupted/repeated/concurrent migrations of both schemas, checksum mismatch and incompatible-schema rejection. River transactional enqueue/completion, rescue, cancellation and leader handover have recorded evidence. Runtime roles cannot run DDL; disable optional River reindex maintenance if incompatible with that requirement. Verify web cannot upload/delete objects, TLS failures, independent bucket/VPS lifecycle and safe test cleanup. `staticart` still runs without either service. Record idle queries and connections; any publication-fencing/API mismatch blocks dependent packages.

## WP2 — Schema, budgets and transaction primitives

**Dependencies:** WP1. **Deliverable:** constrained storage and concrete operations supporting one transaction per studio command.

**Resolved product prerequisite:** 90-day idle identities and retained favourite previews. Implementation uses a bounded aggregate that preserves favourite samples during navigation pruning, plus SQL image pins; see the implementation record for adjusted schema/capacity details.

- Implement application records: studio state, exact recipe bytes, `render_requests`, interests, artifact pointers, upload intents and deletion tasks. River tables own execution state/attempts; do not recreate them in a parallel `render_jobs` scheduler. Product records survive River history pruning.
- Establish transaction ownership across studio updates and River `InsertTx`; propagate contexts and explicit storage-unavailable errors. Args point to immutable application request identity/build, not reserialized recipe hash input. Use River uniqueness where appropriate and database constraints for application rendition/action identity.
- Implement control/capacity records, stable lock ordering, revision checks, bounded action replay and global start/generation rate buckets.
- Implement total outstanding-work admission under a short application lock, using authoritative linked River states. No completion callback is required to free admission. Retain per-workspace/subscriber limits; explicit retries recheck capacity. Document the proposed total-nine cap and River ordering instead of the old eight-waiting/one-running split and owner alternation.
- Implement bounded application expiry/pruning and storage reservation reconciliation, including workspace deletion, shared interests and favourite/parent retention. Design publication generation/epoch checks and supported cancellation/completion lock ordering from WP1 evidence; do not add a renewable job lease engine.
- Specify schema-version compatibility ranges; document constraints and counter ownership next to migrations.

**Acceptance:** real concurrent transactions cannot exceed workspace/favourite/recipe/job/subscriber limits; same-action races coalesce, different payload conflicts; recipe bytes/key survive database round-trip; rollback leaves neither a partial batch nor leaked reservations. Test deadlock/serialization handling with a bounded retry budget. Confirm pruning is safe with two maintainers.

## WP3 — River renderer and artifact publication

**Dependencies:** WP2. **Deliverable:** the renderer consumes River jobs, runs bounded subprocesses and publishes results independently of web.

- Extend existing `artrender` with the River worker client/handler; preserve child stdin/stdout recipe protocol and validation. No `artworker` binary/service, HTTP render RPC or local shadow queue. Initially one local render slot; reap/kill subprocesses on deadlines, cap output and sanitize child environment/inherited descriptors.
- Let River claim, retry, schedule and rescue jobs and elect its maintenance leader. Apply build routing, first-start expiry, total retry lifetime, terminal failure classification and publication generation/epoch checks in the handler. Do not implement custom claim or renewal SQL.
- Preserve subscriber cancellation: one interest leaving does not cancel others; last-interest removal invalidates publication and invokes supported River cancellation. Persist failed/cancelled/expired outcomes idempotently before library history pruning; exercise failures outside the handler too.
- Validate PNGs, reserve an upload intent and upload to its unique key. In one fenced transaction commit the artifact pointer, request outcome, River `JobCompleteTx`, upload transition and application result notification. No network upload or rendering inside SQL transactions. Handle ambiguous PUT/commit through reconciliation, not blind deletion.
- Implement metadata expiry/eviction plus durable asynchronous object deletion, orphan grace periods, fenced intent cleanup and late-PUT reconciliation. Bound upload/deletion backlog and track logical versus actual storage separately. Preserve no-render-on-GET behavior.
- Implement bounded application cleanup/reconciliation as idempotent jobs on a separate maintenance queue with startup catch-up and database locking. Do not build another leader election; do not assume library leadership alone makes application cleanup exactly once.
- Add local renderer health, durable build/queue health only where River metadata is insufficient, and operational counters. Retain process/resource limits; shared renderer/child networking is intentional.

**Acceptance:** kill a renderer during execution and let River recover within the measured bound; stale rescued/cancelled attempts cannot publish; duplicate execution never corrupts the result. Test completion/cancellation/expiry races, queue saturation through retries, attempt exhaustion and unsupported build. Inject after-upload/before-commit failure, ambiguous commit, late PUT and deletion failures; no cleanup deletes a published winner. A real render produces a bucket object and committed pointer/outcome without web running. Confirm no child environment/descriptor leaks of parent credentials/connections, without claiming network isolation.

## WP4 — Persistent studio and public request integration

**Dependencies:** WP3. **Deliverable:** one complete database-backed studio path preserving existing user/security contracts.

- Replace the in-memory store's persistence responsibilities with transactional operations while retaining public domain behavior.
- Persist sessions/CSRF, exploration revision/round, choices, ordered batches, concrete samples, favourites and action replay.
- Make Enter/Generate/Retry/Download admission atomic with navigation; preserve exact replay ordering before revision checks.
- Implement expiry, restore/export and clear against SQL; map all database failures to intentional responses instead of swallowing errors or creating new sessions.
- Resolve SQL pointers and serve PNGs from the private bucket through existing image/download routes; preserve application SHA-256 ETags and attachment headers. Release SQL connections before object transfer; enforce aggregate buffer-byte and response concurrency budgets. Distinguish confirmed missing object (410) from bucket/auth/transport errors (503); never regenerate on GET.
- Wire shared expensive-operation rate limits. Keep read/asset limits explicitly per process.
- Configure web's River client as insert-only. Add a dedicated PostgreSQL result listener (not River's local `Subscribe`), committed `LISTEN` followed by snapshot on startup/reconnect, and bounded reconciliation only for actively awaited results. Missing notifications never prevent persistent completion.
- Subject to final review, add authorized SSE streams with local subscribe-then-snapshot, reconnect snapshots, expiry checks and bounded buffers/connections; retain ordinary HTML/manual refresh. One DB listener per web process, not per browser. Notification/heartbeat activity must not prolong abandoned identities automatically.
- Remove the old production in-memory queue/store and filesystem cache path when the database-backed runtime is selected as the supported implementation; retain narrow test fakes rather than a second runtime backend.

**Acceptance:** existing studio/web/browser journeys continue, including no-JavaScript, ownership/CSRF, conflicts, favourites, replay, cancel, export/import and retry. Web restart preserves the workspace without interrupting renderer work. Alternate requests across two web instances without affinity. Test notification before stream attachment, completion while web is down, DB-listener loss/reconnect, browser reconnect, stale cookie, slow stream and active-result reconciliation. No state-changing completion depends on notification handling. On DB failure, do not clear cookies or report false expiry; static pages remain usable.

## WP5 — Compose runtime, health and operational controls

**Dependencies:** WP4. **Deliverable:** the actual isolated Docker environment runs the new architecture.

- Build updated `artrender`, new `artdb` and CA roots into the app image; keep web/renderer services, add persistent PostgreSQL infrastructure and per-service secrets. No coordinator service.
- Remove the cross-container renderer socket/HTTP transport and old web cache mounts. Give renderer database/bucket connectivity without published ports; retain container resource restrictions and Caddy trust model. Web communicates with renderer only through database state/notifications.
- Implement serving/generation readiness, global admission/dispatch controls and updated `artctl` behavior/metrics.
- Configure resource/pool/time limits from measured evidence. Ensure database restarts and host reboots reconnect without resetting control state.
- Adapt local/browser startup, Caddy SSE streaming behavior and Compose verification. Add disposable two-web/two-renderer topology; each renderer runs its own River client/children, no shared sockets. Verify local concurrency sums across replicas rather than claiming an OSS global execution limit. Do not enable production replicas yet.

**Acceptance:** real Caddy → web → River/PostgreSQL → renderer child → bucket upload + SQL completion → notification → web/browser journey; no web-renderer connection or extra coordinator. Bucket/database/admin paths remain private. Generation pause survives restart; separate maintenance continues during render pause. Kill River's maintenance leader in the two-renderer topology and verify handover/rescue without operator action. Record parent+child memory, local/aggregate concurrency, pool/listener usage, idle SQL and read/render latency through DB/bucket interruptions.

## WP6 — Release pipeline, upgrades and cutover

**Dependencies:** WP5. **Deliverable:** repeatable schema-aware activation and rollback, preserving database state.

- Update immutable image/archive manifests, installer and validation assumptions; retain the data bundle independently from app activation.
- Preserve `/etc/art/secrets` across installer rewrites. Add idempotent data bootstrap to host provisioning without passing secret values through Terraform/cloud-init.
- Change smoke projects to provision isolated databases/local buckets/secrets/networks and verify migrations from empty and prior schemas. Archive the pinned local object-store image and retain production bucket configuration independently of app releases.
- Implement application admission pause + River render-queue drain/pause, renderer stop, application/River migration, build-specific queue/epoch switch, health and policy restoration. Version both library and schema in release metadata; never down-migrate implicitly.
- Add explicit first-cutover handling and compatible-release rollback; block legacy fallback after persistent user writes. Never delete production data on activation failure.
- Update provisioning/destroy/recreate procedures and release compatibility metadata. Keep old-build jobs from being consumed by incompatible renderers and old attempts from publishing after rollback to the same build.

**Acceptance:** failure injection before/after migration, drain timeout, wrong image/schema/build, stale worker, first-install failure, installer failure and rollback all preserve the stated data/control invariants. Repeat install/apply retains credentials and data. Smoke cleanup cannot reference production data. Restore a previous compatible app release without erasing a newly saved favourite.

## WP7 — Restart, reconciliation and failure evidence

**Dependencies:** WP6. **Deliverable:** verified restart durability and measured operating limits. Database backup planning, implementation and restore drills are explicitly excluded.

- Exercise worker kill, web restart, DB interruption, bucket outage/auth failure, orphan/late uploads, deletion backlog, queue saturation, image churn and full/unavailable storage in disposable environments, plus retained-volume container recreation.
- Exercise River leader loss, all renderers stopped/restarted, listen reconnect, missed notifications, rescue past local timeout, maintenance while rendering is paused and terminal-history pruning with retained product references.
- Verify SQL pointers, upload intents and deletion tasks reconcile after crashes without deleting a published result. Distinguish missing objects from temporary unavailability and preserve favourite recipes when a disposable rendition expires.
- Add a retained-volume host-reboot rehearsal procedure; record restart time, lost work and surviving state. Do not describe unperformed VPS exercises as passed.
- Measure database disk usage, bucket/reconciliation costs, memory, latency and cleanup lag under workload. Verify an empty replacement database does not silently authorize cleanup of an existing bucket.

**Acceptance:** sessions, favourites and queued work survive process/container restart; stored digests remain correct; a new render completes after recovery. Expired identities are not revived. The agreed returning-visitor retention policy has browser/clock-based tests. Record the accepted loss boundary if the PostgreSQL volume itself disappears.

## WP8 — Documentation reconciliation and readiness review

**Dependencies:** WP7. **Deliverable:** an implementation matching the approved contract, with current-state docs and a reviewed cutover checklist.

- Update architecture containers/components/runtime/deployment, data/configuration/HTTP references, security, running/releases/provisioning, package map and getting-started guides to match shipped behavior.
- Update `AGENTS.md` and README dependency statements after River/pgx/S3 dependencies land; keep artwork algorithms/CLI guidance accurate. Document changed queue fairness/capacity and the accepted network boundary.
- Run the standard `make check` gate, focused database/race tests, browser journeys and the final Compose/release/restart checks appropriate to the completed change. Add explicit CI jobs for persistent behavior; existing tests alone do not prove it.
- Run `make docs-check`; mark these proposal documents implemented/superseded only after comparing acceptance criteria to evidence.
- Record remaining single points of failure and defer production replication, permanent image archives and orchestration to a separate reviewed scope. Private image object storage is required in this milestone.

**Acceptance:** every required scenario below has evidence, or a clearly identified blocker that prevents production cutover. Present results and deployment instructions for operator review.

## End-to-end evidence matrix

| Claim | Required evidence |
| --- | --- |
| Restart-safe anonymous session | Same cookie, CSRF, revisions and favourites before/after web recreation |
| Durable acknowledged action | Kill web after commit/before response; replay returns one committed batch |
| Atomic admission | Inject failure halfway through a four-image request; no partial navigation/jobs |
| Shared state | Alternate one user's requests across two web processes without affinity |
| Bounded work | Admissions plus River retries/rescue respect the outstanding budget; local concurrency is explicit per renderer |
| Crash recovery | Kill renderer mid-render; River rescues it within measured bounds; stale execution cannot publish |
| Maintenance leadership | Kill elected River leader; another renderer performs maintenance without custom election code |
| Notification recovery | Commit while listeners are absent; reconnect/snapshot finds work/results without durable notification replay |
| Idle operation | Measure queries/connections with empty queues; document fallback and maintenance separately |
| Product history independence | River cleaner prunes old jobs without losing favourite recipes or completed artifact pointers |
| Correct cancellation | One subscriber leaves, others continue; last subscriber fences completion |
| Correct build identity | Wrong-build claim/completion rejected; prior images remain readable |
| Correct retention | Expiry/clear/pruning preserve shared interests and required favourite recipes |
| Safe cross-store publication | Upload succeeds but SQL commit fails; orphan is reclaimed and no ready pointer is published by a stale attempt |
| Safe cleanup | Late PUT/failed DELETE/ambiguous commit cannot delete the current published object; bounded backlog is observable |
| Object failure behavior | Confirmed missing object is 410; outage/auth/corruption is 503; no GET triggers rendering |
| Database failure behavior | No false session expiry or image 410; bounded 503/reconnect; gallery survives |
| Release durability | Compatible upgrade/rollback preserves session and database volume |
| Restart durability | Retained-volume database/container restart preserves state and resumes bounded work |
| Returning visitor | Agreed cookie/identity/favourite retention works across days; deleted cookies are not silently reidentified |
| Resource feasibility | Measured render/read/cleanup workload fits memory, connection and disk budgets |
