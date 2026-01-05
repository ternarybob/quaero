// -----------------------------------------------------------------------
// MarketDataCollectionWorker - Deterministic stock data collection
// Replaces LLM-based data collection with explicit API calls
// Executes MarketFundamentals, MarketAnnouncements, and MarketData workers inline
// (no child jobs - direct worker invocation for immediate document creation)
// Uses EODHD for index/benchmark data via MarketDataWorker
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// MarketDataCollectionWorker handles deterministic stock data collection.
// This worker calls ASX workers directly (inline) for data collection without LLM reasoning.
// Input sources:
// - config.variables[] - array of { ticker, portfolio? }
// - filter_tags - find tickers from tagged documents (e.g., navexa-holdings)
type MarketDataCollectionWorker struct {
	documentStorage interfaces.DocumentStorage
	searchService   interfaces.SearchService
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	// Workers for inline execution (no child jobs)
	fundamentalsWorker  *MarketFundamentalsWorker
	announcementsWorker *MarketAnnouncementsWorker
	marketDataWorker    *MarketDataWorker // For index/benchmark data via EODHD
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*MarketDataCollectionWorker)(nil)

// NewMarketDataCollectionWorker creates a new stock data collection worker
func NewMarketDataCollectionWorker(
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	eodhdClient *eodhd.Client,
	debugEnabled bool,
) *MarketDataCollectionWorker {
	// Create embedded workers for inline execution
	// Note: announcementsWorker gets nil providerFactory as AI summary is not needed in collection mode
	fundamentalsWorker := NewMarketFundamentalsWorker(documentStorage, kvStorage, logger, jobMgr, debugEnabled)
	announcementsWorker := NewMarketAnnouncementsWorker(documentStorage, logger, jobMgr, debugEnabled, nil)
	marketDataWorker := NewMarketDataWorker(documentStorage, logger, jobMgr, eodhdClient)

	return &MarketDataCollectionWorker{
		documentStorage:     documentStorage,
		searchService:       searchService,
		kvStorage:           kvStorage,
		logger:              logger,
		jobMgr:              jobMgr,
		fundamentalsWorker:  fundamentalsWorker,
		announcementsWorker: announcementsWorker,
		marketDataWorker:    marketDataWorker,
	}
}

// GetType returns WorkerTypeMarketDataCollection for the DefinitionWorker interface
func (w *MarketDataCollectionWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketDataCollection
}

// ValidateConfig validates step configuration
func (w *MarketDataCollectionWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - can use job-level variables
	// If no config, will use job definition variables
	return nil
}

// Init performs the initialization/setup phase.
// Collects tickers from all sources: step config, job config, filter documents.
func (w *MarketDataCollectionWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Collect tickers from all sources
	tickers := w.collectTickers(ctx, stepConfig, jobDef)

	if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers found in variables or filter documents")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Strs("tickers", tickers).
		Msg("Stock data collection worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   fmt.Sprintf("ticker_%d", i),
			Name: ticker,
			Type: "market_data_collection",
			Config: map[string]interface{}{
				"ticker": ticker,
			},
		}
	}

	// Extract benchmark codes (default to XJO if not specified)
	benchmarkCodes := []string{"XJO"}
	if codes, ok := stepConfig["benchmark_codes"].([]interface{}); ok {
		benchmarkCodes = nil
		for _, code := range codes {
			if s, ok := code.(string); ok && s != "" {
				benchmarkCodes = append(benchmarkCodes, strings.ToUpper(s))
			}
		}
	}

	// Extract data period (default to Y2 for 2 years)
	dataPeriod := "Y2"
	if period, ok := stepConfig["period"].(string); ok && period != "" {
		dataPeriod = period
	}

	// Extract announcement period (default to M6 for 6 months)
	announcementPeriod := "M6"
	if period, ok := stepConfig["announcement_period"].(string); ok && period != "" {
		announcementPeriod = period
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(tickers),
		Strategy:             interfaces.ProcessingStrategyParallel, // Create child jobs
		SuggestedConcurrency: 3,                                     // Parallel API calls
		Metadata: map[string]interface{}{
			"tickers":             tickers,
			"benchmark_codes":     benchmarkCodes,
			"data_period":         dataPeriod,
			"announcement_period": announcementPeriod,
			"step_config":         stepConfig,
		},
	}, nil
}

