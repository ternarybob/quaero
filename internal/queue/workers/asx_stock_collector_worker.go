// -----------------------------------------------------------------------
// ASXStockCollectorWorker - Consolidated Yahoo Finance data collector
// Fetches price, analyst coverage, and historical financials in a single API call
// DEPRECATED: asx_stock_data, asx_analyst_coverage, asx_historical_financials
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ASXStockCollectorWorker fetches comprehensive stock data in a single Yahoo Finance API call.
// This consolidates asx_stock_data, asx_analyst_coverage, and asx_historical_financials.
// NO AI processing - pure data collection only.
type ASXStockCollectorWorker struct {
	documentStorage interfaces.DocumentStorage
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

	// Price data (from price module)
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

	// Valuation (from summaryDetail)
	PERatio       float64 `json:"pe_ratio"`
	EPS           float64 `json:"eps"`
	DividendYield float64 `json:"dividend_yield"`

	// Technicals (calculated from historical prices)
	SMA20       float64 `json:"sma_20"`
	SMA50       float64 `json:"sma_50"`
	SMA200      float64 `json:"sma_200"`
	RSI14       float64 `json:"rsi_14"`
	Support     float64 `json:"support"`
	Resistance  float64 `json:"resistance"`
	TrendSignal string  `json:"trend_signal"` // "BULLISH", "BEARISH", "NEUTRAL"

	// Analyst coverage (from financialData, recommendationTrend)
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

	// Historical financials (from incomeStatementHistory)
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

// yahooQuoteSummaryFullResponse for Yahoo Finance quoteSummary with all modules
type yahooQuoteSummaryFullResponse struct {
	QuoteSummary struct {
		Result []yahooQuoteSummaryResult `json:"result"`
		Error  interface{}               `json:"error"`
	} `json:"quoteSummary"`
}

type yahooQuoteSummaryResult struct {
	Price struct {
		RegularMarketPrice struct {
			Raw float64 `json:"raw"`
		} `json:"regularMarketPrice"`
		RegularMarketChange struct {
			Raw float64 `json:"raw"`
		} `json:"regularMarketChange"`
		RegularMarketChangePercent struct {
			Raw float64 `json:"raw"`
		} `json:"regularMarketChangePercent"`
		RegularMarketDayLow struct {
			Raw float64 `json:"raw"`
		} `json:"regularMarketDayLow"`
		RegularMarketDayHigh struct {
			Raw float64 `json:"raw"`
		} `json:"regularMarketDayHigh"`
		RegularMarketVolume struct {
			Raw int64 `json:"raw"`
		} `json:"regularMarketVolume"`
		AverageVolume struct {
			Raw int64 `json:"raw"`
		} `json:"averageDailyVolume10Day"`
		MarketCap struct {
			Raw int64 `json:"raw"`
		} `json:"marketCap"`
		ShortName string `json:"shortName"`
		Symbol    string `json:"symbol"`
		Currency  string `json:"currency"`
	} `json:"price"`
	SummaryDetail struct {
		FiftyTwoWeekLow struct {
			Raw float64 `json:"raw"`
		} `json:"fiftyTwoWeekLow"`
		FiftyTwoWeekHigh struct {
			Raw float64 `json:"raw"`
		} `json:"fiftyTwoWeekHigh"`
		TrailingPE struct {
			Raw float64 `json:"raw"`
		} `json:"trailingPE"`
		DividendYield struct {
			Raw float64 `json:"raw"`
		} `json:"dividendYield"`
	} `json:"summaryDetail"`
	DefaultKeyStatistics struct {
		TrailingEps struct {
			Raw float64 `json:"raw"`
		} `json:"trailingEps"`
	} `json:"defaultKeyStatistics"`
	FinancialData struct {
		TargetHighPrice struct {
			Raw float64 `json:"raw"`
		} `json:"targetHighPrice"`
		TargetLowPrice struct {
			Raw float64 `json:"raw"`
		} `json:"targetLowPrice"`
		TargetMeanPrice struct {
			Raw float64 `json:"raw"`
		} `json:"targetMeanPrice"`
		TargetMedianPrice struct {
			Raw float64 `json:"raw"`
		} `json:"targetMedianPrice"`
		RecommendationMean struct {
			Raw float64 `json:"raw"`
		} `json:"recommendationMean"`
		RecommendationKey       string `json:"recommendationKey"`
		NumberOfAnalystOpinions struct {
			Raw int `json:"raw"`
		} `json:"numberOfAnalystOpinions"`
		CurrentPrice struct {
			Raw float64 `json:"raw"`
		} `json:"currentPrice"`
	} `json:"financialData"`
	RecommendationTrend struct {
		Trend []struct {
			Period     string `json:"period"`
			StrongBuy  int    `json:"strongBuy"`
			Buy        int    `json:"buy"`
			Hold       int    `json:"hold"`
			Sell       int    `json:"sell"`
			StrongSell int    `json:"strongSell"`
		} `json:"trend"`
	} `json:"recommendationTrend"`
	UpgradeDowngradeHistory struct {
		History []struct {
			EpochGradeDate int64  `json:"epochGradeDate"`
			Firm           string `json:"firm"`
			ToGrade        string `json:"toGrade"`
			FromGrade      string `json:"fromGrade"`
			Action         string `json:"action"`
		} `json:"history"`
	} `json:"upgradeDowngradeHistory"`
	IncomeStatementHistory struct {
		IncomeStatementHistory []struct {
			EndDate struct {
				Raw int64 `json:"raw"`
			} `json:"endDate"`
			TotalRevenue struct {
				Raw int64 `json:"raw"`
			} `json:"totalRevenue"`
			GrossProfit struct {
				Raw int64 `json:"raw"`
			} `json:"grossProfit"`
			OperatingIncome struct {
				Raw int64 `json:"raw"`
			} `json:"operatingIncome"`
			NetIncome struct {
				Raw int64 `json:"raw"`
			} `json:"netIncome"`
			EBITDA struct {
				Raw int64 `json:"raw"`
			} `json:"ebitda"`
		} `json:"incomeStatementHistory"`
	} `json:"incomeStatementHistory"`
	IncomeStatementHistoryQuarterly struct {
		IncomeStatementHistory []struct {
			EndDate struct {
				Raw int64 `json:"raw"`
			} `json:"endDate"`
			TotalRevenue struct {
				Raw int64 `json:"raw"`
			} `json:"totalRevenue"`
			GrossProfit struct {
				Raw int64 `json:"raw"`
			} `json:"grossProfit"`
			OperatingIncome struct {
				Raw int64 `json:"raw"`
			} `json:"operatingIncome"`
			NetIncome struct {
				Raw int64 `json:"raw"`
			} `json:"netIncome"`
		} `json:"incomeStatementHistory"`
	} `json:"incomeStatementHistoryQuarterly"`
	BalanceSheetHistory struct {
		BalanceSheetStatements []struct {
			EndDate struct {
				Raw int64 `json:"raw"`
			} `json:"endDate"`
			TotalAssets struct {
				Raw int64 `json:"raw"`
			} `json:"totalAssets"`
			TotalLiab struct {
				Raw int64 `json:"raw"`
			} `json:"totalLiab"`
			TotalStockholderEquity struct {
				Raw int64 `json:"raw"`
			} `json:"totalStockholderEquity"`
		} `json:"balanceSheetStatements"`
	} `json:"balanceSheetHistory"`
	CashflowStatementHistory struct {
		CashflowStatements []struct {
			EndDate struct {
				Raw int64 `json:"raw"`
			} `json:"endDate"`
			TotalCashFromOperatingActivities struct {
				Raw int64 `json:"raw"`
			} `json:"totalCashFromOperatingActivities"`
			FreeCashFlow struct {
				Raw int64 `json:"raw"`
			} `json:"freeCashFlow"`
		} `json:"cashflowStatements"`
	} `json:"cashflowStatementHistory"`
}

// yahooChartResponseCollector for Yahoo Finance chart endpoint
type yahooChartResponseCollector struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// NewASXStockCollectorWorker creates a new consolidated stock collector worker
func NewASXStockCollectorWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ASXStockCollectorWorker {
	return &ASXStockCollectorWorker{
		documentStorage: documentStorage,
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

	asxCode, ok := stepConfig["asx_code"].(string)
	if !ok || asxCode == "" {
		return nil, fmt.Errorf("asx_code is required in step config")
	}
	asxCode = strings.ToUpper(asxCode)

	// Period for historical data (default Y2 = 24 months)
	period := "Y2"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
		Msg("ASX stock collector worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   asxCode,
				Name: fmt.Sprintf("Fetch ASX:%s comprehensive stock data", asxCode),
				Type: "asx_stock_collector",
				Config: map[string]interface{}{
					"asx_code": asxCode,
					"period":   period,
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"asx_code":    asxCode,
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

	asxCode, _ := initResult.Metadata["asx_code"].(string)
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

	// Build source identifiers
	sourceType := "asx_stock_collector"
	sourceID := fmt.Sprintf("asx:%s:stock_collector", asxCode)

	// Check for cached data before fetching
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && w.isCacheFresh(existingDoc, cacheHours) {
			w.logger.Info().
				Str("asx_code", asxCode).
				Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
				Int("cache_hours", cacheHours).
				Msg("Using cached stock collector data")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("ASX:%s - Using cached data (last synced: %s)",
						asxCode, existingDoc.LastSynced.Format("2006-01-02 15:04")))
			}
			return stepID, nil
		}
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
		Msg("Fetching ASX comprehensive stock data")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching ASX:%s comprehensive stock data (price, analyst, financials)", asxCode))
	}

	// Fetch all data in single API call
	stockData, err := w.fetchComprehensiveData(ctx, asxCode, period)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch comprehensive stock data")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch stock data: %v", err))
		}
		return "", fmt.Errorf("failed to fetch stock data: %w", err)
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

	// Create and save document
	doc := w.createDocument(ctx, stockData, asxCode, &jobDef, stepID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to save stock collector document")
		return "", fmt.Errorf("failed to save stock data: %w", err)
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Float64("price", stockData.CurrentPrice).
		Str("trend", stockData.TrendSignal).
		Int("analysts", stockData.AnalystCount).
		Float64("upside", stockData.UpsidePotential).
		Msg("ASX comprehensive stock data processed")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("ASX:%s - Price: $%.2f, Trend: %s, Analysts: %d, Target: $%.2f (%.1f%% upside)",
				asxCode, stockData.CurrentPrice, stockData.TrendSignal,
				stockData.AnalystCount, stockData.TargetMean, stockData.UpsidePotential))
	}

	return stepID, nil
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
	asxCode, ok := step.Config["asx_code"].(string)
	if !ok || asxCode == "" {
		return fmt.Errorf("asx_stock_collector step requires 'asx_code' in config")
	}
	return nil
}

