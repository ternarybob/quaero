# Stock Rating System - Staged Refactor Plan

## Overview

This document outlines the staged implementation plan for refactoring the existing worker system into LLM-orchestrated tools. Each stage is designed to be independently testable and deployable.

**Target Architecture:** Transform monolithic workers into composable tools with explicit schemas, enabling LLM-driven orchestration of stock ratings.

**Current State:**
- Signal computations exist in `internal/signals/` (PBAS, VLI, Regime, etc.)
- Data workers exist in `internal/queue/workers/` (market_fundamentals, market_announcements, etc.)
- LLM service abstraction exists in `internal/services/llm/`
- Tool execution framework partially exists in `tool_execution_worker.go`

**Gap Analysis:**
The requirements specify BFS, CDS, NFR, PPS, VRS, OB calculations which differ from existing signal implementations. These need to be built as new tools while leveraging existing data infrastructure.

---

## Stage 0: Preparation & Interfaces

**Goal:** Establish the foundational interfaces and project structure without disrupting existing functionality.

### Tasks

#### 0.1 Create Tool Interface Definition
- [ ] Define `Tool` interface in `internal/tools/interfaces.go`
  ```go
  type Tool interface {
      Name() string
      Description() string
      Category() ToolCategory  // data | calculation | rating | output
      InputSchema() json.RawMessage
      OutputSchema() json.RawMessage
      Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
  }
  ```

#### 0.2 Create Tool Registry
- [ ] Create `internal/tools/registry.go`
- [ ] Implement `ToolRegistry` for LLM tool discovery
- [ ] Add method to export tools in LLM-compatible format (Claude/Gemini tool schemas)

#### 0.3 Create Package Structure
```
internal/tools/
├── interfaces.go          # Tool interface definitions
├── registry.go            # Tool registration and discovery
├── errors.go              # Standardized error types
├── validation.go          # JSON schema validation helpers
├── data/                  # Data workers (Stage 1)
│   ├── announcements.go
│   ├── prices.go
│   └── fundamentals.go
├── gate/                  # Gate calculation workers (Stage 2)
│   ├── bfs.go
│   └── cds.go
├── score/                 # Score calculation workers (Stage 3)
│   ├── nfr.go
│   ├── pps.go
│   ├── vrs.go
│   └── ob.go
├── rating/                # Rating worker (Stage 4)
│   └── composite.go
└── output/                # Output workers (Stage 5)
    ├── summary.go
    ├── report.go
    └── email.go
```

#### 0.4 Define Shared Types
- [ ] Create `internal/tools/types/` package
- [ ] Define core types: `Ticker`, `Announcement`, `PriceBar`, `Fundamentals`
- [ ] Define enums: `AnnouncementType`, `RatingLabel`, `DataQuality`
- [ ] Define score types: `GateScore`, `ComponentScore`, `InvestabilityScore`

#### 0.5 Create JSON Schemas
- [ ] Create `internal/schemas/tools/` directory
- [ ] Add schema files for each tool input/output
- [ ] Set up schema validation in tool execution path

### Deliverables
- Tool interface contract
- Package structure ready for implementation
- Shared types available for all stages
- JSON schemas for validation

### Dependencies
- None (preparation stage)

### Estimated Scope
- ~8 files, ~400 lines of code

---

## Stage 1: Data Workers

**Goal:** Implement data collection tools that wrap existing EODHD and ASX APIs with explicit schemas.

### Tasks

#### 1.1 Implement `get_announcements` Tool
- [ ] Create `internal/tools/data/announcements.go`
- [ ] Wrap existing `market_announcements_worker.go` logic
- [ ] Input: `{ticker, months, types[]}`
- [ ] Output: Classified announcements with types
- [ ] Add announcement type classification:
  - TRADING_HALT, CAPITAL_RAISE, RESULTS, CONTRACT, OPERATIONAL, COMPLIANCE, OTHER
- [ ] Leverage existing Markit Digital API integration

