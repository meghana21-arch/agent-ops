package runs_test

import (
	"testing"

	"github.com/agentops/runtime/internal/runs"
)

func TestRunIsTerminal(t *testing.T) {
	tests := []struct {
		status   runs.RunStatus
		terminal bool
	}{
		{runs.StatusCreated, false},
		{runs.StatusPlanning, false},
		{runs.StatusRunning, false},
		{runs.StatusWaitingForApproval, false},
		{runs.StatusRetrying, false},
		{runs.StatusCompleted, true},
		{runs.StatusFailed, true},
		{runs.StatusCancelled, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.status), func(t *testing.T) {
			run := &runs.Run{Status: tt.status}
			if got := run.IsTerminal(); got != tt.terminal {
				t.Errorf("IsTerminal() for status %q = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}

// TestRunStatusConstants ensures all expected constants are defined and have the
// correct string values. If a constant is renamed or misspelled the test fails.
func TestRunStatusConstants(t *testing.T) {
	want := map[runs.RunStatus]string{
		runs.StatusCreated:            "CREATED",
		runs.StatusPlanning:           "PLANNING",
		runs.StatusRunning:            "RUNNING",
		runs.StatusWaitingForApproval: "WAITING_FOR_APPROVAL",
		runs.StatusRetrying:           "RETRYING",
		runs.StatusCompleted:          "COMPLETED",
		runs.StatusFailed:             "FAILED",
		runs.StatusCancelled:          "CANCELLED",
	}
	for status, wantStr := range want {
		if string(status) != wantStr {
			t.Errorf("constant value mismatch: got %q, want %q", string(status), wantStr)
		}
	}
}

// TestStepStatusConstants ensures StepStatus values are well-formed strings.
func TestStepStatusConstants(t *testing.T) {
	statuses := []struct {
		s    runs.StepStatus
		want string
	}{
		{runs.StepPending, "PENDING"},
		{runs.StepRunning, "RUNNING"},
		{runs.StepSucceeded, "SUCCEEDED"},
		{runs.StepFailed, "FAILED"},
		{runs.StepSkipped, "SKIPPED"},
		{runs.StepRequiresApproval, "REQUIRES_APPROVAL"},
	}
	for _, tt := range statuses {
		if string(tt.s) != tt.want {
			t.Errorf("StepStatus constant: got %q, want %q", string(tt.s), tt.want)
		}
	}
}

// TestStepTypeConstants ensures StepType values are well-formed strings.
func TestStepTypeConstants(t *testing.T) {
	types := []struct {
		s    runs.StepType
		want string
	}{
		{runs.StepTypePlan, "PLAN"},
		{runs.StepTypeToolCall, "TOOL_CALL"},
		{runs.StepTypeObservation, "OBSERVATION"},
		{runs.StepTypeVerification, "VERIFICATION"},
		{runs.StepTypeError, "ERROR"},
	}
	for _, tt := range types {
		if string(tt.s) != tt.want {
			t.Errorf("StepType constant: got %q, want %q", string(tt.s), tt.want)
		}
	}
}

// TestRunZeroValue verifies that a zero-value Run is non-terminal, which is the
// expected safe default for any newly allocated but not-yet-populated Run.
func TestRunZeroValue(t *testing.T) {
	var r runs.Run
	if r.IsTerminal() {
		t.Error("zero-value Run (empty status) must not be terminal")
	}
}

// TestIsTerminalExhaustive re-confirms each terminal and non-terminal bucket
// separately so failure messages are unambiguous.
func TestIsTerminalExhaustive(t *testing.T) {
	t.Run("non-terminal statuses", func(t *testing.T) {
		nonTerminal := []runs.RunStatus{
			runs.StatusCreated,
			runs.StatusPlanning,
			runs.StatusRunning,
			runs.StatusWaitingForApproval,
			runs.StatusRetrying,
		}
		for _, s := range nonTerminal {
			r := &runs.Run{Status: s}
			if r.IsTerminal() {
				t.Errorf("status %q should NOT be terminal", s)
			}
		}
	})

	t.Run("terminal statuses", func(t *testing.T) {
		terminal := []runs.RunStatus{
			runs.StatusCompleted,
			runs.StatusFailed,
			runs.StatusCancelled,
		}
		for _, s := range terminal {
			r := &runs.Run{Status: s}
			if !r.IsTerminal() {
				t.Errorf("status %q MUST be terminal", s)
			}
		}
	})
}
