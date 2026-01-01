// -----------------------------------------------------------------------
// NavexaHoldingsWorker - Fetches holdings for a Navexa portfolio
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

// NavexaHoldingsWorker fetches holdings for a specific Navexa portfolio.
type NavexaHoldingsWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*NavexaHoldingsWorker)(nil)

// NavexaHolding represents a holding from the Navexa API
type NavexaHolding struct {
	ID                int    `json:"id"`
	Symbol            string `json:"symbol"`
	Exchange          string `json:"exchange"`
	Name              string `json:"name"`
	CurrencyCode      string `json:"currencyCode"`
	Sector            string `json:"sector"`
	SectorCode        string `json:"sectorCode"`
	IndustryGroup     string `json:"industryGroup"`
	IndustryGroupCode string `json:"industryGroupCode"`
	Industry          string `json:"industry"`
	IndustryCode      string `json:"industryCode"`
	SubIndustry       string `json:"subIndustry"`
	SubIndustryCode   string `json:"subIndustryCode"`
	PortfolioID       int    `json:"portfolioId"`
	HoldingTypeID     int    `json:"holdingTypeId"`
	DateCreated       string `json:"dateCreated"`
}

// NewNavexaHoldingsWorker creates a new Navexa holdings worker
func NewNavexaHoldingsWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *NavexaHoldingsWorker {
	return &NavexaHoldingsWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeNavexaHoldings
func (w *NavexaHoldingsWorker) GetType() models.WorkerType {
	return models.WorkerTypeNavexaHoldings
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *NavexaHoldingsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *NavexaHoldingsWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("config is required for navexa_holdings")
	}
	if _, ok := step.Config["portfolio_id"]; !ok {
		return fmt.Errorf("portfolio_id is required in config")
	}
	return nil
}

// Init initializes the Navexa holdings worker
func (w *NavexaHoldingsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for navexa_holdings")
	}

	portfolioID, err := w.getPortfolioID(stepConfig)
	if err != nil {
		return nil, err
	}

	portfolioName := "unknown"
	if name, ok := stepConfig["portfolio_name"].(string); ok && name != "" {
		portfolioName = name
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("portfolio_id", portfolioID).
		Str("portfolio_name", portfolioName).
		Msg("Navexa holdings worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     fmt.Sprintf("navexa-holdings-%d", portfolioID),
				Name:   fmt.Sprintf("Fetch holdings for portfolio %s", portfolioName),
				Type:   "navexa_holdings",
				Config: stepConfig,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"portfolio_id":   portfolioID,
			"portfolio_name": portfolioName,
			"step_config":    stepConfig,
		},
	}, nil
}

// CreateJobs fetches holdings from Navexa and stores as document
func (w *NavexaHoldingsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize navexa_holdings worker: %w", err)
		}
	}

	portfolioID, _ := initResult.Metadata["portfolio_id"].(int)
	portfolioName, _ := initResult.Metadata["portfolio_name"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Get API key from KV storage
	apiKey, err := w.getAPIKey(ctx, stepConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get Navexa API key: %w", err)
	}

	// Fetch holdings from Navexa API
	holdings, err := w.fetchHoldings(ctx, apiKey, portfolioID, stepID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Navexa holdings: %w", err)
	}

	// Generate markdown content
	markdown := w.generateMarkdown(holdings, portfolioName, portfolioID)

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"navexa-holdings", portfolioName, dateTag}

	// Add output_tags from step config
	if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range outputTags {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	// Convert holdings to JSON-friendly format for storage
	holdingsData := make([]map[string]interface{}, len(holdings))
	for i, h := range holdings {
		holdingsData[i] = map[string]interface{}{
			"id":                h.ID,
			"symbol":            h.Symbol,
			"exchange":          h.Exchange,
			"name":              h.Name,
			"currencyCode":      h.CurrencyCode,
			"sector":            h.Sector,
			"sectorCode":        h.SectorCode,
			"industryGroup":     h.IndustryGroup,
			"industryGroupCode": h.IndustryGroupCode,
			"industry":          h.Industry,
			"industryCode":      h.IndustryCode,
			"subIndustry":       h.SubIndustry,
			"subIndustryCode":   h.SubIndustryCode,
			"portfolioId":       h.PortfolioID,
			"holdingTypeId":     h.HoldingTypeID,
			"dateCreated":       h.DateCreated,
		}
	}

	// Get base URL for document metadata
	baseURL := w.getBaseURL(ctx)

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Navexa Holdings - %s", portfolioName),
		URL:             fmt.Sprintf("%s/v1/portfolios/%d/holdings", baseURL, portfolioID),
		SourceType:      "navexa_holdings",
		SourceID:        fmt.Sprintf("navexa:portfolio:%d:holdings", portfolioID),
		ContentMarkdown: markdown,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"portfolio_id":   portfolioID,
			"portfolio_name": portfolioName,
			"holdings":       holdingsData,
			"holding_count":  len(holdings),
			"fetched_at":     now.Format(time.RFC3339),
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to store holdings document: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetched %d holdings for portfolio %s", len(holdings), portfolioName))
	}

	w.logger.Info().
		Int("portfolio_id", portfolioID).
		Str("portfolio_name", portfolioName).
		Int("holding_count", len(holdings)).
		Str("document_id", doc.ID).
		Msg("Navexa holdings fetched and stored")

	return stepID, nil
}

