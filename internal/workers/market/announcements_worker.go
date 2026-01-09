// -----------------------------------------------------------------------
// AnnouncementsWorker - Fetches company announcements and produces classified output
// Uses AnnouncementService for exchange-specific data fetching
// Outputs JSON and Markdown documents with classification and signal-noise analysis
//
// WORKER COMMUNICATION PATTERN:
// This worker reads cached market_data documents for price impact analysis.
// It does NOT fetch price data directly - that responsibility belongs to the
// market_data worker. Pipeline configuration should ensure market_data runs
// before market_announcements if price impact analysis is desired.
//
// DATA COLLECTION + PROCESSING:
// - Primary: ASX sources (Markit API, HTML scraping) for ASX-listed stocks
// - Fallback: EODHD for non-ASX exchanges or when ASX sources fail (via AnnouncementService)
// - Applies classification via internal/services/announcements
// - Outputs both raw and processed documents
// -----------------------------------------------------------------------

package market

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

// RawAnnouncement represents a single market announcement (raw data from API)
type RawAnnouncement struct {
	Date           time.Time `json:"date"`
	Headline       string    `json:"headline"`
	PDFURL         string    `json:"pdf_url"`
	PDFFilename    string    `json:"pdf_filename,omitempty"`
	DocumentKey    string    `json:"document_key"`
	FileSize       string    `json:"file_size,omitempty"`
	PriceSensitive bool      `json:"price_sensitive"`
	Type           string    `json:"type"`
}

// AnnouncementsOutput is the schema for JSON output
type AnnouncementsOutput struct {
	Schema         string                   `json:"$schema"`
	Ticker         string                   `json:"ticker"`
	Exchange       string                   `json:"exchange"`
	Code           string                   `json:"code"`
	FetchedAt      string                   `json:"fetched_at"`
	DateRangeStart string                   `json:"date_range_start"`
	DateRangeEnd   string                   `json:"date_range_end"`
	Summary        AnnouncementsSummary     `json:"summary"`
	Announcements  []AnnouncementOutputItem `json:"announcements"`
	WorkerDebug    *WorkerDebugOutput       `json:"worker_debug,omitempty"`
}

// WorkerDebugOutput contains debug/timing information for the worker
type WorkerDebugOutput struct {
	WorkerType   string              `json:"worker_type"`
	Ticker       string              `json:"ticker"`
	StartedAt    string              `json:"started_at"`
	CompletedAt  string              `json:"completed_at"`
	Timing       WorkerTimingOutput  `json:"timing"`
	APIEndpoints []WorkerAPIEndpoint `json:"api_endpoints,omitempty"`
}

// WorkerTimingOutput contains timing breakdown by phase
type WorkerTimingOutput struct {
	APIFetchMs    int64 `json:"api_fetch_ms,omitempty"`
	ComputationMs int64 `json:"computation_ms,omitempty"`
	TotalMs       int64 `json:"total_ms"`
}

// WorkerAPIEndpoint contains details of an API call
type WorkerAPIEndpoint struct {
	Endpoint   string `json:"endpoint"`
	Method     string `json:"method"`
	DurationMs int64  `json:"duration_ms"`
	StatusCode int    `json:"status_code,omitempty"`
}

// AnnouncementsSummary contains classification summary statistics
type AnnouncementsSummary struct {
	TotalCount           int     `json:"total_count"`
	HighRelevanceCount   int     `json:"high_relevance_count"`
	MediumRelevanceCount int     `json:"medium_relevance_count"`
	LowRelevanceCount    int     `json:"low_relevance_count"`
	NoiseCount           int     `json:"noise_count"`
	RoutineCount         int     `json:"routine_count"`
	SignalToNoiseRatio   float64 `json:"signal_to_noise_ratio,omitempty"`

	// Definitions for Relevance and Signal ratings
	RelevanceDefinition string `json:"relevance_definition"`
	SignalDefinition    string `json:"signal_definition"`
}

// RelevanceDefinitionText explains the Relevance rating categories
const RelevanceDefinitionText = "Relevance categorizes announcements by importance: HIGH = price-sensitive or major events (takeovers, financials, guidance); MEDIUM = governance and significant operational (directors, contracts, exploration); LOW = routine disclosures (progress reports, presentations); NOISE = no material indicators found."

// SignalDefinitionText explains the Signal rating categories
const SignalDefinitionText = "Signal measures market impact: HIGH_SIGNAL = significant reaction (>=3% price change OR >=2x volume); MODERATE_SIGNAL = notable reaction (>=1.5% price change OR >=1.5x volume); LOW_SIGNAL = minimal reaction (>=0.5% price change OR >=1.2x volume); NOISE = no meaningful impact; ROUTINE = administrative filings excluded from analysis."

