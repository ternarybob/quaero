# Quaero Tool Orchestration Refactor

## Context

Quaero is a Go-based AI knowledge aggregation system. The current implementation has an issue where AI requests that require tool use (e.g., stock analysis) result in the AI returning fabricated data from its training rather than executing the defined tools. Additionally, tool execution is hidden within a single request rather than being visible as discrete queue jobs.

## Current Problem

1. **AI bypasses tools**: Despite tools being passed to the LLM (Gemini/Claude SDK), the AI often returns answers from its own knowledge rather than calling tools
2. **No visibility**: Tool calls happen inside a single request - they're not visible in the job queue
3. **Uncontrolled output**: AI response structure varies unpredictably

## Desired Architecture

### Job Flow

```
┌─────────────────┐
│  User Request   │
│  (Analysis Job) │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Planning Job   │  ← AI decides which tools, returns structured plan only
│  (LLM Call)     │
└────────┬────────┘
         │ Creates child jobs
         ▼
┌─────────────────────────────────────────┐
│  Tool Jobs (parallel/sequential)        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │ Tool A   │ │ Tool B   │ │ Tool C   │ │  ← Go code executes, no LLM
│  └──────────┘ └──────────┘ └──────────┘ │
└────────┬────────────────────────────────┘
         │ All complete
         ▼
┌─────────────────┐
│  Synthesis Job  │  ← AI formats results, constrained output schema
│  (LLM Call)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Final Response │
└─────────────────┘
```

### Key Principles

1. **AI as planner, not executor**: The planning LLM call returns tool specifications only - it never receives tool implementations or executes them
2. **Tool jobs are queue citizens**: Each tool call is a separate job with parent reference, visible in UI, with independent status tracking
3. **Forced tool use**: Planning call MUST return tool calls - if it can't, it returns an error, never fabricated data
4. **Controlled synthesis**: Final response uses JSON schema constraint to enforce structure

## Implementation Specification

### 1. Job Types

Extend the job system with these types:

```go
const (
    JobTypeAnalysisRequest  = "analysis_request"   // Parent job
    JobTypePlanningCall     = "planning_call"      // LLM decides tools
    JobTypeToolExecution    = "tool_execution"     // Go code runs tool
    JobTypeSynthesis        = "synthesis"          // LLM formats output
)
```

### 2. Job Schema

```go
type Job struct {
    ID          string          `json:"id"`
    Type        string          `json:"type"`
    ParentID    *string         `json:"parent_id,omitempty"`    // Links child to parent
    Status      string          `json:"status"`                  // pending, running, complete, failed
    Input       json.RawMessage `json:"input"`
    Output      json.RawMessage `json:"output,omitempty"`
    Children    []string        `json:"children,omitempty"`      // Child job IDs
    CreatedAt   time.Time       `json:"created_at"`
    CompletedAt *time.Time      `json:"completed_at,omitempty"`
}
```

### 3. Planning Call Implementation

```go
func (s *Service) executePlanningJob(ctx context.Context, job *Job) error {
    var input PlanningInput
    json.Unmarshal(job.Input, &input)
    
    // Build tool definitions (schemas only, no implementations)
    tools := s.buildToolSchemas(input.AvailableTools)
    
    // System prompt that forces tool use
    systemPrompt := `You are a planning agent. Your ONLY job is to decide which tools to call.

CRITICAL RULES:
- You have ZERO knowledge of stock prices, financial data, or any external information
- You MUST call tools to retrieve ANY data - you cannot answer from memory
- Return ONLY tool calls - no explanatory text, no analysis, no data
- If no tools can answer the query, return the error tool with reason

