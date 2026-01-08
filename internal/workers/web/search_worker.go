// -----------------------------------------------------------------------
// SearchWorker - Worker for Gemini-powered web search operations
// -----------------------------------------------------------------------

package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
	"google.golang.org/genai"
)

// SearchWorker handles web search operations using Gemini SDK with GoogleSearch grounding.
// This worker executes web search jobs synchronously (no child jobs).
type SearchWorker struct {
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager // For unified job logging
	debugEnabled    bool
}

// Compile-time assertion: SearchWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*SearchWorker)(nil)

// WebSearchResults holds the results of a web search
type WebSearchResults struct {
	Query           string
	Content         string
	Sources         []WebSearchSource
	ResultCount     int
	SearchQueries   []string
	SearchDate      time.Time
	Depth           int
	Breadth         int
	Errors          []string
	FollowUpQueries []string
}

// WebSearchSource represents a source URL and title from the search
type WebSearchSource struct {
	URL   string
	Title string
}

// NewSearchWorker creates a new web search worker
func NewSearchWorker(
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *SearchWorker {
	return &SearchWorker{
		documentStorage: documentStorage,
		eventService:    eventService,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypeWebSearch for the DefinitionWorker interface
func (w *SearchWorker) GetType() models.WorkerType {
	return models.WorkerTypeWebSearch
}

// Init performs the initialization/setup phase for a web search step.
// This is where we:
//   - Extract and validate configuration (query, depth, breadth)
//   - Resolve API key from storage
//   - Return the search parameters as a single work item
//
// The Init phase does NOT perform the search - it only validates and prepares.
func (w *SearchWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for web_search")
	}

	// Extract query (required)
	query, ok := stepConfig["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required in step config")
	}

	// Extract depth (optional, default 1, max 10)
	depth := 1
	if d, ok := stepConfig["depth"].(float64); ok {
		depth = int(d)
	} else if d, ok := stepConfig["depth"].(int); ok {
		depth = d
	}
	if depth < 1 {
		depth = 1
	}
	if depth > 10 {
		depth = 10
	}

	// Extract breadth (optional, default 3, max 5)
	breadth := 3
	if b, ok := stepConfig["breadth"].(float64); ok {
		breadth = int(b)
	} else if b, ok := stepConfig["breadth"].(int); ok {
		breadth = b
	}
	if breadth < 1 {
		breadth = 1
	}
	if breadth > 5 {
		breadth = 5
	}

	// Extract model (optional, default gemini-3-flash-preview)
	model := "gemini-3-flash-preview"
	if m, ok := stepConfig["model"].(string); ok && m != "" {
		model = m
	}

	// Get API key from step config, fallback to global google_gemini_api_key
	var apiKey string
	if apiKeyValue, ok := stepConfig["api_key"].(string); ok && apiKeyValue != "" {
		// Check if it's still a placeholder (orchestrator failed to resolve)
		if len(apiKeyValue) > 2 && apiKeyValue[0] == '{' && apiKeyValue[len(apiKeyValue)-1] == '}' {
			// Try to resolve the placeholder manually
			cleanAPIKeyName := strings.Trim(apiKeyValue, "{}")
			resolvedAPIKey, err := common.ResolveAPIKey(ctx, w.kvStorage, cleanAPIKeyName, "")
			if err != nil {
				return nil, fmt.Errorf("failed to resolve API key '%s' from storage: %w", cleanAPIKeyName, err)
			}
			apiKey = resolvedAPIKey
			w.logger.Info().
				Str("phase", "init").
				Str("step_name", step.Name).
				Str("api_key_name", cleanAPIKeyName).
				Msg("Resolved API key placeholder from storage")
		} else {
			apiKey = apiKeyValue
		}
	} else {
		// No api_key in step config - try global google_gemini_api_key
		resolvedAPIKey, err := common.ResolveAPIKey(ctx, w.kvStorage, "google_gemini_api_key", "")
		if err != nil {
			return nil, fmt.Errorf("api_key not specified and failed to resolve google_gemini_api_key from storage: %w", err)
		}
		apiKey = resolvedAPIKey
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Msg("Using global google_gemini_api_key from storage")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("query", query).
		Int("depth", depth).
		Int("breadth", breadth).
		Str("model", model).
		Msg("Web search worker initialized")

	// Create a single work item representing the search
	workItems := []interfaces.WorkItem{
		{
			ID:   query,
			Name: fmt.Sprintf("Search: %s", query),
			Type: "search",
			Config: map[string]interface{}{
				"query":   query,
				"depth":   depth,
				"breadth": breadth,
			},
		},
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           1, // Single search operation
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"query":       query,
			"depth":       depth,
			"breadth":     breadth,
			"model":       model,
			"api_key":     apiKey,
			"step_config": stepConfig,
		},
	}, nil
}

// queryToSourceID generates a stable source ID from a query string
func (w *SearchWorker) queryToSourceID(query string) string {
	// Normalize: lowercase, trim whitespace
	normalized := strings.ToLower(strings.TrimSpace(query))
	// Hash for stable, URL-safe ID
	hash := sha256.Sum256([]byte(normalized))
	return "web_search:" + hex.EncodeToString(hash[:8]) // First 8 bytes = 16 chars
}

// isCacheFresh checks if a document was synced within the cache window
func (w *SearchWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// CreateJobs executes a web search using Gemini SDK with GoogleSearch grounding.
// Creates a document with the search results and source URLs.
// Returns the step job ID since web search executes synchronously.
// stepID is the ID of the step job - all jobs should have parent_id = stepID
// If initResult is provided, it uses the parameters from init.
func (w *SearchWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize web_search worker: %w", err)
		}
	}

	// Extract metadata from init result
	query, _ := initResult.Metadata["query"].(string)
	depth, _ := initResult.Metadata["depth"].(int)
	breadth, _ := initResult.Metadata["breadth"].(int)
	model, _ := initResult.Metadata["model"].(string)
	apiKey, _ := initResult.Metadata["api_key"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Default model fallback
	if model == "" {
		model = "gemini-3-flash-preview"
	}

	// Check cache settings
	cacheHours := 24
	if ch, ok := stepConfig["cache_hours"].(float64); ok {
		cacheHours = int(ch)
	}
	forceRefresh := false
	if fr, ok := stepConfig["force_refresh"].(bool); ok {
		forceRefresh = fr
	}

	// Check for cached web search results before executing search
	sourceID := w.queryToSourceID(query)
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource("web_search", sourceID)
		if err == nil && w.isCacheFresh(existingDoc, cacheHours) {
			w.logger.Info().
				Str("query", query).
				Str("source_id", sourceID).
				Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
				Int("cache_hours", cacheHours).
				Msg("Using cached web search results")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info",
					fmt.Sprintf("Using cached search results (last synced: %s)",
						existingDoc.LastSynced.Format("2006-01-02 15:04")))
			}
			return stepID, nil
		}
	}

	// Create debug info if enabled
	debug := workerutil.NewWorkerDebug("web_search", w.debugEnabled)

	w.logger.Info().
		Str("phase", "run").
		Str("originator", "worker").
		Str("step_name", step.Name).
		Str("query", query).
		Int("depth", depth).
		Int("breadth", breadth).
		Str("model", model).
		Str("step_id", stepID).
		Bool("force_refresh", forceRefresh).
		Msg("Starting web search from init result")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Starting web search: %s", query),
		map[string]interface{}{
			"depth":   depth,
			"breadth": breadth,
		})

	// Initialize Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		w.logJobEvent(ctx, stepID, step.Name, "error",
			fmt.Sprintf("Failed to create Gemini client: %v", err), nil)
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Execute web search with timing
	debug.StartPhase("ai_generation")
	results, err := w.executeWebSearch(ctx, client, query, depth, breadth, model, stepID)
	debug.EndPhase("ai_generation")
	if err != nil {
		w.logger.Error().Err(err).Str("query", query).Msg("Web search failed")
		w.logJobEvent(ctx, stepID, step.Name, "error",
			fmt.Sprintf("Web search failed: %v", err), nil)
		return "", fmt.Errorf("web search failed: %w", err)
	}

	// Record AI source info if results have it
	debug.RecordAISource("gemini", model, 0, 0) // Token counts not available from search API

	// Complete debug timing
	debug.Complete()

	// Create document from results (use stable sourceID for caching)
	doc, err := w.createDocument(ctx, results, query, &jobDef, stepID, sourceID, stepConfig, debug)
	if err != nil {
		return "", fmt.Errorf("failed to create document: %w", err)
	}

	// Save document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to save document: %w", err)
	}

	w.logger.Info().
		Str("document_id", doc.ID).
		Str("query", query).
		Int("result_count", results.ResultCount).
		Msg("Web search completed, document saved")

	// Log completion for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Web search completed: %d results, %d sources", results.ResultCount, len(results.Sources)),
		map[string]interface{}{
			"document_id":  doc.ID,
			"result_count": results.ResultCount,
			"source_count": len(results.Sources),
		})

	// Log document saved via Job Manager's unified logging
	if w.jobMgr != nil && stepID != "" {
		message := fmt.Sprintf("Document saved: %s (ID: %s)", doc.Title, doc.ID[:8])
		w.jobMgr.AddJobLog(context.Background(), stepID, "info", message)
	}

	return stepID, nil
}

