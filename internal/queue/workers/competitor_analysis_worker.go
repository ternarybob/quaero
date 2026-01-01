// -----------------------------------------------------------------------
// CompetitorAnalysisWorker - Analyzes competitors and fetches their stock data
// Uses LLM to identify competitor ASX codes, then directly fetches stock data
// for each competitor using ASXStockDataWorker (inline execution).
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

// CompetitorAnalysisWorker analyzes a target company, identifies competitors,
// and fetches stock data for each competitor inline.
type CompetitorAnalysisWorker struct {
	documentStorage      interfaces.DocumentStorage
	kvStorage            interfaces.KeyValueStorage
	jobMgr               *queue.Manager
	logger               arbor.ILogger
	stockCollectorWorker *ASXStockCollectorWorker // Reuses stock collector for competitor data
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*CompetitorAnalysisWorker)(nil)

// NewCompetitorAnalysisWorker creates a new competitor analysis worker
func NewCompetitorAnalysisWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	jobMgr *queue.Manager,
	logger arbor.ILogger,
) *CompetitorAnalysisWorker {
	// Create embedded stock collector worker for fetching competitor data
	stockCollectorWorker := NewASXStockCollectorWorker(documentStorage, kvStorage, logger, jobMgr)

	return &CompetitorAnalysisWorker{
		documentStorage:      documentStorage,
		kvStorage:            kvStorage,
		jobMgr:               jobMgr,
		logger:               logger,
		stockCollectorWorker: stockCollectorWorker,
	}
}

// GetType returns WorkerTypeCompetitorAnalysis
func (w *CompetitorAnalysisWorker) GetType() models.WorkerType {
	return models.WorkerTypeCompetitorAnalysis
}

// Init performs initialization for the competitor analysis step.
// Validates config and resolves API key.
func (w *CompetitorAnalysisWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for competitor_analysis")
	}

	// Extract target ASX code (required)
	asxCode, ok := stepConfig["asx_code"].(string)
	if !ok || asxCode == "" {
		return nil, fmt.Errorf("asx_code is required in step config")
	}
	asxCode = strings.ToUpper(asxCode)

	// Extract prompt for competitor identification
	prompt, _ := stepConfig["prompt"].(string)
	if prompt == "" {
		prompt = fmt.Sprintf("Identify the top 3-5 ASX-listed competitors for %s", asxCode)
	}

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
		Str("asx_code", asxCode).
		Str("prompt", prompt).
		Msg("Competitor analysis worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems:            []interfaces.WorkItem{},
		TotalCount:           0,                                   // Will be determined after LLM call
		Strategy:             interfaces.ProcessingStrategyInline, // Execute inline
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"asx_code":    asxCode,
			"prompt":      prompt,
			"api_key":     apiKey,
			"output_tags": outputTags,
			"period":      period,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs uses LLM to identify competitors, then fetches stock data inline.
func (w *CompetitorAnalysisWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize competitor_analysis worker: %w", err)
		}
	}

	asxCode, _ := initResult.Metadata["asx_code"].(string)
	prompt, _ := initResult.Metadata["prompt"].(string)
	apiKey, _ := initResult.Metadata["api_key"].(string)
	outputTags, _ := initResult.Metadata["output_tags"].([]string)
	period, _ := initResult.Metadata["period"].(string)

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("asx_code", asxCode).
		Str("step_id", stepID).
		Msg("Starting competitor analysis")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Analyzing competitors for ASX:%s", asxCode))
	}

	// Step 1: Use LLM to identify competitors
	competitors, err := w.identifyCompetitors(ctx, asxCode, prompt, apiKey)
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to identify competitors")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to identify competitors: %v", err))
		}
		return "", fmt.Errorf("failed to identify competitors: %w", err)
	}

	if len(competitors) == 0 {
		w.logger.Warn().Str("asx_code", asxCode).Msg("No competitors identified")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", "No competitors identified by LLM")
		}
		return stepID, nil
	}

	w.logger.Info().
		Str("asx_code", asxCode).
		Strs("competitors", competitors).
		Msg("Competitors identified, fetching stock data")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Identified %d competitors: %s", len(competitors), strings.Join(competitors, ", ")))
	}

	// Step 2: Fetch stock data for each competitor inline using ASXStockDataWorker
	successCount := 0
	for _, competitorCode := range competitors {
		err := w.fetchCompetitorStockData(ctx, competitorCode, period, outputTags, stepID, jobDef)
		if err != nil {
			w.logger.Warn().Err(err).Str("competitor", competitorCode).Msg("Failed to fetch stock data")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to fetch data for %s: %v", competitorCode, err))
			}
			continue
		}
		successCount++
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetched stock data for ASX:%s", competitorCode))
		}
	}

	if successCount == 0 {
		return "", fmt.Errorf("failed to fetch any competitor stock data")
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed: fetched stock data for %d/%d competitors", successCount, len(competitors)))
	}

	return stepID, nil
}

// fetchCompetitorStockData fetches stock data for a single competitor using ASXStockCollectorWorker
func (w *CompetitorAnalysisWorker) fetchCompetitorStockData(ctx context.Context, asxCode, period string, outputTags []string, stepID string, jobDef models.JobDefinition) error {
	// Create a synthetic step for the stock collector worker
	stockStep := models.JobStep{
		Name:        fmt.Sprintf("fetch_competitor_%s", strings.ToLower(asxCode)),
		Type:        models.WorkerTypeASXStockCollector,
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
func (w *CompetitorAnalysisWorker) identifyCompetitors(ctx context.Context, asxCode, prompt, apiKey string) ([]string, error) {
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
func (w *CompetitorAnalysisWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *CompetitorAnalysisWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("competitor_analysis step requires config")
	}

	asxCode, ok := step.Config["asx_code"].(string)
	if !ok || asxCode == "" {
		return fmt.Errorf("competitor_analysis step requires 'asx_code' in config")
	}

	return nil
}
