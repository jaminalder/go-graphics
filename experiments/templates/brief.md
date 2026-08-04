# Experiment {{.ID}}

- Created: {{.CreatedAt}}
- Base commit: `{{.BaseCommit}}`
- Stage: `{{.Stage}}`
- Profile: `{{.Profile}}`
- Fixed seeds: `{{.Seeds}}`
- Source experiments: {{.SourceExperiments}}
- Output path: `{{.OutputPath}}`

## Hypothesis

State the specific change and the observable result it should produce.

## Artistic purpose

Explain why the expected result improves the artwork rather than merely
changing it.

## Stage

Work only in `{{.Stage}}`.

## Preserve

List behavior and visual qualities that must remain stable.

## Scope

List the expected implementation and record files.

## Out of scope

List explicit exclusions and tempting adjacent changes.

## Baseline

Run the fixed-seed baseline before implementation:

```sh
{{.BaselineCommand}}
```

Render the candidate with the same profile and seeds:

```sh
{{.CandidateCommand}}
```

Keep all generated renders under `{{.OutputPath}}`.

## Required deliverables

- Focused implementation and tests for the permitted stage.
- Coherent commits containing the worker's implementation and record changes.
- Baseline and candidate renders using profile `{{.Profile}}` and fixed seeds
  `{{.Seeds}}`.
- A visually inspected comparison contact sheet.
- A completed `result.md` naming verification outcomes, commits, comparison
  artifacts, visual consequences, failure modes, strongest and weakest seeds,
  recommendation, reusable behavior, kept/rejected behavior, and possible
  combinations.
- State set to `review-pending`; stop without merging.

## Worker restrictions

Operate only inside this worktree. Do not switch branches. Do not create or
remove worktrees. Do not merge, rebase, or modify master. Do not work outside
the assigned scope. Do not modify another experiment's files.
