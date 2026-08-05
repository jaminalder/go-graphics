package experiment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// VerifyOptions controls the repository command and optional evidence record.
type VerifyOptions struct {
	Command []string
	Record  bool
}

// Drift describes changes since the experiment's original base commit.
type Drift struct {
	BaseCommit      string
	CurrentMaster   string
	MergeBase       string
	MasterPaths     []string
	ExperimentPaths []string
	Overlap         []string
	Ahead           int
	Behind          int
}

// VerifyReport contains the evidence gathered for one experiment revision.
type VerifyReport struct {
	ID               string
	Branch           string
	Commit           string
	Clean            bool
	RecordsPresent   bool
	ArtifactsPresent bool
	TestsPassed      bool
	Passed           bool
	Command          string
	Drift            Drift
	Diagnostics      []Diagnostic
	Output           string
	TestError        string
}

// Verify reconciles and verifies the exact active experiment worktree.
func (m *Manager) Verify(ctx context.Context, value string, opts VerifyOptions) (VerifyReport, error) {
	id, err := ParseID(value)
	if err != nil {
		return VerifyReport{}, err
	}
	command := opts.Command
	if len(command) == 0 {
		command = []string{"make", "check"}
	}
	for _, arg := range command {
		if strings.IndexByte(arg, 0) >= 0 {
			return VerifyReport{}, fmt.Errorf("verification command contains a NUL byte")
		}
	}
	if command[0] == "" {
		return VerifyReport{}, fmt.Errorf("verification command executable is empty")
	}

	experiment, err := m.Show(ctx, id.String())
	if err != nil {
		return VerifyReport{}, err
	}
	if len(experiment.Diagnostics) != 0 {
		return VerifyReport{}, fmt.Errorf("experiment %s failed reconciliation: %s", id.String(), formatDiagnostics(experiment.Diagnostics))
	}
	state := experiment.State
	worktree := experiment.WorktreePath
	expectedRef := "refs/heads/" + state.Branch
	runner := gitRunner{dir: worktree, env: m.gitEnv, disableOptionalLocks: true}
	if err := requireSymbolicHEAD(ctx, runner, expectedRef); err != nil {
		return VerifyReport{}, err
	}
	tip, err := resolveCommit(ctx, runner, expectedRef)
	if err != nil {
		return VerifyReport{}, err
	}
	head, err := resolveCommit(ctx, runner, "HEAD")
	if err != nil {
		return VerifyReport{}, err
	}
	if head != tip {
		return VerifyReport{}, fmt.Errorf("assigned worktree HEAD %s does not match branch tip %s", head, tip)
	}
	master, err := resolveCommit(ctx, runner, "refs/heads/master")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return VerifyReport{}, errors.Join(ctxErr, err)
		}
		master = ""
	}

	report := VerifyReport{
		ID:      id.String(),
		Branch:  state.Branch,
		Commit:  tip,
		Clean:   true,
		Command: strings.Join(command, " "),
	}
	clean, status, err := worktreeClean(ctx, runner)
	if err != nil {
		return report, err
	}
	if !clean {
		return report, fmt.Errorf("dirty-worktree: assigned worktree has uncommitted changes: %s", status)
	}

	report.RecordsPresent, report.Diagnostics, err = checkCommittedRecords(ctx, runner, id, tip, report.Diagnostics)
	if err != nil {
		return report, err
	}
	report.Drift, report.Diagnostics, err = calculateDrift(ctx, runner, state.BaseCommit, master, tip, report.Diagnostics)
	if err != nil {
		return report, err
	}

	output, commandErr := runVerificationCommand(ctx, worktree, m.gitEnv, command)
	report.Output = output
	if commandErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return report, fmt.Errorf("run verification command %q: %w", report.Command, errors.Join(ctxErr, commandErr))
		}
		var exitErr *exec.ExitError
		if !errors.As(commandErr, &exitErr) {
			return report, fmt.Errorf("run verification command %q: %w", report.Command, commandErr)
		}
		report.TestError = commandErr.Error()
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Code: "tests-failed", Message: fmt.Sprintf("verification command %q failed: %v", report.Command, commandErr)})
	} else {
		report.TestsPassed = true
	}

	changed, postDiagnostics, postErr := m.revalidateVerification(ctx, id, experiment, state, expectedRef, tip)
	if postErr != nil {
		return report, postErr
	}
	report.Diagnostics = append(report.Diagnostics, postDiagnostics...)
	if changed {
		report.Clean = false
		report.TestsPassed = false
	}
	if !changed {
		report.RecordsPresent, report.Diagnostics, err = checkCommittedRecords(ctx, runner, id, tip, report.Diagnostics)
		if err != nil {
			return report, err
		}
		report.ArtifactsPresent, report.Diagnostics = checkArtifacts(worktree, id, state, report.Diagnostics)
	}
	report.Passed = verificationPassed(report)
	if opts.Record {
		if changed {
			return report, nil
		}
		if err := m.recordVerification(ctx, id, state, tip, command, report.Passed); err != nil {
			return report, err
		}
	}
	return report, nil
}

