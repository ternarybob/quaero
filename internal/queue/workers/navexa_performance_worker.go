// -----------------------------------------------------------------------
// NavexaPerformanceWorker - Fetches P/L performance for a Navexa portfolio
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// NavexaPerformanceWorker fetches P/L performance for a specific Navexa portfolio.
type NavexaPerformanceWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*NavexaPerformanceWorker)(nil)

// NavexaTotalReturn represents the TotalReturnViewModel from Navexa API
type NavexaTotalReturn struct {
	TotalValue          float64 `json:"totalValue"`
	TotalReturnValue    float64 `json:"totalReturnValue"`
	TotalReturnPercent  float64 `json:"totalReturnPercent"`
	CapitalGainValue    float64 `json:"capitalGainValue"`
	CapitalGainPercent  float64 `json:"capitalGainPercent"`
	DividendReturnValue float64 `json:"dividendReturnValue"`
	DividendReturnPct   float64 `json:"dividendReturnPercent"`
	CurrencyGainValue   float64 `json:"currencyGainValue"`
	CurrencyGainPercent float64 `json:"currencyGainPercent"`
	IsAnnualized        bool    `json:"isAnnualized"`
}

// NavexaPerformanceRaw is the raw API response structure matching Navexa's PortfolioPerformanceViewModel
type NavexaPerformanceRaw struct {
	PortfolioID      int                           `json:"portfolioId"`
	PortfolioName    string                        `json:"portfolioName"`
	BaseCurrencyCode string                        `json:"baseCurrencyCode"`
	TotalValue       float64                       `json:"totalValue"`
	TotalReturn      NavexaTotalReturn             `json:"totalReturn"`
	GeneratedDate    string                        `json:"generatedDate"`
	Holdings         []NavexaHoldingPerformanceRaw `json:"holdings"`
	HasHoldings      bool                          `json:"hasHoldings"`
}

// NavexaHoldingPerformanceRaw matches HoldingSummaryPerformanceViewModel
type NavexaHoldingPerformanceRaw struct {
	Symbol         string            `json:"symbol"`
	Name           string            `json:"name"`
	Exchange       string            `json:"exchange"`
	DisplayExch    string            `json:"displayExchange"`
	Quantity       float64           `json:"quantity"`
	TotalQuantity  float64           `json:"totalQuantity"`
	CurrentPrice   float64           `json:"currentPrice"`
	PercentChange  float64           `json:"percentChange"`
	CurrencyCode   string            `json:"currencyCode"`
	HoldingWeight  float64           `json:"holdingWeight"`
	TotalReturn    NavexaTotalReturn `json:"totalReturn"`
	GroupedByValue string            `json:"groupedByValue"`
}

// NavexaPerformance represents portfolio performance (normalized for internal use)
type NavexaPerformance struct {
	PortfolioID      int                        `json:"portfolioId"`
	PortfolioName    string                     `json:"portfolioName"`
	BaseCurrencyCode string                     `json:"baseCurrencyCode"`
	Holdings         []NavexaHoldingPerformance `json:"holdings"`
	TotalValue       float64                    `json:"totalValue"`
	TotalCostBasis   float64                    `json:"totalCostBasis"`
	TotalReturn      float64                    `json:"totalReturn"`
	TotalReturnPct   float64                    `json:"totalReturnPercent"`
	CapitalGains     float64                    `json:"capitalGains"`
	Dividends        float64                    `json:"dividends"`
	CurrencyGains    float64                    `json:"currencyGains"`
	GeneratedAt      string                     `json:"generatedAt"`
}

// NavexaHoldingPerformance represents individual holding performance (normalized)
type NavexaHoldingPerformance struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Exchange      string  `json:"exchange"`
	CurrentValue  float64 `json:"currentValue"`
	CostBasis     float64 `json:"costBasis"`
	Return        float64 `json:"return"`
	ReturnPercent float64 `json:"returnPercent"`
	Weight        float64 `json:"weight"`
	CapitalGains  float64 `json:"capitalGains"`
	Dividends     float64 `json:"dividends"`
	CurrencyGains float64 `json:"currencyGains"`
	Units         float64 `json:"units"`
	AvgCost       float64 `json:"averageCost"`
}