// AnnouncementOutputItem represents a single announcement in the output
type AnnouncementOutputItem struct {
	Date            string `json:"date"`
	Headline        string `json:"headline"`
	Type            string `json:"type"`
	Link            string `json:"link"`
	PriceSensitive  bool   `json:"price_sensitive"`
	Relevance       string `json:"relevance"`
	RelevanceReason string `json:"relevance_reason,omitempty"`
	SignalRating    string `json:"signal_rating"`
	SignalReason    string `json:"signal_reason,omitempty"`
	IsRoutine       bool   `json:"is_routine"`
	RoutineType     string `json:"routine_type,omitempty"`

	// Price impact data (from market_data worker)
	DayOfChange    *float64 `json:"day_of_change,omitempty"`   // Price change on announcement day (%)
	Day10Change    *float64 `json:"day_10_change,omitempty"`   // Price change after 10 trading days (%)
	VolumeMultiple *float64 `json:"volume_multiple,omitempty"` // Volume on announcement day vs 30-day average
}

// AnnouncementsWorker fetches company announcements and produces classified output.
// This worker executes synchronously (no child jobs).
// Output: JSON and Markdown documents with classification
type AnnouncementsWorker struct {
	documentStorage     interfaces.DocumentStorage
	logger              arbor.ILogger
	jobMgr              *queue.Manager
	announcementSvc     *announcementsvc.Service
	debugEnabled        bool
	documentProvisioner interfaces.DocumentProvisioner // For worker-to-worker communication pattern
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*AnnouncementsWorker)(nil)

// NewAnnouncementsWorker creates a new announcements worker for data collection.
// The documentProvisioner parameter enables worker-to-worker communication for price data.
// Any worker implementing interfaces.DocumentProvisioner can be passed (e.g., DataWorker).
func NewAnnouncementsWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	documentProvisioner interfaces.DocumentProvisioner,
	debugEnabled bool,
) *AnnouncementsWorker {
	return &AnnouncementsWorker{
		documentStorage:     documentStorage,
		logger:              logger,
		jobMgr:              jobMgr,
		announcementSvc:     announcementsvc.NewService(logger, nil, kvStorage),
		debugEnabled:        debugEnabled,
		documentProvisioner: documentProvisioner,
	}
}

// GetType returns the worker type identifier
func (w *AnnouncementsWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketAnnouncements
}

// extractASXCodes extracts ASX codes from step config and job-level variables
func (w *AnnouncementsWorker) extractASXCodes(stepConfig map[string]interface{}, jobDef models.JobDefinition) []string {
	var codes []string
	seen := make(map[string]bool)

	addCode := func(code string) {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code != "" && !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}

	// 1. Single asx_code from step config
	if stepConfig != nil {
		if code, ok := stepConfig["asx_code"].(string); ok {
			addCode(code)
		}
		// Try ticker format
		if ticker, ok := stepConfig["ticker"].(string); ok {
			t := common.ParseTicker(ticker)
			addCode(t.Code)
		}
		// Array of codes
		if arr, ok := stepConfig["asx_codes"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					addCode(s)
				}
			}
		}
		if arr, ok := stepConfig["tickers"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					t := common.ParseTicker(s)
					addCode(t.Code)
				}
			}
		}
	}

	// 2. Job-level variables
	if jobDef.Config != nil {
		if vars, ok := jobDef.Config["variables"].([]interface{}); ok {
			for _, v := range vars {
				varMap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				if ticker, ok := varMap["ticker"].(string); ok {
					t := common.ParseTicker(ticker)
					addCode(t.Code)
				}
				if code, ok := varMap["asx_code"].(string); ok {
					addCode(code)
				}
			}
		}
	}

	return codes
}

