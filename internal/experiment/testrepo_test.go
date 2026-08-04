package experiment

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type testRepo struct {
	root   string
	gitEnv []string
}

var gitRoutingEnvironment = map[string]struct{}{
	"GIT_DIR":                          {},
	"GIT_WORK_TREE":                    {},
	"GIT_COMMON_DIR":                   {},
	"GIT_INDEX_FILE":                   {},
	"GIT_OBJECT_DIRECTORY":             {},
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
}

func newTestRepo(t *testing.T) testRepo {
	t.Helper()

	root := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), "global-gitconfig")
	if err := os.WriteFile(globalConfig, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name := range gitRoutingEnvironment {
		unsetenvForTest(t, name)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	repo := testRepo{
		root: root,
		gitEnv: append(withoutGitRouting(os.Environ()),
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_CONFIG_GLOBAL="+globalConfig,
		),
	}
	repo.git(t, "init", "-b", "master")
	repo.git(t, "config", "--local", "user.name", "Experiment Test")
	repo.git(t, "config", "--local", "user.email", "experiment@example.invalid")

	files := map[string]string{
		".gitignore":                      "/out/experiments/\n",
		"README.md":                       "test repository\n",
		"experiments/templates/brief.md":  "# Brief\n",
		"experiments/templates/result.md": "# Result\n",
		"experiments/active/.gitkeep":     "",
		"experiments/archive/.gitkeep":    "",
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "test: initial commit")

	return repo
}

func (r testRepo) git(t *testing.T, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	cmd.Env = r.gitEnv
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func withoutGitRouting(env []string) []string {
	clean := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if _, routed := gitRoutingEnvironment[key]; !routed {
			clean = append(clean, entry)
		}
	}
	return clean
}

func unsetenvForTest(t *testing.T, name string) {
	t.Helper()

	value, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(name, value)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}

func TestNewTestRepoIgnoresAmbientGitRouting(t *testing.T) {
	cleanEnv := withoutGitRouting(os.Environ())

	ambientRoot := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "master")
	cmd.Dir = ambientRoot
	cmd.Env = cleanEnv
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("initialize ambient repository: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(ambientRoot, "ambient.txt"), []byte("ambient\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	alternate := t.TempDir()
	cmd = exec.Command("git", "init", "--bare")
	cmd.Dir = alternate
	cmd.Env = cleanEnv
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("initialize alternate object store: %v\n%s", err, output)
	}

	t.Setenv("GIT_DIR", filepath.Join(ambientRoot, ".git"))
	t.Setenv("GIT_WORK_TREE", ambientRoot)
	t.Setenv("GIT_COMMON_DIR", filepath.Join(ambientRoot, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(ambientRoot, ".git", "index"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(ambientRoot, ".git", "objects"))
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", filepath.Join(alternate, "objects"))

	repo := newTestRepo(t)
	got, err := (gitRunner{dir: repo.root, env: repo.gitEnv}).run(context.Background(), "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("fixture Git root = %q, want %q", got, want)
	}
}
