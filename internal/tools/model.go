package tools

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// ToolDef describes a built-in tool available in the runtime.
type ToolDef struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Risk        RiskLevel `json:"riskLevel"`
}

// ProjectTool is the per-project DB record enabling a tool for a project.
type ProjectTool struct {
	ID        uuid.UUID       `json:"id"`
	ProjectID uuid.UUID       `json:"projectId"`
	ToolName  string          `json:"toolName"`
	RiskLevel RiskLevel       `json:"riskLevel"`
	Enabled   bool            `json:"enabled"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// ExecuteResult is the output of a tool invocation.
type ExecuteResult struct {
	Output        json.RawMessage `json:"output,omitempty"`
	NeedsApproval bool            `json:"needsApproval,omitempty"`
	Error         *string         `json:"error,omitempty"`
}
