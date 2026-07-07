package runs_test

// service_test.go exercises runs.Service business logic.
//
// Since runs.Repository is a concrete pgx struct we mirror its methods behind a
// locally-defined interface and wire them to a testRunService that replicates
// the production logic, giving us full control of the store without a DB.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agentops/runtime/internal/runs"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// In-memory run store
// ---------------------------------------------------------------------------

var errRunNotFound = errors.New("run not found")

type runStore interface {
	Create(ctx context.Context, projectID uuid.UUID, goal string, maxSteps int) (*runs.Run, error)
	GetByID(ctx context.Context, runID uuid.UUID) (*runs.Run, error)
	List(ctx context.Context, projectID uuid.UUID) ([]runs.Run, error)
	UpdateStatus(ctx context.Context, runID uuid.UUID, status runs.RunStatus) error
	ListSteps(ctx context.Context, runID uuid.UUID) ([]runs.Step, error)
}

type memRunStore struct {
	runs  []runs.Run
	steps []runs.Step
}

func (m *memRunStore) Create(_ context.Context, projectID uuid.UUID, goal string, maxSteps int) (*runs.Run, error) {
	r := runs.Run{
		ID:        uuid.New(),
		ProjectID: projectID,
		Goal:      goal,
		Status:    runs.StatusCreated,
		MaxSteps:  maxSteps,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.runs = append(m.runs, r)
	return &r, nil
}

func (m *memRunStore) GetByID(_ context.Context, runID uuid.UUID) (*runs.Run, error) {
	for i := range m.runs {
		if m.runs[i].ID == runID {
			cp := m.runs[i]
			return &cp, nil
		}
	}
	return nil, errRunNotFound
}

func (m *memRunStore) List(_ context.Context, projectID uuid.UUID) ([]runs.Run, error) {
	var out []runs.Run
	for _, r := range m.runs {
		if r.ProjectID == projectID {
			out = append(out, r)
		}
	}
	if out == nil {
		out = []runs.Run{}
	}
	return out, nil
}

func (m *memRunStore) UpdateStatus(_ context.Context, runID uuid.UUID, status runs.RunStatus) error {
	for i := range m.runs {
		if m.runs[i].ID == runID {
			m.runs[i].Status = status
			return nil
		}
	}
	return errRunNotFound
}

func (m *memRunStore) ListSteps(_ context.Context, runID uuid.UUID) ([]runs.Step, error) {
	var out []runs.Step
	for _, s := range m.steps {
		if s.RunID == runID {
			out = append(out, s)
		}
	}
	if out == nil {
		out = []runs.Step{}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Thin service that mirrors runs.Service logic using the interface
// ---------------------------------------------------------------------------

type testRunService struct {
	store runStore
}

func (s *testRunService) Create(ctx context.Context, input runs.CreateRunInput) (*runs.Run, error) {
	projectID, err := uuid.Parse(input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}
	if input.MaxSteps <= 0 {
		input.MaxSteps = 20
	}
	return s.store.Create(ctx, projectID, input.Goal, input.MaxSteps)
}

func (s *testRunService) Get(ctx context.Context, runID uuid.UUID) (*runs.Run, error) {
	return s.store.GetByID(ctx, runID)
}

func (s *testRunService) List(ctx context.Context, projectID uuid.UUID) ([]runs.Run, error) {
	return s.store.List(ctx, projectID)
}

func (s *testRunService) ListSteps(ctx context.Context, runID uuid.UUID) ([]runs.Step, error) {
	return s.store.ListSteps(ctx, runID)
}

func (s *testRunService) Cancel(ctx context.Context, runID uuid.UUID) error {
	run, err := s.store.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}
	if run.IsTerminal() {
		return fmt.Errorf("run is already in terminal state %s", run.Status)
	}
	return s.store.UpdateStatus(ctx, runID, runs.StatusCancelled)
}

func (s *testRunService) Resume(ctx context.Context, runID uuid.UUID) error {
	run, err := s.store.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}
	if run.Status != runs.StatusFailed && run.Status != runs.StatusWaitingForApproval {
		return fmt.Errorf("cannot resume run in status %s", run.Status)
	}
	return s.store.UpdateStatus(ctx, runID, runs.StatusRunning)
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestRunSvc() (*testRunService, *memRunStore) {
	store := &memRunStore{}
	return &testRunService{store: store}, store
}

func createRunWithStatus(t *testing.T, svc *testRunService, store *memRunStore, status runs.RunStatus) *runs.Run {
	t.Helper()
	projectID := uuid.New()
	r, err := svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: projectID.String(),
		Goal:      "test goal",
		MaxSteps:  10,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	// Manually override status in the store to simulate a run that has progressed.
	for i := range store.runs {
		if store.runs[i].ID == r.ID {
			store.runs[i].Status = status
		}
	}
	// Re-fetch so the returned pointer has the updated status.
	updated, _ := svc.Get(context.Background(), r.ID)
	return updated
}

// ---------------------------------------------------------------------------
// Create tests
// ---------------------------------------------------------------------------

func TestRunServiceCreate_ValidInput(t *testing.T) {
	svc, _ := newTestRunSvc()
	projectID := uuid.New()
	r, err := svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: projectID.String(),
		Goal:      "write a report",
		MaxSteps:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if r.ProjectID != projectID {
		t.Errorf("projectID: got %v, want %v", r.ProjectID, projectID)
	}
	if r.Goal != "write a report" {
		t.Errorf("goal: got %q, want %q", r.Goal, "write a report")
	}
	if r.MaxSteps != 10 {
		t.Errorf("maxSteps: got %d, want 10", r.MaxSteps)
	}
	if r.Status != runs.StatusCreated {
		t.Errorf("initial status: got %q, want %q", r.Status, runs.StatusCreated)
	}
}

func TestRunServiceCreate_InvalidProjectID_ReturnsError(t *testing.T) {
	svc, _ := newTestRunSvc()
	invalidIDs := []string{"", "not-a-uuid", "12345", "abc-def"}
	for _, id := range invalidIDs {
		id := id
		t.Run(id, func(t *testing.T) {
			_, err := svc.Create(context.Background(), runs.CreateRunInput{
				ProjectID: id,
				Goal:      "test",
			})
			if err == nil {
				t.Errorf("projectId %q should fail UUID parse but no error returned", id)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid projectId") {
				t.Errorf("error should mention 'invalid projectId', got: %v", err)
			}
		})
	}
}

func TestRunServiceCreate_MaxStepsZero_DefaultsTwenty(t *testing.T) {
	svc, _ := newTestRunSvc()
	r, err := svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: uuid.New().String(),
		Goal:      "do something",
		MaxSteps:  0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MaxSteps != 20 {
		t.Errorf("maxSteps should default to 20 when 0, got %d", r.MaxSteps)
	}
}

func TestRunServiceCreate_NegativeMaxSteps_DefaultsTwenty(t *testing.T) {
	svc, _ := newTestRunSvc()
	r, err := svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: uuid.New().String(),
		Goal:      "do something",
		MaxSteps:  -5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MaxSteps != 20 {
		t.Errorf("maxSteps should default to 20 when negative, got %d", r.MaxSteps)
	}
}

func TestRunServiceCreate_PositiveMaxSteps_Respected(t *testing.T) {
	svc, _ := newTestRunSvc()
	r, err := svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: uuid.New().String(),
		Goal:      "do something",
		MaxSteps:  50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MaxSteps != 50 {
		t.Errorf("maxSteps: got %d, want 50", r.MaxSteps)
	}
}

// ---------------------------------------------------------------------------
// Cancel tests
// ---------------------------------------------------------------------------

func TestRunServiceCancel_CreatedRun_Succeeds(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusCreated)

	if err := svc.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel CREATED run: unexpected error: %v", err)
	}
	updated, _ := svc.Get(context.Background(), run.ID)
	if updated.Status != runs.StatusCancelled {
		t.Errorf("status after cancel: got %q, want %q", updated.Status, runs.StatusCancelled)
	}
}

