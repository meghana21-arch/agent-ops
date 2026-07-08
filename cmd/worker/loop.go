package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/agentops/runtime/internal/agents"
	"github.com/agentops/runtime/internal/llm"
	"github.com/agentops/runtime/internal/runs"
	"github.com/google/uuid"
)

type agentLoop struct {
	runRepo   *runs.Repository
	agentRepo *agents.Repository
	llm       *llm.Client
}

// processRun drives a single workflow run to completion (or failure).
// It is called once per run and should be safe to re-enter after a crash (Phase 7).
func (l *agentLoop) processRun(ctx context.Context, run *runs.Run) {
	log.Printf("[run %s] starting — goal: %s", run.ID, run.Goal)

	cfg, err := l.loadConfig(ctx, run)
	if err != nil {
		l.failRun(ctx, run.ID, fmt.Sprintf("load agent config: %v", err))
		return
	}

	// --- Phase: PLANNING ---
	if run.Status == runs.StatusCreated {
		if err := l.runPlanningPhase(ctx, run, cfg); err != nil {
			l.failRun(ctx, run.ID, fmt.Sprintf("planning phase: %v", err))
			return
		}
	}

	// --- Phase: RUNNING (action loop) ---
	history, err := l.buildHistory(ctx, run)
	if err != nil {
		l.failRun(ctx, run.ID, fmt.Sprintf("load history: %v", err))
		return
	}

	for {
		// Reload run to get current step index and check for cancellation
		run, err = l.runRepo.GetByID(ctx, run.ID)
		if err != nil {
			log.Printf("[run %s] reload error: %v", run.ID, err)
			return
		}
		if run.IsTerminal() {
			log.Printf("[run %s] run already terminal (%s), exiting loop", run.ID, run.Status)
			return
		}
		if run.CurrentStepIdx >= run.MaxSteps {
			l.failRun(ctx, run.ID, "max steps reached")
			return
		}

		action, usage, err := l.llm.DecideNextAction(ctx, history, cfg.SystemPrompt, cfg.AllowedTools)
		if err != nil {
			l.failRun(ctx, run.ID, fmt.Sprintf("LLM error: %v", err))
			return
		}

		nextIdx := run.CurrentStepIdx + 1

		switch action.ActionType {
		case llm.ActionTypeComplete:
			if err := l.persistCompleteStep(ctx, run, nextIdx, action, usage); err != nil {
				log.Printf("[run %s] persist complete step: %v", run.ID, err)
			}
			if err := l.runRepo.UpdateRunProgress(ctx, run.ID, runs.StatusCompleted, nextIdx, usage.TotalTokens, usage.CostUSD); err != nil {
				log.Printf("[run %s] update run complete: %v", run.ID, err)
			}
			log.Printf("[run %s] COMPLETED — %s", run.ID, action.Summary)
			return

		case llm.ActionTypeToolCall:
			assistantJSON, _ := json.Marshal(action)
			step, err := l.persistToolCallStep(ctx, run, nextIdx, action, usage)
			if err != nil {
				l.failRun(ctx, run.ID, fmt.Sprintf("persist tool call step: %v", err))
				return
			}

			// Phase 4 will replace this stub with real tool execution.
			stubOutput := map[string]string{"result": "tool execution not yet implemented (Phase 4)"}
			stubJSON, _ := json.Marshal(stubOutput)
			if err := l.runRepo.CompleteStep(ctx, step.ID, runs.StepSucceeded, stubJSON, nil); err != nil {
				log.Printf("[run %s] complete step: %v", run.ID, err)
			}
			if err := l.runRepo.UpdateRunProgress(ctx, run.ID, runs.StatusRunning, nextIdx, usage.TotalTokens, usage.CostUSD); err != nil {
				log.Printf("[run %s] update run progress: %v", run.ID, err)
			}

			// Append to conversation history so Claude has full context next iteration
			history = append(history,
				llm.Message{Role: "assistant", Content: string(assistantJSON)},
				llm.Message{Role: "user", Content: fmt.Sprintf("Tool result for %s: %s\n\nContinue. What is your next action?", action.ToolName, string(stubJSON))},
			)
			log.Printf("[run %s] step %d: TOOL_CALL %s", run.ID, nextIdx, action.ToolName)

		default:
			l.failRun(ctx, run.ID, fmt.Sprintf("unexpected actionType: %s", action.ActionType))
			return
		}
	}
}

func (l *agentLoop) loadConfig(ctx context.Context, run *runs.Run) (*agents.AgentConfig, error) {
	if run.AgentConfigID != nil {
		return l.agentRepo.GetByID(ctx, *run.AgentConfigID)
	}
	// No config attached — use defaults
	return &agents.AgentConfig{
		Model:        "claude-sonnet-4-6-20250620",
		AllowedTools: []string{},
		MaxSteps:     run.MaxSteps,
	}, nil
}

