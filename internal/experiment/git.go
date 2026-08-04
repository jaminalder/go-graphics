package experiment

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type gitRunner struct {
	dir string
	env []string
}

func (g gitRunner) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	if g.env != nil {
		cmd.Env = g.env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (g gitRunner) lines(ctx context.Context, args ...string) ([]string, error) {
	output, err := g.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}

// WorktreeInfo describes one record from git worktree list --porcelain.
type WorktreeInfo struct {
	Path     string
	HEAD     string
	Branch   string
	Bare     bool
	Prunable bool
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

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		if key == "worktree" {
			flush()
			if value == "" {
				return nil, fmt.Errorf("parse git worktree list: empty worktree path")
			}
			current = &WorktreeInfo{Path: value}
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("parse git worktree list: %q appears before a worktree", line)
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
		case "detached", "locked":
			// These records carry no data needed by the lifecycle manager.
		}
	}
	flush()
	return records, nil
}
