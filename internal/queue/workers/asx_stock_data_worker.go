// -----------------------------------------------------------------------
// ASXStockDataWorker - Fetches real-time and historical stock data
// Uses Markit Digital API for fundamentals and Yahoo Finance for OHLCV
// Provides accurate price data and technical analysis for summaries
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ASXStockDataWorker fetches stock price data and calculates technical indicators.
// Supports both individual stocks (e.g., ROC, BHP) and market indices (e.g., XJO, XSO).
type ASXStockDataWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// knownIndices maps ASX index codes to their display names.
// These indices don't have company data in Markit API and use different Yahoo symbols.
var knownIndices = map[string]string{
	"XJO": "S&P/ASX 200",
	"XSO": "S&P/ASX Small Ordinaries",
	"XAO": "All Ordinaries",
	"XKO": "S&P/ASX 300",
	"XTO": "S&P/ASX 20",
	"XFJ": "S&P/ASX 200 Financials",
	"XMJ": "S&P/ASX 200 Materials",
	"XEJ": "S&P/ASX 200 Energy",
}

// isIndexCode returns true if the code is a known market index
func isIndexCode(code string) bool {
	_, isIndex := knownIndices[strings.ToUpper(code)]
	return isIndex
}

// getIndexName returns the display name for an index code
func getIndexName(code string) string {
	if name, ok := knownIndices[strings.ToUpper(code)]; ok {
		return name
	}
	return code
}

// getYahooSymbol converts an ASX code to Yahoo Finance symbol format.
// Indices use ^AXJO format, stocks use ROC.AX format.
func getYahooSymbol(asxCode string) string {
	code := strings.ToUpper(asxCode)
	if isIndexCode(code) {
		// Yahoo indices: XJO -> ^AXJO, XSO -> ^AXSO
		return "^AX" + strings.TrimPrefix(code, "X")
	}
	// Regular stocks: ROC -> ROC.AX
	return code + ".AX"
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*ASXStockDataWorker)(nil)

// StockData holds all fetched and calculated stock data
type StockData struct {
	// Current price data
	Symbol        string
	CompanyName   string
	LastPrice     float64
	BidPrice      float64
	AskPrice      float64
	PriceChange   float64
	ChangePercent float64
	DayLow        float64
	DayHigh       float64
	Volume        int64
	AvgVolume     int64

	// Valuation
	MarketCap     int64
	PERatio       float64
	EPS           float64
	DividendYield float64

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
	TrendSignal string // "bullish", "bearish", "neutral"

	// Timestamps
	LastUpdated time.Time
}

// OHLCV represents a single day's price data
type OHLCV struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// markitHeaderResponse for /header endpoint
type markitHeaderResponse struct {
	Data struct {
		DisplayName        string  `json:"displayName"`
		Symbol             string  `json:"symbol"`
		PriceLast          float64 `json:"priceLast"`
		PriceBid           float64 `json:"priceBid"`
		PriceAsk           float64 `json:"priceAsk"`
		PriceChange        float64 `json:"priceChange"`
		PriceChangePercent float64 `json:"priceChangePercent"`
		Volume             int64   `json:"volume"`
		MarketCap          int64   `json:"marketCap"`
	} `json:"data"`
}

// markitStatsResponse for /key-statistics endpoint
type markitStatsResponse struct {
	Data struct {
		PriceBid      float64 `json:"priceBid"`
		PriceAsk      float64 `json:"priceAsk"`
		DayLow        float64 `json:"priceDayLow"`
		DayHigh       float64 `json:"priceDayHigh"`
		Week52Low     float64 `json:"priceFiftyTwoWeekLow"`
		Week52High    float64 `json:"priceFiftyTwoWeekHigh"`
		PERatio       float64 `json:"priceEarningsRatio"`
		EPS           float64 `json:"earningsPerShare"`
		DividendYield float64 `json:"yieldAnnual"`
		AvgVolume     float64 `json:"volumeAverage"`
	} `json:"data"`
}

// yahooChartResponse for Yahoo Finance chart endpoint
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// NewASXStockDataWorker creates a new stock data worker
func NewASXStockDataWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ASXStockDataWorker {
	return &ASXStockDataWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeASXStockData
func (w *ASXStockDataWorker) GetType() models.WorkerType {
	return models.WorkerTypeASXStockData
}

// Init initializes the stock data worker
func (w *ASXStockDataWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for asx_stock_data")
	}

	asxCode, ok := stepConfig["asx_code"].(string)
	if !ok || asxCode == "" {
		return nil, fmt.Errorf("asx_code is required in step config")
	}
	asxCode = strings.ToUpper(asxCode)

	// Period for historical data (default Y2 = 24 months for comprehensive analysis)
	period := "Y2"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
		Msg("ASX stock data worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   asxCode,
				Name: fmt.Sprintf("Fetch ASX:%s stock data", asxCode),
				Type: "asx_stock_data",
				Config: map[string]interface{}{
					"asx_code": asxCode,
					"period":   period,
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"asx_code":    asxCode,
			"period":      period,
			"step_config": stepConfig,
		},
	}, nil
}

