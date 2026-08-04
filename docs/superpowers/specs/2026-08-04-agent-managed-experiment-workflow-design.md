# Agent-Managed Experiment Workflow Design

## Goal

Add a tool-neutral workflow for running generative-art experiments in parallel
without exposing branch or worktree management to the user. One coordinator
session remains in a stable `master` checkout. Every writing worker receives a
dedicated branch, sibling worktree, brief, state record, output namespace, and
scope. Repository files and Git history remain the source of truth across
Claude Code, Codex, OpenCode, other coding agents, and human workers.

The workflow supports creation, parallel work, status, review, revision,
complete merge preparation, semantic partial integration, combinations,
archive, discard, and safe cleanup. It does not implement a proprietary agent
launcher or a large orchestration framework.

## Architecture

The lifecycle is a standard-library Go CLI separate from the artwork CLI:

```text
tools/experiment
    -> go run ./cmd/experiment
        -> internal/experiment
            -> git executable
            -> repository files
            -> shared Git locks
```

`tools/experiment` is the stable repository entry point. `cmd/experiment`
parses commands and prints human-readable or JSON output. The focused
`internal/experiment` package owns identity validation, repository discovery,
paths, state transitions, atomic records, locking, Git operations, safety
checks, verification, archive, and discard.

The implementation adds no third-party dependency and does not alter
`cmd/staticart`, sketch packages, or artwork rendering architecture.

## Repository Layout

```text
AGENTS.md                              canonical tool-neutral rules
CLAUDE.md                              @AGENTS.md plus Claude-specific notes
docs/EXPERIMENT-WORKFLOW.md            lifecycle operator guide
docs/AGENT-ORCHESTRATION.md            coordinator and worker guide
experiments/templates/brief.md         experiment brief template
experiments/templates/result.md        result report template
experiments/active/.gitkeep            tracked namespace marker
experiments/archive/.gitkeep           retained record namespace
cmd/experiment/                        CLI entry point and command parsing
internal/experiment/                   lifecycle implementation and tests
tools/experiment                       executable thin wrapper
```

Generated output is always ignored and isolated by identity:

```text
out/experiments/<piece>/<name>/
    baseline/
    candidate/
    contact-sheet.png
    metadata/
```

The output path is inside the assigned worktree, so independent experiments
cannot overwrite each other's renders even when they run the same commands.
Full-resolution images remain untracked. A worker may deliberately copy a
small review sheet into its experiment record when useful.

## Canonical Instructions

`AGENTS.md` becomes the concise canonical instruction file. It defines the
mandatory repository invariants, coordinator stability, one-writer-per-
worktree rule, experiment workflow, worker restrictions, visual verification,
and links to detailed documentation.

`CLAUDE.md` starts with `@AGENTS.md` and retains only Claude-specific context
or delegation guidance after the import. Its existing repository knowledge is
moved to the canonical file or linked documentation rather than duplicated.
Any future tool-specific instruction file must reference `AGENTS.md` and may
only add adapter details.

## Identity And Paths

An experiment ID is exactly two lower-case kebab-case path components:

```text
<piece>/<generating-relationship>
```

Each component begins and ends with an ASCII letter or digit and may contain
single hyphens. Slashes beyond the one separator, dots, whitespace, empty
components, path traversal, and Git-special ref syntax are rejected.

For `foam/hatching-by-subdivision-depth`:

```text
branch:   exp/foam/hatching-by-subdivision-depth
worktree: ../go-graphics-worktrees/foam--hatching-by-subdivision-depth
record:   experiments/active/foam--hatching-by-subdivision-depth/
output:   out/experiments/foam/hatching-by-subdivision-depth/
```

Integration IDs use the same validation and paths but have an
`integrate/<piece>/<name>` branch. Worktrees live in one predictable sibling
directory, never under the repository or another worktree.

## Experiment Records

Every experiment branch contains:

