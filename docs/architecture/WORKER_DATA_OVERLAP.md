# Worker Data Overlap Analysis

This document analyzes the data collection capabilities of workers and their usage across job templates.

## Workers Overview

### ASX Data Workers

| Worker | Type | Data Source | Document Tags | Data Provided |
|--------|------|-------------|---------------|---------------|
| `asx_stock_collector` | Structured | Yahoo Finance | `asx-stock-data`, `<ticker>` | **Consolidated**: Price, OHLCV, technicals, analyst coverage, historical financials (single API call) |
| `asx_announcements` | Structured | ASX API | `asx-announcement`, `<ticker>` | Official announcements, price sensitivity flags, PDF links |
| `asx_director_interest` | Structured | ASX API | `director-interest`, `<ticker>` | Appendix 3Y filings, insider buying/selling |

### Deprecated Workers (Use asx_stock_collector instead)

| Worker | Status | Replacement |
|--------|--------|-------------|
| `asx_stock_data` | DEPRECATED | `asx_stock_collector` |
| `asx_analyst_coverage` | DEPRECATED | `asx_stock_collector` |
| `asx_historical_financials` | DEPRECATED | `asx_stock_collector` |

> **Migration Note**: These workers are kept for backward compatibility. New integrations should use `asx_stock_collector` which combines all data into a single API call.

### General Workers

| Worker | Type | Data Source | Document Tags | Data Provided |
|--------|------|-------------|---------------|---------------|
| `web_search` | Unstructured | Web/Google AI | `web-search` | News, articles, broker notes, research |
| `summary` | Analysis | LLM | Configurable | Deep analysis of collected documents |
| `macro_data` | Structured | Various APIs | `macro-data`, `<data_type>` | RBA interest rates, commodity prices |

## Consolidated Data Architecture

### asx_stock_collector Output

The `asx_stock_collector` worker fetches all Yahoo Finance data in a single API call:

```
fetch_stock_data (asx_stock_collector)
├── Price Data
│   ├── Current price, change, change %
│   ├── Day range, 52-week range
│   ├── Volume, average volume
│   └── Market cap
├── Valuation
│   ├── P/E ratio
│   ├── EPS
│   └── Dividend yield
├── Technical Indicators
│   ├── SMA 20/50/200
│   ├── RSI 14
│   ├── Support/resistance
│   └── Trend signal (BULLISH/BEARISH/NEUTRAL)
├── Period Performance
│   ├── 7D, 1M, 3M, 6M, 1Y, 2Y returns
│   └── Price at each period start
├── Analyst Coverage
│   ├── Analyst count
│   ├── Consensus rating (buy/hold/sell)
│   ├── Price targets (mean/high/low/median)
│   ├── Upside potential
│   ├── Recommendation distribution
│   └── Recent upgrades/downgrades (last 10)
├── Historical Financials
│   ├── Annual data (4 years): revenue, profit, margins, EBITDA, cash flow
│   ├── Quarterly data (4 quarters): revenue, profit, margins
│   └── Growth metrics: YoY growth, 3Y/5Y CAGR
└── Historical Prices
    └── Full OHLCV array for charting
```

## Template Tool Availability Matrix

| Tool | asx-stock-analysis | asx-purchase-conviction | smsf-portfolio-analysis |
|------|-------------------|------------------------|-------------------------|
| `fetch_stock_data` (asx_stock_collector) | ✓ | ✓ | ✓ |
| `fetch_announcements` | ✓ | ✓ | ✓ |
| `fetch_index_data` (asx_stock_data) | ✓ | ✓ | ✓ |
| `fetch_director_interest` | ✗ | ✓ | ✗ |
| `fetch_macro_data` | ✗ | ✓ | ✗ |
| `search_web` | ✓ | ✓ | ✓ |
| `analyze_summary` | ✓ | ✓ | ✓ |

> **Note**: `fetch_stock_data` now uses `asx_stock_collector` which includes analyst coverage and historical financials. Separate `fetch_analyst_coverage` and `fetch_historical_financials` tools are no longer needed.

## Data Collection Flow

### Before (3 API calls per stock)

