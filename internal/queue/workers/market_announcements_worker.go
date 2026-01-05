// -----------------------------------------------------------------------
// MarketAnnouncementsWorker - Fetches ASX company announcements
// Uses the Markit Digital API to fetch announcements in JSON format
// Produces individual announcement documents AND a summary document
// with relevance classification and price impact analysis
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
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
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
	debugEnabled    bool
	providerFactory *llm.ProviderFactory
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
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
	providerFactory *llm.ProviderFactory,
) *MarketAnnouncementsWorker {
	return &MarketAnnouncementsWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		debugEnabled:    debugEnabled,
		providerFactory: providerFactory,
	}
}

// GetType returns WorkerTypeMarketAnnouncements for the DefinitionWorker interface
func (w *MarketAnnouncementsWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketAnnouncements
}

// extractASXCode extracts ASX code from step config or job-level variables.
// Priority: step config asx_code > job variables ticker > job variables asx_code
func (w *MarketAnnouncementsWorker) extractASXCode(stepConfig map[string]interface{}, jobDef models.JobDefinition) string {
	// Source 1: Direct step config
	if asxCode, ok := stepConfig["asx_code"].(string); ok && asxCode != "" {
		parsed := common.ParseTicker(asxCode)
		return parsed.Code
	}

	// Source 2: Job-level variables
	if jobDef.Config != nil {
		if vars, ok := jobDef.Config["variables"].([]interface{}); ok {
			for _, v := range vars {
				varMap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}

				// Try "ticker" key (e.g., "ASX:GNP")
				if ticker, ok := varMap["ticker"].(string); ok && ticker != "" {
					parsed := common.ParseTicker(ticker)
					if parsed.Code != "" {
						return parsed.Code
					}
				}

				// Try "asx_code" key
				if asxCode, ok := varMap["asx_code"].(string); ok && asxCode != "" {
					parsed := common.ParseTicker(asxCode)
					if parsed.Code != "" {
						return parsed.Code
					}
				}
			}
		}
	}

	return ""
}

// Init performs the initialization/setup phase for an ASX announcements step.
func (w *MarketAnnouncementsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract ASX code from multiple sources:
	// 1. Direct step config (asx_code)
	// 2. Job-level variables
	asxCode := w.extractASXCode(stepConfig, jobDef)
	if asxCode == "" {
		return nil, fmt.Errorf("asx_code is required in step config or job variables")
	}
	asxCode = strings.ToUpper(asxCode)

	// Extract period (optional, used to filter results by date)
	// Supported: D1, W1, M1, M3, M6, Y1, Y5
	period := "Y1"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	// Extract limit (optional, max announcements to fetch)
	limit := 50 // Default
	if l, ok := stepConfig["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := stepConfig["limit"].(int); ok {
		limit = l
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
		Int("limit", limit).
		Msg("ASX announcements worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   asxCode,
				Name: fmt.Sprintf("Fetch ASX:%s announcements", asxCode),
				Type: "market_announcements",
				Config: map[string]interface{}{
					"asx_code": asxCode,
					"period":   period,
					"limit":    limit,
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"asx_code":    asxCode,
			"period":      period,
			"limit":       limit,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs fetches ASX announcements and stores them as documents.
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
	asxCode, _ := initResult.Metadata["asx_code"].(string)
	period, _ := initResult.Metadata["period"].(string)
	limit, _ := initResult.Metadata["limit"].(int)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Initialize debug tracking
	debug := NewWorkerDebug("market_announcements", w.debugEnabled)
	debug.SetTicker(fmt.Sprintf("ASX:%s", asxCode))

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
		Str("step_id", stepID).
		Msg("Fetching ASX announcements")

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
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch announcements: %v", err))
		}
		return "", fmt.Errorf("failed to fetch ASX announcements: %w", err)
	}

	debug.EndPhase("api_fetch")

	if len(announcements) == 0 {
		w.logger.Warn().Str("asx_code", asxCode).Msg("No announcements found")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", "No announcements found")
		}
		return stepID, nil
	}

	// Extract output_tags from step config
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

	// Log progress for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetched %d ASX:%s announcements, now analyzing...", len(announcements), asxCode))
	}

	// Fetch historical price data for price impact analysis
	debug.StartPhase("api_fetch") // Accumulates with earlier API fetch
	var priceData []OHLCV
	priceData, err = w.fetchHistoricalPrices(ctx, asxCode, period)
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

		doc := w.createDocument(ctx, ann, asxCode, &jobDef, stepID, outputTags)
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

	// Create and save summary document (contains ALL deduplicated announcements)
	summaryDoc := w.createSummaryDocument(ctx, analyses, asxCode, &jobDef, stepID, outputTags, debug, dedupStats)
	if err := w.documentStorage.SaveDocument(summaryDoc); err != nil {
		w.logger.Warn().Err(err).Str("asx_code", asxCode).Msg("Failed to save summary document")
	} else {
		w.logger.Info().
			Str("asx_code", asxCode).
			Int("announcements_in_summary", len(analyses)).
			Msg("Saved announcement summary document")
	}

	// Log completion for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Completed ASX:%s - %d total (%d HIGH, %d MEDIUM saved as docs, %d LOW/NOISE in summary only)",
				asxCode, len(analyses), highCount, mediumCount, lowCount+noiseCount))
	}

	return stepID, nil
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
// First tries to get data from asx_stock_data documents (preferred source).
// Falls back to direct Yahoo Finance API if no document exists.
func (w *MarketAnnouncementsWorker) fetchHistoricalPrices(ctx context.Context, asxCode, period string) ([]OHLCV, error) {
	// First, try to get price data from an existing asx_stock_data document
	// This avoids duplicate Yahoo Finance API calls and ensures consistency
	prices, err := w.getPricesFromStockDataDocument(ctx, asxCode)
	if err == nil && len(prices) > 0 {
		w.logger.Info().
			Str("asx_code", asxCode).
			Int("price_count", len(prices)).
			Msg("Using price data from asx_stock_data document")
		return prices, nil
	}

	// Log that we're falling back to direct Yahoo Finance call
	w.logger.Info().
		Str("asx_code", asxCode).
		Err(err).
		Msg("No asx_stock_data document found, fetching directly from Yahoo Finance")

	// Fallback: Fetch directly from Yahoo Finance
	return w.fetchPricesFromYahoo(ctx, asxCode, period)
}

