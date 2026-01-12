// -----------------------------------------------------------------------
// FundamentalsWorker - Consolidated Yahoo Finance data collector
// Fetches price, analyst coverage, and historical financials in a single API call
// DEPRECATED: asx_stock_data, asx_analyst_coverage, asx_historical_financials
// -----------------------------------------------------------------------

package market

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// parseTicker parses a ticker from config, supporting both legacy ("GNP") and
// exchange-qualified ("ASX:GNP") formats.
func parseTicker(config map[string]interface{}) common.Ticker {
	// Try ticker first (new format), then asx_code (legacy)
	if ticker, ok := config["ticker"].(string); ok && ticker != "" {
		return common.ParseTicker(ticker)
	}
	if asxCode, ok := config["asx_code"].(string); ok && asxCode != "" {
		return common.ParseTicker(asxCode)
	}
	return common.Ticker{}
}

// collectTickers collects all tickers from step config only.
// Supports: ticker, asx_code (single) and tickers, asx_codes (array).
// For job-level variables support, use collectTickersWithJobDef instead.
func collectTickers(config map[string]interface{}) []common.Ticker {
	return collectTickersWithJobDef(config, models.JobDefinition{})
}

// collectTickersWithJobDef collects all tickers from both step config and job-level variables.
// Sources (in order of priority):
//  1. Step config: ticker, asx_code (single)
//  2. Step config: tickers, asx_codes (array)
//  3. Job-level: config.variables = [{ ticker = "..." }, { asx_code = "..." }, ...]
func collectTickersWithJobDef(stepConfig map[string]interface{}, jobDef models.JobDefinition) []common.Ticker {
	var tickers []common.Ticker
	seen := make(map[string]bool)

	addTicker := func(t common.Ticker) {
		if t.Code != "" && !seen[t.String()] {
			seen[t.String()] = true
			tickers = append(tickers, t)
		}
	}

	// Source 1: Single ticker from step config (legacy)
	if stepConfig != nil {
		if t := parseTicker(stepConfig); t.Code != "" {
			addTicker(t)
		}

		// Source 2: Array of tickers from step config
		if tickerArray, ok := stepConfig["tickers"].([]interface{}); ok {
			for _, v := range tickerArray {
				if s, ok := v.(string); ok && s != "" {
					addTicker(common.ParseTicker(s))
				}
			}
		}

		// Array of asx_codes (legacy) from step config
		if codeArray, ok := stepConfig["asx_codes"].([]interface{}); ok {
			for _, v := range codeArray {
				if s, ok := v.(string); ok && s != "" {
					addTicker(common.ParseTicker(s))
				}
			}
		}
	}

	// Source 3: Job-level variables (multiple tickers)
	if jobDef.Config != nil {
		if vars, ok := jobDef.Config["variables"].([]interface{}); ok {
			for _, v := range vars {
				varMap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				// Try "ticker" key (e.g., "ASX:GNP" or "GNP")
				if ticker, ok := varMap["ticker"].(string); ok && ticker != "" {
					addTicker(common.ParseTicker(ticker))
				}
				// Try "asx_code" key
				if asxCode, ok := varMap["asx_code"].(string); ok && asxCode != "" {
					addTicker(common.ParseTicker(asxCode))
				}
			}
		}
	}

	return tickers
}

// EODHD API configuration
const (
	eodhdAPIBaseURL   = "https://eodhd.com/api"
	eodhdAPIKeyEnvVar = "eodhd_api_key"
)

// FundamentalsWorker fetches comprehensive stock data using EODHD API.
// This consolidates asx_stock_data, asx_analyst_coverage, and asx_historical_financials.
// Optionally generates a company blurb via LLM if providerFactory is available.
type FundamentalsWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
	providerFactory *llm.ProviderFactory
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*FundamentalsWorker)(nil)

// StockCollectorData holds all consolidated stock data (in-code schema)
type StockCollectorData struct {
	// Core identification
	Symbol       string `json:"symbol"`
	CompanyName  string `json:"company_name"`
	CompanyBlurb string `json:"company_blurb,omitempty"`
	AsxCode      string `json:"asx_code"`
	Currency     string `json:"currency"`
	ISIN         string `json:"isin,omitempty"`
	Sector       string `json:"sector,omitempty"`
	Industry     string `json:"industry,omitempty"`

	// Price data (from EOD)
	CurrentPrice  float64 `json:"current_price"`
	PriceChange   float64 `json:"price_change"`
	ChangePercent float64 `json:"change_percent"`
	DayLow        float64 `json:"day_low"`
	DayHigh       float64 `json:"day_high"`
	Week52Low     float64 `json:"week_52_low"`
	Week52High    float64 `json:"week_52_high"`
	Volume        int64   `json:"volume"`
	AvgVolume     int64   `json:"avg_volume"`
	MarketCap     int64   `json:"market_cap"`

	// Valuation
	PERatio         float64 `json:"pe_ratio"`
	ForwardPE       float64 `json:"forward_pe"`
	PEGRatio        float64 `json:"peg_ratio"`
	EPS             float64 `json:"eps"`
	DividendYield   float64 `json:"dividend_yield"`
	BookValue       float64 `json:"book_value"`
	PriceToBook     float64 `json:"price_to_book"`
	PriceToSales    float64 `json:"price_to_sales"`
	EnterpriseValue int64   `json:"enterprise_value"`
	EVToRevenue     float64 `json:"ev_to_revenue"`
	EVToEBITDA      float64 `json:"ev_to_ebitda"`
	Beta            float64 `json:"beta"`

	// Profitability metrics
	ProfitMargin    float64 `json:"profit_margin"`
	OperatingMargin float64 `json:"operating_margin"`
	ReturnOnAssets  float64 `json:"return_on_assets"`
	ReturnOnEquity  float64 `json:"return_on_equity"`

	// Shares statistics
	SharesOutstanding   int64   `json:"shares_outstanding"`
	SharesFloat         int64   `json:"shares_float"`
	PercentInsiders     float64 `json:"percent_insiders"`
	PercentInstitutions float64 `json:"percent_institutions"`

	// Technicals (calculated from historical prices)
	SMA20       float64 `json:"sma_20"`
	SMA50       float64 `json:"sma_50"`
	SMA200      float64 `json:"sma_200"`
	RSI14       float64 `json:"rsi_14"`
	Support     float64 `json:"support"`
	Resistance  float64 `json:"resistance"`
	TrendSignal string  `json:"trend_signal"` // "BULLISH", "BEARISH", "NEUTRAL"

	// Analyst coverage
	AnalystCount       int     `json:"analyst_count"`
	TargetMean         float64 `json:"target_mean"`
	TargetHigh         float64 `json:"target_high"`
	TargetLow          float64 `json:"target_low"`
	TargetMedian       float64 `json:"target_median"`
	UpsidePotential    float64 `json:"upside_potential"`
	RecommendationMean float64 `json:"recommendation_mean"` // 1=Strong Buy, 5=Strong Sell
	RecommendationKey  string  `json:"recommendation_key"`  // "buy", "hold", "sell"
	StrongBuy          int     `json:"strong_buy"`
	Buy                int     `json:"buy"`
	Hold               int     `json:"hold"`
	Sell               int     `json:"sell"`
	StrongSell         int     `json:"strong_sell"`

	// Upgrade/downgrade history
	UpgradeDowngrades []UpgradeDowngradeEntry `json:"upgrade_downgrades,omitempty"`

	// ESG Scores
	ESGTotalScore       float64 `json:"esg_total_score"`
	ESGEnvironmentScore float64 `json:"esg_environment_score"`
	ESGSocialScore      float64 `json:"esg_social_score"`
	ESGGovernanceScore  float64 `json:"esg_governance_score"`
	ESGControversy      int     `json:"esg_controversy"`

	// Earnings history
	EarningsHistory []EarningsEntry `json:"earnings_history,omitempty"`

	// Dividends
	DividendRate    float64 `json:"dividend_rate"`
	DividendExDate  string  `json:"dividend_ex_date,omitempty"`
	DividendPayDate string  `json:"dividend_pay_date,omitempty"`
	PayoutRatio     float64 `json:"payout_ratio"`

	// Historical financials
	RevenueGrowthYoY float64                `json:"revenue_growth_yoy"`
	ProfitGrowthYoY  float64                `json:"profit_growth_yoy"`
	RevenueCAGR3Y    float64                `json:"revenue_cagr_3y"`
	RevenueCAGR5Y    float64                `json:"revenue_cagr_5y"`
	AnnualData       []FinancialPeriodEntry `json:"annual_data,omitempty"`
	QuarterlyData    []FinancialPeriodEntry `json:"quarterly_data,omitempty"`

	// Historical prices (for charts and downstream analysis)
	HistoricalPrices []OHLCVEntry `json:"historical_prices,omitempty"`

	// Period performance (calculated)
	PeriodPerformance []PeriodPerformanceEntry `json:"period_performance,omitempty"`

	// Financial Health Summary (Step 2: calculated metrics for Kneppy pillars)
	LatestCash        int64   `json:"latest_cash"`         // Most recent cash balance
	LatestTotalDebt   int64   `json:"latest_total_debt"`   // Short + Long term debt
	LatestNetDebt     int64   `json:"latest_net_debt"`     // TotalDebt - Cash
	NetDebtToEBITDA   float64 `json:"net_debt_to_ebitda"`  // Key leverage ratio
	LatestOperatingCF int64   `json:"latest_operating_cf"` // Most recent operating CF
	LatestFreeCF      int64   `json:"latest_free_cf"`      // Most recent FCF
	FCFConversion     float64 `json:"fcf_conversion"`      // FCF/NetIncome ratio (%)
	FCFToRevenue      float64 `json:"fcf_to_revenue"`      // FCF margin (%)
	GrossMargin       float64 `json:"gross_margin"`        // Latest gross margin (%)

	// Share Dilution Tracking (Step 3: historical shares analysis)
	SharesCAGR3Y    float64              `json:"shares_cagr_3y"`    // 3-year share count CAGR
	SharesGrowthYoY float64              `json:"shares_growth_yoy"` // Year-over-year change
	SharesHistory   []SharesHistoryEntry `json:"shares_history,omitempty"`

	// Analyst Estimates (v2: enhanced forward-looking data)
	AnalystEstimates *AnalystEstimatesData `json:"analyst_estimates,omitempty"`

	// Metadata
	LastUpdated time.Time `json:"last_updated"`
}

// AnalystEstimatesData holds forward-looking analyst estimate data from EODHD Earnings.Trend
type AnalystEstimatesData struct {
	// EPS Estimates
	EPSCurrentYear  float64 `json:"eps_current_year"`  // EPSEstimateCurrentYear from Highlights
	EPSNextYear     float64 `json:"eps_next_year"`     // EPSEstimateNextYear from Highlights
	EPSCurrentQtr   float64 `json:"eps_current_qtr"`   // EPSEstimateCurrentQuarter from Highlights
	EPSNextQtr      float64 `json:"eps_next_qtr"`      // EPSEstimateNextQuarter from Highlights
	EPSEstimateAvg  float64 `json:"eps_estimate_avg"`  // Current year consensus
	EPSEstimateLow  float64 `json:"eps_estimate_low"`  // Low estimate
	EPSEstimateHigh float64 `json:"eps_estimate_high"` // High estimate

	// Revenue Estimates
	RevenueEstimateAvg  float64 `json:"revenue_estimate_avg"`  // Consensus revenue estimate
	RevenueEstimateLow  float64 `json:"revenue_estimate_low"`  // Low estimate
	RevenueEstimateHigh float64 `json:"revenue_estimate_high"` // High estimate

	// Analyst Coverage
	EarningsAnalystCount int `json:"earnings_analyst_count"` // Number of analysts covering EPS
	RevenueAnalystCount  int `json:"revenue_analyst_count"`  // Number of analysts covering revenue

	// EPS Estimate Trends (momentum indicators)
	EPSTrendCurrent   float64 `json:"eps_trend_current"`    // Current estimate
	EPSTrend7DaysAgo  float64 `json:"eps_trend_7d_ago"`     // Estimate 7 days ago
	EPSTrend30DaysAgo float64 `json:"eps_trend_30d_ago"`    // Estimate 30 days ago
	EPSTrend60DaysAgo float64 `json:"eps_trend_60d_ago"`    // Estimate 60 days ago
	EPSTrend90DaysAgo float64 `json:"eps_trend_90d_ago"`    // Estimate 90 days ago
	EPSTrendChange30D float64 `json:"eps_trend_change_30d"` // % change in estimates over 30 days

	// EPS Revisions (analyst sentiment)
	RevisionsUp7Days    int `json:"revisions_up_7d"`    // Upward revisions last 7 days
	RevisionsUp30Days   int `json:"revisions_up_30d"`   // Upward revisions last 30 days
	RevisionsDown30Days int `json:"revisions_down_30d"` // Downward revisions last 30 days

	// Coverage Quality Assessment
	CoverageQuality string `json:"coverage_quality"` // "HIGH" (5+), "LOW" (1-4), "NONE" (0)
	EstimatePeriod  string `json:"estimate_period"`  // Period these estimates apply to (e.g., "0y", "+1y")
}

// SharesHistoryEntry tracks historical shares outstanding (Step 3)
type SharesHistoryEntry struct {
	Date              string  `json:"date"`
	SharesOutstanding int64   `json:"shares_outstanding"`
	ChangePercent     float64 `json:"change_percent,omitempty"`
}

// EarningsEntry holds earnings data for a single period
type EarningsEntry struct {
	Date            string  `json:"date"`
	ReportDate      string  `json:"report_date"`
	EPSActual       float64 `json:"eps_actual"`
	EPSEstimate     float64 `json:"eps_estimate"`
	EPSSurprise     float64 `json:"eps_surprise"`
	EPSSurprisePerc float64 `json:"eps_surprise_percent"`
}

