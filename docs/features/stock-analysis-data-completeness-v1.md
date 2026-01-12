# Feature: Enhanced Stock Analysis Data Completeness

## Overview

This feature addresses data gaps identified in the stock deep dive analysis workflow. The current implementation achieves approximately 55% data completeness (3/6 pillars with sufficient data). This enhancement targets 95%+ data completeness by adding missing financial metrics.

## Background

From `test/results/api/stock_deep_dive_20260112-062410/TestStockDeepDiveWorkflow/output.md`:

| Pillar | Current Status | Gap |
|--------|---------------|-----|
| Capital Efficiency (ROIC) | PASS | None |
| Share Allocation (Dilution) | DATA_UNAVAILABLE | Historical share count CAGR |
| Financial Robustness (Net Debt/EBITDA) | DATA_UNAVAILABLE | Cash balance, Net Debt calculation |
| Cash Flow Reality (FCF Conversion) | DATA_UNAVAILABLE | Cash Flow Statement metrics |
| Management Alignment (Insider %) | PASS | None |
| Competitive Moat | PASS | None |

## Implementation Plan

### Step 1: Enhance Balance Sheet Data Extraction

**Target File**: `internal/workers/market/fundamentals_worker.go`

**Current State**:
- Extracts `TotalAssets`, `TotalLiab`, `TotalEquity`
- Does NOT extract Cash and Cash Equivalents
- Does NOT extract individual debt components

**Changes Required**:

1. Add new fields to `FinancialPeriodEntry`:
```go
type FinancialPeriodEntry struct {
    // ... existing fields ...

    // Balance Sheet - Debt/Cash Components
    CashAndEquivalents  int64 `json:"cash_and_equivalents,omitempty"`
    ShortTermDebt       int64 `json:"short_term_debt,omitempty"`
    LongTermDebt        int64 `json:"long_term_debt,omitempty"`
    TotalDebt           int64 `json:"total_debt,omitempty"`
    NetDebt             int64 `json:"net_debt,omitempty"`
}
```

2. Update `parseEODHDFinancials()` to extract from balance sheet:
```go
// Balance sheet - debt and cash components
entry.CashAndEquivalents = extractNumber(balanceData, "cash")
entry.ShortTermDebt = extractNumber(balanceData, "shortTermDebt")
entry.LongTermDebt = extractNumber(balanceData, "longTermDebt")
entry.TotalDebt = entry.ShortTermDebt + entry.LongTermDebt
entry.NetDebt = entry.TotalDebt - entry.CashAndEquivalents
```

**EODHD API Fields** (from Balance Sheet):
- `cash` - Cash and cash equivalents
- `shortTermDebt` - Short-term debt/current portion
- `longTermDebt` - Long-term debt
- `totalCurrentLiabilities` - Includes short-term debt
- `netDebt` - May be available directly

### Step 2: Add Summary-Level Financial Health Metrics

**Target File**: `internal/workers/market/fundamentals_worker.go`

**Add to `StockCollectorData`**:
```go
type StockCollectorData struct {
    // ... existing fields ...

    // Financial Health Summary (calculated)
    LatestCash          int64   `json:"latest_cash"`           // Most recent cash balance
    LatestTotalDebt     int64   `json:"latest_total_debt"`     // Short + Long term debt
    LatestNetDebt       int64   `json:"latest_net_debt"`       // TotalDebt - Cash
    NetDebtToEBITDA     float64 `json:"net_debt_to_ebitda"`    // Key leverage ratio
    LatestOperatingCF   int64   `json:"latest_operating_cf"`   // Most recent operating CF
    LatestFreeCF        int64   `json:"latest_free_cf"`        // Most recent FCF
    FCFConversion       float64 `json:"fcf_conversion"`        // FCF/NetIncome ratio
    FCFToRevenue        float64 `json:"fcf_to_revenue"`        // FCF margin

    // Share Dilution Tracking
    SharesCAGR3Y        float64 `json:"shares_cagr_3y"`        // 3-year share count CAGR
    SharesGrowthYoY     float64 `json:"shares_growth_yoy"`     // Year-over-year change
    RecentCapitalRaises []string `json:"recent_capital_raises,omitempty"` // Detected raises
}
```

