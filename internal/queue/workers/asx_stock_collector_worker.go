// -----------------------------------------------------------------------
// ASXStockCollectorWorker - Consolidated Yahoo Finance data collector
// Fetches price, analyst coverage, and historical financials in a single API call
// DEPRECATED: asx_stock_data, asx_analyst_coverage, asx_historical_financials
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
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

// collectTickers collects all tickers from config, supporting both single and multiple formats.
// Supports: ticker, asx_code (single) and tickers, asx_codes (array).
func collectTickers(config map[string]interface{}) []common.Ticker {
	var tickers []common.Ticker
	seen := make(map[string]bool)

	addTicker := func(t common.Ticker) {
		if t.Code != "" && !seen[t.String()] {
			seen[t.String()] = true
			tickers = append(tickers, t)
		}
	}

	// Single ticker (legacy)
	if t := parseTicker(config); t.Code != "" {
		addTicker(t)
	}

	// Array of tickers
	if tickerArray, ok := config["tickers"].([]interface{}); ok {
		for _, v := range tickerArray {
			if s, ok := v.(string); ok && s != "" {
				addTicker(common.ParseTicker(s))
			}
		}
	}

	// Array of asx_codes (legacy)
	if codeArray, ok := config["asx_codes"].([]interface{}); ok {
		for _, v := range codeArray {
			if s, ok := v.(string); ok && s != "" {
				addTicker(common.ParseTicker(s))
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

// ASXStockCollectorWorker fetches comprehensive stock data using EODHD API.
// This consolidates asx_stock_data, asx_analyst_coverage, and asx_historical_financials.
// NO AI processing - pure data collection only.
type ASXStockCollectorWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*ASXStockCollectorWorker)(nil)

// StockCollectorData holds all consolidated stock data (in-code schema)
type StockCollectorData struct {
	// Core identification
	Symbol      string `json:"symbol"`
	CompanyName string `json:"company_name"`
	AsxCode     string `json:"asx_code"`
	Currency    string `json:"currency"`
	ISIN        string `json:"isin,omitempty"`
	Sector      string `json:"sector,omitempty"`
	Industry    string `json:"industry,omitempty"`

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

	// Metadata
	LastUpdated time.Time `json:"last_updated"`
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
	Date                 string      `json:"date"`
	Period               string      `json:"period"`
	Growth               interface{} `json:"growth"`
	EarningsEstimateAvg  interface{} `json:"earningsEstimateAvg"`
	EarningsEstimateLow  interface{} `json:"earningsEstimateLow"`
	EarningsEstimateHigh interface{} `json:"earningsEstimateHigh"`
	RevenueEstimateAvg   interface{} `json:"revenueEstimateAvg"`
	RevenueEstimateLow   interface{} `json:"revenueEstimateLow"`
	RevenueEstimateHigh  interface{} `json:"revenueEstimateHigh"`
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

// NewASXStockCollectorWorker creates a new consolidated stock collector worker
func NewASXStockCollectorWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ASXStockCollectorWorker {
	return &ASXStockCollectorWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeASXStockCollector
func (w *ASXStockCollectorWorker) GetType() models.WorkerType {
	return models.WorkerTypeASXStockCollector
}

// Init initializes the stock collector worker
func (w *ASXStockCollectorWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for asx_stock_collector")
	}

	// Collect tickers - supports both single and multiple formats
	tickers := collectTickers(stepConfig)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("ticker, asx_code, tickers, or asx_codes is required in step config")
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
			Type: "asx_stock_collector",
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
func (w *ASXStockCollectorWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// CreateJobs fetches comprehensive stock data and stores as document
func (w *ASXStockCollectorWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize asx_stock_collector worker: %w", err)
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

	// Extract output_tags
	var outputTags []string
	if stepConfig != nil {
		if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					outputTags = append(outputTags, tagStr)
				}
			}
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
		SourceType:       "asx_stock_collector",
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
func (w *ASXStockCollectorWorker) processTicker(ctx context.Context, ticker common.Ticker, period string, cacheHours int, forceRefresh bool, jobDef *models.JobDefinition, stepID string, outputTags []string) (*docInfo, error) {
	sourceType := "asx_stock_collector"
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
	stockData, err := w.fetchComprehensiveData(ctx, ticker, period)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stock data: %w", err)
	}

	// Create and save document
	doc := w.createDocument(ctx, stockData, ticker, jobDef, stepID, outputTags)
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
func (w *ASXStockCollectorWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *ASXStockCollectorWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("asx_stock_collector step requires config")
	}
	// Must have either ticker/asx_code (single) or tickers/asx_codes (multiple)
	tickers := collectTickers(step.Config)
	if len(tickers) == 0 {
		return fmt.Errorf("asx_stock_collector step requires 'ticker', 'asx_code', 'tickers', or 'asx_codes' in config")
	}
	return nil
}

// getEODHDAPIKey retrieves the EODHD API key from KV storage
func (w *ASXStockCollectorWorker) getEODHDAPIKey(ctx context.Context) string {
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
func (w *ASXStockCollectorWorker) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	return w.httpClient.Do(req)
}

// fetchComprehensiveData fetches all stock data from EODHD API
func (w *ASXStockCollectorWorker) fetchComprehensiveData(ctx context.Context, ticker common.Ticker, period string) (*StockCollectorData, error) {
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

// fetchEODHDFundamentals fetches all fundamental data from EODHD
func (w *ASXStockCollectorWorker) fetchEODHDFundamentals(ctx context.Context, apiKey, symbol string, data *StockCollectorData) error {
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

	// Parse financial statements
	w.parseEODHDFinancials(fundResp.Financials, data)

	return nil
}

// parseEODHDFinancials parses EODHD financial statements into annual/quarterly data
func (w *ASXStockCollectorWorker) parseEODHDFinancials(financials eodhdFinancials, data *StockCollectorData) {
	// Get sorted years from income statement
	incomeYears := make([]string, 0, len(financials.IncomeStatement.Yearly))
	for year := range financials.IncomeStatement.Yearly {
		incomeYears = append(incomeYears, year)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(incomeYears)))

	// Process yearly financial data
	for _, year := range incomeYears {
		incomeData := financials.IncomeStatement.Yearly[year]
		balanceData := financials.BalanceSheet.Yearly[year]
		cashflowData := financials.CashFlow.Yearly[year]

		entry := FinancialPeriodEntry{
			EndDate:    year,
			PeriodType: "annual",
		}

		// Income statement
		if v, ok := incomeData["totalRevenue"].(float64); ok {
			entry.TotalRevenue = int64(v)
		}
		if v, ok := incomeData["grossProfit"].(float64); ok {
			entry.GrossProfit = int64(v)
		}
		if v, ok := incomeData["operatingIncome"].(float64); ok {
			entry.OperatingIncome = int64(v)
		}
		if v, ok := incomeData["netIncome"].(float64); ok {
			entry.NetIncome = int64(v)
		}
		if v, ok := incomeData["ebitda"].(float64); ok {
			entry.EBITDA = int64(v)
		}

		// Balance sheet
		if balanceData != nil {
			if v, ok := balanceData["totalAssets"].(float64); ok {
				entry.TotalAssets = int64(v)
			}
			if v, ok := balanceData["totalLiab"].(float64); ok {
				entry.TotalLiab = int64(v)
			}
			if v, ok := balanceData["totalStockholderEquity"].(float64); ok {
				entry.TotalEquity = int64(v)
			}
		}

		// Cash flow
		if cashflowData != nil {
			if v, ok := cashflowData["totalCashFromOperatingActivities"].(float64); ok {
				entry.OperatingCF = int64(v)
			}
			if v, ok := cashflowData["freeCashFlow"].(float64); ok {
				entry.FreeCF = int64(v)
			}
		}

		// Calculate margins
		if entry.TotalRevenue > 0 {
			entry.GrossMargin = float64(entry.GrossProfit) / float64(entry.TotalRevenue) * 100
			entry.NetMargin = float64(entry.NetIncome) / float64(entry.TotalRevenue) * 100
		}

		data.AnnualData = append(data.AnnualData, entry)
	}

	// Process quarterly financial data
	quarterKeys := make([]string, 0, len(financials.IncomeStatement.Quarterly))
	for qtr := range financials.IncomeStatement.Quarterly {
		quarterKeys = append(quarterKeys, qtr)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(quarterKeys)))

	for i, qtr := range quarterKeys {
		if i >= 8 { // Limit to last 8 quarters
			break
		}
		incomeData := financials.IncomeStatement.Quarterly[qtr]

		entry := FinancialPeriodEntry{
			EndDate:    qtr,
			PeriodType: "quarterly",
		}

		if v, ok := incomeData["totalRevenue"].(float64); ok {
			entry.TotalRevenue = int64(v)
		}
		if v, ok := incomeData["grossProfit"].(float64); ok {
			entry.GrossProfit = int64(v)
		}
		if v, ok := incomeData["operatingIncome"].(float64); ok {
			entry.OperatingIncome = int64(v)
		}
		if v, ok := incomeData["netIncome"].(float64); ok {
			entry.NetIncome = int64(v)
		}

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
}

// fetchEODHDHistoricalPrices fetches historical OHLCV data from EODHD
func (w *ASXStockCollectorWorker) fetchEODHDHistoricalPrices(ctx context.Context, apiKey, symbol, period string, data *StockCollectorData) error {
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
func (w *ASXStockCollectorWorker) calculateRevenueCAGR(annualData []FinancialPeriodEntry, years int) float64 {
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
func (w *ASXStockCollectorWorker) calculateTechnicals(data *StockCollectorData) {
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
func (w *ASXStockCollectorWorker) calculateSMA(prices []float64, period int) float64 {
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
func (w *ASXStockCollectorWorker) calculateRSI(prices []float64, period int) float64 {
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
func (w *ASXStockCollectorWorker) findMin(values []float64) float64 {
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
func (w *ASXStockCollectorWorker) findMax(values []float64) float64 {
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
func (w *ASXStockCollectorWorker) determineTrend(price, sma20, sma50, sma200, rsi float64) string {
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
func (w *ASXStockCollectorWorker) calculatePeriodPerformance(data *StockCollectorData) {
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
func (w *ASXStockCollectorWorker) createDocument(ctx context.Context, data *StockCollectorData, ticker common.Ticker, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s Comprehensive Stock Data - %s\n\n", ticker.String(), data.CompanyName))
	content.WriteString(fmt.Sprintf("**Last Updated**: %s\n", data.LastUpdated.Format("2 Jan 2006 3:04 PM AEST")))
	content.WriteString(fmt.Sprintf("**Currency**: %s\n", data.Currency))
	content.WriteString(fmt.Sprintf("**Worker**: %s\n\n", models.WorkerTypeASXStockCollector))

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

	// Financial Growth Section
	content.WriteString("## Financial Growth\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Revenue YoY | %.1f%% |\n", data.RevenueGrowthYoY))
	content.WriteString(fmt.Sprintf("| Profit YoY | %.1f%% |\n", data.ProfitGrowthYoY))
	content.WriteString(fmt.Sprintf("| Revenue 3Y CAGR | %.1f%% |\n", data.RevenueCAGR3Y))
	content.WriteString(fmt.Sprintf("| Revenue 5Y CAGR | %.1f%% |\n\n", data.RevenueCAGR5Y))

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
		"ticker":              ticker.String(),
		"asx_code":            ticker.Code, // Keep for backwards compatibility
		"exchange":            ticker.Exchange,
		"company_name":        data.CompanyName,
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
	// Note: annual_data and quarterly_data removed - obtained via company announcements
	if len(data.HistoricalPrices) > 0 {
		metadata["historical_prices"] = data.HistoricalPrices
	}
	if len(data.PeriodPerformance) > 0 {
		metadata["period_performance"] = data.PeriodPerformance
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "asx_stock_collector",
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
func (w *ASXStockCollectorWorker) formatNumber(n int64) string {
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
func (w *ASXStockCollectorWorker) formatLargeNumber(n int64) string {
	if n >= 1e9 {
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	}
	if n >= 1e6 {
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	}
	return w.formatNumber(n)
}