// fetchComprehensiveData fetches all stock data from Yahoo Finance in a single API call
func (w *ASXStockCollectorWorker) fetchComprehensiveData(ctx context.Context, asxCode, period string) (*StockCollectorData, error) {
	data := &StockCollectorData{
		Symbol:      asxCode,
		AsxCode:     asxCode,
		LastUpdated: time.Now(),
	}

	yahooSymbol := strings.ToUpper(asxCode) + ".AX"

	// Single Yahoo Finance API call with all modules
	modules := []string{
		"price",
		"summaryDetail",
		"defaultKeyStatistics",
		"financialData",
		"recommendationTrend",
		"upgradeDowngradeHistory",
		"incomeStatementHistory",
		"incomeStatementHistoryQuarterly",
		"balanceSheetHistory",
		"cashflowStatementHistory",
	}

	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=%s",
		yahooSymbol, strings.Join(modules, ","),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stock data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yahoo Finance API returned status %d", resp.StatusCode)
	}

	var apiResp yahooQuoteSummaryFullResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no data in response for %s", yahooSymbol)
	}

	result := apiResp.QuoteSummary.Result[0]

	// Extract price data
	data.CompanyName = result.Price.ShortName
	data.Currency = result.Price.Currency
	if data.Currency == "" {
		data.Currency = "AUD"
	}
	data.CurrentPrice = result.Price.RegularMarketPrice.Raw
	data.PriceChange = result.Price.RegularMarketChange.Raw
	data.ChangePercent = result.Price.RegularMarketChangePercent.Raw
	data.DayLow = result.Price.RegularMarketDayLow.Raw
	data.DayHigh = result.Price.RegularMarketDayHigh.Raw
	data.Volume = result.Price.RegularMarketVolume.Raw
	data.AvgVolume = result.Price.AverageVolume.Raw
	data.MarketCap = result.Price.MarketCap.Raw

	// Summary detail
	data.Week52Low = result.SummaryDetail.FiftyTwoWeekLow.Raw
	data.Week52High = result.SummaryDetail.FiftyTwoWeekHigh.Raw
	data.PERatio = result.SummaryDetail.TrailingPE.Raw
	data.DividendYield = result.SummaryDetail.DividendYield.Raw * 100 // Convert to percentage
	data.EPS = result.DefaultKeyStatistics.TrailingEps.Raw

	// Analyst coverage
	data.AnalystCount = result.FinancialData.NumberOfAnalystOpinions.Raw
	data.TargetMean = result.FinancialData.TargetMeanPrice.Raw
	data.TargetHigh = result.FinancialData.TargetHighPrice.Raw
	data.TargetLow = result.FinancialData.TargetLowPrice.Raw
	data.TargetMedian = result.FinancialData.TargetMedianPrice.Raw
	data.RecommendationMean = result.FinancialData.RecommendationMean.Raw
	data.RecommendationKey = result.FinancialData.RecommendationKey

	// Calculate upside potential
	if data.CurrentPrice > 0 && data.TargetMean > 0 {
		data.UpsidePotential = ((data.TargetMean - data.CurrentPrice) / data.CurrentPrice) * 100
	}

	// Recommendation distribution
	if len(result.RecommendationTrend.Trend) > 0 {
		currentTrend := result.RecommendationTrend.Trend[0]
		data.StrongBuy = currentTrend.StrongBuy
		data.Buy = currentTrend.Buy
		data.Hold = currentTrend.Hold
		data.Sell = currentTrend.Sell
		data.StrongSell = currentTrend.StrongSell
	}

	// Upgrade/downgrade history (last 10)
	for i, h := range result.UpgradeDowngradeHistory.History {
		if i >= 10 {
			break
		}
		data.UpgradeDowngrades = append(data.UpgradeDowngrades, UpgradeDowngradeEntry{
			Date:      time.Unix(h.EpochGradeDate, 0).Format("2006-01-02"),
			Firm:      h.Firm,
			Action:    h.Action,
			FromGrade: h.FromGrade,
			ToGrade:   h.ToGrade,
		})
	}

	// Build balance sheet and cashflow maps
	balanceMap := make(map[int64]struct {
		Assets int64
		Liab   int64
		Equity int64
	})
	for _, bs := range result.BalanceSheetHistory.BalanceSheetStatements {
		balanceMap[bs.EndDate.Raw] = struct {
			Assets int64
			Liab   int64
			Equity int64
		}{
			Assets: bs.TotalAssets.Raw,
			Liab:   bs.TotalLiab.Raw,
			Equity: bs.TotalStockholderEquity.Raw,
		}
	}

	cashflowMap := make(map[int64]struct {
		OperatingCF int64
		FreeCF      int64
	})
	for _, cf := range result.CashflowStatementHistory.CashflowStatements {
		cashflowMap[cf.EndDate.Raw] = struct {
			OperatingCF int64
			FreeCF      int64
		}{
			OperatingCF: cf.TotalCashFromOperatingActivities.Raw,
			FreeCF:      cf.FreeCashFlow.Raw,
		}
	}

	// Annual financial data
	for _, is := range result.IncomeStatementHistory.IncomeStatementHistory {
		entry := FinancialPeriodEntry{
			EndDate:         time.Unix(is.EndDate.Raw, 0).Format("2006-01-02"),
			PeriodType:      "annual",
			TotalRevenue:    is.TotalRevenue.Raw,
			GrossProfit:     is.GrossProfit.Raw,
			OperatingIncome: is.OperatingIncome.Raw,
			NetIncome:       is.NetIncome.Raw,
			EBITDA:          is.EBITDA.Raw,
		}

		if is.TotalRevenue.Raw > 0 {
			entry.GrossMargin = float64(is.GrossProfit.Raw) / float64(is.TotalRevenue.Raw) * 100
			entry.NetMargin = float64(is.NetIncome.Raw) / float64(is.TotalRevenue.Raw) * 100
		}

		if bs, ok := balanceMap[is.EndDate.Raw]; ok {
			entry.TotalAssets = bs.Assets
			entry.TotalLiab = bs.Liab
			entry.TotalEquity = bs.Equity
		}

		if cf, ok := cashflowMap[is.EndDate.Raw]; ok {
			entry.OperatingCF = cf.OperatingCF
			entry.FreeCF = cf.FreeCF
		}

		data.AnnualData = append(data.AnnualData, entry)
	}

	// Sort annual data by date (newest first)
	sort.Slice(data.AnnualData, func(i, j int) bool {
		return data.AnnualData[i].EndDate > data.AnnualData[j].EndDate
	})

	// Quarterly financial data
	for _, is := range result.IncomeStatementHistoryQuarterly.IncomeStatementHistory {
		entry := FinancialPeriodEntry{
			EndDate:         time.Unix(is.EndDate.Raw, 0).Format("2006-01-02"),
			PeriodType:      "quarterly",
			TotalRevenue:    is.TotalRevenue.Raw,
			GrossProfit:     is.GrossProfit.Raw,
			OperatingIncome: is.OperatingIncome.Raw,
			NetIncome:       is.NetIncome.Raw,
		}

		if is.TotalRevenue.Raw > 0 {
			entry.GrossMargin = float64(is.GrossProfit.Raw) / float64(is.TotalRevenue.Raw) * 100
			entry.NetMargin = float64(is.NetIncome.Raw) / float64(is.TotalRevenue.Raw) * 100
		}

		data.QuarterlyData = append(data.QuarterlyData, entry)
	}

	// Sort quarterly data by date (newest first)
	sort.Slice(data.QuarterlyData, func(i, j int) bool {
		return data.QuarterlyData[i].EndDate > data.QuarterlyData[j].EndDate
	})

	// Calculate growth metrics
	if len(data.AnnualData) >= 2 {
		currentRev := data.AnnualData[0].TotalRevenue
		prevRev := data.AnnualData[1].TotalRevenue
		if prevRev > 0 {
			data.RevenueGrowthYoY = float64(currentRev-prevRev) / float64(prevRev) * 100
		}

		currentIncome := data.AnnualData[0].NetIncome
		prevIncome := data.AnnualData[1].NetIncome
		if prevIncome > 0 {
			data.ProfitGrowthYoY = float64(currentIncome-prevIncome) / float64(prevIncome) * 100
		}
	}

	// Calculate CAGR
	data.RevenueCAGR3Y = w.calculateRevenueCAGR(data.AnnualData, 3)
	data.RevenueCAGR5Y = w.calculateRevenueCAGR(data.AnnualData, 5)

	// Fetch historical prices
	if err := w.fetchHistoricalPrices(ctx, asxCode, period, data); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to fetch historical prices")
	}

	// Calculate technicals
	w.calculateTechnicals(data)

	// Calculate period performance
	w.calculatePeriodPerformance(data)

	return data, nil
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

