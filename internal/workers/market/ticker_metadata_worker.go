// -----------------------------------------------------------------------
// TickerMetadataWorker - Fetches company metadata from EODHD fundamentals
// Creates a metadata document per ticker with company profile information.
// -----------------------------------------------------------------------

package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// TickerMetadataWorker fetches company metadata from EODHD fundamentals API.
type TickerMetadataWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*TickerMetadataWorker)(nil)

// CompanyMetadata holds extracted company information
type CompanyMetadata struct {
	Ticker          common.Ticker
	CompanyName     string
	Description     string
	ISIN            string
	Sector          string
	Industry        string
	Country         string
	Address         string
	Phone           string
	Website         string
	LogoURL         string
	IPODate         string
	Employees       int
	Currency        string
	MarketCap       float64
	EnterpriseValue float64
	PERatio         float64
	DividendYield   float64
	Directors       []OfficerEntry
	Management      []OfficerEntry
	FetchedAt       time.Time
}

// OfficerEntry represents a company officer (director or management)
type OfficerEntry struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// NewTickerMetadataWorker creates a new ticker metadata worker
func NewTickerMetadataWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *TickerMetadataWorker {
	return &TickerMetadataWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypeTickerMetadata
func (w *TickerMetadataWorker) GetType() models.WorkerType {
	return models.WorkerTypeTickerMetadata
}

// Init initializes the ticker metadata worker
func (w *TickerMetadataWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Collect tickers from step config and job-level variables
	tickers := workerutil.CollectTickersWithJobDef(stepConfig, jobDef)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers specified - provide 'ticker', 'tickers', or config.variables")
	}

	// Cache hours (default: 168 = 7 days for stable metadata)
	cacheHours := workerutil.GetIntConfig(stepConfig, "cache_hours", 168)

	// Force refresh
	forceRefresh := workerutil.GetBool(stepConfig, "force_refresh")

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Int("cache_hours", cacheHours).
		Bool("force_refresh", forceRefresh).
		Msg("Ticker metadata worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, 0, len(tickers))
	for _, ticker := range tickers {
		workItems = append(workItems, interfaces.WorkItem{
			ID:   ticker.String(),
			Name: fmt.Sprintf("Fetch metadata for %s", ticker.String()),
			Type: "ticker_metadata",
			Config: map[string]interface{}{
				"ticker": ticker.String(),
			},
		})
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(workItems),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"tickers":       tickers,
			"cache_hours":   cacheHours,
			"force_refresh": forceRefresh,
			"step_config":   stepConfig,
		},
	}, nil
}

// CreateJobs fetches metadata for all tickers and creates documents
func (w *TickerMetadataWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize ticker_metadata worker: %w", err)
		}
	}

	// Get manager_id for job isolation
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	cacheHours, _ := initResult.Metadata["cache_hours"].(int)
	forceRefresh, _ := initResult.Metadata["force_refresh"].(bool)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Extract output tags
	outputTags := workerutil.GetOutputTags(stepConfig)

	// Get EODHD API key
	eodhdAPIKey, err := common.ResolveAPIKey(ctx, w.kvStorage, "eodhd_api_key", "")
	if err != nil {
		return "", fmt.Errorf("failed to resolve EODHD API key: %w", err)
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Msg("Starting ticker metadata collection")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Collecting metadata for %d tickers", len(tickers)))
	}

	// Process each ticker
	for _, ticker := range tickers {
		if err := w.processTickerMetadata(ctx, ticker, cacheHours, forceRefresh, eodhdAPIKey, stepID, managerID, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to process ticker metadata")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to fetch metadata for %s: %v", ticker.String(), err))
			}
			// Continue with other tickers
		}
	}

	return stepID, nil
}

// processTickerMetadata fetches and processes metadata for a single ticker
func (w *TickerMetadataWorker) processTickerMetadata(
	ctx context.Context,
	ticker common.Ticker,
	cacheHours int,
	forceRefresh bool,
	eodhdAPIKey string,
	stepID, managerID string,
	outputTags []string,
) error {
	// Generate source ID for caching (stable ID, not date-based)
	sourceID := fmt.Sprintf("%s:metadata", ticker.String())

	// Check cache
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource("ticker_metadata", sourceID)
		if err == nil && existingDoc != nil {
			if w.isCacheFresh(existingDoc, cacheHours) {
				w.logger.Info().
					Str("ticker", ticker.String()).
					Str("source_id", sourceID).
					Msg("Using cached ticker metadata")
				// Associate with current job for isolation
				if err := workerutil.AssociateDocumentWithJob(ctx, existingDoc, managerID, w.documentStorage, w.logger); err != nil {
					w.logger.Warn().Err(err).Msg("Failed to associate cached document with job")
				}
				return nil
			}
		}
	}

	// Fetch fundamentals from EODHD
	client := eodhd.NewClient(eodhdAPIKey, eodhd.WithLogger(w.logger))
	symbol := ticker.EODHDSymbol()

	fundamentals, err := client.GetFundamentals(ctx, symbol)
	if err != nil {
		return fmt.Errorf("EODHD fundamentals fetch failed for %s: %w", ticker.String(), err)
	}

	// Extract metadata
	metadata := w.extractMetadata(ticker, fundamentals)
	metadata.FetchedAt = time.Now()

	// Create document
	doc := w.createMetadataDocument(metadata, stepID, managerID, outputTags, sourceID)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save metadata document: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Str("company", metadata.CompanyName).
		Int("directors", len(metadata.Directors)).
		Int("management", len(metadata.Management)).
		Msg("Ticker metadata document created")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf(
			"%s: %s (%s, %s)",
			ticker.String(), metadata.CompanyName, metadata.Industry, metadata.Country,
		))
	}

	return nil
}

