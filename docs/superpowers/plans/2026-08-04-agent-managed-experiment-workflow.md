# Agent-Managed Experiment Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a safe, tool-neutral Git branch/worktree lifecycle that lets one coordinator create, delegate, review, integrate, archive, and discard parallel generative-art experiments.

**Architecture:** Add a standard-library Go lifecycle package and a small `cmd/experiment` CLI behind the stable `tools/experiment` wrapper. Active records live and are committed on experiment branches, generated output stays in each worktree's ignored namespace, and terminal records are committed to `master` before safe cleanup. Repository documentation defines agent roles and tool-specific delegation only as adapters over this generic layer.

**Tech Stack:** Go 1.26.5 standard library, Git CLI, POSIX shell wrapper, JSON, Markdown, existing Make/golangci-lint checks.

---

## File Map

- Create `AGENTS.md`: canonical concise repository and experiment rules.
- Modify `CLAUDE.md`: import `AGENTS.md`, retain only Claude-specific adapter notes.
- Modify `.gitignore`: ignore isolated experiment outputs while preserving optional committed review sheets.
- Modify `docs/ARCHITECTURE.md`: link the workflow and record the lifecycle boundary decision.
- Create `docs/EXPERIMENT-WORKFLOW.md`: operator lifecycle, safety, recovery, and integration guide.
- Create `docs/AGENT-ORCHESTRATION.md`: coordinator, worker, reviewer, integration, adapters, and fallback.
- Create `experiments/templates/brief.md`: canonical worker brief source.
- Create `experiments/templates/result.md`: canonical result report source.
- Create `experiments/active/.gitkeep`: tracked active namespace.
- Create `experiments/archive/.gitkeep`: tracked archive namespace.
- Create `internal/experiment/model.go`: IDs, states, records, summaries, transition graph.
- Create `internal/experiment/identity.go`: slug validation and deterministic path derivation.
- Create `internal/experiment/git.go`: narrowly wrapped Git execution and repository discovery.
- Create `internal/experiment/lock.go`: shared Git-directory locks.
- Create `internal/experiment/record.go`: template rendering, JSON reads, atomic writes, record commits.
- Create `internal/experiment/manager.go`: coordinator manager construction and common safety checks.
- Create `internal/experiment/create.go`: ordinary and child experiment creation.
- Create `internal/experiment/inspect.go`: list, show, path, reconciliation, stale diagnostics.
- Create `internal/experiment/state.go`: validated committed state transitions.
- Create `internal/experiment/verify.go`: worktree checks, test execution, artifacts, and base drift.
- Create `internal/experiment/integration.go`: semantic integration worktree preparation.
- Create `internal/experiment/cleanup.go`: archive and discard transaction.
- Create `internal/experiment/testrepo_test.go`: isolated temporary Git repository fixture.
- Create focused `internal/experiment/*_test.go` files matching each lifecycle responsibility.
- Create `cmd/experiment/main.go`: command parsing, text/JSON presentation, exit behavior.
- Create `cmd/experiment/main_test.go`: CLI parsing and temporary-repository smoke tests.
- Create `tools/experiment`: executable repository-local wrapper.
- Create `docs/experiments/2026-08-04-lifecycle-pilot.md`: recorded two-worktree pilot evidence.

### Task 1: Canonical Instructions, Templates, And Output Namespace

**Files:**
- Create: `AGENTS.md`
- Modify: `CLAUDE.md`
- Modify: `.gitignore`
- Create: `experiments/templates/brief.md`
- Create: `experiments/templates/result.md`
- Create: `experiments/active/.gitkeep`
- Create: `experiments/archive/.gitkeep`

- [ ] **Step 1: Add the canonical instructions**

Write `AGENTS.md` with mandatory rules first: remain on `master` in the coordinator; create experiments only through `./tools/experiment`; never share writing checkouts; never switch an assigned worktree's branch; keep generated output under the record's output path; commit coherent worker work and records; visually inspect fixed-seed sweeps; stop at `review-pending`; require explicit merge/discard approval; prohibit hard resets, cleans, force removal, and direct worker merges. Preserve the existing project commands, sketch-specific cautions, invariants, package boundaries, and documentation map currently in `CLAUDE.md`.

