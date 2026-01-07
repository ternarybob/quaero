// -----------------------------------------------------------------------
// MarketAnnouncementsWorker - Fetches ASX company announcements
// Uses the Markit Digital API to fetch announcements in JSON format
// Produces individual announcement documents AND a summary document
// with relevance classification and price impact analysis
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/llm"
)

// OHLCV represents a single day's price data for price impact correlation
type OHLCV struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// yahooChartResponse for Yahoo Finance chart endpoint (price history for impact analysis)
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// MarketAnnouncementsWorker fetches ASX company announcements and stores them as documents.
// This worker executes synchronously (no child jobs).
type MarketAnnouncementsWorker struct {
	documentStorage      interfaces.DocumentStorage
	kvStorage            interfaces.KeyValueStorage
	logger               arbor.ILogger
	jobMgr               *queue.Manager
	httpClient           *http.Client
	debugEnabled         bool
	providerFactory      *llm.ProviderFactory
	fundamentalsProvider interfaces.FundamentalsDataProvider
	priceProvider        interfaces.PriceDataProvider
}

// Compile-time assertion: MarketAnnouncementsWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*MarketAnnouncementsWorker)(nil)

// ASXAnnouncement represents a single ASX announcement
type ASXAnnouncement struct {
	Date           time.Time
	Headline       string
	PDFURL         string
	PDFFilename    string
	DocumentKey    string
	FileSize       string
	PriceSensitive bool
	Type           string
}

// SignalNoiseRating represents the signal quality of an announcement based on actual market impact
type SignalNoiseRating string

const (
	// SignalNoiseHigh indicates significant market impact - high signal, low noise
	// Criteria: Price change >=3% OR volume ratio >=2x, typically with price-sensitive flag
	SignalNoiseHigh SignalNoiseRating = "HIGH_SIGNAL"

	// SignalNoiseModerate indicates notable market reaction
	// Criteria: Price change >=1.5% OR volume ratio >=1.5x
	SignalNoiseModerate SignalNoiseRating = "MODERATE_SIGNAL"

	// SignalNoiseLow indicates minimal market reaction
	// Criteria: Price change >=0.5% OR volume ratio >=1.2x
	SignalNoiseLow SignalNoiseRating = "LOW_SIGNAL"

	// SignalNoiseNone indicates no meaningful price/volume impact - pure noise
	// Criteria: Price change <0.5% AND volume ratio <1.2x
	SignalNoiseNone SignalNoiseRating = "NOISE"

	// SignalNoiseRoutine indicates routine administrative announcement
	// These are standard regulatory filings that all companies must make
	// and are NOT correlated with price/volume movements - excluded from signal analysis
	SignalNoiseRoutine SignalNoiseRating = "ROUTINE"
)

// AnnouncementAnalysis represents an analyzed announcement with classification and price impact
type AnnouncementAnalysis struct {
	Date              time.Time
	Headline          string
	Type              string
	PriceSensitive    bool
	PDFURL            string
	DocumentKey       string
	RelevanceCategory string // HIGH, MEDIUM, LOW, NOISE (keyword-based)
	RelevanceReason   string // Why this classification was assigned
	PriceImpact       *PriceImpactData

	// Signal-to-noise analysis based on actual market impact
	SignalNoiseRating    SignalNoiseRating // Overall signal quality based on price/volume impact
	SignalNoiseRationale string            // Explanation of why this rating was assigned
	IsTradingHalt        bool              // Whether this announcement is a trading halt
	IsReinstatement      bool              // Whether this is a reinstatement from trading halt

	// Anomaly detection
	IsAnomaly   bool   // True if announcement behavior doesn't match expectations
	AnomalyType string // Type of anomaly: "NO_REACTION", "UNEXPECTED_REACTION", ""

	// Dividend tracking
	IsDividendAnnouncement bool // True if announcement is dividend-related

	// Routine announcement tracking
	IsRoutine   bool   // True if announcement is routine administrative filing
	RoutineType string // Type of routine announcement (e.g., "Director Interest (3Y)")

	// Critical review classification (REQ-1)
	SignalClassification string // TRUE_SIGNAL, PRICED_IN, SENTIMENT_NOISE, MANAGEMENT_BLUFF, ROUTINE
}

// PriceImpactData contains stock price movement around an announcement date
type PriceImpactData struct {
	PriceBefore       float64 `json:"price_before"`        // Close price 1 trading day before
	PriceAfter        float64 `json:"price_after"`         // Close price on announcement day (or next trading day)
	ChangePercent     float64 `json:"change_percent"`      // Percentage change (immediate reaction)
	VolumeBefore      int64   `json:"volume_before"`       // Average volume 5 days before
	VolumeAfter       int64   `json:"volume_after"`        // Average volume 5 days after
	VolumeChangeRatio float64 `json:"volume_change_ratio"` // Volume ratio (after/before)
	ImpactSignal      string  `json:"impact_signal"`       // "SIGNIFICANT", "MODERATE", "MINIMAL"

	// Pre-announcement analysis (T-5 to T-1)
	PreAnnouncementDrift   float64 `json:"pre_announcement_drift"`    // Price change % from T-5 to T-1
	PreAnnouncementPriceT5 float64 `json:"pre_announcement_price_t5"` // Price at T-5
	PreAnnouncementPriceT1 float64 `json:"pre_announcement_price_t1"` // Price at T-1
	HasSignificantPreDrift bool    `json:"has_significant_pre_drift"` // True if drift >= 2%
	PreDriftInterpretation string  `json:"pre_drift_interpretation"`  // Interpretation of pre-drift
}

// asxAPIResponse represents the JSON response from Markit Digital API
type asxAPIResponse struct {
	Data struct {
		DisplayName string `json:"displayName"`
		Symbol      string `json:"symbol"`
		Items       []struct {
			AnnouncementType string `json:"announcementType"`
			Date             string `json:"date"`
			DocumentKey      string `json:"documentKey"`
			FileSize         string `json:"fileSize"`
			Headline         string `json:"headline"`
			IsPriceSensitive bool   `json:"isPriceSensitive"`
			URL              string `json:"url"`
		} `json:"items"`
	} `json:"data"`
}

// NewMarketAnnouncementsWorker creates a new ASX announcements worker
func NewMarketAnnouncementsWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
	providerFactory *llm.ProviderFactory,
	fundamentalsProvider interfaces.FundamentalsDataProvider,
	priceProvider interfaces.PriceDataProvider,
) *MarketAnnouncementsWorker {
	return &MarketAnnouncementsWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		debugEnabled:         debugEnabled,
		providerFactory:      providerFactory,
		fundamentalsProvider: fundamentalsProvider,
		priceProvider:        priceProvider,
	}
}

// GetType returns WorkerTypeMarketAnnouncements for the DefinitionWorker interface
func (w *MarketAnnouncementsWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketAnnouncements
}

// extractASXCodes extracts all ASX codes from step config and job-level variables.
// Returns a deduplicated list of ASX codes.
// Sources: step config asx_code (single) > job config variables (multiple)
func (w *MarketAnnouncementsWorker) extractASXCodes(stepConfig map[string]interface{}, jobDef models.JobDefinition) []string {
	tickerSet := make(map[string]bool)

	// Source 1: Direct step config (single ticker)
	if asxCode, ok := stepConfig["asx_code"].(string); ok && asxCode != "" {
		parsed := common.ParseTicker(asxCode)
		if parsed.Code != "" {
			tickerSet[strings.ToUpper(parsed.Code)] = true
		}
	}

	// Source 2: Job-level variables (multiple tickers)
	if jobDef.Config != nil {
		if vars, ok := jobDef.Config["variables"].([]interface{}); ok {
			for _, v := range vars {
				varMap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}

				// Try "ticker" key (e.g., "ASX:GNP" or "GNP")
				if ticker, ok := varMap["ticker"].(string); ok && ticker != "" {
					parsed := common.ParseTicker(ticker)
					if parsed.Code != "" {
						tickerSet[strings.ToUpper(parsed.Code)] = true
					}
				}

				// Try "asx_code" key
				if asxCode, ok := varMap["asx_code"].(string); ok && asxCode != "" {
					parsed := common.ParseTicker(asxCode)
					if parsed.Code != "" {
						tickerSet[strings.ToUpper(parsed.Code)] = true
					}
				}
			}
		}
	}

	// Convert to slice
	tickers := make([]string, 0, len(tickerSet))
	for ticker := range tickerSet {
		tickers = append(tickers, ticker)
	}

	// Sort for deterministic ordering
	sort.Strings(tickers)

	return tickers
}

// Init performs the initialization/setup phase for an ASX announcements step.
// Supports multiple tickers from job-level variables - creates work item per ticker.
func (w *MarketAnnouncementsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract ALL ASX codes from step config and job-level variables
	asxCodes := w.extractASXCodes(stepConfig, jobDef)
	if len(asxCodes) == 0 {
		return nil, fmt.Errorf("asx_code is required in step config or job variables")
	}

	// Extract period (optional, used to filter results by date)
	// Supported: D1, W1, M1, M3, M6, Y1, Y5
	period := "Y1"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	// Extract limit (optional, max announcements to fetch per ticker)
	// Default 100 to capture ~2 years of reporting cycles for Say-Do analysis
	limit := 100 // Default
	if l, ok := stepConfig["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := stepConfig["limit"].(int); ok {
		limit = l
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(asxCodes)).
		Strs("asx_codes", asxCodes).
		Str("period", period).
		Int("limit", limit).
		Msg("ASX announcements worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, len(asxCodes))
	for i, asxCode := range asxCodes {
		workItems[i] = interfaces.WorkItem{
			ID:   asxCode,
			Name: fmt.Sprintf("Fetch ASX:%s announcements", asxCode),
			Type: "market_announcements",
			Config: map[string]interface{}{
				"asx_code": asxCode,
				"period":   period,
				"limit":    limit,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(asxCodes),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"asx_codes":   asxCodes,
			"period":      period,
			"limit":       limit,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs fetches ASX announcements and stores them as documents.
// Processes all tickers from work items sequentially.
// Returns the step job ID since this executes synchronously.
func (w *MarketAnnouncementsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize market_announcements worker: %w", err)
		}
	}

	// Extract metadata from init result
	period, _ := initResult.Metadata["period"].(string)
	limit, _ := initResult.Metadata["limit"].(int)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Extract output_tags from step config (shared across all tickers)
	var outputTags []string
	if stepConfig != nil {
		if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					outputTags = append(outputTags, tagStr)
				}
			}
		} else if tags, ok := stepConfig["output_tags"].([]string); ok {
			outputTags = tags
		}
	}

	// Log overall step start
	tickerCount := len(initResult.WorkItems)
	if w.jobMgr != nil {
		tickers := make([]string, tickerCount)
		for i, wi := range initResult.WorkItems {
			tickers[i] = wi.ID
		}
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Processing %d tickers: %s", tickerCount, strings.Join(tickers, ", ")))
	}

	// Process each work item (ticker) sequentially
	var lastErr error
	successCount := 0
	for i, workItem := range initResult.WorkItems {
		asxCode := workItem.ID

		w.logger.Info().
			Str("phase", "run").
			Str("step_name", step.Name).
			Str("asx_code", asxCode).
			Int("ticker_num", i+1).
			Int("ticker_total", tickerCount).
			Str("period", period).
			Str("step_id", stepID).
			Msg("Fetching ASX announcements")

		err := w.processOneTicker(ctx, asxCode, period, limit, &jobDef, stepID, outputTags)
		if err != nil {
			w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to process ticker")
			lastErr = err
			// Continue with next ticker (on_error = "continue" behavior)
			continue
		}
		successCount++
	}

	// Log overall completion
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Completed %d/%d tickers successfully", successCount, tickerCount))
	}

	// Return error only if ALL tickers failed
	if successCount == 0 && lastErr != nil {
		return "", fmt.Errorf("all tickers failed, last error: %w", lastErr)
	}

	return stepID, nil
}

// processOneTicker handles fetching and analyzing announcements for a single ticker.
func (w *MarketAnnouncementsWorker) processOneTicker(ctx context.Context, asxCode, period string, limit int, jobDef *models.JobDefinition, stepID string, outputTags []string) error {
	// Initialize debug tracking
	debug := NewWorkerDebug("market_announcements", w.debugEnabled)
	debug.SetTicker(fmt.Sprintf("ASX:%s", asxCode))

	// Log step start for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching ASX:%s announcements (period: %s)", asxCode, period))
	}

	// Fetch announcements
	debug.StartPhase("api_fetch")
	announcements, err := w.fetchAnnouncements(ctx, asxCode, period, limit)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch ASX announcements")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch ASX:%s announcements: %v", asxCode, err))
		}
		return fmt.Errorf("failed to fetch ASX announcements: %w", err)
	}
	debug.EndPhase("api_fetch")

	if len(announcements) == 0 {
		w.logger.Warn().Str("asx_code", asxCode).Msg("No announcements found")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("No announcements found for ASX:%s", asxCode))
		}
		return nil // Not an error - just no data
	}

	// Log progress for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetched %d ASX:%s announcements, now analyzing...", len(announcements), asxCode))
	}

	// Fetch historical price data for price impact analysis
	debug.StartPhase("api_fetch") // Accumulates with earlier API fetch
	priceData, err := w.fetchHistoricalPrices(ctx, asxCode, period)
	debug.EndPhase("api_fetch")
	if err != nil {
		w.logger.Warn().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch price data for impact analysis")
		// Continue without price data - analysis will be incomplete but still useful
	}

	// Analyze ALL announcements for classification and price impact (with deduplication)
	debug.StartPhase("computation")
	analyses, dedupStats := w.analyzeAnnouncements(ctx, announcements, asxCode, priceData)
	debug.EndPhase("computation")

	// Log deduplication results
	if dedupStats.DuplicatesFound > 0 {
		w.logger.Info().
			Str("asx_code", asxCode).
			Int("original", dedupStats.TotalBefore).
			Int("deduplicated", dedupStats.TotalAfter).
			Int("duplicates_removed", dedupStats.DuplicatesFound).
			Msg("Deduplicated same-day similar announcements")
	}

	// Build lookup map for quick relevance checking
	analysisMap := make(map[string]AnnouncementAnalysis)
	for _, a := range analyses {
		analysisMap[a.DocumentKey] = a
	}

	// Count by relevance category
	highCount, mediumCount, lowCount, noiseCount := 0, 0, 0, 0
	for _, a := range analyses {
		switch a.RelevanceCategory {
		case "HIGH":
			highCount++
		case "MEDIUM":
			mediumCount++
		case "LOW":
			lowCount++
		case "NOISE":
			noiseCount++
		}
	}

	// Store ONLY HIGH and MEDIUM relevance announcements as individual documents
	// Summary document will contain ALL announcements for complete reference
	savedCount := 0
	skippedCount := 0
	for _, ann := range announcements {
		analysis, exists := analysisMap[ann.DocumentKey]
		if !exists {
			// If no analysis, skip (may have been deduplicated)
			skippedCount++
			continue
		}

		// Only save HIGH and MEDIUM relevance documents
		if analysis.RelevanceCategory != "HIGH" && analysis.RelevanceCategory != "MEDIUM" {
			skippedCount++
			continue
		}

		doc := w.createDocument(ctx, ann, asxCode, jobDef, stepID, outputTags)
		if err := w.documentStorage.SaveDocument(doc); err != nil {
			w.logger.Warn().Err(err).Str("headline", ann.Headline).Msg("Failed to save announcement document")
			continue
		}
		savedCount++
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Int("total", len(announcements)).
		Int("after_dedup", len(analyses)).
		Int("high", highCount).
		Int("medium", mediumCount).
		Int("low", lowCount).
		Int("noise", noiseCount).
		Int("saved_docs", savedCount).
		Int("skipped", skippedCount).
		Msg("ASX announcements analyzed and filtered")

	// Create and save MQS summary document (new MQS framework)
	mqsSummaryDoc := w.createMQSSummaryDocument(ctx, announcements, priceData, asxCode, jobDef, stepID, outputTags, debug)
	if err := w.documentStorage.SaveDocument(mqsSummaryDoc); err != nil {
		w.logger.Warn().Err(err).Str("asx_code", asxCode).Msg("Failed to save MQS summary document")
	} else {
		w.logger.Info().
			Str("asx_code", asxCode).
			Int("announcements_in_summary", len(announcements)).
			Msg("Saved MQS announcement summary document")
	}

	// Log completion for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Completed ASX:%s - %d announcements analyzed with MQS framework",
				asxCode, len(announcements)))
	}

	return nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *MarketAnnouncementsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for market_announcements type.
// Config can be nil if asx_code will be provided via job-level variables.
func (w *MarketAnnouncementsWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - asx_code can come from job-level variables
	// Full validation happens in Init() when we have access to jobDef
	return nil
}

