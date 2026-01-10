// -----------------------------------------------------------------------
// AnnouncementDownloadWorker - Downloads PDFs from filtered announcements
// Uses worker-to-worker pattern: calls announcements_worker inline for each ticker,
// then filters and downloads PDFs for matching announcement types (default: FY results)
// -----------------------------------------------------------------------

package market

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	announcementsvc "github.com/ternarybob/quaero/internal/services/announcements"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// DefaultFYTypes defines the default announcement types to filter for FY results
var DefaultFYTypes = []string{
	"Annual Report",
	"Full Year",
	"FY",
	"Preliminary Final Report",
	"Appendix 4E",
}

// DownloadedAnnouncement represents a downloaded announcement with its storage key
type DownloadedAnnouncement struct {
	Date          string `json:"date"`
	Headline      string `json:"headline"`
	Type          string `json:"type"`
	PDFURL        string `json:"pdf_url"`
	DocumentKey   string `json:"document_key"`
	StorageKey    string `json:"storage_key,omitempty"`
	Downloaded    bool   `json:"downloaded"`
	DownloadError string `json:"download_error,omitempty"`
}

// AnnouncementDownloadOutput is the schema for JSON output
type AnnouncementDownloadOutput struct {
	Schema           string                   `json:"$schema"`
	Ticker           string                   `json:"ticker"`
	FetchedAt        string                   `json:"fetched_at"`
	FilterTypes      []string                 `json:"filter_types"`
	TotalMatched     int                      `json:"total_matched"`
	TotalDownloaded  int                      `json:"total_downloaded"`
	TotalFailed      int                      `json:"total_failed"`
	Announcements    []DownloadedAnnouncement `json:"announcements"`
	SourceDocumentID string                   `json:"source_document_id,omitempty"`
}

// AnnouncementDownloadWorker downloads PDFs from filtered announcements.
// Uses worker-to-worker pattern: calls announcements_worker's DocumentProvider interface
// to ensure announcement documents exist, then filters and downloads PDFs.
type AnnouncementDownloadWorker struct {
	documentStorage      interfaces.DocumentStorage
	searchService        interfaces.SearchService
	kvStorage            interfaces.KeyValueStorage
	logger               arbor.ILogger
	jobMgr               *queue.Manager
	announcementSvc      *announcementsvc.Service
	announcementProvider interfaces.DocumentProvider // Worker-to-worker: DocumentProvider pattern
	debugEnabled         bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*AnnouncementDownloadWorker)(nil)

// NewAnnouncementDownloadWorker creates a new announcement download worker.
// The announcementProvider parameter enables the worker-to-worker pattern using DocumentProvider.
func NewAnnouncementDownloadWorker(
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	announcementProvider interfaces.DocumentProvider,
	debugEnabled bool,
) *AnnouncementDownloadWorker {
	return &AnnouncementDownloadWorker{
		documentStorage:      documentStorage,
		searchService:        searchService,
		kvStorage:            kvStorage,
		logger:               logger,
		jobMgr:               jobMgr,
		announcementSvc:      announcementsvc.NewService(logger, nil, kvStorage),
		announcementProvider: announcementProvider,
		debugEnabled:         debugEnabled,
	}
}

// GetType returns the worker type identifier
func (w *AnnouncementDownloadWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketAnnouncementDownload
}

// ValidateConfig validates step configuration
func (w *AnnouncementDownloadWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - uses defaults if not provided
	return nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *AnnouncementDownloadWorker) ReturnsChildJobs() bool {
	return false
}

