// -----------------------------------------------------------------------
// NavexaPortfolioWorker - Fetches portfolio from Navexa API by portfolio name
// -----------------------------------------------------------------------
// This worker fetches a specific portfolio from Navexa using the portfolio name,
// enriches it with holdings and performance data, and produces a document
// that can be consumed by the portfolio_review worker.
//
// Uses the Navexa service (internal/services/navexa) for API interactions.
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

// NavexaPortfolioWorker fetches a portfolio by name from Navexa API
// and produces a document with portfolio and holdings data.
type NavexaPortfolioWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*NavexaPortfolioWorker)(nil)

// NewNavexaPortfolioWorker creates a new Navexa portfolio worker
func NewNavexaPortfolioWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *NavexaPortfolioWorker {
	return &NavexaPortfolioWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypeNavexaPortfolio
func (w *NavexaPortfolioWorker) GetType() models.WorkerType {
	return models.WorkerTypeNavexaPortfolio
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *NavexaPortfolioWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *NavexaPortfolioWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("config is required for navexa_portfolio")
	}
	if _, ok := step.Config["name"]; !ok {
		return fmt.Errorf("name is required in config (e.g., name = \"smsf\")")
	}
	return nil
}

// Init initializes the Navexa portfolio worker
func (w *NavexaPortfolioWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for navexa_portfolio")
	}

	portfolioName, ok := stepConfig["name"].(string)
	if !ok || portfolioName == "" {
		return nil, fmt.Errorf("name is required in config (e.g., name = \"smsf\")")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("portfolio_name", portfolioName).
		Msg("Navexa portfolio worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     fmt.Sprintf("navexa-portfolio-%s", portfolioName),
				Name:   fmt.Sprintf("Fetch Navexa portfolio %s with holdings", portfolioName),
				Type:   "navexa_portfolio",
				Config: stepConfig,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"portfolio_name": portfolioName,
			"step_config":    stepConfig,
		},
	}, nil
}

