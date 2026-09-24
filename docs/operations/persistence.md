# PostgreSQL, River and image storage

The web studio requires PostgreSQL and a private S3-compatible image bucket. `staticart` remains independent. The production runtime is web → PostgreSQL/River ← renderer, with renderer uploading images and web reading them. There is no direct renderer RPC or extra coordinator service.

## Local development and verification

```sh
make test-persistence       # disposable PostgreSQL + S3, Go race/integration tests
make check-compose         # real images, Caddy, separate runtime roles, restart tests
bash deploy/scripts/browser-server.sh # disposable studio on http://127.0.0.1:8280
```

Stop the browser server with Ctrl-C to remove its disposable containers, volumes and generated secrets. All local helpers own uniquely named projects except the dedicated integration fixture `art-persistence-test` (ports 15439/19009; run it once at a time). Existing manually started applications are not reused or modified.

The local object emulator is a pinned MinIO image, used only for development/testing. Its root credentials are disposable fixture values, not the production credential model. Local tests do not prove Hetzner-specific policy/API compatibility. Production bucket connections use verified HTTPS and separately supplied credentials. PostgreSQL currently uses SCRAM over the same-host private bridge with `sslmode=disable`; remote database placement requires a separately configured verified TLS/tunnel boundary. If a browser run is forcibly terminated, `python3 deploy/scripts/cleanup-browser.py` removes only the recorded test resources. The browser manifest/ports are shared: run one browser helper/test runner at a time.

## Fresh VPS bootstrap

The PostgreSQL data project is separate from replaceable application releases. Its `postgres_data` volume persists through application replacement and retained-volume host reboot. It has no published database port. The image bucket is independent of the VPS Terraform lifecycle.

1. Provision a fresh host with Terraform `deploy_application=false`. For an existing host, keep its data/credentials and use normal release activation.
2. Explicitly create/verify the private bucket using `python3 deploy/scripts/provision-bucket.py --help` and provider-issued credentials. This is a separate billable operation; the helper requires AWS CLI and never passes access keys in argv or Terraform state. Verify bucket policy and separate web/renderer permissions with the provider before deployment. No provider lifecycle rule should delete retained favourites.
3. Copy `deploy/storage.env.example` to `/etc/art/storage.env`, set endpoint/region/bucket/prefix and protect it. Put JSON `{ "access_key": "...", "secret_key": "..." }` in `/etc/art/secrets/web-objects` (read-only bucket operations) and `renderer-objects` (upload/read/list/delete for the application prefix). These files are supplied separately; never put credentials in release archives or cloud-init. Current production bootstrap fixes `/etc/art/secrets`, database hostname `postgres`, database `art` and project `singular-seed-data`; retain those defaults unless updating the bootstrap/wrappers together.
4. Upload/install the checksummed release. If data secrets are absent, the installer deliberately stages it under `/opt/art/releases/<commit>` and exits with bootstrap instructions. It does not start an empty replacement application.
5. Run `sudo bash /opt/art/releases/<commit>/deploy/scripts/bootstrap-data.sh /opt/art/releases/<commit>`. This creates database credentials once, starts PostgreSQL, runs application and River migrations, installs runtime DML roles and explicitly binds an **empty** bucket prefix. Existing secrets are not rotated. A populated unrelated prefix is refused.
6. Retry the release installation/activation (or enable `deploy_application` and reapply Terraform). See [releases](releases.md). The first cutover intentionally resets legacy in-memory sessions.

Bucket binding stores the exact endpoint/bucket/prefix in PostgreSQL. Web, renderer and cleanup reject a different configured location. Changing locations requires an explicit data migration, not editing an environment variable while cleanup is active.

## Runtime credentials and schema

- `/etc/art/secrets/postgres-password`: PostgreSQL initializer, root-only.
- `admin-database`: owner connection string, root-only and used by explicit one-shot administration.
- `web-database`, `renderer-database`: SCRAM login strings, root:10000 mode 0440; distinct login roles with the same application/River table DML and sequence grants. They cannot create schema objects or modify migration records. They are not row-level or producer-versus-consumer authorization boundaries.
- `web-objects`, `renderer-objects`: root:10000 mode 0440. Children receive a sanitized environment and pipes, but sharing a container is not a hostile-code sandbox.

Application startup checks every ordered application migration/checksum and the exact River migration set. `artdb migrate` serializes both under an advisory lock; runtime never migrates automatically. Migration 2 adds monitoring without modifying version 1. The previous binary's exact schema check rejects version 2, so rollback requires a compatible build. River automatic reindexing stays disabled to avoid runtime DDL rights.

Changing `POSTGRES_PASSWORD_FILE` does not rotate an initialized role. Explicit role/password changes and matching secret replacement are required. Do not overwrite generated secret files casually. The installer rewrites `operator.env` but preserves `storage.env` and the secret directory.

## CLI controls

