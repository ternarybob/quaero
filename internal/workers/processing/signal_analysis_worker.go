// -----------------------------------------------------------------------
// SignalWorker - Analyzes announcement signal quality
// Consumes cached announcements and price data to produce signal analysis
// -----------------------------------------------------------------------

package processing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/exchange"
	"github.com/ternarybob/quaero/internal/workers/market"
)

// SignalWorker analyzes announcements for signal quality.
// It consumes cached market_announcements and market_fundamentals documents
// and produces signal_analysis documents with classification and scoring.
type SignalWorker struct {
	*market.BaseMarketWorker
	documentStorage interfaces.DocumentStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: SignalWorker implements DefinitionWorker
var _ interfaces.DefinitionWorker = (*SignalWorker)(nil)

// NewSignalWorker creates a new signal analysis worker.
func NewSignalWorker(
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	exchangeService *exchange.Service,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *SignalWorker {
	return &SignalWorker{
		BaseMarketWorker: market.NewBaseMarketWorker(
			documentStorage, searchService, exchangeService,
			kvStorage, logger, jobMgr, "signal_analysis",
		),
		documentStorage: documentStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeSignalAnalysis.
func (w *SignalWorker) GetType() models.WorkerType {
	return models.WorkerTypeSignalAnalysis
}

// ReturnsChildJobs returns false - this worker executes inline.
func (w *SignalWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration.
func (w *SignalWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("signal_analysis step requires config")
	}
	tickers := w.collectTickers(step.Config)
	if len(tickers) == 0 {
		return fmt.Errorf("signal_analysis requires 'ticker', 'asx_code', 'tickers', or 'asx_codes' in config")
	}
	return nil
}

// Init initializes the worker and returns work items.
func (w *SignalWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	tickers := w.collectTickers(stepConfig)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers specified in config")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Msg("Signal analysis worker initialized")

	// Create work items
	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:     fmt.Sprintf("signal-analysis-%s", ticker.Code),
			Name:   fmt.Sprintf("Analyze signals for %s", ticker.String()),
			Type:   "signal_analysis",
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

// CreateJobs executes signal analysis for each ticker.
func (w *SignalWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize signal_analysis worker: %w", err)
		}
	}

	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Extract output tags
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
	var allErrors []string
	byTicker := make(map[string]*interfaces.TickerResult)

	for _, ticker := range tickers {
		docInfo, err := w.processAnalysis(ctx, ticker, stepID, outputTags)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", ticker.String(), err)
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to analyze signals")
			if w.jobMgr != nil {
				if logErr := w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("%s - Failed: %v", ticker.String(), err)); logErr != nil {
					w.logger.Warn().Err(logErr).Msg("Failed to add job log")
				}
			}
			allErrors = append(allErrors, errMsg)
			errorCount++
			continue
		}

		if docInfo != nil {
			allDocIDs = append(allDocIDs, docInfo.ID)
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
		Msg("Signal analysis complete")

	// Build WorkerResult
	workerResult := &interfaces.WorkerResult{
		DocumentsCreated: processedCount,
		DocumentIDs:      allDocIDs,
		SourceType:       "signal_analysis",
		ByTicker:         byTicker,
		Errors:           allErrors,
	}

	// Store result in step job metadata
	if w.jobMgr != nil {
		metadataMap := map[string]interface{}{
			"worker_result": workerResult,
		}
		if err := w.jobMgr.UpdateJobMetadata(ctx, stepID, metadataMap); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to update job metadata with worker result")
		}
	}

	return stepID, nil
}

// processAnalysis performs signal analysis for a single ticker.
func (w *SignalWorker) processAnalysis(ctx context.Context, ticker common.Ticker, stepID string, outputTags []string) (*DocumentInfo, error) {
	// 1. Fetch announcements document
	announcements, announcementDocID, announcementDate, err := w.getAnnouncementsData(ctx, ticker.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get announcements: %w", err)
	}

	// 2. Fetch price data document
	prices, priceDocID, priceDate, err := w.getPriceData(ctx, ticker.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get price data: %w", err)
	}

	// 3. Build schema
	now := time.Now()
	schema := NewSignalAnalysisSchema(ticker.String())
	schema.AnalysisDate = now
	schema.DataSource = DataSourceMeta{
		AnnouncementsDocID: announcementDocID,
		EODDocID:           priceDocID,
		AnnouncementsDate:  announcementDate,
		EODDate:            priceDate,
		GeneratedAt:        now,
	}

	// 4. Classify each announcement
	var dataGaps []string
	for _, ann := range announcements {
		classification, err := w.classifyAnnouncement(ann, prices)
		if err != nil {
			dataGaps = append(dataGaps, fmt.Sprintf("Classification error for %v: %v", ann["date"], err))
			continue
		}
		schema.Classifications = append(schema.Classifications, classification)
	}
	schema.DataGaps = dataGaps

	// 5. Calculate aggregates
	schema.Summary = CalculateAggregates(schema.Classifications)
	schema.Flags = DeriveRiskFlags(schema.Summary)

	// 6. Set period bounds
	if len(schema.Classifications) > 0 {
		schema.PeriodStart = schema.Classifications[len(schema.Classifications)-1].Date
		schema.PeriodEnd = schema.Classifications[0].Date
	} else {
		schema.PeriodStart = now.AddDate(-1, 0, 0)
		schema.PeriodEnd = now
	}

	// 7. Validate schema
	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	// 8. Convert to map for metadata
	metadata, err := schema.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to map: %w", err)
	}

	// 9. Generate tags
	tags := market.GenerateMarketTags(ticker.String(), "signal_analysis", now)
	tags = append(tags, outputTags...)
	// Add style tag
	tags = append(tags, "style:"+strings.ToLower(schema.Summary.CommunicationStyle))

	// 10. Create document
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("Signal Analysis: %s", ticker.String()),
		ContentMarkdown: w.generateMarkdownSummary(schema),
		URL:             fmt.Sprintf("signal-analysis://%s/%s", ticker.String(), now.Format("2006-01-02")),
		SourceID:        fmt.Sprintf("%s:signal_analysis:%s", ticker.String(), now.Format("2006-01-02")),
		SourceType:      "signal_analysis",
		Tags:            tags,
		Metadata:        metadata,
		CreatedAt:       now,
	}

	// 11. Store document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return nil, fmt.Errorf("failed to store document: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Str("doc_id", doc.ID).
		Int("classifications", len(schema.Classifications)).
		Int("conviction_score", schema.Summary.ConvictionScore).
		Str("style", schema.Summary.CommunicationStyle).
		Msg("Signal analysis document created")

	return &DocumentInfo{
		ID:       doc.ID,
		SourceID: doc.SourceID,
		Tags:     doc.Tags,
	}, nil
}

