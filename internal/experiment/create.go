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

// Create creates and initializes an ordinary experiment branch and sibling worktree.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (_ Created, retErr error) {
	id, err := ParseID(opts.Piece + "/" + opts.Name)
	if err != nil {
		return Created{}, err
	}
	globalLock, err := m.AcquireGlobalLock(ctx, "create "+id.String())
	if err != nil {
		return Created{}, err
	}
	defer func() { retErr = errors.Join(retErr, globalLock.Release()) }()
	idLock, err := m.AcquireExperimentLock(ctx, id, "create "+id.String())
	if err != nil {
		return Created{}, err
	}
	defer func() { retErr = errors.Join(retErr, idLock.Release()) }()

	if err := m.RequireCoordinator(); err != nil {
		return Created{}, err
	}
	runner := gitRunner{dir: m.CoordinatorRoot, env: m.gitEnv}
	currentBranch, err := runner.run(ctx, "branch", "--show-current")
	if err != nil {
		return Created{}, err
	}
	if strings.TrimSpace(currentBranch) != "master" {
		return Created{}, fmt.Errorf("create requires coordinator branch master; current branch is %q", strings.TrimSpace(currentBranch))
	}
	status, err := runner.run(ctx, "status", "--porcelain")
	if err != nil {
		return Created{}, err
	}
	if status != "" {
		return Created{}, fmt.Errorf("coordinator worktree is not clean: %s", strings.TrimSpace(status))
	}

	opts, err = normalizedCreateOptions(opts)
	if err != nil {
		return Created{}, err
	}
	branch := id.ExperimentBranch()
	worktreePath := id.WorktreePath(m.CoordinatorRoot)
	recordDir := id.RecordDir()
	coordinatorRecord := filepath.Join(m.CoordinatorRoot, recordDir)
	refsOutput, err := runner.run(ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads/exp", "refs/heads/integrate")
	if err != nil {
		return Created{}, err
	}
	refs := strings.Fields(refsOutput)
	for _, candidate := range []string{branch, id.IntegrationBranch()} {
		if slices.Contains(refs, candidate) {
			return Created{}, fmt.Errorf("experiment branch already exists: %s", candidate)
		}
	}
	worktreeOutput, err := runner.run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return Created{}, err
	}
	worktrees, err := parseWorktreeList(worktreeOutput)
	if err != nil {
		return Created{}, err
	}
	for _, worktree := range worktrees {
		if filepath.Clean(worktree.Path) == filepath.Clean(worktreePath) || worktree.Branch == "refs/heads/"+branch {
			return Created{}, fmt.Errorf("experiment worktree already registered: %s", worktree.Path)
		}
	}
	if _, err := os.Lstat(worktreePath); err == nil {
		return Created{}, fmt.Errorf("experiment worktree path already exists: %s", worktreePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Created{}, fmt.Errorf("inspect experiment worktree path: %w", err)
	}
	if _, err := os.Lstat(coordinatorRecord); err == nil {
		return Created{}, fmt.Errorf("experiment record already exists: %s", coordinatorRecord)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Created{}, fmt.Errorf("inspect experiment record: %w", err)
	}
	archiveRecord := filepath.Join(m.CoordinatorRoot, filepath.FromSlash(id.ArchiveDir()))
	if _, err := os.Lstat(archiveRecord); err == nil {
		return Created{}, fmt.Errorf("experiment record already exists: %s", archiveRecord)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Created{}, fmt.Errorf("inspect archived experiment record: %w", err)
	}
	if err := m.enforceWriterLimit(ctx, opts.MaxWriters); err != nil {
		return Created{}, err
	}

	baseBranch, baseCommit, err := m.resolveCreateBase(ctx, runner, opts)
	if err != nil {
		return Created{}, err
	}
	branchCreated := false
	worktreeCreated := false
	fail := func(cause error) error {
		if !branchCreated && !worktreeCreated {
			return cause
		}
		commands := []string{
			"git branch --list " + branch,
			"git worktree list --porcelain",
		}
		if worktreeCreated {
			commands = append(commands, "git -C "+strconv.Quote(worktreePath)+" status --short")
		}
		commands = append(commands,
			"git worktree prune --dry-run",
			"git worktree remove "+strconv.Quote(worktreePath),
			"git branch -d "+branch,
		)
		return fmt.Errorf("create experiment %s: %w; created branch=%t %q, worktree=%t %q; inspect and recover without force:\n%s", id.String(), cause, branchCreated, branch, worktreeCreated, worktreePath, strings.Join(commands, "\n"))
	}
	if _, err := runner.run(ctx, "branch", branch, baseCommit); err != nil {
		return Created{}, err
	}
	branchCreated = true
	if _, err := runner.run(ctx, "worktree", "add", worktreePath, branch); err != nil {
		return Created{}, fail(err)
	}
	worktreeCreated = true

	outputRelative := filepath.ToSlash(id.OutputDir())
	outputPath := filepath.Join(worktreePath, filepath.FromSlash(outputRelative))
	for _, dir := range []string{"baseline", "candidate", "metadata"} {
		if err := os.MkdirAll(filepath.Join(outputPath, dir), 0o755); err != nil {
			return Created{}, fail(fmt.Errorf("create output directory %s: %w", dir, err))
		}
	}
	recordPath := filepath.Join(worktreePath, filepath.FromSlash(recordDir))
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		return Created{}, fail(fmt.Errorf("create record directory: %w", err))
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
	if err := writeJSONAtomic(filepath.Join(recordPath, "state.json"), state); err != nil {
		return Created{}, fail(err)
	}
	if err := writeJSONAtomic(filepath.Join(recordPath, "favorites.json"), []any{}); err != nil {
		return Created{}, fail(err)
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
	if err := renderTemplate(filepath.Join(m.TemplatesRoot, "brief.md"), briefPath, templateData); err != nil {
		return Created{}, fail(err)
	}
	if err := renderTemplate(filepath.Join(m.TemplatesRoot, "result.md"), filepath.Join(recordPath, "result.md"), templateData); err != nil {
		return Created{}, fail(err)
	}
	if err := commitRecord(ctx, worktreePath, recordDir, "experiment: create "+id.String(), m.gitEnv); err != nil {
		return Created{}, fail(err)
	}

	instruction := "Work on experiment " + id.String() + " only.\n" +
		"Worktree: " + worktreePath + "\n" +
		"Branch: " + branch + "\n" +
		"Brief: " + briefPath + "\n" +
		"Operate only inside this worktree. Do not switch branches. Do not create or remove worktrees. Do not merge, rebase, or modify master. Do not work outside the assigned scope. Do not modify another experiment's files."
	return Created{State: state, WorktreePath: worktreePath, BriefPath: briefPath, OutputPath: outputPath, WorkerInstruction: instruction}, nil
}

func normalizedCreateOptions(opts CreateOptions) (CreateOptions, error) {
	if opts.BaseBranch == "" {
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
	baseBranch := opts.BaseBranch
	if opts.BaseExperiment != "" {
		parentID, err := ParseID(opts.BaseExperiment)
		if err != nil {
			return "", "", fmt.Errorf("invalid base experiment: %w", err)
		}
		parentBranch := parentID.ExperimentBranch()
		parentCommitOutput, err := runner.run(ctx, "rev-parse", "--verify", parentBranch+"^{commit}")
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
		baseBranch = parent.Branch
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
	commit, err := runner.run(ctx, "rev-parse", "--verify", baseBranch+"^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("resolve base branch %s: %w", baseBranch, err)
	}
	return baseBranch, strings.TrimSpace(commit), nil
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
