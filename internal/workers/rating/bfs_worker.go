// -----------------------------------------------------------------------
// BFSWorker - Business Foundation Score Calculator
// Computes BFS (0-2) from fundamentals data
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

// BFSWorker calculates Business Foundation Score for tickers.
// This worker executes synchronously (no child jobs).
type BFSWorker struct {
	documentStorage      interfaces.DocumentStorage
	fundamentalsProvider interfaces.FundamentalsDataProvider
	logger               arbor.ILogger
	jobMgr               *queue.Manager
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*BFSWorker)(nil)

// NewBFSWorker creates a new BFS rating worker.
func NewBFSWorker(
	documentStorage interfaces.DocumentStorage,
	fundamentalsProvider interfaces.FundamentalsDataProvider,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *BFSWorker {
	return &BFSWorker{
		documentStorage:      documentStorage,
		fundamentalsProvider: fundamentalsProvider,
		logger:               logger,
		jobMgr:               jobMgr,
	}
}

// GetType returns the worker type.
func (w *BFSWorker) GetType() models.WorkerType {
	return models.WorkerTypeRatingBFS
}

// ReturnsChildJobs indicates this worker processes inline (no child jobs).
func (w *BFSWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the step configuration.
func (w *BFSWorker) ValidateConfig(step models.JobStep) error {
	// Ticker can come from step config or job variables
	return nil
}

// Init prepares work items from configuration.
func (w *BFSWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Calculate BFS for %s", t.String()),
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

// CreateJobs processes tickers and calculates BFS scores.
func (w *BFSWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize rating_bfs worker: %w", err)
		}
	}

	tickers := initResult.Metadata["tickers"].([]common.Ticker)

	// Extract output_tags from step config
	var outputTags []string
	if tags, ok := step.Config["output_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr != "" {
				outputTags = append(outputTags, tagStr)
			}
		}
	}

	// Process each ticker
	for _, ticker := range tickers {
		if err := w.processTickerBFS(ctx, ticker, outputTags, stepID); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to calculate BFS")
			// Continue with other tickers
		}
	}

	return stepID, nil
}

// processTickerBFS calculates BFS for a single ticker.
func (w *BFSWorker) processTickerBFS(ctx context.Context, ticker common.Ticker, outputTags []string, stepID string) error {
	// 1. Get fundamentals document
	result, err := w.fundamentalsProvider.GetFundamentals(ctx, ticker.String())
	if err != nil {
		return fmt.Errorf("fundamentals not available: %w", err)
	}
	if result.Document == nil {
		return fmt.Errorf("fundamentals document is nil")
	}

	// 2. Extract fundamentals from document metadata
	fundamentals := w.extractFundamentals(result.Document)

	// 3. Call pure service function
	bfsResult := rating.CalculateBFS(fundamentals)

	// 4. Save result as document
	return w.saveResultDocument(ctx, ticker, bfsResult, outputTags)
}

// extractFundamentals extracts Fundamentals from document metadata.
func (w *BFSWorker) extractFundamentals(doc *models.Document) rating.Fundamentals {
	m := doc.Metadata
	if m == nil {
		m = make(map[string]interface{})
	}

	// Extract shares outstanding 3 years ago if available
	var sharesOutstanding3YAgo *int64
	if v, ok := m["shares_outstanding_3y"].(float64); ok {
		val := int64(v)
		sharesOutstanding3YAgo = &val
	}

	return rating.Fundamentals{
		Ticker:                   ratingGetString(m, "ticker"),
		CompanyName:              ratingGetString(m, "company_name"),
		Sector:                   ratingGetString(m, "sector"),
		MarketCap:                ratingGetFloat64(m, "market_cap"),
		SharesOutstandingCurrent: ratingGetInt64(m, "shares_outstanding"),
		SharesOutstanding3YAgo:   sharesOutstanding3YAgo,
		CashBalance:              ratingGetFloat64(m, "cash_balance"),
		QuarterlyCashBurn:        ratingGetFloat64(m, "quarterly_cash_burn"),
		RevenueTTM:               ratingGetFloat64(m, "revenue_ttm"),
		IsProfitable:             ratingGetBool(m, "is_profitable"),
		HasProducingAsset:        ratingGetBool(m, "has_producing_asset"),
	}
}

// saveResultDocument saves BFS result as a document.
func (w *BFSWorker) saveResultDocument(ctx context.Context, ticker common.Ticker, result rating.BFSResult, outputTags []string) error {
	// Build markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# BFS Score: %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Score:** %d/2\n\n", result.Score))
	content.WriteString(fmt.Sprintf("**Indicators Met:** %d/4\n\n", result.IndicatorCount))
	content.WriteString("## Components\n\n")
	content.WriteString("| Component | Value |\n")
	content.WriteString("|-----------|-------|\n")
	content.WriteString(fmt.Sprintf("| Revenue > $10M | %v |\n", result.Components.HasRevenue))
	content.WriteString(fmt.Sprintf("| Cash Runway > 18mo | %.0f months |\n", result.Components.CashRunwayMonths))
	content.WriteString(fmt.Sprintf("| Producing Asset | %v |\n", result.Components.HasProducingAsset))
	content.WriteString(fmt.Sprintf("| Profitable | %v |\n", result.Components.IsProfitable))
	content.WriteString(fmt.Sprintf("\n## Reasoning\n\n%s\n", result.Reasoning))

	// Build tags
	tags := []string{"rating-bfs", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("BFS Score: %s", ticker.Code),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      models.WorkerTypeRatingBFS.String(),
		SourceID:        ticker.SourceID("rating_bfs"),
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":          ticker.String(),
			"score":           result.Score,
			"indicator_count": result.IndicatorCount,
			"components":      result.Components,
			"reasoning":       result.Reasoning,
			"calculated_at":   now.Format(time.RFC3339),
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	return w.documentStorage.SaveDocument(doc)
}

// Helper functions for metadata extraction (rating workers)
func ratingGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func ratingGetFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func ratingGetInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	if v, ok := m[key].(int64); ok {
		return v
	}
	return 0
}

func ratingGetBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