**Calculation Logic**:
```go
func (w *FundamentalsWorker) calculateFinancialHealthMetrics(data *StockCollectorData) {
    // Get latest annual data
    if len(data.AnnualData) == 0 {
        return
    }
    latest := data.AnnualData[0]

    // Cash and Debt
    data.LatestCash = latest.CashAndEquivalents
    data.LatestTotalDebt = latest.TotalDebt
    data.LatestNetDebt = latest.NetDebt

    // Net Debt / EBITDA (using TTM EBITDA from highlights)
    if latest.EBITDA > 0 {
        data.NetDebtToEBITDA = float64(latest.NetDebt) / float64(latest.EBITDA)
    }

    // Cash Flow Metrics
    data.LatestOperatingCF = latest.OperatingCF
    data.LatestFreeCF = latest.FreeCF

    // FCF Conversion = FCF / Net Income
    if latest.NetIncome > 0 {
        data.FCFConversion = float64(latest.FreeCF) / float64(latest.NetIncome) * 100
    }

    // FCF to Revenue (FCF Margin)
    if latest.TotalRevenue > 0 {
        data.FCFToRevenue = float64(latest.FreeCF) / float64(latest.TotalRevenue) * 100
    }
}
```

### Step 3: Historical Shares Outstanding Tracking

**Target File**: `internal/workers/market/fundamentals_worker.go`

The EODHD API provides shares outstanding in:
1. `SharesStats.SharesOutstanding` - Current snapshot
2. `Financials.BalanceSheet.Yearly[date]["commonStockSharesOutstanding"]` - Historical

**Implementation**:
```go
// Add to StockCollectorData
type SharesHistoryEntry struct {
    Date              string  `json:"date"`
    SharesOutstanding int64   `json:"shares_outstanding"`
    ChangePercent     float64 `json:"change_percent,omitempty"`
}

// In parseEODHDFinancials, also extract shares
func (w *FundamentalsWorker) parseSharesHistory(financials eodhdFinancials, data *StockCollectorData) {
    var sharesHistory []SharesHistoryEntry

    // Sort years descending
    years := getSortedKeys(financials.BalanceSheet.Yearly)

    for _, year := range years {
        balanceData := financials.BalanceSheet.Yearly[year]
        shares := extractNumber(balanceData, "commonStockSharesOutstanding")
        if shares > 0 {
            sharesHistory = append(sharesHistory, SharesHistoryEntry{
                Date:              year,
                SharesOutstanding: shares,
            })
        }
    }

    // Calculate CAGR if we have enough data
    if len(sharesHistory) >= 3 {
        latestShares := float64(sharesHistory[0].SharesOutstanding)
        threeYearsAgoShares := float64(sharesHistory[2].SharesOutstanding)
        if threeYearsAgoShares > 0 {
            data.SharesCAGR3Y = (math.Pow(latestShares/threeYearsAgoShares, 1.0/3.0) - 1) * 100
        }
    }

    // YoY change
    if len(sharesHistory) >= 2 {
        latest := float64(sharesHistory[0].SharesOutstanding)
        previous := float64(sharesHistory[1].SharesOutstanding)
        if previous > 0 {
            data.SharesGrowthYoY = ((latest - previous) / previous) * 100
        }
    }
}
```

### Step 4: Enhanced Markdown Output

**Target File**: `internal/workers/market/fundamentals_worker.go` - `createStockDocument()`

Add new sections to the markdown output for LLM consumption:

```markdown
## Financial Health

### Debt & Liquidity
| Metric | Value | Notes |
|--------|-------|-------|
| Cash & Equivalents | $X.XXM | As of YYYY-MM-DD |
| Total Debt | $X.XXM | Short: $X.XXM + Long: $X.XXM |
| Net Debt | $X.XXM | Debt - Cash |
| Net Debt/EBITDA | X.XXx | Target: <2.0x |
| Net Debt Status | PASS/FAIL | Based on <2.0x threshold |

### Cash Flow Quality
| Metric | Value | Notes |
|--------|-------|-------|
| Operating Cash Flow | $X.XXM | TTM |
| Free Cash Flow | $X.XXM | OCF - CapEx |
| FCF Conversion | XX.X% | FCF/Net Income (Target: >90%) |
| FCF Margin | XX.X% | FCF/Revenue |
| FCF Quality | PASS/FAIL | Based on >90% threshold |

### Share Dilution Control
| Metric | Value | Notes |
|--------|-------|-------|
| Current Shares | XXXM | As of YYYY-MM-DD |
| Shares 3Y CAGR | X.XX% | Target: <=0% |
| Shares YoY Change | X.XX% | vs prior year |
| Dilution Status | PASS/FAIL | Based on <=0% threshold |
| Recent Capital Raises | None/SPP on YYYY | From announcements |
```

### Step 5: Data Completeness Metadata