// getPricesFromStockDataDocument retrieves OHLCV data from an existing stock data document.
// First tries asx_stock_collector (recommended), then falls back to asx_stock_data (deprecated).
// Returns the historical_prices array from document metadata if available.
func (w *MarketAnnouncementsWorker) getPricesFromStockDataDocument(ctx context.Context, asxCode string) ([]OHLCV, error) {
	upperCode := strings.ToUpper(asxCode)

	// Try asx_stock_collector first (preferred source)
	sourceType := "asx_stock_collector"
	sourceID := fmt.Sprintf("asx:%s:stock_collector", upperCode)

	doc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err != nil || doc == nil {
		// Fallback to deprecated asx_stock_data
		sourceType = "asx_stock_data"
		sourceID = fmt.Sprintf("asx:%s:stock_data", upperCode)
		doc, err = w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stock data document: %w", err)
		}
		if doc == nil {
			return nil, fmt.Errorf("no stock data document found for %s (tried asx_stock_collector and asx_stock_data)", asxCode)
		}
	}

	// Check if document is fresh enough (within 24 hours)
	if doc.LastSynced != nil {
		if time.Since(*doc.LastSynced) > 24*time.Hour {
			w.logger.Warn().
				Str("asx_code", asxCode).
				Str("last_synced", doc.LastSynced.Format("2006-01-02 15:04")).
				Msg("Stock data document is stale (>24h), using Yahoo Finance fallback")
			return nil, fmt.Errorf("stock data document is stale")
		}
	}

	// Extract historical_prices from metadata
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
				preDriftInterpretation = fmt.Sprintf("Stock rose %.1f%% in 5 days before announcement", preAnnouncementDrift)
			} else {
				preDriftInterpretation = fmt.Sprintf("Stock fell %.1f%% in 5 days before announcement", -preAnnouncementDrift)
			}
		} else {
			preDriftInterpretation = "No significant pre-announcement movement detected"
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
	HighSignalAnnouncements    []AnnouncementAnalysis
}

