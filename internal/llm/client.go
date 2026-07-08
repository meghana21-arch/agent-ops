package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	// Pricing per million tokens for claude-sonnet-4-6 (approximate)
	inputCostPerMToken  = 3.0
	outputCostPerMToken = 15.0
)

const systemPromptTemplate = `You are an AI agent running inside a durable execution runtime called AgentOps. Your job is to accomplish the given goal by taking actions step by step.

At each step, respond with ONLY valid JSON — no markdown, no explanation outside the JSON.

Use one of these three formats:

1. PLAN (first response only — your strategic approach):
{"actionType":"PLAN","reason":"<why this approach>","plan":["Step 1: ...","Step 2: ...","Step 3: ..."]}

2. TOOL_CALL (invoke a tool):
{"actionType":"TOOL_CALL","toolName":"<name>","reason":"<why>","input":{...}}

3. COMPLETE (goal is done):
{"actionType":"COMPLETE","reason":"<why complete>","summary":"<what was accomplished>"}

Available tools: %s

Rules:
- Output ONLY JSON
- Always include "reason"
- If a tool is unavailable, use COMPLETE and explain`

type Client struct {
	inner *anthropic.Client
	model string
}

func NewClient(apiKey string) *Client {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Client{inner: &c, model: "claude-sonnet-4-6-20250620"}
}

func (c *Client) WithModel(model string) *Client {
	c.model = model
	return c
}

// GeneratePlan sends the goal to Claude and expects a PLAN action back.
func (c *Client) GeneratePlan(ctx context.Context, goal string, systemPrompt string, allowedTools []string) (*Action, *Usage, error) {
	sys := buildSystemPrompt(systemPrompt, allowedTools)
	userMsg := fmt.Sprintf("Goal: %s\n\nGenerate a step-by-step plan to accomplish this goal.", goal)
	msgs := []Message{{Role: "user", Content: userMsg}}
	return c.call(ctx, sys, msgs)
}

// DecideNextAction sends the full conversation history and expects a TOOL_CALL or COMPLETE action.
func (c *Client) DecideNextAction(ctx context.Context, history []Message, systemPrompt string, allowedTools []string) (*Action, *Usage, error) {
	sys := buildSystemPrompt(systemPrompt, allowedTools)
	return c.call(ctx, sys, history)
}

func (c *Client) call(ctx context.Context, systemPrompt string, history []Message) (*Action, *Usage, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: toSDKMessages(history),
	}

	resp, err := c.inner.Messages.New(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("claude API call: %w", err)
	}

	raw := extractText(resp)
	action, err := parseAction(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("parse action JSON: %w (raw: %s)", err, raw)
	}

	u := &Usage{
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
		TotalTokens:  int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}
	u.CostUSD = (float64(u.InputTokens)*inputCostPerMToken + float64(u.OutputTokens)*outputCostPerMToken) / 1_000_000

	return action, u, nil
}

func buildSystemPrompt(override string, tools []string) string {
	if override != "" {
		return override
	}
	toolList := strings.Join(tools, ", ")
	if toolList == "" {
		toolList = "none configured"
	}
	return fmt.Sprintf(systemPromptTemplate, toolList)
}

func toSDKMessages(history []Message) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, len(history))
	for i, m := range history {
		block := anthropic.NewTextBlock(m.Content)
		if m.Role == "assistant" {
			out[i] = anthropic.NewAssistantMessage(block)
		} else {
			out[i] = anthropic.NewUserMessage(block)
		}
	}
	return out
}

func extractText(resp *anthropic.Message) string {
	for _, block := range resp.Content {
		if block.Type == "text" {
			return strings.TrimSpace(block.Text)
		}
	}
	return ""
}

func parseAction(raw string) (*Action, error) {
	// Strip any accidental markdown fences
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var a Action
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, err
	}
	if a.ActionType == "" {
		return nil, fmt.Errorf("missing actionType")
	}
	return &a, nil
}
