package experiment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type gitRunner struct {
	dir                  string
	env                  []string
	indexFile            string
	disableOptionalLocks bool
}

func (g gitRunner) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	env := g.env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = sanitizedGitEnvironment(env)
	if g.indexFile != "" {
		cmd.Env = append(cmd.Env, "GIT_INDEX_FILE="+g.indexFile)
	}
	if g.disableOptionalLocks {
		cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		cause := err
		if ctxErr := ctx.Err(); ctxErr != nil {
			cause = errors.Join(ctxErr, err)
		}
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), detail, cause)
	}
	return stdout.String(), nil
}

func sanitizedGitEnvironment(env []string) []string {
	return filterEnvironment(env, func(key string) bool {
		switch key {
		case "GIT_DIR",
			"GIT_WORK_TREE",
			"GIT_COMMON_DIR",
			"GIT_INDEX_FILE",
			"GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES",
			"GIT_PREFIX",
			"GIT_CEILING_DIRECTORIES",
			"GIT_DISCOVERY_ACROSS_FILESYSTEM",
			"GIT_CONFIG_COUNT",
			"GIT_CONFIG_PARAMETERS",
			"GIT_TEMPLATE_DIR",
			"GIT_OPTIONAL_LOCKS":
			return true
		default:
			return strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_")
		}
	})
}

func filterEnvironment(env []string, blocked func(string) bool) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !blocked(key) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// WorktreeInfo describes one record from git worktree list --porcelain.
type WorktreeInfo struct {
	Path           string
	HEAD           string
	Branch         string
	Bare           bool
	Prunable       bool
	PrunableReason string
}

func parseWorktreeList(output string) ([]WorktreeInfo, error) {
	var records []WorktreeInfo
	var current *WorktreeInfo
	flush := func() {
		if current != nil {
			records = append(records, *current)
			current = nil
		}
	}

	for _, field := range strings.Split(output, "\x00") {
		if field == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(field, " ")
		if key == "worktree" {
			flush()
			if value == "" {
				return nil, fmt.Errorf("parse git worktree list: empty worktree path")
			}
			current = &WorktreeInfo{Path: value}
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("parse git worktree list: %q appears before a worktree", field)
		}
		switch key {
		case "HEAD":
			current.HEAD = value
		case "branch":
			current.Branch = value
		case "bare":
			current.Bare = true
		case "prunable":
			current.Prunable = true
			current.PrunableReason = value
		case "detached", "locked":
			// These records carry no data needed by the lifecycle manager.
		}
	}
	flush()
	return records, nil
}