// Init initializes the worker and returns work items
func (w *AnnouncementsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	codes := w.extractASXCodes(step.Config, jobDef)
	if len(codes) == 0 {
		return nil, fmt.Errorf("no ASX codes found in config or job variables")
	}

	workItems := make([]interfaces.WorkItem, len(codes))
	for i, code := range codes {
		workItems[i] = interfaces.WorkItem{
			ID:   code,
			Name: fmt.Sprintf("Fetch announcements for ASX:%s", code),
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:  workItems,
		TotalCount: len(codes),
		Strategy:   interfaces.ProcessingStrategyInline,
		Metadata:   map[string]interface{}{"asx_codes": codes},
	}, nil
}

// CreateJobs executes the announcement fetching for all tickers
func (w *AnnouncementsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	codes, ok := initResult.Metadata["asx_codes"].([]string)
	if !ok || len(codes) == 0 {
		return "", fmt.Errorf("no ASX codes in init result")
	}

	// Get manager_id for document isolation across pipeline steps
	// All documents from the same pipeline run share the same manager_id
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	// Get config parameters - default to 3 years (36 months rolling)
	period := "Y3"
	limit := 1000
	cacheHours := 24 // Default cache duration for announcements
	if step.Config != nil {
		if p, ok := step.Config["period"].(string); ok && p != "" {
			period = p
		}
		if l, ok := step.Config["limit"].(float64); ok {
			limit = int(l)
		}
		if c, ok := step.Config["cache_hours"].(float64); ok {
			cacheHours = int(c)
		}
	}

	// Extract output tags
	var outputTags []string
	if step.Config != nil {
		if tags, ok := step.Config["output_tags"].([]interface{}); ok {
			for _, t := range tags {
				if s, ok := t.(string); ok {
					outputTags = append(outputTags, s)
				}
			}
		}
	}

	// Process each ticker
	for _, code := range codes {
		if err := w.processOneTicker(ctx, code, period, limit, cacheHours, &jobDef, stepID, managerID, outputTags); err != nil {
			w.logger.Warn().Err(err).Str("asx_code", code).Msg("Failed to fetch announcements")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to fetch announcements for %s: %v", code, err))
			}
			// Continue with other tickers
		}
	}

	return stepID, nil
}

// processOneTicker fetches and stores announcements for a single ticker
func (w *AnnouncementsWorker) processOneTicker(ctx context.Context, code, period string, limit int, cacheHours int, jobDef *models.JobDefinition, stepID, managerID string, outputTags []string) error {
	// Initialize debug info
	tickerStr := fmt.Sprintf("ASX:%s", code)
	debug := workerutil.NewWorkerDebug(models.WorkerTypeMarketAnnouncements.String(), w.debugEnabled)
	debug.SetTicker(tickerStr)
	debug.SetJobID(stepID) // Include job ID in debug output
	defer func() {
		debug.Complete()
	}()

	// Build source identifiers for caching
	sourceType := "announcement"
	sourceID := fmt.Sprintf("ASX:%s:announcement", code)

	// Check for cached data before fetching
	if cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && existingDoc != nil && existingDoc.LastSynced != nil {
			if time.Since(*existingDoc.LastSynced) < time.Duration(cacheHours)*time.Hour {
				w.logger.Info().
					Str("code", code).
					Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
					Int("cache_hours", cacheHours).
					Msg("Using cached announcements data")

				// Associate cached document with current job for downstream workers
				// Use managerID so all steps in the pipeline can find this document
				if err := workerutil.AssociateDocumentWithJob(ctx, existingDoc, managerID, w.documentStorage, w.logger); err != nil {
					w.logger.Warn().Err(err).Str("doc_id", existingDoc.ID).Str("manager_id", managerID).Msg("Failed to associate cached document with job")
				}

				if w.jobMgr != nil {
					w.jobMgr.AddJobLog(ctx, stepID, "info",
						fmt.Sprintf("%s - Using cached announcements (last synced: %s)",
							code, existingDoc.LastSynced.Format("2006-01-02 15:04")))
				}
				return nil
			}
		}
	}

	w.logger.Info().Str("code", code).Str("period", period).Msg("Fetching announcements")

	// Fetch announcements (with timing)
	debug.StartPhase("api_fetch")
	anns, err := w.fetchAnnouncements(ctx, code, period, limit)
	debug.EndPhase("api_fetch")
	if err != nil {
		debug.CompleteWithError(err)
		return fmt.Errorf("failed to fetch announcements: %w", err)
	}

	if len(anns) == 0 {
		w.logger.Info().Str("code", code).Msg("No announcements found")
		return nil
	}

	// Create announcement document with classification (with timing)
	debug.StartPhase("computation")
	doc := w.createDocument(ctx, anns, code, jobDef, outputTags, debug, managerID)
	debug.EndPhase("computation")

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		debug.CompleteWithError(err)
		return fmt.Errorf("failed to save document: %w", err)
	}

	w.logger.Info().
		Str("code", code).
		Int("count", len(anns)).
		Str("doc_id", doc.ID).
		Msg("Saved announcements document")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetched %d announcements for %s", len(anns), code))
	}

	return nil
}

