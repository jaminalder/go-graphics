# Three-stage learning plan

The owner selected Docker Compose as the runtime on 2026-09-10. This replaces
the earlier systemd-first curriculum; see [ADR 0004](../../adr/0004-compose-runtime.md).
The application still runs on one VPS with one admission queue.

| Responsibility | Tool and file |
|---|---|
| Cloud objects and state | Terraform, `deploy/terraform/` |
| First boot | cloud-init, `deploy/cloud-init/user-data.yaml` |
| Host container runtime | `deploy/scripts/bootstrap-host.sh` |
| Packaged application and edge | `deploy/Dockerfile` |
| Process/resource/network/storage policy | `deploy/compose.yaml` |
| HTTPS and request shape | Caddyfile, baked into the edge image |
| Releases | checksummed image archive + smoke/drain/activate/rollback scripts |
| Visibility | bounded Docker logs, host journald, private metrics, external HTTPS check |

Begin with the [visual atlas](index.html) and its request, build and release
lessons; use this plan for the longer operational labs.

Do each exercise as: predict, run, inspect, explain. Answer the matching
[questions](QUESTIONS.md) and write a learning record when an observation changes
your understanding. Keep learning artifacts in `docs/learning/infra/` and follow
the repository’s current worktree workflow for edits. No exercise authorizes cloud spending, DNS
or publication.

## Stage 1 — one containerized server you can recover

**Goal:** operate the real three-container application on staging, demonstrate
its failure boundaries, and recover it from independently retained state.

### 1. Local containers before a server

Read Dockerfile and Compose alongside the [runbook](../../../deploy/README.md).
Build and start the local stack; load http://localhost:8088. Identify the three
containers, two images, internal bridge and published ports. Trace a real batch
through web, Unix socket, renderer supervisor and child back to cached PNGs.

Learn image vs container, build stage vs runtime stage, image tag vs immutable
ID/digest, process vs VM. Inspect the static application image: the compiler,
shell, browser test dependencies and credentials are absent. On macOS the
containers run in a Linux VM; native ARM rehearsal is not an amd64 VPS benchmark.

**Exercise:** explain what `build`, `up`, `stop`, `restart`, recreation and `down`
change. Rebuild an image and show why an existing container has to be replaced
to run it. Observe how restart differs from rebuilding source.

### 2. Networks, permissions and volumes

Caddy publishes HTTP/HTTPS and connects to web over an internal bridge. The
web container trusts one Caddy IP and keeps admin HTTP on its own loopback.
The renderer has `network_mode: none` and a shared group-owned Unix socket.
Web mounts the socket directory read-only and alone writes the cache.

**Exercise:** inspect network membership, port bindings, numeric users and
mounts. Attempt public access to `/metrics` and `/ready` (404). Explain why
`localhost` inside one container is not another container. Identify which
volumes survive recreation and what `down --volumes` would destroy. Never run
that command against a production project.

Learn filesystem layers, mounts, UID/GID ownership, namespaces and proxy trust.
Separate Caddy TLS state, disposable cache/socket and in-memory workspaces.

### 3. Resource limits and observable failure

Inspect actual Docker/cgroup memory, swap, CPU and task limits. Stop renderer:
web liveness and gallery must stay up while readiness fails. Recreate renderer
and confirm recovery without creating a second queue. `make check-compose`
automates local boundaries and real rendering.

**Exercise:** follow a job in both logs; inspect health and restart status.
Explain why an unhealthy container does not automatically restart. On a
throwaway Linux environment, drive an actual renderer OOM and inspect cgroup
memory events. A YAML limit is not an OOM proof. Check target-host OOM again
before launch, together with gallery responsiveness and resource recovery.

Learn how namespaces and cgroups differ, which limits include children, and
why one container per rendering request or a mounted Docker socket is unnecessary.

### 4. Remote state before cloud resources

Answer questions 1–5, including DNS ownership, total cost ceiling and operator
machine. `singularseed.art` is chosen; DNS ownership/access and a staging hostname
still need confirmation. Existing tooling uses Terraform; a switch to OpenTofu
would be a separate decision.

Bootstrap an independent encrypted/versioned backend. Use a disposable state
key to prove two-client lock contention, interrupted operation recovery and
restoration of a prior state version. Never disable locking for a backend.

Learn Terraform's resource mapping, plan vs apply, state vs configuration,
locking vs object retention, and why the managed VPS cannot hold the only copy
of its own recovery state.

### 5. Host bootstrap and firewall

Read a saved Terraform plan before an approved apply. Explain primary-IP
retention, deletion protection and how immutable cloud-init changes can require
replacement. After provisioning, install pinned Docker with bootstrap-host.sh.
Cloud-init contains no app images or credentials; app updates never use Terraform.

