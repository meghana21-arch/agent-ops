package agents

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type CreateAgentConfigInput struct {
	ProjectID      uuid.UUID
	Name           string
	Model          string
	SystemPrompt   string
	AllowedTools   []string
	ApprovalPolicy ApprovalPolicy
	MaxSteps       int
	MaxCostUSD     float64
	MaxRetries     int
}

func (r *Repository) Create(ctx context.Context, in CreateAgentConfigInput) (*AgentConfig, error) {
	toolsJSON, err := json.Marshal(in.AllowedTools)
	if err != nil {
		return nil, err
	}
	policyJSON, err := json.Marshal(in.ApprovalPolicy)
	if err != nil {
		return nil, err
	}
	model := in.Model
	if model == "" {
		model = "claude-sonnet-4-6-20250620"
	}
	maxSteps := in.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 20
	}
	maxCost := in.MaxCostUSD
	if maxCost <= 0 {
		maxCost = 1.0
	}
	maxRetries := in.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	cfg := &AgentConfig{}
	err = r.db.QueryRow(ctx,
		`INSERT INTO agent_configs
		   (project_id, name, model, system_prompt, allowed_tools_json, approval_policy_json, max_steps, max_cost_usd, max_retries)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, project_id, name, model, system_prompt, allowed_tools_json, approval_policy_json,
		           max_steps, max_cost_usd, max_retries, created_at, updated_at`,
		in.ProjectID, in.Name, model, in.SystemPrompt, toolsJSON, policyJSON,
		maxSteps, maxCost, maxRetries,
	).Scan(
		&cfg.ID, &cfg.ProjectID, &cfg.Name, &cfg.Model, &cfg.SystemPrompt,
		&cfg.allowedToolsJSON, &cfg.approvalPolicyJSON,
		&cfg.MaxSteps, &cfg.MaxCostUSD, &cfg.MaxRetries, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cfg, cfg.decode()
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*AgentConfig, error) {
	cfg := &AgentConfig{}
	err := r.db.QueryRow(ctx,
		`SELECT id, project_id, name, model, system_prompt, allowed_tools_json, approval_policy_json,
		        max_steps, max_cost_usd, max_retries, created_at, updated_at
		 FROM agent_configs WHERE id = $1`,
		id,
	).Scan(
		&cfg.ID, &cfg.ProjectID, &cfg.Name, &cfg.Model, &cfg.SystemPrompt,
		&cfg.allowedToolsJSON, &cfg.approvalPolicyJSON,
		&cfg.MaxSteps, &cfg.MaxCostUSD, &cfg.MaxRetries, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cfg, cfg.decode()
}

func (r *Repository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]AgentConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, project_id, name, model, system_prompt, allowed_tools_json, approval_policy_json,
		        max_steps, max_cost_usd, max_retries, created_at, updated_at
		 FROM agent_configs WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AgentConfig
	for rows.Next() {
		var cfg AgentConfig
		if err := rows.Scan(
			&cfg.ID, &cfg.ProjectID, &cfg.Name, &cfg.Model, &cfg.SystemPrompt,
			&cfg.allowedToolsJSON, &cfg.approvalPolicyJSON,
			&cfg.MaxSteps, &cfg.MaxCostUSD, &cfg.MaxRetries, &cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := cfg.decode(); err != nil {
			return nil, err
		}
		result = append(result, cfg)
	}
	if result == nil {
		result = []AgentConfig{}
	}
	return result, rows.Err()
}