// fetchHistoricalPrices fetches historical OHLCV from Yahoo Finance
func (w *ASXStockCollectorWorker) fetchHistoricalPrices(ctx context.Context, asxCode, period string, data *StockCollectorData) error {
	// Convert period to Yahoo format
	yahooRange := "1y"
	switch period {
	case "M1":
		yahooRange = "1mo"
	case "M3":
		yahooRange = "3mo"
	case "M6":
		yahooRange = "6mo"
	case "Y1":
		yahooRange = "1y"
	case "Y2":
		yahooRange = "2y"
	case "Y5":
		yahooRange = "5y"
	}

	yahooSymbol := strings.ToUpper(asxCode) + ".AX"
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%s",
		yahooSymbol, yahooRange)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var apiResp yahooChartResponseCollector
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if len(apiResp.Chart.Result) == 0 || len(apiResp.Chart.Result[0].Indicators.Quote) == 0 {
		return fmt.Errorf("no data in response")
	}

	result := apiResp.Chart.Result[0]
	quote := result.Indicators.Quote[0]

	var prevClose float64
	for i, ts := range result.Timestamp {
		if i >= len(quote.Close) || quote.Close[i] == 0 {
			continue
		}

		entry := OHLCVEntry{
			Date:   time.Unix(ts, 0).Format("2006-01-02"),
			Close:  quote.Close[i],
			Volume: 0,
		}
		if i < len(quote.Open) {
			entry.Open = quote.Open[i]
		}
		if i < len(quote.High) {
			entry.High = quote.High[i]
		}
		if i < len(quote.Low) {
			entry.Low = quote.Low[i]
		}
		if i < len(quote.Volume) {
			entry.Volume = quote.Volume[i]
		}

		// Calculate daily change
		if prevClose > 0 {
			entry.ChangeValue = entry.Close - prevClose
			entry.ChangePercent = (entry.ChangeValue / prevClose) * 100
		}
		prevClose = entry.Close

		data.HistoricalPrices = append(data.HistoricalPrices, entry)
	}

	// Sort by date ascending
	sort.Slice(data.HistoricalPrices, func(i, j int) bool {
		return data.HistoricalPrices[i].Date < data.HistoricalPrices[j].Date
	})

	return nil
}