// createDocument creates a document containing announcement data with classification
func (w *AnnouncementsWorker) createDocument(ctx context.Context, anns []RawAnnouncement, code string, jobDef *models.JobDefinition, outputTags []string, debug *workerutil.WorkerDebugInfo, managerID string) *models.Document {
	ticker := common.Ticker{Exchange: "ASX", Code: code}

	// Load price data from market_data worker (respects separation of concerns)
	ohlcvPrices := w.loadPriceData(ctx, ticker)

	// Convert OHLCV to PriceBar format for announcement service
	var priceBars []announcementsvc.PriceBar
	for _, p := range ohlcvPrices {
		priceBars = append(priceBars, announcementsvc.PriceBar{
			Date:   p.Date,
			Open:   p.Open,
			High:   p.High,
			Low:    p.Low,
			Close:  p.Close,
			Volume: p.Volume,
		})
	}

	// Convert to service layer format for classification
	rawAnns := make([]announcementsvc.RawAnnouncement, len(anns))
	for i, ann := range anns {
		rawAnns[i] = announcementsvc.RawAnnouncement{
			Date:           ann.Date,
			Headline:       ann.Headline,
			Type:           ann.Type,
			PDFURL:         ann.PDFURL,
			DocumentKey:    ann.DocumentKey,
			PriceSensitive: ann.PriceSensitive,
		}
	}

	// Process announcements (classification, signal-noise analysis)
	// Price data enables signal-to-noise analysis based on market reaction
	processed, summary, _ := announcementsvc.ProcessAnnouncements(rawAnns, priceBars)

	// Build tags
	tags := []string{
		"announcement",
		strings.ToLower(ticker.Code),
		fmt.Sprintf("ticker:%s", ticker.String()),
		fmt.Sprintf("source_type:%s", models.WorkerTypeMarketAnnouncements.String()),
	}
	tags = append(tags, outputTags...)

	// Sort announcements by date descending (latest to oldest)
	sort.Slice(processed, func(i, j int) bool {
		return processed[i].Date.After(processed[j].Date)
	})

	// Build structured output using schema types
	announcements := make([]AnnouncementOutputItem, len(processed))
	for i, ann := range processed {
		dateStr := ""
		if !ann.Date.IsZero() {
			dateStr = ann.Date.Format("2006-01-02")
		}
		item := AnnouncementOutputItem{
			Date:            dateStr,
			Headline:        ann.Headline,
			Type:            ann.Type,
			Link:            ann.PDFURL,
			PriceSensitive:  ann.PriceSensitive,
			Relevance:       ann.RelevanceCategory,
			RelevanceReason: ann.RelevanceReason,
			SignalRating:    string(ann.SignalNoiseRating),
			SignalReason:    ann.SignalNoiseRationale,
			IsRoutine:       ann.IsRoutine,
			RoutineType:     ann.RoutineType,
		}

		// Calculate price impact if we have price data
		if len(ohlcvPrices) > 0 && !ann.Date.IsZero() {
			dayOf, day10, _, volumeMult := w.calculateAnnouncementPriceImpact(ann.Date, ohlcvPrices)
			item.DayOfChange = dayOf
			item.Day10Change = day10
			item.VolumeMultiple = volumeMult
		}

		announcements[i] = item
	}

	// Calculate date range
	dateRangeStart := ""
	dateRangeEnd := ""
	if len(processed) > 0 {
		// Already sorted desc, so first is newest, last is oldest
		if !processed[0].Date.IsZero() {
			dateRangeEnd = processed[0].Date.Format("2006-01-02")
		}
		if !processed[len(processed)-1].Date.IsZero() {
			dateRangeStart = processed[len(processed)-1].Date.Format("2006-01-02")
		}
	}

	// Calculate signal-to-noise ratio
	signalCount := summary.HighRelevanceCount + summary.MediumRelevanceCount
	noiseCount := summary.LowRelevanceCount + summary.NoiseCount + summary.RoutineCount
	signalToNoiseRatio := 0.0
	if noiseCount > 0 {
		signalToNoiseRatio = float64(signalCount) / float64(noiseCount)
	}

	// Build worker debug output
	var workerDebug *WorkerDebugOutput
	if debug != nil && debug.IsEnabled() {
		debug.Complete() // Ensure timing is captured
		debugMeta := debug.ToMetadata()
		if debugMeta != nil {
			workerDebug = &WorkerDebugOutput{
				WorkerType: models.WorkerTypeMarketAnnouncements.String(),
				Ticker:     ticker.String(),
			}
			if startedAt, ok := debugMeta["started_at"].(string); ok {
				workerDebug.StartedAt = startedAt
			}
			if completedAt, ok := debugMeta["completed_at"].(string); ok {
				workerDebug.CompletedAt = completedAt
			}
			if timing, ok := debugMeta["timing"].(map[string]interface{}); ok {
				if totalMs, ok := timing["total_ms"].(int64); ok {
					workerDebug.Timing.TotalMs = totalMs
				}
				if apiFetchMs, ok := timing["api_fetch_ms"].(int64); ok {
					workerDebug.Timing.APIFetchMs = apiFetchMs
				}
				if computationMs, ok := timing["computation_ms"].(int64); ok {
					workerDebug.Timing.ComputationMs = computationMs
				}
			}
			if endpoints, ok := debugMeta["api_endpoints"].([]map[string]interface{}); ok {
				for _, ep := range endpoints {
					apiEp := WorkerAPIEndpoint{}
					if endpoint, ok := ep["endpoint"].(string); ok {
						apiEp.Endpoint = endpoint
					}
					if method, ok := ep["method"].(string); ok {
						apiEp.Method = method
					}
					if durationMs, ok := ep["duration_ms"].(int64); ok {
						apiEp.DurationMs = durationMs
					}
					if statusCode, ok := ep["status_code"].(int); ok {
						apiEp.StatusCode = statusCode
					}
					workerDebug.APIEndpoints = append(workerDebug.APIEndpoints, apiEp)
				}
			}
		}
	}

	// Build output using schema
	output := AnnouncementsOutput{
		Schema:         "quaero/announcements/v1",
		Ticker:         ticker.String(),
		Exchange:       ticker.Exchange,
		Code:           ticker.Code,
		FetchedAt:      time.Now().Format(time.RFC3339),
		DateRangeStart: dateRangeStart,
		DateRangeEnd:   dateRangeEnd,
		Summary: AnnouncementsSummary{
			TotalCount:           len(processed),
			HighRelevanceCount:   summary.HighRelevanceCount,
			MediumRelevanceCount: summary.MediumRelevanceCount,
			LowRelevanceCount:    summary.LowRelevanceCount,
			NoiseCount:           summary.NoiseCount,
			RoutineCount:         summary.RoutineCount,
			SignalToNoiseRatio:   signalToNoiseRatio,
			RelevanceDefinition:  RelevanceDefinitionText,
			SignalDefinition:     SignalDefinitionText,
		},
		Announcements: announcements,
		WorkerDebug:   workerDebug,
	}

	// Convert to map for document metadata
	outputJSON, _ := json.Marshal(output)
	var metadata map[string]interface{}
	json.Unmarshal(outputJSON, &metadata)

	// Build Jobs array for job isolation (required by downstream workers like output_formatter)
	// Use managerID so all steps in the same pipeline can find this document
	var jobs []string
	if managerID != "" {
		jobs = []string{managerID}
	}

	// Build content markdown
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# Company Announcements - %s\n\n", code))
	contentBuilder.WriteString(fmt.Sprintf("**Ticker:** %s\n", ticker.String()))
	contentBuilder.WriteString(fmt.Sprintf("**Period:** %s to %s\n", dateRangeStart, dateRangeEnd))
	contentBuilder.WriteString(fmt.Sprintf("**Count:** %d announcements\n\n", len(processed)))

	// Summary statistics (keep at top)
	contentBuilder.WriteString("## Summary\n\n")
	contentBuilder.WriteString(fmt.Sprintf("- **Total Announcements:** %d\n", summary.TotalCount))
	contentBuilder.WriteString(fmt.Sprintf("- **High Relevance:** %d\n", summary.HighRelevanceCount))
	contentBuilder.WriteString(fmt.Sprintf("- **Medium Relevance:** %d\n", summary.MediumRelevanceCount))
	contentBuilder.WriteString(fmt.Sprintf("- **Low/Noise/Routine:** %d\n\n", summary.LowRelevanceCount+summary.NoiseCount+summary.RoutineCount))

	// Relevance definition with keywords (grey font)
	contentBuilder.WriteString("<div style=\"color: #888; font-size: 0.85em;\">\n\n")
	contentBuilder.WriteString("**Relevance Classification** - based on headline keywords and ASX price-sensitive flag:\n\n")
	contentBuilder.WriteString("- **HIGH:** Price-sensitive flag, or contains: TAKEOVER, ACQUISITION, MERGER, DIVIDEND, PLACEMENT, SPP, QUARTERLY, EARNINGS, GUIDANCE, FORECAST\n")
	contentBuilder.WriteString("- **MEDIUM:** Contains: DIRECTOR, CEO, CFO, AGM, CONTRACT, AGREEMENT, EXPLORATION, DRILLING, RESOURCE, APPROVAL, LICENSE\n")
	contentBuilder.WriteString("- **LOW/NOISE:** Routine disclosures: PROGRESS REPORT, UPDATE, APPENDIX, SUBSTANTIAL HOLDER, or no keywords matched\n\n")
	contentBuilder.WriteString("</div>\n\n")

	contentBuilder.WriteString("## Announcements\n\n")

	// Filter to HIGH and MEDIUM relevance, then consolidate by date
	// Prefer price-sensitive announcements when multiple on same day
	var displayAnns []AnnouncementOutputItem
	for _, ann := range announcements {
		if ann.Relevance == "HIGH" || ann.Relevance == "MEDIUM" {
			displayAnns = append(displayAnns, ann)
		}
	}

	// Consolidate by date - keep only one announcement per date, prefer price-sensitive
	consolidatedByDate := make(map[string]AnnouncementOutputItem)
	dateOrder := []string{} // preserve order

	for _, ann := range displayAnns {
		dateStr := ann.Date
		if dateStr == "" {
			dateStr = "-"
		}

		existing, exists := consolidatedByDate[dateStr]
		if !exists {
			consolidatedByDate[dateStr] = ann
			dateOrder = append(dateOrder, dateStr)
		} else {
			// Prefer price-sensitive over non-price-sensitive
			if ann.PriceSensitive && !existing.PriceSensitive {
				consolidatedByDate[dateStr] = ann
			}
			// If both are price-sensitive or both not, keep first (already sorted by date desc)
		}
	}

	// Table with smaller font using HTML wrapper
	contentBuilder.WriteString("<div style=\"font-size: 0.85em;\">\n\n")

	// Table header - includes Signal rating column
	contentBuilder.WriteString("| Date | PS | Signal | Headline | Price | Volume | Link |\n")
	contentBuilder.WriteString("|------|:--:|:------:|----------|------:|-------:|------|\n")

	// Limit to 10 most recent announcements for markdown display
	displayCount := len(dateOrder)
	if displayCount > 10 {
		dateOrder = dateOrder[:10]
	}

	// Table rows - consolidated by date, limited to 10
	for _, dateStr := range dateOrder {
		ann := consolidatedByDate[dateStr]

		ps := ""
		if ann.PriceSensitive {
			ps = "✓"
		}

		// Format signal rating (abbreviated)
		signalStr := "-"
		switch ann.SignalRating {
		case "HIGH_SIGNAL":
			signalStr = "HIGH"
		case "MODERATE_SIGNAL":
			signalStr = "MOD"
		case "LOW_SIGNAL":
			signalStr = "LOW"
		case "NOISE":
			signalStr = "NOISE"
		case "ROUTINE":
			signalStr = "RTN"
		}

		// Build link
		link := ""
		if ann.Link != "" {
			link = fmt.Sprintf("[View](%s)", ann.Link)
		}

		// Truncate headline if too long
		headline := ann.Headline
		if len(headline) > 60 {
			headline = headline[:57] + "..."
		}

		// Format price with color
		priceStr := "-"
		if ann.DayOfChange != nil {
			color := "#228B22" // Forest green for positive
			if *ann.DayOfChange < 0 {
				color = "#CD5C5C" // Indian red for negative
			}
			priceStr = fmt.Sprintf("<span style=\"color:%s\">%+.1f%%</span>", color, *ann.DayOfChange)
		}

		// Format volume with color
		volumeStr := "-"
		if ann.VolumeMultiple != nil {
			color := "#228B22" // Forest green for high volume
			if *ann.VolumeMultiple < 1.0 {
				color = "#CD5C5C" // Indian red for below average
			}
			volumeStr = fmt.Sprintf("<span style=\"color:%s\">%.1fx</span>", color, *ann.VolumeMultiple)
		}

		contentBuilder.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
			dateStr,
			ps,
			signalStr,
			headline,
			priceStr,
			volumeStr,
			link,
		))
	}

	contentBuilder.WriteString("\n</div>\n")

	// Add note showing count (limited to 10 most recent)
	shownCount := len(dateOrder)
	contentBuilder.WriteString(fmt.Sprintf("\n*Showing %d of %d HIGH/MEDIUM relevance (last 10, total %d announcements)*\n",
		shownCount, displayCount, len(processed)))

	// Generate document ID early so it can be included in debug info
	docID := uuid.New().String()
	if debug != nil {
		debug.SetDocumentID(docID) // Include document ID in debug output
	}

	// Add Worker Debug section to markdown (uses workerutil.ToMarkdown())
	if debug != nil && debug.IsEnabled() {
		contentBuilder.WriteString(debug.ToMarkdown())
	}

	now := time.Now()
	return &models.Document{
		ID:              docID,
		SourceType:      "announcement",
		SourceID:        fmt.Sprintf("%s:%s:announcement", ticker.Exchange, ticker.Code),
		Title:           fmt.Sprintf("Company Announcements - %s", code),
		ContentMarkdown: contentBuilder.String(),
		Tags:            tags,
		Jobs:            jobs,
		Metadata:        metadata,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now, // Required for caching to work
	}
}