// CreateJobs fetches portfolio by name from Navexa and stores as document.
// Implements document caching: checks for existing fresh document before making API calls.
// Config options:
//   - cache_hours: Freshness window in hours (default: 24)
//   - force_refresh: If true, bypasses cache and always fetches fresh data
func (w *NavexaPortfolioWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize navexa_portfolio worker: %w", err)
		}
	}

	portfolioName, _ := initResult.Metadata["portfolio_name"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Extract cache options from config
	cacheHours := DefaultCacheHours
	if hours, ok := stepConfig["cache_hours"].(float64); ok {
		cacheHours = int(hours)
	} else if hours, ok := stepConfig["cache_hours"].(int); ok {
		cacheHours = hours
	}
	forceRefresh := false
	if fr, ok := stepConfig["force_refresh"].(bool); ok {
		forceRefresh = fr
	}

	// Check for cached document (unless force_refresh is set)
	if !forceRefresh && w.searchService != nil {
		cachedDoc, isFresh := w.getCachedPortfolio(ctx, portfolioName, cacheHours)
		if cachedDoc != nil && isFresh {
			// Re-fetch full document to ensure complete metadata
			fullDoc, err := w.documentStorage.GetDocument(cachedDoc.ID)
			if err != nil {
				w.logger.Warn().Err(err).
					Str("document_id", cachedDoc.ID).
					Msg("Failed to re-fetch cached document - will proceed with API call")
			} else {
				// Update tags on the full document
				if err := w.addOutputTagsToDocument(ctx, fullDoc, stepConfig); err != nil {
					w.logger.Warn().Err(err).
						Str("document_id", fullDoc.ID).
						Msg("Failed to add output_tags to cached document - continuing anyway")
				}

				if w.jobMgr != nil {
					w.jobMgr.AddJobLog(ctx, stepID, "info",
						fmt.Sprintf("Using cached Navexa portfolio document for '%s' (fresh within %d hours)", portfolioName, cacheHours))
				}
				w.logger.Info().
					Str("portfolio_name", portfolioName).
					Str("document_id", fullDoc.ID).
					Int("cache_hours", cacheHours).
					Msg("Using cached Navexa portfolio document - skipping API fetch")
				return stepID, nil
			}
		}
		if cachedDoc != nil && !isFresh {
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("Cached Navexa portfolio document for '%s' is stale - refreshing", portfolioName))
			}
		}
	}

	// Create Navexa client
	client, err := w.createNavexaClient(ctx, stepConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create Navexa client: %w", err)
	}

	// Fetch portfolio with holdings using the Navexa service
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetching Navexa portfolio '%s' with holdings", portfolioName))
	}

	portfolioWithHoldings, err := client.GetPortfolioWithHoldings(ctx, portfolioName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch portfolio with holdings from Navexa: %w", err)
	}

	portfolio := portfolioWithHoldings.Portfolio

	w.logger.Info().
		Int("portfolio_id", portfolio.ID).
		Str("portfolio_name", portfolio.Name).
		Int("holding_count", len(portfolioWithHoldings.Holdings)).
		Msg("Navexa portfolio with holdings fetched successfully")

	// Generate markdown content
	markdown := w.generateNavexaPortfolioMarkdown(&portfolio, portfolioWithHoldings.Holdings)

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"navexa-portfolio", portfolio.Name, dateTag}

	// Add output_tags from step config
	if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range outputTags {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	} else if outputTags, ok := stepConfig["output_tags"].([]string); ok {
		tags = append(tags, outputTags...)
	}

	// Build schema-compliant metadata structure
	portfolioData := map[string]interface{}{
		"id":               portfolio.ID,
		"name":             portfolio.Name,
		"dateCreated":      portfolio.DateCreated,
		"baseCurrencyCode": portfolio.BaseCurrencyCode,
	}

	// Build holdings data with performance metrics
	holdingsData := make([]map[string]interface{}, len(portfolioWithHoldings.Holdings))
	for i, h := range portfolioWithHoldings.Holdings {
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

	// Get base URL for document URL
	baseURL := w.getNavexaBaseURL(ctx)
	now := time.Now()

	// Create document with schema-compliant metadata
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Portfolio - %s", portfolio.Name),
		URL:             fmt.Sprintf("%s/v1/portfolios/%d", baseURL, portfolio.ID),
		SourceType:      "navexa_portfolio",
		SourceID:        fmt.Sprintf("navexa:portfolio:%d", portfolio.ID),
		ContentMarkdown: markdown,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"portfolio":     portfolioData,
			"holdings":      holdingsData,
			"holding_count": len(portfolioWithHoldings.Holdings),
			"fetched_at":    now.Format(time.RFC3339),
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to store Navexa portfolio document: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Navexa portfolio document created for %s with %d holdings", portfolio.Name, len(portfolioWithHoldings.Holdings)))
	}

	w.logger.Info().
		Int("portfolio_id", portfolio.ID).
		Str("portfolio_name", portfolio.Name).
		Int("holding_count", len(portfolioWithHoldings.Holdings)).
		Str("document_id", doc.ID).
		Msg("Navexa portfolio with holdings fetched and stored")

	return stepID, nil
}

// createNavexaClient creates a Navexa API client with the appropriate configuration
func (w *NavexaPortfolioWorker) createNavexaClient(ctx context.Context, stepConfig map[string]interface{}) (*navexa.Client, error) {
	apiKey, err := w.getNavexaAPIKey(ctx, stepConfig)
	if err != nil {
		return nil, err
	}

	baseURL := w.getNavexaBaseURL(ctx)

	opts := []navexa.ClientOption{
		navexa.WithBaseURL(baseURL),
		navexa.WithLogger(w.logger),
	}

	return navexa.NewClient(apiKey, opts...), nil
}

