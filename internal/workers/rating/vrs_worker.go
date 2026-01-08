// -----------------------------------------------------------------------
// VRSWorker - Volatility Regime Stability Calculator
// Computes VRS (0-1) from price data
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

// VRSWorker calculates Volatility Regime Stability for tickers.
type VRSWorker struct {
	documentStorage interfaces.DocumentStorage
	priceProvider   interfaces.PriceDataProvider
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

var _ interfaces.DefinitionWorker = (*VRSWorker)(nil)

// NewVRSWorker creates a new VRS rating worker.
func NewVRSWorker(
	documentStorage interfaces.DocumentStorage,
	priceProvider interfaces.PriceDataProvider,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *VRSWorker {
	return &VRSWorker{
		documentStorage: documentStorage,
		priceProvider:   priceProvider,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

func (w *VRSWorker) GetType() models.WorkerType {
	return models.WorkerTypeRatingVRS
}

func (w *VRSWorker) ReturnsChildJobs() bool {
	return false
}

func (w *VRSWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

func (w *VRSWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Calculate VRS for %s", t.String()),
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

func (w *VRSWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize rating_vrs worker: %w", err)
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
		if err := w.processTickerVRS(ctx, ticker, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to calculate VRS")
		}
	}

	return stepID, nil
}

func (w *VRSWorker) processTickerVRS(ctx context.Context, ticker common.Ticker, outputTags []string) error {
	// Get price data (need longer history for volatility analysis)
	priceResult, err := w.priceProvider.GetPriceData(ctx, ticker.String(), "2y")
	if err != nil {
		return fmt.Errorf("price data not available: %w", err)
	}

	// Extract and calculate
	prices := extractPriceBarsFromDoc(priceResult.Document)
	vrsResult := rating.CalculateVRS(prices)

	// Save result
	return w.saveResultDocument(ctx, ticker, vrsResult, outputTags)
}

func (w *VRSWorker) saveResultDocument(ctx context.Context, ticker common.Ticker, result rating.VRSResult, outputTags []string) error {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# VRS Score: %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Score:** %.2f/1.0\n\n", result.Score))
	content.WriteString("## Components\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Regime Count | %d |\n", result.Components.RegimeCount))
	content.WriteString(fmt.Sprintf("| Stable Regimes | %.1f%% |\n", result.Components.StableRegimesPct))
	content.WriteString(fmt.Sprintf("| Pattern | %s |\n", result.Components.VolatilityPattern))
	content.WriteString(fmt.Sprintf("\n## Reasoning\n\n%s\n", result.Reasoning))

	tags := []string{"rating-vrs", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("VRS Score: %s", ticker.Code),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      models.WorkerTypeRatingVRS.String(),
		SourceID:        ticker.SourceID("rating_vrs"),
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
