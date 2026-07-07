# AgentOps Runtime — Project Plan
## Durable Execution Layer for AI Agent Loops

> **One-line positioning:** AgentOps Runtime is a durable execution layer for developers building reliable AI agent workflows.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | Next.js, TypeScript, React, Tailwind CSS, TanStack Query |
| Backend API | Go (Gin/Chi/Fiber) |
| Agent Worker | Go |
| CLI | Go |
| Database | PostgreSQL (RDS) |
| Cache / Queue | Redis (ElastiCache) |
| AI | Claude API (Anthropic) |
| Infra | AWS CDK (TypeScript), ECS Fargate, ECR, S3, CloudWatch, Secrets Manager |
| CI/CD | GitHub Actions |
| Local Dev | Docker Compose |

---

## Architecture Overview

```
User / CLI / Customer App
        ↓
Route 53 + HTTPS
        ↓
CloudFront / Vercel
        ↓
Next.js Dashboard
        ↓
Application Load Balancer
        ↓
ECS Fargate: Go API Service
        ↓
ECS Fargate: Go Worker Service
        ↓
RDS PostgreSQL ←→ ElastiCache Redis
        ↓
S3 Artifacts + CloudWatch Logs
        ↓
Claude API + Tool Execution Layer
```

---

## Phase 0 — Planning and System Design

**Goal:** Finalize product scope, architecture, and contracts before writing code.

**Deliverables:**
- [x] Final design doc (this file)
- [ ] Architecture diagram (Excalidraw or draw.io)
- [ ] Database schema draft (all 12 tables)
- [ ] API contract draft (all endpoints, request/response shapes)
- [ ] Tool risk policy document
- [ ] MVP demo scenario script

**Key Decisions:**
- Go for API + Worker (concurrency, performance, single binary deploys)
- Next.js + TypeScript for dashboard
- PostgreSQL for durable state (not NoSQL — relational integrity matters here)
- Redis for retry queues, locks, idempotency keys, rate limits
- AWS CDK TypeScript for infrastructure as code
- Claude API with structured JSON action format

**Success Criteria:**
> Can explain the project in 60 seconds: "I built a durable execution layer for AI agent loops."

---

## Phase 1 — Local Product Skeleton

**Goal:** Create the basic full-stack application running locally end-to-end.

**Features:**
- Next.js dashboard skeleton (layout, routing, nav)
- Go API service with basic structure (`cmd/api`, `internal/`)
- PostgreSQL connection pool
- Redis connection
- Docker Compose setup (`frontend`, `api`, `worker`, `postgres`, `redis`)
- Health check endpoints

**Backend APIs:**
```
GET  /health
POST /v1/projects
GET  /v1/projects
```

**Frontend Pages:**
- `/login` (mock, no real auth yet)
- `/projects` (list + create)

**Repo Structure:**
```
agentops/
├── frontend/          # Next.js dashboard
├── cmd/
│   ├── api/           # Go API server entry
│   ├── worker/        # Go worker entry
│   └── cli/           # Go CLI entry
├── internal/
│   ├── auth/
│   ├── projects/
│   ├── agents/
│   ├── runs/
│   ├── tools/
│   ├── approvals/
│   ├── evals/
│   ├── queue/
│   ├── llm/
│   ├── db/
│   └── telemetry/
├── infra/             # AWS CDK TypeScript
├── migrations/        # SQL migration files
└── docker-compose.yml
```

**Success Criteria:** User can open the dashboard and create a project via UI.

---

## Phase 2 — Workflow Runs and State Machine

**Goal:** Build the durable workflow model — the core data layer.

**Features:**
- Create workflow run API
- Persist run in PostgreSQL with full state machine
- Persist workflow steps
- Implement all run and step statuses
- Run list page
- Run detail/timeline page

**Run State Machine:**
```
CREATED → PLANNING → RUNNING → WAITING_FOR_APPROVAL → RETRYING → COMPLETED
                                                                 → FAILED
                                                                 → CANCELLED
```

**Step Statuses:** `PENDING → RUNNING → SUCCEEDED / FAILED / SKIPPED / REQUIRES_APPROVAL`

**Backend APIs:**
```
POST /v1/runs
GET  /v1/runs
GET  /v1/runs/:runId
GET  /v1/runs/:runId/steps
```