// getNavexaAPIKey retrieves the Navexa API key from KV storage or step config
func (w *NavexaPortfolioWorker) getNavexaAPIKey(ctx context.Context, stepConfig map[string]interface{}) (string, error) {
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

// getNavexaBaseURL retrieves the Navexa API base URL from KV storage or returns default
func (w *NavexaPortfolioWorker) getNavexaBaseURL(ctx context.Context) string {
	if val, err := w.kvStorage.Get(ctx, navexaBaseURLKey); err == nil && val != "" {
		return val
	}
	return navexaDefaultBaseURL
}

// generateNavexaPortfolioMarkdown creates a markdown document from the portfolio and holdings data
func (w *NavexaPortfolioWorker) generateNavexaPortfolioMarkdown(portfolio *navexa.Portfolio, holdings []navexa.EnrichedHolding) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Portfolio - %s\n\n", portfolio.Name))
	sb.WriteString("## Portfolio Information\n\n")
	sb.WriteString(fmt.Sprintf("- **Portfolio ID**: %d\n", portfolio.ID))
	sb.WriteString(fmt.Sprintf("- **Name**: %s\n", portfolio.Name))
	sb.WriteString(fmt.Sprintf("- **Base Currency**: %s\n", portfolio.BaseCurrencyCode))
	sb.WriteString(fmt.Sprintf("- **Created**: %s\n", portfolio.DateCreated))
	sb.WriteString(fmt.Sprintf("- **Fetched**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM")))

	// Calculate total portfolio value
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

// getCachedPortfolio searches for an existing portfolio document and checks its freshness.
func (w *NavexaPortfolioWorker) getCachedPortfolio(ctx context.Context, portfolioName string, cacheHours int) (*models.Document, bool) {
	if w.searchService == nil {
		return nil, false
	}

	// Search for document with navexa-portfolio tag and matching name
	opts := interfaces.SearchOptions{
		Tags:  []string{"navexa-portfolio", portfolioName},
		Limit: 1,
	}

	docs, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		w.logger.Debug().Err(err).Str("portfolio_name", portfolioName).Msg("Failed to search for cached Navexa portfolio document")
		return nil, false
	}

	if len(docs) == 0 {
		return nil, false
	}

	doc := docs[0]

	// Check freshness using fetched_at metadata
	isFresh := w.isNavexaDocumentFresh(doc, cacheHours)

	w.logger.Debug().
		Str("portfolio_name", portfolioName).
		Str("document_id", doc.ID).
		Bool("is_fresh", isFresh).
		Int("cache_hours", cacheHours).
		Msg("Found cached Navexa portfolio document")

	return doc, isFresh
}

// isNavexaDocumentFresh checks if a document is within the freshness window.
func (w *NavexaPortfolioWorker) isNavexaDocumentFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.Metadata == nil {
		return false
	}

	if cacheHours <= 0 {
		return false
	}

	fetchedAtStr, ok := doc.Metadata["fetched_at"].(string)
	if !ok || fetchedAtStr == "" {
		freshnessCutoff := time.Now().Add(-time.Duration(cacheHours) * time.Hour)
		return doc.UpdatedAt.After(freshnessCutoff)
	}

	fetchedAt, err := time.Parse(time.RFC3339, fetchedAtStr)
	if err != nil {
		w.logger.Debug().Err(err).Str("fetched_at", fetchedAtStr).Msg("Failed to parse fetched_at timestamp")
		return false
	}

	freshnessCutoff := time.Now().Add(-time.Duration(cacheHours) * time.Hour)
	return fetchedAt.After(freshnessCutoff)
}

// addOutputTagsToDocument adds output_tags from step config to the document.
func (w *NavexaPortfolioWorker) addOutputTagsToDocument(ctx context.Context, doc *models.Document, stepConfig map[string]interface{}) error {
	// Extract output_tags from step config
	var outputTags []string
	if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				outputTags = append(outputTags, tagStr)
			}
		}
	} else if tags, ok := stepConfig["output_tags"].([]string); ok {
		outputTags = tags
	}

	if len(outputTags) == 0 {
		return nil
	}

	// Check which tags need to be added
	existingTags := make(map[string]bool)
	for _, tag := range doc.Tags {
		existingTags[tag] = true
	}

	var newTags []string
	for _, tag := range outputTags {
		if !existingTags[tag] {
			newTags = append(newTags, tag)
		}
	}

	if len(newTags) == 0 {
		return nil
	}

	doc.Tags = append(doc.Tags, newTags...)

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document with updated tags: %w", err)
	}

	w.logger.Info().
		Str("document_id", doc.ID).
		Strs("added_tags", newTags).
		Msg("Added output_tags to cached Navexa portfolio document")

	return nil
}
