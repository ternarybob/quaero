// -----------------------------------------------------------------------
// MarketCompetitorWorker - Analyzes competitors and fetches their stock data
// Uses LLM to identify competitor ASX codes, then directly fetches stock data
// for each competitor using MarketFundamentalsWorker (inline execution).
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"google.golang.org/genai"
)

// MarketCompetitorWorker analyzes a target company, identifies competitors,
// and fetches stock data for each competitor inline.
type MarketCompetitorWorker struct {
	documentStorage      interfaces.DocumentStorage
	kvStorage            interfaces.KeyValueStorage
	jobMgr               *queue.Manager
	logger               arbor.ILogger
	stockCollectorWorker *MarketFundamentalsWorker // Reuses stock collector for competitor data
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*MarketCompetitorWorker)(nil)

// NewMarketCompetitorWorker creates a new competitor analysis worker
func NewMarketCompetitorWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	jobMgr *queue.Manager,
	logger arbor.ILogger,
	debugEnabled bool,
) *MarketCompetitorWorker {
	// Create embedded stock collector worker for fetching competitor data
	stockCollectorWorker := NewMarketFundamentalsWorker(documentStorage, kvStorage, logger, jobMgr, debugEnabled)

	return &MarketCompetitorWorker{
		documentStorage:      documentStorage,
		kvStorage:            kvStorage,
		jobMgr:               jobMgr,
		logger:               logger,
		stockCollectorWorker: stockCollectorWorker,
	}
}

// GetType returns WorkerTypeMarketCompetitor
func (w *MarketCompetitorWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketCompetitor
}

// Init performs initialization for the competitor analysis step.
// Validates config and resolves API key.
// Supports both step config and job-level variables for ticker configuration
func (w *MarketCompetitorWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Collect tickers - supports both step config and job-level variables
	tickers := collectTickersWithJobDef(stepConfig, jobDef)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("ticker, asx_code, tickers, or asx_codes is required in step config or job variables")
	}

	// Extract prompt template for competitor identification
	promptTemplate, _ := stepConfig["prompt"].(string)

	// Get API key from step config
	var apiKey string
	if apiKeyValue, ok := stepConfig["api_key"].(string); ok && apiKeyValue != "" {
		if len(apiKeyValue) > 2 && apiKeyValue[0] == '{' && apiKeyValue[len(apiKeyValue)-1] == '}' {
			cleanAPIKeyName := strings.Trim(apiKeyValue, "{}")
			resolvedAPIKey, err := common.ResolveAPIKey(ctx, w.kvStorage, cleanAPIKeyName, "")
			if err != nil {
				return nil, fmt.Errorf("failed to resolve API key '%s': %w", cleanAPIKeyName, err)
			}
			apiKey = resolvedAPIKey
		} else {
			apiKey = apiKeyValue
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for competitor_analysis")
	}

	// Extract output tags
	var outputTags []string
	if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				outputTags = append(outputTags, tagStr)
			}
		}
	} else if tags, ok := stepConfig["output_tags"].([]string); ok {
		outputTags = tags
	}

	// Period for historical data (default Y1)
	period := "Y1"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Msg("Competitor analysis worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   ticker.Code,
			Name: fmt.Sprintf("Analyze competitors for %s", ticker.String()),
			Type: "market_competitor",
			Config: map[string]interface{}{
				"asx_code": ticker.Code,
				"period":   period,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(tickers),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"tickers":         tickers,
			"prompt_template": promptTemplate,
			"api_key":         apiKey,
			"output_tags":     outputTags,
			"period":          period,
			"step_config":     stepConfig,
		},
	}, nil
}