// Init initializes the worker and returns work items
func (w *AnnouncementDownloadWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract tickers from config or job variables (same pattern as other workers)
	tickers := w.extractTickers(stepConfig, jobDef)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers found in config or job variables")
	}

	// Extract config parameters for metadata
	filterTypes := DefaultFYTypes
	if types, ok := stepConfig["announcement_types"].([]interface{}); ok && len(types) > 0 {
		filterTypes = make([]string, 0, len(types))
		for _, t := range types {
			if s, ok := t.(string); ok {
				filterTypes = append(filterTypes, s)
			}
		}
	}

	maxDownloads := 10
	if m, ok := stepConfig["max_downloads"].(float64); ok && m > 0 {
		maxDownloads = int(m)
	}

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   ticker,
			Name: fmt.Sprintf("Download FY announcements for %s", ticker),
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:  workItems,
		TotalCount: len(tickers),
		Strategy:   interfaces.ProcessingStrategyInline,
		Metadata: map[string]interface{}{
			"tickers":            tickers,
			"filter_types":       filterTypes,
			"max_downloads":      maxDownloads,
			"parent_output_tags": workerutil.GetOutputTags(stepConfig),
		},
	}, nil
}

// extractTickers extracts ticker symbols from step config and job variables
func (w *AnnouncementDownloadWorker) extractTickers(stepConfig map[string]interface{}, jobDef models.JobDefinition) []string {
	var tickers []string
	seen := make(map[string]bool)

	addTicker := func(ticker string) {
		ticker = strings.TrimSpace(ticker)
		if ticker != "" && !seen[ticker] {
			seen[ticker] = true
			tickers = append(tickers, ticker)
		}
	}

	// From step config (override)
	if stepConfig != nil {
		if ticker, ok := stepConfig["ticker"].(string); ok {
			addTicker(ticker)
		}
		if arr, ok := stepConfig["tickers"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					addTicker(s)
				}
			}
		}
		if arr, ok := stepConfig["tickers"].([]string); ok {
			for _, s := range arr {
				addTicker(s)
			}
		}
	}

	// From job-level variables (primary source)
	if jobDef.Config != nil {
		if vars, ok := jobDef.Config["variables"].([]interface{}); ok {
			for _, v := range vars {
				varMap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				if ticker, ok := varMap["ticker"].(string); ok {
					addTicker(ticker)
				}
			}
		}
	}

	return tickers
}

// CreateJobs executes the announcement download for all tickers using worker-to-worker pattern
func (w *AnnouncementDownloadWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", err
		}
	}

	tickers, _ := initResult.Metadata["tickers"].([]string)
	filterTypes, _ := initResult.Metadata["filter_types"].([]string)
	maxDownloads, _ := initResult.Metadata["max_downloads"].(int)
	parentOutputTags, _ := initResult.Metadata["parent_output_tags"].([]string)

	if len(tickers) == 0 {
		return "", fmt.Errorf("no tickers in init result")
	}
	if len(filterTypes) == 0 {
		filterTypes = DefaultFYTypes
	}
	if maxDownloads == 0 {
		maxDownloads = 10
	}

	// Get manager_id for document isolation
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	w.logger.Info().
		Str("step_id", stepID).
		Int("ticker_count", len(tickers)).
		Strs("filter_types", filterTypes).
		Int("max_downloads", maxDownloads).
		Msg("Starting announcement download - using worker-to-worker pattern")

	// Process each ticker using worker-to-worker pattern
	for _, tickerStr := range tickers {
		if err := w.processOneTicker(ctx, tickerStr, filterTypes, maxDownloads, stepID, managerID, parentOutputTags); err != nil {
			w.logger.Warn().Err(err).Str("ticker", tickerStr).Msg("Failed to download announcements")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to download announcements for %s: %v", tickerStr, err))
			}
			// Continue with other tickers
		}
	}

	return stepID, nil
}

