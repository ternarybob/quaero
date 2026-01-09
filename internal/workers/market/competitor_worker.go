// -----------------------------------------------------------------------
// CompetitorWorker - Identifies ASX-listed competitors for a target company
// Uses LLM (Gemini) to analyze and return competitor tickers with rationale
// NO stock data collection - just competitor identification and reasoning
// -----------------------------------------------------------------------

package market

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
	"google.golang.org/genai"
)

// CompetitorOutput is the schema for JSON output
type CompetitorOutput struct {
	Schema       string             `json:"$schema"`
	TargetTicker string             `json:"target_ticker"`
	TargetCode   string             `json:"target_code"`
	AnalyzedAt   string             `json:"analyzed_at"`
	Competitors  []CompetitorEntry  `json:"competitors"`
	WorkerDebug  *WorkerDebugOutput `json:"worker_debug,omitempty"`
}

// CompetitorEntry represents a single competitor with rationale
type CompetitorEntry struct {
	Code      string `json:"code"`
	Rationale string `json:"rationale"`
}

// CompetitorWorker identifies ASX-listed competitors for a target company.
// Uses LLM to analyze and return competitor tickers with comparison rationale.
type CompetitorWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	jobMgr          *queue.Manager
	logger          arbor.ILogger
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*CompetitorWorker)(nil)

// NewCompetitorWorker creates a new competitor analysis worker
func NewCompetitorWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	jobMgr *queue.Manager,
	logger arbor.ILogger,
	debugEnabled bool,
) *CompetitorWorker {
	return &CompetitorWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		jobMgr:          jobMgr,
		logger:          logger,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypeMarketCompetitor
func (w *CompetitorWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketCompetitor
}

// Init performs initialization for the competitor analysis step.
// Validates config and resolves API key.
// Supports both step config and job-level variables for ticker configuration
func (w *CompetitorWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
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
			"step_config":     stepConfig,
		},
	}, nil
}

// CreateJobs uses LLM to identify competitors and stores the analysis document.
// Supports multiple target tickers - processes each sequentially.
func (w *CompetitorWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize competitor_analysis worker: %w", err)
		}
	}

	// Get manager_id for document isolation across pipeline steps
	// All documents from the same pipeline run share the same manager_id
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	// Get tickers from metadata
	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	promptTemplate, _ := initResult.Metadata["prompt_template"].(string)
	apiKey, _ := initResult.Metadata["api_key"].(string)
	outputTags, _ := initResult.Metadata["output_tags"].([]string)

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
		err := w.processTicker(ctx, ticker.Code, promptTemplate, apiKey, outputTags, stepID, managerID)
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
func (w *CompetitorWorker) processTicker(ctx context.Context, asxCode, promptTemplate, apiKey string, outputTags []string, stepID, managerID string) error {
	ticker := common.Ticker{Exchange: "ASX", Code: asxCode}

	// Initialize debug info
	debug := workerutil.NewWorkerDebug(models.WorkerTypeMarketCompetitor.String(), w.debugEnabled)
	debug.SetTicker(ticker.String())
	debug.SetJobID(stepID) // Include job ID in debug output
	defer func() {
		debug.Complete()
	}()

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

	// Use LLM to identify competitors with rationale
	debug.StartPhase("ai_generation")
	competitors, _, err := w.identifyCompetitors(ctx, asxCode, prompt, apiKey)
	debug.EndPhase("ai_generation")
	if err != nil {
		w.logger.Error().Err(err).Str("asx_code", asxCode).Msg("Failed to identify competitors")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("ASX:%s - Failed to identify competitors: %v", asxCode, err))
		}
		debug.CompleteWithError(err)
		return fmt.Errorf("failed to identify competitors for %s: %w", asxCode, err)
	}

	if len(competitors) == 0 {
		w.logger.Warn().Str("asx_code", asxCode).Msg("No competitors identified")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("ASX:%s - No competitors identified by LLM", asxCode))
		}
	}

	// Log competitors found
	competitorCodes := make([]string, len(competitors))
	for i, c := range competitors {
		competitorCodes[i] = c.Code
	}
	w.logger.Info().
		Str("asx_code", asxCode).
		Strs("competitors", competitorCodes).
		Msg("Competitors identified")

	if w.jobMgr != nil && len(competitors) > 0 {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("ASX:%s - Identified %d competitors: %s", asxCode, len(competitors), strings.Join(competitorCodes, ", ")))
	}

	// Create and save document
	doc := w.createDocument(ticker, competitors, outputTags, debug, managerID)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		debug.CompleteWithError(err)
		return fmt.Errorf("failed to save document: %w", err)
	}

	w.logger.Info().
		Str("code", asxCode).
		Int("competitor_count", len(competitors)).
		Str("doc_id", doc.ID).
		Msg("Saved competitor analysis document")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("ASX:%s - Saved competitor analysis with %d competitors", asxCode, len(competitors)))
	}

	return nil
}

