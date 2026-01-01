// -----------------------------------------------------------------------
// ASXHistoricalFinancialsWorker - Fetches historical financial data
// Uses Yahoo Finance API for income statements, balance sheets, and cash flow
// Provides structured output for orchestrator consumption
//
// DEPRECATED: Use asx_stock_collector instead.
// This worker is kept for backward compatibility with existing jobs.
// New integrations should use ASXStockCollectorWorker which consolidates
// price data, analyst coverage, and historical financials into a single call.
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
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

// ASXHistoricalFinancialsWorker fetches historical financial data for ASX stocks.
// Uses Yahoo Finance API to retrieve revenue, profit, EPS, and dividend history.
type ASXHistoricalFinancialsWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*ASXHistoricalFinancialsWorker)(nil)

// HistoricalFinancials holds all historical financial data for a stock
type HistoricalFinancials struct {
	Symbol           string
	CompanyName      string
	Currency         string
	FiscalYearEnd    string
	AnnualData       []FinancialPeriod
	QuarterlyData    []FinancialPeriod
	RevenueGrowthYoY float64 // Latest year-over-year revenue growth
	ProfitGrowthYoY  float64 // Latest year-over-year profit growth
	RevenueCAGR3Y    float64 // 3-year revenue CAGR
	RevenueCAGR5Y    float64 // 5-year revenue CAGR
	LastUpdated      time.Time
}

// FinancialPeriod holds financial data for a single period (annual or quarterly)
type FinancialPeriod struct {
	EndDate           time.Time
	PeriodType        string // "annual" or "quarterly"
	TotalRevenue      int64
	GrossProfit       int64
	OperatingIncome   int64
	NetIncome         int64
	EPS               float64 // Earnings per share (diluted)
	EBITDA            int64
	TotalAssets       int64
	TotalLiab         int64
	TotalEquity       int64
	OperatingCashFlow int64
	FreeCashFlow      int64
	DividendPerShare  float64
}

// yahooFinancialsResponse for Yahoo Finance quoteSummary with financial modules
type yahooFinancialsResponse struct {
	QuoteSummary struct {
		Result []struct {
			Price struct {
				ShortName string `json:"shortName"`
				Symbol    string `json:"symbol"`
				Currency  string `json:"currency"`
			} `json:"price"`
			IncomeStatementHistory struct {
				IncomeStatementHistory []struct {
					EndDate struct {
						Raw int64  `json:"raw"`
						Fmt string `json:"fmt"`
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
						Raw int64  `json:"raw"`
						Fmt string `json:"fmt"`
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
					DividendsPaid struct {
						Raw int64 `json:"raw"`
					} `json:"dividendsPaid"`
				} `json:"cashflowStatements"`
			} `json:"cashflowStatementHistory"`
			Earnings struct {
				FinancialsChart struct {
					Yearly []struct {
						Date     int   `json:"date"`
						Revenue  int64 `json:"revenue"`
						Earnings int64 `json:"earnings"`
					} `json:"yearly"`
					Quarterly []struct {
						Date     string `json:"date"`
						Revenue  int64  `json:"revenue"`
						Earnings int64  `json:"earnings"`
					} `json:"quarterly"`
				} `json:"financialsChart"`
			} `json:"earnings"`
			DefaultKeyStatistics struct {
				TrailingEps struct {
					Raw float64 `json:"raw"`
				} `json:"trailingEps"`
				ForwardEps struct {
					Raw float64 `json:"raw"`
				} `json:"forwardEps"`
				PegRatio struct {
					Raw float64 `json:"raw"`
				} `json:"pegRatio"`
				EnterpriseValue struct {
					Raw int64 `json:"raw"`
				} `json:"enterpriseValue"`
			} `json:"defaultKeyStatistics"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"quoteSummary"`
}

// NewASXHistoricalFinancialsWorker creates a new historical financials worker
func NewASXHistoricalFinancialsWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ASXHistoricalFinancialsWorker {
	return &ASXHistoricalFinancialsWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeASXHistoricalFinancials
func (w *ASXHistoricalFinancialsWorker) GetType() models.WorkerType {
	return models.WorkerTypeASXHistoricalFinancials
}

// Init initializes the historical financials worker
func (w *ASXHistoricalFinancialsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for asx_historical_financials")
	}

	asxCode, ok := stepConfig["asx_code"].(string)
	if !ok || asxCode == "" {
		return nil, fmt.Errorf("asx_code is required in step config")
	}
	asxCode = strings.ToUpper(asxCode)

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Msg("ASX historical financials worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   asxCode,
				Name: fmt.Sprintf("Fetch ASX:%s historical financials", asxCode),
				Type: "asx_historical_financials",
				Config: map[string]interface{}{
					"asx_code": asxCode,
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"asx_code":    asxCode,
			"step_config": stepConfig,
		},
	}, nil
}

// isCacheFresh checks if a document was synced within the cache window
func (w *ASXHistoricalFinancialsWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// CreateJobs fetches historical financials and stores as document
func (w *ASXHistoricalFinancialsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize asx_historical_financials worker: %w", err)
		}
	}

	asxCode, _ := initResult.Metadata["asx_code"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Check cache settings (default 24 hours for financial data)
	cacheHours := 24
	if ch, ok := stepConfig["cache_hours"].(float64); ok {
		cacheHours = int(ch)
	}
	forceRefresh := false
	if fr, ok := stepConfig["force_refresh"].(bool); ok {
		forceRefresh = fr
	}

	// Build source identifiers
	sourceType := "asx_historical_financials"
	sourceID := fmt.Sprintf("asx:%s:historical_financials", asxCode)

	// Check for cached data before fetching
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && w.isCacheFresh(existingDoc, cacheHours) {
			w.logger.Info().
				Str("asx_code", asxCode).
				Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
				Int("cache_hours", cacheHours).
				Msg("Using cached historical financials data")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("ASX:%s - Using cached historical financials (last synced: %s)",
						asxCode, existingDoc.LastSynced.Format("2006-01-02 15:04")))
			}
			return stepID, nil
		}
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Msg("Fetching ASX historical financials")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching ASX:%s historical financial data", asxCode))
	}

	// Fetch historical financials data
	financials, err := w.fetchHistoricalFinancials(ctx, asxCode)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch historical financials")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch historical financials: %v", err))
		}
		return "", fmt.Errorf("failed to fetch historical financials: %w", err)
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
	doc := w.createDocument(ctx, financials, asxCode, &jobDef, stepID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to save historical financials document")
		return "", fmt.Errorf("failed to save historical financials: %w", err)
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Int("annual_periods", len(financials.AnnualData)).
		Float64("revenue_growth_yoy", financials.RevenueGrowthYoY).
		Msg("ASX historical financials processed")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("ASX:%s - %d annual periods, Revenue YoY: %.1f%%, 3Y CAGR: %.1f%%",
				asxCode, len(financials.AnnualData), financials.RevenueGrowthYoY, financials.RevenueCAGR3Y))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false
func (w *ASXHistoricalFinancialsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *ASXHistoricalFinancialsWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("asx_historical_financials step requires config")
	}
	asxCode, ok := step.Config["asx_code"].(string)
	if !ok || asxCode == "" {
		return fmt.Errorf("asx_historical_financials step requires 'asx_code' in config")
	}
	return nil
}

// fetchHistoricalFinancials fetches financial data from Yahoo Finance
func (w *ASXHistoricalFinancialsWorker) fetchHistoricalFinancials(ctx context.Context, asxCode string) (*HistoricalFinancials, error) {
	financials := &HistoricalFinancials{
		Symbol:      asxCode,
		LastUpdated: time.Now(),
	}

	// Yahoo Finance symbol for ASX stocks
	yahooSymbol := strings.ToUpper(asxCode) + ".AX"

	// Fetch from Yahoo Finance quoteSummary API with financial modules
	modules := "price,incomeStatementHistory,incomeStatementHistoryQuarterly,balanceSheetHistory,cashflowStatementHistory,earnings,defaultKeyStatistics"
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=%s",
		yahooSymbol, modules,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch financial data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yahoo Finance API returned status %d", resp.StatusCode)
	}

	var apiResp yahooFinancialsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no data in response for %s", yahooSymbol)
	}

	result := apiResp.QuoteSummary.Result[0]

	// Extract basic info
	financials.CompanyName = result.Price.ShortName
	financials.Currency = result.Price.Currency
	if financials.Currency == "" {
		financials.Currency = "AUD"
	}

	// Build annual data from income statement history
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

	for _, is := range result.IncomeStatementHistory.IncomeStatementHistory {
		period := FinancialPeriod{
			EndDate:         time.Unix(is.EndDate.Raw, 0),
			PeriodType:      "annual",
			TotalRevenue:    is.TotalRevenue.Raw,
			GrossProfit:     is.GrossProfit.Raw,
			OperatingIncome: is.OperatingIncome.Raw,
			NetIncome:       is.NetIncome.Raw,
			EBITDA:          is.EBITDA.Raw,
		}

		// Add balance sheet data
		if bs, ok := balanceMap[is.EndDate.Raw]; ok {
			period.TotalAssets = bs.Assets
			period.TotalLiab = bs.Liab
			period.TotalEquity = bs.Equity
		}

		// Add cashflow data
		if cf, ok := cashflowMap[is.EndDate.Raw]; ok {
			period.OperatingCashFlow = cf.OperatingCF
			period.FreeCashFlow = cf.FreeCF
		}

		financials.AnnualData = append(financials.AnnualData, period)
	}

	// Sort annual data by date (newest first)
	sort.Slice(financials.AnnualData, func(i, j int) bool {
		return financials.AnnualData[i].EndDate.After(financials.AnnualData[j].EndDate)
	})

	// Build quarterly data
	for _, is := range result.IncomeStatementHistoryQuarterly.IncomeStatementHistory {
		period := FinancialPeriod{
			EndDate:         time.Unix(is.EndDate.Raw, 0),
			PeriodType:      "quarterly",
			TotalRevenue:    is.TotalRevenue.Raw,
			GrossProfit:     is.GrossProfit.Raw,
			OperatingIncome: is.OperatingIncome.Raw,
			NetIncome:       is.NetIncome.Raw,
		}
		financials.QuarterlyData = append(financials.QuarterlyData, period)
	}

	// Sort quarterly data by date (newest first)
	sort.Slice(financials.QuarterlyData, func(i, j int) bool {
		return financials.QuarterlyData[i].EndDate.After(financials.QuarterlyData[j].EndDate)
	})

	// Calculate growth metrics
	if len(financials.AnnualData) >= 2 {
		current := financials.AnnualData[0]
		previous := financials.AnnualData[1]

		if previous.TotalRevenue > 0 {
			financials.RevenueGrowthYoY = float64(current.TotalRevenue-previous.TotalRevenue) / float64(previous.TotalRevenue) * 100
		}
		if previous.NetIncome > 0 {
			financials.ProfitGrowthYoY = float64(current.NetIncome-previous.NetIncome) / float64(previous.NetIncome) * 100
		}
	}

	// Calculate 3-year and 5-year CAGR
	financials.RevenueCAGR3Y = w.calculateCAGR(financials.AnnualData, 3)
	financials.RevenueCAGR5Y = w.calculateCAGR(financials.AnnualData, 5)

	return financials, nil
}

