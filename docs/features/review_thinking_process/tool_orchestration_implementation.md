# Quaero Tool Orchestration - Implementation

This document describes the tool orchestration architecture as implemented, aligned with the original design specification.

## Overview

The orchestration system implements a **Planner-Executor-Reviewer** pattern where:

1. **AI acts as planner, not executor**: The planning LLM call returns tool specifications only
2. **Tool jobs are queue citizens**: Each tool call is a separate job visible in the UI
3. **Forced tool use**: Planning must return tool calls - it cannot fabricate data
4. **Controlled synthesis**: The reviewer validates that the goal was achieved

## Job Flow

```
┌─────────────────┐
│  User Request   │
│  (Step Job)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   PLANNER       │  ← LLM decides which tools, returns structured plan only
│   (Phase 1)     │     FORCED TOOL USE: Must return at least one tool call
└────────┬────────┘
         │ Creates child jobs
         ▼
┌─────────────────────────────────────────┐
│  Tool Execution Jobs (parallel)         │
│  ┌──────────────┐ ┌──────────────┐      │
│  │ tool_exec_1  │ │ tool_exec_2  │      │  ← Queue citizens with independent status
│  │ JobType:     │ │ JobType:     │      │
│  │ tool_execution│ │ tool_execution│    │
│  └──────────────┘ └──────────────┘      │
└────────┬────────────────────────────────┘
         │ Poll for completion
         ▼
┌─────────────────┐
│   REVIEWER      │  ← LLM assesses if goal achieved
│   (Phase 3)     │     Returns: goal_achieved, confidence, missing_data
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Output Document│
└─────────────────┘
```

## Job Types

Three new job types added to `internal/models/crawler_job.go`:

```go
const (
    JobTypePlanningCall   JobType = "planning_call"   // LLM decides which tools to call
    JobTypeToolExecution  JobType = "tool_execution"  // Individual tool execution job (queue citizen)
    JobTypeSynthesis      JobType = "synthesis"       // LLM formats results with schema constraint
)
```

## Forced Tool Use

The planner system prompt enforces that the AI **must** call tools:

```
CRITICAL RULES - FORCED TOOL USE:
1. You have ZERO knowledge of stock prices, financial data, or any external information
2. You MUST call at least one tool to retrieve ANY data - you cannot answer from memory
3. Return ONLY tool calls - no explanatory text, no analysis, no fabricated data
4. If no tools can answer the query, you MUST return an error - NEVER fabricate data
```

**Source:** `internal/queue/workers/orchestrator_worker.go:47-85`

## Tool Execution as Queue Citizens

Each tool call becomes a separate `tool_execution` job:

1. **Visible in Queue UI** - Each tool job appears with independent status
2. **Parent Linkage** - Jobs reference their orchestrator step as parent
3. **Status Tracking** - Each job has its own pending/running/completed/failed state
4. **Independent Failure** - One tool failing doesn't abort others

**Job Payload:**
```go
jobPayload := map[string]interface{}{
    "plan_step_id":   planStep.ID,
    "tool_name":      planStep.Tool,
    "worker_type":    workerType,
    "params":         planStep.Params,
    "tool_config":    toolConfig,
    "job_def_id":     jobDef.ID,
    "manager_id":     stepID,
    "step_name":      step.Name,
    "original_goal":  goal,
}
```

**Source:** `internal/queue/workers/orchestrator_worker.go:466-477`

## ToolExecutionWorker

A new worker processes `tool_execution` jobs:

```go
type ToolExecutionWorker struct {
    stepManager     interfaces.StepManager
    documentStorage interfaces.DocumentStorage
    searchService   interfaces.SearchService
    jobMgr          *queue.Manager
    logger          arbor.ILogger
}
```

The worker:
1. Parses job config to get tool name, worker type, and params
2. Creates a synthetic `JobStep` with the configuration
3. Executes via `StepManager.Execute()`
4. Updates job metadata with output

**Source:** `internal/queue/workers/tool_execution_worker.go`

## Document Extraction Types

New types for structured data extraction at ingestion time:

```go
// Extract represents structured data extracted from document content
type Extract struct {
    Type       string          `json:"type"`        // "financial", "meeting", "technical"
    Schema     string          `json:"schema"`      // Schema version (e.g., "v1")
    Data       json.RawMessage `json:"data"`        // Structured data
    Confidence float64         `json:"confidence"`  // 1.0 for rule-based, 0.0-1.0 for LLM
    Span       *TextSpan       `json:"span"`        // Location in source
    ExtractedAt string         `json:"extracted_at"` // RFC3339 timestamp
}

// Domain-specific extract types
type FinancialExtract struct { ... }  // Stock/financial data
type MeetingExtract struct { ... }    // Meeting notes/decisions
type APIExtract struct { ... }        // API documentation
```

