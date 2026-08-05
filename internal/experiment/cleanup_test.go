package experiment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestArchiveCommitsRetainedRecordBeforeSafeCleanup(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "archive")
	id := IDFromState(t, created.State)
	recordDir := filepath.Join(created.WorktreePath, filepath.FromSlash(id.RecordDir()))
	implementation := filepath.Join(created.WorktreePath, "candidate.txt")
	if err := os.WriteFile(implementation, []byte("candidate implementation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", "candidate.txt")
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "experiment: candidate")
	if err := os.WriteFile(filepath.Join(created.OutputPath, "contact-sheet.png"), []byte("sheet\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, "not-retained.txt"), []byte("omit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join(id.RecordDir(), "not-retained.txt")))
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "experiment: extra record")
	experimentTip := repo.gitOutputAt(t, created.WorktreePath, "rev-parse", "HEAD")
	masterBefore := repo.gitOutput(t, "rev-parse", "HEAD")

	manager.createCheckpoint = func(name string) error {
		if name != "after-archive-ref-update" {
			return nil
		}
		if _, err := os.Stat(created.WorktreePath); err != nil {
			t.Fatalf("worktree removed before archive commit: %v", err)
		}
		if got := repo.gitOutput(t, "rev-parse", "refs/heads/"+created.State.Branch); got != experimentTip {
			t.Fatalf("branch changed before archive commit checkpoint: %s", got)
		}
		return nil
	}
	result, err := manager.Archive(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	if result.ArchiveCommit == "" || result.ArchiveCommit != repo.gitOutput(t, "rev-parse", "master") {
		t.Fatalf("archive result = %#v", result)
	}
	if got := repo.gitOutput(t, "rev-parse", result.ArchiveCommit+"^1"); got != masterBefore {
		t.Fatalf("archive first parent = %s, want master %s", got, masterBefore)
	}
	if got := repo.gitOutput(t, "rev-parse", result.ArchiveCommit+"^2"); got != experimentTip {
		t.Fatalf("archive second parent = %s, want experiment %s", got, experimentTip)
	}
	if _, err := os.Stat(created.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree remains after archive: %v", err)
	}
	if got := repo.gitOutput(t, "branch", "--list", created.State.Branch); got != "" {
		t.Fatalf("branch remains after archive: %q", got)
	}

	archiveDir := filepath.Join(repo.root, filepath.FromSlash(id.ArchiveDir()))
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatal(err)
	}
	gotNames := make([]string, len(entries))
	for i, entry := range entries {
		gotNames[i] = entry.Name()
	}
	slices.Sort(gotNames)
	wantNames := []string{"brief.md", "contact-sheet.png", "favorites.json", "result.md", "state.json"}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("archive files = %v, want %v", gotNames, wantNames)
	}
	changed := strings.Fields(repo.gitOutput(t, "diff", "--name-only", masterBefore, result.ArchiveCommit))
	for i := range changed {
		changed[i] = filepath.ToSlash(changed[i])
	}
	wantChanged := make([]string, len(wantNames))
	for i, name := range wantNames {
		wantChanged[i] = filepath.ToSlash(filepath.Join(id.ArchiveDir(), name))
	}
	if !slices.Equal(changed, wantChanged) {
		t.Fatalf("master changes = %v, want %v", changed, wantChanged)
	}

	repeated, err := manager.Archive(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	if repeated.ArchiveCommit != result.ArchiveCommit || repo.gitOutput(t, "rev-parse", "master") != result.ArchiveCommit {
		t.Fatalf("repeated archive = %#v, first = %#v", repeated, result)
	}
}

