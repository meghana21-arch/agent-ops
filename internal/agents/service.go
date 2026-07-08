package agents

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

type CreateInput struct {
	ProjectID      string         `json:"projectId" binding:"required"`
	Name           string         `json:"name" binding:"required"`
	Model          string         `json:"model"`
	SystemPrompt   string         `json:"systemPrompt"`
	AllowedTools   []string       `json:"allowedTools"`
	ApprovalPolicy ApprovalPolicy `json:"approvalPolicy"`
	MaxSteps       int            `json:"maxSteps"`
	MaxCostUSD     float64        `json:"maxCostUsd"`
	MaxRetries     int            `json:"maxRetries"`
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*AgentConfig, error) {
	pid, err := uuid.Parse(in.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("invalid projectId: %w", err)
	}
	if in.AllowedTools == nil {
		in.AllowedTools = []string{}
	}
	return s.repo.Create(ctx, CreateAgentConfigInput{
		ProjectID:      pid,
		Name:           in.Name,
		Model:          in.Model,
		SystemPrompt:   in.SystemPrompt,
		AllowedTools:   in.AllowedTools,
		ApprovalPolicy: in.ApprovalPolicy,
		MaxSteps:       in.MaxSteps,
		MaxCostUSD:     in.MaxCostUSD,
		MaxRetries:     in.MaxRetries,
	})
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*AgentConfig, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]AgentConfig, error) {
	return s.repo.ListByProject(ctx, projectID)
}
