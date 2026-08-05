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

	if _, _, err := manager.SetState(context.Background(), "foam/rollback", StatusRunning); err == nil || !strings.Contains(err.Error(), "compare-and-swap") {
		t.Fatalf("error = %v, want compare-and-swap failure", err)
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

func assertOnlyStateChanged(t *testing.T, repo testRepo, commit, flatID string) {
	t.Helper()
	got := strings.Fields(repo.gitOutput(t, "diff-tree", "--no-commit-id", "--name-only", "-r", commit))
	want := filepath.ToSlash(filepath.Join("experiments", "active", flatID, "state.json"))
	if len(got) != 1 || got[0] != want {
		t.Fatalf("commit paths = %v, want [%s]", got, want)
	}
}