#### 1.2 Implement `get_prices` Tool
- [ ] Create `internal/tools/data/prices.go`
- [ ] Wrap existing EODHD price fetching
- [ ] Input: `{ticker, months, include_derived}`
- [ ] Output: OHLCV with optional derived metrics (returns, volatility)
- [ ] Use existing exchange-aware staleness checking

#### 1.3 Implement `get_fundamentals` Tool
- [ ] Create `internal/tools/data/fundamentals.go`
- [ ] Wrap existing `market_fundamentals_worker.go` logic
- [ ] Input: `{ticker}`
- [ ] Output: Normalized fundamentals with data quality indicator
- [ ] Map EODHD fields to required schema:
  - `shares_outstanding_current`, `shares_outstanding_3y_ago`
  - `cash_balance`, `quarterly_cash_burn`
  - `revenue_ttm`, `is_profitable`, `has_producing_asset`

#### 1.4 Add Data Caching Layer
- [ ] Implement tool-level caching using existing BadgerDB storage
- [ ] Tag-based retrieval: `{tool_name}:{ticker}:{params_hash}`
- [ ] Staleness checking per exchange trading hours

#### 1.5 Unit Tests
- [ ] Test each data tool independently
- [ ] Mock API responses for deterministic testing
- [ ] Verify schema compliance of outputs

### Deliverables
- 3 data tools with full schema compliance
- Caching layer for tool outputs
- Unit test coverage >80%

### Dependencies
- Stage 0 complete
- Existing EODHD service (`internal/services/eodhd/`)
- Existing announcements service (`internal/services/markit/`)

### Estimated Scope
- ~6 files, ~600 lines of code
- ~3 test files, ~400 lines of tests

---

## Stage 2: Gate Calculation Workers

**Goal:** Implement BFS and CDS calculations that determine if a stock passes the gate for full rating.

### Tasks

#### 2.1 Implement `calculate_bfs` Tool (Business Foundation Score)
- [ ] Create `internal/tools/gate/bfs.go`
- [ ] Input: `{fundamentals}` (output from get_fundamentals)
- [ ] Output: `{score: 0|1|2, components: {...}, reasoning}`
- [ ] Calculation logic:
  ```
  cash_runway = cash_balance / abs(quarterly_cash_burn)
  IF quarterly_cash_burn >= 0: cash_runway = 999

  indicators = 0
  IF revenue_ttm > 10,000,000: indicators++
  IF cash_runway > 18: indicators++
  IF has_producing_asset: indicators++
  IF is_profitable: indicators++

  score = 2 if indicators >= 2 else (1 if indicators == 1 else 0)
  ```

#### 2.2 Implement `calculate_cds` Tool (Capital Discipline Score)
- [ ] Create `internal/tools/gate/cds.go`
- [ ] Input: `{fundamentals, announcements}`
- [ ] Output: `{score: 0|1|2, components: {...}, reasoning}`
- [ ] Calculation logic:
  ```
  shares_cagr = ((current / 3y_ago) ^ (1/3) - 1) * 100
  halts_pa = halt_count / (months / 12)
  raises_pa = raise_count / (months / 12)

  score = 0 if (shares_cagr > 25 OR halts_pa > 4)
  score = 1 if (shares_cagr > 10 OR halts_pa > 2)
  score = 2 if (shares_cagr <= 10 AND halts_pa <= 2 AND raises_pa <= 2)
  ```
- [ ] Count announcement types from classified announcements

#### 2.3 Gate Check Helper
- [ ] Create `internal/tools/gate/check.go`
- [ ] Implement `CheckGate(bfs, cds) (passed bool, reason string)`
- [ ] Gate passes when: `BFS >= 1 AND CDS >= 1`

#### 2.4 Unit Tests
- [ ] Test BFS with various fundamental scenarios
- [ ] Test CDS with various announcement/share dilution scenarios
- [ ] Test gate check logic
- [ ] Edge cases: missing data, zero values, negative cash burn

### Deliverables
- 2 gate calculation tools
- Gate check utility
- Unit tests with edge cases

### Dependencies
- Stage 0 complete (interfaces)
- Stage 1 complete (data tool outputs used as inputs)

### Estimated Scope
- ~4 files, ~400 lines of code
- ~2 test files, ~300 lines of tests

