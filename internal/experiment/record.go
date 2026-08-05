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

// writeJSONAtomic keeps its temporary file beside the destination. Replacement
// uses the platform's replace primitive and depends on the destination
// filesystem's atomic rename guarantees. Replacement errors leave the existing
// destination untouched and are returned.
func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON for %q: %w", path, err)
	}
	return writeBytesAtomicWithRename(path, append(data, '\n'), replaceFile)
}

func writeJSONAtomicWithRename(path string, value any, rename func(string, string) error) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON for %q: %w", path, err)
	}
	return writeBytesAtomicWithRename(path, append(data, '\n'), rename)
}

func writeBytesAtomic(path string, data []byte) error {
	return writeBytesAtomicWithRename(path, data, replaceFile)
}

func writeBytesAtomicWithRename(path string, data []byte, rename func(string, string) error) error {
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
		return fmt.Errorf("same-directory replacement failed for JSON %q; existing destination was preserved: %w", path, err)
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
	if err := replaceFile(tempPath, destination); err != nil {
		return fmt.Errorf("same-directory replacement failed for rendered template %q; existing destination was preserved: %w", destination, err)
	}
	return nil
}

func commitRecord(
	ctx context.Context,
	worktree, recordDir string, paths []string, message, expectedBranch, expectedParent string,
	env []string,
	checkpoint func(string) error,
) (string, error) {
	recordDir = filepath.Clean(recordDir)
	if filepath.IsAbs(recordDir) || recordDir == "." || recordDir == ".." || strings.HasPrefix(recordDir, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid record directory %q", recordDir)
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("record commit requires at least one path")
	}
	cleanPaths := make([]string, len(paths))
	for i, path := range paths {
		path = filepath.Clean(path)
		relative, err := filepath.Rel(recordDir, path)
		if err != nil || filepath.IsAbs(path) || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("record commit path %q is not a file below %q", path, recordDir)
		}
		cleanPaths[i] = filepath.ToSlash(path)
	}
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
	index, err := os.CreateTemp("", "experiment-index-*")
	if err != nil {
		return "", fmt.Errorf("create isolated record index: %w", err)
	}
	indexPath := index.Name()
	if err := index.Close(); err != nil {
		_ = os.Remove(indexPath)
		return "", fmt.Errorf("close isolated record index placeholder: %w", err)
	}
	if err := os.Remove(indexPath); err != nil {
		return "", fmt.Errorf("prepare absent isolated record index %q: %w", indexPath, err)
	}
	defer func() { _ = os.Remove(indexPath) }()
	indexRunner := gitRunner{dir: worktree, env: env, indexFile: indexPath}
	if _, err := indexRunner.run(ctx, "read-tree", expectedParent); err != nil {
		return "", err
	}
	addArgs := append([]string{"add", "--"}, cleanPaths...)
	if _, err := indexRunner.run(ctx, addArgs...); err != nil {
		return "", err
	}
	if checkpoint != nil {
		if err := checkpoint("during-record-commit"); err != nil {
			return "", err
		}
	}
	tree, err := indexRunner.run(ctx, "write-tree")
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
	if err := updateCommittedIndexPaths(ctx, runner, commit, cleanPaths); err != nil {
		return commit, err
	}
	return commit, nil
}

func updateCommittedIndexPaths(ctx context.Context, runner gitRunner, commit string, paths []string) error {
	args := append([]string{"ls-tree", "-z", commit, "--"}, paths...)
	output, err := runner.run(ctx, args...)
	if err != nil {
		return err
	}
	entries := make(map[string][2]string, len(paths))
	for _, record := range strings.Split(output, "\x00") {
		if record == "" {
			continue
		}
		metadata, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			return fmt.Errorf("parse committed record index entry %q", record)
		}
		entries[path] = [2]string{fields[0], fields[2]}
	}
	for _, path := range paths {
		entry, ok := entries[path]
		if !ok {
			return fmt.Errorf("committed record path is missing from tree: %s", path)
		}
		if _, err := runner.run(ctx, "update-index", "--add", "--cacheinfo", entry[0], entry[1], path); err != nil {
			return fmt.Errorf("update real index for committed record path %q: %w", path, err)
		}
	}
	return nil
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
