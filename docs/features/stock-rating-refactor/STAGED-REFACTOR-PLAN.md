# Stock Rating System - Staged Refactor Plan

## Overview

This document outlines the staged implementation plan for replacing the existing worker system with LLM-orchestrated tools. Each stage is designed to be independently testable.

**Approach:** Clean-slate implementation. Breaking changes are expected. Old workers and signals code will be deleted upon completion.

**What Gets Deleted:**
- `internal/signals/` - All existing signal calculations (PBAS, VLI, Regime, etc.)
- `internal/queue/workers/market_*.go` - Replaced by new tools
- `internal/queue/workers/signal_*.go` - Replaced by new tools
- Unused schemas in `internal/schemas/`

**What Gets Kept:**
- `internal/services/eodhd/` - Data API client (used by new tools)
- `internal/services/llm/` - LLM provider abstraction
- `internal/storage/` - BadgerDB storage layer
- `internal/queue/` - Job queue infrastructure (workers replaced, queue kept)
- `internal/queue/workers/orchestrator_worker.go` - LLM orchestration framework (reused)
- `internal/queue/workers/tool_execution_worker.go` - Tool execution wrapper (reused)

**Existing Orchestration (to leverage):**
The `orchestrator_worker.go` already implements:
- Planner-Executor-Reviewer loop with LLM reasoning
- Plan structure with steps, dependencies, params
- Wave-based execution (parallel within wave, sequential across waves)
- Tool execution via queue jobs (`JobTypeToolExecution`)
- Tool lookup from `available_tools` config
- Result collection and review

New rating tools will be registered as `available_tools` in job definitions.

---

## Stage 1: Foundation

**Goal:** Establish tool interfaces, types, and package structure.

### Tasks

#### 1.1 Create Tool Interface
- [ ] Create `internal/tools/tool.go`
  ```go
  type Tool interface {
      Name() string
      Description() string
      Category() ToolCategory  // data | gate | score | rating | output
      InputSchema() json.RawMessage
      OutputSchema() json.RawMessage
      Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
  }

  type ToolCategory string
  const (
      CategoryData    ToolCategory = "data"
      CategoryGate    ToolCategory = "gate"
      CategoryScore   ToolCategory = "score"
      CategoryRating  ToolCategory = "rating"
      CategoryOutput  ToolCategory = "output"
  )
  ```

#### 1.2 Create Tool Registry
- [ ] Create `internal/tools/registry.go`
- [ ] `Register(tool Tool)` - Add tool to registry
- [ ] `Get(name string) Tool` - Retrieve by name
- [ ] `List() []Tool` - All registered tools
- [ ] `ExportForLLM() []LLMToolDefinition` - Export for Claude/Gemini

#### 1.3 Define Core Types
- [ ] Create `internal/tools/types.go`
  ```go
  type Ticker string  // 3-4 char ASX code

  type AnnouncementType string
  const (
      TypeTradingHalt  AnnouncementType = "TRADING_HALT"
      TypeCapitalRaise AnnouncementType = "CAPITAL_RAISE"
      TypeResults      AnnouncementType = "RESULTS"
      TypeContract     AnnouncementType = "CONTRACT"
      TypeOperational  AnnouncementType = "OPERATIONAL"
      TypeCompliance   AnnouncementType = "COMPLIANCE"
      TypeOther        AnnouncementType = "OTHER"
  )

  type RatingLabel string
  const (
      LabelSpeculative    RatingLabel = "SPECULATIVE"
      LabelLowAlpha       RatingLabel = "LOW_ALPHA"
      LabelWatchlist      RatingLabel = "WATCHLIST"
      LabelInvestable     RatingLabel = "INVESTABLE"
      LabelHighConviction RatingLabel = "HIGH_CONVICTION"
  )

  type Announcement struct {
      Date            time.Time
      Headline        string
      Type            AnnouncementType
      IsPriceSensitive bool
      URL             string
  }

  type PriceBar struct {
      Date        time.Time
      Open        float64
      High        float64
      Low         float64
      Close       float64
      Volume      int64
      DailyReturn float64  // optional
      Volatility  float64  // optional, 20-day
  }

  type Fundamentals struct {
      Ticker                  Ticker
      CompanyName             string
      Sector                  string
      MarketCap               float64
      AsOfDate                time.Time
      SharesOutstandingCurrent int64
      SharesOutstanding3YAgo   *int64  // nil if unavailable
      CashBalance             float64
      QuarterlyCashBurn       float64
      RevenueTTM              float64
      IsProfitable            bool
      HasProducingAsset       bool
      DataQuality             DataQuality
  }

  type DataQuality string
  const (
      QualityComplete     DataQuality = "COMPLETE"
      QualityPartial      DataQuality = "PARTIAL"
      QualityStale        DataQuality = "STALE"
      QualityInsufficient DataQuality = "INSUFFICIENT"
  )
  ```