---

## Stage 3: Score Calculation Workers

**Goal:** Implement the four score calculations (NFR, PPS, VRS, OB) that measure investability components.

### Tasks

#### 3.1 Implement `calculate_nfr` Tool (Narrative-to-Fact Ratio)
- [ ] Create `internal/tools/score/nfr.go`
- [ ] Input: `{announcements, prices, sector_index?}`
- [ ] Output: `{score: 0.0-1.0, components: {...}, reasoning}`
- [ ] Calculation logic:
  ```
  FOR each announcement:
    abnormal_return = stock_return[T-1 to T+2] - sector_return[T-1 to T+2]
    IF abs(abnormal_return) > 0.03: impactful++

  nfr = impactful / total
  ```
- [ ] Fetch sector index data (XJO default) for abnormal return calculation
- [ ] Handle weekends/holidays in date window

#### 3.2 Implement `calculate_pps` Tool (Price Progression Score)
- [ ] Create `internal/tools/score/pps.go`
- [ ] Input: `{announcements, prices}`
- [ ] Output: `{score: 0.0-1.0, components: {...}, event_details: [...], reasoning}`
- [ ] Calculation logic:
  ```
  FOR each price_sensitive announcement (deduplicated by date):
    pre_low = MIN(closes[T-10 to T-1])
    post_low = MIN(closes[T+1 to T+10])
    IF post_low > pre_low: improved++

  pps = improved / total_events
  ```
- [ ] Return event-level details for transparency

#### 3.3 Implement `calculate_vrs` Tool (Volatility Regime Stability)
- [ ] Create `internal/tools/score/vrs.go`
- [ ] Input: `{announcements, prices}`
- [ ] Output: `{score: 0.0-1.0, components: {...}, reasoning}`
- [ ] Calculation logic:
  ```
  FOR each price_sensitive announcement:
    vol_pre = stddev(returns[T-15 to T-1])
    vol_post = stddev(returns[T+1 to T+15])
    price_change = (close[T+10] - close[T-1]) / close[T-1]

    IF vol_post < vol_pre AND price_change > 0.01: trend_forming++
    ELSE IF vol_post >= vol_pre AND price_change <= 0.01: destabilising++
    ELSE: neutral++

  vrs = trend_forming / (trend_forming + destabilising)
    IF denominator = 0: vrs = 0.5
  ```

#### 3.4 Implement `calculate_ob` Tool (Optionality Bonus)
- [ ] Create `internal/tools/score/ob.go`
- [ ] Input: `{announcements, fundamentals, bfs_score}`
- [ ] Output: `{score: 0.0|0.5|1.0, components: {...}, reasoning}`
- [ ] Calculation logic:
  ```
  IF bfs_score < 1: return 0

  catalyst_keywords = ["drilling commenced", "phase 3", "offtake", "FID", ...]
  time_keywords = ["Q1", "Q2", "2025", "expected by", ...]

  recent = announcements WHERE date > (today - 6 months)
  has_catalyst = ANY(recent.headline CONTAINS catalyst_keywords)
  has_timeframe = ANY(recent.headline CONTAINS time_keywords)

  score = 1.0 if (has_catalyst AND has_timeframe)
  score = 0.5 if (has_catalyst OR has_timeframe)
  score = 0.0 otherwise
  ```
- [ ] Define comprehensive keyword lists for catalysts and timeframes

#### 3.5 Shared Utilities
- [ ] Create `internal/tools/score/utils.go`
- [ ] Implement date-window helpers for price lookups
- [ ] Implement return calculation helpers
- [ ] Implement volatility (stddev) calculation

#### 3.6 Unit Tests
- [ ] Test each score tool with known scenarios
- [ ] Test edge cases: no announcements, insufficient price history
- [ ] Test date boundary conditions

### Deliverables
- 4 score calculation tools
- Shared calculation utilities
- Unit tests with known-value verification

### Dependencies
- Stage 0 complete (interfaces, types)
- Stage 1 complete (data tools)
- Stage 2 complete (BFS score needed for OB)

