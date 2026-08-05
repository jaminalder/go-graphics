package experiment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var retainedArchiveNames = []string{"brief.md", "result.md", "state.json", "favorites.json"}

// CleanupResult identifies the durable archive commit and any resources left
// for non-forced recovery after cleanup could not finish.
type CleanupResult struct {
	ArchiveCommit        string
	RemainingCoordinator string
	RemainingWorktree    string
	RemainingBranch      string
	RemainingCommands    []string
}

type cleanupSnapshot struct {
	id           ID
	state        State
	branch       string
	branchTip    string
	worktreePath string
	files        map[string][]byte
}

// Archive commits the retained experiment record on master before safely
// removing its clean worktree and merged branch.
func (m *Manager) Archive(ctx context.Context, value string) (CleanupResult, error) {
	id, err := ParseID(value)
	if err != nil {
		return CleanupResult{}, err
	}
	return m.withCleanupLocks(ctx, id, "archive "+id.String(), func() (CleanupResult, error) {
		return m.archiveLocked(ctx, id)
	})
}

// Discard records a dated discarded outcome on the experiment branch, then
// archives and safely removes the experiment resources.
func (m *Manager) Discard(ctx context.Context, value string) (CleanupResult, error) {
	id, err := ParseID(value)
	if err != nil {
		return CleanupResult{}, err
	}
	return m.withCleanupLocks(ctx, id, "discard "+id.String(), func() (CleanupResult, error) {
		discardResult, err := m.discardLocked(ctx, id)
		if err != nil {
			return discardResult, err
		}
		return m.archiveLocked(ctx, id)
	})
}

func (m *Manager) withCleanupLocks(ctx context.Context, id ID, command string, operation func() (CleanupResult, error)) (CleanupResult, error) {
	globalLock, err := m.AcquireGlobalLock(ctx, command)
	if err != nil {
		return CleanupResult{}, err
	}
	idLock, err := m.AcquireExperimentLock(ctx, id, command)
	if err != nil {
		return CleanupResult{}, errors.Join(err, m.release(globalLock))
	}
	result, operationErr := operation()
	return result, errors.Join(operationErr, m.release(idLock), m.release(globalLock))
}

func (m *Manager) archiveLocked(ctx context.Context, id ID) (CleanupResult, error) {
	runner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}
	recoveredCommit, recoveryErr := m.recoverAppliedArchiveIndex(ctx, runner, id)
	if recoveryErr != nil {
		result := CleanupResult{ArchiveCommit: recoveredCommit}
		setRemainingCoordinator(&result, m.CoordinatorRoot, id)
		return result, cleanupRecoveryError(id, result, recoveryErr)
	}
	masterTip, err := m.validateCoordinator(ctx, runner, "")
	if err != nil {
		return CleanupResult{}, err
	}
	snapshot, active, err := m.cleanupSnapshot(ctx, id)
	if err != nil {
		return CleanupResult{}, err
	}
	archivePath := filepath.Join(m.CoordinatorRoot, filepath.FromSlash(id.ArchiveDir()))
	archiveFiles, archiveExists, err := readArchiveFiles(archivePath, id)
	if err != nil {
		return CleanupResult{}, err
	}
	if !active {
		if !archiveExists {
			return CleanupResult{}, fmt.Errorf("experiment %s has no active resources or archive", id.String())
		}
		if err := requireExpectedWorktreeAbsent(id.WorktreePath(m.CoordinatorRoot)); err != nil {
			return CleanupResult{}, err
		}
		commit, err := verifiedArchiveCommit(ctx, runner, id, archiveFiles)
		return CleanupResult{ArchiveCommit: commit}, err
	}
	if !archiveStatusAllowed(snapshot.state.Status) {
		return CleanupResult{}, fmt.Errorf("experiment %s status %s is not archiveable", id.String(), snapshot.state.Status)
	}
	if archiveExists && !equalRetainedFiles(archiveFiles, snapshot.files) {
		return CleanupResult{}, fmt.Errorf("existing archive %s differs from active retained record", id.ArchiveDir())
	}

	archiveCommit := ""
	if archiveExists {
		archiveCommit, err = verifiedArchiveCommit(ctx, runner, id, archiveFiles)
		if err != nil {
			return CleanupResult{}, err
		}
	} else {
		if err := m.revalidateCleanupSnapshot(ctx, runner, masterTip, snapshot); err != nil {
			return CleanupResult{}, err
		}
		if err := installArchiveAtomically(archivePath, snapshot.files); err != nil {
			return CleanupResult{}, err
		}
		paths := archiveRelativePaths(id, snapshot.files)
		commitResult, commitErr := m.commitArchive(ctx, runner, id, snapshot, paths, masterTip)
		archiveCommit = commitResult.Commit
		if commitErr != nil {
			if !commitResult.RefUpdated {
				commitErr = errors.Join(commitErr, removeInstalledArchive(archivePath, snapshot.files))
				return CleanupResult{ArchiveCommit: archiveCommit}, commitErr
			}
			result := CleanupResult{ArchiveCommit: archiveCommit}
			setRemainingResources(&result, snapshot, true, true)
			setRemainingCoordinator(&result, m.CoordinatorRoot, id)
			return result, cleanupRecoveryError(id, result, appliedCommitError(commitResult, "refs/heads/master", commitErr))
		}
	}

	result := CleanupResult{ArchiveCommit: archiveCommit}
	if err := m.cleanupArchivedResources(ctx, runner, archiveCommit, snapshot, &result); err != nil {
		return result, cleanupRecoveryError(id, result, err)
	}
	return result, nil
}

