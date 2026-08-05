package experiment

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// IntegrationOptions configures a semantic integration prepared from source experiments.
type IntegrationOptions struct {
	Name     string
	Sources  []string
	Stage    string
	Keep     string
	Reject   string
	Preserve string
	Profile  string
	Seeds    []uint64
}

// PrepareIntegration creates an integration worktree from current master and
// pins source experiments as evidence without applying their commits.
func (m *Manager) PrepareIntegration(ctx context.Context, opts IntegrationOptions) (Created, error) {
	id, err := ParseID(opts.Name)
	if err != nil {
		return Created{}, err
	}
	sourceIDs, err := integrationSourceIDs(id, opts.Sources)
	if err != nil {
		return Created{}, err
	}
	if strings.TrimSpace(opts.Keep) == "" || strings.TrimSpace(opts.Reject) == "" || strings.TrimSpace(opts.Preserve) == "" {
		return Created{}, fmt.Errorf("integration keep, reject, and preserve behavior must be explicit")
	}

	command := "prepare integration " + id.String()
	globalLock, err := m.AcquireGlobalLock(ctx, command)
	if err != nil {
		return Created{}, err
	}
	locks := []*Lock{globalLock}
	lockIDs := append([]ID{id}, sourceIDs...)
	slices.SortFunc(lockIDs, func(left, right ID) int { return strings.Compare(left.String(), right.String()) })
	for _, lockID := range lockIDs {
		lock, lockErr := m.AcquireExperimentLock(ctx, lockID, command)
		if lockErr != nil {
			return Created{}, errors.Join(lockErr, m.releaseLocks(locks))
		}
		locks = append(locks, lock)
	}

	sources, prepareErr := m.resolveIntegrationSources(ctx, sourceIDs)
	var created Created
	var resources createResources
	if prepareErr == nil {
		created, resources, prepareErr = m.prepareIntegrationLocked(ctx, id, opts, sources)
	}
	releaseErr := m.releaseLocks(locks)
	err = errors.Join(prepareErr, releaseErr)
	if err != nil && (resources.branchCreated || resources.worktreeCreated) {
		return created, createFailure(id, resources, err)
	}
	return created, err
}