### Estimated Scope
- ~6 files, ~800 lines of code
- ~4 test files, ~600 lines of tests

---

## Stage 4: Rating Worker

**Goal:** Implement the composite rating calculation that combines all scores into a final investability rating.

### Tasks

#### 4.1 Implement `calculate_rating` Tool
- [ ] Create `internal/tools/rating/composite.go`
- [ ] Input: `{ticker, bfs, cds, nfr?, pps?, vrs?, ob?}`
- [ ] Output: Full `StockRating` schema with:
  - `gate: {passed, bfs, cds}`
  - `scores: {nfr, pps, vrs, ob}`
  - `investability` (weighted score)
  - `label` (classification)
  - `component_details` (full outputs from each tool)
- [ ] Calculation logic:
  ```
  passed_gate = (bfs.score >= 1) AND (cds.score >= 1)

  IF NOT passed_gate:
    label = "SPECULATIVE"
    investability = null
  ELSE:
    investability = (bfs.score * 12.5) + (cds.score * 12.5)
                  + (nfr.score * 25) + (pps.score * 25)
                  + (vrs.score * 15) + (ob.score * 10)

    label = "HIGH_CONVICTION" if investability >= 80
    label = "INVESTABLE" if investability >= 60
    label = "WATCHLIST" if investability >= 40
    label = "LOW_ALPHA" otherwise
  ```

#### 4.2 Label Assignment Logic
- [ ] Create `internal/tools/rating/labels.go`
- [ ] Define label thresholds as configuration
- [ ] Add label descriptions for reports:
  - SPECULATIVE: Failed gate - high risk of capital destruction
  - LOW_ALPHA: Passed gate but weak execution signals
  - WATCHLIST: Moderate investability - monitor for improvement
  - INVESTABLE: Strong fundamentals and execution
  - HIGH_CONVICTION: Top-tier opportunity

#### 4.3 Rating Persistence
- [ ] Define storage interface for rating history
- [ ] Store ratings with timestamp for trend analysis
- [ ] Query interface: latest rating, rating history by ticker

#### 4.4 Unit Tests
- [ ] Test complete rating flow with mocked tool outputs
- [ ] Test each label threshold boundary
- [ ] Test gate failure scenarios
- [ ] Verify investability score calculations

### Deliverables
- Composite rating tool
- Label assignment logic
- Rating persistence interface
- Unit tests covering all scenarios

### Dependencies
- Stage 0 complete
- Stage 2 complete (gate tools)
- Stage 3 complete (score tools)

### Estimated Scope
- ~4 files, ~400 lines of code
- ~2 test files, ~300 lines of tests

---

## Stage 5: Output Workers

**Goal:** Implement output generation tools for summaries, markdown reports, and email.

### Tasks

#### 5.1 Implement `generate_summary` Tool
- [ ] Create `internal/tools/output/summary.go`
- [ ] Input: `{ticker, announcements, rating, max_announcements?}`
- [ ] Output: `{summary, key_events: [...]}`
- [ ] Use existing LLM service for summary generation
- [ ] Prompt engineering for concise 2-3 sentence summaries
- [ ] Extract key events with significance descriptions

#### 5.2 Implement `generate_report` Tool
- [ ] Create `internal/tools/output/report.go`
- [ ] Input: `{ratings: [], summaries: [], format, include_components?}`
- [ ] Output: `{markdown, metadata}`
- [ ] Implement format templates:
  - **table**: Quick overview table
  - **detailed**: Full breakdown per ticker
  - **full**: Table + detailed for each
- [ ] Use Go `text/template` for markdown generation

#### 5.3 Implement Markdown Templates
- [ ] Create `internal/tools/output/templates/`
- [ ] `table.md.tmpl` - Rating table template
- [ ] `detailed.md.tmpl` - Single ticker detail template
- [ ] `full.md.tmpl` - Combined template
- [ ] Support for conditional rendering (gate pass/fail)

#### 5.4 Implement `generate_email` Tool
- [ ] Create `internal/tools/output/email.go`
- [ ] Input: `{report, subject_template?, recipient}`
- [ ] Output: `{subject, html_body, plain_text_body, recipient}`
- [ ] Markdown to HTML conversion
- [ ] Plain text fallback generation
- [ ] Subject line templating with date/count

