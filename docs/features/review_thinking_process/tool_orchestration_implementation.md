# Quaero Tool Orchestration - Implementation Analysis

## Overview

This document compares the original design proposal with the actual implementation in the Quaero codebase, providing an aligned view of how AI-powered tool orchestration works.

---

## Actual Architecture

### Job Hierarchy (3-Level)

The actual implementation uses a 3-level job hierarchy, not the proposed 4-phase flow:

```
┌─────────────────┐
│    Manager      │  depth=0, type="manager"
│  (Root Parent)  │  Top-level orchestrator
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│      Step       │  depth=1, type="step"
│  (Step Job)     │  Container for work units
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Work Jobs     │  depth=2, type=various
│  (Individual)   │  crawler, summarizer, github_repo, etc.
└─────────────────┘
```

**Source:** `internal/models/job_model.go:19-27`

### Job Model Schema

```go
// QueueJob - Immutable job structure
type QueueJob struct {
    ID        string                  `json:"id"`
    ParentID  *string                 `json:"parent_id"`
    ManagerID *string                 `json:"manager_id,omitempty"`
    Type      string                  `json:"type"`
    Name      string                  `json:"name"`
    Config    map[string]interface{}  `json:"config"`
    Metadata  map[string]interface{}  `json:"metadata"`
    CreatedAt time.Time               `json:"created_at"`
    Depth     int                     `json:"depth"`
}

// QueueJobState - Runtime execution state
type QueueJobState struct {
    // ... QueueJob fields ...
    Status        JobStatus   `json:"status"`
    Progress      JobProgress `json:"progress"`
    StartedAt     *time.Time  `json:"started_at,omitempty"`
    CompletedAt   *time.Time  `json:"completed_at,omitempty"`
    Error         string      `json:"error,omitempty"`
    ResultCount   int         `json:"result_count"`
    FailedCount   int         `json:"failed_count"`
}
```

**Source:** `internal/models/job_model.go:29-50, 365-395`

---

## Orchestration Pattern: Planner-Executor-Reviewer

The OrchestratorWorker implements a **3-phase cognitive loop**, not separate queue jobs:

```
┌─────────────────────────────────────────────────────────────────┐
│                   OrchestratorWorker.CreateJobs()               │
│                                                                 │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐    │
│  │   PLANNER    │ ──► │   EXECUTOR   │ ──► │   REVIEWER   │    │
│  │  LLM Call    │     │  Tool Calls  │     │  LLM Call    │    │
│  │  JSON Plan   │     │  Sequential  │     │  Goal Check  │    │
│  └──────────────┘     └──────────────┘     └──────────────┘    │
│         │                    │                    │             │
│         ▼                    ▼                    ▼             │
│     Plan struct        PlanStepResult[]     ReviewResult       │
└─────────────────────────────────────────────────────────────────┘
```

**Source:** `internal/queue/workers/orchestrator_worker.go:323-551`

### Phase 1: Planner

The LLM generates an execution plan with tools to call:

```go
type Plan struct {
    Reasoning string     `json:"reasoning"`
    Steps     []PlanStep `json:"steps"`
}

type PlanStep struct {
    ID        string                 `json:"id"`
    Tool      string                 `json:"tool"`
    Params    map[string]interface{} `json:"params"`
    DependsOn []string               `json:"depends_on,omitempty"`
}
```

**System Prompt (forced JSON output):**
```
You are an intelligent orchestration planner. Your job is to analyze a goal
and create an execution plan using the available tools.

IMPORTANT RULES:
1. Output ONLY valid JSON - no markdown code blocks, no explanations
2. Each step must use exactly one tool from the available_tools list
3. Steps can depend on other steps using the depends_on field
```

**Source:** `internal/queue/workers/orchestrator_worker.go:24-74`

### Phase 2: Executor

Tools are executed via `StepManager.Execute()` - inline, not as separate queue jobs:

```go
func (w *OrchestratorWorker) executeTool(
    ctx context.Context,
    planStep PlanStep,
    toolConfig map[string]interface{},
    jobDef models.JobDefinition,
    parentJobID string,
) PlanStepResult {
    // Get worker type from tool config
    workerType, _ := toolConfig["worker"].(string)

    // Build step config from tool config + plan params
    stepConfig := make(map[string]interface{})
    for k, v := range toolConfig {
        if k != "name" && k != "description" && k != "worker" {
            stepConfig[k] = v
        }
    }
    for k, v := range planStep.Params {
        stepConfig[k] = v
    }

    // Create synthetic JobStep
    syntheticStep := models.JobStep{
        Name:   fmt.Sprintf("orchestrated_%s", planStep.ID),
        Type:   models.WorkerType(workerType),
        Config: stepConfig,
    }

    // Execute via StepManager (synchronous)
    _, err := w.stepManager.Execute(ctx, syntheticStep, jobDef, parentJobID, nil)
    // ...
}
```