func verificationPassed(report VerifyReport) bool {
	if !report.Clean || !report.RecordsPresent || !report.ArtifactsPresent || !report.TestsPassed {
		return false
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnosticBlocksVerification(diagnostic.Code) {
			return false
		}
	}
	return true
}

func diagnosticBlocksVerification(code string) bool {
	// Computable base drift is represented by Drift itself and is informational.
	// Every diagnostic currently indicates missing or invalid verification evidence.
	return code != "base-drift"
}

func (m *Manager) revalidateVerification(
	ctx context.Context,
	id ID,
	initial Experiment,
	state State,
	expectedRef, tip string,
) (bool, []Diagnostic, error) {
	changed := false
	diagnostics := make([]Diagnostic, 0, 2)
	addChanged := func(message string) {
		changed = true
		diagnostics = append(diagnostics, Diagnostic{Code: "experiment-changed-after-command", Message: message})
	}

	experiment, err := m.Show(ctx, id.String())
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, nil, errors.Join(ctxErr, err)
		}
		addChanged(fmt.Sprintf("experiment reconciliation failed after command: %v", err))
		return changed, diagnostics, nil
	}
	if len(experiment.Diagnostics) != 0 {
		addChanged("experiment reconciliation changed after command: " + formatDiagnostics(experiment.Diagnostics))
	}
	if filepath.Clean(experiment.WorktreePath) != filepath.Clean(initial.WorktreePath) {
		addChanged(fmt.Sprintf("assigned worktree changed from %s to %s", initial.WorktreePath, experiment.WorktreePath))
	}
	if !statesMatchForVerification(experiment.State, state) {
		addChanged("experiment state changed after command")
	}

	runner := gitRunner{dir: initial.WorktreePath, env: m.gitEnv, disableOptionalLocks: true}
	if err := requireSymbolicHEAD(ctx, runner, expectedRef); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, nil, errors.Join(ctxErr, err)
		}
		addChanged(err.Error())
	}
	branchTip, err := resolveCommit(ctx, runner, expectedRef)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, nil, errors.Join(ctxErr, err)
		}
		addChanged(fmt.Sprintf("experiment branch ref is unavailable after command: %v", err))
	} else if branchTip != tip {
		addChanged(fmt.Sprintf("experiment branch tip changed from %s to %s", tip, branchTip))
	}
	head, err := resolveCommit(ctx, runner, "HEAD")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, nil, errors.Join(ctxErr, err)
		}
		addChanged(fmt.Sprintf("worktree HEAD is unavailable after command: %v", err))
	} else if head != tip {
		addChanged(fmt.Sprintf("worktree HEAD changed from %s to %s", tip, head))
	}
	clean, status, err := worktreeClean(ctx, runner)
	if err != nil {
		return false, nil, err
	}
	if !clean {
		changed = true
		diagnostics = append(diagnostics, Diagnostic{Code: "dirty-after-command", Message: fmt.Sprintf("verification command left tracked or untracked changes: %s", status)})
	}
	return changed, diagnostics, nil
}

func formatDiagnostics(diagnostics []Diagnostic) string {
	parts := make([]string, len(diagnostics))
	for i, diagnostic := range diagnostics {
		parts[i] = diagnostic.Code + ": " + diagnostic.Message
	}
	return strings.Join(parts, "; ")
}

