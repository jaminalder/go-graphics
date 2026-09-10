# Learning track: operate Singular Seed with Docker Compose

The owner chose Docker Compose on 2026-09-10. Learn by operating this
application on one inexpensive Hetzner VPS. The runtime files now live in
`deploy/` on `master`; older worktree-only references are historical.

This folder is the curriculum. [The deployment runbook](../../../deploy/README.md)
and [web security contract](../../web/security-and-operations.md) describe the
implemented runtime. Learning notes never constitute cloud-spending, DNS or
public-launch approval. Staging rehearsal has a separate approval from launch.

| File | Purpose |
|---|---|
| [MISSION.md](MISSION.md) | Outcome and boundaries |
| [PLAN.md](PLAN.md) | Three stages with exercises and exit criteria |
| [QUESTIONS.md](QUESTIONS.md) | Owner decisions and explanations to write as you learn |
| [ADDITIONS.md](ADDITIONS.md) | Remaining gaps; distinguish implementation from operational proof |
| [RESOURCES.md](RESOURCES.md) | Official documentation for each layer |
| [GLOSSARY.md](GLOSSARY.md) | Container and infrastructure vocabulary |
| [NOTES.md](NOTES.md) | Teaching preferences and decision history |
| [learning-records/](learning-records/README.md) | What changed in our understanding |

Start with stage 1's local Compose rehearsal. Explain a layer, run it, inspect
its actual behavior, then deliberately break it and recover. Record results;
configuration that looks right is not the same as a demonstrated property.