// NewNavexaPerformanceWorker creates a new Navexa performance worker
func NewNavexaPerformanceWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *NavexaPerformanceWorker {
	return &NavexaPerformanceWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		debugEnabled: debugEnabled,
	}
}

// GetType returns WorkerTypeNavexaPerformance
func (w *NavexaPerformanceWorker) GetType() models.WorkerType {
	return models.WorkerTypeNavexaPerformance
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *NavexaPerformanceWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *NavexaPerformanceWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("config is required for navexa_performance")
	}
	if _, ok := step.Config["portfolio_id"]; !ok {
		return fmt.Errorf("portfolio_id is required in config")
	}
	return nil
}

// Init initializes the Navexa performance worker
func (w *NavexaPerformanceWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for navexa_performance")
	}

	portfolioID, err := w.getPortfolioID(stepConfig)
	if err != nil {
		return nil, err
	}

	portfolioName := "unknown"
	if name, ok := stepConfig["portfolio_name"].(string); ok && name != "" {
		portfolioName = name
	}

	// Default date range: 1 year
	now := time.Now()
	fromDate := now.AddDate(-1, 0, 0).Format("2006-01-02")
	toDate := now.Format("2006-01-02")

	if from, ok := stepConfig["from"].(string); ok && from != "" {
		fromDate = from
	}
	if to, ok := stepConfig["to"].(string); ok && to != "" {
		toDate = to
	}

	groupBy := "holding"
	if gb, ok := stepConfig["group_by"].(string); ok && gb != "" {
		groupBy = gb
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("portfolio_id", portfolioID).
		Str("portfolio_name", portfolioName).
		Str("from", fromDate).
		Str("to", toDate).
		Str("group_by", groupBy).
		Msg("Navexa performance worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     fmt.Sprintf("navexa-performance-%d", portfolioID),
				Name:   fmt.Sprintf("Fetch performance for portfolio %s", portfolioName),
				Type:   "navexa_performance",
				Config: stepConfig,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"portfolio_id":   portfolioID,
			"portfolio_name": portfolioName,
			"from":           fromDate,
			"to":             toDate,
			"group_by":       groupBy,
			"step_config":    stepConfig,
		},
	}, nil
}

