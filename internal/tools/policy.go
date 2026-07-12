package tools

// ApprovalPolicy mirrors agents.ApprovalPolicy to avoid a circular import.
type ApprovalPolicy struct {
	RequireApprovalFor []string `json:"requireApprovalFor"`
}

// PolicyResult describes whether a tool call can proceed automatically.
type PolicyResult struct {
	NeedsApproval bool
	Reason        string
	AuditLog      bool
}

// CheckPolicy evaluates a tool's risk level against the run's approval policy.
//
// Default behaviour (no explicit policy):
//   - LOW    → auto-allow, no audit
//   - MEDIUM → auto-allow, audit log
//   - HIGH   → approval required
//   - CRITICAL → approval required
func CheckPolicy(risk RiskLevel, policy ApprovalPolicy) PolicyResult {
	// If the agent config explicitly lists this risk level, require approval.
	for _, level := range policy.RequireApprovalFor {
		if RiskLevel(level) == risk {
			return PolicyResult{NeedsApproval: true, Reason: string(risk) + " tool requires approval per agent config"}
		}
	}

	// Enforce hard defaults.
	switch risk {
	case RiskHigh:
		return PolicyResult{NeedsApproval: true, Reason: "HIGH risk tools always require approval"}
	case RiskCritical:
		return PolicyResult{NeedsApproval: true, Reason: "CRITICAL risk tools always require approval"}
	case RiskMedium:
		return PolicyResult{NeedsApproval: false, AuditLog: true}
	default:
		return PolicyResult{NeedsApproval: false}
	}
}
