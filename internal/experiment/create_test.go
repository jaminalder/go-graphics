package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestCreateValidatesPlainLocalBaseBranch(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "experiment namespace", base: "exp/foam/parent", want: "cannot use experiment namespace"},
		{name: "integration namespace", base: "integrate/foam/parent", want: "cannot use experiment namespace"},
		{name: "revision expression", base: "master~1", want: "invalid base branch"},
		{name: "commit peel", base: "master^{commit}", want: "invalid base branch"},
		{name: "previous checkout syntax", base: "@{-1}", want: "invalid base branch"},
		{name: "fully qualified ref", base: "refs/heads/master", want: "invalid base branch"},
		{name: "missing local branch", base: "missing", want: "base branch does not exist"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newTestRepo(t)
			_, err := testManager(t, repo).Create(context.Background(), CreateOptions{
				Piece:      "foam",
				Name:       "base-validation",
				BaseBranch: test.base,
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCreateUsesDeclaredOrdinaryBaseBranch(t *testing.T) {
	repo := newTestRepo(t)
	repo.git(t, "branch", "feature/source")
	baseTip := repo.gitOutput(t, "rev-parse", "refs/heads/feature/source")

	created, err := testManager(t, repo).Create(context.Background(), CreateOptions{
		Piece:      "foam",
		Name:       "ordinary-base",
		BaseBranch: "feature/source",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.State.BaseBranch != "feature/source" || created.State.BaseExperiment != "" || created.State.BaseCommit != baseTip {
		t.Fatalf("base state = %#v", created.State)
	}
}

func TestCreateRejectsConflictingChildBase(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "parent"}); err != nil {
		t.Fatal(err)
	}

	_, err := manager.Create(context.Background(), CreateOptions{
		Piece:          "foam",
		Name:           "child",
		BaseBranch:     "feature/source",
		BaseExperiment: "foam/parent",
	})
	if err == nil || !strings.Contains(err.Error(), "base branch cannot be combined with base experiment") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateAcceptsDefaultBaseWithChildDeclaration(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	if _, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "parent"}); err != nil {
		t.Fatal(err)
	}

	created, err := manager.Create(context.Background(), CreateOptions{
		Piece:          "foam",
		Name:           "child",
		BaseBranch:     "master",
		BaseExperiment: "foam/parent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.State.BaseExperiment != "foam/parent" || created.State.BaseBranch != "exp/foam/parent" {
		t.Fatalf("child base = %q / %q", created.State.BaseExperiment, created.State.BaseBranch)
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

func TestWriteJSONAtomicUsesIndentedNewlineTerminatedMode0644File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record.json")
	if err := writeJSONAtomic(path, map[string]any{"name": "record", "values": []int{1, 2}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"name\": \"record\",\n  \"values\": [\n    1,\n    2\n  ]\n}\n"
	if string(data) != want {
		t.Fatalf("JSON = %q, want %q", data, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode = %o, want 644", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("directory entries = %v, want only final file", entries)
	}
}

func TestShellQuoteProducesSafePOSIXSingleWord(t *testing.T) {
	tests := map[string]string{
		"":                  "''",
		"plain":             "'plain'",
		"space and $dollar": "'space and $dollar'",
		"quote'newline\n":   "'quote'\"'\"'newline\n'",
	}
	for input, want := range tests {
		if got := shellQuote(input); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCreateFailureShellQuotesEveryDynamicRecoveryArgument(t *testing.T) {
	id, err := ParseID("foam/recovery")
	if err != nil {
		t.Fatal(err)
	}
	resources := createResources{
		branch:          "branch with 'quote\nand newline",
		worktreePath:    "/tmp/path with 'quote\nand newline",
		branchCreated:   true,
		worktreeCreated: true,
	}
	got := createFailure(id, resources, errors.New("injected failure")).Error()
	for _, command := range []string{
		"git branch --list " + shellQuote(resources.branch),
		"git -C " + shellQuote(resources.worktreePath) + " status --short",
		"git worktree remove " + shellQuote(resources.worktreePath),
		"git branch -d " + shellQuote(resources.branch),
	} {
		if !strings.Contains(got, command) {
			t.Errorf("recovery error does not contain safely quoted command %q:\n%s", command, got)
		}
	}
}

func TestCreateWritesTypedEmptyFavoritesAndDocumentsSchema(t *testing.T) {
	repo := newTestRepo(t)
	created, err := testManager(t, repo).Create(context.Background(), CreateOptions{Piece: "foam", Name: "favorites-schema"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(created.BriefPath), "favorites.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]\n" {
		t.Fatalf("favorites JSON = %q, want empty array", data)
	}
	var favorites []Favorite
	if err := json.Unmarshal(data, &favorites); err != nil {
		t.Fatal(err)
	}
	brief := readTextFile(t, created.BriefPath)
	for _, field := range []string{"seed", "label", "image", "notes"} {
		if !strings.Contains(brief, "`"+field+"`") {
			t.Errorf("brief does not document favorites field %q", field)
		}
	}
}

func TestCreateReportsPartialResourcesWhenLockReleaseFails(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	releases := 0
	manager.releaseLock = func(lock *Lock) error {
		releases++
		releaseErr := lock.Release()
		return errors.Join(releaseErr, fmt.Errorf("injected release failure %d", releases))
	}

	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "release-failure"})
	if err == nil {
		t.Fatal("Create succeeded despite release failures")
	}
	if releases != 2 {
		t.Fatalf("release calls = %d, want 2", releases)
	}
	for _, fragment := range []string{
		"injected release failure 1",
		"injected release failure 2",
		"created branch=true 'exp/foam/release-failure'",
		"worktree=true " + shellQuote(created.WorktreePath),
		"inspect and recover without force",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error does not contain %q:\n%s", fragment, err)
		}
	}
	if created.State.ID != "foam/release-failure" || created.WorktreePath == "" {
		t.Fatalf("Created resources were discarded: %#v", created)
	}
}

func TestCreateRevalidatesDuplicateImmediatelyBeforeBranchMutation(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == checkpointBeforeBranch {
			repo.git(t, "branch", "exp/foam/raced-branch")
		}
		return nil
	}

	_, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "raced-branch"})
	if err == nil || !strings.Contains(err.Error(), "branch already exists") {
		t.Fatalf("error = %v", err)
	}
	id, _ := ParseID("foam/raced-branch")
	if _, err := os.Stat(id.WorktreePath(manager.CoordinatorRoot)); !os.IsNotExist(err) {
		t.Fatalf("worktree was created after duplicate race: %v", err)
	}
}

func TestCreateRevalidatesBaseTipImmediatelyBeforeBranchMutation(t *testing.T) {
	repo := newTestRepo(t)
	repo.git(t, "branch", "feature/source")
	manager := testManager(t, repo)
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == checkpointBeforeBranch {
			repo.git(t, "checkout", "feature/source")
			if err := os.WriteFile(filepath.Join(repo.root, "source.txt"), []byte("advanced\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			repo.git(t, "add", "source.txt")
			repo.git(t, "commit", "-m", "test: advance source")
			repo.git(t, "checkout", "master")
		}
		return nil
	}

	_, err := manager.Create(context.Background(), CreateOptions{
		Piece:      "foam",
		Name:       "raced-base",
		BaseBranch: "feature/source",
	})
	if err == nil || !strings.Contains(err.Error(), "base branch changed before experiment branch creation") {
		t.Fatalf("error = %v", err)
	}
	if output := repo.gitOutput(t, "branch", "--list", "exp/foam/raced-base"); output != "" {
		t.Fatalf("experiment branch created after base race: %s", output)
	}
}

func TestCreateRevalidatesPathImmediatelyBeforeWorktreeMutation(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	id, _ := ParseID("foam/raced-path")
	path := id.WorktreePath(manager.CoordinatorRoot)
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == checkpointBeforeWorktree {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}

	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "raced-path"})
	if err == nil || !strings.Contains(err.Error(), "worktree path already exists") {
		t.Fatalf("error = %v", err)
	}
	if created.WorktreePath != path {
		t.Fatalf("worktree inventory path = %q, want %q", created.WorktreePath, path)
	}
	for _, fragment := range []string{"created branch=true 'exp/foam/raced-path'", "git worktree list --porcelain", "git branch -d 'exp/foam/raced-path'"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error does not contain %q:\n%s", fragment, err)
		}
	}
}

func TestCreateRevalidatesCoordinatorBeforeWorktreeMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, testRepo) func(*testing.T, testRepo)
		want   string
	}{
		{
			name: "branch changed",
			mutate: func(t *testing.T, repo testRepo) func(*testing.T, testRepo) {
				repo.git(t, "checkout", "-b", "human-before-worktree")
				return func(t *testing.T, repo testRepo) {
					repo.git(t, "checkout", "master")
					repo.git(t, "branch", "-d", "human-before-worktree")
				}
			},
			want: "current branch is \"human-before-worktree\"",
		},
		{
			name: "head changed",
			mutate: func(t *testing.T, repo testRepo) func(*testing.T, testRepo) {
				original := repo.gitOutput(t, "rev-parse", "HEAD")
				repo.git(t, "commit", "--allow-empty", "-m", "test: race coordinator before worktree")
				advanced := repo.gitOutput(t, "rev-parse", "HEAD")
				return func(t *testing.T, repo testRepo) {
					repo.git(t, "update-ref", "refs/heads/master", original, advanced)
				}
			},
			want: "coordinator HEAD changed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newTestRepo(t)
			manager := testManager(t, repo)
			var restore func(*testing.T, testRepo)
			manager.createCheckpoint = func(checkpoint string) error {
				if checkpoint == checkpointBeforeWorktree {
					restore = test.mutate(t, repo)
				}
				return nil
			}

			created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "coordinator-before-worktree"})
			if restore == nil {
				t.Fatal("before-worktree checkpoint was not reached")
			}
			restore(t, repo)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			if created.WorktreePath == "" || !strings.Contains(err.Error(), "created branch=true") {
				t.Fatalf("missing partial resource inventory: created=%#v error=%v", created, err)
			}
		})
	}
}