// ReturnsChildJobs returns false since web search executes synchronously
func (w *SearchWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for web search type
func (w *SearchWorker) ValidateConfig(step models.JobStep) error {
	// Validate step config exists
	if step.Config == nil {
		return fmt.Errorf("web_search step requires config")
	}

	// Validate required query field
	query, ok := step.Config["query"].(string)
	if !ok || query == "" {
		return fmt.Errorf("web_search step requires 'query' in config")
	}

	// Validate optional depth field
	if depth, ok := step.Config["depth"].(float64); ok {
		if depth < 1 || depth > 10 {
			return fmt.Errorf("web_search step depth must be between 1 and 10, got: %.0f", depth)
		}
	} else if depth, ok := step.Config["depth"].(int); ok {
		if depth < 1 || depth > 10 {
			return fmt.Errorf("web_search step depth must be between 1 and 10, got: %d", depth)
		}
	}

	// Validate optional breadth field
	if breadth, ok := step.Config["breadth"].(float64); ok {
		if breadth < 1 || breadth > 5 {
			return fmt.Errorf("web_search step breadth must be between 1 and 5, got: %.0f", breadth)
		}
	} else if breadth, ok := step.Config["breadth"].(int); ok {
		if breadth < 1 || breadth > 5 {
			return fmt.Errorf("web_search step breadth must be between 1 and 5, got: %d", breadth)
		}
	}

	return nil
}

// executeWebSearch performs the web search using Gemini SDK with GoogleSearch grounding
func (w *SearchWorker) executeWebSearch(ctx context.Context, client *genai.Client, query string, depth, breadth int, model, parentJobID string) (*WebSearchResults, error) {
	results := &WebSearchResults{
		Query:      query,
		SearchDate: time.Now(),
		Depth:      depth,
		Breadth:    breadth,
	}

	// Configure search tool
	searchTool := &genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{searchTool},
	}

	// Build system prompt for structured response
	// Include current date so AI knows temporal context for "latest", "current", "recent" queries
	currentDate := time.Now().Format("January 2, 2006")
	systemPrompt := fmt.Sprintf(`You are a research assistant. Today's date is %s.
Search the web to answer the following query comprehensively.
Provide detailed information with specific facts, data, and sources.
When searching for "latest", "current", or "recent" information, prioritize results from %d onwards.
Include all relevant URLs from your search.
If there are related topics worth exploring, suggest %d follow-up questions.

Query: %s`, currentDate, time.Now().Year(), breadth, query)

	// Execute initial search with timeout
	searchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	w.logger.Debug().
		Str("query", query).
		Str("parent_job_id", parentJobID).
		Msg("Executing Gemini web search")

	// Make the API call with retry logic for rate limiting
	// Uses 45-60 second backoffs to respect Gemini quota windows
	var resp *genai.GenerateContentResponse
	var apiErr error

	retryConfig := llm.NewDefaultRetryConfig()

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		resp, apiErr = client.Models.GenerateContent(
			searchCtx,
			model,
			[]*genai.Content{
				genai.NewContentFromText(systemPrompt, genai.RoleUser),
			},
			config,
		)

		if apiErr == nil {
			break
		}

		// Check for quota exhaustion (limit: 0) - fail fast, don't retry
		if llm.IsQuotaExhaustedError(apiErr) {
			w.logger.Error().
				Str("query", query).
				Err(apiErr).
				Msg("Quota exhausted (limit: 0) - check billing/plan for this model")
			results.Errors = append(results.Errors, "Quota exhausted (limit: 0) - check billing/plan for this model")
			return results, fmt.Errorf("quota exhausted (limit: 0): %w", apiErr)
		}

		if attempt == retryConfig.MaxRetries {
			break
		}

		// Calculate backoff - use API-provided delay for rate limit errors
		var backoff time.Duration
		if llm.IsRateLimitError(apiErr) {
			apiDelay := llm.ExtractRetryDelay(apiErr)
			backoff = retryConfig.CalculateBackoff(attempt, apiDelay)
			w.logger.Warn().
				Int("attempt", attempt+1).
				Dur("backoff", backoff).
				Dur("api_delay", apiDelay).
				Str("query", query).
				Err(apiErr).
				Msg("Rate limit hit during web search, waiting before retry")
		} else {
			// Non-rate-limit errors: shorter backoff
			backoff = time.Duration(attempt+1) * 2 * time.Second
			w.logger.Warn().
				Int("attempt", attempt+1).
				Dur("backoff", backoff).
				Str("query", query).
				Err(apiErr).
				Msg("Retrying web search")
		}

		select {
		case <-searchCtx.Done():
			results.Errors = append(results.Errors, fmt.Sprintf("Context cancelled during retry: %v", searchCtx.Err()))
			return results, searchCtx.Err()
		case <-time.After(backoff):
			// Continue to next retry attempt
		}
	}

	if apiErr != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Search failed after %d retries: %v", retryConfig.MaxRetries, apiErr))
		return results, apiErr
	}

	// Extract response content
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				results.Content = part.Text
				results.ResultCount++
			}
		}

		// Extract grounding metadata
		if resp.Candidates[0].GroundingMetadata != nil {
			gm := resp.Candidates[0].GroundingMetadata

			// Get search queries used
			if gm.WebSearchQueries != nil {
				results.SearchQueries = gm.WebSearchQueries
			}

			// Get grounding chunks (sources)
			if gm.GroundingChunks != nil {
				for _, chunk := range gm.GroundingChunks {
					if chunk.Web != nil {
						results.Sources = append(results.Sources, WebSearchSource{
							URL:   chunk.Web.URI,
							Title: chunk.Web.Title,
						})
					}
				}
			}
		}
	}

	// Execute follow-up searches if depth > 1
	if depth > 1 && len(results.SearchQueries) > 0 {
		w.executeFollowUpSearches(searchCtx, client, config, results, depth-1, breadth, model, parentJobID)
	}

	return results, nil
}