// calculateTechnicals calculates technical indicators
func (w *ASXStockCollectorWorker) calculateTechnicals(data *StockCollectorData) {
	if len(data.HistoricalPrices) == 0 {
		return
	}

	closes := make([]float64, len(data.HistoricalPrices))
	for i, p := range data.HistoricalPrices {
		closes[i] = p.Close
	}

	// Calculate SMAs
	data.SMA20 = w.calculateSMA(closes, 20)
	data.SMA50 = w.calculateSMA(closes, 50)
	data.SMA200 = w.calculateSMA(closes, 200)

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
func (w *ASXStockCollectorWorker) createDocument(ctx context.Context, data *StockCollectorData, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# ASX:%s Comprehensive Stock Data - %s\n\n", asxCode, data.CompanyName))
	content.WriteString(fmt.Sprintf("**Last Updated**: %s\n", data.LastUpdated.Format("2 Jan 2006 3:04 PM AEST")))
	content.WriteString(fmt.Sprintf("**Currency**: %s\n\n", data.Currency))

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
	content.WriteString(fmt.Sprintf("| EPS | $%.2f |\n", data.EPS))
	content.WriteString(fmt.Sprintf("| Dividend Yield | %.2f%% |\n\n", data.DividendYield))

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

	// Annual Financial History
	if len(data.AnnualData) > 0 {
		content.WriteString("## Annual Financial History\n\n")
		content.WriteString("| FY End | Revenue | Net Income | Gross Margin | Net Margin |\n")
		content.WriteString("|--------|---------|------------|--------------|------------|\n")
		for _, p := range data.AnnualData {
			content.WriteString(fmt.Sprintf("| %s | %s | %s | %.1f%% | %.1f%% |\n",
				p.EndDate, w.formatLargeNumber(p.TotalRevenue), w.formatLargeNumber(p.NetIncome),
				p.GrossMargin, p.NetMargin))
		}
		content.WriteString("\n")
	}

	// Build tags
	tags := []string{"asx-stock-data", strings.ToLower(asxCode)}
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
		"asx_code":            asxCode,
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
	}

	// Add structured arrays to metadata
	if len(data.UpgradeDowngrades) > 0 {
		metadata["upgrade_downgrades"] = data.UpgradeDowngrades
	}
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

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "asx_stock_collector",
		SourceID:        fmt.Sprintf("asx:%s:stock_collector", asxCode),
		URL:             fmt.Sprintf("https://finance.yahoo.com/quote/%s.AX", asxCode),
		Title:           fmt.Sprintf("ASX:%s Comprehensive Stock Data", asxCode),
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
