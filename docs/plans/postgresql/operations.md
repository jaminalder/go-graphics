# PostgreSQL infrastructure and operations proposal

**Approved design baseline, retained as planning history.** Use [current persistence operations](../../operations/persistence.md) for executable commands and [implementation record](implementation.md) for adjustments/evidence. The provisional values below are not runtime configuration.

## Compose and host layout

Keep the existing Terraform-managed VPS for the first deployment. Add a dedicated data Compose definition, proposed as `deploy/compose.data.yaml`, with stable project name `singular-seed-data` and an explicitly named PostgreSQL volume. Application release switching must never recreate or remove that volume.

- The data project owns PostgreSQL and an internal database network. The application project joins that network through an explicit external-network reference.
- PostgreSQL has no host-published port. Web, renderer and one-shot migration clients join the database network with narrowly required access; Caddy does not.
- Web retains its existing trusted Caddy address on the proxy network. The first production topology does not require changing proxy trust to a broad subnet.
- There is no extra worker/coordinator service. `artrender` runs River, child supervision and upload. Remove the web/renderer shared socket volume and old web cache mount when cutover completes; no direct web-to-renderer API remains.
- Web and renderer need outbound HTTPS to the bucket in addition to the internal database network. Add explicit egress-capable networking without public renderer ports; preserve web's Caddy trust checks. Renderer loses `network_mode: none`, as accepted; render children share that namespace. Retain non-root/read-only/container resource restrictions and sanitized child environment, but do not describe the children as network-isolated.
- PostgreSQL needs a writable data directory, runtime socket directory and shared memory. Do not blindly inherit the application's scratch/read-only/user/capability defaults. Verify the chosen official image's initialization and steady-state requirements in real containers, and use the smallest working privileges.
- Select a supported PostgreSQL 17 minor and pin its digest, volume mount path and architecture in the data release metadata. Do not invent a digest in the plan. Keep database image upgrades separate from ordinary app activation; major upgrades are outside this milestone.
- Provision idempotently: an existing database volume and roles must not be reset by `terraform apply`, release install or host reboot. Docker starts the data service after reboot; applications retry startup connections with bounded backoff.

Before production cutover, measure the host's real available resources. Existing service limits permit a 2 GiB renderer, 384 MiB web and 256 MiB edge. Start with a proposed 512 MiB database and modest connection/shared-buffer limits. Renderer memory now covers its parent (River, maintenance, upload buffers) plus the child; there is no separate 128 MiB coordinator. Initially configure one render worker slot and a separately bounded low-concurrency maintenance queue. Measure full render/read/upload/cleanup workloads, database idle activity and OS headroom. Resize the host if needed rather than silently oversubscribing children.

## Private image bucket from the first deployment

**Accepted production choice: managed Hetzner Object Storage**, independent of the application VPS. The application uses S3-compatible operations so endpoint/region/bucket can change without rewriting the studio. No bucket has been created or its provider capabilities verified yet. No production object-storage daemon runs on the existing small VPS.

- Provision a private bucket with public access disabled, a stable region/endpoint and an application-controlled prefix. Verify authentication, access policies, single-PUT/read-after-write behavior, metadata/checksum support, pagination, delete semantics, lifecycle support and costs against current provider documentation and a disposable live bucket before committing to the service.
- Use verified HTTPS. Configuration names proposed for implementation: `ART_S3_ENDPOINT`, `ART_S3_REGION`, `ART_S3_BUCKET`, `ART_S3_PREFIX` and `ART_S3_CREDENTIALS_FILE`. Persist logical storage location + key in SQL, not a hostname URL; location mapping is operator configuration. Path-style addressing is a configurable compatibility option for local/test endpoints.
- Prefer infrastructure-as-code for bucket/policy/lifecycle metadata. During WP1 select a supported provider/tool after verification; do not assume the existing `hcloud` provider manages S3 buckets. Use a separate storage Terraform root/state or idempotent CLI provisioning if provider support is unsuitable. The existing VPS root's destroy must not delete the image bucket. Review storage destruction explicitly; avoid automatic bucket-emptying on destroy.
- Do not create object-store access keys through Terraform resources if doing so would place secret values in state. Supply separately retained credentials through protected files. Web needs read-only object access; renderer needs upload/verification plus list/read/delete for application maintenance, scoped to the image prefix. Do not supply these credentials to children through their environment or stdin. Shared same-UID file mounts are not a hard secret-isolation boundary. If provider policies cannot express the required service roles, document minimum supported permissions for review. Neither service can manage the provider account.
- Use immutable attempt-specific keys. Disable versioning initially for disposable artifacts unless a documented reason requires it; if enabled, explicitly expire noncurrent versions/delete markers. Application-controlled expiry and durable deletion are authoritative. A delayed provider lifecycle safety net is configured only after checking that it cannot preempt live object retention.
- Monitor ready bytes, in-flight uploads, deletion backlog, actual bucket bytes/object count, request errors/latency and egress cost. The 2 GiB ready-cache bound is not a provider quota. Enforce an operational storage ceiling and pause new work on sustained cleanup failure.

