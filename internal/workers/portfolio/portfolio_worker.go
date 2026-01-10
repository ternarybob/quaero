// -----------------------------------------------------------------------
// PortfolioWorker - Fetches a specific portfolio with holdings via Navexa API
// Uses performance API to get holdings with quantity, value, and weight data
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

// PortfolioHolding represents a holding with performance data for portfolio output
type PortfolioHolding struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Exchange      string  `json:"exchange"`
	Quantity      float64 `json:"quantity"`
	AvgBuyPrice   float64 `json:"avgBuyPrice"`
	CurrentValue  float64 `json:"currentValue"`
	HoldingWeight float64 `json:"holdingWeight"`
	CurrencyCode  string  `json:"currencyCode"`
}

// DefaultCacheHours is the default freshness window for cached portfolio documents
const DefaultCacheHours = 24

// PortfolioWorker fetches a specific portfolio with its holdings via Navexa API.
// Unlike PortfoliosWorker (lists all) or HoldingsWorker (needs ID),
// this worker accepts a portfolio name and returns the complete portfolio document.
// Implements document caching with freshness checking to avoid redundant API calls.
type PortfolioWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*PortfolioWorker)(nil)

// NewPortfolioWorker creates a new portfolio fetch worker
func NewPortfolioWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *PortfolioWorker {
	return &PortfolioWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypePortfolioFetch
func (w *PortfolioWorker) GetType() models.WorkerType {
	return models.WorkerTypePortfolioFetch
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *PortfolioWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *PortfolioWorker) ValidateConfig(step models.JobStep) error {
	// name is required to identify the portfolio
	if step.Config == nil {
		return fmt.Errorf("config is required for portfolio_fetch")
	}
	if _, ok := step.Config["name"]; !ok {
		return fmt.Errorf("name is required in config (e.g., name = \"smsf\")")
	}
	return nil
}

// Init initializes the Navexa portfolio worker
func (w *PortfolioWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for portfolio_fetch")
	}

	portfolioName, ok := stepConfig["name"].(string)
	if !ok || portfolioName == "" {
		return nil, fmt.Errorf("name is required in config (e.g., name = \"smsf\")")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("portfolio_name", portfolioName).
		Msg("Portfolio fetch worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     fmt.Sprintf("portfolio-fetch-%s", portfolioName),
				Name:   fmt.Sprintf("Fetch portfolio %s with holdings", portfolioName),
				Type:   "portfolio_fetch",
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

// CreateJobs fetches portfolio by name from Navexa, then fetches its holdings, and stores as document.
// Implements document caching: checks for existing fresh document before making API calls.
// Config options:
//   - cache_hours: Freshness window in hours (default: 24)
//   - force_refresh: If true, bypasses cache and always fetches fresh data
func (w *PortfolioWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize portfolio_fetch worker: %w", err)
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
			// Add output_tags from step config to the cached document for pipeline tag routing
			// IMPORTANT: Must re-fetch the document from storage to ensure we have complete metadata
			// The search service may return a document without all fields populated
			fullDoc, err := w.documentStorage.GetDocument(cachedDoc.ID)
			if err != nil {
				w.logger.Warn().Err(err).
					Str("document_id", cachedDoc.ID).
					Msg("Failed to re-fetch cached document - will proceed with API call")
				// Fall through to API fetch below
			} else {
				// Update tags on the full document
				if err := w.addOutputTagsToDocument(ctx, fullDoc, stepConfig); err != nil {
					w.logger.Warn().Err(err).
						Str("document_id", fullDoc.ID).
						Msg("Failed to add output_tags to cached document - continuing anyway")
				}

				if w.jobMgr != nil {
					w.jobMgr.AddJobLog(ctx, stepID, "info",
						fmt.Sprintf("Using cached portfolio document for '%s' (fresh within %d hours)", portfolioName, cacheHours))
				}
				w.logger.Info().
					Str("portfolio_name", portfolioName).
					Str("document_id", fullDoc.ID).
					Int("cache_hours", cacheHours).
					Msg("Using cached portfolio document - skipping API fetch")
				return stepID, nil
			}
		}
		if cachedDoc != nil && !isFresh {
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("Cached portfolio document for '%s' is stale - refreshing", portfolioName))
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
			fmt.Sprintf("Fetching portfolio '%s' with holdings", portfolioName))
	}

	portfolioWithHoldings, err := client.GetPortfolioWithHoldings(ctx, portfolioName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch portfolio with holdings: %w", err)
	}

	matchedPortfolio := &NavexaPortfolio{
		ID:               portfolioWithHoldings.Portfolio.ID,
		Name:             portfolioWithHoldings.Portfolio.Name,
		DateCreated:      portfolioWithHoldings.Portfolio.DateCreated,
		BaseCurrencyCode: portfolioWithHoldings.Portfolio.BaseCurrencyCode,
	}

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

	w.logger.Info().
		Int("portfolio_id", matchedPortfolio.ID).
		Str("portfolio_name", matchedPortfolio.Name).
		Int("holding_count", len(holdings)).
		Msg("Portfolio with holdings fetched successfully")

	// Generate markdown content
	markdown := w.generateMarkdown(matchedPortfolio, holdings)

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"portfolio-fetch", matchedPortfolio.Name, dateTag}

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
		"id":               matchedPortfolio.ID,
		"name":             matchedPortfolio.Name,
		"dateCreated":      matchedPortfolio.DateCreated,
		"baseCurrencyCode": matchedPortfolio.BaseCurrencyCode,
	}

	// Build holdings data with performance metrics (quantity, avgBuyPrice, holdingWeight)
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

	// Get base URL for document URL
	baseURL := w.getBaseURL(ctx)
	now := time.Now()

	// Create document with schema-compliant metadata
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Portfolio - %s", matchedPortfolio.Name),
		URL:             fmt.Sprintf("%s/v1/portfolios/%d", baseURL, matchedPortfolio.ID),
		SourceType:      "portfolio_fetch",
		SourceID:        fmt.Sprintf("portfolio:%d", matchedPortfolio.ID),
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
			fmt.Sprintf("Portfolio document created for %s with %d holdings", matchedPortfolio.Name, len(holdings)))
	}

	w.logger.Info().
		Int("portfolio_id", matchedPortfolio.ID).
		Str("portfolio_name", matchedPortfolio.Name).
		Int("holding_count", len(holdings)).
		Str("document_id", doc.ID).
		Msg("Portfolio with holdings fetched and stored")

	return stepID, nil
}