// getAnnouncementsData fetches announcements from cached document.
func (w *SignalWorker) getAnnouncementsData(ctx context.Context, ticker string) ([]map[string]interface{}, string, time.Time, error) {
	tickerLower := strings.ToLower(ticker)

	// Search for market_announcements document
	opts := interfaces.SearchOptions{
		Tags:  []string{"ticker:" + tickerLower, "source_type:market_announcements"},
		Limit: 1,
	}
	results, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return nil, "", time.Time{}, fmt.Errorf("no announcements document found for %s", ticker)
	}

	doc := results[0]

	// Extract announcements array from metadata
	var announcements []map[string]interface{}
	if annData, ok := doc.Metadata["announcements"].([]interface{}); ok {
		for _, a := range annData {
			if annMap, ok := a.(map[string]interface{}); ok {
				announcements = append(announcements, annMap)
			}
		}
	}

	// Get document date
	docDate := doc.CreatedAt
	if dateStr, ok := doc.Metadata["date"].(string); ok {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			docDate = parsed
		}
	}

	return announcements, doc.ID, docDate, nil
}

// getPriceData fetches price history from cached document.
func (w *SignalWorker) getPriceData(ctx context.Context, ticker string) ([]market.OHLCV, string, time.Time, error) {
	tickerLower := strings.ToLower(ticker)

	// Search for market_fundamentals document (has historical_prices)
	opts := interfaces.SearchOptions{
		Tags:  []string{"ticker:" + tickerLower, "source_type:market_fundamentals"},
		Limit: 1,
	}
	results, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return nil, "", time.Time{}, fmt.Errorf("no fundamentals document found for %s", ticker)
	}

	doc := results[0]

	// Extract historical_prices from metadata
	var prices []market.OHLCV
	if priceData, ok := doc.Metadata["historical_prices"].([]interface{}); ok {
		for _, p := range priceData {
			if priceMap, ok := p.(map[string]interface{}); ok {
				ohlcv := market.OHLCV{}

				// Parse date
				if dateStr, ok := priceMap["date"].(string); ok {
					if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
						ohlcv.Date = parsed
					}
				}

				// Parse OHLCV values
				if v, ok := priceMap["open"].(float64); ok {
					ohlcv.Open = v
				}
				if v, ok := priceMap["high"].(float64); ok {
					ohlcv.High = v
				}
				if v, ok := priceMap["low"].(float64); ok {
					ohlcv.Low = v
				}
				if v, ok := priceMap["close"].(float64); ok {
					ohlcv.Close = v
				}
				if v, ok := priceMap["volume"].(float64); ok {
					ohlcv.Volume = int64(v)
				}

				if !ohlcv.Date.IsZero() {
					prices = append(prices, ohlcv)
				}
			}
		}
	}

	// Get document date
	docDate := doc.CreatedAt
	if dateStr, ok := doc.Metadata["eod_date"].(string); ok {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			docDate = parsed
		}
	}

	return prices, doc.ID, docDate, nil
}

