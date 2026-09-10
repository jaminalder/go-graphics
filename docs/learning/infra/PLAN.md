# Three-stage learning plan

Operate the stack this project already designed. Add machinery only when you
can name the failure it removes. The standard open-source set for that
discipline is small:

| Job | Tool | Why this and not a bigger one |
|---|---|---|
| Cloud objects | Terraform (or OpenTofu) + hcloud | Declares server, firewall, IPs, key; `plan` is the review artefact |
| First boot | cloud-init | Runs once; no extra agent; matches HashiCorp's post-apply guidance |
| TLS and HTTP edge | Caddy | Automatic HTTPS, reload without dropping listeners |
| Process policy | systemd | Supervision, cgroups, sandboxing already on Ubuntu |
| App release | shell scripts + checksums | Immutable directories; rollback is a symlink |
| Host packets | Hetzner firewall + UFW/nftables | Two layers, same allowlist |
| Secrets | env files / systemd credentials, never Git or tf state | Smallest store that is not the VPS itself |
| Seeing failure | journald + one external HTTPS check | Enough until you have an alert without a recipient |

Ansible, Docker, Prometheus, and Kubernetes each solve a *next* problem.
They are not required to make the table above real.

Keep configuration small by **not mixing layers**. Terraform must not SSH in
to restart `artweb`. cloud-init must not contain the app binary. unit files
must not contain the domain secret. CI must not hold production tokens until
you have a dedicated deploy identity. Every extra tool that straddles two
layers becomes a second source of truth.

## How to work a stage

1. Read the matching section below and the questions it names in
   [QUESTIONS.md](QUESTIONS.md).
2. Read the primary sources in [RESOURCES.md](RESOURCES.md) for that stage.
3. Do the work against `deploy/` (worktree today, `master` after integrate).
4. Write answers into QUESTIONS.md and a short
   [learning record](learning-records/README.md) when something non-obvious
   lands.
5. Do not start the next stage because it is interesting. Start it when the
   current stage's exit is true.

Owner-only actions stay owner-only: `terraform apply`, DNS, buying the VPS,
putting secrets on the host.

---

## Stage 1 — one server that is boring under load

**Goal.** A single Hetzner VPS, one domain, a proven firewall, Caddy, two
isolated services, checksummed deploys, backups you have restored. This is
the production architecture in
[security and operations](../../web/security-and-operations.md)
and ADR 0003. Treat it as the whole product runtime, not a toy on the way to
a cluster.

**What you already have** (worktree `deploy/`):

```text
Internet :80/:443
        → Hetzner firewall (22 from admin CIDRs; 80/443 public)
        → Caddy (TLS, body/header limits, private paths 404)
        → artweb 127.0.0.1:8080 (384 MiB, CPUQuota 50%)
        → Unix socket
        → artrender (2 GiB, CPUQuota 150%, AF_UNIX only)
```

Terraform owns the server, firewall, and primary IPs. cloud-init owns users,
sshd, directories, and a disabled-by-default UFW policy. Scripts own
build → smoke on private ports → drain → symlink → restart → rollback.

**Do not add Docker here.** Isolation for this app is already a second user,
a private socket, and a cgroup that can OOM without killing `artweb`. A
container runtime on one VPS would add image builds, a second supervisor, and
a disk budget, without giving you a second machine or a second queue. If you
still want to *learn* containers, do it as a disposable parallel experiment
that must match the same memory split — see [ADDITIONS.md](ADDITIONS.md) —
and keep systemd as what production runs.

**Do not custom-build Caddy for `rate_limit`.** Standard Caddy does not ship
that module. The expensive resource is render CPU; Go admission already owns
that. Teach yourself the three different limits people collapse into the word
“rate limit”:

1. Request shape at the edge (size, timeouts, blocked paths) — Caddyfile.
2. Generation admission (batches/minute, queue, one active job) — `artweb`.
3. Abusive connection noise (scanner floods, SSH guessing) — firewall, and
   later CrowdSec/fail2ban if SSH is not already CIDR-locked.

**Curriculum (in order)**

1. *Read the layers.* Map every file in `deploy/` onto the table at the top.
   Notice what is *not* there: no `remote-exec`, no Docker Compose, no DNS
   resource yet, no Caddy install in cloud-init.
2. *Remote state before a server.* Bootstrap an independent, encrypted,
   versioned object store. Prove two-client lock contention, interrupted
   apply recovery, and restore of a prior state version. If a “S3 compatible”
   bucket fails the lock test, change backend; do not set `use_lockfile=false`.
3. *Plan, do not apply, until you can explain the plan.* Read
   `prevent_destroy`, `delete_protection`, and the fact that changing
   `user_data` can replace the VM. Answer the cloud-init lifecycle question.
