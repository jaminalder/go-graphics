# Simple Branch And Worktree Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the abandoned custom experiment lifecycle with concise documentation and templates for ordinary Git branch/worktree development.

**Architecture:** Git remains the only lifecycle mechanism. Repository files explain coordinator, worker, review, integration, and cleanup commands; optional Markdown templates help record artistic experiments without becoming a state database.

**Tech Stack:** Git, Markdown, existing Go repository checks.

---

## File Map

- Delete `internal/experiment/`: remove the custom Git lifecycle package and all tests.
- Delete `docs/superpowers/specs/2026-08-04-agent-managed-experiment-workflow-design.md`: remove the superseded design.
- Delete `docs/superpowers/plans/2026-08-04-agent-managed-experiment-workflow.md`: remove the superseded plan.
- Delete `experiments/active/.gitkeep` and `experiments/archive/.gitkeep`: remove unused state-database namespaces.
- Modify `AGENTS.md`: replace CLI/state-machine rules with native Git worktree rules.
- Modify `CLAUDE.md`: make Claude delegation an adapter over the documented Git worktree.
- Rewrite `experiments/templates/brief.md`: make it a copyable Markdown checklist without Go template fields.
- Rewrite `experiments/templates/result.md`: make it a copyable Markdown report without lifecycle state.
- Create `docs/WORKTREE-WORKFLOW.md`: document creation, work, review, integration, conflict handling, and cleanup.
- Modify `README.md`: link the canonical guide and workflow.
- Modify `.gitignore`: remove the redundant `/out/experiments/` rule because `/out/` is already ignored.

### Task 1: Remove The Abandoned Lifecycle Implementation

**Files:**
- Delete: `internal/experiment/`
- Delete: `docs/superpowers/specs/2026-08-04-agent-managed-experiment-workflow-design.md`
- Delete: `docs/superpowers/plans/2026-08-04-agent-managed-experiment-workflow.md`
- Delete: `experiments/active/.gitkeep`
- Delete: `experiments/archive/.gitkeep`

- [ ] **Step 1: Remove the custom implementation and obsolete documents**

Delete the listed paths. Do not replace them with a CLI, shell wrapper, state
record, lock, archive transaction, or tests of Git behavior.

- [ ] **Step 2: Confirm no custom lifecycle references remain outside files that will be rewritten**

Search for:

```text
internal/experiment|tools/experiment|cmd/experiment|review-pending|state.json|experiment-locks
```

Expected: matches only in `AGENTS.md`, `CLAUDE.md`, templates, the approved
simple design, or this plan. Those documentation matches are removed in Task 2.

- [ ] **Step 3: Commit the removal**

```sh
git add -A internal/experiment \
  docs/superpowers/specs/2026-08-04-agent-managed-experiment-workflow-design.md \
  docs/superpowers/plans/2026-08-04-agent-managed-experiment-workflow.md \
  experiments/active experiments/archive
git commit -m "refactor: remove custom experiment lifecycle"
```

### Task 2: Document The Native Git Workflow

**Files:**
- Modify: `AGENTS.md`
- Modify: `CLAUDE.md`
- Modify: `experiments/templates/brief.md`
- Modify: `experiments/templates/result.md`
- Create: `docs/WORKTREE-WORKFLOW.md`
- Modify: `README.md`
- Modify: `.gitignore`

- [ ] **Step 1: Replace mandatory experiment rules in `AGENTS.md`**

Keep all artwork-specific guidance unchanged. Replace only the opening
experiment rules with concise requirements:

```markdown
## Branch and worktree rules

- Keep the coordinator in the canonical checkout on `master`.
- Give every writing worker one dedicated branch and sibling worktree; never
  share a writing checkout.
- Create, inspect, integrate, and remove worktrees with ordinary Git commands
  from `docs/WORKTREE-WORKFLOW.md`.
- Workers operate only in their assigned worktree, do not switch its branch,
  and do not merge or remove worktrees.
- Keep generated renders under `out/` in the assigned worktree. Commit only a
  deliberately selected review sheet when useful.
- Commit coherent changes before review. For artwork changes, compare fixed
  seeds and visually inspect the output; passing tests is not artistic approval.
- Integrate or discard work only after explicit user approval.
- Do not use hard resets, cleans, force deletion, or forced worktree removal as
  routine workflow commands. Inspect and resolve the condition Git reports.
```