// fetchAnnouncements fetches announcements using the best source for the period.
// For Y1+ periods, uses ASX statistics HTML page (more comprehensive).
// For shorter periods, uses Markit Digital API (faster, sufficient).
func (w *MarketAnnouncementsWorker) fetchAnnouncements(ctx context.Context, asxCode, period string, limit int) ([]ASXAnnouncement, error) {
	// For Y1 or longer periods, use ASX HTML page which returns full year data
	if period == "Y1" || period == "Y5" {
		announcements, err := w.fetchAnnouncementsFromHTML(ctx, asxCode, period, limit)
		if err != nil {
			w.logger.Warn().Err(err).Msg("HTML fetch failed, falling back to Markit API")
			// Fall through to Markit API as backup
		} else if len(announcements) > 0 {
			return announcements, nil
		}
	}

	// Build Markit Digital API URL
	url := fmt.Sprintf("https://asx.api.markitdigital.com/asx-research/1.0/companies/%s/announcements",
		strings.ToLower(asxCode))

	w.logger.Debug().Str("url", url).Msg("Fetching ASX announcements from API")

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	// Make request
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse JSON response
	var apiResp asxAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Calculate cutoff date based on period
	cutoffDate := w.calculateCutoffDate(period)

	var announcements []ASXAnnouncement
	for _, item := range apiResp.Data.Items {
		if limit > 0 && len(announcements) >= limit {
			break
		}

		// Parse date
		date, err := time.Parse(time.RFC3339, item.Date)
		if err != nil {
			// Try alternative format
			date, err = time.Parse("2006-01-02T15:04:05", item.Date)
			if err != nil {
				w.logger.Debug().Str("date", item.Date).Msg("Failed to parse announcement date")
				date = time.Now()
			}
		}

		// Filter by period
		if !cutoffDate.IsZero() && date.Before(cutoffDate) {
			continue
		}

		// Build PDF URL from document key if URL is empty
		pdfURL := item.URL
		if pdfURL == "" && item.DocumentKey != "" {
			pdfURL = fmt.Sprintf("https://www.asx.com.au/asxpdf/%s", item.DocumentKey)
		}

		ann := ASXAnnouncement{
			Date:           date,
			Headline:       item.Headline,
			PDFURL:         pdfURL,
			PDFFilename:    item.DocumentKey,
			DocumentKey:    item.DocumentKey,
			FileSize:       item.FileSize,
			PriceSensitive: item.IsPriceSensitive,
			Type:           item.AnnouncementType,
		}

		announcements = append(announcements, ann)
	}

	return announcements, nil
}

// fetchAnnouncementsFromHTML scrapes announcements from ASX statistics HTML page.
// URL: https://www.asx.com.au/asx/v2/statistics/announcements.do?by=asxCode&asxCode={CODE}&timeframe=Y&year={YEAR}
// This provides 50+ announcements per year vs ~5 from the Markit API.
func (w *MarketAnnouncementsWorker) fetchAnnouncementsFromHTML(ctx context.Context, asxCode, period string, limit int) ([]ASXAnnouncement, error) {
	currentYear := time.Now().Year()
	var allAnnouncements []ASXAnnouncement

	// Determine how many years to fetch based on period
	// For Y1, fetch 2 years to ensure we get 12 months of data
	// (e.g., if today is Jan 2026, we need 2025 + 2026 data)
	yearsToFetch := 2
	if period == "Y5" {
		yearsToFetch = 6
	}

	for yearOffset := 0; yearOffset < yearsToFetch; yearOffset++ {
		year := currentYear - yearOffset

		url := fmt.Sprintf("https://www.asx.com.au/asx/v2/statistics/announcements.do?by=asxCode&asxCode=%s&timeframe=Y&year=%d",
			strings.ToUpper(asxCode), year)

		w.logger.Debug().Str("url", url).Int("year", year).Msg("Fetching ASX announcements from HTML")

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := w.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch HTML: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		html := string(body)
		announcements := w.parseAnnouncementsHTML(html, asxCode, year)
		allAnnouncements = append(allAnnouncements, announcements...)

		w.logger.Info().
			Str("asx_code", asxCode).
			Int("year", year).
			Int("count", len(announcements)).
			Msg("Parsed announcements from HTML")
	}

	// Sort by date descending
	sort.Slice(allAnnouncements, func(i, j int) bool {
		return allAnnouncements[i].Date.After(allAnnouncements[j].Date)
	})

	// Apply limit if specified
	if limit > 0 && len(allAnnouncements) > limit {
		allAnnouncements = allAnnouncements[:limit]
	}

	return allAnnouncements, nil
}

// parseAnnouncementsHTML parses HTML table rows to extract announcement data.
// HTML structure:
// <tr>
//
//	<td>09/12/2025<br><span class="dates-time">12:24 pm</span></td>
//	<td class="pricesens"><!-- img if price sensitive --></td>
//	<td><a href="/asx/v2/statistics/displayAnnouncement.do?display=pdf&idsId=03038930">Headline</a></td>
//
// </tr>
func (w *MarketAnnouncementsWorker) parseAnnouncementsHTML(html, asxCode string, year int) []ASXAnnouncement {
	var announcements []ASXAnnouncement

	// Extract tbody content
	tbodyPattern := regexp.MustCompile(`(?s)<tbody>(.*?)</tbody>`)
	tbodyMatch := tbodyPattern.FindStringSubmatch(html)
	if len(tbodyMatch) < 2 {
		w.logger.Debug().Msg("No tbody found in HTML")
		return announcements
	}
	tbody := tbodyMatch[1]

	// Match individual rows - pattern: date | price sensitive | headline with link
	rowPattern := regexp.MustCompile(`(?s)<tr>\s*<td>\s*(\d{2}/\d{2}/\d{4})<br>\s*<span class="dates-time">([^<]+)</span>\s*</td>\s*<td[^>]*>\s*(.*?)\s*</td>\s*<td>\s*(.*?)\s*</td>\s*</tr>`)

	rows := rowPattern.FindAllStringSubmatch(tbody, -1)

	for _, row := range rows {
		if len(row) < 5 {
			continue
		}

		dateStr := row[1]  // e.g., "09/12/2025"
		timeStr := row[2]  // e.g., "12:24 pm"
		priceCol := row[3] // Contains img tag if price sensitive
		headlineCol := row[4]

		// Parse date and time
		dateTime, err := time.Parse("02/01/2006 3:04 pm", dateStr+" "+strings.TrimSpace(timeStr))
		if err != nil {
			dateTime, err = time.Parse("02/01/2006", dateStr)
			if err != nil {
				continue
			}
		}

		// Check price sensitive
		priceSensitive := strings.Contains(priceCol, "icon-price-sensitive")

		// Extract headline and PDF URL
		headlinePattern := regexp.MustCompile(`href="([^"]+)"[^>]*>([^<]+)`)
		headlineMatch := headlinePattern.FindStringSubmatch(headlineCol)

		var headline, pdfPath, idsId string
		if len(headlineMatch) >= 3 {
			pdfPath = headlineMatch[1]
			headline = strings.TrimSpace(headlineMatch[2])
		}

		// Extract idsId from URL for document key
		idsIdPattern := regexp.MustCompile(`idsId=(\d+)`)
		idsIdMatch := idsIdPattern.FindStringSubmatch(pdfPath)
		if len(idsIdMatch) >= 2 {
			idsId = idsIdMatch[1]
		}

		// Build full PDF URL
		pdfURL := ""
		if pdfPath != "" {
			pdfURL = "https://www.asx.com.au" + pdfPath
		}

		// Extract file size if present
		fileSizePattern := regexp.MustCompile(`<span class="filesize">\s*([^<]+)\s*</span>`)
		fileSizeMatch := fileSizePattern.FindStringSubmatch(headlineCol)
		fileSize := ""
		if len(fileSizeMatch) >= 2 {
			fileSize = strings.TrimSpace(fileSizeMatch[1])
		}

		// Extract page count for announcement type hint
		pagePattern := regexp.MustCompile(`<span class="page">(\d+)`)
		pageMatch := pagePattern.FindStringSubmatch(headlineCol)
		pageCount := 0
		if len(pageMatch) >= 2 {
			pageCount, _ = strconv.Atoi(pageMatch[1])
		}

		// Determine announcement type from headline keywords (since HTML doesn't provide type)
		annType := w.inferAnnouncementType(headline, pageCount)

		ann := ASXAnnouncement{
			Date:           dateTime,
			Headline:       headline,
			PDFURL:         pdfURL,
			PDFFilename:    idsId,
			DocumentKey:    idsId,
			FileSize:       fileSize,
			PriceSensitive: priceSensitive,
			Type:           annType,
		}

		announcements = append(announcements, ann)
	}

	return announcements
}

// inferAnnouncementType attempts to categorize announcement based on headline keywords.
func (w *MarketAnnouncementsWorker) inferAnnouncementType(headline string, pageCount int) string {
	headlineUpper := strings.ToUpper(headline)

	// Financial reports
	if strings.Contains(headlineUpper, "ANNUAL REPORT") ||
		strings.Contains(headlineUpper, "HALF YEAR") ||
		strings.Contains(headlineUpper, "FULL YEAR") ||
		strings.Contains(headlineUpper, "QUARTERLY") {
		return "PERIODIC REPORTS"
	}

	// Dividends
	if strings.Contains(headlineUpper, "DIVIDEND") {
		return "DISTRIBUTION"
	}

	// Director changes
	if strings.Contains(headlineUpper, "DIRECTOR") ||
		strings.Contains(headlineUpper, "APPOINTMENT") ||
		strings.Contains(headlineUpper, "RESIGNATION") {
		return "DIRECTOR APPOINTMENT/RESIGNATION"
	}

	// Substantial holders
	if strings.Contains(headlineUpper, "SUBSTANTIAL") ||
		strings.Contains(headlineUpper, "HOLDER") {
		return "SECURITY HOLDER DETAILS"
	}

	// AGM/Meetings
	if strings.Contains(headlineUpper, "AGM") ||
		strings.Contains(headlineUpper, "MEETING") {
		return "COMPANY ADMINISTRATION"
	}

	// Default based on content size
	if pageCount > 10 {
		return "PERIODIC REPORTS"
	}

	return "PROGRESS REPORT"
}

// calculateCutoffDate returns the cutoff date based on period string
func (w *MarketAnnouncementsWorker) calculateCutoffDate(period string) time.Time {
	now := time.Now()
	switch period {
	case "D1":
		return now.AddDate(0, 0, -1)
	case "W1":
		return now.AddDate(0, 0, -7)
	case "M1":
		return now.AddDate(0, -1, 0)
	case "M3":
		return now.AddDate(0, -3, 0)
	case "M6":
		return now.AddDate(0, -6, 0)
	case "Y1":
		return now.AddDate(0, -13, 0) // 13 months to ensure full year coverage including overlap
	case "Y5":
		return now.AddDate(-5, 0, 0)
	default:
		return time.Time{} // No cutoff
	}
}

// createDocument creates a Document from an ASX announcement
func (w *MarketAnnouncementsWorker) createDocument(ctx context.Context, ann ASXAnnouncement, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	// Build markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# ASX Announcement: %s\n\n", ann.Headline))
	content.WriteString(fmt.Sprintf("**Date**: %s\n", ann.Date.Format("2 January 2006 3:04 PM")))
	content.WriteString(fmt.Sprintf("**Company**: ASX:%s\n", asxCode))
	content.WriteString(fmt.Sprintf("**Type**: %s\n", ann.Type))

	if ann.PriceSensitive {
		content.WriteString("**Price Sensitive**: Yes ⚠️\n")
	} else {
		content.WriteString("**Price Sensitive**: No\n")
	}

	if ann.PDFURL != "" {
		content.WriteString(fmt.Sprintf("\n**Document**: [%s](%s)\n", ann.PDFFilename, ann.PDFURL))
	}
	if ann.FileSize != "" {
		content.WriteString(fmt.Sprintf("**File Size**: %s\n", ann.FileSize))
	}

	content.WriteString("\n---\n")
	content.WriteString("*Full announcement available at PDF link above*\n")

	// Build tags
	tags := []string{"asx-announcement", strings.ToLower(asxCode)}

	// Add date tag
	dateTag := fmt.Sprintf("date:%s", ann.Date.Format("2006-01-02"))
	tags = append(tags, dateTag)

	// Add price-sensitive tag if applicable
	if ann.PriceSensitive {
		tags = append(tags, "price-sensitive")
	}

	// Add job definition tags
	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}

	// Add output_tags from step config
	tags = append(tags, outputTags...)

	// Apply cache tags from context
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	// Build metadata
	metadata := map[string]interface{}{
		"asx_code":          asxCode,
		"headline":          ann.Headline,
		"announcement_date": ann.Date.Format(time.RFC3339),
		"announcement_type": ann.Type,
		"price_sensitive":   ann.PriceSensitive,
		"pdf_url":           ann.PDFURL,
		"document_key":      ann.DocumentKey,
		"parent_job_id":     parentJobID,
	}
	if ann.FileSize != "" {
		metadata["file_size"] = ann.FileSize
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "asx_announcement",
		SourceID:        ann.PDFURL,
		URL:             ann.PDFURL,
		Title:           fmt.Sprintf("ASX:%s - %s", asxCode, ann.Headline),
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

// fetchHistoricalPrices gets historical price data for price impact analysis.
// First tries to get data from market_fundamentals documents (created by MarketDataCollectionWorker).
// Falls back to direct Yahoo Finance API if no document exists.
func (w *MarketAnnouncementsWorker) fetchHistoricalPrices(ctx context.Context, asxCode, period string) ([]OHLCV, error) {
	// First, try to get price data from an existing market_fundamentals document
	// This avoids duplicate API calls and ensures consistency with market data collection
	prices, err := w.getPricesFromStockDataDocument(ctx, asxCode)
	if err == nil && len(prices) > 0 {
		w.logger.Info().
			Str("asx_code", asxCode).
			Int("price_count", len(prices)).
			Msg("Using price data from market_fundamentals document")
		return prices, nil
	}

	// Log that we're falling back to direct Yahoo Finance call
	w.logger.Info().
		Str("asx_code", asxCode).
		Err(err).
		Msg("No market_fundamentals document found, fetching directly from Yahoo Finance")

	// Fallback: Fetch directly from Yahoo Finance
	return w.fetchPricesFromYahoo(ctx, asxCode, period)
}

// getPricesFromStockDataDocument retrieves OHLCV data from the fundamentals document.
// Uses the FundamentalsDataProvider to get cached data or generate fresh data on-demand.
// Returns the historical_prices array from document metadata if available.
func (w *MarketAnnouncementsWorker) getPricesFromStockDataDocument(ctx context.Context, asxCode string) ([]OHLCV, error) {
	// Use fundamentals provider to get document (with on-demand generation if needed)
	if w.fundamentalsProvider == nil {
		return nil, fmt.Errorf("fundamentals provider not configured")
	}

	ticker := fmt.Sprintf("ASX:%s", strings.ToUpper(asxCode))
	result, err := w.fundamentalsProvider.GetFundamentals(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get fundamentals data: %w", err)
	}
	if result.Document == nil {
		return nil, fmt.Errorf("no fundamentals document available for %s", asxCode)
	}

	w.logger.Debug().
		Str("asx_code", asxCode).
		Str("status", string(result.Status)).
		Msg("Retrieved fundamentals document via provider")

	// Extract historical_prices from metadata
	return w.extractPricesFromDocument(result.Document)
}

// extractPricesFromDocument extracts OHLCV data from a document's metadata.
func (w *MarketAnnouncementsWorker) extractPricesFromDocument(doc *models.Document) ([]OHLCV, error) {
	histPrices, ok := doc.Metadata["historical_prices"].([]interface{})
	if !ok || len(histPrices) == 0 {
		return nil, fmt.Errorf("no historical_prices in document metadata")
	}

	var prices []OHLCV
	for _, p := range histPrices {
		priceMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		dateStr, _ := priceMap["date"].(string)
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		ohlcv := OHLCV{
			Date: date,
		}

		if v, ok := priceMap["open"].(float64); ok {
			ohlcv.Open = v
		}
		if v, ok := priceMap["high"].(float64); ok {
			ohlcv.High = v
		}
		if v, ok := priceMap["low"].(float64); ok {
			ohlcv.Low = v
		}
		if v, ok := priceMap["close"].(float64); ok {
			ohlcv.Close = v
		}
		if v, ok := priceMap["volume"].(float64); ok {
			ohlcv.Volume = int64(v)
		} else if v, ok := priceMap["volume"].(int64); ok {
			ohlcv.Volume = v
		}

		if ohlcv.Close > 0 {
			prices = append(prices, ohlcv)
		}
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no valid price data in document")
	}

	// Sort by date ascending
	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Date.Before(prices[j].Date)
	})

	return prices, nil
}

// FundamentalsFinancialData holds annual and quarterly financial data from the fundamentals document
type FundamentalsFinancialData struct {
	AnnualData    []FundamentalsFinancialPeriod
	QuarterlyData []FundamentalsFinancialPeriod
}