4. *Domain and DNS.* One hostname, A and AAAA to the primary IPs, no on-demand
   TLS. Rehearse Let's Encrypt limits on a staging hostname if you will iterate.
5. *Firewall as a pair.* Cloud firewall is the real public filter. UFW is
   enabled only after console recovery works, with SSH limited to the same
   CIDRs. Confirm IPv6 rules exist, not only IPv4.
6. *Bootstrap vs release.* Install pinned Caddy the same way CI verifies it
   (checksum). Put `ART_DOMAIN` / `ART_ORIGIN` in `/etc/art`, not in Git.
   Activate a release with the existing scripts. Watch a failed smoke restore
   `previous=`.
7. *Prove isolation on Linux.* Fill the renderer cgroup; confirm the gallery
   still answers. Reboot; confirm certificates persist. Kill the renderer;
   confirm `/ready` degrades without taking public liveness with it.
8. *Rate limits as tests, not feelings.* Saturate generation; browse; hit
   `/metrics` and private paths from the internet (must 404). Spoof
   `X-Forwarded-For`. Confirm Go limits fire and Caddy timeouts match the
   security document.
9. *Recovery drill.* From a second machine, rebuild using state + a retained
   release + Caddy storage. Time it. That number is your availability story.
10. *One external eye.* Gallery HTTPS check with a named recipient and a
    three-line runbook. journald stays bounded. No Prometheus until an alert
    has gone off and you wished for a graph.

**What you will learn**

- The difference between declaring cloud objects and configuring a Unix box.
- Why provisioners make Terraform un-plannable, and why that pushed this repo
  into scripts.
- TLS as a directory you back up, not a certificate you paste.
- cgroups as the actual isolation primitive Kubernetes would use later.
- That “secure” is a list of demonstrated refusals (wrong host, too-big body,
  GET does not render, renderer OOM is local), not a scanner score.
- How little YAML you need when the OS already has a supervisor.

**Exit.** You have a staging host you can destroy and recreate; launch-gate
infra rows have evidence; you can teach the request path from memory; you
have written answers for every stage-1 question. Containers are still
optional homework, not the runtime.

---

## Stage 2 — zero downtime and scale without Kubernetes

**Goal.** Decide, with measurements from stage 1, which of these you actually
need: deploys that do not drop Caddy, deploys that do not drop in-flight
studio sessions, a bigger box, a second box, or automatic capacity. Kubernetes
is still off the table. The honest ceiling of this stage is **a small fleet
you can SSH to**, not infinite scale.

**The constraint this app imposes.** `artweb` owns the only queue and the
workspaces in memory. The security document already warns: starting a second
independently queued web process on the same host **doubles admission and
memory**. Zero-downtime is therefore not “run two containers and a load
balancer”. It is one of:

| Move | What stays up | What you pay |
|---|---|---|
| Current drain + restart | Caddy, existing image bytes | In-memory explorations die; a few seconds of 502/retry |
| Socket activation / process replace | The listening socket | Still one process; in-memory state still dies unless you snapshot it |
| Two local ports, Caddy `reverse_proxy` swap | Browse, if the new process is up first | Easy to run two queues; must pause admission on the old one and *not* admit on the new until swap |
| Split renderer onto host B | Web deploys without touching the worker | The Unix socket becomes a private TCP/WireGuard protocol; still one owner of the queue |
| Persist workspaces (SQLite) | Sessions across web restart | First real database; contradicts v1 “no DB” until you have a reason |
| Bigger VPS | Everything, same topology | Money; still one failure domain |
| Hetzner load balancer + two webs | Host death | Shared admission, shared cache, sticky sessions or a store — this is the fork where k8s starts to look cheaper *in complexity*, not in euros |

Caddy reload is already near-zero-downtime for TLS and routing. The gap is
the Go processes. Learn that distinction before installing a scheduler.

**Curriculum (in order)**

1. *Measure the restart you already have.* Time `activate-release.sh` from
   drain start to `/ready`. Count lost workspaces. That is the SLO you are
   improving, not a generic “99.99%”.
2. *Zero-downtime for the edge only.* Confirm Caddy `reload` never drops
   :443. Put config that can change without an app restart (headers, HSTS
   after HTTPS is proven) in Caddy, not in Go.
3. *One-host rolling web.* Implement a rehearsal: start the candidate on
   :8180 (the smoke path already does this), flip Caddy upstream, then stop
   the old process. **Admission must remain global.** If you cannot do that
   without two queues, you have found the application change stage 2 needs
   (a lock file, a shared counter, or “only the active upstream admits”).
4. *Renderer independently of web.* Restart `artrender` while Caddy+artweb
   stay; jobs fail explicitly. This is the cheap decoupling. It is also the
   seam for a second machine.
5. *Vertical scale as a Terraform change.* CX23 → CX33 is a plan you can
   read. Re-run the 100-seed benchmark. Learn `prevent_destroy` vs resize
   vs replace.