// UpgradeDowngradeEntry represents a single analyst action
type UpgradeDowngradeEntry struct {
	Date      string `json:"date"`
	Firm      string `json:"firm"`
	Action    string `json:"action"` // "up", "down", "init", "main"
	FromGrade string `json:"from_grade"`
	ToGrade   string `json:"to_grade"`
}

// FinancialPeriodEntry holds financial data for a single period
type FinancialPeriodEntry struct {
	EndDate         string  `json:"end_date"`
	PeriodType      string  `json:"period_type"` // "annual" or "quarterly"
	TotalRevenue    int64   `json:"total_revenue"`
	GrossProfit     int64   `json:"gross_profit"`
	OperatingIncome int64   `json:"operating_income"`
	NetIncome       int64   `json:"net_income"`
	EBITDA          int64   `json:"ebitda,omitempty"`
	TotalAssets     int64   `json:"total_assets,omitempty"`
	TotalLiab       int64   `json:"total_liabilities,omitempty"`
	TotalEquity     int64   `json:"total_equity,omitempty"`
	OperatingCF     int64   `json:"operating_cash_flow,omitempty"`
	FreeCF          int64   `json:"free_cash_flow,omitempty"`
	GrossMargin     float64 `json:"gross_margin"`
	NetMargin       float64 `json:"net_margin"`

	// Balance Sheet - Debt/Cash Components (Step 1: Enhanced data extraction)
	CashAndEquivalents int64 `json:"cash_and_equivalents,omitempty"`
	ShortTermDebt      int64 `json:"short_term_debt,omitempty"`
	LongTermDebt       int64 `json:"long_term_debt,omitempty"`
	TotalDebt          int64 `json:"total_debt,omitempty"`
	NetDebt            int64 `json:"net_debt,omitempty"`
	SharesOutstanding  int64 `json:"shares_outstanding,omitempty"`
}

// OHLCVEntry represents a single day's price data
type OHLCVEntry struct {
	Date          string  `json:"date"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Close         float64 `json:"close"`
	Volume        int64   `json:"volume"`
	ChangeValue   float64 `json:"change_value,omitempty"`
	ChangePercent float64 `json:"change_percent,omitempty"`
}

// PeriodPerformanceEntry holds price change data for a time period
type PeriodPerformanceEntry struct {
	Period        string  `json:"period"`
	Days          int     `json:"days"`
	Price         float64 `json:"price"`
	ChangeValue   float64 `json:"change_value"`
	ChangePercent float64 `json:"change_percent"`
	Shares1k      int     `json:"shares_1k"` // Number of shares $1000 would buy
	Value1k       float64 `json:"value_1k"`  // Current value of those shares
}

func init() {
	// Register types for gob encoding (required for BadgerHold storage of interface{} fields)
	gob.Register([]OHLCVEntry{})
	gob.Register([]PeriodPerformanceEntry{})
	gob.Register([]EarningsEntry{})
	gob.Register([]UpgradeDowngradeEntry{})
	gob.Register([]FinancialPeriodEntry{})
	gob.Register([]SharesHistoryEntry{})
	gob.Register(&AnalystEstimatesData{})
}

// eodhdEODData represents a single EOD record from EODHD
type eodhdEODData struct {
	Date          string  `json:"date"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Close         float64 `json:"close"`
	AdjustedClose float64 `json:"adjusted_close"`
	Volume        int64   `json:"volume"`
}

// eodhdFundamentalsResponse for EODHD /api/fundamentals/ endpoint
type eodhdFundamentalsResponse struct {
	General         eodhdGeneral         `json:"General"`
	Highlights      eodhdHighlights      `json:"Highlights"`
	Valuation       eodhdValuation       `json:"Valuation"`
	SharesStats     eodhdSharesStats     `json:"SharesStats"`
	Technicals      eodhdTechnicals      `json:"Technicals"`
	SplitsDividends eodhdSplitsDividends `json:"SplitsDividends"`
	AnalystRatings  eodhdAnalystRatings  `json:"AnalystRatings"`
	Holders         eodhdHolders         `json:"Holders"`
	ESGScores       eodhdESGScores       `json:"ESGScores"`
	Earnings        eodhdEarnings        `json:"Earnings"`
	Financials      eodhdFinancials      `json:"Financials"`
}

type eodhdGeneral struct {
	Code              string `json:"Code"`
	Name              string `json:"Name"`
	Exchange          string `json:"Exchange"`
	CurrencyCode      string `json:"CurrencyCode"`
	CurrencyName      string `json:"CurrencyName"`
	CurrencySymbol    string `json:"CurrencySymbol"`
	CountryName       string `json:"CountryName"`
	CountryISO        string `json:"CountryISO"`
	ISIN              string `json:"ISIN"`
	Sector            string `json:"Sector"`
	Industry          string `json:"Industry"`
	Description       string `json:"Description"`
	FullTimeEmployees int    `json:"FullTimeEmployees"`
	WebURL            string `json:"WebURL"`
}

type eodhdHighlights struct {
	MarketCapitalization       int64   `json:"MarketCapitalization"`
	MarketCapitalizationMln    float64 `json:"MarketCapitalizationMln"`
	EBITDA                     int64   `json:"EBITDA"`
	PERatio                    float64 `json:"PERatio"`
	PEGRatio                   float64 `json:"PEGRatio"`
	WallStreetTargetPrice      float64 `json:"WallStreetTargetPrice"`
	BookValue                  float64 `json:"BookValue"`
	DividendShare              float64 `json:"DividendShare"`
	DividendYield              float64 `json:"DividendYield"`
	EarningsShare              float64 `json:"EarningsShare"`
	EPSEstimateCurrentYear     float64 `json:"EPSEstimateCurrentYear"`
	EPSEstimateNextYear        float64 `json:"EPSEstimateNextYear"`
	EPSEstimateNextQuarter     float64 `json:"EPSEstimateNextQuarter"`
	EPSEstimateCurrentQuarter  float64 `json:"EPSEstimateCurrentQuarter"`
	MostRecentQuarter          string  `json:"MostRecentQuarter"`
	ProfitMargin               float64 `json:"ProfitMargin"`
	OperatingMarginTTM         float64 `json:"OperatingMarginTTM"`
	ReturnOnAssetsTTM          float64 `json:"ReturnOnAssetsTTM"`
	ReturnOnEquityTTM          float64 `json:"ReturnOnEquityTTM"`
	RevenueTTM                 int64   `json:"RevenueTTM"`
	RevenuePerShareTTM         float64 `json:"RevenuePerShareTTM"`
	QuarterlyRevenueGrowthYOY  float64 `json:"QuarterlyRevenueGrowthYOY"`
	GrossProfitTTM             int64   `json:"GrossProfitTTM"`
	DilutedEpsTTM              float64 `json:"DilutedEpsTTM"`
	QuarterlyEarningsGrowthYOY float64 `json:"QuarterlyEarningsGrowthYOY"`
}

type eodhdValuation struct {
	TrailingPE             float64 `json:"TrailingPE"`
	ForwardPE              float64 `json:"ForwardPE"`
	PriceSalesTTM          float64 `json:"PriceSalesTTM"`
	PriceBookMRQ           float64 `json:"PriceBookMRQ"`
	EnterpriseValue        int64   `json:"EnterpriseValue"`
	EnterpriseValueRevenue float64 `json:"EnterpriseValueRevenue"`
	EnterpriseValueEbitda  float64 `json:"EnterpriseValueEbitda"`
}

type eodhdSharesStats struct {
	SharesOutstanding       int64   `json:"SharesOutstanding"`
	SharesFloat             int64   `json:"SharesFloat"`
	PercentInsiders         float64 `json:"PercentInsiders"`
	PercentInstitutions     float64 `json:"PercentInstitutions"`
	SharesShort             int64   `json:"SharesShort"`
	ShortRatio              float64 `json:"ShortRatio"`
	ShortPercentOutstanding float64 `json:"ShortPercentOutstanding"`
	ShortPercentFloat       float64 `json:"ShortPercentFloat"`
}

type eodhdTechnicals struct {
	Beta                  float64 `json:"Beta"`
	FiftyTwoWeekHigh      float64 `json:"52WeekHigh"`
	FiftyTwoWeekLow       float64 `json:"52WeekLow"`
	FiftyDayMA            float64 `json:"50DayMA"`
	TwoHundredDayMA       float64 `json:"200DayMA"`
	SharesShort           int64   `json:"SharesShort"`
	SharesShortPriorMonth int64   `json:"SharesShortPriorMonth"`
	ShortRatio            float64 `json:"ShortRatio"`
	ShortPercent          float64 `json:"ShortPercent"`
}

type eodhdSplitsDividends struct {
	ForwardAnnualDividendRate  float64 `json:"ForwardAnnualDividendRate"`
	ForwardAnnualDividendYield float64 `json:"ForwardAnnualDividendYield"`
	PayoutRatio                float64 `json:"PayoutRatio"`
	DividendDate               string  `json:"DividendDate"`
	ExDividendDate             string  `json:"ExDividendDate"`
	LastSplitFactor            string  `json:"LastSplitFactor"`
	LastSplitDate              string  `json:"LastSplitDate"`
}

type eodhdAnalystRatings struct {
	Rating      float64 `json:"Rating"`
	TargetPrice float64 `json:"TargetPrice"`
	StrongBuy   int     `json:"StrongBuy"`
	Buy         int     `json:"Buy"`
	Hold        int     `json:"Hold"`
	Sell        int     `json:"Sell"`
	StrongSell  int     `json:"StrongSell"`
}

type eodhdHolders struct {
	// EODHD returns Institutions as an object with numeric keys: {"0": {...}, "1": {...}}
	Institutions map[string]eodhdInstitution `json:"Institutions"`
	Funds        map[string]eodhdInstitution `json:"Funds"`
}

type eodhdInstitution struct {
	Name          string  `json:"name"`
	Date          string  `json:"date"`
	TotalShares   float64 `json:"totalShares"`
	TotalAssets   float64 `json:"totalAssets"`
	CurrentShares int64   `json:"currentShares"`
	Change        int64   `json:"change"`
	ChangeP       float64 `json:"change_p"`
}

type eodhdESGScores struct {
	RatingDate       string  `json:"ratingDate"`
	TotalEsg         float64 `json:"totalEsg"`
	EnvironmentScore float64 `json:"environmentScore"`
	SocialScore      float64 `json:"socialScore"`
	GovernanceScore  float64 `json:"governanceScore"`
	ControversyLevel int     `json:"controversyLevel"`
}

type eodhdEarnings struct {
	// EODHD returns History as an object with numeric keys: {"0": {...}, "1": {...}}
	History map[string]eodhdEarningsHistory `json:"History"`
	Trend   map[string]eodhdEarningsTrend   `json:"Trend"`
	Annual  map[string]eodhdEarningsAnnual  `json:"Annual"`
}

type eodhdEarningsHistory struct {
	ReportDate        string  `json:"reportDate"`
	Date              string  `json:"date"`
	BeforeAfterMarket string  `json:"beforeAfterMarket"`
	Currency          string  `json:"currency"`
	EpsActual         float64 `json:"epsActual"`
	EpsEstimate       float64 `json:"epsEstimate"`
	EpsDifference     float64 `json:"epsDifference"`
	SurprisePercent   float64 `json:"surprisePercent"`
}

// eodhdEarningsTrend uses interface{} for numeric fields because EODHD API returns
// these as strings (e.g., "-0.0401") instead of numbers
type eodhdEarningsTrend struct {
	Date                             string      `json:"date"`
	Period                           string      `json:"period"`
	Growth                           interface{} `json:"growth"`
	EarningsEstimateAvg              interface{} `json:"earningsEstimateAvg"`
	EarningsEstimateLow              interface{} `json:"earningsEstimateLow"`
	EarningsEstimateHigh             interface{} `json:"earningsEstimateHigh"`
	EarningsEstimateNumberOfAnalysts interface{} `json:"earningsEstimateNumberOfAnalysts"`
	RevenueEstimateAvg               interface{} `json:"revenueEstimateAvg"`
	RevenueEstimateLow               interface{} `json:"revenueEstimateLow"`
	RevenueEstimateHigh              interface{} `json:"revenueEstimateHigh"`
	RevenueEstimateNumberOfAnalysts  interface{} `json:"revenueEstimateNumberOfAnalysts"`
	EPSTrendCurrent                  interface{} `json:"epsTrendCurrent"`
	EPSTrend7DaysAgo                 interface{} `json:"epsTrend7daysAgo"`
	EPSTrend30DaysAgo                interface{} `json:"epsTrend30daysAgo"`
	EPSTrend60DaysAgo                interface{} `json:"epsTrend60daysAgo"`
	EPSTrend90DaysAgo                interface{} `json:"epsTrend90daysAgo"`
	EPSRevisionsUpLast7Days          interface{} `json:"epsRevisionsUpLast7days"`
	EPSRevisionsUpLast30Days         interface{} `json:"epsRevisionsUpLast30days"`
	EPSRevisionsDownLast30Days       interface{} `json:"epsRevisionsDownLast30days"`
}

type eodhdEarningsAnnual struct {
	Date      string  `json:"date"`
	EpsActual float64 `json:"epsActual"`
}

type eodhdFinancials struct {
	BalanceSheet    eodhdFinancialStatements `json:"Balance_Sheet"`
	CashFlow        eodhdFinancialStatements `json:"Cash_Flow"`
	IncomeStatement eodhdFinancialStatements `json:"Income_Statement"`
}

type eodhdFinancialStatements struct {
	Currency  string                            `json:"currency"`
	Yearly    map[string]map[string]interface{} `json:"yearly"`
	Quarterly map[string]map[string]interface{} `json:"quarterly"`
}

