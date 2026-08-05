package experiment

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
	if !report.Clean || !report.RecordsPresent || report.ArtifactsPresent || !report.TestsPassed || report.Passed {
		t.Fatalf("report evidence = %#v", report)
	}
	for _, code := range []string{"missing-baseline-artifacts", "missing-candidate-artifacts", "missing-contact-sheet"} {
		if !hasVerifyDiagnostic(report.Diagnostics, code) {
			t.Errorf("diagnostics = %#v, want %s", report.Diagnostics, code)
		}
	}
}

func TestVerifyReportsCommittedMissingRecordsAsFailure(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	id := IDFromState(t, created.State)
	missing := filepath.ToSlash(filepath.Join(id.RecordDir(), "result.md"))
	if err := os.Remove(filepath.Join(created.WorktreePath, filepath.FromSlash(missing))); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", missing)
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: remove required record")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}, Record: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.RecordsPresent || report.Passed || !report.TestsPassed || !hasVerifyDiagnostic(report.Diagnostics, "missing-record") {
		t.Fatalf("report = %#v", report)
	}
	var state State
	readJSONFile(t, filepath.Join(created.WorktreePath, filepath.FromSlash(id.RecordDir()), "state.json"), &state)
	if state.Verification.Passed {
		t.Fatalf("verification = %#v, want failed", state.Verification)
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
	if !report.Clean || !report.RecordsPresent || !report.ArtifactsPresent || !report.TestsPassed || !report.Passed || len(report.Diagnostics) != 0 {
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
	if !report.Clean || !report.RecordsPresent || !report.ArtifactsPresent || !report.TestsPassed || !report.Passed || len(report.Diagnostics) != 0 {
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
	_, manager, created := createVerificationExperiment(t, true)
	manager.gitEnv = append(manager.gitEnv, "VERIFY_HELPER_ACTION=dirty")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{os.Args[0], "-test.run=^TestVerifyCommandHelper$"}, Record: true})
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

func TestVerifyAllowsCommandToCreateIgnoredOutputs(t *testing.T) {
	_, manager, created := createVerificationExperiment(t, true)
	manager.gitEnv = append(manager.gitEnv, "VERIFY_HELPER_ACTION=ignored-output")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{os.Args[0], "-test.run=^TestVerifyCommandHelper$"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Clean || !report.TestsPassed || !report.Passed {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(filepath.Join(created.OutputPath, "helper-output.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyAcceptsArtifactsGeneratedByCommand(t *testing.T) {
	_, manager, created := createVerificationExperiment(t, false)
	manager.gitEnv = append(manager.gitEnv, "VERIFY_HELPER_ACTION=artifacts")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{os.Args[0], "-test.run=^TestVerifyCommandHelper$"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.ArtifactsPresent || !report.Passed || len(report.Diagnostics) != 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyRejectsExperimentIdentityChangesMadeByCommand(t *testing.T) {
	tests := []struct {
		name    string
		command func(Created) []string
	}{
		{
			name: "detached HEAD",
			command: func(_ Created) []string {
				return []string{"git", "checkout", "--detach"}
			},
		},
		{
			name: "branch ref",
			command: func(created Created) []string {
				return []string{"git", "update-ref", "refs/heads/" + created.State.Branch, created.State.BaseCommit}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, manager, created := createVerificationExperiment(t, true)

			report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: test.command(created), Record: true})
			if err != nil {
				t.Fatal(err)
			}
			if report.TestsPassed || report.Passed || !hasVerifyDiagnostic(report.Diagnostics, "experiment-changed-after-command") {
				t.Fatalf("report = %#v", report)
			}
		})
	}
}

func TestVerifyCommandIgnoresPoisonedGitEnvironment(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	ambient := newTestRepo(t)
	manager.gitEnv = append(manager.gitEnv,
		"GIT_DIR="+filepath.Join(ambient.root, ".git"),
		"GIT_WORK_TREE="+ambient.root,
		"GIT_INDEX_FILE="+filepath.Join(ambient.root, ".git", "index"),
	)

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "rev-parse", "--show-toplevel"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || strings.TrimSpace(report.Output) != created.WorktreePath {
		t.Fatalf("report = %#v, fixture root %s", report, repo.root)
	}
}

func TestVerifyRejectsSymlinkArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture symlink creation requires Unix test permissions")
	}
	tests := []struct {
		name string
		link func(*testing.T, Created)
	}{
		{
			name: "baseline directory escapes output",
			link: func(t *testing.T, created Created) {
				t.Helper()
				if err := os.Remove(filepath.Join(created.OutputPath, "baseline", "render.png")); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(created.OutputPath, "baseline")); err != nil {
					t.Fatal(err)
				}
				external := t.TempDir()
				if err := os.WriteFile(filepath.Join(external, "render.png"), []byte("image\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, filepath.Join(created.OutputPath, "baseline")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "contact sheet symlink",
			link: func(t *testing.T, created Created) {
				t.Helper()
				if err := os.Remove(filepath.Join(created.OutputPath, "contact-sheet.png")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(created.OutputPath, "candidate", "render.png"), filepath.Join(created.OutputPath, "contact-sheet.png")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, manager, created := createVerificationExperiment(t, false)
			writeVerificationArtifacts(t, created)
			test.link(t, created)

			report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
			if err != nil {
				t.Fatal(err)
			}
			if report.ArtifactsPresent || report.Passed {
				t.Fatalf("report = %#v", report)
			}
		})
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
	for _, name := range []string{"z.txt", "README.md", "a.txt", "line\nbreak.txt"} {
		if err := os.WriteFile(filepath.Join(created.WorktreePath, name), []byte("experiment change\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.gitAt(t, created.WorktreePath, "add", "README.md", "a.txt", "z.txt", "line\nbreak.txt")
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
	if !slices.Contains(report.Drift.ExperimentPaths, "line\nbreak.txt") {
		t.Fatalf("NUL-delimited drift paths lost newline filename: %#v", report.Drift.ExperimentPaths)
	}
	if !report.Passed {
		t.Fatalf("computable base drift blocked verification: %#v", report)
	}
}

func TestVerifyDriftUsesCommitSnapshotsWhenCommandMovesMaster(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	if err := os.WriteFile(filepath.Join(repo.root, "master.txt"), []byte("master\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "add", "master.txt")
	repo.git(t, "commit", "-m", "test: snapshot master")
	snapshotMaster := repo.gitOutput(t, "rev-parse", "refs/heads/master")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "update-ref", "refs/heads/master", created.State.BaseCommit}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Drift.CurrentMaster != snapshotMaster || !slices.Contains(report.Drift.MasterPaths, "master.txt") {
		t.Fatalf("drift did not retain master snapshot: %#v", report.Drift)
	}
}

func TestVerifyMissingBaseBlocksUnifiedSuccessAndRecordedPass(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	id := IDFromState(t, created.State)
	statePath := filepath.Join(created.WorktreePath, filepath.FromSlash(id.RecordDir()), "state.json")
	var state State
	readJSONFile(t, statePath, &state)
	state.BaseCommit = strings.Repeat("f", 40)
	if err := writeJSONAtomic(statePath, state); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json")))
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: unavailable base")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}, Record: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || !report.TestsPassed || !hasVerifyDiagnostic(report.Diagnostics, "missing-base-commit") {
		t.Fatalf("report = %#v", report)
	}
	readJSONFile(t, statePath, &state)
	if state.Verification.Passed {
		t.Fatalf("verification = %#v, want failed", state.Verification)
	}
}

func TestVerifyMissingCurrentMasterBlocksUnifiedSuccess(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	repo.git(t, "update-ref", "-d", "refs/heads/master")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || !report.TestsPassed || !hasVerifyDiagnostic(report.Diagnostics, "missing-current-master") {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyMissingMergeBaseBlocksUnifiedSuccess(t *testing.T) {
	repo, manager, created := createVerificationExperiment(t, true)
	unrelated := repo.gitOutput(t, "commit-tree", repo.gitOutput(t, "rev-parse", "HEAD^{tree}"), "-m", "test: unrelated base")
	id := IDFromState(t, created.State)
	statePath := filepath.Join(created.WorktreePath, filepath.FromSlash(id.RecordDir()), "state.json")
	var state State
	readJSONFile(t, statePath, &state)
	state.BaseCommit = unrelated
	if err := writeJSONAtomic(statePath, state); err != nil {
		t.Fatal(err)
	}
	repo.gitAt(t, created.WorktreePath, "add", filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json")))
	repo.gitAt(t, created.WorktreePath, "commit", "-m", "test: unrelated base")

	report, err := manager.Verify(context.Background(), created.State.ID, VerifyOptions{Command: []string{"git", "diff", "--quiet"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || !report.TestsPassed || !hasVerifyDiagnostic(report.Diagnostics, "missing-merge-base") {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyCommandHelper(t *testing.T) {
	switch os.Getenv("VERIFY_HELPER_ACTION") {
	case "":
		t.Skip("subprocess helper")
	case "dirty":
		if err := os.WriteFile("README.md", []byte("changed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	case "ignored-output":
		if err := os.WriteFile(filepath.Join("out", "experiments", "foam", "verification", "helper-output.txt"), []byte("ignored\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	case "artifacts":
		root := filepath.Join("out", "experiments", "foam", "verification")
		for _, name := range []string{"baseline/render.png", "candidate/render.png", "contact-sheet.png"} {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte("image\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	default:
		t.Fatalf("unknown helper action %q", os.Getenv("VERIFY_HELPER_ACTION"))
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
