// -----------------------------------------------------------------------
// MarketDirectorInterestWorker - Fetches ASX director interest (Appendix 3Y) filings
// Uses the Markit Digital API to fetch announcements and filters to director-related items
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

// MarketDirectorInterestWorker fetches ASX director interest filings (Appendix 3Y) and stores them as documents.
// This worker executes synchronously (no child jobs).
type MarketDirectorInterestWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// Compile-time assertion: MarketDirectorInterestWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*MarketDirectorInterestWorker)(nil)

// DirectorInterestFiling represents a director interest notice
type DirectorInterestFiling struct {
	Date        time.Time
	Director    string
	Headline    string
	PDFURL      string
	DocumentKey string
	FileSize    string
	Type        string
}

// NewMarketDirectorInterestWorker creates a new ASX director interest worker
func NewMarketDirectorInterestWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *MarketDirectorInterestWorker {
	return &MarketDirectorInterestWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeMarketDirectorInterest for the DefinitionWorker interface
func (w *MarketDirectorInterestWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketDirectorInterest
}

// Init performs the initialization/setup phase for an ASX director interest step.
func (w *MarketDirectorInterestWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for market_director_interest")
	}

	// Extract ASX code (required)
	asxCode, ok := stepConfig["asx_code"].(string)
	if !ok || asxCode == "" {
		return nil, fmt.Errorf("asx_code is required in step config")
	}
	asxCode = strings.ToUpper(asxCode)

	// Extract period (optional, used to filter results by date)
	period := "Y1"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	// Extract limit (optional, max filings to fetch)
	limit := 20 // Default
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
		Msg("ASX director interest worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   asxCode,
				Name: fmt.Sprintf("Fetch ASX:%s director interest filings", asxCode),
				Type: "market_director_interest",
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

// CreateJobs fetches ASX director interest filings and stores them as documents.
// Returns the step job ID since this executes synchronously.
func (w *MarketDirectorInterestWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize market_director_interest worker: %w", err)
		}
	}

	// Extract metadata from init result
	asxCode, _ := initResult.Metadata["asx_code"].(string)
	period, _ := initResult.Metadata["period"].(string)
	limit, _ := initResult.Metadata["limit"].(int)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
		Str("step_id", stepID).
		Msg("Fetching ASX director interest filings")

	// Log step start for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching ASX:%s director interest filings (period: %s)", asxCode, period))
	}

	// Fetch director interest filings
	filings, err := w.fetchDirectorInterest(ctx, asxCode, period, limit)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch director interest filings")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch director interest filings: %v", err))
		}
		return "", fmt.Errorf("failed to fetch director interest filings: %w", err)
	}

	if len(filings) == 0 {
		w.logger.Warn().Str("asx_code", asxCode).Msg("No director interest filings found")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", "No director interest filings found")
		}
		// Create a document indicating no filings found
		noFilingsDoc := w.createNoFilingsDocument(ctx, asxCode, &jobDef, stepID, stepConfig)
		if err := w.documentStorage.SaveDocument(noFilingsDoc); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to save no-filings document")
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

	// Store each filing as a document
	savedCount := 0
	for _, filing := range filings {
		doc := w.createDocument(ctx, filing, asxCode, &jobDef, stepID, outputTags)
		if err := w.documentStorage.SaveDocument(doc); err != nil {
			w.logger.Warn().Err(err).Str("headline", filing.Headline).Msg("Failed to save director interest document")
			continue
		}
		savedCount++
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Int("fetched", len(filings)).
		Int("saved", savedCount).
		Msg("ASX director interest filings processed")

	// Log completion for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Saved %d ASX:%s director interest filings", savedCount, asxCode))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *MarketDirectorInterestWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for market_director_interest type
func (w *MarketDirectorInterestWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("market_director_interest step requires config")
	}

	// Validate required asx_code field
	asxCode, ok := step.Config["asx_code"].(string)
	if !ok || asxCode == "" {
		return fmt.Errorf("market_director_interest step requires 'asx_code' in config")
	}

	return nil
}

