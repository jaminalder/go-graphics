# PostgreSQL persistence proposal

**Status: approved for full local implementation and validation.** The owner accepted the final recommendations (90-day anonymous retention/favourite previews, nine outstanding jobs, SSE) and authorized dependency downloads/disposable Docker tests. No live bucket provisioning or public VPS deployment is authorized. See [implementation evidence and reviewed adjustments](implementation.md) and the current [runbook](../../operations/persistence.md).

This proposal adds restart-safe studio state and durable rendering work to Singular Seed. Product and infrastructure development takes place directly on `master`; branches/worktrees are reserved for artistic experiments. See the [development workflow](../../development/workflow.md).

## Read in this order

1. [Target state](target-state.md): behavior, ownership, data model and transactional contracts.
2. [Infrastructure and operations](operations.md): Compose, credentials, migrations, releases and restart behavior.
3. [Work packages](work-packages.md): ordered deliverables, dependencies and acceptance checks.
4. [Design review](review.md): source-based findings, failure analysis and decisions for approval.

The detailed target/work-package documents retain design history; their illustrative schema and provisional knobs are superseded where the [implementation record](implementation.md) says so. [Architecture](../../ARCHITECTURE.md), [data lifetimes](../../reference/data.md) and [operations](../../operations/persistence.md) describe the implemented code, without claiming a production deployment.

## Proposed decisions

| Decision | Proposal |
| --- | --- |
| Database | One self-hosted PostgreSQL 17 instance; exact supported minor/image digest selected and verified during implementation |
| Go dependencies | Pinned River OSS with its `riverpgxv5` driver, `pgx/v5` and AWS SDK for Go v2 S3 modules; explicit application SQL and migrations, no ORM |
| Studio state | Approved: cookie-backed anonymous identity, 90-day idle lifetime and retained favourite previews; no accounts/login |
| Queue | River owns execution state, claims, retries, rescue and maintenance election; application owns admission, recipe identity, interests and safe artifact publication |
| Rendering | Existing renderer service becomes a River consumer and bucket uploader with bounded child processes; no extra coordinator container, no web-to-renderer socket/RPC |
| Isolation | Render children share their container's network; retain subprocess deadlines/output limits and sanitized child environment, without claiming network isolation |
| Notifications | PostgreSQL `LISTEN`/`NOTIFY` for dispatch and result wake-ups; durable rows remain authoritative; reconnect/resync and River fallback/maintenance remain necessary |
| Artifacts | Private bucket; pointers/metadata only in SQL. 2 GiB/5000 accounted objects; ordinary expiry 24 hours, live favourite preview pins exempt |
| Object storage hosting | Approved: managed Hetzner Object Storage separate from the VPS; use a pinned local S3-compatible service for disposable development/CI; verify provider capabilities before provisioning |
| Deployment | Separate persistent data Compose project from replaceable application releases |
| First production topology | One host with Caddy, web, renderer and PostgreSQL; initially one render child at a time, plus the managed bucket |
| First cutover | Drain existing work; existing in-memory sessions reset once; no attempt to import live Go maps |
| Later releases | Preserve unexpired sessions and artifacts; drain release-bound jobs before switching builds; schema-aware rollback |
| Database backups | Explicitly outside this task: no backup design, implementation, destination selection or restore work package |
| Multi-instance proof | Disposable two-web/two-renderer rehearsal including River leader loss; production replication remains a follow-up |

## Outcomes

- Restarting web does not lose an unexpired workspace, its CSRF identity, favourites or committed jobs.
- An acknowledged action has its navigation, jobs and replay result committed together.
- Worker death can trigger a bounded retry of the same job without stale completion corrupting the result.
- Any compatible web instance resolves artifact metadata in PostgreSQL and reads the PNG from the shared private bucket.
- Existing finite resource limits, recipe/build identity and public HTTP protections remain explicit.
- An operator can migrate, deploy, inspect and restart services entirely through documented CLI procedures.

This does not establish application HA: PostgreSQL, ingress and the VPS remain single points of failure, and object storage is an additional external dependency. Automatic database failover, multi-host orchestration and zero-downtime mixed-build deployment are later work.

The owner accepted River and its maintenance election, shared renderer-child networking, managed object storage, single-VPS durability, bounded failure handling, cross-store publication, 90-day cookie identity/favourite preview retention, SSE and the one-time session reset/schema-aware release approach. Database backup work was explicitly removed.

## Final recommendations accepted for implementation

- River replaces the proposed custom claim/lease/scheduler engine. Verify its pinned-version behavior early, especially stale completion and cancellation; do not operate two competing queue managers.
- Prefer notification-driven dispatch with a proposed 30-second fallback fetch interval. River still runs periodic maintenance; measure and document total idle SQL rather than promise zero polling.
- Propose a simple global cap of nine outstanding render jobs (waiting + running + retrying) and one local execution slot initially. This replaces the old exact eight-waiting/one-running split and custom owner alternation; workspace quotas remain. See the target state for the trade-off.
- Recommend SSE for browser updates with snapshot resynchronization and normal HTML/manual refresh fallback. This presentation choice remains part of final review.
- Retain anonymous visitors for 90 days of inactivity with favourite previews, without accounts/login. Use bounded workspace aggregates and SQL preview pins as documented in the implementation record.

## Approval boundary

Implementation/local validation and the subsequent documentation audit/commit are authorized directly on `master`, one writer at a time. Live provider provisioning or public rollout is not authorized. The [implementation record](implementation.md) separates local evidence from external checks requiring operator access.
