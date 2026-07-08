package agents

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ApprovalPolicy struct {
	RequireApprovalFor []string `json:"requireApprovalFor"` // risk levels: "HIGH", "CRITICAL"
}

type AgentConfig struct {
	ID               uuid.UUID      `json:"id"`
	ProjectID        uuid.UUID      `json:"projectId"`
	Name             string         `json:"name"`
	Model            string         `json:"model"`
	SystemPrompt     string         `json:"systemPrompt"`
	AllowedTools     []string       `json:"allowedTools"`
	ApprovalPolicy   ApprovalPolicy `json:"approvalPolicy"`
	MaxSteps         int            `json:"maxSteps"`
	MaxCostUSD       float64        `json:"maxCostUsd"`
	MaxRetries       int            `json:"maxRetries"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`

	// raw JSON columns scanned from DB
	allowedToolsJSON   []byte
	approvalPolicyJSON []byte
}

func (a *AgentConfig) decode() error {
	if err := json.Unmarshal(a.allowedToolsJSON, &a.AllowedTools); err != nil {
		return err
	}
	return json.Unmarshal(a.approvalPolicyJSON, &a.ApprovalPolicy)
}
