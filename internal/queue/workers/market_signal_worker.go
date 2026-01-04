// -----------------------------------------------------------------------
// MarketSignalWorker - Computes signals from stock collector data
// Reads asx_stock_collector documents and produces ticker-signals documents
// -----------------------------------------------------------------------

package workers

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
	"github.com/ternarybob/quaero/internal/signals"
)

// MarketSignalWorker computes signals from existing stock data documents.
type MarketSignalWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	signalComputer  *signals.SignalComputer
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*MarketSignalWorker)(nil)

// NewMarketSignalWorker creates a new signal computer worker
func NewMarketSignalWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *MarketSignalWorker {
	return &MarketSignalWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		signalComputer:  signals.NewSignalComputer(),
	}
}

// GetType returns WorkerTypeMarketSignal
func (w *MarketSignalWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketSignal
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *MarketSignalWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *MarketSignalWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("signal_computer step requires config")
	}
	// Must have either ticker/asx_code (single) or tickers/asx_codes (multiple)
	tickers := w.collectTickers(step.Config)
	if len(tickers) == 0 {
		return fmt.Errorf("signal_computer step requires 'ticker', 'asx_code', 'tickers', or 'asx_codes' in config")
	}
	return nil
}

// Init initializes the signal computer worker
func (w *MarketSignalWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Collect tickers to process (supports both exchange-qualified and legacy formats)
	tickers := w.collectTickers(stepConfig)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers specified in config")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Msg("Signal computer worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:     fmt.Sprintf("signals-%s", ticker.Code),
			Name:   fmt.Sprintf("Compute signals for %s", ticker.String()),
			Type:   "signal_compute",
			Config: stepConfig,
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(tickers),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"tickers":     tickers,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs computes signals for each ticker and stores results
func (w *MarketSignalWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize signal_computer worker: %w", err)
		}
	}

	// Get tickers from metadata
	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Load benchmark returns for RS calculation
	benchmarkReturns := w.loadBenchmarkReturns(ctx)
	w.signalComputer.SetBenchmarkReturns(benchmarkReturns)

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

	processedCount := 0
	errorCount := 0
	var allDocIDs []string
	var allTags []string
	var allSourceIDs []string
	var allErrors []string
	tagsSeen := make(map[string]bool)
	byTicker := make(map[string]*interfaces.TickerResult)

	for _, ticker := range tickers {
		docInfo, err := w.processSignals(ctx, ticker, stepID, outputTags)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", ticker.String(), err)
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to compute signals")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("%s - Failed: %v", ticker.String(), err))
			}
			allErrors = append(allErrors, errMsg)
			errorCount++
			continue
		}

		// Collect document info
		if docInfo != nil {
			allDocIDs = append(allDocIDs, docInfo.ID)
			allSourceIDs = append(allSourceIDs, docInfo.SourceID)
			for _, tag := range docInfo.Tags {
				if !tagsSeen[tag] {
					tagsSeen[tag] = true
					allTags = append(allTags, tag)
				}
			}

			// Store per-ticker result
			byTicker[ticker.String()] = &interfaces.TickerResult{
				DocumentsCreated: 1,
				DocumentIDs:      []string{docInfo.ID},
				Tags:             docInfo.Tags,
			}
		}
		processedCount++
	}

	w.logger.Info().
		Int("processed", processedCount).
		Int("errors", errorCount).
		Int("documents", len(allDocIDs)).
		Msg("Signal computation complete")

	// Build WorkerResult for test validation
	workerResult := &interfaces.WorkerResult{
		DocumentsCreated: processedCount,
		DocumentIDs:      allDocIDs,
		Tags:             allTags,
		SourceType:       "market_signal",
		SourceIDs:        allSourceIDs,
		Errors:           allErrors,
		ByTicker:         byTicker,
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Signal computation complete: %d processed, %d errors", processedCount, errorCount))

		// Store WorkerResult in job metadata for test validation
		if err := w.jobMgr.UpdateJobMetadata(ctx, stepID, map[string]interface{}{
			"worker_result": workerResult.ToMap(),
		}); err != nil {
			w.logger.Warn().Err(err).Str("step_id", stepID).Msg("Failed to update job metadata with worker result")
		}
	}

	return stepID, nil
}

