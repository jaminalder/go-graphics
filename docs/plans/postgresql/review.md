# PostgreSQL proposal review

**Status: pre-implementation design review, subsequently approved for local execution.** This page preserves research/rationale; see [implementation review and evidence](implementation.md) for actual results. Live provider/production work is not authorized.

## Findings from the current implementation

| Evidence | Risk in a naive PostgreSQL addition | Resolution in this plan |
| --- | --- | --- |
| [Studio Store](../../../internal/studio/studio.go) holds a mutex and calls a concrete Manager | Separate session/job commits can admit orphan work or report nonexistent jobs | One transaction owns navigation, replay and admission; WP2/WP4 |
| Enter/Generate check replay before revision; recipes derive fresh entropy | Retrying a failed response can create different images or conflict incorrectly | Persist concrete choices/replay result atomically; preserve check ordering |
| [Manager](../../../internal/renderjob/manager.go) owns subscriptions, budgets, fairness and cache refs | River does not automatically preserve product policies | Application interests/admission/publication stay explicit; proposed total-work budget and River ordering replace custom scheduling, for final review |
| Cache key includes canonical bytes and build | JSONB normalization or current-build substitution can change identity | Exact canonical bytes and build-filtered jobs |
| Web uses concrete stores and sometimes ignores errors | DB outage could become empty favourites, expired sessions or success after failed clear | Context-aware boundaries and complete HTTP error audit |
| [Renderer protocol](../../../internal/renderjob/protocol.go) executes one bounded child on a private socket | Socket/RPC and network isolation no longer match chosen service boundary | Renderer parent consumes River and uploads; remove socket transport, preserve child protocol/deadlines; shared container network accepted |
| [Activation](../../../deploy/scripts/activate-release.sh) drains one queue and rolls back by symlink | Schema change or legacy rollback could hide durable writes | Global control, compatibility metadata, migration step and explicit cutover boundary |
| [Installer](../../../deploy/scripts/install-release.sh) rewrites operator environment | DB credentials could disappear or rotate accidentally on deployment | Separate root-protected secret directory and idempotent data bootstrap |
| [Smoke](../../../deploy/scripts/smoke-release.sh) tears down its project's volumes | Adding a shared data volume carelessly could destroy production | Separate data lifetime; unique disposable smoke networks/secrets/volumes |
| [Build script](../../../deploy/scripts/build-release.sh) archives app/edge only | A new host cannot reconstruct a pinned database environment offline | Retained data bundle and updated checksum/image validation |
| [Current provisioning](../../operations/provisioning.md) treats local state as disposable | Terraform destroy now erases meaningful sessions/jobs | State the volume-loss boundary explicitly; database backup work is excluded, bucket has independent lifetime |

## Failure and concurrency review