// processOneTicker downloads announcements for a single ticker using worker-to-worker pattern
func (w *AnnouncementDownloadWorker) processOneTicker(
	ctx context.Context,
	tickerStr string,
	filterTypes []string,
	maxDownloads int,
	stepID, managerID string,
	parentOutputTags []string,
) error {
	ticker := common.ParseTicker(tickerStr)
	if ticker.Code == "" {
		return fmt.Errorf("invalid ticker: %s", tickerStr)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Strs("filter_types", filterTypes).
		Int("max_downloads", maxDownloads).
		Msg("Processing announcement downloads using worker-to-worker pattern")

	// Step 1: Get announcement document using DocumentProvider pattern
	if w.announcementProvider == nil {
		return fmt.Errorf("no announcement provider configured (need DocumentProvider)")
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetching announcements for %s via DocumentProvider", ticker.String()))
	}

	// Note: We do NOT pass announcement-download tags to the source document provider.
	// The source announcements document should keep its normal tags (announcements, ticker).
	// Only the final output document from this worker gets the announcement-download tags.
	result, err := w.announcementProvider.GetDocument(ctx, ticker.String(),
		interfaces.WithCacheHours(24),
		interfaces.WithForceRefresh(false),
		interfaces.WithManagerID(managerID),
	)
	if err != nil {
		return fmt.Errorf("failed to get announcements via DocumentProvider: %w", err)
	}

	if result.DocumentID == "" {
		w.logger.Info().Str("ticker", ticker.String()).Msg("No announcement document created by provider")
		return nil
	}

	// Get the document by ID
	sourceDoc, err := w.documentStorage.GetDocument(result.DocumentID)
	if err != nil {
		return fmt.Errorf("failed to retrieve announcement document by ID: %w", err)
	}

	w.logger.Debug().
		Str("ticker", ticker.String()).
		Str("doc_id", result.DocumentID).
		Bool("fresh", result.Fresh).
		Bool("created", result.Created).
		Msg("Got announcement document via DocumentProvider")

	// Step 3: Extract announcements from document metadata
	announcements, err := w.extractAnnouncements(sourceDoc)
	if err != nil {
		return fmt.Errorf("failed to extract announcements: %w", err)
	}

	if len(announcements) == 0 {
		w.logger.Info().Str("ticker", ticker.String()).Msg("No announcements in document")
		return nil
	}

	// Step 4: Filter announcements by type
	filtered := w.filterAnnouncements(announcements, filterTypes)
	w.logger.Info().
		Str("ticker", ticker.String()).
		Int("total", len(announcements)).
		Int("filtered", len(filtered)).
		Msg("Filtered announcements by type")

	if len(filtered) == 0 {
		w.logger.Info().Str("ticker", ticker.String()).Strs("filter_types", filterTypes).Msg("No announcements match filter types")
		// Still create output document showing no matches
	}

	// Step 5: Limit to maxDownloads
	if len(filtered) > maxDownloads {
		filtered = filtered[:maxDownloads]
	}

	// Step 6: Download PDFs
	downloadResults := w.downloadPDFs(ctx, filtered, ticker.Code)

	// Step 7: Create output document
	doc := w.createOutputDocument(ticker, filterTypes, downloadResults, sourceDoc.ID, parentOutputTags, managerID)

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// Count results
	downloadedCount := 0
	failedCount := 0
	for _, r := range downloadResults {
		if r.Downloaded {
			downloadedCount++
		} else if r.DownloadError != "" {
			failedCount++
		}
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Int("matched", len(downloadResults)).
		Int("downloaded", downloadedCount).
		Int("failed", failedCount).
		Str("doc_id", doc.ID).
		Msg("Saved announcement download document")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Downloaded %d/%d FY announcements for %s", downloadedCount, len(downloadResults), ticker.String()))
	}

	return nil
}

// extractAnnouncements extracts announcement data from document metadata
func (w *AnnouncementDownloadWorker) extractAnnouncements(doc *models.Document) ([]announcementsvc.RawAnnouncement, error) {
	if doc.Metadata == nil {
		return nil, nil
	}

	// Try to get announcements from metadata
	annsData, ok := doc.Metadata["announcements"]
	if !ok {
		return nil, nil
	}

	// Handle different storage formats
	var announcements []announcementsvc.RawAnnouncement

	switch v := annsData.(type) {
	case []interface{}:
		for _, item := range v {
			ann := w.parseAnnouncement(item)
			if ann != nil {
				announcements = append(announcements, *ann)
			}
		}
	case []map[string]interface{}:
		for _, item := range v {
			ann := w.parseAnnouncementMap(item)
			if ann != nil {
				announcements = append(announcements, *ann)
			}
		}
	default:
		// Try JSON unmarshal as fallback
		jsonBytes, err := json.Marshal(annsData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal announcements: %w", err)
		}
		if err := json.Unmarshal(jsonBytes, &announcements); err != nil {
			// Try parsing as array of AnnouncementOutputItem
			var outputItems []AnnouncementOutputItem
			if err := json.Unmarshal(jsonBytes, &outputItems); err == nil {
				for _, item := range outputItems {
					date, _ := time.Parse("2006-01-02", item.Date)
					announcements = append(announcements, announcementsvc.RawAnnouncement{
						Date:           date,
						Headline:       item.Headline,
						Type:           item.Type,
						PDFURL:         item.Link,
						PriceSensitive: item.PriceSensitive,
					})
				}
			}
		}
	}

	return announcements, nil
}

// parseAnnouncement parses a single announcement from interface{}
func (w *AnnouncementDownloadWorker) parseAnnouncement(item interface{}) *announcementsvc.RawAnnouncement {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}
	return w.parseAnnouncementMap(itemMap)
}