func resolveCommit(ctx context.Context, runner gitRunner, revision string) (string, error) {
	output, err := runner.run(ctx, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func worktreeClean(ctx context.Context, runner gitRunner) (bool, string, error) {
	output, err := runner.run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return false, "", err
	}
	if output == "" {
		return true, "", nil
	}
	return false, strings.ReplaceAll(strings.TrimSuffix(output, "\x00"), "\x00", ", "), nil
}

func checkCommittedRecords(ctx context.Context, runner gitRunner, id ID, tip string, diagnostics []Diagnostic) (bool, []Diagnostic, error) {
	present := true
	for _, name := range []string{"brief.md", "state.json", "result.md", "favorites.json"} {
		path := filepath.ToSlash(filepath.Join(id.RecordDir(), name))
		output, err := runner.run(ctx, "ls-tree", "-z", "--name-only", tip, "--", path)
		if err != nil {
			return false, diagnostics, err
		}
		if output != path+"\x00" {
			present = false
			message := fmt.Sprintf("required record %s is not committed at %s", path, tip)
			diagnostics = append(diagnostics, Diagnostic{Code: "missing-record", Message: message})
		}
	}
	return present, diagnostics, nil
}

func checkArtifacts(worktree string, id ID, state State, diagnostics []Diagnostic) (bool, []Diagnostic) {
	if state.LifecycleOnly {
		return true, diagnostics
	}
	output, err := containedOutputPath(worktree, id, state.Output)
	if err != nil {
		return false, append(diagnostics, Diagnostic{Code: "invalid-output-path", Message: err.Error()})
	}
	present := true
	for _, name := range []string{"baseline", "candidate"} {
		directory := filepath.Join(output, name)
		nonempty, inspectErr := containsNonemptyRegularFile(directory, output)
		if inspectErr != nil || !nonempty {
			present = false
			message := fmt.Sprintf("%s artifact directory has no nonempty files: %s", name, directory)
			if inspectErr != nil {
				message += ": " + inspectErr.Error()
			}
			diagnostics = append(diagnostics, Diagnostic{Code: "missing-" + name + "-artifacts", Message: message})
		}
	}
	contactSheet := filepath.Join(output, "contact-sheet.png")
	info, statErr := os.Lstat(contactSheet)
	if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		present = false
		message := fmt.Sprintf("required nonempty contact sheet is missing: %s", contactSheet)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			message += ": " + statErr.Error()
		}
		diagnostics = append(diagnostics, Diagnostic{Code: "missing-contact-sheet", Message: message})
	}
	return present, diagnostics
}

func containedOutputPath(worktree string, id ID, stateOutput string) (string, error) {
	if stateOutput == "" || filepath.IsAbs(filepath.FromSlash(stateOutput)) {
		return "", fmt.Errorf("state output must be a nonempty worktree-relative path: %q", stateOutput)
	}
	worktree = filepath.Clean(worktree)
	expected := filepath.Join(worktree, filepath.FromSlash(id.OutputDir()))
	output := filepath.Clean(filepath.Join(worktree, filepath.FromSlash(stateOutput)))
	if output != expected {
		return "", fmt.Errorf("state output %q resolves to %s, want exact experiment output %s", stateOutput, output, expected)
	}
	if err := rejectSymlinkComponents(worktree, expected); err != nil {
		return "", err
	}
	resolvedWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return "", fmt.Errorf("resolve assigned worktree: %w", err)
	}
	resolvedExpected, err := filepath.EvalSymlinks(expected)
	if err != nil {
		return "", fmt.Errorf("resolve experiment output root %q: %w", expected, err)
	}
	resolvedOutput, err := filepath.EvalSymlinks(output)
	if err != nil {
		return "", fmt.Errorf("resolve state output %q: %w", output, err)
	}
	if !pathWithin(resolvedExpected, resolvedWorktree) || resolvedOutput != resolvedExpected {
		return "", fmt.Errorf("state output %q escapes the assigned experiment output through a symlink", stateOutput)
	}
	return output, nil
}

func rejectSymlinkComponents(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside root %q", path, root)
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect output path component %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("output path component is a symlink: %s", current)
		}
	}
	return nil
}

func pathWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func containsNonemptyRegularFile(root, outputRoot string) (bool, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("not a real directory")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}
	resolvedOutput, err := filepath.EvalSymlinks(outputRoot)
	if err != nil {
		return false, err
	}
	if !pathWithin(resolvedRoot, resolvedOutput) {
		return false, fmt.Errorf("artifact directory resolves outside output root")
	}
	found := false
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact path is a symlink: %s", path)
		}
		if entry.Type().IsRegular() {
			fileInfo, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			if fileInfo.Size() > 0 {
				found = true
			}
		}
		return nil
	})
	return found, err
}