| Scenario | Required outcome / design response |
| --- | --- |
| Response lost after commit | Existing action token returns recorded batch; no duplicate seeds/jobs |
| Concurrent favourite/generation changes in separate tabs | Workspace lock plus exploration revision preserves limits and conflict semantics |
| Two renderers claim the same candidate | River owns claim synchronization; application tests reject duplicate/stale publication |
| River rescues a still-running execution | At-least-once overlap is possible; verify supported eligibility check and application generation guard |
| Database unreachable during render | No publication without transaction; local deadlines bound work, River later rescues interruption |
| Queue full when work is rescued/retried | Existing execution remains within outstanding budget, not a fresh unbounded admission |
| Completion races last-interest cancellation | One serialized outcome; no orphan ready status or stale publication |
| Renderer reports deterministic failure | Terminal failed status, bounded explicit retry; no automatic poison-job loop |
| Two maintainers expire the same workspace/artifact | Idempotent locked transitions, exact-once reservation release and reconciliation |
| Upload succeeds, database commit fails | Uploaded object may be orphaned; no ready pointer until fenced commit; intent enables cleanup |
| Database commit response is lost | Query intent/result before cleanup; never delete a possibly published object blindly |
| Stale upload finishes after cleanup | Unique attempt key cannot overwrite winner; tombstone/list reconciliation finds late object |
| Bucket timeout or access denial | Bounded 503/backoff, not a false 410 or mass invalidation |
| Referenced object is confirmed missing | Invalidate pointer, preserve recipe; explicit action may rerender |
| Artifact deletion races a slow download | Already copied bounded bytes remain readable; no long-lived DB transaction |
| Migration succeeds, new app fails | Only schema-compatible rollback, never automatic database restoration |
| Old renderer execution appears after a build switch | Build-specific consumption and monotonic epoch/generation checks reject publication |
| Listener disconnects before completion | Durable outcome exists; web reconnect/snapshot makes it visible |
| Renderer misses enqueue notification | Subscribe-then-scan/reconnect and configured fallback find durable jobs |
| River leader dies | Other started renderer client takes over maintenance via River; no custom election |
| All renderers stop | Jobs remain durable; maintenance/rescue resumes when a renderer starts |
| River history cleaner removes completed job | Application outcome/recipe/artifact remains usable; no restrictive/cascading FK |
| PostgreSQL volume is lost with the VPS | Metadata is lost; database backup/restore work is excluded by owner instruction |
| Empty replacement database points at existing bucket | Do not automatically treat every existing object as garbage; require explicit ownership/state resolution |

## Trade-offs accepted in the proposal

1. **One database simplifies consistency but becomes a dependency.** This phase improves process-level durability, not host/database HA. Static pages can remain available during an outage; stateful operations cannot.
2. **Object storage fits image bytes but adds a cross-service boundary.** PostgreSQL holds only pointers/metadata. Upload-first, fenced pointer publication, durable deletion and orphan reconciliation replace atomic blob insertion. The bucket's independent lifecycle and costs must be measured.
3. **Short global admission locks limit throughput.** The current queue is tiny; serialized budget changes are understandable and correct. Avoid global locks on reads and measure before designing more elaborate coordination.
4. **Full release draining is simpler than mixed-build operation.** Sessions survive, but releases may have a brief interruption and must drain old jobs. Rolling mixed-version state/schema compatibility is deliberately a later milestone.
5. **River provides at-least-once execution.** Transactional completion and application publication fencing protect results; uploads remain external effects and need orphan cleanup. Pinned-version rescue/cancel semantics are an early proof obligation, not an assumed exactly-once guarantee.
6. **Anonymous returning-visitor retention is open.** The cookie mechanism can support days/months without login, but cookie, identity, favourites and preview lifetimes must agree. The first cutover intentionally resets legacy memory state; subsequent restarts do not.
7. **Renderer parent and children share a resource/network boundary.** Accepted for trusted compiled artwork, with process deadlines and bounded output. No additional coordinator container; size the renderer for queue/upload/maintenance plus child memory.
8. **Notifications improve latency but are not durable pub/sub.** Reconnect/snapshot and limited fallback are required. River also runs maintenance periodically; measure idle activity rather than imply zero polling.
9. **Use River instead of reimplementing its internals.** Library/driver/schema versions must move together. Exact owner alternation and global execution limits are not OSS guarantees; review the simpler admission policy rather than recreate a scheduler around River.

## Decisions for owner review

Owner feedback accepted River/election, the web/database/renderer boundary, shared child networking, managed bucket, single-host durability and publication/release approach. The later approval also accepted 90-day identity/favourite retention, total-nine admission and SSE. Database backup work is excluded. See implementation evidence for subsequent code and tests.