// NewFundamentalsWorker creates a new consolidated stock collector worker
func NewFundamentalsWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	providerFactory *llm.ProviderFactory,
	debugEnabled bool,
) *FundamentalsWorker {
	return &FundamentalsWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		providerFactory: providerFactory,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypeMarketFundamentals
func (w *FundamentalsWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketFundamentals
}

// Init initializes the stock collector worker
func (w *FundamentalsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Collect tickers - supports both step config and job-level variables
	tickers := collectTickersWithJobDef(stepConfig, jobDef)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("ticker, asx_code, tickers, or asx_codes is required in step config or job variables")
	}

	// Period for historical data (default Y2 = 24 months)
	period := "Y2"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Str("period", period).
		Msg("ASX stock collector worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   ticker.Code,
			Name: fmt.Sprintf("Fetch %s comprehensive stock data", ticker.String()),
			Type: "market_fundamentals",
			Config: map[string]interface{}{
				"ticker":   ticker.String(),
				"asx_code": ticker.Code,
				"period":   period,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(tickers),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"tickers":     tickers,
			"period":      period,
			"step_config": stepConfig,
		},
	}, nil
}

// isCacheFresh checks if a document was synced within the cache window
func (w *FundamentalsWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// CreateJobs fetches comprehensive stock data and stores as document
func (w *FundamentalsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize market_fundamentals worker: %w", err)
		}
	}

	// Get tickers from metadata
	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	period, _ := initResult.Metadata["period"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Check cache settings
	cacheHours := 24
	if ch, ok := stepConfig["cache_hours"].(float64); ok {
		cacheHours = int(ch)
	}
	forceRefresh := false
	if fr, ok := stepConfig["force_refresh"].(bool); ok {
		forceRefresh = fr
	}

	// Extract output_tags (supports both []interface{} from TOML and []string from inline calls)
	var outputTags []string
	if stepConfig != nil {
		if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					outputTags = append(outputTags, tagStr)
				}
			}
		} else if tags, ok := stepConfig["output_tags"].([]string); ok {
			outputTags = tags
		}
	}

	processedCount := 0
	errorCount := 0
	var allDocIDs []string
	var allTags []string
	var allSourceIDs []string
	var allErrors []string
	tagsSeen := make(map[string]bool)
	byTicker := make(map[string]*interfaces.TickerResult)

	for _, ticker := range tickers {
		docInfo, err := w.processTicker(ctx, ticker, period, cacheHours, forceRefresh, &jobDef, stepID, outputTags)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", ticker.String(), err)
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to fetch stock data")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("%s - Failed: %v", ticker.String(), err))
			}
			allErrors = append(allErrors, errMsg)
			errorCount++
			continue
		}

		// Collect document info
		if docInfo != nil {
			allDocIDs = append(allDocIDs, docInfo.ID)
			allSourceIDs = append(allSourceIDs, docInfo.SourceID)
			for _, tag := range docInfo.Tags {
				if !tagsSeen[tag] {
					tagsSeen[tag] = true
					allTags = append(allTags, tag)
				}
			}

			// Store per-ticker result
			byTicker[ticker.String()] = &interfaces.TickerResult{
				DocumentsCreated: 1,
				DocumentIDs:      []string{docInfo.ID},
				Tags:             docInfo.Tags,
			}
		}
		processedCount++
	}

	w.logger.Info().
		Int("processed", processedCount).
		Int("errors", errorCount).
		Int("documents", len(allDocIDs)).
		Msg("Stock collector complete")

	// Build WorkerResult for test validation
	workerResult := &interfaces.WorkerResult{
		DocumentsCreated: processedCount,
		DocumentIDs:      allDocIDs,
		Tags:             allTags,
		SourceType:       "market_fundamentals",
		SourceIDs:        allSourceIDs,
		Errors:           allErrors,
		ByTicker:         byTicker,
	}

	if w.jobMgr != nil {
		// Store WorkerResult in job metadata for test validation
		if err := w.jobMgr.UpdateJobMetadata(ctx, stepID, map[string]interface{}{
			"worker_result": workerResult.ToMap(),
		}); err != nil {
			w.logger.Warn().Err(err).Str("step_id", stepID).Msg("Failed to update job metadata with worker result")
		}
	}

	return stepID, nil
}

// docInfo holds document info for per-ticker results
type docInfo struct {
	ID       string
	SourceID string
	Tags     []string
}

// processTicker processes a single ticker and returns document info
func (w *FundamentalsWorker) processTicker(ctx context.Context, ticker common.Ticker, period string, cacheHours int, forceRefresh bool, jobDef *models.JobDefinition, stepID string, outputTags []string) (*docInfo, error) {
	// Initialize debug tracking
	debug := workerutil.NewWorkerDebug("market_fundamentals", w.debugEnabled)
	debug.SetTicker(ticker.String())
	debug.SetJobID(stepID) // Include job ID in debug output

	sourceType := "market_fundamentals"
	sourceID := ticker.SourceID("stock_collector")

	// Check for cached data before fetching
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && w.isCacheFresh(existingDoc, cacheHours) {
			w.logger.Info().
				Str("ticker", ticker.String()).
				Str("doc_id", existingDoc.ID).
				Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
				Int("cache_hours", cacheHours).
				Msg("Using cached stock collector data")

			// Associate cached document with current job for downstream workers
			if err := workerutil.AssociateDocumentWithJob(ctx, existingDoc, stepID, w.documentStorage, w.logger); err != nil {
				w.logger.Warn().Err(err).Str("doc_id", existingDoc.ID).Str("step_id", stepID).Msg("Failed to associate cached document with job")
			}

			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("%s - Using cached data (last synced: %s)",
						ticker.String(), existingDoc.LastSynced.Format("2006-01-02 15:04")))
			}

			return &docInfo{
				ID:       existingDoc.ID,
				SourceID: existingDoc.SourceID,
				Tags:     existingDoc.Tags,
			}, nil
		}
	}

	w.logger.Info().
		Str("phase", "run").
		Str("ticker", ticker.String()).
		Str("period", period).
		Msg("Fetching comprehensive stock data")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching %s comprehensive stock data (price, analyst, financials)", ticker.String()))
	}

	// Fetch all data using EODHD symbol format
	debug.StartPhase("api_fetch")
	stockData, err := w.fetchComprehensiveData(ctx, ticker, period)
	debug.EndPhase("api_fetch")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stock data: %w", err)
	}

	// Create and save document
	debug.StartPhase("json_generation")
	doc := w.createDocument(ctx, stockData, ticker, jobDef, stepID, outputTags, debug)
	debug.EndPhase("json_generation")

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return nil, fmt.Errorf("failed to save stock data: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Str("doc_id", doc.ID).
		Float64("price", stockData.CurrentPrice).
		Str("trend", stockData.TrendSignal).
		Int("analysts", stockData.AnalystCount).
		Float64("upside", stockData.UpsidePotential).
		Msg("Comprehensive stock data processed and saved")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("%s - Price: $%.2f, Trend: %s, Analysts: %d, Target: $%.2f (%.1f%% upside)",
				ticker.String(), stockData.CurrentPrice, stockData.TrendSignal,
				stockData.AnalystCount, stockData.TargetMean, stockData.UpsidePotential))
	}

	return &docInfo{
		ID:       doc.ID,
		SourceID: doc.SourceID,
		Tags:     doc.Tags,
	}, nil
}

// ReturnsChildJobs returns false
func (w *FundamentalsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
// Config can be nil if tickers will be provided via job-level variables.
func (w *FundamentalsWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - tickers can come from job-level variables
	// Full validation happens in Init() when we have access to jobDef
	return nil
}

// getEODHDAPIKey retrieves the EODHD API key from KV storage
func (w *FundamentalsWorker) getEODHDAPIKey(ctx context.Context) string {
	if w.kvStorage == nil {
		w.logger.Warn().Msg("EODHD API key lookup failed: kvStorage is nil")
		return ""
	}
	apiKey, err := common.ResolveAPIKey(ctx, w.kvStorage, eodhdAPIKeyEnvVar, "")
	if err != nil {
		w.logger.Warn().Err(err).Str("key_name", eodhdAPIKeyEnvVar).Msg("Failed to resolve EODHD API key")
		return ""
	}
	if apiKey == "" {
		w.logger.Warn().Str("key_name", eodhdAPIKeyEnvVar).Msg("EODHD API key is empty")
	}
	return apiKey
}

// makeRequest makes an HTTP request
func (w *FundamentalsWorker) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	return w.httpClient.Do(req)
}

// fetchComprehensiveData fetches all stock data from EODHD API
func (w *FundamentalsWorker) fetchComprehensiveData(ctx context.Context, ticker common.Ticker, period string) (*StockCollectorData, error) {
	data := &StockCollectorData{
		Symbol:      ticker.String(),
		AsxCode:     ticker.Code,
		LastUpdated: time.Now(),
	}

	apiKey := w.getEODHDAPIKey(ctx)
	if apiKey == "" {
		w.logger.Error().
			Str("ticker", ticker.String()).
			Str("key_name", eodhdAPIKeyEnvVar).
			Msg("EODHD API key not found - stock data will be empty")
		return nil, fmt.Errorf("EODHD API key '%s' not configured in KV store", eodhdAPIKeyEnvVar)
	}

	// Use ticker's EODHD symbol format (e.g., "GNP.AU" for ASX:GNP)
	eodhdSymbol := ticker.EODHDSymbol()

	// STEP 1: Fetch fundamentals (includes most data)
	if err := w.fetchEODHDFundamentals(ctx, apiKey, eodhdSymbol, data); err != nil {
		w.logger.Warn().Err(err).Str("ticker", ticker.String()).Msg("Failed to fetch EODHD fundamentals")
		// Continue anyway - we'll try to get historical prices
	}

	// STEP 2: Fetch historical prices
	if err := w.fetchEODHDHistoricalPrices(ctx, apiKey, eodhdSymbol, period, data); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to fetch EODHD historical prices")
	}

	// STEP 3: Calculate technicals from historical data
	w.calculateTechnicals(data)

	// STEP 4: Calculate period performance
	w.calculatePeriodPerformance(data)

	// STEP 5: Generate company blurb via LLM (if available)
	data.CompanyBlurb = w.generateCompanyBlurb(ctx, data)

	// Calculate upside potential
	if data.CurrentPrice > 0 && data.TargetMean > 0 {
		data.UpsidePotential = ((data.TargetMean - data.CurrentPrice) / data.CurrentPrice) * 100
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Float64("price", data.CurrentPrice).
		Str("source", "eodhd").
		Msg("Fetched comprehensive stock data from EODHD")

	return data, nil
}

// generateCompanyBlurb generates a brief description of the company using LLM.
// Returns an empty string if LLM is unavailable or fails (graceful degradation).
func (w *FundamentalsWorker) generateCompanyBlurb(ctx context.Context, data *StockCollectorData) string {
	// Skip if no LLM provider available
	if w.providerFactory == nil {
		w.logger.Debug().
			Str("company", data.CompanyName).
			Msg("LLM provider not available, skipping company blurb generation")
		return ""
	}

	// Skip if we don't have enough data to generate a meaningful blurb
	if data.CompanyName == "" {
		return ""
	}

	// Build prompt for company blurb
	prompt := fmt.Sprintf(`Generate a brief 1-2 sentence description of what %s does and its industry sector.

Company: %s
Sector: %s
Industry: %s

Requirements:
- Keep it factual and concise (1-2 sentences max)
- Focus only on what the company does and its industry
- Do not include financial data, investment advice, or opinions
- Do not mention stock price, market cap, or valuation
- Write in present tense

Response format: Just the description, no preamble or explanation.`,
		data.CompanyName,
		data.CompanyName,
		data.Sector,
		data.Industry,
	)

	request := &llm.ContentRequest{
		Messages: []interfaces.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3, // Low temperature for factual content
		MaxTokens:   150, // Brief response
	}

	response, err := w.providerFactory.GenerateContent(ctx, request)
	if err != nil {
		w.logger.Warn().
			Err(err).
			Str("company", data.CompanyName).
			Msg("Failed to generate company blurb via LLM")
		return ""
	}

	// Clean up response - remove any leading/trailing whitespace or quotes
	blurb := strings.TrimSpace(response.Text)
	blurb = strings.Trim(blurb, "\"")

	w.logger.Debug().
		Str("company", data.CompanyName).
		Str("blurb", blurb).
		Msg("Generated company blurb via LLM")

	return blurb
}

