# Trunk development and artistic worktrees

The project container holds `master/` (the repository and coordinator checkout) and sibling `worktrees/`. The container itself is not a Git repository.

## Product and infrastructure: trunk-based on master

Work directly in `master/` on branch `master` for product, application, infrastructure, tests and documentation changes. Do not create a branch or worktree for this work unless the user explicitly requests it. Keep one writing task at a time in this checkout and preserve unrelated changes. Inspect status before starting, perform focused checks, and keep the trunk buildable as work packages land. Commit/push only when requested.

Substantial changes may have an owner-reviewed proposal under `docs/plans/`, clearly marked as future state. An approved plan does not automatically authorize production deployment. Generic skill instructions to create a branch, PR or writing worktree do not override this repository workflow.

## Artistic experiments: isolated branches and worktrees

Keep the coordinator on `master`; give each artistic experiment its own branch and checkout. The remaining commands and integration rules apply to these experiments.

## Create and inspect

From the master repository:

```sh
git status --short --branch
git worktree list
git worktree add -b exp/example ../worktrees/example master
```

A worker operates only in its assigned worktree, does not switch its branch and does not merge/remove worktrees. Project skills remain on master; open the editor there to discover them rather than copying skills into each experiment. Output belongs in the assigned worktree's ignored `out/`.

## Prepare review

In the artistic worktree, complete the change, run `make check`, inspect generated artwork, and prepare a coherent result. Commit when requested. Review the diff/commits against the experiment's base with ordinary `git diff`/`git log`. One seed or passing tests does not establish artistic approval; use fixed-seed previews and contact sheets for artwork changes.

## Integrate or discard

The coordinator integrates or discards only after explicit user approval. Inspect both checkout status and commit graph before integrating. A fast-forward integration from master can use `git merge --ff-only <worker-branch>` when the graph permits it; otherwise choose the appropriate reviewed merge strategy instead of forcing it.

After approved integration and a clean inspected worker checkout, the coordinator can remove it with `git worktree remove ../worktrees/example` and delete a merged branch with `git branch -d exp/example`. If Git reports uncommitted or unmerged work, inspect and resolve it. Hard resets, cleans and forced deletion/removal are not routine cleanup steps.

These rules are also recorded in [AGENTS.md](../AGENTS.md). The [development workflow](development/workflow.md) covers source/test/documentation boundaries for both modes.
