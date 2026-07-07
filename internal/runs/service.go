package runs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type CreateRunInput struct {
	ProjectID string `json:"projectId" binding:"required"`
	Goal      string `json:"goal" binding:"required"`
	MaxSteps  int    `json:"maxSteps"`
}

func (s *Service) Create(ctx context.Context, input CreateRunInput) (*Run, error) {
	projectID, err := uuid.Parse(input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}
	if input.MaxSteps <= 0 {
		input.MaxSteps = 20
	}
	return s.repo.Create(ctx, projectID, input.Goal, input.MaxSteps)
}

func (s *Service) Get(ctx context.Context, runID uuid.UUID) (*Run, error) {
	return s.repo.GetByID(ctx, runID)
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]Run, error) {
	return s.repo.List(ctx, projectID)
}

func (s *Service) ListSteps(ctx context.Context, runID uuid.UUID) ([]Step, error) {
	return s.repo.ListSteps(ctx, runID)
}

func (s *Service) Cancel(ctx context.Context, runID uuid.UUID) error {
	run, err := s.repo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}
	if run.IsTerminal() {
		return fmt.Errorf("run is already in terminal state %s", run.Status)
	}
	return s.repo.UpdateStatus(ctx, runID, StatusCancelled)
}

func (s *Service) Resume(ctx context.Context, runID uuid.UUID) error {
	run, err := s.repo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}
	if run.Status != StatusFailed && run.Status != StatusWaitingForApproval {
		return fmt.Errorf("cannot resume run in status %s", run.Status)
	}
	return s.repo.UpdateStatus(ctx, runID, StatusRunning)
}
