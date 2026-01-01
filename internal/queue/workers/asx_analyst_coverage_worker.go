// -----------------------------------------------------------------------
// ASXAnalystCoverageWorker - Fetches analyst coverage and broker ratings
// Uses Yahoo Finance API for analyst estimates, price targets, and recommendations
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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ASXAnalystCoverageWorker fetches analyst coverage data for ASX stocks.
// Uses Yahoo Finance API to retrieve broker ratings, price targets, and recommendations.
type ASXAnalystCoverageWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*ASXAnalystCoverageWorker)(nil)

// AnalystCoverage holds all analyst coverage data for a stock
type AnalystCoverage struct {
	Symbol             string
	CompanyName        string
	AnalystCount       int
	PriceTargetMean    float64
	PriceTargetHigh    float64
	PriceTargetLow     float64
	PriceTargetMedian  float64
	CurrentPrice       float64
	UpsidePotential    float64 // Percentage upside/downside to mean target
	RecommendationMean float64 // 1=Strong Buy, 5=Strong Sell
	RecommendationKey  string  // "buy", "hold", "sell"
	StrongBuy          int
	Buy                int
	Hold               int
	Sell               int
	StrongSell         int
	UpgradeDowngrades  []UpgradeDowngrade
	LastUpdated        time.Time
}

// UpgradeDowngrade represents a single analyst action
type UpgradeDowngrade struct {
	Firm       string
	ToGrade    string
	FromGrade  string
	Action     string // "up", "down", "init", "main"
	Date       time.Time
	EpochGrade int64
}

// yahooQuoteSummaryResponse for Yahoo Finance quoteSummary endpoint
type yahooQuoteSummaryResponse struct {
	QuoteSummary struct {
		Result []struct {
			Price struct {
				RegularMarketPrice struct {
					Raw float64 `json:"raw"`
				} `json:"regularMarketPrice"`
				ShortName string `json:"shortName"`
				Symbol    string `json:"symbol"`
			} `json:"price"`
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
			UpgradeDowngradeHistory struct {
				History []struct {
					EpochGradeDate int64  `json:"epochGradeDate"`
					Firm           string `json:"firm"`
					ToGrade        string `json:"toGrade"`
					FromGrade      string `json:"fromGrade"`
					Action         string `json:"action"`
				} `json:"history"`
			} `json:"upgradeDowngradeHistory"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"quoteSummary"`
}

// NewASXAnalystCoverageWorker creates a new analyst coverage worker
func NewASXAnalystCoverageWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ASXAnalystCoverageWorker {
	return &ASXAnalystCoverageWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeASXAnalystCoverage
func (w *ASXAnalystCoverageWorker) GetType() models.WorkerType {
	return models.WorkerTypeASXAnalystCoverage
}

// Init initializes the analyst coverage worker
func (w *ASXAnalystCoverageWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for asx_analyst_coverage")
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
		Msg("ASX analyst coverage worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   asxCode,
				Name: fmt.Sprintf("Fetch ASX:%s analyst coverage", asxCode),
				Type: "asx_analyst_coverage",
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
func (w *ASXAnalystCoverageWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// CreateJobs fetches analyst coverage and stores as document
func (w *ASXAnalystCoverageWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize asx_analyst_coverage worker: %w", err)
		}
	}

	asxCode, _ := initResult.Metadata["asx_code"].(string)
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
	sourceType := "asx_analyst_coverage"
	sourceID := fmt.Sprintf("asx:%s:analyst_coverage", asxCode)

	// Check for cached data before fetching
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && w.isCacheFresh(existingDoc, cacheHours) {
			w.logger.Info().
				Str("asx_code", asxCode).
				Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
				Int("cache_hours", cacheHours).
				Msg("Using cached analyst coverage data")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("ASX:%s - Using cached analyst coverage (last synced: %s)",
						asxCode, existingDoc.LastSynced.Format("2006-01-02 15:04")))
			}
			return stepID, nil
		}
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Msg("Fetching ASX analyst coverage")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching ASX:%s analyst coverage data", asxCode))
	}

	// Fetch analyst coverage data
	coverage, err := w.fetchAnalystCoverage(ctx, asxCode)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch analyst coverage")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch analyst coverage: %v", err))
		}
		return "", fmt.Errorf("failed to fetch analyst coverage: %w", err)
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
	doc := w.createDocument(ctx, coverage, asxCode, &jobDef, stepID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to save analyst coverage document")
		return "", fmt.Errorf("failed to save analyst coverage: %w", err)
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Int("analyst_count", coverage.AnalystCount).
		Float64("target_mean", coverage.PriceTargetMean).
		Str("recommendation", coverage.RecommendationKey).
		Msg("ASX analyst coverage processed")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("ASX:%s - Analysts: %d, Target: $%.2f, Rec: %s, Upside: %.1f%%",
				asxCode, coverage.AnalystCount, coverage.PriceTargetMean,
				coverage.RecommendationKey, coverage.UpsidePotential))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false
