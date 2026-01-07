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
- [ ] Test each calculation function with known inputs/outputs
- [ ] Test edge cases: missing data, zero values, boundary conditions

### Deliverables
- `internal/services/rating/` package
- Pure calculation functions
- Unit test coverage >90%

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
- [ ] Reads: `asx-stock-data` + `asx-announcement-summary` documents
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

### Deliverables
- 7 rating workers
- Worker registration
- Integration with existing document storage

---

## Stage 3: Job Definition

**Goal:** Create job definition that orchestrates the rating flow.

### Tasks

#### 3.1 Create Rating Job Definition
- [ ] Create `deployments/common/job-definitions/stock-rating.toml`
  ```toml
  id = "stock-rating"
  name = "Stock Rating"
  type = "orchestrator"
  description = "Rate stocks using investability framework"
  tags = ["rating", "analysis"]

  [config]
  variables = [
      { ticker = "GNP" },
      { ticker = "SKS" },
  ]

  # Step 1: Collect data (existing workers)
  [step.fetch_fundamentals]
  type = "market_fundamentals"
  description = "Fetch stock fundamentals"
  on_error = "continue"

  [step.fetch_announcements]
  type = "market_announcements"
  description = "Fetch ASX announcements"
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

  # Step 3: Calculate component scores (only if gate passes)
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

  # Step 5: Email report
  [step.email_report]
  type = "email"
  description = "Email rating report"
  depends = "calculate_rating"
  to = "user@example.com"
  subject = "Stock Rating Report"
  list_tags = ["stock-rating"]
  ```

#### 3.2 Test Job Execution
- [ ] Run job with test tickers
- [ ] Verify all steps execute in order
- [ ] Verify documents created with correct tags

### Deliverables
- Job definition file
- Working end-to-end flow

---

## Stage 4: Output & Reporting

**Goal:** Generate formatted reports from rating documents.

### Tasks

#### 4.1 Report Generator Worker
- [ ] Create `internal/queue/workers/rating_report_worker.go`
- [ ] Reads: All `stock-rating` documents
- [ ] Generates: Markdown report with table + details
- [ ] Outputs: `rating-report` document

#### 4.2 Report Templates
- [ ] Create `internal/templates/rating/`
- [ ] `table.md.tmpl` - Summary table
- [ ] `detailed.md.tmpl` - Per-ticker breakdown

#### 4.3 LLM Summary (Optional)
- [ ] Add summary generation to report worker
- [ ] Use LLM service to summarize key announcements

### Deliverables
- Report generation worker
- Markdown templates
- Email-ready output

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

## Stage 6: Verification

**Goal:** End-to-end testing against expected outcomes.

### Tasks

#### 6.1 Golden Test Cases
| Ticker | Expected Label | Gate | Investability |
|--------|----------------|------|---------------|
| BHP | LOW_ALPHA | Pass | 40-50 |
| EXR | SPECULATIVE | Fail (CDS=0) | null |
| CSL | LOW_ALPHA | Pass | 40-50 |
| GNP | INVESTABLE | Pass | 65-75 |
| SKS | INVESTABLE | Pass | 65-75 |

#### 6.2 Integration Tests
- [ ] Create `test/api/rating_test.go`
- [ ] Test each golden case end-to-end
- [ ] Verify document output schemas

#### 6.3 Performance
- [ ] Full rating flow <30s per ticker
- [ ] Batch processing throughput

### Deliverables
- Passing golden tests
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