**Frontend Pages:**
- `/runs` — run list with status badges
- `/runs/:runId` — step timeline

**Key Tables:** `workflow_runs`, `workflow_steps`

**Success Criteria:** User can create a run and see a persisted step timeline in the dashboard.

---

## Phase 3 — Claude Agent Loop

**Goal:** Connect the Claude API and produce the first working agent loop.

**Features:**
- `agent_configs` table (model, system prompt, allowed tools, approval policy)
- Structured JSON action format from Claude
- Claude API client in Go (`internal/llm/`)
- Plan generation step
- Next-action generation step
- Persist model decisions as steps in the database

**Structured Model Output Format:**
```json
{
  "actionType": "TOOL_CALL",
  "toolName": "run_tests",
  "reason": "I need to reproduce the failing test.",
  "input": {
    "command": "npm test"
  }
}
```

**Agent Loop (pseudo-code):**
```go
for !run.IsTerminal() {
    context := LoadRunContext(run.ID)
    action := llm.DecideNextAction(context)
    step := PersistStep(run.ID, action)
    // ... tool execution, verification, etc.
}
```

**Success Criteria:** Given a goal, the agent creates a plan and requests a tool action, all stored in PostgreSQL.

---

## Phase 4 — Tool Execution Layer

**Goal:** Allow the runtime to safely execute controlled tools.

**V1 Tools:**

| Tool | Risk Level | Description |
|---|---|---|
| `list_files` | LOW | List files in directory |
| `read_file` | LOW | Read file contents |
| `search_code` | LOW | Grep/search in codebase |
| `run_tests` | MEDIUM | Execute test suite |
| `summarize_logs` | MEDIUM | Parse and summarize error logs |
| `create_patch` | HIGH | Generate a code patch |
| `apply_patch` | CRITICAL | Apply patch to files |

**Risk Level Policy:**
- `LOW` → auto-allow
- `MEDIUM` → auto-allow + audit log
- `HIGH` → approval required
- `CRITICAL` → approval required + ADMIN role

**Backend APIs:**
```
POST /v1/projects/:projectId/tools
GET  /v1/projects/:projectId/tools
```

**Frontend Pages:**
- `/projects/:projectId/tools` — tool registry UI

**Success Criteria:** Agent calls a registered tool → runtime validates, executes, and persists the result.

---

## Phase 5 — Retry, Idempotency, and Redis Queues

**Goal:** Make failed tool execution reliable without duplicate side-effects.

**Features:**
- Redis-backed retry queue
- Exponential backoff
- Idempotency keys (`run_id + step_index + tool_name + hash(tool_input)`)
- Worker distributed locks (`lock:run:{run_id}`)
- Timeout handling per tool
- Failure categorization

**Failure Categories:**
```
TOOL_TIMEOUT
TOOL_ERROR
INVALID_TOOL_INPUT
MODEL_BAD_ACTION
PERMISSION_DENIED
MAX_RETRIES_EXCEEDED
```

**Redis Key Schema:**
```
lock:run:{run_id}
retry:step:{step_id}
idempotency:{key}
rate_limit:project:{project_id}
worker:heartbeat:{worker_id}
```

**Success Criteria:** A tool that fails temporarily is retried safely; no duplicate execution occurs.

---

## Phase 6 — Human Approval Gates

**Goal:** Prevent risky agent actions from executing without human sign-off.

**Features:**
- `approval_requests` table
- Approval policy engine (based on tool risk level)
- Pause run when approval is required
- Approve/reject APIs
- Approval UI with tool preview and risk context
- Resume run after approval decision
- Audit log entry for every decision

**Approval Statuses:** `PENDING → APPROVED / REJECTED / EXPIRED`

**Backend APIs:**
```
GET  /v1/approvals
POST /v1/approvals/:approvalId/approve
POST /v1/approvals/:approvalId/reject
```

**Frontend Page:**
- `/approvals` — approval cards showing tool, risk level, input preview, approve/reject buttons

**Success Criteria:** HIGH-risk tool pauses the workflow until a human approves; decision is audit-logged.

---

## Phase 7 — Crash Recovery Demo

**Goal:** Prove durable execution — the core differentiator of the system.