// executeFollowUpSearches performs follow-up searches to explore the topic in depth
func (w *SearchWorker) executeFollowUpSearches(ctx context.Context, client *genai.Client, config *genai.GenerateContentConfig, results *WebSearchResults, remainingDepth, breadth int, model, parentJobID string) {
	if remainingDepth <= 0 {
		return
	}

	// Use search queries as follow-up topics
	followUpQueries := results.SearchQueries
	if len(followUpQueries) > breadth {
		followUpQueries = followUpQueries[:breadth]
	}

	for _, followUpQuery := range followUpQueries {
		w.logger.Debug().
			Str("follow_up_query", followUpQuery).
			Int("remaining_depth", remainingDepth).
			Str("parent_job_id", parentJobID).
			Msg("Executing follow-up web search")

		resp, err := client.Models.GenerateContent(
			ctx,
			model,
			[]*genai.Content{
				genai.NewContentFromText(fmt.Sprintf("Provide additional details on: %s", followUpQuery), genai.RoleUser),
			},
			config,
		)
		if err != nil {
			results.Errors = append(results.Errors, fmt.Sprintf("Follow-up search for '%s' failed: %v", followUpQuery, err))
			continue
		}

		// Extract and append content
		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				if part.Text != "" {
					results.Content += fmt.Sprintf("\n\n## Additional Information: %s\n\n%s", followUpQuery, part.Text)
					results.ResultCount++
				}
			}

			// Extract additional sources
			if resp.Candidates[0].GroundingMetadata != nil && resp.Candidates[0].GroundingMetadata.GroundingChunks != nil {
				for _, chunk := range resp.Candidates[0].GroundingMetadata.GroundingChunks {
					if chunk.Web != nil {
						results.Sources = append(results.Sources, WebSearchSource{
							URL:   chunk.Web.URI,
							Title: chunk.Web.Title,
						})
					}
				}
			}
		}

		results.FollowUpQueries = append(results.FollowUpQueries, followUpQuery)
	}
}

