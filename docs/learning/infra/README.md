# Learning track: small-shop infrastructure

This folder is a **learning plan**, not an implementation plan.

The public application's runtime is specified in [`docs/web/`](../../web/README.md)
and implemented on the `public-art-app` worktree under `deploy/`. Those documents
remain the product contract. Nothing here authorises changing them, applying
Terraform, buying a server, or opening DNS.

Use this track to learn infrastructure as code by operating **this** service on
a cost-efficient VPS, and to test the claim that a robust public studio does
not require a hyperscaler account or Kubernetes.

| File | Use it for |
|---|---|
| [MISSION.md](MISSION.md) | Why this track exists and what “done” looks like |
| [PLAN.md](PLAN.md) | The three stages, in order, with what each one teaches |
| [QUESTIONS.md](QUESTIONS.md) | Decisions to answer in writing before adding tools |
| [ADDITIONS.md](ADDITIONS.md) | Suggested later changes to `deploy/`; do not merge these into `docs/web/` yet |
| [RESOURCES.md](RESOURCES.md) | Primary sources to read instead of folklore |
| [GLOSSARY.md](GLOSSARY.md) | Shared words for this track |
| [NOTES.md](NOTES.md) | Preferences that should steer later teaching sessions |
| [learning-records/](learning-records/README.md) | Insights after you have actually done the work |

Start with [MISSION.md](MISSION.md), then [PLAN.md](PLAN.md) stage 1. Record
answers in [QUESTIONS.md](QUESTIONS.md) as you go; do not skip ahead to
containers, zero-downtime, or Kubernetes because they sound like “real infra”.
The existing stack is already the classroom.

When this track and the web implementation disagree, the web implementation
wins for production and this track explains *why* that choice is the lesson.
