@AGENTS.md

# Claude Code Adapter

Prepare the branch and worktree with the native Git workflow in
`docs/WORKTREE-WORKFLOW.md` before delegating writing work. Open Cursor on
`master/`. Set a worker's working directory to `../worktrees/<name>` when
supported. Otherwise run every worker command with the worktree as its
explicit working directory.

## Agent skills

### Issue tracker

Issues and specs live as markdown under `.scratch/<feature>/`.

### Triage labels

Default role names: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`.

### Domain docs

Current project terminology and architecture are documented in `docs/reference/data.md` and `docs/ARCHITECTURE.md`.
