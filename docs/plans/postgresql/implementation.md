# Persistence implementation and review record

Scope: approved local code/infrastructure tooling and disposable testing directly on `master`. The owner subsequently requested a full documentation audit and commit. No branch/worktree, real Hetzner resources or public deployment. Database backups remain excluded.

## Implemented architecture

Later load-test work supersedes the original nine-job/read-limit defaults: see [operational profiles](../../operations/limits.md). Production now defaults to 16 outstanding jobs, local-capacity 64, with two-minute first-start age and actual-byte image buffering. Original run measurements below remain historical evidence.

- Web: PostgreSQL-backed anonymous studio, transaction-bound River enqueue, shared expensive-operation quotas, S3 reads, result listener and authorized SSE.
- Renderer: River OSS v0.47.0 consumer/maintenance client in the existing service, fixed bounded child executable, PNG validation, S3 upload and serializable completion.
- PostgreSQL 17.11: studio aggregates, River state and artifact metadata, not PNGs.
- Private bucket: production Hetzner configuration/bootstrap tooling; pinned disposable MinIO only for local tests.
- Separate data Compose project, protected credentials, explicit `artdb` migrations/control and schema-aware release replacement.

## Reviewed implementation adjustments

1. **Bounded aggregate rather than a navigation table per domain type.** The existing domain already owns a bounded workspace and revision/replay rules. `studio.Persistent` loads one aggregate under a SQL transaction and binds its queue operations to the same transaction. Exact recipe bytes are retained independently. This avoids rewriting candidate generation and command semantics; no process memory is authoritative in production. The aggregate is limited to 2 MiB each/64 MiB total, at most 1000 identities, four explorations/eight batches/24 favourites. Batch pruning preserves favourite samples; separate SQL pins protect their images. There is no independent exploration expiry that can erase favourites.
2. **River completion is not sufficient by itself to fence attempts.** Source review showed `JobCompleteTx` primarily gates on running state. Publication first reads current attempt/state through River's public transactional API, checks application generation/epoch/interests and completes in a serializable transaction. Concurrent rescue/update forces a PostgreSQL serialization failure; at most two local retries reuse the same uploaded object. No custom claim/rescue/election or mutation of River state in production is introduced.
3. **Object cleanup identity is explicit.** Empty-prefix bootstrap records endpoint/bucket/prefix. Runtime and cleanup reject configuration pointing elsewhere. Deletion tombstones retain the first successful deletion time (not refreshed forever); repeated deletes catch late PUTs before bounded tombstone pruning.
4. **Finite retained storage.** Favourite previews are pinned for the 90-day idle identity lifetime. The 2 GiB/5000 accounted-object cap includes uploads and deletion backlog. Full storage rejects publication rather than silently evicting favourites. This is bounded retention, not an unlimited personal account/archive.
5. **Two schemas and explicit bootstrap.** Application checksum/version and River version set are checked. Fresh hosts can provision with `deploy_application=false`; install stages the verified artifact, then explicit secret/data bootstrap precedes activation. Secrets never pass through Terraform/cloud-init. Rollback checks schema compatibility and does not restart legacy code automatically after persistence cutover.
6. **No claim of zero polling.** Dispatch uses River notifications with a 30-second fallback. River v0.47.0 schedules every five seconds and rescues every 30 seconds, plus leadership activity. Active browser streams reconcile every 30 seconds; empty browser pages create no result polling. One render slot is per container, not a cluster-wide limit.
7. **Local fixtures are not production guarantees.** The MinIO image is an explicitly pinned historical test emulator. The managed Hetzner endpoint, current policies, scoped credentials and billing behavior require a later authorized provider contract check. No production storage is silently substituted with local MinIO.

## Work-package coverage

| Package | Local implementation/evidence |
| --- | --- |
| WP1 | Pinned clients/images, PostgreSQL/S3 fixtures, migrator, role setup, secret files, application/River compatibility checks |
| WP2 | Atomic workspace + queue transaction, exact recipe identity, replay/revision rules, global/per-owner admission, pins and shared token buckets |
| WP3 | River renderer/child lifecycle, attempt fencing, image upload/pointer publication, cleanup/tombstones, built-in election/rescue |
| WP4 | Persistent HTTP behavior, 90-day cookie renewal, database errors as 503, image digest reads, result notifications and SSE |
| WP5 | Real Compose runtime with DB/egress networks, private health, no render socket, persistent data project and local test helpers |
| WP6 | Explicit bootstrap, bundled database/emulator images, isolated candidate smoke, migrations/epoch switch and compatible rollback tests |
| WP7 | Real SQL/S3 race tests, restart/cleanup, leader handover/rescue, listener reconnect and failure boundaries; no DB backup work |
| WP8 | Current docs reconciled, full lint/test/browser/container checks and review/refactoring evidence below |

## Validation evidence

Executed during implementation:

- `make check`: formatter, vet, lint and full Go test suite passed; final affected-package tests and lint also passed after review fixes.
- `make test-persistence`: real PostgreSQL/S3 race tests passed, including batch replay, concurrent all-or-nothing admission, actual PNGs, retained favourites, expiry, stale attempt rejection, listener termination/reconnect, storage-error HTTP behavior, River leader handover/rescue and cleanup.
- `bash deploy/scripts/verify-compose.sh`: Caddy → SQL/River → real subprocess → bucket → browser PNGs; runtime DDL denial/container limits; web and PostgreSQL retained-volume recreation with the same workspace; renderer stop/recreation passed.
- `npm test` in `web/browser`: all three desktop/mobile/no-JavaScript journeys passed with SSE, real downloads, sharing and similarity.
- `python3 deploy/tests/test_activation.py`: ten activation/rollback scenarios passed, including legacy fallback refusal, migration failure and preservation of disabled admission.
- `python3 deploy/tests/test_provision.py`: four artifact/SSH preflight scenarios passed.
- `bash deploy/scripts/verify-cloud-init.sh`: real Ubuntu cloud-init/SSH plus installer tests passed.
- Terraform format/validate passed without contacting cloud APIs.

- Linux AMD64 application image cross-build passed; ARM64 application/edge images ran in local Compose. No immutable committed release archive was created from the dirty checkout.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...`: no vulnerabilities found.
- `make docs-check`: 56 pages and local links/anchors passed; `git diff --check` passed.
- Browser cleanup was hardened with a manifest-scoped teardown after Playwright's process-group termination left disposable Docker resources behind. The final browser run passed all three journeys and its test containers were removed; existing manual application containers remained.

Do not infer live VPS or provider checks from these local results.

## Deep review and refactoring findings

- Replaced direct deferred transaction rollback with a bounded cleanup-context helper, so an expired HTTP/job context does not strand a transaction.
- Added bounded local retry of serialization/deadlock aborts without repeating external image upload. Network-ambiguous commits are not treated as serialization failures.
- Checked the exact River migration version/name set, removed stale socket/cache directories from the runtime image, and retained only one production backend.
- Fixed SSE deadlines to cover individual writes rather than idle waiting; Caddy flushes streams and caps stream duration. Snapshot/reconnect restores state. Object reads allocate exactly the declared bounded size and reject trailing/truncated/corrupt bytes.
- Database failures preserve browser cookies; static gallery/about remain usable even for visitors presenting a cookie. Meaningful navigation renews 90-day identity, background result/image traffic does not.
- Added exact bucket-location binding, fixed missing-object cleanup, and kept deletion tombstone age stable so repeated deletes do not retain records forever.
- Favourite actions can explicitly recreate an unavailable preview; pins protect retained images. Removed obsolete interests when domain batch pruning drops samples, preventing unbounded subscriber/request retention.
- Hardened rollout: candidate migration is not blocked by demanding the new schema before migration; compatible rollback does not re-enable admission unless its service readiness succeeds; installer cannot override schema-aware recovery.

## Measured notification/maintenance behavior

With a settled single River client and empty queue, `pg_stat_statements` over 12 seconds recorded three leadership renewals and two scheduler transactions (BEGIN, schedule, COMMIT), plus the measurement reset statement: ten recorded calls total, five database connections in that run. No rapid empty render-fetch loop appeared. This is a short fixture measurement, not an all-day production benchmark; renderer heartbeats, web health probes and scheduled minute cleanup add their own traffic in the full deployment.

Two live River clients successfully handed over maintenance after the leader stopped. Pre-aged abandoned executions were rescued and rendered in approximately 12–38 seconds across test runs. Those jobs were deliberately already beyond the two-minute eligibility threshold; this does not claim 12-second recovery from a fresh real crash.

## External boundaries

Not executed under the local-only permission: real Hetzner bucket creation/conformance, production CPU/disk sizing, host reboot/replacement, actual DNS/HTTPS production cutover. Container builds and disposable runtime tests were performed before the requested commit; no immutable committed release archive or production activation was tested.

The original work-package matrix is a design checklist, not blanket test certification. Two River clients were exercised in integration tests; a full two-web/two-renderer multi-container failover topology was not rehearsed. Tests simulate pre-aged abandoned rows and superseded attempts rather than proving every crash/network-partition timing. Initial/idempotent/checksum migrations and scripted rollback failures were tested; there is no preceding PostgreSQL application schema for a real version-to-version data upgrade test yet. These limits should guide subsequent rollout validation.

## Documentation alignment audit

The pre-commit audit checked C4 context/container/component/code/runtime/deployment views, package/API/data/configuration references and operational commands against source. It corrected stale zero-dependency and memory-queue claims, restored one-container-per-component diagrams including child/CLI views, distinguished anonymous renewal from background reads, and documented actual SQL aggregates/roles/pools and cleanup behavior. Planning diagrams/tables remain explicitly historical where superseded. The current schema reference and persistence runbook are authoritative for implementation behavior.

Pre-commit `make check`, `make docs-check` and `make docs` passed. Browser validation found and fixed a Mermaid sequence-label parse error invisible to link checks. All six architecture pages then rendered successfully: context, containers, four component diagrams, code, runtime and deployment (nine diagrams total). Generated HTML/screenshots remain under ignored `out/`.

Database volume loss still loses identities/favourites/jobs. No backup implementation or restore promise is included. Provider-issued per-service credential policies must be checked before production use.
