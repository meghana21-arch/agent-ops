package runs

import (
	"time"

	"github.com/google/uuid"
)

type RunStatus string

const (
	StatusCreated            RunStatus = "CREATED"
	StatusPlanning           RunStatus = "PLANNING"
	StatusRunning            RunStatus = "RUNNING"
	StatusWaitingForApproval RunStatus = "WAITING_FOR_APPROVAL"
	StatusRetrying           RunStatus = "RETRYING"
	StatusCompleted          RunStatus = "COMPLETED"
	StatusFailed             RunStatus = "FAILED"
	StatusCancelled          RunStatus = "CANCELLED"
)

var terminalStatuses = map[RunStatus]bool{
	StatusCompleted: true,
	StatusFailed:    true,
	StatusCancelled: true,
}

type StepStatus string

const (
	StepPending          StepStatus = "PENDING"
	StepRunning          StepStatus = "RUNNING"
	StepSucceeded        StepStatus = "SUCCEEDED"
	StepFailed           StepStatus = "FAILED"
	StepSkipped          StepStatus = "SKIPPED"
	StepRequiresApproval StepStatus = "REQUIRES_APPROVAL"
)

type StepType string

const (
	StepTypePlan         StepType = "PLAN"
	StepTypeToolCall     StepType = "TOOL_CALL"
	StepTypeObservation  StepType = "OBSERVATION"
	StepTypeVerification StepType = "VERIFICATION"
	StepTypeError        StepType = "ERROR"
)

type Run struct {
	ID             uuid.UUID  `json:"id"`
	ProjectID      uuid.UUID  `json:"projectId"`
	AgentConfigID  *uuid.UUID `json:"agentConfigId,omitempty"`
	Goal           string     `json:"goal"`
	Status         RunStatus  `json:"status"`
	CurrentStepIdx int        `json:"currentStepIndex"`
	MaxSteps       int        `json:"maxSteps"`
	TotalTokens    int        `json:"totalTokens"`
	TotalCostUSD   float64    `json:"totalCostUsd"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (r *Run) IsTerminal() bool {
	return terminalStatuses[r.Status]
}

type Step struct {
	ID             uuid.UUID   `json:"id"`
	RunID          uuid.UUID   `json:"runId"`
	StepIndex      int         `json:"stepIndex"`
	StepType       StepType    `json:"stepType"`
	Status         StepStatus  `json:"status"`
	ActionJSON     interface{} `json:"action,omitempty"`
	ToolName       *string     `json:"toolName,omitempty"`
	ToolInputJSON  interface{} `json:"toolInput,omitempty"`
	ToolOutputJSON interface{} `json:"toolOutput,omitempty"`
	ErrorMessage   *string     `json:"errorMessage,omitempty"`
	RetryCount     int         `json:"retryCount"`
	StartedAt      *time.Time  `json:"startedAt,omitempty"`
	CompletedAt    *time.Time  `json:"completedAt,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
}