```
┌─────────────────┐     ┌─────────────────────┐     ┌────────────────────────┐
│ asx_stock_data  │     │ asx_analyst_coverage│     │ asx_historical_financials│
│ (Yahoo Finance) │     │ (Yahoo Finance)     │     │ (Yahoo Finance)        │
└────────┬────────┘     └──────────┬──────────┘     └───────────┬────────────┘
         │                         │                             │
         ▼                         ▼                             ▼
   asx-stock-data           asx-analyst-coverage         asx-historical-financials
```

### After (1 API call per stock)

```
┌───────────────────────────────────────────────────────────────┐
│                     asx_stock_collector                        │
│               (Single Yahoo Finance API call)                  │
│  Modules: price, summaryDetail, financialData,                │
│           recommendationTrend, incomeStatementHistory, etc.    │
└────────────────────────────┬──────────────────────────────────┘
                             │
                             ▼
                      asx-stock-data
                 (Contains ALL data types)
```

## Data Overlap Analysis

### Shared Data (All Templates)

The following data is collected by all three templates via `asx_stock_collector`:

1. **Stock Price & Technicals**
   - Current price, historical OHLCV
   - Technical indicators (SMA, RSI)
   - Support/resistance levels
   - Trend signal

2. **Analyst Coverage**
   - Broker ratings and consensus
   - Price targets (mean/high/low/median)
   - Upgrade/downgrade history

3. **Historical Financials**
   - Revenue and profit history
   - Margins and cash flow
   - Growth metrics (YoY, 3Y/5Y CAGR)

4. **Company Announcements** (`asx_announcements`)
   - Official ASX announcements
   - Price sensitivity flags

5. **Benchmark Comparison** (`asx_stock_data` as index)
   - ASX 200 (XJO) performance
   - Small Ords (XSO) for small caps

### Template-Specific Data

#### asx-stock-analysis-goal
- **Focus**: Daily stock recommendations
- **Unique Analysis**: 5-year company performance, signal-to-noise ratio, price event analysis
- **Output**: Trader and Super recommendations with quality grades

#### asx-purchase-conviction-goal
- **Focus**: High-conviction purchase decisions
- **Unique Data**:
  - `fetch_director_interest`: Insider buying/selling patterns
  - `fetch_macro_data`: RBA rates, commodity prices
- **Unique Analysis**: Adversarial "Short Seller" review, conviction scoring matrix

#### smsf-portfolio-analysis-goal
- **Focus**: Portfolio management for SMSFs
- **Unique Analysis**: P/L calculations, portfolio mix, rebalancing recommendations
- **Unique Context**: Uses holdings with units and purchase prices

## Data Collection Preferences

| Data Need | Preferred Worker | Notes |
|-----------|-----------------|-------|
| Stock price, technicals, analyst ratings, financials | `asx_stock_collector` | Single call covers all Yahoo data |
| Official ASX announcements | `asx_announcements` | ASX API - official source |
| Insider activity | `asx_director_interest` | ASX Appendix 3Y filings |
| Current news | `search_web` | For sentiment and breaking news |
| Broker notes (qualitative) | `search_web` | Detailed analysis not in structured data |
| Macroeconomic data | `macro_data` | RBA rates, commodity prices |

## Benefits of Consolidated Architecture

1. **Reduced API Calls**: 3x reduction in Yahoo Finance API usage
2. **Data Consistency**: All data fetched at same timestamp
3. **Simpler Templates**: One tool covers most stock data needs
4. **Type Safety**: In-code Go structs instead of external JSON schemas
5. **No AI Processing**: Pure data collection - no LLM summarization in worker

## Migration Guide

### Updating Job Templates

**Before (legacy)**:
```toml
available_tools = [
    { name = "fetch_stock_data", worker = "asx_stock_data" },
    { name = "fetch_analyst_coverage", worker = "asx_analyst_coverage" },
    { name = "fetch_historical_financials", worker = "asx_historical_financials" },
]
```

**After (recommended)**:
```toml
available_tools = [
    { name = "fetch_stock_data", worker = "asx_stock_collector" },
    # analyst_coverage and historical_financials are included in asx_stock_collector
]
```

### Updating Goal Text

**Before**:
```
1. Use fetch_stock_data for price data
2. Use fetch_analyst_coverage for broker ratings
3. Use fetch_historical_financials for revenue history
```

**After**:
```
1. Use fetch_stock_data (single call includes price, analyst, and financial data)
```
