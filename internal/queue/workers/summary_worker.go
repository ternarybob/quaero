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
	"github.com/ternarybob/quaero/internal/services/llm"
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
	providerFactory *llm.ProviderFactory
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
	providerFactory *llm.ProviderFactory,
) *SummaryWorker {
	return &SummaryWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		eventService:    eventService,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		providerFactory: providerFactory,
	}
}

// GetType returns WorkerTypeSummary for the DefinitionWorker interface
func (w *SummaryWorker) GetType() models.WorkerType {
	return models.WorkerTypeSummary
}

// Init performs the initialization/setup phase for a summary step.
// This is where we:
//   - Extract and validate configuration (prompt, filter_tags, model)
//   - Query documents matching the filter criteria
//   - Return the document list as metadata for CreateJobs
//
// The Init phase does NOT generate the summary - it only validates and prepares.
// API key resolution is handled by the provider factory during generation.
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

	// Extract filter_limit from step config (prevents token overflow on large codebases)
	filterLimit := 1000 // Default maximum documents
	if limit, ok := stepConfig["filter_limit"].(int); ok && limit > 0 {
		filterLimit = limit
	} else if limitFloat, ok := stepConfig["filter_limit"].(float64); ok && limitFloat > 0 {
		filterLimit = int(limitFloat)
	} else if limitInt64, ok := stepConfig["filter_limit"].(int64); ok && limitInt64 > 0 {
		filterLimit = int(limitInt64)
	}

	// Query documents matching filter tags
	opts := interfaces.SearchOptions{
		Tags:  filterTags,
		Limit: filterLimit,
	}

	// Apply category filter if specified
	if categories := extractCategoryFilter(stepConfig); len(categories) > 0 {
		if opts.MetadataFilters == nil {
			opts.MetadataFilters = make(map[string]string)
		}
		opts.MetadataFilters["rule_classifier.category"] = strings.Join(categories, ",")
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Strs("categories", categories).
			Msg("Applying category filter to summary query")
	}

	documents, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}

	if len(documents) == 0 {
		return nil, fmt.Errorf("no documents found matching tags: %v", filterTags)
	}

	// Log if filter_limit was applied
	if filterLimit < 1000 {
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Int("filter_limit", filterLimit).
			Int("document_count", len(documents)).
			Msg("filter_limit applied to prevent token overflow")
	}

	// Extract output_validation patterns (optional - for validating LLM output)
	var validationPatterns []string
	if patterns, ok := stepConfig["output_validation"].([]interface{}); ok {
		for _, p := range patterns {
			if pattern, ok := p.(string); ok && pattern != "" {
				validationPatterns = append(validationPatterns, pattern)
			}
		}
	} else if patterns, ok := stepConfig["output_validation"].([]string); ok {
		validationPatterns = patterns
	}

	if len(validationPatterns) > 0 {
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Int("validation_patterns", len(validationPatterns)).
			Msg("Output validation enabled")
	}

	// Extract thinking_level (optional - controls reasoning depth: MINIMAL, LOW, MEDIUM, HIGH)
	var thinkingLevel string
	if level, ok := stepConfig["thinking_level"].(string); ok {
		thinkingLevel = strings.ToUpper(level)
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Str("thinking_level", thinkingLevel).
			Msg("Thinking level configured")
	}

	// Extract model (optional - can include provider prefix like "claude/claude-sonnet-4-20250514")
	// Supported formats:
	//   - "gemini-3-flash" or "gemini/gemini-3-flash" -> Gemini
	//   - "claude-sonnet-4-20250514" or "claude/claude-sonnet-4-20250514" -> Claude
	var model string
	if m, ok := stepConfig["model"].(string); ok && m != "" {
		model = m
		provider := w.providerFactory.DetectProvider(model)
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Str("model", model).
			Str("detected_provider", string(provider)).
			Msg("Model override configured")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("document_count", len(documents)).
		Int("filter_limit", filterLimit).
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
			"prompt":              prompt,
			"filter_tags":         filterTags,
			"documents":           documents,
			"step_config":         stepConfig,
			"filter_limit":        filterLimit,
			"validation_patterns": validationPatterns,
			"thinking_level":      thinkingLevel,
			"model":               model, // Can include provider prefix like "claude/claude-sonnet-4-20250514"
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
	documents, _ := initResult.Metadata["documents"].([]*models.Document)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

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

	// Extract thinking level and model for LLM configuration
	thinkingLevel, _ := initResult.Metadata["thinking_level"].(string)
	model, _ := initResult.Metadata["model"].(string)

	// Detect provider from model name
	provider := w.providerFactory.DetectProvider(model)
	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("model", model).
		Str("provider", string(provider)).
		Msg("Using AI provider for summary generation")

	// Generate summary using provider factory
	summaryContent, err := w.generateSummary(ctx, prompt, documents, stepID, thinkingLevel, model)
	if err != nil {
		w.logger.Error().Err(err).Str("step_name", step.Name).Msg("Summary generation failed")
		w.logJobEvent(ctx, stepID, step.Name, "error",
			fmt.Sprintf("Summary generation failed: %v", err), nil)
		return "", fmt.Errorf("summary generation failed: %w", err)
	}

	// Validate output if patterns are specified
	if validationPatterns, ok := initResult.Metadata["validation_patterns"].([]string); ok && len(validationPatterns) > 0 {
		if err := w.validateOutput(summaryContent, validationPatterns); err != nil {
			w.logger.Error().Err(err).Str("step_name", step.Name).Msg("Output validation failed")
			w.logJobEvent(ctx, stepID, step.Name, "error",
				fmt.Sprintf("Output validation failed: %v", err), nil)
			return "", err
		}
		w.logger.Info().
			Str("step_name", step.Name).
			Int("patterns_verified", len(validationPatterns)).
			Msg("Output validation passed")
		w.logJobEvent(ctx, stepID, step.Name, "info",
			fmt.Sprintf("Output validation passed: %d patterns verified", len(validationPatterns)), nil)
	}

	// Create summary document
	doc, err := w.createDocument(summaryContent, prompt, documents, &jobDef, stepID, stepConfig)
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

	// Validate output_validation format if specified
	if validation, ok := step.Config["output_validation"]; ok {
		switch v := validation.(type) {
		case []interface{}:
			// Valid array format - check all elements are strings
			for i, item := range v {
				if _, ok := item.(string); !ok {
					return fmt.Errorf("output_validation[%d] must be a string", i)
				}
			}
		case []string:
			// Valid string array format
		default:
			return fmt.Errorf("output_validation must be an array of strings")
		}
	}

	return nil
}

