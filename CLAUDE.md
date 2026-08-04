@AGENTS.md

# Claude Code Adapter

Use Claude Code's supported subagent mechanism only after
`./tools/experiment create` has prepared the branch, worktree, brief, and
record. Set the worker's working directory to the printed worktree when the
tool supports it. Otherwise perform the work from the coordinator session with
every command explicitly scoped to that worktree. Never use a Claude-managed
worktree as a substitute for the repository lifecycle.