// signalDocInfo contains info about a created signal document
type signalDocInfo struct {
	ID       string
	SourceID string
	Tags     []string
}

// processSignals computes signals for a single ticker and returns document info
func (w *MarketSignalWorker) processSignals(ctx context.Context, ticker common.Ticker, stepID string, outputTags []string) (*signalDocInfo, error) {
	// Find source document using ticker's source ID format
	sourceType := "asx_stock_collector"
	sourceID := ticker.SourceID("stock_collector")

	w.logger.Debug().
		Str("ticker", ticker.String()).
		Str("source_type", sourceType).
		Str("source_id", sourceID).
		Msg("Looking up source document for signal computation")

	sourceDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err != nil {
		w.logger.Error().
			Err(err).
			Str("ticker", ticker.String()).
			Str("source_type", sourceType).
			Str("source_id", sourceID).
			Msg("Source document not found for signal computation")
		return nil, fmt.Errorf("source document not found for %s (sourceType=%s, sourceID=%s): %w", ticker.String(), sourceType, sourceID, err)
	}

	// Extract raw data from document metadata
	raw, err := w.extractTickerRaw(ticker.Code, sourceDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract ticker data: %w", err)
	}

	// Compute signals
	tickerSignals := w.signalComputer.ComputeSignals(raw)

	// Generate markdown content
	markdown := w.generateMarkdown(ticker.String(), tickerSignals)

	// Build tags - include both exchange-qualified and code for backwards compatibility
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"ticker-signals", strings.ToLower(ticker.Code), strings.ToLower(ticker.String()), dateTag}
	tags = append(tags, outputTags...)

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "market_signal",
		SourceID:        ticker.SourceID("signals"),
		Title:           fmt.Sprintf("Signal Analysis: %s", ticker.String()),
		ContentMarkdown: markdown,
		DetailLevel:     models.DetailLevelFull,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"ticker":              ticker.String(),
			"asx_code":            ticker.Code, // Keep for backwards compatibility
			"exchange":            ticker.Exchange,
			"computed_at":         now.Format(time.RFC3339),
			"pbas_score":          tickerSignals.PBAS.Score,
			"pbas_interpretation": tickerSignals.PBAS.Interpretation,
			"vli_score":           tickerSignals.VLI.Score,
			"vli_label":           tickerSignals.VLI.Label,
			"regime":              tickerSignals.Regime.Classification,
			"regime_confidence":   tickerSignals.Regime.Confidence,
			"is_cooked":           tickerSignals.Cooked.IsCooked,
			"cooked_score":        tickerSignals.Cooked.Score,
			"rs_rank":             tickerSignals.RS.RSRankPercentile,
			"quality_overall":     tickerSignals.Quality.Overall,
			"justified_expected":  tickerSignals.JustifiedReturn.Expected12MPct,
			"justified_actual":    tickerSignals.JustifiedReturn.Actual12MPct,
			"justified_diverge":   tickerSignals.JustifiedReturn.DivergencePct,
			"risk_flags":          tickerSignals.RiskFlags,
			"signals":             tickerSignals,
			"source_document_id":  sourceDoc.ID,
			"source_last_synced":  sourceDoc.LastSynced,
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Error().Err(err).Str("ticker", ticker.String()).Str("doc_id", doc.ID).Msg("Failed to save signals document")
		return nil, fmt.Errorf("failed to save signals document: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Str("doc_id", doc.ID).
		Float64("pbas", tickerSignals.PBAS.Score).
		Str("regime", tickerSignals.Regime.Classification).
		Bool("cooked", tickerSignals.Cooked.IsCooked).
		Int("rs_rank", tickerSignals.RS.RSRankPercentile).
		Msg("Signals computed and stored")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("%s - PBAS: %.2f (%s), Regime: %s, RS: %dth pctl",
				ticker.String(), tickerSignals.PBAS.Score, tickerSignals.PBAS.Interpretation,
				tickerSignals.Regime.Classification, tickerSignals.RS.RSRankPercentile))
	}

	return &signalDocInfo{
		ID:       doc.ID,
		SourceID: doc.SourceID,
		Tags:     doc.Tags,
	}, nil
}

