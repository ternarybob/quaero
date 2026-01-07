# Stock Rating System - Staged Refactor Plan

## Overview

This document outlines the staged implementation plan for the stock rating system using LLM-orchestrated workers.

**Approach:** Clean-slate implementation. Breaking changes expected. Old code deleted upon completion.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Job Definition (TOML)                         │
│  Defines step order, dependencies, available_tools for orchestrator │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                            Workers                                   │
│         (Queueable steps - read documents, call services)           │
│                                                                      │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐     │
│  │ market_         │  │ market_         │  │ market_data     │     │
│  │ fundamentals    │  │ announcements   │  │ (existing)      │     │
│  │ (existing)      │  │ (existing)      │  │                 │     │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘     │
│           │                    │                    │               │
│           ▼                    ▼                    ▼               │
│      [asx-stock-data]    [asx-announcement]   [market-data]        │
│         documents           documents           documents           │
│           │                    │                    │               │
│           └────────────────────┼────────────────────┘               │
│                                ▼                                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Rating Workers (NEW)                      │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │   │
│  │  │ bfs      │ │ cds      │ │ nfr      │ │ pps      │  ...  │   │
│  │  │ worker   │ │ worker   │ │ worker   │ │ worker   │       │   │
│  │  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘       │   │
│  │       │            │            │            │              │   │
│  │       ▼            ▼            ▼            ▼              │   │
│  │   [bfs-score]  [cds-score]  [nfr-score]  [pps-score]       │   │
│  │     documents                                               │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                            Services                                  │
│              (Pure functions - stateless, no documents)             │
│                                                                      │
│  internal/services/rating/                                          │
│  ├── bfs.go      ← CalculateBFS(fundamentals) → BFSResult          │
│  ├── cds.go      ← CalculateCDS(fundamentals, announcements) → ... │
│  ├── nfr.go      ← CalculateNFR(announcements, prices) → ...       │
│  ├── pps.go      ← CalculatePPS(announcements, prices) → ...       │
│  ├── vrs.go      ← CalculateVRS(announcements, prices) → ...       │
│  ├── ob.go       ← CalculateOB(announcements, bfsScore) → ...      │
│  ├── composite.go← CalculateRating(bfs, cds, nfr, pps, vrs, ob) →  │
│  └── types.go    ← Shared types for rating calculations            │
└─────────────────────────────────────────────────────────────────────┘
```

**Key Principles:**
- **Services**: Pure calculation logic, stateless, no document awareness
- **Workers**: Read documents, call services, write output documents
- **Document-based coupling**: Data flows via documents in storage
- **Worker-to-worker comms**: Allowed but limited to:
  - Request: context only (ticker or tickers)
  - Response: document ID(s) only
  - Actual data read from document storage using returned IDs
- **No "tools" package**: Workers ARE the orchestrator-callable tools

---

## What Gets Deleted

- `internal/signals/` - All existing signal calculations (PBAS, VLI, Regime, etc.)
- `internal/queue/workers/mqs_*.go` - Move to services (if keeping MQS)
- Unused schemas in `internal/schemas/`

## What Gets Kept

- `internal/services/eodhd/` - Data API client
- `internal/services/llm/` - LLM provider abstraction
- `internal/storage/` - BadgerDB storage layer
- `internal/queue/` - Job queue infrastructure
- `internal/queue/workers/orchestrator_worker.go` - LLM orchestration framework
- `internal/queue/workers/market_fundamentals_worker.go` - Data collection
- `internal/queue/workers/market_announcements_worker.go` - Data collection
- `internal/queue/workers/market_data_worker.go` - Data collection

---

## Ticker Format

Tickers are exchange-agnostic. Format:
- `ASX.EXR` - Explicit exchange prefix
- `EXR` - Uses default exchange from job config

Default exchange configured in job definition:
```toml
[config]
default_exchange = "ASX"
variables = [
    { ticker = "EXR" },      # Resolves to ASX.EXR
    { ticker = "NYSE.AAPL" }, # Explicit exchange
]
```

Workers and services should handle both formats using `common.ParseTicker()`.

---

## Execution Patterns

Two patterns for orchestrating workers:

### Pattern 1: Deterministic Job Steps

Steps defined in job definition, executed in order. **No LLM decision-making** for tool selection.

```
Job Definition → Workers execute in dependency order → LLM only for summary
```

```toml
# Each step is a deterministic worker - LLM does NOT choose these
[step.fetch_fundamentals]
type = "market_fundamentals"

