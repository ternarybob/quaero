# WORKER Step 1 - ASX:WES Job Definition Created

## Changes Made

### Created: `bin/job-definitions/web-search-asx-wes.toml`

New job definition for ASX:WES (Wesfarmers) with:

### 1. Competitor Stock Data Fetch Steps (NEW)
Added 3 new `asx_stock_data` steps to fetch REAL competitor price data:
- `fetch_competitor_wow` - Woolworths (WOW)
- `fetch_competitor_hvn` - Harvey Norman (HVN)
- `fetch_competitor_jbh` - JB Hi-Fi (JBH)

Each step:
- Fetches real-time prices from ASX Markit API
- Fetches historical OHLCV from Yahoo Finance
- Calculates technical indicators (SMA20/50/200, RSI, Support/Resistance)
- Tags output with `["wes", "asx-wes-competitors", "<code>"]`

### 2. Optimized Summary Prompts

**Signal/Noise Focus**:
- Explicit 2% threshold for large cap price movements
- Credibility scoring system
- Pattern detection for promotional behavior

**Data Accuracy Improvements**:
- Added explicit rules: "Use ONLY the stock data provided"
- "IF YOU DO NOT HAVE DATA, SAY 'UNKNOWN' - never fabricate or estimate"
- Confidence levels added to recommendations
- "DATA UNAVAILABLE" markers throughout template

**Trading Signal Focus**:
- Separate sections for Traders (1-4 weeks) vs Investors (6-12 months)
- Explicit entry/stop/target levels
- Risk/reward ratios
- Confidence indicators

### 3. Competitor Relative Strength Section (NEW)
Added section that uses ACTUAL fetched competitor data:
```
| Stock | Last Price | Change % | SMA20 vs Price | RSI | Trend Signal |
|-------|------------|----------|----------------|-----|--------------|
| WES | $ | % | | | |
| WOW | $ | % | | | |
| HVN | $ | % | | | |
| JBH | $ | % | | | |
```

### 4. Job Configuration
- ID: `web-search-asx-wes`
- Timeout: 20m (extended for competitor fetches)
- Tags: `["web-search", "asx", "stocks", "wes", "retail"]`
- Total steps: 11

### Step Dependency Chain
```
fetch_stock_data
    ├── fetch_competitor_wow
    ├── fetch_competitor_hvn
    └── fetch_competitor_jbh
            │
            v
    fetch_announcements
            │
            v
    search_asx_wes
            │
            v
    search_industry
            │
            v
    search_competitors
            │
            v
    analyze_announcements
            │
            v
    summarize_results
            │
            v
    email_summary
```

## Anti-Creation Compliance
- **EXTENDED** existing CBA pattern (no new Go code)
- **FOLLOWED** exact TOML format from existing jobs
- **REUSED** existing step types (asx_stock_data, web_search, summary, email)

## Build Required
Running build script to verify TOML syntax and job definition loading.