func (m *Manager) cleanupSnapshot(ctx context.Context, id ID) (cleanupSnapshot, bool, error) {
	refs, _, err := m.discoverExperimentRefs(ctx)
	if err != nil {
		return cleanupSnapshot{}, false, err
	}
	worktrees, err := m.worktrees(ctx)
	if err != nil {
		return cleanupSnapshot{}, false, err
	}
	refs, _ = unionWorktreeExperiments(refs, worktrees, nil)
	var matches []discoveredRef
	for _, ref := range refs {
		if ref.id == id {
			matches = append(matches, ref)
		}
	}
	if len(matches) == 0 {
		return cleanupSnapshot{}, false, nil
	}
	if len(matches) != 1 {
		return cleanupSnapshot{}, false, fmt.Errorf("ambiguous experiment %s: both experiment and integration resources exist", id.String())
	}
	experiment, err := m.reconcile(ctx, matches[0], worktrees)
	if err != nil {
		return cleanupSnapshot{}, false, err
	}
	if len(experiment.Diagnostics) != 0 {
		return cleanupSnapshot{}, false, fmt.Errorf("experiment %s is unsafe to clean: %s", id.String(), formatDiagnostics(experiment.Diagnostics))
	}
	branch := matches[0].branch
	expectedRef := "refs/heads/" + branch
	worktreeRunner := gitRunner{dir: experiment.WorktreePath, env: m.gitEnv}
	if err := requireSymbolicHEAD(ctx, worktreeRunner, expectedRef); err != nil {
		return cleanupSnapshot{}, false, err
	}
	branchTip, err := resolveCommit(ctx, worktreeRunner, expectedRef)
	if err != nil {
		return cleanupSnapshot{}, false, err
	}
	files, err := readRetainedFiles(id, experiment)
	if err != nil {
		return cleanupSnapshot{}, false, err
	}
	return cleanupSnapshot{
		id:           id,
		state:        experiment.State,
		branch:       branch,
		branchTip:    branchTip,
		worktreePath: experiment.WorktreePath,
		files:        files,
	}, true, nil
}

func readRetainedFiles(id ID, experiment Experiment) (map[string][]byte, error) {
	recordPath := filepath.Join(experiment.WorktreePath, filepath.FromSlash(id.RecordDir()))
	files := make(map[string][]byte, len(retainedArchiveNames)+1)
	for _, name := range retainedArchiveNames {
		data, err := readRegularFile(filepath.Join(recordPath, name))
		if err != nil {
			return nil, fmt.Errorf("read retained experiment file %s: %w", name, err)
		}
		files[name] = data
	}
	output, err := containedOutputPath(experiment.WorktreePath, id, experiment.State.Output)
	if err != nil {
		return nil, err
	}
	contactSheet := filepath.Join(output, "contact-sheet.png")
	data, err := readRegularFile(contactSheet)
	if err == nil {
		files["contact-sheet.png"] = data
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read optional contact sheet: %w", err)
	}
	return files, nil
}

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.ReadFile(path)
}

