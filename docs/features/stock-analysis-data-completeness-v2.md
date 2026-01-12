# Feature: Enhanced Stock Analysis Data Completeness v2

## Overview

This document builds on the v1 implementation which achieved 6/6 Kneppy framework pillars with fundamental data (100% completeness from EODHD). This v2 addresses the **remaining data gaps** identified in the analysis output, focusing on forward-looking analyst data and detailed financial disclosures from company reports.

## Background: Current State (v1 Complete)

From `test/results/api/stock_deep_dive_20260112-140804/TestStockDeepDiveWorkflow/output.md`:

| Pillar | v1 Status | Data Source |
|--------|-----------|-------------|
| Capital Efficiency (ROIC) | PASS - 25.18% | EODHD Fundamentals |
| Share Allocation (Dilution) | PASS - 4.02% CAGR | EODHD Balance Sheet |
| Financial Robustness (Net Debt/EBITDA) | PASS - -1.21x | EODHD Balance Sheet |
| Cash Flow Reality (FCF Conversion) | PASS - 304.8% | EODHD Cash Flow |
| Management Alignment (Insider %) | PASS - 58.46% | EODHD Holders |
| Competitive Moat (Margins) | PASS - 7.25% | EODHD Income Statement |

**Data Completeness Score:** 85% (6/6 pillars with fundamental data, but missing forward-looking analyst consensus)

## Improvements Needed (from Analysis Output)

### 1. Data Gaps Identified

| Gap | Impact | Root Cause |
|-----|--------|------------|
| **Analyst Target Mean** | No external valuation benchmark to assess market mispricing | EODHD returns $0.00 for smaller ASX stocks with no Wall Street coverage |
| **ROIC vs WACC** | Unable to confirm exact economic value-add spread | WACC requires risk-free rate, beta adjustments, cost of debt from annual report |
| **Specific Capital Raise Details** | Unclear if dilution was for high-return acquisitions or routine operations | Requires announcement parsing or annual report notes |

### 2. Recommended Additional Sources

| Source | Data Available | Implementation Path |
|--------|----------------|---------------------|
| **Annual Report PDF** | ROIC, WACC, detailed debt facility terms, segment analysis | Download via `announcement_download_worker` + new PDF extraction worker |
| **Investor Presentations** | Project margins, pipeline visibility, strategic guidance | Download via `announcement_download_worker` + PDF extraction worker |
| **Industry Data** | Market share, sector benchmarks | EODHD sector data or external sources |

### 3. Analysis Confidence

- **Current Confidence:** 85% - Forward-looking analyst consensus missing
- **Key Uncertainty:** Sustainability of 300%+ FCF conversion rate; $429m facility impact on future interest expenses
- **Target Confidence:** 95%+ with annual report data integration

## EODHD API Data Availability

### Already Extracted (v1)
- `Highlights.WallStreetTargetPrice` - Analyst target (often $0 for small caps)
- `AnalystRatings.Rating`, `TargetPrice`, `StrongBuy`, `Buy`, `Hold`, `Sell`, `StrongSell`
- `Earnings.Trend` - EPS/Revenue estimates with number of analysts

### Available but Not Yet Used
| EODHD Field | Location | Use Case |
|-------------|----------|----------|
| `EPSEstimateCurrentYear` | Highlights | Forward P/E validation |
| `EPSEstimateNextYear` | Highlights | Growth projections |
| `earningsEstimateAvg/Low/High` | Earnings.Trend | Estimate ranges |
| `revenueEstimateAvg/Low/High` | Earnings.Trend | Revenue forecasts |
| `earningsEstimateNumberOfAnalysts` | Earnings.Trend | Coverage depth |
| `epsTrend7daysAgo`, `epsTrend30daysAgo` | Earnings.Trend | Estimate momentum |
| `epsRevisionsUpLast30Days` | Earnings.Trend | Analyst sentiment shifts |

