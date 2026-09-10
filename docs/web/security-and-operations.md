# Security, capacity and operations

Status: implementation requirements aligned with the owner-approved Compose
runtime (ADR 0004, 2026-09-10). Local checks do not establish target-host
security or capacity; see launch-gates.md for outstanding evidence.

## Threat model

The valuable resources are CPU time, RAM, disk/bandwidth, availability, recipe
integrity and visitors' temporary workspaces. The public may send arbitrary
HTTP requests, repeat valid requests, forge forwarding headers, alter tokens,
open many browser workspaces and request expensive but valid combinations.

There are no account credentials, payments, uploads or user-published text in
v1. That reduces scope but does not eliminate CSRF, XSS, resource exhaustion,
path traversal, cache poisoning, dependency vulnerabilities or host compromise.
The main application-specific attack is converting a cheap request into a
large amount of rendering work.

## Admission and resource envelope

Validate before allocating a workspace, parsing large nested data, starting a
job or constructing an image. Use checked multiplication for pixels, sample
counts and histogram dimensions, and validate across combinations as well as
individual fields. Enforce the same publication allowlist on every route into
rendering, including browser restore and optional public links.

| Resource | Proposed starting limit | Required behaviour |
|---|---|---|
| Public batch | 4 samples | Server decides count; not arbitrary client input |
| Active rendering | 1 job across the host | Renderer independently rejects concurrent calls; no hidden second queue |
| Queued rendering | 8 individual jobs | Atomic batch admission; fair interleaving between workspaces |
| Workspace active batch | 1 | Duplicate action returns existing batch |
| Generation requests | 6 batches/minute per workspace, burst 2; 12/minute per IP, burst 2 | Separate bounded key maps; tune for usability/shared networks |
| Limiter keys | 10,000 with idle expiry | At capacity use conservative global fallback, never unbounded allocation |
| Queue wait | 60 s | Expire with explicit retry; release all reservations |
| Render execution | 15 s preview / 30 s download | Hard kill/reap deadline; slower artworks not silently admitted |
| Images | Fixed artwork tiers; downloads max 1200 px long edge | No `print`, deep, arbitrary width, AA, quality or output paths |
| Public request body | 64 KiB | Apply `MaxBytesReader` before parsing; smaller limits where practical |
| One recipe payload | 16 KiB | Reject nesting/count/duplicate-key abuse; optional link has a smaller bound |
| HTTP header / URI | 16 KiB headers / 4 KiB request target | Test consistent app/proxy handling of oversize requests |
| Artifact cache | 2 GiB, 5000 files, 24 h age | Independent byte/count/age/temp limits; prune and stop on low disk |
| Encoded image | 16 MiB | Stop worker output on overflow; never serve a partial image |

These are ceilings, not targets. In particular a queue routinely approaching
60 seconds fails the proposed UX targets. Per-IP limits are imperfect: shared
networks group visitors together and IPv6 clients rotate addresses. Normalize
addresses carefully, test IPv4-mapped forms and consider a conservative IPv6
prefix policy only after observing false positives. Session cookies are
resettable. Global CPU, queue and memory caps are the final protection.

Check existing artifacts and coalesce identical jobs before reserving expensive
work. Still limit metadata lookups, status polling, downloads and cache-hit
requests so those paths cannot allocate unlimited state or bandwidth. Use a
small, bounded idempotency index keyed by workspace/action ID and request
digest; reject reuse with a different body.

