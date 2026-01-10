// -----------------------------------------------------------------------
// UnifiedPortfolioWorker - Accepts multiple portfolio input types
// -----------------------------------------------------------------------
// Supports:
// 1. ticker_list: List of ticker symbols (e.g., ["BHP.AX", "CBA.AX"])
// 2. navexa_portfolio: Fetch portfolio by name from Navexa
// 3. navexa_portfolio_id: Fetch portfolio by ID from Navexa
// -----------------------------------------------------------------------

package portfolio

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/navexa"
)

// UnifiedPortfolioWorker handles multiple portfolio input types.
type UnifiedPortfolioWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*UnifiedPortfolioWorker)(nil)

// NewUnifiedPortfolioWorker creates a new unified portfolio worker
func NewUnifiedPortfolioWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *UnifiedPortfolioWorker {
	return &UnifiedPortfolioWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypePortfolio
func (w *UnifiedPortfolioWorker) GetType() models.WorkerType {
	return models.WorkerTypePortfolio
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *UnifiedPortfolioWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *UnifiedPortfolioWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("config is required for portfolio worker")
	}

	// Must have one of: tickers, navexa_portfolio_name, or navexa_portfolio_id
	hasTickers := false
	hasNavexaName := false
	hasNavexaID := false

	if tickers, ok := step.Config["tickers"]; ok {
		if tickerList, ok := tickers.([]interface{}); ok && len(tickerList) > 0 {
			hasTickers = true
		} else if tickerList, ok := tickers.([]string); ok && len(tickerList) > 0 {
			hasTickers = true
		}
	}

	if name, ok := step.Config["navexa_portfolio_name"].(string); ok && name != "" {
		hasNavexaName = true
	}

	if id, ok := step.Config["navexa_portfolio_id"]; ok {
		switch v := id.(type) {
		case float64:
			hasNavexaID = v > 0
		case int:
			hasNavexaID = v > 0
		}
	}

	if !hasTickers && !hasNavexaName && !hasNavexaID {
		return fmt.Errorf("config must specify one of: tickers (list), navexa_portfolio_name (string), or navexa_portfolio_id (int)")
	}

	return nil
}

// Init initializes the unified portfolio worker
func (w *UnifiedPortfolioWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for portfolio worker")
	}

	// Determine input type and create work item
	inputType, description := w.determineInputType(stepConfig)

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("input_type", inputType).
		Msg("Unified portfolio worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     fmt.Sprintf("portfolio-%s-%d", inputType, time.Now().UnixNano()),
				Name:   description,
				Type:   "portfolio",
				Config: stepConfig,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"input_type":  inputType,
			"step_config": stepConfig,
		},
	}, nil
}

// determineInputType figures out which input type was provided
func (w *UnifiedPortfolioWorker) determineInputType(config map[string]interface{}) (string, string) {
	// Check for ticker list
	if tickers, ok := config["tickers"]; ok {
		var tickerList []string
		switch v := tickers.(type) {
		case []interface{}:
			for _, t := range v {
				if s, ok := t.(string); ok {
					tickerList = append(tickerList, s)
				}
			}
		case []string:
			tickerList = v
		}
		if len(tickerList) > 0 {
			return navexa.InputTypeTickerList, fmt.Sprintf("Process portfolio with %d tickers", len(tickerList))
		}
	}

	// Check for Navexa portfolio name
	if name, ok := config["navexa_portfolio_name"].(string); ok && name != "" {
		return navexa.InputTypeNavexaPortfolio, fmt.Sprintf("Fetch Navexa portfolio '%s'", name)
	}

	// Check for Navexa portfolio ID
	if id, ok := config["navexa_portfolio_id"]; ok {
		switch v := id.(type) {
		case float64:
			if v > 0 {
				return navexa.InputTypeNavexaPortfolioID, fmt.Sprintf("Fetch Navexa portfolio ID %d", int(v))
			}
		case int:
			if v > 0 {
				return navexa.InputTypeNavexaPortfolioID, fmt.Sprintf("Fetch Navexa portfolio ID %d", v)
			}
		}
	}

	return "unknown", "Unknown portfolio input"
}

// CreateJobs processes the portfolio based on input type
func (w *UnifiedPortfolioWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize portfolio worker: %w", err)
		}
	}

	inputType, _ := initResult.Metadata["input_type"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	switch inputType {
	case navexa.InputTypeTickerList:
		return w.processTickerList(ctx, stepConfig, stepID)
	case navexa.InputTypeNavexaPortfolio:
		return w.processNavexaPortfolioByName(ctx, stepConfig, stepID)
	case navexa.InputTypeNavexaPortfolioID:
		return w.processNavexaPortfolioByID(ctx, stepConfig, stepID)
	default:
		return "", fmt.Errorf("unknown input type: %s", inputType)
	}
}

