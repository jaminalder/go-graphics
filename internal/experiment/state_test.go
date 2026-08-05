package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSetStateCommitsOnlyStateWithExactUTCTimestamp(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "transition"})
	if err != nil {
		t.Fatal(err)
	}
	times := []time.Time{
		time.Date(2026, time.August, 5, 10, 0, 0, 123, time.FixedZone("west", -7*60*60)),
		time.Date(2026, time.August, 5, 21, 0, 0, 456, time.FixedZone("east", 3*60*60)),
	}
	manager.now = func() time.Time {
		next := times[0]
		times = times[1:]
		return next
	}

	running, runningCommit, err := manager.SetState(context.Background(), "foam/transition", StatusRunning)
	if err != nil {
		t.Fatal(err)
	}
	if running.Status != StatusRunning || !running.UpdatedAt.Equal(time.Date(2026, time.August, 5, 17, 0, 0, 123, time.UTC)) || running.UpdatedAt.Location() != time.UTC {
		t.Fatalf("running state = %#v", running)
	}
	if got := repo.gitOutput(t, "rev-parse", "exp/foam/transition"); got != runningCommit {
		t.Fatalf("branch tip = %s, want %s", got, runningCommit)
	}
	assertOnlyStateChanged(t, repo, runningCommit, "foam--transition")

	if err := os.WriteFile(filepath.Join(created.WorktreePath, "implementation.txt"), []byte("coherent work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.gitOutputAt(t, created.WorktreePath, "add", "implementation.txt")
	repo.gitOutputAt(t, created.WorktreePath, "commit", "-m", "feat: implementation")
	implementationCommit := repo.gitOutputAt(t, created.WorktreePath, "rev-parse", "HEAD")

	review, reviewCommit, err := manager.SetState(context.Background(), "foam/transition", StatusReviewPending)
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != StatusReviewPending || !review.UpdatedAt.Equal(time.Date(2026, time.August, 5, 18, 0, 0, 456, time.UTC)) || review.UpdatedAt.Location() != time.UTC {
		t.Fatalf("review state = %#v", review)
	}
	if got := repo.gitOutput(t, "rev-parse", reviewCommit+"^"); got != implementationCommit {
		t.Fatalf("review parent = %s, want implementation %s", got, implementationCommit)
	}
	assertOnlyStateChanged(t, repo, reviewCommit, "foam--transition")
}

func TestSetStateRejectsInvalidTransitionWithoutChangingFilesOrRef(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "invalid"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	beforeData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	beforeTip := repo.gitOutput(t, "rev-parse", created.State.Branch)

	if _, _, err := manager.SetState(context.Background(), "foam/invalid", StatusMerged); err == nil || !strings.Contains(err.Error(), "invalid transition") {
		t.Fatalf("error = %v", err)
	}
	afterData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterData) != string(beforeData) || repo.gitOutput(t, "rev-parse", created.State.Branch) != beforeTip {
		t.Fatal("invalid transition changed state file or branch")
	}
}

func TestSetStateRejectsKindThatContradictsBranchNamespace(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "wrong-kind"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	state := created.State
	state.Kind = KindIntegration
	if err := writeJSONAtomic(statePath, state); err != nil {
		t.Fatal(err)
	}
	repo.gitOutputAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join("experiments", "active", "foam--wrong-kind", "state.json")))
	repo.gitOutputAt(t, created.WorktreePath, "commit", "-m", "test: contradict branch kind")
	before := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/wrong-kind")

	if _, _, err := manager.SetState(context.Background(), "foam/wrong-kind", StatusRunning); err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("error = %v, want kind mismatch", err)
	}
	if got := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/wrong-kind"); got != before {
		t.Fatalf("branch changed to %s, want %s", got, before)
	}
}

func TestSetStateRestoresOldFileWhenBranchAdvanceDefeatsCommit(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "rollback"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	oldState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	advanced := ""
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint != "during-record-commit" {
			return nil
		}
		parent := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/rollback")
		tree := repo.gitOutput(t, "rev-parse", parent+"^{tree}")
		advanced = repo.gitOutput(t, "commit-tree", tree, "-p", parent, "-m", "test: concurrent branch advance")
		repo.git(t, "update-ref", "refs/heads/exp/foam/rollback", advanced, parent)
		return nil
	}

	returned, commit, err := manager.SetState(context.Background(), "foam/rollback", StatusRunning)
	if err == nil || !strings.Contains(err.Error(), "compare-and-swap") {
		t.Fatalf("error = %v, want compare-and-swap failure", err)
	}
	if commit != "" || returned.Status != StatusCreated || !statesEqual(returned, created.State) {
		t.Fatalf("returned state = %#v commit = %q, want old created state and empty commit", returned, commit)
	}
	if got := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/rollback"); got != advanced {
		t.Fatalf("branch = %s, want concurrent advance %s", got, advanced)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, oldState) {
		t.Fatalf("state file was not restored\nold: %s\nafter: %s", oldState, after)
	}
	status := repo.gitOutputAt(t, created.WorktreePath, "status", "--porcelain")
	if status != "" {
		t.Fatalf("restored worktree is dirty: %s", status)
	}
}