// collectTickers extracts tickers from config, supporting both legacy and exchange-qualified formats
func (w *MarketSignalWorker) collectTickers(config map[string]interface{}) []common.Ticker {
	var tickers []common.Ticker

	// Try "ticker" first (new format), then "asx_code" (legacy)
	if ticker, ok := config["ticker"].(string); ok && ticker != "" {
		tickers = append(tickers, common.ParseTicker(ticker))
	} else if code, ok := config["asx_code"].(string); ok && code != "" {
		tickers = append(tickers, common.ParseTicker(code))
	}

	// Multiple tickers - try "tickers" first, then "asx_codes"
	if tickerList, ok := config["tickers"].([]interface{}); ok {
		for _, t := range tickerList {
			if ticker, ok := t.(string); ok && ticker != "" {
				tickers = append(tickers, common.ParseTicker(ticker))
			}
		}
	} else if codeList, ok := config["asx_codes"].([]interface{}); ok {
		for _, c := range codeList {
			if code, ok := c.(string); ok && code != "" {
				tickers = append(tickers, common.ParseTicker(code))
			}
		}
	}

	return tickers
}

// loadBenchmarkReturns loads XJO benchmark returns from index data
func (w *MarketSignalWorker) loadBenchmarkReturns(ctx context.Context) map[string]float64 {
	benchmarks := make(map[string]float64)

	// Try to load from market_data document (EODHD source)
	indexDoc, err := w.documentStorage.GetDocumentBySource("market_data", "ASX:XJO:market_data")
	if err != nil {
		w.logger.Debug().Msg("No XJO benchmark data found, using defaults")
		return benchmarks
	}

	// Extract benchmark returns from metadata
	if metadata := indexDoc.Metadata; metadata != nil {
		if ret3m, ok := metadata["return_3m"].(float64); ok {
			benchmarks["3m"] = ret3m
		}
		if ret6m, ok := metadata["return_6m"].(float64); ok {
			benchmarks["6m"] = ret6m
		}
	}

	return benchmarks
}

// extractTickerRaw converts document metadata to TickerRaw
func (w *MarketSignalWorker) extractTickerRaw(asxCode string, doc *models.Document) (signals.TickerRaw, error) {
	metadata := doc.Metadata
	if metadata == nil {
		return signals.TickerRaw{}, fmt.Errorf("document has no metadata")
	}

	raw := signals.TickerRaw{
		Ticker: asxCode,
	}

	// Extract price data
	raw.Price.Current = getFloat64(metadata, "current_price")
	raw.Price.Change1DPct = getFloat64(metadata, "change_percent")
	raw.Price.EMA20 = getFloat64(metadata, "sma20") // Using SMA as proxy for EMA
	raw.Price.EMA50 = getFloat64(metadata, "sma50")
	raw.Price.EMA200 = getFloat64(metadata, "sma200")
	raw.Price.High52W = getFloat64(metadata, "week52_high")
	raw.Price.Low52W = getFloat64(metadata, "week52_low")

	// Extract period performance for returns
	if perfData, ok := metadata["period_performance"].([]interface{}); ok {
		for _, p := range perfData {
			if perf, ok := p.(map[string]interface{}); ok {
				period, _ := perf["period"].(string)
				changePct, _ := perf["change_percent"].(float64)
				switch period {
				case "1W":
					raw.Price.Return1WPct = changePct
				case "4W", "1M":
					raw.Price.Return4WPct = changePct
				case "12W", "3M":
					raw.Price.Return12WPct = changePct
				case "26W", "6M":
					raw.Price.Return26WPct = changePct
				case "52W", "1Y":
					raw.Price.Return52WPct = changePct
				}
			}
		}
	}

	// Extract volume data
	raw.Volume.Current = getInt64(metadata, "volume")
	avgVol := getFloat64(metadata, "avg_volume")
	raw.Volume.SMA20 = avgVol
	// Calculate volume z-score estimate (rough approximation)
	if avgVol > 0 {
		volRatio := float64(raw.Volume.Current) / avgVol
		raw.Volume.ZScore20 = (volRatio - 1.0) * 2.0 // Rough estimate
	}

	// Extract fundamentals
	raw.Fundamentals.RevenueYoYPct = getFloat64(metadata, "revenue_growth_yoy")
	raw.Fundamentals.EBITDAMarginPct = getFloat64(metadata, "profit_margin") // Using profit margin as proxy
	raw.Fundamentals.ROEPct = getFloat64(metadata, "return_on_equity")
	raw.Fundamentals.NetDebtToEBITDA = getFloat64(metadata, "ev_to_ebitda") // Proxy
	raw.Fundamentals.CurrentRatio = 1.5                                     // Default if not available

	// Check if we have meaningful fundamental data
	if raw.Fundamentals.RevenueYoYPct != 0 || raw.Fundamentals.EBITDAMarginPct != 0 {
		raw.HasFundamentals = true
	}

	return raw, nil
}