**Features:**
- Resume run from last safely persisted step
- Detect stuck/running steps on startup
- Recover after API or worker restart
- `agentops resume <run_id>` CLI command

**Resume Rules:**
```
Last step SUCCEEDED   → continue from next step
Last step FAILED      → retry if retries remain, else fail
Last step RUNNING     → check idempotency key:
                          result exists → mark SUCCEEDED and continue
                          no result    → retry safely
WAITING_FOR_APPROVAL  → remain paused
Max steps exceeded    → mark FAILED
```

**Demo Script:**
1. Start run: "Find and fix the failing test"
2. Agent plans and executes tools
3. Kill the backend/worker process (`kill -9`)
4. Restart the service
5. Runtime loads state from PostgreSQL
6. Runtime resumes from last safe step
7. Run completes successfully

**Success Criteria:** Workflow continues and completes after a forced backend crash — no data loss, no duplicate execution.

---

## Phase 8 — Customer Dashboard Polish

**Goal:** Make the system feel like a real developer product (Stripe / Datadog quality).

**Features:**
- Project overview cards (run count, success rate, cost, latency)
- Live run timeline with WebSocket or SSE updates
- Tool call detail drawer
- Cost and latency display per step
- Retry count and error badges
- Approval cards inline
- Run filters (status, date, project)
- Failure category pie chart

**All Dashboard Pages:**
- `/projects` — project list
- `/projects/:projectId` — project overview
- `/runs` — run list with filters
- `/runs/:runId` — live step timeline + detail drawer
- `/approvals` — pending approvals
- `/settings/api-keys` — API key management

**Success Criteria:** A recruiter or engineer understands the product within 2 minutes of using the dashboard.

---

## Phase 9 — Evaluation Harness

**Goal:** Measure agent reliability with repeatable benchmarks.

**Benchmark Tasks (target: 20–50 tasks):**
- Choose correct tool given a goal
- Recover from a tool timeout
- Request approval for a risky tool
- Summarize error logs correctly
- Find the failing test
- Generate a safe, applicable patch
- Resume after crash

**Metrics:**
```
task_success_rate
tool_call_accuracy
approval_correctness
average_retries
average_latency_ms
average_cost_usd
failure_category
```

**Eval Dashboard:**
- `/evals` — benchmark run list
- Prompt version comparison
- Model comparison (future: Claude vs GPT)

**Success Criteria:**
> "AgentOps achieved 84% task success across 50 benchmark runs, with 91% tool-call accuracy and average 1.2 retries per failed tool."

---

## Phase 10 — Cloud Deployment with AWS CDK

**Goal:** Deploy as a real cloud-native SaaS on AWS.

**AWS Services:**

| Service | Purpose |
|---|---|
| ECS Fargate | Run Go API + Worker containers |
| ECR | Docker image registry |
| RDS PostgreSQL | Durable workflow state |
| ElastiCache Redis | Retry queues, locks, idempotency |
| S3 | Run artifacts, patches, eval datasets |
| CloudWatch | Logs, metrics, alarms |
| Secrets Manager | API keys, DB passwords, Redis creds |
| ALB | HTTPS traffic routing |
| AWS CDK (TypeScript) | Infrastructure as code |

**Environments:**
- `local` — Docker Compose
- `staging` — ECS + small RDS + Redis
- `production-demo` — ECS + RDS + Redis + S3 + CloudWatch

**CI/CD (GitHub Actions):**
1. Run frontend tests
2. Run Go tests
3. Build frontend
4. Build Go API Docker image
5. Build Go Worker Docker image
6. Push images to ECR
7. Run database migrations
8. Deploy ECS services
9. Run smoke tests

**Success Criteria:** Customer can access the live dashboard, create a run, and watch a workflow execute on AWS.

---

## Phase 11 — CLI and API Key Access

**Goal:** Let developers use AgentOps from their terminal without opening the dashboard.

**API Key Features:**
- API key creation via dashboard
- Hashed key storage (never store plaintext)
- Key prefix for identification
- Revoke and rotate

**CLI Commands:**
```bash
agentops login
agentops projects list
agentops run --project coding-agent --goal "Find and fix failing tests" --repo ./sample-repo
agentops runs list
agentops runs watch <run_id>
agentops approve <approval_id>
agentops resume <run_id>
agentops eval run <benchmark_name>
```