// processTickerList handles portfolio from a list of tickers
func (w *UnifiedPortfolioWorker) processTickerList(ctx context.Context, stepConfig map[string]interface{}, stepID string) (string, error) {
	// Extract tickers
	var tickers []string
	if tickerList, ok := stepConfig["tickers"].([]interface{}); ok {
		for _, t := range tickerList {
			if s, ok := t.(string); ok {
				tickers = append(tickers, s)
			}
		}
	} else if tickerList, ok := stepConfig["tickers"].([]string); ok {
		tickers = tickerList
	}

	if len(tickers) == 0 {
		return "", fmt.Errorf("no tickers provided")
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Processing portfolio with %d tickers: %s", len(tickers), strings.Join(tickers, ", ")))
	}

	// Extract portfolio name (optional, defaults to "Custom Portfolio")
	portfolioName := "Custom Portfolio"
	if name, ok := stepConfig["name"].(string); ok && name != "" {
		portfolioName = name
	}

	// Build holdings from tickers (minimal info - just symbols)
	holdings := make([]PortfolioHolding, len(tickers))
	for i, ticker := range tickers {
		// Parse ticker for symbol and exchange (e.g., "BHP.AX" -> symbol=BHP, exchange=AX)
		parts := strings.Split(ticker, ".")
		symbol := parts[0]
		exchange := ""
		if len(parts) > 1 {
			exchange = parts[1]
		}

		holdings[i] = PortfolioHolding{
			Symbol:   symbol,
			Name:     symbol, // Will be enriched later if needed
			Exchange: exchange,
		}
	}

	// Generate markdown content
	markdown := w.generateTickerListMarkdown(portfolioName, holdings)

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"portfolio", "ticker-list", dateTag}

	// Add output_tags from step config
	tags = w.appendOutputTags(tags, stepConfig)

	// Build holdings data for metadata
	holdingsData := make([]map[string]interface{}, len(holdings))
	for i, h := range holdings {
		holdingsData[i] = map[string]interface{}{
			"symbol":   h.Symbol,
			"exchange": h.Exchange,
		}
	}

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Portfolio - %s", portfolioName),
		URL:             "",
		SourceType:      "portfolio",
		SourceID:        fmt.Sprintf("portfolio:tickers:%s", portfolioName),
		ContentMarkdown: markdown,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"portfolio": map[string]interface{}{
				"name": portfolioName,
				"type": "ticker_list",
			},
			"holdings":      holdingsData,
			"holding_count": len(holdings),
			"tickers":       tickers,
			"fetched_at":    now.Format(time.RFC3339),
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to store portfolio document: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Portfolio document created for %s with %d tickers", portfolioName, len(tickers)))
	}

	w.logger.Info().
		Str("portfolio_name", portfolioName).
		Int("holding_count", len(holdings)).
		Str("document_id", doc.ID).
		Msg("Ticker list portfolio created")

	return stepID, nil
}

// processNavexaPortfolioByName handles fetching portfolio from Navexa by name
func (w *UnifiedPortfolioWorker) processNavexaPortfolioByName(ctx context.Context, stepConfig map[string]interface{}, stepID string) (string, error) {
	portfolioName, _ := stepConfig["navexa_portfolio_name"].(string)
	if portfolioName == "" {
		return "", fmt.Errorf("navexa_portfolio_name is required")
	}

	// Create Navexa client
	client, err := w.createNavexaClient(ctx, stepConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create Navexa client: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetching Navexa portfolio '%s' with holdings", portfolioName))
	}

	portfolioWithHoldings, err := client.GetPortfolioWithHoldings(ctx, portfolioName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch portfolio with holdings: %w", err)
	}

	return w.storeNavexaPortfolio(ctx, portfolioWithHoldings, stepConfig, stepID)
}

// processNavexaPortfolioByID handles fetching portfolio from Navexa by ID
func (w *UnifiedPortfolioWorker) processNavexaPortfolioByID(ctx context.Context, stepConfig map[string]interface{}, stepID string) (string, error) {
	var portfolioID int
	if id, ok := stepConfig["navexa_portfolio_id"].(float64); ok {
		portfolioID = int(id)
	} else if id, ok := stepConfig["navexa_portfolio_id"].(int); ok {
		portfolioID = id
	}

	if portfolioID <= 0 {
		return "", fmt.Errorf("navexa_portfolio_id must be a positive integer")
	}

	// Create Navexa client
	client, err := w.createNavexaClient(ctx, stepConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create Navexa client: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetching Navexa portfolio ID %d with holdings", portfolioID))
	}

	portfolioWithHoldings, err := client.GetPortfolioByIDWithHoldings(ctx, portfolioID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch portfolio with holdings: %w", err)
	}

	return w.storeNavexaPortfolio(ctx, portfolioWithHoldings, stepConfig, stepID)
}

