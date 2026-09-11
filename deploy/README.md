# Operating Singular Seed with Docker Compose

The chosen public domain is `singularseed.art`. The runtime is one Ubuntu VPS
on a CX23 in Nuremberg (`nbg1`), with three Compose services: Caddy, `web`
(`artweb`), and `renderer` (`artrender`). The owner selected Compose and a CX23
x86-64 target on 2026-09-10; see
[ADR 0004](../docs/adr/0004-compose-runtime.md). Cloud provisioning, DNS and
public launch remain operator actions. The current host serves HTTP at its
assigned IPv4; a full rebuild rehearsal and domain/TLS setup are next.

## Local rehearsal

Requires Docker Engine with Compose v2.24+ (tested locally with Compose 5.4.0).
On macOS, Docker runs inside a Linux VM such as Colima or Docker Desktop.
From the repository root:

```sh
docker compose -p art-local --env-file deploy/local.env -f deploy/compose.yaml build web caddy
docker compose -p art-local --env-file deploy/local.env -f deploy/compose.yaml up -d --no-build --wait
```

Open **http://localhost:8088**. Use that exact hostname: the app validates its
canonical origin. The local configuration publishes only loopback ports and
uses HTTP, so it does not request certificates. Stop with the same command
prefix followed by `down`; volumes survive. `down --volumes` intentionally
removes that project's cache/socket/TLS data and is only for disposable labs.

`make check-compose` runs an isolated rehearsal, checks actual resource limits,
private paths, four rendered PNGs, and renderer failure/recreation. It removes
only its temporary project's volumes. Native Go development remains available:
run `go run ./cmd/artrender` and `go run ./cmd/artweb` in separate terminals.
The defaults remain loopback HTTP, `out/artrender.sock`, and `out/cache`.

## Runtime and ownership

```text
Internet :80/:443 -> Hetzner firewall + Docker-aware host filtering
  -> Caddy container -> internal proxy network -> web container :8080
    -> shared Unix socket -> renderer supervisor -> disposable child process
```

| Layer | Source of truth |
|---|---|
| VPS, IPs, firewall, SSH public key | `terraform/` |
| Host users, SSH, journals, OS update policy | `cloud-init/user-data.yaml` |
| Docker installation | `scripts/bootstrap-host.sh` |
| Binaries and pinned base images | `Dockerfile` |
| Processes, cgroups, networks, mounts, logging | `compose.yaml` |
| Edge HTTP/TLS behavior | `caddy/Caddyfile`, baked into the edge image |
| Release identity and rollback | `scripts/build-release.sh`, `activate-release.sh` |
| Host configuration and deployment record | Root-owned `/etc/art/*`, outside Git |

The web process owns the only queue, temporary workspaces and image cache.
The renderer accepts one active job, starts a fixed child executable without a
shell, and returns bounded image bytes. The web container has a 384 MiB memory
limit and half a CPU; the renderer has 2 GiB and 1.5 CPUs, including children.
Each has 64 tasks maximum, no swap budget, a read-only root filesystem, no Linux
capabilities and `no-new-privileges`. Users are web `10001:10000`, renderer
`10002:10000`, and Caddy `10003:10003`. Caddy has a 256 MiB/half-CPU ceiling.
These are starting budgets, not proven target-host capacity.

Only Caddy publishes ports. It joins an external network for ACME and an
internal network for proxying. The web container joins only that internal
network. `ART_TRUSTED_PROXY` is the single fixed Caddy IP (`172.30.80.2` by
default); other peers cannot supply `X-Art-Client`. Caddy overwrites that header
and removes visitor forwarding headers. `ART_PROXY_NET` changes the private
IPv4 prefix for both Compose and proxy trust; reserve a nonconflicting /29.
The renderer uses `network_mode: none`, and shares only `/run/art` with web.
Web mounts that socket directory read-only. Neither receives the Docker socket,
cloud credentials or host home directories. Admin HTTP remains loopback inside
web; use `docker compose exec`, never publish 8081 or 2019.

Container health checks do not render. Web health is gallery liveness, separate
from `/app/artctl ready`, which checks renderer compatibility. Docker restart
policies restart exited containers; an unhealthy status alone does not trigger
a restart. There is no startup dependency that prevents browsing when rendering
is unavailable. Container shutdown kills remaining children. Prove OOM behavior
on the target: a healthy laptop rehearsal cannot establish VPS capacity.

## Prepare the production host

Follow [the Terraform provisioning workflow](terraform/README.md). It now creates
and deploys the single production environment in one apply, using a retained
release built locally.
Cloud-init installs Docker and enables UFW; Terraform uploads and activates the
release over SSH. Production initially uses HTTP at the server IP, without DNS.
`install-release.sh` writes the runtime configuration and `deployment-approved`
record selected by that apply; no manual launch-approval file is required.
Adding the domain later changes the origin to HTTPS on this same server.

The Hetzner firewall is the public perimeter for both host and Docker traffic.
UFW controls host INPUT; Docker-published ports can bypass those rules. Compose
publishes only 80/443 through Caddy. Keep the cloud firewall attached and do not
publish additional services without updating this policy.