// classifyAnnouncement classifies a single announcement.
func (w *SignalWorker) classifyAnnouncement(ann map[string]interface{}, prices []market.OHLCV) (AnnouncementClassification, error) {
	classification := AnnouncementClassification{}

	// Parse date
	if dateStr, ok := ann["date"].(string); ok {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			classification.Date = parsed
		} else if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
			classification.Date = parsed
		}
	}
	if classification.Date.IsZero() {
		return classification, fmt.Errorf("missing or invalid date")
	}

	// Extract fields
	if headline, ok := ann["headline"].(string); ok {
		classification.Title = headline
	}
	if category, ok := ann["relevance_category"].(string); ok {
		classification.Category = category
	}
	if sensitive, ok := ann["price_sensitive"].(bool); ok {
		classification.ManagementSensitive = sensitive
	}

	// Calculate metrics
	classification.Metrics = w.calculateMetrics(classification.Date, prices)

	// Classify
	classification.Classification = ClassifyAnnouncement(
		classification.Metrics,
		classification.ManagementSensitive,
		classification.Category,
	)

	return classification, nil
}

// calculateMetrics calculates classification metrics for an announcement date.
func (w *SignalWorker) calculateMetrics(annDate time.Time, prices []market.OHLCV) ClassificationMetrics {
	metrics := ClassificationMetrics{}

	// Find prices by date
	priceByDate := make(map[string]market.OHLCV)
	for _, p := range prices {
		priceByDate[p.Date.Format("2006-01-02")] = p
	}

	// Find announcement day price (or next trading day)
	annDateStr := annDate.Format("2006-01-02")
	annPrice, found := priceByDate[annDateStr]
	if !found {
		// Try next few days for weekend announcements
		for i := 1; i <= 3; i++ {
			nextDay := annDate.AddDate(0, 0, i)
			if p, ok := priceByDate[nextDay.Format("2006-01-02")]; ok {
				annPrice = p
				break
			}
		}
	}

	// Find previous day price
	var prevPrice market.OHLCV
	for i := 1; i <= 5; i++ {
		prevDay := annDate.AddDate(0, 0, -i)
		if p, ok := priceByDate[prevDay.Format("2006-01-02")]; ok {
			prevPrice = p
			break
		}
	}

	// Set raw values
	metrics.AnnouncementClose = annPrice.Close
	metrics.PreviousClose = prevPrice.Close
	metrics.DayVolume = annPrice.Volume

	// Calculate day-of change
	if prevPrice.Close > 0 {
		metrics.DayOfChange = ((annPrice.Close - prevPrice.Close) / prevPrice.Close) * 100
	}

	// Calculate 30-day average volume
	volumeSum := int64(0)
	volumeCount := 0
	for i := 1; i <= 30; i++ {
		day := annDate.AddDate(0, 0, -i)
		if p, ok := priceByDate[day.Format("2006-01-02")]; ok {
			volumeSum += p.Volume
			volumeCount++
		}
	}
	if volumeCount > 0 {
		metrics.AvgVolume30d = volumeSum / int64(volumeCount)
	}

	// Calculate volume ratio
	if metrics.AvgVolume30d > 0 {
		metrics.VolumeRatio = float64(metrics.DayVolume) / float64(metrics.AvgVolume30d)
	}

	// Calculate pre-drift (T-5 to T-1)
	var priceT5 market.OHLCV
	for i := 5; i <= 10; i++ {
		day := annDate.AddDate(0, 0, -i)
		if p, ok := priceByDate[day.Format("2006-01-02")]; ok {
			priceT5 = p
			break
		}
	}
	if priceT5.Close > 0 {
		metrics.PreDrift = ((prevPrice.Close - priceT5.Close) / priceT5.Close) * 100
	}

	// Calculate post-drift (T to T+5)
	var priceT5After market.OHLCV
	for i := 5; i <= 10; i++ {
		day := annDate.AddDate(0, 0, i)
		if p, ok := priceByDate[day.Format("2006-01-02")]; ok {
			priceT5After = p
			break
		}
	}
	if annPrice.Close > 0 && priceT5After.Close > 0 {
		metrics.PostDrift = ((priceT5After.Close - annPrice.Close) / annPrice.Close) * 100
	}

	return metrics
}

