// -----------------------------------------------------------------------
// PortfolioWorker - Aggregates ticker signals into portfolio metrics
// Produces portfolio-level analysis with concentration checks and action summary
// -----------------------------------------------------------------------

package market

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/signals"
)

// PortfolioWorker aggregates individual stock signals into portfolio-level metrics.
type PortfolioWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*PortfolioWorker)(nil)

// NewPortfolioWorker creates a new portfolio rollup worker
func NewPortfolioWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *PortfolioWorker {
	return &PortfolioWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeMarketPortfolio
func (w *PortfolioWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketPortfolio
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *PortfolioWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *PortfolioWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("portfolio_rollup step requires config")
	}
	if _, ok := step.Config["portfolio_tag"].(string); !ok {
		return fmt.Errorf("portfolio_rollup step requires 'portfolio_tag' in config")
	}
	return nil
}

// Init initializes the portfolio rollup worker
func (w *PortfolioWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Msg("Portfolio rollup worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     "portfolio-rollup",
				Name:   "Aggregate portfolio signals",
				Type:   "market_portfolio",
				Config: step.Config,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"step_config": step.Config,
		},
	}, nil
}

// CreateJobs aggregates portfolio signals and stores result
func (w *PortfolioWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize portfolio_rollup worker: %w", err)
		}
	}

	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Load portfolio state
	portfolioTag, _ := stepConfig["portfolio_tag"].(string)
	portfolio, err := w.loadPortfolioState(ctx, portfolioTag)
	if err != nil {
		return "", fmt.Errorf("failed to load portfolio state: %w", err)
	}

	w.logger.Info().
		Int("holdings", len(portfolio.Holdings)).
		Str("portfolio", portfolio.Meta.Name).
		Msg("Portfolio loaded")

	// Load signal documents for each holding
	signalsTagPrefix := "ticker-signals"
	if prefix, ok := stepConfig["signals_tag_prefix"].(string); ok {
		signalsTagPrefix = prefix
	}

	tickerSignals := w.loadSignals(ctx, portfolio, signalsTagPrefix)
	w.logger.Info().
		Int("signals_found", len(tickerSignals)).
		Int("holdings", len(portfolio.Holdings)).
		Msg("Signals loaded")

	// Compute rollup
	rollup := w.computeRollup(portfolio, tickerSignals)

	// Generate markdown
	markdown := w.generateMarkdown(portfolio.Meta.Name, rollup)

	// Extract output_tags (supports both []interface{} from TOML and []string from inline calls)
	var outputTags []string
	if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr != "" {
				outputTags = append(outputTags, tagStr)
			}
		}
	} else if tags, ok := stepConfig["output_tags"].([]string); ok {
		outputTags = tags
	}

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"portfolio-rollup", dateTag}
	tags = append(tags, outputTags...)

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "market_portfolio",
		SourceID:        fmt.Sprintf("rollup:%s", portfolio.Meta.Name),
		Title:           fmt.Sprintf("Portfolio Rollup: %s", portfolio.Meta.Name),
		ContentMarkdown: markdown,
		DetailLevel:     models.DetailLevelFull,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"portfolio_name":       portfolio.Meta.Name,
			"computed_at":          now.Format(time.RFC3339),
			"holdings_count":       len(portfolio.Holdings),
			"holdings_assessed":    rollup.Meta.HoldingsAssessed,
			"total_value":          rollup.Performance.TotalValue,
			"total_pnl":            rollup.Performance.TotalPnL,
			"total_pnl_pct":        rollup.Performance.TotalPnLPct,
			"concentration_alerts": rollup.ConcentrationAlerts,
			"immediate_actions":    rollup.ActionSummary.ImmediateActions,
			"rollup":               rollup,
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to save rollup document: %w", err)
	}

	w.logger.Info().
		Str("portfolio", portfolio.Meta.Name).
		Float64("total_value", rollup.Performance.TotalValue).
		Float64("total_pnl_pct", rollup.Performance.TotalPnLPct).
		Int("alerts", len(rollup.ConcentrationAlerts)).
		Msg("Portfolio rollup complete")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Portfolio %s: $%.0f total, %.1f%% return, %d alerts",
				portfolio.Meta.Name, rollup.Performance.TotalValue,
				rollup.Performance.TotalPnLPct, len(rollup.ConcentrationAlerts)))
	}

	return stepID, nil
}

