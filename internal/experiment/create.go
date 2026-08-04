package experiment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jaminalder/go-graphics/internal/render"
)

const defaultMaxWriters = 2

const (
	checkpointBeforeBranch   = "before-branch"
	checkpointBeforeWorktree = "before-worktree"
	checkpointBeforeCommit   = "before-commit"
)

// CreateOptions configures one ordinary experiment worktree.
type CreateOptions struct {
	Piece          string
	Name           string
	BaseBranch     string
	BaseExperiment string
	Stage          string
	Profile        string
	Seeds          []uint64
	WorkerTool     string
	WorkerSession  *string
	MaxWriters     int
	LifecycleOnly  bool
}

// Created describes the isolated resources prepared for a worker.
type Created struct {
	State             State
	WorktreePath      string
	BriefPath         string
	OutputPath        string
	WorkerInstruction string
}

type briefData struct {
	ID                string
	CreatedAt         string
	BaseCommit        string
	Stage             string
	Profile           string
	Seeds             []uint64
	SourceExperiments []string
	OutputPath        string
	BaselineCommand   string
	CandidateCommand  string
}

type createResources struct {
	branch          string
	worktreePath    string
	branchCreated   bool
	worktreeCreated bool
}

// Create creates and initializes an ordinary experiment branch and sibling worktree.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (Created, error) {
	id, err := ParseID(opts.Piece + "/" + opts.Name)
	if err != nil {
		return Created{}, err
	}
	globalLock, err := m.AcquireGlobalLock(ctx, "create "+id.String())
	if err != nil {
		return Created{}, err
	}
	idLock, err := m.AcquireExperimentLock(ctx, id, "create "+id.String())
	if err != nil {
		return Created{}, errors.Join(err, m.release(globalLock))
	}

	created, resources, createErr := m.createLocked(ctx, id, opts)
	releaseErr := errors.Join(m.release(idLock), m.release(globalLock))
	err = errors.Join(createErr, releaseErr)
	if err != nil && (resources.branchCreated || resources.worktreeCreated) {
		return created, createFailure(id, resources, err)
	}
	return created, err
}