// CreateJobs fetches performance from Navexa and stores as document
func (w *NavexaPerformanceWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize navexa_performance worker: %w", err)
		}
	}

	portfolioID, _ := initResult.Metadata["portfolio_id"].(int)
	portfolioName, _ := initResult.Metadata["portfolio_name"].(string)
	fromDate, _ := initResult.Metadata["from"].(string)
	toDate, _ := initResult.Metadata["to"].(string)
	groupBy, _ := initResult.Metadata["group_by"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Get API key from KV storage
	apiKey, err := w.getAPIKey(ctx, stepConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get Navexa API key: %w", err)
	}

	// Fetch performance from Navexa API
	performance, err := w.fetchPerformance(ctx, apiKey, portfolioID, fromDate, toDate, groupBy, stepID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Navexa performance: %w", err)
	}

	// Use API name if available
	if performance.PortfolioName != "" {
		portfolioName = performance.PortfolioName
	}

	// Generate markdown content
	markdown := w.generateMarkdown(performance, portfolioName, portfolioID, fromDate, toDate)

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"navexa-performance", portfolioName, dateTag}

	// Add output_tags from step config
	if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range outputTags {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	// Convert holdings performance to JSON-friendly format
	holdingsData := make([]map[string]interface{}, len(performance.Holdings))
	for i, h := range performance.Holdings {
		holdingsData[i] = map[string]interface{}{
			"symbol":        h.Symbol,
			"name":          h.Name,
			"exchange":      h.Exchange,
			"currentValue":  h.CurrentValue,
			"costBasis":     h.CostBasis,
			"return":        h.Return,
			"returnPercent": h.ReturnPercent,
			"weight":        h.Weight,
			"capitalGains":  h.CapitalGains,
			"dividends":     h.Dividends,
			"currencyGains": h.CurrencyGains,
			"units":         h.Units,
			"averageCost":   h.AvgCost,
		}
	}

	// Create performance data map for storage
	performanceData := map[string]interface{}{
		"portfolioId":        performance.PortfolioID,
		"portfolioName":      performance.PortfolioName,
		"baseCurrencyCode":   performance.BaseCurrencyCode,
		"holdings":           holdingsData,
		"totalValue":         performance.TotalValue,
		"totalCostBasis":     performance.TotalCostBasis,
		"totalReturn":        performance.TotalReturn,
		"totalReturnPercent": performance.TotalReturnPct,
		"capitalGains":       performance.CapitalGains,
		"dividends":          performance.Dividends,
		"currencyGains":      performance.CurrencyGains,
		"generatedAt":        performance.GeneratedAt,
	}

	// Get base URL for document metadata
	baseURL := w.getBaseURL(ctx)

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Navexa Performance - %s", portfolioName),
		URL:             fmt.Sprintf("%s/v1/portfolios/%d/performance", baseURL, portfolioID),
		SourceType:      "navexa_performance",
		SourceID:        fmt.Sprintf("navexa:portfolio:%d:performance", portfolioID),
		ContentMarkdown: markdown,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"portfolio_id":     portfolioID,
			"portfolio_name":   portfolioName,
			"from":             fromDate,
			"to":               toDate,
			"performance":      performanceData,
			"total_value":      performance.TotalValue,
			"total_return":     performance.TotalReturn,
			"total_return_pct": performance.TotalReturnPct,
			"holding_count":    len(performance.Holdings),
			"fetched_at":       now.Format(time.RFC3339),
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to store performance document: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetched performance for %s: %s%.2f (%.2f%%)",
				portfolioName, performance.BaseCurrencyCode, performance.TotalReturn, performance.TotalReturnPct))
	}

	w.logger.Info().
		Int("portfolio_id", portfolioID).
		Str("portfolio_name", portfolioName).
		Float64("total_value", performance.TotalValue).
		Float64("total_return", performance.TotalReturn).
		Str("document_id", doc.ID).
		Msg("Navexa performance fetched and stored")

	return stepID, nil
}

// getPortfolioID extracts the portfolio ID from step config
func (w *NavexaPerformanceWorker) getPortfolioID(stepConfig map[string]interface{}) (int, error) {
	if id, ok := stepConfig["portfolio_id"].(float64); ok {
		return int(id), nil
	}
	if id, ok := stepConfig["portfolio_id"].(int); ok {
		return id, nil
	}
	return 0, fmt.Errorf("portfolio_id is required (integer)")
}

// getAPIKey retrieves the Navexa API key from KV storage or step config
func (w *NavexaPerformanceWorker) getAPIKey(ctx context.Context, stepConfig map[string]interface{}) (string, error) {
	// Check step config first
	if apiKey, ok := stepConfig["api_key"].(string); ok && apiKey != "" {
		// Check if it's a KV placeholder like {navexa_api_key}
		if strings.HasPrefix(apiKey, "{") && strings.HasSuffix(apiKey, "}") {
			keyName := strings.Trim(apiKey, "{}")
			if val, err := w.kvStorage.Get(ctx, keyName); err == nil && val != "" {
				return val, nil
			}
		}
		return apiKey, nil
	}

	// Try default key from KV storage
	if val, err := w.kvStorage.Get(ctx, navexaAPIKeyEnvVar); err == nil && val != "" {
		return val, nil
	}

	return "", fmt.Errorf("navexa_api_key not found in KV storage or step config")
}