Available tools will be provided. Select the minimum set needed to answer the query.`

    resp, err := s.llmClient.CreateMessage(ctx, &MessageRequest{
        System:   systemPrompt,
        Messages: []Message{{Role: "user", Content: input.Query}},
        Tools:    tools,
        ToolChoice: &ToolChoice{Type: "required"},  // FORCE tool use
    })
    
    if err != nil {
        return err
    }
    
    // Extract tool calls - DO NOT EXECUTE
    var toolCalls []ToolCall
    for _, block := range resp.Content {
        if block.Type == "tool_use" {
            toolCalls = append(toolCalls, ToolCall{
                ID:     block.ID,
                Name:   block.Name,
                Params: block.Input,
            })
        }
    }
    
    if len(toolCalls) == 0 {
        return errors.New("planning failed: no tool calls returned")
    }
    
    // Create child jobs for each tool call
    childIDs := make([]string, 0, len(toolCalls))
    for _, tc := range toolCalls {
        childJob := &Job{
            ID:       uuid.New().String(),
            Type:     JobTypeToolExecution,
            ParentID: &job.ID,
            Status:   "pending",
            Input:    mustMarshal(tc),
        }
        s.queue.Enqueue(childJob)
        childIDs = append(childIDs, childJob.ID)
    }
    
    job.Children = childIDs
    job.Output = mustMarshal(PlanningOutput{ToolCalls: toolCalls})
    
    return nil
}
```

### 4. Tool Execution (No LLM)

```go
func (s *Service) executeToolJob(ctx context.Context, job *Job) error {
    var tc ToolCall
    json.Unmarshal(job.Input, &tc)
    
    // Direct Go execution - no LLM involved
    executor, ok := s.toolExecutors[tc.Name]
    if !ok {
        return fmt.Errorf("unknown tool: %s", tc.Name)
    }
    
    result, err := executor.Execute(ctx, tc.Params)
    if err != nil {
        job.Output = mustMarshal(ToolResult{Error: err.Error()})
        return err
    }
    
    job.Output = mustMarshal(ToolResult{Data: result})
    
    // Check if all siblings complete, trigger synthesis
    s.checkAndTriggerSynthesis(ctx, *job.ParentID)
    
    return nil
}
```

### 5. Synthesis with Schema Constraint

```go
func (s *Service) executeSynthesisJob(ctx context.Context, job *Job) error {
    var input SynthesisInput
    json.Unmarshal(job.Input, &input)
    
    // Build context from tool results
    var resultsContext strings.Builder
    resultsContext.WriteString("## Tool Results\n\n")
    for _, tr := range input.ToolResults {
        resultsContext.WriteString(fmt.Sprintf("### %s\n```json\n%s\n```\n\n", 
            tr.ToolName, tr.Data))
    }
    
    systemPrompt := `You are a synthesis agent. Format the provided tool results into a structured response.

CRITICAL RULES:
- Use ONLY data from the tool results below - add nothing from your own knowledge
- If tool results are incomplete or errored, report that honestly
- Follow the exact output schema provided`

    // Define output schema
    outputSchema := s.getOutputSchema(input.ResponseType)
    
    resp, err := s.llmClient.CreateMessage(ctx, &MessageRequest{
        System: systemPrompt,
        Messages: []Message{
            {Role: "user", Content: fmt.Sprintf("Original query: %s\n\n%s", 
                input.OriginalQuery, resultsContext.String())},
        },
        ResponseFormat: &ResponseFormat{
            Type:   "json_schema",
            Schema: outputSchema,
        },
    })
    
    if err != nil {
        return err
    }
    
    job.Output = mustMarshal(resp.Content[0].Text)
    return nil
}
```

### 6. Parent Job Orchestration