Add `docs/WORKTREE-WORKFLOW.md` to the documentation map.

- [ ] **Step 2: Simplify `CLAUDE.md`**

Use:

```markdown
@AGENTS.md

# Claude Code Adapter

Prepare the branch and worktree with the native Git workflow in
`docs/WORKTREE-WORKFLOW.md` before delegating writing work. Set a worker's
working directory to that worktree when supported. Otherwise run every worker
command with the worktree as its explicit working directory.
```

- [ ] **Step 3: Rewrite the optional templates**

Make `brief.md` a plain Markdown form with sections for experiment name,
branch, worktree, base commit, hypothesis, artistic purpose, scope, preserved
behavior, excluded changes, fixed seeds, profile, baseline command, candidate
command, and expected deliverables.

Make `result.md` a plain Markdown form with sections for summary, commits,
verification commands, baseline/candidate artifacts, visual consequences,
strongest and weakest seeds, keep, reject, and recommendation. Do not include
state transitions, JSON records, archive requirements, or executable template
syntax.

- [ ] **Step 4: Write `docs/WORKTREE-WORKFLOW.md`**

Document these exact native operations, using `<name>` consistently:

```sh
mkdir -p ../go-graphics-worktrees
git worktree add ../go-graphics-worktrees/<name> -b exp/<name> master
git -C ../go-graphics-worktrees/<name> status --short --branch
git log --oneline master..exp/<name>
git diff --stat master...exp/<name>
git merge --no-ff exp/<name>
git cherry-pick <commit>
git worktree remove ../go-graphics-worktrees/<name>
git branch -d exp/<name>
```

Explain:

- coordinator and worker responsibilities;
- baseline/candidate renders under each worktree's ignored `out/`;
- full merge versus selected cherry-pick versus manual reimplementation;
- using a fresh integration branch when combining or adapting experiments;
- resolving merge/cherry-pick conflicts with normal Git status and abort
  commands;
- inspecting dirty worktrees and unmerged branches instead of force removal;
- optional use of the brief/result templates;
- a practical limit of two concurrent rendering workers as guidance, not an
  enforced lock.

- [ ] **Step 5: Update links and ignore rules**

Link `AGENTS.md` and `docs/WORKTREE-WORKFLOW.md` from `README.md`. Remove only
the redundant `/out/experiments/` comment/rule from `.gitignore`; retain the
existing `/out/` and worktree ignores.

- [ ] **Step 6: Check documentation consistency**

Search again for:

```text
internal/experiment|tools/experiment|cmd/experiment|review-pending|state.json|experiment-locks
```

Expected: no matches except historical wording in the approved simple design
that explicitly says those mechanisms are out of scope.

Run:

```sh
git diff --check
```

Expected: no output.

- [ ] **Step 7: Commit the workflow documentation**

```sh
git add AGENTS.md CLAUDE.md .gitignore README.md \
  docs/WORKTREE-WORKFLOW.md experiments/templates
git commit -m "docs: establish native git worktree workflow"
```

### Task 3: Verify The Simplified Result

**Files:**
- Review all files changed by Tasks 1 and 2.

- [ ] **Step 1: Verify the documented Git commands exist**

Run:

```sh
git worktree --help
git merge -h
git cherry-pick -h
```

Expected: Git recognizes all three commands. No temporary branches or
worktrees are required to test Git itself.

- [ ] **Step 2: Run the repository gate**

Run:

```sh
make check
```

Expected: formatting, vet, lint, and tests pass.

- [ ] **Step 3: Inspect the final scope**

Run:

```sh
git status --short --branch
git diff --stat master...HEAD
git worktree list
```

Expected: clean feature branch; no `internal/experiment`, `cmd/experiment`, or
`tools/experiment`; the final tree differs from `master` only by the simplified
agent guidance, workflow documentation, templates, ignore cleanup, and the
approved replacement design/plan.

- [ ] **Step 4: Commit only if verification required corrections**

If verification exposes a documentation error, make the smallest correction,
rerun `git diff --check` and `make check`, then commit only corrected files:

```sh
git add <corrected-documentation-files>
git commit -m "docs: correct worktree workflow"
```