// fetchEODHDFundamentals fetches all fundamental data from EODHD
func (w *FundamentalsWorker) fetchEODHDFundamentals(ctx context.Context, apiKey, symbol string, data *StockCollectorData) error {
	url := fmt.Sprintf("%s/fundamentals/%s?api_token=%s&fmt=json", eodhdAPIBaseURL, symbol, apiKey)

	resp, err := w.makeRequest(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch EODHD fundamentals: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("EODHD fundamentals API returned status %d", resp.StatusCode)
	}

	var fundResp eodhdFundamentalsResponse
	if err := json.NewDecoder(resp.Body).Decode(&fundResp); err != nil {
		return fmt.Errorf("failed to decode EODHD fundamentals: %w", err)
	}

	// Debug logging for EODHD fundamentals data
	w.logger.Debug().
		Str("symbol", symbol).
		Int64("market_cap", fundResp.Highlights.MarketCapitalization).
		Int64("shares_outstanding", fundResp.SharesStats.SharesOutstanding).
		Float64("50_day_ma", fundResp.Technicals.FiftyDayMA).
		Float64("200_day_ma", fundResp.Technicals.TwoHundredDayMA).
		Float64("52_week_high", fundResp.Technicals.FiftyTwoWeekHigh).
		Float64("52_week_low", fundResp.Technicals.FiftyTwoWeekLow).
		Msg("EODHD fundamentals decoded")

	// General info
	data.CompanyName = fundResp.General.Name
	data.Currency = fundResp.General.CurrencyCode
	if data.Currency == "" {
		data.Currency = "AUD"
	}
	data.ISIN = fundResp.General.ISIN
	data.Sector = fundResp.General.Sector
	data.Industry = fundResp.General.Industry

	// Highlights
	data.MarketCap = fundResp.Highlights.MarketCapitalization
	data.PERatio = fundResp.Highlights.PERatio
	data.PEGRatio = fundResp.Highlights.PEGRatio
	data.EPS = fundResp.Highlights.EarningsShare
	data.DividendYield = fundResp.Highlights.DividendYield * 100 // Convert to percentage
	data.BookValue = fundResp.Highlights.BookValue
	data.TargetMean = fundResp.Highlights.WallStreetTargetPrice
	data.ProfitMargin = fundResp.Highlights.ProfitMargin * 100
	data.OperatingMargin = fundResp.Highlights.OperatingMarginTTM * 100
	data.ReturnOnAssets = fundResp.Highlights.ReturnOnAssetsTTM * 100
	data.ReturnOnEquity = fundResp.Highlights.ReturnOnEquityTTM * 100
	data.RevenueGrowthYoY = fundResp.Highlights.QuarterlyRevenueGrowthYOY * 100

	// Valuation
	data.ForwardPE = fundResp.Valuation.ForwardPE
	data.PriceToBook = fundResp.Valuation.PriceBookMRQ
	data.PriceToSales = fundResp.Valuation.PriceSalesTTM
	data.EnterpriseValue = fundResp.Valuation.EnterpriseValue
	data.EVToRevenue = fundResp.Valuation.EnterpriseValueRevenue
	data.EVToEBITDA = fundResp.Valuation.EnterpriseValueEbitda

	// Shares stats
	data.SharesOutstanding = fundResp.SharesStats.SharesOutstanding
	data.SharesFloat = fundResp.SharesStats.SharesFloat
	data.PercentInsiders = fundResp.SharesStats.PercentInsiders
	data.PercentInstitutions = fundResp.SharesStats.PercentInstitutions

	// Technicals from EODHD (52-week range, beta, and SMAs)
	data.Week52High = fundResp.Technicals.FiftyTwoWeekHigh
	data.Week52Low = fundResp.Technicals.FiftyTwoWeekLow
	data.Beta = fundResp.Technicals.Beta
	data.SMA50 = fundResp.Technicals.FiftyDayMA
	data.SMA200 = fundResp.Technicals.TwoHundredDayMA

	// Calculate current price from market cap and shares outstanding
	// This works when EOD endpoint is not available (Fundamental Data subscription only)
	if fundResp.SharesStats.SharesOutstanding > 0 && fundResp.Highlights.MarketCapitalization > 0 {
		data.CurrentPrice = float64(fundResp.Highlights.MarketCapitalization) / float64(fundResp.SharesStats.SharesOutstanding)
	} else if fundResp.Technicals.FiftyDayMA > 0 {
		// Fallback to 50-day MA as price proxy
		data.CurrentPrice = fundResp.Technicals.FiftyDayMA
	}

	// Splits/Dividends
	data.DividendRate = fundResp.SplitsDividends.ForwardAnnualDividendRate
	data.PayoutRatio = fundResp.SplitsDividends.PayoutRatio * 100
	data.DividendExDate = fundResp.SplitsDividends.ExDividendDate
	data.DividendPayDate = fundResp.SplitsDividends.DividendDate

	// Analyst ratings
	data.RecommendationMean = fundResp.AnalystRatings.Rating
	data.TargetMean = fundResp.AnalystRatings.TargetPrice
	data.StrongBuy = fundResp.AnalystRatings.StrongBuy
	data.Buy = fundResp.AnalystRatings.Buy
	data.Hold = fundResp.AnalystRatings.Hold
	data.Sell = fundResp.AnalystRatings.Sell
	data.StrongSell = fundResp.AnalystRatings.StrongSell
	data.AnalystCount = data.StrongBuy + data.Buy + data.Hold + data.Sell + data.StrongSell

	// Determine recommendation key from rating
	if data.RecommendationMean > 0 {
		if data.RecommendationMean <= 1.5 {
			data.RecommendationKey = "strong_buy"
		} else if data.RecommendationMean <= 2.5 {
			data.RecommendationKey = "buy"
		} else if data.RecommendationMean <= 3.5 {
			data.RecommendationKey = "hold"
		} else if data.RecommendationMean <= 4.5 {
			data.RecommendationKey = "sell"
		} else {
			data.RecommendationKey = "strong_sell"
		}
	}

	// ESG Scores
	data.ESGTotalScore = fundResp.ESGScores.TotalEsg
	data.ESGEnvironmentScore = fundResp.ESGScores.EnvironmentScore
	data.ESGSocialScore = fundResp.ESGScores.SocialScore
	data.ESGGovernanceScore = fundResp.ESGScores.GovernanceScore
	data.ESGControversy = fundResp.ESGScores.ControversyLevel

	// Earnings history (last 10 - iterate over map and limit to 10 entries)
	count := 0
	for _, eh := range fundResp.Earnings.History {
		if count >= 10 {
			break
		}
		data.EarningsHistory = append(data.EarningsHistory, EarningsEntry{
			Date:            eh.Date,
			ReportDate:      eh.ReportDate,
			EPSActual:       eh.EpsActual,
			EPSEstimate:     eh.EpsEstimate,
			EPSSurprise:     eh.EpsDifference,
			EPSSurprisePerc: eh.SurprisePercent,
		})
		count++
	}

	// Extract analyst estimates (v2: enhanced forward-looking data)
	data.AnalystEstimates = w.extractAnalystEstimates(&fundResp)

	// Parse financial statements
	w.parseEODHDFinancials(fundResp.Financials, data)

	return nil
}

// parseEODHDFinancials parses EODHD financial statements into annual/quarterly data
func (w *FundamentalsWorker) parseEODHDFinancials(financials eodhdFinancials, data *StockCollectorData) {
	// Get sorted years from income statement
	incomeYears := make([]string, 0, len(financials.IncomeStatement.Yearly))
	for year := range financials.IncomeStatement.Yearly {
		incomeYears = append(incomeYears, year)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(incomeYears)))

	// Log financial data availability for debugging
	w.logger.Debug().
		Int("yearly_income_statements", len(financials.IncomeStatement.Yearly)).
		Int("quarterly_income_statements", len(financials.IncomeStatement.Quarterly)).
		Int("yearly_balance_sheets", len(financials.BalanceSheet.Yearly)).
		Int("yearly_cash_flows", len(financials.CashFlow.Yearly)).
		Msg("EODHD financial statements availability")

	// Helper function to extract numeric value from interface{}
	// EODHD API may return values as float64, string, or nil
	extractNumber := func(data map[string]interface{}, key string) int64 {
		if data == nil {
			return 0
		}
		val, exists := data[key]
		if !exists || val == nil {
			return 0
		}
		switch v := val.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			// Try to parse string as number
			if v == "" || v == "None" || v == "null" {
				return 0
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return 0
			}
			return int64(f)
		default:
			return 0
		}
	}

	// Process yearly financial data
	for _, year := range incomeYears {
		incomeData := financials.IncomeStatement.Yearly[year]
		balanceData := financials.BalanceSheet.Yearly[year]
		cashflowData := financials.CashFlow.Yearly[year]

		entry := FinancialPeriodEntry{
			EndDate:    year,
			PeriodType: "annual",
		}

		// Income statement - use extractNumber for robust type handling
		entry.TotalRevenue = extractNumber(incomeData, "totalRevenue")
		entry.GrossProfit = extractNumber(incomeData, "grossProfit")
		entry.OperatingIncome = extractNumber(incomeData, "operatingIncome")
		entry.NetIncome = extractNumber(incomeData, "netIncome")
		entry.EBITDA = extractNumber(incomeData, "ebitda")

		// Balance sheet
		entry.TotalAssets = extractNumber(balanceData, "totalAssets")
		entry.TotalLiab = extractNumber(balanceData, "totalLiab")
		entry.TotalEquity = extractNumber(balanceData, "totalStockholderEquity")

		// Balance sheet - debt and cash components (Step 1: Enhanced data extraction)
		entry.CashAndEquivalents = extractNumber(balanceData, "cash")
		if entry.CashAndEquivalents == 0 {
			// Fallback to cashAndShortTermInvestments if cash not available
			entry.CashAndEquivalents = extractNumber(balanceData, "cashAndShortTermInvestments")
		}
		entry.ShortTermDebt = extractNumber(balanceData, "shortTermDebt")
		entry.LongTermDebt = extractNumber(balanceData, "longTermDebt")
		entry.TotalDebt = entry.ShortTermDebt + entry.LongTermDebt
		entry.NetDebt = entry.TotalDebt - entry.CashAndEquivalents
		entry.SharesOutstanding = extractNumber(balanceData, "commonStockSharesOutstanding")

		// Cash flow
		entry.OperatingCF = extractNumber(cashflowData, "totalCashFromOperatingActivities")
		entry.FreeCF = extractNumber(cashflowData, "freeCashFlow")

		// Calculate margins
		if entry.TotalRevenue > 0 {
			entry.GrossMargin = float64(entry.GrossProfit) / float64(entry.TotalRevenue) * 100
			entry.NetMargin = float64(entry.NetIncome) / float64(entry.TotalRevenue) * 100
		}

		data.AnnualData = append(data.AnnualData, entry)
	}

	// Process quarterly financial data - limit to 20 quarters (5 years)
	quarterKeys := make([]string, 0, len(financials.IncomeStatement.Quarterly))
	for qtr := range financials.IncomeStatement.Quarterly {
		quarterKeys = append(quarterKeys, qtr)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(quarterKeys)))

	for i, qtr := range quarterKeys {
		if i >= 20 { // Limit to last 20 quarters (5 years)
			break
		}
		incomeData := financials.IncomeStatement.Quarterly[qtr]

		entry := FinancialPeriodEntry{
			EndDate:    qtr,
			PeriodType: "quarterly",
		}

		// Use extractNumber for robust type handling
		entry.TotalRevenue = extractNumber(incomeData, "totalRevenue")
		entry.GrossProfit = extractNumber(incomeData, "grossProfit")
		entry.OperatingIncome = extractNumber(incomeData, "operatingIncome")
		entry.NetIncome = extractNumber(incomeData, "netIncome")

		if entry.TotalRevenue > 0 {
			entry.GrossMargin = float64(entry.GrossProfit) / float64(entry.TotalRevenue) * 100
			entry.NetMargin = float64(entry.NetIncome) / float64(entry.TotalRevenue) * 100
		}

		data.QuarterlyData = append(data.QuarterlyData, entry)
	}

	// Calculate CAGR from annual data
	data.RevenueCAGR3Y = w.calculateRevenueCAGR(data.AnnualData, 3)
	data.RevenueCAGR5Y = w.calculateRevenueCAGR(data.AnnualData, 5)

	// Calculate profit growth YoY
	if len(data.AnnualData) >= 2 {
		currentIncome := data.AnnualData[0].NetIncome
		prevIncome := data.AnnualData[1].NetIncome
		if prevIncome > 0 {
			data.ProfitGrowthYoY = float64(currentIncome-prevIncome) / float64(prevIncome) * 100
		}
	}

	// Step 2: Calculate financial health metrics from latest annual data
	w.calculateFinancialHealthMetrics(data)

	// Step 3: Calculate shares dilution tracking
	w.calculateSharesDilution(data)
}

// calculateFinancialHealthMetrics calculates summary-level financial health metrics
// from the latest annual data. These metrics support the Kneppy framework pillars:
// - Financial Robustness (Net Debt/EBITDA)
// - Cash Flow Reality (FCF Conversion)
func (w *FundamentalsWorker) calculateFinancialHealthMetrics(data *StockCollectorData) {
	if len(data.AnnualData) == 0 {
		return
	}
	latest := data.AnnualData[0]

	// Cash and Debt from balance sheet
	data.LatestCash = latest.CashAndEquivalents
	data.LatestTotalDebt = latest.TotalDebt
	data.LatestNetDebt = latest.NetDebt

	// Net Debt / EBITDA (key leverage ratio for Financial Robustness pillar)
	// Target: <2.0x is healthy
	if latest.EBITDA > 0 {
		data.NetDebtToEBITDA = float64(latest.NetDebt) / float64(latest.EBITDA)
	}

	// Cash Flow Metrics
	data.LatestOperatingCF = latest.OperatingCF
	data.LatestFreeCF = latest.FreeCF

	// FCF Conversion = FCF / Net Income (Cash Flow Reality pillar)
	// Target: >90% indicates high-quality earnings
	if latest.NetIncome > 0 {
		data.FCFConversion = float64(latest.FreeCF) / float64(latest.NetIncome) * 100
	} else if latest.NetIncome < 0 && latest.FreeCF > 0 {
		// Company is loss-making but generating positive FCF - still report it
		data.FCFConversion = 0 // Cannot calculate meaningful ratio with negative denominator
	}

	// FCF to Revenue (FCF Margin)
	if latest.TotalRevenue > 0 {
		data.FCFToRevenue = float64(latest.FreeCF) / float64(latest.TotalRevenue) * 100
	}

	// Gross Margin from latest annual data
	if latest.GrossMargin > 0 {
		data.GrossMargin = latest.GrossMargin
	}
}

