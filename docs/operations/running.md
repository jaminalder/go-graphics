# Running and diagnosing services

Use [persistence operations](persistence.md) for complete local startup, database bootstrap, secrets and CLI commands. The supported web runtime requires PostgreSQL and a private bucket; running bare `artweb` without them fails explicitly.

```sh
make check-compose
bash deploy/scripts/browser-server.sh
```

## Health and controls

- `artctl live`: process-local web liveness.
- `artctl ready`: both schemas and a matching recent renderer/storage-health record in PostgreSQL.
- `artctl renderer-live` / `renderer-ready`: renderer-local HTTP at loopback `:8082`.
- `artctl metrics`: queued/running jobs and artifact bytes/count.
- `artctl generation-off/on`: durable global admission; off lets existing work drain. Restart does not re-enable it.
- `artdb pause/resume`: River dispatch for the active build queue; separate application maintenance continues.

Caddy blocks public health/control paths. A Docker health status alone does not restart a running process. Renderer heartbeat runs every 15 seconds; readiness considers it stale after 45 seconds, so abrupt failure detection is bounded rather than instantaneous.

## Diagnosis

| Symptom | Inspect / meaning |
| --- | --- |
| 421 | Canonical hostname and `ART_ORIGIN` / Caddy address |
| 403 | Origin or CSRF mismatch; refresh the form |
| 409 | Exploration revision changed in another tab |
| 410 | Expired identity or confirmed missing/expired image |
| 429 | Shared expensive-action quotas or per-process read limits |
| 503 | Admission/storage limits, database failure or bucket unavailability; cookies are not cleared on storage failure |
| Delayed results | Renderer build queue, River states/errors, listener reconnect and SSE network connection |
| Failed render | Child deadline/output error; deterministic failures need explicit retry |
| Accumulating objects | Upload intents, deletion errors and bound bucket location; never delete based on a disconnected/empty database |
| Failed activation | Schema compatibility and preserved admission state; rollback must use a compatible persistent release |

Use structured service logs and private SQL inspection. Logs omit raw request cookies/query parameters; the River/job identifiers in internal logs are not browser capabilities. Compose rotates local logs. There is no external monitoring service or database backup system in this change.

State survives service restarts and retained-volume reboots. It does not survive losing the PostgreSQL data volume. The managed bucket has a separate lifetime, but its objects cannot reconstruct visitor identities or favourite recipes. [Data reference](../reference/data.md) states retention and capacity limits.