// FundamentalsFinancialPeriod represents financial data for a single period (matches FinancialPeriodEntry in market_fundamentals_worker.go)
type FundamentalsFinancialPeriod struct {
	EndDate         string  // Date string in YYYY-MM-DD format
	PeriodType      string  // "annual" or "quarterly"
	TotalRevenue    int64   // Revenue in currency units
	GrossProfit     int64   // Gross profit
	OperatingIncome int64   // Operating income
	NetIncome       int64   // Net income (profit/loss)
	EBITDA          int64   // EBITDA
	TotalAssets     int64   // Total assets
	TotalLiab       int64   // Total liabilities
	TotalEquity     int64   // Total equity
	OperatingCF     int64   // Operating cash flow
	FreeCF          int64   // Free cash flow
	GrossMargin     float64 // Gross margin percentage
	NetMargin       float64 // Net margin percentage
}

// getFinancialsFromFundamentalsDocument retrieves annual/quarterly financial data via the FundamentalsDataProvider.
// Uses the provider to get cached data or generate fresh data on-demand.
// Returns nil if no document found or no financial data available.
func (w *MarketAnnouncementsWorker) getFinancialsFromFundamentalsDocument(ctx context.Context, asxCode string) *FundamentalsFinancialData {
	// Use fundamentals provider to get document (with on-demand generation if needed)
	if w.fundamentalsProvider == nil {
		w.logger.Debug().
			Str("asx_code", asxCode).
			Msg("Fundamentals provider not configured")
		return nil
	}

	ticker := fmt.Sprintf("ASX:%s", strings.ToUpper(asxCode))
	result, err := w.fundamentalsProvider.GetFundamentals(ctx, ticker)
	if err != nil {
		w.logger.Debug().
			Str("asx_code", asxCode).
			Err(err).
			Msg("Failed to get fundamentals data via provider")
		return nil
	}
	if result.Document == nil {
		return nil
	}

	w.logger.Debug().
		Str("asx_code", asxCode).
		Str("status", string(result.Status)).
		Msg("Retrieved fundamentals document via provider")

	return w.extractFinancialsFromDocument(result.Document, asxCode)
}

// extractFinancialsFromDocument extracts financial data from a document's metadata.
func (w *MarketAnnouncementsWorker) extractFinancialsFromDocument(doc *models.Document, asxCode string) *FundamentalsFinancialData {
	result := &FundamentalsFinancialData{}

	// Extract annual_data from metadata - handle both []interface{} and []FinancialPeriodEntry types
	if annualRaw, exists := doc.Metadata["annual_data"]; exists {
		switch annualData := annualRaw.(type) {
		case []interface{}:
			for _, entry := range annualData {
				if period := parseFundamentalsFinancialPeriod(entry); period != nil {
					result.AnnualData = append(result.AnnualData, *period)
				}
			}
		case []FinancialPeriodEntry:
			for _, entry := range annualData {
				result.AnnualData = append(result.AnnualData, FundamentalsFinancialPeriod{
					EndDate:         entry.EndDate,
					PeriodType:      entry.PeriodType,
					TotalRevenue:    entry.TotalRevenue,
					GrossProfit:     entry.GrossProfit,
					OperatingIncome: entry.OperatingIncome,
					NetIncome:       entry.NetIncome,
					EBITDA:          entry.EBITDA,
					TotalAssets:     entry.TotalAssets,
					TotalLiab:       entry.TotalLiab,
					TotalEquity:     entry.TotalEquity,
					OperatingCF:     entry.OperatingCF,
					FreeCF:          entry.FreeCF,
					GrossMargin:     entry.GrossMargin,
					NetMargin:       entry.NetMargin,
				})
			}
		default:
			w.logger.Debug().
				Str("asx_code", asxCode).
				Str("type", fmt.Sprintf("%T", annualRaw)).
				Msg("Unexpected type for annual_data in fundamentals document")
		}
	}

	// Extract quarterly_data from metadata - handle both []interface{} and []FinancialPeriodEntry types
	if quarterlyRaw, exists := doc.Metadata["quarterly_data"]; exists {
		switch quarterlyData := quarterlyRaw.(type) {
		case []interface{}:
			for _, entry := range quarterlyData {
				if period := parseFundamentalsFinancialPeriod(entry); period != nil {
					result.QuarterlyData = append(result.QuarterlyData, *period)
				}
			}
		case []FinancialPeriodEntry:
			for _, entry := range quarterlyData {
				result.QuarterlyData = append(result.QuarterlyData, FundamentalsFinancialPeriod{
					EndDate:         entry.EndDate,
					PeriodType:      entry.PeriodType,
					TotalRevenue:    entry.TotalRevenue,
					GrossProfit:     entry.GrossProfit,
					OperatingIncome: entry.OperatingIncome,
					NetIncome:       entry.NetIncome,
					EBITDA:          entry.EBITDA,
					TotalAssets:     entry.TotalAssets,
					TotalLiab:       entry.TotalLiab,
					TotalEquity:     entry.TotalEquity,
					OperatingCF:     entry.OperatingCF,
					FreeCF:          entry.FreeCF,
					GrossMargin:     entry.GrossMargin,
					NetMargin:       entry.NetMargin,
				})
			}
		default:
			w.logger.Debug().
				Str("asx_code", asxCode).
				Str("type", fmt.Sprintf("%T", quarterlyRaw)).
				Msg("Unexpected type for quarterly_data in fundamentals document")
		}
	}

	if len(result.AnnualData) == 0 && len(result.QuarterlyData) == 0 {
		w.logger.Debug().
			Str("asx_code", asxCode).
			Msg("No annual/quarterly data in fundamentals document")
		return nil
	}

	w.logger.Debug().
		Str("asx_code", asxCode).
		Int("annual_periods", len(result.AnnualData)).
		Int("quarterly_periods", len(result.QuarterlyData)).
		Msg("Extracted financial data from fundamentals document")

	return result
}

// parseFundamentalsFinancialPeriod parses a single period entry from document metadata
func parseFundamentalsFinancialPeriod(entry interface{}) *FundamentalsFinancialPeriod {
	m, ok := entry.(map[string]interface{})
	if !ok {
		return nil
	}

	period := &FundamentalsFinancialPeriod{}

	if v, ok := m["end_date"].(string); ok {
		period.EndDate = v
	}
	if v, ok := m["period_type"].(string); ok {
		period.PeriodType = v
	}
	if v, ok := m["total_revenue"].(float64); ok {
		period.TotalRevenue = int64(v)
	}
	if v, ok := m["gross_profit"].(float64); ok {
		period.GrossProfit = int64(v)
	}
	if v, ok := m["operating_income"].(float64); ok {
		period.OperatingIncome = int64(v)
	}
	if v, ok := m["net_income"].(float64); ok {
		period.NetIncome = int64(v)
	}
	if v, ok := m["ebitda"].(float64); ok {
		period.EBITDA = int64(v)
	}
	if v, ok := m["total_assets"].(float64); ok {
		period.TotalAssets = int64(v)
	}
	if v, ok := m["total_liabilities"].(float64); ok {
		period.TotalLiab = int64(v)
	}
	if v, ok := m["total_equity"].(float64); ok {
		period.TotalEquity = int64(v)
	}
	if v, ok := m["operating_cash_flow"].(float64); ok {
		period.OperatingCF = int64(v)
	}
	if v, ok := m["free_cash_flow"].(float64); ok {
		period.FreeCF = int64(v)
	}
	if v, ok := m["gross_margin"].(float64); ok {
		period.GrossMargin = v
	}
	if v, ok := m["net_margin"].(float64); ok {
		period.NetMargin = v
	}

	// Only return if we have at least some data
	if period.EndDate == "" {
		return nil
	}

	return period
}

// fetchPricesFromYahoo is the fallback that fetches directly from Yahoo Finance API.
// This is used when no asx_stock_data document is available.
func (w *MarketAnnouncementsWorker) fetchPricesFromYahoo(ctx context.Context, asxCode, period string) ([]OHLCV, error) {
	// Convert period to Yahoo range - fetch extra history for impact analysis
	yahooRange := "2y" // Default to 2 years for comprehensive analysis
	switch period {
	case "D1", "W1", "M1":
		yahooRange = "3mo"
	case "M3":
		yahooRange = "6mo"
	case "M6":
		yahooRange = "1y"
	case "Y1":
		yahooRange = "2y"
	case "Y5":
		yahooRange = "5y"
	}

	yahooSymbol := asxCode + ".AX"
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%s",
		yahooSymbol, yahooRange)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Yahoo data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yahoo API returned status %d", resp.StatusCode)
	}

	var apiResp yahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Yahoo response: %w", err)
	}

	if len(apiResp.Chart.Result) == 0 || len(apiResp.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no data in Yahoo response")
	}

	result := apiResp.Chart.Result[0]
	quote := result.Indicators.Quote[0]

	var prices []OHLCV
	for i, ts := range result.Timestamp {
		if i >= len(quote.Close) || quote.Close[i] == 0 {
			continue
		}

		ohlcv := OHLCV{
			Date:  time.Unix(ts, 0),
			Close: quote.Close[i],
		}
		if i < len(quote.Open) {
			ohlcv.Open = quote.Open[i]
		}
		if i < len(quote.High) {
			ohlcv.High = quote.High[i]
		}
		if i < len(quote.Low) {
			ohlcv.Low = quote.Low[i]
		}
		if i < len(quote.Volume) {
			ohlcv.Volume = quote.Volume[i]
		}

		prices = append(prices, ohlcv)
	}

	// Sort by date ascending
	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Date.Before(prices[j].Date)
	})

	return prices, nil
}

// detectTradingHalt checks if an announcement is a trading halt or reinstatement
func detectTradingHalt(headline string) (isTradingHalt bool, isReinstatement bool) {
	headlineUpper := strings.ToUpper(headline)

	// Trading halt keywords
	haltKeywords := []string{
		"TRADING HALT",
		"VOLUNTARY SUSPENSION",
		"SUSPENSION FROM QUOTATION",
		"SUSPENDED FROM TRADING",
	}

	for _, kw := range haltKeywords {
		if strings.Contains(headlineUpper, kw) {
			return true, false
		}
	}

	// Reinstatement keywords
	reinstatementKeywords := []string{
		"REINSTATEMENT",
		"RESUMPTION OF TRADING",
		"TRADING RESUMES",
		"LIFTED SUSPENSION",
		"END OF SUSPENSION",
	}

	for _, kw := range reinstatementKeywords {
		if strings.Contains(headlineUpper, kw) {
			return false, true
		}
	}

	return false, false
}

// detectDividendAnnouncement checks if an announcement is dividend-related
func detectDividendAnnouncement(headline string, annType string) bool {
	headlineUpper := strings.ToUpper(headline)
	typeUpper := strings.ToUpper(annType)

	dividendKeywords := []string{
		"DIVIDEND",
		"DRP", // Dividend Reinvestment Plan
		"DISTRIBUTION",
		"EX-DATE",
		"EX DATE",
		"RECORD DATE",
		"PAYMENT DATE",
		"FRANKING",
		"UNFRANKED",
		"FRANKED",
	}

	for _, kw := range dividendKeywords {
		if strings.Contains(headlineUpper, kw) || strings.Contains(typeUpper, kw) {
			return true
		}
	}

	return false
}

// DeduplicationStats tracks announcements that were consolidated
type DeduplicationStats struct {
	TotalBefore     int                  // Count before deduplication
	TotalAfter      int                  // Count after deduplication
	DuplicatesFound int                  // Number of duplicates removed
	Groups          []DeduplicationGroup // Details of each duplicate group
}

// DeduplicationGroup represents a set of similar announcements consolidated into one
type DeduplicationGroup struct {
	Date      time.Time
	Headlines []string // All headlines in the group
	Count     int      // Number of announcements in group
}

// normalizeHeadline removes trailing ticker codes and whitespace for comparison
func normalizeHeadline(headline string) string {
	h := strings.TrimSpace(headline)
	// Remove trailing " - CODE" pattern (e.g., "Proposed issue of securities - EXR")
	if idx := strings.LastIndex(h, " - "); idx > 0 {
		// Check if suffix looks like a ticker code (2-4 uppercase letters)
		suffix := strings.TrimSpace(h[idx+3:])
		if len(suffix) >= 2 && len(suffix) <= 4 && isAllUpperAlpha(suffix) {
			h = strings.TrimSpace(h[:idx])
		}
	}
	return strings.ToUpper(h)
}

// isAllUpperAlpha checks if string is all uppercase letters
func isAllUpperAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return len(s) > 0
}

// appendixBasePattern matches "APPENDIX 3X", "APPENDIX 3Y", etc.
var appendixBasePattern = regexp.MustCompile(`APPENDIX\s+\d+[A-Z]`)

// getAppendixBase extracts the appendix base type (e.g., "APPENDIX 3Y" from "Appendix 3Y SN")
func getAppendixBase(headline string) string {
	headlineUpper := strings.ToUpper(headline)
	match := appendixBasePattern.FindString(headlineUpper)
	return match
}

// areSimilarHeadlines checks if two headlines should be considered duplicates
func areSimilarHeadlines(h1, h2 string) bool {
	// Exact match
	if h1 == h2 {
		return true
	}

	// Normalized match (removes trailing ticker code)
	norm1 := normalizeHeadline(h1)
	norm2 := normalizeHeadline(h2)
	if norm1 == norm2 {
		return true
	}

	// Appendix pattern: "Appendix 3Y XX" variants all match
	base1 := getAppendixBase(h1)
	base2 := getAppendixBase(h2)
	if base1 != "" && base2 != "" && base1 == base2 {
		return true
	}

	return false
}

// deduplicateAnnouncements consolidates same-day announcements with similar headlines
// Returns deduplicated list and statistics about what was consolidated
func deduplicateAnnouncements(announcements []ASXAnnouncement) ([]ASXAnnouncement, DeduplicationStats) {
	stats := DeduplicationStats{TotalBefore: len(announcements)}

	if len(announcements) == 0 {
		return announcements, stats
	}

	// Group by date
	byDate := make(map[string][]ASXAnnouncement)
	for _, ann := range announcements {
		dateKey := ann.Date.Format("2006-01-02")
		byDate[dateKey] = append(byDate[dateKey], ann)
	}

	var result []ASXAnnouncement

	for dateKey, dayAnnouncements := range byDate {
		// Within each day, find similar headline groups
		used := make(map[int]bool)

		for i := 0; i < len(dayAnnouncements); i++ {
			if used[i] {
				continue
			}

			// Start a new group with this announcement
			group := []ASXAnnouncement{dayAnnouncements[i]}
			used[i] = true

			// Find all similar announcements on same day
			for j := i + 1; j < len(dayAnnouncements); j++ {
				if used[j] {
					continue
				}
				if areSimilarHeadlines(dayAnnouncements[i].Headline, dayAnnouncements[j].Headline) {
					group = append(group, dayAnnouncements[j])
					used[j] = true
				}
			}

			// Keep only one representative (first one - typically most recent by time)
			result = append(result, group[0])

			// Track groups with duplicates
			if len(group) > 1 {
				headlines := make([]string, len(group))
				for k, a := range group {
					headlines[k] = a.Headline
				}
				date, _ := time.Parse("2006-01-02", dateKey)
				stats.Groups = append(stats.Groups, DeduplicationGroup{
					Date:      date,
					Headlines: headlines,
					Count:     len(group),
				})
			}
		}
	}

	stats.TotalAfter = len(result)
	stats.DuplicatesFound = stats.TotalBefore - stats.TotalAfter

	// Sort result by date descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.After(result[j].Date)
	})

	return result, stats
}

// isRoutineAnnouncement checks if announcement is a standard administrative filing
// that should be excluded from signal/noise analysis.
// These are regulatory filings that all companies must make and are NOT correlated
// with price/volume movements.
func isRoutineAnnouncement(headline string) (isRoutine bool, routineType string) {
	headlineUpper := strings.ToUpper(headline)

	// Ordered by specificity (more specific patterns first)
	routinePatterns := []struct {
		pattern     string
		routineType string
	}{
		{"NOTICE OF ANNUAL GENERAL MEETING", "AGM Notice"},
		{"NOTICE OF GENERAL MEETING", "Meeting Notice"},
		{"RESULTS OF MEETING", "Meeting Results"},
		{"PROPOSED ISSUE OF SECURITIES", "Securities Issue"},
		{"APPLICATION FOR QUOTATION OF SECURITIES", "Quotation Application"},
		{"APPLICATION FOR QUOTATION", "Quotation Application"},
		{"NOTIFICATION OF CESSATION OF SECURITIES", "Securities Cessation"},
		{"NOTIFICATION OF CESSATION", "Securities Cessation"},
		{"NOTIFICATION REGARDING UNQUOTED SECURITIES", "Unquoted Securities"},
		{"NOTIFICATION REGARDING UNQUOTED", "Unquoted Securities"},
		{"CHANGE OF DIRECTOR'S INTEREST NOTICE", "Director Interest Change"},
		{"CHANGE OF DIRECTORS INTEREST", "Director Interest Change"},
		{"APPENDIX 3Y", "Director Interest (3Y)"},
		{"APPENDIX 3X", "Initial Director Interest (3X)"},
		{"APPENDIX 3B", "New Issue (3B)"},
		{"APPENDIX 3G", "Issue Notification (3G)"},
		{"CLEANSING NOTICE", "Cleansing Notice"},
		{"CLEANSING STATEMENT", "Cleansing Notice"},
	}

	for _, p := range routinePatterns {
		if strings.Contains(headlineUpper, p.pattern) {
			return true, p.routineType
		}
	}
	return false, ""
}

