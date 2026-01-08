# Stock Rating System - Staged Refactor Plan

## Overview

This document outlines the staged implementation plan for the stock rating system using LLM-orchestrated workers.

**Approach:** Clean-slate implementation. Breaking changes expected. Old code deleted upon completion.

---

## CRITICAL: Market Worker Separation of Concerns

The market workers must be separated into three distinct responsibilities:

### 1. Data Collection Workers (market_*)
**Purpose:** Fetch raw data from external APIs and store as documents.
**No Processing:** Must NOT include analysis, classification, or scoring logic.

| Worker | Responsibility | Output |
|--------|---------------|--------|
| `market_fundamentals` | Fetch stock fundamentals from EODHD | `[asx-stock-data, {ticker}]` |
| `market_announcements` | Fetch raw announcements from ASX API | `[asx-announcement-raw, {ticker}]` |
| `market_data` | Fetch OHLCV price history from EODHD | `[market-data, {ticker}]` |

### 2. Processing Workers (processing_* or rating_*)
**Purpose:** Read raw data documents, apply analysis/classification, write enriched documents.
**Pure calculation:** Business logic only, no external API calls.

| Worker | Responsibility | Input | Output |
|--------|---------------|-------|--------|
| `processing_announcements` | Classify announcements (relevance, signal-to-noise) | `asx-announcement-raw` | `[asx-announcement-summary, {ticker}]` |
| `rating_bfs` | Calculate Business Foundation Score | `asx-stock-data` | `[rating-bfs, {ticker}]` |
| `rating_cds` | Calculate Capital Discipline Score | `asx-stock-data`, `asx-announcement-summary` | `[rating-cds, {ticker}]` |
| ... | | | |

### 3. Summary Workers
**Purpose:** Aggregate multiple documents into consolidated reports.

| Worker | Responsibility |
|--------|---------------|
| `output_formatter` | Collect documents by tags, format for delivery |
| `email` | Send formatted output via email |

### Current Problem: market_announcements_worker.go

The current `market_announcements_worker.go` violates separation of concerns by mixing:
- **Data Collection:** Fetching announcements from Markit Digital API and ASX HTML ✅
- **Processing:** (SHOULD BE SEPARATE)
  - `classifyRelevance()` - keyword-based relevance classification
  - `calculateSignalNoiseRating()` - signal-to-noise analysis
  - `PriceImpactData` - price impact correlation
  - `analyzeAnnouncements()` - full analysis pipeline
  - MQS (Management Quality Score) calculations

### Required Refactor

1. **Slim down `market_announcements_worker.go`:**
   - Keep ONLY: API fetching, HTML scraping, raw document storage
   - Remove: All classification, analysis, and scoring logic
   - Output: Raw announcement data only (date, headline, type, PDF URL, price-sensitive flag)

2. **Create `processing_announcements_worker.go`:**
   - Reads: Raw announcement documents
   - Applies: Relevance classification, signal-to-noise analysis, price impact
   - Outputs: Enriched announcement summary document

3. **Update job definitions:**
   ```toml
   # announcements-watchlist.toml - Data collection only
   [step.fetch_announcements]
   type = "market_announcements"  # Raw data only

   # stock-rating-watchlist.toml - Full pipeline
   [step.fetch_announcements]
   type = "market_announcements"  # Raw data

   [step.process_announcements]
   type = "processing_announcements"  # Classification & analysis
   depends = "fetch_announcements"
   ```

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

## Market Interface for Worker-to-Worker Communication

Rating workers need data from market workers. To maintain separation of concerns and consistent data flow:

### Interface Definition

```go
// MarketDocumentProvider enables rating workers to request document IDs from market workers.
// This is the only allowed form of worker-to-worker communication.
//
// Protocol:
//   - Request: ticker(s) only (context)
//   - Response: document ID(s) only
//   - Data: caller reads from document storage using returned IDs
type MarketDocumentProvider interface {
    // GetDocumentID returns the document ID for a single ticker.
    // If document doesn't exist or is stale, fetches fresh data first.
    // Returns (docID, nil) on success, ("", error) on failure.
    GetDocumentID(ctx context.Context, ticker string) (string, error)

    // GetDocumentIDs returns document IDs for multiple tickers.
    // Returns map[ticker]docID. Missing/failed tickers have empty string values.
    GetDocumentIDs(ctx context.Context, tickers []string) (map[string]string, error)
}
```

