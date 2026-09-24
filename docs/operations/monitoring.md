# Terminal queue monitor

A read-only terminal view of PostgreSQL/River state, without another monitoring service, application Docker socket mount or public database port. Counts describe execution jobs, not HTTP requests or unique visitors.

## Run locally

Start/rebuild `bash deploy/scripts/browser-server.sh`, then in another terminal:

```sh
make watch
# Explicit project, optional slower refresh or one snapshot:
python3 deploy/scripts/watch.py --project art-persistent-79509 --interval 5
python3 deploy/scripts/watch.py --project art-persistent-79509 --once
```

Default project selection reads `out/browser-runtime.json`; use the current generated project name. Do not source `local-persistence.sh` for inspection: it creates new secrets. Ctrl-C stops only the monitor.

## Run on Hetzner

After SSHing to the host:

```sh
sudo python3 /opt/art/current/deploy/scripts/watch.py --project singular-seed
```

The operator needs Docker access and Python 3. Specify the production project explicitly so a local smoke/browser manifest cannot select another project. No production resource is provisioned by this command.

## Direct container command

```sh
docker exec -it art-persistent-79509-web-1 /app/artctl watch
docker exec art-persistent-79509-web-1 /app/artctl status
docker exec art-persistent-79509-web-1 /app/artctl status --json
```

`artctl watch --interval 5s` changes its refresh interval. It accesses web's private loopback `/monitor`; Caddy blocks public access. Direct artctl displays recorded hostnames (normally Docker short IDs) or explicit `ART_INSTANCE_NAME`. The host helper resolves those hostnames to **full container names** via `docker inspect` and passes a non-secret name map to artctl. No fixed `container_name` or replica-one hostname prevents scaling.

The helper refreshes discovery every iteration and tries another web replica if one is restarting. It retains up to 1000 observed name mappings during the watch session, so a removed replica remains named if already observed. A container removed before the monitor first sees it can only be shown by its stored hostname/explicit name and boot ID. For future multi-host deployment, names on other Docker hosts require explicit `ART_INSTANCE_NAME` injection or separate discovery; database jobs still appear. `--json` emits raw stored identities, not Docker-resolved names.

## Display semantics

```text
QUEUE  waiting=3  running=2  retrying=0  failed(1h)=1  completed(1h)=80

INSTANCES
INSTANCE                         ROLE      BOOT      BUILD    STATE  SEEN  STORAGE  RUNNING  DONE(1h)
art-persistent-79509-web-1        web       1234abcd  8ce7cd6  live   3s    —        0        0
art-persistent-79509-renderer-1   renderer  5678abcd  8ce7cd6  live   4s    ok       1        42

WORK FLOW
PRODUCER / BOOT                       LAST RENDERER / BOOT                       STATE      JOBS
art-persistent-79509-web-1 / 1234abcd  art-persistent-79509-renderer-1 / 5678abcd  completed  42
```

- Waiting: available/scheduled/pending jobs; retryable jobs are counted separately. Oldest-wait includes both and measures age since original enqueue, not time since last retry.
- Failed (1h): discarded jobs plus deterministic render failures marked `art_terminal_outcome=failed` when River cancels them. It excludes ordinary cancellation/expiry and failed attempts that ultimately succeeded.
- Completed (1h): finalized successful jobs, assigned to the last claimant.
- Instances: boots seen within an hour plus older owners of running work. A 15-second heartbeat becomes stale after 45 seconds; clean shutdown marks stopped. Unique boot suffixes distinguish restarts of the same container. Storage is the latest renderer bucket probe, not a guarantee for every object operation.
- Flow: all outstanding jobs plus those finalized within an hour. Producer is the web boot which originally inserted the execution; coalesced subscriber requests do not rewrite it. Last renderer may be a previous attempt or dead worker, not proof of liveness.
- Legacy jobs show unknown producer; unrecognized historical claimant IDs remain visible. No attribution is invented.

The monitor shows `MONITOR UNAVAILABLE` on failure, retries and does not present stale counts as current. At least one running web admin endpoint and the database are required. This is not independent outage alerting.

## Cost, retention and upgrade

Refresh defaults to two seconds (1–60 seconds configurable). Each snapshot uses a short read-only repeatable-read transaction, released before waiting. No dashboard polling runs after closing the command; service heartbeats continue every 15 seconds. Output caps at 200 instance boots and 200 flow groups with truncation indicated, and strips terminal control characters.

Application migration 2 adds `art_instances` and a render-history index, preserving version 1 and its data. Normal activation runs ordered/checksummed migrations before new binaries. The previous binary's exact schema check rejects version 2; rollback requires a compatible build or roll-forward, not deleting migration history.

Instance records are retained eight days; default display is one hour. River history keeps existing 24-hour/seven-day policies. This is recent operational visibility, not permanent audit history or host CPU/disk/public HTTPS monitoring.

Sources: [snapshot SQL](../../internal/persistence/monitor.go), [identity](../../internal/persistence/instances.go), [CLI](../../cmd/artctl/monitor.go), [host helper](../../deploy/scripts/watch.py), [migration](../../internal/persistence/monitor-schema.sql).
