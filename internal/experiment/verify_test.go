package experiment

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestVerifyRefusesDirtyWorktreeBeforeRunningCommand(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, false)
	dirtyPath := filepath.Join(created.WorktreePath, "source.go")
	if err := os.WriteFile(dirtyPath, []byte("package source\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err == nil || !strings.Contains(err.Error(), "dirty-worktree") {
		t.Fatalf("Verify error = %v, want dirty-worktree", err)
	}
	if _, statErr := os.Stat(filepath.Join(created.WorktreePath, ".verification-ran")); !os.IsNotExist(statErr) {
		t.Fatalf("verification command unexpectedly ran: %v", statErr)
	}
	_ = repo
}

func TestVerifyRefusesUncommittedRecordAndSourceChanges(t *testing.T) {
	tests := []struct {
		name string
		path func(Created) string
	}{
		{name: "record", path: func(created Created) string { return created.BriefPath }},
		{name: "source", path: func(created Created) string { return filepath.Join(created.WorktreePath, "README.md") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, manager, created := createVerificationExperiment(t, false)
			if err := os.WriteFile(test.path(created), []byte("uncommitted\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
			if err == nil || !strings.Contains(err.Error(), "dirty-worktree") {
				t.Fatalf("Verify error = %v, want dirty-worktree", err)
			}
		})
	}
}

func TestVerifyReportsMissingArtisticArtifacts(t *testing.T) {
	_, manager, created := createVerificationExperiment(t, false)

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Clean || !report.RecordsPresent || report.ArtifactsPresent || !report.TestsPassed {
		t.Fatalf("report evidence = %#v", report)
	}
	for _, code := range []string{"missing-baseline-artifacts", "missing-candidate-artifacts", "missing-contact-sheet"} {
		if !hasVerifyDiagnostic(report.Diagnostics, code) {
			t.Errorf("diagnostics = %#v, want %s", report.Diagnostics, code)
		}
	}
}

func TestVerifyRejectsOutputPathTraversalAsArtifactDiagnostic(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, false)
	statePath := filepath.Join(created.WorktreePath, filepath.FromSlash(IDFromState(t, created.State).RecordDir()), "state.json")
	var state State
	readJSONFile(t, statePath, &state)
	state.Output = "../outside"
	if err := writeJSONAtomic(statePath, state); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join(IDFromState(t, created.State).RecordDir(), "state.json")))
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: malicious output path")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.ArtifactsPresent || !hasVerifyDiagnostic(report.Diagnostics, "invalid-output-path") {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyDefaultsToMakeCheck(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	makefile := filepath.Join(created.WorktreePath, "Makefile")
	if err := os.WriteFile(makefile, []byte("check:\n\t@git diff --quiet\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", "Makefile")
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: add verification target")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Command != "make check" || !report.TestsPassed {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyLifecycleOnlyPassesWithoutImageArtifacts(t *testing.T) {
	_, manager, created := createVerificationExperiment(t, true)

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Clean || !report.RecordsPresent || !report.ArtifactsPresent || !report.TestsPassed || len(report.Diagnostics) != 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyArtisticExperimentPassesWithComparisonArtifacts(t *testing.T) {
	_, manager, created := createVerificationExperiment(t, false)
	writeVerificationArtifacts(t, created)

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "rev-parse", "--show-toplevel"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Clean || !report.RecordsPresent || !report.ArtifactsPresent || !report.TestsPassed || len(report.Diagnostics) != 0 {
		t.Fatalf("report = %#v", report)
	}
	if strings.TrimSpace(report.Output) != created.WorktreePath {
		t.Fatalf("command output = %q, want assigned worktree %q", report.Output, created.WorktreePath)
	}
}

func TestVerifyCapturesFailingCommand(t *testing.T) {
	_, manager, created := createVerificationExperiment(t, true)

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "rev-parse", "--verify", "refs/heads/does-not-exist"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.TestsPassed || report.TestError == "" || !hasVerifyDiagnostic(report.Diagnostics, "tests-failed") {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyReadOnlyDoesNotChangeRefsOrTrackedFiles(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	beforeTip := repo.gitOutput(t, "rev-parse", "refs/heads/"+created.State.Branch)
	beforeState := readTextFile(t, filepath.Join(created.WorktreePath, filepath.FromSlash(IDFromState(t, created.State).RecordDir()), "state.json"))

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.TestsPassed {
		t.Fatalf("report = %#v", report)
	}
	if got := repo.gitOutput(t, "rev-parse", "refs/heads/"+created.State.Branch); got != beforeTip {
		t.Fatalf("branch tip = %s, want unchanged %s", got, beforeTip)
	}
	statePath := filepath.Join(created.WorktreePath, filepath.FromSlash(IDFromState(t, created.State).RecordDir()), "state.json")
	if got := readTextFile(t, statePath); got != beforeState {
		t.Fatal("read-only verification changed state.json")
	}
}

func TestVerifyRejectsTrackedChangesCreatedByCommand(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)

	readme := filepath.Join(created.WorktreePath, "README.md")
	if err := os.WriteFile(readme, []byte("test repository   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Commit a script whose direct execution modifies a tracked file while exiting zero.
	script := filepath.Join(created.WorktreePath, "dirty-command.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'changed\\n' > README.md\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", "README.md", "dirty-command.sh")
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: add dirty verification command")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"./dirty-command.sh"}, Record: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.TestsPassed || !hasVerifyDiagnostic(report.Diagnostics, "dirty-after-command") {
		t.Fatalf("report = %#v", report)
	}
	var state State
	readJSONFile(t, filepath.Join(created.WorktreePath, filepath.FromSlash(IDFromState(t, created.State).RecordDir()), "state.json"), &state)
	if state.Verification.CheckedAt != nil {
		t.Fatalf("dirty verification was recorded: %#v", state.Verification)
	}
}

func TestVerifyRecordsPassingAndFailingResultsWhenRequested(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		passed  bool
	}{
		{name: "passing", command: []string{"git", "diff", "--quiet"}, passed: true},
		{name: "failing", command: []string{"git", "rev-parse", "--verify", "refs/heads/missing"}, passed: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, manager, created := createVerificationExperiment(t, true)
			checkedAt := time.Date(2026, time.August, 5, 9, 30, 0, 0, time.UTC)
			manager.now = func() time.Time { return checkedAt }
			verifiedCommit := repo.gitOutput(t, "rev-parse", "refs/heads/"+created.State.Branch)

			report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: test.command, Record: true})
			if err != nil {
				t.Fatal(err)
			}
			if report.TestsPassed != test.passed {
				t.Fatalf("TestsPassed = %t, want %t", report.TestsPassed, test.passed)
			}
			var state State
			statePath := filepath.Join(created.WorktreePath, filepath.FromSlash(IDFromState(t, created.State).RecordDir()), "state.json")
			readJSONFile(t, statePath, &state)
			if state.Verification.CheckedAt == nil || !state.Verification.CheckedAt.Equal(checkedAt) || state.Verification.Commit != verifiedCommit || state.Verification.Command != strings.Join(test.command, " ") || state.Verification.Passed != test.passed {
				t.Fatalf("verification = %#v", state.Verification)
			}
			if got := repo.gitOutputAt(t, created.WorktreePath, "show", "--format=", "--name-only", "HEAD"); got != filepath.ToSlash(filepath.Join(IDFromState(t, created.State).RecordDir(), "state.json")) {
				t.Fatalf("verification commit paths = %q", got)
			}
		})
	}
}