For local development, CI and smoke, run a maintained, pinned S3-compatible service in disposable Compose infrastructure. WP1 selects/verifies its image and license/maintenance status; a local emulator alone is not provider conformance evidence. Each test project gets its own credentials, bucket and volume. Exercise the same Go SDK adapter, and run an opt-in provider contract check against an explicitly disposable live bucket. Ordinary CI/smoke must never require production credentials or contact the production bucket.

## Credentials and connections

Proposed host secret directory: `/etc/art/secrets/`, separate from the installer-rewritten `/etc/art/operator.env`.

- Generate unique secrets during explicit data bootstrap. Keep them out of Git, cloud-init content, Terraform variables/state, image layers, release archives, logs and command-line arguments. The local development setup creates ignored disposable secrets; examples contain placeholders only.
- Bootstrap/administrator credential is used only for initialization and role administration. A migration owner owns application/River DDL. Web and renderer have only required DML/function permissions, including River operations required by their roles. Neither can alter schema or roles. Verify River maintenance permissions on the pinned version; disable optional automatic reindexing if it would require granting runtime ownership/DDL privileges, rather than broadening roles silently.
- Supply credentials through read-only Compose secret/file mounts, with actual UID/mode access tested. Compose local secrets are file mounts, not an encrypted secrets-management service. Root-protect and separately retain the operator's secret bundle.
- Use proposed `ART_DATABASE_URL_FILE` for a per-service connection string; support certificate file configuration for later remote TLS. Never log the URL/password. Keep migration connection configuration distinct from application credentials.
- Use SCRAM authentication. Plain transport is limited to the same-host private bridge for this first phase; cross-host database access must add verified TLS or an explicitly secured private tunnel before deployment.
- Proposed initial budget: web query pool 4 plus one dedicated result-listener connection; renderer pool 6 plus any River listener connection outside that pool, verified from the selected driver. PostgreSQL starts with a 30-connection budget allowing migration/diagnostics; recompute for the two-instance rehearsal. River needs connections for fetching, leadership and maintenance as well as result commits. Avoid pool starvation; document observed connections and keep listeners out of long transactions. Direct/session connections support `LISTEN`; do not introduce transaction-pooling middleware for listeners. Bound acquisition/query/lock/transaction timeouts and propagate cancellation.
- Rotation must change the database role password and the mounted client secret together, then verify reconnects. Changing `POSTGRES_PASSWORD_FILE` on an existing data directory does not rotate an existing role.

The scratch application image needs CA roots from the first bucket-backed deployment for verified HTTPS. Install them during the image build and test TLS verification; never disable certificate validation. Database and bucket remain optional for `staticart`, required for web/renderer.

## Initialization and migrations

Introduce a one-shot database administration command, proposed as `artdb`, with explicit initialize/migrate/status capabilities and SQL migrations embedded in the built application image.

1. Data bootstrap starts PostgreSQL, creates roles/database idempotently and verifies persistent volume identity.
2. A migration process acquires a PostgreSQL advisory lock and verifies applied migration checksums.
3. Apply application migrations and pinned River migrations in explicit dependency order using River's supported migrator. Use its own version metadata; do not copy/edit library migrations or promise all upstream migrations are transactional. Inspect selected migrations and verify interruption/retry behavior. Record application checksum/version only after success; bound lock/statement waits.
4. Release metadata declares application schema range, River library/driver version and required River schema version. Runtime startup/readiness rejects incompatibility in either schema.
5. Web/renderer do not race automatic DDL at startup. Migration credentials are not mounted into them. Library upgrades include migration and rollback-compatibility review, not just `go.mod` updates.

