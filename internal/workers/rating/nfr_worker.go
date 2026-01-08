// -----------------------------------------------------------------------
// NFRWorker - Narrative-to-Fact Ratio Calculator
// Computes NFR (0-1) from announcement data
// -----------------------------------------------------------------------

package rating

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
	"github.com/ternarybob/quaero/internal/services/rating"
)

// NFRWorker calculates Narrative-to-Fact Ratio for tickers.
type NFRWorker struct {
	documentStorage       interfaces.DocumentStorage
	announcementsProvider interfaces.AnnouncementDataProvider
	logger                arbor.ILogger
	jobMgr                *queue.Manager
}

var _ interfaces.DefinitionWorker = (*NFRWorker)(nil)

// NewNFRWorker creates a new NFR rating worker.
func NewNFRWorker(
	documentStorage interfaces.DocumentStorage,
	announcementsProvider interfaces.AnnouncementDataProvider,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *NFRWorker {
	return &NFRWorker{
		documentStorage:       documentStorage,
		announcementsProvider: announcementsProvider,
		logger:                logger,
		jobMgr:                jobMgr,
	}
}

func (w *NFRWorker) GetType() models.WorkerType {
	return models.WorkerTypeRatingNFR
}

func (w *NFRWorker) ReturnsChildJobs() bool {
	return false
}

func (w *NFRWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

func (w *NFRWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Calculate NFR for %s", t.String()),
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

func (w *NFRWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize rating_nfr worker: %w", err)
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
		if err := w.processTickerNFR(ctx, ticker, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to calculate NFR")
		}
	}

	return stepID, nil
}

func (w *NFRWorker) processTickerNFR(ctx context.Context, ticker common.Ticker, outputTags []string) error {
	// Get announcements
	annResult, err := w.announcementsProvider.GetAnnouncements(ctx, ticker.String(), 500)
	if err != nil {
		return fmt.Errorf("announcements not available: %w", err)
	}

	// Extract and calculate
	announcements := extractAnnouncementsFromDoc(annResult.Document)
	nfrResult := rating.CalculateNFR(announcements)

	// Save result
	return w.saveResultDocument(ctx, ticker, nfrResult, outputTags)
}

func (w *NFRWorker) saveResultDocument(ctx context.Context, ticker common.Ticker, result rating.NFRResult, outputTags []string) error {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# NFR Score: %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Score:** %.2f/1.0\n\n", result.Score))
	content.WriteString("## Components\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Total Announcements | %d |\n", result.Components.TotalAnnouncements))
	content.WriteString(fmt.Sprintf("| Fact-based | %d |\n", result.Components.FactAnnouncements))
	content.WriteString(fmt.Sprintf("| Narrative-based | %d |\n", result.Components.NarrativeAnnouncements))
	content.WriteString(fmt.Sprintf("| Fact Ratio | %.1f%% |\n", result.Components.FactRatio*100))
	content.WriteString(fmt.Sprintf("\n## Reasoning\n\n%s\n", result.Reasoning))

	tags := []string{"rating-nfr", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("NFR Score: %s", ticker.Code),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      models.WorkerTypeRatingNFR.String(),
		SourceID:        ticker.SourceID("rating_nfr"),
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":        ticker.String(),
			"score":         result.Score,
			"components":    result.Components,
			"reasoning":     result.Reasoning,
			"calculated_at": now.Format(time.RFC3339),
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	return w.documentStorage.SaveDocument(doc)
}

// extractAnnouncementsFromDoc extracts announcements from document.
func extractAnnouncementsFromDoc(doc *models.Document) []rating.Announcement {
	if doc == nil || doc.Metadata == nil {
		return nil
	}

	annData, ok := doc.Metadata["announcements"].([]interface{})
	if !ok {
		return nil
	}

	var announcements []rating.Announcement
	for _, a := range annData {
		annMap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}

		ann := rating.Announcement{
			Headline:         ratingGetString(annMap, "headline"),
			IsPriceSensitive: ratingGetBool(annMap, "is_price_sensitive"),
		}

		if dateStr, ok := annMap["date"].(string); ok {
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				ann.Date = t
			} else if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				ann.Date = t
			}
		}

		ann.Type = mapAnnouncementType(ratingGetString(annMap, "type"))
		announcements = append(announcements, ann)
	}

	return announcements
}