**Document Model Updated:**
```go
type Document struct {
    // ... existing fields ...

    // Structured extracts - populated at ingestion time
    Extracts    []Extract  `json:"extracts,omitempty"`
    ExtractedAt *time.Time `json:"extracted_at,omitempty"`
    ContentHash string     `json:"content_hash,omitempty"`  // For change detection
}
```

**Source:** `internal/models/document.go:227-345`

## Polling for Tool Job Completion

The orchestrator polls for tool job completion:

```go
func (w *OrchestratorWorker) waitForToolJobs(
    ctx context.Context,
    parentStepID string,
    toolJobIDs []string,
    toolJobMap map[string]PlanStep,
) ([]PlanStepResult, error) {
    // Poll every 2 seconds, max 10 minutes
    for {
        // Check each job status
        for _, jobID := range toolJobIDs {
            jobState, _ := w.jobMgr.GetJob(ctx, jobID)
            if isTerminal(jobState.Status) {
                // Collect result
            }
        }
        if allComplete {
            break
        }
        time.Sleep(2 * time.Second)
    }
    return results, nil
}
```

**Source:** `internal/queue/workers/orchestrator_worker.go:956-1076`

## UI Hierarchy View

After implementation, the queue UI displays:

```
▼ Orchestrator Step: "Analyze BHP" [running]
  ├─ Phase: PLANNER [complete] 0.8s
  │   └─ Selected: get_stock_price, get_financials
  ├─ Tool: get_stock_price [complete] 0.4s
  │   └─ type: tool_execution, params: {ticker: "BHP"}
  ├─ Tool: get_financials [running] ...
  │   └─ type: tool_execution, params: {ticker: "BHP", report_type: "quarterly"}
  └─ Phase: REVIEWER [pending]
```

## Job Definition Configuration

Orchestrated jobs use the `type = "orchestrator"` job type with an `available_tools` configuration:

```toml
[step.analyze_stocks]
type = "orchestrator"
goal = "Perform comprehensive stock analysis..."
thinking_level = "HIGH"
model_preference = "auto"

# Each tool becomes a tool_execution queue job when invoked
available_tools = [
    { name = "fetch_stock_data", description = "Fetch ASX stock prices", worker = "asx_stock_data" },
    { name = "run_stock_review", description = "Execute stock review", worker = "job_template", template = "asx-stock-review" }
]
```

**Tool Configuration Fields:**
- `name`: Tool identifier used by the planner
- `description`: Human-readable description for LLM context
- `worker`: The worker type that executes this tool
- `template`: (optional) For `job_template` workers, the template to execute

## Key Files

| Purpose | File Path |
|---------|-----------|
| Job Types | `internal/models/crawler_job.go:31-34` |
| Orchestrator Worker | `internal/queue/workers/orchestrator_worker.go` |
| Tool Execution Worker | `internal/queue/workers/tool_execution_worker.go` |
| Document Model | `internal/models/document.go` |
| Worker Registration | `internal/app/app.go:766-776` |
| Orchestrated Job Definition | `deployments/common/job-definitions/asx-stocks-daily-orchestrated.toml` |
| Portfolio Orchestrated Job | `deployments/common/job-definitions/smsf-portfolio-daily-orchestrated.toml` |

## Implementation Status

### Completed

- [x] New job types: `planning_call`, `tool_execution`, `synthesis`
- [x] Forced tool use in planner system prompt
- [x] Plan validation (must have steps, check for error flag)
- [x] Tool jobs created as queue citizens
- [x] ToolExecutionWorker to process tool jobs
- [x] Polling for tool job completion
- [x] Worker registration in app.go
- [x] Document Extract types for structured extraction
- [x] Document model with Extracts, ExtractedAt, ContentHash fields

### Future Enhancements

- [ ] Parallel tool execution optimization
- [ ] Synthesis job with JSON schema constraint
- [ ] Rule-based extractors for financial tables
- [ ] LLM-based extractors for unstructured content
- [ ] Extraction job type for visibility
- [ ] Re-extraction triggers on content hash change
- [ ] Recovery loop when reviewer identifies missing data