// createDocument creates a document containing competitor analysis with schema output
func (w *CompetitorWorker) createDocument(ticker common.Ticker, competitors []CompetitorEntry, outputTags []string, debug *workerutil.WorkerDebugInfo, managerID string) *models.Document {
	// Build tags
	tags := []string{
		"competitor-analysis",
		strings.ToLower(ticker.Code),
		fmt.Sprintf("ticker:%s", ticker.String()),
		fmt.Sprintf("source_type:%s", models.WorkerTypeMarketCompetitor.String()),
	}
	tags = append(tags, outputTags...)

	// Build worker debug output
	var workerDebug *WorkerDebugOutput
	if debug != nil && debug.IsEnabled() {
		debug.Complete() // Ensure timing is captured
		debugMeta := debug.ToMetadata()
		if debugMeta != nil {
			workerDebug = &WorkerDebugOutput{
				WorkerType: models.WorkerTypeMarketCompetitor.String(),
				Ticker:     ticker.String(),
			}
			if startedAt, ok := debugMeta["started_at"].(string); ok {
				workerDebug.StartedAt = startedAt
			}
			if completedAt, ok := debugMeta["completed_at"].(string); ok {
				workerDebug.CompletedAt = completedAt
			}
			if timing, ok := debugMeta["timing"].(map[string]interface{}); ok {
				if totalMs, ok := timing["total_ms"].(int64); ok {
					workerDebug.Timing.TotalMs = totalMs
				}
				// AI generation time is captured in api_fetch_ms for simplicity
				// (the LLM call is the "API fetch" for this worker)
				if aiGenMs, ok := timing["ai_generation_ms"].(int64); ok {
					workerDebug.Timing.APIFetchMs = aiGenMs
				}
			}
		}
	}

	// Build schema output
	output := CompetitorOutput{
		Schema:       "quaero/competitor/v1",
		TargetTicker: ticker.String(),
		TargetCode:   ticker.Code,
		AnalyzedAt:   time.Now().Format(time.RFC3339),
		Competitors:  competitors,
		WorkerDebug:  workerDebug,
	}

	// Convert to map for document metadata
	outputJSON, _ := json.Marshal(output)
	var metadata map[string]interface{}
	json.Unmarshal(outputJSON, &metadata)

	// Build Jobs array for job isolation (required by downstream workers like output_formatter)
	// Use managerID so all steps in the same pipeline can find this document
	var jobs []string
	if managerID != "" {
		jobs = []string{managerID}
	}

	// Build content markdown
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# Competitor Analysis - %s\n\n", ticker.Code))
	contentBuilder.WriteString(fmt.Sprintf("**Target:** %s\n", ticker.String()))
	contentBuilder.WriteString(fmt.Sprintf("**Analyzed:** %s\n", time.Now().Format(time.RFC3339)))
	contentBuilder.WriteString(fmt.Sprintf("**Competitors Found:** %d\n\n", len(competitors)))

	// Competitors table
	contentBuilder.WriteString("## Competitors\n\n")
	if len(competitors) > 0 {
		contentBuilder.WriteString("| Code | Rationale |\n")
		contentBuilder.WriteString("|------|-----------|\n")
		for _, c := range competitors {
			// Escape pipe characters in rationale
			rationale := strings.ReplaceAll(c.Rationale, "|", "\\|")
			contentBuilder.WriteString(fmt.Sprintf("| %s | %s |\n", c.Code, rationale))
		}
	} else {
		contentBuilder.WriteString("*No competitors identified*\n")
	}

	// Generate document ID early so it can be included in debug info
	docID := uuid.New().String()
	if debug != nil {
		debug.SetDocumentID(docID) // Include document ID in debug output
	}

	// Add Worker Debug section to markdown
	if debug != nil && debug.IsEnabled() {
		contentBuilder.WriteString(debug.ToMarkdown())
	}

	return &models.Document{
		ID:              docID,
		SourceType:      "competitor-analysis",
		SourceID:        fmt.Sprintf("%s:%s:competitor-analysis", ticker.Exchange, ticker.Code),
		Title:           fmt.Sprintf("Competitor Analysis - %s", ticker.Code),
		ContentMarkdown: contentBuilder.String(),
		Tags:            tags,
		Jobs:            jobs,
		Metadata:        metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// identifyCompetitors uses Gemini to identify competitor ASX codes with rationale.
// Uses schema-constrained output (ResponseSchema) to ensure structured JSON response.
// Returns the competitors and the actual prompt sent to the LLM.
func (w *CompetitorWorker) identifyCompetitors(ctx context.Context, asxCode, prompt, apiKey string) ([]CompetitorEntry, string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Build prompt for competitor identification - schema enforces JSON structure
	systemPrompt := fmt.Sprintf(`You are a financial analyst identifying ASX-listed competitors.

Target Company: ASX:%s

Task: %s

Requirements:
- Do NOT include the target company (%s) in the list
- Only include companies actually listed on the ASX
- Provide a clear rationale for why each company competes with the target`, asxCode, prompt, asxCode)

	// Execute with timeout
	llmCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Use schema-constrained output - Gemini enforces JSON structure
	config := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr(float32(0.2)),
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"code":      {Type: genai.TypeString, Description: "ASX ticker code (e.g., RIO, FMG, S32)"},
					"rationale": {Type: genai.TypeString, Description: "Why this company is a competitor to the target"},
				},
				Required: []string{"code", "rationale"},
			},
		},
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
		return nil, systemPrompt, fmt.Errorf("Gemini API call failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, systemPrompt, fmt.Errorf("no response from Gemini API")
	}

	responseText := resp.Text()
	if responseText == "" {
		return nil, systemPrompt, fmt.Errorf("empty response from Gemini API")
	}

	w.logger.Debug().
		Str("asx_code", asxCode).
		Str("response", responseText).
		Msg("LLM competitor identification response")

	// Parse JSON array of competitor entries (schema guarantees correct structure)
	competitors, err := parseCompetitorEntries(responseText, asxCode)
	if err != nil {
		return nil, systemPrompt, fmt.Errorf("failed to parse competitor entries: %w", err)
	}

	return competitors, systemPrompt, nil
}