// fetchDirectorInterest fetches director interest filings from Markit Digital API
func (w *MarketDirectorInterestWorker) fetchDirectorInterest(ctx context.Context, asxCode, period string, limit int) ([]DirectorInterestFiling, error) {
	// Build Markit Digital API URL (same as announcements)
	url := fmt.Sprintf("https://asx.api.markitdigital.com/asx-research/1.0/companies/%s/announcements",
		strings.ToLower(asxCode))

	w.logger.Debug().Str("url", url).Msg("Fetching ASX announcements for director interest filtering")

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

	// Parse JSON response (reuse asxAPIResponse type from announcements)
	var apiResp asxAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Calculate cutoff date based on period
	cutoffDate := w.calculateCutoffDate(period)

	var filings []DirectorInterestFiling
	for _, item := range apiResp.Data.Items {
		if limit > 0 && len(filings) >= limit {
			break
		}

		// Filter to director interest related announcements
		// Appendix 3Y: "Director's Interest Notice"
		// Appendix 3X: "Initial Director's Interest Notice"
		// Appendix 3Z: "Final Director's Interest Notice"
		headline := strings.ToLower(item.Headline)
		announcementType := strings.ToLower(item.AnnouncementType)

		isDirectorInterest := strings.Contains(headline, "director") ||
			strings.Contains(headline, "appendix 3y") ||
			strings.Contains(headline, "appendix 3x") ||
			strings.Contains(headline, "appendix 3z") ||
			strings.Contains(announcementType, "director") ||
			strings.Contains(announcementType, "3y") ||
			strings.Contains(announcementType, "3x") ||
			strings.Contains(announcementType, "3z")

		if !isDirectorInterest {
			continue
		}

		// Parse date
		date, err := time.Parse(time.RFC3339, item.Date)
		if err != nil {
			date, err = time.Parse("2006-01-02T15:04:05", item.Date)
			if err != nil {
				w.logger.Debug().Str("date", item.Date).Msg("Failed to parse filing date")
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

		// Extract director name from headline if possible
		directorName := w.extractDirectorName(item.Headline)

		filing := DirectorInterestFiling{
			Date:        date,
			Director:    directorName,
			Headline:    item.Headline,
			PDFURL:      pdfURL,
			DocumentKey: item.DocumentKey,
			FileSize:    item.FileSize,
			Type:        item.AnnouncementType,
		}

		filings = append(filings, filing)
	}

	return filings, nil
}

// extractDirectorName attempts to extract director name from headline
func (w *MarketDirectorInterestWorker) extractDirectorName(headline string) string {
	// Common patterns: "Appendix 3Y - John Smith", "Director Interest Notice - J Smith"
	// For now, return empty - would need more sophisticated parsing
	return ""
}

// calculateCutoffDate returns the cutoff date based on period string
func (w *MarketDirectorInterestWorker) calculateCutoffDate(period string) time.Time {
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
		return now.AddDate(-1, 0, 0)
	case "Y5":
		return now.AddDate(-5, 0, 0)
	default:
		return time.Time{} // No cutoff
	}
}

// createDocument creates a Document from a director interest filing
func (w *MarketDirectorInterestWorker) createDocument(ctx context.Context, filing DirectorInterestFiling, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	// Build markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Director Interest Notice: %s\n\n", filing.Headline))
	content.WriteString(fmt.Sprintf("**Date**: %s\n", filing.Date.Format("2 January 2006")))
	content.WriteString(fmt.Sprintf("**Company**: ASX:%s\n", asxCode))
	content.WriteString(fmt.Sprintf("**Type**: %s\n", filing.Type))

	if filing.Director != "" {
		content.WriteString(fmt.Sprintf("**Director**: %s\n", filing.Director))
	}

	if filing.PDFURL != "" {
		content.WriteString(fmt.Sprintf("\n**Document**: [%s](%s)\n", filing.DocumentKey, filing.PDFURL))
	}
	if filing.FileSize != "" {
		content.WriteString(fmt.Sprintf("**File Size**: %s\n", filing.FileSize))
	}

	content.WriteString("\n---\n")
	content.WriteString("\n## Significance for Investment Analysis\n")
	content.WriteString("Director interest notices indicate insider buying or selling activity.\n")
	content.WriteString("- **Director buying** is often considered a bullish signal (insider confidence)\n")
	content.WriteString("- **Director selling** may warrant investigation (but can be for personal reasons)\n")
	content.WriteString("\n*Review the PDF document for full details on shares acquired/disposed.*\n")

	// Build tags
	tags := []string{"director-interest", strings.ToLower(asxCode)}

	// Add date tag
	dateTag := fmt.Sprintf("date:%s", filing.Date.Format("2006-01-02"))
	tags = append(tags, dateTag)

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
		"asx_code":      asxCode,
		"headline":      filing.Headline,
		"filing_date":   filing.Date.Format(time.RFC3339),
		"filing_type":   filing.Type,
		"pdf_url":       filing.PDFURL,
		"document_key":  filing.DocumentKey,
		"parent_job_id": parentJobID,
	}
	if filing.Director != "" {
		metadata["director_name"] = filing.Director
	}
	if filing.FileSize != "" {
		metadata["file_size"] = filing.FileSize
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "market_director_interest",
		SourceID:        filing.PDFURL,
		URL:             filing.PDFURL,
		Title:           fmt.Sprintf("ASX:%s Director Interest - %s", asxCode, filing.Headline),
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

// createNoFilingsDocument creates a document indicating no director interest filings were found
func (w *MarketDirectorInterestWorker) createNoFilingsDocument(ctx context.Context, asxCode string, jobDef *models.JobDefinition, parentJobID string, stepConfig map[string]interface{}) *models.Document {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Director Interest Analysis: ASX:%s\n\n", asxCode))
	content.WriteString("**Status**: No director interest filings found in the specified period.\n\n")
	content.WriteString("## Analysis Implications\n")
	content.WriteString("- No recent insider buying or selling activity detected\n")
	content.WriteString("- Directors may be in a blackout period (pre-results)\n")
	content.WriteString("- Consider this neutral for investment analysis\n")

	// Build tags
	tags := []string{"director-interest", strings.ToLower(asxCode), "no-filings"}

	// Add job definition tags
	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}

	// Extract and add output_tags from step config
	if stepConfig != nil {
		if stepTags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range stepTags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					tags = append(tags, tagStr)
				}
			}
		} else if stepTags, ok := stepConfig["output_tags"].([]string); ok {
			tags = append(tags, stepTags...)
		}
	}

	// Apply cache tags from context
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "market_director_interest",
		SourceID:        fmt.Sprintf("no-filings-%s-%s", asxCode, now.Format("2006-01-02")),
		Title:           fmt.Sprintf("ASX:%s Director Interest - No Recent Filings", asxCode),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		Metadata: map[string]interface{}{
			"asx_code":      asxCode,
			"status":        "no_filings",
			"parent_job_id": parentJobID,
		},
		Tags:       tags,
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	return doc
}
