# WORKER Step 1: Enhance ASX:EXR Job Definition

## Changes Made

### 1. Added New Step: `analyze_announcements` (Step 4)

**Purpose**: Correlate announcements with share price movements to identify noise vs signal

**Key Features**:
- Creates announcement-price correlation table
- Classifies each announcement as SIGNAL (>3% price move) or NOISE (<3%)
- Calculates company credibility score (1-10)
- Identifies patterns in promotional behavior
- Tags: `["exr", "asx-exr-announcement-analysis"]`

### 2. Enhanced `summarize_results` (Now Step 5)

**Purpose**: Comprehensive investment analysis incorporating noise vs signal findings

**Key Enhancements**:
- Executive summary with credibility score
- Technical analysis with specific price targets
- Announcement quality assessment
- Signal-to-noise ratio calculation
- Bull/Bear cases based on SIGNAL announcements only
- Final verdict on trustworthiness

### 3. Updated Email Step (Now Step 6)

- Changed subject line to reflect new analysis type
- Dependencies correctly chain to new step

## New Job Definition Structure

```
Step 1: fetch_stock_data     → Historical daily data (1 year)
Step 2: fetch_announcements  → ASX announcements with dates
Step 3: search_asx_exr       → Web search for news
Step 4: analyze_announcements → NEW: Noise vs Signal analysis
Step 5: summarize_results    → ENHANCED: Comprehensive report
Step 6: email_summary        → Email the report
```

## Key Analysis Outputs

The new workflow produces:

1. **Announcement-Price Correlation Table**: Shows which announcements moved the stock
2. **Credibility Score**: 1-10 rating of management communication reliability
3. **Signal-to-Noise Ratio**: Percentage of announcements that actually mattered
4. **Red Flags**: Identified promotional patterns
5. **Technical Assessment**: Price vs SMAs, RSI, support/resistance
6. **Actionable Recommendations**: Entry/exit points based on evidence

## Build Result

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

BUILD PASSED