**Key Difference from Proposal:** Tools execute inline within the orchestrator step, not as separate queue jobs with independent status tracking.

**Source:** `internal/queue/workers/orchestrator_worker.go:773-872`

### Phase 3: Reviewer

The LLM assesses whether the goal was achieved:

```go
type ReviewResult struct {
    GoalAchieved    bool                     `json:"goal_achieved"`
    Confidence      float64                  `json:"confidence"`
    Summary         string                   `json:"summary"`
    MissingData     []string                 `json:"missing_data"`
    RecoveryActions []map[string]interface{} `json:"recovery_actions"`
}
```

**Source:** `internal/queue/workers/orchestrator_worker.go:709-716`

---

## Worker Types

The system has 24+ worker types defined:

| Category | Worker Types |
|----------|-------------|
| **Data Collection** | `crawler`, `github_repo`, `github_actions`, `github_git`, `local_dir`, `places_search`, `web_search`, `asx_announcements`, `asx_stock_data` |
| **Processing** | `agent`, `transform`, `reindex`, `code_map`, `summary` |
| **Enrichment** | `analyze_build`, `classify`, `dependency_graph`, `aggregate_summary` |
| **Communication** | `email`, `email_watcher` |
| **Advanced** | `orchestrator`, `job_template`, `test_job_generator` |

**Source:** `internal/models/worker_type.go`

---

## Document Model

### Actual Schema

```go
type Document struct {
    ID              string                 `json:"id"`           // doc_{uuid}
    SourceType      string                 `json:"source_type"`  // jira, confluence, github, crawler
    SourceID        string                 `json:"source_id"`
    Title           string                 `json:"title"`
    ContentMarkdown string                 `json:"content_markdown"` // PRIMARY CONTENT
    DetailLevel     string                 `json:"detail_level"`     // "metadata" or "full"
    Metadata        map[string]interface{} `json:"metadata"`
    URL             string                 `json:"url"`
    Tags            []string               `json:"tags"`
    LastSynced      *time.Time             `json:"last_synced,omitempty"`
    SourceVersion   string                 `json:"source_version,omitempty"`
    CreatedAt       time.Time              `json:"created_at"`
    UpdatedAt       time.Time              `json:"updated_at"`
}
```

**Source:** `internal/models/document.go:49-77`

### Key Design: Markdown-First + Structured Metadata

The document model follows a two-part architecture:

1. **ContentMarkdown** - Clean, unified text format for AI processing and full-text search
2. **Metadata** - Structured JSON with source-specific data for efficient filtering

**Query Pattern:**
```
Step 1: Filter documents using structured metadata (SQL WHERE on JSON fields)
Step 2: Reason and synthesize from clean Markdown content of filtered results
```

**Source:** `internal/models/document.go:1-48`

### Source-Specific Metadata Types

```go
// Jira
type JiraMetadata struct {
    IssueKey, ProjectKey, IssueType, Status, Priority string
    Assignee, Reporter string
    Labels, Components []string
}

// Confluence
type ConfluenceMetadata struct {
    PageID, PageTitle, SpaceKey, SpaceName string
    Author string
    Version int
}

// GitHub
type GitHubMetadata struct {
    RepoName, FilePath, CommitSHA, Branch string
    FunctionName, Author string
}

// Code Map
type CodeMapMetadata struct {
    NodeType string  // "project", "directory", "file"
    Languages []string
    Summary, Purpose string  // AI-generated
    KeyConcepts []string
}
```

**Source:** `internal/models/document.go:82-194`

---

## Extraction Pipeline

### Actual Implementation: Crawl-Time Processing

Extraction happens at **crawl-time**, not query-time, through the ContentProcessor:

```
Seed URL → ChromeDP Render → HTML
         ↓
ContentProcessor.ProcessHTML()
  - Extract title (OG, Twitter, h1, <title>)
  - Extract links
  - Extract metadata
  - Convert to Markdown
         ↓
CrawledDocument.NewCrawledDocument()
  + Set crawler metadata (depth, status, response time)
         ↓
DocumentPersister.SaveCrawledDocument()
         ↓
Document with ContentMarkdown + Metadata
```