// fetchAnnouncements fetches announcements using the announcement service.
// ASX is primary for ASX tickers, EODHD is used for non-ASX or as fallback.
func (w *AnnouncementsWorker) fetchAnnouncements(ctx context.Context, code, period string, limit int) ([]RawAnnouncement, error) {
	// Parse ticker to determine exchange
	ticker := common.ParseTicker(code)
	if ticker.Exchange == "" {
		// Default to ASX for bare codes
		ticker.Exchange = "ASX"
	}

	// Use announcement service to fetch
	svcAnns, err := w.announcementSvc.FetchAnnouncements(ctx, ticker, period, limit)
	if err != nil {
		return nil, fmt.Errorf("announcement service failed: %w", err)
	}

	// Convert from service format to worker format
	anns := make([]RawAnnouncement, len(svcAnns))
	for i, a := range svcAnns {
		anns[i] = RawAnnouncement{
			Date:           a.Date,
			Headline:       a.Headline,
			PDFURL:         a.PDFURL,
			DocumentKey:    a.DocumentKey,
			PriceSensitive: a.PriceSensitive,
			Type:           a.Type,
		}
	}

	w.logger.Debug().
		Str("ticker", ticker.String()).
		Int("count", len(anns)).
		Msg("Fetched announcements via service")

	return anns, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *AnnouncementsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *AnnouncementsWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - asx_code can come from job-level variables
	return nil
}

