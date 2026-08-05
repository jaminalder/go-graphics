package experiment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errStateChangedDuringCommit = errors.New("state file changed after state write")

// SetState validates and commits one lifecycle transition on its assigned branch.
func (m *Manager) SetState(ctx context.Context, value string, target Status) (State, string, error) {
	id, err := ParseID(value)
	if err != nil {
		return State{}, "", err
	}
	if _, err := ParseStatus(string(target)); err != nil {
		return State{}, "", err
	}
	lock, err := m.AcquireExperimentLock(ctx, id, "state "+id.String()+" "+string(target))
	if err != nil {
		return State{}, "", err
	}
	state, commit, transitionErr := m.setStateLocked(ctx, id, target)
	return state, commit, errors.Join(transitionErr, m.release(lock))
}

func (m *Manager) setStateLocked(ctx context.Context, id ID, target Status) (State, string, error) {
	refs, err := m.discoverExperimentRefs(ctx)
	if err != nil {
		return State{}, "", err
	}
	var branch string
	for _, ref := range refs {
		if ref.id != id {
			continue
		}
		if branch != "" {
			return State{}, "", fmt.Errorf("ambiguous experiment %s: both experiment and integration refs exist", id.String())
		}
		branch = ref.branch
	}
	if branch == "" {
		return State{}, "", fmt.Errorf("experiment %s has no active branch", id.String())
	}
	worktrees, err := m.worktrees(ctx)
	if err != nil {
		return State{}, "", err
	}
	expectedPath := filepath.Clean(id.WorktreePath(m.CoordinatorRoot))
	var assigned *WorktreeInfo
	for i := range worktrees {
		if filepath.Clean(worktrees[i].Path) == expectedPath {
			assigned = &worktrees[i]
			break
		}
	}
	if assigned == nil {
		return State{}, "", fmt.Errorf("assigned worktree is missing or not registered: %s", expectedPath)
	}
	info, err := os.Stat(expectedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, "", fmt.Errorf("assigned worktree directory is missing: %s", expectedPath)
		}
		return State{}, "", fmt.Errorf("inspect assigned worktree: %w", err)
	}
	if !info.IsDir() {
		return State{}, "", fmt.Errorf("assigned worktree path is not a directory: %s", expectedPath)
	}
	if assigned.Prunable {
		return State{}, "", fmt.Errorf("assigned worktree has stale prunable metadata: %s", assigned.PrunableReason)
	}
	expectedRef := "refs/heads/" + branch
	if assigned.Branch != expectedRef {
		return State{}, "", fmt.Errorf("assigned worktree branch changed: got %q, want %q", assigned.Branch, expectedRef)
	}
	runner := gitRunner{dir: expectedPath, env: m.gitEnv}
	if err := requireSymbolicHEAD(ctx, runner, expectedRef); err != nil {
		return State{}, "", err
	}
	statePath := filepath.Join(expectedPath, filepath.FromSlash(id.RecordDir()), "state.json")
	state, err := readState(statePath)
	if err != nil {
		return State{}, "", err
	}
	if state.ID != id.String() {
		return State{}, "", fmt.Errorf("state ID mismatch: got %q, want %q", state.ID, id.String())
	}
	if state.Branch != branch {
		return State{}, "", fmt.Errorf("state branch mismatch: got %q, want %q", state.Branch, branch)
	}
	if strings.HasPrefix(branch, "exp/") && state.Kind != KindExperiment {
		return State{}, "", fmt.Errorf("state kind mismatch: branch %s requires %s, got %s", branch, KindExperiment, state.Kind)
	}
	if strings.HasPrefix(branch, "integrate/") && state.Kind != KindIntegration {
		return State{}, "", fmt.Errorf("state kind mismatch: branch %s requires %s, got %s", branch, KindIntegration, state.Kind)
	}
	stateWorktree := filepath.Clean(filepath.Join(m.CoordinatorRoot, filepath.FromSlash(state.Worktree)))
	if stateWorktree != expectedPath {
		return State{}, "", fmt.Errorf("state worktree path mismatch: got %q, want %q", stateWorktree, expectedPath)
	}
	status, err := runner.run(ctx, "status", "--porcelain")
	if err != nil {
		return State{}, "", err
	}
	if status != "" {
		return State{}, "", fmt.Errorf("assigned worktree is not clean: %s", strings.TrimSpace(status))
	}
	if !CanTransition(state.Status, target) {
		return State{}, "", fmt.Errorf("invalid transition from %s to %s; allowed: %v", state.Status, target, AllowedTransitions(state.Status))
	}
	parent, err := runner.run(ctx, "rev-parse", "--verify", expectedRef+"^{commit}")
	if err != nil {
		return State{}, "", err
	}
	parent = strings.TrimSpace(parent)
	if assigned.HEAD != parent {
		return State{}, "", fmt.Errorf("assigned worktree tip mismatch: metadata has %s, branch has %s", assigned.HEAD, parent)
	}

	now := m.now
	if now == nil {
		now = time.Now
	}
	state.Status = target
	state.UpdatedAt = now().UTC()
	oldState, err := os.ReadFile(statePath)
	if err != nil {
		return State{}, "", fmt.Errorf("capture old state before transition: %w", err)
	}
	if err := writeJSONAtomic(statePath, state); err != nil {
		return State{}, "", err
	}
	newState, err := os.ReadFile(statePath)
	if err != nil {
		return State{}, "", fmt.Errorf("capture written state before commit: %w", err)
	}
	recordDir := filepath.FromSlash(id.RecordDir())
	commitResult, err := commitRecord(
		ctx,
		expectedPath,
		recordDir,
		[]string{filepath.Join(recordDir, "state.json")},
		"experiment: set "+id.String()+" "+string(target),
		branch,
		parent,
		m.gitEnv,
		m.createCheckpoint,
	)
	if err != nil {
		if commitResult.RefUpdated {
			return state, commitResult.Commit, appliedCommitError(commitResult, expectedRef, err)
		}
		return state, commitResult.Commit, errors.Join(err, restoreStateAfterCommitFailure(statePath, oldState, newState))
	}
	return state, commitResult.Commit, nil
}

func restoreStateAfterCommitFailure(path string, oldState, writtenState []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("inspect state for guarded rollback: %w", err)
	}
	if !bytes.Equal(current, writtenState) {
		return fmt.Errorf("%w; preserving current content", errStateChangedDuringCommit)
	}
	if err := writeBytesAtomic(path, oldState); err != nil {
		return fmt.Errorf("restore state after commit failure: %w", err)
	}
	return nil
}