**Source:** `internal/services/crawler/content_processor.go`

### Metadata Extractors

The system has specialized extractors:

1. **HTML Metadata Extractor** (`content_processor.go`)
   - Open Graph tags
   - Twitter Card tags
   - JSON-LD structured data
   - Standard HTML meta tags

2. **Cross-Reference Extractor** (`internal/services/metadata/extractor.go`)
   - JIRA issue keys: `[A-Z]+-\d+`
   - User mentions: `@\w+`
   - PR references: `#\d+`
   - Confluence page references

3. **Identifier Extractor** (`internal/services/identifiers/extractor.go`)
   - Git commit SHAs
   - GitHub PR numbers
   - Jira-style issue keys

---

## Comparison: Proposed vs. Implemented

| Aspect | Proposed Design | Actual Implementation |
|--------|-----------------|----------------------|
| **Job Flow** | 4-phase: Planning → Tool Jobs → Synthesis | 3-level hierarchy: Manager → Step → Jobs |
| **Tool Execution** | Separate queue jobs with independent status | Inline execution via StepManager.Execute() |
| **Visibility** | Each tool call visible as queue citizen | Tools execute within orchestrator step |
| **Document Extraction** | `Extract` type with Type, Schema, Data, Confidence | Metadata map with source-specific types |
| **Extraction Timing** | Ingestion-time with separate extraction jobs | Crawl-time via ContentProcessor |
| **Schema Enforcement** | JSON schema constraint on synthesis | JSON output format in system prompts |
| **Tool Definitions** | Separate schema and executor registry | Combined in `available_tools` config |

---

## Configuration Example

### OrchestratorWorker Step Config

```toml
[[steps]]
name = "analyze_portfolio"
type = "orchestrator"
description = "AI-powered portfolio analysis"

[steps.config]
goal = "Analyze the portfolio stocks and generate a performance report"
thinking_level = "MEDIUM"  # MINIMAL, LOW, MEDIUM, HIGH
model_preference = "auto"  # auto, flash, pro

[[steps.config.available_tools]]
name = "fetch_stock_data"
description = "Fetch current stock price and historical data"
worker = "asx_stock_data"

[[steps.config.available_tools]]
name = "search_documents"
description = "Search knowledge base for relevant information"
worker = "web_search"
```

**Source:** Job definition TOML format in `internal/models/job_definition.go`

---

## Key Files Reference

| Purpose | File Path |
|---------|-----------|
| Job Model | `internal/models/job_model.go` |
| Document Model | `internal/models/document.go` |
| Worker Types | `internal/models/worker_type.go` |
| Orchestrator Worker | `internal/queue/workers/orchestrator_worker.go` |
| Job Processor | `internal/queue/workers/job_processor.go` |
| Job Dispatcher | `internal/queue/dispatcher.go` |
| Content Processor | `internal/services/crawler/content_processor.go` |
| LLM Provider | `internal/services/llm/provider.go` |
| Tool Router (MCP) | `internal/services/mcp/router.go` |

---

## Implementation Status

### Implemented

- [x] 3-level job hierarchy (Manager → Step → Jobs)
- [x] Planner-Executor-Reviewer loop in OrchestratorWorker
- [x] Tool execution via StepManager
- [x] LLM planning with JSON output
- [x] LLM review with goal achievement check
- [x] Document model with ContentMarkdown + Metadata
- [x] Crawl-time metadata extraction
- [x] 24+ worker types
- [x] Multi-provider LLM support (Gemini, Claude)

### Not Implemented (From Original Proposal)

- [ ] Separate queue jobs for each tool call
- [ ] Independent status tracking per tool
- [ ] `Extract` type with schema versioning
- [ ] Separate extraction job type
- [ ] Rule-based extractors for structured content
- [ ] Confidence scores on extractions
- [ ] Re-extraction triggers on content hash change
- [ ] Forced tool use (`ToolChoice: required`)
- [ ] Output schema validation in synthesis

### Future Considerations

1. **Tool Job Visibility**: Consider exposing tool executions as child jobs for better observability
2. **Schema-Constrained Output**: Add JSON schema validation to synthesis responses
3. **Extraction Confidence**: Add confidence scoring to metadata extraction
4. **Recovery Loop**: Implement automatic recovery when reviewer identifies missing data