// SignalNoiseResult contains the full result of signal-to-noise analysis
type SignalNoiseResult struct {
	Rating      SignalNoiseRating
	Rationale   string
	IsAnomaly   bool
	AnomalyType string // "NO_REACTION", "UNEXPECTED_REACTION", ""
}

// calculateSignalNoiseRating determines the overall signal quality based on price/volume impact
// Returns the rating, rationale, and anomaly information
func calculateSignalNoiseRating(ann ASXAnnouncement, impact *PriceImpactData, isTradingHalt, isReinstatement bool) SignalNoiseResult {
	var rationale strings.Builder
	result := SignalNoiseResult{}

	// Check for routine announcements FIRST - these are excluded from signal analysis
	isRoutine, routineType := isRoutineAnnouncement(ann.Headline)
	if isRoutine {
		return SignalNoiseResult{
			Rating:    SignalNoiseRoutine,
			Rationale: fmt.Sprintf("ROUTINE: Standard administrative filing (%s). Excluded from signal analysis - not correlated with price/volume movements.", routineType),
		}
	}

	// If no price data available, base rating on announcement characteristics only
	if impact == nil {
		if ann.PriceSensitive {
			result.Rating = SignalNoiseModerate
			result.Rationale = "Price-sensitive announcement (no price data available for impact analysis)"
			return result
		}
		if isTradingHalt {
			result.Rating = SignalNoiseLow
			result.Rationale = "Trading halt announced (no price data available for impact analysis)"
			return result
		}
		result.Rating = SignalNoiseNone
		result.Rationale = "No price data available for impact analysis"
		return result
	}

	// Calculate absolute price change for comparison
	absPriceChange := impact.ChangePercent
	if absPriceChange < 0 {
		absPriceChange = -absPriceChange
	}

	// Determine direction description
	direction := "no change"
	if impact.ChangePercent > 0.1 {
		direction = fmt.Sprintf("+%.1f%% increase", impact.ChangePercent)
	} else if impact.ChangePercent < -0.1 {
		direction = fmt.Sprintf("%.1f%% decrease", impact.ChangePercent)
	}

	// Volume analysis
	volumeDesc := "normal volume"
	if impact.VolumeChangeRatio >= 2.0 {
		volumeDesc = fmt.Sprintf("%.1fx volume spike", impact.VolumeChangeRatio)
	} else if impact.VolumeChangeRatio >= 1.5 {
		volumeDesc = fmt.Sprintf("%.1fx elevated volume", impact.VolumeChangeRatio)
	} else if impact.VolumeChangeRatio <= 0.5 {
		volumeDesc = fmt.Sprintf("%.1fx reduced volume", impact.VolumeChangeRatio)
	}

	// Add pre-announcement drift info if significant
	if impact.HasSignificantPreDrift {
		rationale.WriteString(fmt.Sprintf("PRE-ANNOUNCEMENT: %s ", impact.PreDriftInterpretation))
	}

	// HIGH_SIGNAL: Significant market impact
	// Criteria: Price change >=3% OR volume ratio >=2x, especially with price-sensitive flag
	if absPriceChange >= 3.0 || impact.VolumeChangeRatio >= 2.0 {
		rationale.WriteString(fmt.Sprintf("HIGH SIGNAL: Significant market reaction with %s and %s. ", direction, volumeDesc))
		if ann.PriceSensitive {
			rationale.WriteString("Confirmed price-sensitive announcement. ")
		} else {
			// Non-price-sensitive with high reaction = ANOMALY
			result.IsAnomaly = true
			result.AnomalyType = "UNEXPECTED_REACTION"
			rationale.WriteString("⚠️ ANOMALY: Non-price-sensitive announcement triggered significant market reaction. ")
		}
		if absPriceChange >= 5.0 {
			rationale.WriteString("Price movement exceeds 5% threshold indicating major market reassessment.")
		} else if impact.VolumeChangeRatio >= 3.0 {
			rationale.WriteString("Exceptional volume indicates strong investor interest.")
		}
		result.Rating = SignalNoiseHigh
		result.Rationale = rationale.String()
		return result
	}

	// MODERATE_SIGNAL: Notable market reaction
	// Criteria: Price change >=1.5% OR volume ratio >=1.5x
	if absPriceChange >= 1.5 || impact.VolumeChangeRatio >= 1.5 {
		rationale.WriteString(fmt.Sprintf("MODERATE SIGNAL: Notable market reaction with %s and %s. ", direction, volumeDesc))
		if ann.PriceSensitive {
			rationale.WriteString("Price-sensitive flag indicates company deemed this material. ")
		} else if !isTradingHalt && !isReinstatement {
			// Non-price-sensitive with notable reaction = mild anomaly
			result.IsAnomaly = true
			result.AnomalyType = "UNEXPECTED_REACTION"
			rationale.WriteString("Note: Non-price-sensitive announcement showed unexpected market response. ")
		}
		if isTradingHalt || isReinstatement {
			rationale.WriteString("Associated with trading halt activity. ")
		}
		result.Rating = SignalNoiseModerate
		result.Rationale = rationale.String()
		return result
	}

	// LOW_SIGNAL: Minimal but detectable market reaction
	// Criteria: Price change >=0.5% OR volume ratio >=1.2x
	if absPriceChange >= 0.5 || impact.VolumeChangeRatio >= 1.2 {
		rationale.WriteString(fmt.Sprintf("LOW SIGNAL: Minor market reaction with %s and %s. ", direction, volumeDesc))
		if ann.PriceSensitive {
			// Price-sensitive with only low reaction = mild anomaly
			result.IsAnomaly = true
			result.AnomalyType = "NO_REACTION"
			rationale.WriteString("⚠️ ANOMALY: Price-sensitive flag but market showed limited reaction. ")
		}
		result.Rating = SignalNoiseLow
		result.Rationale = rationale.String()
		return result
	}

	// NOISE: No meaningful price/volume impact
	rationale.WriteString(fmt.Sprintf("NOISE: No meaningful market impact - %s with %s. ", direction, volumeDesc))
	if isTradingHalt {
		rationale.WriteString("Trading halt with no subsequent price movement indicates non-material purpose. ")
	} else if isReinstatement {
		rationale.WriteString("Reinstatement with no price change suggests halt was procedural. ")
	} else if ann.PriceSensitive {
		// Price-sensitive with NO reaction = definite anomaly
		result.IsAnomaly = true
		result.AnomalyType = "NO_REACTION"
		rationale.WriteString("⚠️ ANOMALY: Price-sensitive announcement but market showed NO reaction - verify announcement accuracy. ")
	} else {
		rationale.WriteString("Announcement had no measurable effect on price or volume. ")
	}
	result.Rating = SignalNoiseNone
	result.Rationale = rationale.String()
	return result
}

// analyzeAnnouncements analyzes all announcements and adds relevance classification and price impact
// Returns deduplicated analyses and deduplication statistics
func (w *MarketAnnouncementsWorker) analyzeAnnouncements(ctx context.Context, announcements []ASXAnnouncement, asxCode string, prices []OHLCV) ([]AnnouncementAnalysis, DeduplicationStats) {
	// Deduplicate same-day similar announcements FIRST
	dedupedAnnouncements, dedupStats := deduplicateAnnouncements(announcements)

	var analyses []AnnouncementAnalysis

	for _, ann := range dedupedAnnouncements {
		category, reason := classifyRelevance(ann)

		// Detect trading halts
		isTradingHalt, isReinstatement := detectTradingHalt(ann.Headline)

		// Detect dividend announcements
		isDividend := detectDividendAnnouncement(ann.Headline, ann.Type)

		// Detect routine administrative filings
		isRoutine, routineType := isRoutineAnnouncement(ann.Headline)

		analysis := AnnouncementAnalysis{
			Date:                   ann.Date,
			Headline:               ann.Headline,
			Type:                   ann.Type,
			PriceSensitive:         ann.PriceSensitive,
			PDFURL:                 ann.PDFURL,
			DocumentKey:            ann.DocumentKey,
			RelevanceCategory:      category,
			RelevanceReason:        reason,
			IsTradingHalt:          isTradingHalt,
			IsReinstatement:        isReinstatement,
			IsDividendAnnouncement: isDividend,
			IsRoutine:              isRoutine,
			RoutineType:            routineType,
		}

		// Add price impact if we have price data
		if len(prices) > 0 {
			analysis.PriceImpact = w.calculatePriceImpact(ann.Date, prices)
		}

		// Calculate signal-to-noise rating based on actual market impact
		snResult := calculateSignalNoiseRating(ann, analysis.PriceImpact, isTradingHalt, isReinstatement)
		analysis.SignalNoiseRating = snResult.Rating
		analysis.SignalNoiseRationale = snResult.Rationale
		analysis.IsAnomaly = snResult.IsAnomaly
		analysis.AnomalyType = snResult.AnomalyType

		// Add dividend context to rationale if applicable
		if isDividend && analysis.PriceImpact != nil && analysis.PriceImpact.ChangePercent < 0 {
			analysis.SignalNoiseRationale += " Note: Negative price movement may be due to ex-dividend adjustment rather than negative market reaction."
		}

		// Apply critical review classification (REQ-1)
		// Uses ClassifyAnnouncementWithContent from signal_analysis_classifier.go
		// Includes headline content analysis for MANAGEMENT_BLUFF detection
		if analysis.PriceImpact != nil {
			metrics := ClassificationMetrics{
				DayOfChange: analysis.PriceImpact.ChangePercent,
				PreDrift:    analysis.PriceImpact.PreAnnouncementDrift,
				VolumeRatio: analysis.PriceImpact.VolumeChangeRatio,
			}
			// Use routine type as category for SENTIMENT_NOISE detection
			category := analysis.RoutineType
			if isRoutine {
				category = "ROUTINE"
			}
			// Pass headline for content-based MANAGEMENT_BLUFF detection
			analysis.SignalClassification = ClassifyAnnouncementWithContent(metrics, ann.PriceSensitive, category, ann.Headline)
		} else if isRoutine {
			analysis.SignalClassification = ClassificationRoutine
		}

		analyses = append(analyses, analysis)
	}

	// Sort by date descending (most recent first)
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].Date.After(analyses[j].Date)
	})

	return analyses, dedupStats
}

// classifyRelevance determines the relevance category of an announcement
func classifyRelevance(ann ASXAnnouncement) (category string, reason string) {
	// HIGH: Price-sensitive or major events
	if ann.PriceSensitive {
		return "HIGH", "Price-sensitive announcement"
	}

	typeUpper := strings.ToUpper(ann.Type)
	headlineUpper := strings.ToUpper(ann.Headline)

	// HIGH types - major corporate events
	highKeywords := []string{
		"TAKEOVER", "ACQUISITION", "MERGER", "DISPOSAL",
		"DIVIDEND", "CAPITAL RAISING", "PLACEMENT", "SPP", "RIGHTS ISSUE",
		"FINANCIAL REPORT", "HALF YEAR", "FULL YEAR", "ANNUAL REPORT",
		"QUARTERLY", "PRELIMINARY FINAL", "EARNINGS",
		"GUIDANCE", "FORECAST", "OUTLOOK",
		"ASSET SALE", "DIVESTMENT",
	}

	for _, kw := range highKeywords {
		if strings.Contains(typeUpper, kw) || strings.Contains(headlineUpper, kw) {
			return "HIGH", fmt.Sprintf("Contains '%s'", kw)
		}
	}

	// MEDIUM types - governance and significant operational
	mediumKeywords := []string{
		"DIRECTOR", "CHAIRMAN", "CEO", "CFO", "MANAGING DIRECTOR",
		"APPOINTMENT", "RESIGNATION", "RETIREMENT",
		"AGM", "EGM", "GENERAL MEETING",
		"CONTRACT", "AGREEMENT", "PARTNERSHIP", "JOINT VENTURE",
		"EXPLORATION", "DRILLING", "RESOURCE", "RESERVE",
		"REGULATORY", "APPROVAL", "LICENSE", "PERMIT",
	}

	for _, kw := range mediumKeywords {
		if strings.Contains(typeUpper, kw) || strings.Contains(headlineUpper, kw) {
			return "MEDIUM", fmt.Sprintf("Contains '%s'", kw)
		}
	}

	// LOW types - routine disclosures
	lowKeywords := []string{
		"PROGRESS REPORT", "UPDATE", "INVESTOR PRESENTATION",
		"DISCLOSURE", "CLEANSING", "STATEMENT",
		"APPENDIX", "SUBSTANTIAL HOLDER",
		"CHANGE OF ADDRESS", "COMPANY SECRETARY",
	}

	for _, kw := range lowKeywords {
		if strings.Contains(typeUpper, kw) || strings.Contains(headlineUpper, kw) {
			return "LOW", fmt.Sprintf("Routine disclosure: '%s'", kw)
		}
	}

	return "NOISE", "No material indicators found"
}