- [ ] **Step 2: Make Claude import the canonical file**

Replace duplicated repository rules in `CLAUDE.md` with:

```markdown
@AGENTS.md

# Claude Code Adapter

Use Claude Code's supported subagent mechanism only after
`./tools/experiment create` has prepared the branch, worktree, brief, and
record. Set the worker's working directory to the printed worktree when the
tool supports it. Otherwise perform the work from the coordinator session with
every command explicitly scoped to that worktree. Never use a Claude-managed
worktree as a substitute for the repository lifecycle.
```

- [ ] **Step 3: Add the brief and result templates**

Create templates using Go `text/template` fields `.ID`, `.Stage`,
`.BaseCommit`, `.Profile`, `.Seeds`, `.BaselineCommand`, `.CandidateCommand`,
`.OutputPath`, `.SourceExperiments`, and `.CreatedAt`. Include every required
brief/result section from the approved design, plus this worker restriction:

```text
Operate only inside this worktree. Do not switch branches. Do not create or
remove worktrees. Do not merge, rebase, or modify master. Do not work outside
the assigned scope. Do not modify another experiment's files.
```

- [ ] **Step 4: Isolate generated output**

Append to `.gitignore`:

```gitignore
# Experiment renders are isolated per worktree and never committed here.
/out/experiments/
```

Keep `experiments/**/contact-sheet.png` trackable for optional low-resolution review artifacts.

- [ ] **Step 5: Verify and commit**

Run: `git diff --check`

Expected: no output.

Run: `git status --short`

Expected: only the files in this task.

Commit:

```sh
git add AGENTS.md CLAUDE.md .gitignore experiments
git commit -m "docs: establish canonical experiment instructions"
```

### Task 2: Identity, State, And Path Model

**Files:**
- Create: `internal/experiment/model.go`
- Create: `internal/experiment/identity.go`
- Create: `internal/experiment/model_test.go`
- Create: `internal/experiment/identity_test.go`

- [ ] **Step 1: Write identity and transition tests**

Use table-driven tests that assert `ParseID("foam/hatching-by-depth")`
succeeds; uppercase, leading/trailing/repeated hyphens, dots, whitespace,
extra slashes, empty components, and `..` fail. Assert branch, record, output,
and sibling worktree paths exactly. Assert all transitions in the approved
state graph and reject `created -> merged`, transitions out of terminal states,
and unknown statuses.

Key assertions:

```go
id, err := ParseID("foam/hatching-by-depth")
if err != nil { t.Fatal(err) }
if got := id.ExperimentBranch(); got != "exp/foam/hatching-by-depth" {
    t.Fatalf("branch = %q", got)
}
if got := id.RecordDir(); got != "experiments/active/foam--hatching-by-depth" {
    t.Fatalf("record = %q", got)
}
if !CanTransition(StatusCreated, StatusRunning) {
    t.Fatal("created must transition to running")
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/experiment -run 'Test(ParseID|DerivedPaths|StateTransitions)'`

Expected: FAIL because the package/types do not exist.

- [ ] **Step 3: Implement the model**

Define:

```go
type Kind string
const (
    KindExperiment  Kind = "experiment"
    KindIntegration Kind = "integration"
)

type Status string
const (
    StatusCreated            Status = "created"
    StatusRunning            Status = "running"
    StatusReviewPending      Status = "review-pending"
    StatusRevisionRequested  Status = "revision-requested"
    StatusIntegrationPending Status = "integration-pending"
    StatusIntegrating        Status = "integrating"
    StatusMergeReady         Status = "merge-ready"
    StatusMerged             Status = "merged"
    StatusDiscarded          Status = "discarded"
    StatusFailed             Status = "failed"
)

type Worker struct { Tool string `json:"tool"`; Session *string `json:"session"` }
type Verification struct {
    CheckedAt *time.Time `json:"checked_at,omitempty"`
    Commit string `json:"commit,omitempty"`
    Command string `json:"command,omitempty"`
    Passed bool `json:"passed"`
}
type State struct {
    SchemaVersion int `json:"schema_version"`
    ID string `json:"id"`
    Kind Kind `json:"kind"`
    Branch string `json:"branch"`
    Worktree string `json:"worktree"`
    BaseBranch string `json:"base_branch"`
    BaseCommit string `json:"base_commit"`
    Status Status `json:"status"`
    Stage string `json:"stage"`
    Worker Worker `json:"worker"`
    Seeds []uint64 `json:"seeds"`
    Profile string `json:"profile"`
    Output string `json:"output"`
    Sources []Source `json:"sources,omitempty"`
    Verification Verification `json:"verification"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