// extractMetadata extracts CompanyMetadata from EODHD FundamentalsResponse
func (w *TickerMetadataWorker) extractMetadata(ticker common.Ticker, fund *eodhd.FundamentalsResponse) *CompanyMetadata {
	metadata := &CompanyMetadata{
		Ticker: ticker,
	}

	// Extract from General
	if fund.General != nil {
		metadata.CompanyName = fund.General.Name
		metadata.Description = fund.General.Description
		metadata.ISIN = fund.General.ISIN
		metadata.Sector = fund.General.Sector
		metadata.Industry = fund.General.Industry
		metadata.Country = fund.General.CountryName
		metadata.Address = fund.General.Address
		metadata.Phone = fund.General.Phone
		metadata.Website = fund.General.WebURL
		metadata.LogoURL = fund.General.LogoURL
		metadata.IPODate = fund.General.IPODate
		metadata.Employees = fund.General.FullTimeEmployees
		metadata.Currency = fund.General.CurrencyCode

		// Extract officers (directors and management)
		if fund.General.Officers != nil {
			for _, officer := range fund.General.Officers {
				entry := OfficerEntry{
					Name:  officer.Name,
					Title: officer.Title,
				}
				// Categorize based on title
				titleLower := strings.ToLower(officer.Title)
				if strings.Contains(titleLower, "director") ||
					strings.Contains(titleLower, "chairman") ||
					strings.Contains(titleLower, "board") {
					metadata.Directors = append(metadata.Directors, entry)
				} else {
					metadata.Management = append(metadata.Management, entry)
				}
			}
		}
	}

	// Extract from Highlights
	if fund.Highlights != nil {
		metadata.MarketCap = fund.Highlights.MarketCapitalization
		metadata.PERatio = fund.Highlights.PERatio
		metadata.DividendYield = fund.Highlights.DividendYield
	}

	// Extract from Valuation
	if fund.Valuation != nil {
		metadata.EnterpriseValue = fund.Valuation.EnterpriseValue
	}

	// Sort directors and management by name
	sort.Slice(metadata.Directors, func(i, j int) bool {
		return metadata.Directors[i].Name < metadata.Directors[j].Name
	})
	sort.Slice(metadata.Management, func(i, j int) bool {
		return metadata.Management[i].Name < metadata.Management[j].Name
	})

	return metadata
}