// generateMarkdownSummary creates a markdown summary of the analysis.
func (w *SignalWorker) generateMarkdownSummary(schema *SignalAnalysisSchema) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Signal Analysis: %s\n\n", schema.Ticker))
	sb.WriteString(fmt.Sprintf("**Analysis Date:** %s\n\n", schema.AnalysisDate.Format("2 January 2006")))
	sb.WriteString(fmt.Sprintf("**Period:** %s to %s\n\n", schema.PeriodStart.Format("2 Jan 2006"), schema.PeriodEnd.Format("2 Jan 2006")))

	// Summary metrics
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Conviction Score:** %d/10\n", schema.Summary.ConvictionScore))
	sb.WriteString(fmt.Sprintf("- **Communication Style:** %s\n", schema.Summary.CommunicationStyle))
	sb.WriteString(fmt.Sprintf("- **Total Announcements:** %d\n\n", schema.Summary.TotalAnnouncements))

	// Classification breakdown
	sb.WriteString("## Classification Breakdown\n\n")
	sb.WriteString(fmt.Sprintf("| Classification | Count |\n"))
	sb.WriteString(fmt.Sprintf("|---------------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| TRUE_SIGNAL | %d |\n", schema.Summary.CountTrueSignal))
	sb.WriteString(fmt.Sprintf("| PRICED_IN | %d |\n", schema.Summary.CountPricedIn))
	sb.WriteString(fmt.Sprintf("| SENTIMENT_NOISE | %d |\n", schema.Summary.CountSentimentNoise))
	sb.WriteString(fmt.Sprintf("| MANAGEMENT_BLUFF | %d |\n", schema.Summary.CountManagementBluff))
	sb.WriteString(fmt.Sprintf("| ROUTINE | %d |\n\n", schema.Summary.CountRoutine))

	// Metrics
	sb.WriteString("## Metrics\n\n")
	sb.WriteString(fmt.Sprintf("- **Signal Ratio:** %.1f%%\n", schema.Summary.SignalRatio*100))
	sb.WriteString(fmt.Sprintf("- **Leak Score:** %.1f%%\n", schema.Summary.LeakScore*100))
	sb.WriteString(fmt.Sprintf("- **Credibility Score:** %.1f%%\n", schema.Summary.CredibilityScore*100))
	sb.WriteString(fmt.Sprintf("- **Noise Ratio:** %.1f%%\n\n", schema.Summary.NoiseRatio*100))

	// Risk flags
	sb.WriteString("## Risk Flags\n\n")
	if schema.Flags.HighLeakRisk {
		sb.WriteString("- ⚠️ **High Leak Risk**: Significant pre-announcement price movement detected\n")
	}
	if schema.Flags.SpeculativeBase {
		sb.WriteString("- ⚠️ **Speculative Base**: High noise ratio indicates retail speculation\n")
	}
	if schema.Flags.ReliableSignals {
		sb.WriteString("- ✅ **Reliable Signals**: Consistent market response to announcements\n")
	}
	if schema.Flags.InsufficientData {
		sb.WriteString("- ℹ️ **Insufficient Data**: Less than 5 announcements analyzed\n")
	}

	// Data gaps
	if len(schema.DataGaps) > 0 {
		sb.WriteString("\n## Data Gaps\n\n")
		for _, gap := range schema.DataGaps {
			sb.WriteString(fmt.Sprintf("- %s\n", gap))
		}
	}

	return sb.String()
}

// collectTickers extracts tickers from config.
func (w *SignalWorker) collectTickers(config map[string]interface{}) []common.Ticker {
	var tickers []common.Ticker

	// Single ticker
	if ticker, ok := config["ticker"].(string); ok && ticker != "" {
		tickers = append(tickers, common.ParseTicker(ticker))
	} else if code, ok := config["asx_code"].(string); ok && code != "" {
		tickers = append(tickers, common.ParseTicker(code))
	}

	// Multiple tickers
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

// DocumentInfo contains information about a created document.
type DocumentInfo struct {
	ID       string
	SourceID string
	Tags     []string
}

// Suppress unused variable warning for math package
var _ = math.Abs
