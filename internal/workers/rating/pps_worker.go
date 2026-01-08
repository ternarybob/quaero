// -----------------------------------------------------------------------
// PPSWorker - Price Progression Score Calculator
// Computes PPS (0-1) from announcements and price data
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
	"github.com/ternarybob/quaero/internal/services/rating"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// PPSWorker calculates Price Progression Score for tickers.
type PPSWorker struct {
	documentStorage       interfaces.DocumentStorage
	announcementsProvider interfaces.AnnouncementDataProvider
	priceProvider         interfaces.PriceDataProvider
	logger                arbor.ILogger
	jobMgr                *queue.Manager
}

var _ interfaces.DefinitionWorker = (*PPSWorker)(nil)

// NewPPSWorker creates a new PPS rating worker.
func NewPPSWorker(
	documentStorage interfaces.DocumentStorage,
	announcementsProvider interfaces.AnnouncementDataProvider,
	priceProvider interfaces.PriceDataProvider,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *PPSWorker {
	return &PPSWorker{
		documentStorage:       documentStorage,
		announcementsProvider: announcementsProvider,
		priceProvider:         priceProvider,
		logger:                logger,
		jobMgr:                jobMgr,
	}
}

func (w *PPSWorker) GetType() models.WorkerType {
	return models.WorkerTypeRatingPPS
}

func (w *PPSWorker) ReturnsChildJobs() bool {
	return false
}

func (w *PPSWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

func (w *PPSWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Calculate PPS for %s", t.String()),
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

func (w *PPSWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize rating_pps worker: %w", err)
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
		if err := w.processTickerPPS(ctx, ticker, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to calculate PPS")
		}
	}

	return stepID, nil
}

func (w *PPSWorker) processTickerPPS(ctx context.Context, ticker common.Ticker, outputTags []string) error {
	// Get announcements
	annResult, err := w.announcementsProvider.GetAnnouncements(ctx, ticker.String(), 500)
	if err != nil {
		return fmt.Errorf("announcements not available: %w", err)
	}

	// Get price data
	priceResult, err := w.priceProvider.GetPriceData(ctx, ticker.String(), "1y")
	if err != nil {
		return fmt.Errorf("price data not available: %w", err)
	}

	// Extract and calculate
	announcements := extractAnnouncementsFromDoc(annResult.Document)
	prices := extractPriceBarsFromDoc(priceResult.Document)
	ppsResult := rating.CalculatePPS(announcements, prices)

	// Save result
	return w.saveResultDocument(ctx, ticker, ppsResult, outputTags)
}

func (w *PPSWorker) saveResultDocument(ctx context.Context, ticker common.Ticker, result rating.PPSResult, outputTags []string) error {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# PPS Score: %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Score:** %.2f/1.0\n\n", result.Score))

	if len(result.EventDetails) > 0 {
		content.WriteString("## Price Events\n\n")
		content.WriteString("| Date | Headline | Retention |\n")
		content.WriteString("|------|----------|----------|\n")
		for _, e := range result.EventDetails {
			headline := e.Headline
			if len(headline) > 50 {
				headline = headline[:47] + "..."
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %.0f%% |\n",
				e.Date.Format("2006-01-02"), headline, e.RetentionPct))
		}
	}

	content.WriteString(fmt.Sprintf("\n## Reasoning\n\n%s\n", result.Reasoning))

	tags := []string{"rating-pps", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("PPS Score: %s", ticker.Code),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      models.WorkerTypeRatingPPS.String(),
		SourceID:        ticker.SourceID("rating_pps"),
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":        ticker.String(),
			"score":         result.Score,
			"event_details": result.EventDetails,
			"reasoning":     result.Reasoning,
			"calculated_at": now.Format(time.RFC3339),
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	return w.documentStorage.SaveDocument(doc)
}

// extractPriceBarsFromDoc extracts price bars from document metadata.
func extractPriceBarsFromDoc(doc *models.Document) []rating.PriceBar {
	if doc == nil || doc.Metadata == nil {
		return nil
	}

	priceData, ok := doc.Metadata["price_data"].([]interface{})
	if !ok {
		return nil
	}

	var bars []rating.PriceBar
	for _, p := range priceData {
		pMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		bar := rating.PriceBar{
			Open:   ratingGetFloat64(pMap, "open"),
			High:   ratingGetFloat64(pMap, "high"),
			Low:    ratingGetFloat64(pMap, "low"),
			Close:  ratingGetFloat64(pMap, "close"),
			Volume: ratingGetInt64(pMap, "volume"),
		}

		if dateStr, ok := pMap["date"].(string); ok {
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				bar.Date = t
			} else if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				bar.Date = t
			}
		}

		bars = append(bars, bar)
	}

	return bars
}
