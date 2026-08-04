package experiment

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ID identifies one experiment by its piece and name.
type ID struct {
	piece string
	name  string
}

// ParseID parses an experiment ID with exactly two lowercase kebab-case components.
func ParseID(value string) (ID, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 || !validIDComponent(parts[0]) || !validIDComponent(parts[1]) {
		return ID{}, fmt.Errorf("invalid experiment ID %q: want <piece>/<name> in lowercase kebab-case", value)
	}
	return ID{piece: parts[0], name: parts[1]}, nil
}

func validIDComponent(value string) bool {
	if value == "" || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	previousHyphen := false
	for i := range len(value) {
		char := value[i]
		if char == '-' {
			if previousHyphen {
				return false
			}
			previousHyphen = true
			continue
		}
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
		previousHyphen = false
	}
	return true
}

// String returns the canonical piece/name identity.
func (id ID) String() string { return id.piece + "/" + id.name }

// Piece returns the piece component.
func (id ID) Piece() string { return id.piece }

// Name returns the experiment name component.
func (id ID) Name() string { return id.name }

// Flat returns the identity in filename-safe form.
func (id ID) Flat() string { return id.piece + "--" + id.name }

// ExperimentBranch returns the ordinary experiment branch name.
func (id ID) ExperimentBranch() string { return "exp/" + id.String() }

// IntegrationBranch returns the semantic integration branch name.
func (id ID) IntegrationBranch() string { return "integrate/" + id.String() }

// RecordDir returns the active record directory relative to the repository root.
func (id ID) RecordDir() string { return "experiments/active/" + id.Flat() }

// ArchiveDir returns the archived record directory relative to the repository root.
func (id ID) ArchiveDir() string { return "experiments/archive/" + id.Flat() }

// OutputDir returns the generated output directory relative to the worktree root.
func (id ID) OutputDir() string { return "out/experiments/" + id.String() }

// WorktreeName returns the directory name reserved for the experiment worktree.
func (id ID) WorktreeName() string { return id.Flat() }

// WorktreePath returns the absolute sibling worktree path for a coordinator root.
func (id ID) WorktreePath(coordinatorRoot string) string {
	return filepath.Join(filepath.Dir(coordinatorRoot), filepath.Base(coordinatorRoot)+"-worktrees", id.WorktreeName())
}