Implement `ParseStatus`, `CanTransition`, `AllowedTransitions`, and an `ID`
value with deterministic branch/path methods. Use `regexp` with
`^[a-z0-9](?:[a-z0-9]|-(?=[a-z0-9]))*$` semantics implemented without
lookahead because Go regexp does not support it: validate allowed bytes, then
reject leading, trailing, and repeated hyphens.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./internal/experiment -run 'Test(ParseID|DerivedPaths|StateTransitions)'`

Expected: PASS.

```sh
git add internal/experiment/model.go internal/experiment/identity.go internal/experiment/*_test.go
git commit -m "feat: model experiment identity and state"
```

### Task 3: Isolated Git Fixture, Repository Discovery, And Locks

**Files:**
- Create: `internal/experiment/testrepo_test.go`
- Create: `internal/experiment/git.go`
- Create: `internal/experiment/git_test.go`
- Create: `internal/experiment/lock.go`
- Create: `internal/experiment/lock_test.go`
- Create: `internal/experiment/manager.go`

- [ ] **Step 1: Build a temporary-repository fixture**

Implement `newTestRepo(t)` with `t.TempDir()`, `git init -b master`, local
`user.name`/`user.email`, minimum template files, `.gitignore`, and an initial
commit. Return root and a helper that runs Git with `cmd.Dir = root`. Never use
the developer repository or global Git configuration.

- [ ] **Step 2: Write failing discovery and lock tests**

Assert manager discovery returns the first `git worktree list --porcelain`
entry as coordinator root, resolves the absolute shared common Git directory,
and rejects coordinator-only mutation from a linked worktree. Concurrently
acquire the same experiment lock and assert the second acquisition times out
with owner metadata while a different ID lock succeeds.

- [ ] **Step 3: Run tests to verify failure**

Run: `go test ./internal/experiment -run 'Test(Discover|Coordinator|Lock)'`

Expected: FAIL with undefined manager/lock functions.

- [ ] **Step 4: Implement narrow Git execution**

Define a runner that always uses argument slices, captures stdout/stderr, and
returns errors containing the command's meaningful stderr without shell
evaluation:

```go
type gitRunner struct { dir string }
func (g gitRunner) run(ctx context.Context, args ...string) (string, error)
func (g gitRunner) lines(ctx context.Context, args ...string) ([]string, error)
```

Parse `git worktree list --porcelain` into `WorktreeInfo{Path, HEAD, Branch,
Bare, Prunable}` records. `NewManager(cwd)` finds the current toplevel, shared
common directory, coordinator root, and templates root without changing CWD.

- [ ] **Step 5: Implement shared locks**

Acquire `<common>/experiment-locks/<escaped>.lock` using `os.Mkdir`; write
`owner.json` containing PID, hostname, command, and UTC time. Retry with a
short sleep until context timeout. Release only the lock acquired by this
process. Do not infer staleness or steal locks by age.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/experiment -run 'Test(Discover|Coordinator|Lock)'`

Expected: PASS.

```sh
git add internal/experiment
git commit -m "feat: discover repositories and coordinate lifecycle locks"
```

### Task 4: Atomic Records And Experiment Creation

**Files:**
- Create: `internal/experiment/record.go`
- Create: `internal/experiment/create.go`
- Create: `internal/experiment/create_test.go`

- [ ] **Step 1: Write failing creation tests**

In independent temporary repositories, test that `Create`:

- keeps the coordinator on `master` at its original HEAD;
- creates `exp/foam/hatching-spacing-variation` from that HEAD;
- creates sibling `go-graphics-worktrees/foam--hatching-spacing-variation`;
- creates and commits all four record files;
- records relative worktree/output paths, default seeds `1,2,3,5,8,13`, and
  profile `preview`;
- prints a worker handoff with exact worktree, branch, and brief;
- rejects dirty coordinator state, duplicate ID/ref/path, invalid stage, and a
  third running writer when max writers is two;
- creates a child from a named parent tip only with `BaseExperiment` recorded.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/experiment -run 'TestCreate'`

Expected: FAIL because `Create` is undefined.

- [ ] **Step 3: Implement atomic record helpers**

Implement `readState`, `writeJSONAtomic`, `renderTemplate`, and
`commitRecord`. JSON uses `json.MarshalIndent` plus a trailing newline. Atomic
writes create a sibling temporary file with mode `0644`, sync, close, and
rename. Record commits stage only the experiment directory and use explicit
messages such as `experiment: create foam/hatching-spacing-variation`.

- [ ] **Step 4: Implement creation as an ordered transaction**

Define `CreateOptions` and `Created`:

```go
type CreateOptions struct {
    Piece, Name, BaseBranch, BaseExperiment, Stage, Profile string
    Seeds []uint64
    WorkerTool string
    WorkerSession *string
    MaxWriters int
    LifecycleOnly bool
}
type Created struct {
    State State
    WorktreePath, BriefPath, OutputPath, WorkerInstruction string
}
```

Under the global and ID locks: require coordinator `master`; require `git
status --porcelain` empty; validate all options; inspect refs/worktrees/paths;
record HEAD; run `git branch <branch> <base>` then `git worktree add <path>
<branch>`; make the output subdirectories; render all records; commit only the
record directory in the experiment worktree. If a later step fails, report
which branch/path now exists and safe manual recovery commands; do not force a
rollback.

- [ ] **Step 5: Run focused tests and commit**

Run: `go test ./internal/experiment -run 'TestCreate'`

Expected: PASS.

```sh
git add internal/experiment/record.go internal/experiment/create.go internal/experiment/create_test.go
git commit -m "feat: create isolated experiment worktrees"
```

### Task 5: Inspection, Reconciliation, And Committed State Changes

**Files:**
- Create: `internal/experiment/inspect.go`
- Create: `internal/experiment/inspect_test.go`
- Create: `internal/experiment/state.go`
- Create: `internal/experiment/state_test.go`

- [ ] **Step 1: Write failing inspection tests**

Create two experiments and assert deterministic ID-sorted listing, exact
`Show` state, and absolute `Path`. Remove one directory without pruning Git
metadata and assert a stale diagnostic rather than recreation. Delete a branch
record in a fixture and assert a missing-record diagnostic. Ensure read-only
commands do not change refs, files, or coordinator HEAD.

- [ ] **Step 2: Write failing transition tests**

Move `created -> running -> review-pending`, checking each transition is an
isolated commit on the assigned branch and timestamps advance through an
injected clock. Reject invalid transitions and mismatched branch/worktree.
Launch two updates to one state and assert locking prevents a corrupt JSON
file.

- [ ] **Step 3: Run tests to verify failure**

Run: `go test ./internal/experiment -run 'Test(List|Show|Path|Reconcile|SetState)'`

Expected: FAIL with missing methods.

- [ ] **Step 4: Implement discovery and diagnostics**

List refs below `refs/heads/exp/` and `refs/heads/integrate/`, map them to
worktree porcelain records, read `state.json` from a present worktree or via
`git show <branch>:<path>`, and return:

```go
type Diagnostic struct { Code, Message string }
type Experiment struct {
    State State
    WorktreePath string
    Diagnostics []Diagnostic
}
```

Use diagnostic codes `missing-worktree-directory`, `stale-worktree-metadata`,
`missing-record`, `branch-mismatch`, `path-mismatch`, and `dirty-worktree`.

- [ ] **Step 5: Implement committed transitions**

Under the ID lock, re-read current state, validate `CanTransition`, update UTC
time, atomically write, and commit only `state.json` in that worktree. Return
the new commit. Do not stage implementation files or silently accept an
invalid branch.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/experiment -run 'Test(List|Show|Path|Reconcile|SetState)'`

Expected: PASS.

```sh
git add internal/experiment/inspect.go internal/experiment/inspect_test.go internal/experiment/state.go internal/experiment/state_test.go
git commit -m "feat: inspect experiments and persist state transitions"
```

### Task 6: Verification And Base Drift

**Files:**
- Create: `internal/experiment/verify.go`
- Create: `internal/experiment/verify_test.go`

- [ ] **Step 1: Write failing verification tests**

Assert verification refuses dirty worktrees and uncommitted records; runs a
fixture verification command in the experiment worktree; reports missing
baseline/candidate/contact sheet for an artistic experiment; permits declared
lifecycle-only experiments without image artifacts; records pass/fail,
command, commit, and timestamp when requested; and reports original/current
master commits plus overlapping changed paths after both branches change the
same file.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/experiment -run 'TestVerify'`

Expected: FAIL because verification is undefined.

- [ ] **Step 3: Implement verification**

Define:

```go
type VerifyOptions struct { Command []string; Record bool }
type Drift struct {
    BaseCommit, CurrentMaster, MergeBase string
    MasterPaths, ExperimentPaths, Overlap []string
}
type VerifyReport struct {
    ID, Branch, Commit string
    Clean, RecordsPresent, ArtifactsPresent, TestsPassed bool
    Command string
    Drift Drift
    Diagnostics []Diagnostic
}
```

Default command is `make check`. Execute directly with `exec.CommandContext`
in the worktree, streaming/capturing output. Artifact checks require non-empty
baseline and candidate directories and `contact-sheet.png` unless state marks
the experiment lifecycle-only. Calculate paths with `git diff --name-only` and
set intersection. Never rebase or mutate history. If `Record` is true, update
verification in state and commit it through the state-record helper.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./internal/experiment -run 'TestVerify'`

Expected: PASS.

```sh
git add internal/experiment/verify.go internal/experiment/verify_test.go
git commit -m "feat: verify experiment work and report base drift"
```

### Task 7: Semantic Integration Preparation

**Files:**
- Create: `internal/experiment/integration.go`
- Create: `internal/experiment/integration_test.go`

- [ ] **Step 1: Write failing integration tests**

Create two source experiments, advance master, then prepare
`foam/depth-hatching-v1`. Assert the branch is
`integrate/foam/depth-hatching-v1`, its base is current master rather than a
source branch, its state starts `integration-pending`, each source ID/commit is
pinned, and its brief has explicit Keep/Reject/Preserve sections. Assert no
source implementation commit is merged or cherry-picked. Reject missing,
duplicate, discarded, or uncommitted source records.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/experiment -run 'TestPrepareIntegration'`

Expected: FAIL because preparation is undefined.

- [ ] **Step 3: Implement integration preparation**

Define `IntegrationOptions{Name, Sources, Stage, Keep, Reject, Preserve,
Profile, Seeds}`. Resolve each source under locks, pin its branch tip, then
reuse the creation transaction with `KindIntegration`, current `master` as
base, `integrate/` branch naming, and initial `integration-pending`. Render an
integration-specific brief; do not execute merge/cherry-pick/rebase commands.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./internal/experiment -run 'TestPrepareIntegration'`

Expected: PASS.

```sh
git add internal/experiment/integration.go internal/experiment/integration_test.go
git commit -m "feat: prepare semantic integration experiments"
```

### Task 8: Safe Archive And Discard

**Files:**
- Create: `internal/experiment/cleanup.go`
- Create: `internal/experiment/cleanup_test.go`

- [ ] **Step 1: Write failing archive/discard tests**

Assert archive copies brief/result/state/favorites and optional sheet to
`experiments/archive/<flat-id>` on master, commits before removing worktree,
deletes the now-unchecked branch, and leaves master otherwise unchanged.
Assert discard first commits `discarded` state/result, then archives and
cleans. Refuse dirty coordinator, dirty experiment, active lifecycle lock,
branch mismatch, missing retained files, and differing existing archive.
Assert repeating a completed archive/discard is a no-op success.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/experiment -run 'Test(Archive|Discard)'`

Expected: FAIL because cleanup methods are undefined.

- [ ] **Step 3: Implement archive transaction**

Under global and ID locks: require canonical clean `master`; reconcile the
experiment; require clean assigned worktree and allowed status; copy only the
retained allowlist; atomically write final archive state; commit the archive
directory on master; run non-forced `git worktree remove <path>`; verify the
branch is no longer checked out; run `git branch -d <branch>`. If cleanup after
the archive commit fails, return the archive commit and exact remaining safe
commands without reverting it.

- [ ] **Step 4: Implement discard transaction**

Require explicit experiment ID. If not already discarded, append a dated
discard outcome to `result.md`, transition to `discarded`, and commit those two
record files on the experiment branch. Call the same archive transaction. Do
not expose a force flag.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/experiment -run 'Test(Archive|Discard)'`

Expected: PASS.

```sh
git add internal/experiment/cleanup.go internal/experiment/cleanup_test.go
git commit -m "feat: archive and discard experiments safely"
```

### Task 9: Repository CLI And Stable Wrapper

**Files:**
- Create: `cmd/experiment/main.go`
- Create: `cmd/experiment/main_test.go`
- Create: `tools/experiment`
- Modify: `Makefile`

- [ ] **Step 1: Write failing CLI tests**

Test `run(ctx, args, stdout, stderr, cwd)` rather than spawning for every case.
Cover help, unknown command, invalid flags, create text output, list/show JSON,
path-only output, state, verify with `--record`, integration sources repeated
via flags, archive, discard, and non-zero errors. Assert JSON decodes rather
than comparing whitespace.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./cmd/experiment`

Expected: FAIL because the command does not exist.

- [ ] **Step 3: Implement command parsing and presentation**

Use one `flag.FlagSet` per subcommand with `ContinueOnError`. Support:

```text
create --piece --name [--base] [--base-experiment] [--stage] [--profile]
       [--seeds] [--worker-tool] [--worker-session] [--max-writers]
       [--lifecycle-only]
list [--json]
show <id> [--json]
path <id>
state <id> <status>
verify <id> [--record] [--command "make check"]
prepare-integration --name <id> --source <id>... [--stage]
                    [--keep text] [--reject text] [--preserve text]
archive <id>
discard <id>
```

Parse comma/range-free seeds as comma-separated unsigned integers with clear
errors. Text `create` output includes the exact required worker instruction.
`list --json` emits a stable array; `show --json` emits state and diagnostics.

- [ ] **Step 4: Add the wrapper and Make target**

Create executable `tools/experiment`:

```sh
#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec go run "$repo/cmd/experiment" "$@"
```

Add `experiment-test` to `.PHONY` and:

```make
experiment-test: ## Test the experiment lifecycle tooling
	go test ./internal/experiment ./cmd/experiment
```

- [ ] **Step 5: Run tests and smoke help**

Run: `go test ./cmd/experiment ./internal/experiment`

Expected: PASS.

Run: `./tools/experiment help`

Expected: usage containing all nine lifecycle commands.

- [ ] **Step 6: Commit**

```sh
git add cmd/experiment tools/experiment Makefile
git commit -m "feat: expose the experiment lifecycle CLI"
```

### Task 10: Workflow And Orchestration Documentation

**Files:**
- Create: `docs/EXPERIMENT-WORKFLOW.md`
- Create: `docs/AGENT-ORCHESTRATION.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `README.md`

- [ ] **Step 1: Write the lifecycle operator guide**

Document create/list/show/path/state/verify/integration/archive/discard with
copy-pastable commands. Include the state graph; branch/worktree/record/output
naming; fixed seeds; review and revision; complete merge in a temporary
integration; semantic partial merge; combining experiments; child versus
independent experiments; base drift; maximum writers; locks; dirty and stale
failure recovery; non-force cleanup; and archive-on-master behavior.

- [ ] **Step 2: Write the orchestration guide**

Define coordinator, writing worker, read-only reviewer, and integration worker
permissions. Include the exact generic delegation signature and worker prompt.
For Claude Code, Codex, and OpenCode, provide a capability table with cautious
wording: adapter capabilities depend on the installed version; use native
subagents only when they can be constrained to the prepared worktree; otherwise
use the coordinator fallback. State the default maximum of two writers and
resource considerations.

- [ ] **Step 3: Update architecture and README links**

Add the lifecycle package/CLI to the layout as development tooling outside the
artwork dependency graph. Add a decision-log entry: ordinary Git/files plus a
small Go CLI are authoritative; agent launchers are adapters. Link both guides
and `AGENTS.md` from `README.md`.

- [ ] **Step 4: Validate docs against CLI**

Run: `./tools/experiment help`

Compare every documented command and flag to actual help. Search for obsolete
claims that `CLAUDE.md` is canonical or that worktrees live inside the repo.

Run: `git diff --check`

Expected: no output.

- [ ] **Step 5: Commit**

```sh
git add docs/EXPERIMENT-WORKFLOW.md docs/AGENT-ORCHESTRATION.md docs/ARCHITECTURE.md README.md
git commit -m "docs: explain experiment lifecycle and agent orchestration"
```

### Task 11: Two-Worktree Lifecycle Pilot

**Files:**
- Create: `docs/experiments/2026-08-04-lifecycle-pilot.md`
- Create temporarily, then archive through CLI: two experiment branches and worktrees

- [ ] **Step 1: Record coordinator baseline**

Run:

```sh
git branch --show-current
git status --porcelain
git rev-parse HEAD
```

Expected: `master`, no status output, and one baseline commit hash.

- [ ] **Step 2: Create both lifecycle-only experiments**

Run:

```sh
./tools/experiment create --piece foam --name hatching-spacing-variation \
  --stage hatching --lifecycle-only
./tools/experiment create --piece foam --name color-order-variation \
  --stage coloring --lifecycle-only
```

Expected: distinct `exp/foam/*` branches, sibling worktrees, briefs, and output
paths; coordinator still on `master`.

- [ ] **Step 3: Demonstrate independent lifecycle execution**

For each ID, set `running`, run `verify --record`, write a no-artwork-change
result explaining this is a lifecycle-only pilot, commit the result in its own
worktree, and set `review-pending`. Run `list --json`, `git worktree list
--porcelain`, and compare both records/output directories.

Expected: two simultaneous review-pending worktrees, independent commits and
paths, passing verification, no coordinator implementation changes.

- [ ] **Step 4: Write pilot evidence before cleanup**

Record IDs, branches, worktrees, base commit, record commits, verification
commands, state transitions, isolated output paths, and coordinator branch/HEAD
checks in `docs/experiments/2026-08-04-lifecycle-pilot.md`. Explicitly state
that no artistic changes or renders were produced or merged.

- [ ] **Step 5: Archive and clean both pilots**

Run:

```sh
./tools/experiment discard foam/hatching-spacing-variation
./tools/experiment discard foam/color-order-variation
git worktree list --porcelain
git branch --list 'exp/foam/*variation'
git status --short --branch
```

Expected: both records retained under `experiments/archive`, no pilot
worktrees/branches, and coordinator on `master`. Commit the pilot evidence if
the archive commands did not include it:

```sh
git add docs/experiments/2026-08-04-lifecycle-pilot.md
git commit -m "docs: record parallel experiment lifecycle pilot"
```

### Task 12: Full Verification And Review

**Files:**
- Review all files changed by the plan.

- [ ] **Step 1: Run lifecycle tests with race detection**

Run: `go test -race ./internal/experiment ./cmd/experiment`

Expected: PASS with no race reports.

- [ ] **Step 2: Run the repository gate**

Run: `make check`

Expected: formatting, vet, lint, and all tests PASS. If formatting changes
files, inspect and commit only intentional formatting.

- [ ] **Step 3: Inspect repository safety evidence**

Run:

```sh
git status --short --branch
git worktree list --porcelain
git branch --list 'exp/*' 'integrate/*'
git log --oneline -12
```

Expected: clean `master`, only the coordinator worktree, no pilot experiment or
integration branches, and coherent task commits.

- [ ] **Step 4: Review against the definition of done**

Confirm every accepted design section maps to code/tests/docs: stable
coordinator, one writer checkout, records, output isolation, states, locks,
status diagnostics, base drift, complete/partial/combine procedures, safe
archive/discard, generic delegation and fallback, reviewer restrictions,
parallelism limit, temporary-repository tests, and completed two-worktree
pilot.

- [ ] **Step 5: Commit any final corrections**

If review finds corrections, make the smallest scoped changes, rerun the
focused test and `make check`, then commit:

```sh
git add <only-corrected-files>
git commit -m "fix: complete experiment workflow safeguards"
```
