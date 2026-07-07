package runs_test

// state_machine_test.go exercises the state transitions enforced by
// runs.Service.Cancel and runs.Service.Resume.
//
// The mock repo here (mockRunRepo) is intentionally minimal — it tracks only
// the single Run instance that is under test so the transition assertions are
// clear and deterministic.

import (
	"context"
	"testing"
	"time"

	"github.com/agentops/runtime/internal/runs"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Minimal mock repo — tracks one Run
// ---------------------------------------------------------------------------

type mockRunRepo struct {
	run *runs.Run
}

func (m *mockRunRepo) Create(_ context.Context, projectID uuid.UUID, goal string, maxSteps int) (*runs.Run, error) {
	r := &runs.Run{
		ID:        uuid.New(),
		ProjectID: projectID,
		Goal:      goal,
		Status:    runs.StatusCreated,
		MaxSteps:  maxSteps,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.run = r
	return r, nil
}

func (m *mockRunRepo) GetByID(_ context.Context, id uuid.UUID) (*runs.Run, error) {
	if m.run == nil || m.run.ID != id {
		return nil, errRunNotFound
	}
	cp := *m.run
	return &cp, nil
}

func (m *mockRunRepo) List(_ context.Context, _ uuid.UUID) ([]runs.Run, error) {
	if m.run == nil {
		return []runs.Run{}, nil
	}
	return []runs.Run{*m.run}, nil
}

func (m *mockRunRepo) UpdateStatus(_ context.Context, id uuid.UUID, status runs.RunStatus) error {
	if m.run == nil || m.run.ID != id {
		return errRunNotFound
	}
	m.run.Status = status
	return nil
}

func (m *mockRunRepo) ListSteps(_ context.Context, _ uuid.UUID) ([]runs.Step, error) {
	return []runs.Step{}, nil
}

// newRepoWithStatus creates a mockRunRepo that already holds a run at the
// specified status, skipping the CREATED initial state.
func newRepoWithStatus(status runs.RunStatus) (*mockRunRepo, *runs.Run) {
	r := &runs.Run{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Goal:      "state machine test",
		Status:    status,
		MaxSteps:  20,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return &mockRunRepo{run: r}, r
}

// newSmSvc creates a testRunService backed by the given store.
func newSmSvc(store runStore) *testRunService {
	return &testRunService{store: store}
}

// ---------------------------------------------------------------------------
// Valid Cancel transitions (non-terminal → CANCELLED)
// ---------------------------------------------------------------------------

func TestStateMachine_Cancel_ValidTransitions(t *testing.T) {
	cancelableStatuses := []runs.RunStatus{
		runs.StatusCreated,
		runs.StatusPlanning,
		runs.StatusRunning,
		runs.StatusWaitingForApproval,
		runs.StatusRetrying,
	}

	for _, status := range cancelableStatuses {
		status := status
		t.Run(string(status)+"→CANCELLED", func(t *testing.T) {
			repo, run := newRepoWithStatus(status)
			svc := newSmSvc(repo)
			if err := svc.Cancel(context.Background(), run.ID); err != nil {
				t.Errorf("Cancel from %s should succeed, got: %v", status, err)
			}
			if repo.run.Status != runs.StatusCancelled {
				t.Errorf("expected status CANCELLED after cancel, got %q", repo.run.Status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Invalid Cancel transitions (terminal → should return error)
// ---------------------------------------------------------------------------

func TestStateMachine_Cancel_InvalidTransitions(t *testing.T) {
	terminalStatuses := []runs.RunStatus{
		runs.StatusCompleted,
		runs.StatusFailed,
		runs.StatusCancelled,
	}

	for _, status := range terminalStatuses {
		status := status
		t.Run(string(status)+"→CANCELLED_blocked", func(t *testing.T) {
			repo, run := newRepoWithStatus(status)
			svc := newSmSvc(repo)
			err := svc.Cancel(context.Background(), run.ID)
			if err == nil {
				t.Errorf("Cancel from terminal status %s should return error", status)
			}
			// Status must not have changed.
			if repo.run.Status != status {
				t.Errorf("status should remain %q after failed cancel, got %q", status, repo.run.Status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Valid Resume transitions
// ---------------------------------------------------------------------------

func TestStateMachine_Resume_ValidTransitions(t *testing.T) {
	resumableStatuses := []struct {
		from runs.RunStatus
		to   runs.RunStatus
	}{
		{runs.StatusFailed, runs.StatusRunning},
		{runs.StatusWaitingForApproval, runs.StatusRunning},
	}

	for _, tt := range resumableStatuses {
		tt := tt
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			repo, run := newRepoWithStatus(tt.from)
			svc := newSmSvc(repo)
			if err := svc.Resume(context.Background(), run.ID); err != nil {
				t.Errorf("Resume from %s should succeed, got: %v", tt.from, err)
			}
			if repo.run.Status != tt.to {
				t.Errorf("expected status %q after resume, got %q", tt.to, repo.run.Status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Invalid Resume transitions
// ---------------------------------------------------------------------------

func TestStateMachine_Resume_InvalidTransitions(t *testing.T) {
	nonResumableStatuses := []runs.RunStatus{
		runs.StatusCreated,
		runs.StatusPlanning,
		runs.StatusRunning,
		runs.StatusRetrying,
		runs.StatusCompleted,
		runs.StatusCancelled,
	}

	for _, status := range nonResumableStatuses {
		status := status
		t.Run(string(status)+"→RUNNING_blocked", func(t *testing.T) {
			repo, run := newRepoWithStatus(status)
			svc := newSmSvc(repo)
			err := svc.Resume(context.Background(), run.ID)
			if err == nil {
				t.Errorf("Resume from %s should return error", status)
			}
			// Status must not have changed.
			if repo.run.Status != status {
				t.Errorf("status should remain %q after failed resume, got %q", status, repo.run.Status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Idempotency — Cancel an already-cancelled run should fail on second attempt
// ---------------------------------------------------------------------------

func TestStateMachine_DoubleCancel_Fails(t *testing.T) {
	repo, run := newRepoWithStatus(runs.StatusRunning)
	svc := newSmSvc(repo)

	if err := svc.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("first cancel: unexpected error: %v", err)
	}
	if err := svc.Cancel(context.Background(), run.ID); err == nil {
		t.Fatal("second cancel on already-CANCELLED run should fail, got nil")
	}
}

// ---------------------------------------------------------------------------
// Resume after cancel is blocked (CANCELLED is terminal)
// ---------------------------------------------------------------------------

func TestStateMachine_ResumeCancelledRun_Fails(t *testing.T) {
	repo, run := newRepoWithStatus(runs.StatusRunning)
	svc := newSmSvc(repo)

	_ = svc.Cancel(context.Background(), run.ID)

	if err := svc.Resume(context.Background(), run.ID); err == nil {
		t.Fatal("resume after cancel should fail, got nil")
	}
}

// ---------------------------------------------------------------------------
// Full lifecycle: CREATED → cancel → attempt resume (blocked)
// ---------------------------------------------------------------------------

func TestStateMachine_FullLifecycle_CreateCancelResumeBlocked(t *testing.T) {
	repo, run := newRepoWithStatus(runs.StatusCreated)
	svc := newSmSvc(repo)

	// Cancel succeeds
	if err := svc.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel CREATED: %v", err)
	}
	// Resume must fail — CANCELLED is terminal
	if err := svc.Resume(context.Background(), run.ID); err == nil {
		t.Fatal("resume after cancel should fail")
	}
}

// ---------------------------------------------------------------------------
// Resume → running → cancel: valid sequence
// ---------------------------------------------------------------------------

func TestStateMachine_ResumeFromFailed_ThenCancel(t *testing.T) {
	repo, run := newRepoWithStatus(runs.StatusFailed)
	svc := newSmSvc(repo)

	// Resume: FAILED → RUNNING
	if err := svc.Resume(context.Background(), run.ID); err != nil {
		t.Fatalf("resume FAILED: %v", err)
	}
	if repo.run.Status != runs.StatusRunning {
		t.Fatalf("expected RUNNING after resume, got %q", repo.run.Status)
	}

	// Cancel: RUNNING → CANCELLED
	if err := svc.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel RUNNING: %v", err)
	}
	if repo.run.Status != runs.StatusCancelled {
		t.Errorf("expected CANCELLED after cancel, got %q", repo.run.Status)
	}
}

// ---------------------------------------------------------------------------
// Table-driven summary: all known status transitions via Cancel/Resume
// ---------------------------------------------------------------------------

func TestStateMachine_AllTransitionsTable(t *testing.T) {
	type transition struct {
		from   runs.RunStatus
		action string // "cancel" | "resume"
		wantOK bool
	}

	tests := []transition{
		// Cancel transitions
		{runs.StatusCreated, "cancel", true},
		{runs.StatusPlanning, "cancel", true},
		{runs.StatusRunning, "cancel", true},
		{runs.StatusWaitingForApproval, "cancel", true},
		{runs.StatusRetrying, "cancel", true},
		{runs.StatusCompleted, "cancel", false},
		{runs.StatusFailed, "cancel", false},
		{runs.StatusCancelled, "cancel", false},

		// Resume transitions
		{runs.StatusFailed, "resume", true},
		{runs.StatusWaitingForApproval, "resume", true},
		{runs.StatusCreated, "resume", false},
		{runs.StatusPlanning, "resume", false},
		{runs.StatusRunning, "resume", false},
		{runs.StatusRetrying, "resume", false},
		{runs.StatusCompleted, "resume", false},
		{runs.StatusCancelled, "resume", false},
	}

	for _, tt := range tests {
		tt := tt
		name := string(tt.from) + "_" + tt.action
		t.Run(name, func(t *testing.T) {
			repo, run := newRepoWithStatus(tt.from)
			svc := newSmSvc(repo)

			var err error
			switch tt.action {
			case "cancel":
				err = svc.Cancel(context.Background(), run.ID)
			case "resume":
				err = svc.Resume(context.Background(), run.ID)
			default:
				t.Fatalf("unknown action %q", tt.action)
			}

			if tt.wantOK && err != nil {
				t.Errorf("[%s] expected success, got error: %v", name, err)
			}
			if !tt.wantOK && err == nil {
				t.Errorf("[%s] expected error, got nil", name)
			}
		})
	}
}