func TestSetStatePreservesHumanFileChangeWhenCommitFails(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "human-race"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	humanContent := []byte("human content\n")
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint != "during-record-commit" {
			return nil
		}
		parent := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/human-race")
		tree := repo.gitOutput(t, "rev-parse", parent+"^{tree}")
		advanced := repo.gitOutput(t, "commit-tree", tree, "-p", parent, "-m", "test: concurrent branch advance")
		repo.git(t, "update-ref", "refs/heads/exp/foam/human-race", advanced, parent)
		return os.WriteFile(statePath, humanContent, 0o644)
	}

	_, _, err = manager.SetState(context.Background(), "foam/human-race", StatusRunning)
	if err == nil || !strings.Contains(err.Error(), "changed after state write") {
		t.Fatalf("error = %v, want guarded rollback error", err)
	}
	after, readErr := os.ReadFile(statePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(after, humanContent) {
		t.Fatalf("human state content overwritten: %q", after)
	}
	if !errors.Is(err, errStateChangedDuringCommit) {
		t.Fatalf("error = %v, want errStateChangedDuringCommit", err)
	}
}

func TestSetStateReturnsValidHumanStateWhenPreCommitRollbackIsGuarded(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "human-valid-race"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	humanState := created.State
	humanState.Status = StatusFailed
	humanState.UpdatedAt = humanState.UpdatedAt.Add(time.Minute)
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint != "during-record-commit" {
			return nil
		}
		parent := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/human-valid-race")
		tree := repo.gitOutput(t, "rev-parse", parent+"^{tree}")
		advanced := repo.gitOutput(t, "commit-tree", tree, "-p", parent, "-m", "test: concurrent branch advance")
		repo.git(t, "update-ref", "refs/heads/exp/foam/human-valid-race", advanced, parent)
		return writeJSONAtomic(statePath, humanState)
	}

	returned, commit, err := manager.SetState(context.Background(), "foam/human-valid-race", StatusRunning)
	if err == nil || !errors.Is(err, errStateChangedDuringCommit) {
		t.Fatalf("error = %v, want guarded rollback error", err)
	}
	if commit != "" || !statesEqual(returned, created.State) {
		t.Fatalf("returned state = %#v commit = %q, want authoritative branch state", returned, commit)
	}
	current, readErr := readState(statePath)
	if readErr != nil || !statesEqual(current, humanState) {
		t.Fatalf("human state was not preserved: state %#v error %v", current, readErr)
	}
}

func TestSetStateFallsBackToOldStateWhenGuardedCurrentStateIsMalformed(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "human-malformed-race"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	malformed := []byte("human content\n")
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint != "during-record-commit" {
			return nil
		}
		parent := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/human-malformed-race")
		tree := repo.gitOutput(t, "rev-parse", parent+"^{tree}")
		advanced := repo.gitOutput(t, "commit-tree", tree, "-p", parent, "-m", "test: concurrent branch advance")
		repo.git(t, "update-ref", "refs/heads/exp/foam/human-malformed-race", advanced, parent)
		return os.WriteFile(statePath, malformed, 0o644)
	}

	returned, commit, err := manager.SetState(context.Background(), "foam/human-malformed-race", StatusRunning)
	if err == nil || !errors.Is(err, errStateChangedDuringCommit) {
		t.Fatalf("error = %v, want guarded rollback error", err)
	}
	if commit != "" || !statesEqual(returned, created.State) {
		t.Fatalf("returned state = %#v commit = %q, want old state fallback", returned, commit)
	}
	current, readErr := os.ReadFile(statePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(current, malformed) {
		t.Fatalf("malformed human content changed to %q", current)
	}
}

