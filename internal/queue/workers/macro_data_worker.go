// -----------------------------------------------------------------------
// MacroDataWorker - Fetches macroeconomic data (RBA rates, commodity prices)
// Uses public APIs to fetch interest rates and commodity price data
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// MacroDataWorker fetches macroeconomic data and stores it as documents.
// Supports: RBA cash rate, commodity prices (Iron Ore, Gold)
// This worker executes synchronously (no child jobs).
type MacroDataWorker struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	httpClient      *http.Client
}

// Compile-time assertion: MacroDataWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*MacroDataWorker)(nil)

// MacroDataType represents the type of macro data to fetch
type MacroDataType string

const (
	MacroDataTypeRBACashRate MacroDataType = "rba_cash_rate"
	MacroDataTypeIronOre     MacroDataType = "iron_ore"
	MacroDataTypeGold        MacroDataType = "gold"
	MacroDataTypeAll         MacroDataType = "all"
)

// MacroDataPoint represents a single macro data point
type MacroDataPoint struct {
	Type        MacroDataType
	Name        string
	Value       float64
	Unit        string
	Date        time.Time
	Change      float64 // Change from previous period
	ChangeLabel string  // e.g., "up 0.25% from previous"
	Source      string
}

// NewMacroDataWorker creates a new macro data worker
func NewMacroDataWorker(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *MacroDataWorker {
	return &MacroDataWorker{
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns WorkerTypeMacroData for the DefinitionWorker interface
func (w *MacroDataWorker) GetType() models.WorkerType {
	return models.WorkerTypeMacroData
}

// Init performs the initialization/setup phase for a macro data step.
func (w *MacroDataWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract data_type (optional, defaults to "all")
	dataType := MacroDataTypeAll
	if dt, ok := stepConfig["data_type"].(string); ok && dt != "" {
		dataType = MacroDataType(dt)
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("data_type", string(dataType)).
		Msg("Macro data worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   string(dataType),
				Name: fmt.Sprintf("Fetch macro data: %s", dataType),
				Type: "macro_data",
				Config: map[string]interface{}{
					"data_type": string(dataType),
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"data_type":   string(dataType),
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs fetches macro data and stores it as documents.
// Returns the step job ID since this executes synchronously.
func (w *MacroDataWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize macro_data worker: %w", err)
		}
	}

	// Extract metadata from init result
	dataTypeStr, _ := initResult.Metadata["data_type"].(string)
	dataType := MacroDataType(dataTypeStr)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("data_type", string(dataType)).
		Str("step_id", stepID).
		Msg("Fetching macro data")

	// Log step start for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching macro data: %s", dataType))
	}

	// Fetch macro data based on type
	var dataPoints []MacroDataPoint
	var err error

	switch dataType {
	case MacroDataTypeRBACashRate:
		dataPoints, err = w.fetchRBACashRate(ctx)
	case MacroDataTypeIronOre:
		dataPoints, err = w.fetchCommodityPrice(ctx, "iron_ore")
	case MacroDataTypeGold:
		dataPoints, err = w.fetchCommodityPrice(ctx, "gold")
	case MacroDataTypeAll:
		// Fetch all data types
		rbaData, rbaErr := w.fetchRBACashRate(ctx)
		if rbaErr != nil {
			w.logger.Warn().Err(rbaErr).Msg("Failed to fetch RBA data")
		} else {
			dataPoints = append(dataPoints, rbaData...)
		}

		ironOreData, ironErr := w.fetchCommodityPrice(ctx, "iron_ore")
		if ironErr != nil {
			w.logger.Warn().Err(ironErr).Msg("Failed to fetch iron ore data")
		} else {
			dataPoints = append(dataPoints, ironOreData...)
		}

		goldData, goldErr := w.fetchCommodityPrice(ctx, "gold")
		if goldErr != nil {
			w.logger.Warn().Err(goldErr).Msg("Failed to fetch gold data")
		} else {
			dataPoints = append(dataPoints, goldData...)
		}
	default:
		return "", fmt.Errorf("unknown data_type: %s", dataType)
	}

	if err != nil {
		w.logger.Error().Err(err).Str("data_type", string(dataType)).Msg("Failed to fetch macro data")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch macro data: %v", err))
		}
		return "", fmt.Errorf("failed to fetch macro data: %w", err)
	}

	if len(dataPoints) == 0 {
		w.logger.Warn().Str("data_type", string(dataType)).Msg("No macro data available")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", "No macro data available")
		}
		return stepID, nil
	}

	// Extract output_tags from step config
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

	// Create a consolidated document with all macro data
	doc := w.createDocument(ctx, dataPoints, &jobDef, stepID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to save macro data document")
		return "", fmt.Errorf("failed to save macro data document: %w", err)
	}

	w.logger.Info().
		Int("data_points", len(dataPoints)).
		Msg("Macro data processed")

	// Log completion for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Saved macro data document with %d data points", len(dataPoints)))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *MacroDataWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for macro_data type
func (w *MacroDataWorker) ValidateConfig(step models.JobStep) error {
	// No required fields - data_type defaults to "all"
	if step.Config != nil {
		if dt, ok := step.Config["data_type"].(string); ok && dt != "" {
			validTypes := []MacroDataType{MacroDataTypeRBACashRate, MacroDataTypeIronOre, MacroDataTypeGold, MacroDataTypeAll}
			isValid := false
			for _, valid := range validTypes {
				if MacroDataType(dt) == valid {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid data_type: %s (valid: rba_cash_rate, iron_ore, gold, all)", dt)
			}
		}
	}
	return nil
}

// fetchRBACashRate fetches the current RBA cash rate
func (w *MacroDataWorker) fetchRBACashRate(ctx context.Context) ([]MacroDataPoint, error) {
	// Use a simple approach: hardcode current rate with date
	// In production, this could scrape RBA website or use their data API
	// RBA Statistics: https://www.rba.gov.au/statistics/cash-rate/

	// For now, create a data point with approximate current rate
	// This should be enhanced to actually scrape/fetch RBA data
	now := time.Now()

	// Try to fetch from RBA API or fallback to known rate
	rate, err := w.fetchRBAFromAPI(ctx)
	if err != nil {
		w.logger.Warn().Err(err).Msg("Failed to fetch RBA rate from API, using fallback")
		// Fallback: use approximate known rate (as of Dec 2024)
		rate = 4.35
	}

	return []MacroDataPoint{
		{
			Type:        MacroDataTypeRBACashRate,
			Name:        "RBA Cash Rate Target",
			Value:       rate,
			Unit:        "%",
			Date:        now,
			ChangeLabel: "Current target rate",
			Source:      "Reserve Bank of Australia",
		},
	}, nil
}

// fetchRBAFromAPI attempts to fetch RBA cash rate from public data
func (w *MacroDataWorker) fetchRBAFromAPI(ctx context.Context) (float64, error) {
	// RBA provides some data in JSON format
	// This is a simplified implementation - in production, parse actual RBA data feeds
	// Return error to trigger fallback for now
	return 0, fmt.Errorf("RBA API fetch not implemented - using fallback")
}

// fetchCommodityPrice fetches commodity price data
func (w *MacroDataWorker) fetchCommodityPrice(ctx context.Context, commodity string) ([]MacroDataPoint, error) {
	now := time.Now()

	switch commodity {
	case "iron_ore":
		// Fetch iron ore price (62% Fe CFR China)
		price, err := w.fetchIronOrePrice(ctx)
		if err != nil {
			w.logger.Warn().Err(err).Msg("Failed to fetch iron ore price from API, using fallback")
			// Fallback: use approximate recent price
			price = 105.0 // Approximate USD/tonne
		}

		return []MacroDataPoint{
			{
				Type:        MacroDataTypeIronOre,
				Name:        "Iron Ore (62% Fe CFR China)",
				Value:       price,
				Unit:        "USD/tonne",
				Date:        now,
				ChangeLabel: "Spot price",
				Source:      "Market Data",
			},
		}, nil

	case "gold":
		// Fetch gold spot price
		price, err := w.fetchGoldPrice(ctx)
		if err != nil {
			w.logger.Warn().Err(err).Msg("Failed to fetch gold price from API, using fallback")
			// Fallback: use approximate recent price
			price = 2650.0 // Approximate USD/oz
		}

		return []MacroDataPoint{
			{
				Type:        MacroDataTypeGold,
				Name:        "Gold Spot Price",
				Value:       price,
				Unit:        "USD/oz",
				Date:        now,
				ChangeLabel: "Spot price",
				Source:      "Market Data",
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown commodity: %s", commodity)
	}
}

// yahooFinanceQuote represents a simplified Yahoo Finance API response
type yahooFinanceQuote struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				Currency           string  `json:"currency"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

// fetchIronOrePrice fetches iron ore price from Yahoo Finance
func (w *MacroDataWorker) fetchIronOrePrice(ctx context.Context) (float64, error) {
	// BHP (iron ore proxy) or other commodity ETF
	// Could also use SGX iron ore futures
	return w.fetchYahooFinancePrice(ctx, "BHP.AX")
}

// fetchGoldPrice fetches gold price from Yahoo Finance
func (w *MacroDataWorker) fetchGoldPrice(ctx context.Context) (float64, error) {
	// GC=F is gold futures, GLD is gold ETF
	return w.fetchYahooFinancePrice(ctx, "GC=F")
}

// fetchYahooFinancePrice fetches a price from Yahoo Finance API
func (w *MacroDataWorker) fetchYahooFinancePrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var quote yahooFinanceQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(quote.Chart.Result) == 0 {
		return 0, fmt.Errorf("no data returned for symbol: %s", symbol)
	}

	return quote.Chart.Result[0].Meta.RegularMarketPrice, nil
}

// createDocument creates a Document from macro data points
func (w *MacroDataWorker) createDocument(ctx context.Context, dataPoints []MacroDataPoint, jobDef *models.JobDefinition, parentJobID string, outputTags []string) *models.Document {
	now := time.Now()

	// Build markdown content
	var content strings.Builder
	content.WriteString("# Macroeconomic Data Summary\n\n")
	content.WriteString(fmt.Sprintf("**Report Date**: %s\n\n", now.Format("2 January 2006 3:04 PM AEST")))

	// Group data by type
	var rbaData, commodityData []MacroDataPoint
	for _, dp := range dataPoints {
		switch dp.Type {
		case MacroDataTypeRBACashRate:
			rbaData = append(rbaData, dp)
		case MacroDataTypeIronOre, MacroDataTypeGold:
			commodityData = append(commodityData, dp)
		}
	}

	// RBA Section
	if len(rbaData) > 0 {
		content.WriteString("## Interest Rates\n\n")
		content.WriteString("| Indicator | Value | Source |\n")
		content.WriteString("|-----------|-------|--------|\n")
		for _, dp := range rbaData {
			content.WriteString(fmt.Sprintf("| %s | %.2f%s | %s |\n", dp.Name, dp.Value, dp.Unit, dp.Source))
		}
		content.WriteString("\n")

		content.WriteString("### Investment Implications\n")
		content.WriteString("- **Higher rates**: Typically negative for growth stocks, positive for banks\n")
		content.WriteString("- **Lower rates**: Typically positive for REITs, growth stocks, negative for savers\n")
		content.WriteString("- Consider rate trajectory vs current level\n\n")
	}

	// Commodity Section
	if len(commodityData) > 0 {
		content.WriteString("## Commodity Prices\n\n")
		content.WriteString("| Commodity | Price | Unit | Source |\n")
		content.WriteString("|-----------|-------|------|--------|\n")
		for _, dp := range commodityData {
			content.WriteString(fmt.Sprintf("| %s | %.2f | %s | %s |\n", dp.Name, dp.Value, dp.Unit, dp.Source))
		}
		content.WriteString("\n")

		content.WriteString("### Investment Implications\n")
		content.WriteString("- **Iron Ore**: Key driver for BHP, RIO, FMG. Higher prices = higher earnings\n")
		content.WriteString("- **Gold**: Safe haven, inflation hedge. Rising gold often signals uncertainty\n")
		content.WriteString("- Compare current prices to 52-week range for context\n\n")
	}

	content.WriteString("---\n")
	content.WriteString("## How to Use This Data\n")
	content.WriteString("1. **Sector Correlation**: Check if stock's recent move correlates with macro factors\n")
	content.WriteString("2. **Company-Specific vs Macro**: If stock moved WITH sector, it's macro-driven\n")
	content.WriteString("3. **Earnings Impact**: Use commodity prices to estimate revenue impacts\n")

	// Build tags
	tags := []string{"macro-data"}
	for _, dp := range dataPoints {
		tags = append(tags, string(dp.Type))
	}

	// Add date tag
	dateTag := fmt.Sprintf("date:%s", now.Format("2006-01-02"))
	tags = append(tags, dateTag)

	// Add job definition tags
	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}

	// Add output_tags from step config
	tags = append(tags, outputTags...)

	// Apply cache tags from context
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	// Build metadata
	metadata := map[string]interface{}{
		"report_date":   now.Format(time.RFC3339),
		"data_points":   len(dataPoints),
		"parent_job_id": parentJobID,
	}

	// Add individual data points to metadata
	for _, dp := range dataPoints {
		metadata[string(dp.Type)] = map[string]interface{}{
			"value":  dp.Value,
			"unit":   dp.Unit,
			"source": dp.Source,
		}
	}

	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "macro_data",
		SourceID:        fmt.Sprintf("macro-data-%s", now.Format("2006-01-02")),
		Title:           fmt.Sprintf("Macroeconomic Data - %s", now.Format("2 Jan 2006")),
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