// parseCompetitorEntries extracts CompetitorEntry objects from schema-constrained LLM response.
// With ResponseSchema enforcement, the response is guaranteed to be a valid JSON array.
func parseCompetitorEntries(response, targetCode string) ([]CompetitorEntry, error) {
	response = strings.TrimSpace(response)

	// Parse JSON array of objects (schema guarantees this structure)
	var entries []CompetitorEntry
	if err := json.Unmarshal([]byte(response), &entries); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Filter and validate entries
	return filterValidEntries(entries, targetCode), nil
}

// filterValidEntries removes invalid entries and the target code
func filterValidEntries(entries []CompetitorEntry, targetCode string) []CompetitorEntry {
	var valid []CompetitorEntry
	seen := make(map[string]bool)

	for _, entry := range entries {
		code := strings.ToUpper(strings.TrimSpace(entry.Code))
		if isValidASXCode(code) && code != targetCode && !seen[code] {
			seen[code] = true
			valid = append(valid, CompetitorEntry{
				Code:      code,
				Rationale: entry.Rationale,
			})
		}
	}

	return valid
}

// isValidASXCode checks if a code is a valid ASX code format
func isValidASXCode(code string) bool {
	if len(code) < 3 || len(code) > 4 {
		return false
	}
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// ReturnsChildJobs returns false - we execute inline, not via child jobs
func (w *CompetitorWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
// Config can be nil if tickers will be provided via job-level variables.
func (w *CompetitorWorker) ValidateConfig(step models.JobStep) error {
	// Config is optional - tickers can come from job-level variables
	// Full validation happens in Init() when we have access to jobDef
	return nil
}