// storeNavexaPortfolio stores a portfolio fetched from Navexa
func (w *UnifiedPortfolioWorker) storeNavexaPortfolio(ctx context.Context, portfolioWithHoldings *navexa.PortfolioWithHoldings, stepConfig map[string]interface{}, stepID string) (string, error) {
	portfolio := portfolioWithHoldings.Portfolio

	// Convert enriched holdings to PortfolioHolding
	holdings := make([]PortfolioHolding, len(portfolioWithHoldings.Holdings))
	for i, h := range portfolioWithHoldings.Holdings {
		holdings[i] = PortfolioHolding{
			Symbol:        h.Symbol,
			Name:          h.Name,
			Exchange:      h.Exchange,
			Quantity:      h.Quantity,
			AvgBuyPrice:   h.AvgBuyPrice,
			CurrentValue:  h.CurrentValue,
			HoldingWeight: h.HoldingWeight,
			CurrencyCode:  h.CurrencyCode,
		}
	}

	// Generate markdown content
	navexaPortfolio := &NavexaPortfolio{
		ID:               portfolio.ID,
		Name:             portfolio.Name,
		DateCreated:      portfolio.DateCreated,
		BaseCurrencyCode: portfolio.BaseCurrencyCode,
	}
	markdown := w.generateNavexaMarkdown(navexaPortfolio, holdings)

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"portfolio", "navexa", portfolio.Name, dateTag}

	// Add output_tags from step config
	tags = w.appendOutputTags(tags, stepConfig)

	// Build schema-compliant metadata structure
	portfolioData := map[string]interface{}{
		"id":               portfolio.ID,
		"name":             portfolio.Name,
		"dateCreated":      portfolio.DateCreated,
		"baseCurrencyCode": portfolio.BaseCurrencyCode,
	}

	holdingsData := make([]map[string]interface{}, len(holdings))
	for i, h := range holdings {
		holdingsData[i] = map[string]interface{}{
			"symbol":        h.Symbol,
			"name":          h.Name,
			"exchange":      h.Exchange,
			"quantity":      h.Quantity,
			"avgBuyPrice":   h.AvgBuyPrice,
			"currentValue":  h.CurrentValue,
			"holdingWeight": h.HoldingWeight,
			"currencyCode":  h.CurrencyCode,
		}
	}

	baseURL := w.getBaseURL(ctx)
	now := time.Now()

	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Portfolio - %s", portfolio.Name),
		URL:             fmt.Sprintf("%s/v1/portfolios/%d", baseURL, portfolio.ID),
		SourceType:      "portfolio",
		SourceID:        fmt.Sprintf("portfolio:navexa:%d", portfolio.ID),
		ContentMarkdown: markdown,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"portfolio":     portfolioData,
			"holdings":      holdingsData,
			"holding_count": len(holdings),
			"fetched_at":    now.Format(time.RFC3339),
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to store portfolio document: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Portfolio document created for %s with %d holdings", portfolio.Name, len(holdings)))
	}

	w.logger.Info().
		Int("portfolio_id", portfolio.ID).
		Str("portfolio_name", portfolio.Name).
		Int("holding_count", len(holdings)).
		Str("document_id", doc.ID).
		Msg("Navexa portfolio with holdings fetched and stored")

	return stepID, nil
}

// createNavexaClient creates a Navexa API client
func (w *UnifiedPortfolioWorker) createNavexaClient(ctx context.Context, stepConfig map[string]interface{}) (*navexa.Client, error) {
	apiKey, err := w.getAPIKey(ctx, stepConfig)
	if err != nil {
		return nil, err
	}

	baseURL := w.getBaseURL(ctx)

	opts := []navexa.ClientOption{
		navexa.WithBaseURL(baseURL),
		navexa.WithLogger(w.logger),
	}

	return navexa.NewClient(apiKey, opts...), nil
}