func readArchiveFiles(path string, id ID) (map[string][]byte, bool, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read archive directory: %w", err)
	}
	allowed := append(append([]string(nil), retainedArchiveNames...), "contact-sheet.png")
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !slices.Contains(allowed, entry.Name()) {
			return nil, false, fmt.Errorf("existing archive %s differs: unexpected entry %s", id.ArchiveDir(), entry.Name())
		}
		data, readErr := readRegularFile(filepath.Join(path, entry.Name()))
		if readErr != nil {
			return nil, false, fmt.Errorf("read existing archive %s: %w", entry.Name(), readErr)
		}
		files[entry.Name()] = data
	}
	for _, name := range retainedArchiveNames {
		if _, ok := files[name]; !ok {
			return nil, false, fmt.Errorf("existing archive %s differs: missing %s", id.ArchiveDir(), name)
		}
	}
	state, err := decodeState(filepath.Join(path, "state.json"), files["state.json"])
	if err != nil {
		return nil, false, err
	}
	if state.ID != id.String() {
		return nil, false, fmt.Errorf("existing archive state ID is %q, want %q", state.ID, id.String())
	}
	return files, true, nil
}

func equalRetainedFiles(left, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for name, data := range left {
		if !bytes.Equal(data, right[name]) {
			return false
		}
	}
	return true
}

func archiveStatusAllowed(status Status) bool {
	return status == StatusReviewPending || status == StatusMergeReady || status == StatusMerged || status == StatusDiscarded
}

func installArchiveAtomically(destination string, files map[string][]byte) error {
	parent := filepath.Dir(destination)
	temp, err := os.MkdirTemp(parent, "."+filepath.Base(destination)+"-*")
	if err != nil {
		return fmt.Errorf("create temporary archive: %w", err)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	for name, data := range files {
		if err := writeBytesAtomic(filepath.Join(temp, name), data); err != nil {
			return fmt.Errorf("write temporary archive %s: %w", name, err)
		}
	}
	if err := os.Rename(temp, destination); err != nil {
		return fmt.Errorf("atomically install archive %s: %w", destination, err)
	}
	return nil
}

func removeInstalledArchive(path string, expected map[string][]byte) error {
	actual, exists, err := readArchiveFiles(path, mustIDFromArchiveState(expected))
	if err != nil || !exists || !equalRetainedFiles(actual, expected) {
		return errors.Join(err, fmt.Errorf("preserve archive %s because guarded rollback could not prove it unchanged", path))
	}
	for name := range expected {
		if err := os.Remove(filepath.Join(path, name)); err != nil {
			return fmt.Errorf("remove uncommitted archive file %s: %w", name, err)
		}
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove uncommitted archive directory: %w", err)
	}
	return nil
}

func mustIDFromArchiveState(files map[string][]byte) ID {
	state, _ := decodeState("archive state", files["state.json"])
	id, _ := ParseID(state.ID)
	return id
}

func archiveRelativePaths(id ID, files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = filepath.ToSlash(filepath.Join(id.ArchiveDir(), name))
	}
	return paths
}