### Data Limitations for Small-Cap ASX Stocks
- **Problem:** EODHD's analyst data comes from Wall Street sources - smaller ASX stocks typically have zero coverage
- **Example:** GNP (ASX) shows 0 analysts, $0.00 target price
- **Solution:** Supplement with company-disclosed guidance from annual reports

## Implementation Plan v2

### Phase 1: Enhanced EODHD Extraction

**Target File:** `internal/workers/market/fundamentals_worker.go`

Add extraction for analyst estimate trends:

```go
// Add to StockCollectorData
type AnalystEstimates struct {
    EPSCurrentYear      float64 `json:"eps_current_year"`
    EPSNextYear         float64 `json:"eps_next_year"`
    RevenueEstimateAvg  float64 `json:"revenue_estimate_avg"`
    NumberOfAnalysts    int     `json:"number_of_analysts"`
    EPSTrend30Days      float64 `json:"eps_trend_30d"`      // % change in estimates
    RevisionsUp30Days   int     `json:"revisions_up_30d"`
    RevisionsDown30Days int     `json:"revisions_down_30d"`
    CoverageQuality     string  `json:"coverage_quality"`   // "HIGH", "LOW", "NONE"
}

// Extraction from EODHD Earnings.Trend
func (w *FundamentalsWorker) extractAnalystEstimates(fundResp *eodhdFundamentals) *AnalystEstimates {
    est := &AnalystEstimates{}

    // From Highlights
    est.EPSCurrentYear = fundResp.Highlights.EPSEstimateCurrentYear
    est.EPSNextYear = fundResp.Highlights.EPSEstimateNextYear

    // From Earnings Trend (current year period)
    for _, trend := range fundResp.Earnings.Trend {
        if trend.Period == "0y" { // Current year
            est.RevenueEstimateAvg = trend.RevenueEstimateAvg
            est.NumberOfAnalysts = trend.EarningsEstimateNumberOfAnalysts
            est.EPSTrend30Days = trend.EPSTrend30DaysAgo
            est.RevisionsUp30Days = trend.EPSRevisionsUpLast30Days
            est.RevisionsDown30Days = trend.EPSRevisionsDownLast30Days
        }
    }

    // Classify coverage quality
    if est.NumberOfAnalysts >= 5 {
        est.CoverageQuality = "HIGH"
    } else if est.NumberOfAnalysts >= 1 {
        est.CoverageQuality = "LOW"
    } else {
        est.CoverageQuality = "NONE"
    }

    return est
}
```

### Phase 2: Annual Report PDF Extraction Worker

**New File:** `internal/workers/market/annual_report_worker.go`

This worker processes downloaded annual report PDFs to extract structured financial data.

#### Architecture

```
announcement_download_worker (existing)
    |
    | Downloads PDFs to storage (storage_key)
    v
annual_report_worker (new)
    |
    | 1. Reads PDF content from storage
    | 2. Extracts text using PDF library
    | 3. Uses LLM to extract structured data
    v
Document with extracted metrics
```

#### Extracted Metrics

| Metric | Section in Annual Report | JSON Field |
|--------|-------------------------|------------|
| ROIC | Directors Report / Financial Highlights | `roic_reported` |
| WACC | Notes to Financial Statements | `wacc_disclosed` |
| Debt Facility Terms | Notes - Borrowings | `debt_facilities[]` |
| Capital Raise Purpose | Directors Report | `capital_raise_purposes[]` |
| Management Guidance | Outlook Section | `management_guidance` |
| Segment Revenue | Segment Note | `segment_data[]` |

#### Worker Implementation