Location: `internal/interfaces/market_provider.go`

### Implementations

Each market worker implements `MarketDocumentProvider`:

| Worker | SourceType | SourceID Format | Document Tags |
|--------|------------|-----------------|---------------|
| `market_fundamentals` | `market_fundamentals` | `{exchange}:{code}:stock_collector` | `ticker:{code}`, `source_type:market_fundamentals` |
| `market_announcements` | `asx_announcement_summary` | `{exchange}:{code}:announcement_summary` | `ticker:{code}`, `source_type:market_announcements` |
| `market_data` | `market_data` | `{exchange}:{code}:market_data` | `ticker:{code}`, `source_type:market_data` |

### Usage in Rating Workers

```go
// RatingBFSWorker depends on fundamentals data
type RatingBFSWorker struct {
    fundamentalsProvider MarketDocumentProvider  // Injected at creation
    documentStorage      interfaces.DocumentStorage
    logger               arbor.ILogger
}

func (w *RatingBFSWorker) Execute(ctx context.Context, job *QueueJob) error {
    ticker := job.Payload["ticker"].(string)

    // 1. Request document ID (context only - just the ticker)
    docID, err := w.fundamentalsProvider.GetDocumentID(ctx, ticker)
    if err != nil {
        return fmt.Errorf("fundamentals not available: %w", err)
    }

    // 2. Read document from storage using returned ID
    doc, err := w.documentStorage.GetDocument(docID)
    if err != nil {
        return fmt.Errorf("failed to read fundamentals doc: %w", err)
    }

    // 3. Extract data for service call
    fundamentals := w.extractFundamentals(doc)

    // 4. Call pure service function
    result := rating.CalculateBFS(fundamentals)

    // 5. Save result as document
    return w.saveResultDocument(ctx, ticker, result)
}
```

### Batch Processing

For workers that need multiple data sources:

```go
// RatingCDSWorker needs fundamentals AND announcements
func (w *RatingCDSWorker) Execute(ctx context.Context, job *QueueJob) error {
    ticker := job.Payload["ticker"].(string)

    // Request both document IDs (can be parallelized)
    fundDocID, err := w.fundamentalsProvider.GetDocumentID(ctx, ticker)
    if err != nil {
        return err
    }
    annDocID, err := w.announcementsProvider.GetDocumentID(ctx, ticker)
    if err != nil {
        return err
    }

    // Read both documents
    fundDoc, _ := w.documentStorage.GetDocument(fundDocID)
    annDoc, _ := w.documentStorage.GetDocument(annDocID)

    // Extract and calculate
    fundamentals := w.extractFundamentals(fundDoc)
    announcements := w.extractAnnouncements(annDoc)
    result := rating.CalculateCDS(fundamentals, announcements, 36) // 3 years

    return w.saveResultDocument(ctx, ticker, result)
}
```

### Multi-Ticker Requests

For batch job processing:

```go
// Process multiple tickers at once
tickers := []string{"ASX.GNP", "ASX.SKS", "ASX.EXR"}
docIDs, err := w.fundamentalsProvider.GetDocumentIDs(ctx, tickers)
if err != nil {
    return err
}

for ticker, docID := range docIDs {
    if docID == "" {
        w.logger.Warn().Str("ticker", ticker).Msg("no fundamentals available")
        continue
    }
    doc, _ := w.documentStorage.GetDocument(docID)
    // Process...
}
```

### Provider Factory

Providers are created via factory to ensure proper dependency injection:

```go
// MarketProviderFactory creates MarketDocumentProvider instances
type MarketProviderFactory interface {
    CreateFundamentalsProvider() MarketDocumentProvider
    CreateAnnouncementsProvider() MarketDocumentProvider
    CreatePriceDataProvider() MarketDocumentProvider
}
```

### Implementation Notes