func (m *Manager) commitArchive(ctx context.Context, runner gitRunner, id ID, snapshot cleanupSnapshot, paths []string, masterParent string) (recordCommitResult, error) {
	if err := m.validateCoordinatorArchiveMutation(ctx, runner, masterParent, paths); err != nil {
		return recordCommitResult{}, err
	}
	currentExperiment, err := resolveCommit(ctx, runner, "refs/heads/"+snapshot.branch)
	if err != nil {
		return recordCommitResult{}, err
	}
	if currentExperiment != snapshot.branchTip {
		return recordCommitResult{}, fmt.Errorf("experiment branch changed before archive commit: got %s, want %s", currentExperiment, snapshot.branchTip)
	}
	index, err := os.CreateTemp("", "experiment-archive-index-*")
	if err != nil {
		return recordCommitResult{}, fmt.Errorf("create isolated archive index: %w", err)
	}
	indexPath := index.Name()
	if err := index.Close(); err != nil {
		_ = os.Remove(indexPath)
		return recordCommitResult{}, err
	}
	if err := os.Remove(indexPath); err != nil {
		return recordCommitResult{}, err
	}
	defer func() { _ = os.Remove(indexPath) }()
	indexRunner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv, indexFile: indexPath}
	if _, err := indexRunner.run(ctx, "read-tree", masterParent); err != nil {
		return recordCommitResult{}, err
	}
	if _, err := indexRunner.run(ctx, append([]string{"add", "--"}, paths...)...); err != nil {
		return recordCommitResult{}, err
	}
	if err := m.checkpoint("during-archive-commit"); err != nil {
		return recordCommitResult{}, err
	}
	if err := m.validateCoordinatorArchiveMutation(ctx, runner, masterParent, paths); err != nil {
		return recordCommitResult{}, err
	}
	tree, err := indexRunner.run(ctx, "write-tree")
	if err != nil {
		return recordCommitResult{}, err
	}
	commit, err := runner.run(ctx, "commit-tree", strings.TrimSpace(tree), "-p", masterParent, "-p", snapshot.branchTip, "-m", "experiment: archive "+id.String())
	if err != nil {
		return recordCommitResult{}, err
	}
	result := recordCommitResult{Commit: strings.TrimSpace(commit)}
	currentExperiment, err = resolveCommit(ctx, runner, "refs/heads/"+snapshot.branch)
	if err != nil {
		return result, err
	}
	if currentExperiment != snapshot.branchTip {
		return result, fmt.Errorf("experiment branch changed before archive ref update: got %s, want %s", currentExperiment, snapshot.branchTip)
	}
	if err := m.checkpoint("before-archive-ref-update"); err != nil {
		return result, err
	}
	if err := m.checkpoint("before-archive-ref-transaction"); err != nil {
		return result, err
	}
	if err := m.validateCoordinatorArchiveMutation(ctx, runner, masterParent, paths); err != nil {
		return result, err
	}
	if err := m.revalidateExperimentSnapshot(ctx, snapshot, "archive ref transaction"); err != nil {
		return result, err
	}
	transaction := fmt.Sprintf(
		"start\nverify refs/heads/%s %s\nupdate refs/heads/master %s %s\nprepare\ncommit\n",
		snapshot.branch, snapshot.branchTip, result.Commit, masterParent,
	)
	if _, err := runner.runInput(ctx, transaction, "update-ref", "--stdin"); err != nil {
		return result, fmt.Errorf("atomic archive ref transaction: %w", err)
	}
	result.RefUpdated = true
	if err := m.checkpoint("after-archive-ref-update"); err != nil {
		return result, appliedCommitError(result, "refs/heads/master", err)
	}
	if err := requireSymbolicHEAD(ctx, runner, "refs/heads/master"); err != nil {
		return result, appliedCommitError(result, "refs/heads/master", err)
	}
	if err := updateCommittedIndexPaths(ctx, runner, result.Commit, paths); err != nil {
		return result, appliedCommitError(result, "refs/heads/master", err)
	}
	return result, nil
}

func (m *Manager) revalidateExperimentSnapshot(ctx context.Context, expected cleanupSnapshot, operation string) error {
	current, active, err := m.cleanupSnapshot(ctx, expected.id)
	if err != nil {
		return err
	}
	if !active || current.branch != expected.branch || current.branchTip != expected.branchTip || current.worktreePath != expected.worktreePath || !equalRetainedFiles(current.files, expected.files) {
		return fmt.Errorf("experiment %s changed before %s", expected.id.String(), operation)
	}
	return nil
}