```go
package market

// AnnualReportWorker extracts structured financial data from downloaded annual report PDFs.
type AnnualReportWorker struct {
    documentStorage interfaces.DocumentStorage
    kvStorage       interfaces.KeyValueStorage
    pdfExtractor    interfaces.PDFExtractor  // New interface
    llmService      interfaces.LLMService
    logger          arbor.ILogger
}

// AnnualReportMetrics represents extracted data from annual report
type AnnualReportMetrics struct {
    Schema          string `json:"$schema"`
    Ticker          string `json:"ticker"`
    FiscalYear      string `json:"fiscal_year"`
    ReportDate      string `json:"report_date"`

    // Capital Efficiency
    ROICReported    *float64 `json:"roic_reported,omitempty"`
    WACCDisclosed   *float64 `json:"wacc_disclosed,omitempty"`
    ROICSource      string   `json:"roic_source,omitempty"` // Page/section reference

    // Debt Details
    DebtFacilities  []DebtFacility `json:"debt_facilities,omitempty"`

    // Capital Raises
    CapitalRaises   []CapitalRaiseDetail `json:"capital_raises,omitempty"`

    // Management Guidance
    RevenueGuidance     *GuidanceRange `json:"revenue_guidance,omitempty"`
    EBITDAGuidance      *GuidanceRange `json:"ebitda_guidance,omitempty"`
    ManagementOutlook   string         `json:"management_outlook,omitempty"`

    // Extraction Metadata
    ExtractionConfidence float64 `json:"extraction_confidence"`
    PagesProcessed       int     `json:"pages_processed"`
    SourceDocumentID     string  `json:"source_document_id"`
}

type DebtFacility struct {
    Name          string  `json:"name"`
    Type          string  `json:"type"`           // "Revolving", "Term Loan", etc.
    Limit         float64 `json:"limit"`
    Drawn         float64 `json:"drawn"`
    Maturity      string  `json:"maturity"`
    InterestRate  string  `json:"interest_rate"`  // e.g., "BBSY + 1.5%"
}

type CapitalRaiseDetail struct {
    Date        string  `json:"date"`
    Type        string  `json:"type"`        // "SPP", "Placement", "Rights Issue"
    Amount      float64 `json:"amount"`
    Purpose     string  `json:"purpose"`     // LLM-extracted purpose
    Shares      int64   `json:"shares_issued"`
}

type GuidanceRange struct {
    Low      float64 `json:"low,omitempty"`
    High     float64 `json:"high,omitempty"`
    Midpoint float64 `json:"midpoint,omitempty"`
    Notes    string  `json:"notes,omitempty"`
}
```

### Phase 3: PDF Extraction Interface

**New File:** `internal/interfaces/pdf_extractor.go`

```go
package interfaces

import "context"

// PDFExtractor extracts text content from PDF documents.
type PDFExtractor interface {
    // ExtractText extracts all text content from a PDF
    ExtractText(ctx context.Context, storageKey string) (string, error)

    // ExtractPages extracts text by page with page numbers
    ExtractPages(ctx context.Context, storageKey string) ([]PageContent, error)

    // ExtractTables extracts tabular data from a PDF
    ExtractTables(ctx context.Context, storageKey string) ([]TableData, error)
}

type PageContent struct {
    PageNumber int    `json:"page_number"`
    Text       string `json:"text"`
}

type TableData struct {
    PageNumber int        `json:"page_number"`
    Headers    []string   `json:"headers"`
    Rows       [][]string `json:"rows"`
}
```

**Implementation Options:**
1. **pdfcpu** (Go native) - Basic text extraction
2. **Apache Tika** (via HTTP) - Full-featured extraction with table support
3. **AWS Textract** - High-quality OCR + table extraction (paid)
4. **OpenAI Vision** - Send PDF pages as images for LLM extraction

### Phase 4: LLM-Based Extraction Prompt

The annual report worker uses structured prompts to extract metrics:

```go
const annualReportExtractionPrompt = `
You are a financial analyst extracting structured data from an annual report.

## Document Context
Company: {{.Ticker}}
Fiscal Year: {{.FiscalYear}}
Document Type: Annual Report

## Extraction Tasks

1. **ROIC (Return on Invested Capital)**
   - Search for: "return on invested capital", "ROIC", "return on capital employed", "ROCE"
   - Expected location: Directors Report, Financial Highlights
   - Extract the percentage value if disclosed

2. **WACC (Weighted Average Cost of Capital)**
   - Search for: "weighted average cost of capital", "WACC", "discount rate", "hurdle rate"
   - Expected location: Notes to Financial Statements (Impairment testing)
   - Extract the percentage value if disclosed

