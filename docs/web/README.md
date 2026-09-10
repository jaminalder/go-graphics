# Public art application

Planning baseline: **2026-09-09**, repository `63c622a` on `master`.
Status: **implemented locally on `exp/public-art-app`; public launch pending**.
The architecture documents retain the planning baseline; the [product brief](product-and-ux.md) and [UX rationale](ux-simplification.md) record the owner’s simpler 2026-09-10 journey. See the
[implementation record](IMPLEMENTATION.md), [local run and deployment guide](../../deploy/README.md),
and [remaining launch gates](launch-gates.md) for the completed work and its limits.

## Recommendation

Keep the local art laboratory and build a curated public experience alongside
it in the same Go module. Preserve the existing artwork mechanisms and private
plans. Extract the exploration logic currently inside `staticart flock`, add
typed recipe/configuration seams for the artworks selected for publication,
and separate the public catalogue from the CLI registry.

Use Go `net/http`, `html/template`, embedded assets, self-hosted htmx, and a
small amount of ordinary JavaScript. Give visitors a visual path from art form
to style and colour, then four images to open or favourite. Keep the
artwork large and the controls sparse. A normal form-based path must work
without htmx.

Start on one Hetzner VPS with Terraform-managed infrastructure, Caddy for TLS,
and systemd for application and renderer supervision. Use a bounded in-memory
queue and disposable disk image cache. Isolate expensive rendering from the
web process, including hard process deadlines and memory limits. No database,
accounts, Redis, Kubernetes, or public rendering API are required.

The crucial product constraint is that free public generation is **bounded**:
small batches, curated render settings, finite queues, and friendly overload
states. Browsing the gallery must remain usable when rendering is busy.

## Reading order

| Document | Purpose |
|---|---|
| [Current project](current-project.md) | Evidence-backed inventory, strengths, gaps, and precise refactor targets |
| [Product and UX](product-and-ux.md) | User journey, visual choices, favourites, refinement, download/share, acceptance criteria |
| [Architecture](architecture.md) | Ownership, package seams, recipes, state, jobs, process isolation, migration alternatives |
| [Security and operations](security-and-operations.md) | Threat model, limits, HTTP behaviour, deployment, recovery, launch gates |
| [Research](research.md) | Primary-source findings, version/pricing caveats, QQL lessons, infrastructure choices |
| [Implementation plan](implementation-plan.md) | Ordered, reviewable slices and handoff instructions for later sessions |
| [Domain glossary](../../CONTEXT.md) | Shared terminology |

Read this set as the design for the public-app extension to
[the existing architecture](../ARCHITECTURE.md). The older
[pipeline design](../pipeline-design.md) still governs artwork internals;
its existing refactor checklist is historical work, not a fresh web backlog.

## Requirements and planning assumptions

| Topic | User requirement / planning choice |
|---|---|
| Local work | Keep offline CLI generation, experiments, new sketches, and print output |
| Public content | Selected long-form artworks; publication is explicit, not automatic |
| Interaction | Visual examples first; generated samples, favourites, and a few choices later |
| Stack | Go SSR + htmx, few dependencies, robust public operation |
| Access | Free, open access, no authentication or application database in v1 |
| Output | Download and share a low-resolution image |
| Infrastructure | DIY infrastructure as code, Terraform, cost-efficient Hetzner VPS; Caddy recommended |
| Current task | Local implementation complete; deployment and publication remain separate owner decisions |
| Launch set | Implemented provisional `pools`, `foam`, and `iris`; owner approves final publication |
| Sharing | Baseline is image-file sharing; reproducible public links are an optional extension pending owner preference |
| State | Bounded server-side favourites; no visible recovery or persistence controls |
| Size | 600 px previews and 1200 px downloads; target-host capacity validation remains a launch gate |
| Budget | Propose a €15/month operating target for the initial small deployment, excluding tax/domain/optional services; not a user-approved spending limit |
| Availability | One VPS, recoverable deployment; no high-availability claim |

"Open" is taken to mean open access. A source-code licence and the rights
granted with downloaded artwork still need explicit selection. No licence is
assigned by this plan.

## Decisions the owner still needs to make

These do not prevent the foundational work in the implementation plan:

1. Public name/domain, final launch artworks, and final visual examples.
2. Whether to add durable artwork links in a later edition; v1 implements image-file
   sharing. Durable links require an explicit edition-retention commitment.
3. Source/output licences and provenance clearance, especially for the QQL port.
4. Actual spending ceiling, expected audience, and desired recovery target.
5. Review the simplified exploration and favourites journey with unfamiliar users.

Defaults are recommendations, not settled user choices. Implementing this plan
does not authorise cloud purchases, DNS changes, public launch, or a licence.

## How the Matt Pocock skills informed this work

The installed `research` skill supplied primary-source research in a background
agent; its findings were incorporated here by the coordinator. `codebase-design`
guided small interfaces with useful behaviour behind them and extraction only
where the CLI and web application are real consumers. `domain-modeling`
provided the glossary and the proposed decision records.

`grilling` would be useful for a later focused review of the unresolved product
choices, but an extended interrogation is unnecessary for this analysis.
`prototype` belongs in the visual validation phase; `tdd` in recipe,
exploration, and admission-control implementation; `code-review` at phase
reviews. TypeScript-specific setup, shoehorn migration, exercise scaffolding,
and pre-commit setup are not relevant to the Go design. No skills were installed
or modified, and no external tickets were created.