// getBaseURL retrieves the Navexa API base URL from KV storage or returns default
func (w *NavexaPerformanceWorker) getBaseURL(ctx context.Context) string {
	if val, err := w.kvStorage.Get(ctx, navexaBaseURLKey); err == nil && val != "" {
		return val
	}
	return navexaDefaultBaseURL
}

// fetchPerformance fetches performance from the Navexa API
func (w *NavexaPerformanceWorker) fetchPerformance(ctx context.Context, apiKey string, portfolioID int, fromDate, toDate, groupBy string, stepID string) (*NavexaPerformance, error) {
	apiBaseURL := w.getBaseURL(ctx)
	baseURL := fmt.Sprintf("%s/v1/portfolios/%d/performance", apiBaseURL, portfolioID)

	// Build query parameters
	params := url.Values{}
	params.Set("from", fromDate)
	params.Set("to", toDate)
	params.Set("isPortfolioGroup", "false")
	params.Set("groupBy", groupBy)
	params.Set("showLocalCurrency", "false")

	fullURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetching performance for portfolio %d (%s to %s)", portfolioID, fromDate, toDate))
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse raw response matching Navexa API structure
	var raw NavexaPerformanceRaw
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Normalize holdings from raw API response
	holdings := make([]NavexaHoldingPerformance, len(raw.Holdings))
	for i, h := range raw.Holdings {
		// Calculate cost basis from total value and return
		costBasis := h.TotalReturn.TotalValue - h.TotalReturn.TotalReturnValue

		holdings[i] = NavexaHoldingPerformance{
			Symbol:        h.Symbol,
			Name:          h.Name,
			Exchange:      h.Exchange,
			CurrentValue:  h.TotalReturn.TotalValue,
			CostBasis:     costBasis,
			Return:        h.TotalReturn.TotalReturnValue,
			ReturnPercent: h.TotalReturn.TotalReturnPercent,
			Weight:        h.HoldingWeight,
			CapitalGains:  h.TotalReturn.CapitalGainValue,
			Dividends:     h.TotalReturn.DividendReturnValue,
			CurrencyGains: h.TotalReturn.CurrencyGainValue,
			Units:         h.TotalQuantity,
			AvgCost:       0, // Not directly available in API response
		}
	}

	// Calculate total cost basis from total value and return
	totalCostBasis := raw.TotalReturn.TotalValue - raw.TotalReturn.TotalReturnValue

	// Normalize to NavexaPerformance
	performance := &NavexaPerformance{
		PortfolioID:      raw.PortfolioID,
		PortfolioName:    raw.PortfolioName,
		BaseCurrencyCode: raw.BaseCurrencyCode,
		Holdings:         holdings,
		TotalValue:       raw.TotalReturn.TotalValue,
		TotalCostBasis:   totalCostBasis,
		TotalReturn:      raw.TotalReturn.TotalReturnValue,
		TotalReturnPct:   raw.TotalReturn.TotalReturnPercent,
		CapitalGains:     raw.TotalReturn.CapitalGainValue,
		Dividends:        raw.TotalReturn.DividendReturnValue,
		CurrencyGains:    raw.TotalReturn.CurrencyGainValue,
		GeneratedAt:      raw.GeneratedDate,
	}

	return performance, nil
}

// formatMoney formats a float as currency with comma thousands separators
func formatMoney(val float64) string {
	neg := val < 0
	if neg {
		val = -val
	}

	// Format with 2 decimal places
	str := fmt.Sprintf("%.2f", val)
	parts := strings.Split(str, ".")

	// Add commas to integer part
	intPart := parts[0]
	var result strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}

	formatted := "$" + result.String() + "." + parts[1]
	if neg {
		formatted = "-" + formatted
	}
	return formatted
}

