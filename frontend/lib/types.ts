export interface Project {
  id: string;
  organizationId: string;
  name: string;
  description: string;
  environment: 'development' | 'staging' | 'production';
  createdAt: string;
  updatedAt: string;
}

export type RunStatus =
  | 'CREATED'
  | 'PLANNING'
  | 'RUNNING'
  | 'WAITING_FOR_APPROVAL'
  | 'RETRYING'
  | 'COMPLETED'
  | 'FAILED'
  | 'CANCELLED';

export type StepStatus =
  | 'PENDING'
  | 'RUNNING'
  | 'SUCCEEDED'
  | 'FAILED'
  | 'SKIPPED'
  | 'REQUIRES_APPROVAL';

export type StepType = 'PLAN' | 'TOOL_CALL' | 'OBSERVATION' | 'VERIFICATION' | 'ERROR';

export interface Run {
  id: string;
  projectId: string;
  agentConfigId?: string;
  goal: string;
  status: RunStatus;
  currentStepIndex: number;
  maxSteps: number;
  totalTokens: number;
  totalCostUsd: number;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Step {
  id: string;
  runId: string;
  stepIndex: number;
  stepType: StepType;
  status: StepStatus;
  action?: Record<string, unknown>;
  toolName?: string;
  toolInput?: Record<string, unknown>;
  toolOutput?: Record<string, unknown>;
  errorMessage?: string;
  retryCount: number;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}
