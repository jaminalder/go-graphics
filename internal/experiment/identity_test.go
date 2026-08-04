package experiment

import (
	"path/filepath"
	"testing"
)

func TestParseIDAcceptsExactlyTwoLowercaseKebabComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		piece string
		name  string
	}{
		{value: "foam/hatching-by-depth", piece: "foam", name: "hatching-by-depth"},
		{value: "qql/2d-rings-7", piece: "qql", name: "2d-rings-7"},
		{value: "1/a", piece: "1", name: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			id, err := ParseID(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if got := id.String(); got != tt.value {
				t.Fatalf("String() = %q", got)
			}
			if got := id.Piece(); got != tt.piece {
				t.Fatalf("Piece() = %q", got)
			}
			if got := id.Name(); got != tt.name {
				t.Fatalf("Name() = %q", got)
			}
		})
	}
}

func TestParseIDRejectsUnsafeOrNoncanonicalIdentities(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"",
		"foam",
		"foam/",
		"/hatching",
		"foam/hatching/extra",
		"Foam/hatching",
		"foam/Hatching",
		"foam/hatching_by_depth",
		"foam/hatching.by.depth",
		"foam/hatching by depth",
		"foam/hatching\tby-depth",
		"foam/-hatching",
		"foam/hatching-",
		"foam/hatching--depth",
		"foo--bar/hatching",
		"../hatching",
		"foam/..",
		"./hatching",
	}

	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseID(input); err == nil {
				t.Fatalf("ParseID(%q) succeeded", input)
			}
		})
	}
}

func TestIDDerivesCanonicalBranchesAndPaths(t *testing.T) {
	t.Parallel()

	id, err := ParseID("foam/hatching-by-depth")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "flat ID", got: id.Flat(), want: "foam--hatching-by-depth"},
		{name: "experiment branch", got: id.ExperimentBranch(), want: "exp/foam/hatching-by-depth"},
		{name: "integration branch", got: id.IntegrationBranch(), want: "integrate/foam/hatching-by-depth"},
		{name: "active record", got: id.RecordDir(), want: "experiments/active/foam--hatching-by-depth"},
		{name: "archive record", got: id.ArchiveDir(), want: "experiments/archive/foam--hatching-by-depth"},
		{name: "output", got: id.OutputDir(), want: "out/experiments/foam/hatching-by-depth"},
		{name: "worktree name", got: id.WorktreeName(), want: "foam--hatching-by-depth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Fatalf("path = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestIDDerivesSiblingWorktreeFromCoordinatorRoot(t *testing.T) {
	t.Parallel()

	id, err := ParseID("foam/hatching-by-depth")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(string(filepath.Separator), "tmp", "go-graphics")
	want := filepath.Join(string(filepath.Separator), "tmp", "go-graphics-worktrees", "foam--hatching-by-depth")
	if got := id.WorktreePath(root); got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}
