# Running and diagnosing services

## Local Linux rehearsal

The supplied Compose file runs Caddy, web and renderer with their actual network/user/resource restrictions. Docker with Compose is required.

```sh
docker compose --env-file deploy/local.env -f deploy/compose.yaml up --build -d --wait
docker compose --env-file deploy/local.env -f deploy/compose.yaml ps
```

Open the origin from `deploy/local.env` (normally `http://localhost:8088`). For the complete disposable runtime check, run `make check-compose`; it uses a separate project and loopback ports 18088/18443 and cleans up its own volumes. The [deployment view](../architecture/deployment.md) explains the difference between direct Go development and container rehearsal.

## Host operations

Use the release wrapper so commands address the one correct project, operator environment and immutable images:

```sh
/opt/art/current/deploy/scripts/compose-release.sh /opt/art/current ps
/opt/art/current/deploy/scripts/compose-release.sh /opt/art/current logs --tail 100 web renderer
/opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl ready
/opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl metrics
```

Run commands with the host privileges needed to control Docker. `artctl live` tests web liveness, `ready` tests the renderer plus build match, and `renderer-ready` runs inside the renderer container to check its own socket. The public `/health/ready` route is only a web 204 response; it is not the operational renderer readiness probe. Caddy blocks public health/control paths.

## Pause generation

```sh
/opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl generation-off
/opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl metrics
/opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl generation-on
```

Disabling admission leaves existing queued/running work and cached browsing available. Inspect `art_jobs_queued` and `art_jobs_running` to observe draining. The remaining counters are `art_cache_files` and `art_cache_bytes`. This switch is process memory; a restart takes its initial value from `ART_GENERATION`.

## Diagnose a failure

| Symptom | Evidence to inspect | Meaning / action |
| --- | --- | --- |
| 421 unknown host | Requested hostname vs `ART_ORIGIN` | Use canonical origin; check proxy/site configuration |
| 403 form rejected | Origin and current session/form | Refresh stale page; verify canonical HTTPS settings |
| 409 conflict | Another tab changed the exploration | Reload current revision before another action |
| 410 expiry | Session lifetime or artifact age | Restore saved recipes or make a new explicit request |
| 429 pause | Rate limits | Wait; repeated submits do not increase capacity |
| 503 generation busy | Queue/running counters and generation switch | Let admitted work finish; inspect resource pressure/readiness |
| Renderer unavailable | Private `ready`, renderer logs, matching image IDs/socket | Restore a consistent release; verify renderer is running |
| Failed samples | Child timeout/error logs, CPU/memory constraints | Retry explicitly only after examining repeated failures |
| Activation failed | Activation output and current symlink | Verify rollback service/readiness before resuming changes |

Logs are structured key=value text on stderr with service and operation context and bounded HTTP outcome fields. HTTP logging avoids raw query values, cookies and full request URLs; see [logging implementation](../../internal/logging/logging.go) and [HTTP logger](../../internal/logging/http.go). Compose uses local log rotation (10 MiB, three files per service). These are local observability facilities, not a deployed monitoring service.

## State and recovery

Workspaces are transient and disappear on restart; cache volumes are disposable and reconciled. Keep exported recipes/images for artist recovery, immutable release directories for rollback, and protected Terraform state/operator configuration for infrastructure recovery. Caddy data/config volumes contain certificate state. The repository does not implement a coordinated backup/restore command for all these stores. [Data lifetimes](../reference/data.md) distinguishes what can be regenerated from what the operator must retain.

Source: [runtime configuration](../../deploy/compose.yaml), [private control command](../../cmd/artctl/main.go), [application shutdown](../../cmd/artweb/main.go), [release wrapper](../../deploy/scripts/compose-release.sh).
