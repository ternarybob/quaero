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
type ASXStockDataWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
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

	// Period for historical data (default 1y)
	period := "1y"
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

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("period", period).
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
	doc := w.createDocument(stockData, asxCode, &jobDef, stepID, outputTags)
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

// fetchStockData fetches data from multiple sources
func (w *ASXStockDataWorker) fetchStockData(ctx context.Context, asxCode, period string) (*StockData, error) {
	stockData := &StockData{
		Symbol:      asxCode,
		LastUpdated: time.Now(),
	}

	// Fetch from Markit Digital API (header)
	if err := w.fetchMarkitHeader(ctx, asxCode, stockData); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to fetch Markit header data")
	}

	// Fetch from Markit Digital API (key-statistics)
	if err := w.fetchMarkitStats(ctx, asxCode, stockData); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to fetch Markit stats data")
	}

	// Fetch historical data from Yahoo Finance
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

// fetchYahooHistory fetches historical OHLCV from Yahoo Finance
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

	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s.AX?interval=1d&range=%s",
		strings.ToUpper(asxCode), yahooRange)

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

// createDocument creates a document from stock data
func (w *ASXStockDataWorker) createDocument(data *StockData, asxCode string, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# ASX:%s Stock Data - %s\n\n", asxCode, data.CompanyName))
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

	// Period Performance Section (like screenshot)
	content.WriteString("## Period Performance\n\n")
	content.WriteString("| Period | Price | Change ($) | Change (%) |\n")
	content.WriteString("|--------|-------|------------|------------|\n")
	periodPerf := calculatePeriodPerformance(data.HistoricalPrices, data.LastPrice)
	for _, p := range periodPerf {
		changeColor := ""
		if p.ChangePercent > 0 {
			changeColor = "+"
		}
		content.WriteString(fmt.Sprintf("| %s | $%.2f | %s$%.2f | %s%.2f%% |\n",
			p.Period, p.Price, changeColor, p.ChangeValue, changeColor, p.ChangePercent))
	}
	content.WriteString("\n")

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

	// Build tags
	tags := []string{"asx-stock-data", strings.ToLower(asxCode)}
	tags = append(tags, fmt.Sprintf("date:%s", time.Now().Format("2006-01-02")))

	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}
	tags = append(tags, outputTags...)

	// Build metadata
	metadata := map[string]interface{}{
		"asx_code":       asxCode,
		"company_name":   data.CompanyName,
		"last_price":     data.LastPrice,
		"price_change":   data.PriceChange,
		"change_percent": data.ChangePercent,
		"market_cap":     data.MarketCap,
		"pe_ratio":       data.PERatio,
		"week52_low":     data.Week52Low,
		"week52_high":    data.Week52High,
		"sma20":          data.SMA20,
		"sma50":          data.SMA50,
		"sma200":         data.SMA200,
		"rsi14":          data.RSI14,
		"support":        data.Support,
		"resistance":     data.Resistance,
		"trend_signal":   data.TrendSignal,
		"parent_job_id":  parentJobID,
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "asx_stock_data",
		SourceID:        fmt.Sprintf("asx:%s:stock_data", asxCode),
		URL:             fmt.Sprintf("https://www.asx.com.au/markets/company/%s", asxCode),
		Title:           fmt.Sprintf("ASX:%s Stock Data & Technical Analysis", asxCode),
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
	Price         float64
	ChangeValue   float64
	ChangePercent float64
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
			results = append(results, PeriodPerformance{
				Period:        period.name,
				Price:         closestPrice,
				ChangeValue:   changeValue,
				ChangePercent: changePercent,
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

// Ensure math is used
var _ = math.Abs