// calculateSharesDilution calculates share dilution metrics from historical balance sheet data.
// This supports the Share Allocation pillar of the Kneppy framework.
// Target: <=0% CAGR (no dilution or buybacks)
func (w *FundamentalsWorker) calculateSharesDilution(data *StockCollectorData) {
	if len(data.AnnualData) == 0 {
		return
	}

	// Build shares history from annual data
	var sharesHistory []SharesHistoryEntry
	for _, annual := range data.AnnualData {
		if annual.SharesOutstanding > 0 {
			sharesHistory = append(sharesHistory, SharesHistoryEntry{
				Date:              annual.EndDate,
				SharesOutstanding: annual.SharesOutstanding,
			})
		}
	}

	// Calculate year-over-year change percentages
	for i := 0; i < len(sharesHistory)-1; i++ {
		current := sharesHistory[i].SharesOutstanding
		previous := sharesHistory[i+1].SharesOutstanding
		if previous > 0 {
			sharesHistory[i].ChangePercent = (float64(current) - float64(previous)) / float64(previous) * 100
		}
	}

	data.SharesHistory = sharesHistory

	// Calculate 3-year CAGR if we have enough data
	if len(sharesHistory) >= 4 { // Need at least 4 data points for 3-year CAGR
		latestShares := float64(sharesHistory[0].SharesOutstanding)
		threeYearsAgoShares := float64(sharesHistory[3].SharesOutstanding)
		if threeYearsAgoShares > 0 && latestShares > 0 {
			data.SharesCAGR3Y = (math.Pow(latestShares/threeYearsAgoShares, 1.0/3.0) - 1) * 100
		}
	}

	// YoY change (most recent vs prior year)
	if len(sharesHistory) >= 2 {
		latest := float64(sharesHistory[0].SharesOutstanding)
		previous := float64(sharesHistory[1].SharesOutstanding)
		if previous > 0 {
			data.SharesGrowthYoY = ((latest - previous) / previous) * 100
		}
	}
}

// extractAnalystEstimates extracts forward-looking analyst estimates from EODHD fundamentals
// This provides data for the v2 analyst coverage enhancement
func (w *FundamentalsWorker) extractAnalystEstimates(fundResp *eodhdFundamentalsResponse) *AnalystEstimatesData {
	est := &AnalystEstimatesData{}

	// Helper to convert interface{} to float64
	toFloat64 := func(v interface{}) float64 {
		if v == nil {
			return 0
		}
		switch val := v.(type) {
		case float64:
			return val
		case int64:
			return float64(val)
		case int:
			return float64(val)
		case string:
			if val == "" || val == "None" || val == "null" {
				return 0
			}
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return 0
			}
			return f
		default:
			return 0
		}
	}

	// Helper to convert interface{} to int
	toInt := func(v interface{}) int {
		if v == nil {
			return 0
		}
		switch val := v.(type) {
		case float64:
			return int(val)
		case int64:
			return int(val)
		case int:
			return val
		case string:
			if val == "" || val == "None" || val == "null" {
				return 0
			}
			i, err := strconv.Atoi(val)
			if err != nil {
				f, err := strconv.ParseFloat(val, 64)
				if err != nil {
					return 0
				}
				return int(f)
			}
			return i
		default:
			return 0
		}
	}

	// From Highlights: EPS estimates
	est.EPSCurrentYear = fundResp.Highlights.EPSEstimateCurrentYear
	est.EPSNextYear = fundResp.Highlights.EPSEstimateNextYear
	est.EPSCurrentQtr = fundResp.Highlights.EPSEstimateCurrentQuarter
	est.EPSNextQtr = fundResp.Highlights.EPSEstimateNextQuarter

	// From Earnings.Trend: detailed estimates and revisions
	// Find the current year ("0y") period for estimates
	for _, trend := range fundResp.Earnings.Trend {
		if trend.Period == "0y" {
			est.EPSEstimateAvg = toFloat64(trend.EarningsEstimateAvg)
			est.EPSEstimateLow = toFloat64(trend.EarningsEstimateLow)
			est.EPSEstimateHigh = toFloat64(trend.EarningsEstimateHigh)
			est.EarningsAnalystCount = toInt(trend.EarningsEstimateNumberOfAnalysts)

			est.RevenueEstimateAvg = toFloat64(trend.RevenueEstimateAvg)
			est.RevenueEstimateLow = toFloat64(trend.RevenueEstimateLow)
			est.RevenueEstimateHigh = toFloat64(trend.RevenueEstimateHigh)
			est.RevenueAnalystCount = toInt(trend.RevenueEstimateNumberOfAnalysts)

			// EPS trend data (estimate momentum)
			est.EPSTrendCurrent = toFloat64(trend.EPSTrendCurrent)
			est.EPSTrend7DaysAgo = toFloat64(trend.EPSTrend7DaysAgo)
			est.EPSTrend30DaysAgo = toFloat64(trend.EPSTrend30DaysAgo)
			est.EPSTrend60DaysAgo = toFloat64(trend.EPSTrend60DaysAgo)
			est.EPSTrend90DaysAgo = toFloat64(trend.EPSTrend90DaysAgo)

			// Calculate 30-day trend change
			if est.EPSTrend30DaysAgo != 0 && est.EPSTrendCurrent != 0 {
				est.EPSTrendChange30D = ((est.EPSTrendCurrent - est.EPSTrend30DaysAgo) / math.Abs(est.EPSTrend30DaysAgo)) * 100
			}

			// Revision counts
			est.RevisionsUp7Days = toInt(trend.EPSRevisionsUpLast7Days)
			est.RevisionsUp30Days = toInt(trend.EPSRevisionsUpLast30Days)
			est.RevisionsDown30Days = toInt(trend.EPSRevisionsDownLast30Days)

			est.EstimatePeriod = "0y"
			break
		}
	}

	// Determine coverage quality
	maxAnalysts := est.EarningsAnalystCount
	if est.RevenueAnalystCount > maxAnalysts {
		maxAnalysts = est.RevenueAnalystCount
	}

	switch {
	case maxAnalysts >= 5:
		est.CoverageQuality = "HIGH"
	case maxAnalysts >= 1:
		est.CoverageQuality = "LOW"
	default:
		est.CoverageQuality = "NONE"
	}

	// Only return if there's meaningful data
	if est.EPSCurrentYear == 0 && est.EPSNextYear == 0 && est.EarningsAnalystCount == 0 && est.RevenueAnalystCount == 0 {
		// Still return with coverage quality NONE to indicate no data
		est.CoverageQuality = "NONE"
	}

	return est
}

// fetchEODHDHistoricalPrices fetches historical OHLCV data from EODHD
func (w *FundamentalsWorker) fetchEODHDHistoricalPrices(ctx context.Context, apiKey, symbol, period string, data *StockCollectorData) error {
	now := time.Now()
	var dateFrom time.Time

	switch period {
	case "M1":
		dateFrom = now.AddDate(0, -1, 0)
	case "M3":
		dateFrom = now.AddDate(0, -3, 0)
	case "M6":
		dateFrom = now.AddDate(0, -6, 0)
	case "Y1":
		dateFrom = now.AddDate(-1, 0, 0)
	case "Y2":
		dateFrom = now.AddDate(-2, 0, 0)
	case "Y5":
		dateFrom = now.AddDate(-5, 0, 0)
	case "Y10":
		dateFrom = now.AddDate(-10, 0, 0)
	default:
		dateFrom = now.AddDate(-1, 0, 0)
	}

	url := fmt.Sprintf("%s/eod/%s?api_token=%s&from=%s&to=%s&fmt=json",
		eodhdAPIBaseURL, symbol, apiKey,
		dateFrom.Format("2006-01-02"), now.Format("2006-01-02"))

	resp, err := w.makeRequest(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch EODHD historical prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("EODHD EOD API returned status %d", resp.StatusCode)
	}

	var eodData []eodhdEODData
	if err := json.NewDecoder(resp.Body).Decode(&eodData); err != nil {
		return fmt.Errorf("failed to decode EODHD EOD response: %w", err)
	}

	if len(eodData) == 0 {
		return fmt.Errorf("no historical data returned from EODHD")
	}

	var prevClose float64
	for _, eod := range eodData {
		entry := OHLCVEntry{
			Date:   eod.Date,
			Open:   eod.Open,
			High:   eod.High,
			Low:    eod.Low,
			Close:  eod.Close,
			Volume: eod.Volume,
		}

		// Calculate daily change
		if prevClose > 0 {
			entry.ChangeValue = entry.Close - prevClose
			entry.ChangePercent = (entry.ChangeValue / prevClose) * 100
		}
		prevClose = entry.Close

		data.HistoricalPrices = append(data.HistoricalPrices, entry)
	}

	// Set current price from latest EOD data
	if len(eodData) > 0 {
		latest := eodData[len(eodData)-1]
		data.CurrentPrice = latest.Close
		data.DayLow = latest.Low
		data.DayHigh = latest.High
		data.Volume = latest.Volume

		// Calculate change from previous day
		if len(eodData) > 1 {
			prevDay := eodData[len(eodData)-2]
			data.PriceChange = latest.Close - prevDay.Close
			if prevDay.Close > 0 {
				data.ChangePercent = (data.PriceChange / prevDay.Close) * 100
			}
		}
	}

	return nil
}

// calculateRevenueCAGR calculates Compound Annual Growth Rate for revenue
func (w *FundamentalsWorker) calculateRevenueCAGR(annualData []FinancialPeriodEntry, years int) float64 {
	if len(annualData) < years+1 {
		return 0
	}

	endValue := float64(annualData[0].TotalRevenue)
	startValue := float64(annualData[years].TotalRevenue)

	if startValue <= 0 || endValue <= 0 {
		return 0
	}

	// CAGR = (End/Start)^(1/years) - 1
	return (math.Pow(endValue/startValue, 1.0/float64(years)) - 1) * 100
}

// calculateTechnicals calculates technical indicators from historical data
// If no historical data available, uses SMA50/SMA200 from fundamentals (if set)
func (w *FundamentalsWorker) calculateTechnicals(data *StockCollectorData) {
	if len(data.HistoricalPrices) == 0 {
		// No historical data - determine trend from fundamentals-provided SMAs
		if data.CurrentPrice > 0 && data.SMA50 > 0 {
			data.TrendSignal = w.determineTrend(data.CurrentPrice, 0, data.SMA50, data.SMA200, 50)
		}
		return
	}

	closes := make([]float64, len(data.HistoricalPrices))
	for i, p := range data.HistoricalPrices {
		closes[i] = p.Close
	}

	// Calculate SMAs from historical data (override fundamentals values for accuracy)
	data.SMA20 = w.calculateSMA(closes, 20)
	if data.SMA50 == 0 {
		data.SMA50 = w.calculateSMA(closes, 50)
	}
	if data.SMA200 == 0 {
		data.SMA200 = w.calculateSMA(closes, 200)
	}

	// Calculate RSI
	data.RSI14 = w.calculateRSI(closes, 14)

	// Calculate support and resistance
	recentPrices := data.HistoricalPrices
	if len(data.HistoricalPrices) > 20 {
		recentPrices = data.HistoricalPrices[len(data.HistoricalPrices)-20:]
	}

	var highs, lows []float64
	for _, p := range recentPrices {
		highs = append(highs, p.High)
		lows = append(lows, p.Low)
	}

	if len(lows) > 0 {
		data.Support = w.findMin(lows)
	}
	if len(highs) > 0 {
		data.Resistance = w.findMax(highs)
	}

	// Determine trend signal
	currentPrice := data.CurrentPrice
	if currentPrice == 0 && len(closes) > 0 {
		currentPrice = closes[len(closes)-1]
	}

	data.TrendSignal = w.determineTrend(currentPrice, data.SMA20, data.SMA50, data.SMA200, data.RSI14)
}

// calculateSMA calculates Simple Moving Average
func (w *FundamentalsWorker) calculateSMA(prices []float64, period int) float64 {
	if len(prices) < period {
		if len(prices) == 0 {
			return 0
		}
		period = len(prices)
	}

	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}
	return sum / float64(period)
}

// calculateRSI calculates Relative Strength Index
func (w *FundamentalsWorker) calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50 // Neutral if not enough data
	}

	gains := 0.0
	losses := 0.0

	for i := len(prices) - period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	if losses == 0 {
		return 100
	}

	rs := gains / losses
	return 100 - (100 / (1 + rs))
}

// findMin finds minimum value
func (w *FundamentalsWorker) findMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values {
		if v < min && v > 0 {
			min = v
		}
	}
	return min
}

// findMax finds maximum value
func (w *FundamentalsWorker) findMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

// determineTrend determines the overall trend signal
func (w *FundamentalsWorker) determineTrend(price, sma20, sma50, sma200, rsi float64) string {
	bullishSignals := 0
	bearishSignals := 0

	// Price vs SMAs
	if price > sma20 && sma20 > 0 {
		bullishSignals++
	} else if sma20 > 0 {
		bearishSignals++
	}

	if price > sma50 && sma50 > 0 {
		bullishSignals++
	} else if sma50 > 0 {
		bearishSignals++
	}

	if price > sma200 && sma200 > 0 {
		bullishSignals++
	} else if sma200 > 0 {
		bearishSignals++
	}

	// SMA alignment
	if sma20 > sma50 && sma50 > 0 {
		bullishSignals++
	} else if sma50 > 0 {
		bearishSignals++
	}

	// RSI
	if rsi > 50 && rsi < 70 {
		bullishSignals++
	} else if rsi < 50 && rsi > 30 {
		bearishSignals++
	} else if rsi >= 70 {
		bearishSignals++ // Overbought
	} else if rsi <= 30 {
		bullishSignals++ // Oversold - potential reversal
	}

	if bullishSignals > bearishSignals+1 {
		return "BULLISH"
	} else if bearishSignals > bullishSignals+1 {
		return "BEARISH"
	}
	return "NEUTRAL"
}

// calculatePeriodPerformance calculates price changes over various periods
func (w *FundamentalsWorker) calculatePeriodPerformance(data *StockCollectorData) {
	if len(data.HistoricalPrices) == 0 || data.CurrentPrice == 0 {
		return
	}

	now := time.Now()
	periods := []struct {
		name string
		days int
	}{
		{"7D", 7},
		{"1M", 30},
		{"3M", 91},
		{"6M", 183},
		{"1Y", 365},
		{"2Y", 730},
	}

	for _, period := range periods {
		targetDate := now.AddDate(0, 0, -period.days)
		var closestPrice float64
		minDiff := time.Duration(math.MaxInt64)

		for _, p := range data.HistoricalPrices {
			pDate, _ := time.Parse("2006-01-02", p.Date)
			diff := pDate.Sub(targetDate)
			if diff < 0 {
				diff = -diff
			}
			if diff < minDiff {
				minDiff = diff
				closestPrice = p.Close
			}
		}

		if closestPrice > 0 {
			changeValue := data.CurrentPrice - closestPrice
			changePercent := (changeValue / closestPrice) * 100
			shares1k := int(1000 / closestPrice)
			value1k := float64(shares1k) * data.CurrentPrice

			data.PeriodPerformance = append(data.PeriodPerformance, PeriodPerformanceEntry{
				Period:        period.name,
				Days:          period.days,
				Price:         closestPrice,
				ChangeValue:   changeValue,
				ChangePercent: changePercent,
				Shares1k:      shares1k,
				Value1k:       value1k,
			})
		}
	}
}