3. **Debt Facilities**
   - Search for: "borrowings", "debt facilities", "bank facilities", "syndicated facility"
   - Expected location: Notes to Financial Statements
   - Extract: facility name, limit, amount drawn, maturity date, interest rate

4. **Capital Raises (past 12 months)**
   - Search for: "share placement", "share purchase plan", "SPP", "capital raising"
   - Extract: date, type, amount raised, purpose stated

5. **Management Guidance**
   - Search for: "outlook", "guidance", "FY26 expectations"
   - Extract any revenue/EBITDA guidance ranges

## Response Format
Respond ONLY with valid JSON matching the AnnualReportMetrics schema.
Set extraction_confidence between 0.0 and 1.0 based on how clearly the data was stated.

## Document Text
{{.DocumentText}}
`
```

### Phase 5: Integration with Stock Deep Dive Workflow

**Update:** `test/api/config/job-definitions/stock-deep-dive-test.toml`

```toml
[[steps]]
name = "fetch_annual_report"
worker = "market_annual_report"
depends_on = ["fetch_announcements"]  # Uses downloaded PDFs
[steps.config]
ticker = "{{ticker}}"
report_types = ["Annual Report", "Appendix 4E", "Preliminary Final Report"]
max_reports = 1  # Most recent only

[[steps]]
name = "deep_dive_analysis"
worker = "summary_llm"
depends_on = ["fetch_fundamentals", "fetch_announcements", "fetch_market_data", "analyze_competitors", "fetch_annual_report"]
```

## Data Flow Architecture

```
┌────────────────────────────────────────────────────────────────────────────┐
│                        Stock Deep Dive Workflow                             │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐     │
│  │  fundamentals    │    │  announcements   │    │    competitor    │     │
│  │     worker       │    │     worker       │    │     worker       │     │
│  │  (EODHD API)     │    │   (ASX/EODHD)    │    │   (LLM-based)    │     │
│  └────────┬─────────┘    └────────┬─────────┘    └────────┬─────────┘     │
│           │                       │                       │               │
│           │              ┌────────┴─────────┐             │               │
│           │              │  announcement    │             │               │
│           │              │  download worker │             │               │
│           │              │  (PDF storage)   │             │               │
│           │              └────────┬─────────┘             │               │
│           │                       │                       │               │
│           │              ┌────────┴─────────┐             │               │
│           │              │  annual_report   │ ◄── NEW     │               │
│           │              │     worker       │             │               │
│           │              │ (PDF extraction) │             │               │
│           │              └────────┬─────────┘             │               │
│           │                       │                       │               │
│           └───────────────────────┼───────────────────────┘               │
│                                   │                                       │
│                          ┌────────┴─────────┐                             │
│                          │   summary_llm    │                             │
│                          │     worker       │                             │
│                          │ (Deep Dive AI)   │                             │
│                          └────────┬─────────┘                             │
│                                   │                                       │
│                          ┌────────┴─────────┐                             │
│                          │  Final Analysis  │                             │
│                          │    Document      │                             │
│                          └──────────────────┘                             │
└────────────────────────────────────────────────────────────────────────────┘
```

## Existing Workers to Extend

### 1. `fundamentals_worker.go` - Phase 1
**Change:** Extract additional EODHD analyst estimate fields
**Effort:** 2 hours
**Risk:** Low (additive changes)

### 2. `announcement_download_worker.go` - No changes needed
**Status:** Already downloads annual reports with filter types:
- "Annual Report"
- "Full Year"
- "FY"
- "Preliminary Final Report"
- "Appendix 4E"

## New Workers Required

### 1. `annual_report_worker.go` - Phase 2
**Purpose:** Extract structured data from annual report PDFs
**Dependencies:**
- PDF extraction library (pdfcpu or Tika)
- LLM service for structured extraction
**Effort:** 8 hours