[step.calculate_bfs]
type = "rating_bfs"
depends = "fetch_fundamentals"

# LLM only used here - generates narrative from collected data
[step.summarize]
type = "summary"
template = "stock-rating"
filter_tags = ["stock-rating"]
```

**Use when:**
- Workflow is predictable and well-defined
- All steps known in advance
- Maximum reliability and reproducibility

### Pattern 2: LLM-Orchestrated Tool Selection

LLM Planner decides which tools to call based on goal. Uses `orchestrator_worker.go`.

```
Goal → LLM Planner → selects tools → Executor runs → Reviewer validates → repeat
```

```toml
[step.orchestrate]
type = "orchestrator"
goal = "Rate the stock and explain the investment thesis"
thinking_level = "MEDIUM"
available_tools = [
    { name = "get_fundamentals", worker = "market_fundamentals", description = "Fetch stock fundamentals. REQUIRED params: ticker (string)." },
    { name = "calculate_bfs", worker = "rating_bfs", description = "Calculate Business Foundation Score (0-2). REQUIRED params: ticker (string)." },
    { name = "calculate_cds", worker = "rating_cds", description = "Calculate Capital Discipline Score (0-2). REQUIRED params: ticker (string)." },
    # ... more tools
]
```

**Use when:**
- Need LLM reasoning about results
- Want explanations for each decision
- Open-ended analysis tasks

### Pattern Comparison

| Aspect | Deterministic | LLM-Orchestrated |
|--------|---------------|------------------|
| Tool selection | Job definition | LLM decides |
| Predictability | High | High (consistent output) |
| Cost | Lower (no planner calls) | Higher (LLM reasoning) |
| Flexibility | Fixed workflow | Adaptive execution |
| Reasoning | None | Explains decisions |

**Important:** Both patterns must produce **consistent output**. All scores are always calculated. The difference is in execution flexibility and reasoning, NOT in skipping work.

### Recommended Approach for Rating

**Hybrid**: Use deterministic steps for data collection and calculation, with optional LLM orchestration for reasoning:

```toml
# Option A: Fully deterministic (simpler, cheaper)
[step.calculate_bfs]
type = "rating_bfs"
[step.calculate_cds]
type = "rating_cds"
# ... all steps defined, all always run

# Option B: LLM-orchestrated (adds reasoning to output)
[step.orchestrate_rating]
type = "orchestrator"
goal = "Calculate all rating scores and explain the results. Always calculate all components for consistent output."
available_tools = [...]
```

---

## Concurrent Execution

When a job defines multiple tickers as variables, the workflow executes **concurrently** for each ticker:

```
Job: stock-rating-watchlist
Config: variables = [GNP, SKS, EXR]

Execution:
┌─────────────────────────────────────────────────────────┐
│                    CONCURRENT                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │ GNP         │  │ SKS         │  │ EXR         │    │
│  │ ─────────── │  │ ─────────── │  │ ─────────── │    │
│  │ fetch_fund  │  │ fetch_fund  │  │ fetch_fund  │    │
│  │     ↓       │  │     ↓       │  │     ↓       │    │
│  │ fetch_ann   │  │ fetch_ann   │  │ fetch_ann   │    │
│  │     ↓       │  │     ↓       │  │     ↓       │    │
│  │ calc_bfs    │  │ calc_bfs    │  │ calc_bfs    │    │
│  │     ↓       │  │     ↓       │  │     ↓       │    │
│  │ calc_rating │  │ calc_rating │  │ calc_rating │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
└─────────────────────────────────────────────────────────┘
                          ↓
                    [All complete]
                          ↓
                   email_report (aggregates all ratings)