// loadPriceData retrieves OHLCV data using the DocumentProvisioner interface.
//
// ARCHITECTURE: This uses the worker-to-worker communication pattern:
//  1. Call DocumentProvisioner.EnsureDocuments() with ticker identifier
//  2. Receive document ID(s) back
//  3. Use document ID to retrieve document content from DocumentStorage
//
// This ensures the provisioner (e.g., DataWorker) owns all market data fetching and caching logic.
// If documentProvisioner is nil (backward compatibility), falls back to direct document lookup.
func (w *AnnouncementsWorker) loadPriceData(ctx context.Context, ticker common.Ticker) []OHLCV {
	if w.documentProvisioner == nil {
		w.logger.Debug().
			Str("ticker", ticker.String()).
			Msg("No DocumentProvisioner available, falling back to direct document lookup")
		return w.loadPriceDataDirect(ctx, ticker)
	}

	// Call DocumentProvisioner to ensure market data exists (worker-to-worker communication)
	// Use 24-hour cache, no force refresh
	docIDs, err := w.documentProvisioner.EnsureDocuments(ctx, []string{ticker.String()}, interfaces.DocumentProvisionOptions{
		CacheHours:   24,
		ForceRefresh: false,
	})
	if err != nil {
		w.logger.Debug().
			Err(err).
			Str("ticker", ticker.String()).
			Msg("Failed to ensure market data via DocumentProvisioner")
		return nil
	}

	// Get document ID for this ticker
	docID, ok := docIDs[ticker.String()]
	if !ok || docID == "" {
		w.logger.Debug().
			Str("ticker", ticker.String()).
			Msg("No document ID returned for ticker from DocumentProvisioner")
		return nil
	}

	// Retrieve document by ID
	doc, err := w.documentStorage.GetDocument(docID)
	if err != nil || doc == nil {
		w.logger.Debug().
			Err(err).
			Str("ticker", ticker.String()).
			Str("doc_id", docID).
			Msg("Failed to retrieve market data document by ID")
		return nil
	}

	return w.extractOHLCVFromDocument(doc, ticker)
}