// CreateJobs uses LLM to identify competitors, then fetches stock data inline.
// Supports multiple target tickers - processes each sequentially.
func (w *MarketCompetitorWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize competitor_analysis worker: %w", err)
		}
	}

	// Get tickers from metadata
	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	promptTemplate, _ := initResult.Metadata["prompt_template"].(string)
	apiKey, _ := initResult.Metadata["api_key"].(string)
	outputTags, _ := initResult.Metadata["output_tags"].([]string)
	period, _ := initResult.Metadata["period"].(string)

	// Log overall step start
	tickerCount := len(tickers)
	if w.jobMgr != nil {
		tickerStrs := make([]string, tickerCount)
		for i, t := range tickers {
			tickerStrs[i] = t.String()
		}
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Analyzing competitors for %d tickers: %s", tickerCount, strings.Join(tickerStrs, ", ")))
	}

	// Process each target ticker sequentially
	var lastErr error
	successCount := 0
	for _, ticker := range tickers {
		err := w.processTicker(ctx, ticker.Code, promptTemplate, apiKey, outputTags, period, stepID, jobDef)
		if err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to analyze competitors")
			lastErr = err
			// Continue with next ticker (on_error = "continue" behavior)
			continue
		}
		successCount++
	}

	// Log overall completion
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Completed %d/%d target tickers successfully", successCount, tickerCount))
	}

	// Return error only if ALL tickers failed
	if successCount == 0 && lastErr != nil {
		return "", fmt.Errorf("all target tickers failed, last error: %w", lastErr)
	}

	return stepID, nil
}

// processTicker analyzes competitors for a single target ticker
func (w *MarketCompetitorWorker) processTicker(ctx context.Context, asxCode, promptTemplate, apiKey string, outputTags []string, period, stepID string, jobDef models.JobDefinition) error {
	w.logger.Info().
		Str("phase", "run").
		Str("asx_code", asxCode).
		Str("step_id", stepID).
		Msg("Starting competitor analysis for ticker")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Analyzing competitors for ASX:%s", asxCode))
	}

	// Build prompt for this ticker
	prompt := promptTemplate
	if prompt == "" {
		prompt = fmt.Sprintf("Identify the top 3-5 ASX-listed competitors for %s", asxCode)
	}

	// Step 1: Use LLM to identify competitors
	competitors, err := w.identifyCompetitors(ctx, asxCode, prompt, apiKey)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to identify competitors")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("ASX:%s - Failed to identify competitors: %v", asxCode, err))
		}
		return fmt.Errorf("failed to identify competitors for %s: %w", asxCode, err)
	}

	if len(competitors) == 0 {
		w.logger.Warn().Str("asx_code", asxCode).Msg("No competitors identified")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("ASX:%s - No competitors identified by LLM", asxCode))
		}
		return nil // Not an error - just no competitors found
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Strs("competitors", competitors).
		Msg("Competitors identified, fetching stock data")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("ASX:%s - Identified %d competitors: %s", asxCode, len(competitors), strings.Join(competitors, ", ")))
	}

	// Step 2: Fetch stock data for each competitor inline using MarketFundamentalsWorker
	fetchSuccessCount := 0
	for _, competitorCode := range competitors {
		err := w.fetchCompetitorStockData(ctx, competitorCode, period, outputTags, stepID, jobDef)
		if err != nil {
			w.logger.Warn().Err(err).Str("competitor", competitorCode).Msg("Failed to fetch stock data")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to fetch data for %s: %v", competitorCode, err))
			}
			continue
		}
		fetchSuccessCount++
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetched stock data for ASX:%s", competitorCode))
		}
	}

	if fetchSuccessCount == 0 && len(competitors) > 0 {
		return fmt.Errorf("failed to fetch any competitor stock data for %s", asxCode)
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("ASX:%s - Completed: fetched stock data for %d/%d competitors", asxCode, fetchSuccessCount, len(competitors)))
	}

	return nil
}