// createDocument creates a document from stock collector data
func (w *FundamentalsWorker) createDocument(ctx context.Context, data *StockCollectorData, ticker common.Ticker, jobDef *models.JobDefinition, parentJobID string, outputTags []string, debug *workerutil.WorkerDebugInfo) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s Comprehensive Stock Data - %s\n\n", ticker.String(), data.CompanyName))
	content.WriteString(fmt.Sprintf("**Last Updated**: %s\n", data.LastUpdated.Format("2 Jan 2006 3:04 PM AEST")))
	content.WriteString(fmt.Sprintf("**Currency**: %s\n", data.Currency))
	content.WriteString(fmt.Sprintf("**Worker**: %s\n", models.WorkerTypeMarketFundamentals))
	content.WriteString("\n")
	if data.CompanyBlurb != "" {
		content.WriteString("## About\n\n")
		content.WriteString(fmt.Sprintf("%s\n\n", data.CompanyBlurb))
	}

	// Current Price Section
	content.WriteString("## Current Price\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| **Last Price** | **$%.2f** |\n", data.CurrentPrice))
	content.WriteString(fmt.Sprintf("| Change | $%.2f (%.2f%%) |\n", data.PriceChange, data.ChangePercent))
	content.WriteString(fmt.Sprintf("| Day Range | $%.2f - $%.2f |\n", data.DayLow, data.DayHigh))
	content.WriteString(fmt.Sprintf("| 52-Week Range | $%.2f - $%.2f |\n", data.Week52Low, data.Week52High))
	content.WriteString(fmt.Sprintf("| Volume | %s |\n", w.formatNumber(data.Volume)))
	content.WriteString(fmt.Sprintf("| Market Cap | $%s |\n\n", w.formatLargeNumber(data.MarketCap)))

	// Valuation Section
	content.WriteString("## Valuation\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| P/E Ratio | %.2f |\n", data.PERatio))
	content.WriteString(fmt.Sprintf("| Forward P/E | %.2f |\n", data.ForwardPE))
	content.WriteString(fmt.Sprintf("| PEG Ratio | %.2f |\n", data.PEGRatio))
	content.WriteString(fmt.Sprintf("| EPS | $%.2f |\n", data.EPS))
	content.WriteString(fmt.Sprintf("| Dividend Yield | %.2f%% |\n", data.DividendYield))
	content.WriteString(fmt.Sprintf("| Price/Book | %.2f |\n", data.PriceToBook))
	content.WriteString(fmt.Sprintf("| Price/Sales | %.2f |\n", data.PriceToSales))
	content.WriteString(fmt.Sprintf("| EV/EBITDA | %.2f |\n", data.EVToEBITDA))
	content.WriteString(fmt.Sprintf("| Beta | %.2f |\n\n", data.Beta))

	// ESG Scores Section (if available)
	if data.ESGTotalScore > 0 {
		content.WriteString("## ESG Scores\n\n")
		content.WriteString("| Category | Score |\n")
		content.WriteString("|----------|-------|\n")
		content.WriteString(fmt.Sprintf("| Total ESG | %.1f |\n", data.ESGTotalScore))
		content.WriteString(fmt.Sprintf("| Environment | %.1f |\n", data.ESGEnvironmentScore))
		content.WriteString(fmt.Sprintf("| Social | %.1f |\n", data.ESGSocialScore))
		content.WriteString(fmt.Sprintf("| Governance | %.1f |\n", data.ESGGovernanceScore))
		content.WriteString(fmt.Sprintf("| Controversy Level | %d |\n\n", data.ESGControversy))
	}

	// Step 4: Financial Health Section (Kneppy Framework: Financial Robustness Pillar)
	w.writeFinancialHealthSection(&content, data)

	// Profitability Section
	content.WriteString("## Profitability\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Profit Margin | %.2f%% |\n", data.ProfitMargin))
	content.WriteString(fmt.Sprintf("| Operating Margin | %.2f%% |\n", data.OperatingMargin))
	content.WriteString(fmt.Sprintf("| Return on Assets | %.2f%% |\n", data.ReturnOnAssets))
	content.WriteString(fmt.Sprintf("| Return on Equity | %.2f%% |\n\n", data.ReturnOnEquity))

	// Ownership Section
	if data.SharesOutstanding > 0 {
		content.WriteString("## Ownership\n\n")
		content.WriteString("| Metric | Value |\n")
		content.WriteString("|--------|-------|\n")
		content.WriteString(fmt.Sprintf("| Shares Outstanding | %s |\n", w.formatLargeNumber(data.SharesOutstanding)))
		content.WriteString(fmt.Sprintf("| Float | %s |\n", w.formatLargeNumber(data.SharesFloat)))
		content.WriteString(fmt.Sprintf("| Insider Ownership | %.2f%% |\n", data.PercentInsiders))
		content.WriteString(fmt.Sprintf("| Institutional | %.2f%% |\n\n", data.PercentInstitutions))
	}

	// Technical Analysis Section
	content.WriteString("## Technical Analysis\n\n")
	content.WriteString(fmt.Sprintf("### Trend Signal: **%s**\n\n", data.TrendSignal))
	content.WriteString("| Indicator | Value |\n")
	content.WriteString("|-----------|-------|\n")
	content.WriteString(fmt.Sprintf("| SMA 20 | $%.2f |\n", data.SMA20))
	content.WriteString(fmt.Sprintf("| SMA 50 | $%.2f |\n", data.SMA50))
	content.WriteString(fmt.Sprintf("| SMA 200 | $%.2f |\n", data.SMA200))
	content.WriteString(fmt.Sprintf("| RSI (14) | %.1f |\n", data.RSI14))
	content.WriteString(fmt.Sprintf("| Support | $%.2f |\n", data.Support))
	content.WriteString(fmt.Sprintf("| Resistance | $%.2f |\n\n", data.Resistance))

	// Period Performance Section
	if len(data.PeriodPerformance) > 0 {
		content.WriteString("## Period Performance\n\n")
		content.WriteString("| Period | Price | Change | % Change |\n")
		content.WriteString("|--------|-------|--------|----------|\n")
		for _, p := range data.PeriodPerformance {
			sign := ""
			if p.ChangePercent > 0 {
				sign = "+"
			}
			content.WriteString(fmt.Sprintf("| %s | $%.2f | %s$%.2f | %s%.2f%% |\n",
				p.Period, p.Price, sign, p.ChangeValue, sign, p.ChangePercent))
		}
		content.WriteString("\n")
	}

	// Analyst Coverage Section
	content.WriteString("## Analyst Coverage\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Analyst Count | %d |\n", data.AnalystCount))
	content.WriteString(fmt.Sprintf("| Consensus | %s |\n", strings.ToUpper(data.RecommendationKey)))
	content.WriteString(fmt.Sprintf("| Rating Score | %.2f |\n", data.RecommendationMean))
	content.WriteString(fmt.Sprintf("| Mean Target | $%.2f |\n", data.TargetMean))
	content.WriteString(fmt.Sprintf("| Target Range | $%.2f - $%.2f |\n", data.TargetLow, data.TargetHigh))
	content.WriteString(fmt.Sprintf("| Upside Potential | %.1f%% |\n\n", data.UpsidePotential))

	// Recommendation Distribution
	content.WriteString("### Recommendation Distribution\n\n")
	content.WriteString("| Rating | Count |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Strong Buy | %d |\n", data.StrongBuy))
	content.WriteString(fmt.Sprintf("| Buy | %d |\n", data.Buy))
	content.WriteString(fmt.Sprintf("| Hold | %d |\n", data.Hold))
	content.WriteString(fmt.Sprintf("| Sell | %d |\n", data.Sell))
	content.WriteString(fmt.Sprintf("| Strong Sell | %d |\n\n", data.StrongSell))

	// Enhanced Analyst Estimates (v2)
	w.writeAnalystEstimatesSection(&content, data)

	// Financial Growth Section
	content.WriteString("## Financial Growth\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Revenue YoY | %.1f%% |\n", data.RevenueGrowthYoY))
	content.WriteString(fmt.Sprintf("| Profit YoY | %.1f%% |\n", data.ProfitGrowthYoY))
	content.WriteString(fmt.Sprintf("| Revenue 3Y CAGR | %.1f%% |\n", data.RevenueCAGR3Y))
	content.WriteString(fmt.Sprintf("| Revenue 5Y CAGR | %.1f%% |\n\n", data.RevenueCAGR5Y))

	// YoY Financial Performance Table (annual data with derived ratios)
	w.writeFinancialPerformanceTable(&content, data)

	// Historical Prices (last 20 entries for readability, full data in metadata)
	if len(data.HistoricalPrices) > 0 {
		content.WriteString("## Historical Prices (Last 24 Months)\n\n")
		content.WriteString("| Date | Open | High | Low | Close | Volume |\n")
		content.WriteString("|------|------|------|-----|-------|--------|\n")

		// Show most recent 20 entries (reverse order - newest first)
		startIdx := 0
		if len(data.HistoricalPrices) > 20 {
			startIdx = len(data.HistoricalPrices) - 20
		}
		for i := len(data.HistoricalPrices) - 1; i >= startIdx; i-- {
			p := data.HistoricalPrices[i]
			content.WriteString(fmt.Sprintf("| %s | $%.2f | $%.2f | $%.2f | $%.2f | %s |\n",
				p.Date, p.Open, p.High, p.Low, p.Close, w.formatNumber(p.Volume)))
		}
		if len(data.HistoricalPrices) > 20 {
			content.WriteString(fmt.Sprintf("\n*Showing 20 of %d trading days. Full data available in metadata.*\n", len(data.HistoricalPrices)))
		}
		content.WriteString("\n")
	}

	// Build tags - include both exchange-qualified ticker and just the code for backwards compatibility
	tags := []string{"asx-stock-data", strings.ToLower(ticker.Code), strings.ToLower(ticker.String())}
	tags = append(tags, fmt.Sprintf("date:%s", time.Now().Format("2006-01-02")))

	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}
	tags = append(tags, outputTags...)

	// Add cache tags from context
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	// Build metadata - full structured data
	metadata := map[string]interface{}{
		"symbol":              data.Symbol, // For schema compliance
		"ticker":              ticker.String(),
		"asx_code":            ticker.Code, // Keep for backwards compatibility
		"exchange":            ticker.Exchange,
		"company_name":        data.CompanyName,
		"company_blurb":       data.CompanyBlurb,
		"currency":            data.Currency,
		"current_price":       data.CurrentPrice,
		"price_change":        data.PriceChange,
		"change_percent":      data.ChangePercent,
		"day_low":             data.DayLow,
		"day_high":            data.DayHigh,
		"week52_low":          data.Week52Low,
		"week52_high":         data.Week52High,
		"volume":              data.Volume,
		"avg_volume":          data.AvgVolume,
		"market_cap":          data.MarketCap,
		"pe_ratio":            data.PERatio,
		"eps":                 data.EPS,
		"dividend_yield":      data.DividendYield,
		"sma20":               data.SMA20,
		"sma50":               data.SMA50,
		"sma200":              data.SMA200,
		"rsi14":               data.RSI14,
		"support":             data.Support,
		"resistance":          data.Resistance,
		"trend_signal":        data.TrendSignal,
		"analyst_count":       data.AnalystCount,
		"target_mean":         data.TargetMean,
		"target_high":         data.TargetHigh,
		"target_low":          data.TargetLow,
		"target_median":       data.TargetMedian,
		"upside_potential":    data.UpsidePotential,
		"recommendation_mean": data.RecommendationMean,
		"recommendation_key":  data.RecommendationKey,
		"strong_buy":          data.StrongBuy,
		"buy":                 data.Buy,
		"hold":                data.Hold,
		"sell":                data.Sell,
		"strong_sell":         data.StrongSell,
		"revenue_growth_yoy":  data.RevenueGrowthYoY,
		"profit_growth_yoy":   data.ProfitGrowthYoY,
		"revenue_cagr_3y":     data.RevenueCAGR3Y,
		"revenue_cagr_5y":     data.RevenueCAGR5Y,
		"parent_job_id":       parentJobID,
		// Extended fields from EODHD
		"isin":                  data.ISIN,
		"sector":                data.Sector,
		"industry":              data.Industry,
		"forward_pe":            data.ForwardPE,
		"peg_ratio":             data.PEGRatio,
		"book_value":            data.BookValue,
		"price_to_book":         data.PriceToBook,
		"price_to_sales":        data.PriceToSales,
		"enterprise_value":      data.EnterpriseValue,
		"ev_to_revenue":         data.EVToRevenue,
		"ev_to_ebitda":          data.EVToEBITDA,
		"beta":                  data.Beta,
		"profit_margin":         data.ProfitMargin,
		"operating_margin":      data.OperatingMargin,
		"return_on_assets":      data.ReturnOnAssets,
		"return_on_equity":      data.ReturnOnEquity,
		"shares_outstanding":    data.SharesOutstanding,
		"shares_float":          data.SharesFloat,
		"percent_insiders":      data.PercentInsiders,
		"percent_institutions":  data.PercentInstitutions,
		"esg_total_score":       data.ESGTotalScore,
		"esg_environment_score": data.ESGEnvironmentScore,
		"esg_social_score":      data.ESGSocialScore,
		"esg_governance_score":  data.ESGGovernanceScore,
		"esg_controversy":       data.ESGControversy,
		"dividend_rate":         data.DividendRate,
		"dividend_ex_date":      data.DividendExDate,
		"dividend_pay_date":     data.DividendPayDate,
		"payout_ratio":          data.PayoutRatio,
	}

	// Add structured arrays to metadata
	if len(data.EarningsHistory) > 0 {
		metadata["earnings_history"] = data.EarningsHistory
	}
	if len(data.UpgradeDowngrades) > 0 {
		metadata["upgrade_downgrades"] = data.UpgradeDowngrades
	}
	// Annual and quarterly financial data from EODHD - used by market_announcements worker
	if len(data.AnnualData) > 0 {
		metadata["annual_data"] = data.AnnualData
	}
	if len(data.QuarterlyData) > 0 {
		metadata["quarterly_data"] = data.QuarterlyData
	}
	if len(data.HistoricalPrices) > 0 {
		metadata["historical_prices"] = data.HistoricalPrices
	}
	if len(data.PeriodPerformance) > 0 {
		metadata["period_performance"] = data.PeriodPerformance
	}
	if len(data.SharesHistory) > 0 {
		metadata["shares_history"] = data.SharesHistory
	}

	// Step 5: Add Financial Health metrics to metadata
	metadata["latest_cash"] = data.LatestCash
	metadata["latest_total_debt"] = data.LatestTotalDebt
	metadata["latest_net_debt"] = data.LatestNetDebt
	metadata["net_debt_to_ebitda"] = data.NetDebtToEBITDA
	metadata["latest_operating_cf"] = data.LatestOperatingCF
	metadata["latest_free_cf"] = data.LatestFreeCF
	metadata["fcf_conversion"] = data.FCFConversion
	metadata["fcf_to_revenue"] = data.FCFToRevenue
	metadata["shares_cagr_3y"] = data.SharesCAGR3Y
	metadata["shares_growth_yoy"] = data.SharesGrowthYoY

	// v2: Add analyst estimates to metadata
	if data.AnalystEstimates != nil {
		metadata["analyst_estimates"] = data.AnalystEstimates
	}

	// Step 5: Add data completeness tracking for Kneppy framework pillars
	metadata["data_completeness"] = w.buildDataCompletenessMetadata(data)

	// Generate document ID early so it can be included in debug info
	docID := "doc_" + uuid.New().String()

	// Add worker debug metadata if enabled
	if debug != nil {
		debug.SetDocumentID(docID) // Include document ID in debug output
		debug.Complete()
		if debugMeta := debug.ToMetadata(); debugMeta != nil {
			metadata["worker_debug"] = debugMeta
		}
		// Append debug markdown to output
		if debugMd := debug.ToMarkdown(); debugMd != "" {
			content.WriteString(debugMd)
		}
	}

	now := time.Now()
	doc := &models.Document{
		ID:              docID,
		SourceType:      "market_fundamentals",
		SourceID:        ticker.SourceID("stock_collector"),
		URL:             fmt.Sprintf("https://eodhd.com/financial-summary/%s", ticker.EODHDSymbol()),
		Title:           fmt.Sprintf("%s Comprehensive Stock Data", ticker.String()),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		Metadata:        metadata,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}

	return doc
}

