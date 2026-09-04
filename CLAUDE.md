@AGENTS.md

# Claude Code Adapter

Prepare the branch and worktree with the native Git workflow in
`docs/WORKTREE-WORKFLOW.md` before delegating writing work. Open Cursor on
`master/`. Set a worker's working directory to `../worktrees/<name>` when
supported. Otherwise run every worker command with the worktree as its
explicit working directory.

## Agent skills

### Issue tracker

Issues and specs live as markdown under `.scratch/<feature>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default role names: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` and `docs/adr/` at the repo root, created when a term or decision lands. See `docs/agents/domain.md`.