// parseAnnouncementMap parses a single announcement from map
func (w *AnnouncementDownloadWorker) parseAnnouncementMap(itemMap map[string]interface{}) *announcementsvc.RawAnnouncement {
	ann := &announcementsvc.RawAnnouncement{}

	// Parse date
	if dateStr, ok := itemMap["date"].(string); ok {
		if date, err := time.Parse(time.RFC3339, dateStr); err == nil {
			ann.Date = date
		} else if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			ann.Date = date
		}
	}

	// Parse headline
	if headline, ok := itemMap["headline"].(string); ok {
		ann.Headline = headline
	}

	// Parse type
	if t, ok := itemMap["type"].(string); ok {
		ann.Type = t
	}

	// Parse PDF URL (different field names)
	if pdfURL, ok := itemMap["pdf_url"].(string); ok {
		ann.PDFURL = pdfURL
	} else if link, ok := itemMap["link"].(string); ok {
		ann.PDFURL = link
	}

	// Parse document key
	if docKey, ok := itemMap["document_key"].(string); ok {
		ann.DocumentKey = docKey
	}

	// Parse price sensitive
	if ps, ok := itemMap["price_sensitive"].(bool); ok {
		ann.PriceSensitive = ps
	}

	return ann
}

// filterAnnouncements filters announcements by type keywords
func (w *AnnouncementDownloadWorker) filterAnnouncements(announcements []announcementsvc.RawAnnouncement, filterTypes []string) []announcementsvc.RawAnnouncement {
	var filtered []announcementsvc.RawAnnouncement

	for _, ann := range announcements {
		if w.matchesFilterType(ann, filterTypes) {
			filtered = append(filtered, ann)
		}
	}

	return filtered
}

// matchesFilterType checks if an announcement matches any filter type
func (w *AnnouncementDownloadWorker) matchesFilterType(ann announcementsvc.RawAnnouncement, filterTypes []string) bool {
	headlineUpper := strings.ToUpper(ann.Headline)
	typeUpper := strings.ToUpper(ann.Type)

	for _, ft := range filterTypes {
		ftUpper := strings.ToUpper(ft)
		if strings.Contains(headlineUpper, ftUpper) || strings.Contains(typeUpper, ftUpper) {
			return true
		}
	}

	return false
}