// createNavexaClient creates a Navexa API client with the appropriate configuration
func (w *PortfolioWorker) createNavexaClient(ctx context.Context, stepConfig map[string]interface{}) (*navexa.Client, error) {
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
func (w *PortfolioWorker) getAPIKey(ctx context.Context, stepConfig map[string]interface{}) (string, error) {
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
func (w *PortfolioWorker) getBaseURL(ctx context.Context) string {
	if val, err := w.kvStorage.Get(ctx, navexaBaseURLKey); err == nil && val != "" {
		return val
	}
	return navexaDefaultBaseURL
}

// generateMarkdown creates a markdown document from the portfolio and holdings data
func (w *PortfolioWorker) generateMarkdown(portfolio *NavexaPortfolio, holdings []PortfolioHolding) string {
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
// Returns the document and a boolean indicating if it's fresh (within cacheHours).
// Returns (nil, false) if no document is found.
func (w *PortfolioWorker) getCachedPortfolio(ctx context.Context, portfolioName string, cacheHours int) (*models.Document, bool) {
	if w.searchService == nil {
		return nil, false
	}

	// Search for document with portfolio-fetch tag and matching name
	opts := interfaces.SearchOptions{
		Tags:  []string{"portfolio-fetch", portfolioName},
		Limit: 1,
	}

	docs, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		w.logger.Debug().Err(err).Str("portfolio_name", portfolioName).Msg("Failed to search for cached portfolio document")
		return nil, false
	}

	if len(docs) == 0 {
		return nil, false
	}

	doc := docs[0]

	// Check freshness using fetched_at metadata
	isFresh := w.isDocumentFresh(doc, cacheHours)

	w.logger.Debug().
		Str("portfolio_name", portfolioName).
		Str("document_id", doc.ID).
		Bool("is_fresh", isFresh).
		Int("cache_hours", cacheHours).
		Msg("Found cached portfolio document")

	return doc, isFresh
}

// isDocumentFresh checks if a document is within the freshness window.
// Uses the "fetched_at" metadata field to determine age.
func (w *PortfolioWorker) isDocumentFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.Metadata == nil {
		return false
	}

	// If cache_hours is 0, always consider stale (force refresh)
	if cacheHours <= 0 {
		return false
	}

	// Check fetched_at metadata field
	fetchedAtStr, ok := doc.Metadata["fetched_at"].(string)
	if !ok || fetchedAtStr == "" {
		// No fetched_at field - fall back to UpdatedAt
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
// This ensures pipeline tag routing works even when using cached documents.
func (w *PortfolioWorker) addOutputTagsToDocument(ctx context.Context, doc *models.Document, stepConfig map[string]interface{}) error {
	// Log document state before update
	w.logger.Debug().
		Str("document_id", doc.ID).
		Bool("has_metadata", doc.Metadata != nil).
		Int("metadata_keys", len(doc.Metadata)).
		Strs("existing_tags", doc.Tags).
		Msg("addOutputTagsToDocument: Document state before update")

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

	// If no output_tags, nothing to do
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

	// If all tags already exist, nothing to do
	if len(newTags) == 0 {
		return nil
	}

	// Add new tags to document
	doc.Tags = append(doc.Tags, newTags...)

	// Log document state after update
	w.logger.Debug().
		Str("document_id", doc.ID).
		Bool("has_metadata", doc.Metadata != nil).
		Int("metadata_keys", len(doc.Metadata)).
		Strs("new_tags", doc.Tags).
		Msg("addOutputTagsToDocument: Document state after adding tags")

	// Save the updated document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document with updated tags: %w", err)
	}

	w.logger.Info().
		Str("document_id", doc.ID).
		Strs("added_tags", newTags).
		Strs("final_tags", doc.Tags).
		Msg("Added output_tags to cached portfolio document")

	return nil
}