Prove console recovery before enabling UFW. Match SSH administrator CIDRs in
cloud/host rules. Inspect Docker's published-port forwarding: UFW INPUT rules
alone do not protect that path. Configure DOCKER-USER policy for the chosen
iptables backend and test public refusal over IPv4 and IPv6 after reboot.

Learn first-boot vs ongoing configuration, signed package repositories,
Docker's daemon privilege, NAT/forwarding vs host input and deliberate updates.

### 6. Staging HTTPS and release operations

Use an owner-controlled staging hostname and correct origin. Configure TLS
through Caddy; retain its account/certificate volumes. Use ACME staging when
iterating certificate setup. Record staging authorization separately from
public-launch approval.

Build a committed Linux amd64 release. Read its archive checksums and immutable
image IDs. Upload it using the operator identity, then run the activation script.
Watch private candidate smoke, generation drain, replacement, readiness and
external HTTPS checks. Cause a bad candidate or readiness failure; observe
rollback and verify the actual images running afterward.

Learn image distribution without requiring a registry, trusted artifact delivery,
release identity, drain vs restart and the loss of in-memory explorations.
Compose alone does not coordinate a zero-downtime deployment.

### 7. Recovery and one external alert

Measure real allowed artwork configurations on the target host, including
saturated rendering while browsing/downloading. Verify timeout/body bounds,
forwarded-header spoofing, low disk, stop/start, OOM, and certificate persistence.

From a second machine, restore backend access/state, rebuild the host, restore
operator files/TLS volumes, load retained images and activate. Time the drill;
two hours is a proposed objective, not a demonstrated result. Connect an external
gallery HTTPS check to a named recipient with a short runbook. Bound Docker logs
and journald, monitor disk and restart loops, schedule security-update reboots.

**Stage-1 exit:** staging deployment, rollback, OOM isolation, reboot, dual-stack
firewall, state locking and clean-host restore have recorded evidence; questions
1–15 have answers. You can explain the path and recover it without undocumented
laptop files. Production launch still requires its own gates and owner approval.

## Stage 2 — availability and scale with Compose

Start from measurements of restart downtime, lost explorations, CPU, memory
and queue wait. Decide which property needs improvement before adding replicas.

| Change | What it addresses | Constraint |
|---|---|---|
| Keep drain/recreate | Small operational surface | Brief outage and lost workspaces |
| Reload unchanged Caddy separately | Edge config continuity | Does not preserve web state |
| Larger VPS | CPU/RAM pressure | Same failure domain; remeasure |
| Two web containers with upstream switch | Potentially smoother deploy | Must preserve one global admission and address workspace loss |
| Dedicated renderer host | Separates CPU capacity | Unix socket becomes an authenticated private-network protocol |
| Multiple renderers | Queue wait | One queue owner needs explicit dispatch and capacity accounting |
| Durable workspaces | Sessions across restarts | Real application persistence change |

**Exercises:** measure the existing restart; rehearse a routing change; stop and
restart only renderer; compare vertical sizing using target measurements. Do
not use `compose --scale web=2` as a substitute for a shared-admission design.
Define autoscaling as an observable threshold and action before automating it.

**Learn:** image portability does not create shared state, load balancing does
not merge queues, and recovery/availability are application properties as well
as infrastructure properties. A single Docker host is still a single host.

**Exit:** measured availability loss, a written choice, and a demonstrated
improvement or a justified decision to keep drain/recreate. Answers 16–22.

## Stage 3 — Kubernetes as a second operating language

Use a separate disposable kind/k3s environment. Reuse the same application
images and map Compose services, health checks, mounts, resource limits and
network policy to Kubernetes. Measure control-plane RAM and configuration cost.
Investigate whether Caddy belongs alongside the application or behind another
ingress; do not copy ingress examples without preserving proxy trust.

Try a rolling update and identify how admission and workspaces behave. Shared
pod namespaces/volumes, Deployments, Services, probes, storage and scheduler
placement are new concepts, not reasons to change the production runtime.
A NetworkPolicy alone is not identical to disabling renderer networking.

**Learn:** which management work a scheduler supplies, which state and protocol
problems still belong to this app, and when multiple hosts/operators justify it.

**Exit:** answers 23–27 plus measured file count, RAM, update behavior and restore
story. Production stays on Compose unless a new owner-approved decision names
a concrete unmet requirement.

## Suggested rhythm

One layer per session: local images; network/storage; limits/failure; remote
state; host/firewall; TLS/releases; restore/monitoring. Begin with a prediction
and finish with an observation, not a pile of new tooling.
