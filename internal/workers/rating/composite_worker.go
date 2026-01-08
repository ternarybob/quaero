// -----------------------------------------------------------------------
// CompositeWorker - Composite Rating Calculator
// Combines all component scores into final investability rating
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

// CompositeWorker calculates final investability rating from all component scores.
type CompositeWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

var _ interfaces.DefinitionWorker = (*CompositeWorker)(nil)

// NewCompositeWorker creates a new composite rating worker.
func NewCompositeWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *CompositeWorker {
	return &CompositeWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

func (w *CompositeWorker) GetType() models.WorkerType {
	return models.WorkerTypeRatingComposite
}

func (w *CompositeWorker) ReturnsChildJobs() bool {
	return false
}

func (w *CompositeWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

func (w *CompositeWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Calculate rating for %s", t.String()),
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

func (w *CompositeWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize rating_composite worker: %w", err)
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
		if err := w.processTickerRating(ctx, ticker, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to calculate rating")
		}
	}

	return stepID, nil
}

func (w *CompositeWorker) processTickerRating(ctx context.Context, ticker common.Ticker, outputTags []string) error {
	// Load all component scores
	bfs := w.loadBFSResult(ticker)
	cds := w.loadCDSResult(ticker)
	nfr := w.loadNFRResult(ticker)
	pps := w.loadPPSResult(ticker)
	vrs := w.loadVRSResult(ticker)
	ob := w.loadOBResult(ticker)

	// Calculate composite rating
	ratingResult := rating.CalculateRating(bfs, cds, nfr, pps, vrs, ob)

	// Save result
	return w.saveResultDocument(ctx, ticker, ratingResult, outputTags)
}

func (w *CompositeWorker) loadBFSResult(ticker common.Ticker) rating.BFSResult {
	doc, err := w.documentStorage.GetDocumentBySource(
		models.WorkerTypeRatingBFS.String(),
		ticker.SourceID("rating_bfs"),
	)
	if err != nil || doc == nil {
		return rating.BFSResult{}
	}

	m := doc.Metadata
	result := rating.BFSResult{
		Score:          int(ratingGetFloat64(m, "score")),
		IndicatorCount: int(ratingGetFloat64(m, "indicator_count")),
		Reasoning:      ratingGetString(m, "reasoning"),
	}

	if comp, ok := m["components"].(map[string]interface{}); ok {
		result.Components = rating.BFSComponents{
			HasRevenue:        ratingGetBool(comp, "has_revenue"),
			RevenueAmount:     ratingGetFloat64(comp, "revenue_amount"),
			CashRunwayMonths:  ratingGetFloat64(comp, "cash_runway_months"),
			HasProducingAsset: ratingGetBool(comp, "has_producing_asset"),
			IsProfitable:      ratingGetBool(comp, "is_profitable"),
		}
	}

	return result
}

func (w *CompositeWorker) loadCDSResult(ticker common.Ticker) rating.CDSResult {
	doc, err := w.documentStorage.GetDocumentBySource(
		models.WorkerTypeRatingCDS.String(),
		ticker.SourceID("rating_cds"),
	)
	if err != nil || doc == nil {
		return rating.CDSResult{}
	}

	m := doc.Metadata
	result := rating.CDSResult{
		Score:     int(ratingGetFloat64(m, "score")),
		Reasoning: ratingGetString(m, "reasoning"),
	}

	if comp, ok := m["components"].(map[string]interface{}); ok {
		result.Components = rating.CDSComponents{
			SharesCAGR:       ratingGetFloat64(comp, "shares_cagr"),
			TradingHaltsPA:   ratingGetFloat64(comp, "trading_halts_pa"),
			CapitalRaisesPA:  ratingGetFloat64(comp, "capital_raises_pa"),
			AnalysisPeriodMo: int(ratingGetFloat64(comp, "analysis_period_mo")),
		}
	}

	return result
}

func (w *CompositeWorker) loadNFRResult(ticker common.Ticker) rating.NFRResult {
	doc, err := w.documentStorage.GetDocumentBySource(
		models.WorkerTypeRatingNFR.String(),
		ticker.SourceID("rating_nfr"),
	)
	if err != nil || doc == nil {
		return rating.NFRResult{Score: 0.5} // Neutral default
	}

	m := doc.Metadata
	result := rating.NFRResult{
		Score:     ratingGetFloat64(m, "score"),
		Reasoning: ratingGetString(m, "reasoning"),
	}

	if comp, ok := m["components"].(map[string]interface{}); ok {
		result.Components = rating.NFRComponents{
			TotalAnnouncements:     int(ratingGetFloat64(comp, "total_announcements")),
			FactAnnouncements:      int(ratingGetFloat64(comp, "fact_announcements")),
			NarrativeAnnouncements: int(ratingGetFloat64(comp, "narrative_announcements")),
			FactRatio:              ratingGetFloat64(comp, "fact_ratio"),
		}
	}

	return result
}

func (w *CompositeWorker) loadPPSResult(ticker common.Ticker) rating.PPSResult {
	doc, err := w.documentStorage.GetDocumentBySource(
		models.WorkerTypeRatingPPS.String(),
		ticker.SourceID("rating_pps"),
	)
	if err != nil || doc == nil {
		return rating.PPSResult{Score: 0.5} // Neutral default
	}

	m := doc.Metadata
	return rating.PPSResult{
		Score:     ratingGetFloat64(m, "score"),
		Reasoning: ratingGetString(m, "reasoning"),
	}
}

func (w *CompositeWorker) loadVRSResult(ticker common.Ticker) rating.VRSResult {
	doc, err := w.documentStorage.GetDocumentBySource(
		models.WorkerTypeRatingVRS.String(),
		ticker.SourceID("rating_vrs"),
	)
	if err != nil || doc == nil {
		return rating.VRSResult{Score: 0.5} // Neutral default
	}

	m := doc.Metadata
	result := rating.VRSResult{
		Score:     ratingGetFloat64(m, "score"),
		Reasoning: ratingGetString(m, "reasoning"),
	}

	if comp, ok := m["components"].(map[string]interface{}); ok {
		result.Components = rating.VRSComponents{
			RegimeCount:       int(ratingGetFloat64(comp, "regime_count")),
			StableRegimesPct:  ratingGetFloat64(comp, "stable_regimes_pct"),
			VolatilityPattern: ratingGetString(comp, "volatility_pattern"),
		}
	}

	return result
}

func (w *CompositeWorker) loadOBResult(ticker common.Ticker) rating.OBResult {
	doc, err := w.documentStorage.GetDocumentBySource(
		models.WorkerTypeRatingOB.String(),
		ticker.SourceID("rating_ob"),
	)
	if err != nil || doc == nil {
		return rating.OBResult{}
	}

	m := doc.Metadata
	return rating.OBResult{
		Score:          ratingGetFloat64(m, "score"),
		CatalystFound:  ratingGetBool(m, "catalyst_found"),
		TimeframeFound: ratingGetBool(m, "timeframe_found"),
		Reasoning:      ratingGetString(m, "reasoning"),
	}
}

func (w *CompositeWorker) saveResultDocument(ctx context.Context, ticker common.Ticker, result rating.RatingResult, outputTags []string) error {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Stock Rating: %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Label:** %s\n\n", result.Label))

	if result.Investability != nil {
		content.WriteString(fmt.Sprintf("**Investability:** %.1f/100\n\n", *result.Investability))
	} else {
		content.WriteString("**Investability:** N/A (gate failed)\n\n")
	}

	content.WriteString(fmt.Sprintf("**Gate Passed:** %v\n\n", result.GatePassed))

	content.WriteString("## Component Scores\n\n")
	content.WriteString("| Component | Score | Max |\n")
	content.WriteString("|-----------|-------|-----|\n")
	content.WriteString(fmt.Sprintf("| BFS (Business Foundation) | %d | 2 |\n", result.Scores.BFS.Score))
	content.WriteString(fmt.Sprintf("| CDS (Capital Discipline) | %d | 2 |\n", result.Scores.CDS.Score))
	content.WriteString(fmt.Sprintf("| NFR (Narrative-to-Fact) | %.2f | 1.0 |\n", result.Scores.NFR.Score))
	content.WriteString(fmt.Sprintf("| PPS (Price Progression) | %.2f | 1.0 |\n", result.Scores.PPS.Score))
	content.WriteString(fmt.Sprintf("| VRS (Volatility Stability) | %.2f | 1.0 |\n", result.Scores.VRS.Score))
	content.WriteString(fmt.Sprintf("| OB (Optionality Bonus) | %.1f | 1.0 |\n", result.Scores.OB.Score))

	content.WriteString(fmt.Sprintf("\n## Reasoning\n\n%s\n", result.Reasoning))

	// Build tags
	tags := []string{"stock-rating", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Stock Rating: %s - %s", ticker.Code, result.Label),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      models.WorkerTypeRatingComposite.String(),
		SourceID:        ticker.SourceID("rating_composite"),
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":        ticker.String(),
			"label":         string(result.Label),
			"investability": result.Investability,
			"gate_passed":   result.GatePassed,
			"scores":        result.Scores,
			"reasoning":     result.Reasoning,
			"calculated_at": now.Format(time.RFC3339),
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	return w.documentStorage.SaveDocument(doc)
}