#### 1.4 Define Error Types
- [ ] Create `internal/tools/errors.go`
  ```go
  type ToolError struct {
      Code    ErrorCode
      Message string
      Partial json.RawMessage  // partial result if available
  }

  type ErrorCode string
  const (
      ErrTickerNotFound    ErrorCode = "TICKER_NOT_FOUND"
      ErrInsufficientData  ErrorCode = "INSUFFICIENT_DATA"
      ErrStaleData         ErrorCode = "STALE_DATA"
      ErrCalculationError  ErrorCode = "CALCULATION_ERROR"
      ErrMissingInput      ErrorCode = "MISSING_INPUT"
      ErrGenerationError   ErrorCode = "GENERATION_ERROR"
  )
  ```

#### 1.5 Package Structure
```
internal/tools/
├── tool.go              # Tool interface
├── registry.go          # Tool registration
├── types.go             # Shared types
├── errors.go            # Error types
├── data/                # Stage 2
├── gate/                # Stage 3
├── score/               # Stage 4
├── rating/              # Stage 5
├── output/              # Stage 6
└── orchestrator/        # Stage 7
```

### Deliverables
- Tool interface and registry
- All shared types
- Error definitions
- Package structure

---

## Stage 2: Data Tools

**Goal:** Implement data fetching tools using existing EODHD/ASX services.

### Tasks

#### 2.1 Implement `get_announcements`
- [ ] Create `internal/tools/data/announcements.go`
- [ ] Input: `{ticker, months, types[]}`
- [ ] Output:
  ```json
  {
    "ticker": "GNP",
    "count": 54,
    "period_start": "2023-01-07",
    "period_end": "2026-01-07",
    "announcements": [...]
  }
  ```
- [ ] Classify announcements by type (headline keyword matching)
- [ ] Use existing Markit Digital API via `internal/services/`

#### 2.2 Implement `get_prices`
- [ ] Create `internal/tools/data/prices.go`
- [ ] Input: `{ticker, months, include_derived}`
- [ ] Output:
  ```json
  {
    "ticker": "GNP",
    "count": 756,
    "period_start": "2023-01-07",
    "period_end": "2026-01-07",
    "latest_close": 2.45,
    "prices": [...]
  }
  ```
- [ ] Compute derived metrics if requested (daily returns, 20d volatility)
- [ ] Use existing EODHD service

#### 2.3 Implement `get_fundamentals`
- [ ] Create `internal/tools/data/fundamentals.go`
- [ ] Input: `{ticker}`
- [ ] Output: `Fundamentals` struct as JSON
- [ ] Map EODHD consolidated data to schema
- [ ] Determine `has_producing_asset` from sector/description
- [ ] Set `data_quality` based on field completeness

#### 2.4 Add Caching
- [ ] Create `internal/tools/data/cache.go`
- [ ] Cache key: `tool:{name}:{ticker}:{hash(params)}`
- [ ] TTL based on exchange trading hours
- [ ] Use existing BadgerDB storage

#### 2.5 Tests
- [ ] Unit tests with mocked API responses
- [ ] Verify output schema compliance

### Deliverables
- 3 data tools
- Caching layer
- Unit tests

---

## Stage 3: Gate Tools

**Goal:** Implement BFS and CDS calculations that determine if a stock passes the rating gate.

### Tasks

#### 3.1 Implement `calculate_bfs` (Business Foundation Score)
- [ ] Create `internal/tools/gate/bfs.go`
- [ ] Input: `{fundamentals}`
- [ ] Output:
  ```json
  {
    "ticker": "GNP",
    "score": 2,
    "components": {
      "revenue_ttm": 45000000,
      "revenue_passes": true,
      "cash_runway_months": 24,
      "cash_runway_passes": true,
      "has_producing_asset": true,
      "is_profitable": false
    },
    "indicator_count": 3,
    "reasoning": "3/4 indicators met: revenue >$10M, cash runway >18mo, has producing asset"
  }
  ```