// getAPIKey retrieves the Navexa API key from KV storage or step config
func (w *UnifiedPortfolioWorker) getAPIKey(ctx context.Context, stepConfig map[string]interface{}) (string, error) {
	if apiKey, ok := stepConfig["api_key"].(string); ok && apiKey != "" {
		if strings.HasPrefix(apiKey, "{") && strings.HasSuffix(apiKey, "}") {
			keyName := strings.Trim(apiKey, "{}")
			if val, err := w.kvStorage.Get(ctx, keyName); err == nil && val != "" {
				return val, nil
			}
		}
		return apiKey, nil
	}

	if val, err := w.kvStorage.Get(ctx, navexaAPIKeyEnvVar); err == nil && val != "" {
		return val, nil
	}

	return "", fmt.Errorf("navexa_api_key not found in KV storage or step config")
}

// getBaseURL retrieves the Navexa API base URL from KV storage
func (w *UnifiedPortfolioWorker) getBaseURL(ctx context.Context) string {
	if val, err := w.kvStorage.Get(ctx, navexaBaseURLKey); err == nil && val != "" {
		return val
	}
	return navexaDefaultBaseURL
}

// appendOutputTags adds output_tags from step config to tags
func (w *UnifiedPortfolioWorker) appendOutputTags(tags []string, stepConfig map[string]interface{}) []string {
	if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range outputTags {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	} else if outputTags, ok := stepConfig["output_tags"].([]string); ok {
		tags = append(tags, outputTags...)
	}
	return tags
}

// generateTickerListMarkdown creates markdown for a ticker list portfolio
func (w *UnifiedPortfolioWorker) generateTickerListMarkdown(portfolioName string, holdings []PortfolioHolding) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Portfolio - %s\n\n", portfolioName))
	sb.WriteString("## Portfolio Information\n\n")
	sb.WriteString(fmt.Sprintf("- **Type**: Ticker List\n"))
	sb.WriteString(fmt.Sprintf("- **Holdings**: %d\n", len(holdings)))
	sb.WriteString(fmt.Sprintf("- **Created**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM")))

	sb.WriteString(fmt.Sprintf("## Holdings (%d)\n\n", len(holdings)))

	if len(holdings) == 0 {
		sb.WriteString("No holdings.\n")
		return sb.String()
	}

	sb.WriteString("| Symbol | Exchange |\n")
	sb.WriteString("|--------|----------|\n")

	for _, h := range holdings {
		exchange := h.Exchange
		if exchange == "" {
			exchange = "-"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s |\n", h.Symbol, exchange))
	}

	return sb.String()
}

// generateNavexaMarkdown creates markdown for a Navexa portfolio
func (w *UnifiedPortfolioWorker) generateNavexaMarkdown(portfolio *NavexaPortfolio, holdings []PortfolioHolding) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Portfolio - %s\n\n", portfolio.Name))
	sb.WriteString("## Portfolio Information\n\n")
	sb.WriteString(fmt.Sprintf("- **Portfolio ID**: %d\n", portfolio.ID))
	sb.WriteString(fmt.Sprintf("- **Name**: %s\n", portfolio.Name))
	sb.WriteString(fmt.Sprintf("- **Base Currency**: %s\n", portfolio.BaseCurrencyCode))
	sb.WriteString(fmt.Sprintf("- **Created**: %s\n", portfolio.DateCreated))
	sb.WriteString(fmt.Sprintf("- **Fetched**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM")))

	var totalValue float64
	for _, h := range holdings {
		totalValue += h.CurrentValue
	}

	sb.WriteString(fmt.Sprintf("## Holdings (%d)\n\n", len(holdings)))
	sb.WriteString(fmt.Sprintf("**Total Value**: %s\n\n", formatMoney(totalValue)))

	if len(holdings) == 0 {
		sb.WriteString("No holdings found.\n")
		return sb.String()
	}

	sb.WriteString("| Symbol | Name | Qty | Avg Price | Value | Weight |\n")
	sb.WriteString("|--------|------|----:|----------:|------:|-------:|\n")

	for _, h := range holdings {
		symbol := h.Symbol
		if symbol == "" {
			symbol = "-"
		}
		name := h.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %.2f | %s | %s | %.1f%% |\n",
			symbol, name, h.Quantity, formatMoney(h.AvgBuyPrice), formatMoney(h.CurrentValue), h.HoldingWeight))
	}

	// Add weight breakdown by exchange
	sb.WriteString("\n## Exchange Breakdown\n\n")
	exchangeWeights := make(map[string]float64)
	for _, h := range holdings {
		exchange := h.Exchange
		if exchange == "" {
			exchange = "Unknown"
		}
		exchangeWeights[exchange] += h.HoldingWeight
	}

	sb.WriteString("| Exchange | Weight |\n")
	sb.WriteString("|----------|-------:|\n")
	for exchange, weight := range exchangeWeights {
		sb.WriteString(fmt.Sprintf("| %s | %.1f%% |\n", exchange, weight))
	}

	return sb.String()
}