// generateAISummary uses AI to create an executive summary of the announcement analysis
func (w *MarketAnnouncementsWorker) generateAISummary(ctx context.Context, data AnnouncementSummaryData) (string, error) {
	if w.providerFactory == nil {
		return "", nil
	}

	// Build prompt with analysis data
	prompt := w.buildSummaryPrompt(data)

	systemInstruction := `You are a senior financial analyst providing executive summaries of company announcement histories.
Your summaries should be:
- Concise (2-3 paragraphs maximum)
- Focused on actionable investor insights
- Objective and data-driven
- Written in third person

Key aspects to cover:
1. ANNOUNCEMENT QUALITY: Are the company's announcements generally informative and market-moving, or mostly routine filings?
2. PRE-MARKET AWARENESS: Is there evidence of information leakage or insider knowledge based on pre-announcement price movements?
3. COMPANY COMMUNICATION: What themes emerge from the announcement patterns - transparency, growth signals, compliance focus, etc.?

Do NOT include any markdown formatting in your response - just plain text paragraphs.`

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

	sb.WriteString("Based on this data, provide a 2-3 paragraph executive summary covering:\n")
	sb.WriteString("1. The quality of announcements from a buyer/seller perspective\n")
	sb.WriteString("2. Evidence of pre-market awareness or information leakage\n")
	sb.WriteString("3. Overall themes in the company's market communication\n")

	return sb.String()
}

