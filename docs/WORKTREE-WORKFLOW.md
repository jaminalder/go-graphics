# Branch And Worktree Workflow

Use ordinary Git branches and sibling worktrees for isolated experiments and
parallel development. Git is the lifecycle mechanism; the optional files in
`experiments/templates/` are working notes, not a state database.

## Roles

The coordinator stays in the canonical checkout on `master`. It creates and
removes worktrees, reviews branches, and integrates approved work.

A writing worker gets one branch and one worktree. It edits only there, does
not switch branches, and does not merge, rebase, or remove worktrees. A
read-only reviewer may inspect commits, diffs, tests, renders, and reports but
does not modify the branch.

Two concurrent rendering workers is a practical default limit for CPU and
memory use. This is guidance, not an enforced lock.

## Create

Choose a short lower-case `<name>` such as `foam-depth-hatching`. From the
coordinator checkout on a clean `master`:

```sh
mkdir -p ../go-graphics-worktrees
git worktree add ../go-graphics-worktrees/<name> -b exp/<name> master
git -C ../go-graphics-worktrees/<name> status --short --branch
```

Give the worker the absolute worktree path, branch name, scope, relevant sketch
specification, and any fixed comparison seeds. Copy
`experiments/templates/brief.md` into the worktree when a written experiment
brief is useful.

For a dependent experiment, name the parent branch explicitly instead of
`master`. For independent comparisons, start every branch from the same
`master` commit.

## Work

Run every worker command in its assigned worktree. Keep generated baseline and
candidate images under that worktree's ignored `out/` directory so concurrent
workers cannot overwrite each other's output.

For artwork changes:

1. Render the baseline before editing.
2. Implement only the assigned scope.
3. Run focused tests and `make check`.
4. Render the candidate with the same profile and fixed seeds.
5. Inspect the contact sheet and individual strongest and weakest seeds.
6. Commit coherent changes and complete a result report if one was requested.
7. Stop without integrating or removing the worktree.

Passing tests does not approve the visual result.

## Review

From the coordinator checkout:

```sh
git log --oneline master..exp/<name>
git diff --stat master...exp/<name>
git diff master...exp/<name>
git -C ../go-graphics-worktrees/<name> status --short --branch
```

Review the hypothesis, commits, test results, fixed-seed comparison, visible
gains and regressions, strongest and weakest seeds, and the worker's
recommendation. Integration or discard requires explicit user approval.

## Integrate

Use the operation that matches the review decision.

To integrate the complete branch, run from a clean coordinator checkout:

```sh
git merge --no-ff exp/<name>
```

To integrate selected self-contained commits:

```sh
git cherry-pick <commit>
```

If wanted behavior is entangled with rejected changes, do not merge the whole
branch for convenience. Create a fresh integration branch/worktree from the
current `master` and reimplement the selected behavior there. Use the same
approach to combine selected behavior from multiple experiments.

After integration, run `make check` and repeat the relevant fixed-seed visual
comparison before considering the work complete.

## Conflicts

Git reports merge and cherry-pick conflicts. Inspect them normally:

```sh
git status
git diff --name-only --diff-filter=U
```

Resolve each file, stage it, then continue with `git merge --continue` or
`git cherry-pick --continue`. To abandon the attempted integration without
discarding branch work:

```sh
git merge --abort
git cherry-pick --abort
```

Do not hide conflicts with hard resets or force operations.

## Cleanup

After approved work has been integrated or deliberately discarded, ensure the
worker worktree is clean:

```sh
git -C ../go-graphics-worktrees/<name> status --short --branch
git worktree remove ../go-graphics-worktrees/<name>
git branch -d exp/<name>
```

If removal reports a dirty worktree, inspect and commit or preserve the files
before retrying. If branch deletion reports that the branch is not merged,
confirm whether commits still need integration or preservation. Do not make
`--force` or `git branch -D` the routine answer to either condition.

Use `git worktree list` to inspect registered worktrees. If a directory was
removed outside Git, inspect `git worktree prune --dry-run` before deciding
whether to run `git worktree prune`.