func TestSetStatePreservesAppliedCommitWhenHEADChangesAfterRefUpdate(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "post-commit-head"})
	if err != nil {
		t.Fatal(err)
	}
	repo.git(t, "branch", "human-branch")
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == "after-record-ref-update" {
			repo.gitOutputAt(t, created.WorktreePath, "symbolic-ref", "HEAD", "refs/heads/human-branch")
		}
		return nil
	}

	state, commit, err := manager.SetState(context.Background(), "foam/post-commit-head", StatusRunning)
	repo.gitOutputAt(t, created.WorktreePath, "symbolic-ref", "HEAD", "refs/heads/exp/foam/post-commit-head")
	if err == nil || commit == "" || !strings.Contains(err.Error(), commit) || !strings.Contains(err.Error(), "was applied") {
		t.Fatalf("SetState = state %#v commit %q error %v", state, commit, err)
	}
	var applied *AppliedCommitError
	if !errors.As(err, &applied) || applied.Commit != commit {
		t.Fatalf("applied commit error = %#v, want commit %s", applied, commit)
	}
	if state.Status != StatusRunning || repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/post-commit-head") != commit {
		t.Fatalf("applied state/commit was lost: state=%#v commit=%q", state, commit)
	}
	stored, readErr := readState(filepath.Join(filepath.Dir(created.BriefPath), "state.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if stored.Status != StatusRunning {
		t.Fatalf("stored status = %s, want running", stored.Status)
	}
}

func TestSetStatePreservesAppliedCommitOnPostIndexSyncError(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "post-index-sync"})
	if err != nil {
		t.Fatal(err)
	}
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == "after-record-index-sync" {
			return errors.New("injected post-index synchronization failure")
		}
		return nil
	}

	state, commit, err := manager.SetState(context.Background(), "foam/post-index-sync", StatusRunning)
	if err == nil || commit == "" || !strings.Contains(err.Error(), commit) || !strings.Contains(err.Error(), "was applied") {
		t.Fatalf("SetState = state %#v commit %q error %v", state, commit, err)
	}
	if state.Status != StatusRunning || repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/post-index-sync") != commit {
		t.Fatalf("applied state/commit was lost: state=%#v commit=%q", state, commit)
	}
	stored, readErr := readState(filepath.Join(filepath.Dir(created.BriefPath), "state.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if stored.Status != StatusRunning {
		t.Fatalf("stored status = %s, want running", stored.Status)
	}
	if status := repo.gitOutputAt(t, created.WorktreePath, "status", "--porcelain"); status != "" {
		t.Fatalf("post-index-sync error left dirty worktree: %s", status)
	}
}

func TestSetStateExposesAppliedCommitWhenRealIndexSyncFails(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "index-sync-failure"})
	if err != nil {
		t.Fatal(err)
	}
	indexPath := repo.gitOutputAt(t, created.WorktreePath, "rev-parse", "--path-format=absolute", "--git-path", "index")
	indexLock := indexPath + ".lock"
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == "after-record-ref-update" {
			return os.WriteFile(indexLock, []byte("locked\n"), 0o644)
		}
		return nil
	}

	state, commit, err := manager.SetState(context.Background(), "foam/index-sync-failure", StatusRunning)
	if removeErr := os.Remove(indexLock); removeErr != nil {
		t.Fatal(removeErr)
	}
	if err == nil || commit == "" || !strings.Contains(err.Error(), "update real index") {
		t.Fatalf("SetState = state %#v commit %q error %v", state, commit, err)
	}
	var applied *AppliedCommitError
	if !errors.As(err, &applied) || applied.Commit != commit {
		t.Fatalf("applied commit error = %#v, want commit %s", applied, commit)
	}
	if got := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/index-sync-failure"); got != commit {
		t.Fatalf("branch = %s, want applied commit %s", got, commit)
	}
	statePath := filepath.ToSlash(filepath.Join("experiments", "active", "foam--index-sync-failure", "state.json"))
	branchState := repo.gitOutput(t, "show", "refs/heads/exp/foam/index-sync-failure:"+statePath)
	fileState, readErr := os.ReadFile(filepath.Join(created.WorktreePath, filepath.FromSlash(statePath)))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(fileState)) != branchState || state.Status != StatusRunning {
		t.Fatalf("state file does not match applied tree: state=%#v\nfile=%s\nbranch=%s", state, fileState, branchState)
	}
}

func TestSetStateUsesFullyQualifiedBranchWhenTagHasSameShortName(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "state-tag-shadow"}); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "tag", "exp/foam/state-tag-shadow", "master")

	state, _, err := manager.SetState(context.Background(), "foam/state-tag-shadow", StatusRunning)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != StatusRunning {
		t.Fatalf("status = %s", state.Status)
	}
}