Bootstrap pins Docker 29.8.0, Compose 5.4.0, containerd 2.3.5 and Buildx 0.37.0
from Docker's signed Ubuntu repository. Image builds pin Go 1.26.8 and Caddy
2.11.4 by digest in `Dockerfile`. Review and test updates deliberately; the OS
security-update policy disables automatic reboot. Schedule host/runtime restarts
and monitor overdue updates. Bootstrap is intended for a new host, not an
unreviewed runtime upgrade on a busy server.

## Build, activate, roll back

From a clean committed checkout:

```sh
deploy/scripts/build-release.sh
```

This builds Linux amd64 images and writes `out/releases/<full-commit>/` with
`images.tar`, immutable image IDs in `images.env`, versioned deployment files,
source/tool/edition manifest, catalogue and SHA256 checksums. No registry is
required: upload the complete directory over the operator's SSH connection to
`/opt/art/releases/<full-commit>`. Keep images/archives for retained artwork
editions, including at least the previous working release. The archive checksum
is meaningful only when delivered through that trusted operator channel.

Terraform apply runs installation and activation automatically. For a manual
update on an initialized host, run the uploaded release’s
`deploy/scripts/activate-release.sh <full-commit>` as root.
It takes an exclusive deployment lock, checks the deployment record and
archive checksums, loads images, validates Compose/Caddy, then rehearses a private
candidate with generation disabled. The smoke project uses its own network and
volumes and no published ports. It cannot add a second active rendering queue.
After smoke passes, activation pauses admission and requires a confirmed empty
queue; missing metrics or a drain timeout aborts the change. It stops old web
and renderer, switches `/opt/art/current`, recreates services from verified image
IDs, checks readiness/liveness, and checks the canonical HTTP(S) URL.

Failure restores the previous release's images/configuration and verifies its
renderer readiness. First-install failure removes the candidate containers and
current pointer while preserving volumes. Inspect the exit status, logs and
`previous=` message; failure to recover needs operator intervention. Manual
rollback is activation of the retained previous commit through the same script.
Never use moving tags such as `latest` as production release identity.

The single fixed host project is `singular-seed`. Do not start another public
project or scale `web`: that would create another admission queue. A brief
restart is expected. In-memory explorations/jobs are lost. Caddy may also be
recreated when its release image changes; Compose is not a rolling-deploy system.
Deploy only when generation should resume: recovery from a failed drain reopens
admission on the old release; an incident kill switch must be handled separately.

## Private operations

Use the release wrapper so every command uses the same project/config/images:

```sh
sudo /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current ps
sudo /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current logs --tail 100 web renderer
sudo /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl metrics
sudo /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl ready
sudo /opt/art/current/deploy/scripts/compose-release.sh /opt/art/current exec -T web /app/artctl generation-off
```

`generation-on` resumes admission after recovery. `/app/artctl live` checks the
web process without requiring rendering. A stopped renderer must degrade ready
while the gallery and existing downloads remain available. For a failed job,
correlate `job=` in web and renderer logs. Logs omit cookies, raw recipes and
headers. `ART_LOG_LEVEL=debug` is temporary diagnostic configuration.

Compose uses the bounded `local` log driver (three 10 MiB files per service).
Host journald retains 256 MiB/seven days. Inspect `docker stats`, container
`State.OOMKilled`, health and restart counts, cgroup memory events, and host free
disk. An OOM child can fail without the supervisor exiting, so container restart
count alone is insufficient. Configure an external gallery HTTPS check, recipient,
escalation hours and a short runbook; track disk reserve, restart loops and TLS
trouble. Keep metrics private.

## Volumes, backups and recovery

| State | Lifetime / recovery |
|---|---|
| `singular-seed_caddy_data`, `singular-seed_caddy_config` | Retain and back up account/certificate/config state with correct ownership |
| `singular-seed_cache` | Disposable generated images, only web can write; app enforces cache bounds |
| `singular-seed_socket` | Runtime socket only; may recreate when services are stopped |
| `/etc/art`, release archives/checksums, Terraform state | Independent encrypted backup outside the VPS |
| Workspaces/jobs in web memory | Lost on restart; no durable-session promise |

A volume surviving container replacement is not a backup. Never run production
`down --volumes` or indiscriminate Docker prune commands. Review release/image
retention and free disk before upload; keep previous archives and any renderer
needed for retained editions. Restore TLS volumes while Caddy is stopped, then
verify permissions, certificates and renewal. Do not store the only copy of
state or recovery credentials on the managed VPS.

Proposed recovery objective: two hours, still to be measured. From a second
machine, recover backend access/state, rebuild an approved host, install Docker,
restore operator files and TLS volumes, load a retained release and activate it.
Review IP/DNS lifecycle, verify HTTPS, render and download an image, reboot and
check again. Record operator, elapsed time, state version, image IDs, release and
gaps. Linux amd64 OOM, reboot, dual-stack firewall, live TLS/renewal, state locking
and clean-host restore remain target-host launch gates.