6. *Manual horizontal: dedicated renderer.* Second CX, private network or
   WireGuard, no public :80 on the worker. Terraform grows by one server and
   one firewall. systemd units stay. This is the furthest typical small shop
   needs to go for a CPU-bound generator.
7. *Autoscaling without a cluster.* Hetzner will not scale you like AWS ASG.
   The realistic automation is: metrics → alert → `hcloud server create`
   from the same image/user-data, or a scheduled bigger type for known
   peaks. Write a script only after a human has done the same steps twice.
   If the script needs a consensus store and a service registry, you are in
   stage 3 territory.

**What you will learn**

- Availability is a property of *state*, not of YAML. Stateless Caddy is easy
  to keep up; stateful `artweb` is not.
- Load balancers multiply processes; they do not merge queues.
- Most “we need Kubernetes to scale” stories for this workload are actually
  “we need one more VPS and a private protocol”.
- Autoscaling is a policy (when, from what image, with which secrets, how
  traffic finds the new node). The policy is the hard part; `kubectl` would
  not write it for you.

**How far can you go without Kubernetes?**

Far enough for this product, probably permanently:

- One VPS: the designed production.
- One VPS, rolling Caddy + drain: good enough if visitors can retry a batch.
- Two VPS (web | renderer): when CPU/RAM fight each other on CX23.
- N renderers behind one `artweb`: when the queue wait is the SLO you miss
  and you are willing to fan the existing supervisor protocol out over a
  private network.
- Two web replicas: only with shared admission and either sticky sessions or
  stored workspaces. That is the first time a scheduler's *standard* rolling
  Deployment matches the problem.

You do not get multi-AZ failover, fancy ingress, or a Helm ecosystem. You also
do not spend a vCPU on etcd. For a free art studio on a €7–15 host, that is
the correct trade until stage 1's restore drill is slower than you can
tolerate *and* you have two humans operating it.

**Exit.** You have numbers for restart loss, a written choice among the table
above, and either a working one-host roll or a documented reason you kept
drain-and-restart. Kubernetes is still a learning lab, not a migration.

---

## Stage 3 — Kubernetes as a second language

**Goal.** Understand what the control plane buys, by running a **throwaway**
k3s (or kind) copy of the studio that is not the public host. Migrate
production only if stage 2 left a problem Kubernetes uniquely solves.

**What Kubernetes would add, mapped to this app**

| Benefit people cite | On this studio |
|---|---|
| Rolling updates | Needs 2+ ready replicas and no double queue — app change first |
| Self-heal | systemd `Restart=` already does this on one node |
| Resource limits | You already have `MemoryMax` / `CPUQuota`; k8s would wrap the same cgroups |
| Service discovery | Replaces a Unix socket path with DNS and NetworkPolicies |
| Horizontal Pod Autoscaler | Useful only after the queue is a cluster-wide object |
| Declarative desired state | You already have this for cloud objects in Terraform; k8s adds it for *processes* |
| Ecosystem (ingress, cert-manager, Helm) | Replaces Caddy-on-the-host with a pile of YAML that does the same TLS |

**Curriculum**

1. Read [k3s architecture](https://docs.k3s.io/architecture). Install k3s on
   a **separate** CX or a local VM. Measure idle RAM before any app pod.
   That number is the tax.
2. Wrap `artweb` and `artrender` as two Deployments, one ClusterIP, one
   NetworkPolicy that is as strict as `RestrictAddressFamilies=AF_UNIX`.
   Put Caddy or ingress-nginx in front. Compare the file count to `deploy/`.
3. Attempt a rolling update. Watch whether you now run two admissions. Fix
   that in the app or admit that k8s did not give you zero-downtime for free.
4. Only then look at a managed Kubernetes (Hetzner CKS, Civo, a tiny EKS).
   Price it against two CX33s. Include your time.

**What you will learn**

- Kubernetes is an API for desired process state, not a requirement for
  HTTPS or firewalls.
- On one node it is a more expensive systemd. Its value appears at *many
  heterogeneous services* or *many operators*, which this repo does not have.
- The case for the VPS is won or lost in stages 1–2. Stage 3 is literacy so
  you can reject (or accept) a cluster with specifics.

**Exit.** A short written comparison: file count, RAM tax, rolling-update
behaviour, restore story. Production stays on systemd unless that comparison
names a failure you are willing to pay for.

---

## Suggested weekly rhythm

Do not binge stages. A useful week is one layer:

- Week of state/DNS/firewall (no public app).
- Week of Caddy + first release (staging hostname).
- Week of isolation and load (the proofs).
- Week of restore and monitoring.
- Only then a zero-downtime experiment on staging.

If a week produces a new YAML stack instead of a proof (lock, OOM, restore,
reload), you drifted into collecting tools. Return to the exit criteria.