// isCacheFresh checks if a document was synced within the cache window
func (w *ASXStockDataWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// CreateJobs fetches stock data and stores as document
func (w *ASXStockDataWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize asx_stock_data worker: %w", err)
		}
	}

	asxCode, _ := initResult.Metadata["asx_code"].(string)
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

	// Check for delete_history flag (propagated from parent when job/template changes)
	// This triggers cache invalidation to ensure fresh data is collected
	deleteHistory := false
	if dh, ok := stepConfig["delete_history"].(bool); ok {
		deleteHistory = dh
	} else if dh, ok := jobDef.Config["delete_history"].(bool); ok {
		deleteHistory = dh
	}

	// Build source identifiers
	sourceType := "asx_stock_data"
	if isIndexCode(asxCode) {
		sourceType = "asx_index"
	}
	sourceID := fmt.Sprintf("asx:%s:stock_data", asxCode)
	if isIndexCode(asxCode) {
		sourceID = fmt.Sprintf("asx:%s:index_data", asxCode)
	}

	// Delete existing cached document if delete_history is set
	// This ensures stale data is removed when job/template content changes
	if deleteHistory {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && existingDoc != nil {
			if err := w.documentStorage.DeleteDocument(existingDoc.ID); err != nil {
				w.logger.Warn().Err(err).
					Str("asx_code", asxCode).
					Str("doc_id", existingDoc.ID).
					Msg("Failed to delete cached document for history cleanup")
			} else {
				w.logger.Info().
					Str("asx_code", asxCode).
					Str("doc_id", existingDoc.ID).
					Msg("Deleted cached document - job/template changed (delete_history)")
				if w.jobMgr != nil {
					w.jobMgr.AddJobLog(ctx, stepID, "info",
						fmt.Sprintf("ASX:%s - Deleted cached document (job/template changed)", asxCode))
				}
			}
		}
	}

	// Check for cached data before fetching (skip if force_refresh or delete_history)
	if !forceRefresh && !deleteHistory && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && w.isCacheFresh(existingDoc, cacheHours) {
			w.logger.Info().
				Str("asx_code", asxCode).
				Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
				Int("cache_hours", cacheHours).
				Msg("Using cached stock data")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("ASX:%s - Using cached data (last synced: %s)",
						asxCode, existingDoc.LastSynced.Format("2006-01-02 15:04")))
			}
			return stepID, nil
		}
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
		Bool("force_refresh", forceRefresh).
		Bool("delete_history", deleteHistory).
		Msg("Fetching ASX stock data")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching ASX:%s stock data and calculating technicals", asxCode))
	}

	// Fetch all data
	stockData, err := w.fetchStockData(ctx, asxCode, period)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to fetch stock data")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch stock data: %v", err))
		}
		return "", fmt.Errorf("failed to fetch stock data: %w", err)
	}

	// Calculate technical indicators
	w.calculateTechnicals(stockData)

	// Extract output_tags
	var outputTags []string
	if stepConfig != nil {
		if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					outputTags = append(outputTags, tagStr)
				}
			}
		}
	}

	// Create and save document
	doc := w.createDocument(ctx, stockData, asxCode, &jobDef, stepID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to save stock data document")
		return "", fmt.Errorf("failed to save stock data: %w", err)
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Float64("price", stockData.LastPrice).
		Str("trend", stockData.TrendSignal).
		Msg("ASX stock data processed")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("ASX:%s - Price: $%.2f, Trend: %s, SMA20: $%.2f",
				asxCode, stockData.LastPrice, stockData.TrendSignal, stockData.SMA20))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false
func (w *ASXStockDataWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *ASXStockDataWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("asx_stock_data step requires config")
	}
	asxCode, ok := step.Config["asx_code"].(string)
	if !ok || asxCode == "" {
		return fmt.Errorf("asx_stock_data step requires 'asx_code' in config")
	}
	return nil
}