// loadPortfolioState loads portfolio from document storage
func (w *PortfolioWorker) loadPortfolioState(ctx context.Context, portfolioTag string) (signals.PortfolioState, error) {
	// Try to find portfolio document by source
	doc, err := w.documentStorage.GetDocumentBySource("navexa_holdings", portfolioTag)
	if err != nil {
		// Try alternative lookup patterns
		doc, err = w.documentStorage.GetDocumentBySource("portfolio_state", portfolioTag)
		if err != nil {
			return signals.PortfolioState{}, fmt.Errorf("portfolio not found: %s", portfolioTag)
		}
	}

	// Extract holdings from metadata
	portfolio := signals.PortfolioState{
		Meta: signals.PortfolioMeta{
			Name:         portfolioTag,
			AsOf:         time.Now(),
			BaseCurrency: "AUD",
		},
	}

	if doc.Metadata != nil {
		// Try to unmarshal holdings from metadata
		if holdingsData, ok := doc.Metadata["holdings"].([]interface{}); ok {
			for _, h := range holdingsData {
				if hMap, ok := h.(map[string]interface{}); ok {
					holding := signals.Holding{
						Ticker:      getString(hMap, "ticker"),
						Name:        getString(hMap, "name"),
						Sector:      getString(hMap, "sector"),
						HoldingType: getString(hMap, "holding_type"),
						Units:       MapGetFloat64(hMap, "units"),
						AvgPrice:    MapGetFloat64(hMap, "avg_price"),
					}
					holding.ComputeCostBasis()
					portfolio.Holdings = append(portfolio.Holdings, holding)
				}
			}
		}
	}

	portfolio.ComputeAggregations()
	return portfolio, nil
}

// loadSignals loads signal documents for portfolio holdings
func (w *PortfolioWorker) loadSignals(ctx context.Context, portfolio signals.PortfolioState, tagPrefix string) map[string]signals.TickerSignals {
	result := make(map[string]signals.TickerSignals)

	for _, holding := range portfolio.Holdings {
		sourceType := "signal_computer"
		sourceID := fmt.Sprintf("signals:%s", holding.Ticker)

		doc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err != nil {
			w.logger.Debug().Str("ticker", holding.Ticker).Msg("No signal document found")
			continue
		}

		if doc.Metadata != nil {
			// Try to extract full signals struct
			if signalsData, ok := doc.Metadata["signals"]; ok {
				// Convert to JSON and back to get proper type
				jsonBytes, err := json.Marshal(signalsData)
				if err == nil {
					var ts signals.TickerSignals
					if json.Unmarshal(jsonBytes, &ts) == nil {
						result[holding.Ticker] = ts
						continue
					}
				}
			}

			// Fallback: construct minimal signals from metadata
			ts := signals.TickerSignals{
				Ticker: holding.Ticker,
				Price: signals.PriceSignals{
					Current: MapGetFloat64(doc.Metadata, "current_price"),
				},
				Regime: signals.RegimeSignal{
					Classification: getString(doc.Metadata, "regime"),
				},
			}
			if ts.Price.Current == 0 {
				// Try from nested signals
				if pbaScore, ok := doc.Metadata["pbas_score"].(float64); ok {
					ts.PBAS.Score = pbaScore
				}
			}
			result[holding.Ticker] = ts
		}
	}

	return result
}

// computeRollup calculates portfolio-level metrics
func (w *PortfolioWorker) computeRollup(portfolio signals.PortfolioState, tickerSignals map[string]signals.TickerSignals) signals.PortfolioRollup {
	rollup := signals.PortfolioRollup{
		Meta: signals.RollupMeta{
			AsOf:             time.Now(),
			HoldingsAssessed: len(tickerSignals),
		},
	}

	// Calculate performance
	var totalValue, totalCost float64
	for _, holding := range portfolio.Holdings {
		if sig, ok := tickerSignals[holding.Ticker]; ok {
			currentValue := holding.Units * sig.Price.Current
			totalValue += currentValue
			totalCost += holding.CostBasis
		}
	}

	if totalCost > 0 {
		rollup.Performance = signals.PerformanceMetrics{
			TotalValue:  totalValue,
			TotalCost:   totalCost,
			TotalPnL:    totalValue - totalCost,
			TotalPnLPct: (totalValue - totalCost) / totalCost * 100,
		}
	}

	// Calculate allocations
	rollup.Allocation = w.computeAllocation(portfolio, tickerSignals, totalValue)

	// Check concentration
	rollup.ConcentrationAlerts = w.checkConcentration(portfolio, tickerSignals, totalValue)

	return rollup
}

// computeAllocation calculates sector and regime allocations
func (w *PortfolioWorker) computeAllocation(portfolio signals.PortfolioState, tickerSignals map[string]signals.TickerSignals, totalValue float64) signals.AllocationMetrics {
	alloc := signals.AllocationMetrics{
		BySector:      make(map[string]float64),
		ByHoldingType: make(map[string]float64),
		ByRegime:      make(map[string]float64),
	}

	if totalValue == 0 {
		return alloc
	}

	for _, holding := range portfolio.Holdings {
		sig, ok := tickerSignals[holding.Ticker]
		if !ok {
			continue
		}

		currentValue := holding.Units * sig.Price.Current
		weight := currentValue / totalValue * 100

		// By sector
		sector := holding.Sector
		if sector == "" {
			sector = "Unknown"
		}
		alloc.BySector[sector] += weight

		// By holding type
		holdingType := holding.HoldingType
		if holdingType == "" {
			holdingType = "unknown"
		}
		alloc.ByHoldingType[holdingType] += weight

		// By regime
		regime := sig.Regime.Classification
		if regime == "" {
			regime = "undefined"
		}
		alloc.ByRegime[regime] += weight
	}

	return alloc
}