- [ ] Logic:
  ```
  cash_runway = cash_balance / abs(quarterly_cash_burn)
    IF quarterly_cash_burn >= 0: cash_runway = 999 (infinite)

  indicators = count of:
    - revenue_ttm > 10,000,000
    - cash_runway > 18
    - has_producing_asset
    - is_profitable

  score = 2 if indicators >= 2
  score = 1 if indicators == 1
  score = 0 if indicators == 0
  ```

#### 3.2 Implement `calculate_cds` (Capital Discipline Score)
- [ ] Create `internal/tools/gate/cds.go`
- [ ] Input: `{fundamentals, announcements}`
- [ ] Output:
  ```json
  {
    "ticker": "GNP",
    "score": 2,
    "components": {
      "shares_cagr_3y": 5.2,
      "trading_halts_total": 3,
      "trading_halts_per_year": 1.0,
      "capital_raises_total": 2,
      "capital_raises_per_year": 0.67
    },
    "reasoning": "Low dilution (5.2% CAGR), infrequent halts (1/yr), minimal raises (0.67/yr)"
  }
  ```
- [ ] Logic:
  ```
  shares_cagr = ((current / 3y_ago) ^ (1/3) - 1) * 100
    IF 3y_ago unavailable: shares_cagr = 0 (assume no dilution)

  halts_pa = count(TRADING_HALT) / (months / 12)
  raises_pa = count(CAPITAL_RAISE) / (months / 12)

  score = 0 if (shares_cagr > 25 OR halts_pa > 4)
  score = 1 if (shares_cagr > 10 OR halts_pa > 2)
  score = 2 if (shares_cagr <= 10 AND halts_pa <= 2 AND raises_pa <= 2)
  ```

#### 3.3 Gate Check Utility
- [ ] Create `internal/tools/gate/gate.go`
- [ ] `CheckGate(bfs, cds int) (passed bool, reason string)`
- [ ] Gate passes when: `bfs >= 1 AND cds >= 1`

#### 3.4 Tests
- [ ] BFS edge cases: zero revenue, negative cash burn, missing fields
- [ ] CDS edge cases: no 3y share data, no trading halts
- [ ] Gate boundary conditions

### Deliverables
- 2 gate calculation tools
- Gate check utility
- Unit tests

---

## Stage 4: Score Tools

**Goal:** Implement NFR, PPS, VRS, OB calculations for stocks that pass the gate.

### Tasks

#### 4.1 Implement `calculate_nfr` (Narrative-to-Fact Ratio)
- [ ] Create `internal/tools/score/nfr.go`
- [ ] Input: `{announcements, prices, sector_index?}`
- [ ] Output: `{score: 0.0-1.0, components, reasoning}`
- [ ] Logic:
  ```
  FOR each announcement:
    stock_return = (close[T+2] - close[T-1]) / close[T-1]
    sector_return = (index[T+2] - index[T-1]) / index[T-1]
    abnormal_return = stock_return - sector_return
    IF abs(abnormal_return) > 0.03: impactful++

  nfr = impactful / total
  ```
- [ ] Fetch XJO index data for sector return
- [ ] Handle missing price days (weekends/holidays)

#### 4.2 Implement `calculate_pps` (Price Progression Score)
- [ ] Create `internal/tools/score/pps.go`
- [ ] Input: `{announcements, prices}`
- [ ] Output: `{score: 0.0-1.0, components, event_details[], reasoning}`
- [ ] Logic:
  ```
  FOR each price_sensitive announcement (dedupe by date):
    pre_low = MIN(closes[T-10 to T-1])
    post_low = MIN(closes[T+1 to T+10])
    IF post_low > pre_low: improved++

  pps = improved / total_events
  ```
- [ ] Return per-event details for transparency

#### 4.3 Implement `calculate_vrs` (Volatility Regime Stability)
- [ ] Create `internal/tools/score/vrs.go`
- [ ] Input: `{announcements, prices}`
- [ ] Output: `{score: 0.0-1.0, components, reasoning}`
- [ ] Logic:
  ```
  FOR each price_sensitive announcement:
    vol_pre = stddev(returns[T-15 to T-1])
    vol_post = stddev(returns[T+1 to T+15])
    price_change = (close[T+10] - close[T-1]) / close[T-1]

    IF vol_post < vol_pre AND price_change > 0.01: trend_forming++
    ELSE IF vol_post >= vol_pre AND price_change <= 0.01: destabilising++
    ELSE: neutral++

  vrs = trend_forming / (trend_forming + destabilising)
    IF denominator == 0: vrs = 0.5
  ```

