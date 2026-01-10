// -----------------------------------------------------------------------
// PortfoliosWorker - Fetches all portfolios via Navexa API
// -----------------------------------------------------------------------

package portfolio

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

// Navexa API configuration
const (
	navexaDefaultBaseURL = "https://api.navexa.com.au"
	navexaBaseURLKey     = "navexa_base_url"
	navexaAPIKeyEnvVar   = "navexa_api_key"
)

// PortfoliosWorker fetches all portfolios for the authenticated user.
type PortfoliosWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*PortfoliosWorker)(nil)

// NavexaPortfolio represents a portfolio from the Navexa API
type NavexaPortfolio struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	DateCreated      string `json:"dateCreated"`
	BaseCurrencyCode string `json:"baseCurrencyCode"`
}

// NewPortfoliosWorker creates a new portfolio list worker
func NewPortfoliosWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *PortfoliosWorker {
	return &PortfoliosWorker{
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

// GetType returns WorkerTypePortfolioList
func (w *PortfoliosWorker) GetType() models.WorkerType {
	return models.WorkerTypePortfolioList
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *PortfoliosWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *PortfoliosWorker) ValidateConfig(step models.JobStep) error {
	// No required config - API key comes from KV storage
	return nil
}

// Init initializes the portfolio list worker
func (w *PortfoliosWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Msg("Portfolio list worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     "portfolio-list",
				Name:   "Fetch all portfolios",
				Type:   "portfolio_list",
				Config: step.Config,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"step_config": step.Config,
		},
	}, nil
}

// CreateJobs fetches all portfolios from Navexa and stores as document
func (w *PortfoliosWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize portfolio_list worker: %w", err)
		}
	}

	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Get API key from KV storage
	apiKey, err := w.getAPIKey(ctx, stepConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get Navexa API key: %w", err)
	}

	// Fetch portfolios from Navexa API
	portfolios, err := w.fetchPortfolios(ctx, apiKey, stepID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Navexa portfolios: %w", err)
	}

	// Generate markdown content
	markdown := w.generateMarkdown(portfolios)

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"portfolio-list", dateTag}

	// Add output_tags from step config (supports both []interface{} from TOML and []string from inline calls)
	if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range outputTags {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	} else if outputTags, ok := stepConfig["output_tags"].([]string); ok {
		tags = append(tags, outputTags...)
	}

	// Convert portfolios to JSON-friendly format for storage
	portfolioData := make([]map[string]interface{}, len(portfolios))
	for i, p := range portfolios {
		portfolioData[i] = map[string]interface{}{
			"id":               p.ID,
			"name":             p.Name,
			"dateCreated":      p.DateCreated,
			"baseCurrencyCode": p.BaseCurrencyCode,
		}
	}

	// Get base URL for document metadata
	baseURL := w.getBaseURL(ctx)

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           "Portfolio List",
		URL:             baseURL + "/v1/portfolios",
		SourceType:      "portfolio_list",
		SourceID:        "portfolio:list",
		ContentMarkdown: markdown,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"portfolios":      portfolioData,
			"portfolio_count": len(portfolios),
			"fetched_at":      now.Format(time.RFC3339),
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to store portfolios document: %w", err)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetched %d portfolios", len(portfolios)))
	}

	w.logger.Info().
		Int("portfolio_count", len(portfolios)).
		Str("document_id", doc.ID).
		Msg("Portfolio list fetched and stored")

	return stepID, nil
}

// getAPIKey retrieves the Navexa API key from KV storage or step config
func (w *PortfoliosWorker) getAPIKey(ctx context.Context, stepConfig map[string]interface{}) (string, error) {
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
func (w *PortfoliosWorker) getBaseURL(ctx context.Context) string {
	if val, err := w.kvStorage.Get(ctx, navexaBaseURLKey); err == nil && val != "" {
		return val
	}
	return navexaDefaultBaseURL
}

// fetchPortfolios fetches all portfolios from the Navexa API
func (w *PortfoliosWorker) fetchPortfolios(ctx context.Context, apiKey string, stepID string) ([]NavexaPortfolio, error) {
	baseURL := w.getBaseURL(ctx)
	url := baseURL + "/v1/portfolios"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", "Fetching portfolios from Navexa API")
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var portfolios []NavexaPortfolio
	if err := json.NewDecoder(resp.Body).Decode(&portfolios); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return portfolios, nil
}

// generateMarkdown creates a markdown document from the portfolios data
func (w *PortfoliosWorker) generateMarkdown(portfolios []NavexaPortfolio) string {
	var sb strings.Builder

	sb.WriteString("# Portfolio List\n\n")
	sb.WriteString(fmt.Sprintf("**Fetched**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM")))
	sb.WriteString(fmt.Sprintf("**Total Portfolios**: %d\n\n", len(portfolios)))

	if len(portfolios) == 0 {
		sb.WriteString("No portfolios found.\n")
		return sb.String()
	}

	sb.WriteString("## Portfolio List\n\n")
	sb.WriteString("| ID | Name | Currency | Created |\n")
	sb.WriteString("|---:|------|----------|--------|\n")

	for _, p := range portfolios {
		name := p.Name
		if name == "" {
			name = "(unnamed)"
		}
		created := p.DateCreated
		if len(created) > 10 {
			created = created[:10] // Just the date portion
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n",
			p.ID, name, p.BaseCurrencyCode, created))
	}

	return sb.String()
}