```go
func (s *Service) processAnalysisRequest(ctx context.Context, job *Job) error {
    // Create planning job as first child
    planningJob := &Job{
        ID:       uuid.New().String(),
        Type:     JobTypePlanningCall,
        ParentID: &job.ID,
        Status:   "pending",
        Input:    job.Input,
    }
    
    s.queue.Enqueue(planningJob)
    job.Children = []string{planningJob.ID}
    job.Status = "waiting_for_children"
    
    return nil
}

func (s *Service) checkAndTriggerSynthesis(ctx context.Context, parentID string) {
    parent, _ := s.queue.Get(parentID)
    
    // Get all tool execution children
    toolJobs := s.queue.GetChildrenByType(parentID, JobTypeToolExecution)
    
    allComplete := true
    var results []ToolResult
    for _, tj := range toolJobs {
        if tj.Status != "complete" && tj.Status != "failed" {
            allComplete = false
            break
        }
        var tr ToolResult
        json.Unmarshal(tj.Output, &tr)
        results = append(results, tr)
    }
    
    if !allComplete {
        return
    }
    
    // All tools done - create synthesis job
    synthJob := &Job{
        ID:       uuid.New().String(),
        Type:     JobTypeSynthesis,
        ParentID: &parentID,
        Status:   "pending",
        Input: mustMarshal(SynthesisInput{
            OriginalQuery: parent.Input,
            ToolResults:   results,
            ResponseType:  "stock_analysis", // or derive from request
        }),
    }
    
    s.queue.Enqueue(synthJob)
    parent.Children = append(parent.Children, synthJob.ID)
}
```

### 7. Tool Definitions

Tools are defined with schema only - execution is separate:

```go
type ToolDefinition struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`  // JSON Schema
}

var stockTools = []ToolDefinition{
    {
        Name:        "get_stock_price",
        Description: "Get current and historical price data for an ASX stock",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "ticker": {"type": "string", "description": "ASX ticker symbol e.g. BHP"},
                "period": {"type": "string", "enum": ["1d", "5d", "1m", "3m", "1y"]}
            },
            "required": ["ticker"]
        }`),
    },
    {
        Name:        "get_financials",
        Description: "Get financial statements and ratios for a company",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "ticker": {"type": "string"},
                "report_type": {"type": "string", "enum": ["quarterly", "annual"]}
            },
            "required": ["ticker"]
        }`),
    },
    // ... more tools
}

// Separate executor registry
var toolExecutors = map[string]ToolExecutor{
    "get_stock_price": &StockPriceExecutor{},
    "get_financials":  &FinancialsExecutor{},
}
```

### 8. Output Schemas

Define strict schemas for synthesis output:

```go
var stockAnalysisSchema = json.RawMessage(`{
    "type": "object",
    "properties": {
        "ticker": {"type": "string"},
        "summary": {"type": "string", "maxLength": 500},
        "current_price": {
            "type": "object",
            "properties": {
                "value": {"type": "number"},
                "currency": {"type": "string"},
                "as_of": {"type": "string", "format": "date-time"}
            }
        },
        "metrics": {
            "type": "object",
            "properties": {
                "pe_ratio": {"type": ["number", "null"]},
                "market_cap": {"type": ["number", "null"]},
                "dividend_yield": {"type": ["number", "null"]}
            }
        },
        "data_sources": {
            "type": "array",
            "items": {"type": "string"},
            "description": "List of tools that provided data"
        },
        "data_gaps": {
            "type": "array", 
            "items": {"type": "string"},
            "description": "Any requested data that could not be retrieved"
        }
    },
    "required": ["ticker", "summary", "data_sources"]
}`)
```

## UI Considerations

The job queue UI should display:

1. **Parent job** with expandable children
2. **Status badges** for each job phase
3. **Tool parameters** visible on tool jobs
4. **Timing** for each phase
5. **Raw output** expandable for debugging

Example hierarchy view:
```
▼ Analysis: "Analyse BHP for investment" [complete] 2.3s
  ├─ Planning [complete] 0.8s
  │   └─ Selected: get_stock_price, get_financials
  ├─ Tool: get_stock_price {ticker: "BHP"} [complete] 0.4s
  ├─ Tool: get_financials {ticker: "BHP", report_type: "quarterly"} [complete] 0.6s
  └─ Synthesis [complete] 0.5s