// createSummaryDocument creates a summary document with all analyzed announcements
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

	// Deduplication Summary (if any duplicates were found)
	if dedupStats.DuplicatesFound > 0 {
		content.WriteString("## Deduplication Summary\n\n")
		content.WriteString(fmt.Sprintf("**Original Announcements**: %d\n", dedupStats.TotalBefore))
		content.WriteString(fmt.Sprintf("**After Deduplication**: %d\n", dedupStats.TotalAfter))
		content.WriteString(fmt.Sprintf("**Duplicates Consolidated**: %d\n\n", dedupStats.DuplicatesFound))

		if len(dedupStats.Groups) > 0 {
			content.WriteString("| Date | Consolidated Headlines | Count |\n")
			content.WriteString("|------|----------------------|-------|\n")
			for _, g := range dedupStats.Groups {
				// Truncate first headline for display
				headline := g.Headlines[0]
				if len(headline) > 40 {
					headline = headline[:37] + "..."
				}
				content.WriteString(fmt.Sprintf("| %s | %s | %d |\n",
					g.Date.Format("2006-01-02"),
					headline,
					g.Count,
				))
			}
			content.WriteString("\n")
		}
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

	// High Signal Summary Table (quick overview)
	if highSignalCount > 0 {
		content.WriteString("### High Signal Announcements\n\n")
		content.WriteString("| Date | Headline | Price Change | Volume |\n")
		content.WriteString("|------|----------|--------------|--------|\n")
		for _, a := range analyses {
			if a.SignalNoiseRating == SignalNoiseHigh {
				headline := a.Headline
				if len(headline) > 45 {
					headline = headline[:42] + "..."
				}
				priceStr := "N/A"
				volStr := "N/A"
				if a.PriceImpact != nil {
					sign := ""
					if a.PriceImpact.ChangePercent > 0 {
						sign = "+"
					}
					priceStr = fmt.Sprintf("%s%.1f%%", sign, a.PriceImpact.ChangePercent)
					volStr = fmt.Sprintf("%.1fx", a.PriceImpact.VolumeChangeRatio)
				}
				content.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
					a.Date.Format("2006-01-02"),
					headline,
					priceStr,
					volStr,
				))
			}
		}
		content.WriteString("\n")
	}

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

	// Pre-Announcement Drift Section (HIGH-SIGNAL ONLY)
	// Only analyze announcements that are HIGH_SIGNAL and not routine filings
	// Routine filings (Appendix 3X, 3Y, etc.) are excluded as they don't drive price movement
	if preDriftCount > 0 {
		content.WriteString("## Pre-Announcement Movement Analysis (High-Signal Only)\n\n")
		content.WriteString("*Analysis limited to HIGH_SIGNAL announcements - routine filings excluded as they do not drive price movement.*\n\n")

		// Count unique dates with significant pre-drift for high-signal announcements
		seenDates := make(map[string]bool)
		uniqueDateCount := 0
		for _, a := range analyses {
			// Only count high-signal, non-routine announcements
			if a.PriceImpact != nil && a.PriceImpact.HasSignificantPreDrift &&
				a.SignalNoiseRating == SignalNoiseHigh && !a.IsRoutine {
				dateKey := a.Date.Format("2006-01-02")
				if !seenDates[dateKey] {
					seenDates[dateKey] = true
					uniqueDateCount++
				}
			}
		}

		content.WriteString(fmt.Sprintf("**Trading Days with Significant Pre-Drift (T-5 to T-1 ≥ 2%%)**: %d\n\n", uniqueDateCount))
		content.WriteString("Pre-announcement price movement may indicate:\n")
		content.WriteString("- Information leakage before official announcement\n")
		content.WriteString("- Market anticipation of news\n")
		content.WriteString("- Insider trading activity (requires further investigation)\n\n")

		// List high-signal announcements with significant pre-drift (deduplicated by date)
		// Excludes routine filings as they are not market-moving events
		content.WriteString("| Date | Headline | Pre-Drift | Interpretation |\n")
		content.WriteString("|------|----------|-----------|----------------|\n")
		seenDates = make(map[string]bool) // Reset for output loop
		for _, a := range analyses {
			// Only show high-signal, non-routine announcements in pre-announcement analysis
			if a.PriceImpact != nil && a.PriceImpact.HasSignificantPreDrift &&
				a.SignalNoiseRating == SignalNoiseHigh && !a.IsRoutine {
				dateKey := a.Date.Format("2006-01-02")
				if seenDates[dateKey] {
					continue // Skip - already output an announcement for this date
				}
				seenDates[dateKey] = true

				headline := a.Headline
				if len(headline) > 40 {
					headline = headline[:37] + "..."
				}
				sign := ""
				if a.PriceImpact.PreAnnouncementDrift > 0 {
					sign = "+"
				}
				content.WriteString(fmt.Sprintf("| %s | %s | %s%.1f%% | %s |\n",
					a.Date.Format("2006-01-02"),
					headline,
					sign,
					a.PriceImpact.PreAnnouncementDrift,
					a.PriceImpact.PreDriftInterpretation,
				))
			}
		}
		content.WriteString("\n")
	}

	// Dividend Announcements Section
	if dividendCount > 0 {
		content.WriteString("## Dividend Announcements\n\n")
		content.WriteString(fmt.Sprintf("**Total Dividend-Related Announcements**: %d\n\n", dividendCount))
		content.WriteString("*Note: Negative price movement on dividend announcements may be due to ex-dividend adjustment rather than negative market sentiment.*\n\n")
	}

	// Routine Administrative Announcements Section
	if routineCount > 0 {
		content.WriteString("## Routine Administrative Announcements\n\n")
		content.WriteString(fmt.Sprintf("**Total Routine Announcements**: %d\n\n", routineCount))
		content.WriteString("*These are standard regulatory filings excluded from signal/noise analysis - not correlated with price/volume movements.*\n\n")

		content.WriteString("| Date | Headline | Type |\n")
		content.WriteString("|------|----------|------|\n")
		for _, a := range analyses {
			if a.IsRoutine {
				headline := a.Headline
				if len(headline) > 45 {
					headline = headline[:42] + "..."
				}
				content.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
					a.Date.Format("2006-01-02"),
					headline,
					a.RoutineType,
				))
			}
		}
		content.WriteString("\n")
	}

	// Keyword-based classification summary (legacy)
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

	content.WriteString("## Relevance Distribution\n\n")
	content.WriteString("| Category | Count | Description |\n")
	content.WriteString("|----------|-------|-------------|\n")
	content.WriteString(fmt.Sprintf("| **HIGH** | %d | Price-sensitive, major corporate events |\n", highCount))
	content.WriteString(fmt.Sprintf("| MEDIUM | %d | Governance, operational updates |\n", mediumCount))
	content.WriteString(fmt.Sprintf("| LOW | %d | Routine disclosures |\n", lowCount))
	content.WriteString(fmt.Sprintf("| NOISE | %d | Non-material announcements |\n\n", noiseCount))

	// Announcements table with signal-to-noise column
	content.WriteString("## Announcements\n\n")
	content.WriteString("| Date | Headline | Signal | Price Change | Volume |\n")
	content.WriteString("|------|----------|--------|--------------|--------|\n")

	for _, a := range analyses {
		// Truncate headline for table
		headline := a.Headline
		if len(headline) > 50 {
			headline = headline[:47] + "..."
		}

		priceChangeStr := "N/A"
		volumeStr := "N/A"
		if a.PriceImpact != nil {
			sign := ""
			if a.PriceImpact.ChangePercent > 0 {
				sign = "+"
			}
			priceChangeStr = fmt.Sprintf("%s%.1f%%", sign, a.PriceImpact.ChangePercent)
			volumeStr = fmt.Sprintf("%.1fx", a.PriceImpact.VolumeChangeRatio)
		}

		markers := ""
		if a.PriceSensitive {
			markers += "⚠️"
		}
		if a.IsTradingHalt {
			markers += "⏸️"
		}
		if a.IsReinstatement {
			markers += "▶️"
		}
		if markers != "" {
			markers = " " + markers
		}

		content.WriteString(fmt.Sprintf("| %s | %s%s | %s | %s | %s |\n",
			a.Date.Format("2006-01-02"),
			headline,
			markers,
			string(a.SignalNoiseRating),
			priceChangeStr,
			volumeStr,
		))
	}
	content.WriteString("\n")
	content.WriteString("*Legend: ⚠️ Price-sensitive | ⏸️ Trading Halt | ▶️ Reinstatement*\n\n")

	// High signal detail section
	if highSignalCount > 0 {
		content.WriteString("## High Signal Announcements (Detail)\n\n")
		for _, a := range analyses {
			if a.SignalNoiseRating == SignalNoiseHigh {
				content.WriteString(fmt.Sprintf("### %s\n", a.Headline))
				content.WriteString(fmt.Sprintf("- **Date**: %s\n", a.Date.Format("2 January 2006")))
				content.WriteString(fmt.Sprintf("- **Type**: %s\n", a.Type))
				content.WriteString(fmt.Sprintf("- **Price Sensitive**: %v\n", a.PriceSensitive))
				content.WriteString(fmt.Sprintf("- **Signal Rating**: %s\n", string(a.SignalNoiseRating)))
				content.WriteString(fmt.Sprintf("- **Rationale**: %s\n", a.SignalNoiseRationale))

				if a.PriceImpact != nil {
					content.WriteString(fmt.Sprintf("- **Price Before**: $%.2f\n", a.PriceImpact.PriceBefore))
					content.WriteString(fmt.Sprintf("- **Price After**: $%.2f\n", a.PriceImpact.PriceAfter))
					content.WriteString(fmt.Sprintf("- **Price Change**: %.2f%%\n", a.PriceImpact.ChangePercent))
					content.WriteString(fmt.Sprintf("- **Volume Ratio**: %.2fx\n", a.PriceImpact.VolumeChangeRatio))
				}

				if a.PDFURL != "" {
					content.WriteString(fmt.Sprintf("- **Document**: [View PDF](%s)\n", a.PDFURL))
				}
				content.WriteString("\n")
			}
		}
	}

	// Moderate signal announcements
	if modSignalCount > 0 {
		content.WriteString("## Moderate Signal Announcements\n\n")
		for _, a := range analyses {
			if a.SignalNoiseRating == SignalNoiseModerate {
				content.WriteString(fmt.Sprintf("### %s\n", a.Headline))
				content.WriteString(fmt.Sprintf("- **Date**: %s\n", a.Date.Format("2 January 2006")))
				content.WriteString(fmt.Sprintf("- **Signal Rating**: %s\n", string(a.SignalNoiseRating)))
				content.WriteString(fmt.Sprintf("- **Rationale**: %s\n", a.SignalNoiseRationale))

				if a.PriceImpact != nil {
					content.WriteString(fmt.Sprintf("- **Price Change**: %.2f%% ($%.2f -> $%.2f)\n",
						a.PriceImpact.ChangePercent, a.PriceImpact.PriceBefore, a.PriceImpact.PriceAfter))
					content.WriteString(fmt.Sprintf("- **Volume Ratio**: %.2fx\n", a.PriceImpact.VolumeChangeRatio))
				}

				if a.PDFURL != "" {
					content.WriteString(fmt.Sprintf("- **Document**: [View PDF](%s)\n", a.PDFURL))
				}
				content.WriteString("\n")
			}
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
		// Signal-to-noise summary
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
