# Learning track: operate Singular Seed with Docker Compose

**Start with the [visual learning atlas](index.html).** Open it in a browser for
interactive SVG system maps, three short lessons, a staged roadmap and a
searchable [field guide](reference/field-guide.html). No build or server is needed.
Keep `assets/` next to the HTML; source links rely on this checkout’s layout.

The atlas assumes familiarity with Docker/Linux/CI basics and focuses on how
this application fits together. Its status is a dated source snapshot, not live
monitoring. Browser practice notes are optional, exportable self-assessments;
opening pages never records mastery or clears launch gates.

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
