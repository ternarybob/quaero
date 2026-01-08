// -----------------------------------------------------------------------
// DataWorker - Fetches market price data for any exchange
// Uses EODHD API for price/volume history and calculates technical indicators.
// Supports any exchange via exchange-qualified tickers (ASX:GNP, NYSE:AAPL).
// -----------------------------------------------------------------------

package market

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// DataWorker fetches market price data and calculates technical indicators.
// Supports any exchange via the EODHD API.
type DataWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*DataWorker)(nil)

// MarketData holds all fetched and calculated market data
type Data struct {
	// Identification
	Ticker      common.Ticker
	Symbol      string
	DisplayName string
	Currency    string

	// Current price data
	LastPrice     float64
	PriceChange   float64
	ChangePercent float64
	DayLow        float64
	DayHigh       float64
	Volume        int64
	AvgVolume     int64

	// 52-week range
	Week52Low  float64
	Week52High float64

	// Historical data
	HistoricalPrices []OHLCV

	// Technical indicators
	SMA20       float64
	SMA50       float64
	SMA200      float64
	RSI14       float64
	Support     float64
	Resistance  float64
	TrendSignal string // "BULLISH", "BEARISH", "NEUTRAL"

	// Timestamps
	LastUpdated time.Time
}

// NewDataWorker creates a new market data worker
func NewDataWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *DataWorker {
	return &DataWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeMarketData
func (w *DataWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketData
}

// Init initializes the market data worker
// Supports both step config and job-level variables for ticker configuration
func (w *DataWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Collect tickers - supports both step config and job-level variables
	tickers := collectTickersWithJobDef(stepConfig, jobDef)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("ticker, asx_code, tickers, or asx_codes is required in step config or job variables")
	}

	// Period for historical data (default Y1 = 12 months)
	period := "Y1"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Str("period", period).
		Msg("Market data worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   ticker.String(),
			Name: fmt.Sprintf("Fetch %s market data", ticker.String()),
			Type: "market_data",
			Config: map[string]interface{}{
				"ticker": ticker.String(),
				"period": period,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(tickers),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"tickers":     tickers,
			"period":      period,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs fetches market data and stores as document
// Supports multiple tickers - processes each sequentially
func (w *DataWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize market_data worker: %w", err)
		}
	}

	// Get tickers from metadata
	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	period, _ := initResult.Metadata["period"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Check cache settings
	cacheHours := 24
	if ch, ok := stepConfig["cache_hours"].(float64); ok {
		cacheHours = int(ch)
	}
	forceRefresh := false
	if fr, ok := stepConfig["force_refresh"].(bool); ok {
		forceRefresh = fr
	}

	// Extract output_tags
	var outputTags []string
	if stepConfig != nil {
		if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					outputTags = append(outputTags, tagStr)
				}
			}
		} else if tags, ok := stepConfig["output_tags"].([]string); ok {
			outputTags = tags
		}
	}

	// Log overall step start
	tickerCount := len(tickers)
	if w.jobMgr != nil {
		tickerStrs := make([]string, tickerCount)
		for i, t := range tickers {
			tickerStrs[i] = t.String()
		}
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Processing %d tickers: %s", tickerCount, strings.Join(tickerStrs, ", ")))
	}

	// Process each ticker sequentially
	var lastErr error
	successCount := 0
	for _, ticker := range tickers {
		err := w.processTicker(ctx, ticker, period, cacheHours, forceRefresh, &jobDef, stepID, outputTags)
		if err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to process ticker")
			lastErr = err
			// Continue with next ticker (on_error = "continue" behavior)
			continue
		}
		successCount++
	}

	// Log overall completion
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Completed %d/%d tickers successfully", successCount, tickerCount))
	}

	// Return error only if ALL tickers failed
	if successCount == 0 && lastErr != nil {
		return "", fmt.Errorf("all tickers failed, last error: %w", lastErr)
	}

	return stepID, nil
}