func TestCreateReportsBranchCreatedWhenCheckpointFailsBeforeWorktree(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == checkpointBeforeWorktree {
			return errors.New("injected checkpoint failure")
		}
		return nil
	}

	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "partial-branch"})
	if err == nil {
		t.Fatal("Create succeeded despite checkpoint failure")
	}
	id, _ := ParseID("foam/partial-branch")
	wantPath := id.WorktreePath(manager.CoordinatorRoot)
	if created.WorktreePath != wantPath {
		t.Fatalf("created worktree inventory = %q, want %q", created.WorktreePath, wantPath)
	}
	for _, fragment := range []string{
		"injected checkpoint failure",
		"created branch=true 'exp/foam/partial-branch'",
		"worktree=false " + shellQuote(wantPath),
		"git branch -d 'exp/foam/partial-branch'",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error does not contain %q:\n%s", fragment, err)
		}
	}
	if strings.Contains(err.Error(), "git worktree remove") {
		t.Fatalf("recovery suggests removing a worktree that was not created:\n%s", err)
	}
}

func TestCreateRevalidatesAssignedWorktreeBeforeRecordCommit(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == checkpointBeforeCommit {
			id, _ := ParseID("foam/raced-commit")
			repo.gitOutputAt(t, id.WorktreePath(manager.CoordinatorRoot), "checkout", "--detach")
		}
		return nil
	}

	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "raced-commit"})
	if err == nil || !strings.Contains(err.Error(), "assigned worktree branch changed") {
		t.Fatalf("error = %v", err)
	}
	if created.WorktreePath == "" || !strings.Contains(err.Error(), shellQuote(created.WorktreePath)) {
		t.Fatalf("missing partial worktree inventory: created=%#v error=%v", created, err)
	}
}

