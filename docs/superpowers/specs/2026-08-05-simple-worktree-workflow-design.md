# Simple Branch And Worktree Workflow Design

## Goal

Document a predictable process for parallel development with ordinary Git
branches and worktrees. Agents and humans should be able to create an isolated
checkout, commit an experiment, review it, integrate all or part of it, and
remove it without custom lifecycle software.

## Scope

The repository provides:

- concise canonical instructions in `AGENTS.md`;
- a worktree workflow guide with copy-pastable Git commands;
- lightweight brief and result templates for experiments;
- an ignored output namespace for experiment renders;
- tool-specific agent notes that defer to the same Git workflow.

The repository does not provide a worktree manager, experiment CLI, state
machine, lock service, branch database, archive transaction, or wrappers around
merge and cherry-pick. Git already owns those responsibilities.

## Workflow

The coordinator stays in the main checkout on `master`. Each writing task gets
one branch and one sibling worktree:

```sh
git worktree add ../go-graphics-worktrees/<name> -b exp/<name> master
```

The worker operates only in that worktree, renders into its ignored output
directory, and makes coherent commits. Review uses normal Git inspection:

```sh
git diff master...exp/<name>
git log --oneline master..exp/<name>
```

Approved work is integrated using the Git operation that matches the decision:

- `git merge --no-ff exp/<name>` for the complete branch;
- `git cherry-pick <commit>` for selected self-contained commits;
- manual reimplementation on a fresh branch when only entangled behavior is
  wanted.

Cleanup remains non-forced:

```sh
git worktree remove ../go-graphics-worktrees/<name>
git branch -d exp/<name>
```

Git reports dirty worktrees, conflicts, unmerged branches, and invalid cleanup.
The workflow guide explains how to inspect those conditions rather than hiding
or reproducing Git's safeguards.

## Records

Brief and result files are optional working aids, not a lifecycle database.
They capture the hypothesis, scope, fixed comparison seeds, commands, visual
findings, and recommendation. The branch and its commits remain authoritative.

Generated renders stay ignored. A deliberately selected low-resolution review
sheet may be committed when useful.

## Agent Roles

The coordinator creates and integrates branches and worktrees. A writing
worker edits only its assigned worktree and does not merge or remove it. A
reviewer is read-only. Tool-specific subagent launchers are optional adapters;
if they cannot constrain a worker to the prepared worktree, the coordinator
runs commands with an explicit working directory.

## Testing

No automated tests reproduce Git branch, merge, cherry-pick, locking, or
worktree behavior. Documentation commands are checked against the installed
Git version, and ordinary repository changes continue to pass `make check`.

## Superseded Design

This design supersedes the custom agent-managed experiment lifecycle proposed
on 2026-08-04. Its Go package, CLI, state records, locks, archive transactions,
and extensive temporary-repository tests are intentionally out of scope.