// getPortfolioID extracts the portfolio ID from step config
func (w *NavexaHoldingsWorker) getPortfolioID(stepConfig map[string]interface{}) (int, error) {
	if id, ok := stepConfig["portfolio_id"].(float64); ok {
		return int(id), nil
	}
	if id, ok := stepConfig["portfolio_id"].(int); ok {
		return id, nil
	}
	return 0, fmt.Errorf("portfolio_id is required (integer)")
}

// getAPIKey retrieves the Navexa API key from KV storage or step config
func (w *NavexaHoldingsWorker) getAPIKey(ctx context.Context, stepConfig map[string]interface{}) (string, error) {
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
func (w *NavexaHoldingsWorker) getBaseURL(ctx context.Context) string {
	if val, err := w.kvStorage.Get(ctx, navexaBaseURLKey); err == nil && val != "" {
		return val
	}
	return navexaDefaultBaseURL
}

// fetchHoldings fetches holdings from the Navexa API
func (w *NavexaHoldingsWorker) fetchHoldings(ctx context.Context, apiKey string, portfolioID int, stepID string) ([]NavexaHolding, error) {
	baseURL := w.getBaseURL(ctx)
	url := fmt.Sprintf("%s/v1/portfolios/%d/holdings", baseURL, portfolioID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetching holdings for portfolio %d from Navexa API", portfolioID))
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var holdings []NavexaHolding
	if err := json.NewDecoder(resp.Body).Decode(&holdings); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return holdings, nil
}

// generateMarkdown creates a markdown document from the holdings data
func (w *NavexaHoldingsWorker) generateMarkdown(holdings []NavexaHolding, portfolioName string, portfolioID int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Navexa Holdings - %s\n\n", portfolioName))
	sb.WriteString(fmt.Sprintf("**Portfolio ID**: %d\n", portfolioID))
	sb.WriteString(fmt.Sprintf("**Fetched**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM")))
	sb.WriteString(fmt.Sprintf("**Total Holdings**: %d\n\n", len(holdings)))

	if len(holdings) == 0 {
		sb.WriteString("No holdings found.\n")
		return sb.String()
	}

	sb.WriteString("## Holdings\n\n")
	sb.WriteString("| Symbol | Name | Exchange | Sector | Industry |\n")
	sb.WriteString("|--------|------|----------|--------|----------|\n")

	for _, h := range holdings {
		symbol := h.Symbol
		if symbol == "" {
			symbol = "-"
		}
		name := h.Name
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		exchange := h.Exchange
		if exchange == "" {
			exchange = "-"
		}
		sector := h.Sector
		if sector == "" {
			sector = "-"
		}
		industry := h.Industry
		if industry == "" {
			industry = "-"
		}
		if len(industry) > 30 {
			industry = industry[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			symbol, name, exchange, sector, industry))
	}

	// Add sector summary
	sb.WriteString("\n## Sector Breakdown\n\n")
	sectorCounts := make(map[string]int)
	for _, h := range holdings {
		sector := h.Sector
		if sector == "" {
			sector = "Unknown"
		}
		sectorCounts[sector]++
	}

	sb.WriteString("| Sector | Count |\n")
	sb.WriteString("|--------|------:|\n")
	for sector, count := range sectorCounts {
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", sector, count))
	}

	return sb.String()
}