Prefer expand/contract changes. Any nontransactional operation requires a reviewed, resumable procedure. No automatic application or River down-migration accompanies application rollback.

## Release pipeline changes

The current [builder](../../../deploy/scripts/build-release.sh) archives two images; [activation](../../../deploy/scripts/activate-release.sh) assumes two immutable IDs and one web-owned queue; [smoke](../../../deploy/scripts/smoke-release.sh) creates an isolated application project. All must be adapted together.

### Build and smoke

- Update existing `artrender` to run the River worker and child mode; add `artdb`, but no `artworker` binary. Archive application/River migration and dependency-version metadata with the release.
- Build a separately versioned/pinned data bundle containing the PostgreSQL image identity and Compose/bootstrap files. Retain bucket configuration/provisioning definitions separately from secrets, plus the pinned local object-store image used by smoke. Update checksum and platform verification so a fresh host can run a smoke check without fetching unrecorded container images; managed bucket access still requires network connectivity.
- Smoke creates unique disposable PostgreSQL and local S3 services, buckets, secrets, networks and volumes, applies migrations and tests the candidate against them. It must never use production credentials, external production networks, production endpoints or data volumes. Bootstrap creates test buckets explicitly and waits for both SQL and object-store readiness.
- Rehearse migrations from a prior schema with synthetic retained state as well as from empty state. A fresh-database smoke alone does not prove upgrade compatibility.

### First database cutover

1. Take/export any recipes the operator wants to retain; communicate the one-time workspace reset.
2. Provision the private image bucket and scoped credentials; bootstrap database service, roles and migrations. Verify bucket operations and runtime reconciliation in an isolated environment.
3. Smoke the candidate against disposable infrastructure.
4. Disable legacy admission and drain the old queue using its current release wrapper. A drain timeout aborts before switching.
5. Stop old web/renderer, initialize the active database build/queue, start the River-enabled renderer and web, then check serving/generation readiness, notification connectivity and a real render/download.
6. Confirm that a web restart preserves the new workspace and its artifacts.

There is no live import of the old Go maps or disposable cache. Keep the old volume/release available for inspection until cutover is accepted; remove it only as deliberate cleanup.

Before the new service accepts user writes, failure can return to the legacy release. Once persistent writes have been accepted, automatically falling back to a pre-database binary would hide them; stop for an explicit recovery decision instead. Do not label that a seamless rollback.

### Subsequent database-backed releases

1. Verify image and schema compatibility and isolated smoke results.
2. Acquire the existing host deployment lock. Globally disable admission, but leave dispatch enabled to drain old-build jobs. Preserve the previously intended admission state for success/rollback.
3. Wait for no outstanding render jobs within a bounded deadline (including River rescue/retries). Abort safely if draining fails. Pause the render queue through River and gracefully stop old renderer/web before migrations/build switching. Stop maintenance before incompatible migrations too; intentional render pause alone must not disable cleanup.
4. Apply migrations once. Recheck that a retained rollback release accepts the resulting schema; otherwise declare a roll-forward-only release before changing production.
5. Switch the active build/queue and increment control epoch transactionally. Start matching renderer/web. Renderer consumes only its build queue and publication checks generation/epoch plus River eligibility; old executions cannot publish after a switch. Use supported River queue controls to resume the intended queue; do not let process startup override an operator pause.
6. Verify both schemas, renderer build/queue and recent health, notification reconnect/snapshot behavior, origin and a full studio action. Restore the prior admission policy only after checks succeed.

Unexpired sessions, favourites and existing images survive releases. Workers never execute another build's pending jobs. Unfinished legacy-build work discovered unexpectedly at startup blocks activation with a diagnostic rather than being silently dropped or reinterpreted.

On failure, restart the prior **schema-compatible database-backed** release and restore its control state. A symlink rollback alone is insufficient. Never drop the database or down-migrate automatically. First-install failure also preserves the data volume for diagnosis.

## Data lifetime and scope

Database backup planning and implementation are explicitly excluded at the owner's request. There are no backup jobs, backup credentials, destinations, retention schedules, restore procedures or RPO/RTO commitments in this milestone.