func TestRunServiceCancel_RunningRun_Succeeds(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusRunning)

	if err := svc.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel RUNNING run: unexpected error: %v", err)
	}
}

func TestRunServiceCancel_PlanningRun_Succeeds(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusPlanning)

	if err := svc.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel PLANNING run: unexpected error: %v", err)
	}
}

func TestRunServiceCancel_WaitingForApprovalRun_Succeeds(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusWaitingForApproval)

	if err := svc.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel WAITING_FOR_APPROVAL run: unexpected error: %v", err)
	}
}

func TestRunServiceCancel_CompletedRun_ReturnsTerminalError(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusCompleted)

	err := svc.Cancel(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected error cancelling COMPLETED run, got nil")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("error should mention 'terminal', got: %v", err)
	}
}

func TestRunServiceCancel_FailedRun_ReturnsTerminalError(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusFailed)

	err := svc.Cancel(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected error cancelling FAILED run, got nil")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("error should mention 'terminal', got: %v", err)
	}
}

func TestRunServiceCancel_CancelledRun_ReturnsTerminalError(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusCancelled)

	err := svc.Cancel(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected error cancelling already-CANCELLED run, got nil")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("error should mention 'terminal', got: %v", err)
	}
}

func TestRunServiceCancel_NonExistentRun_ReturnsError(t *testing.T) {
	svc, _ := newTestRunSvc()
	err := svc.Cancel(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent run, got nil")
	}
}

