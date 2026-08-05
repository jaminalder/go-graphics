package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func readState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read experiment state %q: %w", path, err)
	}
	return decodeState(path, data)
}

func decodeState(source string, data []byte) (State, error) {
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode experiment state %q: %w", source, err)
	}
	if state.SchemaVersion != 1 {
		return State{}, fmt.Errorf("invalid experiment state %q: unsupported schema version %d", source, state.SchemaVersion)
	}
	if state.ID == "" {
		return State{}, fmt.Errorf("invalid experiment state %q: missing experiment ID", source)
	}
	if _, err := ParseID(state.ID); err != nil {
		return State{}, fmt.Errorf("invalid experiment state %q: %w", source, err)
	}
	if state.Kind != KindExperiment && state.Kind != KindIntegration {
		return State{}, fmt.Errorf("invalid experiment state %q: unknown kind %q", source, state.Kind)
	}
	if state.Branch == "" || state.BaseBranch == "" || state.BaseCommit == "" {
		return State{}, fmt.Errorf("invalid experiment state %q: incomplete Git identity", source)
	}
	if _, err := ParseStatus(string(state.Status)); err != nil {
		return State{}, fmt.Errorf("invalid experiment state %q: %w", source, err)
	}
	return state, nil
}

// writeJSONAtomic requires same-directory rename to atomically replace an
// existing destination. Filesystems or platforms without that guarantee
// return an error and leave the destination untouched.
func writeJSONAtomic(path string, value any) error {
	return writeJSONAtomicWithRename(path, value, os.Rename)
}

func writeJSONAtomicWithRename(path string, value any, rename func(string, string) error) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON for %q: %w", path, err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temporary JSON beside %q: %w", path, err)
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o644); err != nil {
		return fmt.Errorf("set temporary JSON mode %q: %w", tempPath, err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write temporary JSON %q: %w", tempPath, err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary JSON %q: %w", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary JSON %q: %w", tempPath, err)
	}
	closed = true
	if err := rename(tempPath, path); err != nil {
		return fmt.Errorf("same-directory atomic replacement is unsupported or failed for JSON %q: %w", path, err)
	}
	return nil
}

func renderTemplate(source, destination string, data any) error {
	tmpl, err := template.ParseFiles(source)
	if err != nil {
		return fmt.Errorf("parse template %q: %w", source, err)
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+"-*")
	if err != nil {
		return fmt.Errorf("create rendered template %q: %w", destination, err)
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o644); err != nil {
		return fmt.Errorf("set rendered template mode %q: %w", tempPath, err)
	}
	if err := tmpl.ExecuteTemplate(temp, filepath.Base(source), data); err != nil {
		return fmt.Errorf("render template %q: %w", source, err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync rendered template %q: %w", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close rendered template %q: %w", tempPath, err)
	}
	closed = true
	if err := os.Rename(tempPath, destination); err != nil {
		return fmt.Errorf("replace rendered template %q: %w", destination, err)
	}
	return nil
}

func commitRecord(
	ctx context.Context,
	worktree, recordDir, message, expectedBranch, expectedParent string,
	env []string,
	checkpoint func(string) error,
) (string, error) {
	runner := gitRunner{dir: worktree, env: env}
	expectedRef := "refs/heads/" + expectedBranch
	if err := requireSymbolicHEAD(ctx, runner, expectedRef); err != nil {
		return "", err
	}
	parent, err := runner.run(ctx, "rev-parse", "--verify", expectedRef+"^{commit}")
	if err != nil {
		return "", err
	}
	if parent = strings.TrimSpace(parent); parent != expectedParent {
		return "", fmt.Errorf("record branch %q changed before commit: got %s, want %s", expectedBranch, parent, expectedParent)
	}
	if _, err := runner.run(ctx, "add", "--", filepath.ToSlash(recordDir)); err != nil {
		return "", err
	}
	if checkpoint != nil {
		if err := checkpoint("during-record-commit"); err != nil {
			return "", err
		}
	}
	tree, err := runner.run(ctx, "write-tree")
	if err != nil {
		return "", err
	}
	tree = strings.TrimSpace(tree)
	commit, err := runner.run(ctx, "commit-tree", tree, "-p", expectedParent, "-m", message)
	if err != nil {
		return "", err
	}
	commit = strings.TrimSpace(commit)
	if _, err := runner.run(ctx, "update-ref", expectedRef, commit, expectedParent); err != nil {
		return "", fmt.Errorf("compare-and-swap record branch %q: %w", expectedBranch, err)
	}
	if err := requireSymbolicHEAD(ctx, runner, expectedRef); err != nil {
		return commit, err
	}
	return commit, nil
}

func requireSymbolicHEAD(ctx context.Context, runner gitRunner, expectedRef string) error {
	ref, err := runner.run(ctx, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return fmt.Errorf("record worktree HEAD changed: expected symbolic ref %q: %w", expectedRef, err)
	}
	if ref = strings.TrimSpace(ref); ref != expectedRef {
		return fmt.Errorf("record worktree HEAD changed: got %q, want %q", ref, expectedRef)
	}
	return nil
}
