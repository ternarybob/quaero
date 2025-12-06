// -----------------------------------------------------------------------
// SummaryWorker - Generates summaries from tagged documents
// Aggregates all documents matching filter tags and generates a comprehensive summary
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

// SummaryWorker handles corpus summary generation from tagged documents.
// This worker executes synchronously (no child jobs) and creates a single
// summary document from all documents matching the filter criteria.
type SummaryWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: SummaryWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*SummaryWorker)(nil)

// NewSummaryWorker creates a new summary worker
func NewSummaryWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *SummaryWorker {
	return &SummaryWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		eventService:    eventService,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeSummary for the DefinitionWorker interface
func (w *SummaryWorker) GetType() models.WorkerType {
	return models.WorkerTypeSummary
}

// Init performs the initialization/setup phase for a summary step.
// This is where we:
//   - Extract and validate configuration (prompt, filter_tags)
//   - Resolve API key from storage
//   - Query documents matching the filter criteria
//   - Return the document list as metadata for CreateJobs
//
// The Init phase does NOT generate the summary - it only validates and prepares.
func (w *SummaryWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for summary")
	}

	// Extract prompt (required) - natural language instruction for the summary
	prompt, ok := stepConfig["prompt"].(string)
	if !ok || prompt == "" {
		return nil, fmt.Errorf("prompt is required in step config")
	}

	// Extract filter_tags (required) - documents to include in summary
	var filterTags []string
	if tags, ok := stepConfig["filter_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				filterTags = append(filterTags, tagStr)
			}
		}
	} else if tags, ok := stepConfig["filter_tags"].([]string); ok {
		filterTags = tags
	}

	if len(filterTags) == 0 {
		return nil, fmt.Errorf("filter_tags is required in step config")
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
				Str("phase", "init").
				Str("step_name", step.Name).
				Str("api_key_name", cleanAPIKeyName).
				Msg("Resolved API key placeholder from storage")
		} else {
			apiKey = apiKeyValue
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for summary")
	}

	// Query documents matching filter tags
	opts := interfaces.SearchOptions{
		Tags:  filterTags,
		Limit: 1000, // Maximum documents to include in summary
	}

	documents, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}

	if len(documents) == 0 {
		return nil, fmt.Errorf("no documents found matching tags: %v", filterTags)
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("document_count", len(documents)).
		Strs("filter_tags", filterTags).
		Msg("Summary worker initialized - found documents")

	// Create work items from documents (for reference in metadata)
	workItems := make([]interfaces.WorkItem, len(documents))
	for i, doc := range documents {
		workItems[i] = interfaces.WorkItem{
			ID:   doc.ID,
			Name: doc.Title,
			Type: "document",
			Config: map[string]interface{}{
				"document_id": doc.ID,
				"title":       doc.Title,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(documents),
		Strategy:             interfaces.ProcessingStrategyInline, // Synchronous execution
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"prompt":      prompt,
			"filter_tags": filterTags,
			"api_key":     apiKey,
			"documents":   documents,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs generates a summary from all documents and saves it as a new document.
// This executes synchronously - no child jobs are created.
// Returns the step job ID since summary executes synchronously.
func (w *SummaryWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize summary worker: %w", err)
		}
	}

	// Extract metadata from init result
	prompt, _ := initResult.Metadata["prompt"].(string)
	filterTags, _ := initResult.Metadata["filter_tags"].([]string)
	apiKey, _ := initResult.Metadata["api_key"].(string)
	documents, _ := initResult.Metadata["documents"].([]*models.Document)

	w.logger.Info().
		Str("phase", "run").
		Str("originator", "worker").
		Str("step_name", step.Name).
		Int("document_count", len(documents)).
		Str("step_id", stepID).
		Msg("Starting summary generation from init result")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Generating summary from %d documents", len(documents)),
		map[string]interface{}{
			"filter_tags": filterTags,
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

	// Generate summary
	summaryContent, err := w.generateSummary(ctx, client, prompt, documents, stepID)
	if err != nil {
		w.logger.Error().Err(err).Str("step_name", step.Name).Msg("Summary generation failed")
		w.logJobEvent(ctx, stepID, step.Name, "error",
			fmt.Sprintf("Summary generation failed: %v", err), nil)
		return "", fmt.Errorf("summary generation failed: %w", err)
	}

	// Create summary document
	doc, err := w.createDocument(summaryContent, prompt, documents, &jobDef, stepID)
	if err != nil {
		return "", fmt.Errorf("failed to create document: %w", err)
	}

	// Save document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to save document: %w", err)
	}

	w.logger.Info().
		Str("document_id", doc.ID).
		Int("source_document_count", len(documents)).
		Msg("Summary completed, document saved")

	// Log completion for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Summary completed: document saved with ID %s", doc.ID[:8]),
		map[string]interface{}{
			"document_id":           doc.ID,
			"source_document_count": len(documents),
		})

	// Log document saved via Job Manager's unified logging
	if w.jobMgr != nil && stepID != "" {
		message := fmt.Sprintf("Summary document saved: %s (ID: %s)", doc.Title, doc.ID[:8])
		w.jobMgr.AddJobLog(context.Background(), stepID, "info", message)
	}

	return stepID, nil
}

// ReturnsChildJobs returns false since summary executes synchronously
func (w *SummaryWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for summary type
func (w *SummaryWorker) ValidateConfig(step models.JobStep) error {
	// Validate step config exists
	if step.Config == nil {
		return fmt.Errorf("summary step requires config")
	}

	// Validate required prompt field
	prompt, ok := step.Config["prompt"].(string)
	if !ok || prompt == "" {
		return fmt.Errorf("summary step requires 'prompt' in config")
	}

	// Validate required filter_tags field
	var hasFilterTags bool
	if tags, ok := step.Config["filter_tags"].([]interface{}); ok && len(tags) > 0 {
		hasFilterTags = true
	} else if tags, ok := step.Config["filter_tags"].([]string); ok && len(tags) > 0 {
		hasFilterTags = true
	}

	if !hasFilterTags {
		return fmt.Errorf("summary step requires 'filter_tags' in config")
	}

	return nil
}

// generateSummary generates a summary from documents using Gemini
func (w *SummaryWorker) generateSummary(ctx context.Context, client *genai.Client, prompt string, documents []*models.Document, parentJobID string) (string, error) {
	// Build document content for the LLM
	var docsContent strings.Builder
	docsContent.WriteString("# Documents to Summarize\n\n")

	for i, doc := range documents {
		docsContent.WriteString(fmt.Sprintf("## Document %d: %s\n\n", i+1, doc.Title))
		docsContent.WriteString(fmt.Sprintf("**Source:** %s\n", doc.SourceType))
		if doc.URL != "" {
			docsContent.WriteString(fmt.Sprintf("**URL:** %s\n", doc.URL))
		}
		docsContent.WriteString("\n### Content\n\n")
		// Truncate very long documents to avoid token limits
		content := doc.ContentMarkdown
		if len(content) > 50000 {
			content = content[:50000] + "\n\n... [content truncated]"
		}
		docsContent.WriteString(content)
		docsContent.WriteString("\n\n---\n\n")
	}

	// Build the full prompt
	systemPrompt := fmt.Sprintf(`You are an expert document analyst and summarizer.

## Task
%s

## Instructions
- Analyze all the documents provided below
- Generate a comprehensive, well-structured summary based on the task above
- Use markdown formatting for clarity
- Include relevant details, patterns, and insights from the documents
- If the task asks for specific information (like architecture), focus on that aspect
- Be thorough but concise

## Documents

%s`, prompt, docsContent.String())

	w.logger.Debug().
		Int("document_count", len(documents)).
		Str("parent_job_id", parentJobID).
		Msg("Executing Gemini summary generation")

	// Execute with timeout
	summaryCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Configure generation
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.3)),
	}

	// Make the API call with retry logic
	var resp *genai.GenerateContentResponse
	var apiErr error

	const maxRetries = 3
	const initialBackoff = 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, apiErr = client.Models.GenerateContent(
			summaryCtx,
			"gemini-2.0-flash",
			[]*genai.Content{
				genai.NewContentFromText(systemPrompt, genai.RoleUser),
			},
			config,
		)

		if apiErr == nil {
			break
		}

		if attempt == maxRetries {
			break
		}

		// Wait before retrying with exponential backoff
		multiplier := uint(1) << uint(attempt)
		backoff := initialBackoff * time.Duration(multiplier)
		select {
		case <-summaryCtx.Done():
			return "", fmt.Errorf("context cancelled during retry: %w", summaryCtx.Err())
		case <-time.After(backoff):
			w.logger.Warn().
				Int("attempt", attempt+1).
				Dur("backoff", backoff).
				Err(apiErr).
				Msg("Retrying summary generation")
		}
	}

	if apiErr != nil {
		return "", fmt.Errorf("failed to generate summary after %d retries: %w", maxRetries, apiErr)
	}

	// Extract response text
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response from Gemini API")
	}

	responseText := resp.Text()
	if responseText == "" {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	return responseText, nil
}