// isCacheFresh checks if document is within cache window
func (w *TickerMetadataWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// formatMarketCap formats market cap in human-readable format
func formatMarketCap(cap float64) string {
	if cap >= 1e12 {
		return fmt.Sprintf("$%.2fT", cap/1e12)
	}
	if cap >= 1e9 {
		return fmt.Sprintf("$%.2fB", cap/1e9)
	}
	if cap >= 1e6 {
		return fmt.Sprintf("$%.2fM", cap/1e6)
	}
	return fmt.Sprintf("$%.0f", cap)
}

// createMetadataDocument creates a company metadata document
func (w *TickerMetadataWorker) createMetadataDocument(
	metadata *CompanyMetadata,
	stepID, managerID string,
	outputTags []string,
	sourceID string,
) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s (%s)\n\n", metadata.CompanyName, metadata.Ticker.String()))

	// Company Overview
	content.WriteString("## Company Overview\n\n")

	if metadata.Industry != "" {
		content.WriteString(fmt.Sprintf("**Industry**: %s\n", metadata.Industry))
	}
	if metadata.Sector != "" {
		content.WriteString(fmt.Sprintf("**Sector**: %s\n", metadata.Sector))
	}
	if metadata.Country != "" {
		content.WriteString(fmt.Sprintf("**Location**: %s\n", metadata.Country))
	}
	if metadata.Address != "" {
		content.WriteString(fmt.Sprintf("**Address**: %s\n", metadata.Address))
	}
	if metadata.Phone != "" {
		content.WriteString(fmt.Sprintf("**Phone**: %s\n", metadata.Phone))
	}
	if metadata.Website != "" {
		content.WriteString(fmt.Sprintf("**Website**: [%s](%s)\n", metadata.Website, metadata.Website))
	}
	if metadata.ISIN != "" {
		content.WriteString(fmt.Sprintf("**ISIN**: %s\n", metadata.ISIN))
	}
	if metadata.IPODate != "" {
		content.WriteString(fmt.Sprintf("**IPO Date**: %s\n", metadata.IPODate))
	}
	if metadata.Employees > 0 {
		content.WriteString(fmt.Sprintf("**Employees**: %d\n", metadata.Employees))
	}
	content.WriteString("\n")

	// Description
	if metadata.Description != "" {
		content.WriteString("### Description\n\n")
		// Truncate very long descriptions
		desc := metadata.Description
		if len(desc) > 2000 {
			desc = desc[:2000] + "..."
		}
		content.WriteString(desc + "\n\n")
	}

	// Key Financials
	content.WriteString("## Key Financials\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	if metadata.MarketCap > 0 {
		content.WriteString(fmt.Sprintf("| Market Cap | %s |\n", formatMarketCap(metadata.MarketCap)))
	}
	if metadata.EnterpriseValue > 0 {
		content.WriteString(fmt.Sprintf("| Enterprise Value | %s |\n", formatMarketCap(metadata.EnterpriseValue)))
	}
	if metadata.PERatio > 0 {
		content.WriteString(fmt.Sprintf("| P/E Ratio | %.2f |\n", metadata.PERatio))
	}
	if metadata.DividendYield > 0 {
		content.WriteString(fmt.Sprintf("| Dividend Yield | %.2f%% |\n", metadata.DividendYield*100))
	}
	if metadata.Currency != "" {
		content.WriteString(fmt.Sprintf("| Currency | %s |\n", metadata.Currency))
	}
	content.WriteString("\n")

	// Directors
	if len(metadata.Directors) > 0 {
		content.WriteString("## Directors\n\n")
		content.WriteString("| Name | Title |\n")
		content.WriteString("|------|-------|\n")
		for _, d := range metadata.Directors {
			content.WriteString(fmt.Sprintf("| %s | %s |\n", d.Name, d.Title))
		}
		content.WriteString("\n")
	}

	// Management
	if len(metadata.Management) > 0 {
		content.WriteString("## Management\n\n")
		content.WriteString("| Name | Title |\n")
		content.WriteString("|------|-------|\n")
		for _, m := range metadata.Management {
			content.WriteString(fmt.Sprintf("| %s | %s |\n", m.Name, m.Title))
		}
		content.WriteString("\n")
	}

	// Build tags
	today := time.Now().Format("2006-01-02")
	tags := []string{
		"ticker-metadata",
		strings.ToLower(metadata.Ticker.String()),
		strings.ToLower(metadata.Ticker.Exchange),
		strings.ToLower(metadata.Ticker.Code),
		"date:" + today,
	}
	if metadata.Industry != "" {
		tags = append(tags, "industry:"+strings.ToLower(strings.ReplaceAll(metadata.Industry, " ", "-")))
	}
	if metadata.Sector != "" {
		tags = append(tags, "sector:"+strings.ToLower(strings.ReplaceAll(metadata.Sector, " ", "-")))
	}
	if metadata.Country != "" {
		tags = append(tags, "country:"+strings.ToLower(strings.ReplaceAll(metadata.Country, " ", "-")))
	}
	tags = append(tags, outputTags...)

	// Build directors and management arrays for metadata
	directorsData := make([]map[string]string, len(metadata.Directors))
	for i, d := range metadata.Directors {
		directorsData[i] = map[string]string{"name": d.Name, "title": d.Title}
	}
	managementData := make([]map[string]string, len(metadata.Management))
	for i, m := range metadata.Management {
		managementData[i] = map[string]string{"name": m.Name, "title": m.Title}
	}

	// Build metadata map
	docMetadata := map[string]interface{}{
		"ticker":           metadata.Ticker.String(),
		"exchange":         metadata.Ticker.Exchange,
		"code":             metadata.Ticker.Code,
		"company_name":     metadata.CompanyName,
		"industry":         metadata.Industry,
		"sector":           metadata.Sector,
		"location":         metadata.Country,
		"address":          metadata.Address,
		"isin":             metadata.ISIN,
		"ipo_date":         metadata.IPODate,
		"employees":        metadata.Employees,
		"market_cap":       metadata.MarketCap,
		"enterprise_value": metadata.EnterpriseValue,
		"pe_ratio":         metadata.PERatio,
		"dividend_yield":   metadata.DividendYield,
		"currency":         metadata.Currency,
		"directors":        directorsData,
		"management":       managementData,
		"director_count":   len(metadata.Directors),
		"management_count": len(metadata.Management),
		"fetched_at":       metadata.FetchedAt.Format(time.RFC3339),
		"job_id":           stepID,
	}

	now := time.Now()
	return &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("%s Company Profile", metadata.Ticker.String()),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      "ticker_metadata",
		SourceID:        sourceID,
		Tags:            tags,
		Jobs:            []string{managerID},
		Metadata:        docMetadata,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}
}

// ReturnsChildJobs returns false
func (w *TickerMetadataWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *TickerMetadataWorker) ValidateConfig(step models.JobStep) error {
	// Tickers can come from step config or job-level variables
	// So minimal validation here - Init will do the full check
	return nil
}
