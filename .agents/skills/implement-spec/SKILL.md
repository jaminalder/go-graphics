---
name: implement-spec
description: "Implement a specification in code."
disable-model-invocation: true
---

You have been provided a spec. This spec should have tickets associated with it, describing how to implement the spec.

Follow the repository workflow in `AGENTS.md` and `docs/WORKTREE-WORKFLOW.md`.
For product/infrastructure work, implement directly on `master` with one writer
at a time; do not create branches, worktrees or PRs unless explicitly requested.
Only artistic experiments use the branch/worktree flow below. A planning-only
request stops for owner review before implementation.

The tickets are not a list of steps. They are a **task graph** with blocking relationships between them. This means there is always a **frontier** of tickets which are ready to be grabbed.

Communication to and from subagents should be sparse. Communicate primarily through **context pointers**: to the spec, tickets, research notes, and previous commits. Don't duplicate information already available via pointers.

For artistic experiments, independent implementer subagents may use assigned
worktrees when delegation is authorized. Never run concurrent writers in the
shared `master` checkout.

## Steps

1. Read the spec and tickets. Read enough to understand the task graph.

2. (optional) Use an **exploration subagent** to conduct any exploration required by the tickets - relevant codebase files or external documentation. Ensure the exploration subagent can save files - it should save its markdown notes in a directory outside the repo, accessible by all future subagents. This lets **implementer subagents** focus on implementation rather than exploration.

3. For product/infrastructure, stay on `master`. For artistic experiments, prepare the assigned branch/worktree. Create a PR only if requested.

4. Implement ready tickets in dependency order. Product/infrastructure tickets use one writer on `master`; only artistic implementers use separate assigned branches/worktrees.

5. Review each completed ticket. Integrate artistic branches only after explicit owner approval; trunk work needs no merge.

6. Advance to the next unblocked ticket, retaining the workflow's single-writer constraint for trunk development.

7. Once all tickets are complete, review the implementation and fix findings in the same permitted checkout.

8. Present the completed work for review. Update a PR only if one was requested.

9. Clean up artistic worktrees only after approved integration/discard and inspection. There are no product/infrastructure worktrees to clean up.