// createDocument creates a Document from the summary results
func (w *SummaryWorker) createDocument(summaryContent, prompt string, documents []*models.Document, jobDef *models.JobDefinition, parentJobID string) (*models.Document, error) {
	// Build tags - include job name and any job-level tags
	tags := []string{"summary"}
	if jobDef != nil {
		// Add job name as a tag (sanitized)
		jobNameTag := strings.ToLower(strings.ReplaceAll(jobDef.Name, " ", "-"))
		tags = append(tags, jobNameTag)

		if len(jobDef.Tags) > 0 {
			tags = append(tags, jobDef.Tags...)
		}
	}

	// Build source document IDs
	sourceDocIDs := make([]string, len(documents))
	for i, doc := range documents {
		sourceDocIDs[i] = doc.ID
	}

	// Build metadata
	metadata := map[string]interface{}{
		"prompt":                prompt,
		"source_document_ids":   sourceDocIDs,
		"source_document_count": len(documents),
		"parent_job_id":         parentJobID,
		"generated_at":          time.Now().Format(time.RFC3339),
	}
	if jobDef != nil {
		metadata["job_name"] = jobDef.Name
		metadata["job_id"] = jobDef.ID
	}

	now := time.Now()
	title := "Summary"
	if jobDef != nil && jobDef.Name != "" {
		title = fmt.Sprintf("Summary: %s", jobDef.Name)
	}

	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "summary",
		SourceID:        parentJobID,
		Title:           title,
		ContentMarkdown: summaryContent,
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
func (w *SummaryWorker) logJobEvent(ctx context.Context, parentJobID, _, level, message string, _ map[string]interface{}) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