func TestSetStateRejectsDirtyMismatchedOrMissingAssignedWorktree(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, testRepo, Created)
		want    string
	}{
		{
			name: "dirty",
			prepare: func(t *testing.T, _ testRepo, created Created) {
				if err := os.WriteFile(filepath.Join(created.WorktreePath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "not clean",
		},
		{
			name: "wrong branch",
			prepare: func(t *testing.T, repo testRepo, created Created) {
				repo.git(t, "branch", "other")
				repo.gitOutputAt(t, created.WorktreePath, "checkout", "other")
			},
			want: "branch",
		},
		{
			name: "missing directory",
			prepare: func(t *testing.T, _ testRepo, created Created) {
				if err := os.Rename(created.WorktreePath, created.WorktreePath+"-missing"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Rename(created.WorktreePath+"-missing", created.WorktreePath) })
			},
			want: "missing",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newTestRepo(t)
			manager := testManager(t, repo)
			created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "unsafe"})
			if err != nil {
				t.Fatal(err)
			}
			test.prepare(t, repo, created)
			beforeTip := repo.gitOutput(t, "rev-parse", created.State.Branch)

			if _, _, err := manager.SetState(context.Background(), "foam/unsafe", StatusRunning); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			if got := repo.gitOutput(t, "rev-parse", created.State.Branch); got != beforeTip {
				t.Fatalf("branch changed to %s, want %s", got, beforeTip)
			}
		})
	}
}

func TestSetStateConcurrentUpdatesSerializeAndLeaveValidJSON(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "concurrent"})
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC) }

	start := make(chan struct{})
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := manager.SetState(context.Background(), "foam/concurrent", StatusRunning)
			errors <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errors)
	successes := 0
	for err := range errors {
		if err == nil {
			successes++
			continue
		}
		if !strings.Contains(err.Error(), "invalid transition") {
			t.Errorf("concurrent error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful transitions = %d, want 1", successes)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(created.BriefPath), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("state JSON is corrupt: %v\n%s", err, data)
	}
	if state.Status != StatusRunning {
		t.Fatalf("status = %s", state.Status)
	}
}

func TestSetStateRejectsThirdActiveWriter(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	for _, name := range []string{"first", "second", "third"} {
		if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: name, MaxWriters: 3}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"first", "second"} {
		if _, _, err := manager.SetStateWithOptions(context.Background(), "foam/"+name, StatusRunning, StateOptions{MaxWriters: 2}); err != nil {
			t.Fatal(err)
		}
	}

	state, commit, err := manager.SetStateWithOptions(context.Background(), "foam/third", StatusRunning, StateOptions{MaxWriters: 2})
	if err == nil || !strings.Contains(err.Error(), "maximum active writers reached (2)") {
		t.Fatalf("SetState = state %#v commit %q error %v", state, commit, err)
	}
	if commit != "" || state.Status != "" {
		t.Fatalf("rejected transition returned state %#v commit %q", state, commit)
	}
}

func TestSetStateConcurrentWriterAdmissionHonorsOneWriterLimit(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	for _, name := range []string{"first", "second"} {
		if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: name, MaxWriters: 2}); err != nil {
			t.Fatal(err)
		}
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := manager.SetStateWithOptions(context.Background(), "foam/"+name, StatusRunning, StateOptions{MaxWriters: 1})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	succeeded, rejected := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case strings.Contains(err.Error(), "maximum active writers reached (1)"):
			rejected++
		default:
			t.Fatalf("unexpected transition error: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("successes=%d rejections=%d, want 1 each", succeeded, rejected)
	}
}

func TestSetStateRecognizesExternallyAppliedGeneratedCommit(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "external-generated"})
	if err != nil {
		t.Fatal(err)
	}
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint != "before-record-ref-update" {
			return nil
		}
		commit := findUnreachableCommitBySubject(t, repo, "experiment: set foam/external-generated running")
		parent := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/external-generated")
		repo.git(t, "update-ref", "refs/heads/exp/foam/external-generated", commit, parent)
		return nil
	}

	state, commit, err := manager.SetState(context.Background(), "foam/external-generated", StatusRunning)
	if err == nil || commit == "" || state.Status != StatusRunning {
		t.Fatalf("SetState = state %#v commit %q error %v", state, commit, err)
	}
	var applied *AppliedCommitError
	if !errors.As(err, &applied) || applied.Commit != commit {
		t.Fatalf("applied error = %#v, commit %q", applied, commit)
	}
	if got := repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/external-generated"); got != commit {
		t.Fatalf("branch = %s, want %s", got, commit)
	}
	stored, readErr := readState(filepath.Join(filepath.Dir(created.BriefPath), "state.json"))
	if readErr != nil || stored.Status != StatusRunning {
		t.Fatalf("stored state = %#v error %v", stored, readErr)
	}
}

