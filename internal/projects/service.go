package projects

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

type CreateProjectInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Environment string `json:"environment"`
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, input CreateProjectInput) (*Project, error) {
	if input.Environment == "" {
		input.Environment = "development"
	}
	switch input.Environment {
	case "development", "staging", "production":
	default:
		return nil, fmt.Errorf("invalid environment %q: must be development, staging, or production", input.Environment)
	}
	return s.repo.Create(ctx, orgID, input.Name, input.Description, input.Environment)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	return s.repo.List(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Project, error) {
	return s.repo.GetByID(ctx, id)
}