### 2. PDF Extractor Service - Phase 3
**Purpose:** Low-level PDF text extraction
**Options:**
1. Go native (pdfcpu) - Free, basic
2. Tika Server - Feature-rich, requires Docker
3. Cloud API - High quality, per-page cost
**Effort:** 4 hours

## Testing Strategy

### Unit Tests
1. Test PDF text extraction with sample annual reports
2. Test LLM prompt with known inputs/outputs
3. Test metric parsing from extracted text

### Integration Tests
```go
func TestAnnualReportExtraction(t *testing.T) {
    // 1. Download annual report via announcement_download_worker
    // 2. Extract metrics via annual_report_worker
    // 3. Verify ROIC, WACC, debt facilities extracted
    // 4. Verify integration with deep dive summary
}
```

## Expected Outcomes

### Before v2 (Current State)
```markdown
#### 9. CURRENT VALUATION
- **Analyst Target Mean:** $0.00 (DATA_UNAVAILABLE)
- **Upside to Target:** DATA_UNAVAILABLE

#### 10. PROJECTION CONFIDENCE
- **Analyst Coverage:** 0 analysts
- **Projection Reliability:** LOW
```

### After v2 (Target State)
```markdown
#### 9. CURRENT VALUATION
- **Analyst Target Mean:** DATA_UNAVAILABLE (0 Wall Street analysts)
- **Company Guidance:** Revenue $450-500M (FY26)
- **ROIC vs WACC:** 25.18% ROIC vs 8.5% WACC = 16.7% spread

#### 10. PROJECTION CONFIDENCE
- **Analyst Coverage:** 0 analysts (supplemented with company disclosures)
- **Debt Facility Details:** $429M revolving facility at BBSY+1.5%, maturing Dec 2028
- **Projection Reliability:** MEDIUM (company guidance available)
```

### Completeness Improvement
| Metric | v1 | v2 Target |
|--------|----|----|
| Fundamental Pillars | 6/6 (100%) | 6/6 (100%) |
| Analyst Estimates | 0% | 50%+ where available |
| Forward Guidance | 0% | 80%+ from annual reports |
| Overall Confidence | 85% | 95% |

## Implementation Priority

| Phase | Effort | Value | Priority |
|-------|--------|-------|----------|
| Phase 1: EODHD Analyst Estimates | 2h | Medium | P1 |
| Phase 2: Annual Report Worker | 8h | High | P1 |
| Phase 3: PDF Extractor | 4h | Required | P1 |
| Phase 4: LLM Prompts | 3h | High | P1 |
| Phase 5: Workflow Integration | 2h | Required | P1 |
| **Total** | **19h** | | |

## Dependencies

1. **PDF Library Selection** - Recommend pdfcpu for Go-native solution
2. **LLM Service** - Already available via existing summary_worker infrastructure
3. **Storage Access** - Already available via kvStorage interface

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/workers/market/fundamentals_worker.go` | Modify | Add analyst estimate extraction |
| `internal/workers/market/annual_report_worker.go` | Create | New worker for PDF extraction |
| `internal/interfaces/pdf_extractor.go` | Create | PDF extraction interface |
| `internal/services/pdf/extractor.go` | Create | PDF extraction implementation |
| `internal/models/worker_type.go` | Modify | Add `WorkerTypeMarketAnnualReport` |
| `internal/app/app.go` | Modify | Register new worker |
| `test/api/portfolio/annual_report_worker_test.go` | Create | Integration tests |

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| PDF extraction quality varies | Use LLM to handle imperfect extraction |
| Annual report format varies by company | Generic extraction prompts with confidence scoring |
| Large PDF files slow processing | Extract only relevant pages (first 50) |
| ASX announcements API rate limits | Already handled by existing workers |

## Related Files (Reference)

- `internal/workers/market/fundamentals_worker.go` - v1 implementation
- `internal/workers/market/announcement_download_worker.go` - PDF download
- `internal/services/announcements/service.go` - Announcement fetching
- `docs/features/stock-analysis-data-completeness.md` - v1 spec