#### 5.5 Unit Tests
- [ ] Test summary generation (mock LLM responses)
- [ ] Test each report format
- [ ] Test email HTML/plain text generation
- [ ] Verify template rendering

### Deliverables
- 3 output tools
- Markdown templates
- Email conversion utilities
- Unit tests

### Dependencies
- Stage 0 complete
- Stage 4 complete (rating outputs)
- LLM service (`internal/services/llm/`)

### Estimated Scope
- ~8 files, ~600 lines of code
- ~3 test files, ~400 lines of tests

---

## Stage 6: LLM Orchestration

**Goal:** Enable LLM-driven orchestration of tools to rate stocks autonomously.

### Tasks

#### 6.1 Tool Schema Export
- [ ] Create `internal/tools/export.go`
- [ ] Export tool definitions in Claude tool format
- [ ] Export tool definitions in Gemini function format
- [ ] Include input/output schemas in export

#### 6.2 Implement Orchestrator
- [ ] Create `internal/tools/orchestrator/orchestrator.go`
- [ ] Accept natural language requests (e.g., "Rate GNP")
- [ ] Generate tool call sequence via LLM
- [ ] Execute tools and collect results
- [ ] Handle gate failures (early exit)
- [ ] Support batch processing (multiple tickers)

#### 6.3 Tool Execution Engine
- [ ] Create `internal/tools/orchestrator/executor.go`
- [ ] Parse LLM tool calls
- [ ] Route to appropriate tool
- [ ] Validate inputs/outputs against schemas
- [ ] Return structured results to LLM

#### 6.4 Conversation State Management
- [ ] Track tool call history
- [ ] Maintain intermediate results between calls
- [ ] Support multi-turn orchestration

#### 6.5 Orchestration Worker
- [ ] Update `orchestrator_worker.go` to use new tool system
- [ ] Support job queue integration
- [ ] Batch rating jobs

#### 6.6 Integration Tests
- [ ] Test full orchestration flow: "Rate GNP"
- [ ] Test batch rating: "Rate GNP, SKS, EXR"
- [ ] Test gate failure handling
- [ ] Test error recovery

### Deliverables
- Tool schema export utilities
- Orchestrator implementation
- Tool execution engine
- Integration tests

### Dependencies
- Stages 0-5 complete (all tools implemented)
- LLM service

### Estimated Scope
- ~6 files, ~800 lines of code
- ~3 test files, ~500 lines of tests

---

## Stage 7: Error Handling & Validation

**Goal:** Implement comprehensive error handling and cross-worker validation.

### Tasks

#### 7.1 Standardized Error Types
- [ ] Define error codes in `internal/tools/errors.go`:
  - TICKER_NOT_FOUND
  - INSUFFICIENT_DATA
  - STALE_DATA
  - CALCULATION_ERROR
  - MISSING_INPUT
  - GENERATION_ERROR
- [ ] Implement error schema with partial results

#### 7.2 Input Validation
- [ ] JSON Schema validation on tool inputs
- [ ] Ticker format validation (3-4 uppercase alphanumeric)
- [ ] Date range validation
- [ ] Required field validation

#### 7.3 Output Validation
- [ ] JSON Schema validation on tool outputs
- [ ] Score range validation (0-2 for gates, 0-1 for components)
- [ ] Data consistency checks

#### 7.4 Cross-Tool Validation
- [ ] Verify ticker consistency across tool chain
- [ ] Validate date range alignment (prices and announcements)
- [ ] Ensure gate check before score calculation
- [ ] Verify all scores within bounds

#### 7.5 Error Recovery
- [ ] Implement retry logic for transient errors
- [ ] Partial rating on data quality issues
- [ ] Graceful degradation for missing components

### Deliverables
- Error type definitions
- Validation utilities
- Recovery mechanisms
- Validation test suite

### Dependencies
- Stages 0-6 complete

### Estimated Scope
- ~4 files, ~400 lines of code
- ~2 test files, ~300 lines of tests