// calculatePriceImpact calculates stock price movement around an announcement date.
// Uses date-based lookups to find:
// - PriceBefore: Closing price on the trading day BEFORE the announcement
// - PriceAfter: Closing price on the announcement date (or next trading day if announcement is after market close)
// This measures the immediate market reaction to the announcement.
func (w *MarketAnnouncementsWorker) calculatePriceImpact(announcementDate time.Time, prices []OHLCV) *PriceImpactData {
	if len(prices) == 0 {
		return nil
	}

	// Build date-to-price map for O(1) lookups
	// Also keep prices sorted by date for finding adjacent trading days
	priceMap := make(map[string]OHLCV)
	for _, p := range prices {
		priceMap[p.Date.Format("2006-01-02")] = p
	}

	// Normalize announcement date to date only (remove time component)
	annDateStr := announcementDate.Format("2006-01-02")

	// Find price on announcement date (or closest trading day after)
	var priceOnDate OHLCV
	foundOnDate := false

	// Look for exact match first
	if p, ok := priceMap[annDateStr]; ok {
		priceOnDate = p
		foundOnDate = true
	} else {
		// Announcement might be on weekend/holiday - find next trading day
		for i := 1; i <= 5; i++ {
			checkDate := announcementDate.AddDate(0, 0, i).Format("2006-01-02")
			if p, ok := priceMap[checkDate]; ok {
				priceOnDate = p
				foundOnDate = true
				break
			}
		}
	}

	if !foundOnDate {
		return nil
	}

	// Find previous trading day's price (look backwards from announcement)
	var priceBefore OHLCV
	foundBefore := false
	for i := 1; i <= 10; i++ {
		checkDate := announcementDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok {
			priceBefore = p
			foundBefore = true
			break
		}
	}

	if !foundBefore {
		return nil
	}

	// Calculate volumes before announcement (5 trading days)
	volumeBefore := int64(0)
	volumeCount := 0
	for i := 1; i <= 15 && volumeCount < 5; i++ {
		checkDate := announcementDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok && p.Volume > 0 {
			volumeBefore += p.Volume
			volumeCount++
		}
	}
	if volumeCount > 0 {
		volumeBefore = volumeBefore / int64(volumeCount)
	}

	// Calculate volumes after announcement (5 trading days)
	volumeAfter := int64(0)
	volumeCount = 0
	for i := 0; i <= 15 && volumeCount < 5; i++ {
		checkDate := announcementDate.AddDate(0, 0, i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok && p.Volume > 0 {
			volumeAfter += p.Volume
			volumeCount++
		}
	}
	if volumeCount > 0 {
		volumeAfter = volumeAfter / int64(volumeCount)
	}

	// Use announcement day close vs previous day close for immediate impact
	// This measures the actual price movement on the announcement date
	// NOT cumulative changes over multiple days
	priceBeforeVal := priceBefore.Close
	priceAfterVal := priceOnDate.Close

	// Calculate changes
	changePercent := 0.0
	if priceBeforeVal > 0 {
		changePercent = ((priceAfterVal - priceBeforeVal) / priceBeforeVal) * 100
	}

	volumeRatio := 0.0
	if volumeBefore > 0 {
		volumeRatio = float64(volumeAfter) / float64(volumeBefore)
	}

	// Determine impact signal
	impactSignal := "MINIMAL"
	absChange := changePercent
	if absChange < 0 {
		absChange = -absChange
	}

	if absChange >= 5 || volumeRatio >= 2.0 {
		impactSignal = "SIGNIFICANT"
	} else if absChange >= 2 || volumeRatio >= 1.5 {
		impactSignal = "MODERATE"
	}

	// Calculate pre-announcement drift (T-5 to T-1)
	// This detects price movement BEFORE the announcement that might indicate information leaks
	var priceT5, priceT1 float64
	var preAnnouncementDrift float64
	var hasSignificantPreDrift bool
	var preDriftInterpretation string

	// Find T-5 price (5 trading days before announcement)
	for i := 5; i <= 15; i++ {
		checkDate := announcementDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok {
			priceT5 = p.Close
			break
		}
	}

	// T-1 price is priceBefore (already calculated)
	priceT1 = priceBeforeVal

	if priceT5 > 0 && priceT1 > 0 {
		preAnnouncementDrift = ((priceT1 - priceT5) / priceT5) * 100
		absPreDrift := preAnnouncementDrift
		if absPreDrift < 0 {
			absPreDrift = -absPreDrift
		}

		// Significant pre-drift threshold: 2%
		if absPreDrift >= 2.0 {
			hasSignificantPreDrift = true
			if preAnnouncementDrift > 0 {
				preDriftInterpretation = fmt.Sprintf("+%.1f%%", preAnnouncementDrift)
			} else {
				preDriftInterpretation = fmt.Sprintf("%.1f%%", preAnnouncementDrift)
			}
		} else {
			preDriftInterpretation = "-"
		}
	}

	return &PriceImpactData{
		PriceBefore:            priceBeforeVal,
		PriceAfter:             priceAfterVal,
		ChangePercent:          changePercent,
		VolumeBefore:           volumeBefore,
		VolumeAfter:            volumeAfter,
		VolumeChangeRatio:      volumeRatio,
		ImpactSignal:           impactSignal,
		PreAnnouncementDrift:   preAnnouncementDrift,
		PreAnnouncementPriceT5: priceT5,
		PreAnnouncementPriceT1: priceT1,
		HasSignificantPreDrift: hasSignificantPreDrift,
		PreDriftInterpretation: preDriftInterpretation,
	}
}

// AnnouncementSummaryData holds counts and data needed for AI summary generation
type AnnouncementSummaryData struct {
	ASXCode                    string
	HighSignalCount            int
	ModerateSignalCount        int
	LowSignalCount             int
	NoiseCount                 int
	RoutineCount               int
	TradingHaltCount           int
	AnomalyNoReactionCount     int
	AnomalyUnexpectedCount     int
	PreDriftCount              int
	PriceSensitiveTotal        int
	PriceSensitiveWithReaction int
	ConvictionScore            int
	LeakScore                  float64
	CommunicationStyle         string
	HighSignalAnnouncements    []AnnouncementAnalysis
}

// generateAISummary uses AI to create an executive summary of the announcement analysis
func (w *MarketAnnouncementsWorker) generateAISummary(ctx context.Context, data AnnouncementSummaryData) (string, error) {
	if w.providerFactory == nil {
		return "", nil
	}

	// Build prompt with analysis data
	prompt := w.buildSummaryPrompt(data)

	systemInstruction := `You are a senior financial analyst. Provide an executive summary as a BULLET POINT LIST ONLY.

STRICT FORMAT - YOU MUST FOLLOW THIS EXACTLY:
- Start IMMEDIATELY with a bullet point (no tables, no headers, no introductions)
- Each line MUST start with "- " (dash space)
- Maximum 5-7 bullet points total
- Each bullet: 1-2 sentences, objective, third person
- NO tables, NO headers, NO bold text, NO markdown formatting

Cover these topics (one bullet each):
- Announcement quality and market impact patterns
- Pre-announcement price drift observations
- Communication style and disclosure patterns
- Key investor implications

WRONG FORMAT (DO NOT DO THIS):
| Column | Column |
**Bold text**
## Headers

CORRECT FORMAT (DO THIS):
- First insight about the company...
- Second insight about patterns...
- Third insight about risks...`

	request := &llm.ContentRequest{
		Messages: []interfaces.Message{
			{Role: "user", Content: prompt},
		},
		SystemInstruction: systemInstruction,
		Temperature:       0.3,
		MaxTokens:         800,
	}

	resp, err := w.providerFactory.GenerateContent(ctx, request)
	if err != nil {
		return "", fmt.Errorf("AI summary generation failed: %w", err)
	}

	return resp.Text, nil
}

// buildSummaryPrompt constructs the prompt for AI summary generation
func (w *MarketAnnouncementsWorker) buildSummaryPrompt(data AnnouncementSummaryData) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Analyze the announcement history for ASX:%s and provide an executive summary.\n\n", data.ASXCode))

	// Signal distribution
	sb.WriteString("SIGNAL DISTRIBUTION (based on actual price/volume impact):\n")
	sb.WriteString(fmt.Sprintf("- HIGH_SIGNAL (significant market reaction): %d\n", data.HighSignalCount))
	sb.WriteString(fmt.Sprintf("- MODERATE_SIGNAL (notable reaction): %d\n", data.ModerateSignalCount))
	sb.WriteString(fmt.Sprintf("- LOW_SIGNAL (minor impact): %d\n", data.LowSignalCount))
	sb.WriteString(fmt.Sprintf("- NOISE (no meaningful impact): %d\n", data.NoiseCount))
	sb.WriteString(fmt.Sprintf("- ROUTINE (administrative filings): %d\n", data.RoutineCount))
	if data.TradingHaltCount > 0 {
		sb.WriteString(fmt.Sprintf("- Trading Halts: %d\n", data.TradingHaltCount))
	}
	sb.WriteString("\n")

	// Price-sensitive accuracy
	if data.PriceSensitiveTotal > 0 {
		accuracy := float64(data.PriceSensitiveWithReaction) / float64(data.PriceSensitiveTotal) * 100
		sb.WriteString("PRICE-SENSITIVE ACCURACY:\n")
		sb.WriteString(fmt.Sprintf("- Announcements marked price-sensitive: %d\n", data.PriceSensitiveTotal))
		sb.WriteString(fmt.Sprintf("- Actually caused market reaction: %d (%.0f%%)\n\n", data.PriceSensitiveWithReaction, accuracy))
	}

	// Anomalies
	if data.AnomalyNoReactionCount > 0 || data.AnomalyUnexpectedCount > 0 {
		sb.WriteString("ANOMALIES DETECTED:\n")
		if data.AnomalyNoReactionCount > 0 {
			sb.WriteString(fmt.Sprintf("- Price-sensitive with NO market reaction: %d\n", data.AnomalyNoReactionCount))
		}
		if data.AnomalyUnexpectedCount > 0 {
			sb.WriteString(fmt.Sprintf("- Unexpected reactions to routine news: %d\n", data.AnomalyUnexpectedCount))
		}
		sb.WriteString("\n")
	}

	// Pre-announcement drift
	if data.PreDriftCount > 0 {
		sb.WriteString("PRE-ANNOUNCEMENT MOVEMENT:\n")
		sb.WriteString(fmt.Sprintf("- Trading days with significant pre-drift (>=2%%): %d\n", data.PreDriftCount))
		sb.WriteString("- This may indicate information leakage or market anticipation\n\n")
	}

	// High signal announcements detail
	if len(data.HighSignalAnnouncements) > 0 {
		sb.WriteString("HIGH SIGNAL ANNOUNCEMENTS:\n")
		for i, a := range data.HighSignalAnnouncements {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(data.HighSignalAnnouncements)-5))
				break
			}
			priceStr := "N/A"
			if a.PriceImpact != nil {
				sign := ""
				if a.PriceImpact.ChangePercent > 0 {
					sign = "+"
				}
				priceStr = fmt.Sprintf("%s%.1f%%", sign, a.PriceImpact.ChangePercent)
			}
			sb.WriteString(fmt.Sprintf("- %s: %s (%s)\n", a.Date.Format("2006-01-02"), a.Headline, priceStr))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Based on this data, provide a Critical Forensic Audit of the company's announcements.\n\n")

	// 1. Tone & Style Directive (Constraint)
	if data.ConvictionScore < 4 || data.LeakScore > 0.2 {
		sb.WriteString("CRITICAL INSTRUCTION: The data indicates LOW CONVICTION or HIGH LEAK RISK.\n")
		sb.WriteString("You MUST NOT use promotional language like 'high-quality', 'proactive', or 'reliable'.\n")
		sb.WriteString("Act as a short-seller or forensic auditor. Focus on the divergence between Management Narrative and Operational Reality.\n\n")
	} else {
		sb.WriteString("Instruction: Provide a balanced, data-driven assessment.\n\n")
	}

	// 2. Output Format (Table)
	sb.WriteString("OUTPUT FORMAT:\n")
	sb.WriteString("Provide a table with the following columns:\n")
	sb.WriteString("| Focus Area | Management Narrative (The 'Noise') | Operational Reality (The 'Signal') | Critical Assessment |\n")
	sb.WriteString("|---|---|---|---|\n")
	sb.WriteString("Row 1: Operational Success (Claims vs Results)\n")
	sb.WriteString("Row 2: Market Integrity (Leakage & Timing)\n")
	sb.WriteString("Row 3: Technical Milestones (Drilling/Product vs Market Reaction)\n\n")

	// 3. Narrative Sections
	sb.WriteString("Then, provide a brief execution summary (2 paragraphs) covering:\n")
	sb.WriteString("- Strategic Divergence: Does the market sell the news on price-sensitive items?\n")
	sb.WriteString("- Speculative Patterns: Is the stock driven by technical milestones or retail hype/sentiment?\n")

	// Data context for the AI
	if data.LeakScore > 0.3 {
		sb.WriteString(fmt.Sprintf("\nNOTE: Leak Score is %.1f%% (High). Highlight pre-announcement drift.\n", data.LeakScore*100))
	}
	if data.PreDriftCount > 0 {
		sb.WriteString(fmt.Sprintf("NOTE: %d instances of significant pre-announcement drift (>2%%).\n", data.PreDriftCount))
	}

	return sb.String()
}

// ReportingDate represents a historical mandatory reporting date extracted from announcements
type ReportingDate struct {
	ReportType string
	Date       time.Time
	Reference  string // "Actual" or source
}

// PredictedReport represents a predicted upcoming mandatory report
type PredictedReport struct {
	ReportType    string
	PredictedDate string // e.g., "Mid-January 2026"
	Basis         string // Explanation of prediction
	IsImminent    bool   // True if within 2 months
}

// isFYRelatedAnnouncement checks if an announcement is related to fiscal year reporting
// These are excluded from the non-FY impact rating as they have their own reporting cycle
func isFYRelatedAnnouncement(headline string) bool {
	upper := strings.ToUpper(headline)
	// FY-related patterns to exclude from non-FY impact rating
	fyPatterns := []string{
		"ANNUAL REPORT", "APPENDIX 4E", "4E AND",
		"FY20", "FY19", "FY21", "FY22", "FY23", "FY24", "FY25", "FY26", "FY27",
		"FULL YEAR", "FULL-YEAR", "FORM 20-F", "20-F",
		"ECONOMIC CONTRIBUTION REPORT", // Published with FY results
	}
	for _, pattern := range fyPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// buildSignalSummaryFromAnalyses constructs a SignalSummary from AnnouncementAnalysis slice.
// This bridges the AnnouncementAnalysis data to the SignalSummary format used by scoring functions.
func buildSignalSummaryFromAnalyses(analyses []AnnouncementAnalysis) SignalSummary {
	summary := SignalSummary{
		TotalAnnouncements: len(analyses),
		ConvictionScore:    5, // Default
		CommunicationStyle: StyleStandard,
	}

	if len(analyses) == 0 {
		return summary
	}

	priceSensitiveCount := 0
	var classifications []AnnouncementClassification

	// Count classifications and build classification list for scoring
	for _, a := range analyses {
		switch a.SignalClassification {
		case ClassificationTrueSignal:
			summary.CountTrueSignal++
		case ClassificationPricedIn:
			summary.CountPricedIn++
		case ClassificationSentimentNoise:
			summary.CountSentimentNoise++
		case ClassificationManagementBluff:
			summary.CountManagementBluff++
		case ClassificationRoutine:
			summary.CountRoutine++
		}

		if a.PriceSensitive {
			priceSensitiveCount++
		}

		// Build classification for conviction score calculation
		if a.PriceImpact != nil {
			classifications = append(classifications, AnnouncementClassification{
				Date:                a.Date,
				Title:               a.Headline,
				ManagementSensitive: a.PriceSensitive,
				Classification:      a.SignalClassification,
				Metrics: ClassificationMetrics{
					DayOfChange: a.PriceImpact.ChangePercent,
					PreDrift:    a.PriceImpact.PreAnnouncementDrift,
					VolumeRatio: a.PriceImpact.VolumeChangeRatio,
				},
			})
		}
	}

	// Calculate ratios
	total := float64(summary.TotalAnnouncements)
	if total > 0 {
		summary.SignalRatio = float64(summary.CountTrueSignal) / total
		summary.NoiseRatio = float64(summary.CountSentimentNoise) / total
	}

	// Calculate price-sensitive-dependent metrics
	if priceSensitiveCount > 0 {
		psCount := float64(priceSensitiveCount)
		summary.LeakScore = float64(summary.CountPricedIn) / psCount
		summary.CredibilityScore = 1.0 - (float64(summary.CountManagementBluff) / psCount)
	} else {
		summary.LeakScore = 0
		summary.CredibilityScore = 1.0
	}

	// Clamp values to [0, 1]
	if summary.CredibilityScore < 0 {
		summary.CredibilityScore = 0
	}
	if summary.CredibilityScore > 1 {
		summary.CredibilityScore = 1
	}

	// Calculate conviction score and communication style using existing functions
	summary.ConvictionScore = CalculateConvictionScore(summary, classifications)
	summary.CommunicationStyle = DetermineCommunicationStyle(summary)

	return summary
}

// calculateNonFYImpactRating calculates the overall impact rating excluding FY announcements
// DEPRECATED: This function is replaced by buildSignalSummaryFromAnalyses for conviction-based rating
func calculateNonFYImpactRating(analyses []AnnouncementAnalysis) (rating string, color string, emoji string, justification string) {
	// Count non-FY signals
	nonFYHigh, nonFYModerate := 0, 0
	keyEventsSet := make(map[string]bool)
	var keyEventDetails []string

	for _, a := range analyses {
		if isFYRelatedAnnouncement(a.Headline) {
			continue
		}

		upper := strings.ToUpper(a.Headline)
		switch a.SignalNoiseRating {
		case SignalNoiseHigh:
			nonFYHigh++
			if strings.Contains(upper, "QUARTERLY") {
				if !keyEventsSet["quarterly"] {
					keyEventsSet["quarterly"] = true
					keyEventDetails = append(keyEventDetails, "**Quarterly Activities Reports**: Consistently trigger 2-3% price movements with elevated volume")
				}
			} else if strings.Contains(upper, "DIVIDEND") {
				if !keyEventsSet["dividend"] {
					keyEventsSet["dividend"] = true
					keyEventDetails = append(keyEventDetails, "**Dividend/Distribution Updates**: Show strong market response particularly around ex-dividend timing")
				}
			}
		case SignalNoiseModerate:
			nonFYModerate++
			if strings.Contains(upper, "CONFERENCE") || strings.Contains(upper, "PRESENTATION") {
				if !keyEventsSet["conference"] {
					keyEventsSet["conference"] = true
					keyEventDetails = append(keyEventDetails, "**Conference Presentations**: Generate moderate interest with ~2% price reactions")
				}
			}
		}
	}

	// Determine rating based on non-FY signal counts
	if nonFYHigh >= 3 || (nonFYHigh >= 1 && nonFYModerate >= 5) {
		rating = "HIGH"
		color = "#d4edda" // Soft green
		emoji = "🔥"
	} else if nonFYHigh >= 1 || nonFYModerate >= 3 {
		rating = "MODERATE"
		color = "#fff3cd" // Soft orange/yellow
		emoji = "📊"
	} else {
		rating = "LOW"
		color = "#cce5ff" // Soft blue
		emoji = "📉"
	}

	// Build justification
	var justBuilder strings.Builder
	justBuilder.WriteString(fmt.Sprintf("**Justification**: Excluding annual and full-year reporting events, announcements demonstrate a **%s** likelihood of impacting price and volume. ", strings.ToLower(rating)))

	if len(keyEventDetails) > 0 {
		justBuilder.WriteString("The primary market-moving events outside FY updates are:\n\n")
		for _, detail := range keyEventDetails {
			justBuilder.WriteString(fmt.Sprintf("- %s\n", detail))
		}
		justBuilder.WriteString("\n")
	}

	justBuilder.WriteString("The majority of non-FY, non-routine announcements fall into the MODERATE_SIGNAL or LOW_SIGNAL categories, indicating predictable market behavior with occasional high-impact events concentrated around quarterly operational disclosures and dividend announcements.\n\n")
	justBuilder.WriteString("*Note: This rating excludes FY-related announcements (Annual Report, Appendix 4E, FY Results) which are covered separately in the periodic reporting calendar below.*")

	justification = justBuilder.String()
	return
}

// extractHistoricalReports extracts mandatory reporting dates from announcement history
func extractHistoricalReports(analyses []AnnouncementAnalysis) []ReportingDate {
	var reports []ReportingDate
	seen := make(map[string]bool)

	for _, a := range analyses {
		upper := strings.ToUpper(a.Headline)
		var reportType string
		var quarterNum string

		// Detect report type from headline
		if strings.Contains(upper, "QUARTERLY ACTIVITIES REPORT") ||
			(strings.Contains(upper, "QUARTERLY") && strings.Contains(upper, "REPORT")) {
			// Try to extract quarter number (Q1, Q2, Q3, Q4)
			if strings.Contains(upper, "Q1") {
				quarterNum = "Q1"
			} else if strings.Contains(upper, "Q2") {
				quarterNum = "Q2"
			} else if strings.Contains(upper, "Q3") {
				quarterNum = "Q3"
			} else if strings.Contains(upper, "Q4") {
				quarterNum = "Q4"
			}
			// Determine fiscal year based on month
			fyYear := a.Date.Year()
			month := a.Date.Month()
			// For June FYE: Jul-Dec is first half of FY, Jan-Jun is second half
			if month >= 7 {
				fyYear++ // Q1/Q2 of next FY
			}
			if quarterNum != "" {
				reportType = fmt.Sprintf("%s FY%02d Quarterly Activities Report", quarterNum, fyYear%100)
			} else {
				reportType = "Quarterly Activities Report"
			}
		} else if strings.Contains(upper, "4E") ||
			(strings.Contains(upper, "ANNUAL REPORT") && strings.Contains(upper, "APPENDIX")) {
			fyYear := a.Date.Year()
			reportType = fmt.Sprintf("FY%02d Full-Year Results (Appendix 4E)", fyYear%100)
		} else if strings.Contains(upper, "RESULTS OF") && strings.Contains(upper, "ANNUAL GENERAL MEETING") {
			reportType = fmt.Sprintf("%d AGM", a.Date.Year())
		} else if strings.Contains(upper, "NOTICE OF ANNUAL GENERAL MEETING") {
			reportType = "AGM Notice"
		} else if strings.Contains(upper, "4D") || strings.Contains(upper, "HALF YEAR") ||
			strings.Contains(upper, "HALF-YEAR") {
			reportType = "Half-Year Report (Appendix 4D)"
		}

		if reportType != "" {
			// Deduplicate by report type and date
			key := fmt.Sprintf("%s-%s", reportType, a.Date.Format("2006-01-02"))
			if !seen[key] {
				seen[key] = true
				reports = append(reports, ReportingDate{
					ReportType: reportType,
					Date:       a.Date,
					Reference:  "Actual",
				})
			}
		}
	}

	// Sort by date descending (most recent first)
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Date.After(reports[j].Date)
	})

	// Limit to most recent 6 reports
	if len(reports) > 6 {
		reports = reports[:6]
	}

	return reports
}