func TestCreateRevalidatesCoordinatorBeforeRecordCommit(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, testRepo) func(*testing.T, testRepo)
		want   string
	}{
		{
			name: "branch changed",
			mutate: func(t *testing.T, repo testRepo) func(*testing.T, testRepo) {
				repo.git(t, "checkout", "-b", "human-before-record")
				return func(t *testing.T, repo testRepo) {
					repo.git(t, "checkout", "master")
					repo.git(t, "branch", "-d", "human-before-record")
				}
			},
			want: "current branch is \"human-before-record\"",
		},
		{
			name: "head changed",
			mutate: func(t *testing.T, repo testRepo) func(*testing.T, testRepo) {
				original := repo.gitOutput(t, "rev-parse", "HEAD")
				repo.git(t, "commit", "--allow-empty", "-m", "test: race coordinator before record")
				advanced := repo.gitOutput(t, "rev-parse", "HEAD")
				return func(t *testing.T, repo testRepo) {
					repo.git(t, "update-ref", "refs/heads/master", original, advanced)
				}
			},
			want: "coordinator HEAD changed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newTestRepo(t)
			manager := testManager(t, repo)
			var restore func(*testing.T, testRepo)
			manager.createCheckpoint = func(checkpoint string) error {
				if checkpoint == checkpointBeforeCommit {
					restore = test.mutate(t, repo)
				}
				return nil
			}

			created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "coordinator-before-record"})
			if restore == nil {
				t.Fatal("before-record-commit checkpoint was not reached")
			}
			restore(t, repo)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			if created.WorktreePath == "" || !strings.Contains(err.Error(), "worktree=true") {
				t.Fatalf("missing partial resource inventory: created=%#v error=%v", created, err)
			}
		})
	}
}