#### 4.4 Implement `calculate_ob` (Optionality Bonus)
- [ ] Create `internal/tools/score/ob.go`
- [ ] Input: `{announcements, fundamentals, bfs_score}`
- [ ] Output: `{score: 0.0|0.5|1.0, components, reasoning}`
- [ ] Logic:
  ```
  IF bfs_score < 1: return 0

  catalyst_keywords = [
    "drilling commenced", "phase 3", "offtake agreement", "FID",
    "binding agreement", "first production", "FDA approval",
    "commercial agreement", "construction commenced"
  ]

  time_keywords = [
    "Q1", "Q2", "Q3", "Q4", "2025", "2026", "2027",
    "expected by", "scheduled for", "targeting", "on track for"
  ]

  recent = announcements WHERE date > (today - 6 months)
  has_catalyst = ANY(recent.headline contains catalyst_keywords)
  has_timeframe = ANY(recent.headline contains time_keywords)

  score = 1.0 if (has_catalyst AND has_timeframe)
  score = 0.5 if (has_catalyst OR has_timeframe)
  score = 0.0 otherwise
  ```

#### 4.5 Shared Utilities
- [ ] Create `internal/tools/score/math.go`
- [ ] `PriceWindow(prices, date, before, after)` - Get price slice around date
- [ ] `Returns(prices)` - Calculate daily returns
- [ ] `Stddev(values)` - Standard deviation
- [ ] `MinClose(prices)` - Minimum closing price

#### 4.6 Tests
- [ ] Each score tool with deterministic test data
- [ ] Edge cases: no price-sensitive announcements, insufficient history
- [ ] Verify score bounds (0.0-1.0)

### Deliverables
- 4 score tools
- Math utilities
- Unit tests

---

## Stage 5: Rating Tool

**Goal:** Combine gate and score results into final investability rating.

### Tasks

#### 5.1 Implement `calculate_rating`
- [ ] Create `internal/tools/rating/rating.go`
- [ ] Input: `{ticker, bfs, cds, nfr?, pps?, vrs?, ob?}`
- [ ] Output:
  ```json
  {
    "ticker": "GNP",
    "company_name": "Genusplus Group Ltd",
    "rated_at": "2026-01-07T10:30:00Z",
    "gate": {
      "passed": true,
      "bfs": 2,
      "cds": 2
    },
    "scores": {
      "nfr": 0.35,
      "pps": 0.65,
      "vrs": 0.52,
      "ob": 0.5
    },
    "investability": 72.5,
    "label": "INVESTABLE",
    "component_details": { ... }
  }
  ```
- [ ] Logic:
  ```
  passed_gate = (bfs >= 1) AND (cds >= 1)

  IF NOT passed_gate:
    label = SPECULATIVE
    investability = null
  ELSE:
    investability = (bfs * 12.5) + (cds * 12.5)
                  + (nfr * 25) + (pps * 25)
                  + (vrs * 15) + (ob * 10)

    label = HIGH_CONVICTION if investability >= 80
    label = INVESTABLE      if investability >= 60
    label = WATCHLIST       if investability >= 40
    label = LOW_ALPHA       otherwise
  ```

#### 5.2 Label Descriptions
- [ ] Create `internal/tools/rating/labels.go`
- [ ] Add descriptions for report generation:
  - **SPECULATIVE**: Failed gate - high risk of capital destruction
  - **LOW_ALPHA**: Passed gate but weak execution signals
  - **WATCHLIST**: Moderate potential - monitor for improvement
  - **INVESTABLE**: Strong fundamentals and execution
  - **HIGH_CONVICTION**: Top-tier opportunity

#### 5.3 Tests
- [ ] All label threshold boundaries
- [ ] Gate failure produces SPECULATIVE with null investability
- [ ] Score weights sum correctly

### Deliverables
- Rating calculation tool
- Label definitions
- Unit tests

---

## Stage 6: Output Tools

**Goal:** Generate summaries, markdown reports, and email output.

### Tasks

#### 6.1 Implement `generate_summary`
- [ ] Create `internal/tools/output/summary.go`
- [ ] Input: `{ticker, announcements, rating, max_announcements?}`
- [ ] Output:
  ```json
  {
    "ticker": "GNP",
    "summary": "GNP secured multiple BESS contracts in Q4, expanding renewable energy exposure. Strong order book visibility through FY26.",
    "key_events": [
      {"date": "2025-11-15", "headline": "...", "significance": "..."}
    ]
  }
  ```
