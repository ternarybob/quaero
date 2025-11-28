package managers

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
	"google.golang.org/genai"
)

// WebSearchManager orchestrates web search jobs using Gemini SDK with GoogleSearch grounding
type WebSearchManager struct {
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
}

// Compile-time assertion: WebSearchManager implements StepManager interface
var _ interfaces.StepManager = (*WebSearchManager)(nil)

// NewWebSearchManager creates a new web search manager for orchestrating Gemini-powered web searches
func NewWebSearchManager(
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
) *WebSearchManager {
	return &WebSearchManager{
		documentStorage: documentStorage,
		eventService:    eventService,
		kvStorage:       kvStorage,
		logger:          logger,
	}
}

// CreateParentJob executes a web search using Gemini SDK with GoogleSearch grounding.
// Creates a document with the search results and source URLs.
// Returns the parent job ID since web search executes synchronously.
func (m *WebSearchManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return "", fmt.Errorf("step config is required for web_search")
	}

	// Extract query (required)
	query, ok := stepConfig["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required in step config")
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
	// The orchestrator already resolves {placeholders} before calling CreateParentJob,
	// so the api_key value should already be the actual key, not a placeholder
	var apiKey string
	if apiKeyValue, ok := stepConfig["api_key"].(string); ok && apiKeyValue != "" {
		// Check if it's still a placeholder (orchestrator failed to resolve)
		if len(apiKeyValue) > 2 && apiKeyValue[0] == '{' && apiKeyValue[len(apiKeyValue)-1] == '}' {
			// Try to resolve the placeholder manually
			cleanAPIKeyName := strings.Trim(apiKeyValue, "{}")
			resolvedAPIKey, err := common.ResolveAPIKey(ctx, m.kvStorage, cleanAPIKeyName, "")
			if err != nil {
				return "", fmt.Errorf("failed to resolve API key '%s' from storage: %w", cleanAPIKeyName, err)
			}
			apiKey = resolvedAPIKey
			m.logger.Info().
				Str("step_name", step.Name).
				Str("api_key_name", cleanAPIKeyName).
				Msg("Resolved API key placeholder from storage")
		} else {
			// Use the already-resolved API key value directly
			apiKey = apiKeyValue
			m.logger.Debug().
				Str("step_name", step.Name).
				Msg("Using pre-resolved API key from orchestrator")
		}
	}

	if apiKey == "" {
		return "", fmt.Errorf("api_key is required for web_search")
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("query", query).
		Int("depth", depth).
		Int("breadth", breadth).
		Str("parent_job_id", parentJobID).
		Msg("Starting web search")

	// Initialize Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Execute web search
	results, err := m.executeWebSearch(ctx, client, query, depth, breadth, parentJobID)
	if err != nil {
		m.logger.Error().Err(err).Str("query", query).Msg("Web search failed")
		return "", fmt.Errorf("web search failed: %w", err)
	}

	// Create document from results
	doc, err := m.createDocument(results, query, jobDef, parentJobID)
	if err != nil {
		return "", fmt.Errorf("failed to create document: %w", err)
	}

	// Save document
	if err := m.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to save document: %w", err)
	}

	m.logger.Info().
		Str("document_id", doc.ID).
		Str("query", query).
		Int("result_count", results.ResultCount).
		Msg("Web search completed, document saved")

	// Publish document saved event for parent job document count tracking
	if m.eventService != nil && parentJobID != "" {
		payload := map[string]interface{}{
			"job_id":        parentJobID,
			"parent_job_id": parentJobID,
			"document_id":   doc.ID,
			"source_type":   "web_search",
			"timestamp":     time.Now().Format(time.RFC3339),
		}
		event := interfaces.Event{
			Type:    interfaces.EventDocumentSaved,
			Payload: payload,
		}
		if err := m.eventService.PublishSync(context.Background(), event); err != nil {
			m.logger.Warn().
				Err(err).
				Str("document_id", doc.ID).
				Str("parent_job_id", parentJobID).
				Msg("Failed to publish document_saved event")
		}
	}

	return parentJobID, nil
}

// GetManagerType returns "web_search" - the action type this manager handles
func (m *WebSearchManager) GetManagerType() string {
	return "web_search"
}

// ReturnsChildJobs returns false - web search executes synchronously
func (m *WebSearchManager) ReturnsChildJobs() bool {
	return false
}

// WebSearchResults holds the collected search results
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

// executeWebSearch performs the web search using Gemini SDK with GoogleSearch grounding
func (m *WebSearchManager) executeWebSearch(ctx context.Context, client *genai.Client, query string, depth, breadth int, parentJobID string) (*WebSearchResults, error) {
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

	m.logger.Debug().
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
		m.executeFollowUpSearches(searchCtx, client, config, results, depth-1, breadth, parentJobID)
	}

	return results, nil
}

// executeFollowUpSearches performs follow-up searches to explore the topic in depth
func (m *WebSearchManager) executeFollowUpSearches(ctx context.Context, client *genai.Client, config *genai.GenerateContentConfig, results *WebSearchResults, remainingDepth, breadth int, parentJobID string) {
	if remainingDepth <= 0 {
		return
	}

	// Use search queries as follow-up topics
	followUpQueries := results.SearchQueries
	if len(followUpQueries) > breadth {
		followUpQueries = followUpQueries[:breadth]
	}

	for _, followUpQuery := range followUpQueries {
		m.logger.Debug().
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
func (m *WebSearchManager) createDocument(results *WebSearchResults, query string, jobDef *models.JobDefinition, parentJobID string) (*models.Document, error) {
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
		"query":            query,
		"search_date":      results.SearchDate.Format(time.RFC3339),
		"result_count":     results.ResultCount,
		"depth":            results.Depth,
		"breadth":          results.Breadth,
		"source_count":     len(results.Sources),
		"search_queries":   results.SearchQueries,
		"follow_up_count":  len(results.FollowUpQueries),
		"parent_job_id":    parentJobID,
		"has_errors":       len(results.Errors) > 0,
		"error_count":      len(results.Errors),
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