func TestDiscardCommitsOutcomeAndStateBeforeArchive(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "discard")
	id := IDFromState(t, created.State)
	discardedAt := time.Date(2026, time.August, 5, 9, 45, 0, 0, time.UTC)
	manager.now = func() time.Time { return discardedAt }
	branchBefore := repo.gitOutputAt(t, created.WorktreePath, "rev-parse", "HEAD")

	result, err := manager.Discard(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	discardCommit := repo.gitOutput(t, "rev-parse", result.ArchiveCommit+"^2")
	if got := repo.gitOutput(t, "rev-parse", discardCommit+"^"); got != branchBefore {
		t.Fatalf("discard parent = %s, want %s", got, branchBefore)
	}
	changed := strings.Fields(repo.gitOutput(t, "diff", "--name-only", branchBefore, discardCommit))
	wantChanged := []string{
		filepath.ToSlash(filepath.Join(id.RecordDir(), "result.md")),
		filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json")),
	}
	if !slices.Equal(changed, wantChanged) {
		t.Fatalf("discard commit paths = %v, want %v", changed, wantChanged)
	}
	archiveDir := filepath.Join(repo.root, filepath.FromSlash(id.ArchiveDir()))
	state := State{}
	readJSONFile(t, filepath.Join(archiveDir, "state.json"), &state)
	if state.Status != StatusDiscarded || !state.UpdatedAt.Equal(discardedAt) {
		t.Fatalf("archived state = %#v", state)
	}
	resultText := readTextFile(t, filepath.Join(archiveDir, "result.md"))
	if !strings.Contains(resultText, "2026-08-05") || !strings.Contains(strings.ToLower(resultText), "discard") {
		t.Fatalf("discard result missing dated outcome:\n%s", resultText)
	}

	repeated, err := manager.Discard(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	if repeated.ArchiveCommit != result.ArchiveCommit {
		t.Fatalf("repeated discard = %#v, first = %#v", repeated, result)
	}
}

func TestArchiveRefusesUnsafeResources(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, testRepo, *Manager, Created)
		want   string
	}{
		{
			name: "dirty coordinator",
			mutate: func(t *testing.T, repo testRepo, _ *Manager, _ Created) {
				if err := os.WriteFile(filepath.Join(repo.root, "README.md"), []byte("dirty\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "coordinator worktree is not clean",
		},
		{
			name: "dirty experiment",
			mutate: func(t *testing.T, _ testRepo, _ *Manager, created Created) {
				if err := os.WriteFile(filepath.Join(created.WorktreePath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "dirty-worktree",
		},
		{
			name: "branch mismatch",
			mutate: func(t *testing.T, repo testRepo, _ *Manager, created Created) {
				repo.gitAt(t, created.WorktreePath, "branch", "-m", "exp/foam/wrong-branch")
			},
			want: "branch-mismatch",
		},
		{
			name: "missing retained file",
			mutate: func(t *testing.T, repo testRepo, _ *Manager, created Created) {
				id := IDFromState(t, created.State)
				if err := os.Remove(filepath.Join(created.WorktreePath, filepath.FromSlash(id.RecordDir()), "brief.md")); err != nil {
					t.Fatal(err)
				}
				repo.gitAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join(id.RecordDir(), "brief.md")))
				repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: remove required record")
			},
			want: "brief.md",
		},
		{
			name: "differing archive",
			mutate: func(t *testing.T, repo testRepo, _ *Manager, created Created) {
				id := IDFromState(t, created.State)
				path := filepath.Join(repo.root, filepath.FromSlash(id.ArchiveDir()), "brief.md")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("different\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				repo.git(t, "add", filepath.ToSlash(id.ArchiveDir()))
				repo.git(t, "commit", "-m", "test: conflicting archive")
			},
			want: "differs",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, manager, created := createArchiveableExperiment(t, strings.ReplaceAll(test.name, " ", "-"))
			test.mutate(t, repo, manager, created)
			_, err := manager.Archive(context.Background(), created.State.ID)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Archive error = %v, want %q", err, test.want)
			}
			if _, statErr := os.Stat(created.WorktreePath); statErr != nil {
				t.Fatalf("refused archive removed worktree: %v", statErr)
			}
			if got := repo.gitOutput(t, "branch", "--list", "exp/foam/*", "integrate/foam/*"); got == "" {
				t.Fatal("refused archive deleted branch")
			}
		})
	}
}

func TestArchiveReturnsAppliedCommitAndSafeRecoveryAfterCleanupFailure(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "cleanup-recovery")
	id := IDFromState(t, created.State)
	manager.createCheckpoint = func(name string) error {
		if name != "after-archive-ref-update" {
			return nil
		}
		return os.WriteFile(filepath.Join(created.WorktreePath, "concurrent.txt"), []byte("preserve\n"), 0o644)
	}

	result, err := manager.Archive(context.Background(), id.String())
	if err == nil {
		t.Fatal("Archive succeeded after worktree became dirty")
	}
	if result.ArchiveCommit == "" || result.ArchiveCommit != repo.gitOutput(t, "rev-parse", "master") {
		t.Fatalf("archive result = %#v, error = %v", result, err)
	}
	wantCommands := []string{
		"git -C " + shellQuote(created.WorktreePath) + " status --short",
		"git worktree remove " + shellQuote(created.WorktreePath),
		"git branch -d " + shellQuote(created.State.Branch),
	}
	if !slices.Equal(result.RemainingCommands, wantCommands) {
		t.Fatalf("remaining commands = %v, want %v", result.RemainingCommands, wantCommands)
	}
	if !strings.Contains(err.Error(), strings.Join(wantCommands, "\n")) {
		t.Fatalf("error lacks exact recovery commands: %v", err)
	}
	if got := readTextFile(t, filepath.Join(created.WorktreePath, "concurrent.txt")); got != "preserve\n" {
		t.Fatalf("concurrent worktree file = %q", got)
	}
	if got := repo.gitOutput(t, "branch", "--list", created.State.Branch); got == "" {
		t.Fatal("cleanup failure deleted branch")
	}
}

func TestArchiveReportsResourcesWhenPostCommitSynchronizationFails(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "post-commit-recovery")
	id := IDFromState(t, created.State)
	manager.createCheckpoint = func(name string) error {
		if name == "after-archive-ref-update" {
			return errors.New("injected post-commit failure")
		}
		return nil
	}

	result, err := manager.Archive(context.Background(), id.String())
	if err == nil || !strings.Contains(err.Error(), "injected post-commit failure") {
		t.Fatalf("Archive error = %v", err)
	}
	if result.ArchiveCommit == "" || result.ArchiveCommit != repo.gitOutput(t, "rev-parse", "master") {
		t.Fatalf("archive result = %#v", result)
	}
	if result.RemainingWorktree != created.WorktreePath || result.RemainingBranch != created.State.Branch || len(result.RemainingCommands) != 3 {
		t.Fatalf("remaining resources = %#v", result)
	}
}

