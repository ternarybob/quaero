// -----------------------------------------------------------------------
// AnnouncementsWorker - Announcement Processing and Classification
// Reads raw announcement documents and produces enriched summary documents
// with relevance classification, signal-noise analysis, and price impact
//
// SEPARATION OF CONCERNS:
// - MarketAnnouncementsWorker: DATA COLLECTION (fetch from APIs)
// - This worker: PROCESSING (classify, analyze, enrich)
// -----------------------------------------------------------------------

package processing

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
	"github.com/ternarybob/quaero/internal/services/announcements"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// AnnouncementsWorker processes raw announcement documents
// and produces enriched summary documents with classification and analysis.
type AnnouncementsWorker struct {
	documentStorage interfaces.DocumentStorage
	priceProvider   interfaces.PriceDataProvider
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*AnnouncementsWorker)(nil)

// NewAnnouncementsWorker creates a new processing worker.
func NewAnnouncementsWorker(
	documentStorage interfaces.DocumentStorage,
	priceProvider interfaces.PriceDataProvider,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *AnnouncementsWorker {
	return &AnnouncementsWorker{
		documentStorage: documentStorage,
		priceProvider:   priceProvider,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns the worker type identifier.
func (w *AnnouncementsWorker) GetType() models.WorkerType {
	return models.WorkerTypeProcessingAnnouncements
}

// ReturnsChildJobs returns false - this worker executes inline.
func (w *AnnouncementsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration.
func (w *AnnouncementsWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

// Init initializes the worker and returns work items.
func (w *AnnouncementsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Process announcements for %s", t.String()),
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:  workItems,
		TotalCount: len(tickers),
		Strategy:   interfaces.ProcessingStrategyInline,
		Metadata: map[string]interface{}{
			"tickers": tickers,
		},
	}, nil
}

// CreateJobs processes raw announcements for each ticker.
func (w *AnnouncementsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize processing_announcements worker: %w", err)
		}
	}

	tickers := initResult.Metadata["tickers"].([]common.Ticker)

	var outputTags []string
	if tags, ok := step.Config["output_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr != "" {
				outputTags = append(outputTags, tagStr)
			}
		}
	}

	for _, ticker := range tickers {
		if err := w.processTickerAnnouncements(ctx, ticker, step, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to process announcements")
		}
	}

	return stepID, nil
}

// processTickerAnnouncements processes announcements for a single ticker.
func (w *AnnouncementsWorker) processTickerAnnouncements(ctx context.Context, ticker common.Ticker, step models.JobStep, outputTags []string) error {
	// Find raw announcement document by tags
	rawDoc, err := w.findRawAnnouncementDocument(ctx, ticker)
	if err != nil {
		return fmt.Errorf("failed to find raw announcement document: %w", err)
	}

	// Extract raw announcements from document metadata
	rawAnnouncements, err := w.extractRawAnnouncements(rawDoc)
	if err != nil {
		return fmt.Errorf("failed to extract raw announcements: %w", err)
	}

	if len(rawAnnouncements) == 0 {
		w.logger.Info().Str("ticker", ticker.String()).Msg("no announcements to process")
		return nil
	}

	// Fetch price data for impact analysis
	var priceBars []announcements.PriceBar
	if w.priceProvider != nil {
		priceResult, err := w.priceProvider.GetPriceData(ctx, ticker.String(), "1y")
		if err != nil {
			w.logger.Warn().Err(err).Str("ticker", ticker.String()).Msg("price data not available, proceeding without price impact")
		} else if priceResult != nil && priceResult.Document != nil {
			priceBars = w.extractPriceBars(priceResult.Document)
		}
	}

	// Process announcements using the service package
	_, summary, _ := announcements.ProcessAnnouncements(rawAnnouncements, priceBars)

	// Create and save enriched summary document
	return w.saveSummaryDocument(ctx, ticker, summary, rawDoc, outputTags)
}

