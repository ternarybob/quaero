// -----------------------------------------------------------------------
// OBWorker - Optionality Bonus Calculator
// Computes OB (0, 0.5, or 1.0) from announcements and BFS score
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

// OBWorker calculates Optionality Bonus for tickers.
type OBWorker struct {
	documentStorage       interfaces.DocumentStorage
	announcementsProvider interfaces.AnnouncementDataProvider
	logger                arbor.ILogger
	jobMgr                *queue.Manager
}

var _ interfaces.DefinitionWorker = (*OBWorker)(nil)

// NewOBWorker creates a new OB rating worker.
func NewOBWorker(
	documentStorage interfaces.DocumentStorage,
	announcementsProvider interfaces.AnnouncementDataProvider,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *OBWorker {
	return &OBWorker{
		documentStorage:       documentStorage,
		announcementsProvider: announcementsProvider,
		logger:                logger,
		jobMgr:                jobMgr,
	}
}

func (w *OBWorker) GetType() models.WorkerType {
	return models.WorkerTypeRatingOB
}

func (w *OBWorker) ReturnsChildJobs() bool {
	return false
}

func (w *OBWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

func (w *OBWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Calculate OB for %s", t.String()),
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

func (w *OBWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize rating_ob worker: %w", err)
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
		if err := w.processTickerOB(ctx, ticker, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to calculate OB")
		}
	}

	return stepID, nil
}

func (w *OBWorker) processTickerOB(ctx context.Context, ticker common.Ticker, outputTags []string) error {
	// Get announcements
	annResult, err := w.announcementsProvider.GetAnnouncements(ctx, ticker.String(), 500)
	if err != nil {
		return fmt.Errorf("announcements not available: %w", err)
	}

	// Get BFS score from existing document
	bfsScore := w.getBFSScore(ticker)

	// Extract and calculate
	announcements := extractAnnouncementsFromDoc(annResult.Document)
	obResult := rating.CalculateOB(announcements, bfsScore)

	// Save result
	return w.saveResultDocument(ctx, ticker, obResult, outputTags)
}

// getBFSScore retrieves BFS score from existing document.
func (w *OBWorker) getBFSScore(ticker common.Ticker) int {
	sourceType := models.WorkerTypeRatingBFS.String()
	sourceID := ticker.SourceID("rating_bfs")

	doc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err != nil || doc == nil {
		w.logger.Warn().Str("ticker", ticker.String()).Msg("BFS score not found, assuming 0")
		return 0
	}

	if score, ok := doc.Metadata["score"].(float64); ok {
		return int(score)
	}
	if score, ok := doc.Metadata["score"].(int); ok {
		return score
	}

	return 0
}

func (w *OBWorker) saveResultDocument(ctx context.Context, ticker common.Ticker, result rating.OBResult, outputTags []string) error {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# OB Score: %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Score:** %.1f/1.0\n\n", result.Score))
	content.WriteString("## Components\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Catalyst Found | %v |\n", result.CatalystFound))
	content.WriteString(fmt.Sprintf("| Timeframe Found | %v |\n", result.TimeframeFound))
	content.WriteString(fmt.Sprintf("\n## Reasoning\n\n%s\n", result.Reasoning))

	tags := []string{"rating-ob", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("OB Score: %s", ticker.Code),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      models.WorkerTypeRatingOB.String(),
		SourceID:        ticker.SourceID("rating_ob"),
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":          ticker.String(),
			"score":           result.Score,
			"catalyst_found":  result.CatalystFound,
			"timeframe_found": result.TimeframeFound,
			"reasoning":       result.Reasoning,
			"calculated_at":   now.Format(time.RFC3339),
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	return w.documentStorage.SaveDocument(doc)
}
