// types.go - Shared types for market workers
// These types are used by multiple workers for data exchange

package market

import "time"

// OHLCV represents a single day's price data for price impact correlation
type OHLCV struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// FundamentalsFinancialData holds annual and quarterly financial data from the fundamentals document
type FundamentalsFinancialData struct {
	AnnualData    []FundamentalsFinancialPeriod
	QuarterlyData []FundamentalsFinancialPeriod
	MarketCap     int64  // Market capitalization in currency units
	Sector        string // Industry sector
}

// FundamentalsFinancialPeriod represents financial data for a single period (matches FinancialPeriodEntry in market_fundamentals_worker.go)
type FundamentalsFinancialPeriod struct {
	EndDate         string  // Date string in YYYY-MM-DD format
	PeriodType      string  // "annual" or "quarterly"
	TotalRevenue    int64   // Revenue in currency units
	GrossProfit     int64   // Gross profit
	OperatingIncome int64   // Operating income
	NetIncome       int64   // Net income (profit/loss)
	EBITDA          int64   // EBITDA
	TotalAssets     int64   // Total assets
	TotalLiab       int64   // Total liabilities
	TotalEquity     int64   // Total equity
	OperatingCF     int64   // Operating cash flow
	FreeCF          int64   // Free cash flow
	GrossMargin     float64 // Gross margin percentage
	NetMargin       float64 // Net margin percentage
}
