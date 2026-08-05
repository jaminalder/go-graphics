package experiment

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareIntegrationPinsSourcesOnCurrentMasterWithoutApplyingThem(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	first, firstTip := createIntegrationSource(t, repo, manager, "depth-fields")
	second, secondTip := createIntegrationSource(t, repo, manager, "hatching-pass")

	if err := os.WriteFile(filepath.Join(repo.root, "master-advance.txt"), []byte("current architecture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "add", "master-advance.txt")
	repo.git(t, "commit", "-m", "test: advance master")
	masterTip := repo.gitOutput(t, "rev-parse", "master")

	created, err := manager.PrepareIntegration(context.Background(), IntegrationOptions{
		Name:     "foam/depth-hatching-v1",
		Sources:  []string{first.State.ID, second.State.ID},
		Stage:    "hatching",
		Keep:     "Depth-responsive hatch density from the depth experiment.",
		Reject:   "Its unrelated palette and layout changes.",
		Preserve: "Existing foam boundaries and deterministic seed behavior.",
		Profile:  "preview",
		Seeds:    []uint64{2, 3, 5},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := created.State.Branch, "integrate/foam/depth-hatching-v1"; got != want {
		t.Fatalf("branch = %q, want %q", got, want)
	}
	if branch := repo.gitOutput(t, "branch", "--list", "exp/foam/depth-hatching-v1"); branch != "" {
		t.Fatalf("ordinary experiment branch remains after integration preparation: %s", branch)
	}
	if created.State.Kind != KindIntegration || created.State.Status != StatusIntegrationPending {
		t.Fatalf("integration identity = kind %q, status %q", created.State.Kind, created.State.Status)
	}
	if created.State.BaseBranch != "master" || created.State.BaseCommit != masterTip || created.State.BaseExperiment != "" {
		t.Fatalf("integration base = %#v, want current master %s", created.State, masterTip)
	}
	if got := repo.gitOutput(t, "rev-parse", created.State.Branch+"^"); got != masterTip {
		t.Fatalf("integration parent = %s, want current master %s", got, masterTip)
	}
	wantSources := []Source{{ID: first.State.ID, Commit: firstTip}, {ID: second.State.ID, Commit: secondTip}}
	if len(created.State.Sources) != len(wantSources) {
		t.Fatalf("sources = %#v, want %#v", created.State.Sources, wantSources)
	}
	for i, want := range wantSources {
		if created.State.Sources[i] != want {
			t.Errorf("source %d = %#v, want %#v", i, created.State.Sources[i], want)
		}
		if gitIsAncestor(t, repo, want.Commit, created.State.Branch) {
			t.Errorf("source implementation commit %s is an ancestor of integration", want.Commit)
		}
	}
	shown, err := manager.Show(context.Background(), created.State.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shown.Diagnostics) != 0 || !statesEqual(shown.State, created.State) {
		t.Fatalf("prepared integration does not reconcile: %#v", shown)
	}
	changed := strings.Fields(repo.gitOutput(t, "diff-tree", "--no-commit-id", "--name-only", "-r", created.State.Branch))
	for _, path := range changed {
		if !strings.HasPrefix(path, "experiments/active/foam--depth-hatching-v1/") {
			t.Errorf("integration preparation applied source path %q", path)
		}
	}

	brief := readTextFile(t, created.BriefPath)
	for _, fragment := range []string{
		"## Keep",
		"Depth-responsive hatch density",
		"## Reject",
		"unrelated palette and layout changes",
		"## Preserve",
		"Existing foam boundaries",
		"## Sources",
		first.State.ID,
		firstTip,
		second.State.ID,
		secondTip,
		"## Dependencies",
		"## Comparison requirements",
	} {
		if !strings.Contains(brief, fragment) {
			t.Errorf("integration brief does not contain %q", fragment)
		}
	}
}

func TestPrepareIntegrationRejectsNonAuthoritativeSources(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, testRepo, *Manager) []string
		want    string
	}{
		{
			name: "missing source",
			prepare: func(_ *testing.T, _ testRepo, _ *Manager) []string {
				return []string{"foam/missing-source"}
			},
			want: "has no active branch",
		},
		{
			name: "duplicate source",
			prepare: func(t *testing.T, repo testRepo, manager *Manager) []string {
				source, _ := createIntegrationSource(t, repo, manager, "duplicate-source")
				return []string{source.State.ID, source.State.ID}
			},
			want: "duplicate integration source",
		},
		{
			name: "discarded source",
			prepare: func(t *testing.T, repo testRepo, manager *Manager) []string {
				source, _ := createIntegrationSource(t, repo, manager, "discarded-source")
				if _, _, err := manager.SetState(context.Background(), source.State.ID, StatusDiscarded); err != nil {
					t.Fatal(err)
				}
				return []string{source.State.ID}
			},
			want: "is discarded",
		},
		{
			name: "uncommitted source record",
			prepare: func(t *testing.T, repo testRepo, manager *Manager) []string {
				source, _ := createIntegrationSource(t, repo, manager, "dirty-source")
				if err := os.WriteFile(source.BriefPath, []byte("uncommitted source record\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return []string{source.State.ID}
			},
			want: "dirty-worktree",
		},
		{
			name: "missing committed source record",
			prepare: func(t *testing.T, repo testRepo, manager *Manager) []string {
				source, _ := createIntegrationSource(t, repo, manager, "missing-record")
				statePath := filepath.Join(filepath.Dir(source.BriefPath), "state.json")
				repo.gitOutputAt(t, source.WorktreePath, "rm", statePath)
				repo.gitOutputAt(t, source.WorktreePath, "commit", "-m", "test: remove source record")
				return []string{source.State.ID}
			},
			want: "missing-record",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newTestRepo(t)
			manager := testManager(t, repo)
			sources := test.prepare(t, repo, manager)
			_, err := manager.PrepareIntegration(context.Background(), IntegrationOptions{
				Name:     "foam/rejected-integration",
				Sources:  sources,
				Keep:     "selected behavior",
				Reject:   "unselected behavior",
				Preserve: "stable behavior",
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			if branch := repo.gitOutput(t, "branch", "--list", "integrate/foam/rejected-integration"); branch != "" {
				t.Fatalf("integration branch exists after rejected source: %s", branch)
			}
		})
	}
}

func createIntegrationSource(t *testing.T, repo testRepo, manager *Manager, name string) (Created, string) {
	t.Helper()
	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: name})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(created.WorktreePath, name+".txt")
	if err := os.WriteFile(path, []byte(name+" implementation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.gitOutputAt(t, created.WorktreePath, "add", filepath.Base(path))
	repo.gitOutputAt(t, created.WorktreePath, "commit", "-m", "experiment: implement "+name)
	return created, repo.gitOutputAt(t, created.WorktreePath, "rev-parse", "HEAD")
}

func gitIsAncestor(t *testing.T, repo testRepo, ancestor, descendant string) bool {
	t.Helper()
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = repo.root
	cmd.Env = repo.gitEnv
	err := cmd.Run()
	if err == nil {
		return true
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false
	}
	t.Fatalf("check ancestry %s..%s: %v", ancestor, descendant, err)
	return false
}