func TestVerifyReportsBaseDriftAndSortedOverlap(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	base := created.State.BaseCommit
	if err := os.WriteFile(filepath.Join(repo.root, "README.md"), []byte("master change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "add", "README.md")
	repo.git(t, "commit", "-m", "test: advance master")
	master := repo.gitOutput(t, "rev-parse", "refs/heads/master")
	for _, name := range []string{"z.txt", "README.md", "a.txt"} {
		if err := os.WriteFile(filepath.Join(created.WorktreePath, name), []byte("experiment change\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.gitAt(t, created.WorktreePath, "add", "README.md", "a.txt", "z.txt")
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: experiment changes")
	experimentTip := repo.gitOutput(t, "rev-parse", "refs/heads/"+created.State.Branch)

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Drift.BaseCommit != base || report.Drift.CurrentMaster != master || report.Drift.MergeBase != base {
		t.Fatalf("drift commits = %#v", report.Drift)
	}
	if report.Commit != experimentTip || !slices.Equal(report.Drift.Overlap, []string{"README.md"}) {
		t.Fatalf("report drift = %#v", report)
	}
	if !slices.Contains(report.Drift.MasterPaths, "README.md") || !slices.Contains(report.Drift.ExperimentPaths, "README.md") {
		t.Fatalf("drift paths = %#v", report.Drift)
	}
}

func createVerificationExperiment(t *testing.T, lifecycleOnly bool) (testRepo, *Manager, Created) {
	t.Helper()
	repo := newTestRepo(t)
	manager := testManager(t, repo)
	created, err := manager.Create(context.Background(), CreateOptions{
		Piece:         "foam",
		Name:          "verification",
		LifecycleOnly: lifecycleOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	return repo, manager, created
}

func writeVerificationArtifacts(t *testing.T, created Created) {
	t.Helper()
	for _, name := range []string{"baseline/render.png", "candidate/render.png", "contact-sheet.png"} {
		path := filepath.Join(created.OutputPath, filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte("image\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func hasVerifyDiagnostic(diagnostics []Diagnostic, code string) bool {
	return slices.ContainsFunc(diagnostics, func(diagnostic Diagnostic) bool { return diagnostic.Code == code })
}

func IDFromState(t *testing.T, state State) ID {
	t.Helper()
	id, err := ParseID(state.ID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (r testRepo) gitAt(t *testing.T, dir string, args ...string) {
	t.Helper()
	r.root = dir
	r.git(t, args...)
}
