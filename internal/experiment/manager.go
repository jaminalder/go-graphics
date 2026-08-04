package experiment

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Manager holds canonical paths shared by experiment lifecycle operations.
type Manager struct {
	CoordinatorRoot  string
	CurrentRoot      string
	CommonDir        string
	TemplatesRoot    string
	gitEnv           []string
	now              func() time.Time
	releaseLock      func(*Lock) error
	createCheckpoint func(string) error
}

// NewManager discovers repository paths without changing the process working directory.
func NewManager(cwd string) (*Manager, error) {
	start, err := canonicalPath(cwd, "")
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	runner := gitRunner{dir: start}
	ctx := context.Background()

	current, err := runner.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	current = strings.TrimSuffix(current, "\n")
	current, err = canonicalPath(current, start)
	if err != nil {
		return nil, fmt.Errorf("resolve current worktree: %w", err)
	}
	common, err := runner.run(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	common = strings.TrimSuffix(common, "\n")
	common, err = canonicalPath(common, start)
	if err != nil {
		return nil, fmt.Errorf("resolve Git common directory: %w", err)
	}
	porcelain, err := runner.run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	worktrees, err := parseWorktreeList(porcelain)
	if err != nil {
		return nil, err
	}
	coordinator := ""
	for _, worktree := range worktrees {
		if worktree.Bare {
			continue
		}
		coordinator, err = canonicalPath(worktree.Path, start)
		if err != nil {
			return nil, fmt.Errorf("resolve coordinator worktree: %w", err)
		}
		break
	}
	if coordinator == "" {
		return nil, fmt.Errorf("git worktree list contains no non-bare coordinator worktree")
	}

	return &Manager{
		CoordinatorRoot: coordinator,
		CurrentRoot:     current,
		CommonDir:       common,
		TemplatesRoot:   filepath.Join(coordinator, "experiments", "templates"),
		now:             time.Now,
	}, nil
}

// RequireCoordinator rejects a coordinator-only operation from a linked worktree.
func (m *Manager) RequireCoordinator() error {
	if m.CurrentRoot != m.CoordinatorRoot {
		return fmt.Errorf("operation requires coordinator worktree %q; current worktree is %q", m.CoordinatorRoot, m.CurrentRoot)
	}
	return nil
}

func canonicalPath(path, relativeTo string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(relativeTo, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
