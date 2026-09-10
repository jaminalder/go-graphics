# Small-shop VPS resources

Prefer these over blog posts that assume every service is a container in EKS.
Read the contract for the tool you are about to run; then run it against this
repository's `deploy/` files.

## Knowledge

- [Terraform provisioners](https://developer.hashicorp.com/terraform/language/provisioners)
  Official last-resort guidance. Use for: why this project deploys with
  scripts and cloud-init instead of `remote-exec`.
- [Terraform post-apply operations](https://developer.hashicorp.com/terraform/language/post-apply-operations)
  Cloud-init and images as the supported path. Use for: the Layer 1 / Layer 2
  split in [PLAN.md](PLAN.md).
- [Terraform S3 backend](https://developer.hashicorp.com/terraform/language/backend/s3)
  Lockfile, versioning, and the Amazon-S3 compatibility warning. Use for:
  proving remote state before the first real apply.
- [Terraform sensitive data](https://developer.hashicorp.com/terraform/language/manage-sensitive-data)
  `sensitive` hides UI output, not state bytes. Use for: what must never enter
  `user_data` or tfvars.
- [hcloud provider](https://registry.terraform.io/providers/hetznercloud/hcloud/1.68.0/docs)
  The pinned provider this implementation used. Use for: server, firewall,
  primary IP, user_data fields you are about to change.
- [cloud-init documentation](https://cloudinit.readthedocs.io/en/latest/)
  First-boot contract. Use for: what `user-data.yaml` may safely do, and why
  changing it can replace a server.
- [Caddy automatic HTTPS](https://caddyserver.com/docs/automatic-https)
  Certificate issuance and HTTP redirects with no extra ACME client. Use for:
  domain cutover and why Caddy's data directory is a backup target.
- [Caddy reverse_proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
  Upstream timeouts and forwarded-header behaviour. Use for: why the Caddyfile
  strips `X-Forwarded-*` and sets `X-Art-Client`.
- [Caddy `rate_limit` module status](https://caddyserver.com/docs/modules/http.handlers.rate_limit)
  Not in standard Caddy. Use for: the rate-limit question in
  [QUESTIONS.md](QUESTIONS.md).
- [systemd.resource-control(5)](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html)
  `MemoryMax`, CPU quota, OOM policy. Use for: proving renderer isolation.
- [systemd.exec(5)](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html)
  `ProtectSystem`, `RestrictAddressFamilies`, credentials. Use for: reading
  `artweb.service` / `artrender.service` as a security policy, not boilerplate.
- [Hetzner Cloud servers](https://docs.hetzner.com/cloud/servers/overview/)
  Product facts. Use for: CX sizing, backups vs snapshots, rebuild protection.
- [Hetzner firewalls](https://docs.hetzner.com/cloud/firewalls/overview)
  Cloud-level packet filter. Use for: IPv4+IPv6 rules and why UFW is defence
  in depth, not a duplicate source of truth you can ignore.
- [Hetzner backups and snapshots](https://docs.hetzner.com/cloud/servers/backups-snapshots/overview/)
  Seven backup slots, tied to the server. Use for: why they are not the only
  recovery copy.
- [Let's Encrypt rate limits](https://letsencrypt.org/docs/rate-limits/)
  Certificate issuance budgets. Use for: staging vs production domain tests.
- [OpenTofu](https://opentofu.org/docs/)
  Open-source Terraform-compatible CLI. Use for: the licensing question, not
  as a required rewrite.
- [k3s architecture](https://docs.k3s.io/architecture)
  Single-server vs HA control plane. Use for: stage 3, after you can name a
  problem k3s would solve on this app.
- [Kubernetes documentation: Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
  What a rolling update actually requires (more than one ready replica, shared
  nothing or shared storage). Use for: mapping zero-downtime to this app's
  in-memory queue.
- [Google SRE book, chapter on eliminating toil](https://sre.google/sre-book/eliminating-toil/)
  Automation that removes repetitive operator work. Use for: deciding whether
  a new tool deletes toil or creates dashboard toil.
- [CIS Ubuntu Linux 24.04 benchmark](https://www.cisecurity.org/benchmark/ubuntu_linux)
  Host hardening checklist. Use for: comparing `user-data.yaml` to a known
  baseline; do not apply the whole benchmark blindly on a 4 GB box.

## Wisdom (communities)

- [Caddy community](https://caddy.community/)
  High-signal for Caddyfile and HTTPS surprises. Use for: certificate and
  reverse-proxy behaviour that the docs left implicit.
- [Terraform section on HashiCorp Discuss](https://discuss.hashicorp.com/c/terraform-core/27)
  Backend locking and provider oddities. Use for: S3-compatible lock failures.
- [r/selfhosted](https://www.reddit.com/r/selfhosted/)
  Mixed quality; useful for cheap monitoring and backup tools after you can
  filter marketing. Use for: Uptime Kuma / CrowdSec operator experience, not
  architecture.
- Local: any Linux/sysadmin meetup where you can describe *this* two-process
  design and ask what they would delete. Use for: the “is this too small?”
  question after stage 1 is running.

## Gaps

- No first-party, current comparison of Hetzner Object Storage versus Amazon S3
  for Terraform `use_lockfile`. The lock/recovery proof in stage 1 has to be
  an experiment you run, not a citation.
- Hetzner has no AWS-style autoscaling group. Stage 2 autoscaling resources
  are therefore scripts-plus-metrics, or a later scheduler — there is no
  official “Hetzner ASG” document to follow.
- systemd socket-activated rolling restart of a Go process that owns an
  in-memory queue is under-documented as a *pattern*. Prove it on this app
  rather than copying nginx-centric tutorials.
