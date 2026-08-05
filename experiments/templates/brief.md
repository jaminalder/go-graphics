# Experiment brief

- Name:
- Branch: `exp/<name>`
- Worktree: `../go-graphics-worktrees/<name>`
- Base commit:
- Stage:
- Profile: `preview`
- Fixed seeds: `1,2,3,5,8,13`

## Hypothesis

State the specific change and the observable result it should produce.

## Artistic purpose

Explain why the expected result improves the artwork rather than merely
changing it.

## Stage

Name the one stage this experiment may change.

## Preserve

List behavior and visual qualities that must remain stable.

## Scope

List the expected implementation, test, and documentation files.

## Out of scope

List explicit exclusions and tempting adjacent changes.

## Baseline

Record and run the fixed-seed baseline before implementation:

```sh
# Baseline command
```

Render the candidate with the same profile and seeds:

```sh
# Candidate command
```

Keep generated renders under this worktree's ignored `out/` directory.

## Required deliverables

- Focused implementation and tests for the permitted stage.
- Coherent commits containing the worker's scoped changes.
- Baseline and candidate renders using the same profile and fixed seeds.
- A visually inspected comparison contact sheet.
- A completed result report naming verification outcomes, commits, comparison
  artifacts, visual consequences, strongest and weakest seeds, recommendation,
  and behavior to keep or reject.
- Stop without merging or removing the worktree.

## Worker restrictions

Operate only inside the assigned worktree. Do not switch branches, create or
remove worktrees, merge, rebase, or modify `master`. Do not work outside the
assigned scope or modify another experiment's files.