// loadPriceDataDirect attempts to load price data directly from document storage.
// This is the fallback when DataWorker is not available.
func (w *AnnouncementsWorker) loadPriceDataDirect(ctx context.Context, ticker common.Ticker) []OHLCV {
	sourceType := models.WorkerTypeMarketData.String()
	sourceID := ticker.SourceID("market_data")

	doc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err != nil || doc == nil {
		w.logger.Debug().
			Str("ticker", ticker.String()).
			Str("source_type", sourceType).
			Str("source_id", sourceID).
			Msg("No cached market data document found (direct lookup)")
		return nil
	}

	return w.extractOHLCVFromDocument(doc, ticker)
}

// extractOHLCVFromDocument extracts OHLCV price data from a market data document.
func (w *AnnouncementsWorker) extractOHLCVFromDocument(doc *models.Document, ticker common.Ticker) []OHLCV {
	// Extract historical prices from document metadata
	// BadgerHold uses gob encoding, so we need to handle both []interface{} and []map[string]interface{}
	var pricesData []interface{}
	switch v := doc.Metadata["historical_prices"].(type) {
	case []interface{}:
		pricesData = v
	case []map[string]interface{}:
		// Convert []map[string]interface{} to []interface{}
		for _, m := range v {
			pricesData = append(pricesData, m)
		}
	default:
		w.logger.Debug().
			Str("ticker", ticker.String()).
			Str("type", fmt.Sprintf("%T", doc.Metadata["historical_prices"])).
			Msg("No historical prices in market data document (unexpected type)")
		return nil
	}

	if len(pricesData) == 0 {
		w.logger.Debug().
			Str("ticker", ticker.String()).
			Msg("No historical prices in market data document (empty)")
		return nil
	}

	var prices []OHLCV
	for _, p := range pricesData {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		dateStr, _ := pm["date"].(string)
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		ohlcv := OHLCV{
			Date:   date,
			Open:   MapGetFloat64(pm, "open"),
			High:   MapGetFloat64(pm, "high"),
			Low:    MapGetFloat64(pm, "low"),
			Close:  MapGetFloat64(pm, "close"),
			Volume: MapGetInt64(pm, "volume"),
		}
		prices = append(prices, ohlcv)
	}

	w.logger.Debug().
		Str("ticker", ticker.String()).
		Int("price_count", len(prices)).
		Msg("Extracted price data from market data document")

	return prices
}