// formatMoneyInt formats a float as currency with no decimals
func formatMoneyInt(val float64) string {
	neg := val < 0
	if neg {
		val = -val
	}

	// Format with 0 decimal places
	intPart := fmt.Sprintf("%.0f", val)

	// Add commas
	var result strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}

	formatted := "$" + result.String()
	if neg {
		formatted = "-" + formatted
	}
	return formatted
}

// generateMarkdown creates a markdown document from the performance data
func (w *NavexaPerformanceWorker) generateMarkdown(perf *NavexaPerformance, portfolioName string, portfolioID int, fromDate, toDate string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Navexa Performance - %s\n\n", portfolioName))
	sb.WriteString(fmt.Sprintf("**Portfolio ID**: %d\n", portfolioID))
	sb.WriteString(fmt.Sprintf("**Period**: %s to %s\n", fromDate, toDate))
	sb.WriteString(fmt.Sprintf("**Currency**: %s\n", perf.BaseCurrencyCode))
	sb.WriteString(fmt.Sprintf("**Fetched**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM")))

	// Portfolio Summary
	sb.WriteString("## Portfolio Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|------:|\n")
	sb.WriteString(fmt.Sprintf("| Total Value | %s |\n", formatMoney(perf.TotalValue)))
	sb.WriteString(fmt.Sprintf("| Cost Basis | %s |\n", formatMoney(perf.TotalCostBasis)))
	sb.WriteString(fmt.Sprintf("| Total Return | %s |\n", formatMoney(perf.TotalReturn)))
	sb.WriteString(fmt.Sprintf("| Return %% | %.2f%% |\n", perf.TotalReturnPct))
	sb.WriteString(fmt.Sprintf("| Capital Gains | %s |\n", formatMoney(perf.CapitalGains)))
	sb.WriteString(fmt.Sprintf("| Dividends | %s |\n", formatMoney(perf.Dividends)))
	sb.WriteString(fmt.Sprintf("| Currency Gains | %s |\n", formatMoney(perf.CurrencyGains)))

	if len(perf.Holdings) == 0 {
		sb.WriteString("\nNo holding performance data available.\n")
		return sb.String()
	}

	// Holdings Performance
	sb.WriteString("\n## Holdings Performance\n\n")
	sb.WriteString("| Symbol | Name | Value | Cost | P/L | Return % | Weight |\n")
	sb.WriteString("|--------|------|------:|-----:|----:|--------:|-------:|\n")

	for _, h := range perf.Holdings {
		symbol := h.Symbol
		if symbol == "" {
			symbol = "-"
		}
		name := h.Name
		if len(name) > 25 {
			name = name[:22] + "..."
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %.1f%% | %.1f%% |\n",
			symbol, name, formatMoneyInt(h.CurrentValue), formatMoneyInt(h.CostBasis),
			formatMoneyInt(h.Return), h.ReturnPercent, h.Weight))
	}

	// Top/Bottom performers
	if len(perf.Holdings) > 3 {
		sb.WriteString("\n## Top Performers\n\n")
		sb.WriteString("| Symbol | Return % |\n")
		sb.WriteString("|--------|--------:|\n")

		// Find top 3 by return percent
		sorted := make([]NavexaHoldingPerformance, len(perf.Holdings))
		copy(sorted, perf.Holdings)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].ReturnPercent > sorted[i].ReturnPercent {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		for i := 0; i < 3 && i < len(sorted); i++ {
			sb.WriteString(fmt.Sprintf("| %s | %.1f%% |\n", sorted[i].Symbol, sorted[i].ReturnPercent))
		}

		sb.WriteString("\n## Bottom Performers\n\n")
		sb.WriteString("| Symbol | Return % |\n")
		sb.WriteString("|--------|--------:|\n")
		for i := len(sorted) - 1; i >= len(sorted)-3 && i >= 0; i-- {
			sb.WriteString(fmt.Sprintf("| %s | %.1f%% |\n", sorted[i].Symbol, sorted[i].ReturnPercent))
		}
	}

	return sb.String()
}