- [ ] Use LLM service for summary generation
- [ ] Prompt: summarize recent announcements in 2-3 sentences

#### 6.2 Implement `generate_report`
- [ ] Create `internal/tools/output/report.go`
- [ ] Input: `{ratings[], summaries[], format, include_components?}`
- [ ] Formats:
  - **table**: Quick overview with scores
  - **detailed**: Full breakdown per ticker
  - **full**: Table + detailed sections
- [ ] Use Go `text/template`

#### 6.3 Create Templates
- [ ] Create `internal/tools/output/templates/`
- [ ] `table.md.tmpl`:
  ```markdown
  # Stock Ratings

  | Ticker | Label | Score | BFS | CDS | NFR | PPS | VRS | OB |
  |--------|-------|-------|-----|-----|-----|-----|-----|-----|
  {{range .Ratings}}
  | {{.Ticker}} | {{.Label}} | {{.Investability | fmt}} | {{.Gate.BFS}} | {{.Gate.CDS}} | ... |
  {{end}}

  Generated: {{.Timestamp}}
  ```
- [ ] `detailed.md.tmpl` - Full single-ticker view
- [ ] `full.md.tmpl` - Combined

#### 6.4 Implement `generate_email`
- [ ] Create `internal/tools/output/email.go`
- [ ] Input: `{report, subject_template?, recipient}`
- [ ] Output: `{subject, html_body, plain_text_body, recipient}`
- [ ] Markdown → HTML conversion
- [ ] Plain text fallback

#### 6.5 Tests
- [ ] Template rendering
- [ ] Email HTML output
- [ ] Summary generation (mock LLM)

### Deliverables
- 3 output tools
- Markdown templates
- Unit tests

---

## Stage 7: Orchestration Integration

**Goal:** Register new tools with existing orchestrator and create job definitions.

The orchestrator framework already exists in `orchestrator_worker.go`. This stage focuses on:
1. Creating tool execution workers for each new tool
2. Defining job definitions that wire tools together
3. Testing the integrated flow

### Tasks

#### 7.1 Create Tool Execution Workers
Each tool needs a worker that the orchestrator can invoke via `JobTypeToolExecution`.

- [ ] Create `internal/queue/workers/rating/` package
- [ ] `data_worker.go` - Wraps get_announcements, get_prices, get_fundamentals
- [ ] `gate_worker.go` - Wraps calculate_bfs, calculate_cds
- [ ] `score_worker.go` - Wraps calculate_nfr, calculate_pps, calculate_vrs, calculate_ob
- [ ] `rating_worker.go` - Wraps calculate_rating
- [ ] `output_worker.go` - Wraps generate_summary, generate_report, generate_email

Each worker:
- Implements `interfaces.JobWorker`
- Parses params from job payload
- Calls the corresponding tool from `internal/tools/`
- Returns result as document or metadata

#### 7.2 Register Workers
- [ ] Update `internal/queue/workers/registry.go`
- [ ] Register new workers with `JobTypeToolExecution` handler
- [ ] Map tool names to worker types:
  ```go
  "get_announcements"  → WorkerTypeRatingData
  "get_prices"         → WorkerTypeRatingData
  "get_fundamentals"   → WorkerTypeRatingData
  "calculate_bfs"      → WorkerTypeRatingGate
  "calculate_cds"      → WorkerTypeRatingGate
  "calculate_nfr"      → WorkerTypeRatingScore
  "calculate_pps"      → WorkerTypeRatingScore
  "calculate_vrs"      → WorkerTypeRatingScore
  "calculate_ob"       → WorkerTypeRatingScore
  "calculate_rating"   → WorkerTypeRatingComposite
  "generate_summary"   → WorkerTypeRatingOutput
  "generate_report"    → WorkerTypeRatingOutput
  "generate_email"     → WorkerTypeRatingOutput
  ```

