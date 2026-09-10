# Suggested additions to the infrastructure implementation

**Not an implementation plan.** Do not merge this into `docs/web/*` or patch
`deploy/` from the learning track. When the owner next revises Phase 5, these
are the gaps and forks worth considering. Ordered by how much they help
**stage 1** of [PLAN.md](PLAN.md) without changing the product topology.

The current worktree already has the right shape: ~70 lines of Terraform,
cloud-init without secrets, two sandboxed units, a small Caddyfile, and
checksummed activate/rollback. The additions below are mostly *closing loops
that are still prose*, not new products.

## Keep as-is (do not “fix” these)

- **No Docker / Compose / k8s in v1.** Isolation is cgroups + users + a Unix
  socket. A container runtime would not remove a v1 requirement.
- **No Caddy `rate_limit` module.** Application admission is the expensive
  limiter; standard Caddy should stay standard.
- **No Terraform `remote-exec`.** Releases stay scripts. This matches
  HashiCorp's provisioner guidance.
- **Brief drain-and-restart.** Honest given in-memory workspaces. Do not
  pretend a second local `artweb` is a rolling deploy until admission is
  shared.
- **UFW not enabled in cloud-init.** Locking yourself out on first boot is
  worse than a delayed host firewall.

## Stage 1 — small, high-leverage closes

### 1. Pin Caddy install as a script, not README prose

CI already downloads Caddy 2.11.4 and checks a SHA-512. The host path is
still “operator installs the binary”. Add `deploy/scripts/install-caddy.sh`
that uses the same URL and digest, installs the upstream unit, and stops.
cloud-init can call it or the operator can. Either way the checksum is the
source of truth, once.

### 2. One host-side bootstrap script for the remaining owner steps

After `terraform apply`, the README still lists: domain env files, Caddy
drop-in `EnvironmentFile`, UFW SSH allow + enable, launch-approval file.
A `bootstrap-host.sh` that is *idempotent*, prints every command, and refuses
to enable UFW unless an `--i-can-use-the-console` flag is set would turn a
wiki into an auditable run. Use the `wizard` skill only for the credential
and DNS clicks that cannot be scripted.

### 3. Configure `unattended-upgrades` without surprise reboot

The package is in cloud-init; the policy is not. Decide automatic security
updates vs automatic reboot (question 12) and encode it. A 4 GB single VPS
that reboots at 06:00 UTC *is* the availability story.

### 4. Terraform: variable environment, DNS once the domain exists, ignore or replace `user_data`

- `labels.environment = "staging"` is hardcoded; production needs a variable.
- When the hostname is chosen, a tiny DNS resource (Hetzner DNS or Cloudflare
  provider) removes a manual A/AAAA drift source — still owner-approved apply.
- Document, in `deploy/terraform/README` (new file), the `user_data` replace
  hazard. Either `ignore_changes = [user_data]` after first boot or treat
  bootstrap as immutable and rebuild. Pick one; today it is implicit.

### 5. Remote-state bootstrap as its own tiny root

`backend.hcl.example` is correct (`encrypt`, `use_lockfile`) but the bucket
does not create itself. A separate `deploy/terraform/state/` root (or a short
owner runbook with the exact console/API steps) is the missing chicken-and-egg
piece. Keep it independent of the VPS. Record the two-client lock test as a
script you run, not a checkbox in launch-gates.

### 6. Release retention on the host

`activate-release.sh` keeps “at least previous” by social convention. CX23
has 40 GB. Add prune-keep-N (binaries + checksums) and a disk check before
unpack. Do not delete `/var/cache/art` from that script.

### 7. `MemoryHigh` plus `MemoryMax` on the renderer

systemd's own docs treat `MemoryHigh` as the throttle and `MemoryMax` as the
last line. Units today only set `MemoryMax=2G`. A High/Max pair is more
likely to degrade the job than to surprise-OOM under burst, and it is still
one unit file.

### 8. Tighten the operator user after first login

cloud-init gives `NOPASSWD:ALL`. That is bootstrap convenience. A later
hardening step: remove passwordless sudo, or limit it to the deploy scripts
via sudoers. Do not put your daily laptop SSH key and unlimited sudo on the
same sentence forever.

### 9. HSTS as a commented Caddy block

The security doc wants HSTS only after HTTPS is proven, without
`includeSubDomains`/preload by default. A commented snippet in the Caddyfile
(or a drop-in) makes that a one-line flip instead of a remembered wiki edit.

### 10. Write-timeout vs download path

Global Caddy `write 30s` may cut a slow but valid download. Before launch,
either raise the write timeout on `/downloads/*` or prove 30s is enough on
the VPS. This is a config gap, not a new proxy.

### 11. External uptime as an operator inventory file

Not code: a root-owned `/etc/art/operator.env` or an off-host note naming the
uptime vendor, URL, recipient, and the four alerts from the README. Phase 5
already requires this; it is still easy to ship the app without it.

## Stage 1 — optional learning forks (keep off production until justified)

### A. Containers as a **parallel** experiment

If the goal is to learn containerization, add a worktree experiment:
Podman *quadlets* (systemd units that start pods) wrapping the same two
binaries, same users, same `MemoryMax`. Success = renderer OOM still does
not kill `artweb`. Failure = Compose file that shares a cgroup and a
network namespace “for simplicity”. Do not replace `deploy/systemd/` until
the experiment wins on a written criterion (question 11).

Avoid Docker-in-production-on-one-box as the default: it adds a daemon you
must update, while Podman/quadlets at least stay inside the systemd model
you already operate.

### B. CrowdSec or nftables rate at the packet layer

Only after SSH is CIDR-scoped and Go admission is proven. CrowdSec is the
modern open-source choice if you want shared abusive-IP lists; nftables
`limit` is smaller. Neither replaces application admission.

### C. WireGuard for admin SSH

Replaces “update `admin_cidrs` every time the laptop leaves home”. Adds a
key to back up. Good stage 1.5 if question 8's answer is “my IP is not
stable”.

## Stage 2 — do not add until stage 1 exits

These would be real implementation-plan changes later; they fight v1 if
done now.

1. **Caddy upstream flip** (`127.0.0.1:8080` ↔ `:8180`) with a global
   admission lock so two `artweb` processes cannot both enqueue.
2. **Renderer protocol over a private TCP/WireGuard socket**, keeping the
   “one active job” supervisor. That is the two-VPS split.
3. **SQLite (or similar) for workspaces** only if question 16 forbids losing
   sessions. That is a product change, not a unit-file change.
4. **Hetzner Load Balancer resource in Terraform** only with a written
   admission story.
5. **CI deploy identity** (OIDC → something that can SSH or call hcloud)
   only after laptop-upload is boring and question 15 is decided.
6. **Prometheus/Grafana** only after an alert has fired and you needed
   history. journald + `/metrics` on loopback is the v1 stack.

## Stage 3 — lab only

A `deploy/k3s/` (or a sibling worktree) with two Deployments and a
NetworkPolicy, **not** pointed at the production IPs. Use it to answer
questions 24–26. Promoting it would be a new ADR that explicitly supersedes
ADR 0003.

## Doc/process nits (safe whenever)

- Add a `deploy/README` section that states the layering rule in one
  paragraph (Terraform / cloud-init / systemd / scripts) so future agents do
  not “help” by putting `apt install docker` in `user-data`.
- Mention this learning track from `docs/web/README.md` *when you next edit
  that file* — a single line under reading order. Not done now, to avoid
  conflicting with the implementation worktree.
- After integrate, copy nothing from this folder into `deploy/`. Learning
  records stay here.
