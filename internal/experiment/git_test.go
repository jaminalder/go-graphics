package experiment

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseWorktreeListHandlesBranchesDetachedBareAndPrunableRecords(t *testing.T) {
	t.Parallel()

	whitespacePath := "/repo/line one\nline two "
	input := strings.Join([]string{
		"worktree " + whitespacePath,
		"HEAD 0123456789abcdef",
		"branch refs/heads/master",
		"",
		"worktree /repo/detached",
		"HEAD fedcba9876543210",
		"detached",
		"prunable stale administrative files",
		"",
		"worktree /repo/bare.git",
		"bare",
		"",
	}, "\x00")

	want := []WorktreeInfo{
		{Path: whitespacePath, HEAD: "0123456789abcdef", Branch: "refs/heads/master"},
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

	if _, err := parseWorktreeList("HEAD abc123\x00"); err == nil {
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
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %T does not wrap *exec.ExitError", err)
	}
}

func TestGitRunnerPreservesContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (gitRunner{dir: t.TempDir()}).run(ctx, "version")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestNewManagerIgnoresAmbientGitRouting(t *testing.T) {
	repo := newTestRepo(t)
	ambient := newTestRepo(t)
	nested := filepath.Join(repo.root, "experiments", "active")

	cmd := exec.Command(os.Args[0], "-test.run=^TestNewManagerWithPoisonedProcess$")
	cmd.Env = append([]string(nil), repo.gitEnv...)
	cmd.Env = append(cmd.Env,
		"EXPERIMENT_TEST_TARGET="+nested,
		"GIT_DIR="+filepath.Join(ambient.root, ".git"),
		"GIT_WORK_TREE="+ambient.root,
		"GIT_COMMON_DIR="+filepath.Join(ambient.root, ".git"),
		"GIT_INDEX_FILE="+filepath.Join(ambient.root, ".git", "index"),
		"GIT_OBJECT_DIRECTORY="+filepath.Join(ambient.root, ".git", "objects"),
		"GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(ambient.root, ".git", "objects"),
		"GIT_PREFIX=poisoned/",
		"GIT_CEILING_DIRECTORIES="+nested,
		"GIT_DISCOVERY_ACROSS_FILESYSTEM=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.bare",
		"GIT_CONFIG_VALUE_0=true",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("poisoned NewManager subprocess: %v\n%s", err, output)
	}
}

func TestNewManagerWithPoisonedProcess(t *testing.T) {
	target := os.Getenv("EXPERIMENT_TEST_TARGET")
	if target == "" {
		t.Skip("subprocess helper")
	}

	manager, err := NewManager(target)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(target, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if manager.CurrentRoot != want || manager.CoordinatorRoot != want {
		t.Fatalf("roots = current %q coordinator %q, want %q", manager.CurrentRoot, manager.CoordinatorRoot, want)
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
