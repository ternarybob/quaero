# ARCHITECT Analysis: Enhance ASX:EXR Job Definition

## Task Requirements

1. **Update fetch_stock_data step**: Collect daily data for past year and store as document
   - **ALREADY DONE**: The `asx_stock_data` worker already fetches 1 year of historical data (`period = "Y1"`) and stores a document with technical analysis

2. **Add summary step**: Analyze announcements vs share price relevance
   - **MODIFY**: The existing `summarize_results` step needs an enhanced prompt to correlate announcements with price movements

3. **Enhanced summary**: Include stock data + technical assessment + noise vs signal analysis
   - **MODIFY**: Update the existing summary prompt to include critical analysis

## Analysis of Existing Code

### asx_stock_data Worker (✓ Already captures daily data)
- Location: `internal/queue/workers/asx_stock_data_worker.go`
- Fetches: Historical OHLCV data from Yahoo Finance for 1 year
- Stores: Document with technicals (SMA, RSI, support/resistance)
- **Historical data IS stored** in the document (see `data.HistoricalPrices`)

### asx_announcements Worker (✓ Already captures announcements)
- Location: `internal/queue/workers/asx_announcements_worker.go`
- Fetches: ASX announcements with dates, headlines, price sensitivity flags
- **Already has date info** for correlation

### Current Job Definition Structure
```
Step 1: fetch_stock_data → output_tags: ["exr", "asx-exr-data"]
Step 2: fetch_announcements → output_tags: ["exr", "asx-exr-announcements"]
Step 3: search_asx_exr → output_tags: ["exr", "asx-exr-search"]
Step 4: summarize_results → filter_tags: ["exr"]
Step 5: email_summary
```

## Recommendation: MODIFY (Not Create)

**No new code needed.** The task is achieved by enhancing the TOML job definition only:

1. The `asx_stock_data` step already stores daily historical data
2. The `summarize_results` step's prompt needs enhancement to:
   - Correlate announcement dates with price movements
   - Identify noise vs signal
   - Provide critical analysis of PR vs actual performance

### Key Insight
The current prompt asks for generic investment analysis. The enhanced prompt should:
- Explicitly request correlation of announcement dates with price data
- Ask for noise vs signal identification
- Request critical analysis of company PR claims

## Files to Modify

1. `bin/job-definitions/web-search-asx-exr.toml` - Enhance the summary prompt only

## Anti-Creation Compliance

- No new files created ✓
- No new workers needed ✓
- No Go code changes ✓
- Only TOML configuration change ✓

## Build Verification

Since this is a TOML file change, no build is required. However, we can validate:
- `./scripts/build.sh` - Ensure no syntax errors