- PostgreSQL 17 plus pinned River OSS/`riverpgxv5`, `pgx/v5` and AWS SDK for Go v2 S3 dependencies; exact compatible versions/images selected and checked in WP1.
- Private S3-compatible image bucket from the beginning, with metadata/pointers only in PostgreSQL and the existing 2 GiB logical ready-cache bound.
- Managed Hetzner Object Storage is accepted as the production choice; verify permissions, API/provisioning/lifecycle support and costs before provisioning. A separate pinned local service supports disposable tests.
- Web proxies bucket reads through existing URLs; attempt-specific object keys, upload intents and asynchronous reconciliation address partial failure without cross-store transactions.
- Web uses River insert-only enqueueing and listens for results. Existing renderer container runs River consumption/maintenance, bounded child processes and upload. No extra coordinator or web render loop. River handles election; no custom leader service, lease engine or `pg_cron`.
- Returning without login: recommend 90 days of inactivity for a cookie-backed visitor identity and favourite recipes/previews, subject to owner review and explicit capacity design. Do not treat current short session/image expiry as approved. One-time legacy reset is accepted.
- River rescues abandoned attempts and handles explicitly classified transient retries within bounded budgets; deterministic render failures remain terminal/user-retryable. Exact timeout/rescue/fetch settings require pinned-version tests.
- One production build/renderer with one child slot initially; replica and maintenance-leader failure proved in a disposable rehearsal.
- Separate stable data Compose project; no automatic schema down-migration or pre-database fallback after persistent writes.
- No database backup planning or implementation in this task, including destinations, schedules, roles and restore drills.

**Review decisions resolved:** 90-day idle identities with favourite previews; total-nine outstanding cap and per-workspace quotas/River ordering; SSE. Provider policy and production sizing still require external verification; measured local connections/idle SQL/rescue evidence is in the implementation record.

## Research grounding and implementation gates

- [PostgreSQL LISTEN](https://www.postgresql.org/docs/17/sql-listen.html) and [NOTIFY](https://www.postgresql.org/docs/17/sql-notify.html): commit ordering, subscribe-before-snapshot, live-session broadcast and bounded payload. No replay/acknowledgement or timestamp-driven trigger.
- [River getting started](https://riverqueue.com/docs), [transactional completion](https://riverqueue.com/docs/transactional-job-completion) and [reliable workers](https://riverqueue.com/docs/reliable-workers): transaction integration and at-least-once execution. Prove cancellation/rescue overlap for our publication guard against the selected version.
- [River maintenance](https://riverqueue.com/docs/maintenance-services) and [leader election](https://riverqueue.com/docs/leader-election): built-in in-process maintenance, not a separate deployed service. [Configuration source](https://github.com/riverqueue/river/blob/master/client.go) currently documents a one-second fallback fetch default; maintenance documentation describes a five-second scheduler. These are research observations, not pinned implementation guarantees.
- [River subscriptions](https://riverqueue.com/docs/subscriptions): local worker-client events do not provide renderer-to-web result delivery. Implement an application SQL notification with durable resynchronization.
- [River uniqueness](https://riverqueue.com/docs/unique-jobs): job deduplication is not permanent artifact identity or exactly-once execution; history can be pruned. Keep exact canonical recipes and domain records independent.

River is selected; PGMQ, a custom SQL queue and `pg_cron` are not parallel implementations to build. Use only OSS APIs/features for the planned milestone. Any requirement for a fork, unsupported internal table mutation or Pro-only feature must be reviewed rather than added implicitly.

## Review conclusion

The self-review refined these boundaries: exact recipe bytes remain outside River JSON normalization; deployment epochs increase even on rollback; replay is scoped to an established cookie; result notifications never own essential state transitions; River execution history is not product storage; SQL/object storage cannot commit atomically. Application publication guards, unique upload keys and reconciliation address external-effect races without duplicating River's queue engine.

The packages cover application persistence, queue ownership, bucket-backed artifacts, private execution, Compose/bucket lifecycles, secrets, migrations, release/rollback, restart and multi-instance verification. The highest-risk seams are explicitly tested: admission atomicity, attempt fencing, upload/pointer partial failure, orphan cleanup, schema-aware rollback and production/test data isolation.

This pre-implementation review was approved. The [implementation record](implementation.md) documents actual changes and review findings; it does not authorize production rollout.
