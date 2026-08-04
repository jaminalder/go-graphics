package experiment

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type testRepo struct {
	root   string
	gitEnv []string
}

func newTestRepo(t *testing.T) testRepo {
	t.Helper()

	root := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), "global-gitconfig")
	if err := os.WriteFile(globalConfig, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	repo := testRepo{
		root: root,
		gitEnv: append(os.Environ(),
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