// checkConcentration checks for concentration limit violations
func (w *PortfolioWorker) checkConcentration(portfolio signals.PortfolioState, tickerSignals map[string]signals.TickerSignals, totalValue float64) []string {
	alerts := []string{}

	if totalValue == 0 {
		return alerts
	}

	// Calculate position weights
	type positionWeight struct {
		ticker string
		weight float64
	}
	positions := make([]positionWeight, 0)

	for _, holding := range portfolio.Holdings {
		if sig, ok := tickerSignals[holding.Ticker]; ok {
			currentValue := holding.Units * sig.Price.Current
			weight := currentValue / totalValue * 100
			positions = append(positions, positionWeight{
				ticker: holding.Ticker,
				weight: weight,
			})
		}
	}

	// Sort by weight descending
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].weight > positions[j].weight
	})

	// Check largest position
	if len(positions) > 0 && positions[0].weight > signals.MaxPositionPct {
		alerts = append(alerts, fmt.Sprintf("Largest position %s at %.1f%% (limit: %.0f%%)",
			positions[0].ticker, positions[0].weight, signals.MaxPositionPct))
	}

	// Check top 5
	top5Weight := 0.0
	for i := 0; i < 5 && i < len(positions); i++ {
		top5Weight += positions[i].weight
	}
	if top5Weight > signals.MaxTop5Pct {
		alerts = append(alerts, fmt.Sprintf("Top 5 positions at %.1f%% (limit: %.0f%%)",
			top5Weight, signals.MaxTop5Pct))
	}

	// Check sector concentration
	sectorWeights := make(map[string]float64)
	for _, holding := range portfolio.Holdings {
		if sig, ok := tickerSignals[holding.Ticker]; ok {
			currentValue := holding.Units * sig.Price.Current
			weight := currentValue / totalValue * 100
			sectorWeights[holding.Sector] += weight
		}
	}

	for sector, weight := range sectorWeights {
		if weight > signals.MaxSectorPct {
			alerts = append(alerts, fmt.Sprintf("Sector %s at %.1f%% (limit: %.0f%%)",
				sector, weight, signals.MaxSectorPct))
		}
	}

	return alerts
}

// generateMarkdown creates markdown content from rollup
func (w *PortfolioWorker) generateMarkdown(portfolioName string, rollup signals.PortfolioRollup) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Portfolio Rollup: %s\n\n", portfolioName))
	sb.WriteString(fmt.Sprintf("**As Of**: %s\n", rollup.Meta.AsOf.Format("2 January 2006 3:04 PM")))
	sb.WriteString(fmt.Sprintf("**Holdings Assessed**: %d\n\n", rollup.Meta.HoldingsAssessed))

	// Performance Section
	sb.WriteString("## Performance\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total Value | $%.2f |\n", rollup.Performance.TotalValue))
	sb.WriteString(fmt.Sprintf("| Total Cost | $%.2f |\n", rollup.Performance.TotalCost))
	pnlSign := ""
	if rollup.Performance.TotalPnL >= 0 {
		pnlSign = "+"
	}
	sb.WriteString(fmt.Sprintf("| Total P&L | %s$%.2f (%s%.1f%%) |\n\n",
		pnlSign, rollup.Performance.TotalPnL, pnlSign, rollup.Performance.TotalPnLPct))

	// Allocation by Sector
	if len(rollup.Allocation.BySector) > 0 {
		sb.WriteString("## Allocation by Sector\n\n")
		sb.WriteString("| Sector | Weight |\n")
		sb.WriteString("|--------|--------|\n")
		for sector, weight := range rollup.Allocation.BySector {
			sb.WriteString(fmt.Sprintf("| %s | %.1f%% |\n", sector, weight))
		}
		sb.WriteString("\n")
	}

	// Allocation by Regime
	if len(rollup.Allocation.ByRegime) > 0 {
		sb.WriteString("## Allocation by Regime\n\n")
		sb.WriteString("| Regime | Weight |\n")
		sb.WriteString("|--------|--------|\n")
		for regime, weight := range rollup.Allocation.ByRegime {
			sb.WriteString(fmt.Sprintf("| %s | %.1f%% |\n", regime, weight))
		}
		sb.WriteString("\n")
	}

	// Concentration Alerts
	sb.WriteString("## Concentration Alerts\n\n")
	if len(rollup.ConcentrationAlerts) == 0 {
		sb.WriteString("No concentration alerts.\n\n")
	} else {
		for _, alert := range rollup.ConcentrationAlerts {
			sb.WriteString(fmt.Sprintf("- %s\n", alert))
		}
		sb.WriteString("\n")
	}

	// Action Summary
	sb.WriteString("## Action Summary\n\n")
	sb.WriteString(fmt.Sprintf("- Immediate Actions: %d\n", rollup.ActionSummary.ImmediateActions))
	sb.WriteString(fmt.Sprintf("- Watch Closely: %d\n", rollup.ActionSummary.WatchClosely))
	sb.WriteString(fmt.Sprintf("- Hold: %d\n", rollup.ActionSummary.HoldNoAction))

	return sb.String()
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