func (m *Manager) createLocked(ctx context.Context, id ID, opts CreateOptions) (Created, createResources, error) {
	if err := m.RequireCoordinator(); err != nil {
		return Created{}, createResources{}, err
	}
	runner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}
	if err := m.validateCoordinator(ctx, runner); err != nil {
		return Created{}, createResources{}, err
	}

	opts, err := normalizedCreateOptions(opts)
	if err != nil {
		return Created{}, createResources{}, err
	}
	branch := id.ExperimentBranch()
	worktreePath := id.WorktreePath(m.CoordinatorRoot)
	recordDir := id.RecordDir()
	resources := createResources{branch: branch, worktreePath: worktreePath}
	partial := Created{WorktreePath: worktreePath}
	if err := m.validateAvailableResources(ctx, runner, id, branch, worktreePath); err != nil {
		return partial, resources, err
	}
	if err := m.enforceWriterLimit(ctx, opts.MaxWriters); err != nil {
		return partial, resources, err
	}

	baseBranch, baseCommit, err := m.resolveCreateBase(ctx, runner, opts)
	if err != nil {
		return partial, resources, err
	}
	if err := m.checkpoint(checkpointBeforeBranch); err != nil {
		return partial, resources, err
	}
	if err := m.validateCoordinator(ctx, runner); err != nil {
		return partial, resources, err
	}
	if err := m.validateAvailableResources(ctx, runner, id, branch, worktreePath); err != nil {
		return partial, resources, err
	}
	currentBase, err := runner.run(ctx, "rev-parse", "--verify", "refs/heads/"+baseBranch+"^{commit}")
	if err != nil {
		return partial, resources, fmt.Errorf("revalidate base branch before experiment branch creation: %w", err)
	}
	if strings.TrimSpace(currentBase) != baseCommit {
		return partial, resources, fmt.Errorf("base branch changed before experiment branch creation: got %s, want %s", strings.TrimSpace(currentBase), baseCommit)
	}
	if _, err := runner.run(ctx, "branch", branch, baseCommit); err != nil {
		return partial, resources, err
	}
	resources.branchCreated = true
	if err := m.checkpoint(checkpointBeforeWorktree); err != nil {
		return partial, resources, err
	}
	if err := m.validateBranchForWorktree(ctx, runner, branch, baseCommit, worktreePath); err != nil {
		return partial, resources, err
	}
	if _, err := runner.run(ctx, "worktree", "add", worktreePath, branch); err != nil {
		return partial, resources, err
	}
	resources.worktreeCreated = true

	outputRelative := filepath.ToSlash(id.OutputDir())
	outputPath := filepath.Join(worktreePath, filepath.FromSlash(outputRelative))
	partial.OutputPath = outputPath
	for _, dir := range []string{"baseline", "candidate", "metadata"} {
		if err := os.MkdirAll(filepath.Join(outputPath, dir), 0o755); err != nil {
			return partial, resources, fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}
	recordPath := filepath.Join(worktreePath, filepath.FromSlash(recordDir))
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		return partial, resources, fmt.Errorf("create record directory: %w", err)
	}
	now := m.now
	if now == nil {
		now = time.Now
	}
	createdAt := now().UTC()
	state := State{
		SchemaVersion:  1,
		ID:             id.String(),
		Kind:           KindExperiment,
		Branch:         branch,
		Worktree:       filepath.ToSlash(relativePath(m.CoordinatorRoot, worktreePath)),
		BaseBranch:     baseBranch,
		BaseCommit:     baseCommit,
		BaseExperiment: opts.BaseExperiment,
		Status:         StatusCreated,
		Stage:          opts.Stage,
		Worker:         Worker{Tool: opts.WorkerTool, Session: opts.WorkerSession},
		Seeds:          opts.Seeds,
		Profile:        opts.Profile,
		Output:         outputRelative,
		Sources:        []Source{},
		Verification:   Verification{},
		LifecycleOnly:  opts.LifecycleOnly,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	partial.State = state
	if err := writeJSONAtomic(filepath.Join(recordPath, "state.json"), state); err != nil {
		return partial, resources, err
	}
	if err := writeJSONAtomic(filepath.Join(recordPath, "favorites.json"), []Favorite{}); err != nil {
		return partial, resources, err
	}
	seeds := formatSeeds(opts.Seeds)
	templateData := briefData{
		ID:                id.String(),
		CreatedAt:         createdAt.Format(time.RFC3339),
		BaseCommit:        baseCommit,
		Stage:             opts.Stage,
		Profile:           opts.Profile,
		Seeds:             opts.Seeds,
		SourceExperiments: []string{},
		OutputPath:        outputRelative,
		BaselineCommand:   "go run ./cmd/staticart sweep " + id.Piece() + " --seeds " + seeds + " --profile " + opts.Profile + " --out " + outputRelative + "/baseline",
		CandidateCommand:  "go run ./cmd/staticart sweep " + id.Piece() + " --seeds " + seeds + " --profile " + opts.Profile + " --out " + outputRelative + "/candidate",
	}
	briefPath := filepath.Join(recordPath, "brief.md")
	partial.BriefPath = briefPath
	if err := renderTemplate(filepath.Join(m.TemplatesRoot, "brief.md"), briefPath, templateData); err != nil {
		return partial, resources, err
	}
	if err := renderTemplate(filepath.Join(m.TemplatesRoot, "result.md"), filepath.Join(recordPath, "result.md"), templateData); err != nil {
		return partial, resources, err
	}
	if err := m.checkpoint(checkpointBeforeCommit); err != nil {
		return partial, resources, err
	}
	if err := m.validateAssignedWorktree(ctx, runner, branch, worktreePath, baseCommit); err != nil {
		return partial, resources, err
	}
	if err := commitRecord(ctx, worktreePath, recordDir, "experiment: create "+id.String(), m.gitEnv); err != nil {
		return partial, resources, err
	}

	instruction := "Work on experiment " + id.String() + " only.\n" +
		"Worktree: " + worktreePath + "\n" +
		"Branch: " + branch + "\n" +
		"Brief: " + briefPath + "\n" +
		"Operate only inside this worktree. Do not switch branches. Do not create or remove worktrees. Do not merge, rebase, or modify master. Do not work outside the assigned scope. Do not modify another experiment's files."
	partial.WorkerInstruction = instruction
	return partial, resources, nil
}