// processTicker processes a single ticker and saves the document
func (w *DataWorker) processTicker(ctx context.Context, ticker common.Ticker, period string, cacheHours int, forceRefresh bool, jobDef *models.JobDefinition, stepID string, outputTags []string) error {
	// Build source identifiers
	sourceType := "market_data"
	sourceID := ticker.SourceID("market_data")

	// Check for cached data before fetching
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && existingDoc != nil && existingDoc.LastSynced != nil {
			if time.Since(*existingDoc.LastSynced) < time.Duration(cacheHours)*time.Hour {
				w.logger.Info().
					Str("ticker", ticker.String()).
					Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
					Int("cache_hours", cacheHours).
					Msg("Using cached market data")
				if w.jobMgr != nil {
					w.jobMgr.AddJobLog(ctx, stepID, "info",
						fmt.Sprintf("%s - Using cached data (last synced: %s)",
							ticker.String(), existingDoc.LastSynced.Format("2006-01-02 15:04")))
				}
				return nil
			}
		}
	}

	w.logger.Info().
		Str("phase", "run").
		Str("ticker", ticker.String()).
		Str("period", period).
		Msg("Fetching market data via EODHD")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching %s market data and calculating technicals", ticker.String()))
	}

	// Fetch data from EODHD
	marketData, err := w.fetchMarketData(ctx, ticker, period)
	if err != nil {
		w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to fetch market data")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch market data: %v", err))
		}
		return fmt.Errorf("failed to fetch market data: %w", err)
	}

	// Calculate technical indicators
	w.calculateMarketTechnicals(marketData)

	// Create and save document
	doc := w.createMarketDocument(ctx, marketData, jobDef, stepID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to save market data document")
		return fmt.Errorf("failed to save market data: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Float64("price", marketData.LastPrice).
		Str("trend", marketData.TrendSignal).
		Msg("Market data processed")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("%s - Price: $%.2f, Trend: %s, SMA20: $%.2f",
				ticker.String(), marketData.LastPrice, marketData.TrendSignal, marketData.SMA20))
	}

	return nil
}

