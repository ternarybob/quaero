# ARCHITECT Analysis: Enhance Stock Data Document Output

## Task Requirements

1. **Include source of stock data** in markdown
2. **Daily data as table/CSV** for LLM consumption
3. **Period performance table** (like screenshot: 1W, 1M, 3M, 6M, 1Y, 2Y changes)
4. **Volume data and analysis**

## Current Implementation

**File**: `internal/queue/workers/asx_stock_data_worker.go`
**Function**: `createDocument()` (line 651-773)

Current document structure:
- Current Price (table)
- Valuation (table)
- Technical Analysis (moving averages, RSI, support/resistance)
- Position in 52-week range

**Missing**:
- Data sources attribution
- Historical daily OHLCV data
- Period performance table
- Volume analysis

## Data Sources Available

1. **Markit Digital API** (lines 322-395)
   - `header` endpoint: Current price, bid/ask, volume, market cap
   - `key-statistics` endpoint: P/E, EPS, dividend yield, 52-week range

2. **Yahoo Finance** (lines 398-479)
   - Historical OHLCV data stored in `data.HistoricalPrices`
   - Has Date, Open, High, Low, Close, Volume for each day

## Recommendation: MODIFY (Not Create)

Enhance `createDocument()` function to add:

1. **Data Sources section** - List APIs used
2. **Period Performance table** - Calculate changes for 1W, 1M, 3M, 6M, 1Y
3. **Volume Analysis section** - Current vs avg, trend
4. **Historical Data CSV** - Last 30 days OHLCV in table format

## Files to Modify

1. `internal/queue/workers/asx_stock_data_worker.go`
   - Modify `createDocument()` function only
   - Add helper functions for period calculations

## Anti-Creation Compliance

- No new files ✓
- No new workers ✓
- Extends existing function ✓
- Uses existing `HistoricalPrices` data ✓

## Build Verification Required

After modification, run: `./scripts/build.sh`