func normalizedCreateOptions(opts CreateOptions) (CreateOptions, error) {
	if opts.BaseExperiment != "" && opts.BaseBranch != "" && opts.BaseBranch != "master" {
		return CreateOptions{}, fmt.Errorf("base branch cannot be combined with base experiment")
	}
	if opts.BaseBranch == "" || opts.BaseExperiment != "" {
		opts.BaseBranch = "master"
	}
	if opts.Stage == "" {
		if opts.LifecycleOnly {
			opts.Stage = "lifecycle"
		} else {
			opts.Stage = "rendering"
		}
	}
	if err := ValidateStage(opts.Stage); err != nil {
		return CreateOptions{}, err
	}
	if opts.LifecycleOnly && opts.Stage != "lifecycle" {
		return CreateOptions{}, fmt.Errorf("lifecycle-only experiments require lifecycle stage")
	}
	if opts.Profile == "" {
		opts.Profile = "preview"
	}
	if _, err := render.ProfileByName(opts.Profile); err != nil {
		return CreateOptions{}, err
	}
	if len(opts.Seeds) == 0 {
		opts.Seeds = []uint64{1, 2, 3, 5, 8, 13}
	}
	for _, seed := range opts.Seeds {
		if seed == 0 {
			return CreateOptions{}, fmt.Errorf("experiment seeds must be positive")
		}
	}
	if opts.WorkerTool == "" {
		opts.WorkerTool = "unknown"
	}
	if opts.MaxWriters == 0 {
		opts.MaxWriters = defaultMaxWriters
		if value := os.Getenv("EXPERIMENT_MAX_WRITERS"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return CreateOptions{}, fmt.Errorf("parse EXPERIMENT_MAX_WRITERS: %w", err)
			}
			opts.MaxWriters = parsed
		}
	}
	if opts.MaxWriters < 1 {
		return CreateOptions{}, fmt.Errorf("maximum writers must be positive")
	}
	return opts, nil
}

func (m *Manager) resolveCreateBase(ctx context.Context, runner gitRunner, opts CreateOptions) (string, string, error) {
	if opts.BaseExperiment != "" {
		parentID, err := ParseID(opts.BaseExperiment)
		if err != nil {
			return "", "", fmt.Errorf("invalid base experiment: %w", err)
		}
		parentBranch := parentID.ExperimentBranch()
		parentCommitOutput, err := runner.run(ctx, "rev-parse", "--verify", "refs/heads/"+parentBranch+"^{commit}")
		if err != nil {
			return "", "", fmt.Errorf("resolve base experiment %s branch: %w", opts.BaseExperiment, err)
		}
		parentCommit := strings.TrimSpace(parentCommitOutput)
		parentStatePath := filepath.ToSlash(filepath.Join(parentID.RecordDir(), "state.json"))
		data, err := runner.run(ctx, "show", parentCommit+":"+parentStatePath)
		if err != nil {
			return "", "", fmt.Errorf("resolve base experiment %s: %w", opts.BaseExperiment, err)
		}
		parent, err := decodeState(parentBranch, []byte(data))
		if err != nil {
			return "", "", fmt.Errorf("resolve base experiment %s: %w", opts.BaseExperiment, err)
		}
		if parent.ID != opts.BaseExperiment || parent.Kind != KindExperiment || parent.Branch != parentID.ExperimentBranch() {
			return "", "", fmt.Errorf("base experiment %s has inconsistent identity", opts.BaseExperiment)
		}
		if parent.Status == StatusMerged || parent.Status == StatusDiscarded {
			return "", "", fmt.Errorf("base experiment %s is not active", opts.BaseExperiment)
		}
		baseBranch := parent.Branch
		worktreeOutput, err := runner.run(ctx, "worktree", "list", "--porcelain", "-z")
		if err != nil {
			return "", "", err
		}
		worktrees, err := parseWorktreeList(worktreeOutput)
		if err != nil {
			return "", "", err
		}
		registered := false
		for _, worktree := range worktrees {
			if filepath.Clean(worktree.Path) == filepath.Clean(parentID.WorktreePath(m.CoordinatorRoot)) && worktree.Branch == "refs/heads/"+parent.Branch {
				registered = true
				break
			}
		}
		if !registered {
			return "", "", fmt.Errorf("base experiment %s does not have its expected active worktree", opts.BaseExperiment)
		}
		return baseBranch, parentCommit, nil
	}
	baseBranch := opts.BaseBranch
	if strings.HasPrefix(baseBranch, "refs/") {
		return "", "", fmt.Errorf("invalid base branch %q: want a plain local branch name", baseBranch)
	}
	if strings.HasPrefix(baseBranch, "exp/") || strings.HasPrefix(baseBranch, "integrate/") {
		return "", "", fmt.Errorf("base branch %q cannot use experiment namespace; use base experiment", baseBranch)
	}
	validated, err := runner.run(ctx, "check-ref-format", "--branch", baseBranch)
	if err != nil {
		return "", "", fmt.Errorf("invalid base branch %q: %w", baseBranch, err)
	}
	if strings.TrimSpace(validated) != baseBranch {
		return "", "", fmt.Errorf("invalid base branch %q: Git resolved it as %q", baseBranch, strings.TrimSpace(validated))
	}
	exists, err := localBranchExists(ctx, runner, baseBranch)
	if err != nil {
		return "", "", err
	}
	if !exists {
		return "", "", fmt.Errorf("base branch does not exist: %s", baseBranch)
	}
	commit, err := runner.run(ctx, "rev-parse", "--verify", "refs/heads/"+baseBranch+"^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("resolve base branch %s: %w", baseBranch, err)
	}
	return baseBranch, strings.TrimSpace(commit), nil
}