```text
experiments/active/<piece>--<name>/
    brief.md
    state.json
    result.md
    favorites.json
```

`state.json` contains schema version, ID, kind, branch, worktree path relative
to the coordinator root, base branch and commit, status, allowed stage, worker
tool/session, fixed seeds, render profile, output path, source experiments for
integrations, and UTC creation/update timestamps. A missing worker session is
valid. `favorites.json` starts as an empty array with a documented schema.

The branch-local record is authoritative while active. Commands discover
active experiments from `exp/*` and `integrate/*` refs plus Git worktree
metadata, then read records from those branches or worktrees. This avoids a
coordinator dirty tree and allows state to survive agent-product changes.

After merge or discard, retained brief, result, final state, favorites, and an
optional low-resolution contact sheet are copied to
`experiments/archive/<piece>--<name>/` and committed on `master`. The archive
commit occurs before source branch deletion.

## Briefs And Results

`create` renders `experiments/templates/brief.md` into the new worktree with:

- hypothesis and artistic purpose prompts;
- one permitted stage;
- preserved behavior;
- expected files and explicit exclusions;
- base commit, default preview profile, fixed comparison seeds, and editable
  baseline and candidate commands using the isolated output path;
- required implementation, tests, commits, renders, contact sheet, strongest
  and weakest seeds, reusable commits, accepted/rejected behavior, and possible
  combinations.

`result.md` records verification commands and outcomes, commit summary,
comparison artifacts, visual consequences, failure modes, strongest/weakest
seeds, recommendation, reusable behavior, rejected behavior, and combinations.
Passing tests is explicitly not artistic approval.

## State Machine

Supported states are:

```text
created
running
review-pending
revision-requested
integration-pending
integrating
merge-ready
merged
discarded
failed
```

Normal transitions are explicit:

```text
created -> running | discarded | failed
running -> review-pending | revision-requested | discarded | failed
review-pending -> revision-requested | integration-pending | merge-ready |
                  discarded | failed
revision-requested -> running | discarded | failed
integration-pending -> integrating | discarded | failed
integrating -> review-pending | merge-ready | discarded | failed
merge-ready -> merged | revision-requested | failed
failed -> revision-requested | running | discarded
```

`merged` and `discarded` are terminal in active records. Archiving preserves
the terminal state. Integration experiments start at `integration-pending`;
ordinary experiments start at `created`. Invalid transitions fail with the
current status and allowed destinations. State writes use a temporary file,
file sync where practical, and atomic rename under an experiment lock.

## Commands

The stable interface is:

```sh
./tools/experiment create --piece foam --name hatching-by-subdivision-depth
./tools/experiment list [--json]
./tools/experiment show foam/hatching-by-subdivision-depth [--json]
./tools/experiment path foam/hatching-by-subdivision-depth
./tools/experiment state foam/hatching-by-subdivision-depth review-pending
./tools/experiment verify foam/hatching-by-subdivision-depth
./tools/experiment prepare-integration --name foam/depth-hatching-v1 \
    --source foam/hatching-by-subdivision-depth
./tools/experiment archive foam/hatching-by-subdivision-depth
./tools/experiment discard foam/hatching-by-subdivision-depth
```

`create` accepts optional `--base`, `--stage`, `--profile`, `--seeds`, and
worker metadata. It verifies that the invocation is from the canonical
coordinator checkout, that checkout is on `master`, and its tracked/untracked
state is clean except ignored output. It inspects refs and worktrees without
fetching or changing history, records current `master`, creates the branch and
worktree without switching the coordinator, writes and commits the initial
record on the experiment branch, and prints the ID, branch, worktree, brief,
output, and exact worker restriction prompt.

`list`, `show`, and `path` are read-only. They reconcile branch, record,
directory, and Git worktree metadata and report missing or stale components.
They do not silently recreate or delete anything.