func calculateDrift(ctx context.Context, runner gitRunner, base, master, experimentTip string, diagnostics []Diagnostic) (Drift, []Diagnostic, error) {
	drift := Drift{BaseCommit: base, CurrentMaster: master}
	if master == "" {
		diagnostics = append(diagnostics, Diagnostic{Code: "missing-current-master", Message: "refs/heads/master is unavailable"})
		return drift, diagnostics, nil
	}
	if _, err := resolveCommit(ctx, runner, base); err != nil {
		diagnostics = append(diagnostics, Diagnostic{Code: "missing-base-commit", Message: fmt.Sprintf("original base commit %s is unavailable: %v", base, err)})
		return drift, diagnostics, nil
	}
	mergeBase, err := runner.run(ctx, "merge-base", base, master)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{Code: "missing-merge-base", Message: fmt.Sprintf("calculate merge base for %s and %s: %v", base, master, err)})
		return drift, diagnostics, nil
	}
	drift.MergeBase = strings.TrimSpace(mergeBase)
	drift.MasterPaths, err = changedPaths(ctx, runner, base, master)
	if err != nil {
		return drift, diagnostics, err
	}
	drift.ExperimentPaths, err = changedPaths(ctx, runner, base, experimentTip)
	if err != nil {
		return drift, diagnostics, err
	}
	sort.Strings(drift.MasterPaths)
	sort.Strings(drift.ExperimentPaths)
	drift.Overlap = sortedIntersection(drift.MasterPaths, drift.ExperimentPaths)
	counts, err := runner.run(ctx, "rev-list", "--left-right", "--count", master+"..."+experimentTip)
	if err != nil {
		return drift, diagnostics, err
	}
	fields := strings.Fields(counts)
	if len(fields) != 2 {
		return drift, diagnostics, fmt.Errorf("parse ahead/behind counts %q", strings.TrimSpace(counts))
	}
	drift.Behind, err = strconv.Atoi(fields[0])
	if err != nil {
		return drift, diagnostics, fmt.Errorf("parse behind count %q: %w", fields[0], err)
	}
	drift.Ahead, err = strconv.Atoi(fields[1])
	if err != nil {
		return drift, diagnostics, fmt.Errorf("parse ahead count %q: %w", fields[1], err)
	}
	return drift, diagnostics, nil
}

func sortedIntersection(left, right []string) []string {
	var overlap []string
	for i, j := 0, 0; i < len(left) && j < len(right); {
		switch {
		case left[i] < right[j]:
			i++
		case left[i] > right[j]:
			j++
		default:
			overlap = append(overlap, left[i])
			i++
			j++
		}
	}
	return overlap
}