func TestSetStateReconcilesLocalFileToDifferentAuthoritativeCommit(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "external-different"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	resultPath := filepath.Join(filepath.Dir(created.BriefPath), "result.md")
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint != "during-record-commit" {
			return nil
		}
		attempted, err := os.ReadFile(statePath)
		if err != nil {
			return err
		}
		authoritative := created.State
		authoritative.Status = StatusFailed
		authoritative.UpdatedAt = authoritative.UpdatedAt.Add(time.Minute)
		if err := writeJSONAtomic(statePath, authoritative); err != nil {
			return err
		}
		if err := os.WriteFile(resultPath, []byte("external result\n"), 0o644); err != nil {
			return err
		}
		repo.gitOutputAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join("experiments", "active", "foam--external-different", "state.json")), filepath.ToSlash(filepath.Join("experiments", "active", "foam--external-different", "result.md")))
		repo.gitOutputAt(t, created.WorktreePath, "commit", "-m", "test: external authoritative state")
		return os.WriteFile(statePath, attempted, 0o644)
	}

	state, commit, err := manager.SetState(context.Background(), "foam/external-different", StatusRunning)
	if err == nil || commit != "" || state.Status != StatusFailed {
		t.Fatalf("SetState = state %#v commit %q error %v", state, commit, err)
	}
	stored, readErr := readState(statePath)
	if readErr != nil || stored.Status != StatusFailed {
		t.Fatalf("stored state = %#v error %v", stored, readErr)
	}
	if got := repo.gitOutputAt(t, created.WorktreePath, "status", "--porcelain"); got != "" {
		t.Fatalf("reconciliation left dirty worktree: %s", got)
	}
}

func TestSetStateReturnsCompetingCommitWhenItAppliesTargetState(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "external-target"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	resultPath := filepath.Join(filepath.Dir(created.BriefPath), "result.md")
	competingCommit := ""
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint != "during-record-commit" {
			return nil
		}
		attempted, err := os.ReadFile(statePath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(resultPath, []byte("external result\n"), 0o644); err != nil {
			return err
		}
		repo.gitOutputAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join("experiments", "active", "foam--external-target", "state.json")), filepath.ToSlash(filepath.Join("experiments", "active", "foam--external-target", "result.md")))
		repo.gitOutputAt(t, created.WorktreePath, "commit", "-m", "test: external target state")
		competingCommit = repo.gitOutput(t, "rev-parse", "refs/heads/exp/foam/external-target")
		return os.WriteFile(statePath, attempted, 0o644)
	}

	state, commit, err := manager.SetState(context.Background(), "foam/external-target", StatusRunning)
	if err == nil || commit != competingCommit || state.Status != StatusRunning {
		t.Fatalf("SetState = state %#v commit %q error %v, want competing commit %q", state, commit, err, competingCommit)
	}
	var applied *AppliedCommitError
	if !errors.As(err, &applied) || applied.Commit != competingCommit {
		t.Fatalf("applied error = %#v, want commit %q", applied, competingCommit)
	}
}

func findUnreachableCommitBySubject(t *testing.T, repo testRepo, subject string) string {
	t.Helper()
	output := repo.gitOutput(t, "fsck", "--unreachable", "--no-reflogs")
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[1] != "commit" {
			continue
		}
		if repo.gitOutput(t, "show", "-s", "--format=%s", fields[2]) == subject {
			return fields[2]
		}
	}
	t.Fatalf("unreachable commit with subject %q not found in %q", subject, output)
	return ""
}

func assertOnlyStateChanged(t *testing.T, repo testRepo, commit, flatID string) {
	t.Helper()
	got := strings.Fields(repo.gitOutput(t, "diff-tree", "--no-commit-id", "--name-only", "-r", commit))
	want := filepath.ToSlash(filepath.Join("experiments", "active", flatID, "state.json"))
	if len(got) != 1 || got[0] != want {
		t.Fatalf("commit paths = %v, want [%s]", got, want)
	}
}
