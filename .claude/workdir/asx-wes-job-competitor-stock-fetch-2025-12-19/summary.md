# Task Summary - ASX:WES Job with Dynamic Competitor Stock Data

## Task Completed Successfully

### Requirements
1. ✓ Create a new job for ASX:WES
2. ✓ Optimize summary for signal/noise focus and data accuracy (if unknown, say so)
3. ✓ Dynamically spawn competitor stock data fetching from competitor analysis

### Files Created

#### `internal/queue/workers/competitor_analysis_worker.go`
New worker that:
- Uses LLM (Gemini) to identify competitors based on target ASX code and prompt
- Directly calls ASXStockDataWorker for each identified competitor (inline execution)
- Tags all fetched competitor data with configurable output_tags
- Implements DefinitionWorker interface with proper error handling and logging

**Key Features:**
```go
// LLM-based competitor identification
func (w *CompetitorAnalysisWorker) identifyCompetitors(ctx, asxCode, prompt, apiKey) ([]string, error)

// Inline stock data fetching via composition
func (w *CompetitorAnalysisWorker) fetchCompetitorStockData(ctx, asxCode, period, outputTags, stepID, jobDef) error
```

### Files Modified

#### `internal/models/worker_type.go`
- Added `WorkerTypeCompetitorAnalysis WorkerType = "competitor_analysis"`
- Updated `IsValid()` switch statement
- Updated `AllWorkerTypes()` slice

#### `internal/app/app.go`
- Registered CompetitorAnalysisWorker with StepManager

#### `bin/job-definitions/web-search-asx-wes.toml`
- Replaced static competitor steps with single `competitor_analysis` step
- Updated prompts to reference dynamically-fetched competitor data
- Added data accuracy instructions throughout prompts

### Architecture Decision

**Why Inline Execution (not Child Job Spawning)?**

The ASXStockDataWorker only implements `DefinitionWorker`, not `JobWorker`. This means:
- It cannot be spawned as a queue job via `CreateChildJob`
- It can only be invoked via `StepManager.Execute` or directly via `CreateJobs`

The CompetitorAnalysisWorker uses **composition** to call ASXStockDataWorker directly:
```go
stockDataWorker := NewASXStockDataWorker(documentStorage, logger, jobMgr)
w.stockDataWorker.CreateJobs(ctx, stockStep, jobDef, stepID, nil)
```

This approach:
- Avoids code duplication
- Follows existing patterns (SummaryWorker is also inline)
- Correctly tags all competitor data for downstream steps
- Returns `ReturnsChildJobs() = false`

### Job Step Flow

```
1. fetch_stock_data      → Fetch WES stock data & technicals
2. fetch_announcements   → Fetch WES ASX announcements
3. search_asx_wes        → Web search for WES news
4. search_industry       → Web search for retail sector outlook
5. search_competitors    → Web search for competitor context
6. analyze_competitors   → LLM identifies competitors, fetches their stock data ← NEW
7. analyze_announcements → Noise vs signal analysis with all data
8. summarize_results     → Final investment report
9. email_summary         → Email the report
```

### Prompt Optimizations

1. **Signal-to-Noise Focus**
   - 2% price movement threshold for large cap stocks
   - Credibility scoring system (X/10)
   - Announcement-price correlation analysis

2. **Data Accuracy**
   - Explicit "IF YOU DO NOT HAVE DATA, SAY 'UNKNOWN'" instructions
   - "DATA UNAVAILABLE" markers in templates
   - Confidence levels for all recommendations
   - Warning that competitors were dynamically identified

3. **Dynamic Competitor Reference**
   - Prompts reference "dynamically fetched competitor stock data"
   - Tables expect variable number of competitors
   - Instructions to list which competitors were found

### Build Verification
```
Build Status: PASS
- Main executable: bin/quaero.exe
- MCP server: bin/quaero-mcp/quaero-mcp.exe
```

### Skill Compliance
- ✓ Refactoring Skill: CREATE justified (composition pattern, no existing worker fits)
- ✓ Go Skill: Proper error handling, context propagation, logging
- ✓ Anti-Creation Bias: Reuses ASXStockDataWorker via composition

### Iterations
- Iteration 1 (Original): Static competitor steps - **REJECTED** by user
- Iteration 2 (Revised): Dynamic competitor analysis with inline execution - **PASS**

### Usage

Run the job via:
1. **Quaero UI**: Jobs → "ASX:WES Investment Analysis" → Run
2. **API**: `POST /api/jobs/web-search-asx-wes/run`

The job will:
1. Fetch WES stock data
2. Use LLM to identify 3-5 retail sector competitors
3. Fetch real-time stock data for each competitor
4. Generate comprehensive analysis with all data
5. Email the report