// createDocument creates a Document from the search results.
// sourceID is a stable identifier based on the query hash (for caching).
func (w *SearchWorker) createDocument(ctx context.Context, results *WebSearchResults, query string, jobDef *models.JobDefinition, parentJobID string, sourceID string, stepConfig map[string]interface{}, debug *workerutil.WorkerDebugInfo) (*models.Document, error) {
	// Build markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Web Search Results: %s\n\n", query))
	content.WriteString(fmt.Sprintf("*Search performed: %s*\n\n", results.SearchDate.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("*Depth: %d | Breadth: %d | Results: %d*\n\n", results.Depth, results.Breadth, results.ResultCount))

	// Main content
	content.WriteString("## Summary\n\n")
	content.WriteString(results.Content)
	content.WriteString("\n\n")

	// Sources section
	if len(results.Sources) > 0 {
		content.WriteString("## Sources\n\n")
		seen := make(map[string]bool)
		for _, source := range results.Sources {
			if !seen[source.URL] {
				seen[source.URL] = true
				if source.Title != "" {
					content.WriteString(fmt.Sprintf("- [%s](%s)\n", source.Title, source.URL))
				} else {
					content.WriteString(fmt.Sprintf("- <%s>\n", source.URL))
				}
			}
		}
		content.WriteString("\n")
	}

	// Search queries used
	if len(results.SearchQueries) > 0 {
		content.WriteString("## Search Queries Used\n\n")
		for _, sq := range results.SearchQueries {
			content.WriteString(fmt.Sprintf("- %s\n", sq))
		}
		content.WriteString("\n")
	}

	// Errors if any
	if len(results.Errors) > 0 {
		content.WriteString("## Errors\n\n")
		for _, err := range results.Errors {
			content.WriteString(fmt.Sprintf("- %s\n", err))
		}
		content.WriteString("\n")
	}

	// Append debug info to markdown if enabled
	if debug != nil && debug.IsEnabled() {
		content.WriteString(debug.ToMarkdown())
	}

	// Build tags
	tags := []string{"web-search"}
	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}

	// Add date tag for filtering (format: date:YYYY-MM-DD)
	dateTag := fmt.Sprintf("date:%s", results.SearchDate.Format("2006-01-02"))
	tags = append(tags, dateTag)

	// Add output_tags from step config (allows downstream steps to find this document)
	if stepConfig != nil {
		if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range outputTags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					tags = append(tags, tagStr)
				}
			}
		} else if outputTags, ok := stepConfig["output_tags"].([]string); ok {
			tags = append(tags, outputTags...)
		}
	}

	// Add cache tags from context (for caching/deduplication)
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	// Build metadata
	metadata := map[string]interface{}{
		"query":           query,
		"search_date":     results.SearchDate.Format(time.RFC3339),
		"result_count":    results.ResultCount,
		"depth":           results.Depth,
		"breadth":         results.Breadth,
		"source_count":    len(results.Sources),
		"search_queries":  results.SearchQueries,
		"follow_up_count": len(results.FollowUpQueries),
		"parent_job_id":   parentJobID,
		"has_errors":      len(results.Errors) > 0,
		"error_count":     len(results.Errors),
	}
	if len(results.Errors) > 0 {
		metadata["errors"] = results.Errors
	}

	// Add worker debug metadata if enabled
	if debug != nil && debug.IsEnabled() {
		metadata["worker_debug"] = debug.ToMetadata()
	}

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "web_search",
		SourceID:        sourceID, // Stable ID for caching
		Title:           fmt.Sprintf("Web Search: %s", query),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		Metadata:        metadata,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}

	return doc, nil
}

// logJobEvent logs a job event for real-time UI display using the unified logging system
func (w *SearchWorker) logJobEvent(ctx context.Context, parentJobID, _, level, message string, _ map[string]interface{}) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
