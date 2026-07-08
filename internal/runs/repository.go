package runs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, projectID uuid.UUID, goal string, maxSteps int) (*Run, error) {
	run := &Run{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO workflow_runs (project_id, goal, max_steps)
		 VALUES ($1, $2, $3)
		 RETURNING id, project_id, agent_config_id, goal, status, current_step_index,
		           max_steps, total_tokens, total_cost_usd, started_at, completed_at, created_at, updated_at`,
		projectID, goal, maxSteps,
	).Scan(
		&run.ID, &run.ProjectID, &run.AgentConfigID, &run.Goal, &run.Status,
		&run.CurrentStepIdx, &run.MaxSteps, &run.TotalTokens, &run.TotalCostUSD,
		&run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
	)
	return run, err
}

func (r *Repository) GetByID(ctx context.Context, runID uuid.UUID) (*Run, error) {
	run := &Run{}
	err := r.db.QueryRow(ctx,
		`SELECT id, project_id, agent_config_id, goal, status, current_step_index,
		        max_steps, total_tokens, total_cost_usd, started_at, completed_at, created_at, updated_at
		 FROM workflow_runs WHERE id = $1`,
		runID,
	).Scan(
		&run.ID, &run.ProjectID, &run.AgentConfigID, &run.Goal, &run.Status,
		&run.CurrentStepIdx, &run.MaxSteps, &run.TotalTokens, &run.TotalCostUSD,
		&run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *Repository) List(ctx context.Context, projectID uuid.UUID) ([]Run, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, project_id, agent_config_id, goal, status, current_step_index,
		        max_steps, total_tokens, total_cost_usd, started_at, completed_at, created_at, updated_at
		 FROM workflow_runs WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Run
	for rows.Next() {
		var run Run
		if err := rows.Scan(
			&run.ID, &run.ProjectID, &run.AgentConfigID, &run.Goal, &run.Status,
			&run.CurrentStepIdx, &run.MaxSteps, &run.TotalTokens, &run.TotalCostUSD,
			&run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	if result == nil {
		result = []Run{}
	}
	return result, rows.Err()
}

func (r *Repository) UpdateStatus(ctx context.Context, runID uuid.UUID, status RunStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflow_runs SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, runID,
	)
	return err
}

func (r *Repository) ListByStatus(ctx context.Context, statuses ...RunStatus) ([]Run, error) {
	statusStrings := make([]string, len(statuses))
	for i, s := range statuses {
		statusStrings[i] = string(s)
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, project_id, agent_config_id, goal, status, current_step_index,
		        max_steps, total_tokens, total_cost_usd, started_at, completed_at, created_at, updated_at
		 FROM workflow_runs WHERE status = ANY($1::run_status[]) ORDER BY created_at ASC`,
		statusStrings,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Run
	for rows.Next() {
		var run Run
		if err := rows.Scan(
			&run.ID, &run.ProjectID, &run.AgentConfigID, &run.Goal, &run.Status,
			&run.CurrentStepIdx, &run.MaxSteps, &run.TotalTokens, &run.TotalCostUSD,
			&run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	return result, rows.Err()
}

func (r *Repository) CreateStep(ctx context.Context, runID uuid.UUID, stepIndex int, stepType StepType) (*Step, error) {
	step := &Step{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO workflow_steps (run_id, step_index, step_type)
		 VALUES ($1, $2, $3)
		 RETURNING id, run_id, step_index, step_type, status,
		           action_json, tool_name, tool_input_json, tool_output_json,
		           error_message, retry_count, started_at, completed_at, created_at`,
		runID, stepIndex, stepType,
	).Scan(
		&step.ID, &step.RunID, &step.StepIndex, &step.StepType, &step.Status,
		&step.ActionJSON, &step.ToolName, &step.ToolInputJSON, &step.ToolOutputJSON,
		&step.ErrorMessage, &step.RetryCount, &step.StartedAt, &step.CompletedAt, &step.CreatedAt,
	)
	return step, err
}

func (r *Repository) ListSteps(ctx context.Context, runID uuid.UUID) ([]Step, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, run_id, step_index, step_type, status,
		        action_json, tool_name, tool_input_json, tool_output_json,
		        error_message, retry_count, started_at, completed_at, created_at
		 FROM workflow_steps WHERE run_id = $1 ORDER BY step_index ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []Step
	for rows.Next() {
		var s Step
		if err := rows.Scan(
			&s.ID, &s.RunID, &s.StepIndex, &s.StepType, &s.Status,
			&s.ActionJSON, &s.ToolName, &s.ToolInputJSON, &s.ToolOutputJSON,
			&s.ErrorMessage, &s.RetryCount, &s.StartedAt, &s.CompletedAt, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		steps = append(steps, s)
	}
	if steps == nil {
		steps = []Step{}
	}
	return steps, rows.Err()
}

func (r *Repository) UpdateStepStatus(ctx context.Context, stepID uuid.UUID, status StepStatus, errMsg *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflow_steps
		 SET status = $1, error_message = $2,
		     completed_at = CASE WHEN $1 IN ('SUCCEEDED','FAILED','SKIPPED') THEN NOW() ELSE completed_at END
		 WHERE id = $3`,
		status, errMsg, stepID,
	)
	return err
}

// StartStep marks a step as running and stores the model's action decision.
func (r *Repository) StartStep(ctx context.Context, stepID uuid.UUID, actionJSON []byte, toolName *string, toolInputJSON []byte) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflow_steps
		 SET status = 'RUNNING', started_at = NOW(),
		     action_json = $1, tool_name = $2, tool_input_json = $3
		 WHERE id = $4`,
		actionJSON, toolName, toolInputJSON, stepID,
	)
	return err
}

// CompleteStep marks a step terminal and stores the tool output.
func (r *Repository) CompleteStep(ctx context.Context, stepID uuid.UUID, status StepStatus, toolOutputJSON []byte, errMsg *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflow_steps
		 SET status = $1, completed_at = NOW(), tool_output_json = $2, error_message = $3
		 WHERE id = $4`,
		status, toolOutputJSON, errMsg, stepID,
	)
	return err
}

// UpdateRunProgress updates the run's status, current step index, and running token/cost totals atomically.
func (r *Repository) UpdateRunProgress(ctx context.Context, runID uuid.UUID, status RunStatus, stepIdx int, tokensToAdd int, costToAdd float64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflow_runs
		 SET status = $1,
		     current_step_index = $2,
		     total_tokens = total_tokens + $3,
		     total_cost_usd = total_cost_usd + $4,
		     updated_at = NOW(),
		     started_at   = CASE WHEN started_at IS NULL AND $1 != 'CREATED' THEN NOW() ELSE started_at END,
		     completed_at = CASE WHEN $1 IN ('COMPLETED','FAILED','CANCELLED') THEN NOW() ELSE completed_at END
		 WHERE id = $5`,
		status, stepIdx, tokensToAdd, costToAdd, runID,
	)
	return err
}