// findRawAnnouncementDocument finds the raw announcement document for a ticker.
func (w *AnnouncementsWorker) findRawAnnouncementDocument(ctx context.Context, ticker common.Ticker) (*models.Document, error) {
	// Search for documents with the raw announcement tag and ticker
	// Tags use OR logic, so we search for the ticker-specific raw document
	searchTags := []string{"asx-announcement-raw", strings.ToLower(ticker.Code)}

	docs, err := w.documentStorage.ListDocuments(&interfaces.ListOptions{
		Tags:   searchTags,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Filter to find the document that has BOTH tags (ListDocuments uses OR logic)
	for _, doc := range docs {
		hasRawTag := false
		hasTickerTag := false
		for _, tag := range doc.Tags {
			if tag == "asx-announcement-raw" {
				hasRawTag = true
			}
			if strings.EqualFold(tag, ticker.Code) {
				hasTickerTag = true
			}
		}
		if hasRawTag && hasTickerTag {
			return doc, nil
		}
	}

	return nil, fmt.Errorf("no raw announcement document found for %s", ticker.String())
}

// extractRawAnnouncements extracts raw announcements from the document metadata.
func (w *AnnouncementsWorker) extractRawAnnouncements(doc *models.Document) ([]announcements.RawAnnouncement, error) {
	if doc.Metadata == nil {
		return nil, fmt.Errorf("document has no metadata")
	}

	annsData, ok := doc.Metadata["announcements"]
	if !ok {
		return nil, fmt.Errorf("no announcements in metadata")
	}

	// Convert to JSON and back to ensure proper typing
	jsonData, err := json.Marshal(annsData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal announcements: %w", err)
	}

	var rawAnns []announcements.RawAnnouncement
	if err := json.Unmarshal(jsonData, &rawAnns); err != nil {
		return nil, fmt.Errorf("failed to unmarshal announcements: %w", err)
	}

	return rawAnns, nil
}

// extractPriceBars extracts price bars from a price document.
func (w *AnnouncementsWorker) extractPriceBars(doc *models.Document) []announcements.PriceBar {
	if doc == nil || doc.Metadata == nil {
		return nil
	}

	priceData, ok := doc.Metadata["price_history"]
	if !ok {
		return nil
	}

	jsonData, err := json.Marshal(priceData)
	if err != nil {
		return nil
	}

	var bars []announcements.PriceBar
	if err := json.Unmarshal(jsonData, &bars); err != nil {
		return nil
	}

	return bars
}

// saveSummaryDocument creates and saves the enriched summary document.
func (w *AnnouncementsWorker) saveSummaryDocument(ctx context.Context, ticker common.Ticker, summary announcements.ProcessingSummary, rawDoc *models.Document, outputTags []string) error {
	// Build content markdown
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# ASX Announcements Summary - %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Total Announcements:** %d\n", summary.TotalCount))
	content.WriteString(fmt.Sprintf("**High Relevance:** %d\n", summary.HighRelevanceCount))
	content.WriteString(fmt.Sprintf("**Medium Relevance:** %d\n", summary.MediumRelevanceCount))
	content.WriteString(fmt.Sprintf("**Low Relevance:** %d\n", summary.LowRelevanceCount))
	content.WriteString(fmt.Sprintf("**Noise:** %d\n\n", summary.NoiseCount))

	// Signal-to-noise metrics
	if summary.MQSScores != nil {
		content.WriteString("## Signal Quality Metrics\n\n")
		content.WriteString(fmt.Sprintf("- Signal-to-Noise Ratio: %.2f\n", summary.MQSScores.SignalToNoiseRatio))
		content.WriteString(fmt.Sprintf("- High Signal Count: %d\n", summary.MQSScores.HighSignalCount))
		content.WriteString(fmt.Sprintf("- Routine Count: %d\n\n", summary.MQSScores.RoutineCount))
	}

	// Announcement details
	if len(summary.Announcements) > 0 {
		content.WriteString("## Announcements\n\n")
		displayCount := len(summary.Announcements)
		if displayCount > 20 {
			displayCount = 20
		}
		for i := 0; i < displayCount; i++ {
			ann := summary.Announcements[i]
			sensitiveMarker := ""
			if ann.PriceSensitive {
				sensitiveMarker = " [PS]"
			}
			content.WriteString(fmt.Sprintf("### %s%s\n", ann.Headline, sensitiveMarker))
			content.WriteString(fmt.Sprintf("- Date: %s\n", ann.Date.Format("2006-01-02")))
			content.WriteString(fmt.Sprintf("- Relevance: %s (%s)\n", ann.RelevanceCategory, ann.RelevanceReason))
			content.WriteString(fmt.Sprintf("- Signal: %s\n", ann.SignalNoiseRating))
			if ann.PriceImpact != nil {
				content.WriteString(fmt.Sprintf("- Price Impact: %.2f%% (Volume %.1fx)\n",
					ann.PriceImpact.ChangePercent, ann.PriceImpact.VolumeChangeRatio))
			}
			content.WriteString("\n")
		}
		if len(summary.Announcements) > displayCount {
			content.WriteString(fmt.Sprintf("... and %d more announcements\n", len(summary.Announcements)-displayCount))
		}
	}

	// Build tags
	tags := []string{"asx-announcement-summary", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	// Build metadata
	metadata := map[string]interface{}{
		"ticker":                ticker.Code,
		"exchange":              ticker.Exchange,
		"total_count":           summary.TotalCount,
		"high_relevance_count":  summary.HighRelevanceCount,
		"medium_relevance_count": summary.MediumRelevanceCount,
		"low_relevance_count":   summary.LowRelevanceCount,
		"noise_count":           summary.NoiseCount,
		"announcements":         summary.Announcements,
		"mqs_scores":            summary.MQSScores,
		"processed_at":          time.Now().UTC().Format(time.RFC3339),
	}

	// Add period from raw doc if available
	if rawDoc.Metadata != nil {
		if period, ok := rawDoc.Metadata["period"].(string); ok {
			metadata["period"] = period
		}
	}

	doc := &models.Document{
		ID:              uuid.New().String(),
		SourceType:      "asx_announcement_summary",
		SourceID:        fmt.Sprintf("%s:%s:announcement_summary", ticker.Exchange, ticker.Code),
		Title:           fmt.Sprintf("ASX Announcements Summary - %s", ticker.Code),
		ContentMarkdown: content.String(),
		Tags:            tags,
		Metadata:        metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save summary document: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Int("total_count", summary.TotalCount).
		Int("high_relevance", summary.HighRelevanceCount).
		Msg("announcement summary document saved")

	return nil
}
