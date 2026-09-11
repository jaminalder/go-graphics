> State setup update (2026-09-11): local Terraform state for one operator
> supersedes the remote-state recommendations and lock-test gates below.
> See [the current setup](../../deploy/terraform/README.md).

# Primary-source research

Accessed **2026-09-09**. This note records external findings used by the plan.
Recommendations are our design judgments, not claims made by the cited sources.
Recheck versions, availability, prices and security advisories when implementing.

## QQL: what to take from the reference

QQL's documented workflow begins with optional trait selections, generation,
then queue/favourite curation. It supports copying a result's traits into the
generator and rendering a selected result for download. Its documented batch
options include 1, 10 and 100, and its queue/favourite tools support much larger
curation sessions than this proposed application needs.
[Create, Curate, and Manage Seeds](https://qql.art/docs/how-to-qql/create-curate-and-manage-seeds).

The parameter guide describes twelve categories with several choices each.
This makes a large exploration space but also gives newcomers vocabulary to
learn. The proposed visual onboarding is our response to the user's request,
not evidence that QQL has failed a usability test.
[Parameter Overview](https://qql.art/docs/qql-parameters/parameter-overview).

Tyler Hobbs' design essay emphasizes independent, distinct features,
probabilistic ranges behind simple choices, and a persistent motif. The useful
principle here is to preserve each art form's identity while allowing visitors
to steer its output space. It does not require a public interface exposing
every dimension at once.
[The Design Philosophy of QQL](https://qql.art/docs/context/the-design-philosophy-of-qql).

QQL also documents named visual styles and combinations as exploration aids.
This supports studying image examples as entry points, while our own style
names, assets and recipe mappings must come from this project's artworks.
We should not promise a rare emergent arrangement just because a preset makes
it more likely.
[Iconic Styles](https://qql.art/docs/exploration/iconic-styles),
[Popular Presets](https://qql.art/docs/exploration/popular-presets).

The live homepage/create page was accessible to the text browser, but rich
interactive rendering was not exercised end to end. This investigation used
the first-party written workflow and examples; it is not a browser usability
audit of qql.art.

### Provenance is separate from inspiration

The local [QQL spec](../sketches/007-qql.md) explicitly identifies a port of
`wchargin/qqlrs`. Attempts to retrieve that repository and its presumed licence
file through the research browser did not establish a source licence. Record
this as **unresolved**, not proof of either permission or prohibition.

QQL's website terms describe a limited personal, non-commercial licence for
its online-service works. That is not a blanket permission to republish a
source port or operate a derivative service. Establish the applicable source,
asset and naming permissions separately before admitting this sketch.
[QQL terms](https://qql.art/tou).

Recommendation: use QQL as the interaction/design reference, not the assumed
launch brand or gallery asset source. Audit all selected artworks' provenance,
including borrowed code, palette datasets and reference images. No licensing
conclusion is made here.

## Go and frontend choices

Go's standard HTTP and template packages cover routing, request lifecycle and
contextual HTML escaping, while `embed` can package a release's templates and
assets. Recommendation: start with these, ordinary CSS and a small external JS
file. A framework, generated templating tool or frontend build system is not
needed for the proposed screens. [HTTP](https://pkg.go.dev/net/http),
[templates](https://pkg.go.dev/html/template), [embedding](https://pkg.go.dev/embed).

Security updates still matter with no third-party modules. Go's security
guidance describes its supported-release policy and vulnerability tooling.
Recommendation: pin a supported patched toolchain per release and rebuild on
relevant security fixes, with `govulncheck` in release checks.
[Go security](https://go.dev/doc/security/).

### htmx version transition

At research time the official homepage announces htmx 4 but intentionally keeps
the npm `latest` designation on 2.x until a future point in 2027; its quick
start pins **2.0.10**. Recommendation: use that explicit 2.x version as the
planning baseline and review maintained releases when coding starts. Selecting
4.x is also reasonable after a deliberate compatibility/security pass; never
download an unversioned asset.
[htmx homepage](https://htmx.org/).

The 4.x migration document removes `allowEval`, `allowScriptTags` and
`historyCacheSize`, renames `includeIndicatorStyles`, and changes error-response
swapping. Therefore the 2.x settings in the security plan must not be copied
unchanged into a 4.x application.
[What's new in htmx 4](https://four.htmx.org/docs/whats-new-in-htmx-4).

Native sharing supports files where the browser/platform permits it, requires
a secure context and user activation, and exposes `canShare` for capability
checks. Recommendation: progressive file sharing with a normal download
fallback; no third-party social scripts.
[Web Share specification](https://www.w3.org/TR/web-share/).

## Caddy and process isolation

Caddy's automatic HTTPS is a good fit for one public domain. Use persistent
TLS storage and explicit networking/timeouts. Its reverse-proxy documentation
explains forwarded-header trust and backend transport settings; defaults are
not a complete application security policy.
[HTTPS](https://caddyserver.com/docs/automatic-https),
[reverse proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy).

The documented `rate_limit` handler is **not included in standard Caddy**.
Recommendation: keep an ordinary maintained Caddy build and put bounded,
cost-aware admission in Go. Introducing a custom proxy build merely to obtain
this directive adds an avoidable maintenance obligation.
[Module status](https://caddyserver.com/docs/modules/http.handlers.rate_limit).

Historical baseline, superseded by ADR 0004: systemd service restrictions and
cgroups supported a small host-service deployment with one supervised child per
job. The owner subsequently made container packaging and operating experience
explicit goals and selected Compose. The separate process/resource boundary
remains; the preferred packaging and supervision changed. Linux resource and
execution controls remain useful background reading.
[Execution controls](https://github.com/systemd/systemd/blob/main/man/systemd.exec.xml),
[resource controls](https://github.com/systemd/systemd/blob/main/man/systemd.resource-control.xml).

## Hetzner cost and sizing

The official price adjustment effective **15 June 2026** lists these EU
monthly server caps, excluding VAT and IPv4. These replace common older price
examples; region, stock and current order details still need verification.
[Dated Hetzner prices](https://docs.hetzner.com/general/infrastructure-and-availability/price-adjustment/).

| Tier | Monthly server cap | Planning use |
|---|---:|---|
| CX23 | €5.49 | First benchmark / smallest candidate deployment |
| CX33 | €8.49 | Additional shared capacity if measurements justify it |
| CAX11 | €5.99 | Arm alternative after platform validation |
| CPX22 | €19.49 | Compare measured value; not automatically a better starting tier |
| CCX13 | €42.99 | Dedicated CPU option if sustained load requires predictable capacity |

CX23 is a shared two-vCPU, 4 GB RAM, 40 GB disk tier. Shared CPU performance can
vary; plan names/core counts do not establish render latency. The product page
showed availability ambiguity in fetched text, so confirm stock in the actual
console/API. [Cost-optimized servers](https://www.hetzner.com/cloud/cost-optimized/),
[server FAQ](https://docs.hetzner.com/cloud/servers/faq/).

One Primary IPv4 is €0.50/month before VAT; IPv6 is free. Include IPv4 for broad
public access. Automatic server backups add 20% of server cost. That gives
illustrative subtotals of **€7.09/month for CX23** or **€10.69/month for CX33**,
including IPv4 and server backups, before VAT and rounding on the invoice.
[IPv4 pricing](https://docs.hetzner.com/general/infrastructure-and-availability/ipv4-pricing/),
[billing FAQ](https://docs.hetzner.com/cloud/billing/faq/).

These subtotals exclude domain, independent recovery storage, Terraform state
hosting, external monitoring, extra snapshots and traffic overage. A €15/month
initial target is our provisional budget proposal, not a quotation or approval
to spend. Avoid adding a paid object-storage service solely for a tiny state
file without including its minimum monthly charge in the full budget.

Choose the first production architecture deliberately: Go floating-point art
and PNG metadata need verification on the chosen Linux target even if local
determinism tests pass. Do not claim Arm/x86 byte equality without evidence.

## Terraform and remote state

The hcloud server resource supports `user_data` and explicit networking;
`datacenter` is deprecated in provider versions from 1.67.0 in favour of
`location`. Cloud-init/user-data size is bounded. Recommendation: pin a tested
provider and use nonsecret bootstrap; keep binary release deployment separate.
[hcloud server resource](https://registry.terraform.io/providers/hetznercloud/hcloud/latest/docs/resources/server).

Terraform's `sensitive` marking does not keep a value out of state. Keep secrets
out of cloud-init/HCL, secure state and saved plans, and inject runtime secrets
through a separate mechanism.
[Sensitive-data guidance](https://developer.hashicorp.com/terraform/language/manage-sensitive-data).

For S3 remote state, Terraform documents `use_lockfile`, recommends versioning,
and only guarantees Amazon S3 compatibility; other implementations are
best-effort. Recommendation: use an independently hosted versioned backend
with demonstrated locking and restore. Verify a proposed Hetzner bucket rather
than inferring backend safety from the phrase “S3 compatible”.
[S3 backend](https://developer.hashicorp.com/terraform/language/backend/s3).

HCP Terraform's local-execution mode is an alternative remote-state option if
the DIY backend fails or costs more. Its current plan eligibility/pricing was
not evaluated here; select and document it before treating it as free.
[Execution modes](https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings).

## Remaining experiments, not research claims

The architecture choice is supported by the inspected code and platform
capabilities. Production readiness still requires measured VPS CPU/RSS/tail
latency, real-browser CSP and accessibility checks, style curation, backend
locking tests and a restore drill. We have not provisioned a server, measured
Hetzner throughput, performed a public load test or assigned source/output
licences in this task.

## Docker Compose decision (2026-09-10)

The owner chose Compose for standard image packaging and container operations
learning. Current implementation is described in [the runbook](../../deploy/README.md).
It retains separate resource groups and the private renderer protocol.
Official references: [Compose service controls](https://docs.docker.com/reference/compose-file/services/),
[resource limits](https://docs.docker.com/engine/containers/resource_constraints/),
[firewall integration](https://docs.docker.com/engine/network/packet-filtering-firewalls/),
[Ubuntu installation](https://docs.docker.com/engine/install/ubuntu/).
The earlier preference for host units above is historical, not a constraint
against the owner's approved runtime change.
