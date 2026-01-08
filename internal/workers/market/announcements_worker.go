// -----------------------------------------------------------------------
// AnnouncementsWorker - Fetches RAW ASX company announcements
// Uses the Markit Digital API and ASX HTML page to fetch announcements
// Produces raw announcement documents for downstream processing
//
// SEPARATION OF CONCERNS:
// - This worker: DATA COLLECTION ONLY (fetch from APIs, store raw data)
// - Processing: Handled by processing_announcements_worker
// -----------------------------------------------------------------------

package market

import (
	"context"
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
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ASXAnnouncement represents a single ASX announcement (raw data)
type ASXAnnouncement struct {
	Date           time.Time `json:"date"`
	Headline       string    `json:"headline"`
	PDFURL         string    `json:"pdf_url"`
	PDFFilename    string    `json:"pdf_filename,omitempty"`
	DocumentKey    string    `json:"document_key"`
	FileSize       string    `json:"file_size,omitempty"`
	PriceSensitive bool      `json:"price_sensitive"`
	Type           string    `json:"type"`
}

// AnnouncementsWorker fetches raw ASX company announcements.
// This worker executes synchronously (no child jobs).
// Output: Raw announcement documents (no classification or analysis)
type AnnouncementsWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*AnnouncementsWorker)(nil)

// asxAPIResponse represents the JSON response from Markit Digital API
type asxAPIResponse struct {
	Data struct {
		Items []struct {
			Date             string `json:"date"`
			Headline         string `json:"headline"`
			URL              string `json:"url"`
			DocumentKey      string `json:"documentKey"`
			FileSize         string `json:"fileSize"`
			IsPriceSensitive bool   `json:"isPriceSensitive"`
			AnnouncementType string `json:"announcementType"`
		} `json:"items"`
	} `json:"data"`
}

// NewAnnouncementsWorker creates a new announcements worker for data collection
func NewAnnouncementsWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *AnnouncementsWorker {
	jar, _ := cookiejar.New(nil)
	return &AnnouncementsWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		debugEnabled: debugEnabled,
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

	// Get config parameters
	period := "Y1"
	limit := 500
	if step.Config != nil {
		if p, ok := step.Config["period"].(string); ok && p != "" {
			period = p
		}
		if l, ok := step.Config["limit"].(float64); ok {
			limit = int(l)
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
		if err := w.processOneTicker(ctx, code, period, limit, &jobDef, stepID, outputTags); err != nil {
			w.logger.Warn().Err(err).Str("asx_code", code).Msg("Failed to fetch announcements")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to fetch announcements for %s: %v", code, err))
			}
			// Continue with other tickers
		}
	}

	return stepID, nil
}

// processOneTicker fetches and stores raw announcements for a single ticker
func (w *AnnouncementsWorker) processOneTicker(ctx context.Context, asxCode, period string, limit int, jobDef *models.JobDefinition, stepID string, outputTags []string) error {
	w.logger.Info().Str("asx_code", asxCode).Str("period", period).Msg("Fetching ASX announcements")

	// Fetch announcements
	announcements, err := w.fetchAnnouncements(ctx, asxCode, period, limit)
	if err != nil {
		return fmt.Errorf("failed to fetch announcements: %w", err)
	}

	if len(announcements) == 0 {
		w.logger.Info().Str("asx_code", asxCode).Msg("No announcements found")
		return nil
	}

	// Create raw announcement document
	doc := w.createRawDocument(ctx, announcements, asxCode, jobDef, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Int("count", len(announcements)).
		Str("doc_id", doc.ID).
		Msg("Saved raw announcements document")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Fetched %d announcements for ASX:%s", len(announcements), asxCode))
	}

	return nil
}

