// -----------------------------------------------------------------------
// ASXAnnouncementsWorker - Fetches ASX company announcements
// Uses the Markit Digital API to fetch announcements in JSON format
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

// ASXAnnouncementsWorker fetches ASX company announcements and stores them as documents.
// This worker executes synchronously (no child jobs).
type ASXAnnouncementsWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// Compile-time assertion: ASXAnnouncementsWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*ASXAnnouncementsWorker)(nil)

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

// NewASXAnnouncementsWorker creates a new ASX announcements worker
func NewASXAnnouncementsWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ASXAnnouncementsWorker {
	return &ASXAnnouncementsWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeASXAnnouncements for the DefinitionWorker interface
func (w *ASXAnnouncementsWorker) GetType() models.WorkerType {
	return models.WorkerTypeASXAnnouncements
}

// Init performs the initialization/setup phase for an ASX announcements step.
func (w *ASXAnnouncementsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for asx_announcements")
	}

	// Extract ASX code (required)
	asxCode, ok := stepConfig["asx_code"].(string)
	if !ok || asxCode == "" {
		return nil, fmt.Errorf("asx_code is required in step config")
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
				Type: "asx_announcements",
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
func (w *ASXAnnouncementsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize asx_announcements worker: %w", err)
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
		Msg("Fetching ASX announcements")

	// Log step start for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching ASX:%s announcements (period: %s)", asxCode, period))
	}

	// Fetch announcements
	announcements, err := w.fetchAnnouncements(ctx, asxCode, period, limit)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch ASX announcements")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch announcements: %v", err))
		}
		return "", fmt.Errorf("failed to fetch ASX announcements: %w", err)
	}

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

	// Store each announcement as a document
	savedCount := 0
	for _, ann := range announcements {
		doc := w.createDocument(ann, asxCode, &jobDef, stepID, outputTags)
		if err := w.documentStorage.SaveDocument(doc); err != nil {
			w.logger.Warn().Err(err).Str("headline", ann.Headline).Msg("Failed to save announcement document")
			continue
		}
		savedCount++
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Int("fetched", len(announcements)).
		Int("saved", savedCount).
		Msg("ASX announcements processed")

	// Log completion for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Saved %d ASX:%s announcements", savedCount, asxCode))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *ASXAnnouncementsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for asx_announcements type
func (w *ASXAnnouncementsWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("asx_announcements step requires config")
	}

	// Validate required asx_code field
	asxCode, ok := step.Config["asx_code"].(string)
	if !ok || asxCode == "" {
		return fmt.Errorf("asx_announcements step requires 'asx_code' in config")
	}

	return nil
}

// fetchAnnouncements fetches announcements from Markit Digital API
func (w *ASXAnnouncementsWorker) fetchAnnouncements(ctx context.Context, asxCode, period string, limit int) ([]ASXAnnouncement, error) {
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

// calculateCutoffDate returns the cutoff date based on period string
func (w *ASXAnnouncementsWorker) calculateCutoffDate(period string) time.Time {
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

// createDocument creates a Document from an ASX announcement
func (w *ASXAnnouncementsWorker) createDocument(ann ASXAnnouncement, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
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