// fetchStockData fetches data from multiple sources.
// For indices (XJO, XSO, etc.), only Yahoo Finance is used as Markit API doesn't support indices.
func (w *ASXStockDataWorker) fetchStockData(ctx context.Context, asxCode, period string) (*StockData, error) {
	stockData := &StockData{
		Symbol:      asxCode,
		LastUpdated: time.Now(),
	}

	// Check if this is an index code
	isIndex := isIndexCode(asxCode)

	if isIndex {
		// For indices, use predefined name and skip Markit API
		stockData.CompanyName = getIndexName(asxCode)
		w.logger.Info().
			Str("asx_code", asxCode).
			Str("index_name", stockData.CompanyName).
			Msg("Fetching index data (Markit API skipped)")
	} else {
		// Fetch from Markit Digital API (header) - stocks only
		if err := w.fetchMarkitHeader(ctx, asxCode, stockData); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to fetch Markit header data")
		}

		// Fetch from Markit Digital API (key-statistics) - stocks only
		if err := w.fetchMarkitStats(ctx, asxCode, stockData); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to fetch Markit stats data")
		}
	}

	// Fetch historical data from Yahoo Finance (works for both stocks and indices)
	if err := w.fetchYahooHistory(ctx, asxCode, period, stockData); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to fetch Yahoo historical data")
	}

	return stockData, nil
}

// fetchMarkitHeader fetches current price from Markit Digital
func (w *ASXStockDataWorker) fetchMarkitHeader(ctx context.Context, asxCode string, data *StockData) error {
	url := fmt.Sprintf("https://asx.api.markitdigital.com/asx-research/1.0/companies/%s/header",
		strings.ToLower(asxCode))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var apiResp markitHeaderResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	data.CompanyName = apiResp.Data.DisplayName
	data.LastPrice = apiResp.Data.PriceLast
	data.BidPrice = apiResp.Data.PriceBid
	data.AskPrice = apiResp.Data.PriceAsk
	data.PriceChange = apiResp.Data.PriceChange
	data.ChangePercent = apiResp.Data.PriceChangePercent
	data.Volume = apiResp.Data.Volume
	data.MarketCap = apiResp.Data.MarketCap

	return nil
}

// fetchMarkitStats fetches statistics from Markit Digital
func (w *ASXStockDataWorker) fetchMarkitStats(ctx context.Context, asxCode string, data *StockData) error {
	url := fmt.Sprintf("https://asx.api.markitdigital.com/asx-research/1.0/companies/%s/key-statistics",
		strings.ToLower(asxCode))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var apiResp markitStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	data.DayLow = apiResp.Data.DayLow
	data.DayHigh = apiResp.Data.DayHigh
	data.Week52Low = apiResp.Data.Week52Low
	data.Week52High = apiResp.Data.Week52High
	data.PERatio = apiResp.Data.PERatio
	data.EPS = apiResp.Data.EPS
	data.DividendYield = apiResp.Data.DividendYield
	data.AvgVolume = int64(apiResp.Data.AvgVolume)

	return nil
}

// fetchYahooHistory fetches historical OHLCV from Yahoo Finance.
// Uses getYahooSymbol to handle both stocks (ROC.AX) and indices (^AXJO).
func (w *ASXStockDataWorker) fetchYahooHistory(ctx context.Context, asxCode, period string, data *StockData) error {
	// Convert period to Yahoo format
	yahooRange := "1y"
	switch period {
	case "M1":
		yahooRange = "1mo"
	case "M3":
		yahooRange = "3mo"
	case "M6":
		yahooRange = "6mo"
	case "Y1":
		yahooRange = "1y"
	case "Y2":
		yahooRange = "2y"
	case "Y5":
		yahooRange = "5y"
	}

	// Get the correct Yahoo symbol (handles both stocks and indices)
	yahooSymbol := getYahooSymbol(asxCode)
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%s",
		yahooSymbol, yahooRange)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var apiResp yahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if len(apiResp.Chart.Result) == 0 || len(apiResp.Chart.Result[0].Indicators.Quote) == 0 {
		return fmt.Errorf("no data in response")
	}

	result := apiResp.Chart.Result[0]
	quote := result.Indicators.Quote[0]

	for i, ts := range result.Timestamp {
		if i >= len(quote.Close) || quote.Close[i] == 0 {
			continue
		}

		ohlcv := OHLCV{
			Date:  time.Unix(ts, 0),
			Close: quote.Close[i],
		}
		if i < len(quote.Open) {
			ohlcv.Open = quote.Open[i]
		}
		if i < len(quote.High) {
			ohlcv.High = quote.High[i]
		}
		if i < len(quote.Low) {
			ohlcv.Low = quote.Low[i]
		}
		if i < len(quote.Volume) {
			ohlcv.Volume = quote.Volume[i]
		}

		data.HistoricalPrices = append(data.HistoricalPrices, ohlcv)
	}

	// Sort by date ascending
	sort.Slice(data.HistoricalPrices, func(i, j int) bool {
		return data.HistoricalPrices[i].Date.Before(data.HistoricalPrices[j].Date)
	})

	return nil
}