func TestArchiveRevalidatesCoordinatorImmediatelyBeforeRefUpdate(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "coordinator-race")
	id := IDFromState(t, created.State)
	masterBefore := repo.gitOutput(t, "rev-parse", "master")
	repo.git(t, "branch", "coordinator-race", masterBefore)
	manager.createCheckpoint = func(name string) error {
		if name != "before-archive-ref-update" {
			return nil
		}
		repo.git(t, "switch", "coordinator-race")
		return nil
	}

	_, err := manager.Archive(context.Background(), id.String())
	if err == nil || !strings.Contains(err.Error(), "HEAD changed") {
		t.Fatalf("Archive error = %v", err)
	}
	if got := repo.gitOutput(t, "rev-parse", "master"); got != masterBefore {
		t.Fatalf("master changed across coordinator race: got %s, want %s", got, masterBefore)
	}
	if _, statErr := os.Stat(created.WorktreePath); statErr != nil {
		t.Fatalf("coordinator race removed worktree: %v", statErr)
	}
}

func TestArchiveRevalidatesExperimentImmediatelyBeforeRefUpdate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, testRepo, Created)
		want   string
	}{
		{
			name: "dirty worktree",
			mutate: func(t *testing.T, _ testRepo, created Created) {
				if err := os.WriteFile(filepath.Join(created.WorktreePath, "concurrent.txt"), []byte("dirty\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "dirty-worktree",
		},
		{
			name: "retained input",
			mutate: func(t *testing.T, _ testRepo, created Created) {
				id := IDFromState(t, created.State)
				path := filepath.Join(created.WorktreePath, filepath.FromSlash(id.RecordDir()), "result.md")
				if err := os.WriteFile(path, []byte("concurrent result\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "dirty-worktree",
		},
		{
			name: "contact sheet",
			mutate: func(t *testing.T, _ testRepo, created Created) {
				if err := os.WriteFile(filepath.Join(created.OutputPath, "contact-sheet.png"), []byte("concurrent sheet\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "changed before archive ref update",
		},
		{
			name: "symbolic head",
			mutate: func(t *testing.T, repo testRepo, created Created) {
				repo.gitAt(t, created.WorktreePath, "branch", "concurrent-branch")
				repo.gitAt(t, created.WorktreePath, "switch", "concurrent-branch")
			},
			want: "branch-mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, manager, created := createArchiveableExperiment(t, "pre-ref-"+strings.ReplaceAll(test.name, " ", "-"))
			id := IDFromState(t, created.State)
			masterBefore := repo.gitOutput(t, "rev-parse", "master")
			manager.createCheckpoint = func(name string) error {
				if name == "before-archive-ref-update" {
					test.mutate(t, repo, created)
				}
				return nil
			}

			_, err := manager.Archive(context.Background(), id.String())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Archive error = %v, want %q", err, test.want)
			}
			if got := repo.gitOutput(t, "rev-parse", "master"); got != masterBefore {
				t.Fatalf("master changed across experiment mutation: got %s, want %s", got, masterBefore)
			}
			if _, statErr := os.Stat(created.WorktreePath); statErr != nil {
				t.Fatalf("experiment mutation removed worktree: %v", statErr)
			}
		})
	}
}

func TestArchiveAtomicallyVerifiesExperimentRefWhileAdvancingMaster(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "atomic-refs")
	id := IDFromState(t, created.State)
	masterBefore := repo.gitOutput(t, "rev-parse", "master")
	checkpointCalled := false
	manager.createCheckpoint = func(name string) error {
		if name != "before-archive-ref-transaction" {
			return nil
		}
		checkpointCalled = true
		if err := os.WriteFile(filepath.Join(created.WorktreePath, "concurrent.txt"), []byte("branch advance\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		repo.gitAt(t, created.WorktreePath, "add", "concurrent.txt")
		repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: advance experiment during archive")
		return nil
	}

	_, err := manager.Archive(context.Background(), id.String())
	if !checkpointCalled {
		t.Fatal("archive did not reach ref transaction checkpoint")
	}
	if err == nil || !strings.Contains(err.Error(), "atomic archive ref transaction") {
		t.Fatalf("Archive error = %v", err)
	}
	if got := repo.gitOutput(t, "rev-parse", "master"); got != masterBefore {
		t.Fatalf("master changed despite experiment ref race: got %s, want %s", got, masterBefore)
	}
	if _, statErr := os.Stat(created.WorktreePath); statErr != nil {
		t.Fatalf("experiment ref race removed worktree: %v", statErr)
	}
}

func TestArchiveRevalidatesAssignedBranchImmediatelyBeforeWorktreeRemoval(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "worktree-race")
	id := IDFromState(t, created.State)
	repo.gitAt(t, created.WorktreePath, "branch", "worker-switched")
	manager.createCheckpoint = func(name string) error {
		if name != "before-archive-worktree-remove" {
			return nil
		}
		repo.gitAt(t, created.WorktreePath, "switch", "worker-switched")
		return nil
	}

	result, err := manager.Archive(context.Background(), id.String())
	if err == nil || !strings.Contains(err.Error(), "branch-mismatch") {
		t.Fatalf("Archive error = %v", err)
	}
	if result.ArchiveCommit == "" || result.RemainingWorktree != created.WorktreePath {
		t.Fatalf("archive result = %#v", result)
	}
	if _, statErr := os.Stat(created.WorktreePath); statErr != nil {
		t.Fatalf("branch race removed worktree: %v", statErr)
	}
}

func TestArchiveRefusesCommittedChangesToCompletedArchive(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "archive-tamper")
	id := IDFromState(t, created.State)
	if _, err := manager.Archive(context.Background(), id.String()); err != nil {
		t.Fatal(err)
	}
	briefPath := filepath.Join(repo.root, filepath.FromSlash(id.ArchiveDir()), "brief.md")
	if err := os.WriteFile(briefPath, []byte("later replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "add", filepath.ToSlash(filepath.Join(id.ArchiveDir(), "brief.md")))
	repo.git(t, "commit", "-m", "test: alter completed archive")

	if _, err := manager.Archive(context.Background(), id.String()); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("Archive error = %v", err)
	}
}

func TestArchiveRefusesActiveLifecycleLock(t *testing.T) {
	_, manager, created := createArchiveableExperiment(t, "locked")
	id := IDFromState(t, created.State)
	lock, err := manager.AcquireExperimentLock(context.Background(), id, "other lifecycle operation")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Release() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := manager.Archive(ctx, id.String()); err == nil || !strings.Contains(err.Error(), "other lifecycle operation") {
		t.Fatalf("Archive error = %v", err)
	}
}

func TestDiscardRequiresExplicitValidID(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	for _, value := range []string{"", "foam", "foam/Bad"} {
		if _, err := manager.Discard(context.Background(), value); err == nil {
			t.Fatalf("Discard(%q) succeeded", value)
		}
	}
}

func TestDiscardRefusesAnExistingNonDiscardArchive(t *testing.T) {
	_, manager, created := createArchiveableExperiment(t, "archive-not-discard")
	if _, err := manager.Archive(context.Background(), created.State.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Discard(context.Background(), created.State.ID); err == nil || !strings.Contains(err.Error(), "not discarded") {
		t.Fatalf("Discard error = %v", err)
	}
}

func TestDiscardRecoversAppliedCommitAfterIndexSyncFailure(t *testing.T) {
	repo, manager, created := createArchiveableExperiment(t, "discard-index-recovery")
	id := IDFromState(t, created.State)
	gitDir := repo.gitOutputAt(t, created.WorktreePath, "rev-parse", "--git-dir")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(created.WorktreePath, gitDir)
	}
	indexLock := filepath.Join(gitDir, "index.lock")
	manager.createCheckpoint = func(name string) error {
		if name != "after-record-ref-update" {
			return nil
		}
		return os.WriteFile(indexLock, []byte("held\n"), 0o644)
	}

	first, err := manager.Discard(context.Background(), id.String())
	if err == nil || !strings.Contains(err.Error(), "index.lock") {
		t.Fatalf("first Discard error = %v", err)
	}
	if first.RemainingWorktree != created.WorktreePath || first.RemainingBranch != created.State.Branch {
		t.Fatalf("first discard recovery = %#v", first)
	}
	statePath := filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json"))
	stateData := repo.gitOutput(t, "show", "refs/heads/"+created.State.Branch+":"+statePath)
	state, decodeErr := decodeState(statePath, []byte(stateData))
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if state.Status != StatusDiscarded {
		t.Fatalf("committed state = %s, want discarded", state.Status)
	}
	if err := os.Remove(indexLock); err != nil {
		t.Fatal(err)
	}
	manager.createCheckpoint = nil

	second, err := manager.Discard(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	if second.ArchiveCommit == "" || second.RemainingWorktree != "" || second.RemainingBranch != "" {
		t.Fatalf("second discard = %#v", second)
	}
	if _, statErr := os.Stat(created.WorktreePath); !os.IsNotExist(statErr) {
		t.Fatalf("worktree remains after retry: %v", statErr)
	}
}

func createArchiveableExperiment(t *testing.T, name string) (testRepo, *Manager, Created) {
	t.Helper()
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.SetState(context.Background(), created.State.ID, StatusRunning); err != nil {
		t.Fatal(err)
	}
	state, _, err := manager.SetState(context.Background(), created.State.ID, StatusReviewPending)
	if err != nil {
		t.Fatal(err)
	}
	created.State = state
	return repo, manager, created
}