// fetchCompetitorStockData fetches stock data for a single competitor using MarketFundamentalsWorker
func (w *MarketCompetitorWorker) fetchCompetitorStockData(ctx context.Context, asxCode, period string, outputTags []string, stepID string, jobDef models.JobDefinition) error {
	// Create a synthetic step for the stock collector worker
	stockStep := models.JobStep{
		Name:        fmt.Sprintf("fetch_competitor_%s", strings.ToLower(asxCode)),
		Type:        models.WorkerTypeMarketFundamentals,
		Description: fmt.Sprintf("Fetch stock data for competitor ASX:%s", asxCode),
		Config: map[string]interface{}{
			"asx_code":    asxCode,
			"period":      period,
			"output_tags": outputTags,
		},
	}

	// Call the stock collector worker directly
	_, err := w.stockCollectorWorker.CreateJobs(ctx, stockStep, jobDef, stepID, nil)
	if err != nil {
		return fmt.Errorf("stock data fetch failed for %s: %w", asxCode, err)
	}

	return nil
}

// identifyCompetitors uses Gemini to identify competitor ASX codes
func (w *MarketCompetitorWorker) identifyCompetitors(ctx context.Context, asxCode, prompt, apiKey string) ([]string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Build prompt for competitor identification
	systemPrompt := fmt.Sprintf(`You are a financial analyst. Your task is to identify ASX-listed competitors.

**Target Company**: ASX:%s

**Task**: %s

**IMPORTANT**: Return ONLY a JSON array of ASX stock codes (3-4 letter codes).
Do NOT include the target company (%s) in the list.
Do NOT include any explanation or other text.
Only include companies that are actually listed on the ASX.

**Example Response**:
["WOW", "HVN", "JBH"]

Return the JSON array now:`, asxCode, prompt, asxCode)

	// Execute with timeout
	llmCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.2)),
	}

	resp, err := client.Models.GenerateContent(
		llmCtx,
		"gemini-3-pro-preview",
		[]*genai.Content{
			genai.NewContentFromText(systemPrompt, genai.RoleUser),
		},
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no response from Gemini API")
	}

	responseText := resp.Text()
	if responseText == "" {
		return nil, fmt.Errorf("empty response from Gemini API")
	}

	w.logger.Debug().
		Str("asx_code", asxCode).
		Str("response", responseText).
		Msg("LLM competitor identification response")

	// Parse JSON array of competitor codes
	competitors, err := parseCompetitorCodes(responseText, asxCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse competitor codes: %w", err)
	}

	return competitors, nil
}

// parseCompetitorCodes extracts ASX codes from LLM response
func parseCompetitorCodes(response, targetCode string) ([]string, error) {
	// Try to parse as JSON array first
	response = strings.TrimSpace(response)

	// Remove markdown code blocks if present
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock || !strings.HasPrefix(line, "```") {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
		response = strings.TrimSpace(response)
	}

	var codes []string
	if err := json.Unmarshal([]byte(response), &codes); err == nil {
		// Successfully parsed JSON
		return filterValidCodes(codes, targetCode), nil
	}

	// Fallback: extract ASX codes using regex (3-4 uppercase letters)
	re := regexp.MustCompile(`\b([A-Z]{3,4})\b`)
	matches := re.FindAllStringSubmatch(response, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		code := match[1]
		if !seen[code] && code != targetCode {
			seen[code] = true
			codes = append(codes, code)
		}
	}

	if len(codes) == 0 {
		return nil, fmt.Errorf("no valid ASX codes found in response")
	}

	return codes, nil
}

// filterValidCodes removes invalid codes and the target code
func filterValidCodes(codes []string, targetCode string) []string {
	var valid []string
	seen := make(map[string]bool)

	for _, code := range codes {
		code = strings.ToUpper(strings.TrimSpace(code))
		// Valid ASX codes are 3-4 uppercase letters
		if len(code) >= 3 && len(code) <= 4 && code != targetCode && !seen[code] {
			isAlpha := true
			for _, c := range code {
				if c < 'A' || c > 'Z' {
					isAlpha = false
					break
				}
			}
			if isAlpha {
				seen[code] = true
				valid = append(valid, code)
			}
		}
	}

	return valid
}

// ReturnsChildJobs returns false - we execute inline, not via child jobs
func (w *MarketCompetitorWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
// Config can be nil if tickers will be provided via job-level variables.
func (w *MarketCompetitorWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - tickers can come from job-level variables
	// Full validation happens in Init() when we have access to jobDef
	return nil
}