// generateMarkdown creates markdown content from signals
func (w *MarketSignalWorker) generateMarkdown(ticker string, s signals.TickerSignals) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Signal Analysis: %s\n\n", ticker))
	sb.WriteString(fmt.Sprintf("**Computed**: %s\n\n", s.ComputeTimestamp.Format("2 January 2006 3:04 PM")))

	// PBAS Section
	sb.WriteString("## PBAS (Price-Business Alignment)\n\n")
	if s.PBAS.Description != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.PBAS.Description))
	}
	sb.WriteString(fmt.Sprintf("**Score**: %.2f (%s)\n\n", s.PBAS.Score, s.PBAS.Interpretation))
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Business Momentum | %+.2f |\n", s.PBAS.BusinessMomentum))
	sb.WriteString(fmt.Sprintf("| Price Momentum | %+.2f |\n", s.PBAS.PriceMomentum))
	sb.WriteString(fmt.Sprintf("| Divergence | %+.2f |\n\n", s.PBAS.Divergence))
	if s.PBAS.Comment != "" {
		sb.WriteString(fmt.Sprintf("> **AI Review**: %s\n\n", s.PBAS.Comment))
	}

	// VLI Section
	sb.WriteString("## VLI (Volume Lead Indicator)\n\n")
	if s.VLI.Description != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.VLI.Description))
	}
	sb.WriteString(fmt.Sprintf("**Score**: %.2f | **Label**: %s\n\n", s.VLI.Score, s.VLI.Label))
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Volume Z-Score | %.2f |\n", s.VLI.VolZScore))
	sb.WriteString(fmt.Sprintf("| Price vs VWAP | %.2f |\n\n", s.VLI.PriceVsVWAP))
	if s.VLI.Comment != "" {
		sb.WriteString(fmt.Sprintf("> **AI Review**: %s\n\n", s.VLI.Comment))
	}

	// Regime Section
	sb.WriteString("## Regime\n\n")
	if s.Regime.Description != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.Regime.Description))
	}
	sb.WriteString(fmt.Sprintf("**Classification**: %s (%.0f%% confidence)\n\n",
		s.Regime.Classification, s.Regime.Confidence*100))
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Trend Bias | %s |\n", s.Regime.TrendBias))
	sb.WriteString(fmt.Sprintf("| EMA Stack | %s |\n\n", s.Regime.EMAStack))
	if s.Regime.Comment != "" {
		sb.WriteString(fmt.Sprintf("> **AI Review**: %s\n\n", s.Regime.Comment))
	}

	// Cooked Status Section
	sb.WriteString("## Cooked Status\n\n")
	if s.Cooked.Description != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.Cooked.Description))
	}
	cookedStatus := "No"
	if s.Cooked.IsCooked {
		cookedStatus = "**YES**"
	}
	sb.WriteString(fmt.Sprintf("**Is Cooked**: %s (Score: %d/5)\n\n", cookedStatus, s.Cooked.Score))
	if s.Cooked.IsCooked && len(s.Cooked.Reasons) > 0 {
		sb.WriteString("**Triggers**:\n")
		for _, reason := range s.Cooked.Reasons {
			sb.WriteString(fmt.Sprintf("- %s\n", reason))
		}
		sb.WriteString("\n")
	}
	if s.Cooked.Comment != "" {
		sb.WriteString(fmt.Sprintf("> **AI Review**: %s\n\n", s.Cooked.Comment))
	}

	// Relative Strength Section
	sb.WriteString("## Relative Strength\n\n")
	if s.RS.Description != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.RS.Description))
	}
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| vs XJO (3M) | %.2f |\n", s.RS.VsXJO3M))
	sb.WriteString(fmt.Sprintf("| vs XJO (6M) | %.2f |\n", s.RS.VsXJO6M))
	sb.WriteString(fmt.Sprintf("| Rank Percentile | %dth |\n\n", s.RS.RSRankPercentile))
	if s.RS.Comment != "" {
		sb.WriteString(fmt.Sprintf("> **AI Review**: %s\n\n", s.RS.Comment))
	}

	// Quality Section
	sb.WriteString("## Quality Assessment\n\n")
	if s.Quality.Description != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.Quality.Description))
	}
	sb.WriteString("| Dimension | Rating |\n")
	sb.WriteString("|-----------|--------|\n")
	sb.WriteString(fmt.Sprintf("| **Overall** | **%s** |\n", strings.ToUpper(s.Quality.Overall)))
	sb.WriteString(fmt.Sprintf("| Cash Conversion | %s |\n", s.Quality.CashConversion))
	sb.WriteString(fmt.Sprintf("| Balance Sheet Risk | %s |\n", s.Quality.BalanceSheetRisk))
	sb.WriteString(fmt.Sprintf("| Margin Trend | %s |\n\n", s.Quality.MarginTrend))
	if s.Quality.Comment != "" {
		sb.WriteString(fmt.Sprintf("> **AI Review**: %s\n\n", s.Quality.Comment))
	}

	// Justified Return Section
	sb.WriteString("## Justified Return\n\n")
	if s.JustifiedReturn.Description != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.JustifiedReturn.Description))
	}
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Expected 12M | %.1f%% |\n", s.JustifiedReturn.Expected12MPct))
	sb.WriteString(fmt.Sprintf("| Actual 12M | %.1f%% |\n", s.JustifiedReturn.Actual12MPct))
	sb.WriteString(fmt.Sprintf("| Divergence | %+.1f%% |\n", s.JustifiedReturn.DivergencePct))
	sb.WriteString(fmt.Sprintf("| Interpretation | %s |\n\n", s.JustifiedReturn.Interpretation))
	if s.JustifiedReturn.Comment != "" {
		sb.WriteString(fmt.Sprintf("> **AI Review**: %s\n\n", s.JustifiedReturn.Comment))
	}

	// Risk Flags Section
	sb.WriteString("## Risk Flags\n\n")
	if s.RiskFlagsDescription != "" {
		sb.WriteString(fmt.Sprintf("*%s*\n\n", s.RiskFlagsDescription))
	}
	if len(s.RiskFlags) == 0 {
		sb.WriteString("(none)\n")
	} else {
		for _, flag := range s.RiskFlags {
			sb.WriteString(fmt.Sprintf("- %s\n", flag))
		}
	}

	return sb.String()
}

// Helper functions
func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	if v, ok := m[key].(int64); ok {
		return v
	}
	if v, ok := m[key].(int); ok {
		return int64(v)
	}
	return 0
}