```

## Structured Document Processing

### Problem

Raw markdown documents passed as context are unreliable - the AI frequently fails to locate relevant information even when present. The model becomes a "needle finder" which is flaky and unpredictable.

### Solution: Ingestion-Time Extraction

Extract structured data **when documents are crawled**, not at query time. Store both raw and structured representations.

```
┌─────────────────┐
│  Crawl Source   │  (Confluence, GitHub, SharePoint, etc.)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Raw Markdown   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Extraction     │  ← Rule-based OR one-time LLM extraction
│  Pipeline       │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────┐
│  Document Store                     │
│  ┌─────────────┐ ┌────────────────┐ │
│  │ Raw         │ │ Structured     │ │
│  │ Markdown    │ │ Extracts       │ │
│  └─────────────┘ └────────────────┘ │
└─────────────────────────────────────┘
```

### Document Schema

```go
type Document struct {
    ID            string          `json:"id"`
    SourceType    string          `json:"source_type"`    // confluence, github, etc.
    SourceURL     string          `json:"source_url"`
    Title         string          `json:"title"`
    RawContent    string          `json:"raw_content"`    // Original markdown
    ContentHash   string          `json:"content_hash"`   // For change detection
    
    // Structured extracts - populated at ingestion
    Extracts      []Extract       `json:"extracts"`
    
    CrawledAt     time.Time       `json:"crawled_at"`
    ExtractedAt   *time.Time      `json:"extracted_at,omitempty"`
}

type Extract struct {
    Type       string          `json:"type"`        // "financial", "meeting_notes", "api_spec", etc.
    Schema     string          `json:"schema"`      // Schema version used
    Data       json.RawMessage `json:"data"`        // Structured data
    Confidence float64         `json:"confidence"`  // Extraction confidence
    Span       *TextSpan       `json:"span"`        // Location in raw content (for reference)
}

type TextSpan struct {
    Start int `json:"start"`
    End   int `json:"end"`
}
```

### Domain-Specific Extract Types

Define schemas for your data domains. Tools return these, not raw markdown.

```go
// Financial data extracted from documents
type FinancialExtract struct {
    Ticker          string    `json:"ticker"`
    ReportDate      string    `json:"report_date,omitempty"`
    ReportType      string    `json:"report_type,omitempty"`  // quarterly, annual
    Revenue         *float64  `json:"revenue,omitempty"`
    NetIncome       *float64  `json:"net_income,omitempty"`
    EPS             *float64  `json:"eps,omitempty"`
    DividendYield   *float64  `json:"dividend_yield,omitempty"`
    PERatio         *float64  `json:"pe_ratio,omitempty"`
    MarketCap       *float64  `json:"market_cap,omitempty"`
    Notes           []string  `json:"notes,omitempty"`        // Qualitative observations
}

// Meeting notes / decisions
type MeetingExtract struct {
    Date        string   `json:"date"`
    Attendees   []string `json:"attendees,omitempty"`
    Topics      []string `json:"topics"`
    Decisions   []string `json:"decisions,omitempty"`
    ActionItems []Action `json:"action_items,omitempty"`
}

type Action struct {
    Description string  `json:"description"`
    Owner       string  `json:"owner,omitempty"`
    DueDate     string  `json:"due_date,omitempty"`
}

// API/Technical documentation
type APIExtract struct {
    ServiceName string     `json:"service_name"`
    Endpoints   []Endpoint `json:"endpoints,omitempty"`
    AuthMethod  string     `json:"auth_method,omitempty"`
    BaseURL     string     `json:"base_url,omitempty"`
}
```

### Extraction Pipeline

Two approaches - use based on document type and reliability needs:

#### 1. Rule-Based Extraction (Preferred for structured sources)

```go
type Extractor interface {
    CanHandle(doc *Document) bool
    Extract(doc *Document) ([]Extract, error)
}