func (m *Manager) validateCoordinator(ctx context.Context, runner gitRunner) error {
	if err := m.RequireCoordinator(); err != nil {
		return err
	}
	currentRoot, err := runner.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	root, err := canonicalPath(strings.TrimSpace(currentRoot), m.CoordinatorRoot)
	if err != nil {
		return fmt.Errorf("resolve coordinator before mutation: %w", err)
	}
	if root != m.CoordinatorRoot {
		return fmt.Errorf("coordinator changed before mutation: got %q, want %q", root, m.CoordinatorRoot)
	}
	currentBranch, err := runner.run(ctx, "branch", "--show-current")
	if err != nil {
		return err
	}
	if strings.TrimSpace(currentBranch) != "master" {
		return fmt.Errorf("create requires coordinator branch master; current branch is %q", strings.TrimSpace(currentBranch))
	}
	status, err := runner.run(ctx, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("coordinator worktree is not clean: %s", strings.TrimSpace(status))
	}
	return nil
}

func (m *Manager) validateAvailableResources(ctx context.Context, runner gitRunner, id ID, branch, worktreePath string) error {
	refsOutput, err := runner.run(ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads/exp", "refs/heads/integrate")
	if err != nil {
		return err
	}
	refs := strings.Fields(refsOutput)
	for _, candidate := range []string{branch, id.IntegrationBranch()} {
		if slices.Contains(refs, candidate) {
			return fmt.Errorf("experiment branch already exists: %s", candidate)
		}
	}
	worktreeOutput, err := runner.run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return err
	}
	worktrees, err := parseWorktreeList(worktreeOutput)
	if err != nil {
		return err
	}
	for _, worktree := range worktrees {
		if filepath.Clean(worktree.Path) == filepath.Clean(worktreePath) || worktree.Branch == "refs/heads/"+branch {
			return fmt.Errorf("experiment worktree already registered: %s", worktree.Path)
		}
	}
	if err := requirePathAbsent(worktreePath, "experiment worktree path already exists", "inspect experiment worktree path"); err != nil {
		return err
	}
	coordinatorRecord := filepath.Join(m.CoordinatorRoot, filepath.FromSlash(id.RecordDir()))
	if err := requirePathAbsent(coordinatorRecord, "experiment record already exists", "inspect experiment record"); err != nil {
		return err
	}
	archiveRecord := filepath.Join(m.CoordinatorRoot, filepath.FromSlash(id.ArchiveDir()))
	return requirePathAbsent(archiveRecord, "experiment record already exists", "inspect archived experiment record")
}

func (m *Manager) validateBranchForWorktree(ctx context.Context, runner gitRunner, branch, baseCommit, worktreePath string) error {
	commit, err := runner.run(ctx, "rev-parse", "--verify", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		return fmt.Errorf("verify experiment branch before worktree add: %w", err)
	}
	if strings.TrimSpace(commit) != baseCommit {
		return fmt.Errorf("experiment branch %q changed before worktree add: got %s, want %s", branch, strings.TrimSpace(commit), baseCommit)
	}
	worktreeOutput, err := runner.run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return err
	}
	worktrees, err := parseWorktreeList(worktreeOutput)
	if err != nil {
		return err
	}
	for _, worktree := range worktrees {
		if filepath.Clean(worktree.Path) == filepath.Clean(worktreePath) || worktree.Branch == "refs/heads/"+branch {
			return fmt.Errorf("experiment worktree already registered: %s", worktree.Path)
		}
	}
	return requirePathAbsent(worktreePath, "experiment worktree path already exists", "inspect experiment worktree path")
}