Persistence covers application/container restarts and retained-volume host reboots. Losing the PostgreSQL volume loses anonymous identities, favourites, jobs and image pointers; the independently surviving image bucket cannot reconstruct that metadata. Runtime reconciliation of uploads, pointers and deletion intents remains in scope because ordinary process crashes can interrupt them.

Update the current destroy/recreate documentation: the VPS Terraform root removes the VM and its local PostgreSQL volume, while the separately provisioned image bucket survives. Protecting a Compose volume from app scripts does not protect it from server deletion. Avoid routine `down --volumes` against production; disposable cleanup must prove project and bucket identity. Never start automatic orphan deletion against a pre-existing bucket after initializing an empty replacement database: bucket ownership/state must be resolved explicitly first.

## Maintenance, diagnosis and availability

- Add CLI diagnostics/actions using supported River APIs for schema status, queue states/oldest age, attempts/errors, pause/resume, cancel/retry and drain. Keep application admission separate from dispatch pause. Raw operator retry must not bypass build/interest/capacity checks. River UI is optional later, private if enabled; no UI is required for this milestone.
- River owns leader election and internal scheduling/rescue/cleanup. Application expiry, terminal-outcome reconciliation and bucket cleanup run as bounded idempotent maintenance jobs in a separate queue, with transactional eligibility and startup catch-up. Do not add custom election, claim/lease loops or `pg_cron`. Render pause does not pause maintenance; when all renderer services are stopped no application maintenance runs.
- Observe queue age, retries/rescues, terminal failures, renderer/leader health, listener reconnects, pool waits, idle query rate, database latency, artifact logical/physical usage, disk/WAL growth and autovacuum. Measure the pinned version's fallback and scheduler activity. Proposed 30-second fetch fallback is not a promise of zero idle SQL or that all River maintenance uses that interval.
- Database unavailable: static browsing remains possible; session/job/image operations return bounded 503s; River cannot claim or commit results. Local child/job deadlines bound ongoing execution. Reconnect listeners using subscribe-then-snapshot, and let River recover interrupted executions. No empty replacement store or assumed custom lease-renewal cancellation.
- Bucket unavailable: navigation remains readable, image retrieval returns bounded 503 and generation readiness fails. Check storage availability before expensive rendering, bound retries, and use River's supported controls/backoff rather than repeatedly rendering into a known outage. Track a storage-outage pause separately from operator pause so recovery cannot undo the latter. Job context limits total work (proposed 90 seconds); classify transient failures for capped River retries, deterministic failures as terminal. Upload/delete retries and orphan age remain observable.
- Liveness is process-local; serving readiness checks SQL/both schemas; generation readiness uses a fresh matching renderer status/queue and storage health from PostgreSQL, not direct web-to-renderer RPC. Prefer River's supported metadata where sufficient; add only minimal build/health status if needed. Notification connections have their own health; bounded active-result reconciliation prevents silent UI stalls. Compose `unhealthy` does not itself restart a running process.
- Verify pinned-major patch updates against disposable retained-volume instances before changing the deployed image. Major-version upgrades require a separately reviewed task.

## Production cutover gate

Required evidence before a public deployment:

- Owner approval of target behavior and one-time legacy-session reset.
- All work packages through release integration and recovery checks complete.
- Passing real PostgreSQL concurrency/failure tests and existing HTTP/browser/security checks.
- Pinned River rescue/cancel/transactional-completion behavior verified, including stale publication rejection, maintenance leader loss, reconnect and measured idle SQL. Review documented queue-order/capacity differences from the old manager.
- A real Compose render/download and restart-preserved workspace.
- No public database port; runtime roles cannot perform DDL; credentials absent from archives/logs/state.
- Private bucket, scoped credentials, verified TLS, conformance check and production/test bucket isolation verified. Upload/pointer failure, stale upload, late PUT, deletion failure and runtime reconciliation exercised.
- Host CPU/memory/disk budget measured with artifact churn.
- Documented schema-compatible rollback and first-cutover failure behavior.

Multi-host PostgreSQL replication, durable artwork archives and orchestrator-specific volume placement remain later decisions. Image object storage is part of this first deployment; the application/database still have one host failure domain.
