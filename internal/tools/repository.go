package tools

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

type UpsertInput struct {
	ProjectID uuid.UUID
	ToolName  string
	Enabled   bool
	Config    json.RawMessage
}

func (r *Repository) Upsert(ctx context.Context, in UpsertInput) (*ProjectTool, error) {
	cfg := in.Config
	if cfg == nil {
		cfg = json.RawMessage("{}")
	}
	def, _ := NewRegistry().Get(in.ToolName)
	riskLevel := RiskLow
	if def != nil {
		riskLevel = def.Risk
	}

	pt := &ProjectTool{}
	var rawCfg []byte
	err := r.db.QueryRow(ctx,
		`INSERT INTO project_tools (project_id, tool_name, enabled, config_json)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (project_id, tool_name)
		 DO UPDATE SET enabled = EXCLUDED.enabled, config_json = EXCLUDED.config_json, updated_at = NOW()
		 RETURNING id, project_id, tool_name, enabled, config_json, created_at, updated_at`,
		in.ProjectID, in.ToolName, in.Enabled, cfg,
	).Scan(&pt.ID, &pt.ProjectID, &pt.ToolName, &pt.Enabled, &rawCfg, &pt.CreatedAt, &pt.UpdatedAt)
	if err != nil {
		return nil, err
	}
	pt.Config = rawCfg
	pt.RiskLevel = riskLevel
	return pt, nil
}

func (r *Repository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]ProjectTool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, project_id, tool_name, enabled, config_json, created_at, updated_at
		 FROM project_tools WHERE project_id = $1 ORDER BY tool_name`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reg := NewRegistry()
	var result []ProjectTool
	for rows.Next() {
		var pt ProjectTool
		var rawCfg []byte
		if err := rows.Scan(&pt.ID, &pt.ProjectID, &pt.ToolName, &pt.Enabled, &rawCfg, &pt.CreatedAt, &pt.UpdatedAt); err != nil {
			return nil, err
		}
		pt.Config = rawCfg
		if def, ok := reg.Get(pt.ToolName); ok {
			pt.RiskLevel = def.Risk
		}
		result = append(result, pt)
	}
	if result == nil {
		result = []ProjectTool{}
	}
	return result, rows.Err()
}
