# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root, or
- **`CONTEXT-MAP.md`** at the repo root if it exists: it points at one `CONTEXT.md` per context. Read each one relevant to the topic.
- **`docs/adr/`**: read ADRs that touch the area you're about to work in. In multi-context repos, also check `src/<context>/docs/adr/` for context-scoped decisions.
- **`docs/ARCHITECTURE.md` section 8 (Decision log)**: this repo's existing numbered decisions. Read entries that touch the area you're about to work in. A new ADR must also add a row there so the log and `docs/adr/` stay one history.

If `CONTEXT.md` or `docs/adr/` don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

`docs/ARCHITECTURE.md` already exists; its decision log is not optional reading.

## File structure

Single-context repo:

```
/
├── CONTEXT.md
├── docs/ARCHITECTURE.md               ← existing decision log (section 8)
├── docs/adr/
│   └── 0001-<decision>.md
└── internal/
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

Until `CONTEXT.md` exists, use the vocabulary already fixed in `AGENTS.md` and `docs/ARCHITECTURE.md`.

## Flag ADR conflicts

If your output contradicts an existing ADR or an architecture decision-log row, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 / architecture decision 52, but worth reopening because…_