func (l *agentLoop) runPlanningPhase(ctx context.Context, run *runs.Run, cfg *agents.AgentConfig) error {
	if err := l.runRepo.UpdateRunProgress(ctx, run.ID, runs.StatusPlanning, 0, 0, 0); err != nil {
		return err
	}

	action, usage, err := l.llm.GeneratePlan(ctx, run.Goal, cfg.SystemPrompt, cfg.AllowedTools)
	if err != nil {
		return fmt.Errorf("generate plan: %w", err)
	}

	step, err := l.runRepo.CreateStep(ctx, run.ID, 0, runs.StepTypePlan)
	if err != nil {
		return fmt.Errorf("create plan step: %w", err)
	}

	actionJSON, _ := json.Marshal(action)
	if err := l.runRepo.StartStep(ctx, step.ID, actionJSON, nil, nil); err != nil {
		return fmt.Errorf("start plan step: %w", err)
	}
	if err := l.runRepo.CompleteStep(ctx, step.ID, runs.StepSucceeded, nil, nil); err != nil {
		return fmt.Errorf("complete plan step: %w", err)
	}
	if err := l.runRepo.UpdateRunProgress(ctx, run.ID, runs.StatusRunning, 0, usage.TotalTokens, usage.CostUSD); err != nil {
		return err
	}

	log.Printf("[run %s] plan generated: %v", run.ID, action.Plan)
	return nil
}

// buildHistory reconstructs the conversation from persisted steps so the loop is crash-safe.
func (l *agentLoop) buildHistory(ctx context.Context, run *runs.Run) ([]llm.Message, error) {
	steps, err := l.runRepo.ListSteps(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	history := []llm.Message{
		{Role: "user", Content: fmt.Sprintf("Goal: %s\n\nGenerate a step-by-step plan to accomplish this goal.", run.Goal)},
	}

	for _, s := range steps {
		if s.ActionJSON == nil {
			continue
		}
		raw, err := json.Marshal(s.ActionJSON)
		if err != nil {
			continue
		}
		history = append(history, llm.Message{Role: "assistant", Content: string(raw)})

		if s.StepType == runs.StepTypePlan {
			history = append(history, llm.Message{
				Role:    "user",
				Content: "Plan received. Begin execution. What is your first action?",
			})
		} else if s.ToolOutputJSON != nil {
			outputRaw, _ := json.Marshal(s.ToolOutputJSON)
			toolName := ""
			if s.ToolName != nil {
				toolName = *s.ToolName
			}
			history = append(history, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool result for %s: %s\n\nContinue. What is your next action?", toolName, string(outputRaw)),
			})
		}
	}

	// If only the plan step exists, add the "begin execution" prompt
	if len(steps) == 1 && steps[0].StepType == runs.StepTypePlan {
		// already appended above
	} else if len(steps) == 0 {
		// no steps yet — fresh run that was somehow in RUNNING state
		history = append(history, llm.Message{
			Role:    "user",
			Content: "Begin execution. What is your first action?",
		})
	}

	return history, nil
}

func (l *agentLoop) persistToolCallStep(ctx context.Context, run *runs.Run, idx int, action *llm.Action, usage *llm.Usage) (*runs.Step, error) {
	step, err := l.runRepo.CreateStep(ctx, run.ID, idx, runs.StepTypeToolCall)
	if err != nil {
		return nil, err
	}
	actionJSON, _ := json.Marshal(action)
	toolInputJSON, _ := json.Marshal(action.Input)
	toolName := action.ToolName
	if err := l.runRepo.StartStep(ctx, step.ID, actionJSON, &toolName, toolInputJSON); err != nil {
		return nil, err
	}
	return step, nil
}

func (l *agentLoop) persistCompleteStep(ctx context.Context, run *runs.Run, idx int, action *llm.Action, usage *llm.Usage) error {
	step, err := l.runRepo.CreateStep(ctx, run.ID, idx, runs.StepTypeVerification)
	if err != nil {
		return err
	}
	actionJSON, _ := json.Marshal(action)
	if err := l.runRepo.StartStep(ctx, step.ID, actionJSON, nil, nil); err != nil {
		return err
	}
	return l.runRepo.CompleteStep(ctx, step.ID, runs.StepSucceeded, nil, nil)
}

func (l *agentLoop) failRun(ctx context.Context, runID uuid.UUID, reason string) {
	log.Printf("[run %s] FAILED — %s", runID, reason)
	if err := l.runRepo.UpdateRunProgress(ctx, runID, runs.StatusFailed, 0, 0, 0); err != nil {
		log.Printf("[run %s] error marking failed: %v", runID, err)
	}
}
