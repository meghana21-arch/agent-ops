package llm

import "encoding/json"

type ActionType string

const (
	ActionTypePlan     ActionType = "PLAN"
	ActionTypeToolCall ActionType = "TOOL_CALL"
	ActionTypeComplete ActionType = "COMPLETE"
)

// Action is the structured JSON response the model emits at each step.
type Action struct {
	ActionType ActionType      `json:"actionType"`
	Reason     string          `json:"reason"`
	Plan       []string        `json:"plan,omitempty"`
	ToolName   string          `json:"toolName,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Summary    string          `json:"summary,omitempty"`
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
}

type Message struct {
	Role    string // "user" or "assistant"
	Content string
}