func (w *ASXAnalystCoverageWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *ASXAnalystCoverageWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("asx_analyst_coverage step requires config")
	}
	asxCode, ok := step.Config["asx_code"].(string)
	if !ok || asxCode == "" {
		return fmt.Errorf("asx_analyst_coverage step requires 'asx_code' in config")
	}
	return nil
}

// fetchAnalystCoverage fetches analyst data from Yahoo Finance
func (w *ASXAnalystCoverageWorker) fetchAnalystCoverage(ctx context.Context, asxCode string) (*AnalystCoverage, error) {
	coverage := &AnalystCoverage{
		Symbol:      asxCode,
		LastUpdated: time.Now(),
	}

	// Yahoo Finance symbol for ASX stocks
	yahooSymbol := strings.ToUpper(asxCode) + ".AX"

	// Fetch from Yahoo Finance quoteSummary API
	// Modules: price, financialData, recommendationTrend, upgradeDowngradeHistory
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=price,financialData,recommendationTrend,upgradeDowngradeHistory",
		yahooSymbol,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch analyst data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yahoo Finance API returned status %d", resp.StatusCode)
	}

	var apiResp yahooQuoteSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no data in response for %s", yahooSymbol)
	}

	result := apiResp.QuoteSummary.Result[0]

	// Extract price data
	coverage.CompanyName = result.Price.ShortName
	coverage.CurrentPrice = result.Price.RegularMarketPrice.Raw
	if coverage.CurrentPrice == 0 {
		coverage.CurrentPrice = result.FinancialData.CurrentPrice.Raw
	}

	// Extract financial data (analyst targets)
	coverage.PriceTargetMean = result.FinancialData.TargetMeanPrice.Raw
	coverage.PriceTargetHigh = result.FinancialData.TargetHighPrice.Raw
	coverage.PriceTargetLow = result.FinancialData.TargetLowPrice.Raw
	coverage.PriceTargetMedian = result.FinancialData.TargetMedianPrice.Raw
	coverage.AnalystCount = result.FinancialData.NumberOfAnalystOpinions.Raw
	coverage.RecommendationMean = result.FinancialData.RecommendationMean.Raw
	coverage.RecommendationKey = result.FinancialData.RecommendationKey

	// Calculate upside potential
	if coverage.CurrentPrice > 0 && coverage.PriceTargetMean > 0 {
		coverage.UpsidePotential = ((coverage.PriceTargetMean - coverage.CurrentPrice) / coverage.CurrentPrice) * 100
	}

	// Extract recommendation trend (current month)
	if len(result.RecommendationTrend.Trend) > 0 {
		currentTrend := result.RecommendationTrend.Trend[0]
		coverage.StrongBuy = currentTrend.StrongBuy
		coverage.Buy = currentTrend.Buy
		coverage.Hold = currentTrend.Hold
		coverage.Sell = currentTrend.Sell
		coverage.StrongSell = currentTrend.StrongSell
	}

	// Extract upgrade/downgrade history (last 10)
	historyLimit := 10
	for i, h := range result.UpgradeDowngradeHistory.History {
		if i >= historyLimit {
			break
		}
		coverage.UpgradeDowngrades = append(coverage.UpgradeDowngrades, UpgradeDowngrade{
			Firm:       h.Firm,
			ToGrade:    h.ToGrade,
			FromGrade:  h.FromGrade,
			Action:     h.Action,
			Date:       time.Unix(h.EpochGradeDate, 0),
			EpochGrade: h.EpochGradeDate,
		})
	}

	return coverage, nil
}