func (m *Manager) validateAssignedWorktree(ctx context.Context, runner gitRunner, branch, worktreePath, baseCommit string) error {
	worktreeOutput, err := runner.run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return err
	}
	worktrees, err := parseWorktreeList(worktreeOutput)
	if err != nil {
		return err
	}
	for _, worktree := range worktrees {
		if filepath.Clean(worktree.Path) != filepath.Clean(worktreePath) {
			continue
		}
		if worktree.Branch != "refs/heads/"+branch {
			return fmt.Errorf("assigned worktree branch changed: got %q, want %q", worktree.Branch, "refs/heads/"+branch)
		}
		if worktree.HEAD != baseCommit {
			return fmt.Errorf("assigned worktree tip changed: got %s, want %s", worktree.HEAD, baseCommit)
		}
		return nil
	}
	return fmt.Errorf("assigned worktree is no longer registered: %s", worktreePath)
}

func localBranchExists(ctx context.Context, runner gitRunner, branch string) (bool, error) {
	output, err := runner.run(ctx, "for-each-ref", "--format=%(refname)", "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "refs/heads/"+branch, nil
}

func requirePathAbsent(path, existsMessage, inspectMessage string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("%s: %s", existsMessage, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s: %w", inspectMessage, err)
	}
	return nil
}

func (m *Manager) checkpoint(name string) error {
	if m.createCheckpoint == nil {
		return nil
	}
	return m.createCheckpoint(name)
}

func (m *Manager) release(lock *Lock) error {
	if m.releaseLock != nil {
		return m.releaseLock(lock)
	}
	return lock.Release()
}

func createFailure(id ID, resources createResources, cause error) error {
	commands := []string{
		"git branch --list " + shellQuote(resources.branch),
		"git worktree list --porcelain",
	}
	if resources.worktreeCreated {
		commands = append(commands, "git -C "+shellQuote(resources.worktreePath)+" status --short")
	}
	commands = append(commands, "git worktree prune --dry-run")
	if resources.worktreeCreated {
		commands = append(commands, "git worktree remove "+shellQuote(resources.worktreePath))
	}
	if resources.branchCreated {
		commands = append(commands, "git branch -d "+shellQuote(resources.branch))
	}
	return fmt.Errorf("create experiment %s: %w; created branch=%t %s, worktree=%t %s; inspect and recover without force:\n%s", id.String(), cause, resources.branchCreated, shellQuote(resources.branch), resources.worktreeCreated, shellQuote(resources.worktreePath), strings.Join(commands, "\n"))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (m *Manager) enforceWriterLimit(ctx context.Context, maximum int) error {
	runner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}
	refs, err := runner.run(ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads/exp", "refs/heads/integrate")
	if err != nil {
		return err
	}
	active := 0
	for _, branch := range strings.Fields(refs) {
		prefix := "exp/"
		if strings.HasPrefix(branch, "integrate/") {
			prefix = "integrate/"
		}
		id, err := ParseID(strings.TrimPrefix(branch, prefix))
		if err != nil {
			continue
		}
		data, err := runner.run(ctx, "show", branch+":"+filepath.ToSlash(filepath.Join(id.RecordDir(), "state.json")))
		if err != nil {
			return fmt.Errorf("read active writer state for %s: %w", branch, err)
		}
		state, err := decodeState(branch, []byte(data))
		if err != nil {
			return err
		}
		if state.Status == StatusRunning || state.Status == StatusIntegrating {
			active++
		}
	}
	if active >= maximum {
		return fmt.Errorf("maximum active writers reached (%d)", maximum)
	}
	return nil
}

func relativePath(base, target string) string {
	relative, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return relative
}

func formatSeeds(seeds []uint64) string {
	values := make([]string, len(seeds))
	for i, seed := range seeds {
		values[i] = strconv.FormatUint(seed, 10)
	}
	return strings.Join(values, ",")
}