#### 7.3 Create Job Definition
- [ ] Create `jobs/stock-rating.toml`
  ```toml
  [job]
  name = "Stock Rating"
  description = "Rate stocks using investability framework"

  [config]
  variables = []  # Tickers injected at runtime

  [[steps]]
  name = "orchestrate_rating"
  type = "orchestrator"

  [steps.config]
  goal = "Rate each stock and generate a report"
  thinking_level = "MEDIUM"
  output_tags = ["stock-rating"]

  [[steps.config.available_tools]]
  name = "get_announcements"
  worker = "rating_data"
  description = "Fetch ASX announcements for a ticker"

  [[steps.config.available_tools]]
  name = "get_prices"
  worker = "rating_data"
  description = "Fetch OHLCV price data for a ticker"

  # ... (all 13 tools)
  ```

#### 7.4 Update Orchestrator Planner Prompt
- [ ] Add rating-specific guidance to `plannerSystemPrompt`
- [ ] Document tool dependencies:
  - Data tools run first (parallel)
  - Gate tools depend on data tools
  - Score tools depend on data tools AND gate pass
  - Rating tool depends on all scores
  - Output tools depend on rating

#### 7.5 Integration Tests
- [ ] Full flow: "Rate GNP" → markdown report
- [ ] Batch: "Rate GNP, SKS, EXR"
- [ ] Gate failure handling (early exit)
- [ ] Verify document tagging for downstream consumers

### Deliverables
- Tool execution workers
- Worker registration
- Job definition file
- Integration tests

---

## Stage 8: Cleanup

**Goal:** Remove old code and verify system integrity.

### Tasks

#### 8.1 Delete Old Signals
- [ ] Delete `internal/signals/` directory entirely:
  - `pbas.go`, `vli.go`, `regime.go`, `cooked.go`
  - `rs.go`, `quality.go`, `justified.go`
  - `computer.go`, `types.go`

#### 8.2 Delete Old Workers
- [ ] Delete from `internal/queue/workers/`:
  - `market_data_worker.go`
  - `market_fundamentals_worker.go`
  - `market_announcements_worker.go`
  - `market_signal_worker.go`
  - `market_assessor_worker.go`
  - `signal_analysis_worker.go`
  - `signal_analysis_classifier.go`
  - `market_base.go` (if no longer needed)

#### 8.3 Delete Unused Schemas
- [ ] Review `internal/schemas/`
- [ ] Delete schemas not used by new tools

#### 8.4 Update Imports
- [ ] Find all imports of deleted packages
- [ ] Update or remove dependent code

#### 8.5 Verification
- [ ] All tests pass
- [ ] No orphaned imports
- [ ] Application builds cleanly

### Deliverables
- Clean codebase
- No dead code
- Passing build

---

## Stage 9: Verification

**Goal:** End-to-end testing against expected outcomes.

### Tasks

#### 9.1 Golden Test Cases
| Ticker | Expected Label | Gate | Investability |
|--------|----------------|------|---------------|
| BHP | LOW_ALPHA | Pass | 40-50 |
| EXR | SPECULATIVE | Fail (CDS=0) | null |
| CSL | LOW_ALPHA | Pass | 40-50 |
| GNP | INVESTABLE | Pass | 65-75 |
| SKS | INVESTABLE | Pass | 65-75 |

#### 9.2 Integration Tests
- [ ] Create `tests/integration/rating_test.go`
- [ ] Test each golden case
- [ ] Verify output schemas
- [ ] Verify report generation

#### 9.3 Performance
- [ ] Benchmark tool execution
- [ ] Full rating flow <30s per ticker
- [ ] Batch processing throughput

### Deliverables
- Passing golden tests
- Performance benchmarks

---

## Implementation Order

```
Stage 1: Foundation          ─── Types, interfaces, registry
          ↓
Stage 2: Data Tools          ─── get_announcements, get_prices, get_fundamentals
          ↓
Stage 3: Gate Tools          ─── calculate_bfs, calculate_cds
          ↓
Stage 4: Score Tools         ─── calculate_nfr, calculate_pps, calculate_vrs, calculate_ob
          ↓
Stage 5: Rating Tool         ─── calculate_rating
          ↓
Stage 6: Output Tools        ─── generate_summary, generate_report, generate_email
          ↓
Stage 7: Orchestration       ─── Register tools with existing orchestrator
          ↓
Stage 8: Cleanup             ─── Delete old code
          ↓
Stage 9: Verification        ─── Golden tests, benchmarks
```

## Success Criteria

- [ ] All golden test cases pass
- [ ] Full rating flow <30s per ticker
- [ ] Schema validation 100% compliant
- [ ] No dead code remaining
- [ ] Clean build with no warnings

---

*Document Version: 2.0*
*Created: 2026-01-07*
