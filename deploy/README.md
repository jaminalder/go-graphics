# Operating the public studio

The repository now contains the local application and reviewable deployment
artifacts. No cloud resource, DNS record or public release has been created.
The owner must approve publication separately from implementation.

## Local operation

From the implementation checkout, run these in separate terminals:

```sh
mkdir -p out
go build -o out/artrender ./cmd/artrender
go build -o out/artweb ./cmd/artweb
out/artrender
out/artweb
```

Open `http://127.0.0.1:8080`. Both executables default to build identity
`development`, socket `out/artrender.sock`, and disposable `out/cache`.
`ART_ORIGIN`, `ART_ADDR`, `ART_ADMIN_ADDR`, `ART_SOCKET`, `ART_CACHE`,
`ART_GENERATION=off` and `ART_TRUST_PROXY=yes` are startup configuration.
Only loopback app/admin listeners are accepted. Trust proxy is only appropriate
when the configured Caddy owns the sole public edge; its `X-Art-Client` value
replaces any visitor-supplied value. Direct forwarding headers are ignored.

`npm ci && npm test` inside `web/browser` starts isolated local test processes,
uses pinned Playwright, and verifies real enhancement and no-JavaScript forms.
Ordinary image files and recipe exports are the sharing contract; no durable
links, user accounts, database, image uploads or public API are offered.

## Pinned deployment and configuration

Target: Ubuntu 24.04, Linux amd64, Go 1.26.8, Caddy 2.11.4, Terraform 1.14.9,
hcloud provider 1.68.0. Initial candidate host: CX23, with separate 384 MiB web
and 2 GiB renderer memory limits. These are unproven capacity assumptions.
Never report workstation timing as target-host evidence.

1. Bootstrap an independently hosted private encrypted, versioned S3 backend.
   Copy `terraform/backend.hcl.example` outside Git; supply credentials using
   an approved credential profile/environment. Give state and lockfile separate
   least-privilege permissions. Do not put tokens in tfvars, cloud-init or logs.
2. Configure public SSH key and restricted IPv4/IPv6 administrator CIDRs. Run
   `terraform init -backend-config=/private/path/backend.hcl`, then review a
   saved plan. `apply` requires explicit infrastructure approval. Server and
   primary-IP deletion protections are deliberate; do not bypass them routinely.
3. Before production, prove simultaneous lock contention from two clients,
   interrupted operation recovery and restoration of a prior state version.
   Object retention is not state locking. If a compatible S3 backend fails the
   test, choose a supported backend; do not turn off locking.
4. Bootstrap creates separate service users, SSH hardening and bounded journals.
   Install the checksum-verified pinned Caddy binary and its upstream service
   unit. Create `/etc/art/web.env` containing `ART_ORIGIN=https://your.domain`;
   `/etc/art/domain.env` contains `ART_DOMAIN=your.domain`. Set a Caddy systemd
   drop-in with `EnvironmentFile=/etc/art/domain.env` and preserve Caddy's TLS
   storage. Confirm console recovery, add administrator CIDR UFW rules and only
   then enable UFW. Keep 8080/8081/8180/8181/2019 and Unix sockets private.
5. Finish `docs/web/launch-gates.md`. Record the actual owner's approval and
   evidence references in root-owned `/etc/art/launch-approved`. This is an
   operational gate, not evidence created by the build or a licence grant.
6. From a clean committed checkout run `deploy/scripts/build-release.sh`.
   It cross-builds static Linux binaries and packages only tracked deployment
   files, a source/tool/edition manifest and SHA256 checksums. Upload that
   directory to `/opt/art/releases/<full-commit>` without changing its contents.
   Keep at least the previous working release and its checksum manifest.
7. Run `activate-release.sh <full-commit>` on the host. It verifies checksums,
   validates candidate units/Caddy, pauses admission and drains active work,
   smoke-tests candidate private services in separate cgroups, switches release
   pointers, restarts, verifies application and renderer health, and reloads
   Caddy. Failure restores prior binaries, units and Caddy configuration.
   First deployment failure stops the newly started services. Review the
   `previous=` result and keep that directory for rollback.
8. Verify HTTPS from a second machine, asset/image delivery, CSRF/cookie/CSP,
   the edge's private-path blocking and certificate persistence. HSTS is
   deliberately a separate post-HTTPS readiness step; do not add preload or
   includeSubDomains without domain-wide approval.

Sources used for configuration contracts:
[Caddy server limits](https://caddyserver.com/docs/caddyfile/options),
[Terraform S3 backend](https://developer.hashicorp.com/terraform/language/backend/s3),
[hcloud provider](https://registry.terraform.io/providers/hetznercloud/hcloud/1.68.0/docs).
The checked-in provider lockfile and actual local validators are the current
configuration evidence; remote resource creation remains untested.

## Private operation and failure handling

- `GET http://127.0.0.1:8081/metrics`: bounded queue/running/cache file/byte counts.
  Render completion logs include a correlation key, state, duration and bytes;
  no cookies, CSRF values, addresses or raw recipes are logged.
- `GET /ready` on the private listener checks compatible renderer availability
  without making an image. Public gallery liveness stays independent of a
  failing renderer. Jobs fail explicitly and never retry on a status GET.
- `POST /generation/off`: immediate admission kill switch. Existing artifacts
  keep downloading. Use `POST /generation/on` only after the incident clears.
- Inspect `journalctl -u artweb -u artrender --since -15min` and
  `systemctl show ... -p MemoryCurrent -p MemoryPeak -p NRestarts`. Journald is
  bounded to 256 MiB/seven days. Watch OS free disk, cgroup OOM events, repeated
  failures, sustained queue occupancy and certificate expiry.
- A bad release: disable admission, retain logs, run the same activation script
  with the previous committed release. Old in-memory exploration/jobs can be
  lost; browser recipes and downloaded files remain the recovery path.
- Renderer hangs/panics/oversized stdout are killed/reaped by the supervisor.
  A renderer OOM must affect only its service cgroup. Prove this on staging
  while gallery and downloads remain available; Darwin tests cannot prove it.
- Cache full/low disk: stop generation, inspect the dedicated cache path and
  available filesystem space. The app removes expired unused artifacts and
  respects open-download leases. Do not remove release assets or Terraform
  state to reclaim cache space. Only the web service owns generated artifacts.

Select an external HTTPS uptime service and an alert recipient before launch.
Check the gallery and alert on repeated failures, disk below the operating
reserve, renderer restart loops and certificate trouble. Keep all detailed
metrics private (SSH tunnel or on-host collector). Document recipient, escalation
hours and the chosen service in the operator's environment inventory.

## Backup and clean-host recovery

Back up independently of the VPS: encrypted/versioned Terraform state, release
artifacts/checksums, operator configuration/approval records, and Caddy account
and certificate storage. Do not back up transient workspaces/jobs or promise
permanent generated-image retention. Credentials use a separate encrypted
operator-controlled store; keep neither plaintext copies nor state backups here.

Proposed recovery objective: two hours, to be measured. From a second machine,
restore state credentials and the latest verified state, build a clean host,
restore service configuration and TLS data with correct ownership, verify the
retained release manifest and activate it. Check DNS/IP lifecycle before switching
traffic. Confirm old browser recipe recovery and image download on the new host.
Record elapsed time, discovered gaps, tested state version, release identity and
operator in the external run log. Do not mark this demonstrated until performed.

An artwork edition does not execute old code by itself. Retain a compatible
renderer release and its Linux/toolchain manifest or retire that edition. Never
silently reinterpret a saved recipe using a newer artistic default. A security
issue may require retiring an old renderer even if some images cannot regenerate.
