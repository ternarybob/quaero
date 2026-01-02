# ASX Stock Workers Process Flow

This document traces the data flow and processing for each ASX stock-related worker, identifying API calls, LLM usage, and providing conclusions on efficiency improvements.

## Table of Contents

- [Overview](#overview)
- [Worker Process Flows](#worker-process-flows)
  - [ASX Stock Collector Worker](#asx-stock-collector-worker)
  - [ASX Announcements Worker](#asx-announcements-worker)
  - [ASX Index Data Worker](#asx-index-data-worker)
  - [ASX Director Interest Worker](#asx-director-interest-worker)
- [Summary Worker (LLM Markdown Generation)](#summary-worker-llm-markdown-generation)
- [Conclusions and Recommendations](#conclusions-and-recommendations)

---

## Overview

The ASX stock workers are data collection workers that fetch stock-related data from external APIs and store them as documents. **None of these workers use LLM for data processing** - they perform pure data collection and computation. The LLM is only used downstream by the `summary_worker` for markdown generation and analysis.

### Worker Data Flow Summary

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          ASX DATA COLLECTION LAYER                          │
│                         (No LLM - Pure Data Fetch)                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────┐     ┌─────────────────────┐                       │
│  │ asx_stock_collector │     │  asx_announcements  │                       │
│  │     (EODHD API)     │     │ (ASX HTML + Yahoo)  │                       │
│  │                     │     │                     │                       │
│  │ • Fundamentals      │     │ • Announcements     │                       │
│  │ • Historical EOD    │     │ • Price Impact      │                       │
│  │ • Technicals        │     │ • Relevance Class   │                       │
│  └──────────┬──────────┘     └──────────┬──────────┘                       │
│             │                           │                                   │
│             ▼                           ▼                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      DOCUMENT STORAGE                                │   │
│  │   • asx_stock_collector documents (comprehensive stock data)        │   │
│  │   • asx_announcement documents (individual announcements)           │   │
│  │   • asx_announcement_summary documents (summary with analysis)      │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          LLM PROCESSING LAYER                               │
│                        (Gemini/Claude via LLM Service)                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────┐     ┌─────────────────────┐                       │
│  │   summary_worker    │     │   ai_assessor       │                       │
│  │   (LLM Analysis)    │     │ (LLM Recommendations)│                      │
│  │                     │     │                     │                       │
│  │ • Read documents    │     │ • Read signals      │                       │
│  │ • Generate summary  │     │ • Generate recs     │                       │
│  │ • Schema-constrained│     │ • Validate output   │                       │
│  └─────────────────────┘     └─────────────────────┘                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Worker Process Flows

### ASX Stock Collector Worker

**File**: `internal/queue/workers/asx_stock_collector_worker.go`

**Purpose**: Consolidated data collector fetching comprehensive stock data from EODHD API in a single workflow.

#### Process Flow

```
┌────────────────────────────────────────────────────────────────────────────┐
│                    ASX STOCK COLLECTOR PROCESS FLOW                        │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. INIT PHASE                                                             │
│     ├── Parse ticker(s) from config (supports single or array)            │
│     ├── Set period (default: Y2 = 24 months)                              │
│     └── Create WorkItems for each ticker                                   │
│                                                                            │
│  2. CACHE CHECK                                                            │
│     ├── Query documentStorage.GetDocumentBySource()                        │
│     ├── Check if LastSynced within cache_hours (default: 24h)             │
│     └── If fresh → Return cached document, skip API calls                 │
│                                                                            │
│  3. API FETCH PHASE (if cache miss)                                        │
│     │                                                                      │
│     ├── STEP 1: Fetch Fundamentals (EODHD API)                            │
│     │   URL: https://eodhd.com/api/fundamentals/{symbol}.AU              │
│     │   └── Returns: General, Highlights, Valuation, SharesStats,        │
│     │                Technicals, SplitsDividends, AnalystRatings,         │
│     │                Holders, ESGScores, Earnings, Financials             │
│     │                                                                      │
│     ├── STEP 2: Fetch Historical Prices (EODHD API)                       │
│     │   URL: https://eodhd.com/api/eod/{symbol}.AU?from=X&to=Y           │
│     │   └── Returns: Array of OHLCV data for period                       │
│     │                                                                      │
│     ├── STEP 3: Calculate Technicals (LOCAL COMPUTATION)                  │
│     │   └── SMA20, SMA50, SMA200, RSI14, Support/Resistance, TrendSignal │
│     │                                                                      │
│     └── STEP 4: Calculate Period Performance (LOCAL COMPUTATION)          │
│         └── 7D, 1M, 3M, 6M, 1Y, 2Y price changes                          │
│                                                                            │
│  4. DOCUMENT CREATION (NO LLM)                                             │
│     ├── Build markdown content with formatted tables                       │
│     ├── Build structured metadata (all numeric values)                    │
│     ├── Build tags (asx-stock-data, ticker, date)                         │
│     └── Save to documentStorage                                            │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

#### API Calls

| Endpoint | Provider | Purpose | Rate Limit |
|----------|----------|---------|------------|
| `/api/fundamentals/{symbol}.AU` | EODHD | Company info, valuation, analyst ratings, ESG, financials | API key required |
| `/api/eod/{symbol}.AU` | EODHD | Historical OHLCV price data | API key required |

#### LLM Usage

**NONE** - This worker performs pure data collection and mathematical calculations.

#### Output Document

```
SourceType: "asx_stock_collector"
SourceID: "asx:{ticker}:stock_collector"
Tags: ["asx-stock-data", "{ticker}", "asx:{ticker}", "date:YYYY-MM-DD"]
```

---

### ASX Announcements Worker

**File**: `internal/queue/workers/asx_announcements_worker.go`

**Purpose**: Fetches ASX company announcements, classifies relevance, and calculates price impact.

#### Process Flow

```
┌────────────────────────────────────────────────────────────────────────────┐
│                   ASX ANNOUNCEMENTS PROCESS FLOW                           │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. INIT PHASE                                                             │
│     ├── Parse ASX code from config                                         │
│     ├── Set period (default: Y1) and limit (default: 50)                  │
│     └── Create single WorkItem for the ticker                              │
│                                                                            │
│  2. FETCH ANNOUNCEMENTS                                                    │
│     │                                                                      │
│     ├── PRIMARY: ASX HTML Page (for Y1+ periods)                          │
│     │   URL: https://www.asx.com.au/asx/v2/statistics/announcements.do   │
│     │        ?by=asxCode&asxCode={CODE}&timeframe=Y&year={YEAR}          │
│     │   └── Parse HTML table rows via regex                               │
│     │                                                                      │
│     └── FALLBACK: Markit Digital API                                      │
│         URL: https://asx.api.markitdigital.com/asx-research/1.0/         │
│              companies/{code}/announcements                               │
│         └── Parse JSON response                                            │
│                                                                            │
│  3. FETCH PRICE DATA (for impact analysis)                                 │
│     │                                                                      │
│     ├── PRIMARY: Check for existing asx_stock_collector document          │
│     │   └── Use historical_prices from document metadata                  │
│     │                                                                      │
│     └── FALLBACK: Yahoo Finance API                                       │
│         URL: https://query1.finance.yahoo.com/v8/finance/chart/           │
│              {symbol}.AX?interval=1d&range={range}                        │
│                                                                            │
│  4. ANALYZE ANNOUNCEMENTS (LOCAL COMPUTATION - NO LLM)                    │
│     │                                                                      │
│     ├── classifyRelevance(): Keyword-based categorization                 │
│     │   └── HIGH: Price-sensitive, takeover, dividend, earnings           │
│     │   └── MEDIUM: Director changes, contracts, AGM                      │
│     │   └── LOW: Progress reports, disclosures                            │
│     │   └── NOISE: No material indicators                                 │
│     │                                                                      │
│     ├── calculatePriceImpact(): Price change around announcement date    │
│     │   └── Compare close prices before/after announcement                │
│     │   └── Calculate volume change ratio                                 │
│     │                                                                      │
│     └── calculateSignalNoiseRating(): Market impact assessment            │
│         └── HIGH_SIGNAL: Price >=3% OR volume >=2x                        │
│         └── MODERATE_SIGNAL: Price >=1.5% OR volume >=1.5x                │
│         └── LOW_SIGNAL: Price >=0.5% OR volume >=1.2x                     │
│         └── NOISE: Below thresholds                                       │
│                                                                            │
│  5. DOCUMENT CREATION (NO LLM)                                             │
│     │                                                                      │
│     ├── Create individual documents for HIGH/MEDIUM relevance only        │
│     │                                                                      │
│     └── Create summary document with ALL announcements                    │
│         └── Signal-to-noise analysis                                       │
│         └── Relevance distribution                                         │
│         └── Announcements table with price impact                         │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

#### API Calls

| Endpoint | Provider | Purpose | Rate Limit |
|----------|----------|---------|------------|
| ASX HTML Page | ASX | Fetch announcements (primary for Y1+) | None (public) |
| Markit Digital API | ASX/Markit | Fetch announcements (fallback) | None (public) |
| Yahoo Finance Chart API | Yahoo | Historical prices (fallback) | None (public) |

#### LLM Usage

**NONE** - Relevance classification uses keyword matching. Signal-to-noise rating uses numeric thresholds.

#### Output Documents

```
Individual announcements:
  SourceType: "asx_announcement"
  SourceID: "{pdf_url}"
  Tags: ["asx-announcement", "{ticker}", "date:YYYY-MM-DD", "price-sensitive"?]

Summary document:
  SourceType: "asx_announcement_summary"
  SourceID: "asx:{ticker}:announcement_summary"
  Tags: ["asx-announcement-summary", "{ticker}", "date:YYYY-MM-DD"]
```

---

### ASX Index Data Worker

**File**: `internal/queue/workers/asx_index_data_worker.go`

**Purpose**: Fetches real-time and historical data for ASX indices (XJO, XSO) used as benchmarks.

#### Process Flow

```
┌────────────────────────────────────────────────────────────────────────────┐
│                    ASX INDEX DATA PROCESS FLOW                             │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. INIT PHASE                                                             │
│     ├── Parse index code from config (XJO, XSO, etc.)                     │
│     ├── Set period (default: Y2)                                          │
│     └── Create WorkItem for the index                                      │
│                                                                            │
│  2. CACHE CHECK                                                            │
│     └── Same logic as asx_stock_collector (24h default)                   │
│                                                                            │
│  3. API FETCH (if cache miss)                                              │
│     │                                                                      │
│     └── Yahoo Finance API                                                  │
│         URL: https://query1.finance.yahoo.com/v8/finance/chart/           │
│              ^{code}.AX?interval=1d&range={period}                        │
│         └── Returns: OHLCV array for index                                │
│                                                                            │
│  4. CALCULATE TECHNICALS (LOCAL COMPUTATION)                               │
│     └── SMA20, SMA50, SMA200, period performance                          │
│                                                                            │
│  5. DOCUMENT CREATION (NO LLM)                                             │
│     └── Build markdown and metadata                                        │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

#### API Calls

| Endpoint | Provider | Purpose | Rate Limit |
|----------|----------|---------|------------|
| Yahoo Finance Chart API | Yahoo | Historical index data | None (public) |

#### LLM Usage

**NONE**

---

### ASX Director Interest Worker

**File**: `internal/queue/workers/asx_director_interest_worker.go`

**Purpose**: Fetches director interest notices (Form 604) from ASX.

#### Process Flow

```
┌────────────────────────────────────────────────────────────────────────────┐
│                  ASX DIRECTOR INTEREST PROCESS FLOW                        │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. INIT PHASE                                                             │
│     └── Parse ASX code and period from config                              │
│                                                                            │
│  2. FETCH DIRECTOR NOTICES                                                 │
│     └── ASX Announcements filtered by type "Form 604"                     │
│                                                                            │
│  3. DOCUMENT CREATION (NO LLM)                                             │
│     └── Build markdown with director transaction details                   │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

#### LLM Usage

**NONE**

---

## Summary Worker (LLM Markdown Generation)

**File**: `internal/queue/workers/summary_worker.go`

**Purpose**: The only worker that uses LLM for content generation. Generates summaries from collected documents.

#### Process Flow

```
┌────────────────────────────────────────────────────────────────────────────┐
│                      SUMMARY WORKER PROCESS FLOW                           │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  1. INIT PHASE                                                             │
│     ├── Parse filter_tags from config                                      │
│     ├── Parse prompt (required)                                            │
│     ├── Parse output_schema or schema_ref (optional)                       │
│     └── Parse thinking_level (MINIMAL/LOW/MEDIUM/HIGH)                     │
│                                                                            │
│  2. DOCUMENT COLLECTION                                                    │
│     └── documentStorage.ListDocuments() with filter_tags                   │
│                                                                            │
│  3. CONTENT AGGREGATION                                                    │
│     └── Combine document markdown into single context                      │
│                                                                            │
│  4. LLM GENERATION  ← ONLY LLM CALL IN DATA PIPELINE                      │
│     │                                                                      │
│     ├── If output_schema provided:                                         │
│     │   └── Set ResponseMIMEType = "application/json"                     │
│     │   └── Set ResponseSchema from schema                                │
│     │   └── Generate JSON, convert to markdown via jsonToMarkdown()       │
│     │                                                                      │
│     └── If no schema:                                                      │
│         └── Generate markdown directly                                     │
│                                                                            │
│  5. OUTPUT VALIDATION (optional)                                           │
│     ├── Check required_tickers appear in output                           │
│     ├── Check benchmark_codes not treated as primary                      │
│     └── Regenerate if validation fails (max 3 iterations)                 │
│                                                                            │
│  6. DOCUMENT CREATION                                                      │
│     └── Save summary document with generated content                       │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

#### LLM Providers

| Provider | Models | Configuration |
|----------|--------|---------------|
| Gemini | gemini-3-flash-preview, gemini-3-pro-preview | `llm.default_provider = "gemini"` |
| Claude | claude-sonnet-4-5, claude-opus-4-5, claude-haiku-4-5 | `llm.default_provider = "claude"` |

---

## Conclusions and Recommendations

### Current Architecture Analysis

#### Strengths

1. **Clean Separation**: Data collection workers (no LLM) are separate from analysis workers (LLM). This allows:
   - Caching of raw data without LLM cost
   - Parallel data fetching
   - Reproducible data collection

2. **Efficient Caching**: 24-hour default cache prevents redundant API calls

3. **Single API Provider**: EODHD consolidates stock data (vs multiple Yahoo Finance endpoints)

4. **No LLM in Critical Path**: Stock data accuracy doesn't depend on LLM interpretation

#### Identified Issues

1. **Validation Step Concern** (User's Original Issue):
   The user reported incorrect stock prices and non-existent announcements in the validation step. Based on code analysis:

   - **Stock prices**: The `asx_stock_collector` fetches real data from EODHD API. If prices are wrong, possible causes:
     - EODHD API returning stale/incorrect data
     - Cache returning old data (check `cache_hours` setting)
     - Fundamentals-only subscription (no EOD endpoint access - price calculated from market_cap/shares)

   - **Announcements**: The `asx_announcements` worker fetches from ASX website. If announcements are non-existent:
     - HTML parsing may have failed
     - Markit API may have returned empty response
     - Period filter may be excluding recent announcements

2. **Data Verification Gap**: There's no automated validation that fetched data is current or accurate.

### Recommendations for Efficiency and Effectiveness

#### 1. Add Data Freshness Validation

```go
// Add to asx_stock_collector_worker.go
func (w *ASXStockCollectorWorker) validateDataFreshness(data *StockCollectorData) error {
    // Ensure price is from today or last trading day
    if len(data.HistoricalPrices) > 0 {
        latestDate := data.HistoricalPrices[len(data.HistoricalPrices)-1].Date
        if time.Since(latestDate) > 3*24*time.Hour {
            return fmt.Errorf("stale price data: latest date is %s", latestDate)
        }
    }
    return nil
}
```

#### 2. Add Announcement Count Validation

```go
// Add to asx_announcements_worker.go
func (w *ASXAnnouncementsWorker) validateAnnouncements(announcements []ASXAnnouncement, period string) error {
    expectedMin := map[string]int{
        "Y1": 10,  // Most companies have >10 announcements per year
        "M6": 5,
        "M3": 2,
    }
    if min, ok := expectedMin[period]; ok && len(announcements) < min {
        return fmt.Errorf("unexpectedly low announcement count: %d (expected >=%d for period %s)",
            len(announcements), min, period)
    }
    return nil
}
```

#### 3. Reduce API Calls via Data Sharing

The `asx_announcements` worker already implements this pattern:
- Checks for existing `asx_stock_collector` document for price data
- Only calls Yahoo Finance as fallback

**Recommendation**: Ensure job definitions run `asx_stock_collector` before `asx_announcements` for the same ticker.

```toml
[step.fetch_stock_data]
type = "asx_stock_collector"
asx_code = "GNP"

[step.fetch_announcements]
type = "asx_announcements"
depends = "fetch_stock_data"  # Ensures price data is available
asx_code = "GNP"
```

#### 4. Add Debug Output for Troubleshooting

The workers already support `debugEnabled` flag. Enable it to trace API calls:

```go
debug := NewWorkerDebug("asx_stock_collector", w.debugEnabled)
debug.SetTicker(ticker.String())
debug.StartPhase("api_fetch")
// ... API call ...
debug.EndPhase("api_fetch")
```

#### 5. Consider Real-Time Price Fallback

If EODHD returns stale data, add a real-time price check:

```go
// If latest price is stale, try Yahoo Finance real-time
if isStale(data.CurrentPrice) {
    rtPrice, err := fetchRealTimeFromYahoo(ticker)
    if err == nil {
        data.CurrentPrice = rtPrice
    }
}
```

### Summary

| Worker | API Calls | LLM Usage | Computation |
|--------|-----------|-----------|-------------|
| `asx_stock_collector` | EODHD (2 endpoints) | **None** | SMA, RSI, Trend |
| `asx_announcements` | ASX HTML + Yahoo (fallback) | **None** | Relevance, Price Impact |
| `asx_index_data` | Yahoo Finance | **None** | SMA, Performance |
| `asx_director_interest` | ASX Announcements | **None** | Parsing |
| `summary` | None | **Gemini/Claude** | JSON to Markdown |

The data pipeline is efficient with clear separation. The validation issue likely stems from:
1. Stale cached data
2. API returning incomplete/stale data
3. Missing data freshness validation

Implementing the recommendations above will improve data quality and debugging capability.
