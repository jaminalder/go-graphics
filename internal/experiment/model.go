package experiment

import (
	"fmt"
	"time"
)

// Kind distinguishes ordinary experiments from semantic integrations.
type Kind string

const (
	// KindExperiment is an ordinary experiment.
	KindExperiment Kind = "experiment"
	// KindIntegration combines selected behavior from source experiments.
	KindIntegration Kind = "integration"
)

// Status is a lifecycle state for an experiment record.
type Status string

const (
	StatusCreated            Status = "created"
	StatusRunning            Status = "running"
	StatusReviewPending      Status = "review-pending"
	StatusRevisionRequested  Status = "revision-requested"
	StatusIntegrationPending Status = "integration-pending"
	StatusIntegrating        Status = "integrating"
	StatusMergeReady         Status = "merge-ready"
	StatusMerged             Status = "merged"
	StatusDiscarded          Status = "discarded"
	StatusFailed             Status = "failed"
)

var transitions = map[Status][]Status{
	StatusCreated:            {StatusRunning, StatusDiscarded, StatusFailed},
	StatusRunning:            {StatusReviewPending, StatusRevisionRequested, StatusDiscarded, StatusFailed},
	StatusReviewPending:      {StatusRevisionRequested, StatusIntegrationPending, StatusMergeReady, StatusDiscarded, StatusFailed},
	StatusRevisionRequested:  {StatusRunning, StatusDiscarded, StatusFailed},
	StatusIntegrationPending: {StatusIntegrating, StatusDiscarded, StatusFailed},
	StatusIntegrating:        {StatusReviewPending, StatusMergeReady, StatusDiscarded, StatusFailed},
	StatusMergeReady:         {StatusMerged, StatusRevisionRequested, StatusFailed},
	StatusMerged:             nil,
	StatusDiscarded:          nil,
	StatusFailed:             {StatusRevisionRequested, StatusRunning, StatusDiscarded},
}

// ParseStatus parses a canonical lifecycle status.
func ParseStatus(value string) (Status, error) {
	status := Status(value)
	if _, ok := transitions[status]; !ok {
		return "", fmt.Errorf("unknown experiment status %q", value)
	}
	return status, nil
}

// CanTransition reports whether the approved lifecycle permits the state change.
func CanTransition(from, to Status) bool {
	allowed, ok := transitions[from]
	if !ok {
		return false
	}
	for _, status := range allowed {
		if status == to {
			return true
		}
	}
	return false
}

// AllowedTransitions returns a copy of the approved destinations from status.
func AllowedTransitions(status Status) []Status {
	allowed, ok := transitions[status]
	if !ok {
		return nil
	}
	return append([]Status(nil), allowed...)
}

var stages = map[string]struct{}{
	"geometry":            {},
	"layout":              {},
	"subdivision":         {},
	"field-construction":  {},
	"coloring":            {},
	"shading":             {},
	"hatching":            {},
	"smoothing":           {},
	"material-evaluation": {},
	"painting":            {},
	"sampling":            {},
	"rendering":           {},
	"lifecycle":           {},
}

// ValidateStage rejects stages outside the canonical workflow vocabulary.
func ValidateStage(stage string) error {
	if _, ok := stages[stage]; !ok {
		return fmt.Errorf("invalid experiment stage %q", stage)
	}
	return nil
}

// Worker records the tool and optional session assigned to an experiment.
type Worker struct {
	Tool    string  `json:"tool"`
	Session *string `json:"session"`
}

// Source pins one source experiment used by a semantic integration.
type Source struct {
	ID     string `json:"id"`
	Commit string `json:"commit"`
}

// Verification records the latest explicit verification run.
type Verification struct {
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	Commit    string     `json:"commit,omitempty"`
	Command   string     `json:"command,omitempty"`
	Passed    bool       `json:"passed"`
}

// State is the authoritative JSON record for an active experiment.
type State struct {
	SchemaVersion  int          `json:"schema_version"`
	ID             string       `json:"id"`
	Kind           Kind         `json:"kind"`
	Branch         string       `json:"branch"`
	Worktree       string       `json:"worktree"`
	BaseBranch     string       `json:"base_branch"`
	BaseCommit     string       `json:"base_commit"`
	BaseExperiment string       `json:"base_experiment,omitempty"`
	Status         Status       `json:"status"`
	Stage          string       `json:"stage"`
	Worker         Worker       `json:"worker"`
	Seeds          []uint64     `json:"seeds"`
	Profile        string       `json:"profile"`
	Output         string       `json:"output"`
	Sources        []Source     `json:"sources,omitempty"`
	Verification   Verification `json:"verification"`
	LifecycleOnly  bool         `json:"lifecycle_only"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}