// ReturnsChildJobs returns false
func (w *DataWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
// Config can be nil if tickers will be provided via job-level variables.
func (w *DataWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - tickers can come from job-level variables
	// Full validation happens in Init() when we have access to jobDef
	return nil
}

// getEODHDAPIKey retrieves the EODHD API key from KV storage
func (w *DataWorker) getEODHDAPIKey(ctx context.Context) string {
	if w.kvStorage == nil {
		w.logger.Warn().Msg("EODHD API key lookup failed: kvStorage is nil")
		return ""
	}
	apiKey, err := common.ResolveAPIKey(ctx, w.kvStorage, "eodhd_api_key", "")
	if err != nil {
		w.logger.Warn().Err(err).Str("key_name", "eodhd_api_key").Msg("Failed to resolve EODHD API key")
		return ""
	}
	if apiKey == "" {
		w.logger.Warn().Str("key_name", "eodhd_api_key").Msg("EODHD API key is empty")
	}
	return apiKey
}

// fetchMarketData fetches EOD data from EODHD API
func (w *DataWorker) fetchMarketData(ctx context.Context, ticker common.Ticker, period string) (*Data, error) {
	// Get API key from KV store
	apiKey := w.getEODHDAPIKey(ctx)
	if apiKey == "" {
		return nil, fmt.Errorf("EODHD API key 'eodhd_api_key' not configured in KV store")
	}

	// Create EODHD client with resolved API key
	eodhdClient := eodhd.NewClient(apiKey, eodhd.WithLogger(w.logger))

	marketData := &Data{
		Ticker:      ticker,
		Symbol:      ticker.Code,
		DisplayName: ticker.String(),
		LastUpdated: time.Now(),
	}

	// Calculate date range from period
	to := time.Now()
	var from time.Time
	switch period {
	case "M1":
		from = to.AddDate(0, -1, 0)
	case "M3":
		from = to.AddDate(0, -3, 0)
	case "M6":
		from = to.AddDate(0, -6, 0)
	case "Y1":
		from = to.AddDate(-1, 0, 0)
	case "Y2":
		from = to.AddDate(-2, 0, 0)
	case "Y5":
		from = to.AddDate(-5, 0, 0)
	default:
		from = to.AddDate(-1, 0, 0) // Default 1 year
	}

	// Fetch EOD data
	symbol := ticker.EODHDSymbol()
	eodData, err := eodhdClient.GetEOD(ctx, symbol,
		eodhd.WithDateRange(from, to),
		eodhd.WithOrder("a"), // Ascending order
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EOD data: %w", err)
	}

	if len(eodData) == 0 {
		return nil, fmt.Errorf("no EOD data returned for %s", symbol)
	}

	// Convert to OHLCV
	for _, d := range eodData {
		marketData.HistoricalPrices = append(marketData.HistoricalPrices, OHLCV{
			Date:   d.Date,
			Open:   d.Open,
			High:   d.High,
			Low:    d.Low,
			Close:  d.Close,
			Volume: d.Volume,
		})
	}

	// Sort by date ascending
	sort.Slice(marketData.HistoricalPrices, func(i, j int) bool {
		return marketData.HistoricalPrices[i].Date.Before(marketData.HistoricalPrices[j].Date)
	})

	// Set current price from last EOD
	if len(marketData.HistoricalPrices) > 0 {
		last := marketData.HistoricalPrices[len(marketData.HistoricalPrices)-1]
		marketData.LastPrice = last.Close
		marketData.DayHigh = last.High
		marketData.DayLow = last.Low
		marketData.Volume = last.Volume

		// Calculate change from previous day
		if len(marketData.HistoricalPrices) > 1 {
			prev := marketData.HistoricalPrices[len(marketData.HistoricalPrices)-2]
			marketData.PriceChange = last.Close - prev.Close
			if prev.Close > 0 {
				marketData.ChangePercent = (marketData.PriceChange / prev.Close) * 100
			}
		}
	}

	// Calculate 52-week high/low
	var week52Prices []OHLCV
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	for _, p := range marketData.HistoricalPrices {
		if p.Date.After(oneYearAgo) {
			week52Prices = append(week52Prices, p)
		}
	}
	if len(week52Prices) > 0 {
		marketData.Week52High = week52Prices[0].High
		marketData.Week52Low = week52Prices[0].Low
		for _, p := range week52Prices {
			if p.High > marketData.Week52High {
				marketData.Week52High = p.High
			}
			if p.Low < marketData.Week52Low && p.Low > 0 {
				marketData.Week52Low = p.Low
			}
		}
	}

	// Calculate average volume (last 20 days)
	if len(marketData.HistoricalPrices) > 0 {
		volumeCount := 20
		if len(marketData.HistoricalPrices) < volumeCount {
			volumeCount = len(marketData.HistoricalPrices)
		}
		var totalVolume int64
		for i := len(marketData.HistoricalPrices) - volumeCount; i < len(marketData.HistoricalPrices); i++ {
			totalVolume += marketData.HistoricalPrices[i].Volume
		}
		marketData.AvgVolume = totalVolume / int64(volumeCount)
	}

	return marketData, nil
}

// calculateMarketTechnicals calculates technical indicators
func (w *DataWorker) calculateMarketTechnicals(data *Data) {
	prices := data.HistoricalPrices
	if len(prices) == 0 {
		return
	}

	// Get closing prices
	closes := make([]float64, len(prices))
	for i, p := range prices {
		closes[i] = p.Close
	}

	// Calculate SMAs
	data.SMA20 = w.calculateSMA(closes, 20)
	data.SMA50 = w.calculateSMA(closes, 50)
	data.SMA200 = w.calculateSMA(closes, 200)

	// Calculate RSI
	data.RSI14 = w.calculateRSI(closes, 14)

	// Calculate support and resistance (from last 20 days)
	recentPrices := prices
	if len(prices) > 20 {
		recentPrices = prices[len(prices)-20:]
	}

	var highs, lows []float64
	for _, p := range recentPrices {
		highs = append(highs, p.High)
		lows = append(lows, p.Low)
	}

	if len(lows) > 0 {
		data.Support = w.findMin(lows)
	}
	if len(highs) > 0 {
		data.Resistance = w.findMax(highs)
	}

	// Determine trend signal
	currentPrice := data.LastPrice
	if currentPrice == 0 && len(closes) > 0 {
		currentPrice = closes[len(closes)-1]
	}

	data.TrendSignal = w.determineTrend(currentPrice, data.SMA20, data.SMA50, data.SMA200, data.RSI14)
}

// calculateSMA calculates Simple Moving Average
func (w *DataWorker) calculateSMA(prices []float64, period int) float64 {
	if len(prices) < period {
		if len(prices) == 0 {
			return 0
		}
		period = len(prices)
	}

	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}
	return sum / float64(period)
}

// calculateRSI calculates Relative Strength Index
func (w *DataWorker) calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50 // Neutral if not enough data
	}

	gains := 0.0
	losses := 0.0

	for i := len(prices) - period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	if losses == 0 {
		return 100
	}

	rs := gains / losses
	return 100 - (100 / (1 + rs))
}

// findMin finds minimum positive value
func (w *DataWorker) findMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := math.MaxFloat64
	for _, v := range values {
		if v < min && v > 0 {
			min = v
		}
	}
	if min == math.MaxFloat64 {
		return 0
	}
	return min
}

// findMax finds maximum value
func (w *DataWorker) findMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