**Add to document metadata**:
```go
metadata["data_completeness"] = map[string]interface{}{
    "pillars": map[string]interface{}{
        "capital_efficiency": map[string]interface{}{
            "available": data.ReturnOnEquity > 0,
            "metric":    "ROE/ROIC",
            "value":     data.ReturnOnEquity,
        },
        "share_allocation": map[string]interface{}{
            "available": data.SharesCAGR3Y != 0 || data.SharesOutstanding > 0,
            "metric":    "Shares CAGR 3Y",
            "value":     data.SharesCAGR3Y,
        },
        "financial_robustness": map[string]interface{}{
            "available": data.NetDebtToEBITDA != 0,
            "metric":    "Net Debt/EBITDA",
            "value":     data.NetDebtToEBITDA,
        },
        "cash_flow_reality": map[string]interface{}{
            "available": data.FCFConversion != 0,
            "metric":    "FCF Conversion",
            "value":     data.FCFConversion,
        },
        "management_alignment": map[string]interface{}{
            "available": data.PercentInsiders > 0,
            "metric":    "Insider %",
            "value":     data.PercentInsiders,
        },
        "competitive_moat": map[string]interface{}{
            "available": data.GrossMargin > 0 || data.OperatingMargin > 0,
            "metric":    "Margins",
            "value":     data.OperatingMargin,
        },
    },
    "completeness_score": calculateCompletenessScore(data),
    "missing_metrics":    getMissingMetrics(data),
}
```

## Testing Strategy

### Unit Tests
Location: `internal/workers/market/fundamentals_worker_test.go`

1. Test Net Debt calculation with various cash/debt scenarios
2. Test FCF Conversion calculation
3. Test Shares CAGR calculation
4. Test edge cases (zero EBITDA, negative FCF, etc.)

### Integration Tests
Location: `test/api/portfolio/stock_deep_dive_test.go`

1. Verify all 6 pillars have data (or explicit DATA_UNAVAILABLE with reason)
2. Verify markdown output includes new sections
3. Verify metadata includes completeness scoring

### Expected Output Validation

The summary output should now show:
```markdown
| Pillar | Metric | Value | Standard | Pass/Fail |
|--------|--------|-------|----------|-----------|
| Capital Efficiency | ROIC | 25.18% | >15% | PASS |
| Share Allocation | Shares CAGR | -1.2% | <=0% | PASS |
| Balance Sheet | Net Debt/EBITDA | 1.5x | <2x | PASS |
| Cash Flow | FCF Conversion | 95% | >90% | PASS |
| Alignment | Insider % | 58.46% | HIGH | PASS |
| Moat | Operating Margin | 7.25% | N/A | PASS |

**Data Completeness Score:** 6/6 pillars (100%)
```

## EODHD API Field Reference

### Balance Sheet Fields
| EODHD Field | Description | Use |
|-------------|-------------|-----|
| `cash` | Cash and cash equivalents | Net Debt calculation |
| `cashAndShortTermInvestments` | Cash + short-term investments | Alternative cash measure |
| `shortTermDebt` | Current portion of long-term debt | Total Debt |
| `longTermDebt` | Long-term debt | Total Debt |
| `totalCurrentLiabilities` | All current liabilities | Context |
| `commonStockSharesOutstanding` | Historical shares | Dilution tracking |
| `netDebt` | Pre-calculated net debt | Validation |

### Cash Flow Fields
| EODHD Field | Description | Use |
|-------------|-------------|-----|
| `totalCashFromOperatingActivities` | Operating cash flow | FCF Conversion |
| `freeCashFlow` | Free cash flow | FCF Conversion |
| `capitalExpenditures` | CapEx | FCF validation |
| `dividendsPaid` | Cash dividends | Payout analysis |

## Migration Notes

1. **Backward Compatibility**: New fields are optional (`omitempty`). Existing documents remain valid.
2. **Cache Invalidation**: Consider adding a version field to documents to trigger re-fetch for enhanced data.
3. **API Rate Limits**: No additional API calls required - data is already in fundamentals response.

## Success Criteria

1. **Quantitative**: Data completeness score improves from 55% to 95%+
2. **Qualitative**: Summary output includes actionable Pass/Fail for all 6 pillars
3. **No Regression**: Existing tests continue to pass
4. **Performance**: No additional API calls (data extraction only)

## Timeline Estimate

| Step | Effort | Dependencies |
|------|--------|--------------|
| Step 1: Balance Sheet Extraction | 2 hours | None |
| Step 2: Financial Health Metrics | 3 hours | Step 1 |
| Step 3: Shares Tracking | 2 hours | None |
| Step 4: Markdown Output | 2 hours | Steps 1-3 |
| Step 5: Completeness Metadata | 1 hour | Steps 1-4 |
| Testing | 3 hours | All |
| **Total** | **~13 hours** | |

## Related Files

- `internal/workers/market/fundamentals_worker.go` - Primary implementation
- `internal/workers/market/types.go` - Shared types
- `internal/templates/stock-deep-dive.toml` - Job definition template
- `test/api/portfolio/stock_deep_dive_test.go` - Integration tests