1. **Caching**: Providers use `BaseMarketWorker` for cache-aware document retrieval
2. **Staleness**: If document is stale, provider triggers refresh before returning ID
3. **Errors**: Return error only for unrecoverable failures; missing data returns empty string
4. **Thread Safety**: Providers must be safe for concurrent use

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

LLM Planner decides which tools to call based on prompt. Uses `orchestrator_worker.go`.

```
Prompt → LLM Planner → selects tools → Executor runs → Reviewer validates → repeat
```

```toml
[step.orchestrate]
type = "orchestrator"
prompt = "Rate the stock and explain the investment thesis"
model = "sonnet"  # Options: haiku, sonnet, opus
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
prompt = "Calculate all rating scores and explain the results. Always calculate all components for consistent output."
model = "sonnet"
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

## Stage 0: Market Worker Separation (PREREQUISITE)

**Goal:** Separate data collection from processing in market workers.

This stage MUST be completed before Stage 1. The market_announcements_worker currently mixes data collection with analysis/classification logic, violating separation of concerns.

### Tasks

#### 0.1 Create Announcement Processing Service
- [ ] Create `internal/services/announcements/` package
- [ ] Move classification logic from worker to service:
  ```go
  // internal/services/announcements/types.go
  type RawAnnouncement struct {
      Date           time.Time
      Headline       string
      Type           string
      PDFURL         string
      DocumentKey    string
      PriceSensitive bool
  }

  type ProcessedAnnouncement struct {
      RawAnnouncement
      RelevanceCategory    string  // HIGH, MEDIUM, LOW, NOISE
      RelevanceReason      string
      SignalNoiseRating    string  // HIGH_SIGNAL, MODERATE_SIGNAL, LOW_SIGNAL, NOISE, ROUTINE
      SignalNoiseRationale string
      PriceImpact          *PriceImpactData
  }
  ```

- [ ] Create `internal/services/announcements/classify.go`:
  ```go
  // ClassifyRelevance returns relevance category based on keywords
  func ClassifyRelevance(headline string, annType string) (category, reason string)

  // CalculateSignalNoise analyzes market impact
  func CalculateSignalNoise(ann RawAnnouncement, priceData []PriceBar) SignalNoiseResult

  // CalculatePriceImpact correlates announcement with price movement
  func CalculatePriceImpact(ann RawAnnouncement, prices []PriceBar) *PriceImpactData
  ```

#### 0.2 Refactor market_announcements_worker.go
- [ ] Remove ALL processing logic from `market_announcements_worker.go`:
  - Delete `classifyRelevance()` function
  - Delete `calculateSignalNoiseRating()` function
  - Delete `analyzeAnnouncements()` function
  - Delete `PriceImpactData` calculation
  - Delete MQS-related calculations
- [ ] Change output document tag from `asx-announcement-summary` to `asx-announcement-raw`
- [ ] Store raw announcements only (no analysis fields)
- [ ] Remove dependencies on `priceProvider` and `fundamentalsProvider`
- [ ] Target file size: <500 lines (currently ~1800 lines)

#### 0.3 Create processing_announcements_worker.go
- [ ] Create new worker `internal/queue/workers/processing_announcements_worker.go`
- [ ] Implements `DefinitionWorker` interface
- [ ] Worker type: `WorkerTypeProcessingAnnouncements = "processing_announcements"`
- [ ] Reads: `asx-announcement-raw` documents
- [ ] Calls: `announcements.ClassifyRelevance()`, `announcements.CalculateSignalNoise()`
- [ ] Outputs: `asx-announcement-summary` documents (enriched with analysis)

#### 0.4 Update Job Definitions
- [ ] Update `announcements-watchlist.toml`:
  - Step 1: `market_announcements` (raw data only)
  - Step 2: `processing_announcements` (classification) - depends on step 1
  - Step 3: `output_formatter` - depends on step 2
  - Step 4: `email` - depends on step 3

- [ ] Update `stock-rating-watchlist.toml`:
  - Add `processing_announcements` step after `fetch_announcements`
  - Update rating worker dependencies to use processed data

#### 0.5 Register New Worker
- [ ] Add `WorkerTypeProcessingAnnouncements` to `internal/models/worker_type.go`
- [ ] Register in `internal/app/app.go`

#### 0.6 Update Tests
- [ ] Update tests for slimmed-down `market_announcements_worker`
- [ ] Create tests for `processing_announcements_worker`
- [ ] Create unit tests for `internal/services/announcements/`

### Deliverables
- Pure data collection in `market_announcements_worker.go` (<500 lines)
- Processing logic in `internal/services/announcements/`
- New `processing_announcements_worker.go` for classification
- Updated job definitions with proper step dependencies
- All existing tests passing

### Verification
```bash
# market_announcements_worker should be pure data collection
grep -c "classifyRelevance\|SignalNoise\|PriceImpact" internal/queue/workers/market_announcements_worker.go
# Expected: 0