`golang.org/x/time/rate` is a reasonable single Go dependency for a tested token
bucket. It does not supply key eviction, client identification, fairness or
global work accounting; these remain in the application. Document that
dependency when implementing it. Do not promise zero dependencies at the cost
of inventing security mechanisms. [Rate limiter documentation](https://pkg.go.dev/golang.org/x/time/rate).

Render jobs do not run on GET, HEAD, image requests, social unfurls or crawler
visits. Expose a generation kill switch; browsing, existing images and download
remain available when it is off. Global saturation returns 503, client limits
429, both with `Retry-After` and useful HTML. Recipe conflicts/expiry have
separate messages. Never automatically loop retry from an error response.

## HTTP and browser controls

Use a constructed `http.Server` with explicit timeouts and body/header bounds.
Start with 5 s header, 10 s read, 30 s write and 60 s idle timeouts for the
public app, then test slow-but-valid downloads. Generation POSTs return after
admission, so these are not render timeouts. Go's CSRF-oriented
`CrossOriginProtection` is available, but allows safe methods and is not bot
protection. [Go HTTP documentation](https://pkg.go.dev/net/http).

For cookie-bound POSTs use same-origin protection and synchronizer CSRF tokens
validated against the workspace, including htmx requests and normal forms.
Reject unexpected content types, duplicate/conflicting form values, unknown
keys, malformed seeds and mismatched workspace ownership. Do not infer a
request is safe because it has `HX-Request`; that header is forgeable.

`html/template` supplies contextual escaping for trusted template authors.
Use typed view models and plain strings for user-derived content; never cast
it to `template.HTML`, `template.JS`, `template.URL`, or template source.
Sanitize header values and use server-generated download filenames.
[Go template security model](https://pkg.go.dev/html/template#hdr-Security_Model).

Self-host pinned htmx and scripts; use a restrictive CSP. Starting policy:

```text
default-src 'self'; script-src 'self'; style-src 'self';
img-src 'self'; font-src 'self'; connect-src 'self';
object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'
```

Only add `blob:` to a directive if a tested feature actually needs it; fetching
a same-origin image for file sharing does not itself require a blob image URL.
Avoid inline event handlers, evaluated htmx expressions and dynamic script
injection. For the proposed htmx 2.x baseline use `allowEval=false`,
`allowScriptTags=false`, `selfRequestsOnly=true`,
`includeIndicatorStyles=false`, and `historyCacheSize=0`. Provide indicator CSS
and event handlers in external assets. These names are version-specific;
htmx 4 changed them. [htmx API](https://htmx.org/api/),
[4.x changes](https://four.htmx.org/docs/whats-new-in-htmx-4).

Add `X-Content-Type-Options: nosniff`, a restrictive referrer policy, and a
Permissions Policy disabling unused device features while preserving
`web-share=(self)` if native sharing is offered. Enable HSTS after verifying
the production domain's HTTPS operation; `includeSubDomains`/preload require
separate domain-wide readiness. Test CSP, cookies and navigation in real
browsers, not just header assertions.

Reject unknown Host values; generate absolute links from configured canonical
origin. Do not trust arbitrary incoming forwarding headers. If Caddy is the
only edge, the Go listener is reachable only by Caddy and trusts only its
sanitized client information. Handle direct spoofed `X-Forwarded-For` tests.
Adding a CDN later requires explicit trusted ranges and origin access control.
[Caddy reverse proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy).

Session pages/fragments are `private, no-store`. Full-page and fragment routes
are distinct. Catalogue assets have content hashes and long cache lifetimes;
errors, expiring workspace state and responses with cookies do not enter a
shared cache. No CORS permission is needed for this same-origin application.

## One-VPS deployment

The owner selected Docker Compose on one Ubuntu 24.04 VPS:

```text
Internet :80/:443 -> Hetzner firewall + Docker-aware host packet filtering
  -> Caddy container -> internal bridge -> web container :8080
    -> shared Unix socket -> renderer container -> disposable child
```

The web container owns the only queue, workspaces and image cache. The renderer
supervisor has no queue and no network interfaces beyond loopback. Web and
renderer have separate numeric users, read-only roots, dropped capabilities,
`no-new-privileges`, 64-task ceilings and limited writable mounts. The renderer
shares only a group-owned socket directory with web; web's mount is read-only.
The application containers never receive a Docker socket, cloud credentials,
operator files or host home directories.

Compose configures web at 384 MiB / 0.5 CPU and renderer at 2 GiB / 1.5 CPUs,
with no swap headroom. Caddy has 256 MiB / 0.5 CPU. Reserve capacity for Docker,
OS and page cache. Renderer uses `GOMAXPROCS=2`; never change per-request process
scheduling or the flame's deterministic orbit partitioning. Confirm actual
cgroup limits and OOM behavior under target load. Container limits include
children. Docker's `unless-stopped` policy restarts exited processes; unhealthy
status alone does not restart a container. Monitor restart loops and OOM events.
[Docker resource controls](https://docs.docker.com/engine/containers/resource_constraints/).

Caddy and web use fixed addresses on a small internal network. Only Caddy
publishes ports, and only Caddy's configured IP may supply sanitized client
identity through `X-Art-Client`. `ART_TRUSTED_PROXY` is one literal IP, never
an arbitrary forwarded range. Native development defaults to loopback without
proxy trust. The web admin listener is always loopback inside its container;
operators use `compose exec ... /app/artctl`, not a published admin port.
The renderer uses `network_mode: none` and its existing private Unix protocol.

Caddy handles certificate issuance/renewal and HTTP redirects. Preserve its
certificate/account storage. Use normal automatic HTTPS for a configured
hostname; no on-demand TLS or DNS plugin is required. Set both downstream HTTP
limits and finite upstream connection/response-header timeouts. Ordinary Caddy
does not contain the nonstandard rate-limit module; application admission is
the primary control. [Automatic HTTPS](https://caddyserver.com/docs/automatic-https),
[rate-limit module status](https://caddyserver.com/docs/modules/http.handlers.rate_limit).

Open only 80/443 publicly and SSH from approved administration networks, with
both IP families covered. Keep app, renderer, metrics, profiling and Caddy
admin inaccessible from the internet. Use SSH keys, a non-root administrator,
disabled password login and a documented console recovery route. Host firewall
rules are defence in depth and must be checked against the cloud firewall.
Docker-published ports can bypass UFW INPUT rules. For the chosen Docker
iptables backend use DOCKER-USER forwarding policy, preserve established flows,
allow only intended public TCP 80/443 and deny other unsolicited forwarding.
Check IPv4 and IPv6, including any userland-proxy path, after reboot. Never
assume UFW alone protects a published port or disable Docker firewall management.
[Docker firewall integration](https://docs.docker.com/engine/network/packet-filtering-firewalls/).

## Infrastructure as code and state

Terraform owns the Hetzner project resources: server, firewall, labels, public
SSH keys and explicit IP lifecycle. It can manage DNS once the domain/provider
is selected. Pin Terraform/provider versions and commit `.terraform.lock.hcl`.
Keep variable examples and a backend bootstrap README. Ignore state, plan
files, `.terraform`, secret tfvars and crash logs when implementation begins.

Cloud-init bootstraps packages, users, directories and service installation
prerequisites. It is not the app release mechanism and must contain no signing
keys, provider credentials or private SSH keys. Changing bootstrap/server
fields can replace infrastructure; inspect the plan and use production
deletion protection / Terraform `prevent_destroy` where appropriate. Do not
assume those controls replace state backup or operator review.

Application deployment is a separate script shipping prebuilt versioned
image archives and validated Compose configuration. Avoid `remote-exec` provisioners
as the normal deploy loop. Terraform's own guidance treats provisioners as a
last resort. [Provisioner guidance](https://developer.hashicorp.com/terraform/language/provisioners).

Remote Terraform state is necessary for safe operation from other machines.
Bootstrap it independently of the application VPS and document access/recovery
in the repository without credentials. Recommendation: use a private encrypted,
versioned S3 backend that passes lock tests; if choosing Hetzner Object Storage,
verify its compatibility explicitly. Terraform supports S3 lockfiles via
`use_lockfile`; S3-compatible storage is not a guarantee of correct concurrent
locking. Object retention lock is a different feature.
[S3 backend contract](https://developer.hashicorp.com/terraform/language/backend/s3).

Test two simultaneous lock acquisitions, interrupted apply recovery and state
version restoration before production. If the candidate backend fails, choose
a supported remote-state service; never silently disable locking. HCP Terraform
with local execution is an alternative, with its account/pricing decision
separate from the VPS. Keeping only local state or the only state copy on the
managed VPS fails the cross-machine/recovery requirement.

Terraform `sensitive` hides presentation, not state contents. Use environment
or credential mechanisms for backend/provider tokens; restrict state and plan
access, encrypt backups, and separate deploy credentials from infrastructure
credentials. Deliver runtime secrets outside Terraform/cloud-init, for example
root-owned operator files or explicit read-only secret mounts via a restricted
deployment step; never bake secrets into images. This app needs no cloud tokens.
[Sensitive data](https://developer.hashicorp.com/terraform/language/manage-sensitive-data).

## Deployment, observability and recovery

CI becomes a release requirement, superseding the local-only decision 3.
Run `make check`, race tests, relevant fuzz seeds, supported-toolchain
`govulncheck`, Linux builds and browser/security tests. Pin downloaded tools and
CI actions to reviewed versions/commits. Keep browser-test dependencies in a
separate tooling environment; they do not belong in production.

Create immutable releases with checksums and manifest containing source
revision, Go/tool versions, catalogue/edition IDs and asset hashes. In the
deployment script: upload to a staging directory, verify hashes and modes,
validate Caddy and Compose config, smoke-test a generation-disabled candidate
on its own private network/socket/cache, pause new admission, drain or explicitly fail active jobs, switch
release pointers, recreate renderer/web containers and verify externally.
Use a fixed production Compose project and stop old application containers
before replacement. Caddy configuration belongs to its versioned image; its
certificate volumes survive recreation. Production uses immutable image IDs
loaded from the checksummed archive, with no registry required. A host deploy
lock serializes activation. Separate staging approval from public-launch approval. Restore previous
pointers/configuration on failure. Web and renderer must reject incompatible
protocol/build identities; do not cache an image under the wrong release key.

Keep at least the previous working release. A brief planned restart is
acceptable; in-memory jobs/workspaces can be lost. A future rolling deployment
cannot simply start a second independently queued web instance over the same
host: it would double admission and memory limits.

Provide cheap liveness and readiness endpoints; distinguish a healthy gallery
with temporarily unavailable generation from a broken web application.
Renderer health/degraded state is separately visible to operations. Never
perform an artwork render in a routine health check.

Use `slog` structured logs with request/job correlation and low-cardinality
metrics for admission/rejection, queue depth/wait, render time/failure/timeout,
cache usage/hits/evictions, bytes served, host CPU/RAM/disk and worker restarts.
Do not log cookies, CSRF tokens, full recipe tokens or raw request bodies.
Avoid unbounded metric labels per seed, IP or job. Cap/rotate logs and document
retention. Application IP counters can be ephemeral; permanent visitor tracking
is unnecessary.

Use an external uptime check and alerts for disk exhaustion, persistent render
failures, service restarts and certificate trouble. Start with bounded Docker logs, host journald and
small private metrics; a full Prometheus/Grafana stack is optional, not a v1
dependency. An alert must have a recipient and a short runbook.

Proposed service targets: gallery/status p95 below 250 ms server time under
the admitted generation load; a four-image preview batch normally ready within
8 seconds on the chosen VPS; representative download rendition within 15
seconds. Test uncached expensive combinations separately. Failure to meet these
targets means tune the public set, reduce load or change capacity; do not
present workstation results as proof.

Propose a two-hour clean-host recovery objective for the small service. Active
jobs may be lost. Browser-saved recipes/downloads remain visitor-controlled;
infrastructure state, release image archives/manifests, operator configuration,
and Caddy account/certificate volumes need independent backups. Volume survival
across container replacement is not a backup. Disposable image
cache is not a backup priority. Restore once from another workstation before
launch. Hetzner automatic server backups have seven slots, exclude attached
volumes, and are removed with the server; they are not the only recovery copy.
[Backup documentation](https://docs.hetzner.com/cloud/servers/backups-snapshots/overview/).

## Public-launch gates

Pass adversarial tests for oversized/duplicate inputs, invalid recipes, path
traversal attempts, spoofed forwarded headers, CSRF, stale tabs, duplicate
submissions and bounded state creation. Saturate rendering while browsing,
polling and downloading; kill/timeout/OOM the renderer and fill the cache.
Confirm resources return to steady state and no late or partial result leaks.

Complete the [UX validation](product-and-ux.md), licence/provenance inventory,
operator contact, privacy/retention statement and download usage terms. The
owner must choose jurisdiction-appropriate publication information; this plan
does not declare a legal exemption because the app has no accounts or analytics.

Demonstrate deployment, rollback, reboot, certificate persistence, state-lock
recovery and clean-host rebuild. Obtain final artwork and launch approval.
Revisit multi-host architecture only when observed demand requires it; a second
web replica needs shared coordination or an explicit routing/state strategy.