func TestCreateRevalidatesCoordinatorImmediatelyAfterRecordCommit(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	original := repo.gitOutput(t, "rev-parse", "HEAD")
	advanced := ""
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == checkpointAfterCommit {
			repo.git(t, "commit", "--allow-empty", "-m", "test: race coordinator after record")
			advanced = repo.gitOutput(t, "rev-parse", "HEAD")
		}
		return nil
	}

	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "coordinator-after-record"})
	if advanced == "" {
		t.Fatal("after-record-commit checkpoint was not reached")
	}
	repo.git(t, "update-ref", "refs/heads/master", original, advanced)
	if err == nil || !strings.Contains(err.Error(), "coordinator HEAD changed") {
		t.Fatalf("error = %v", err)
	}
	if created.WorktreePath == "" || !strings.Contains(err.Error(), "worktree=true") {
		t.Fatalf("missing partial resource inventory: created=%#v error=%v", created, err)
	}
}

func TestCreateRevalidatesAssignedBranchTipBeforeRecordCommit(t *testing.T) {
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	manager.createCheckpoint = func(checkpoint string) error {
		if checkpoint == checkpointBeforeCommit {
			id, _ := ParseID("foam/raced-tip")
			repo.gitOutputAt(t, id.WorktreePath(manager.CoordinatorRoot), "commit", "--allow-empty", "-m", "test: advance assigned branch")
		}
		return nil
	}

	created, err := manager.Create(context.Background(), CreateOptions{Piece: "foam", Name: "raced-tip"})
	if err == nil || !strings.Contains(err.Error(), "assigned worktree tip changed") {
		t.Fatalf("error = %v", err)
	}
	if created.WorktreePath == "" || !strings.Contains(err.Error(), shellQuote(created.WorktreePath)) {
		t.Fatalf("missing partial resource inventory: created=%#v error=%v", created, err)
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