`verify` checks branch identity, expected worktree, record consistency,
committed state, clean worktree, required files, declared output directories,
verification evidence, current base drift, and configurable repository test
commands. It records verification only when explicitly requested by a writing
operation; the default verification report is read-only.

`prepare-integration` creates a fresh branch/worktree from current `master`
and writes an integration brief containing source IDs and commits, selected
behavior, rejected behavior, dependencies, preserved stable behavior, and
comparison requirements. It never merges a complete source branch
automatically.

`archive` requires a terminal or explicitly archiveable review state, clean
and committed experiment worktree, and clean `master`. It copies retained
records to `master`, commits them, removes the clean worktree, and deletes the
branch. `discard` first writes and commits a discarded result/state on the
experiment branch, archives the record on `master`, then performs the same
safe cleanup. Both are idempotent when the matching archive already exists
and active resources are gone.

There is no implicit force option. Recovery documentation gives an explicit
inspection and preservation path for dirty or stale worktrees before cleanup.

## Complete And Semantic Integration

A complete merge is a coordinator procedure, not a shortcut in a worker
worktree:

1. Confirm explicit user approval and experiment identity.
2. Verify committed state, tests, required comparisons, and base drift.
3. Create a temporary integration worktree from current `master`.
4. Apply the accepted experiment, normally as a squash unless reusable commits
   should retain identity.
5. Run `make check` and fixed-seed renders in the integration worktree.
6. Mark the integration `merge-ready`.
7. Merge into `master`, verify `master`, mark the source `merged`, archive,
   remove the worktree, and delete the branch.

Partial merge and combination requests always create a fresh integration
experiment from current `master`. Clearly isolated prerequisite commits may be
cherry-picked. Entangled selected behavior is reimplemented against current
architecture rather than merging rejected behavior for convenience. Combining
experiments specifies selected behavior from each source and rejects all
unselected changes. The integration returns to `review-pending`; it requires a
second explicit approval before complete merge.

## Base Drift

Every record pins its original base commit. Before integration, commands show
the current `master`, ahead/behind relationship, merge base, changed paths on
both sides, and whether source-touched paths overlap master changes. The tool
reports evidence but does not automatically rebase. Small independent drift
may use a normal merge or cherry-pick. Material architecture drift should use
semantic reapplication from current `master`.

Child experiments are explicit: `create --base-experiment <id>` starts from
the parent branch tip and records that dependency. They are not presented as
independent experiments from `master`. A later integration experiment is the
preferred model when only selected behavior depends on another experiment.

## Locking And Concurrency

Locks live below the shared Git common directory, making them visible to every
worktree:

```text
<git-common-dir>/experiment-locks/global.lock/
<git-common-dir>/experiment-locks/<escaped-id>.lock/
```

Atomic directory creation acquires a lock. Lock metadata records PID,
hostname, command, and acquisition time. Commands time out with owner details.
Stale locks are reported and require a documented, explicit recovery command;
they are never silently stolen solely because of age.

The global lock protects branch/worktree creation, archive writes on master,
integration identity, and cleanup. Experiment locks protect state and deletion
of one experiment. Ordinary source editing in separate worktrees is never
locked.

The default writing-worker limit is two, configurable through
`EXPERIMENT_MAX_WRITERS` and an optional command flag. Running states count as
writers; integrations count while `integrating`. The coordinator may choose a
lower limit based on rendering CPU/memory pressure or agent limits.

## Safety And Failure Handling

All mutating operations validate assumptions immediately before each Git
change. The tool never invokes `git reset --hard`, `git clean -fd`, forced
branch deletion, or forced worktree removal. It refuses to:

- operate from a nested or non-coordinator worktree for coordinator commands;
- switch the coordinator away from `master`;
- reuse an existing path or unrelated branch;
- remove a dirty, locked, or mismatched worktree;
- delete a checked-out branch;
- overwrite a differing archive record;
- silently repair stale Git metadata;
- change another experiment's records.

