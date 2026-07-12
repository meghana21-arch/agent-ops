package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
)

type Service struct {
	registry *Registry
	executor *Executor
	repo     *Repository
}

func NewService(workspaceDir string, repo *Repository) *Service {
	return &Service{
		registry: NewRegistry(),
		executor: NewExecutor(workspaceDir),
		repo:     repo,
	}
}

// Execute validates, policy-checks, and runs a tool.
// Returns ExecuteResult.NeedsApproval=true if the tool requires human sign-off.
func (s *Service) Execute(ctx context.Context, projectID uuid.UUID, toolName string, input json.RawMessage, policy ApprovalPolicy) (*ExecuteResult, error) {
	def, ok := s.registry.Get(toolName)
	if !ok {
		errStr := fmt.Sprintf("unknown tool: %s", toolName)
		return &ExecuteResult{Error: &errStr}, nil
	}

	pr := CheckPolicy(def.Risk, policy)
	if pr.NeedsApproval {
		return &ExecuteResult{NeedsApproval: true}, nil
	}
	if pr.AuditLog {
		log.Printf("[audit] project=%s tool=%s risk=%s", projectID, toolName, def.Risk)
	}

	output, err := s.executor.Execute(ctx, toolName, input)
	if err != nil {
		errStr := err.Error()
		return &ExecuteResult{Error: &errStr}, nil
	}
	return &ExecuteResult{Output: output}, nil
}

// Register enables a tool for a project (upsert).
func (s *Service) Register(ctx context.Context, projectID uuid.UUID, toolName string, enabled bool, config json.RawMessage) (*ProjectTool, error) {
	if _, ok := s.registry.Get(toolName); !ok {
		return nil, fmt.Errorf("unknown tool %q — not a built-in tool", toolName)
	}
	return s.repo.Upsert(ctx, UpsertInput{
		ProjectID: projectID,
		ToolName:  toolName,
		Enabled:   enabled,
		Config:    config,
	})
}

// List returns registered tools for a project, merged with all built-ins.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]ProjectTool, error) {
	registered, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Index registered tools by name
	byName := make(map[string]ProjectTool, len(registered))
	for _, pt := range registered {
		byName[pt.ToolName] = pt
	}

	// Return all built-ins, merging DB state where present
	all := s.registry.All()
	result := make([]ProjectTool, 0, len(all))
	for _, def := range all {
		if pt, ok := byName[def.Name]; ok {
			result = append(result, pt)
		} else {
			result = append(result, ProjectTool{
				ProjectID: projectID,
				ToolName:  def.Name,
				RiskLevel: def.Risk,
				Enabled:   false,
				Config:    json.RawMessage("{}"),
			})
		}
	}
	return result, nil
}

// Defs returns all built-in tool definitions (no DB required).
func (s *Service) Defs() []*ToolDef {
	return s.registry.All()
}
