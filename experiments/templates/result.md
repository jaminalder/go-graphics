# Experiment result: {{.ID}}

- Created: {{.CreatedAt}}
- Base commit: `{{.BaseCommit}}`
- Stage: `{{.Stage}}`
- Profile: `{{.Profile}}`
- Fixed seeds: `{{.Seeds}}`
- Source experiments: {{.SourceExperiments}}
- Output path: `{{.OutputPath}}`

## Summary

Describe what changed and whether it supported the hypothesis.

## Verification

Record each command and its outcome. Passing tests is not artistic approval.

## Commits

List commit hashes and the coherent change represented by each.

## Comparisons

- Baseline command: `{{.BaselineCommand}}`
- Candidate command: `{{.CandidateCommand}}`
- Baseline artifacts:
- Candidate artifacts:
- Contact sheet:

## Visual consequences and failure modes

Describe the visible gains, losses, regressions, and conditions where the
candidate fails.

## Strongest seeds

Name the seeds and explain what succeeds.

## Weakest seeds

Name the seeds and explain what fails.

## Recommendation

Recommend merge, revise, selected-behavior integration, combination, keep
open, or discard. The user makes the final artistic and lifecycle decision.

## Reusable

List independently reusable commits or behavior.

## Keep

List behavior worth retaining.

## Reject

List behavior that should not be integrated.

## Combinations

List promising combinations with other experiments and identify selected and
rejected behavior from each source.