// calculateAnnouncementPriceImpact calculates price impact for an announcement.
// Returns Day-Of change, Day+10 change, Retention, and Volume multiple.
func (w *AnnouncementsWorker) calculateAnnouncementPriceImpact(annDate time.Time, prices []OHLCV) (dayOf, day10, retention, volumeMult *float64) {
	if len(prices) == 0 {
		return nil, nil, nil, nil
	}

	// Build date-to-price map for O(1) lookups
	priceMap := make(map[string]OHLCV)
	for _, p := range prices {
		priceMap[p.Date.Format("2006-01-02")] = p
	}

	annDateStr := annDate.Format("2006-01-02")

	// Find price on announcement date (or closest trading day after)
	var priceOnDate OHLCV
	foundOnDate := false
	for i := 0; i <= 5; i++ {
		checkDate := annDate.AddDate(0, 0, i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok {
			priceOnDate = p
			foundOnDate = true
			if i == 0 {
				annDateStr = checkDate
			}
			break
		}
	}
	if !foundOnDate {
		return nil, nil, nil, nil
	}

	// Find previous trading day's price
	var priceBefore OHLCV
	for i := 1; i <= 10; i++ {
		checkDate := annDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok {
			priceBefore = p
			break
		}
	}
	if priceBefore.Close == 0 {
		return nil, nil, nil, nil
	}

	// Day-Of change: (announcement day close - previous day close) / previous day close * 100
	dayOfChange := ((priceOnDate.Close - priceBefore.Close) / priceBefore.Close) * 100
	dayOf = &dayOfChange

	// Find T+10 trading day price
	var priceT10 OHLCV
	tradingDays := 0
	for i := 1; i <= 20 && tradingDays < 10; i++ {
		checkDate := annDate.AddDate(0, 0, i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok {
			tradingDays++
			if tradingDays == 10 {
				priceT10 = p
			}
		}
	}

	if priceT10.Close > 0 {
		// Day+10 change: (T+10 close - previous day close) / previous day close * 100
		day10Change := ((priceT10.Close - priceBefore.Close) / priceBefore.Close) * 100
		day10 = &day10Change

		// Retention: How much of Day-Of change was retained at T+10
		// If Day-Of was positive and Day+10 is still positive, retention = Day+10/Day-Of
		if dayOfChange != 0 {
			retentionVal := (day10Change / dayOfChange) * 100
			retention = &retentionVal
		}
	}

	// Volume multiple: announcement day volume vs 30-day average
	var totalVolume int64
	volumeCount := 0
	for i := 1; i <= 60 && volumeCount < 30; i++ {
		checkDate := annDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok && p.Volume > 0 {
			totalVolume += p.Volume
			volumeCount++
		}
	}
	if volumeCount > 0 {
		avgVolume := float64(totalVolume) / float64(volumeCount)
		if avgVolume > 0 {
			volMult := float64(priceOnDate.Volume) / avgVolume
			volumeMult = &volMult
		}
	}

	// Log the calculation for the specific announcement date
	_ = annDateStr // suppress unused warning

	return dayOf, day10, retention, volumeMult
}