// downloadPDFs downloads PDFs for the filtered announcements
func (w *AnnouncementDownloadWorker) downloadPDFs(ctx context.Context, announcements []announcementsvc.RawAnnouncement, code string) []DownloadedAnnouncement {
	results := make([]DownloadedAnnouncement, len(announcements))

	for i, ann := range announcements {
		result := DownloadedAnnouncement{
			Date:        ann.Date.Format("2006-01-02"),
			Headline:    ann.Headline,
			Type:        ann.Type,
			PDFURL:      ann.PDFURL,
			DocumentKey: ann.DocumentKey,
		}

		if ann.PDFURL == "" {
			result.DownloadError = "No PDF URL available"
			results[i] = result
			continue
		}

		// Use the announcement service to download
		anns := []announcementsvc.RawAnnouncement{ann}
		downloadedAnns := w.announcementSvc.DownloadAndStorePDFs(ctx, anns, code, 1)

		if len(downloadedAnns) > 0 && downloadedAnns[0].PDFDownloaded {
			result.StorageKey = downloadedAnns[0].PDFStorageKey
			result.Downloaded = true
		} else {
			result.DownloadError = "Download failed"
		}

		results[i] = result
	}

	return results
}

// createOutputDocument creates the output document with download results
func (w *AnnouncementDownloadWorker) createOutputDocument(
	ticker common.Ticker,
	filterTypes []string,
	downloadResults []DownloadedAnnouncement,
	sourceDocID string,
	outputTags []string,
	managerID string,
) *models.Document {
	// Count results
	downloadedCount := 0
	failedCount := 0
	for _, r := range downloadResults {
		if r.Downloaded {
			downloadedCount++
		} else if r.DownloadError != "" {
			failedCount++
		}
	}

	// Build output
	output := AnnouncementDownloadOutput{
		Schema:           "quaero/announcement_download/v1",
		Ticker:           ticker.String(),
		FetchedAt:        time.Now().Format(time.RFC3339),
		FilterTypes:      filterTypes,
		TotalMatched:     len(downloadResults),
		TotalDownloaded:  downloadedCount,
		TotalFailed:      failedCount,
		Announcements:    downloadResults,
		SourceDocumentID: sourceDocID,
	}

	// Convert to metadata
	outputJSON, _ := json.Marshal(output)
	var metadata map[string]interface{}
	json.Unmarshal(outputJSON, &metadata)

	// Build tags - include parent output_tags for pipeline routing
	tags := []string{
		"announcement-download",
		strings.ToLower(ticker.Code),
		fmt.Sprintf("ticker:%s", ticker.String()),
		fmt.Sprintf("source_type:%s", models.WorkerTypeMarketAnnouncementDownload.String()),
	}
	tags = append(tags, outputTags...)

	// Build jobs array for job isolation
	var jobs []string
	if managerID != "" {
		jobs = []string{managerID}
	}

	// Build markdown content
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# Announcement Downloads - %s\n\n", ticker.String()))
	contentBuilder.WriteString(fmt.Sprintf("**Ticker:** %s\n", ticker.String()))
	contentBuilder.WriteString(fmt.Sprintf("**Filter Types:** %s\n", strings.Join(filterTypes, ", ")))
	contentBuilder.WriteString(fmt.Sprintf("**Downloaded:** %d/%d\n\n", downloadedCount, len(downloadResults)))

	if len(downloadResults) > 0 {
		contentBuilder.WriteString("## Downloaded Announcements\n\n")
		contentBuilder.WriteString("| Date | Headline | Status | Storage Key |\n")
		contentBuilder.WriteString("|------|----------|--------|-------------|\n")

		for _, r := range downloadResults {
			status := "Failed"
			if r.Downloaded {
				status = "OK"
			}
			storageKey := r.StorageKey
			if storageKey == "" && r.DownloadError != "" {
				storageKey = r.DownloadError
			}
			// Truncate headline if too long
			headline := r.Headline
			if len(headline) > 50 {
				headline = headline[:47] + "..."
			}
			contentBuilder.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", r.Date, headline, status, storageKey))
		}
	}

	now := time.Now()
	return &models.Document{
		ID:              uuid.New().String(),
		SourceType:      "announcement_download",
		SourceID:        fmt.Sprintf("%s:%s:announcement_download", ticker.Exchange, ticker.Code),
		Title:           fmt.Sprintf("Announcement Downloads - %s", ticker.String()),
		ContentMarkdown: contentBuilder.String(),
		Tags:            tags,
		Jobs:            jobs,
		Metadata:        metadata,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}
}