func (m *Manager) validateCoordinatorArchiveMutation(ctx context.Context, runner gitRunner, expectedHEAD string, allowedPaths []string) error {
	if err := m.RequireCoordinator(); err != nil {
		return err
	}
	if err := requireSymbolicHEAD(ctx, runner, "refs/heads/master"); err != nil {
		return err
	}
	head, err := resolveCommit(ctx, runner, "HEAD")
	if err != nil {
		return err
	}
	if head != expectedHEAD {
		return fmt.Errorf("coordinator HEAD changed: got %s, want original %s", head, expectedHEAD)
	}
	status, err := runner.run(ctx, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	allowed := make(map[string]bool, len(allowedPaths))
	for _, path := range allowedPaths {
		allowed[path] = true
	}
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if line == "" {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if !allowed[path] {
			return fmt.Errorf("coordinator worktree changed during archive: %s", line)
		}
	}
	return nil
}

func (m *Manager) revalidateCleanupSnapshot(ctx context.Context, runner gitRunner, masterTip string, snapshot cleanupSnapshot) error {
	if _, err := m.validateCoordinator(ctx, runner, masterTip); err != nil {
		return err
	}
	current, active, err := m.cleanupSnapshot(ctx, snapshot.id)
	if err != nil {
		return err
	}
	if !active || current.branch != snapshot.branch || current.branchTip != snapshot.branchTip || current.worktreePath != snapshot.worktreePath || !equalRetainedFiles(current.files, snapshot.files) {
		return fmt.Errorf("experiment %s changed before archive mutation", snapshot.id.String())
	}
	return nil
}

func (m *Manager) cleanupArchivedResources(ctx context.Context, runner gitRunner, archiveCommit string, snapshot cleanupSnapshot, result *CleanupResult) error {
	setRemainingResources(result, snapshot, true, true)
	if _, err := m.validateCoordinator(ctx, runner, archiveCommit); err != nil {
		return err
	}
	current, active, err := m.cleanupSnapshot(ctx, snapshot.id)
	if err != nil {
		return err
	}
	if !active || current.branch != snapshot.branch || current.branchTip != snapshot.branchTip || current.worktreePath != snapshot.worktreePath || !equalRetainedFiles(current.files, snapshot.files) {
		return fmt.Errorf("experiment %s changed before worktree removal", snapshot.id.String())
	}
	if err := m.checkpoint("before-archive-worktree-remove"); err != nil {
		return err
	}
	if _, err := m.validateCoordinator(ctx, runner, archiveCommit); err != nil {
		return err
	}
	current, active, err = m.cleanupSnapshot(ctx, snapshot.id)
	if err != nil {
		return err
	}
	if !active || current.branch != snapshot.branch || current.branchTip != snapshot.branchTip || current.worktreePath != snapshot.worktreePath || !equalRetainedFiles(current.files, snapshot.files) {
		return fmt.Errorf("experiment %s changed immediately before worktree removal", snapshot.id.String())
	}
	if _, err := runner.run(ctx, "worktree", "remove", snapshot.worktreePath); err != nil {
		return err
	}
	setRemainingResources(result, snapshot, false, true)
	worktrees, err := m.worktrees(ctx)
	if err != nil {
		return err
	}
	for _, worktree := range worktrees {
		if worktree.Branch == "refs/heads/"+snapshot.branch || filepath.Clean(worktree.Path) == filepath.Clean(snapshot.worktreePath) {
			return fmt.Errorf("worktree or branch remains checked out after removal: %s", worktree.Path)
		}
	}
	currentTip, err := resolveCommit(ctx, runner, "refs/heads/"+snapshot.branch)
	if err != nil {
		return err
	}
	if currentTip != snapshot.branchTip {
		return fmt.Errorf("experiment branch changed before deletion: got %s, want %s", currentTip, snapshot.branchTip)
	}
	if _, err := m.validateCoordinator(ctx, runner, archiveCommit); err != nil {
		return err
	}
	if _, err := runner.run(ctx, "branch", "-d", snapshot.branch); err != nil {
		return err
	}
	setRemainingResources(result, snapshot, false, false)
	return nil
}

func setRemainingResources(result *CleanupResult, snapshot cleanupSnapshot, worktree, branch bool) {
	result.RemainingWorktree = ""
	result.RemainingBranch = ""
	result.RemainingCommands = nil
	if result.RemainingCoordinator != "" {
		setRemainingCoordinator(result, result.RemainingCoordinator, snapshot.id)
	}
	if worktree {
		result.RemainingWorktree = snapshot.worktreePath
		result.RemainingCommands = append(result.RemainingCommands,
			"git -C "+shellQuote(snapshot.worktreePath)+" status --short",
			"git worktree remove "+shellQuote(snapshot.worktreePath),
		)
	}
	if branch {
		result.RemainingBranch = snapshot.branch
		result.RemainingCommands = append(result.RemainingCommands, "git branch -d "+shellQuote(snapshot.branch))
	}
}

func setRemainingCoordinator(result *CleanupResult, coordinator string, id ID) {
	result.RemainingCoordinator = coordinator
	result.RemainingCommands = append(result.RemainingCommands,
		"git -C "+shellQuote(coordinator)+" status --short",
		"git -C "+shellQuote(coordinator)+" diff --cached --name-only -- "+shellQuote(id.ArchiveDir()),
	)
}

func cleanupRecoveryError(id ID, result CleanupResult, cause error) error {
	return fmt.Errorf("archive experiment %s committed as %s, but safe cleanup did not finish: %w; remaining resources: coordinator=%q worktree=%q branch=%q; inspect and continue without force:\n%s", id.String(), result.ArchiveCommit, cause, result.RemainingCoordinator, result.RemainingWorktree, result.RemainingBranch, strings.Join(result.RemainingCommands, "\n"))
}

func (m *Manager) recoverAppliedArchiveIndex(ctx context.Context, runner gitRunner, id ID) (string, error) {
	if err := m.RequireCoordinator(); err != nil {
		return "", err
	}
	if err := requireSymbolicHEAD(ctx, runner, "refs/heads/master"); err != nil {
		return "", nil
	}
	tip, err := resolveCommit(ctx, runner, "refs/heads/master")
	if err != nil {
		return "", err
	}
	archivePath := filepath.Join(m.CoordinatorRoot, filepath.FromSlash(id.ArchiveDir()))
	files, exists, err := readArchiveFiles(archivePath, id)
	if err != nil || !exists {
		return "", err
	}
	paths := archiveRelativePaths(id, files)
	status, err := runner.run(ctx, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	changed := statusPaths(status)
	if len(changed) == 0 || !pathsSubset(changed, paths) {
		return "", nil
	}
	for name, local := range files {
		path := filepath.ToSlash(filepath.Join(id.ArchiveDir(), name))
		committed, showErr := runner.run(ctx, "show", tip+":"+path)
		if showErr != nil || !bytes.Equal(local, []byte(committed)) {
			return "", nil
		}
	}
	if err := updateCommittedIndexPaths(ctx, runner, tip, changed); err != nil {
		return tip, fmt.Errorf("synchronize already-applied archive commit %s: %w", tip, err)
	}
	return tip, nil
}

func pathsSubset(values, allowed []string) bool {
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return false
		}
	}
	return true
}

