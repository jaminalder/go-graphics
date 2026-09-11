# Branch and worktree workflow

The project container holds `master/` (the repository and coordinator checkout) and sibling `worktrees/`. The container itself is not a Git repository. Keep the coordinator in `master/` on branch `master`; give every writing worker its own branch and checkout.

## Create and inspect

From the master repository:

```sh
git status --short --branch
git worktree list
git worktree add -b docs/example ../worktrees/example master
```

A worker operates only in its assigned worktree, does not switch its branch and does not merge/remove worktrees. Project skills remain on master; open the editor there to discover them rather than copying skills into each experiment. Output belongs in the assigned worktree's ignored `out/`.

## Prepare review

In the writing worktree, complete the change, run `make check`, inspect generated artwork when applicable, and commit a coherent result. Review the commit against its base with ordinary `git diff`/`git log`. One seed or passing tests does not establish artistic approval; use fixed-seed previews and contact sheets for artwork changes.

## Integrate or discard

The coordinator integrates or discards only after explicit user approval. Inspect both checkout status and commit graph before integrating. A fast-forward integration from master can use `git merge --ff-only <worker-branch>` when the graph permits it; otherwise choose the appropriate reviewed merge strategy instead of forcing it.

After approved integration and a clean inspected worker checkout, the coordinator can remove it with `git worktree remove ../worktrees/example` and delete a merged branch with `git branch -d docs/example`. If Git reports uncommitted or unmerged work, inspect and resolve it. Hard resets, cleans and forced deletion/removal are not routine cleanup steps.

These rules are also recorded in [AGENTS.md](../AGENTS.md). The [development workflow](development/workflow.md) covers source/test/documentation boundaries within a worktree.