// validateOutput checks if the summary contains all required patterns.
// Returns an error listing missing patterns if validation fails.
func (w *SummaryWorker) validateOutput(content string, patterns []string) error {
	var missing []string
	for _, pattern := range patterns {
		if !strings.Contains(content, pattern) {
			missing = append(missing, pattern)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("summary output validation failed - missing required sections: %v", missing)
	}
	return nil
}

// generateSummary generates a summary from documents using the provider factory.
// thinkingLevel controls reasoning depth: MINIMAL, LOW, MEDIUM, HIGH (Gemini only).
// model specifies the model to use, can include provider prefix (e.g., "claude/claude-sonnet-4-20250514").
func (w *SummaryWorker) generateSummary(ctx context.Context, prompt string, documents []*models.Document, parentJobID string, thinkingLevel string, modelOverride string) (string, error) {
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

	// Get current date for the analysis
	currentDate := time.Now().Format("January 2, 2006")

	// Build the full prompt with current date context
	systemPrompt := fmt.Sprintf(`You are an expert document analyst and summarizer.

## Current Date
Today's date is %s. Use this as the analysis date in your output. Do NOT use any other date.

## Task
%s

## Instructions
- Analyze all the documents provided below
- Generate a comprehensive, well-structured summary based on the task above
- Use markdown formatting for clarity
- Include relevant details, patterns, and insights from the documents
- If the task asks for specific information (like architecture), focus on that aspect
- Be thorough but concise
- Always use the current date provided above for any "Analysis Date" fields

## Documents

%s`, currentDate, prompt, docsContent.String())

	// Detect provider and normalize model
	provider := w.providerFactory.DetectProvider(modelOverride)
	model := w.providerFactory.NormalizeModel(modelOverride)

	// Use default model for provider if not specified
	if model == "" {
		model = w.providerFactory.GetDefaultModel(provider, "default")
	}

	w.logger.Debug().
		Int("document_count", len(documents)).
		Str("parent_job_id", parentJobID).
		Str("model", model).
		Str("provider", string(provider)).
		Str("thinking_level", thinkingLevel).
		Msg("Executing summary generation with provider")

	// Execute with timeout
	summaryCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Build content request for provider factory
	request := &llm.ContentRequest{
		Messages: []interfaces.Message{
			{Role: "user", Content: systemPrompt},
		},
		Model:         model,
		Temperature:   0.3,
		ThinkingLevel: thinkingLevel, // Only used by Gemini
	}

	// Generate content using provider factory (handles retries internally)
	resp, err := w.providerFactory.GenerateContent(summaryCtx, request)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	if resp.Text == "" {
		return "", fmt.Errorf("empty response from %s API", resp.Provider)
	}

	w.logger.Debug().
		Str("provider", string(resp.Provider)).
		Str("model", resp.Model).
		Int("response_length", len(resp.Text)).
		Msg("Summary generation completed")

	return resp.Text, nil
}

// createDocument creates a Document from the summary results
func (w *SummaryWorker) createDocument(summaryContent, prompt string, documents []*models.Document, jobDef *models.JobDefinition, parentJobID string, stepConfig map[string]interface{}) (*models.Document, error) {
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

	// Add output_tags from step config (allows downstream steps to find this document)
	if stepConfig != nil {
		if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
			w.logger.Debug().
				Int("output_tags_count", len(outputTags)).
				Msg("Found output_tags ([]interface{}) in step config")
			for _, tag := range outputTags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					tags = append(tags, tagStr)
				}
			}
		} else if outputTags, ok := stepConfig["output_tags"].([]string); ok {
			w.logger.Debug().
				Strs("output_tags", outputTags).
				Msg("Found output_tags ([]string) in step config")
			tags = append(tags, outputTags...)
		} else {
			w.logger.Debug().
				Str("output_tags_type", fmt.Sprintf("%T", stepConfig["output_tags"])).
				Msg("output_tags not found or unexpected type in step config")
		}
	}

	w.logger.Info().
		Strs("tags", tags).
		Msg("Creating document with tags")

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
