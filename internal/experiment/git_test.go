package experiment

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseWorktreeListHandlesBranchesDetachedBareAndPrunableRecords(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"worktree /repo/main",
		"HEAD 0123456789abcdef",
		"branch refs/heads/master",
		"",
		"",
		"worktree /repo/detached",
		"HEAD fedcba9876543210",
		"detached",
		"prunable stale administrative files",
		"",
		"worktree /repo/bare.git",
		"bare",
	}, "\n")

	want := []WorktreeInfo{
		{Path: "/repo/main", HEAD: "0123456789abcdef", Branch: "refs/heads/master"},
		{Path: "/repo/detached", HEAD: "fedcba9876543210", Prunable: true},
		{Path: "/repo/bare.git", Bare: true},
	}
	got, err := parseWorktreeList(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWorktreeList() = %#v, want %#v", got, want)
	}
}

func TestParseWorktreeListRejectsFieldsBeforeWorktree(t *testing.T) {
	t.Parallel()

	if _, err := parseWorktreeList("HEAD abc123\n"); err == nil {
		t.Fatal("parseWorktreeList accepted a field without a worktree record")
	}
}

func TestGitRunnerReturnsMeaningfulStderr(t *testing.T) {
	repo := newTestRepo(t)
	_, err := (gitRunner{dir: repo.root, env: repo.gitEnv}).run(context.Background(), "rev-parse", "--verify", "missing-ref")
	if err == nil {
		t.Fatal("git command unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "missing-ref") || !strings.Contains(err.Error(), "fatal:") {
		t.Fatalf("error %q does not include command and stderr", err)
	}
}

func TestDiscoverFindsCoordinatorCommonDirectoryAndCurrentWorktree(t *testing.T) {
	repo := newTestRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	repo.git(t, "worktree", "add", "-b", "exp/foam/test", linked)

	manager, err := NewManager(linked)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, err := filepath.EvalSymlinks(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	wantLinked, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	wantCommon, err := filepath.EvalSymlinks(filepath.Join(repo.root, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if manager.CoordinatorRoot != wantRoot {
		t.Errorf("CoordinatorRoot = %q, want %q", manager.CoordinatorRoot, wantRoot)
	}
	if manager.CurrentRoot != wantLinked {
		t.Errorf("CurrentRoot = %q, want %q", manager.CurrentRoot, wantLinked)
	}
	if manager.CommonDir != wantCommon {
		t.Errorf("CommonDir = %q, want %q", manager.CommonDir, wantCommon)
	}
	if manager.TemplatesRoot != filepath.Join(wantRoot, "experiments", "templates") {
		t.Errorf("TemplatesRoot = %q", manager.TemplatesRoot)
	}
}

func TestDiscoverHandlesSymlinkedWorkingDirectory(t *testing.T) {
	repo := newTestRepo(t)
	link := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(repo.root, link); err != nil {
		t.Fatal(err)
	}

	manager, err := NewManager(filepath.Join(link, "experiments"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	if manager.CurrentRoot != want || manager.CoordinatorRoot != want {
		t.Fatalf("roots = current %q coordinator %q, want %q", manager.CurrentRoot, manager.CoordinatorRoot, want)
	}
}

func TestCoordinatorMutationIsRejectedFromLinkedWorktree(t *testing.T) {
	repo := newTestRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	repo.git(t, "worktree", "add", "-b", "exp/foam/test", linked)

	manager, err := NewManager(linked)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RequireCoordinator(); err == nil {
		t.Fatal("RequireCoordinator succeeded in a linked worktree")
	} else if !strings.Contains(err.Error(), manager.CoordinatorRoot) {
		t.Fatalf("error %q does not identify coordinator", err)
	}

	coordinator, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.RequireCoordinator(); err != nil {
		t.Fatalf("RequireCoordinator at coordinator: %v", err)
	}
}