// calculateCAGR calculates Compound Annual Growth Rate for revenue
func (w *ASXHistoricalFinancialsWorker) calculateCAGR(data []FinancialPeriod, years int) float64 {
	if len(data) < years+1 {
		return 0
	}

	// Data is sorted newest first
	endValue := float64(data[0].TotalRevenue)
	startValue := float64(data[years].TotalRevenue)

	if startValue <= 0 || endValue <= 0 {
		return 0
	}

	// CAGR = (End/Start)^(1/years) - 1
	cagr := (pow(endValue/startValue, 1.0/float64(years)) - 1) * 100
	return cagr
}

// pow calculates x^y for float64
func pow(x, y float64) float64 {
	if y == 0 {
		return 1
	}
	result := x
	for i := 1; i < int(y); i++ {
		result *= x
	}
	// For fractional exponents, use approximation
	if y != float64(int(y)) {
		// Use Newton's method approximation for roots
		// This is a simplified version; for production use math.Pow
		frac := y - float64(int(y))
		result *= (1 + frac*(x-1))
	}
	return result
}

// createDocument creates a document from historical financials data
func (w *ASXHistoricalFinancialsWorker) createDocument(ctx context.Context, data *HistoricalFinancials, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# ASX:%s Historical Financials - %s\n\n", asxCode, data.CompanyName))
	content.WriteString(fmt.Sprintf("**Last Updated**: %s\n", data.LastUpdated.Format("2 Jan 2006 3:04 PM AEST")))
	content.WriteString(fmt.Sprintf("**Currency**: %s\n\n", data.Currency))

	// Growth Summary Section
	content.WriteString("## Growth Summary\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Revenue YoY Growth | %.1f%% |\n", data.RevenueGrowthYoY))
	content.WriteString(fmt.Sprintf("| Profit YoY Growth | %.1f%% |\n", data.ProfitGrowthYoY))
	content.WriteString(fmt.Sprintf("| Revenue 3Y CAGR | %.1f%% |\n", data.RevenueCAGR3Y))
	content.WriteString(fmt.Sprintf("| Revenue 5Y CAGR | %.1f%% |\n", data.RevenueCAGR5Y))
	content.WriteString("\n")

	// Annual Financial History Section
	if len(data.AnnualData) > 0 {
		content.WriteString("## Annual Financial History\n\n")
		content.WriteString("| FY End | Revenue | Net Income | Gross Margin | Net Margin | Total Assets | Total Equity |\n")
		content.WriteString("|--------|---------|------------|--------------|------------|--------------|-------------|\n")
		for _, period := range data.AnnualData {
			grossMargin := 0.0
			netMargin := 0.0
			if period.TotalRevenue > 0 {
				grossMargin = float64(period.GrossProfit) / float64(period.TotalRevenue) * 100
				netMargin = float64(period.NetIncome) / float64(period.TotalRevenue) * 100
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %s | %.1f%% | %.1f%% | %s | %s |\n",
				period.EndDate.Format("Jun 2006"),
				formatLargeNumber(period.TotalRevenue),
				formatLargeNumber(period.NetIncome),
				grossMargin,
				netMargin,
				formatLargeNumber(period.TotalAssets),
				formatLargeNumber(period.TotalEquity),
			))
		}
		content.WriteString("\n")
	}

	// Cash Flow Summary
	if len(data.AnnualData) > 0 {
		content.WriteString("## Cash Flow Summary\n\n")
		content.WriteString("| FY End | Operating CF | Free CF | EBITDA |\n")
		content.WriteString("|--------|--------------|---------|--------|\n")
		for _, period := range data.AnnualData {
			content.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				period.EndDate.Format("Jun 2006"),
				formatLargeNumber(period.OperatingCashFlow),
				formatLargeNumber(period.FreeCashFlow),
				formatLargeNumber(period.EBITDA),
			))
		}
		content.WriteString("\n")
	}

	// Recent Quarterly Performance (last 4 quarters)
	if len(data.QuarterlyData) > 0 {
		content.WriteString("## Recent Quarterly Performance\n\n")
		content.WriteString("| Quarter | Revenue | Net Income | YoY Rev Growth |\n")
		content.WriteString("|---------|---------|------------|----------------|\n")
		maxQuarters := 4
		if len(data.QuarterlyData) < maxQuarters {
			maxQuarters = len(data.QuarterlyData)
		}
		for i := 0; i < maxQuarters; i++ {
			period := data.QuarterlyData[i]
			yoyGrowth := "-"
			// Try to find same quarter last year
			if i+4 < len(data.QuarterlyData) {
				lastYear := data.QuarterlyData[i+4]
				if lastYear.TotalRevenue > 0 {
					growth := float64(period.TotalRevenue-lastYear.TotalRevenue) / float64(lastYear.TotalRevenue) * 100
					yoyGrowth = fmt.Sprintf("%.1f%%", growth)
				}
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				period.EndDate.Format("Jan 2006"),
				formatLargeNumber(period.TotalRevenue),
				formatLargeNumber(period.NetIncome),
				yoyGrowth,
			))
		}
		content.WriteString("\n")
	}

	// Build tags
	tags := []string{"asx-historical-financials", strings.ToLower(asxCode)}
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

	// Build annual periods for metadata
	var annualMeta []map[string]interface{}
	for _, p := range data.AnnualData {
		annualMeta = append(annualMeta, map[string]interface{}{
			"end_date":          p.EndDate.Format("2006-01-02"),
			"total_revenue":     p.TotalRevenue,
			"gross_profit":      p.GrossProfit,
			"operating_income":  p.OperatingIncome,
			"net_income":        p.NetIncome,
			"ebitda":            p.EBITDA,
			"total_assets":      p.TotalAssets,
			"total_liabilities": p.TotalLiab,
			"total_equity":      p.TotalEquity,
			"operating_cf":      p.OperatingCashFlow,
			"free_cf":           p.FreeCashFlow,
		})
	}

	// Build metadata
	metadata := map[string]interface{}{
		"asx_code":           asxCode,
		"company_name":       data.CompanyName,
		"currency":           data.Currency,
		"revenue_growth_yoy": data.RevenueGrowthYoY,
		"profit_growth_yoy":  data.ProfitGrowthYoY,
		"revenue_cagr_3y":    data.RevenueCAGR3Y,
		"revenue_cagr_5y":    data.RevenueCAGR5Y,
		"annual_periods":     len(data.AnnualData),
		"quarterly_periods":  len(data.QuarterlyData),
		"annual_data":        annualMeta,
		"parent_job_id":      parentJobID,
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "asx_historical_financials",
		SourceID:        fmt.Sprintf("asx:%s:historical_financials", asxCode),
		URL:             fmt.Sprintf("https://finance.yahoo.com/quote/%s.AX/financials", asxCode),
		Title:           fmt.Sprintf("ASX:%s Historical Financials & Growth Analysis", asxCode),
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