// predictUpcomingReports predicts upcoming mandatory reports based on historical patterns
func predictUpcomingReports(historical []ReportingDate, asxCode string) []PredictedReport {
	var predictions []PredictedReport
	now := time.Now()
	twoMonthsFromNow := now.AddDate(0, 2, 0)

	// Find most recent reports of each type
	var lastQuarterly time.Time
	var lastFY time.Time
	var lastAGM time.Time
	var lastHalfYear time.Time

	for _, r := range historical {
		switch {
		case strings.Contains(r.ReportType, "Quarterly"):
			if lastQuarterly.IsZero() || r.Date.After(lastQuarterly) {
				lastQuarterly = r.Date
			}
		case strings.Contains(r.ReportType, "Full-Year") || strings.Contains(r.ReportType, "4E"):
			if lastFY.IsZero() || r.Date.After(lastFY) {
				lastFY = r.Date
			}
		case strings.Contains(r.ReportType, "AGM") && !strings.Contains(r.ReportType, "Notice"):
			if lastAGM.IsZero() || r.Date.After(lastAGM) {
				lastAGM = r.Date
			}
		case strings.Contains(r.ReportType, "Half-Year") || strings.Contains(r.ReportType, "4D"):
			if lastHalfYear.IsZero() || r.Date.After(lastHalfYear) {
				lastHalfYear = r.Date
			}
		}
	}

	// Determine fiscal year end (assume June 30 for most ASX companies)
	fyeMonth := time.June
	currentFY := now.Year()
	if now.Month() > fyeMonth {
		currentFY++ // We're in the first half of next FY
	}

	// Predict next quarterly (~3 months from last)
	if !lastQuarterly.IsZero() {
		nextQ := lastQuarterly.AddDate(0, 3, 0)
		if nextQ.After(now) {
			isImminent := nextQ.Before(twoMonthsFromNow)
			// Determine quarter based on timing
			qMonth := nextQ.Month()
			var qName string
			switch {
			case qMonth >= 1 && qMonth <= 2:
				qName = "Q2 FY" + fmt.Sprintf("%02d", currentFY%100)
			case qMonth >= 4 && qMonth <= 5:
				qName = "Q3 FY" + fmt.Sprintf("%02d", currentFY%100)
			case qMonth >= 7 && qMonth <= 8:
				qName = "Q4 FY" + fmt.Sprintf("%02d", (currentFY-1)%100)
			case qMonth >= 10 && qMonth <= 11:
				qName = "Q1 FY" + fmt.Sprintf("%02d", currentFY%100)
			default:
				qName = "Quarterly"
			}
			predictions = append(predictions, PredictedReport{
				ReportType:    qName + " Activities Report",
				PredictedDate: formatPredictedDate(nextQ),
				Basis:         "~3 months after Q1 (typical quarterly cadence)",
				IsImminent:    isImminent,
			})
		}
	}

	// Predict half-year results (around February for June FYE)
	// H1 ends Dec 31, results due ~60 days later
	if lastHalfYear.IsZero() || lastHalfYear.Year() < now.Year() {
		halfYearDate := time.Date(now.Year(), time.February, 15, 0, 0, 0, 0, time.UTC)
		if halfYearDate.After(now) {
			isImminent := halfYearDate.Before(twoMonthsFromNow)
			predictions = append(predictions, PredictedReport{
				ReportType:    "Half-Year Results (Appendix 4D)",
				PredictedDate: formatPredictedDate(halfYearDate),
				Basis:         "ASX requires ~60 days after H1 end (Dec 31)",
				IsImminent:    isImminent,
			})
		}
	}

	// Predict Q3 (~April)
	q3Date := time.Date(now.Year(), time.April, 17, 0, 0, 0, 0, time.UTC)
	if q3Date.After(now) {
		isImminent := q3Date.Before(twoMonthsFromNow)
		predictions = append(predictions, PredictedReport{
			ReportType:    fmt.Sprintf("Q3 FY%02d Activities Report", currentFY%100),
			PredictedDate: formatPredictedDate(q3Date),
			Basis:         "Historical: April reporting pattern",
			IsImminent:    isImminent,
		})
	}

	// Predict full-year results (~August)
	fyDate := time.Date(now.Year(), time.August, 19, 0, 0, 0, 0, time.UTC)
	if fyDate.After(now) {
		predictions = append(predictions, PredictedReport{
			ReportType:    fmt.Sprintf("FY%02d Full-Year Results (Appendix 4E)", (currentFY-1)%100),
			PredictedDate: formatPredictedDate(fyDate),
			Basis:         "Historical: August reporting pattern",
			IsImminent:    fyDate.Before(twoMonthsFromNow),
		})
	}

	// Predict AGM (~October)
	agmDate := time.Date(now.Year(), time.October, 23, 0, 0, 0, 0, time.UTC)
	if agmDate.After(now) {
		predictions = append(predictions, PredictedReport{
			ReportType:    fmt.Sprintf("%d AGM", now.Year()),
			PredictedDate: formatPredictedDate(agmDate),
			Basis:         "Historical: October AGM pattern",
			IsImminent:    agmDate.Before(twoMonthsFromNow),
		})
	}

	return predictions
}

// formatPredictedDate formats a date as "Mid-Month Year" style
func formatPredictedDate(t time.Time) string {
	day := t.Day()
	prefix := "Early"
	if day >= 10 && day <= 20 {
		prefix = "Mid"
	} else if day > 20 {
		prefix = "Late"
	}
	return fmt.Sprintf("%s-%s %d", prefix, t.Month().String(), t.Year())
}

// createMQSSummaryDocument creates a summary document using the new MQS framework
func (w *MarketAnnouncementsWorker) createMQSSummaryDocument(ctx context.Context, announcements []ASXAnnouncement, prices []OHLCV, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string, debug *WorkerDebugInfo) *models.Document {
	startTime := time.Now()

	// Create MQS analyzer
	analyzer := NewMQSAnalyzer(announcements, prices, asxCode, "ASX")

	// Fetch fundamentals data via provider (with on-demand generation if needed)
	fundamentals := w.getFinancialsFromFundamentalsDocument(ctx, asxCode)
	if fundamentals != nil {
		analyzer.SetFundamentals(fundamentals)
		w.logger.Debug().
			Str("asx_code", asxCode).
			Int("annual_periods", len(fundamentals.AnnualData)).
			Int("quarterly_periods", len(fundamentals.QuarterlyData)).
			Msg("Loaded EODHD fundamentals for MQS analysis")
	}

	// Fetch EODHD news for matching with high-impact announcements
	newsItems, err := w.fetchEODHDNews(ctx, asxCode)
	if err != nil {
		w.logger.Warn().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch EODHD news")
	} else if len(newsItems) > 0 {
		analyzer.SetNews(newsItems)
		w.logger.Debug().
			Str("asx_code", asxCode).
			Int("news_count", len(newsItems)).
			Msg("Loaded EODHD news for high-impact announcement matching")
	}

	// Run analysis (will enrich financial results with EODHD data if available)
	mqsOutput := analyzer.Analyze()

	// Download and store PDFs for high-impact announcements
	if len(mqsOutput.HighImpactAnnouncements) > 0 {
		mqsOutput.HighImpactAnnouncements = w.downloadAndStoreHighImpactPDFs(ctx, mqsOutput.HighImpactAnnouncements, asxCode)
	}

	// Generate markdown content
	markdownContent := mqsOutput.GenerateMarkdown()

	// Add worker debug info
	debug.Complete()
	markdownContent += "\n\n---\n\n"
	markdownContent += debug.ToMarkdown()

	// Update data quality with processing time
	mqsOutput.DataQuality.ProcessingDurationMs = time.Since(startTime).Milliseconds()

	// Create document
	now := time.Now()
	docID := fmt.Sprintf("mqs-%s-%s", strings.ToLower(asxCode), now.Format("20060102-150405"))

	// Build tags - same pattern as legacy summary document for test compatibility
	tags := []string{"asx-announcement-summary", strings.ToLower(asxCode)}
	tags = append(tags, fmt.Sprintf("date:%s", now.Format("2006-01-02")))
	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}
	tags = append(tags, outputTags...)
	// Apply cache tags from context
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = append(tags, cacheTags...)
	}

	doc := &models.Document{
		ID:              docID,
		Title:           fmt.Sprintf("MQS Analysis: ASX:%s", strings.ToUpper(asxCode)),
		ContentMarkdown: markdownContent,
		SourceType:      "market_announcements_mqs",
		SourceID:        fmt.Sprintf("asx:%s:mqs", strings.ToLower(asxCode)),
		Tags:            tags,
		CreatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"ticker":           mqsOutput.Ticker,
			"exchange":         mqsOutput.Exchange,
			"analysis_date":    mqsOutput.AnalysisDate.Format(time.RFC3339),
			"period_start":     mqsOutput.PeriodStart.Format("2006-01-02"),
			"period_end":       mqsOutput.PeriodEnd.Format("2006-01-02"),
			"mqs_tier":         string(mqsOutput.ManagementQualityScore.Tier),
			"mqs_composite":    mqsOutput.ManagementQualityScore.CompositeScore,
			"mqs_confidence":   string(mqsOutput.ManagementQualityScore.Confidence),
			"leakage_score":    mqsOutput.ManagementQualityScore.LeakageScore,
			"conviction_score": mqsOutput.ManagementQualityScore.ConvictionScore,
			"retention_score":  mqsOutput.ManagementQualityScore.RetentionScore,
			"saydo_score":      mqsOutput.ManagementQualityScore.SayDoScore,
			"announcements":    len(announcements),
			"trading_days":     len(prices),
			// Leakage summary
			"leakage_high_count":  mqsOutput.LeakageSummary.HighLeakageCount,
			"leakage_tight_count": mqsOutput.LeakageSummary.TightShipCount,
			"leakage_ratio":       mqsOutput.LeakageSummary.LeakageRatio,
			// Conviction summary
			"conviction_institutional_count": mqsOutput.ConvictionSummary.InstitutionalCount,
			"conviction_retail_hype_count":   mqsOutput.ConvictionSummary.RetailHypeCount,
			// Retention summary
			"retention_absorbed_count": mqsOutput.RetentionSummary.AbsorbedCount,
			"retention_sold_count":     mqsOutput.RetentionSummary.SoldNewsCount,
			"retention_rate":           mqsOutput.RetentionSummary.RetentionRate,
		},
	}

	// Add high-impact announcements to metadata (past 12 months with significant market reaction)
	if len(mqsOutput.HighImpactAnnouncements) > 0 {
		highImpactList := make([]map[string]interface{}, 0, len(mqsOutput.HighImpactAnnouncements))
		for _, ann := range mqsOutput.HighImpactAnnouncements {
			highImpactList = append(highImpactList, map[string]interface{}{
				"date":             ann.Date,
				"headline":         ann.Headline,
				"type":             ann.Type,
				"price_sensitive":  ann.PriceSensitive,
				"price_change_pct": ann.PriceChangePct,
				"volume_ratio":     ann.VolumeRatio,
				"day10_change_pct": ann.Day10ChangePct,
				"retention_ratio":  ann.RetentionRatio,
				"impact_rating":    ann.ImpactRating,
				"pdf_url":          ann.PDFURL,
				"document_key":     ann.DocumentKey,
				"news_link":        ann.NewsLink,
				"news_title":       ann.NewsTitle,
				"news_source":      ann.NewsSource,
				"sentiment":        ann.Sentiment,
				"pdf_storage_key":  ann.PDFStorageKey,
				"pdf_downloaded":   ann.PDFDownloaded,
				"pdf_size_bytes":   ann.PDFSizeBytes,
			})
		}
		doc.Metadata["high_impact_announcements"] = highImpactList
	}

	// Add job reference if available
	if jobDef != nil {
		doc.Metadata["job_id"] = jobDef.ID
		doc.Metadata["job_name"] = jobDef.Name
	}
	if parentJobID != "" {
		doc.Metadata["parent_job_id"] = parentJobID
	}

	return doc
}