func verifiedArchiveCommit(ctx context.Context, runner gitRunner, id ID, current map[string][]byte) (string, error) {
	statePath := filepath.ToSlash(filepath.Join(id.ArchiveDir(), "state.json"))
	output, err := runner.run(ctx, "log", "--first-parent", "-m", "--format=%H", "--diff-filter=A", "master", "--", statePath)
	if err != nil {
		return "", err
	}
	commits := strings.Fields(output)
	if len(commits) != 1 {
		return "", fmt.Errorf("archive %s differs: state introduction commits are %v", id.ArchiveDir(), commits)
	}
	commit := commits[0]
	archived := make(map[string][]byte, len(current))
	for name := range current {
		path := filepath.ToSlash(filepath.Join(id.ArchiveDir(), name))
		data, showErr := runner.run(ctx, "show", commit+":"+path)
		if showErr != nil {
			return "", fmt.Errorf("archive %s differs from original commit %s: %w", id.ArchiveDir(), commit, showErr)
		}
		archived[name] = []byte(data)
	}
	if !equalRetainedFiles(archived, current) {
		return "", fmt.Errorf("archive %s differs from original commit %s", id.ArchiveDir(), commit)
	}
	return commit, nil
}

func requireExpectedWorktreeAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("experiment worktree path remains without an active branch: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (m *Manager) discardLocked(ctx context.Context, id ID) (CleanupResult, error) {
	runner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}
	masterTip, err := m.validateCoordinator(ctx, runner, "")
	if err != nil {
		return CleanupResult{}, err
	}
	discardBranch, err := m.recoverAppliedDiscardIndex(ctx, id)
	if err != nil {
		return discardRecoveryResult(id, id.WorktreePath(m.CoordinatorRoot), discardBranch), err
	}
	snapshot, active, err := m.cleanupSnapshot(ctx, id)
	if err != nil {
		return CleanupResult{}, err
	}
	if !active {
		archiveFiles, archiveExists, archiveErr := readArchiveFiles(filepath.Join(m.CoordinatorRoot, filepath.FromSlash(id.ArchiveDir())), id)
		if archiveErr != nil {
			return CleanupResult{}, archiveErr
		}
		if archiveExists {
			state, decodeErr := decodeState(id.ArchiveDir()+"/state.json", archiveFiles["state.json"])
			if decodeErr != nil {
				return CleanupResult{}, decodeErr
			}
			if state.Status != StatusDiscarded {
				return CleanupResult{}, fmt.Errorf("existing archive %s is not discarded; status is %s", id.ArchiveDir(), state.Status)
			}
			return CleanupResult{}, nil
		}
		return CleanupResult{}, fmt.Errorf("experiment %s has no active resources", id.String())
	}
	if snapshot.state.Status == StatusDiscarded {
		return CleanupResult{}, nil
	}
	if !CanTransition(snapshot.state.Status, StatusDiscarded) {
		return CleanupResult{}, fmt.Errorf("invalid transition from %s to %s; allowed: %v", snapshot.state.Status, StatusDiscarded, AllowedTransitions(snapshot.state.Status))
	}
	archivePath := filepath.Join(m.CoordinatorRoot, filepath.FromSlash(id.ArchiveDir()))
	if _, archiveExists, err := readArchiveFiles(archivePath, id); err != nil {
		return CleanupResult{}, err
	} else if archiveExists {
		return CleanupResult{}, fmt.Errorf("existing archive %s differs from requested discarded state", id.ArchiveDir())
	}
	if err := m.revalidateCleanupSnapshot(ctx, runner, masterTip, snapshot); err != nil {
		return CleanupResult{}, err
	}

	recordDir := filepath.Join(snapshot.worktreePath, filepath.FromSlash(id.RecordDir()))
	resultPath := filepath.Join(recordDir, "result.md")
	statePath := filepath.Join(recordDir, "state.json")
	oldResult := snapshot.files["result.md"]
	oldState := snapshot.files["state.json"]
	now := m.now
	if now == nil {
		now = time.Now
	}
	discardedAt := now().UTC()
	newResult := append([]byte(nil), oldResult...)
	if len(newResult) != 0 && newResult[len(newResult)-1] != '\n' {
		newResult = append(newResult, '\n')
	}
	newResult = append(newResult, []byte("\n## Discard outcome ("+discardedAt.Format("2006-01-02")+")\n\nDiscarded on "+discardedAt.Format(time.RFC3339)+" UTC.\n")...)
	state := snapshot.state
	state.Status = StatusDiscarded
	state.UpdatedAt = discardedAt
	if err := writeBytesAtomic(resultPath, newResult); err != nil {
		return CleanupResult{}, err
	}
	if err := writeJSONAtomic(statePath, state); err != nil {
		return CleanupResult{}, errors.Join(err, restoreIfUnchanged(resultPath, newResult, oldResult))
	}
	newState, err := os.ReadFile(statePath)
	if err != nil {
		return CleanupResult{}, errors.Join(err, restoreIfUnchanged(resultPath, newResult, oldResult), restoreIfUnchanged(statePath, nil, oldState))
	}
	paths := []string{
		filepath.Join(id.RecordDir(), "result.md"),
		filepath.Join(id.RecordDir(), "state.json"),
	}
	commitResult, commitErr := commitRecord(ctx, snapshot.worktreePath, id.RecordDir(), paths, "experiment: discard "+id.String(), snapshot.branch, snapshot.branchTip, m.gitEnv, m.createCheckpoint)
	if commitErr != nil {
		if commitResult.RefUpdated {
			result := discardRecoveryResult(id, snapshot.worktreePath, snapshot.branch)
			return result, fmt.Errorf("discard commit %s was applied, but post-commit synchronization failed: %w; inspect and retry discard without force:\n%s", commitResult.Commit, appliedCommitError(commitResult, "refs/heads/"+snapshot.branch, commitErr), strings.Join(result.RemainingCommands, "\n"))
		}
		currentTip, resolveErr := resolveCommit(ctx, runner, "refs/heads/"+snapshot.branch)
		if resolveErr != nil {
			return CleanupResult{}, errors.Join(commitErr, resolveErr)
		}
		if commitResult.Commit != "" && currentTip == commitResult.Commit {
			if syncErr := updateCommittedIndexPaths(ctx, gitRunner{dir: snapshot.worktreePath, env: m.gitEnv}, currentTip, paths); syncErr != nil {
				result := discardRecoveryResult(id, snapshot.worktreePath, snapshot.branch)
				return result, appliedCommitError(recordCommitResult{Commit: currentTip, RefUpdated: true}, "refs/heads/"+snapshot.branch, errors.Join(commitErr, syncErr))
			}
			return CleanupResult{}, nil
		}
		if currentTip != snapshot.branchTip {
			return CleanupResult{}, errors.Join(commitErr, fmt.Errorf("experiment branch changed during discard; attempted result and state were preserved"))
		}
		return CleanupResult{}, errors.Join(commitErr,
			restoreIfUnchanged(resultPath, newResult, oldResult),
			restoreIfUnchanged(statePath, newState, oldState),
		)
	}
	return CleanupResult{}, nil
}