// Table-based financial data
type TableFinancialExtractor struct{}

func (e *TableFinancialExtractor) Extract(doc *Document) ([]Extract, error) {
    // Parse markdown tables
    tables := markdown.ParseTables(doc.RawContent)
    
    var extracts []Extract
    for _, table := range tables {
        if looksLikeFinancialTable(table) {
            data := parseFinancialTable(table)
            extracts = append(extracts, Extract{
                Type:       "financial",
                Schema:     "v1",
                Data:       mustMarshal(data),
                Confidence: 1.0,  // Deterministic extraction
            })
        }
    }
    return extracts, nil
}
```

#### 2. LLM Extraction (For unstructured prose)

Run **once at ingestion**, not at query time. This is a background job.

```go
type LLMExtractor struct {
    client    LLMClient
    rateLimit *rate.Limiter
}

func (e *LLMExtractor) Extract(doc *Document) ([]Extract, error) {
    // Only for documents that need it
    if doc.SourceType == "api_docs" {
        return nil, nil  // Use rule-based for structured sources
    }
    
    prompt := `Extract structured information from this document.
    
Return JSON matching this schema:
{
    "type": "financial" | "meeting" | "technical" | "general",
    "data": { ... schema-specific fields ... }
}

If no structured data can be extracted, return {"type": "general", "data": null}.

Document:
` + doc.RawContent

    resp, err := e.client.CreateMessage(ctx, &MessageRequest{
        Messages: []Message{{Role: "user", Content: prompt}},
        ResponseFormat: &ResponseFormat{Type: "json_object"},
    })
    
    // ... parse response into Extract
}
```

### Extraction Job Type

Add to job queue for visibility and retry handling:

```go
const JobTypeExtraction = "extraction"

type ExtractionInput struct {
    DocumentID string   `json:"document_id"`
    Extractors []string `json:"extractors"`  // Which extractors to run
}

