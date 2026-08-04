package experiment

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCreateBuildsAnIsolatedExperimentFromCoordinatorHEAD(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	originalHEAD := repo.gitOutput(t, "rev-parse", "HEAD")
	createdAt := time.Date(2026, time.August, 4, 12, 30, 0, 0, time.UTC)
	manager.now = func() time.Time { return createdAt }

	created, err := manager.Create(context.Background(), CreateOptions{
		Piece: "foam",
		Name:  "hatching-spacing-variation",
	})
	if err != nil {
		t.Fatal(err)
	}

	id := "foam/hatching-spacing-variation"
	branch := "exp/" + id
	wantWorktree := filepath.Join(filepath.Dir(manager.CoordinatorRoot), filepath.Base(manager.CoordinatorRoot)+"-worktrees", "foam--hatching-spacing-variation")
	wantBrief := filepath.Join(wantWorktree, "experiments", "active", "foam--hatching-spacing-variation", "brief.md")
	wantOutput := filepath.Join(wantWorktree, "out", "experiments", "foam", "hatching-spacing-variation")
	if got := repo.gitOutput(t, "branch", "--show-current"); got != "master" {
		t.Fatalf("coordinator branch = %q, want master", got)
	}
	if got := repo.gitOutput(t, "rev-parse", "HEAD"); got != originalHEAD {
		t.Fatalf("coordinator HEAD = %q, want %q", got, originalHEAD)
	}
	if got := repo.gitOutput(t, "rev-parse", branch+"^"); got != originalHEAD {
		t.Fatalf("experiment parent = %q, want %q", got, originalHEAD)
	}
	if created.WorktreePath != wantWorktree || created.BriefPath != wantBrief || created.OutputPath != wantOutput {
		t.Fatalf("created paths = %#v", created)
	}

	wantState := State{
		SchemaVersion: 1,
		ID:            id,
		Kind:          KindExperiment,
		Branch:        branch,
		Worktree:      filepath.ToSlash(filepath.Join("..", filepath.Base(manager.CoordinatorRoot)+"-worktrees", "foam--hatching-spacing-variation")),
		BaseBranch:    "master",
		BaseCommit:    originalHEAD,
		Status:        StatusCreated,
		Stage:         "rendering",
		Worker:        Worker{Tool: "unknown"},
		Seeds:         []uint64{1, 2, 3, 5, 8, 13},
		Profile:       "preview",
		Output:        "out/experiments/foam/hatching-spacing-variation",
		Sources:       []Source{},
		Verification:  Verification{},
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
	if !statesEqual(created.State, wantState) {
		t.Fatalf("state = %#v, want %#v", created.State, wantState)
	}

	recordDir := filepath.Join(wantWorktree, "experiments", "active", "foam--hatching-spacing-variation")
	for _, name := range []string{"state.json", "brief.md", "result.md", "favorites.json"} {
		if _, err := os.Stat(filepath.Join(recordDir, name)); err != nil {
			t.Errorf("record %s: %v", name, err)
		}
	}
	var favorites []json.RawMessage
	readJSONFile(t, filepath.Join(recordDir, "favorites.json"), &favorites)
	if favorites == nil || len(favorites) != 0 {
		t.Fatalf("favorites = %#v, want non-nil empty array", favorites)
	}
	for _, dir := range []string{"baseline", "candidate", "metadata"} {
		info, err := os.Stat(filepath.Join(wantOutput, dir))
		if err != nil || !info.IsDir() {
			t.Errorf("output directory %s: %v", dir, err)
		}
	}
	tracked := repo.gitOutputAt(t, wantWorktree, "show", "--name-only", "--format=", "HEAD")
	for _, path := range []string{
		"experiments/active/foam--hatching-spacing-variation/brief.md",
		"experiments/active/foam--hatching-spacing-variation/favorites.json",
		"experiments/active/foam--hatching-spacing-variation/result.md",
		"experiments/active/foam--hatching-spacing-variation/state.json",
	} {
		if !slices.Contains(strings.Fields(tracked), path) {
			t.Errorf("initial commit does not contain %s; files: %s", path, tracked)
		}
	}
	if got := repo.gitOutputAt(t, wantWorktree, "log", "-1", "--format=%s"); got != "experiment: create "+id {
		t.Fatalf("commit message = %q", got)
	}
	brief := readTextFile(t, wantBrief)
	for _, fragment := range []string{
		"Fixed seeds: `[1 2 3 5 8 13]`",
		"--profile preview",
		"--seeds 1,2,3,5,8,13",
		"--out out/experiments/foam/hatching-spacing-variation/baseline",
		"--out out/experiments/foam/hatching-spacing-variation/candidate",
	} {
		if !strings.Contains(brief, fragment) {
			t.Errorf("brief does not contain %q", fragment)
		}
	}
	wantInstruction := "Work on experiment foam/hatching-spacing-variation only.\n" +
		"Worktree: " + wantWorktree + "\n" +
		"Branch: exp/foam/hatching-spacing-variation\n" +
		"Brief: " + wantBrief + "\n" +
		"Operate only inside this worktree. Do not switch branches. Do not create or remove worktrees. Do not merge, rebase, or modify master. Do not work outside the assigned scope. Do not modify another experiment's files."
	if created.WorkerInstruction != wantInstruction {
		t.Fatalf("worker instruction:\n%s\nwant:\n%s", created.WorkerInstruction, wantInstruction)
	}
}

func TestCreateLifecycleOnlyDefaultsToLifecycleStage(t *testing.T) {
	repo := newTestRepo(t)
	created, err := testManager(t, repo).Create(context.Background(), CreateOptions{
		Piece:         "foam",
		Name:          "lifecycle-pilot",
		LifecycleOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.State.Stage != "lifecycle" {
		t.Fatalf("stage = %q, want lifecycle", created.State.Stage)
	}
}

func TestCreateRejectsInvalidOrConflictingResources(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, testRepo, *Manager)
		opts    CreateOptions
		want    string
	}{
		{
			name: "dirty coordinator",
			prepare: func(t *testing.T, repo testRepo, _ *Manager) {
				if err := os.WriteFile(filepath.Join(repo.root, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			opts: CreateOptions{Piece: "foam", Name: "dirty"},
			want: "coordinator worktree is not clean",
		},
		{
			name: "invalid stage",
			opts: CreateOptions{Piece: "foam", Name: "bad-stage", Stage: "composition"},
			want: "invalid experiment stage",
		},
		{
			name: "duplicate branch",
			prepare: func(t *testing.T, repo testRepo, _ *Manager) {
				repo.git(t, "branch", "exp/foam/duplicate-branch")
			},
			opts: CreateOptions{Piece: "foam", Name: "duplicate-branch"},
			want: "branch already exists",
		},
		{
			name: "duplicate integration branch",
			prepare: func(t *testing.T, repo testRepo, _ *Manager) {
				repo.git(t, "branch", "integrate/foam/duplicate-integration")
			},
			opts: CreateOptions{Piece: "foam", Name: "duplicate-integration"},
			want: "branch already exists",
		},
		{
			name: "duplicate path",
			prepare: func(t *testing.T, repo testRepo, _ *Manager) {
				id, _ := ParseID("foam/duplicate-path")
				if err := os.MkdirAll(id.WorktreePath(repo.root), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			opts: CreateOptions{Piece: "foam", Name: "duplicate-path"},
			want: "worktree path already exists",
		},
		{
			name: "duplicate record",
			prepare: func(t *testing.T, repo testRepo, _ *Manager) {
				path := filepath.Join(repo.root, "experiments", "active", "foam--duplicate-record")
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			opts: CreateOptions{Piece: "foam", Name: "duplicate-record"},
			want: "record already exists",
		},
		{
			name: "archived record",
			prepare: func(t *testing.T, repo testRepo, _ *Manager) {
				path := filepath.Join(repo.root, "experiments", "archive", "foam--archived-record")
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			opts: CreateOptions{Piece: "foam", Name: "archived-record"},
			want: "record already exists",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newTestRepo(t)
			manager := testManager(t, repo)
			if test.prepare != nil {
				test.prepare(t, repo, manager)
			}
			_, err := manager.Create(context.Background(), test.opts)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCreateRejectsThirdActiveWriter(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	for _, name := range []string{"first", "second"} {
		created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: name, MaxWriters: 2})
		if err != nil {
			t.Fatal(err)
		}
		created.State.Status = StatusRunning
		created.State.UpdatedAt = created.State.UpdatedAt.Add(time.Minute)
		if err := writeJSONAtomic(filepath.Join(filepath.Dir(created.BriefPath), "state.json"), created.State); err != nil {
			t.Fatal(err)
		}
		if err := commitRecord(context.Background(), created.WorktreePath, filepath.Join("experiments", "active", "foam--"+name), "experiment: start foam/"+name, repo.gitEnv); err != nil {
			t.Fatal(err)
		}
	}

	_, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "third", MaxWriters: 2})
	if err == nil || !strings.Contains(err.Error(), "maximum active writers reached (2)") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateChildStartsAtValidParentTip(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	parent, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	parentTip := repo.gitOutputAt(t, parent.WorktreePath, "rev-parse", "HEAD")

	child, err := manager.Create(context.Background(), CreateOptions{
		Piece:          "foam",
		Name:           "child",
		BaseExperiment: "foam/parent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if child.State.BaseExperiment != "foam/parent" || child.State.BaseBranch != "exp/foam/parent" {
		t.Fatalf("child base = %q / %q", child.State.BaseExperiment, child.State.BaseBranch)
	}
	if child.State.BaseCommit != parentTip {
		t.Fatalf("base commit = %q, want parent tip %q", child.State.BaseCommit, parentTip)
	}
	if got := repo.gitOutput(t, "rev-parse", "exp/foam/child^"); got != parentTip {
		t.Fatalf("child parent = %q, want %q", got, parentTip)
	}
}

func TestCreateChildRejectsParentWithMismatchedState(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	parent, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	parent.State.Branch = "exp/foam/not-parent"
	if err := writeJSONAtomic(filepath.Join(filepath.Dir(parent.BriefPath), "state.json"), parent.State); err != nil {
		t.Fatal(err)
	}
	if err := commitRecord(context.Background(), parent.WorktreePath, filepath.Join("experiments", "active", "foam--parent"), "experiment: corrupt fixture", repo.gitEnv); err != nil {
		t.Fatal(err)
	}

	_, err = manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "child", BaseExperiment: "foam/parent"})
	if err == nil || !strings.Contains(err.Error(), "inconsistent identity") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateCountsCommittedWriterStateWhenWorktreeRecordIsMissing(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	for _, name := range []string{"first", "second"} {
		created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: name, MaxWriters: 2})
		if err != nil {
			t.Fatal(err)
		}
		created.State.Status = StatusRunning
		statePath := filepath.Join(filepath.Dir(created.BriefPath), "state.json")
		if err := writeJSONAtomic(statePath, created.State); err != nil {
			t.Fatal(err)
		}
		if err := commitRecord(context.Background(), created.WorktreePath, filepath.Join("experiments", "active", "foam--"+name), "experiment: start foam/"+name, repo.gitEnv); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(statePath); err != nil {
			t.Fatal(err)
		}
	}

	_, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "third", MaxWriters: 2})
	if err == nil || !strings.Contains(err.Error(), "maximum active writers reached (2)") {
		t.Fatalf("error = %v", err)
	}
}

func TestReadStateRejectsCorruptIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"id":"","kind":"experiment"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(path); err == nil || !strings.Contains(err.Error(), "missing experiment ID") {
		t.Fatalf("error = %v", err)
	}
}

func testManager(t *testing.T, repo testRepo) *Manager {
	t.Helper()
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	manager.gitEnv = repo.gitEnv
	return manager
}

func (r testRepo) gitOutput(t *testing.T, args ...string) string {
	t.Helper()
	return r.gitOutputAt(t, r.root, args...)
}

func (r testRepo) gitOutputAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = r.gitEnv
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func readJSONFile(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatal(err)
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func statesEqual(a, b State) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