func (m *Manager) recoverAppliedDiscardIndex(ctx context.Context, id ID) (string, error) {
	refs, _, err := m.discoverExperimentRefs(ctx)
	if err != nil {
		return "", err
	}
	var branch string
	for _, ref := range refs {
		if ref.id == id {
			if branch != "" {
				return "", nil
			}
			branch = ref.branch
		}
	}
	if branch == "" {
		return "", nil
	}
	worktreePath := id.WorktreePath(m.CoordinatorRoot)
	runner := gitRunner{dir: worktreePath, env: m.gitEnv}
	if err := requireSymbolicHEAD(ctx, runner, "refs/heads/"+branch); err != nil {
		return branch, nil
	}
	tip, err := resolveCommit(ctx, runner, "refs/heads/"+branch)
	if err != nil {
		return branch, nil
	}
	stateRelative := filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json"))
	stateData, err := runner.run(ctx, "show", tip+":"+stateRelative)
	if err != nil {
		return branch, nil
	}
	state, err := decodeState(tip+":"+stateRelative, []byte(stateData))
	if err != nil || state.ID != id.String() || state.Branch != branch || state.Status != StatusDiscarded {
		return branch, nil
	}
	paths := []string{filepath.ToSlash(filepath.Join(id.RecordDir(), "result.md")), stateRelative}
	status, err := runner.run(ctx, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return branch, nil
	}
	changed := statusPaths(status)
	if len(changed) == 0 || !pathsSubset(changed, paths) {
		return branch, nil
	}
	for _, path := range paths {
		committed, showErr := runner.run(ctx, "show", tip+":"+path)
		if showErr != nil {
			return branch, nil
		}
		local, readErr := os.ReadFile(filepath.Join(worktreePath, filepath.FromSlash(path)))
		if readErr != nil || !bytes.Equal(local, []byte(committed)) {
			return branch, nil
		}
	}
	if err := updateCommittedIndexPaths(ctx, runner, tip, changed); err != nil {
		return branch, fmt.Errorf("synchronize already-applied discard commit %s: %w", tip, err)
	}
	return branch, nil
}

func statusPaths(status string) []string {
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if len(line) >= 4 {
			paths = append(paths, strings.TrimSpace(line[3:]))
		}
	}
	return paths
}

func discardRecoveryResult(id ID, worktree, branch string) CleanupResult {
	stateRef := filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json"))
	if branch != "" {
		stateRef = "refs/heads/" + branch + ":" + stateRef
	}
	return CleanupResult{
		RemainingWorktree: worktree,
		RemainingBranch:   branch,
		RemainingCommands: []string{
			"git -C " + shellQuote(worktree) + " status --short",
			"git show " + shellQuote(stateRef),
		},
	}
}

func restoreIfUnchanged(path string, written, old []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("inspect %s for guarded rollback: %w", path, err)
	}
	if written != nil && !bytes.Equal(current, written) {
		return fmt.Errorf("preserve %s because it changed during guarded rollback", path)
	}
	return writeBytesAtomic(path, old)
}