// createDocument creates a document from analyst coverage data
func (w *ASXAnalystCoverageWorker) createDocument(ctx context.Context, data *AnalystCoverage, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# ASX:%s Analyst Coverage - %s\n\n", asxCode, data.CompanyName))
	content.WriteString(fmt.Sprintf("**Last Updated**: %s\n\n", data.LastUpdated.Format("2 Jan 2006 3:04 PM AEST")))

	// Analyst Summary Section
	content.WriteString("## Analyst Summary\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| **Analyst Count** | %d |\n", data.AnalystCount))
	content.WriteString(fmt.Sprintf("| **Consensus Rating** | %s |\n", strings.ToUpper(data.RecommendationKey)))
	content.WriteString(fmt.Sprintf("| **Rating Score** | %.2f (1=Strong Buy, 5=Strong Sell) |\n", data.RecommendationMean))
	content.WriteString("\n")

	// Price Targets Section
	content.WriteString("## Price Targets\n\n")
	content.WriteString("| Target | Price | vs Current |\n")
	content.WriteString("|--------|-------|------------|\n")
	content.WriteString(fmt.Sprintf("| **Current Price** | $%.2f | - |\n", data.CurrentPrice))
	content.WriteString(fmt.Sprintf("| Mean Target | $%.2f | %.1f%% |\n", data.PriceTargetMean, data.UpsidePotential))
	content.WriteString(fmt.Sprintf("| Median Target | $%.2f | %.1f%% |\n", data.PriceTargetMedian,
		((data.PriceTargetMedian-data.CurrentPrice)/data.CurrentPrice)*100))
	content.WriteString(fmt.Sprintf("| High Target | $%.2f | %.1f%% |\n", data.PriceTargetHigh,
		((data.PriceTargetHigh-data.CurrentPrice)/data.CurrentPrice)*100))
	content.WriteString(fmt.Sprintf("| Low Target | $%.2f | %.1f%% |\n", data.PriceTargetLow,
		((data.PriceTargetLow-data.CurrentPrice)/data.CurrentPrice)*100))
	content.WriteString("\n")

	// Recommendation Distribution Section
	content.WriteString("## Recommendation Distribution\n\n")
	content.WriteString("| Rating | Count |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Strong Buy | %d |\n", data.StrongBuy))
	content.WriteString(fmt.Sprintf("| Buy | %d |\n", data.Buy))
	content.WriteString(fmt.Sprintf("| Hold | %d |\n", data.Hold))
	content.WriteString(fmt.Sprintf("| Sell | %d |\n", data.Sell))
	content.WriteString(fmt.Sprintf("| Strong Sell | %d |\n", data.StrongSell))
	content.WriteString("\n")

	// Visual distribution bar
	total := data.StrongBuy + data.Buy + data.Hold + data.Sell + data.StrongSell
	if total > 0 {
		buyPct := float64(data.StrongBuy+data.Buy) / float64(total) * 100
		holdPct := float64(data.Hold) / float64(total) * 100
		sellPct := float64(data.Sell+data.StrongSell) / float64(total) * 100
		content.WriteString(fmt.Sprintf("**Distribution**: Buy %.0f%% | Hold %.0f%% | Sell %.0f%%\n\n", buyPct, holdPct, sellPct))
	}

	// Recent Upgrade/Downgrade History
	if len(data.UpgradeDowngrades) > 0 {
		content.WriteString("## Recent Analyst Actions\n\n")
		content.WriteString("| Date | Firm | Action | From | To |\n")
		content.WriteString("|------|------|--------|------|----|\n")
		for _, ud := range data.UpgradeDowngrades {
			actionLabel := ud.Action
			switch ud.Action {
			case "up":
				actionLabel = "Upgrade"
			case "down":
				actionLabel = "Downgrade"
			case "init":
				actionLabel = "Initiated"
			case "main":
				actionLabel = "Maintained"
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				ud.Date.Format("02 Jan 2006"), ud.Firm, actionLabel, ud.FromGrade, ud.ToGrade))
		}
		content.WriteString("\n")
	}

	// Build tags
	tags := []string{"asx-analyst-coverage", strings.ToLower(asxCode)}
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

	// Build upgrade/downgrade history for metadata
	var udHistory []map[string]interface{}
	for _, ud := range data.UpgradeDowngrades {
		udHistory = append(udHistory, map[string]interface{}{
			"date":       ud.Date.Format("2006-01-02"),
			"firm":       ud.Firm,
			"action":     ud.Action,
			"from_grade": ud.FromGrade,
			"to_grade":   ud.ToGrade,
		})
	}

	// Build metadata
	metadata := map[string]interface{}{
		"asx_code":            asxCode,
		"company_name":        data.CompanyName,
		"analyst_count":       data.AnalystCount,
		"current_price":       data.CurrentPrice,
		"target_mean":         data.PriceTargetMean,
		"target_high":         data.PriceTargetHigh,
		"target_low":          data.PriceTargetLow,
		"target_median":       data.PriceTargetMedian,
		"upside_potential":    data.UpsidePotential,
		"recommendation_mean": data.RecommendationMean,
		"recommendation_key":  data.RecommendationKey,
		"strong_buy":          data.StrongBuy,
		"buy":                 data.Buy,
		"hold":                data.Hold,
		"sell":                data.Sell,
		"strong_sell":         data.StrongSell,
		"upgrade_downgrades":  udHistory,
		"parent_job_id":       parentJobID,
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "asx_analyst_coverage",
		SourceID:        fmt.Sprintf("asx:%s:analyst_coverage", asxCode),
		URL:             fmt.Sprintf("https://finance.yahoo.com/quote/%s.AX/analysis", asxCode),
		Title:           fmt.Sprintf("ASX:%s Analyst Coverage & Price Targets", asxCode),
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