---

## Stage 8: Testing & Verification

**Goal:** Comprehensive end-to-end testing and verification against expected outcomes.

### Tasks

#### 8.1 Golden Test Cases
- [ ] Create expected outcomes for test tickers:
  | Ticker | Expected Label | Gate | Investability |
  |--------|----------------|------|---------------|
  | BHP | LOW_ALPHA | Pass | 40-50 |
  | EXR | SPECULATIVE | Fail (CDS=0) | null |
  | CSL | LOW_ALPHA | Pass | 40-50 |
  | GNP | INVESTABLE | Pass | 65-75 |
  | SKS | INVESTABLE | Pass | 65-75 |

#### 8.2 Integration Test Suite
- [ ] Create `tests/integration/rating_test.go`
- [ ] Test each golden case end-to-end
- [ ] Verify output schema compliance
- [ ] Verify markdown report generation

#### 8.3 Performance Testing
- [ ] Benchmark individual tool execution
- [ ] Measure full rating flow latency
- [ ] Test batch processing throughput
- [ ] Identify bottlenecks

#### 8.4 Load Testing
- [ ] Test concurrent rating requests
- [ ] Verify cache effectiveness
- [ ] Test API rate limit handling

#### 8.5 Documentation
- [ ] Update API documentation
- [ ] Create tool usage examples
- [ ] Document orchestration patterns
- [ ] Update architecture diagrams

### Deliverables
- Golden test suite
- Integration tests
- Performance benchmarks
- Updated documentation

### Dependencies
- Stages 0-7 complete

### Estimated Scope
- ~6 test files, ~800 lines of tests
- Documentation updates

---

## Stage 9: Migration & Deployment

**Goal:** Migrate existing workflows to use new tool system and deploy.

### Tasks

#### 9.1 Feature Flag
- [ ] Add feature flag for new rating system
- [ ] Support gradual rollout
- [ ] A/B comparison with existing system

#### 9.2 Migration Path
- [ ] Identify existing rating consumers
- [ ] Create adapter for backward compatibility
- [ ] Migrate consumers incrementally

#### 9.3 Deprecation Plan
- [ ] Mark old workers as deprecated
- [ ] Set deprecation timeline
- [ ] Communication to stakeholders

#### 9.4 Monitoring
- [ ] Add metrics for tool execution
- [ ] Alert on error rates
- [ ] Track rating distribution changes

#### 9.5 Deployment
- [ ] Staged rollout plan
- [ ] Rollback procedure
- [ ] Verification checklist

### Deliverables
- Feature flag implementation
- Migration adapters
- Monitoring dashboards
- Deployment runbook

### Dependencies
- Stages 0-8 complete
- Staging environment

---

## Implementation Order Summary

```
Stage 0: Preparation & Interfaces     ─┐
                                       │
Stage 1: Data Workers                 ─┤  Foundation
                                       │
Stage 2: Gate Calculation Workers     ─┘

Stage 3: Score Calculation Workers    ─┐
                                       │  Core Logic
Stage 4: Rating Worker                ─┘

Stage 5: Output Workers               ─┐
                                       │  User-Facing
Stage 6: LLM Orchestration            ─┘

Stage 7: Error Handling & Validation  ─┐
                                       │  Quality
Stage 8: Testing & Verification       ─┘

Stage 9: Migration & Deployment       ─   Release
```

## Risk Considerations

| Risk | Mitigation |
|------|------------|
| Data API rate limits | Implement caching layer in Stage 1 |
| LLM cost for summaries | Batch processing, cache summaries |
| Calculation accuracy | Golden tests in Stage 8 |
| Breaking existing workflows | Feature flag, migration adapters |
| Performance degradation | Benchmarking in Stage 8 |

## Success Criteria

- [ ] All golden test cases pass
- [ ] Tool execution latency <2s per tool
- [ ] Full rating flow <30s per ticker
- [ ] Schema validation 100% compliant
- [ ] Zero regressions in existing functionality

---

*Document Version: 1.0*
*Created: 2026-01-07*
*For: Quaero Stock Rating Worker Refactor*
