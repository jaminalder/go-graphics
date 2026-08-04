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

func newTestRepo(t *testing.T) testRepo {
	return newTestRepoWithEnv(t, os.Environ())
}

func newTestRepoWithEnv(t *testing.T, baseEnv []string) testRepo {
	t.Helper()

	root := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), "global-gitconfig")
	if err := os.WriteFile(globalConfig, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	gitEnv := filterEnvironment(sanitizedGitEnvironment(baseEnv), func(key string) bool {
		switch key {
		case "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM":
			return true
		default:
			return false
		}
	})
	repo := testRepo{
		root: root,
		gitEnv: append(gitEnv,
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_CONFIG_GLOBAL="+globalConfig,
		),
	}
	repo.git(t, "init", "-b", "master")
	repo.git(t, "config", "--local", "user.name", "Experiment Test")
	repo.git(t, "config", "--local", "user.email", "experiment@example.invalid")

	files := map[string]string{
		".gitignore": "/out/experiments/\n",
		"README.md":  "test repository\n",
		"experiments/templates/brief.md": "# Experiment {{.ID}}\n\n" +
			"- Fixed seeds: `{{.Seeds}}`\n" +
			"- Source experiments: {{.SourceExperiments}}\n" +
			"- Output path: `{{.OutputPath}}`\n\n" +
			"```sh\n{{.BaselineCommand}}\n{{.CandidateCommand}}\n```\n",
		"experiments/templates/result.md": "# Experiment result: {{.ID}}\n\n" +
			"- Fixed seeds: `{{.Seeds}}`\n" +
			"- Output path: `{{.OutputPath}}`\n",
		"experiments/active/.gitkeep":  "",
		"experiments/archive/.gitkeep": "",
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

func TestNewTestRepoIgnoresAmbientGitRouting(t *testing.T) {
	t.Parallel()

	cleanEnv := sanitizedGitEnvironment(os.Environ())

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

	templateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(templateDir, "injected-template"), []byte("injected\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	poisoned := append([]string(nil), cleanEnv...)
	poisoned = append(poisoned,
		"GIT_DIR="+filepath.Join(ambientRoot, ".git"),
		"GIT_WORK_TREE="+ambientRoot,
		"GIT_COMMON_DIR="+filepath.Join(ambientRoot, ".git"),
		"GIT_INDEX_FILE="+filepath.Join(ambientRoot, ".git", "index"),
		"GIT_OBJECT_DIRECTORY="+filepath.Join(ambientRoot, ".git", "objects"),
		"GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(alternate, "objects"),
		"GIT_PREFIX=poisoned/",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=experiment.injected",
		"GIT_CONFIG_VALUE_0=true",
		"GIT_TEMPLATE_DIR="+templateDir,
	)

	repo := newTestRepoWithEnv(t, poisoned)
	got, err := (gitRunner{dir: repo.root, env: repo.gitEnv}).run(context.Background(), "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSuffix(got, "\n")
	want, err := filepath.EvalSymlinks(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("fixture Git root = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(repo.root, ".git", "injected-template")); !os.IsNotExist(err) {
		t.Fatalf("injected Git template reached fixture: %v", err)
	}
	cmd = exec.Command("git", "config", "--get", "experiment.injected")
	cmd.Dir = repo.root
	cmd.Env = repo.gitEnv
	if err := cmd.Run(); err == nil {
		t.Fatal("injected command-line config reached fixture")
	}
}