// formatNumber formats a number with commas
func (w *FundamentalsWorker) formatNumber(n int64) string {
	if n == 0 {
		return "0"
	}
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// formatLargeNumber formats large numbers with M/B suffix
func (w *FundamentalsWorker) formatLargeNumber(n int64) string {
	if n >= 1e9 {
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	}
	if n >= 1e6 {
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	}
	return w.formatNumber(n)
}

// writeFinancialPerformanceTable generates a comprehensive YoY financial performance table
// similar to EODHD's financial summary presentation
func (w *FundamentalsWorker) writeFinancialPerformanceTable(content *strings.Builder, data *StockCollectorData) {
	if len(data.AnnualData) == 0 && len(data.QuarterlyData) == 0 {
		return
	}

	// Combine annual and half-year data for the table
	// We'll show up to 10 periods (mix of annual and quarterly)
	type periodData struct {
		EndDate         string
		PeriodLabel     string
		TotalRevenue    int64
		GrossProfit     int64
		OperatingIncome int64
		NetIncome       int64
		EBITDA          int64
		TotalAssets     int64
		TotalEquity     int64
		GrossMargin     float64
		OperatingMargin float64
		NetMargin       float64
		ROE             float64
		ROA             float64
	}

	periods := make([]periodData, 0, 10)

	// Add annual data first (most important)
	for i, a := range data.AnnualData {
		if i >= 10 { // Limit to 10 annual periods (10 years if available)
			break
		}
		label := a.EndDate[:7] // YYYY-MM format for label
		pd := periodData{
			EndDate:         a.EndDate,
			PeriodLabel:     label,
			TotalRevenue:    a.TotalRevenue,
			GrossProfit:     a.GrossProfit,
			OperatingIncome: a.OperatingIncome,
			NetIncome:       a.NetIncome,
			EBITDA:          a.EBITDA,
			TotalAssets:     a.TotalAssets,
			TotalEquity:     a.TotalEquity,
			GrossMargin:     a.GrossMargin,
			NetMargin:       a.NetMargin,
		}
		// Calculate operating margin
		if a.TotalRevenue > 0 {
			pd.OperatingMargin = float64(a.OperatingIncome) / float64(a.TotalRevenue) * 100
		}
		// Calculate ROE and ROA
		if a.TotalEquity > 0 {
			pd.ROE = float64(a.NetIncome) / float64(a.TotalEquity) * 100
		}
		if a.TotalAssets > 0 {
			pd.ROA = float64(a.NetIncome) / float64(a.TotalAssets) * 100
		}
		periods = append(periods, pd)
	}

	if len(periods) < 2 {
		return // Need at least 2 periods for YoY comparison
	}

	content.WriteString("## Financial Performance (Year-over-Year)\n\n")
	content.WriteString("*Financial data sourced from EODHD. Values in millions (M) or billions (B).*\n\n")

	// Build header row with period labels
	content.WriteString("| Metric |")
	for _, p := range periods {
		content.WriteString(fmt.Sprintf(" %s |", p.PeriodLabel))
	}
	content.WriteString("\n|--------|")
	for range periods {
		content.WriteString("--------|")
	}
	content.WriteString("\n")

	// Revenue row
	content.WriteString("| Revenue |")
	for _, p := range periods {
		content.WriteString(fmt.Sprintf(" %s |", w.formatLargeNumber(p.TotalRevenue)))
	}
	content.WriteString("\n")

	// Revenue YoY row
	content.WriteString("| Revenue YoY |")
	for i, p := range periods {
		if i < len(periods)-1 && periods[i+1].TotalRevenue > 0 {
			yoy := (float64(p.TotalRevenue) - float64(periods[i+1].TotalRevenue)) / float64(periods[i+1].TotalRevenue) * 100
			content.WriteString(fmt.Sprintf(" %.1f%% |", yoy))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// Gross Profit row
	content.WriteString("| Gross Profit |")
	for _, p := range periods {
		content.WriteString(fmt.Sprintf(" %s |", w.formatLargeNumber(p.GrossProfit)))
	}
	content.WriteString("\n")

	// Operating Income row
	content.WriteString("| Operating Income |")
	for _, p := range periods {
		content.WriteString(fmt.Sprintf(" %s |", w.formatLargeNumber(p.OperatingIncome)))
	}
	content.WriteString("\n")

	// Net Income row
	content.WriteString("| Net Income |")
	for _, p := range periods {
		content.WriteString(fmt.Sprintf(" %s |", w.formatLargeNumber(p.NetIncome)))
	}
	content.WriteString("\n")

	// Net Income YoY row
	content.WriteString("| Net Income YoY |")
	for i, p := range periods {
		if i < len(periods)-1 && periods[i+1].NetIncome != 0 {
			yoy := (float64(p.NetIncome) - float64(periods[i+1].NetIncome)) / math.Abs(float64(periods[i+1].NetIncome)) * 100
			content.WriteString(fmt.Sprintf(" %.1f%% |", yoy))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// EBITDA row
	content.WriteString("| EBITDA |")
	for _, p := range periods {
		if p.EBITDA > 0 {
			content.WriteString(fmt.Sprintf(" %s |", w.formatLargeNumber(p.EBITDA)))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// Separator for ratios
	content.WriteString("| **Profitability** |")
	for range periods {
		content.WriteString(" |")
	}
	content.WriteString("\n")

	// Gross Margin row
	content.WriteString("| Gross Margin |")
	for _, p := range periods {
		if p.GrossMargin > 0 {
			content.WriteString(fmt.Sprintf(" %.1f%% |", p.GrossMargin))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// Operating Margin row
	content.WriteString("| Operating Margin |")
	for _, p := range periods {
		if p.OperatingMargin > 0 {
			content.WriteString(fmt.Sprintf(" %.1f%% |", p.OperatingMargin))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// Net Margin row
	content.WriteString("| Net Margin |")
	for _, p := range periods {
		if p.TotalRevenue > 0 {
			content.WriteString(fmt.Sprintf(" %.1f%% |", p.NetMargin))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// ROE row
	content.WriteString("| Return on Equity |")
	for _, p := range periods {
		if p.ROE != 0 {
			content.WriteString(fmt.Sprintf(" %.1f%% |", p.ROE))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// ROA row
	content.WriteString("| Return on Assets |")
	for _, p := range periods {
		if p.ROA != 0 {
			content.WriteString(fmt.Sprintf(" %.1f%% |", p.ROA))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// Separator for balance sheet
	content.WriteString("| **Balance Sheet** |")
	for range periods {
		content.WriteString(" |")
	}
	content.WriteString("\n")

	// Total Assets row
	content.WriteString("| Total Assets |")
	for _, p := range periods {
		if p.TotalAssets > 0 {
			content.WriteString(fmt.Sprintf(" %s |", w.formatLargeNumber(p.TotalAssets)))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n")

	// Total Equity row
	content.WriteString("| Total Equity |")
	for _, p := range periods {
		if p.TotalEquity > 0 {
			content.WriteString(fmt.Sprintf(" %s |", w.formatLargeNumber(p.TotalEquity)))
		} else {
			content.WriteString(" - |")
		}
	}
	content.WriteString("\n\n")
}

// writeFinancialHealthSection writes the Financial Health section to the document.
// This section provides data for Kneppy framework pillars:
// - Financial Robustness (Net Debt/EBITDA)
// - Cash Flow Reality (FCF Conversion)
// - Share Allocation (Share Dilution)
func (w *FundamentalsWorker) writeFinancialHealthSection(content *strings.Builder, data *StockCollectorData) {
	// Only write if we have relevant data
	hasDebtData := data.LatestCash > 0 || data.LatestTotalDebt > 0
	hasCashFlowData := data.LatestOperatingCF != 0 || data.LatestFreeCF != 0
	hasSharesData := len(data.SharesHistory) > 0 || data.SharesOutstanding > 0

	if !hasDebtData && !hasCashFlowData && !hasSharesData {
		return
	}

	content.WriteString("## Financial Health (Kneppy Framework)\n\n")

	// Debt & Liquidity Section (Financial Robustness Pillar)
	if hasDebtData {
		content.WriteString("### Debt & Liquidity\n\n")
		content.WriteString("| Metric | Value | Notes |\n")
		content.WriteString("|--------|-------|-------|\n")

		// Cash position
		content.WriteString(fmt.Sprintf("| Cash & Equivalents | $%s | Latest annual balance |\n", w.formatLargeNumber(data.LatestCash)))

		// Total Debt
		content.WriteString(fmt.Sprintf("| Total Debt | $%s | Short + Long term debt |\n", w.formatLargeNumber(data.LatestTotalDebt)))

		// Net Debt
		netDebtStr := w.formatLargeNumber(data.LatestNetDebt)
		if data.LatestNetDebt < 0 {
			netDebtStr = fmt.Sprintf("($%s)", w.formatLargeNumber(-data.LatestNetDebt))
			content.WriteString(fmt.Sprintf("| Net Debt | %s | **Net Cash Position** |\n", netDebtStr))
		} else {
			content.WriteString(fmt.Sprintf("| Net Debt | $%s | Debt minus Cash |\n", netDebtStr))
		}

		// Net Debt/EBITDA with pass/fail indicator
		if data.NetDebtToEBITDA != 0 {
			status := "PASS"
			notes := "Target: <2.0x"
			if data.NetDebtToEBITDA > 2.0 {
				status = "WATCH"
				notes = "Above 2.0x threshold"
			} else if data.NetDebtToEBITDA > 3.0 {
				status = "FAIL"
				notes = "High leverage"
			} else if data.NetDebtToEBITDA < 0 {
				status = "PASS"
				notes = "Net cash position"
			}
			content.WriteString(fmt.Sprintf("| Net Debt/EBITDA | %.2fx | %s - %s |\n", data.NetDebtToEBITDA, status, notes))
		} else {
			content.WriteString("| Net Debt/EBITDA | N/A | EBITDA not available |\n")
		}
		content.WriteString("\n")
	}

	// Cash Flow Quality Section (Cash Flow Reality Pillar)
	if hasCashFlowData {
		content.WriteString("### Cash Flow Quality\n\n")
		content.WriteString("| Metric | Value | Notes |\n")
		content.WriteString("|--------|-------|-------|\n")

		// Operating Cash Flow
		ocfStr := w.formatLargeNumber(data.LatestOperatingCF)
		if data.LatestOperatingCF < 0 {
			ocfStr = fmt.Sprintf("($%s)", w.formatLargeNumber(-data.LatestOperatingCF))
		} else {
			ocfStr = fmt.Sprintf("$%s", ocfStr)
		}
		content.WriteString(fmt.Sprintf("| Operating Cash Flow | %s | Latest annual |\n", ocfStr))

		// Free Cash Flow
		fcfStr := w.formatLargeNumber(data.LatestFreeCF)
		if data.LatestFreeCF < 0 {
			fcfStr = fmt.Sprintf("($%s)", w.formatLargeNumber(-data.LatestFreeCF))
		} else {
			fcfStr = fmt.Sprintf("$%s", fcfStr)
		}
		content.WriteString(fmt.Sprintf("| Free Cash Flow | %s | OCF minus CapEx |\n", fcfStr))

		// FCF Conversion with pass/fail indicator
		if data.FCFConversion != 0 {
			status := "PASS"
			notes := "Target: >90%"
			if data.FCFConversion < 90 && data.FCFConversion >= 70 {
				status = "WATCH"
				notes = "Below 90% threshold"
			} else if data.FCFConversion < 70 {
				status = "FAIL"
				notes = "Poor earnings quality"
			} else if data.FCFConversion > 150 {
				notes = "Excellent conversion"
			}
			content.WriteString(fmt.Sprintf("| FCF Conversion | %.1f%% | %s - %s |\n", data.FCFConversion, status, notes))
		} else {
			content.WriteString("| FCF Conversion | N/A | Net Income <= 0 |\n")
		}

		// FCF Margin
		if data.FCFToRevenue != 0 {
			content.WriteString(fmt.Sprintf("| FCF Margin | %.1f%% | FCF/Revenue |\n", data.FCFToRevenue))
		}
		content.WriteString("\n")
	}

	// Share Dilution Control Section (Share Allocation Pillar)
	if hasSharesData {
		content.WriteString("### Share Dilution Control\n\n")
		content.WriteString("| Metric | Value | Notes |\n")
		content.WriteString("|--------|-------|-------|\n")

		// Current Shares Outstanding
		if data.SharesOutstanding > 0 {
			content.WriteString(fmt.Sprintf("| Current Shares | %s | Latest outstanding |\n", w.formatLargeNumber(data.SharesOutstanding)))
		}

		// Shares 3Y CAGR with pass/fail indicator
		if data.SharesCAGR3Y != 0 || len(data.SharesHistory) >= 4 {
			status := "PASS"
			notes := "Target: <=0% (no dilution)"
			if data.SharesCAGR3Y > 0 && data.SharesCAGR3Y <= 2 {
				status = "WATCH"
				notes = "Minor dilution"
			} else if data.SharesCAGR3Y > 2 {
				status = "FAIL"
				notes = "Significant dilution"
			} else if data.SharesCAGR3Y < 0 {
				notes = "Buybacks reducing shares"
			}
			content.WriteString(fmt.Sprintf("| Shares 3Y CAGR | %.2f%% | %s - %s |\n", data.SharesCAGR3Y, status, notes))
		}

		// YoY Share Change
		if data.SharesGrowthYoY != 0 || len(data.SharesHistory) >= 2 {
			yoySign := ""
			if data.SharesGrowthYoY > 0 {
				yoySign = "+"
			}
			content.WriteString(fmt.Sprintf("| Shares YoY Change | %s%.2f%% | vs prior year |\n", yoySign, data.SharesGrowthYoY))
		}

		// Share History Summary
		if len(data.SharesHistory) > 0 {
			content.WriteString(fmt.Sprintf("| Historical Data | %d years | Available for analysis |\n", len(data.SharesHistory)))
		}
		content.WriteString("\n")
	}
}

// writeAnalystEstimatesSection writes the enhanced analyst estimates section (v2)
func (w *FundamentalsWorker) writeAnalystEstimatesSection(content *strings.Builder, data *StockCollectorData) {
	if data.AnalystEstimates == nil {
		return
	}

	est := data.AnalystEstimates

	// Only write if there's meaningful data
	hasEPSEstimates := est.EPSCurrentYear != 0 || est.EPSNextYear != 0
	hasRevenueEstimates := est.RevenueEstimateAvg != 0
	hasCoverage := est.EarningsAnalystCount > 0 || est.RevenueAnalystCount > 0

	if !hasEPSEstimates && !hasRevenueEstimates && !hasCoverage {
		// Write a note about no coverage
		content.WriteString("### Forward Estimates\n\n")
		content.WriteString("*No analyst coverage available for this stock.*\n\n")
		return
	}

	content.WriteString("### Forward Estimates\n\n")
	content.WriteString(fmt.Sprintf("**Coverage Quality:** %s\n\n", est.CoverageQuality))

	// EPS Estimates Table
	if hasEPSEstimates {
		content.WriteString("#### EPS Estimates\n\n")
		content.WriteString("| Period | Estimate | Notes |\n")
		content.WriteString("|--------|----------|-------|\n")
		if est.EPSCurrentYear != 0 {
			content.WriteString(fmt.Sprintf("| Current Year | $%.4f | FY estimate |\n", est.EPSCurrentYear))
		}
		if est.EPSNextYear != 0 {
			content.WriteString(fmt.Sprintf("| Next Year | $%.4f | FY+1 estimate |\n", est.EPSNextYear))
		}
		if est.EPSCurrentQtr != 0 {
			content.WriteString(fmt.Sprintf("| Current Quarter | $%.4f | Q estimate |\n", est.EPSCurrentQtr))
		}
		if est.EPSEstimateAvg != 0 && est.EPSEstimateLow != 0 && est.EPSEstimateHigh != 0 {
			content.WriteString(fmt.Sprintf("| Consensus Range | $%.4f - $%.4f | Avg: $%.4f |\n",
				est.EPSEstimateLow, est.EPSEstimateHigh, est.EPSEstimateAvg))
		}
		if est.EarningsAnalystCount > 0 {
			content.WriteString(fmt.Sprintf("| Analyst Count | %d | EPS coverage |\n", est.EarningsAnalystCount))
		}
		content.WriteString("\n")
	}

	// Revenue Estimates
	if hasRevenueEstimates {
		content.WriteString("#### Revenue Estimates\n\n")
		content.WriteString("| Metric | Value |\n")
		content.WriteString("|--------|-------|\n")
		content.WriteString(fmt.Sprintf("| Consensus Avg | $%s |\n", w.formatLargeNumber(int64(est.RevenueEstimateAvg))))
		if est.RevenueEstimateLow != 0 && est.RevenueEstimateHigh != 0 {
			content.WriteString(fmt.Sprintf("| Range | $%s - $%s |\n",
				w.formatLargeNumber(int64(est.RevenueEstimateLow)),
				w.formatLargeNumber(int64(est.RevenueEstimateHigh))))
		}
		if est.RevenueAnalystCount > 0 {
			content.WriteString(fmt.Sprintf("| Analyst Count | %d |\n", est.RevenueAnalystCount))
		}
		content.WriteString("\n")
	}

	// EPS Trend/Momentum (only if we have meaningful trend data)
	hasTrendData := est.EPSTrendCurrent != 0 && est.EPSTrend30DaysAgo != 0
	if hasTrendData {
		content.WriteString("#### Estimate Momentum\n\n")
		content.WriteString("| Period | EPS Estimate | Change |\n")
		content.WriteString("|--------|-------------|--------|\n")
		content.WriteString(fmt.Sprintf("| Current | $%.4f | - |\n", est.EPSTrendCurrent))
		if est.EPSTrend7DaysAgo != 0 {
			change7d := ((est.EPSTrendCurrent - est.EPSTrend7DaysAgo) / math.Abs(est.EPSTrend7DaysAgo)) * 100
			sign := ""
			if change7d > 0 {
				sign = "+"
			}
			content.WriteString(fmt.Sprintf("| 7 Days Ago | $%.4f | %s%.1f%% |\n", est.EPSTrend7DaysAgo, sign, change7d))
		}
		if est.EPSTrend30DaysAgo != 0 {
			sign := ""
			if est.EPSTrendChange30D > 0 {
				sign = "+"
			}
			content.WriteString(fmt.Sprintf("| 30 Days Ago | $%.4f | %s%.1f%% |\n", est.EPSTrend30DaysAgo, sign, est.EPSTrendChange30D))
		}
		if est.EPSTrend90DaysAgo != 0 {
			change90d := ((est.EPSTrendCurrent - est.EPSTrend90DaysAgo) / math.Abs(est.EPSTrend90DaysAgo)) * 100
			sign := ""
			if change90d > 0 {
				sign = "+"
			}
			content.WriteString(fmt.Sprintf("| 90 Days Ago | $%.4f | %s%.1f%% |\n", est.EPSTrend90DaysAgo, sign, change90d))
		}
		content.WriteString("\n")
	}

	// Revision Activity
	hasRevisions := est.RevisionsUp30Days > 0 || est.RevisionsDown30Days > 0
	if hasRevisions {
		content.WriteString("#### Revision Activity (Last 30 Days)\n\n")
		content.WriteString("| Direction | Count |\n")
		content.WriteString("|-----------|-------|\n")
		content.WriteString(fmt.Sprintf("| Upward Revisions | %d |\n", est.RevisionsUp30Days))
		content.WriteString(fmt.Sprintf("| Downward Revisions | %d |\n", est.RevisionsDown30Days))
		net := est.RevisionsUp30Days - est.RevisionsDown30Days
		sentiment := "Neutral"
		if net > 0 {
			sentiment = "Positive"
		} else if net < 0 {
			sentiment = "Negative"
		}
		content.WriteString(fmt.Sprintf("| Net Sentiment | %s (%+d) |\n", sentiment, net))
		content.WriteString("\n")
	}
}

// buildDataCompletenessMetadata creates metadata tracking data availability for each Kneppy framework pillar.
// This enables downstream consumers to understand data completeness without re-parsing.
func (w *FundamentalsWorker) buildDataCompletenessMetadata(data *StockCollectorData) map[string]interface{} {
	pillars := make(map[string]interface{})
	var availablePillars, totalPillars int
	var missingMetrics []string

	// 1. Capital Efficiency (ROIC/ROE)
	capitalEfficiencyAvailable := data.ReturnOnEquity > 0 || data.ReturnOnAssets > 0
	pillars["capital_efficiency"] = map[string]interface{}{
		"available": capitalEfficiencyAvailable,
		"metric":    "ROE/ROA",
		"value":     data.ReturnOnEquity,
		"threshold": ">15%",
	}
	totalPillars++
	if capitalEfficiencyAvailable {
		availablePillars++
	} else {
		missingMetrics = append(missingMetrics, "capital_efficiency (ROE/ROA)")
	}

	// 2. Share Allocation (Dilution Control)
	shareAllocationAvailable := data.SharesCAGR3Y != 0 || len(data.SharesHistory) >= 2 || data.SharesOutstanding > 0
	pillars["share_allocation"] = map[string]interface{}{
		"available": shareAllocationAvailable,
		"metric":    "Shares CAGR 3Y",
		"value":     data.SharesCAGR3Y,
		"threshold": "<=0%",
	}
	totalPillars++
	if shareAllocationAvailable {
		availablePillars++
	} else {
		missingMetrics = append(missingMetrics, "share_allocation (Shares CAGR)")
	}

	// 3. Financial Robustness (Net Debt/EBITDA)
	financialRobustnessAvailable := data.NetDebtToEBITDA != 0 || (data.LatestNetDebt != 0 && len(data.AnnualData) > 0 && data.AnnualData[0].EBITDA > 0)
	pillars["financial_robustness"] = map[string]interface{}{
		"available": financialRobustnessAvailable,
		"metric":    "Net Debt/EBITDA",
		"value":     data.NetDebtToEBITDA,
		"threshold": "<2.0x",
	}
	totalPillars++
	if financialRobustnessAvailable {
		availablePillars++
	} else {
		missingMetrics = append(missingMetrics, "financial_robustness (Net Debt/EBITDA)")
	}

	// 4. Cash Flow Reality (FCF Conversion)
	cashFlowRealityAvailable := data.FCFConversion != 0 || (data.LatestFreeCF != 0 && len(data.AnnualData) > 0 && data.AnnualData[0].NetIncome > 0)
	pillars["cash_flow_reality"] = map[string]interface{}{
		"available": cashFlowRealityAvailable,
		"metric":    "FCF Conversion",
		"value":     data.FCFConversion,
		"threshold": ">90%",
	}
	totalPillars++
	if cashFlowRealityAvailable {
		availablePillars++
	} else {
		missingMetrics = append(missingMetrics, "cash_flow_reality (FCF Conversion)")
	}

	// 5. Management Alignment (Insider Ownership)
	managementAlignmentAvailable := data.PercentInsiders > 0
	pillars["management_alignment"] = map[string]interface{}{
		"available": managementAlignmentAvailable,
		"metric":    "Insider %",
		"value":     data.PercentInsiders,
		"threshold": "HIGH (subjective)",
	}
	totalPillars++
	if managementAlignmentAvailable {
		availablePillars++
	} else {
		missingMetrics = append(missingMetrics, "management_alignment (Insider %)")
	}

	// 6. Competitive Moat (Margins)
	competitiveMoatAvailable := data.GrossMargin > 0 || data.OperatingMargin > 0
	pillars["competitive_moat"] = map[string]interface{}{
		"available": competitiveMoatAvailable,
		"metric":    "Operating Margin",
		"value":     data.OperatingMargin,
		"threshold": "Sector dependent",
	}
	totalPillars++
	if competitiveMoatAvailable {
		availablePillars++
	} else {
		missingMetrics = append(missingMetrics, "competitive_moat (Margins)")
	}

	// Calculate completeness score
	completenessScore := float64(availablePillars) / float64(totalPillars) * 100

	return map[string]interface{}{
		"pillars":            pillars,
		"available_pillars":  availablePillars,
		"total_pillars":      totalPillars,
		"completeness_score": fmt.Sprintf("%.0f%%", completenessScore),
		"completeness_ratio": fmt.Sprintf("%d/%d", availablePillars, totalPillars),
		"missing_metrics":    missingMetrics,
	}
}