func integrationSourceIDs(target ID, values []string) ([]ID, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("integration requires at least one source experiment")
	}
	seen := make(map[ID]bool, len(values))
	ids := make([]ID, 0, len(values))
	for _, value := range values {
		id, err := ParseID(value)
		if err != nil {
			return nil, fmt.Errorf("invalid integration source: %w", err)
		}
		if id == target {
			return nil, fmt.Errorf("integration source %s matches the integration ID", id.String())
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate integration source %s", id.String())
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *Manager) resolveIntegrationSources(ctx context.Context, ids []ID) ([]Source, error) {
	sources := make([]Source, len(ids))
	for i, id := range ids {
		experiment, err := m.Show(ctx, id.String())
		if err != nil {
			return nil, fmt.Errorf("resolve integration source %s: %w", id.String(), err)
		}
		if len(experiment.Diagnostics) != 0 {
			return nil, fmt.Errorf("integration source %s is not authoritative: %s", id.String(), formatDiagnostics(experiment.Diagnostics))
		}
		if experiment.State.Status == StatusDiscarded {
			return nil, fmt.Errorf("integration source %s is discarded", id.String())
		}
		runner := gitRunner{dir: experiment.WorktreePath, env: m.gitEnv, disableOptionalLocks: true}
		tip, err := resolveCommit(ctx, runner, "refs/heads/"+experiment.State.Branch)
		if err != nil {
			return nil, fmt.Errorf("pin integration source %s: %w", id.String(), err)
		}
		present, diagnostics, err := checkCommittedRecords(ctx, runner, id, tip, nil)
		if err != nil {
			return nil, fmt.Errorf("check integration source %s records: %w", id.String(), err)
		}
		if !present {
			return nil, fmt.Errorf("integration source %s has uncommitted or missing records: %s", id.String(), formatDiagnostics(diagnostics))
		}
		sources[i] = Source{ID: id.String(), Commit: tip}
	}
	return sources, nil
}

func (m *Manager) prepareIntegrationLocked(ctx context.Context, id ID, opts IntegrationOptions, sources []Source) (Created, createResources, error) {
	createOpts, err := normalizedCreateOptions(CreateOptions{
		Piece:      id.Piece(),
		Name:       id.Name(),
		BaseBranch: "master",
		Stage:      opts.Stage,
		Profile:    opts.Profile,
		Seeds:      opts.Seeds,
	})
	if err != nil {
		return Created{}, createResources{}, err
	}
	created, resources, err := m.createLocked(ctx, id, createOpts)
	if err != nil {
		return created, resources, err
	}

	currentSources, err := m.resolveIntegrationSources(ctx, integrationIDsFromSources(sources))
	if err != nil {
		return created, resources, err
	}
	if !equalSources(currentSources, sources) {
		return created, resources, fmt.Errorf("integration source tips changed during preparation: got %#v, want %#v", currentSources, sources)
	}
	runner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}
	if _, err := m.validateCoordinator(ctx, runner, created.State.BaseCommit); err != nil {
		return created, resources, err
	}

	originalCommit := created.RecordCommit
	integrationBranch := id.IntegrationBranch()
	worktreeRunner := gitRunner{dir: created.WorktreePath, env: m.gitEnv}
	if _, err := worktreeRunner.run(ctx, "branch", "-m", integrationBranch); err != nil {
		return created, resources, fmt.Errorf("rename prepared branch for integration: %w", err)
	}
	resources.branch = integrationBranch
	if _, err := worktreeRunner.run(ctx, "update-ref", "refs/heads/"+integrationBranch, created.State.BaseCommit, originalCommit); err != nil {
		return created, resources, fmt.Errorf("replace ordinary preparation commit with integration record: %w", err)
	}

	created.State.Kind = KindIntegration
	created.State.Branch = integrationBranch
	created.State.Status = StatusIntegrationPending
	created.State.Sources = sources
	recordPath := filepath.Join(created.WorktreePath, filepath.FromSlash(id.RecordDir()))
	if err := writeJSONAtomic(filepath.Join(recordPath, "state.json"), created.State); err != nil {
		return created, resources, err
	}
	if err := writeBytesAtomic(created.BriefPath, []byte(integrationBrief(created.State, opts))); err != nil {
		return created, resources, err
	}
	templateData := integrationTemplateData(created.State)
	if err := renderTemplate(filepath.Join(m.TemplatesRoot, "result.md"), filepath.Join(recordPath, "result.md"), templateData); err != nil {
		return created, resources, err
	}

	if _, err := m.validateCoordinator(ctx, runner, created.State.BaseCommit); err != nil {
		return created, resources, err
	}
	currentSources, err = m.resolveIntegrationSources(ctx, integrationIDsFromSources(sources))
	if err != nil {
		return created, resources, err
	}
	if !equalSources(currentSources, sources) {
		return created, resources, fmt.Errorf("integration source tips changed before record commit: got %#v, want %#v", currentSources, sources)
	}
	if err := m.validateAssignedWorktree(ctx, runner, integrationBranch, created.WorktreePath, created.State.BaseCommit); err != nil {
		return created, resources, err
	}
	recordFiles := []string{"brief.md", "favorites.json", "result.md", "state.json"}
	for i := range recordFiles {
		recordFiles[i] = filepath.Join(id.RecordDir(), recordFiles[i])
	}
	result, err := commitRecord(ctx, created.WorktreePath, id.RecordDir(), recordFiles, "experiment: prepare integration "+id.String(), integrationBranch, created.State.BaseCommit, m.gitEnv, m.createCheckpoint)
	created.RecordCommit = result.Commit
	if err != nil {
		return created, resources, err
	}
	postCommitErr := m.revalidateIntegrationInputs(ctx, runner, created.State.BaseCommit, sources)
	if postCommitErr != nil {
		return created, resources, appliedCommitError(result, "refs/heads/"+integrationBranch, postCommitErr)
	}
	created.WorkerInstruction = integrationWorkerInstruction(id, created)
	return created, resources, nil
}