func (s *Service) executeExtractionJob(ctx context.Context, job *Job) error {
    var input ExtractionInput
    json.Unmarshal(job.Input, &input)
    
    doc, err := s.docStore.Get(input.DocumentID)
    if err != nil {
        return err
    }
    
    var allExtracts []Extract
    for _, extractorName := range input.Extractors {
        extractor := s.extractors[extractorName]
        if extractor.CanHandle(doc) {
            extracts, err := extractor.Extract(doc)
            if err != nil {
                // Log but continue with other extractors
                continue
            }
            allExtracts = append(allExtracts, extracts...)
        }
    }
    
    // Update document with extracts
    now := time.Now()
    doc.Extracts = allExtracts
    doc.ExtractedAt = &now
    s.docStore.Update(doc)
    
    job.Output = mustMarshal(ExtractionOutput{
        ExtractCount: len(allExtracts),
        Types:        extractTypes(allExtracts),
    })
    
    return nil
}
```

### Tool Returns Structured Data

Tools query extracts, not raw markdown:

```go
func (e *DocFinancialsExecutor) Execute(ctx context.Context, params map[string]any) (any, error) {
    ticker := params["ticker"].(string)
    
    // Query by extract type and content
    docs := e.store.FindByExtract(ExtractQuery{
        Type: "financial",
        Filter: map[string]any{
            "ticker": ticker,
        },
    })
    
    // Return structured extracts only
    var results []FinancialExtract
    for _, doc := range docs {
        for _, ext := range doc.Extracts {
            if ext.Type == "financial" {
                var fe FinancialExtract
                json.Unmarshal(ext.Data, &fe)
                if fe.Ticker == ticker {
                    results = append(results, fe)
                }
            }
        }
    }
    
    if len(results) == 0 {
        return ToolResult{
            Data:  nil,
            Error: fmt.Sprintf("No financial data found for %s in document store", ticker),
        }, nil
    }
    
    return ToolResult{Data: results}, nil
}
```

### Fallback to Raw Content

For queries that need full context, provide structured data with optional raw reference:

```go
type DocumentResult struct {
    Extracts   []Extract `json:"extracts"`            // Structured (primary)
    RawContent *string   `json:"raw_content,omitempty"` // Only if explicitly requested
    SourceURL  string    `json:"source_url"`          // For human reference
}
```

### Re-extraction Triggers

Documents need re-extraction when:

1. Content changes (detected via hash)
2. Schema version updates
3. Manual trigger (new extractor added)

```go
func (s *Service) onDocumentUpdated(doc *Document, oldHash string) {
    if doc.ContentHash != oldHash {
        // Content changed - queue re-extraction
        s.queue.Enqueue(&Job{
            Type: JobTypeExtraction,
            Input: mustMarshal(ExtractionInput{
                DocumentID: doc.ID,
                Extractors: s.getExtractorsFor(doc),
            }),
        })
    }
}
```

### Summary: AI Role Shift

| Before | After |
|--------|-------|
| AI searches raw markdown | AI receives structured data |
| "Find the revenue figure" | "Here is revenue: $X" |
| Flaky, inconsistent | Deterministic retrieval |
| Query-time processing | Ingestion-time processing |
| AI as needle-finder | AI as synthesiser |

The AI now **combines and explains** clean inputs rather than **hunting through** messy documents.

## Files to Modify/Create

### Job System
1. `internal/jobs/types.go` - Extended job types and schemas
2. `internal/jobs/orchestrator.go` - Parent job management, child tracking
3. `internal/jobs/planning.go` - Planning LLM call implementation
4. `internal/jobs/synthesis.go` - Synthesis LLM call with schema
5. `internal/jobs/extraction.go` - Document extraction job handler

### Tools
6. `internal/tools/registry.go` - Tool definitions and executor registry
7. `internal/tools/executors/` - Individual tool implementations (return structured data)

### Document Storage & Extraction
8. `internal/documents/schema.go` - Document and Extract type definitions
9. `internal/documents/store.go` - Document storage with extract querying
10. `internal/extraction/extractor.go` - Extractor interface
11. `internal/extraction/rules/` - Rule-based extractors (tables, structured formats)
12. `internal/extraction/llm.go` - LLM-based extractor for unstructured content
13. `internal/extraction/pipeline.go` - Extraction orchestration, extractor selection

### Domain Schemas
14. `internal/extraction/schemas/financial.go` - Financial data extract schema
15. `internal/extraction/schemas/meeting.go` - Meeting notes extract schema
16. `internal/extraction/schemas/technical.go` - API/technical doc extract schema

### API & UI
17. `internal/api/handlers/jobs.go` - API updates for job hierarchy
18. `internal/api/handlers/documents.go` - Document and extract queries
19. UI components for hierarchical job display
20. UI components for document extract viewing

## Testing Checklist

### Job Orchestration
- [ ] Planning call returns tool_use blocks, never text answers
- [ ] Tool jobs appear in queue with correct parent linkage
- [ ] Tool execution is pure Go, no LLM calls
- [ ] Synthesis respects schema constraint
- [ ] Failed tool jobs handled gracefully
- [ ] UI displays job hierarchy correctly
- [ ] Parallel tool execution works
- [ ] Original query context preserved through chain

### Document Extraction
- [ ] Rule-based extractors parse tables/structured content correctly
- [ ] LLM extraction runs at ingestion, not query time
- [ ] Extract schemas validate correctly
- [ ] Content hash change triggers re-extraction
- [ ] Extraction jobs visible in queue
- [ ] Tools return structured extracts, not raw markdown
- [ ] Fallback handling when no extracts found
- [ ] Extract confidence scores populated

### End-to-End
- [ ] Query → Planning → Tool (returns extracts) → Synthesis → Structured output
- [ ] AI cannot fabricate data not in tool results
- [ ] Missing data reported honestly in output
- [ ] Source document references preserved