```

- Each ticker runs the full template workflow independently
- Steps within a ticker respect `depends` ordering
- Final aggregation steps (e.g., email_report) wait for all tickers to complete

---

## Stage 1: Rating Service

**Goal:** Create pure calculation functions with no document awareness.

### Tasks

#### 1.1 Create Service Package
- [ ] Create `internal/services/rating/`

#### 1.2 Define Types
- [ ] Create `internal/services/rating/types.go`
  ```go
  // Input types (passed from workers)
  type Fundamentals struct {
      Ticker                   string
      CompanyName              string
      Sector                   string
      MarketCap                float64
      SharesOutstandingCurrent int64
      SharesOutstanding3YAgo   *int64
      CashBalance              float64
      QuarterlyCashBurn        float64
      RevenueTTM               float64
      IsProfitable             bool
      HasProducingAsset        bool
  }

  type Announcement struct {
      Date             time.Time
      Headline         string
      Type             AnnouncementType
      IsPriceSensitive bool
  }

  type PriceBar struct {
      Date   time.Time
      Open   float64
      High   float64
      Low    float64
      Close  float64
      Volume int64
  }

  // Output types
  type BFSResult struct {
      Score          int     // 0, 1, or 2
      IndicatorCount int
      Components     BFSComponents
      Reasoning      string
  }

  type CDSResult struct {
      Score      int // 0, 1, or 2
      Components CDSComponents
      Reasoning  string
  }

  // ... similar for NFR, PPS, VRS, OB, Rating
  ```

#### 1.3 Implement BFS Calculation
- [ ] Create `internal/services/rating/bfs.go`
  ```go
  func CalculateBFS(f Fundamentals) BFSResult {
      // Pure function - no document access
      cashRunway := calculateCashRunway(f.CashBalance, f.QuarterlyCashBurn)

      indicators := 0
      if f.RevenueTTM > 10_000_000 { indicators++ }
      if cashRunway > 18 { indicators++ }
      if f.HasProducingAsset { indicators++ }
      if f.IsProfitable { indicators++ }

      score := 0
      if indicators >= 2 { score = 2 }
      else if indicators == 1 { score = 1 }

      return BFSResult{Score: score, ...}
  }
  ```

#### 1.4 Implement CDS Calculation
- [ ] Create `internal/services/rating/cds.go`
  ```go
  func CalculateCDS(f Fundamentals, announcements []Announcement, months int) CDSResult {
      sharesCagr := calculateSharesCAGR(f.SharesOutstandingCurrent, f.SharesOutstanding3YAgo)
      haltsPA := countByType(announcements, TypeTradingHalt) / (float64(months) / 12)
      raisesPA := countByType(announcements, TypeCapitalRaise) / (float64(months) / 12)

      // Score logic...
      return CDSResult{...}
  }
  ```

#### 1.5 Implement Score Calculations
- [ ] Create `internal/services/rating/nfr.go` - Narrative-to-Fact Ratio
- [ ] Create `internal/services/rating/pps.go` - Price Progression Score
- [ ] Create `internal/services/rating/vrs.go` - Volatility Regime Stability
- [ ] Create `internal/services/rating/ob.go` - Optionality Bonus

#### 1.6 Implement Composite Rating
- [ ] Create `internal/services/rating/composite.go`
  ```go
  func CalculateRating(bfs BFSResult, cds CDSResult, nfr, pps, vrs, ob *float64) RatingResult {
      passed := bfs.Score >= 1 && cds.Score >= 1
      if !passed {
          return RatingResult{Label: LabelSpeculative, ...}
      }

      investability := float64(bfs.Score)*12.5 + float64(cds.Score)*12.5 +
                       *nfr*25 + *pps*25 + *vrs*15 + *ob*10

      label := determineLabel(investability)
      return RatingResult{...}
  }
  ```

#### 1.7 Math Utilities
- [ ] Create `internal/services/rating/math.go`
- [ ] `PriceWindow(prices []PriceBar, date time.Time, before, after int) []PriceBar`
- [ ] `DailyReturns(prices []PriceBar) []float64`
- [ ] `Stddev(values []float64) float64`
- [ ] `CAGR(start, end float64, years float64) float64`

#### 1.8 Unit Tests
- [ ] Create `internal/services/rating/*_test.go` for each calculation
- [ ] Test each calculation function with known inputs/outputs
- [ ] Test edge cases: missing data, zero values, boundary conditions
- [ ] Pure function tests - no mocks, no external dependencies

### Deliverables
- `internal/services/rating/` package
- Pure calculation functions
- Unit tests in same package (`*_test.go`)
- Coverage >90%

---

## Stage 2: Rating Workers

**Goal:** Create workers that read documents, call services, write output documents.

### Tasks

#### 2.1 BFS Worker
- [ ] Create `internal/queue/workers/rating_bfs_worker.go`
  ```go
  type RatingBFSWorker struct {
      documentStorage interfaces.DocumentStorage
      logger          arbor.ILogger
  }

  func (w *RatingBFSWorker) Execute(ctx context.Context, job *QueueJob) error {
      ticker := job.Payload["ticker"].(string)

      // 1. Request document ID from fundamentals worker (context only)
      docID, err := w.requestDocumentID(ctx, "market_fundamentals", ticker)
      if err != nil {
          return err
      }

      // 2. Read document from storage using ID
      doc, err := w.documentStorage.GetDocument(docID)
      if err != nil {
          return err
      }

      // 3. Transform document to service input
      fundamentals := w.extractFundamentals(doc)

      // 4. Call service (pure function)
      result := rating.CalculateBFS(fundamentals)

      // 5. Save result as document, return document ID
      return w.saveResultDocument(ctx, ticker, result)
  }
  ```

#### 2.2 CDS Worker
- [ ] Create `internal/queue/workers/rating_cds_worker.go`
- [ ] Reads: `stock-data` + `announcement-summary` documents
- [ ] Calls: `rating.CalculateCDS()`
- [ ] Outputs: `rating-cds` document

#### 2.3 Score Workers
- [ ] Create `internal/queue/workers/rating_nfr_worker.go`
- [ ] Create `internal/queue/workers/rating_pps_worker.go`
- [ ] Create `internal/queue/workers/rating_vrs_worker.go`
- [ ] Create `internal/queue/workers/rating_ob_worker.go`

Each reads required documents, calls service, outputs score document.

#### 2.4 Composite Rating Worker
- [ ] Create `internal/queue/workers/rating_composite_worker.go`
- [ ] Reads: All score documents (`rating-bfs`, `rating-cds`, `rating-nfr`, etc.)
- [ ] Calls: `rating.CalculateRating()`
- [ ] Outputs: `stock-rating` document

#### 2.5 Register Workers
- [ ] Update `internal/models/worker_type.go`:
  ```go
  WorkerTypeRatingBFS       WorkerType = "rating_bfs"
  WorkerTypeRatingCDS       WorkerType = "rating_cds"
  WorkerTypeRatingNFR       WorkerType = "rating_nfr"
  WorkerTypeRatingPPS       WorkerType = "rating_pps"
  WorkerTypeRatingVRS       WorkerType = "rating_vrs"
  WorkerTypeRatingOB        WorkerType = "rating_ob"
  WorkerTypeRatingComposite WorkerType = "rating_composite"
  ```
- [ ] Register in worker factory

#### 2.6 Worker API Tests
- [ ] Create `test/api/market_workers/rating_bfs_test.go`
- [ ] Create `test/api/market_workers/rating_cds_test.go`
- [ ] Create `test/api/market_workers/rating_nfr_test.go`
- [ ] Create `test/api/market_workers/rating_pps_test.go`
- [ ] Create `test/api/market_workers/rating_vrs_test.go`
- [ ] Create `test/api/market_workers/rating_ob_test.go`
- [ ] Create `test/api/market_workers/rating_composite_test.go`

Follow existing patterns in `test/api/market_workers/common_test.go`.

### Deliverables
- 7 rating workers
- Worker registration
- API tests following existing framework
- Integration with existing document storage

---

## Stage 3: Template & Job Definition

**Goal:** Create rating template and job definition that orchestrates the rating flow.

### Tasks

#### 3.1 Create Rating Prompt Template
- [ ] Create `internal/templates/stock-rating.toml`
  ```toml
  # Stock Rating Prompt Template
  # Type: prompt (for summary step - generates rating reasoning)

  type = "prompt"
  schema_ref = "stock-rating.schema.json"

  prompt = """
  Analyze the collected rating scores and provide investability assessment.

  For each stock with rating data:
  1. Gate Assessment: Explain BFS and CDS scores
  2. Component Analysis: Interpret NFR, PPS, VRS, OB scores
  3. Overall Rating: Justify the investability label
  4. Key Risks: Highlight concerns from low scores

  Output MUST match the stock-rating.schema.json structure.
  """
  ```

#### 3.2 Create Job Definition
- [ ] Create `deployments/common/job-definitions/stock-rating-watchlist.toml`
  ```toml
  # Stock Rating - Watchlist
  # Rates stocks using investability framework

  id = "stock-rating-watchlist"
  name = "Stock Rating Watchlist"
  type = "orchestrator"
  description = "Rate watchlist stocks using investability framework"
  tags = ["rating", "watchlist"]

  [config]
  default_exchange = "ASX"
  variables = [
      { ticker = "GNP" },
      { ticker = "SKS" },
      { ticker = "EXR" },
  ]

  # Step 1: Collect data (existing workers)
  [step.fetch_fundamentals]
  type = "market_fundamentals"
  description = "Fetch stock fundamentals"
  on_error = "continue"

  [step.fetch_announcements]
  type = "market_announcements"
  description = "Fetch announcements"
  depends = "fetch_fundamentals"
  on_error = "continue"

  [step.fetch_prices]
  type = "market_data"
  description = "Fetch price data"
  depends = "fetch_fundamentals"
  on_error = "continue"

  # Step 2: Calculate gate scores
  [step.calculate_bfs]
  type = "rating_bfs"
  description = "Calculate Business Foundation Score"
  depends = "fetch_fundamentals"

  [step.calculate_cds]
  type = "rating_cds"
  description = "Calculate Capital Discipline Score"
  depends = "fetch_announcements"

  # Step 3: Calculate component scores
  [step.calculate_nfr]
  type = "rating_nfr"
  description = "Calculate Narrative-to-Fact Ratio"
  depends = "fetch_announcements,fetch_prices"

  [step.calculate_pps]
  type = "rating_pps"
  description = "Calculate Price Progression Score"
  depends = "fetch_announcements,fetch_prices"

  [step.calculate_vrs]
  type = "rating_vrs"
  description = "Calculate Volatility Regime Stability"
  depends = "fetch_announcements,fetch_prices"

  [step.calculate_ob]
  type = "rating_ob"
  description = "Calculate Optionality Bonus"
  depends = "fetch_announcements,calculate_bfs"

  # Step 4: Composite rating
  [step.calculate_rating]
  type = "rating_composite"
  description = "Calculate final investability rating"
  depends = "calculate_bfs,calculate_cds,calculate_nfr,calculate_pps,calculate_vrs,calculate_ob"
  output_tags = ["stock-rating"]

  # Step 5: AI Summary (optional - uses template at step level)
  [step.summarize]
  type = "summary"
  description = "Generate AI summary of ratings"
  depends = "calculate_rating"
  template = "stock-rating"  # References internal/templates/stock-rating.toml
  filter_tags = ["stock-rating"]
  output_tags = ["stock-rating-summary"]

  # Step 6: Format output
  [step.format_output]
  type = "output_formatter"
  description = "Format ratings for email delivery"
  depends = "summarize"
  on_error = "fail"
  input_tags = ["stock-rating-summary"]
  output_tags = ["email-output"]
  title = "Stock Rating Report"

  # Step 7: Email report
  [step.email_report]
  type = "email"
  description = "Email rating report"
  depends = "format_output"
  always_run = true
  on_error = "fail"
  to = "user@example.com"
  subject = "Stock Rating Report"
  ```

#### 3.3 Alternative: LLM-Orchestrated Job Definition
- [ ] Create `deployments/common/job-definitions/stock-rating-orchestrated.toml`
  ```toml
  # Stock Rating - LLM Orchestrated
  # LLM decides tool order, can skip scores if gate fails

  id = "stock-rating-orchestrated"
  name = "Stock Rating (Orchestrated)"
  type = "orchestrator"
  description = "LLM-orchestrated stock rating with adaptive workflow"
  tags = ["rating", "orchestrated"]

  [config]
  default_exchange = "ASX"
  variables = [
      { ticker = "GNP" },
  ]

  # Single orchestrator step - LLM plans and executes
  [step.rate_stock]
  type = "orchestrator"
  description = "Rate stock using investability framework"
  goal = """
  Rate the stock using the investability framework:
  1. Fetch fundamentals, announcements, and price data
  2. Calculate ALL scores (BFS, CDS, NFR, PPS, VRS, OB) - never skip any
  3. Calculate composite rating and determine label
  4. Provide reasoning for each score and the final rating
  5. Output must be consistent - always include all components

  If gate fails (BFS<1 or CDS<1), still calculate all scores but label as SPECULATIVE.
  """
  thinking_level = "MEDIUM"
  output_tags = ["stock-rating"]

  # Available tools for LLM to choose from
  available_tools = [
      { name = "get_fundamentals", worker = "market_fundamentals", description = "Fetch stock fundamentals including revenue, cash, shares outstanding. Creates document tagged ['stock-data', '<ticker>']. REQUIRED params: ticker (string)." },
      { name = "get_announcements", worker = "market_announcements", description = "Fetch company announcements with price sensitivity flags. Creates document tagged ['announcement-summary', '<ticker>']. REQUIRED params: ticker (string). OPTIONAL: period (string) - M6, Y1, Y3 (default Y3)." },
      { name = "get_prices", worker = "market_data", description = "Fetch OHLCV price history. Creates document tagged ['market-data', '<ticker>']. REQUIRED params: ticker (string). OPTIONAL: period (string)." },
      { name = "calculate_bfs", worker = "rating_bfs", description = "Calculate Business Foundation Score (0-2). Reads stock-data document. REQUIRED params: ticker (string). Returns: score, components, reasoning." },
      { name = "calculate_cds", worker = "rating_cds", description = "Calculate Capital Discipline Score (0-2). Reads stock-data + announcement-summary documents. REQUIRED params: ticker (string). Returns: score, components, reasoning." },
      { name = "calculate_nfr", worker = "rating_nfr", description = "Calculate Narrative-to-Fact Ratio (0.0-1.0). Reads announcement-summary + market-data documents. REQUIRED params: ticker (string). Returns: score, components, reasoning." },
      { name = "calculate_pps", worker = "rating_pps", description = "Calculate Price Progression Score (0.0-1.0). Reads announcement-summary + market-data documents. REQUIRED params: ticker (string). Returns: score, event_details, reasoning." },
      { name = "calculate_vrs", worker = "rating_vrs", description = "Calculate Volatility Regime Stability (0.0-1.0). Reads announcement-summary + market-data documents. REQUIRED params: ticker (string). Returns: score, components, reasoning." },
      { name = "calculate_ob", worker = "rating_ob", description = "Calculate Optionality Bonus (0.0, 0.5, or 1.0). Reads announcement-summary + bfs-score documents. REQUIRED params: ticker (string). Returns: score, catalyst_found, timeframe_found, reasoning." },
      { name = "calculate_rating", worker = "rating_composite", description = "Calculate final investability rating from all component scores. Reads all score documents. REQUIRED params: ticker (string). Returns: label (SPECULATIVE|LOW_ALPHA|WATCHLIST|INVESTABLE|HIGH_CONVICTION), investability (0-100), gate_passed, all_scores." },
  ]

  # Format and email (deterministic, after orchestration)
  [step.format_output]
  type = "output_formatter"
  depends = "rate_stock"
  input_tags = ["stock-rating"]
  output_tags = ["email-output"]
  title = "Stock Rating Report"

  [step.email_report]
  type = "email"
  depends = "format_output"
  to = "user@example.com"
  subject = "Stock Rating Report"
  ```

#### 3.4 Test Job Execution
- [ ] Run deterministic job with test tickers
- [ ] Run orchestrated job with test tickers
- [ ] Verify both patterns produce identical output structure
- [ ] Verify all scores calculated even when gate fails
- [ ] Verify documents created with correct tags

### Deliverables
- Rating prompt template
- Deterministic job definition
- LLM-orchestrated job definition (alternative)
- Working end-to-end flow for both patterns

---

## Stage 4: Output & Reporting

**Goal:** Configure output formatting for rating documents.

The `output_formatter` worker already exists. This stage focuses on:
1. Ensuring rating documents have correct structure for formatting
2. Creating rating-specific templates if needed

### Tasks

#### 4.1 Rating Document Structure
- [ ] Ensure `rating_composite_worker` outputs markdown-compatible content
- [ ] Include summary table in document body
- [ ] Include per-ticker details

#### 4.2 Report Templates (Optional)
- [ ] Create `internal/templates/stock-rating-report.toml` if custom formatting needed
- [ ] Otherwise, use default `output_formatter` behavior

#### 4.3 Verify Email Flow
- [ ] Test `format_output` step collects `stock-rating` documents
- [ ] Test `email_report` step sends formatted output
- [ ] Verify HTML rendering in email

### Deliverables
- Rating documents formatted for output_formatter
- Working email flow

---

## Stage 5: Cleanup

**Goal:** Remove old code, verify system integrity.

### Tasks

#### 5.1 Delete Old Signals
- [ ] Delete `internal/signals/` directory

#### 5.2 Move MQS to Services
- [ ] Move `mqs_analyzer.go` logic to `internal/services/mqs/`
- [ ] Move `mqs_classifier.go` logic to `internal/services/mqs/`
- [ ] Move `mqs_types.go` to `internal/services/mqs/`
- [ ] Update workers to call service

#### 5.3 Verify Build
- [ ] All tests pass
- [ ] No orphaned imports
- [ ] Clean build

### Deliverables
- Clean codebase
- Passing build

---

## Stage 6: Testing & Verification

**Goal:** Refactor tests for updated workers and services using existing test framework.

### Testing Framework

Existing test framework in `test/api/market_workers/` - **DO NOT CHANGE**:
- `common_test.go` - Shared test utilities and helpers
- `*_test.go` - Individual worker tests

New tests follow the same patterns and conventions.

### Tasks

#### 6.1 Service Unit Tests
- [ ] Create `internal/services/rating/bfs_test.go`
- [ ] Create `internal/services/rating/cds_test.go`
- [ ] Create `internal/services/rating/nfr_test.go`
- [ ] Create `internal/services/rating/pps_test.go`
- [ ] Create `internal/services/rating/vrs_test.go`
- [ ] Create `internal/services/rating/ob_test.go`
- [ ] Create `internal/services/rating/composite_test.go`

Pure function tests - no dependencies on workers or documents.

#### 6.2 Worker API Tests
- [ ] Create `test/api/market_workers/rating_bfs_test.go`
- [ ] Create `test/api/market_workers/rating_cds_test.go`
- [ ] Create `test/api/market_workers/rating_nfr_test.go`
- [ ] Create `test/api/market_workers/rating_pps_test.go`
- [ ] Create `test/api/market_workers/rating_vrs_test.go`
- [ ] Create `test/api/market_workers/rating_ob_test.go`
- [ ] Create `test/api/market_workers/rating_composite_test.go`

Follow existing test patterns in `common_test.go`.

#### 6.3 Golden Test Cases
| Ticker | Expected Label | Gate | Investability |
|--------|----------------|------|---------------|
| BHP | LOW_ALPHA | Pass | 40-50 |
| EXR | SPECULATIVE | Fail (CDS=0) | null |
| CSL | LOW_ALPHA | Pass | 40-50 |
| GNP | INVESTABLE | Pass | 65-75 |
| SKS | INVESTABLE | Pass | 65-75 |

#### 6.4 Integration Tests
- [ ] Create `test/api/market_workers/rating_integration_test.go`
- [ ] Test full job flow with golden tickers
- [ ] Verify document output schemas
- [ ] Test concurrent execution with multiple tickers

#### 6.5 Performance
- [ ] Full rating flow <30s per ticker
- [ ] Batch processing throughput

### Deliverables
- Service unit tests (>90% coverage)
- Worker API tests (follow existing patterns)
- Integration tests with golden cases
- Performance benchmarks

---

## Implementation Order

```
Stage 1: Rating Service      ─── Pure calculation functions
          ↓
Stage 2: Rating Workers      ─── Document I/O, call services
          ↓
Stage 3: Job Definition      ─── Wire workers together
          ↓
Stage 4: Output & Reporting  ─── Generate reports
          ↓
Stage 5: Cleanup             ─── Delete old code, move MQS
          ↓
Stage 6: Verification        ─── Golden tests, benchmarks
```

## Success Criteria

- [ ] All golden test cases pass
- [ ] Full rating flow <30s per ticker
- [ ] Clean separation: services (pure) / workers (I/O)
- [ ] No dead code remaining
- [ ] Clean build with no warnings

---

*Document Version: 3.0*
*Created: 2026-01-07*
