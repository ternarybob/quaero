# Orchestrator Worker Process Flow

This document traces the data flow and processing for the Orchestrator Worker, identifying LLM calls, tool execution, and providing conclusions on efficiency improvements.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Process Flow](#process-flow)
- [LLM Usage](#llm-usage)
- [Tool Execution](#tool-execution)
- [Conclusions and Recommendations](#conclusions-and-recommendations)

---

## Overview

The Orchestrator Worker (`internal/queue/workers/orchestrator_worker.go`) is the **AI-powered cognitive orchestration layer** that uses LLM reasoning to dynamically plan and execute workflows based on natural language goals. It implements the **Planner-Executor-Reviewer** pattern.

### Key Characteristics

| Aspect | Description |
|--------|-------------|
| **LLM Usage** | 2 calls per execution (Planner + Reviewer) |
| **Tool Execution** | Delegates to other workers (no direct API calls) |
| **Child Jobs** | Creates queue jobs for each tool invocation |
| **Processing** | Inline execution with async tool job polling |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       ORCHESTRATOR WORKER ARCHITECTURE                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────────┐                                                       │
│  │   JOB DEFINITION │  Variables, Benchmarks, Goal Template                 │
│  │    (TOML/JSON)   │                                                       │
│  └────────┬─────────┘                                                       │
│           │                                                                 │
│           ▼                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        ORCHESTRATOR WORKER                           │   │
│  ├─────────────────────────────────────────────────────────────────────┤   │
│  │                                                                      │   │
│  │  PHASE 1: PLANNER ───────────────────────────────────────────────── │   │
│  │  │                                                                   │   │
│  │  ├─ Read variables from job definition [config]                     │   │
│  │  ├─ Load goal_template if specified (from job-templates/)           │   │
│  │  ├─ Format variables + benchmarks as context for LLM                │   │
│  │  ├─ Format available_tools for LLM                                  │   │
│  │  │                                                                   │   │
│  │  └─ LLM CALL #1 (Planner) ────────────────────────────────────────  │   │
│  │     ├─ System: plannerSystemPrompt (FORCED TOOL USE)                │   │
│  │     ├─ User: GOAL + CONTEXT + TOOLS                                 │   │
│  │     ├─ Model: Based on model_preference (flash/pro/opus/sonnet)     │   │
│  │     └─ Output: JSON Plan with steps[]                                │   │
│  │                                                                      │   │
│  │  PHASE 2: EXECUTOR ──────────────────────────────────────────────── │   │
│  │  │                                                                   │   │
│  │  ├─ Validate plan steps (check mutually exclusive tags)             │   │
│  │  ├─ Build tool lookup map from available_tools                      │   │
│  │  ├─ Identify terminal steps (for output_tags)                       │   │
│  │  │                                                                   │   │
│  │  └─ Execute in WAVES (dependency ordering) ─────────────────────    │   │
│  │     │                                                                │   │
│  │     ├─ Wave 1: Steps with no dependencies (data collection)         │   │
│  │     │   └─ CreateChildJob() for each step                           │   │
│  │     │   └─ Poll for completion (2s interval)                        │   │
│  │     │                                                                │   │
│  │     ├─ Wave 2: Steps depending on Wave 1 (analysis)                 │   │
│  │     │   └─ CreateChildJob() for each step                           │   │
│  │     │   └─ Poll for completion                                      │   │
│  │     │                                                                │   │
│  │     └─ Wave N: Continue until all steps complete                    │   │
│  │                                                                      │   │
│  │  PHASE 3: REVIEWER ──────────────────────────────────────────────── │   │
│  │  │                                                                   │   │
│  │  └─ LLM CALL #2 (Reviewer) ───────────────────────────────────────  │   │
│  │     ├─ System: reviewerSystemPrompt                                 │   │
│  │     ├─ User: ORIGINAL GOAL + EXECUTION RESULTS                      │   │
│  │     └─ Output: JSON ReviewResult {goal_achieved, confidence, ...}   │   │
│  │                                                                      │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│           │                                                                 │
│           ▼                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        OUTPUT DOCUMENTS                              │   │
│  │   • Worker outputs (stock data, announcements, analyses)            │   │
│  │   • Orchestrator execution log (internal, not for email)            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Process Flow

### Phase 1: PLANNER

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           PLANNER PHASE                                    │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. INIT                                                                   │
│     ├── Load goal_template if specified                                   │
│     │   └── Template provides: goal, thinking_level, model_preference,   │
│     │       output_tags, available_tools, output_schema                   │
│     ├── Merge step config overrides (step-level wins)                     │
│     ├── Extract variables from jobDef.Config["variables"]                 │
│     └── Extract benchmarks from jobDef.Config["benchmarks"]               │
│                                                                            │
│  2. FORMAT CONTEXT                                                         │
│     └── formatVariablesAsContext():                                       │
│         ├── "=== STOCKS TO ANALYZE (PRIMARY TARGETS) ==="                 │
│         │   └── JSON array of stock variables                             │
│         └── "=== BENCHMARK INDICES (for comparison only) ==="             │
│             └── JSON array of benchmark indices                            │
│                                                                            │
│  3. FORMAT TOOLS                                                           │
│     └── formatToolsForPrompt():                                           │
│         ├── "1. fetch_stock_data (worker: asx_stock_collector)"           │
│         ├── "2. fetch_announcements (worker: asx_announcements)"          │
│         ├── "3. search_web (worker: web_search)"                          │
│         └── "4. analyze_summary (worker: summary)"                         │
│                                                                            │
│  4. LLM CALL                                                               │
│     ├── System Prompt: plannerSystemPrompt (enforces FORCED TOOL USE)    │
│     ├── User Prompt: GOAL + CONTEXT + TOOLS                               │
│     ├── Model Selection:                                                  │
│     │   ├── "flash" → gemini-3-flash-preview                             │
│     │   ├── "pro" → gemini-3-pro-preview                                  │
│     │   ├── "opus" → claude-opus-4-5-20251101                            │
│     │   ├── "sonnet" → claude-sonnet-4-5-20250929                        │
│     │   └── "haiku" → claude-haiku-4-5-20251001                          │
│     └── ThinkingLevel: MINIMAL, LOW, MEDIUM, HIGH                         │
│                                                                            │
│  5. PARSE RESPONSE                                                         │
│     ├── Strip markdown code blocks                                        │
│     ├── Parse JSON into Plan struct                                       │
│     ├── Validate: plan.Error == false                                     │
│     ├── Validate: len(plan.Steps) > 0 (forced tool use)                   │
│     └── Validate: validatePlanSteps() (check mutually exclusive tags)     │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

### Planner System Prompt Key Rules

The `plannerSystemPrompt` enforces:

1. **FORCED TOOL USE**: LLM cannot answer from memory, must call tools
2. **Dependency Ordering**: Analysis steps must depend on data collection steps
3. **Tag Filtering Rules**:
   - Each document type has specific tags
   - Mutually exclusive tags cannot be combined (AND logic)
   - Separate `analyze_summary` calls for each document type

4. **Stocks vs Benchmarks**:
   - Variables (stocks) = PRIMARY TARGETS requiring full analysis
   - Benchmarks = SECONDARY reference data for comparison only

### Phase 2: EXECUTOR

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           EXECUTOR PHASE                                   │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. BUILD LOOKUPS                                                          │
│     ├── toolLookup: map[toolName] → toolConfig                            │
│     ├── stepLookup: map[stepID] → PlanStep                                │
│     └── terminalSteps: map[stepID] → bool (steps no one depends on)       │
│                                                                            │
│  2. WAVE EXECUTION LOOP (max 10 waves)                                     │
│     │                                                                      │
│     ├── Find steps for this wave:                                          │
│     │   └── Steps where ALL dependencies are completed                    │
│     │                                                                      │
│     ├── For each step in wave:                                            │
│     │   │                                                                  │
│     │   ├── Lookup tool config from available_tools                       │
│     │   ├── Get worker type (e.g., asx_stock_collector)                   │
│     │   │                                                                  │
│     │   ├── Build job payload:                                            │
│     │   │   ├── plan_step_id, tool_name, worker_type                      │
│     │   │   ├── params (from plan step)                                   │
│     │   │   ├── tool_config (from available_tools)                        │
│     │   │   ├── job_def_id, manager_id (orchestrator step ID)            │
│     │   │   │                                                              │
│     │   │   └── For TERMINAL analyze_summary steps ONLY:                  │
│     │   │       ├── output_tags (from template/step config)               │
│     │   │       ├── required_tickers (from variables)                     │
│     │   │       ├── benchmark_codes (from benchmarks)                     │
│     │   │       └── output_schema (for structured JSON output)            │
│     │   │                                                                  │
│     │   └── jobMgr.CreateChildJob(JobTypeToolExecution)                   │
│     │                                                                      │
│     ├── waitForToolJobs() - Poll for wave completion:                     │
│     │   ├── Poll interval: 2 seconds                                      │
│     │   ├── Max wait: 10 minutes                                          │
│     │   ├── Check status: completed, failed, cancelled                    │
│     │   └── Collect PlanStepResult for each job                           │
│     │                                                                      │
│     └── Mark steps as completed, continue to next wave                    │
│                                                                            │
│  3. VERIFY COMPLETION                                                      │
│     └── Log warning for any steps not executed (cycle detection)          │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

### Phase 3: REVIEWER

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           REVIEWER PHASE                                   │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. FORMAT EXECUTION RESULTS                                               │
│     └── For each result:                                                  │
│         ├── Step: {id} (tool: {name})                                     │
│         ├── Status: SUCCESS/FAILED                                        │
│         └── Output/Error: {truncated to 1000 chars}                       │
│                                                                            │
│  2. LLM CALL                                                               │
│     ├── System Prompt: reviewerSystemPrompt                               │
│     ├── User Prompt: ORIGINAL GOAL + EXECUTION RESULTS                    │
│     └── ThinkingLevel: Same as planner                                    │
│                                                                            │
│  3. PARSE RESPONSE                                                         │
│     └── JSON ReviewResult:                                                │
│         ├── goal_achieved: bool                                           │
│         ├── confidence: 0.0-1.0                                           │
│         ├── summary: string                                               │
│         ├── missing_data: []string                                        │
│         └── recovery_actions: []map (suggested fixes)                     │
│                                                                            │
│  4. RESULT HANDLING                                                        │
│     ├── Log review status to job logs                                     │
│     ├── If goal_achieved == false:                                        │
│     │   └── Return error (marks step as failed)                           │
│     └── TODO: Implement recovery loop with suggested actions              │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## LLM Usage

### LLM Call Summary

| Phase | Purpose | Model | System Prompt | Output Format |
|-------|---------|-------|---------------|---------------|
| Planner | Generate execution plan | Configurable | plannerSystemPrompt | JSON Plan |
| Reviewer | Assess goal achievement | Same | reviewerSystemPrompt | JSON ReviewResult |

### Model Selection

The `model_preference` config option controls which LLM is used:

| Preference | Provider | Model ID |
|------------|----------|----------|
| `flash`, `gemini-flash` | Gemini | gemini-3-flash-preview |
| `pro`, `gemini-pro` | Gemini | gemini-3-pro-preview |
| `claude`, `sonnet`, `claude-sonnet` | Claude | claude-sonnet-4-5-20250929 |
| `opus`, `claude-opus` | Claude | claude-opus-4-5-20251101 |
| `haiku`, `claude-haiku` | Claude | claude-haiku-4-5-20251001 |
| `auto` (default) | Config | llm.default_provider setting |

### Thinking Levels

| Level | Use Case |
|-------|----------|
| MINIMAL | Quick planning, minimal reasoning |
| LOW | Light reasoning, fast execution |
| MEDIUM | Balanced reasoning (default) |
| HIGH | Deep reasoning, thorough analysis |

---

## Tool Execution

### Tool Execution Job Flow

When the orchestrator calls a tool (e.g., `fetch_stock_data`), it creates a **queue citizen job** that is processed by the tool execution system:

```
Orchestrator Step                     Queue System
      │                                    │
      ├─ CreateChildJob(                   │
      │    type: "tool_execution",         │
      │    payload: {                      │
      │      worker_type: "asx_stock_collector",
      │      params: {asx_code: "GNP"}     │
      │    }                               │
      │  )                                 │
      │                                    │
      │ ──────────────────────────────────►│
      │                                    │
      │                            Queue picks up job
      │                            Dispatches to worker
      │                            Worker fetches data
      │                            Worker saves document
      │                                    │
      │◄───────────────────────────────────│
      │                                    │
      │  Job status: completed             │
      │  Document created with tags        │
      │                                    │
```

### Available Tools Mapping

Typical tool configuration in goal templates:

| Tool Name | Worker Type | Description |
|-----------|-------------|-------------|
| `fetch_stock_data` | `asx_stock_collector` | Fetch comprehensive stock data from EODHD |
| `fetch_index_data` | `asx_index_data` | Fetch benchmark index data |
| `fetch_announcements` | `asx_announcements` | Fetch ASX company announcements |
| `search_web` | `web_search` | Search web for news/information |
| `analyze_summary` | `summary` | LLM-powered document analysis |

### Terminal Step Handling

The orchestrator identifies **terminal steps** (steps that no other step depends on) and adds special handling for `analyze_summary` terminal steps:

1. **output_tags**: Applied to final analysis document (for downstream email)
2. **required_tickers**: Validates all stocks from variables appear in output
3. **benchmark_codes**: Validates benchmarks aren't treated as primary targets
4. **output_schema**: Enables structured JSON output with schema validation

---

## Conclusions and Recommendations

### Current Architecture Analysis

#### Strengths

1. **Cognitive Planning**: LLM-powered planning adapts to goal requirements
2. **Wave Execution**: Dependency-aware execution ensures correct ordering
3. **Queue Citizens**: Tool jobs are visible in queue with independent tracking
4. **Validation**: Plan validation catches common mistakes (mutually exclusive tags)
5. **Output Validation**: Terminal steps validate required tickers and benchmarks

#### Identified Issues

Based on the user's reported validation concerns (incorrect stock prices, non-existent announcements):

1. **Data Staleness**: The orchestrator doesn't validate data freshness from child workers
2. **No Data Verification**: Child job success != data quality
3. **LLM Hallucination Risk**: Without output validation, LLM may fabricate analysis

### Recommendations for Efficiency and Effectiveness

#### 1. Add Data Quality Verification to Tool Jobs

The orchestrator should verify that tool jobs produced valid data, not just completed successfully:

```go
// After waitForToolJobs(), verify data quality
func (w *OrchestratorWorker) verifyDataQuality(ctx context.Context, results []PlanStepResult) error {
    for _, result := range results {
        if !result.Success {
            continue
        }

        // Check document was actually created
        if result.Tool == "fetch_stock_data" {
            ticker := extractTickerFromParams(result.Params)
            doc, err := w.documentStorage.GetDocumentBySource("asx_stock_collector",
                fmt.Sprintf("asx:%s:stock_collector", ticker))
            if err != nil || doc == nil {
                return fmt.Errorf("tool %s completed but no document found for %s",
                    result.Tool, ticker)
            }

            // Verify data freshness
            if doc.LastSynced != nil && time.Since(*doc.LastSynced) > 24*time.Hour {
                return fmt.Errorf("stale data for %s: last synced %v ago",
                    ticker, time.Since(*doc.LastSynced))
            }
        }
    }
    return nil
}
```

#### 2. Enhanced Planner Prompt for Data Validation

Add explicit instructions to the planner system prompt:

```
DATA VALIDATION REQUIREMENTS:
1. After data collection, ALWAYS call analyze_summary to verify data was retrieved
2. The analyze_summary step should include a validation check in its prompt
3. If data is missing or stale, the reviewer should flag goal_achieved = false
```

#### 3. Implement Recovery Loop

The reviewer already suggests `recovery_actions`, but they're not implemented:

```go
// After review, implement recovery loop
if !review.GoalAchieved && len(review.RecoveryActions) > 0 {
    // Re-run failed steps with recovery actions
    for _, action := range review.RecoveryActions {
        // Execute recovery action as new tool call
    }
}
```

#### 4. Add Schema Validation for Tool Outputs

Extend the output_schema feature to validate tool job outputs:

```go
// Validate tool job output against expected schema
func (w *OrchestratorWorker) validateToolOutput(result PlanStepResult, expectedSchema map[string]interface{}) error {
    if result.Tool == "fetch_stock_data" {
        // Check required fields exist
        requiredFields := []string{"current_price", "market_cap", "trend_signal"}
        // Validate against schema
    }
    return nil
}
```

#### 5. Parallel Tool Execution Within Waves

Currently, jobs within a wave are created sequentially then polled together. Consider fully async creation:

```go
// Create all wave jobs in parallel using goroutines
var wg sync.WaitGroup
var mu sync.Mutex
var waveJobIDs []string

for _, planStep := range waveSteps {
    wg.Add(1)
    go func(ps PlanStep) {
        defer wg.Done()
        jobID, err := w.jobMgr.CreateChildJob(...)
        if err == nil {
            mu.Lock()
            waveJobIDs = append(waveJobIDs, jobID)
            mu.Unlock()
        }
    }(planStep)
}
wg.Wait()
```

### Summary

| Component | LLM Calls | API Calls | Issue Area |
|-----------|-----------|-----------|------------|
| Orchestrator (Planner) | 1 | None | Plan quality depends on context |
| Orchestrator (Executor) | 0 | None (delegates) | No data quality verification |
| Orchestrator (Reviewer) | 1 | None | Recovery loop not implemented |
| Tool Jobs (e.g., asx_stock_collector) | 0 | EODHD API | Data freshness |
| Terminal analyze_summary | 1+ | None | Output validation exists |

**Root Cause of User's Issue**: The orchestrator marks jobs as successful based on completion status, not data quality. If EODHD returns stale/empty data, or if the ASX HTML parser fails, the orchestrator doesn't detect this.

**Primary Recommendation**: Add data quality verification after tool job completion, checking that documents exist and contain fresh, valid data before proceeding to analysis steps.