**Success Criteria:** Developer can run a full agent workflow from terminal without touching the dashboard.

---

## Phase 12 — Final Demo, README, and Portfolio Packaging

**Goal:** Package the project for recruiters, interviews, GitHub, and LinkedIn.

**Deliverables:**
- GitHub README with architecture diagram
- Demo video (Loom, 3–5 min)
- Live dashboard link
- CLI demo recording (asciinema)
- Resume bullets (see below)
- LinkedIn post
- Technical blog post

**Final Demo Flow:**
1. Open dashboard
2. Create project
3. Register coding tools (list_files, read_file, run_tests, create_patch)
4. Start run: "Fix failing tests"
5. Watch agent plan and execute tools live
6. Show retry after simulated tool failure
7. Show approval gate before `apply_patch`
8. Kill worker process
9. Restart worker
10. Resume workflow from last step
11. Show completed result with cost/latency
12. Open eval dashboard — show benchmark metrics

---

## MVP Feature Cut (Time-Constrained)

If time is limited, ship only this core:

| # | Feature | Phase |
|---|---|---|
| 1 | Next.js dashboard | 1 |
| 2 | Go API service | 1 |
| 3 | Go worker service | 1 |
| 4 | PostgreSQL workflow state | 2 |
| 5 | Redis retry + idempotency | 5 |
| 6 | Claude structured agent loop | 3 |
| 7 | 5 core tools | 4 |
| 8 | Approval gate | 6 |
| 9 | Run trace page | 2 |
| 10 | Crash recovery demo | 7 |

Do **not** build multi-agent workflows in V1.

---

## Data Model Summary

| Table | Purpose |
|---|---|
| `organizations` | Multi-tenant root |
| `users` | Auth + RBAC |
| `projects` | Workspace grouping |
| `api_keys` | Auth (hashed, never plaintext) |
| `agent_configs` | Model, prompt, tool policy per agent |
| `tool_registry` | Tool definitions, schemas, risk levels |
| `workflow_runs` | Durable run records with state machine |
| `workflow_steps` | Every agent action persisted |
| `approval_requests` | Human-in-the-loop gates |
| `eval_runs` | Benchmark execution records |
| `eval_results` | Per-task benchmark results |
| `audit_logs` | Immutable event trail |

---

## Security Model

**Roles:** `OWNER → ADMIN → DEVELOPER → APPROVER → VIEWER`

**Auth:** JWT for dashboard sessions, hashed API keys for REST/CLI access.

**Tool permission enforcement:** Risk level → approval policy → execution or pause.

**Never store:** Plaintext API keys, unredacted secrets in logs, user data cross-tenant.

---

## V2 Roadmap (Post-MVP)

- GitHub repo integration + PR creation
- MCP-compatible tool registry
- OpenTelemetry export
- Multi-agent reviewer (agent checks another agent's output)
- Model comparison: Claude vs GPT vs Gemini
- Prompt versioning
- Webhook support
- Slack approval integration
- LangGraph adapter
- Organization billing simulation

---

## Resume Bullets

**AgentOps Runtime — Durable Execution Layer for AI Agent Loops**
`Next.js · TypeScript · Go · PostgreSQL · Redis · AWS CDK · Docker · Claude API`

- Built a cloud-native durable execution layer for Claude-powered agent loops, enabling customer projects to run, trace, approve, retry, and resume multi-step AI workflows through a dashboard, REST API, and CLI.
- Developed a Go-based agent orchestration service with persisted state machines, tool execution, timeout handling, idempotency keys, and Redis-backed retry queues for reliable long-running workflows.
- Designed PostgreSQL schemas for workflow runs, steps, tool calls, approval requests, eval results, audit logs, projects, API keys, and multi-tenant customer isolation.
- Built a Next.js + TypeScript customer dashboard for project setup, tool registry management, live run traces, human approval gates, eval metrics, and workflow replay/debugging.
- Provisioned AWS infrastructure using TypeScript-based CDK, deploying containerized Go API and worker services with RDS PostgreSQL, ElastiCache Redis, S3, CloudWatch, and GitHub Actions CI/CD.
