package experiment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Diagnostic describes one inconsistency in an active experiment resource.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Experiment combines the authoritative state with its reconciled worktree.
type Experiment struct {
	State        State        `json:"state"`
	WorktreePath string       `json:"worktree_path"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}

type discoveredRef struct {
	id        ID
	branch    string
	refExists bool
}

// ListReport contains valid experiments and repository-level discovery diagnostics.
type ListReport struct {
	Experiments []Experiment `json:"experiments"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// List returns active experiments in deterministic ID order.
func (m *Manager) List(ctx context.Context) ([]Experiment, error) {
	report, err := m.ListReport(ctx)
	if err != nil {
		return nil, err
	}
	return report.Experiments, nil
}

// ListReport returns active experiments plus malformed discovery diagnostics.
func (m *Manager) ListReport(ctx context.Context) (ListReport, error) {
	refs, diagnostics, err := m.discoverExperimentRefs(ctx)
	if err != nil {
		return ListReport{}, err
	}
	worktrees, err := m.worktrees(ctx)
	if err != nil {
		return ListReport{}, err
	}
	refs, diagnostics = unionWorktreeExperiments(refs, worktrees, diagnostics)
	byID := make(map[string][]discoveredRef)
	for _, ref := range refs {
		byID[ref.id.String()] = append(byID[ref.id.String()], ref)
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	experiments := make([]Experiment, 0, len(ids))
	for _, value := range ids {
		candidates := byID[value]
		if len(candidates) != 1 {
			experiments = append(experiments, Experiment{
				State: State{ID: value},
				Diagnostics: []Diagnostic{{
					Code:    "ambiguous-ref",
					Message: fmt.Sprintf("both experiment and integration refs exist for %s", value),
				}},
			})
			continue
		}
		experiment, err := m.reconcile(ctx, candidates[0], worktrees)
		if err != nil {
			return ListReport{}, err
		}
		experiments = append(experiments, experiment)
	}
	return ListReport{Experiments: experiments, Diagnostics: diagnostics}, nil
}

// Show returns one active experiment after reconciling its resources.
func (m *Manager) Show(ctx context.Context, value string) (Experiment, error) {
	id, err := ParseID(value)
	if err != nil {
		return Experiment{}, err
	}
	refs, _, err := m.discoverExperimentRefs(ctx)
	if err != nil {
		return Experiment{}, err
	}
	worktrees, err := m.worktrees(ctx)
	if err != nil {
		return Experiment{}, err
	}
	refs, _ = unionWorktreeExperiments(refs, worktrees, nil)
	var matches []discoveredRef
	for _, ref := range refs {
		if ref.id == id {
			matches = append(matches, ref)
		}
	}
	if len(matches) == 0 {
		return Experiment{}, fmt.Errorf("experiment %s has no active branch", id.String())
	}
	if len(matches) > 1 {
		return Experiment{}, fmt.Errorf("ambiguous experiment %s: both %s and %s exist", id.String(), id.ExperimentBranch(), id.IntegrationBranch())
	}
	return m.reconcile(ctx, matches[0], worktrees)
}

// Path returns the absolute assigned worktree path when reconciliation proves
// that it is safe to route a writing worker there.
func (m *Manager) Path(ctx context.Context, value string) (string, error) {
	experiment, err := m.Show(ctx, value)
	if err != nil {
		return "", err
	}
	unsafe := map[string]bool{
		"missing-worktree-directory": true,
		"stale-worktree-metadata":    true,
		"missing-record":             true,
		"malformed-state":            true,
		"branch-mismatch":            true,
		"path-mismatch":              true,
		"missing-branch":             true,
	}
	var codes []string
	for _, diagnostic := range experiment.Diagnostics {
		if unsafe[diagnostic.Code] {
			codes = append(codes, diagnostic.Code)
		}
	}
	if len(codes) != 0 {
		return "", fmt.Errorf("experiment %s has unsafe worktree reconciliation: %s", value, strings.Join(codes, ", "))
	}
	if !filepath.IsAbs(experiment.WorktreePath) {
		return "", fmt.Errorf("experiment %s worktree path is not absolute: %q", value, experiment.WorktreePath)
	}
	return experiment.WorktreePath, nil
}

func (m *Manager) discoverExperimentRefs(ctx context.Context) ([]discoveredRef, []Diagnostic, error) {
	runner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}
	output, err := runner.run(ctx, "for-each-ref", "--format=%(refname)%00", "refs/heads/exp", "refs/heads/integrate")
	if err != nil {
		return nil, nil, err
	}
	var refs []discoveredRef
	var diagnostics []Diagnostic
	for _, field := range strings.Split(output, "\x00") {
		name := strings.TrimPrefix(field, "\n")
		name = strings.TrimSuffix(name, "\n")
		if name == "" {
			continue
		}
		var branch, value string
		switch {
		case strings.HasPrefix(name, "refs/heads/exp/"):
			branch = strings.TrimPrefix(name, "refs/heads/")
			value = strings.TrimPrefix(name, "refs/heads/exp/")
		case strings.HasPrefix(name, "refs/heads/integrate/"):
			branch = strings.TrimPrefix(name, "refs/heads/")
			value = strings.TrimPrefix(name, "refs/heads/integrate/")
		default:
			continue
		}
		id, err := ParseID(value)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Code: "malformed-ref", Message: fmt.Sprintf("malformed active experiment ref %q: %v", name, err)})
			continue
		}
		refs = append(refs, discoveredRef{id: id, branch: branch, refExists: true})
	}
	return refs, diagnostics, nil
}