// collectTickers gathers tickers from all sources
func (w *MarketDataCollectionWorker) collectTickers(ctx context.Context, stepConfig map[string]interface{}, jobDef models.JobDefinition) []string {
	tickerSet := make(map[string]bool)

	// Source 1: Step-level variables
	w.extractTickersFromVariables(stepConfig, tickerSet)

	// Source 2: Job-level variables
	if jobDef.Config != nil {
		w.extractTickersFromVariables(jobDef.Config, tickerSet)
	}

	// Source 3: Filter documents by tags (e.g., navexa-holdings)
	if filterTags, ok := w.extractFilterTags(stepConfig); ok && len(filterTags) > 0 {
		opts := interfaces.SearchOptions{
			Tags:  filterTags,
			Limit: 100,
		}
		docs, err := w.searchService.Search(ctx, "", opts)
		if err == nil {
			for _, doc := range docs {
				tickers := w.extractTickersFromDocument(doc)
				for _, ticker := range tickers {
					tickerSet[strings.ToUpper(ticker)] = true
				}
			}
		} else {
			w.logger.Warn().Err(err).Strs("filter_tags", filterTags).Msg("Failed to search documents by filter_tags")
		}
	}

	// Convert to slice
	tickers := make([]string, 0, len(tickerSet))
	for ticker := range tickerSet {
		tickers = append(tickers, ticker)
	}

	return tickers
}

// extractTickersFromVariables extracts tickers from a config map
func (w *MarketDataCollectionWorker) extractTickersFromVariables(config map[string]interface{}, tickerSet map[string]bool) {
	vars, ok := config["variables"].([]interface{})
	if !ok {
		return
	}

	for _, v := range vars {
		varMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		// Try "ticker" key first
		if ticker, ok := varMap["ticker"].(string); ok && ticker != "" {
			// Parse ticker to handle ASX:GNP format
			parsed := common.ParseTicker(ticker)
			if parsed.Code != "" {
				tickerSet[strings.ToUpper(parsed.Code)] = true
			}
		}

		// Also try "asx_code" for backward compatibility
		if asxCode, ok := varMap["asx_code"].(string); ok && asxCode != "" {
			parsed := common.ParseTicker(asxCode)
			if parsed.Code != "" {
				tickerSet[strings.ToUpper(parsed.Code)] = true
			}
		}
	}
}

// extractFilterTags extracts filter_tags from step config
func (w *MarketDataCollectionWorker) extractFilterTags(stepConfig map[string]interface{}) ([]string, bool) {
	if tags, ok := stepConfig["filter_tags"].([]interface{}); ok {
		result := make([]string, 0, len(tags))
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				result = append(result, s)
			}
		}
		return result, len(result) > 0
	}
	if tags, ok := stepConfig["filter_tags"].([]string); ok {
		return tags, len(tags) > 0
	}
	return nil, false
}

// extractTickersFromDocument extracts tickers from a document (e.g., navexa-holdings)
func (w *MarketDataCollectionWorker) extractTickersFromDocument(doc *models.Document) []string {
	var tickers []string

	if doc.Metadata == nil {
		return tickers
	}

	// Check for holdings array in metadata (navexa-holdings format)
	if holdings, ok := doc.Metadata["holdings"].([]interface{}); ok {
		for _, h := range holdings {
			holding, ok := h.(map[string]interface{})
			if !ok {
				continue
			}

			symbol, _ := holding["symbol"].(string)
			exchange, _ := holding["exchange"].(string)

			// Only include ASX tickers
			if symbol != "" && (exchange == "ASX" || exchange == "AU" || exchange == "") {
				tickers = append(tickers, symbol)
			}
		}
	}

	// Check for tickers array in metadata
	if tickerArray, ok := doc.Metadata["tickers"].([]interface{}); ok {
		for _, t := range tickerArray {
			if ticker, ok := t.(string); ok && ticker != "" {
				tickers = append(tickers, ticker)
			}
		}
	}

	return tickers
}

