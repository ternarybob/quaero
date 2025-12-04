// -----------------------------------------------------------------------
// WebSearchWorker - Worker for Gemini-powered web search operations
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"google.golang.org/genai"
)

// WebSearchWorker handles web search operations using Gemini SDK with GoogleSearch grounding.
// This worker executes web search jobs synchronously (no child jobs).
type WebSearchWorker struct {
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager // For unified job logging
}

// Compile-time assertion: WebSearchWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*WebSearchWorker)(nil)

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

// NewWebSearchWorker creates a new web search worker
func NewWebSearchWorker(
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *WebSearchWorker {
	return &WebSearchWorker{
		documentStorage: documentStorage,
		eventService:    eventService,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeWebSearch for the DefinitionWorker interface
func (w *WebSearchWorker) GetType() models.WorkerType {
	return models.WorkerTypeWebSearch
}

// Init performs the initialization/setup phase for a web search step.
// This is where we:
//   - Extract and validate configuration (query, depth, breadth)
//   - Resolve API key from storage
//   - Return the search parameters as a single work item
//
// The Init phase does NOT perform the search - it only validates and prepares.
func (w *WebSearchWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
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

	// Get API key from step config
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
				Str("phase", "step").
				Str("step_name", step.Name).
				Str("api_key_name", cleanAPIKeyName).
				Msg("Resolved API key placeholder from storage")
		} else {
			apiKey = apiKeyValue
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for web_search")
	}

	w.logger.Info().
		Str("phase", "step").
		Str("step_name", step.Name).
		Str("query", query).
		Int("depth", depth).
		Int("breadth", breadth).
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
			"api_key":     apiKey,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs executes a web search using Gemini SDK with GoogleSearch grounding.
// Creates a document with the search results and source URLs.
// Returns the step job ID since web search executes synchronously.
// stepID is the ID of the step job - all jobs should have parent_id = stepID
// If initResult is provided, it uses the parameters from init.
func (w *WebSearchWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
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
	apiKey, _ := initResult.Metadata["api_key"].(string)

	w.logger.Info().
		Str("phase", "run").
		Str("originator", "worker").
		Str("step_name", step.Name).
		Str("query", query).
		Int("depth", depth).
		Int("breadth", breadth).
		Str("step_id", stepID).
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

	// Execute web search
	results, err := w.executeWebSearch(ctx, client, query, depth, breadth, stepID)
	if err != nil {
		w.logger.Error().Err(err).Str("query", query).Msg("Web search failed")
		w.logJobEvent(ctx, stepID, step.Name, "error",
			fmt.Sprintf("Web search failed: %v", err), nil)
		return "", fmt.Errorf("web search failed: %w", err)
	}

	// Create document from results
	doc, err := w.createDocument(results, query, &jobDef, stepID)
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
func (w *WebSearchWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for web search type
func (w *WebSearchWorker) ValidateConfig(step models.JobStep) error {
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
func (w *WebSearchWorker) executeWebSearch(ctx context.Context, client *genai.Client, query string, depth, breadth int, parentJobID string) (*WebSearchResults, error) {
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
	systemPrompt := fmt.Sprintf(`You are a research assistant. Search the web to answer the following query comprehensively.
Provide detailed information with specific facts, data, and sources.
Include all relevant URLs from your search.
If there are related topics worth exploring, suggest %d follow-up questions.

Query: %s`, breadth, query)

	// Execute initial search with timeout
	searchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	w.logger.Debug().
		Str("query", query).
		Str("parent_job_id", parentJobID).
		Msg("Executing Gemini web search")

	// Make the API call
	resp, err := client.Models.GenerateContent(
		searchCtx,
		"gemini-2.0-flash",
		[]*genai.Content{
			genai.NewContentFromText(systemPrompt, genai.RoleUser),
		},
		config,
	)
	if err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Initial search failed: %v", err))
		return results, err
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
		w.executeFollowUpSearches(searchCtx, client, config, results, depth-1, breadth, parentJobID)
	}

	return results, nil
}

// executeFollowUpSearches performs follow-up searches to explore the topic in depth
func (w *WebSearchWorker) executeFollowUpSearches(ctx context.Context, client *genai.Client, config *genai.GenerateContentConfig, results *WebSearchResults, remainingDepth, breadth int, parentJobID string) {
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
			"gemini-2.0-flash",
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

// createDocument creates a Document from the search results
func (w *WebSearchWorker) createDocument(results *WebSearchResults, query string, jobDef *models.JobDefinition, parentJobID string) (*models.Document, error) {
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

	// Build tags
	tags := []string{"web_search"}
	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
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

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "web_search",
		SourceID:        parentJobID,
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
func (w *WebSearchWorker) logJobEvent(ctx context.Context, parentJobID, _, level, message string, _ map[string]interface{}) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