Use `make watch` locally or `sudo python3 /opt/art/current/deploy/scripts/watch.py --project singular-seed` on the VPS for the live queue/instance/producer-to-renderer view with Docker names. [Terminal monitoring](monitoring.md) explains counts, naming and direct `artctl status/watch` commands.

```sh
sudo bash /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current ps
sudo bash /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current logs --tail 100 web renderer
sudo bash /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl ready
sudo bash /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl metrics
sudo bash /opt/art/current/deploy/scripts/admin-release.sh /opt/art/current status
sudo bash /opt/art/current/deploy/scripts/admin-release.sh /opt/art/current admission
sudo bash /opt/art/current/deploy/scripts/admin-release.sh /opt/art/current disable
sudo bash /opt/art/current/deploy/scripts/admin-release.sh /opt/art/current pause
sudo bash /opt/art/current/deploy/scripts/admin-release.sh /opt/art/current resume
sudo bash /opt/art/current/deploy/scripts/admin-release.sh /opt/art/current enable
```

`disable` stops new admission but lets current jobs drain. `pause` stops dispatch on the current build's River render queue; application maintenance uses a separate queue. `enable`/`disable` are durable; restarting web does not change them. `artctl generation-on` additionally checks renderer readiness. `artdb activate` rejects unfinished render jobs and increments the active build epoch; it is a release operation, not a retry command.

Inspect attempts/failures through private SQL against `river_job` and application records; never directly mutate River execution state in production. User retry actions recheck admission and ownership. An operator UI and arbitrary per-job administration are not exposed publicly.

## Execution and failure behavior

- River v0.47.0, one render worker per renderer container; a second container adds a second execution slot. Nine outstanding render jobs globally, four active interests per visitor, 24 subscribers per rendition.
- Render children: 15-second preview / 30-second download limit, 16 MiB maximum PNG, fixed executable, sanitized environment. Parent job: 90-second deadline, at most three attempts. Deterministic invalid/render/output failures are terminal; database/bucket interruption can retry.
- First start after one minute is expired; already-started retries have a five-minute lifetime. River rescue eligibility is two minutes; its pinned rescuer runs every 30 seconds and retry scheduling adds delay. This is not a promise of exact two-minute recovery.
- River's fallback fetch interval is 30 seconds. Its scheduler runs every five seconds; leader renewal and other maintenance also cause SQL. `LISTEN`/`NOTIFY` avoids rapid idle job polling, not all background queries.
- Web uses one dedicated result listener per process and SSE only for active pages. Reconnect signals a snapshot; active streams reconcile every 30 seconds. No listener per browser, durable event log or essential completion callback in web.
- Upload intent is committed before unique-key upload. Publication uses a serializable transaction to check River attempt/state, application generation/epoch/interests and commit pointer + outcome + `JobCompleteTx` + notification. PostgreSQL serialization aborts are retried at most twice locally, without rerendering/reuploading; ambiguous network commits are not blindly retried/deleted.
- Application cleanup runs through a minute-period River maintenance job, with bounded batches and idempotent SQL/deletion. No custom election. All renderers stopped means application maintenance waits until one returns.

Each service uses an eight-connection pgx pool. Web additionally owns one dedicated result-listener connection; River's own listener/maintenance connections are included when sizing the database. PostgreSQL's 40-connection ceiling must be reconsidered before increasing replicas. Acquisition is context-bound; SQL statements have a ten-second timeout and lock waits five seconds.

During bucket failure the renderer checks bucket health before expensive rendering and lets River retry failures up to the configured attempt budget. It does not automatically pause the render queue or disable admission. Operators can use the distinct admission/dispatch controls; generation readiness fails while the recent renderer storage probe reports failure.

## Storage bounds and retention

Anonymous cookies/identities expire after 90 days of inactivity. Meaningful navigation/actions refresh them; image/fragment/SSE traffic does not. No login, cross-device discovery or lost-cookie recovery is added.

Favourite previews are pinned while their owner remains unexpired; un-favouriting or clearing releases the pin. Normal artifacts expire after 24 hours. Pinned and disposable images share a hard 2 GiB / 5000-object accounted budget, including uploads/deletion backlog. A full budget rejects new publication rather than deleting retained favourites. Four concurrent web image buffers cap encoded image memory at 64 MiB.

The workspace uses a bounded serialized aggregate in PostgreSQL (`bytea`), retaining the existing domain rules: four explorations, eight recent batches/actions, 24 favourites. It is loaded under a workspace transaction, not kept authoritatively in process memory. Canonical recipes are also stored byte-exact in render requests. There are at most 1000 identities and a 64 MiB aggregate snapshot budget; no unbounded permanent account store. Favourite ownership stays in the same durable aggregate; navigation pruning preserves favourite recipes. This deliberately replaces the plan's larger normalized navigation schema.

No database backup work is included. Destroying the VPS/data volume loses identity/job/recipe metadata; surviving bucket objects cannot reconstruct it. See the [implementation evidence](../plans/postgresql/implementation.md) for tested boundaries and outstanding external checks.