func unionWorktreeExperiments(refs []discoveredRef, worktrees []WorktreeInfo, diagnostics []Diagnostic) ([]discoveredRef, []Diagnostic) {
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		seen[ref.branch] = true
	}
	for _, worktree := range worktrees {
		branch := strings.TrimPrefix(worktree.Branch, "refs/heads/")
		var value string
		switch {
		case strings.HasPrefix(branch, "exp/"):
			value = strings.TrimPrefix(branch, "exp/")
		case strings.HasPrefix(branch, "integrate/"):
			value = strings.TrimPrefix(branch, "integrate/")
		default:
			continue
		}
		if seen[branch] {
			continue
		}
		id, err := ParseID(value)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Code: "malformed-ref", Message: fmt.Sprintf("malformed experiment worktree branch %q: %v", worktree.Branch, err)})
			continue
		}
		refs = append(refs, discoveredRef{id: id, branch: branch})
		seen[branch] = true
	}
	return refs, diagnostics
}

func (m *Manager) worktrees(ctx context.Context) ([]WorktreeInfo, error) {
	output, err := (gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}).run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(output)
}

func (m *Manager) reconcile(ctx context.Context, ref discoveredRef, worktrees []WorktreeInfo) (Experiment, error) {
	expectedPath := filepath.Clean(ref.id.WorktreePath(m.CoordinatorRoot))
	experiment := Experiment{State: State{ID: ref.id.String()}, WorktreePath: expectedPath}
	if !ref.refExists {
		experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "missing-branch", Message: fmt.Sprintf("worktree branch ref refs/heads/%s is missing", ref.branch)})
	}
	var byBranch, atExpected *WorktreeInfo
	for i := range worktrees {
		worktree := &worktrees[i]
		if worktree.Branch == "refs/heads/"+ref.branch {
			byBranch = worktree
		}
		if filepath.Clean(worktree.Path) == expectedPath {
			atExpected = worktree
		}
	}
	if byBranch != nil {
		experiment.WorktreePath = filepath.Clean(byBranch.Path)
	}
	if byBranch == nil {
		experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{
			Code:    "missing-worktree-directory",
			Message: fmt.Sprintf("branch %s has no registered worktree at %s", ref.branch, expectedPath),
		})
	}
	if byBranch != nil && filepath.Clean(byBranch.Path) != expectedPath {
		experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{
			Code:    "path-mismatch",
			Message: fmt.Sprintf("branch %s is registered at %s, expected %s", ref.branch, byBranch.Path, expectedPath),
		})
	}
	if atExpected != nil && atExpected.Branch != "refs/heads/"+ref.branch {
		experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{
			Code:    "branch-mismatch",
			Message: fmt.Sprintf("expected worktree %s has branch %s, expected refs/heads/%s", expectedPath, atExpected.Branch, ref.branch),
		})
	}

	registered := byBranch
	if registered == nil {
		registered = atExpected
	}
	if registered != nil && registered.Prunable {
		message := fmt.Sprintf("Git marks worktree %s as prunable", registered.Path)
		if registered.PrunableReason != "" {
			message += ": " + registered.PrunableReason
		}
		experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "stale-worktree-metadata", Message: message})
	}
	directoryPresent := false
	if registered != nil {
		info, statErr := os.Stat(registered.Path)
		switch {
		case statErr == nil && info.IsDir():
			directoryPresent = true
		case errors.Is(statErr, os.ErrNotExist):
			experiment.Diagnostics = append(experiment.Diagnostics,
				Diagnostic{Code: "missing-worktree-directory", Message: fmt.Sprintf("registered worktree directory is missing: %s", registered.Path)},
			)
			if !registered.Prunable {
				experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "stale-worktree-metadata", Message: fmt.Sprintf("Git still registers missing worktree %s", registered.Path)})
			}
		case statErr != nil:
			return Experiment{}, fmt.Errorf("inspect worktree directory %q: %w", registered.Path, statErr)
		default:
			experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "missing-worktree-directory", Message: fmt.Sprintf("worktree path is not a directory: %s", registered.Path)})
		}
	} else if _, statErr := os.Stat(expectedPath); errors.Is(statErr, os.ErrNotExist) {
		// The no-registration diagnostic above also describes this absent path.
	} else if statErr != nil {
		return Experiment{}, fmt.Errorf("inspect expected worktree directory %q: %w", expectedPath, statErr)
	}

	validExpectedWorktree := byBranch != nil && atExpected != nil && byBranch == atExpected && directoryPresent && !byBranch.Prunable
	statePath := filepath.ToSlash(filepath.Join(ref.id.RecordDir(), "state.json"))
	var state State
	var stateErr error
	if validExpectedWorktree {
		state, stateErr = readState(filepath.Join(expectedPath, filepath.FromSlash(statePath)))
	} else {
		state, stateErr = m.readStateFromBranch(ctx, ref.branch, statePath)
	}
	if stateErr != nil {
		code := "malformed-state"
		if errors.Is(stateErr, os.ErrNotExist) || strings.Contains(stateErr.Error(), "does not exist in") || strings.Contains(stateErr.Error(), "exists on disk, but not in") {
			code = "missing-record"
		}
		experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: code, Message: stateErr.Error()})
	} else {
		experiment.State = state
		if state.ID != ref.id.String() {
			experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "malformed-state", Message: fmt.Sprintf("state ID is %s, expected %s", state.ID, ref.id.String())})
		}
		if state.Branch != ref.branch {
			experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "branch-mismatch", Message: fmt.Sprintf("state branch is %s, actual ref is %s", state.Branch, ref.branch)})
		}
		if strings.HasPrefix(ref.branch, "exp/") && state.Kind != KindExperiment || strings.HasPrefix(ref.branch, "integrate/") && state.Kind != KindIntegration {
			experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "malformed-state", Message: fmt.Sprintf("state kind %s contradicts branch %s", state.Kind, ref.branch)})
		}
		stateWorktree := filepath.Clean(filepath.Join(m.CoordinatorRoot, filepath.FromSlash(state.Worktree)))
		if stateWorktree != expectedPath || (byBranch != nil && stateWorktree != filepath.Clean(byBranch.Path)) {
			experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "path-mismatch", Message: fmt.Sprintf("state worktree resolves to %s, expected %s", stateWorktree, expectedPath)})
		}
	}
	if directoryPresent && !registered.Prunable {
		status, err := (gitRunner{dir: registered.Path, env: m.gitEnv, disableOptionalLocks: true}).run(ctx, "status", "--porcelain")
		if err != nil {
			return Experiment{}, err
		}
		if status != "" {
			experiment.Diagnostics = append(experiment.Diagnostics, Diagnostic{Code: "dirty-worktree", Message: fmt.Sprintf("worktree %s has uncommitted changes", registered.Path)})
		}
	}
	return experiment, nil
}

func (m *Manager) readStateFromBranch(ctx context.Context, branch, path string) (State, error) {
	data, err := (gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}).run(ctx, "show", "refs/heads/"+branch+":"+path)
	if err != nil {
		return State{}, err
	}
	return decodeState(branch+":"+path, []byte(data))
}
