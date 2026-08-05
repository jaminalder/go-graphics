package experiment

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestListSortsExperimentsByIDWithoutDuplicates(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	for _, name := range []string{"zebra", "alpha"} {
		if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	repo.git(t, "branch", "integrate/foam/alpha", "exp/foam/alpha")

	experiments, err := manager.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := experimentIDs(experiments); !slices.Equal(got, []string{"foam/alpha", "foam/zebra"}) {
		t.Fatalf("IDs = %v", got)
	}
	if !hasDiagnostic(experiments[0], "ambiguous-ref") {
		t.Fatalf("alpha diagnostics = %#v, want ambiguous-ref", experiments[0].Diagnostics)
	}
}

func TestShowReturnsExactStateAndPathReturnsAbsoluteAssignedWorktree(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "inspect"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := manager.Show(context.Background(), "foam/inspect")
	if err != nil {
		t.Fatal(err)
	}
	if !statesEqual(got.State, created.State) || got.WorktreePath != created.WorktreePath || len(got.Diagnostics) != 0 {
		t.Fatalf("Show = %#v, want state %#v at %q", got, created.State, created.WorktreePath)
	}
	path, err := manager.Path(context.Background(), "foam/inspect")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || path != created.WorktreePath {
		t.Fatalf("Path = %q, want absolute %q", path, created.WorktreePath)
	}
}

func TestReconcileReportsRegisteredMissingDirectoryAsStaleWithoutRepair(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "stale"})
	if err != nil {
		t.Fatal(err)
	}
	moved := created.WorktreePath + "-moved"
	if err := os.Rename(created.WorktreePath, moved); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(moved, created.WorktreePath) })

	got, err := manager.Show(context.Background(), "foam/stale")
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"missing-worktree-directory", "stale-worktree-metadata"} {
		if !hasDiagnostic(got, code) {
			t.Errorf("diagnostics = %#v, want %s", got.Diagnostics, code)
		}
	}
	if got.State.ID != created.State.ID {
		t.Fatalf("fallback state ID = %q", got.State.ID)
	}
	if _, err := os.Stat(created.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("Show recreated missing worktree: %v", err)
	}
}

func TestReconcileKeepsBranchRecordShowableWithoutRegisteredWorktree(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "branch-only"})
	if err != nil {
		t.Fatal(err)
	}
	repo.git(t, "worktree", "remove", created.WorktreePath)

	got, err := manager.Show(context.Background(), "foam/branch-only")
	if err != nil {
		t.Fatal(err)
	}
	if got.State.ID != "foam/branch-only" || !hasDiagnostic(got, "missing-worktree-directory") || hasDiagnostic(got, "stale-worktree-metadata") {
		t.Fatalf("Show = %#v", got)
	}
}

func TestReconcileReportsMissingBranchRecord(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "missing-record"})
	if err != nil {
		t.Fatal(err)
	}
	repo.gitOutputAt(t, created.WorktreePath, "rm", filepath.ToSlash(filepath.Join("experiments", "active", "foam--missing-record", "state.json")))
	repo.gitOutputAt(t, created.WorktreePath, "commit", "-m", "test: remove state record")

	got, err := manager.Show(context.Background(), "foam/missing-record")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(got, "missing-record") {
		t.Fatalf("diagnostics = %#v", got.Diagnostics)
	}
}

func TestReconcileReportsStateBranchPathAndDirtyDrift(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "drift"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
	state := created.State
	state.Branch = "exp/foam/other"
	state.Worktree = "../wrong/path"
	if err := writeJSONAtomic(statePath, state); err != nil {
		t.Fatal(err)
	}
	repo.gitOutputAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join("experiments", "active", "foam--drift", "state.json")))
	repo.gitOutputAt(t, created.WorktreePath, "commit", "-m", "test: record drift")
	if err := os.WriteFile(filepath.Join(created.WorktreePath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := manager.Show(context.Background(), "foam/drift")
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"branch-mismatch", "path-mismatch", "dirty-worktree"} {
		if !hasDiagnostic(got, code) {
			t.Errorf("diagnostics = %#v, want %s", got.Diagnostics, code)
		}
	}
	if _, err := manager.Path(context.Background(), "foam/drift"); err == nil || !strings.Contains(err.Error(), "path-mismatch") {
		t.Fatalf("Path error = %v, want path-mismatch", err)
	}
}

func TestReconcileReportsActualWorktreeAtWrongPath(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "moved"})
	if err != nil {
		t.Fatal(err)
	}
	wrongPath := created.WorktreePath + "-wrong"
	repo.git(t, "worktree", "move", created.WorktreePath, wrongPath)

	got, err := manager.Show(context.Background(), "foam/moved")
	if err != nil {
		t.Fatal(err)
	}
	if got.WorktreePath != wrongPath || !hasDiagnostic(got, "path-mismatch") {
		t.Fatalf("Show = %#v, want actual path and path-mismatch", got)
	}
	if _, err := manager.Path(context.Background(), "foam/moved"); err == nil || !strings.Contains(err.Error(), "path-mismatch") {
		t.Fatalf("Path error = %v, want path-mismatch", err)
	}
}

func TestShowRejectsAmbiguousExperimentAndIntegrationRefs(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "ambiguous"}); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "branch", "integrate/foam/ambiguous", "exp/foam/ambiguous")

	if _, err := manager.Show(context.Background(), "foam/ambiguous"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("Show error = %v", err)
	}
	if _, err := manager.Path(context.Background(), "foam/ambiguous"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("Path error = %v", err)
	}
}

func TestListShowAndPathAreReadOnly(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "read-only"}); err != nil {
		t.Fatal(err)
	}
	before := repositorySnapshot(t, repo)

	if _, err := manager.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Show(context.Background(), "foam/read-only"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Path(context.Background(), "foam/read-only"); err != nil {
		t.Fatal(err)
	}
	after := repositorySnapshot(t, repo)
	if before != after {
		t.Fatalf("read-only inspection changed repository\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func experimentIDs(experiments []Experiment) []string {
	ids := make([]string, len(experiments))
	for i := range experiments {
		ids[i] = experiments[i].State.ID
	}
	return ids
}

func hasDiagnostic(experiment Experiment, code string) bool {
	return slices.ContainsFunc(experiment.Diagnostics, func(diagnostic Diagnostic) bool {
		return diagnostic.Code == code
	})
}

func repositorySnapshot(t *testing.T, repo testRepo) string {
	t.Helper()
	return strings.Join([]string{
		repo.gitOutput(t, "branch", "--show-current"),
		repo.gitOutput(t, "rev-parse", "HEAD"),
		repo.gitOutput(t, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads"),
		repo.gitOutput(t, "status", "--porcelain=v1", "--untracked-files=all"),
	}, "\n---\n")
}