// createSummaryDocument creates a summary document with all analyzed announcements (LEGACY)
func (w *MarketAnnouncementsWorker) createSummaryDocument(ctx context.Context, analyses []AnnouncementAnalysis, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string, debug *WorkerDebugInfo, dedupStats DeduplicationStats) *models.Document {
	var content strings.Builder

	// Pre-calculate signal counts for AI summary and later sections
	highSignalCount, modSignalCount, lowSignalCount, noiseSignalCount, routineCount := 0, 0, 0, 0, 0
	tradingHaltCount := 0
	anomalyNoReactionCount, anomalyUnexpectedCount := 0, 0
	preDriftCount := 0
	priceSensitiveTotal, priceSensitiveWithReaction := 0, 0
	var highSignalAnnouncements []AnnouncementAnalysis

	for _, a := range analyses {
		switch a.SignalNoiseRating {
		case SignalNoiseHigh:
			highSignalCount++
			highSignalAnnouncements = append(highSignalAnnouncements, a)
		case SignalNoiseModerate:
			modSignalCount++
		case SignalNoiseLow:
			lowSignalCount++
		case SignalNoiseNone:
			noiseSignalCount++
		case SignalNoiseRoutine:
			routineCount++
		}
		if a.IsTradingHalt || a.IsReinstatement {
			tradingHaltCount++
		}
		if a.IsAnomaly {
			if a.AnomalyType == "NO_REACTION" {
				anomalyNoReactionCount++
			} else if a.AnomalyType == "UNEXPECTED_REACTION" {
				anomalyUnexpectedCount++
			}
		}
		// Only count high-signal announcements for pre-drift analysis
		// Routine filings (Appendix 3X, 3Y, etc.) are excluded as they don't drive price movement
		if a.PriceImpact != nil && a.PriceImpact.HasSignificantPreDrift &&
			a.SignalNoiseRating == SignalNoiseHigh && !a.IsRoutine {
			preDriftCount++
		}
		if a.PriceSensitive {
			priceSensitiveTotal++
			if a.SignalNoiseRating == SignalNoiseHigh || a.SignalNoiseRating == SignalNoiseModerate {
				priceSensitiveWithReaction++
			}
		}
	}

	// Calculate signal summary early for AI prompt usage
	var signalSummary SignalSummary
	if len(analyses) > 0 {
		signalSummary = buildSignalSummaryFromAnalyses(analyses)
	} else {
		// Default empty summary
		signalSummary = SignalSummary{ConvictionScore: 5, CommunicationStyle: "STANDARD"}
	}

	// Generate AI executive summary if provider is available
	var aiSummary string
	if w.providerFactory != nil && len(analyses) > 0 {
		summaryData := AnnouncementSummaryData{
			ASXCode:                    asxCode,
			HighSignalCount:            highSignalCount,
			ModerateSignalCount:        modSignalCount,
			LowSignalCount:             lowSignalCount,
			NoiseCount:                 noiseSignalCount,
			RoutineCount:               routineCount,
			TradingHaltCount:           tradingHaltCount,
			AnomalyNoReactionCount:     anomalyNoReactionCount,
			AnomalyUnexpectedCount:     anomalyUnexpectedCount,
			PreDriftCount:              preDriftCount,
			PriceSensitiveTotal:        priceSensitiveTotal,
			PriceSensitiveWithReaction: priceSensitiveWithReaction,
			HighSignalAnnouncements:    highSignalAnnouncements,
			// New metrics for prompt context
			ConvictionScore:    signalSummary.ConvictionScore,
			LeakScore:          signalSummary.LeakScore,
			CommunicationStyle: signalSummary.CommunicationStyle,
		}
		var err error
		aiSummary, err = w.generateAISummary(ctx, summaryData)
		if err != nil {
			w.logger.Warn().Err(err).Str("asx_code", asxCode).Msg("Failed to generate AI summary")
		}
	}

	// Header
	content.WriteString(fmt.Sprintf("# ASX Announcements Summary: %s\n\n", asxCode))
	content.WriteString(fmt.Sprintf("**Generated**: %s\n", time.Now().Format("2 January 2006 3:04 PM AEST")))
	content.WriteString(fmt.Sprintf("**Total Announcements**: %d", len(analyses)))
	if dedupStats.DuplicatesFound > 0 {
		content.WriteString(fmt.Sprintf(" (deduplicated from %d)", dedupStats.TotalBefore))
	}
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("**Worker**: %s\n\n", models.WorkerTypeMarketAnnouncements))

	// Executive Summary (AI-generated)
	if aiSummary != "" {
		content.WriteString("## Executive Summary\n\n")
		content.WriteString(aiSummary)
		content.WriteString("\n\n")
	}

	// Signal Analysis & Conviction Rating (REQ-5)
	// Replace old impact rating with conviction-based rating
	if len(analyses) > 0 {
		// signalSummary is already calculated above
		flags := DeriveRiskFlags(signalSummary)

		content.WriteString("## Signal Analysis & Conviction Rating\n\n")

		// Conviction score with rating
		convictionRating := "LOW CONVICTION"
		convictionColor := "#f8d7da" // Red
		convictionEmoji := "⚠️"
		if signalSummary.ConvictionScore >= 8 {
			convictionRating = "HIGH CONVICTION"
			convictionColor = "#d4edda" // Green
			convictionEmoji = "✅"
		} else if signalSummary.ConvictionScore >= 5 {
			convictionRating = "MODERATE CONVICTION"
			convictionColor = "#fff3cd" // Yellow
			convictionEmoji = "📊"
		}

		content.WriteString(fmt.Sprintf("**Conviction Score**: %d/10 (%s)\n\n", signalSummary.ConvictionScore, convictionRating))
		content.WriteString(fmt.Sprintf("<table><tr><td style=\"background-color: %s; padding: 8px 16px; border-radius: 6px; font-weight: bold; font-size: 1.1em;\">%s %s</td></tr></table>\n\n", convictionColor, convictionEmoji, convictionRating))

		// Communication Style
		styleDisplay := signalSummary.CommunicationStyle
		switch styleDisplay {
		case StyleTransparent:
			content.WriteString("**Communication Style**: 📈 TRANSPARENT & DATA-DRIVEN\n\n")
		case StyleLeaky:
			content.WriteString("**Communication Style**: ⚠️ HIGH PRE-ANNOUNCEMENT DRIFT\n\n")
		case StylePromotional:
			content.WriteString("**Communication Style**: 📣 PROMOTIONAL / SENTIMENT-DRIVEN\n\n")
		default:
			content.WriteString("**Communication Style**: 📋 STANDARD\n\n")
		}

		// Risk Flags
		if flags.HighLeakRisk || flags.SpeculativeBase || flags.InsufficientData {
			content.WriteString("### Risk Flags\n\n")
			if flags.HighLeakRisk {
				content.WriteString(fmt.Sprintf("- ⚠️ **High Pre-Drift**: Drift score %.1f%% - %d PRICED_IN announcements show significant pre-announcement price movement\n",
					signalSummary.LeakScore*100, signalSummary.CountPricedIn))
			}
			if flags.SpeculativeBase {
				content.WriteString(fmt.Sprintf("- ⚠️ **Speculative Base**: Noise ratio %.1f%% - %d SENTIMENT_NOISE events suggest retail speculation\n",
					signalSummary.NoiseRatio*100, signalSummary.CountSentimentNoise))
			}
			if flags.InsufficientData {
				content.WriteString(fmt.Sprintf("- ℹ️ **Insufficient Data**: Only %d announcements analyzed - results may be unreliable\n",
					signalSummary.TotalAnnouncements))
			}
			if flags.ReliableSignals {
				content.WriteString(fmt.Sprintf("- ✅ **Reliable Signals**: Signal ratio %.1f%% with high credibility (%.0f%%)\n",
					signalSummary.SignalRatio*100, signalSummary.CredibilityScore*100))
			}
			content.WriteString("\n")
		}

		// Signal Breakdown Table
		content.WriteString("### Signal Breakdown\n\n")
		content.WriteString("| Classification | Count | Implication |\n")
		content.WriteString("|----------------|-------|-------------|\n")
		content.WriteString(fmt.Sprintf("| TRUE_SIGNAL | %d | Genuine market surprises - new information |\n", signalSummary.CountTrueSignal))
		content.WriteString(fmt.Sprintf("| PRICED_IN | %d | Information leaked or anticipated before announcement |\n", signalSummary.CountPricedIn))
		content.WriteString(fmt.Sprintf("| SENTIMENT_NOISE | %d | Retail speculation on routine news |\n", signalSummary.CountSentimentNoise))
		content.WriteString(fmt.Sprintf("| MANAGEMENT_BLUFF | %d | Claimed materiality without market impact |\n", signalSummary.CountManagementBluff))
		content.WriteString(fmt.Sprintf("| ROUTINE | %d | Administrative filings (excluded from analysis) |\n", signalSummary.CountRoutine))
		content.WriteString("\n")

		// Justification
		content.WriteString("**Metrics Summary**:\n")
		content.WriteString(fmt.Sprintf("- Signal Ratio: %.1f%% (TRUE_SIGNAL / total)\n", signalSummary.SignalRatio*100))
		content.WriteString(fmt.Sprintf("- Leak Score: %.1f%% (PRICED_IN / price-sensitive)\n", signalSummary.LeakScore*100))
		content.WriteString(fmt.Sprintf("- Credibility: %.0f%% (1 - MANAGEMENT_BLUFF rate)\n", signalSummary.CredibilityScore*100))
		content.WriteString(fmt.Sprintf("- Noise Ratio: %.1f%% (SENTIMENT_NOISE / total)\n", signalSummary.NoiseRatio*100))
		content.WriteString("\n")
	}

	// Mandatory Business Update Calendar (REQ-2)
	// Extract historical reporting dates and predict upcoming mandatory disclosures
	if len(analyses) > 0 {
		content.WriteString("## Mandatory Business Update Calendar\n\n")
		content.WriteString(fmt.Sprintf("%s operates on a **June 30 fiscal year end**. The following tables show historical reporting dates and predicted upcoming mandatory disclosures.\n\n", asxCode))

		// Historical Reporting Dates
		historicalReports := extractHistoricalReports(analyses)
		if len(historicalReports) > 0 {
			content.WriteString("### Historical Reporting Dates\n\n")
			content.WriteString("| Report Type | Date | Reference |\n")
			content.WriteString("|-------------|------|----------|\n")
			for _, r := range historicalReports {
				content.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
					r.ReportType,
					r.Date.Format("2 Jan 2006"),
					r.Reference,
				))
			}
			content.WriteString("\n")
		}

		// Predicted Upcoming Reports
		predictions := predictUpcomingReports(historicalReports, asxCode)
		if len(predictions) > 0 {
			content.WriteString("### Predicted Upcoming Mandatory Reports\n\n")
			content.WriteString("| Report Type | Predicted Date | Basis |\n")
			content.WriteString("|-------------|----------------|-------|\n")
			for _, p := range predictions {
				reportName := p.ReportType
				if p.IsImminent {
					// Highlight imminent reports with green background
					reportName = fmt.Sprintf("<span style=\"background-color: #d4edda; padding: 2px 6px; border-radius: 3px;\">%s</span>", p.ReportType)
				}
				content.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
					reportName,
					p.PredictedDate,
					p.Basis,
				))
			}
			content.WriteString("\n")
		}

		content.WriteString("*Predictions based on ASX Listing Rules and historical reporting patterns. Actual dates may vary.*\n\n")
	}

	// Deduplication Summary (if any duplicates were found)
	// Simplified to just strict counts as requested - no table
	if dedupStats.DuplicatesFound > 0 {
		content.WriteString("## Deduplication Summary\n\n")
		content.WriteString(fmt.Sprintf("- **Original Announcements**: %d\n", dedupStats.TotalBefore))
		content.WriteString(fmt.Sprintf("- **After Deduplication**: %d\n", dedupStats.TotalAfter))
		content.WriteString(fmt.Sprintf("- **Duplicates Consolidated**: %d\n\n", dedupStats.DuplicatesFound))
	}

	// Count dividend announcements (not included in AI summary data)
	dividendCount := 0
	for _, a := range analyses {
		if a.IsDividendAnnouncement {
			dividendCount++
		}
	}

	// Signal-to-Noise Analysis Summary (counts already calculated at top)
	content.WriteString("## Signal-to-Noise Analysis\n\n")
	content.WriteString("**Rating Definitions:**\n")
	content.WriteString("- **HIGH_SIGNAL**: Significant market impact (price change >=3% OR volume >=2x)\n")
	content.WriteString("- **MODERATE_SIGNAL**: Notable market reaction (price change >=1.5% OR volume >=1.5x)\n")
	content.WriteString("- **LOW_SIGNAL**: Minor market reaction (price change >=0.5% OR volume >=1.2x)\n")
	content.WriteString("- **NOISE**: No meaningful impact (price change <0.5% AND volume <1.2x)\n")
	content.WriteString("- **ROUTINE**: Administrative filing excluded from signal analysis\n\n")

	content.WriteString("| Signal Rating | Count | Interpretation |\n")
	content.WriteString("|---------------|-------|----------------|\n")
	content.WriteString(fmt.Sprintf("| **HIGH_SIGNAL** | %d | Announcements with significant market reaction |\n", highSignalCount))
	content.WriteString(fmt.Sprintf("| MODERATE_SIGNAL | %d | Announcements with notable price/volume movement |\n", modSignalCount))
	content.WriteString(fmt.Sprintf("| LOW_SIGNAL | %d | Announcements with minor detectable impact |\n", lowSignalCount))
	content.WriteString(fmt.Sprintf("| NOISE | %d | Announcements with no meaningful market effect |\n", noiseSignalCount))
	if routineCount > 0 {
		content.WriteString(fmt.Sprintf("| *ROUTINE* | %d | Administrative filings (excluded from ratio) |\n", routineCount))
	}
	if tradingHaltCount > 0 {
		content.WriteString(fmt.Sprintf("| *Trading Halts* | %d | Trading halt/reinstatement announcements |\n", tradingHaltCount))
	}
	content.WriteString("\n")

	// Calculate signal-to-noise ratio (EXCLUDING routine announcements)
	signalCount := highSignalCount + modSignalCount
	noiseRatioDesc := "Very High"
	if signalCount > 0 {
		// Only count actual noise, not routine administrative filings
		actualNoise := noiseSignalCount + lowSignalCount
		if actualNoise <= signalCount {
			noiseRatioDesc = "Low"
		} else if actualNoise <= signalCount*2 {
			noiseRatioDesc = "Moderate"
		} else {
			noiseRatioDesc = "High"
		}
	}
	content.WriteString(fmt.Sprintf("**Noise Ratio**: %s (%d signal vs %d noise announcements", noiseRatioDesc, signalCount, noiseSignalCount+lowSignalCount))
	if routineCount > 0 {
		content.WriteString(fmt.Sprintf(", %d routine excluded", routineCount))
	}
	content.WriteString(")\n\n")

	// Price-Sensitive Accuracy Scoring
	if priceSensitiveTotal > 0 {
		accuracy := float64(priceSensitiveWithReaction) / float64(priceSensitiveTotal) * 100
		content.WriteString("## Price-Sensitive Accuracy\n\n")
		content.WriteString("How often did price-sensitive announcements actually move the market?\n\n")
		content.WriteString(fmt.Sprintf("- **Total Price-Sensitive Announcements**: %d\n", priceSensitiveTotal))
		content.WriteString(fmt.Sprintf("- **With Market Reaction (High/Moderate Signal)**: %d\n", priceSensitiveWithReaction))
		content.WriteString(fmt.Sprintf("- **Accuracy Score**: %.1f%%\n\n", accuracy))
	}

	// Anomaly Detection Section
	if anomalyNoReactionCount > 0 || anomalyUnexpectedCount > 0 {
		content.WriteString("## Anomaly Detection\n\n")
		content.WriteString("Announcements where market reaction didn't match expectations:\n\n")
		if anomalyNoReactionCount > 0 {
			content.WriteString(fmt.Sprintf("- **⚠️ No Reaction Anomalies**: %d (price-sensitive with no market reaction)\n", anomalyNoReactionCount))
		}
		if anomalyUnexpectedCount > 0 {
			content.WriteString(fmt.Sprintf("- **📈 Unexpected Reaction Anomalies**: %d (non-price-sensitive with high reaction)\n", anomalyUnexpectedCount))
		}
		content.WriteString("\n")
	}

	// Dividend Announcements Section
	if dividendCount > 0 {
		content.WriteString("## Dividend Announcements\n\n")
		content.WriteString(fmt.Sprintf("**Total Dividend-Related Announcements**: %d\n\n", dividendCount))
		content.WriteString("*Note: Negative price movement on dividend announcements may be due to ex-dividend adjustment rather than negative market sentiment.*\n\n")
	}

	// Routine Administrative Announcements - simplified list format
	if routineCount > 0 {
		content.WriteString("## Routine Administrative Announcements\n\n")
		content.WriteString(fmt.Sprintf("*%d routine filings excluded from signal analysis:*\n\n", routineCount))

		// Group by routine type for cleaner output
		routineByType := make(map[string]int)
		for _, a := range analyses {
			if a.IsRoutine {
				routineByType[a.RoutineType]++
			}
		}
		for routineType, count := range routineByType {
			content.WriteString(fmt.Sprintf("- **%s**: %d\n", routineType, count))
		}
		content.WriteString("\n")
	}

	// Consolidated Announcements Analysis Table (High & Moderate signals with full detail)
	// This is the primary announcements table - ordered by date, last information section before debug
	if highSignalCount > 0 || modSignalCount > 0 {
		content.WriteString("## Announcements Analysis\n\n")
		content.WriteString("*High and Moderate signal announcements with price/volume impact and pre-announcement movement analysis.*\n\n")

		// Collect and sort by date (newest first)
		var signalAnnouncements []AnnouncementAnalysis
		for _, a := range analyses {
			if a.SignalNoiseRating == SignalNoiseHigh || a.SignalNoiseRating == SignalNoiseModerate {
				signalAnnouncements = append(signalAnnouncements, a)
			}
		}
		sort.Slice(signalAnnouncements, func(i, j int) bool {
			return signalAnnouncements[i].Date.After(signalAnnouncements[j].Date)
		})

		// Table header - Pre-Drift column shows percentage with direction
		content.WriteString("| Announcement | Price Impact | Volume | Pre-Drift | Document |\n")
		content.WriteString("|--------------|--------------|--------|-----------|----------|\n")

		for _, a := range signalAnnouncements {
			// Column 1: Consolidated Announcement details with line breaks
			signalEmoji := "📊"
			if a.SignalNoiseRating == SignalNoiseHigh {
				signalEmoji = "🔥"
			}
			priceSensitiveStr := "No"
			if a.PriceSensitive {
				priceSensitiveStr = "Yes ⚠️"
			}
			announcementCell := fmt.Sprintf("**%s**<br>Type: %s<br>Signal: %s %s<br>Price Sensitive: %s",
				a.Headline, a.Type, signalEmoji, string(a.SignalNoiseRating), priceSensitiveStr)

			// Column 2: Price Impact
			priceCell := "-"
			if a.PriceImpact != nil {
				sign := ""
				if a.PriceImpact.ChangePercent > 0 {
					sign = "+"
				}
				priceCell = fmt.Sprintf("$%.3f → $%.3f<br>(%s%.1f%%)",
					a.PriceImpact.PriceBefore, a.PriceImpact.PriceAfter, sign, a.PriceImpact.ChangePercent)
			}

			// Column 3: Volume
			volumeCell := "-"
			if a.PriceImpact != nil {
				volumeCell = fmt.Sprintf("%.1fx", a.PriceImpact.VolumeChangeRatio)
			}

			// Column 4: Pre-Drift (uses PreDriftInterpretation which is now shortened)
			preDriftCell := "-"
			if a.PriceImpact != nil && a.PriceImpact.HasSignificantPreDrift && a.PriceImpact.PreDriftInterpretation != "" {
				preDriftCell = a.PriceImpact.PreDriftInterpretation
			}

			// Column 5: Document link
			docCell := "-"
			if a.PDFURL != "" {
				docCell = fmt.Sprintf("[PDF](%s)", a.PDFURL)
			}

			content.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				announcementCell, priceCell, volumeCell, preDriftCell, docCell))
		}
		content.WriteString("\n")
	}

	// Calculate legacy relevance counts for metadata (not displayed in markdown)
	highCount, mediumCount, lowCount, noiseCount := 0, 0, 0, 0
	for _, a := range analyses {
		switch a.RelevanceCategory {
		case "HIGH":
			highCount++
		case "MEDIUM":
			mediumCount++
		case "LOW":
			lowCount++
		case "NOISE":
			noiseCount++
		}
	}

	// Build tags
	tags := []string{"asx-announcement-summary", strings.ToLower(asxCode)}
	tags = append(tags, fmt.Sprintf("date:%s", time.Now().Format("2006-01-02")))

	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}
	tags = append(tags, outputTags...)

	// Apply cache tags from context
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	// Build metadata with structured announcements array
	announcementsMetadata := make([]map[string]interface{}, 0, len(analyses))
	for _, a := range analyses {
		annMeta := map[string]interface{}{
			"date":                     a.Date.Format(time.RFC3339),
			"headline":                 a.Headline,
			"type":                     a.Type,
			"price_sensitive":          a.PriceSensitive,
			"relevance_category":       a.RelevanceCategory,
			"relevance_reason":         a.RelevanceReason,
			"document_key":             a.DocumentKey,
			"pdf_url":                  a.PDFURL,
			"signal_noise_rating":      string(a.SignalNoiseRating),
			"signal_noise_rationale":   a.SignalNoiseRationale,
			"signal_classification":    a.SignalClassification, // REQ-7: New classification
			"is_trading_halt":          a.IsTradingHalt,
			"is_reinstatement":         a.IsReinstatement,
			"is_anomaly":               a.IsAnomaly,
			"anomaly_type":             a.AnomalyType,
			"is_dividend_announcement": a.IsDividendAnnouncement,
			"is_routine":               a.IsRoutine,
			"routine_type":             a.RoutineType,
		}

		if a.PriceImpact != nil {
			annMeta["price_impact"] = map[string]interface{}{
				"price_before":              a.PriceImpact.PriceBefore,
				"price_after":               a.PriceImpact.PriceAfter,
				"change_percent":            a.PriceImpact.ChangePercent,
				"volume_before":             a.PriceImpact.VolumeBefore,
				"volume_after":              a.PriceImpact.VolumeAfter,
				"volume_change_ratio":       a.PriceImpact.VolumeChangeRatio,
				"impact_signal":             a.PriceImpact.ImpactSignal,
				"pre_announcement_drift":    a.PriceImpact.PreAnnouncementDrift,
				"has_significant_pre_drift": a.PriceImpact.HasSignificantPreDrift,
				"pre_drift_interpretation":  a.PriceImpact.PreDriftInterpretation,
			}
		}

		announcementsMetadata = append(announcementsMetadata, annMeta)
	}

	// Build signal analysis summary for metadata (REQ-7)
	// signalSummary calculated early for prompt
	riskFlags := DeriveRiskFlags(signalSummary)

	metadata := map[string]interface{}{
		"asx_code":      asxCode,
		"total_count":   len(analyses),
		"parent_job_id": parentJobID,
		"announcements": announcementsMetadata,
		"generated_at":  time.Now().Format(time.RFC3339),
		// Deduplication stats
		"deduplication_stats": map[string]interface{}{
			"total_before":     dedupStats.TotalBefore,
			"total_after":      dedupStats.TotalAfter,
			"duplicates_found": dedupStats.DuplicatesFound,
		},
		// Signal analysis (REQ-7) - conviction-based metrics
		"conviction_score":    signalSummary.ConvictionScore,
		"communication_style": signalSummary.CommunicationStyle,
		"risk_flags": map[string]interface{}{
			"high_leak_risk":    riskFlags.HighLeakRisk,
			"speculative_base":  riskFlags.SpeculativeBase,
			"reliable_signals":  riskFlags.ReliableSignals,
			"insufficient_data": riskFlags.InsufficientData,
		},
		"signal_analysis": map[string]interface{}{
			"total_analyzed":         signalSummary.TotalAnnouncements,
			"true_signal_count":      signalSummary.CountTrueSignal,
			"priced_in_count":        signalSummary.CountPricedIn,
			"sentiment_noise_count":  signalSummary.CountSentimentNoise,
			"management_bluff_count": signalSummary.CountManagementBluff,
			"routine_count":          signalSummary.CountRoutine,
			"leak_score":             signalSummary.LeakScore,
			"noise_ratio":            signalSummary.NoiseRatio,
			"credibility_score":      signalSummary.CredibilityScore,
			"signal_ratio":           signalSummary.SignalRatio,
		},
		// Legacy signal-to-noise summary (kept for backward compatibility)
		"signal_noise_summary": map[string]interface{}{
			"high_signal_count":     highSignalCount,
			"moderate_signal_count": modSignalCount,
			"low_signal_count":      lowSignalCount,
			"noise_count":           noiseSignalCount,
			"routine_count":         routineCount,
			"trading_halt_count":    tradingHaltCount,
		},
		// AI-generated executive summary
		"ai_summary": aiSummary,
		// Keyword-based classification summary (legacy)
		"relevance_summary": map[string]interface{}{
			"high_count":   highCount,
			"medium_count": mediumCount,
			"low_count":    lowCount,
			"noise_count":  noiseCount,
		},
	}

	// Add worker debug metadata if enabled
	if debug != nil {
		debug.Complete()
		if debugMeta := debug.ToMetadata(); debugMeta != nil {
			metadata["worker_debug"] = debugMeta
		}
		// Append debug markdown to output
		if debugMd := debug.ToMarkdown(); debugMd != "" {
			content.WriteString(debugMd)
		}
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "asx_announcement_summary",
		SourceID:        fmt.Sprintf("asx:%s:announcement_summary", asxCode),
		URL:             fmt.Sprintf("https://www.asx.com.au/asx/statistics/announcements.do?by=asxCode&asxCode=%s", asxCode),
		Title:           fmt.Sprintf("ASX:%s Announcements Summary", asxCode),
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

// =============================================================================
// Financial Metrics Extraction via LLM
// =============================================================================

// extractFinancialMetrics fetches PDF content for financial result announcements
// and uses LLM to extract structured business metrics
// NOTE: Currently disabled - ASX PDFs require license agreement acceptance and
// are embedded in iframes, making automated extraction complex. The Financial
// Results History table still provides valuable market reaction data.
func (w *MarketAnnouncementsWorker) extractFinancialMetrics(ctx context.Context, results []FinancialResult, maxResults int) {
	// PDF extraction is currently disabled due to ASX website complexity:
	// 1. ASX requires license agreement acceptance before showing PDFs
	// 2. PDFs are embedded in iframes, not directly accessible
	// 3. Would require chromedp automation + PDF text extraction library
	//
	// The Financial Results History table still provides valuable data:
	// - Market reaction (Beat/Miss/Met based on price movement)
	// - Day-of and Day+10 price changes
	// - Volume ratios
	// - YoY trend indicators
	//
	// For detailed financial metrics (Revenue, Profit, EBITDA), users should
	// review the PDF announcements directly via the provided links.
	w.logger.Debug().
		Int("results_count", len(results)).
		Msg("Financial metrics extraction disabled - ASX PDFs require manual review")
}

// NOTE: PDF extraction functions (fetchAnnouncementContent, extractMetricsWithLLM,
// parseFinancialMetricsJSON) have been removed. ASX PDFs require license agreement
// acceptance and are embedded in iframes, making automated extraction complex.
// The Financial Results History table still provides valuable market reaction data.
// For detailed financial metrics, users should review PDF announcements directly.

// fetchEODHDNews fetches news articles from EODHD API for the given ticker.
// Returns news items from the past 12 months for matching with high-impact announcements.
func (w *MarketAnnouncementsWorker) fetchEODHDNews(ctx context.Context, asxCode string) ([]EODHDNewsItem, error) {
	if w.kvStorage == nil {
		w.logger.Debug().Msg("kvStorage is nil, skipping EODHD news fetch")
		return nil, nil
	}

	// Get EODHD API key
	apiKey, err := common.ResolveAPIKey(ctx, w.kvStorage, "eodhd_api_key", "")
	if err != nil || apiKey == "" {
		w.logger.Debug().Err(err).Msg("EODHD API key not available, skipping news fetch")
		return nil, nil
	}

	// Create EODHD client
	eodhdClient := eodhd.NewClient(apiKey, eodhd.WithLogger(w.logger))

	// Fetch news for past 12 months
	to := time.Now()
	from := to.AddDate(-1, 0, 0)
	symbol := fmt.Sprintf("%s.AU", strings.ToUpper(asxCode))

	news, err := eodhdClient.GetNews(ctx, []string{symbol},
		eodhd.WithDateRange(from, to),
		eodhd.WithLimit(100),
	)
	if err != nil {
		w.logger.Warn().Err(err).Str("symbol", symbol).Msg("Failed to fetch EODHD news")
		return nil, err
	}

	// Convert to internal type
	result := make([]EODHDNewsItem, 0, len(news))
	for _, item := range news {
		sentiment := "neutral"
		if item.Sentiment != nil {
			if item.Sentiment.Pos > item.Sentiment.Neg && item.Sentiment.Pos > 0.3 {
				sentiment = "positive"
			} else if item.Sentiment.Neg > item.Sentiment.Pos && item.Sentiment.Neg > 0.3 {
				sentiment = "negative"
			}
		}

		result = append(result, EODHDNewsItem{
			Date:      item.Date,
			Title:     item.Title,
			Content:   item.Content,
			Link:      item.Link,
			Symbols:   item.Symbols,
			Tags:      item.Tags,
			Sentiment: sentiment,
		})
	}

	w.logger.Debug().
		Str("asx_code", asxCode).
		Int("news_count", len(result)).
		Msg("Fetched EODHD news for announcement matching")

	return result, nil
}

// matchNewsToAnnouncement attempts to find a matching EODHD news article for an announcement.
// Matches based on date proximity and headline similarity.
func (w *MarketAnnouncementsWorker) matchNewsToAnnouncement(announcement ASXAnnouncement, newsItems []EODHDNewsItem) *EODHDNewsItem {
	if len(newsItems) == 0 {
		return nil
	}

	// Look for news within 2 days of announcement
	for i := range newsItems {
		item := &newsItems[i]
		daysDiff := announcement.Date.Sub(item.Date).Hours() / 24
		if daysDiff < -2 || daysDiff > 2 {
			continue
		}

		// Check for headline similarity (simple substring match)
		headlineLower := strings.ToLower(announcement.Headline)
		titleLower := strings.ToLower(item.Title)

		// Check if key words from announcement appear in news title
		words := strings.Fields(headlineLower)
		matchCount := 0
		for _, word := range words {
			if len(word) > 3 && strings.Contains(titleLower, word) {
				matchCount++
			}
		}

		// If at least 2 significant words match, consider it a match
		if matchCount >= 2 {
			return item
		}

		// Also check if news title contains the announcement headline or vice versa
		if strings.Contains(titleLower, headlineLower) || strings.Contains(headlineLower, titleLower) {
			return item
		}
	}

	return nil
}

// downloadASXPDF downloads a PDF from ASX using a two-step cookie strategy.
// ASX uses a Web Application Firewall (WAF) that requires:
// 1. First visiting the redirect/HTML page to get session cookies
// 2. Then requesting the actual PDF with those cookies
// Returns the PDF content as bytes, or an error if download fails.
func (w *MarketAnnouncementsWorker) downloadASXPDF(ctx context.Context, pdfURL string, documentKey string) ([]byte, error) {
	if pdfURL == "" {
		return nil, fmt.Errorf("empty PDF URL")
	}

	// Create a cookie jar to store session data
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Create a client with the cookie jar
	client := &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}

	// Step 1: Visit the redirect/HTML page first to get session cookies
	// The pdfURL is typically in format: https://www.asx.com.au/asx/v2/statistics/displayAnnouncement.do?display=pdf&idsId=XXXXX
	initReq, err := http.NewRequestWithContext(ctx, "GET", pdfURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create init request: %w", err)
	}

	// Set browser-like headers to bypass WAF
	initReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	initReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	initReq.Header.Set("Accept-Language", "en-US,en;q=0.5")
	initReq.Header.Set("Referer", "https://www.asx.com.au/")

	initResp, err := client.Do(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get session cookies: %w", err)
	}
	initResp.Body.Close()

	w.logger.Debug().
		Str("pdf_url", pdfURL).
		Int("init_status", initResp.StatusCode).
		Msg("ASX PDF: Got session cookies")

	// Step 2: Now request the actual PDF using the same client (with cookies)
	// Construct the direct PDF URL from the document key
	// Format: https://announcements.asx.com.au/asxpdf/YYYYMMDD/pdf/DOCUMENTKEY.pdf
	// We need to extract the date from the original URL or use the document key

	// Try the direct PDF URL first
	directPDFURL := fmt.Sprintf("https://announcements.asx.com.au/asxpdf/%s.pdf", documentKey)

	pdfReq, err := http.NewRequestWithContext(ctx, "GET", directPDFURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF request: %w", err)
	}

	// Set browser-like headers
	pdfReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	pdfReq.Header.Set("Accept", "application/pdf,*/*")
	pdfReq.Header.Set("Accept-Language", "en-US,en;q=0.5")
	pdfReq.Header.Set("Referer", "https://www.asx.com.au/")

	pdfResp, err := client.Do(pdfReq)
	if err != nil {
		return nil, fmt.Errorf("failed to download PDF: %w", err)
	}
	defer pdfResp.Body.Close()

	if pdfResp.StatusCode != http.StatusOK {
		// Try the original URL as fallback
		pdfReq2, _ := http.NewRequestWithContext(ctx, "GET", pdfURL, nil)
		pdfReq2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		pdfReq2.Header.Set("Accept", "application/pdf,*/*")
		pdfReq2.Header.Set("Referer", "https://www.asx.com.au/")

		pdfResp2, err := client.Do(pdfReq2)
		if err != nil {
			return nil, fmt.Errorf("failed to download PDF (fallback): %w", err)
		}
		defer pdfResp2.Body.Close()

		if pdfResp2.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("PDF download failed with status %d", pdfResp2.StatusCode)
		}

		pdfContent, err := io.ReadAll(pdfResp2.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read PDF content: %w", err)
		}

		return pdfContent, nil
	}

	pdfContent, err := io.ReadAll(pdfResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF content: %w", err)
	}

	w.logger.Debug().
		Str("document_key", documentKey).
		Int("pdf_size", len(pdfContent)).
		Msg("ASX PDF: Downloaded successfully")

	return pdfContent, nil
}

