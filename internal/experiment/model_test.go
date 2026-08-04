package experiment

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestParseStatusAcceptsEveryCanonicalStatus(t *testing.T) {
	t.Parallel()

	statuses := []Status{
		StatusCreated,
		StatusRunning,
		StatusReviewPending,
		StatusRevisionRequested,
		StatusIntegrationPending,
		StatusIntegrating,
		StatusMergeReady,
		StatusMerged,
		StatusDiscarded,
		StatusFailed,
	}
	for _, want := range statuses {
		t.Run(string(want), func(t *testing.T) {
			t.Parallel()
			got, err := ParseStatus(string(want))
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("ParseStatus() = %q, want %q", got, want)
			}
		})
	}
}

func TestParseStatusRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "review_pending", "complete", "Created"} {
		if _, err := ParseStatus(input); err == nil {
			t.Fatalf("ParseStatus(%q) succeeded", input)
		}
	}
}

func TestStateTransitionsMatchApprovedLifecycle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from Status
		want []Status
	}{
		{StatusCreated, []Status{StatusRunning, StatusDiscarded, StatusFailed}},
		{StatusRunning, []Status{StatusReviewPending, StatusRevisionRequested, StatusDiscarded, StatusFailed}},
		{StatusReviewPending, []Status{StatusRevisionRequested, StatusIntegrationPending, StatusMergeReady, StatusDiscarded, StatusFailed}},
		{StatusRevisionRequested, []Status{StatusRunning, StatusDiscarded, StatusFailed}},
		{StatusIntegrationPending, []Status{StatusIntegrating, StatusDiscarded, StatusFailed}},
		{StatusIntegrating, []Status{StatusReviewPending, StatusMergeReady, StatusDiscarded, StatusFailed}},
		{StatusMergeReady, []Status{StatusMerged, StatusRevisionRequested, StatusFailed}},
		{StatusFailed, []Status{StatusRevisionRequested, StatusRunning, StatusDiscarded}},
		{StatusMerged, nil},
		{StatusDiscarded, nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.from), func(t *testing.T) {
			t.Parallel()
			if got := AllowedTransitions(tt.from); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("AllowedTransitions(%q) = %v, want %v", tt.from, got, tt.want)
			}
			for _, to := range tt.want {
				if !CanTransition(tt.from, to) {
					t.Errorf("CanTransition(%q, %q) = false", tt.from, to)
				}
			}
		})
	}

	if CanTransition(StatusCreated, StatusMerged) {
		t.Fatal("created must not transition directly to merged")
	}
	if CanTransition(Status("unknown"), StatusRunning) || CanTransition(StatusCreated, Status("unknown")) {
		t.Fatal("unknown statuses must not transition")
	}
}

func TestValidateStageAcceptsOnlyCanonicalWorkflowStages(t *testing.T) {
	t.Parallel()

	valid := []string{
		"geometry", "layout", "subdivision", "field-construction", "coloring",
		"shading", "hatching", "smoothing", "material-evaluation", "painting",
		"sampling", "rendering", "lifecycle",
	}
	for _, stage := range valid {
		if err := ValidateStage(stage); err != nil {
			t.Errorf("ValidateStage(%q): %v", stage, err)
		}
	}
	for _, stage := range []string{"", "field_construction", "Field-construction", "material--evaluation", "geometry/layout", "render"} {
		if err := ValidateStage(stage); err == nil {
			t.Errorf("ValidateStage(%q) succeeded", stage)
		}
	}
}

func TestStateJSONUsesClearStableFieldNames(t *testing.T) {
	t.Parallel()

	session := "worker-7"
	checkedAt := time.Date(2026, time.August, 4, 12, 30, 0, 0, time.UTC)
	state := State{
		SchemaVersion:  1,
		ID:             "foam/hatching-by-depth",
		Kind:           KindExperiment,
		Branch:         "exp/foam/hatching-by-depth",
		Worktree:       "../go-graphics-worktrees/foam--hatching-by-depth",
		BaseBranch:     "master",
		BaseCommit:     "abc123",
		BaseExperiment: "foam/base",
		Status:         StatusRunning,
		Stage:          "hatching",
		Worker:         Worker{Tool: "opencode", Session: &session},
		Seeds:          []uint64{1, 2, 3},
		Profile:        "preview",
		Output:         "out/experiments/foam/hatching-by-depth",
		Sources:        []Source{{ID: "foam/source", Commit: "def456"}},
		Verification: Verification{
			CheckedAt: &checkedAt,
			Commit:    "abc123",
			Command:   "make check",
			Passed:    true,
		},
		LifecycleOnly: true,
		CreatedAt:     checkedAt,
		UpdatedAt:     checkedAt,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{
		"schema_version", "id", "kind", "branch", "worktree", "base_branch",
		"base_commit", "base_experiment", "status", "stage", "worker", "seeds",
		"profile", "output", "sources", "verification", "lifecycle_only",
		"created_at", "updated_at",
	}
	for _, key := range wantKeys {
		if _, ok := got[key]; !ok {
			t.Errorf("JSON is missing %q", key)
		}
	}
}
