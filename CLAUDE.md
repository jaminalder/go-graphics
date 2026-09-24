@AGENTS.md

# Claude Code Adapter

Product and infrastructure work is trunk-based directly in `master/` on `master`,
with one writer at a time. Do not create a branch/worktree for that work.
Branches and worktrees are only for artistic experiments, following
`docs/WORKTREE-WORKFLOW.md`. Open Cursor on `master/`. For artistic workers,
set the working directory to their assigned `../worktrees/<name>` explicitly.
Repository workflow overrides generic skill branch/worktree/PR instructions.

## Agent skills

### Issue tracker

Issues and specs may live as markdown under `.scratch/<feature>/`. Durable,
owner-reviewed architecture proposals and work packages live under `docs/plans/`
and are clearly distinguished from current-state documentation. The PostgreSQL
proposal starts at `docs/plans/postgresql/README.md`.

### Triage labels

Default role names: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`.

### Domain docs

Current project terminology and architecture are documented in `docs/reference/data.md` and `docs/ARCHITECTURE.md`.