// determineTrend determines the overall trend signal
func (w *DataWorker) determineTrend(price, sma20, sma50, sma200, rsi float64) string {
	bullishSignals := 0
	bearishSignals := 0

	// Price vs SMAs
	if price > sma20 && sma20 > 0 {
		bullishSignals++
	} else if sma20 > 0 {
		bearishSignals++
	}

	if price > sma50 && sma50 > 0 {
		bullishSignals++
	} else if sma50 > 0 {
		bearishSignals++
	}

	if price > sma200 && sma200 > 0 {
		bullishSignals++
	} else if sma200 > 0 {
		bearishSignals++
	}

	// SMA alignment (golden/death cross)
	if sma20 > sma50 && sma50 > 0 {
		bullishSignals++
	} else if sma50 > 0 {
		bearishSignals++
	}

	// RSI
	if rsi > 50 && rsi < 70 {
		bullishSignals++
	} else if rsi < 50 && rsi > 30 {
		bearishSignals++
	} else if rsi >= 70 {
		bearishSignals++ // Overbought
	} else if rsi <= 30 {
		bullishSignals++ // Oversold - potential reversal
	}

	if bullishSignals > bearishSignals+1 {
		return "BULLISH"
	} else if bearishSignals > bullishSignals+1 {
		return "BEARISH"
	}
	return "NEUTRAL"
}

// createMarketDocument creates a document from market data
func (w *DataWorker) createMarketDocument(ctx context.Context, data *Data, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s Market Data\n\n", data.Ticker.String()))
	content.WriteString(fmt.Sprintf("**Last Updated**: %s\n\n", data.LastUpdated.Format("2 Jan 2006 3:04 PM")))

	// Current Price Section
	content.WriteString("## Current Price\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| **Last Price** | **$%.2f** |\n", data.LastPrice))
	content.WriteString(fmt.Sprintf("| Change | $%.2f (%.2f%%) |\n", data.PriceChange, data.ChangePercent))
	content.WriteString(fmt.Sprintf("| Day Range | $%.2f - $%.2f |\n", data.DayLow, data.DayHigh))
	content.WriteString(fmt.Sprintf("| 52-Week Range | $%.2f - $%.2f |\n", data.Week52Low, data.Week52High))
	content.WriteString(fmt.Sprintf("| Volume | %d |\n", data.Volume))
	content.WriteString(fmt.Sprintf("| Avg Volume | %d |\n\n", data.AvgVolume))

	// Technical Indicators Section
	content.WriteString("## Technical Indicators\n\n")
	content.WriteString("| Indicator | Value |\n")
	content.WriteString("|-----------|-------|\n")
	content.WriteString(fmt.Sprintf("| SMA(20) | $%.2f |\n", data.SMA20))
	content.WriteString(fmt.Sprintf("| SMA(50) | $%.2f |\n", data.SMA50))
	content.WriteString(fmt.Sprintf("| SMA(200) | $%.2f |\n", data.SMA200))
	content.WriteString(fmt.Sprintf("| RSI(14) | %.1f |\n", data.RSI14))
	content.WriteString(fmt.Sprintf("| Support | $%.2f |\n", data.Support))
	content.WriteString(fmt.Sprintf("| Resistance | $%.2f |\n", data.Resistance))
	content.WriteString(fmt.Sprintf("| **Trend Signal** | **%s** |\n\n", data.TrendSignal))

	// Price summary for LLM extraction
	content.WriteString(fmt.Sprintf("**Summary**: %s is trading at $%.2f with a %s trend. ", data.Ticker.String(), data.LastPrice, data.TrendSignal))
	if data.RSI14 >= 70 {
		content.WriteString("RSI indicates overbought conditions. ")
	} else if data.RSI14 <= 30 {
		content.WriteString("RSI indicates oversold conditions. ")
	}
	if data.LastPrice > data.SMA50 && data.SMA50 > 0 {
		content.WriteString("Price is above the 50-day moving average.")
	} else if data.SMA50 > 0 {
		content.WriteString("Price is below the 50-day moving average.")
	}
	content.WriteString("\n")

	// Build tags
	tags := []string{
		data.Ticker.String(),
		strings.ToLower(data.Ticker.Exchange),
		data.Ticker.Code,
		"market-data",
		strings.ToLower(data.TrendSignal),
	}
	tags = append(tags, outputTags...)

	now := time.Now()
	sourceType := "market_data"
	sourceID := data.Ticker.SourceID("market_data")

	return &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("%s Market Data", data.Ticker.String()),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      sourceType,
		SourceID:        sourceID,
		URL:             "",
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":         data.Ticker.String(),
			"exchange":       data.Ticker.Exchange,
			"code":           data.Ticker.Code,
			"last_price":     data.LastPrice,
			"change_percent": data.ChangePercent,
			"trend_signal":   data.TrendSignal,
			"sma20":          data.SMA20,
			"sma50":          data.SMA50,
			"sma200":         data.SMA200,
			"rsi14":          data.RSI14,
			"support":        data.Support,
			"resistance":     data.Resistance,
			"week52_high":    data.Week52High,
			"week52_low":     data.Week52Low,
			"job_id":         parentJobID,
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}
}