// ---------------------------------------------------------------------------
// Resume tests
// ---------------------------------------------------------------------------

func TestRunServiceResume_FailedRun_Succeeds(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusFailed)

	if err := svc.Resume(context.Background(), run.ID); err != nil {
		t.Fatalf("resume FAILED run: unexpected error: %v", err)
	}
	updated, _ := svc.Get(context.Background(), run.ID)
	if updated.Status != runs.StatusRunning {
		t.Errorf("status after resume: got %q, want %q", updated.Status, runs.StatusRunning)
	}
}

func TestRunServiceResume_WaitingForApprovalRun_Succeeds(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusWaitingForApproval)

	if err := svc.Resume(context.Background(), run.ID); err != nil {
		t.Fatalf("resume WAITING_FOR_APPROVAL run: unexpected error: %v", err)
	}
	updated, _ := svc.Get(context.Background(), run.ID)
	if updated.Status != runs.StatusRunning {
		t.Errorf("status after resume: got %q, want %q", updated.Status, runs.StatusRunning)
	}
}

func TestRunServiceResume_RunningRun_ReturnsError(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusRunning)

	err := svc.Resume(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected error resuming RUNNING run, got nil")
	}
	if !strings.Contains(err.Error(), "cannot resume") {
		t.Errorf("error should mention 'cannot resume', got: %v", err)
	}
}

func TestRunServiceResume_CompletedRun_ReturnsError(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusCompleted)

	err := svc.Resume(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected error resuming COMPLETED run, got nil")
	}
}

func TestRunServiceResume_CreatedRun_ReturnsError(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusCreated)

	err := svc.Resume(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected error resuming CREATED run, got nil")
	}
}

func TestRunServiceResume_CancelledRun_ReturnsError(t *testing.T) {
	svc, store := newTestRunSvc()
	run := createRunWithStatus(t, svc, store, runs.StatusCancelled)

	err := svc.Resume(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected error resuming CANCELLED run, got nil")
	}
}

func TestRunServiceResume_NonExistentRun_ReturnsError(t *testing.T) {
	svc, _ := newTestRunSvc()
	err := svc.Resume(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent run, got nil")
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestRunServiceList_ReturnsEmpty_WhenNoRuns(t *testing.T) {
	svc, _ := newTestRunSvc()
	result, err := svc.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("List should return empty slice, not nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 runs, got %d", len(result))
	}
}

func TestRunServiceList_ReturnsOnlyProjectRuns(t *testing.T) {
	svc, _ := newTestRunSvc()
	projectA := uuid.New()
	projectB := uuid.New()

	for i := 0; i < 3; i++ {
		_, _ = svc.Create(context.Background(), runs.CreateRunInput{
			ProjectID: projectA.String(),
			Goal:      fmt.Sprintf("goal-%d", i),
		})
	}
	_, _ = svc.Create(context.Background(), runs.CreateRunInput{
		ProjectID: projectB.String(),
		Goal:      "other goal",
	})

	result, err := svc.List(context.Background(), projectA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 runs for projectA, got %d", len(result))
	}
	for _, r := range result {
		if r.ProjectID != projectA {
			t.Errorf("run %v does not belong to projectA", r.ID)
		}
	}
}