# New service should have the processing logic
ls internal/services/announcements/*.go
# Expected: types.go, classify.go

# New worker should exist
ls internal/queue/workers/processing_announcements_worker.go
# Expected: file exists
```

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
  model = "sonnet"  # Options: haiku, sonnet, opus
  prompt = """
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

**Goal:** Create tests for rating workers using existing test framework patterns. Configuration changes only - no test infrastructure changes.

### Testing Framework Reference

Existing test framework in `test/api/market_workers/` - **DO NOT CHANGE STRUCTURE**:
- `common_test.go` - Shared utilities, schemas, helpers
- `*_test.go` - Individual worker tests

**Pattern Reference:** Use existing tests as templates:
- `fundamentals_test.go` - Single worker, schema validation
- `announcements_test.go` - Worker with business rules (MQS scores)
- `data_collection_test.go` - Multi-step job, aggregation

---

### 6.1 Remove Redundant Tests

Delete tests for removed signal workers:
- [ ] Delete `test/api/market_workers/signal_test.go` (market_signal worker removed)
- [ ] Delete `test/api/market_workers/signal_computer_test.go` (signal_computer worker removed)

These workers are replaced by the rating system.

---

### 6.2 Add Rating WorkerSchemas to common_test.go

Add schema definitions following existing pattern in `common_test.go`:

```go
// Rating Worker Schemas - add to common_test.go

// BFSSchema - Business Foundation Score
var BFSSchema = WorkerSchema{
    RequiredFields: []string{"ticker", "score", "indicator_count", "components", "reasoning"},
    OptionalFields: []string{"calculated_at"},
    FieldTypes: map[string]string{
        "ticker":          "string",
        "score":           "number",  // 0, 1, or 2
        "indicator_count": "number",
        "components":      "object",
        "reasoning":       "string",
    },
}

// CDSSchema - Capital Discipline Score
var CDSSchema = WorkerSchema{
    RequiredFields: []string{"ticker", "score", "components", "reasoning"},
    OptionalFields: []string{"calculated_at", "analysis_period_months"},
    FieldTypes: map[string]string{
        "ticker":     "string",
        "score":      "number",  // 0, 1, or 2
        "components": "object",
        "reasoning":  "string",
    },
}

// NFRSchema - Narrative-to-Fact Ratio
var NFRSchema = WorkerSchema{
    RequiredFields: []string{"ticker", "score", "components", "reasoning"},
    OptionalFields: []string{"calculated_at"},
    FieldTypes: map[string]string{
        "ticker":     "string",
        "score":      "number",  // 0.0 to 1.0
        "components": "object",
        "reasoning":  "string",
    },
}

// PPSSchema - Price Progression Score
var PPSSchema = WorkerSchema{
    RequiredFields: []string{"ticker", "score", "event_details", "reasoning"},
    OptionalFields: []string{"calculated_at"},
    FieldTypes: map[string]string{
        "ticker":        "string",
        "score":         "number",  // 0.0 to 1.0
        "event_details": "array",
        "reasoning":     "string",
    },
}

// VRSSchema - Volatility Regime Stability
var VRSSchema = WorkerSchema{
    RequiredFields: []string{"ticker", "score", "components", "reasoning"},
    OptionalFields: []string{"calculated_at"},
    FieldTypes: map[string]string{
        "ticker":     "string",
        "score":      "number",  // 0.0 to 1.0
        "components": "object",
        "reasoning":  "string",
    },
}

// OBSchema - Optionality Bonus
var OBSchema = WorkerSchema{
    RequiredFields: []string{"ticker", "score", "catalyst_found", "timeframe_found", "reasoning"},
    OptionalFields: []string{"calculated_at"},
    FieldTypes: map[string]string{
        "ticker":          "string",
        "score":           "number",  // 0.0, 0.5, or 1.0
        "catalyst_found":  "boolean",
        "timeframe_found": "boolean",
        "reasoning":       "string",
    },
}

// RatingCompositeSchema - Final investability rating
var RatingCompositeSchema = WorkerSchema{
    RequiredFields: []string{"ticker", "label", "investability", "gate_passed", "scores"},
    OptionalFields: []string{"calculated_at", "reasoning"},
    FieldTypes: map[string]string{
        "ticker":        "string",
        "label":         "string",  // SPECULATIVE|LOW_ALPHA|WATCHLIST|INVESTABLE|HIGH_CONVICTION
        "investability": "number",  // 0-100 or null if gate failed
        "gate_passed":   "boolean",
        "scores":        "object",  // All component scores
        "reasoning":     "string",
    },
}
```

---

### 6.3 Add Business Rule Validators to common_test.go

```go
// Rating business rule validators - add to common_test.go

// ValidGateScores - BFS and CDS must be 0, 1, or 2
var ValidGateScores = []float64{0, 1, 2}

// ValidOBScores - OB must be 0.0, 0.5, or 1.0
var ValidOBScores = []float64{0.0, 0.5, 1.0}

// ValidRatingLabels - Enum values for rating label
var ValidRatingLabels = []string{
    "SPECULATIVE",
    "LOW_ALPHA",
    "WATCHLIST",
    "INVESTABLE",
    "HIGH_CONVICTION",
}

// AssertGateScore validates BFS/CDS score is 0, 1, or 2
func AssertGateScore(t *testing.T, score float64, fieldName string) {
    t.Helper()
    valid := score == 0 || score == 1 || score == 2
    assert.True(t, valid, "%s must be 0, 1, or 2, got %v", fieldName, score)
}

// AssertComponentScore validates NFR/PPS/VRS score is 0.0 to 1.0
func AssertComponentScore(t *testing.T, score float64, fieldName string) {
    t.Helper()
    assert.GreaterOrEqual(t, score, 0.0, "%s must be >= 0.0", fieldName)
    assert.LessOrEqual(t, score, 1.0, "%s must be <= 1.0", fieldName)
}

// AssertOBScore validates OB score is 0.0, 0.5, or 1.0
func AssertOBScore(t *testing.T, score float64) {
    t.Helper()
    valid := score == 0.0 || score == 0.5 || score == 1.0
    assert.True(t, valid, "OB score must be 0.0, 0.5, or 1.0, got %v", score)
}

// AssertRatingLabel validates label is valid enum value
func AssertRatingLabel(t *testing.T, label string) {
    t.Helper()
    valid := false
    for _, v := range ValidRatingLabels {
        if label == v {
            valid = true
            break
        }
    }
    assert.True(t, valid, "Invalid rating label: %s", label)
}

// AssertInvestabilityScore validates investability is 0-100 or nil (if gate failed)
func AssertInvestabilityScore(t *testing.T, score interface{}, gatePassed bool) {
    t.Helper()
    if !gatePassed {
        assert.Nil(t, score, "Investability must be nil when gate fails")
        return
    }
    if s, ok := score.(float64); ok {
        assert.GreaterOrEqual(t, s, 0.0, "Investability must be >= 0")
        assert.LessOrEqual(t, s, 100.0, "Investability must be <= 100")
    } else {
        t.Errorf("Investability must be a number, got %T", score)
    }
}
```

---

### 6.4 Rating Worker API Tests

Create test files following existing patterns. Each test:
1. Uses `SetupFreshEnvironment(t)`
2. Creates job definition with worker config
3. Executes job via `CreateAndExecuteJob`
4. Validates schema via `ValidateSchema`
5. Validates business rules
6. Saves output via `SaveWorkerOutput`

#### 6.4.1 rating_bfs_test.go
```go
// test/api/market_workers/rating_bfs_test.go
func TestRatingBFSSingle(t *testing.T) {
    env := SetupFreshEnvironment(t)
    if env == nil { return }
    defer env.Cleanup()

    RequireEODHD(t, env)  // Needs fundamentals data

    helper := env.NewHTTPTestHelper(t)
    body := map[string]interface{}{
        "id": "test-rating-bfs",
        "name": "Test Rating BFS",
        "type": "rating_bfs",
        "enabled": true,
        "tags": []string{"rating", "test"},
        "steps": []map[string]interface{}{
            {
                "name": "calculate-bfs",
                "type": "rating_bfs",
                "config": map[string]interface{}{
                    "ticker": "ASX:GNP",
                },
            },
        },
    }

    jobID, _ := CreateAndExecuteJob(t, helper, body)
    finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
    if finalStatus != "completed" {
        t.Skipf("Job ended with status %s", finalStatus)
    }

    // ===== ASSERTIONS =====
    tags := []string{"rating-bfs", "gnp"}
    metadata, content := AssertOutputNotEmpty(t, helper, tags)

    // Schema validation
    ValidateSchema(t, metadata, BFSSchema)

    // Business rules
    if score, ok := metadata["score"].(float64); ok {
        AssertGateScore(t, score, "BFS score")
    }

    SaveWorkerOutput(t, env, helper, tags, "GNP")
}
```

#### 6.4.2 rating_cds_test.go
- [ ] Create `test/api/market_workers/rating_cds_test.go`
- Config: `ticker`, requires fundamentals + announcements
- Tags: `["rating-cds", "{ticker}"]`
- Validates: `AssertGateScore` (0, 1, 2)

#### 6.4.3 rating_nfr_test.go
- [ ] Create `test/api/market_workers/rating_nfr_test.go`
- Config: `ticker`, requires announcements + prices
- Tags: `["rating-nfr", "{ticker}"]`
- Validates: `AssertComponentScore` (0.0-1.0)

#### 6.4.4 rating_pps_test.go
- [ ] Create `test/api/market_workers/rating_pps_test.go`
- Config: `ticker`, requires announcements + prices
- Tags: `["rating-pps", "{ticker}"]`
- Validates: `AssertComponentScore` (0.0-1.0)

#### 6.4.5 rating_vrs_test.go
- [ ] Create `test/api/market_workers/rating_vrs_test.go`
- Config: `ticker`, requires announcements + prices
- Tags: `["rating-vrs", "{ticker}"]`
- Validates: `AssertComponentScore` (0.0-1.0)

#### 6.4.6 rating_ob_test.go
- [ ] Create `test/api/market_workers/rating_ob_test.go`
- Config: `ticker`, requires announcements + BFS score
- Tags: `["rating-ob", "{ticker}"]`
- Validates: `AssertOBScore` (0.0, 0.5, 1.0)

#### 6.4.7 rating_composite_test.go
- [ ] Create `test/api/market_workers/rating_composite_test.go`
- Config: `ticker`, requires all score documents
- Tags: `["stock-rating", "{ticker}"]`
- Validates: `AssertRatingLabel`, `AssertInvestabilityScore`

---

### 6.5 Integration Test

```go
// test/api/market_workers/rating_integration_test.go
func TestRatingFullFlow(t *testing.T) {
    // Tests complete rating flow for golden tickers
    // Uses job definition that runs all rating steps
}

func TestRatingGoldenCases(t *testing.T) {
    goldenCases := []struct {
        ticker        string
        expectedLabel string
        gatePass      bool
        investMin     float64
        investMax     float64
    }{
        {"ASX:BHP", "LOW_ALPHA", true, 40, 50},
        {"ASX:EXR", "SPECULATIVE", false, 0, 0},  // Gate fails
        {"ASX:CSL", "LOW_ALPHA", true, 40, 50},
        {"ASX:GNP", "INVESTABLE", true, 65, 75},
        {"ASX:SKS", "INVESTABLE", true, 65, 75},
    }

    for _, tc := range goldenCases {
        t.Run(tc.ticker, func(t *testing.T) {
            // Execute full rating flow
            // Assert label matches expected
            // Assert investability in expected range
        })
    }
}

func TestRatingMultiTicker(t *testing.T) {
    // Tests concurrent execution with multiple tickers
    // Validates all outputs created
    // Checks no cross-contamination between tickers
}
```

---

### 6.6 Service Unit Tests

Pure function tests in `internal/services/rating/`:

| File | Tests |
|------|-------|
| `bfs_test.go` | `CalculateBFS` with various Fundamentals inputs |
| `cds_test.go` | `CalculateCDS` with various dilution scenarios |
| `nfr_test.go` | `CalculateNFR` with fact vs narrative counts |
| `pps_test.go` | `CalculatePPS` with price reaction scenarios |
| `vrs_test.go` | `CalculateVRS` with volatility patterns |
| `ob_test.go` | `CalculateOB` with catalyst detection |
| `composite_test.go` | `CalculateRating` gate pass/fail scenarios |
| `math_test.go` | `PriceWindow`, `DailyReturns`, `Stddev`, `CAGR` |

**Test patterns:**
- Table-driven tests with named cases
- Edge cases: nil inputs, zero values, boundary conditions
- No mocks - pure function testing

---

### 6.7 Test Coverage Matrix

| Worker | API Test | Unit Test | Golden Cases | Tags |
|--------|----------|-----------|--------------|------|
| `rating_bfs` | `rating_bfs_test.go` | `bfs_test.go` | BHP, GNP | `rating-bfs` |
| `rating_cds` | `rating_cds_test.go` | `cds_test.go` | EXR (fail), GNP | `rating-cds` |
| `rating_nfr` | `rating_nfr_test.go` | `nfr_test.go` | All golden | `rating-nfr` |
| `rating_pps` | `rating_pps_test.go` | `pps_test.go` | All golden | `rating-pps` |
| `rating_vrs` | `rating_vrs_test.go` | `vrs_test.go` | All golden | `rating-vrs` |
| `rating_ob` | `rating_ob_test.go` | `ob_test.go` | All golden | `rating-ob` |
| `rating_composite` | `rating_composite_test.go` | `composite_test.go` | All golden | `stock-rating` |
| Integration | `rating_integration_test.go` | - | All golden | - |

**Coverage targets:**
- Service unit tests: >90%
- API tests: All workers covered
- Golden cases: All 5 tickers pass expected outcomes

---

### 6.8 Cleanup Verification

After test updates, verify:
- [ ] `go test ./test/api/market_workers/...` passes
- [ ] No import errors from deleted signal tests
- [ ] All rating tests pass with golden tickers
- [ ] Test results directory structure matches existing pattern

---

### Deliverables

- [ ] `signal_test.go` deleted
- [ ] `signal_computer_test.go` deleted
- [ ] 7 WorkerSchema definitions in `common_test.go`
- [ ] 6 business rule validators in `common_test.go`
- [ ] 7 rating worker API tests
- [ ] 1 integration test with golden cases
- [ ] 8 service unit test files
- [ ] All golden test cases pass

---

## Implementation Order

```
Stage 0: Market Worker Separation ─── PREREQUISITE: Separate data collection from processing
          ↓
Stage 1: Rating Service           ─── Pure calculation functions
          ↓
Stage 2: Rating Workers           ─── Document I/O, call services
          ↓
Stage 3: Job Definition           ─── Wire workers together
          ↓
Stage 4: Output & Reporting       ─── Generate reports
          ↓
Stage 5: Cleanup                  ─── Delete old code, move MQS
          ↓
Stage 6: Verification             ─── Golden tests, benchmarks
```

---

## Success Criteria

- [ ] Market workers are pure data collection (no processing logic)
- [ ] Processing workers handle analysis/classification
- [ ] All golden test cases pass
- [ ] Full rating flow <30s per ticker
- [ ] Clean separation: services (pure) / workers (I/O)
- [ ] No dead code remaining
- [ ] Clean build with no warnings

---

*Document Version: 3.4*
*Updated: 2026-01-08*