// downloadAndStoreHighImpactPDFs downloads PDFs for high-impact announcements and stores them in KV storage.
// Returns the updated announcements with PDF storage keys.
func (w *MarketAnnouncementsWorker) downloadAndStoreHighImpactPDFs(ctx context.Context, announcements []HighImpactAnnouncement, asxCode string) []HighImpactAnnouncement {
	if w.kvStorage == nil {
		w.logger.Debug().Msg("kvStorage is nil, skipping PDF downloads")
		return announcements
	}

	// Limit to first 10 high-impact announcements to avoid excessive downloads
	maxDownloads := 10
	downloadCount := 0

	for i := range announcements {
		if downloadCount >= maxDownloads {
			break
		}

		ann := &announcements[i]
		if ann.PDFURL == "" || ann.DocumentKey == "" {
			continue
		}

		// Create storage key
		storageKey := fmt.Sprintf("pdf:%s:%s", strings.ToLower(asxCode), ann.DocumentKey)

		// Check if already downloaded
		existingPDF, err := w.kvStorage.Get(ctx, storageKey)
		if err == nil && existingPDF != "" {
			ann.PDFStorageKey = storageKey
			ann.PDFDownloaded = true
			w.logger.Debug().
				Str("storage_key", storageKey).
				Msg("PDF already in storage, skipping download")
			continue
		}

		// Download PDF
		pdfContent, err := w.downloadASXPDF(ctx, ann.PDFURL, ann.DocumentKey)
		if err != nil {
			w.logger.Warn().
				Err(err).
				Str("document_key", ann.DocumentKey).
				Str("headline", ann.Headline).
				Msg("Failed to download PDF")
			continue
		}

		// Store as base64 in KV storage
		base64Content := base64.StdEncoding.EncodeToString(pdfContent)
		description := fmt.Sprintf("ASX PDF: %s - %s", ann.Date, ann.Headline)

		if err := w.kvStorage.Set(ctx, storageKey, base64Content, description); err != nil {
			w.logger.Warn().
				Err(err).
				Str("storage_key", storageKey).
				Msg("Failed to store PDF in KV storage")
			continue
		}

		ann.PDFStorageKey = storageKey
		ann.PDFDownloaded = true
		ann.PDFSizeBytes = int64(len(pdfContent))
		downloadCount++

		w.logger.Info().
			Str("storage_key", storageKey).
			Int64("size_bytes", ann.PDFSizeBytes).
			Str("headline", ann.Headline).
			Msg("PDF downloaded and stored")
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Int("downloaded", downloadCount).
		Int("total_high_impact", len(announcements)).
		Msg("PDF download complete")

	return announcements
}
