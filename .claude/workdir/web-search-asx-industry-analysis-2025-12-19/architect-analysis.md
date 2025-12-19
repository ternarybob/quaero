# ARCHITECT ANALYSIS: ASX Web Search Industry & Competitor Analysis

**Date:** 2025-12-19
**Task:** Enhance web-search-asx.toml with industry outlook and competitor analysis

## 1. EXISTING CODE ANALYSIS

### Current Job Definitions

**web-search-asx.toml (GNP)** - 5 steps:
1. `fetch_stock_data` - asx_stock_data worker
2. `fetch_announcements` - asx_announcements worker
3. `search_asx_gnp` - web_search worker
4. `summarize_results` - summary worker (basic analysis)
5. `email_summary` - email worker

**web-search-asx-cba.toml** - 6 steps (MORE COMPLETE):
1. `fetch_stock_data` - asx_stock_data worker
2. `fetch_announcements` - asx_announcements worker
3. `search_asx_cba` - web_search worker
4. `analyze_announcements` - **summary worker for NOISE VS SIGNAL** ← Missing in GNP
5. `summarize_results` - summary worker (comprehensive with noise/signal)
6. `email_summary` - email worker

**web-search-asx-exr.toml** - Same as CBA (6 steps with noise vs signal)

### Key Observations

1. **GNP is missing the `analyze_announcements` step** that CBA and EXR have
2. **No industry/sector analysis step** exists in any file
3. **No competitor comparison step** exists in any file
4. **Web search references ARE being stored** in the document metadata (`results.Sources`)

### Existing Worker Types Available

From `internal/models/worker_type.go`:
- `web_search` - Can execute multiple searches with depth/breadth
- `summary` - Can analyze documents with custom prompts using AI
- `agent` - Can process documents with various agent types

### How Web Search References Are Already Stored

From `web_search_worker.go:469-574`:
- Sources are extracted from Gemini's grounding metadata
- Sources include URL and Title
- They are written into the markdown content under `## Sources` section
- They are stored in document metadata as `source_count`
- **The sources ARE being stored** - they're in the markdown content

## 2. RECOMMENDATION: EXTEND, NOT CREATE

Following the anti-creation bias principle, we should:

### A. EXTEND web-search-asx.toml (GNP) with new steps:

1. **Add noise vs signal step** (copy from CBA/EXR pattern)
2. **Add industry outlook step** (new web_search step)
3. **Add competitor analysis step** (new web_search step)
4. **Enhance final summary** to include all analysis

### B. ALIGN other TOML files

Copy the new industry/competitor steps to CBA and EXR files.

### C. Web Search References

**ALREADY IMPLEMENTED** - The `WebSearchWorker` already stores references:
- In markdown content under `## Sources`
- Each source has URL and Title
- Metadata includes `source_count`

No code changes needed for storing references - it's already working.

## 3. IMPLEMENTATION PLAN

### Step 1: Update web-search-asx.toml

Add steps in this order:
1. `fetch_stock_data` (existing)
2. `fetch_announcements` (existing)
3. `search_asx_gnp` (existing - company news)
4. **NEW: `search_industry`** - web_search for industry outlook
5. **NEW: `search_competitors`** - web_search for similar companies
6. **NEW: `analyze_announcements`** - summary for noise vs signal (from CBA)
7. `summarize_results` (update prompt to include industry/competitors)
8. `email_summary` (existing)

### Step 2: Fix bugs in existing files

**BUG FOUND in web-search-asx-cba.toml line 42:**
```toml
query = "ASX:CBA GenusPlus financial results..."
```
This says "GenusPlus" but should be for CBA (Commonwealth Bank).

**BUG FOUND in web-search-asx-exr.toml line 42:**
```toml
query = "ASX:EXR GenusPlus financial results..."
```
This says "GenusPlus" but should be for EXR (Elixir Energy).

### Step 3: Update other TOML files

Add the same industry/competitor steps to CBA and EXR files.

## 4. NO NEW GO CODE REQUIRED

The existing workers support everything needed:
- `web_search` worker can perform industry and competitor searches
- `summary` worker with custom prompts can do critical analysis
- References are already stored in documents

This is EXTEND only - no CREATE.

## 5. PROMPTS FOR CRITICAL THINKING

### Industry Outlook Prompt
The AI will be instructed to:
- Search for industry trends and outlook
- Identify macroeconomic factors
- Look for regulatory changes
- Find expert forecasts

### Competitor Analysis Prompt
The AI will be instructed to:
- Identify similar listed and unlisted companies
- Compare financial metrics
- Analyze competitive positioning
- Look for market share data

### Noise vs Signal Analysis
Use existing CBA/EXR pattern which:
- Correlates announcements with price movements
- Identifies PR fluff vs material information
- Assigns credibility scores
- Flags promotional patterns

## 6. FILES TO MODIFY

1. `bin/job-definitions/web-search-asx.toml` - Major update
2. `bin/job-definitions/web-search-asx-cba.toml` - Fix bug + add industry/competitor
3. `bin/job-definitions/web-search-asx-exr.toml` - Fix bug + add industry/competitor

## 7. DECISION

**EXTEND existing TOML job definitions** - No new Go code required.

All functionality can be achieved by:
1. Adding new `web_search` steps with appropriate queries
2. Adding `summary` steps with critical thinking prompts
3. Copying proven patterns from CBA/EXR to GNP

The workers already support:
- Multiple sequential searches (depth > 1)
- Custom AI prompts for analysis
- Storing sources in documents