// createRawDocument creates a document containing raw announcement data
func (w *AnnouncementsWorker) createRawDocument(ctx context.Context, announcements []ASXAnnouncement, asxCode string, jobDef *models.JobDefinition, outputTags []string) *models.Document {
	ticker := common.Ticker{Exchange: "ASX", Code: asxCode}

	// Build tags
	// Include lowercase ticker code for tag-based lookups (consistent with processing worker)
	tags := []string{
		"asx-announcement-raw",
		strings.ToLower(ticker.Code),
		fmt.Sprintf("ticker:%s", ticker.String()),
		fmt.Sprintf("source_type:%s", models.WorkerTypeMarketAnnouncements.String()),
	}
	tags = append(tags, outputTags...)

	// Convert announcements to JSON-friendly format
	annJSON := make([]map[string]interface{}, len(announcements))
	for i, ann := range announcements {
		annJSON[i] = map[string]interface{}{
			"date":            ann.Date.Format(time.RFC3339),
			"headline":        ann.Headline,
			"type":            ann.Type,
			"pdf_url":         ann.PDFURL,
			"document_key":    ann.DocumentKey,
			"price_sensitive": ann.PriceSensitive,
		}
	}

	metadata := map[string]interface{}{
		"ticker":             ticker.String(),
		"exchange":           ticker.Exchange,
		"code":               ticker.Code,
		"announcement_count": len(announcements),
		"announcements":      annJSON,
		"fetched_at":         time.Now().Format(time.RFC3339),
	}

	// Add date range
	if len(announcements) > 0 {
		// Sort by date descending
		sort.Slice(announcements, func(i, j int) bool {
			return announcements[i].Date.After(announcements[j].Date)
		})
		metadata["date_range_start"] = announcements[len(announcements)-1].Date.Format("2006-01-02")
		metadata["date_range_end"] = announcements[0].Date.Format("2006-01-02")
	}

	// Build content summary
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# ASX Announcements - %s\n\n", asxCode))
	contentBuilder.WriteString(fmt.Sprintf("**Ticker:** %s\n", ticker.String()))
	contentBuilder.WriteString(fmt.Sprintf("**Count:** %d announcements\n\n", len(announcements)))
	contentBuilder.WriteString("## Recent Announcements\n\n")

	// Include first 20 announcements in content
	displayCount := 20
	if len(announcements) < displayCount {
		displayCount = len(announcements)
	}
	for i := 0; i < displayCount; i++ {
		ann := announcements[i]
		sensitive := ""
		if ann.PriceSensitive {
			sensitive = " [Price Sensitive]"
		}
		contentBuilder.WriteString(fmt.Sprintf("- **%s**: %s%s\n",
			ann.Date.Format("2006-01-02"), ann.Headline, sensitive))
	}
	if len(announcements) > displayCount {
		contentBuilder.WriteString(fmt.Sprintf("\n... and %d more announcements\n", len(announcements)-displayCount))
	}

	return &models.Document{
		ID:              uuid.New().String(),
		SourceType:      "asx_announcement_raw",
		SourceID:        fmt.Sprintf("%s:%s:announcement_raw", ticker.Exchange, ticker.Code),
		Title:           fmt.Sprintf("ASX Announcements - %s (Raw)", asxCode),
		ContentMarkdown: contentBuilder.String(),
		Tags:            tags,
		Metadata:        metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// fetchAnnouncements fetches announcements using the best source for the period
func (w *AnnouncementsWorker) fetchAnnouncements(ctx context.Context, asxCode, period string, limit int) ([]ASXAnnouncement, error) {
	// For Y1 or longer periods, use ASX HTML page which returns full year data
	if period == "Y1" || period == "Y5" {
		announcements, err := w.fetchAnnouncementsFromHTML(ctx, asxCode, period, limit)
		if err != nil {
			w.logger.Warn().Err(err).Msg("HTML fetch failed, falling back to Markit API")
		} else if len(announcements) > 0 {
			return announcements, nil
		}
	}

	// Build Markit Digital API URL
	url := fmt.Sprintf("https://asx.api.markitdigital.com/asx-research/1.0/companies/%s/announcements",
		strings.ToLower(asxCode))

	w.logger.Debug().Str("url", url).Msg("Fetching ASX announcements from API")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp asxAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	cutoffDate := w.calculateCutoffDate(period)

	var announcements []ASXAnnouncement
	for _, item := range apiResp.Data.Items {
		if limit > 0 && len(announcements) >= limit {
			break
		}

		date, err := time.Parse(time.RFC3339, item.Date)
		if err != nil {
			date, err = time.Parse("2006-01-02T15:04:05", item.Date)
			if err != nil {
				date = time.Now()
			}
		}

		if !cutoffDate.IsZero() && date.Before(cutoffDate) {
			continue
		}

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

// fetchAnnouncementsFromHTML scrapes announcements from ASX statistics HTML page
func (w *AnnouncementsWorker) fetchAnnouncementsFromHTML(ctx context.Context, asxCode, period string, limit int) ([]ASXAnnouncement, error) {
	currentYear := time.Now().Year()
	var allAnnouncements []ASXAnnouncement

	yearsToFetch := 2
	if period == "Y3" {
		yearsToFetch = 4
	} else if period == "Y5" {
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

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := w.httpClient.Do(req)
		if err != nil {
			w.logger.Warn().Err(err).Int("year", year).Msg("Failed to fetch HTML page")
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		announcements := w.parseAnnouncementsHTML(string(body), asxCode, year)
		allAnnouncements = append(allAnnouncements, announcements...)
	}

	// Sort by date descending and apply limit
	sort.Slice(allAnnouncements, func(i, j int) bool {
		return allAnnouncements[i].Date.After(allAnnouncements[j].Date)
	})

	if limit > 0 && len(allAnnouncements) > limit {
		allAnnouncements = allAnnouncements[:limit]
	}

	return allAnnouncements, nil
}

// parseAnnouncementsHTML extracts announcements from HTML content
func (w *AnnouncementsWorker) parseAnnouncementsHTML(html, asxCode string, year int) []ASXAnnouncement {
	var announcements []ASXAnnouncement

	// Pattern to match table rows
	rowPattern := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	rows := rowPattern.FindAllStringSubmatch(html, -1)

	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		rowContent := row[1]

		// Skip header rows
		if strings.Contains(rowContent, "<th") {
			continue
		}

		// Extract cells
		cellPattern := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
		cells := cellPattern.FindAllStringSubmatch(rowContent, -1)
		if len(cells) < 4 {
			continue
		}

		// Parse date (first cell)
		dateStr := strings.TrimSpace(stripHTML(cells[0][1]))
		date, err := time.Parse("02/01/2006", dateStr)
		if err != nil {
			date, err = time.Parse("2/01/2006", dateStr)
			if err != nil {
				continue
			}
		}

		// Parse headline and link (second cell)
		headlineCell := cells[1][1]
		headline := strings.TrimSpace(stripHTML(headlineCell))

		// Extract PDF URL
		pdfURL := ""
		documentKey := ""
		linkPattern := regexp.MustCompile(`href="([^"]+)"`)
		if linkMatch := linkPattern.FindStringSubmatch(headlineCell); len(linkMatch) > 1 {
			pdfURL = linkMatch[1]
			if strings.Contains(pdfURL, "/asxpdf/") {
				parts := strings.Split(pdfURL, "/")
				if len(parts) > 0 {
					documentKey = parts[len(parts)-1]
				}
			}
		}

		// Parse price-sensitive flag (third cell)
		priceSensitive := strings.Contains(cells[2][1], "Yes") || strings.Contains(cells[2][1], "Y")

		// Parse pages/type (fourth cell if exists)
		pageCount := 0
		if len(cells) > 3 {
			if p, err := strconv.Atoi(strings.TrimSpace(stripHTML(cells[3][1]))); err == nil {
				pageCount = p
			}
		}

		// Infer type from headline
		annType := w.inferAnnouncementType(headline, pageCount)

		announcements = append(announcements, ASXAnnouncement{
			Date:           date,
			Headline:       headline,
			PDFURL:         pdfURL,
			DocumentKey:    documentKey,
			PriceSensitive: priceSensitive,
			Type:           annType,
		})
	}

	return announcements
}

// stripHTML removes HTML tags from a string
func stripHTML(s string) string {
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	s = tagPattern.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}

// inferAnnouncementType guesses announcement type from headline
func (w *AnnouncementsWorker) inferAnnouncementType(headline string, pageCount int) string {
	headlineUpper := strings.ToUpper(headline)

	typePatterns := []struct {
		keywords []string
		annType  string
	}{
		{[]string{"TRADING HALT", "SUSPENSION"}, "Trading Halt"},
		{[]string{"QUARTERLY", "ACTIVITIES REPORT", "4C"}, "Quarterly Report"},
		{[]string{"HALF YEAR", "HALF-YEAR", "1H", "H1"}, "Half Year Report"},
		{[]string{"ANNUAL REPORT", "FULL YEAR", "FY"}, "Annual Report"},
		{[]string{"DIVIDEND"}, "Dividend"},
		{[]string{"AGM", "GENERAL MEETING"}, "Meeting"},
		{[]string{"APPENDIX"}, "Appendix"},
		{[]string{"DIRECTOR"}, "Director Related"},
		{[]string{"SUBSTANTIAL", "HOLDER"}, "Substantial Holder"},
	}

	for _, tp := range typePatterns {
		for _, kw := range tp.keywords {
			if strings.Contains(headlineUpper, kw) {
				return tp.annType
			}
		}
	}

	if pageCount > 20 {
		return "Report"
	}
	return "Announcement"
}

// calculateCutoffDate calculates the cutoff date based on period
func (w *AnnouncementsWorker) calculateCutoffDate(period string) time.Time {
	now := time.Now()

	switch period {
	case "M1":
		return now.AddDate(0, -1, 0)
	case "M3":
		return now.AddDate(0, -3, 0)
	case "M6":
		return now.AddDate(0, -6, 0)
	case "Y1":
		return now.AddDate(-1, 0, 0)
	case "Y3":
		return now.AddDate(-3, 0, 0)
	case "Y5":
		return now.AddDate(-5, 0, 0)
	default:
		return now.AddDate(-1, 0, 0) // Default to 1 year
	}
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
