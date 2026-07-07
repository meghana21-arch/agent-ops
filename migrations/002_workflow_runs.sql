CREATE TYPE run_status AS ENUM (
    'CREATED',
    'PLANNING',
    'RUNNING',
    'WAITING_FOR_APPROVAL',
    'RETRYING',
    'COMPLETED',
    'FAILED',
    'CANCELLED'
);

CREATE TYPE step_status AS ENUM (
    'PENDING',
    'RUNNING',
    'SUCCEEDED',
    'FAILED',
    'SKIPPED',
    'REQUIRES_APPROVAL'
);

CREATE TYPE step_type AS ENUM (
    'PLAN',
    'TOOL_CALL',
    'OBSERVATION',
    'VERIFICATION',
    'ERROR'
);

CREATE TABLE agent_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT 'claude-sonnet-4-6',
    system_prompt TEXT NOT NULL DEFAULT '',
    allowed_tools_json JSONB NOT NULL DEFAULT '[]',
    approval_policy_json JSONB NOT NULL DEFAULT '{}',
    max_steps INTEGER NOT NULL DEFAULT 20,
    max_cost_usd NUMERIC(10,4) NOT NULL DEFAULT 1.00,
    max_retries INTEGER NOT NULL DEFAULT 3,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE workflow_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    agent_config_id UUID REFERENCES agent_configs(id),
    goal TEXT NOT NULL,
    status run_status NOT NULL DEFAULT 'CREATED',
    current_step_index INTEGER NOT NULL DEFAULT 0,
    max_steps INTEGER NOT NULL DEFAULT 20,
    max_cost_usd NUMERIC(10,4) NOT NULL DEFAULT 1.00,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    total_cost_usd NUMERIC(10,6) NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_runs_project_id ON workflow_runs(project_id);
CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);
CREATE INDEX idx_workflow_runs_created_at ON workflow_runs(created_at DESC);

CREATE TABLE workflow_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    step_index INTEGER NOT NULL,
    step_type step_type NOT NULL,
    status step_status NOT NULL DEFAULT 'PENDING',
    action_json JSONB,
    tool_name TEXT,
    tool_input_json JSONB,
    tool_output_json JSONB,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, step_index)
);

CREATE INDEX idx_workflow_steps_run_id ON workflow_steps(run_id);
CREATE INDEX idx_workflow_steps_status ON workflow_steps(status);