func (m *Manager) revalidateIntegrationInputs(ctx context.Context, runner gitRunner, baseCommit string, sources []Source) error {
	_, coordinatorErr := m.validateCoordinator(ctx, runner, baseCommit)
	currentSources, sourceErr := m.resolveIntegrationSources(ctx, integrationIDsFromSources(sources))
	if sourceErr == nil && !equalSources(currentSources, sources) {
		sourceErr = fmt.Errorf("source tips changed after record commit: got %#v, want %#v", currentSources, sources)
	}
	return errors.Join(coordinatorErr, sourceErr)
}

func integrationTemplateData(state State) briefData {
	sourceExperiments := make([]string, len(state.Sources))
	for i, source := range state.Sources {
		sourceExperiments[i] = source.ID + "@" + source.Commit
	}
	seeds := formatSeeds(state.Seeds)
	return briefData{
		ID:                state.ID,
		CreatedAt:         state.CreatedAt.Format(time.RFC3339),
		BaseCommit:        state.BaseCommit,
		Stage:             state.Stage,
		Profile:           state.Profile,
		Seeds:             state.Seeds,
		SourceExperiments: sourceExperiments,
		OutputPath:        state.Output,
		BaselineCommand:   "go run ./cmd/staticart sweep " + strings.SplitN(state.ID, "/", 2)[0] + " --seeds " + seeds + " --profile " + state.Profile + " --out " + state.Output + "/baseline",
		CandidateCommand:  "go run ./cmd/staticart sweep " + strings.SplitN(state.ID, "/", 2)[0] + " --seeds " + seeds + " --profile " + state.Profile + " --out " + state.Output + "/candidate",
	}
}

func integrationBrief(state State, opts IntegrationOptions) string {
	data := integrationTemplateData(state)
	var brief strings.Builder
	fmt.Fprintf(&brief, "# Semantic integration %s\n\n", state.ID)
	fmt.Fprintf(&brief, "- Created: %s\n- Base branch: `master`\n- Base commit: `%s`\n- Stage: `%s`\n- Profile: `%s`\n- Fixed seeds: `%v`\n- Output path: `%s`\n\n", data.CreatedAt, state.BaseCommit, state.Stage, state.Profile, state.Seeds, state.Output)
	brief.WriteString("## Sources\n\n")
	for _, source := range state.Sources {
		fmt.Fprintf(&brief, "- `%s` pinned at `%s`\n", source.ID, source.Commit)
	}
	fmt.Fprintf(&brief, "\n## Keep\n\n%s\n\n## Reject\n\n%s\n\n## Preserve\n\n%s\n\n", opts.Keep, opts.Reject, opts.Preserve)
	brief.WriteString("## Dependencies\n\nSource pins are evidence and dependencies only. Reimplement selected behavior against current `master`; no source branch or commit has been applied automatically.\n\n")
	fmt.Fprintf(&brief, "## Comparison requirements\n\nCompare current-master baseline and integrated candidate with the same profile and fixed seeds. Visually inspect the contact sheet.\n\n```sh\n%s\n%s\n```\n\n", data.BaselineCommand, data.CandidateCommand)
	brief.WriteString("## Worker restrictions\n\nOperate only inside this worktree. Do not switch branches. Do not create or remove worktrees. Do not merge, cherry-pick, rebase, or modify master. Stop at `review-pending`.\n")
	return brief.String()
}

func integrationWorkerInstruction(id ID, created Created) string {
	return "Work on semantic integration " + id.String() + " only.\n" +
		"Worktree: " + created.WorktreePath + "\n" +
		"Branch: " + id.IntegrationBranch() + "\n" +
		"Brief: " + created.BriefPath + "\n" +
		"Operate only inside this worktree. Do not switch branches. Do not create or remove worktrees. Do not merge, cherry-pick, rebase, or modify master. Do not work outside the assigned scope. Do not modify another experiment's files."
}

func integrationIDsFromSources(sources []Source) []ID {
	ids := make([]ID, len(sources))
	for i, source := range sources {
		ids[i], _ = ParseID(source.ID)
	}
	return ids
}

func equalSources(left, right []Source) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (m *Manager) releaseLocks(locks []*Lock) error {
	var err error
	for i := len(locks) - 1; i >= 0; i-- {
		err = errors.Join(err, m.release(locks[i]))
	}
	return err
}