// calculateTechnicals calculates technical indicators
func (w *ASXStockDataWorker) calculateTechnicals(data *StockData) {
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
	data.SMA20 = calculateSMA(closes, 20)
	data.SMA50 = calculateSMA(closes, 50)
	data.SMA200 = calculateSMA(closes, 200)

	// Calculate RSI
	data.RSI14 = calculateRSI(closes, 14)

	// Calculate support and resistance (simple pivot points)
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
		data.Support = findMin(lows)
	}
	if len(highs) > 0 {
		data.Resistance = findMax(highs)
	}

	// Determine trend signal
	currentPrice := data.LastPrice
	if currentPrice == 0 && len(closes) > 0 {
		currentPrice = closes[len(closes)-1]
	}

	data.TrendSignal = determineTrend(currentPrice, data.SMA20, data.SMA50, data.SMA200, data.RSI14)
}

// calculateSMA calculates Simple Moving Average
func calculateSMA(prices []float64, period int) float64 {
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
func calculateRSI(prices []float64, period int) float64 {
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

// findMin finds minimum value
func findMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values {
		if v < min && v > 0 {
			min = v
		}
	}
	return min
}

// findMax finds maximum value
func findMax(values []float64) float64 {
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
func determineTrend(price, sma20, sma50, sma200, rsi float64) string {
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

// createDocument creates a document from stock data.
// For indices, uses "asx_index" source type and adds index-specific tags.
func (w *ASXStockDataWorker) createDocument(ctx context.Context, data *StockData, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	var content strings.Builder

	isIndex := isIndexCode(asxCode)

	if isIndex {
		content.WriteString(fmt.Sprintf("# ASX Index: %s (%s)\n\n", data.CompanyName, asxCode))
	} else {
		content.WriteString(fmt.Sprintf("# ASX:%s Stock Data - %s\n\n", asxCode, data.CompanyName))
	}
	content.WriteString(fmt.Sprintf("**Last Updated**: %s\n\n", data.LastUpdated.Format("2 Jan 2006 3:04 PM AEST")))

	// Data Sources Section
	content.WriteString("## Data Sources\n\n")
	content.WriteString("| Source | Data Provided |\n")
	content.WriteString("|--------|---------------|\n")
	content.WriteString("| ASX Markit Digital API | Current price, bid/ask, market cap, P/E, EPS, dividend yield |\n")
	content.WriteString("| Yahoo Finance | Historical OHLCV data, technical indicators |\n\n")

	// Current Price Section
	content.WriteString("## Current Price\n\n")
	content.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	content.WriteString(fmt.Sprintf("|--------|-------|\n"))
	content.WriteString(fmt.Sprintf("| **Last Price** | **$%.2f** |\n", data.LastPrice))
	content.WriteString(fmt.Sprintf("| Change | $%.2f (%.2f%%) |\n", data.PriceChange, data.ChangePercent))
	content.WriteString(fmt.Sprintf("| Bid/Ask | $%.2f / $%.2f |\n", data.BidPrice, data.AskPrice))
	content.WriteString(fmt.Sprintf("| Day Range | $%.2f - $%.2f |\n", data.DayLow, data.DayHigh))
	content.WriteString(fmt.Sprintf("| Volume | %s |\n", formatNumber(data.Volume)))
	content.WriteString(fmt.Sprintf("| Avg Volume | %s |\n\n", formatNumber(data.AvgVolume)))

	// Period Performance Section (like TradingView screenshot)
	content.WriteString("## Period Performance\n\n")
	content.WriteString("| Period | Price | Change ($) | Change (%) | 1k Shares | 1k Value |\n")
	content.WriteString("|--------|-------|------------|------------|-----------|----------|\n")
	periodPerf := calculatePeriodPerformance(data.HistoricalPrices, data.LastPrice)
	for _, p := range periodPerf {
		changeSign := ""
		if p.ChangePercent > 0 {
			changeSign = "+"
		}
		content.WriteString(fmt.Sprintf("| %s | $%.2f | %s$%.2f | %s%.2f%% | %d | $%s |\n",
			p.Period, p.Price, changeSign, p.ChangeValue, changeSign, p.ChangePercent,
			p.Shares1k, formatDecimal(p.Value1k)))
	}
	content.WriteString("\n")

	// Add explicit period changes summary for easy LLM extraction
	// This single line makes it easy for downstream summaries to extract period performance
	periodLabels := map[int]string{7: "7D", 30: "1M", 91: "3M", 183: "6M", 365: "1Y", 730: "2Y"}
	var summaryParts []string
	for _, p := range periodPerf {
		if label, ok := periodLabels[p.Days]; ok {
			sign := ""
			if p.ChangePercent > 0 {
				sign = "+"
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%s: %s%.1f%%", label, sign, p.ChangePercent))
		}
	}
	if len(summaryParts) > 0 {
		content.WriteString(fmt.Sprintf("**Period Changes Summary**: %s\n\n", strings.Join(summaryParts, ", ")))
	}

	content.WriteString("*1k Shares = shares purchasable with $1,000 at period start price. 1k Value = current value of those shares.*\n\n")

	// Volume Analysis Section
	content.WriteString("## Volume Analysis\n\n")
	content.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	content.WriteString(fmt.Sprintf("|--------|-------|\n"))
	content.WriteString(fmt.Sprintf("| Today's Volume | %s |\n", formatNumber(data.Volume)))
	content.WriteString(fmt.Sprintf("| Average Volume | %s |\n", formatNumber(data.AvgVolume)))
	volRatio := 0.0
	if data.AvgVolume > 0 {
		volRatio = float64(data.Volume) / float64(data.AvgVolume) * 100
	}
	volSignal := "Normal"
	if volRatio > 150 {
		volSignal = "High (Unusual Activity)"
	} else if volRatio < 50 {
		volSignal = "Low (Quiet Trading)"
	}
	content.WriteString(fmt.Sprintf("| Volume vs Avg | %.1f%% |\n", volRatio))
	content.WriteString(fmt.Sprintf("| Volume Signal | %s |\n\n", volSignal))

	// Recent Volume Trend (last 10 days)
	if len(data.HistoricalPrices) >= 10 {
		content.WriteString("### Recent Volume Trend (Last 10 Days)\n\n")
		content.WriteString("| Date | Close | Volume | vs Avg |\n")
		content.WriteString("|------|-------|--------|--------|\n")
		startIdx := len(data.HistoricalPrices) - 10
		for i := len(data.HistoricalPrices) - 1; i >= startIdx; i-- {
			p := data.HistoricalPrices[i]
			volVsAvg := 0.0
			if data.AvgVolume > 0 {
				volVsAvg = float64(p.Volume) / float64(data.AvgVolume) * 100
			}
			content.WriteString(fmt.Sprintf("| %s | $%.2f | %s | %.0f%% |\n",
				p.Date.Format("02 Jan"), p.Close, formatNumber(p.Volume), volVsAvg))
		}
		content.WriteString("\n")
	}

	// Valuation Section
	content.WriteString("## Valuation\n\n")
	content.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	content.WriteString(fmt.Sprintf("|--------|-------|\n"))
	content.WriteString(fmt.Sprintf("| Market Cap | $%s |\n", formatLargeNumber(data.MarketCap)))
	content.WriteString(fmt.Sprintf("| P/E Ratio | %.2f |\n", data.PERatio))
	content.WriteString(fmt.Sprintf("| EPS | $%.2f |\n", data.EPS))
	content.WriteString(fmt.Sprintf("| Dividend Yield | %.2f%% |\n", data.DividendYield))
	content.WriteString(fmt.Sprintf("| 52-Week Low | $%.2f |\n", data.Week52Low))
	content.WriteString(fmt.Sprintf("| 52-Week High | $%.2f |\n\n", data.Week52High))

	// Technical Analysis Section
	content.WriteString("## Technical Analysis\n\n")
	content.WriteString(fmt.Sprintf("### Trend Signal: **%s**\n\n", data.TrendSignal))

	content.WriteString("### Moving Averages\n\n")
	content.WriteString(fmt.Sprintf("| Indicator | Value | Signal |\n"))
	content.WriteString(fmt.Sprintf("|-----------|-------|--------|\n"))

	sma20Signal := "Bearish"
	if data.LastPrice > data.SMA20 {
		sma20Signal = "Bullish"
	}
	content.WriteString(fmt.Sprintf("| SMA 20 | $%.2f | %s |\n", data.SMA20, sma20Signal))

	sma50Signal := "Bearish"
	if data.LastPrice > data.SMA50 {
		sma50Signal = "Bullish"
	}
	content.WriteString(fmt.Sprintf("| SMA 50 | $%.2f | %s |\n", data.SMA50, sma50Signal))

	sma200Signal := "Bearish"
	if data.LastPrice > data.SMA200 {
		sma200Signal = "Bullish"
	}
	content.WriteString(fmt.Sprintf("| SMA 200 | $%.2f | %s |\n\n", data.SMA200, sma200Signal))

	content.WriteString("### Momentum & Levels\n\n")
	content.WriteString(fmt.Sprintf("| Indicator | Value |\n"))
	content.WriteString(fmt.Sprintf("|-----------|-------|\n"))

	rsiSignal := "Neutral"
	if data.RSI14 >= 70 {
		rsiSignal = "Overbought"
	} else if data.RSI14 <= 30 {
		rsiSignal = "Oversold"
	}
	content.WriteString(fmt.Sprintf("| RSI (14) | %.1f (%s) |\n", data.RSI14, rsiSignal))
	content.WriteString(fmt.Sprintf("| Support | $%.2f |\n", data.Support))
	content.WriteString(fmt.Sprintf("| Resistance | $%.2f |\n\n", data.Resistance))

	// Position in range
	rangePercent := 0.0
	if data.Week52High > data.Week52Low {
		rangePercent = (data.LastPrice - data.Week52Low) / (data.Week52High - data.Week52Low) * 100
	}
	content.WriteString(fmt.Sprintf("**Position in 52-Week Range**: %.1f%% (%.2f low, %.2f high)\n\n", rangePercent, data.Week52Low, data.Week52High))

	// 6-Month Price Chart (ASCII visualization)
	if len(data.HistoricalPrices) >= 20 {
		chart := generateASCIIPriceChart(data.HistoricalPrices, 126) // 126 trading days ≈ 6 months
		if chart != "" {
			content.WriteString("## 6-Month Price Chart\n\n")
			content.WriteString("```\n")
			content.WriteString(chart)
			content.WriteString("```\n\n")
		}
	}

	// Historical Daily Data (CSV format for LLM consumption)
	if len(data.HistoricalPrices) > 0 {
		content.WriteString("## Historical Daily Data (OHLCV)\n\n")
		content.WriteString("```csv\n")
		content.WriteString("Date,Open,High,Low,Close,Volume\n")
		for _, p := range data.HistoricalPrices {
			content.WriteString(fmt.Sprintf("%s,%.2f,%.2f,%.2f,%.2f,%d\n",
				p.Date.Format("2006-01-02"), p.Open, p.High, p.Low, p.Close, p.Volume))
		}
		content.WriteString("```\n\n")
	}

	// Build tags - different for indices vs stocks
	var tags []string
	if isIndex {
		tags = []string{"asx-index", strings.ToLower(asxCode), "benchmark"}
	} else {
		tags = []string{"asx-stock-data", strings.ToLower(asxCode)}
	}
	tags = append(tags, fmt.Sprintf("date:%s", time.Now().Format("2006-01-02")))

	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}
	tags = append(tags, outputTags...)

	// Add cache tags from context (for caching/deduplication)
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	// Build period performance for metadata (structured data for downstream consumption)
	// Note: periodPerf was already calculated above for the markdown content, reuse it here
	var periodPerfMeta []map[string]interface{}
	for _, p := range periodPerf {
		periodPerfMeta = append(periodPerfMeta, map[string]interface{}{
			"period":         p.Period,
			"days":           p.Days,
			"price":          p.Price,
			"change_value":   p.ChangeValue,
			"change_percent": p.ChangePercent,
			"shares_1k":      p.Shares1k,
			"value_1k":       p.Value1k,
		})
	}

	// Build OHLCV data for metadata (for downstream workers like asx_announcements)
	// Includes daily change calculations for each trading day
	var ohlcvMeta []map[string]interface{}
	var prevClose float64
	for i, p := range data.HistoricalPrices {
		entry := map[string]interface{}{
			"date":   p.Date.Format("2006-01-02"),
			"open":   p.Open,
			"high":   p.High,
			"low":    p.Low,
			"close":  p.Close,
			"volume": p.Volume,
		}

		// Add daily change metrics (skip first day - no previous day to compare)
		if i > 0 && prevClose > 0 {
			changeValue := p.Close - prevClose
			changePercent := (changeValue / prevClose) * 100
			entry["change_value"] = changeValue
			entry["change_percent"] = changePercent
			entry["prev_close"] = prevClose
		}

		ohlcvMeta = append(ohlcvMeta, entry)
		prevClose = p.Close
	}

	// Build metadata
	metadata := map[string]interface{}{
		"asx_code":           asxCode,
		"company_name":       data.CompanyName,
		"is_index":           isIndex,
		"last_price":         data.LastPrice,
		"price_change":       data.PriceChange,
		"change_percent":     data.ChangePercent,
		"market_cap":         data.MarketCap,
		"pe_ratio":           data.PERatio,
		"week52_low":         data.Week52Low,
		"week52_high":        data.Week52High,
		"sma20":              data.SMA20,
		"sma50":              data.SMA50,
		"sma200":             data.SMA200,
		"rsi14":              data.RSI14,
		"support":            data.Support,
		"resistance":         data.Resistance,
		"trend_signal":       data.TrendSignal,
		"parent_job_id":      parentJobID,
		"period_performance": periodPerfMeta, // Structured price change data for 7D, 1M, 3M, 6M, 1Y, 2Y
		"historical_prices":  ohlcvMeta,      // Raw OHLCV data for downstream workers (e.g., asx_announcements)
	}

	// Set source type and URL based on whether this is an index or stock
	sourceType := "asx_stock_data"
	sourceID := fmt.Sprintf("asx:%s:stock_data", asxCode)
	docURL := fmt.Sprintf("https://www.asx.com.au/markets/company/%s", asxCode)
	title := fmt.Sprintf("ASX:%s Stock Data & Technical Analysis", asxCode)

	if isIndex {
		sourceType = "asx_index"
		sourceID = fmt.Sprintf("asx:%s:index_data", asxCode)
		docURL = fmt.Sprintf("https://www.asx.com.au/indices/%s", strings.ToLower(asxCode))
		title = fmt.Sprintf("ASX Index: %s (%s) - Market Data & Technical Analysis", data.CompanyName, asxCode)
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      sourceType,
		SourceID:        sourceID,
		URL:             docURL,
		Title:           title,
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		Metadata:        metadata,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}

	return doc
}

// PeriodPerformance holds price change data for a time period
type PeriodPerformance struct {
	Period        string
	Days          int
	Price         float64
	ChangeValue   float64
	ChangePercent float64
	Shares1k      int     // Number of shares $1000 would buy at period start price
	Value1k       float64 // Current value of those shares at current price
}

// calculatePeriodPerformance calculates price changes over various periods
func calculatePeriodPerformance(prices []OHLCV, currentPrice float64) []PeriodPerformance {
	var results []PeriodPerformance
	if len(prices) == 0 || currentPrice == 0 {
		return results
	}

	now := time.Now()
	periods := []struct {
		name string
		days int
	}{
		{"1 Week (7d)", 7},
		{"1 Month (30d)", 30},
		{"3 Month (91d)", 91},
		{"6 Month (183d)", 183},
		{"1 Year (365d)", 365},
		{"2 Year (730d)", 730},
	}

	for _, period := range periods {
		targetDate := now.AddDate(0, 0, -period.days)
		// Find closest price to target date
		var closestPrice float64
		minDiff := time.Duration(math.MaxInt64)
		for _, p := range prices {
			diff := p.Date.Sub(targetDate)
			if diff < 0 {
				diff = -diff
			}
			if diff < minDiff {
				minDiff = diff
				closestPrice = p.Close
			}
		}

		if closestPrice > 0 {
			changeValue := currentPrice - closestPrice
			changePercent := (changeValue / closestPrice) * 100
			// Calculate $1000 investment: how many shares and current value
			shares1k := int(1000 / closestPrice)
			value1k := float64(shares1k) * currentPrice
			results = append(results, PeriodPerformance{
				Period:        period.name,
				Days:          period.days,
				Price:         closestPrice,
				ChangeValue:   changeValue,
				ChangePercent: changePercent,
				Shares1k:      shares1k,
				Value1k:       value1k,
			})
		}
	}

	return results
}

// formatNumber formats a number with commas
func formatNumber(n int64) string {
	if n == 0 {
		return "0"
	}
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// formatLargeNumber formats large numbers with M/B suffix
func formatLargeNumber(n int64) string {
	if n >= 1e9 {
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	}
	if n >= 1e6 {
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	}
	return formatNumber(n)
}

// formatDecimal formats a decimal number with commas and 2 decimal places
func formatDecimal(n float64) string {
	intPart := int64(n)
	decPart := n - float64(intPart)
	return fmt.Sprintf("%s.%02d", formatNumber(intPart), int(decPart*100))
}

// generateASCIIPriceChart creates a simple ASCII chart of closing prices
// Uses approximately 6 months of data (126 trading days) with 60 columns
func generateASCIIPriceChart(prices []OHLCV, days int) string {
	if len(prices) == 0 {
		return ""
	}

	// Get last N days of data
	startIdx := 0
	if len(prices) > days {
		startIdx = len(prices) - days
	}
	chartPrices := prices[startIdx:]

	if len(chartPrices) < 5 {
		return ""
	}

	// Find min and max for scaling
	minPrice := chartPrices[0].Close
	maxPrice := chartPrices[0].Close
	for _, p := range chartPrices {
		if p.Close < minPrice && p.Close > 0 {
			minPrice = p.Close
		}
		if p.Close > maxPrice {
			maxPrice = p.Close
		}
	}

	// Add 5% padding to range
	priceRange := maxPrice - minPrice
	if priceRange < 0.01 {
		priceRange = maxPrice * 0.1 // 10% range for flat stocks
	}
	minPrice -= priceRange * 0.05
	maxPrice += priceRange * 0.05
	priceRange = maxPrice - minPrice

	// Chart dimensions
	chartWidth := 60
	chartHeight := 12

	// Sample prices to fit width
	sampleInterval := len(chartPrices) / chartWidth
	if sampleInterval < 1 {
		sampleInterval = 1
	}

	var sampledPrices []float64
	var sampledDates []time.Time
	for i := 0; i < len(chartPrices); i += sampleInterval {
		sampledPrices = append(sampledPrices, chartPrices[i].Close)
		sampledDates = append(sampledDates, chartPrices[i].Date)
	}
	// Ensure we include the last price
	if len(sampledPrices) > 0 && sampledPrices[len(sampledPrices)-1] != chartPrices[len(chartPrices)-1].Close {
		sampledPrices = append(sampledPrices, chartPrices[len(chartPrices)-1].Close)
		sampledDates = append(sampledDates, chartPrices[len(chartPrices)-1].Date)
	}

	// Create the chart grid
	var result strings.Builder

	// Price labels for Y-axis (show 5 levels)
	priceLabels := make([]string, chartHeight)
	for i := 0; i < chartHeight; i++ {
		price := maxPrice - (priceRange * float64(i) / float64(chartHeight-1))
		priceLabels[i] = fmt.Sprintf("$%.2f", price)
	}
	labelWidth := 8

	// Draw chart line by line (top to bottom)
	for row := 0; row < chartHeight; row++ {
		priceThreshold := maxPrice - (priceRange * float64(row) / float64(chartHeight-1))
		nextThreshold := maxPrice - (priceRange * float64(row+1) / float64(chartHeight-1))

		// Y-axis label
		if row == 0 || row == chartHeight/2 || row == chartHeight-1 {
			result.WriteString(fmt.Sprintf("%*s ", labelWidth, priceLabels[row]))
		} else {
			result.WriteString(fmt.Sprintf("%*s ", labelWidth, ""))
		}

		// Draw chart line for this row
		result.WriteString("│")
		for col := 0; col < len(sampledPrices) && col < chartWidth; col++ {
			price := sampledPrices[col]
			if price >= nextThreshold && price <= priceThreshold {
				// Price is at this level
				if col > 0 {
					prevPrice := sampledPrices[col-1]
					if price > prevPrice {
						result.WriteString("╱")
					} else if price < prevPrice {
						result.WriteString("╲")
					} else {
						result.WriteString("─")
					}
				} else {
					result.WriteString("─")
				}
			} else if row < chartHeight-1 {
				// Check if line passes through this cell
				if col > 0 {
					prevPrice := sampledPrices[col-1]
					if (prevPrice <= priceThreshold && price >= nextThreshold) ||
						(price <= priceThreshold && prevPrice >= nextThreshold) {
						if price > prevPrice {
							result.WriteString("│")
						} else {
							result.WriteString("│")
						}
					} else {
						result.WriteString(" ")
					}
				} else {
					result.WriteString(" ")
				}
			} else {
				result.WriteString(" ")
			}
		}
		result.WriteString("\n")
	}

	// X-axis
	result.WriteString(fmt.Sprintf("%*s └", labelWidth, ""))
	for i := 0; i < chartWidth && i < len(sampledPrices); i++ {
		result.WriteString("─")
	}
	result.WriteString("\n")

	// Month labels on X-axis
	result.WriteString(fmt.Sprintf("%*s  ", labelWidth, ""))
	lastMonth := -1
	for col := 0; col < len(sampledDates) && col < chartWidth; col++ {
		month := int(sampledDates[col].Month())
		if month != lastMonth && col > 0 {
			result.WriteString(sampledDates[col].Format("Jan"))
			col += 2 // Skip next 2 positions
			lastMonth = month
		} else if lastMonth == -1 {
			result.WriteString(sampledDates[col].Format("Jan"))
			col += 2
			lastMonth = month
		} else {
			result.WriteString(" ")
		}
	}
	result.WriteString("\n")

	// Summary line
	currentPrice := chartPrices[len(chartPrices)-1].Close
	result.WriteString(fmt.Sprintf("\nHigh: $%.2f  Low: $%.2f  Current: $%.2f\n",
		findMax(sampledPrices), findMin(sampledPrices), currentPrice))

	return result.String()
}

// Ensure math is used
var _ = math.Abs