func runVerificationCommand(ctx context.Context, worktree string, env, command []string) (string, error) {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = worktree
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = sanitizedGitEnvironment(env)
	cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (m *Manager) recordVerification(ctx context.Context, id ID, expected State, parent string, command []string, passed bool) error {
	lock, err := m.AcquireExperimentLock(ctx, id, "verify --record "+id.String())
	if err != nil {
		return err
	}
	recordErr := m.recordVerificationLocked(ctx, id, expected, parent, command, passed)
	return errors.Join(recordErr, m.release(lock))
}

func (m *Manager) recordVerificationLocked(ctx context.Context, id ID, expected State, parent string, command []string, passed bool) error {
	experiment, err := m.Show(ctx, id.String())
	if err != nil {
		return err
	}
	if len(experiment.Diagnostics) != 0 {
		return fmt.Errorf("experiment %s changed before recording verification: %s", id.String(), formatDiagnostics(experiment.Diagnostics))
	}
	state := experiment.State
	if !statesMatchForVerification(state, expected) {
		return fmt.Errorf("experiment state changed before recording verification")
	}
	worktree := experiment.WorktreePath
	runner := gitRunner{dir: worktree, env: m.gitEnv}
	tip, err := resolveCommit(ctx, runner, "refs/heads/"+state.Branch)
	if err != nil {
		return err
	}
	if tip != parent {
		return fmt.Errorf("experiment branch changed before recording verification: got %s, want %s", tip, parent)
	}
	clean, status, err := worktreeClean(ctx, gitRunner{dir: worktree, env: m.gitEnv, disableOptionalLocks: true})
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("experiment became dirty before recording verification: %s", status)
	}

	statePath := filepath.Join(worktree, filepath.FromSlash(id.RecordDir()), "state.json")
	oldState, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("capture old state before verification record: %w", err)
	}
	recordPath := filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json"))
	oldIndex, err := readIndexEntry(ctx, runner, recordPath)
	if err != nil {
		return fmt.Errorf("capture state index before verification record: %w", err)
	}
	now := m.now
	if now == nil {
		now = time.Now
	}
	state.Verification = Verification{
		CheckedAt: pointerToTime(now().UTC()),
		Commit:    parent,
		Command:   strings.Join(command, " "),
		Passed:    passed,
	}
	state.UpdatedAt = *state.Verification.CheckedAt
	if err := writeJSONAtomic(statePath, state); err != nil {
		return err
	}
	writtenState, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("capture written verification state: %w", err)
	}
	result, err := commitRecord(
		ctx,
		worktree,
		filepath.FromSlash(id.RecordDir()),
		[]string{filepath.Join(filepath.FromSlash(id.RecordDir()), "state.json")},
		"experiment: verify "+id.String(),
		state.Branch,
		parent,
		m.gitEnv,
		m.createCheckpoint,
	)
	if err == nil {
		return nil
	}
	if result.RefUpdated {
		return appliedCommitError(result, "refs/heads/"+state.Branch, err)
	}
	recoveryErr := recoverVerificationAfterCommitFailure(ctx, runner, "refs/heads/"+state.Branch, parent, result, statePath, recordPath, state.Verification, expected, oldState, writtenState, oldIndex)
	return errors.Join(err, recoveryErr)
}

func statesMatchForVerification(left, right State) bool {
	return reflect.DeepEqual(left, right)
}

func pointerToTime(value time.Time) *time.Time { return &value }

func recoverVerificationAfterCommitFailure(
	ctx context.Context,
	runner gitRunner,
	expectedRef, expectedParent string,
	result recordCommitResult,
	statePath, recordPath string,
	wanted Verification,
	oldDecoded State,
	oldState, writtenState []byte,
	oldIndex indexEntry,
) error {
	tip, err := resolveCommit(ctx, runner, expectedRef)
	if err != nil {
		return indeterminateStateError(expectedRef, expectedParent, result.Commit, recordPath, err)
	}
	if result.Commit != "" && tip == result.Commit {
		return updateCommittedIndexPaths(ctx, runner, tip, []string{recordPath})
	}
	if tip == expectedParent {
		_, rollbackErr := recoverLocalStateAfterCommitFailure(statePath, oldDecoded, oldState, writtenState)
		return rollbackErr
	}
	authoritativeData, err := runner.run(ctx, "show", tip+":"+recordPath)
	if err != nil {
		return indeterminateStateError(expectedRef, expectedParent, result.Commit, recordPath, err)
	}
	authoritative, err := decodeState(tip+":"+recordPath, []byte(authoritativeData))
	if err != nil {
		return err
	}
	if reflect.DeepEqual(authoritative.Verification, wanted) {
		return updateCommittedIndexPaths(ctx, runner, tip, []string{recordPath})
	}
	current, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, writtenState) {
		return fmt.Errorf("%w; preserving current state content", errStateChangedDuringCommit)
	}
	changed, err := changedPaths(ctx, runner, expectedParent, tip)
	if err != nil {
		return err
	}
	if len(changed) != 1 || changed[0] != recordPath {
		return fmt.Errorf("competing branch %s changed paths beyond %s: %v; local attempted state preserved", tip, recordPath, changed)
	}
	currentIndex, err := readIndexEntry(ctx, runner, recordPath)
	if err != nil {
		return err
	}
	if currentIndex != oldIndex {
		return fmt.Errorf("%w; state index changed while recording verification", errStateChangedDuringCommit)
	}
	authoritativeIndex, err := readTreeEntry(ctx, runner, tip, recordPath)
	if err != nil {
		return err
	}
	if err := writeBytesAtomic(statePath, []byte(authoritativeData)); err != nil {
		return err
	}
	return writeIndexEntry(ctx, runner, recordPath, authoritativeIndex)
}