Partial creation failures are reported with the resources created and exact
safe recovery commands. A branch without a worktree or a missing directory
with registered Git metadata appears as `stale` diagnostics in status output.
Cleanup uses `git worktree prune --dry-run` for diagnosis; pruning is an
explicit recovery action after inspection.

## Agent Orchestration

The generic delegation concept is:

```text
delegate(role, worktreePath, branch, prompt, permissions)
```

The repository implements preparation and records, not universal process
spawning. The coordinator creates the lifecycle object, receives its paths,
and invokes the active tool's agent mechanism when available. The worker is
always told its ID, worktree, branch, brief, scope, and prohibitions. A tool
that cannot start a worktree-isolated worker falls back to the coordinator
performing the work itself with commands scoped to that worktree while leaving
the main checkout untouched.

The orchestration guide truthfully compares Claude Code, Codex, and OpenCode
capabilities without claiming identical APIs. It also defines a read-only
reviewer role that may inspect diffs, tests, benchmarks, renders, sheets, and
reports but may not edit, merge, remove resources, update goldens, or grant
artistic approval.

Workers read `AGENTS.md`, their brief, and the relevant sketch specification;
render the baseline; implement only the assigned stage; add focused tests;
commit coherent work; render fixed-seed comparisons; produce a contact sheet;
write results; set `review-pending`; and stop without merging.

## Review And Revision

At `review-pending`, the coordinator presents the hypothesis, stage, branch,
worktree, commits, baseline and candidate sheets, strongest and weakest seeds,
visual consequences, failure modes, worker recommendation, and available
actions: merge, revise, merge only a selected behavior, combine, keep open, or
discard.

Revision appends dated user feedback to the brief, changes state to
`revision-requested`, and reuses the branch/worktree. The same worker is
preferred, but a replacement receives the original brief, current result,
feedback, current branch, and current worktree. Revised comparisons replace
only that experiment's candidate output and return to `review-pending`.

## Testing

Tests create temporary Git repositories with local identity configuration and
invoke lifecycle services or the built CLI against them. They never create
refs or worktrees in the developer repository.

Focused coverage includes:

- creation and coordinator branch stability;
- duplicate and invalid identities;
- branch and worktree existence;
- generated brief, record, and isolated output paths;
- valid and invalid state transitions;
- atomic concurrent state updates and duplicate creation locking;
- dirty coordinator and dirty experiment protection;
- stale directory and Git metadata diagnostics;
- safe archive and discard, including retained master records;
- integration creation from current master with pinned sources;
- base drift reporting;
- idempotent archive/discard behavior;
- maximum writer enforcement.

`make check` remains the repository gate. Tests use dependency injection for
the clock and command runner where it improves determinism, but Git itself is
exercised in temporary repositories for branch/worktree safety behavior.

## Pilot

The initial pilot is lifecycle-only and creates no artistic implementation:

```text
foam/hatching-spacing-variation
foam/color-order-variation
```

The pilot creates two real branches and simultaneous sibling worktrees from
the same stable `master`, confirms independent records and output directories,
runs verification independently, records harmless no-change results, and puts
both into `review-pending`. It then archives both records onto `master` and
safely removes their worktrees and branches. The pilot report demonstrates
that the coordinator remained on `master` and no artwork changes were merged.

## Acceptance Criteria

- The coordinator checkout remains clean and on `master` during experiments.
- Every active writer has one branch, one sibling worktree, one brief, one
  state record, one output namespace, and one scope.
- Two independent experiments can run and be reviewed concurrently.
- State and records survive switching agent products.
- Status reports missing, dirty, stale, drifted, and locked resources without
  destructive repair.
- Complete merges are verified in integration before reaching `master`.
- Partial and combined work use semantic integration and explicit approval.
- Archive/discard preserve useful records on `master` before safe cleanup.
- Core lifecycle behavior is standard Git, files, shell entry points, and a
  small standard-library Go implementation.
- Tool-specific delegation remains an adapter over the generic lifecycle.