// CreateJobs executes ASX workers inline (no child jobs) for data collection
func (w *MarketDataCollectionWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", err
		}
	}

	tickers, _ := initResult.Metadata["tickers"].([]string)
	benchmarkCodes, _ := initResult.Metadata["benchmark_codes"].([]string)
	dataPeriod, _ := initResult.Metadata["data_period"].(string)
	announcementPeriod, _ := initResult.Metadata["announcement_period"].(string)

	w.logger.Info().
		Str("step_id", stepID).
		Int("ticker_count", len(tickers)).
		Strs("benchmarks", benchmarkCodes).
		Str("data_period", dataPeriod).
		Str("announcement_period", announcementPeriod).
		Msg("Starting stock data collection - executing ASX workers inline")

	var totalDocs int
	var errors []string

	// Fetch benchmark index data first via EODHD (market_data worker)
	for _, code := range benchmarkCodes {
		// Use EODHD index format: XJO.INDX for ASX indices
		ticker := fmt.Sprintf("ASX:%s", code)
		indexStep := models.JobStep{
			Name:        fmt.Sprintf("market_data_%s", code),
			Type:        models.WorkerTypeMarketData,
			Description: fmt.Sprintf("Fetch index data for %s via EODHD", code),
			Config: map[string]interface{}{
				"ticker":      ticker,
				"period":      dataPeriod,
				"output_tags": []string{"stock-data-collected", "benchmark", code},
			},
		}
		_, err := w.marketDataWorker.CreateJobs(ctx, indexStep, jobDef, stepID, nil)
		if err != nil {
			w.logger.Error().Err(err).Str("code", code).Msg("Failed to fetch index data")
			errors = append(errors, fmt.Sprintf("index %s: %v", code, err))
		} else {
			totalDocs++
			w.logger.Debug().Str("code", code).Msg("Fetched index data via EODHD")
		}
	}

	// Fetch stock data and announcements for each ticker
	for _, ticker := range tickers {
		// Stock collector (inline)
		stockStep := models.JobStep{
			Name:        fmt.Sprintf("stock_collector_%s", ticker),
			Type:        models.WorkerTypeMarketFundamentals,
			Description: fmt.Sprintf("Fetch stock data for %s", ticker),
			Config: map[string]interface{}{
				"asx_code":    ticker,
				"period":      dataPeriod,
				"output_tags": []string{"stock-data-collected", ticker},
			},
		}
		_, err := w.fundamentalsWorker.CreateJobs(ctx, stockStep, jobDef, stepID, nil)
		if err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker).Msg("Failed to fetch stock data")
			errors = append(errors, fmt.Sprintf("stock %s: %v", ticker, err))
		} else {
			totalDocs++
			w.logger.Debug().Str("ticker", ticker).Msg("Fetched stock data")
		}

		// Announcements (inline)
		announcementStep := models.JobStep{
			Name:        fmt.Sprintf("announcements_%s", ticker),
			Type:        models.WorkerTypeMarketAnnouncements,
			Description: fmt.Sprintf("Fetch announcements for %s", ticker),
			Config: map[string]interface{}{
				"asx_code":    ticker,
				"period":      announcementPeriod,
				"output_tags": []string{"stock-data-collected", ticker},
			},
		}
		_, err = w.announcementsWorker.CreateJobs(ctx, announcementStep, jobDef, stepID, nil)
		if err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker).Msg("Failed to fetch announcements")
			errors = append(errors, fmt.Sprintf("announcements %s: %v", ticker, err))
		} else {
			totalDocs++
			w.logger.Debug().Str("ticker", ticker).Msg("Fetched announcements")
		}
	}

	w.logger.Info().
		Str("step_id", stepID).
		Int("documents_created", totalDocs).
		Int("tickers", len(tickers)).
		Int("benchmarks", len(benchmarkCodes)).
		Int("errors", len(errors)).
		Msg("Stock data collection completed")

	// Dead-man check #1: Verify at least one document was created
	// Expected: len(benchmarkCodes) index docs + len(tickers) stock docs + len(tickers) announcement docs
	expectedDocs := len(benchmarkCodes) + len(tickers)*2
	if totalDocs == 0 {
		errMsg := fmt.Sprintf("DEAD-MAN CHECK FAILED: No documents created. Expected %d documents (%d benchmarks + %d tickers × 2)",
			expectedDocs, len(benchmarkCodes), len(tickers))
		w.logger.Error().Msg(errMsg)
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", errMsg)
		}
		return "", fmt.Errorf("%s", errMsg)
	}

	// Dead-man check #2: Verify no worker failures
	// If any inline worker failed, the step must fail to prevent downstream LLM hallucination
	if len(errors) > 0 {
		errMsg := fmt.Sprintf("DEAD-MAN CHECK FAILED: %d of %d expected workers failed: %s",
			len(errors), expectedDocs, strings.Join(errors, "; "))
		w.logger.Error().Msg(errMsg)
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", errMsg)
		}
		return "", fmt.Errorf("%s", errMsg)
	}

	// Log event for UI (success case only)
	if w.jobMgr != nil {
		message := fmt.Sprintf("Collected data for %d tickers, %d documents created (all workers succeeded)", len(tickers), totalDocs)
		w.jobMgr.AddJobLog(ctx, stepID, "info", message)
	}

	return stepID, nil
}

// ReturnsChildJobs returns false since this worker executes ASX workers inline (no child jobs)
func (w *MarketDataCollectionWorker) ReturnsChildJobs() bool {
	return false
}
