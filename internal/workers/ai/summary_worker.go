// -----------------------------------------------------------------------
// SummaryWorker - Generates summaries from tagged documents
// Aggregates all documents matching filter tags and generates a comprehensive summary
// -----------------------------------------------------------------------

package ai

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/schemas"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/templates"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// SummaryWorker handles corpus summary generation from tagged documents.
// This worker executes synchronously (no child jobs) and creates a single
// summary document from all documents matching the filter criteria.
//
// Template support:
// - template: Name of prompt template to load (e.g., "stock-analysis")
// - If template specified, loads prompt and schema from templates package
// - Template resolution: user override (templatesDir) → embedded default
type SummaryWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	providerFactory *llm.ProviderFactory
	debugEnabled    bool
	templatesDir    string // Directory for user template overrides

	// LLM timing for current execution (reset per CreateJobs call)
	currentLLMTiming *workerutil.WorkerDebugInfo
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
	debugEnabled bool,
	templatesDir string,
) *SummaryWorker {
	return &SummaryWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		eventService:    eventService,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		providerFactory: providerFactory,
		debugEnabled:    debugEnabled,
		templatesDir:    templatesDir,
	}
}

// GetType returns WorkerTypeSummary for the DefinitionWorker interface
func (w *SummaryWorker) GetType() models.WorkerType {
	return models.WorkerTypeSummary
}

// loadSchemaFromFile loads a JSON schema from the embedded schemas.
// schemaRef is the filename (e.g., "stock-analysis.schema.json")
func (w *SummaryWorker) loadSchemaFromFile(schemaRef string) (map[string]interface{}, error) {
	if schemaRef == "" {
		return nil, nil
	}

	// Read schema file from embedded FS
	schemaContent, err := schemas.GetSchema(schemaRef)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded schema file: %w", err)
	}

	// Parse JSON schema
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaContent, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	w.logger.Info().
		Str("schema_ref", schemaRef).
		Msg("Loaded embedded JSON schema")

	return schema, nil
}

// resolvePromptAndSchema resolves prompt and schema from template or inline config.
// Resolution order:
// 1. If "template" specified → load from templates (user override → embedded)
// 2. If "prompt" specified → use inline prompt
// 3. Template takes precedence over inline prompt
func (w *SummaryWorker) resolvePromptAndSchema(stepConfig map[string]interface{}) (string, string, error) {
	// Check for template reference
	if templateName, ok := stepConfig["template"].(string); ok && templateName != "" {
		tmpl, err := templates.GetTemplate(templateName, w.templatesDir)
		if err != nil {
			return "", "", fmt.Errorf("failed to load template '%s': %w", templateName, err)
		}

		if tmpl.Type != templates.TemplateTypePrompt {
			return "", "", fmt.Errorf("template '%s' is type '%s', expected 'prompt'", templateName, tmpl.Type)
		}

		prompt := tmpl.Prompt
		schemaRef := tmpl.SchemaRef

		// Allow inline schema_ref to override template schema
		if override, ok := stepConfig["schema_ref"].(string); ok && override != "" {
			schemaRef = override
		}
		// Also check output_schema_ref for backward compatibility
		if override, ok := stepConfig["output_schema_ref"].(string); ok && override != "" {
			schemaRef = override
		}

		w.logger.Info().
			Str("template", templateName).
			Str("schema_ref", schemaRef).
			Msg("Loaded prompt from template")

		return prompt, schemaRef, nil
	}

	// Fall back to inline prompt
	prompt, ok := stepConfig["prompt"].(string)
	if !ok || prompt == "" {
		return "", "", fmt.Errorf("either 'template' or 'prompt' is required in step config")
	}

	// Get schema ref from config
	schemaRef, _ := stepConfig["schema_ref"].(string)
	if schemaRef == "" {
		schemaRef, _ = stepConfig["output_schema_ref"].(string)
	}

	return prompt, schemaRef, nil
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

	// Resolve prompt and schema from template or inline config
	prompt, schemaRef, err := w.resolvePromptAndSchema(stepConfig)
	if err != nil {
		return nil, err
	}

	// Extract filter_tags for documents to include in summary
	// First check explicit filter_tags, then fall back to input_tags (defaults to step name)
	// This supports the pipeline pattern where upstream steps tag output with their step name
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

	// Fall back to input_tags with step name as default
	if len(filterTags) == 0 {
		filterTags = workerutil.GetInputTags(stepConfig, step.Name)
	}

	if len(filterTags) == 0 {
		return nil, fmt.Errorf("filter_tags is required in step config (or use input_tags, or set step name for default)")
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

	// Extract max_iterations (optional - for critique loop)
	maxIterations := 0
	if max, ok := stepConfig["max_iterations"].(int); ok && max > 0 {
		maxIterations = max
	} else if maxFloat, ok := stepConfig["max_iterations"].(float64); ok && maxFloat > 0 {
		maxIterations = int(maxFloat)
	}

	// Extract critique_prompt (optional - for critique loop)
	critiquePrompt, _ := stepConfig["critique_prompt"].(string)

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

	// Extract required_tickers (optional - for validating all stocks are in output)
	// This is passed from the orchestrator for terminal analyze_summary steps
	// If not explicitly set, auto-extract from jobDef.Config["variables"] for step-based orchestration
	var requiredTickers []string
	if tickers, ok := stepConfig["required_tickers"].([]interface{}); ok {
		for _, t := range tickers {
			if ticker, ok := t.(string); ok && ticker != "" {
				requiredTickers = append(requiredTickers, strings.ToUpper(ticker))
			}
		}
	} else if tickers, ok := stepConfig["required_tickers"].([]string); ok {
		for _, t := range tickers {
			if t != "" {
				requiredTickers = append(requiredTickers, strings.ToUpper(t))
			}
		}
	}

	// Auto-extract tickers from job-level variables if required_tickers not explicitly set
	// This enables ticker validation for step-based orchestrator pipelines
	if len(requiredTickers) == 0 && jobDef.Config != nil {
		tickers := workerutil.CollectTickersWithJobDef(nil, jobDef)
		for _, t := range tickers {
			if t.Code != "" {
				requiredTickers = append(requiredTickers, strings.ToUpper(t.Code))
			}
		}
		if len(requiredTickers) > 0 {
			w.logger.Debug().
				Str("phase", "init").
				Str("step_name", step.Name).
				Strs("auto_extracted_tickers", requiredTickers).
				Msg("Auto-extracted required_tickers from job variables for validation")
		}
	}

	// Extract benchmark_codes (optional - for validating benchmarks aren't treated as stocks)
	// This is passed from the orchestrator for terminal analyze_summary steps
	var benchmarkCodes []string
	if codes, ok := stepConfig["benchmark_codes"].([]interface{}); ok {
		for _, c := range codes {
			if code, ok := c.(string); ok && code != "" {
				benchmarkCodes = append(benchmarkCodes, strings.ToUpper(code))
			}
		}
	} else if codes, ok := stepConfig["benchmark_codes"].([]string); ok {
		for _, c := range codes {
			if c != "" {
				benchmarkCodes = append(benchmarkCodes, strings.ToUpper(c))
			}
		}
	}

	if len(requiredTickers) > 0 || len(benchmarkCodes) > 0 {
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Strs("required_tickers", requiredTickers).
			Strs("benchmark_codes", benchmarkCodes).
			Msg("Ticker validation enabled")
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

	// Extract output_schema (optional - JSON schema for structured output)
	// When provided, the LLM MUST return JSON matching this schema
	// Can be specified inline, via schema_ref (config), or from template
	var outputSchema map[string]interface{}
	if schema, ok := stepConfig["output_schema"].(map[string]interface{}); ok && len(schema) > 0 {
		outputSchema = schema
		w.logger.Info().
			Str("phase", "init").
			Str("step_name", step.Name).
			Msg("Output schema configured for structured JSON generation (inline)")
	} else if schemaRef != "" {
		// schemaRef was resolved from template or config by resolvePromptAndSchema
		schema, err := w.loadSchemaFromFile(schemaRef)
		if err != nil {
			w.logger.Warn().
				Err(err).
				Str("schema_ref", schemaRef).
				Msg("Failed to load external schema, continuing without schema")
		} else {
			outputSchema = schema
			w.logger.Info().
				Str("phase", "init").
				Str("step_name", step.Name).
				Str("schema_ref", schemaRef).
				Msg("Output schema configured for structured JSON generation (external)")
		}
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

	// Compute content hash from prompt AND document IDs for cache invalidation
	// When prompt changes OR document set changes, hash changes, causing cache miss
	// This ensures that adding/removing stocks to a job triggers re-generation
	var hashInput strings.Builder
	hashInput.WriteString(prompt)
	hashInput.WriteString("|docs:")
	for _, doc := range documents {
		hashInput.WriteString(doc.ID)
		hashInput.WriteString(",")
	}
	hash := md5.Sum([]byte(hashInput.String()))
	contentHash := hex.EncodeToString(hash[:])[:8] // First 8 chars of MD5 hex

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(documents),
		Strategy:             interfaces.ProcessingStrategyInline, // Synchronous execution
		SuggestedConcurrency: 1,
		ContentHash:          contentHash, // For cache invalidation when prompt changes
		Metadata: map[string]interface{}{
			"prompt":              prompt,
			"filter_tags":         filterTags,
			"documents":           documents,
			"step_config":         stepConfig,
			"filter_limit":        filterLimit,
			"validation_patterns": validationPatterns,
			"required_tickers":    requiredTickers, // For ticker validation in output
			"benchmark_codes":     benchmarkCodes,  // For benchmark misuse validation
			"thinking_level":      thinkingLevel,
			"model":               model,        // Can include provider prefix like "claude/claude-sonnet-4-20250514"
			"output_schema":       outputSchema, // JSON schema for structured LLM output (Gemini only)
			"max_iterations":      maxIterations,
			"critique_prompt":     critiquePrompt,
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

	// Get the manager ID for document isolation
	// This ensures documents are tagged with the orchestrator's job ID,
	// enabling downstream steps (like format_output) to find them
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

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

	// Extract thinking level, model, and output schema for LLM configuration
	thinkingLevel, _ := initResult.Metadata["thinking_level"].(string)
	model, _ := initResult.Metadata["model"].(string)
	outputSchema, _ := initResult.Metadata["output_schema"].(map[string]interface{})

	// Detect provider from model name
	provider := w.providerFactory.DetectProvider(model)
	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("model", model).
		Str("provider", string(provider)).
		Msg("Using AI provider for summary generation")

	// Generate summary using provider factory
	// Pass jobDef to include portfolio config variables in the prompt context

	// Initialize LLM timing for this execution (reset from previous runs)
	if w.debugEnabled {
		w.currentLLMTiming = workerutil.NewWorkerDebug("summary_llm", true)
	}

	// INITIAL DRAFT GENERATION
	currentPrompt := prompt
	var summaryContent string
	var err error

	// Extract iteration config from initResult metadata (added in Init step)
	maxIterations := 0
	if max, ok := initResult.Metadata["max_iterations"].(int); ok {
		maxIterations = max
	}
	critiquePrompt, _ := initResult.Metadata["critique_prompt"].(string)

	// If no critique configured, just run once
	if maxIterations <= 0 || critiquePrompt == "" {
		if w.currentLLMTiming != nil {
			w.currentLLMTiming.StartPhase("ai_generation")
		}
		summaryContent, err = w.generateSummary(ctx, currentPrompt, documents, stepID, thinkingLevel, model, &jobDef, outputSchema)
		if w.currentLLMTiming != nil {
			w.currentLLMTiming.EndPhase("ai_generation")
			w.currentLLMTiming.Complete()
		}
		if err != nil {
			w.logger.Error().Err(err).Str("step_name", step.Name).Msg("Summary generation failed")
			w.logJobEvent(ctx, stepID, step.Name, "error",
				fmt.Sprintf("Summary generation failed: %v", err), nil)
			return "", fmt.Errorf("summary generation failed: %w", err)
		}
	} else {
		// ITERATIVE CRITIQUE LOOP
		w.logger.Info().
			Str("step_name", step.Name).
			Int("max_iterations", maxIterations).
			Msg("Starting iterative summary generation with critique")

		// Start overall LLM timing for iterative mode
		if w.currentLLMTiming != nil {
			w.currentLLMTiming.StartPhase("ai_generation")
		}

		for i := 0; i <= maxIterations; i++ {
			iterationLabel := fmt.Sprintf("Iteration %d/%d", i+1, maxIterations+1)
			w.logJobEvent(ctx, stepID, step.Name, "info",
				fmt.Sprintf("Generating draft (%s)", iterationLabel), nil)

			// 1. Generate Draft (or Refined Draft)
			summaryContent, err = w.generateSummary(ctx, currentPrompt, documents, stepID, thinkingLevel, model, &jobDef, outputSchema)
			if err != nil {
				w.logger.Error().Err(err).Str("step_name", step.Name).Msg("Summary generation failed in loop")
				if w.currentLLMTiming != nil {
					w.currentLLMTiming.EndPhase("ai_generation")
					w.currentLLMTiming.Complete()
				}
				return "", fmt.Errorf("summary generation failed (iter %d): %w", i, err)
			}

			// If this was the last iteration, break and save
			if i == maxIterations {
				break
			}

			// 2. Generate Critique
			w.logJobEvent(ctx, stepID, step.Name, "info",
				fmt.Sprintf("Critiquing draft (%s)", iterationLabel), nil)

			critique, err := w.generateCritique(ctx, critiquePrompt, summaryContent, stepID, model)
			if err != nil {
				w.logger.Warn().Err(err).Msg("Critique generation failed, proceeding with current draft")
				break
			}

			// 3. Check Signal
			if strings.Contains(strings.ToUpper(critique), "NO_CHANGES_NEEDED") {
				w.logger.Info().Msg("Critique passed with NO_CHANGES_NEEDED")
				w.logJobEvent(ctx, stepID, step.Name, "info", "Critique passed - no changes needed", nil)
				break
			}

			// 4. Update Prompts for Next Loop
			w.logger.Info().Str("critique_length", fmt.Sprintf("%d", len(critique))).Msg("Critique received, refining...")

			// Append critique to the prompt for the next run
			currentPrompt = fmt.Sprintf(`%s

---
## PREVIOUS DRAFT CRITIQUE (MUST ADDRESS)
The following is a critique of your previous draft. You must address EVERY issue raised here in your next version:

%s

---
`, prompt, critique)
		}

		// End LLM timing for iterative mode
		if w.currentLLMTiming != nil {
			w.currentLLMTiming.EndPhase("ai_generation")
			w.currentLLMTiming.Complete()
		}
	}

	// =========================================================================
	// OUTPUT VALIDATION WITH REGENERATION LOOP
	// Validates ticker presence, benchmark usage, and required patterns
	// Regenerates up to 3 times if validation fails
	// =========================================================================

	// Extract validation parameters from metadata
	validationPatterns, _ := initResult.Metadata["validation_patterns"].([]string)
	requiredTickers, _ := initResult.Metadata["required_tickers"].([]string)
	benchmarkCodes, _ := initResult.Metadata["benchmark_codes"].([]string)

	// Track validation result for metadata
	var validationResult *ValidationResult
	validationEnabled := len(validationPatterns) > 0 || len(requiredTickers) > 0 || len(benchmarkCodes) > 0

	if validationEnabled {
		const maxValidationIterations = 3
		originalPrompt := prompt

		for validationIteration := 1; validationIteration <= maxValidationIterations; validationIteration++ {
			// Perform comprehensive validation
			validationResult = w.validateOutputComprehensive(summaryContent, requiredTickers, benchmarkCodes, validationPatterns)
			validationResult.IterationCount = validationIteration

			if validationResult.Valid {
				w.logger.Info().
					Str("step_name", step.Name).
					Int("iteration", validationIteration).
					Strs("tickers_validated", requiredTickers).
					Msg("Output validation passed")
				w.logJobEvent(ctx, stepID, step.Name, "info",
					fmt.Sprintf("Output validation passed (iteration %d/%d)", validationIteration, maxValidationIterations), nil)
				break
			}

			// Validation failed
			w.logger.Warn().
				Str("step_name", step.Name).
				Int("iteration", validationIteration).
				Str("validation_errors", validationResult.String()).
				Msg("Output validation failed, attempting regeneration")

			// If this was the last iteration, fail
			if validationIteration == maxValidationIterations {
				errMsg := fmt.Sprintf("output validation failed after %d iterations: %s",
					maxValidationIterations, validationResult.String())
				w.logger.Error().
					Str("step_name", step.Name).
					Str("final_errors", validationResult.String()).
					Msg(errMsg)
				w.logJobEvent(ctx, stepID, step.Name, "error", errMsg, nil)
				return "", fmt.Errorf("%s", errMsg)
			}

			// Build validation feedback prompt for regeneration
			feedbackPrompt := w.buildValidationFeedbackPrompt(originalPrompt, validationResult)

			w.logJobEvent(ctx, stepID, step.Name, "warning",
				fmt.Sprintf("Validation failed (iteration %d/%d): %s - regenerating...",
					validationIteration, maxValidationIterations, validationResult.String()), nil)

			// Regenerate with feedback
			summaryContent, err = w.generateSummary(ctx, feedbackPrompt, documents, stepID, thinkingLevel, model, &jobDef, outputSchema)
			if err != nil {
				w.logger.Error().Err(err).Str("step_name", step.Name).Msg("Summary regeneration failed")
				return "", fmt.Errorf("summary regeneration failed (validation iteration %d): %w", validationIteration, err)
			}
		}
	} else {
		// No validation enabled - create a minimal result for metadata
		validationResult = &ValidationResult{Valid: true, IterationCount: 0}
	}

	// Create summary document with validation metadata
	// Use managerID (not stepID) so document can be found by downstream steps in the pipeline
	doc, err := w.createDocument(ctx, summaryContent, prompt, documents, &jobDef, managerID, stepConfig, validationResult)
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

	// Validate that either 'template' or 'prompt' is specified
	templateName, hasTemplate := step.Config["template"].(string)
	prompt, hasPrompt := step.Config["prompt"].(string)

	if (!hasTemplate || templateName == "") && (!hasPrompt || prompt == "") {
		return fmt.Errorf("summary step requires either 'template' or 'prompt' in config")
	}

	// Validate that we can resolve input tags
	// First check explicit filter_tags, then fall back to input_tags (defaults to step name)
	var hasFilterTags bool
	if tags, ok := step.Config["filter_tags"].([]interface{}); ok && len(tags) > 0 {
		hasFilterTags = true
	} else if tags, ok := step.Config["filter_tags"].([]string); ok && len(tags) > 0 {
		hasFilterTags = true
	} else {
		// Fall back to input_tags with step name as default (supports pipeline pattern)
		inputTags := workerutil.GetInputTags(step.Config, step.Name)
		hasFilterTags = len(inputTags) > 0
	}

	if !hasFilterTags {
		return fmt.Errorf("summary step requires 'filter_tags' or 'input_tags' in config, or a non-empty step name")
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

// validateOutput checks if the summary contains all required patterns and no placeholder patterns.
// Returns an error listing missing patterns or detected placeholders if validation fails.
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

	// Check for placeholder patterns that indicate fabricated or incomplete data
	placeholderPatterns := []string{
		"$[X",       // Dollar placeholder like $[X.XX]
		"[X]",       // Generic placeholder
		"[Pending]", // Pending placeholder
		"[YYYY-MM",  // Date placeholder
		"| [XXX] |", // Table row placeholder
	}

	var foundPlaceholders []string
	for _, placeholder := range placeholderPatterns {
		if strings.Contains(content, placeholder) {
			foundPlaceholders = append(foundPlaceholders, placeholder)
		}
	}

	if len(foundPlaceholders) > 0 {
		w.logger.Warn().
			Strs("placeholders_found", foundPlaceholders).
			Msg("Summary contains placeholder patterns - output may have incomplete data")
		// Note: This is a warning, not an error, as some placeholders in template sections
		// (like risk assessment dropdowns) may be intentional
	}

	return nil
}

// ValidationResult holds the comprehensive result of output validation
type ValidationResult struct {
	Valid           bool
	MissingTickers  []string
	BenchmarkIssues []string
	PatternIssues   []string
	IterationCount  int
}

// String returns a human-readable summary of validation failures
func (v *ValidationResult) String() string {
	if v.Valid {
		return "validation passed"
	}
	var issues []string
	if len(v.MissingTickers) > 0 {
		issues = append(issues, fmt.Sprintf("missing tickers: %v", v.MissingTickers))
	}
	if len(v.BenchmarkIssues) > 0 {
		issues = append(issues, fmt.Sprintf("benchmark issues: %v", v.BenchmarkIssues))
	}
	if len(v.PatternIssues) > 0 {
		issues = append(issues, fmt.Sprintf("pattern issues: %v", v.PatternIssues))
	}
	return strings.Join(issues, "; ")
}

// validateTickerPresence ensures ALL tickers from variables appear in output.
// Returns error listing missing tickers. Tickers are checked case-insensitively.
// Checks for patterns like "ASX: GNP", "ASX:GNP", "| GNP |" (table cells), or "## GNP" (headers).
func validateTickerPresence(content string, requiredTickers []string) []string {
	if len(requiredTickers) == 0 {
		return nil
	}

	contentUpper := strings.ToUpper(content)
	var missing []string

	for _, ticker := range requiredTickers {
		tickerUpper := strings.ToUpper(ticker)

		// Check various patterns where tickers typically appear
		patterns := []string{
			fmt.Sprintf("ASX: %s", tickerUpper), // "ASX: GNP"
			fmt.Sprintf("ASX:%s", tickerUpper),  // "ASX:GNP"
			fmt.Sprintf("| %s |", tickerUpper),  // "| GNP |" in tables
			fmt.Sprintf("| %s\t", tickerUpper),  // "| GNP\t" in tables
			fmt.Sprintf("## %s", tickerUpper),   // "## GNP" section header
			fmt.Sprintf("### %s", tickerUpper),  // "### GNP" subsection
			fmt.Sprintf("**%s**", tickerUpper),  // "**GNP**" bold
			fmt.Sprintf("(%s)", tickerUpper),    // "(GNP)" in parentheses
		}

		found := false
		for _, pattern := range patterns {
			if strings.Contains(contentUpper, pattern) {
				found = true
				break
			}
		}

		if !found {
			missing = append(missing, ticker)
		}
	}

	return missing
}

// validateBenchmarkNotAsStock ensures benchmark indices are NOT treated as primary stocks.
// Checks for patterns that indicate the benchmark is being analyzed as a stock, such as:
// - "ASX: XJO" in a stock analysis section
// - Being the ONLY ticker in a summary table (with no actual stocks)
// - Conviction scores for benchmarks
func validateBenchmarkNotAsStock(content string, benchmarkCodes []string) []string {
	if len(benchmarkCodes) == 0 {
		return nil
	}

	contentUpper := strings.ToUpper(content)
	var issues []string

	for _, code := range benchmarkCodes {
		codeUpper := strings.ToUpper(code)

		// Check for patterns that indicate benchmark is treated as a stock
		problematicPatterns := []string{
			fmt.Sprintf("ASX: %s\nFUNDAMENTAL ANALYSIS:", codeUpper), // Stock analysis format
			fmt.Sprintf("ASX: %s (%s)", codeUpper, ""),               // Will catch any ASX: XJO (Something)
			fmt.Sprintf("CONVICTION SCORE: %s", codeUpper),           // Conviction for benchmark
			fmt.Sprintf("| %s | QUALITY", codeUpper),                 // In quality rating table
			fmt.Sprintf("| %s |\t", codeUpper),                       // First column of summary table
		}

		for _, pattern := range problematicPatterns {
			if strings.Contains(contentUpper, pattern) {
				issues = append(issues, fmt.Sprintf("benchmark '%s' appears to be analyzed as a stock", code))
				break
			}
		}

		// Special check: if XJO is the ONLY ticker-like pattern in summary table
		// This is a heuristic - look for "Summary Table" followed by only benchmark codes
		summaryIdx := strings.Index(contentUpper, "SUMMARY TABLE")
		if summaryIdx != -1 {
			// Look at the next 500 chars after "Summary Table"
			endIdx := summaryIdx + 500
			if endIdx > len(contentUpper) {
				endIdx = len(contentUpper)
			}
			tableSection := contentUpper[summaryIdx:endIdx]

			// Check if the benchmark appears in what looks like a stock table
			if strings.Contains(tableSection, fmt.Sprintf("| %s |", codeUpper)) {
				// This is a warning case, not necessarily an error
				// The benchmark might legitimately be in a comparison row
			}
		}
	}

	return issues
}

// validateOutputComprehensive performs comprehensive validation of summary output.
// It checks for required tickers, benchmark misuse, and pattern validation.
// Returns a ValidationResult with details of any issues found.
func (w *SummaryWorker) validateOutputComprehensive(
	content string,
	requiredTickers []string,
	benchmarkCodes []string,
	requiredPatterns []string,
) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Check required patterns (existing validation)
	if len(requiredPatterns) > 0 {
		for _, pattern := range requiredPatterns {
			if !strings.Contains(content, pattern) {
				result.PatternIssues = append(result.PatternIssues, pattern)
				result.Valid = false
			}
		}
	}

	// Check ticker presence
	if len(requiredTickers) > 0 {
		missing := validateTickerPresence(content, requiredTickers)
		if len(missing) > 0 {
			result.MissingTickers = missing
			result.Valid = false
			w.logger.Warn().
				Strs("missing_tickers", missing).
				Strs("required_tickers", requiredTickers).
				Msg("Output validation: missing required tickers")
		}
	}

	// Check benchmark misuse
	if len(benchmarkCodes) > 0 {
		issues := validateBenchmarkNotAsStock(content, benchmarkCodes)
		if len(issues) > 0 {
			result.BenchmarkIssues = issues
			result.Valid = false
			w.logger.Warn().
				Strs("benchmark_issues", issues).
				Strs("benchmark_codes", benchmarkCodes).
				Msg("Output validation: benchmark treated as stock")
		}
	}

	return result
}

// buildValidationFeedbackPrompt creates a prompt with specific validation failures
// that the LLM must address in regeneration
func (w *SummaryWorker) buildValidationFeedbackPrompt(originalPrompt string, validationResult *ValidationResult) string {
	var feedback strings.Builder

	feedback.WriteString(originalPrompt)
	feedback.WriteString("\n\n")
	feedback.WriteString("---\n")
	feedback.WriteString("## CRITICAL: VALIDATION FAILURES (MUST FIX)\n\n")
	feedback.WriteString("Your previous output FAILED validation. You MUST address ALL issues below:\n\n")

	if len(validationResult.MissingTickers) > 0 {
		feedback.WriteString("### MISSING STOCK TICKERS\n")
		feedback.WriteString("The following tickers MUST appear in your output with full analysis:\n")
		for _, ticker := range validationResult.MissingTickers {
			feedback.WriteString(fmt.Sprintf("- **%s**: Include complete analysis with format 'ASX: %s'\n", ticker, ticker))
		}
		feedback.WriteString("\nYou analyzed benchmarks instead of stocks. These are the STOCKS you must analyze, NOT benchmark indices.\n\n")
	}

	if len(validationResult.BenchmarkIssues) > 0 {
		feedback.WriteString("### BENCHMARK MISUSE\n")
		feedback.WriteString("You incorrectly treated benchmark indices as stocks:\n")
		for _, issue := range validationResult.BenchmarkIssues {
			feedback.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		feedback.WriteString("\nBenchmarks (like XJO) are for COMPARISON only. Do NOT create stock analysis sections for them.\n\n")
	}

	if len(validationResult.PatternIssues) > 0 {
		feedback.WriteString("### MISSING REQUIRED SECTIONS\n")
		feedback.WriteString("The following required sections are missing:\n")
		for _, pattern := range validationResult.PatternIssues {
			feedback.WriteString(fmt.Sprintf("- %s\n", pattern))
		}
		feedback.WriteString("\n")
	}

	feedback.WriteString("---\n")
	feedback.WriteString("REGENERATE your output now, ensuring ALL issues above are fixed.\n")

	return feedback.String()
}

// buildPortfolioHoldingsSection extracts portfolio holdings from job config variables
// and formats them as a markdown section for inclusion in the summary prompt.
// This enables portfolio summaries to include units, avg_price, and weighting data.
func (w *SummaryWorker) buildPortfolioHoldingsSection(jobDef *models.JobDefinition) string {
	if jobDef == nil {
		w.logger.Warn().Msg("buildPortfolioHoldingsSection: jobDef is nil")
		return ""
	}
	if jobDef.Config == nil {
		w.logger.Warn().Msg("buildPortfolioHoldingsSection: jobDef.Config is nil")
		return ""
	}

	w.logger.Debug().
		Int("config_size", len(jobDef.Config)).
		Msg("buildPortfolioHoldingsSection: inspecting config")

	// Extract variables from job config
	variablesRaw, ok := jobDef.Config["variables"]
	if !ok {
		w.logger.Warn().Msg("buildPortfolioHoldingsSection: 'variables' key not found in jobDef.Config")
		// Debug print keys
		for k := range jobDef.Config {
			w.logger.Debug().Str("key", k).Msg("Config key available")
		}
		return ""
	}

	// Parse variables array
	var variables []map[string]interface{}
	switch v := variablesRaw.(type) {
	case []interface{}:
		w.logger.Debug().Int("count", len(v)).Msg("Found variables as []interface{}")
		for _, item := range v {
			if varMap, ok := item.(map[string]interface{}); ok {
				variables = append(variables, varMap)
			}
		}
	case []map[string]interface{}:
		w.logger.Debug().Int("count", len(v)).Msg("Found variables as []map[string]interface{}")
		variables = v
	default:
		w.logger.Warn().Str("type", fmt.Sprintf("%T", variablesRaw)).Msg("buildPortfolioHoldingsSection: variables has unexpected type")
		return ""
	}

	if len(variables) == 0 {
		w.logger.Warn().Msg("buildPortfolioHoldingsSection: formatted variables list is empty")
		return ""
	}

	w.logger.Debug().Int("variables_count", len(variables)).Msg("buildPortfolioHoldingsSection: parsing variables for portfolio data")

	// Check if this looks like portfolio data (has ticker, units, avg_price)
	hasPortfolioData := false
	for _, varSet := range variables {
		if _, hasTicker := varSet["ticker"]; hasTicker {
			if _, hasUnits := varSet["units"]; hasUnits {
				hasPortfolioData = true
				break
			}
		}
	}

	if !hasPortfolioData {
		return ""
	}

	// Build portfolio holdings table
	var holdings strings.Builder
	holdings.WriteString("\n## Portfolio Holdings Data\n\n")
	holdings.WriteString("The following portfolio data was provided in the job configuration. ")
	holdings.WriteString("Use these EXACT values for units, average price, and weighting calculations:\n\n")
	holdings.WriteString("| Ticker | Name | Industry | Units | Avg Price | Weighting |\n")
	holdings.WriteString("|--------|------|----------|-------|-----------|----------|\n")

	for _, varSet := range variables {
		ticker := getStringValue(varSet, "ticker", "N/A")
		name := getStringValue(varSet, "name", "N/A")
		industry := getStringValue(varSet, "industry", "N/A")
		units := getNumericValue(varSet, "units")
		avgPrice := getNumericValue(varSet, "avg_price")
		weighting := getNumericValue(varSet, "weighting")

		holdings.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f | $%.3f | %.2f%% |\n",
			ticker, name, industry, units, avgPrice, weighting))
	}

	holdings.WriteString("\n**IMPORTANT**: The above data is authoritative. ")
	holdings.WriteString("Calculate Cost Basis = Units × Avg Price. ")
	holdings.WriteString("Calculate Current Value = Units × Current Price (from stock analysis documents). ")
	holdings.WriteString("Calculate Unrealized P/L = Current Value - Cost Basis.\n\n")

	w.logger.Info().
		Int("holdings_count", len(variables)).
		Msg("Included portfolio holdings data in summary prompt")

	return holdings.String()
}

// getStringValue extracts a string value from a map, returning defaultVal if not found
func getStringValue(m map[string]interface{}, key, defaultVal string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

// getNumericValue extracts a numeric value from a map, handling both float64 and int types
func getNumericValue(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0
}

// generateSummary generates a summary from documents using the provider factory.
// thinkingLevel controls reasoning depth: MINIMAL, LOW, MEDIUM, HIGH (Gemini only).
// model specifies the model to use, can include provider prefix (e.g., "claude/claude-sonnet-4-20250514").
// jobDef provides access to job-level config variables (e.g., portfolio holdings with units, avg_price).
// outputSchema, when provided, enforces structured JSON output which is then converted to markdown.
func (w *SummaryWorker) generateSummary(ctx context.Context, prompt string, documents []*models.Document, parentJobID string, thinkingLevel string, modelOverride string, jobDef *models.JobDefinition, outputSchema map[string]interface{}) (string, error) {
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

	// Build portfolio holdings section if job config contains variables (e.g., for portfolio summaries)
	portfolioHoldings := w.buildPortfolioHoldingsSection(jobDef)

	// Build the full prompt with current date context
	systemPrompt := fmt.Sprintf(`You are an expert document analyst and summarizer.

## Current Date
Today's date is %s. Use this as the analysis date in your output. Do NOT use any other date.
%s
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
- IMPORTANT: Use the exact values from the Portfolio Holdings Data section above for units, avg_price, and weighting - DO NOT use placeholders like [X] or $[X.XX]

## Documents

%s`, currentDate, portfolioHoldings, prompt, docsContent.String())

	// Detect provider and normalize model
	provider := w.providerFactory.DetectProvider(modelOverride)
	model := w.providerFactory.NormalizeModel(modelOverride)

	// Use default model for provider if not specified
	if model == "" {
		model = w.providerFactory.GetDefaultModel(provider)
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
		OutputSchema:  outputSchema,  // JSON schema for structured output (Gemini only)
	}

	// Log schema usage with SCHEMA_ENFORCEMENT marker for test assertions
	if outputSchema != nil && len(outputSchema) > 0 {
		schemaType, _ := outputSchema["type"].(string)
		requiredFields := []string{}
		if required, ok := outputSchema["required"].([]interface{}); ok {
			for _, r := range required {
				if s, ok := r.(string); ok {
					requiredFields = append(requiredFields, s)
				}
			}
		}

		w.logger.Info().
			Str("parent_job_id", parentJobID).
			Str("schema_type", schemaType).
			Strs("required_fields", requiredFields).
			Msg("SCHEMA_ENFORCEMENT: Using output schema for structured JSON generation")
	}

	// Generate content using provider factory (handles retries internally)
	resp, err := w.providerFactory.GenerateContent(summaryCtx, request)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	if resp.Text == "" {
		return "", fmt.Errorf("empty response from %s API", resp.Provider)
	}

	// If schema was used, the response is JSON - convert to markdown
	var cleanedText string
	if outputSchema != nil && len(outputSchema) > 0 {
		// Parse JSON response and convert to markdown
		markdown, err := w.jsonToMarkdown(resp.Text)
		if err != nil {
			w.logger.Warn().
				Err(err).
				Msg("Failed to convert JSON to markdown, using raw response")
			cleanedText = resp.Text
		} else {
			cleanedText = markdown
			w.logger.Debug().
				Int("json_length", len(resp.Text)).
				Int("markdown_length", len(markdown)).
				Msg("Converted structured JSON to markdown")
		}
	} else {
		// Strip any echoed personality/role text from LLM output
		// LLMs sometimes echo back the system prompt (e.g., "You are a Senior Investment Strategist...")
		cleanedText = w.stripLeadingPersonalityText(resp.Text)
	}

	w.logger.Debug().
		Str("provider", string(resp.Provider)).
		Str("model", resp.Model).
		Int("response_length", len(cleanedText)).
		Bool("schema_used", outputSchema != nil && len(outputSchema) > 0).
		Msg("Summary generation completed")

	return cleanedText, nil
}

// createDocument creates a Document from the summary results
func (w *SummaryWorker) createDocument(ctx context.Context, summaryContent, prompt string, documents []*models.Document, jobDef *models.JobDefinition, parentJobID string, stepConfig map[string]interface{}, validationResult *ValidationResult) (*models.Document, error) {
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

	// Add cache tags from context (for caching/deduplication)
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
		w.logger.Debug().
			Strs("cache_tags", cacheTags).
			Msg("Applied cache tags to document")
	}

	// Propagate ticker tags from source documents
	// This enables downstream steps (format_output, email_report) to identify the stock
	tickerSet := make(map[string]bool)
	for _, doc := range documents {
		for _, tag := range doc.Tags {
			// Check for ticker: prefix format
			if strings.HasPrefix(tag, "ticker:") {
				ticker := strings.TrimPrefix(tag, "ticker:")
				tickerSet[strings.ToLower(ticker)] = true
			} else if isTickerTag(tag) {
				// Short lowercase tags that look like tickers (2-5 chars, not system tags)
				tickerSet[tag] = true
			}
		}
	}
	// Add unique ticker tags
	for ticker := range tickerSet {
		tags = append(tags, ticker)
		w.logger.Debug().
			Str("ticker_tag", ticker).
			Msg("Propagated ticker tag from source documents")
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

	// Add output validation metadata
	if validationResult != nil {
		validationMeta := map[string]interface{}{
			"enabled":           validationResult.IterationCount > 0 || len(validationResult.MissingTickers) > 0 || len(validationResult.BenchmarkIssues) > 0 || len(validationResult.PatternIssues) > 0,
			"validation_passed": validationResult.Valid,
			"iteration_count":   validationResult.IterationCount,
			"max_iterations":    3,
		}

		// Add validated tickers if present
		requiredTickers, _ := stepConfig["required_tickers"].([]string)
		if len(requiredTickers) == 0 {
			// Try []interface{} type
			if tickers, ok := stepConfig["required_tickers"].([]interface{}); ok {
				for _, t := range tickers {
					if ticker, ok := t.(string); ok {
						requiredTickers = append(requiredTickers, ticker)
					}
				}
			}
		}
		if len(requiredTickers) > 0 {
			validationMeta["tickers_validated"] = requiredTickers
		}

		// Add benchmark check status
		if len(validationResult.BenchmarkIssues) > 0 {
			validationMeta["benchmark_check"] = strings.Join(validationResult.BenchmarkIssues, "; ")
		} else {
			validationMeta["benchmark_check"] = "passed"
		}

		metadata["output_validation"] = validationMeta
	}

	// Aggregate worker debug metadata from source documents (if debug enabled)
	var debugAggregate map[string]interface{}
	if w.debugEnabled {
		debugAggregate = w.aggregateWorkerDebug(documents)
		if debugAggregate != nil {
			metadata["worker_debug_aggregate"] = debugAggregate
		}
	}

	// Append debug aggregate markdown to content
	finalContent := summaryContent
	if w.debugEnabled && debugAggregate != nil {
		if debugMd := w.workerDebugAggregateToMarkdown(debugAggregate); debugMd != "" {
			finalContent = summaryContent + debugMd
		}
	}

	now := time.Now()
	title := "Summary"
	if jobDef != nil && jobDef.Name != "" {
		title = fmt.Sprintf("Summary: %s", jobDef.Name)
	}

	// Build Jobs array - include parentJobID for document isolation
	// This enables downstream steps (like format_output) to find documents
	// from the same pipeline execution using JobID filter
	var jobs []string
	if parentJobID != "" {
		jobs = []string{parentJobID}
	}

	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "summary",
		SourceID:        parentJobID,
		Title:           title,
		ContentMarkdown: finalContent,
		DetailLevel:     models.DetailLevelFull,
		Metadata:        metadata,
		Tags:            tags,
		Jobs:            jobs,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}

	return doc, nil
}

// aggregateWorkerDebug collects and aggregates worker debug metadata from source documents
func (w *SummaryWorker) aggregateWorkerDebug(documents []*models.Document) map[string]interface{} {
	if !w.debugEnabled || len(documents) == 0 {
		return nil
	}

	var workerInstances []map[string]interface{}
	var totalDurationMs int64
	apiEndpointsCount := 0
	aiSources := make(map[string]map[string]interface{}) // key: provider:model

	for _, doc := range documents {
		if doc.Metadata == nil {
			continue
		}

		// Extract worker_debug from each document
		debugRaw, ok := doc.Metadata["worker_debug"]
		if !ok {
			continue
		}

		debugMeta, ok := debugRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Build worker instance entry
		instance := map[string]interface{}{
			"worker_type": debugMeta["worker_type"],
		}

		if ticker, ok := debugMeta["ticker"].(string); ok && ticker != "" {
			instance["ticker"] = ticker
		}

		if timing, ok := debugMeta["timing"].(map[string]interface{}); ok {
			instance["timing"] = timing
			if totalMs, ok := timing["total_ms"].(int64); ok {
				totalDurationMs += totalMs
			} else if totalMsFloat, ok := timing["total_ms"].(float64); ok {
				totalDurationMs += int64(totalMsFloat)
			}
		}

		workerInstances = append(workerInstances, instance)

		// Count API endpoints
		if endpoints, ok := debugMeta["api_endpoints"].([]map[string]interface{}); ok {
			apiEndpointsCount += len(endpoints)
		} else if endpointsRaw, ok := debugMeta["api_endpoints"].([]interface{}); ok {
			apiEndpointsCount += len(endpointsRaw)
		}

		// Aggregate AI sources
		if aiSource, ok := debugMeta["ai_source"].(map[string]interface{}); ok {
			provider, _ := aiSource["provider"].(string)
			model, _ := aiSource["model"].(string)
			key := provider + ":" + model

			if existing, ok := aiSources[key]; ok {
				// Add tokens
				if inputTokens, ok := aiSource["input_tokens"].(int); ok {
					if existingInput, ok := existing["input_tokens"].(int); ok {
						existing["input_tokens"] = existingInput + inputTokens
					}
				}
				if outputTokens, ok := aiSource["output_tokens"].(int); ok {
					if existingOutput, ok := existing["output_tokens"].(int); ok {
						existing["output_tokens"] = existingOutput + outputTokens
					}
				}
			} else {
				aiSources[key] = map[string]interface{}{
					"provider":      provider,
					"model":         model,
					"input_tokens":  aiSource["input_tokens"],
					"output_tokens": aiSource["output_tokens"],
				}
			}
		}
	}

	// Add summary worker's own LLM timing to worker instances
	if w.currentLLMTiming != nil && w.currentLLMTiming.IsEnabled() {
		summaryDebugMeta := w.currentLLMTiming.ToMetadata()
		if summaryDebugMeta != nil {
			summaryInstance := map[string]interface{}{
				"worker_type": "summary_llm",
			}
			if timing, ok := summaryDebugMeta["timing"].(map[string]interface{}); ok {
				summaryInstance["timing"] = timing
				if totalMs, ok := timing["total_ms"].(int64); ok {
					totalDurationMs += totalMs
				}
			}
			workerInstances = append(workerInstances, summaryInstance)
		}
	}

	if len(workerInstances) == 0 {
		return nil
	}

	// Collect source document info for the aggregate
	sourceDocuments := make([]map[string]interface{}, 0, len(documents))
	for _, doc := range documents {
		sourceDocuments = append(sourceDocuments, map[string]interface{}{
			"id":          doc.ID,
			"title":       doc.Title,
			"source_type": doc.SourceType,
		})
	}

	result := map[string]interface{}{
		"total_duration_ms":     totalDurationMs,
		"worker_instances":      workerInstances,
		"api_endpoints_called":  apiEndpointsCount,
		"source_document_count": len(documents),
		"source_documents":      sourceDocuments,
	}

	// Convert AI sources map to slice
	if len(aiSources) > 0 {
		aiSourcesList := make([]map[string]interface{}, 0, len(aiSources))
		for _, source := range aiSources {
			aiSourcesList = append(aiSourcesList, source)
		}
		result["ai_sources"] = aiSourcesList
	}

	return result
}

// workerDebugAggregateToMarkdown converts aggregated debug metadata to a markdown section
func (w *SummaryWorker) workerDebugAggregateToMarkdown(aggregate map[string]interface{}) string {
	if aggregate == nil || !w.debugEnabled {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n---\n")
	sb.WriteString("## Worker Debug Aggregate\n\n")

	// Summary stats
	if totalMs, ok := aggregate["total_duration_ms"].(int64); ok {
		sb.WriteString(fmt.Sprintf("**Total Duration**: %dms\n", totalMs))
	}
	if sourceCount, ok := aggregate["source_document_count"].(int); ok {
		sb.WriteString(fmt.Sprintf("**Source Documents**: %d\n", sourceCount))
	}
	if apiCount, ok := aggregate["api_endpoints_called"].(int); ok {
		sb.WriteString(fmt.Sprintf("**API Endpoints Called**: %d\n", apiCount))
	}
	sb.WriteString("\n")

	// Worker instances breakdown
	if instances, ok := aggregate["worker_instances"].([]map[string]interface{}); ok && len(instances) > 0 {
		sb.WriteString("### Worker Instances\n\n")
		sb.WriteString("| Worker Type | Ticker | Total (ms) | API Fetch (ms) | Markdown Gen (ms) | LLM Gen (ms) |\n")
		sb.WriteString("|-------------|--------|------------|----------------|-------------------|---------------|\n")
		for _, instance := range instances {
			workerType, _ := instance["worker_type"].(string)
			ticker, _ := instance["ticker"].(string)
			if ticker == "" {
				ticker = "-"
			}

			totalMs := int64(0)
			apiFetchMs := int64(0)
			jsonGenMs := int64(0)
			markdownMs := int64(0)
			aiGenMs := int64(0)

			if timing, ok := instance["timing"].(map[string]interface{}); ok {
				if t, ok := timing["total_ms"].(int64); ok {
					totalMs = t
				} else if t, ok := timing["total_ms"].(float64); ok {
					totalMs = int64(t)
				}
				if t, ok := timing["api_fetch_ms"].(int64); ok {
					apiFetchMs = t
				} else if t, ok := timing["api_fetch_ms"].(float64); ok {
					apiFetchMs = int64(t)
				}
				if t, ok := timing["json_generation_ms"].(int64); ok {
					jsonGenMs = t
				} else if t, ok := timing["json_generation_ms"].(float64); ok {
					jsonGenMs = int64(t)
				}
				if t, ok := timing["markdown_conversion_ms"].(int64); ok {
					markdownMs = t
				} else if t, ok := timing["markdown_conversion_ms"].(float64); ok {
					markdownMs = int64(t)
				}
				if t, ok := timing["ai_generation_ms"].(int64); ok {
					aiGenMs = t
				} else if t, ok := timing["ai_generation_ms"].(float64); ok {
					aiGenMs = int64(t)
				}
			}

			// Use markdown_conversion_ms if present, otherwise fall back to json_generation_ms
			markdownGenMs := markdownMs
			if markdownGenMs == 0 {
				markdownGenMs = jsonGenMs
			}

			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %d |\n",
				workerType, ticker, totalMs, apiFetchMs, markdownGenMs, aiGenMs))
		}
		sb.WriteString("\n")
	}

	// AI sources
	if aiSources, ok := aggregate["ai_sources"].([]map[string]interface{}); ok && len(aiSources) > 0 {
		sb.WriteString("### AI Sources\n\n")
		sb.WriteString("| Provider | Model | Input Tokens | Output Tokens |\n")
		sb.WriteString("|----------|-------|--------------|---------------|\n")
		for _, source := range aiSources {
			provider, _ := source["provider"].(string)
			model, _ := source["model"].(string)
			inputTokens := 0
			outputTokens := 0
			if it, ok := source["input_tokens"].(int); ok {
				inputTokens = it
			}
			if ot, ok := source["output_tokens"].(int); ok {
				outputTokens = ot
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d |\n",
				provider, model, inputTokens, outputTokens))
		}
		sb.WriteString("\n")
	}

	// Source documents list
	if sourceDocs, ok := aggregate["source_documents"].([]map[string]interface{}); ok && len(sourceDocs) > 0 {
		sb.WriteString("### Source Documents\n\n")
		sb.WriteString("| Document ID | Title | Source Type |\n")
		sb.WriteString("|-------------|-------|-------------|\n")
		for _, doc := range sourceDocs {
			id, _ := doc["id"].(string)
			title, _ := doc["title"].(string)
			sourceType, _ := doc["source_type"].(string)
			// Truncate ID for display
			shortID := id
			if len(id) > 20 {
				shortID = id[:20] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", shortID, title, sourceType))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// logJobEvent logs a job event for real-time UI display using the unified logging system
func (w *SummaryWorker) logJobEvent(ctx context.Context, parentJobID, _, level, message string, _ map[string]interface{}) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}

// stripLeadingPersonalityText removes echoed system prompt personality text from LLM output.
// LLMs sometimes echo back role instructions like "You are a Senior Investment Strategist..."
// at the beginning of their response. This function strips such text to produce clean output.
func (w *SummaryWorker) stripLeadingPersonalityText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}

	// Common personality/role prefixes that LLMs echo back
	personalityPrefixes := []string{
		"You are a ",
		"You are an ",
		"As a ",
		"As an ",
		"I am a ",
		"I am an ",
	}

	// Check if content starts with a personality prefix
	startsWithPersonality := false
	for _, prefix := range personalityPrefixes {
		if strings.HasPrefix(content, prefix) {
			startsWithPersonality = true
			break
		}
	}

	if !startsWithPersonality {
		return content
	}

	// Find where the personality text ends - look for:
	// 1. Double newline (paragraph break)
	// 2. Horizontal rule (---)
	// 3. First markdown header (# or ##)
	// The actual content typically starts after one of these

	// Look for common delimiters that separate personality text from actual content
	delimiters := []string{
		"\n\n---", // Horizontal rule
		"\n\n# ",  // H1 header
		"\n\n## ", // H2 header
		"\n---",   // Horizontal rule without double newline
		"\n# ",    // H1 header without double newline
		"\n## ",   // H2 header without double newline
	}

	for _, delimiter := range delimiters {
		idx := strings.Index(content, delimiter)
		if idx != -1 {
			// Found delimiter - strip everything before it
			// Keep the delimiter if it's a header (starts with #)
			remainder := content[idx:]
			remainder = strings.TrimPrefix(remainder, "\n\n")
			remainder = strings.TrimPrefix(remainder, "\n")

			w.logger.Debug().
				Int("original_len", len(content)).
				Int("stripped_len", len(remainder)).
				Str("delimiter", delimiter).
				Msg("Stripped leading personality text from LLM output")

			return remainder
		}
	}

	// If we didn't find a clear delimiter, try finding the first double newline
	// and check if what follows looks like actual content (starts with #, *, -, |, etc.)
	doubleNewline := strings.Index(content, "\n\n")
	if doubleNewline != -1 && doubleNewline < 500 { // Personality text is usually short
		remainder := strings.TrimSpace(content[doubleNewline+2:])
		// Check if remainder starts with markdown content indicators
		contentStarters := []string{"#", "*", "-", "|", "**", "##", "###", "1.", "2.", "3."}
		for _, starter := range contentStarters {
			if strings.HasPrefix(remainder, starter) {
				w.logger.Debug().
					Int("original_len", len(content)).
					Int("stripped_len", len(remainder)).
					Msg("Stripped leading personality paragraph from LLM output")
				return remainder
			}
		}
	}

	// No clear delimiter found - return original content
	// Better to show personality text than accidentally strip real content
	w.logger.Warn().
		Str("prefix", content[:min(100, len(content))]).
		Msg("Content starts with personality prefix but no clear delimiter found - keeping original")

	return content
}

// generateCritique generates a critique of the draft summary
func (w *SummaryWorker) generateCritique(ctx context.Context, critiquePrompt, draftContent, parentJobID, modelOverride string) (string, error) {
	// Build system prompt for critique
	systemPrompt := fmt.Sprintf(`You are a strict document reviewer and editor.
Your task is to critique the following Draft Document based on the Rules provided.

## RULES
%s

## INSTRUCTIONS
1. Analyze the Draft Document below.
2. If the document follows all rules and is accurate, output EXACTLY: "NO_CHANGES_NEEDED"
3. If there are issues (data mismatch, bad formatting, forbidden sections, tone issues), LIST THEM SPECIFICALLY.
4. Be pedantic about data accuracy.

## DRAFT DOCUMENT
%s`, critiquePrompt, draftContent)

	// Use lightweight model for critique if possible, or same as generation
	// For now using the same model as generation to ensure reasoning capability

	// Detect provider and normalize model
	provider := w.providerFactory.DetectProvider(modelOverride)
	model := w.providerFactory.NormalizeModel(modelOverride)

	if model == "" {
		model = w.providerFactory.GetDefaultModel(provider)
	}

	w.logger.Debug().Msg("Executing critique generation")

	critiqueCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	request := &llm.ContentRequest{
		Messages: []interfaces.Message{
			{Role: "user", Content: systemPrompt},
		},
		Model:       model,
		Temperature: 0.1, // Low temp for critique
	}

	resp, err := w.providerFactory.GenerateContent(critiqueCtx, request)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// jsonToMarkdown converts a structured JSON response to markdown format.
// This function handles the output from schema-constrained LLM generation,
// parsing the JSON and formatting it as a readable markdown document.
func (w *SummaryWorker) jsonToMarkdown(jsonStr string) (string, error) {
	// Parse JSON - try direct parse first
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// Try to repair truncated JSON
		repairedJSON := repairTruncatedJSON(jsonStr)
		if repairedJSON != jsonStr {
			w.logger.Debug().
				Int("original_length", len(jsonStr)).
				Int("repaired_length", len(repairedJSON)).
				Msg("Attempting to parse repaired JSON")

			if err2 := json.Unmarshal([]byte(repairedJSON), &data); err2 != nil {
				return "", fmt.Errorf("failed to parse JSON (also tried repair): %w", err)
			}
			w.logger.Info().Msg("Successfully parsed repaired truncated JSON")
		} else {
			return "", fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	var md strings.Builder

	// Handle announcement-analysis.schema.json format (must check before generic ticker check)
	// Identified by having signal_noise_assessment field
	if _, hasSignalNoise := data["signal_noise_assessment"]; hasSignalNoise {
		return w.formatAnnouncementAnalysisToMarkdown(data)
	}

	// Handle stock analysis output format
	// Check for single stock object (ticker at root level) - stock-analysis.schema.json format
	if _, hasTicker := data["ticker"]; hasTicker {
		md.WriteString("# Stock Analysis Report\n\n")
		md.WriteString(fmt.Sprintf("**Analysis Date:** %s\n\n", time.Now().Format("January 2, 2006")))
		w.formatStockToMarkdown(&md, data)
		return md.String(), nil
	}

	// Handle announcement-analysis-report.schema.json format (multi-stock announcement analysis)
	// Identified by having 'stocks' array where items have signal_noise_assessment
	if stocks, ok := data["stocks"].([]interface{}); ok && len(stocks) > 0 {
		// Check if first stock has signal_noise_assessment (announcement-analysis-report schema)
		if firstStock, ok := stocks[0].(map[string]interface{}); ok {
			if _, hasSignalNoise := firstStock["signal_noise_assessment"]; hasSignalNoise {
				return w.formatAnnouncementAnalysisReportToMarkdown(data)
			}
		}
	}

	// Handle stocks array format (stock-analysis multi-stock format)
	if stocks, ok := data["stocks"].([]interface{}); ok {
		md.WriteString("# Stock Analysis Report\n\n")
		md.WriteString(fmt.Sprintf("**Analysis Date:** %s\n\n", time.Now().Format("January 2, 2006")))

		// Process each stock
		for _, stockRaw := range stocks {
			stock, ok := stockRaw.(map[string]interface{})
			if !ok {
				continue
			}

			ticker := getStringVal(stock, "ticker", "Unknown")
			name := getStringVal(stock, "name", "Unknown")

			md.WriteString(fmt.Sprintf("## ASX: %s (%s)\n\n", ticker, name))

			// Industry
			if industry := getStringVal(stock, "industry", ""); industry != "" {
				md.WriteString(fmt.Sprintf("**Industry:** %s\n\n", industry))
			}

			// Stock data summary
			if summary := getStringVal(stock, "stock_data_summary", ""); summary != "" {
				md.WriteString(fmt.Sprintf("### Stock Data\n%s\n\n", summary))
			}

			// Technical analysis
			if techAnalysis, ok := stock["technical_analysis"].(map[string]interface{}); ok {
				md.WriteString("### Technical Analysis\n")
				if techSummary := getStringVal(techAnalysis, "summary", ""); techSummary != "" {
					md.WriteString(fmt.Sprintf("%s\n\n", techSummary))
				}

				// Technical metrics table
				md.WriteString("| Metric | Value |\n")
				md.WriteString("|--------|-------|\n")
				if sma20 := getFloatVal(techAnalysis, "sma_20", 0); sma20 > 0 {
					md.WriteString(fmt.Sprintf("| SMA 20 | $%.2f |\n", sma20))
				}
				if sma50 := getFloatVal(techAnalysis, "sma_50", 0); sma50 > 0 {
					md.WriteString(fmt.Sprintf("| SMA 50 | $%.2f |\n", sma50))
				}
				if sma200 := getFloatVal(techAnalysis, "sma_200", 0); sma200 > 0 {
					md.WriteString(fmt.Sprintf("| SMA 200 | $%.2f |\n", sma200))
				}
				if rsi := getFloatVal(techAnalysis, "rsi_14", 0); rsi > 0 {
					md.WriteString(fmt.Sprintf("| RSI (14) | %.1f |\n", rsi))
				}
				if support := getFloatVal(techAnalysis, "support_level", 0); support > 0 {
					md.WriteString(fmt.Sprintf("| Support | $%.2f |\n", support))
				}
				if resistance := getFloatVal(techAnalysis, "resistance_level", 0); resistance > 0 {
					md.WriteString(fmt.Sprintf("| Resistance | $%.2f |\n", resistance))
				}
				if shortTrend := getStringVal(techAnalysis, "short_term_trend", ""); shortTrend != "" {
					md.WriteString(fmt.Sprintf("| Short-term Trend | %s |\n", shortTrend))
				}
				if longTrend := getStringVal(techAnalysis, "long_term_trend", ""); longTrend != "" {
					md.WriteString(fmt.Sprintf("| Long-term Trend | %s |\n", longTrend))
				}
				md.WriteString("\n")
			}

			// Announcement analysis
			if announcements := getStringVal(stock, "announcement_analysis", ""); announcements != "" {
				md.WriteString(fmt.Sprintf("### Announcement Analysis\n%s\n\n", announcements))
			}

			// Price event analysis
			if priceEvents := getStringVal(stock, "price_event_analysis", ""); priceEvents != "" {
				md.WriteString(fmt.Sprintf("### Price Event Analysis\n%s\n\n", priceEvents))
			}

			// 5-year performance
			if fiveYear := getStringVal(stock, "five_year_performance", ""); fiveYear != "" {
				md.WriteString(fmt.Sprintf("### 5-Year Performance\n%s\n\n", fiveYear))
			}

			// Quality rating
			qualityRating := getStringVal(stock, "quality_rating", "")
			qualityReasoning := getStringVal(stock, "quality_reasoning", "")
			if qualityRating != "" {
				md.WriteString(fmt.Sprintf("### Quality Assessment\n**Rating:** %s\n\n", qualityRating))
				if qualityReasoning != "" {
					md.WriteString(fmt.Sprintf("%s\n\n", qualityReasoning))
				}
			}

			// Signal-noise ratio
			if snr := getStringVal(stock, "signal_noise_ratio", ""); snr != "" {
				md.WriteString(fmt.Sprintf("**Signal-to-Noise Ratio:** %s\n\n", snr))
				if snrReasoning := getStringVal(stock, "signal_noise_reasoning", ""); snrReasoning != "" {
					md.WriteString(fmt.Sprintf("%s\n\n", snrReasoning))
				}
			}

			// 5-year CAGR
			if cagr, ok := stock["five_year_cagr"].(float64); ok {
				md.WriteString(fmt.Sprintf("**5-Year CAGR:** %.1f%%\n\n", cagr*100))
			}

			// Key metrics table
			currentPrice := getFloatVal(stock, "current_price", 0)
			marketCap := getStringVal(stock, "market_cap", "")
			peRatio := getFloatVal(stock, "pe_ratio", 0)
			eps := getFloatVal(stock, "eps", 0)
			divYield := getFloatVal(stock, "dividend_yield", 0)

			if currentPrice > 0 || marketCap != "" {
				md.WriteString("### Key Metrics\n")
				md.WriteString("| Metric | Value |\n")
				md.WriteString("|--------|-------|\n")
				if currentPrice > 0 {
					md.WriteString(fmt.Sprintf("| Current Price | $%.2f |\n", currentPrice))
				}
				if marketCap != "" {
					md.WriteString(fmt.Sprintf("| Market Cap | %s |\n", marketCap))
				}
				if peRatio > 0 {
					md.WriteString(fmt.Sprintf("| P/E Ratio | %.1f |\n", peRatio))
				}
				if eps != 0 {
					md.WriteString(fmt.Sprintf("| EPS | $%.2f |\n", eps))
				}
				if divYield > 0 {
					md.WriteString(fmt.Sprintf("| Dividend Yield | %.2f%% |\n", divYield*100))
				}
				md.WriteString("\n")
			}

			// Trader recommendation
			if traderRec, ok := stock["trader_recommendation"].(map[string]interface{}); ok {
				md.WriteString("### Trader Recommendation (1-6 week horizon)\n")
				action := getStringVal(traderRec, "action", "N/A")
				conviction := getFloatVal(traderRec, "conviction", 0)
				triggers := getStringVal(traderRec, "triggers", "")

				md.WriteString(fmt.Sprintf("**Action:** %s | **Conviction:** %.0f/10\n\n", action, conviction))
				if triggers != "" {
					md.WriteString(fmt.Sprintf("**Key Triggers:** %s\n\n", triggers))
				}
			}

			// Super recommendation
			if superRec, ok := stock["super_recommendation"].(map[string]interface{}); ok {
				md.WriteString("### Super Recommendation (6-12+ month horizon)\n")
				action := getStringVal(superRec, "action", "N/A")
				conviction := getFloatVal(superRec, "conviction", 0)
				rationale := getStringVal(superRec, "rationale", "")

				md.WriteString(fmt.Sprintf("**Action:** %s | **Conviction:** %.0f/10\n\n", action, conviction))
				if rationale != "" {
					md.WriteString(fmt.Sprintf("**Rationale:** %s\n\n", rationale))
				}
			}

			md.WriteString("---\n\n")
		}
	}

	// Handle purchase conviction output format
	if execSummary := getStringVal(data, "executive_summary", ""); execSummary != "" {
		if md.Len() == 0 {
			md.WriteString("# Purchase Conviction Analysis\n\n")
			md.WriteString(fmt.Sprintf("**Analysis Date:** %s\n\n", time.Now().Format("January 2, 2006")))
		}
		md.WriteString("## Executive Summary\n")
		md.WriteString(execSummary)
		md.WriteString("\n\n")
	}

	// Handle conviction-based stocks (different from stock analysis format)
	if stocks, ok := data["stocks"].([]interface{}); ok && md.Len() > 0 {
		// Check if these are conviction-style stocks
		for _, stockRaw := range stocks {
			stock, ok := stockRaw.(map[string]interface{})
			if !ok {
				continue
			}

			// Check for conviction_score which indicates purchase conviction format
			if _, hasConviction := stock["conviction_score"]; hasConviction {
				ticker := getStringVal(stock, "ticker", "Unknown")
				name := getStringVal(stock, "name", "Unknown")
				tier := getStringVal(stock, "tier", "Unknown")
				convictionScore := getFloatVal(stock, "conviction_score", 0)

				md.WriteString(fmt.Sprintf("## ASX: %s (%s)\n\n", ticker, name))
				md.WriteString(fmt.Sprintf("**Tier:** %s | **Conviction Score:** %.0f/100\n\n", tier, convictionScore))

				// Fundamental analysis
				if fundamental := getStringVal(stock, "fundamental_analysis", ""); fundamental != "" {
					md.WriteString(fmt.Sprintf("### Fundamental Analysis\n%s\n\n", fundamental))
				}

				// Technical analysis
				if technical := getStringVal(stock, "technical_analysis", ""); technical != "" {
					md.WriteString(fmt.Sprintf("### Technical Analysis\n%s\n\n", technical))
				}

				// Short seller bear case
				if bearCase := getStringVal(stock, "short_seller_bear_case", ""); bearCase != "" {
					md.WriteString(fmt.Sprintf("### Short Seller Bear Case\n%s\n\n", bearCase))
				}

				// Analyst resolution
				if resolution := getStringVal(stock, "analyst_resolution", ""); resolution != "" {
					md.WriteString(fmt.Sprintf("### Analyst Resolution\n%s\n\n", resolution))
				}

				// Conviction breakdown
				if breakdown, ok := stock["conviction_breakdown"].(map[string]interface{}); ok {
					md.WriteString("### Conviction Breakdown\n")
					md.WriteString("| Component | Score |\n")
					md.WriteString("|-----------|-------|\n")
					md.WriteString(fmt.Sprintf("| Fundamental | %.0f/35 |\n", getFloatVal(breakdown, "fundamental", 0)))
					md.WriteString(fmt.Sprintf("| Technical | %.0f/25 |\n", getFloatVal(breakdown, "technical", 0)))
					md.WriteString(fmt.Sprintf("| Risk | %.0f/25 |\n", getFloatVal(breakdown, "risk", 0)))
					md.WriteString(fmt.Sprintf("| Insider | %.0f/10 |\n", getFloatVal(breakdown, "insider", 0)))
					md.WriteString(fmt.Sprintf("| Macro | %.0f/5 |\n", getFloatVal(breakdown, "macro", 0)))
					md.WriteString("\n")
				}

				md.WriteString("---\n\n")
			}
		}
	}

	// Summary table
	if summaryTable, ok := data["summary_table"].([]interface{}); ok && len(summaryTable) > 0 {
		md.WriteString("## Summary Table\n\n")
		md.WriteString("| Ticker | Name | Quality | Trader Rec | Conv | Super Rec | Conv | Signal:Noise | 5Y CAGR |\n")
		md.WriteString("|--------|------|---------|------------|------|-----------|------|--------------|----------|\n")

		for _, rowRaw := range summaryTable {
			row, ok := rowRaw.(map[string]interface{})
			if !ok {
				continue
			}
			md.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %.0f | %s | %.0f | %s | %s |\n",
				getStringVal(row, "ticker", ""),
				getStringVal(row, "name", ""),
				getStringVal(row, "quality", ""),
				getStringVal(row, "trader_rec", ""),
				getFloatVal(row, "trader_conv", 0),
				getStringVal(row, "super_rec", ""),
				getFloatVal(row, "super_conv", 0),
				getStringVal(row, "signal_noise", ""),
				getStringVal(row, "cagr", ""),
			))
		}
		md.WriteString("\n")
	}

	// Comparative table (for purchase conviction)
	if compTable, ok := data["comparative_table"].([]interface{}); ok && len(compTable) > 0 {
		md.WriteString("## Comparative Analysis\n\n")
		md.WriteString("| Ticker | Name | Tier | Conviction | Fundamental | Technical | Risk | Insider | Macro |\n")
		md.WriteString("|--------|------|------|------------|-------------|-----------|------|---------|-------|\n")

		for _, rowRaw := range compTable {
			row, ok := rowRaw.(map[string]interface{})
			if !ok {
				continue
			}
			md.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f |\n",
				getStringVal(row, "ticker", ""),
				getStringVal(row, "name", ""),
				getStringVal(row, "tier", ""),
				getFloatVal(row, "conviction_score", 0),
				getFloatVal(row, "fundamental", 0),
				getFloatVal(row, "technical", 0),
				getFloatVal(row, "risk", 0),
				getFloatVal(row, "insider", 0),
				getFloatVal(row, "macro", 0),
			))
		}
		md.WriteString("\n")
	}

	// Watchlists
	if watchlists, ok := data["watchlists"].(map[string]interface{}); ok {
		md.WriteString("## Watchlists\n\n")

		if traderMomentum, ok := watchlists["trader_momentum"].([]interface{}); ok && len(traderMomentum) > 0 {
			md.WriteString("### Trader Momentum (Short-term Opportunities)\n")
			for _, ticker := range traderMomentum {
				if t, ok := ticker.(string); ok {
					md.WriteString(fmt.Sprintf("- %s\n", t))
				}
			}
			md.WriteString("\n")
		}

		if superAccumulate, ok := watchlists["super_accumulate"].([]interface{}); ok && len(superAccumulate) > 0 {
			md.WriteString("### Super Accumulate (Quality A/B Long-term)\n")
			for _, ticker := range superAccumulate {
				if t, ok := ticker.(string); ok {
					md.WriteString(fmt.Sprintf("- %s\n", t))
				}
			}
			md.WriteString("\n")
		}
	}

	// Alerts
	if alerts, ok := data["alerts"].([]interface{}); ok && len(alerts) > 0 {
		md.WriteString("## Alerts\n\n")
		for _, alert := range alerts {
			if a, ok := alert.(string); ok {
				md.WriteString(fmt.Sprintf("- %s\n", a))
			}
		}
		md.WriteString("\n")
	}

	// Warnings (for purchase conviction)
	if warnings, ok := data["warnings"].([]interface{}); ok && len(warnings) > 0 {
		md.WriteString("## Warnings\n\n")
		for _, warning := range warnings {
			if w, ok := warning.(string); ok {
				md.WriteString(fmt.Sprintf("- %s\n", w))
			}
		}
		md.WriteString("\n")
	}

	// Definitions
	if definitions, ok := data["definitions"].(map[string]interface{}); ok {
		md.WriteString("## Definitions\n\n")
		for key, val := range definitions {
			if v, ok := val.(string); ok {
				md.WriteString(fmt.Sprintf("**%s:** %s\n\n", key, v))
			}
		}
	}

	// Handle SMSF portfolio output format
	if portfolioVal, ok := data["portfolio_valuation"].([]interface{}); ok && len(portfolioVal) > 0 {
		if md.Len() == 0 {
			md.WriteString("# SMSF Portfolio Review\n\n")
			md.WriteString(fmt.Sprintf("**Analysis Date:** %s\n\n", time.Now().Format("January 2, 2006")))
		}

		// Portfolio Valuation Table
		md.WriteString("## Portfolio Valuation\n\n")
		md.WriteString("| Ticker | Name | Units | Avg Cost | Current | Value | Cost Basis | P/L | Return % | Weight |\n")
		md.WriteString("|--------|------|-------|----------|---------|-------|------------|-----|----------|--------|\n")

		for _, rowRaw := range portfolioVal {
			row, ok := rowRaw.(map[string]interface{})
			if !ok {
				continue
			}
			md.WriteString(fmt.Sprintf("| %s | %s | %.0f | $%.2f | $%.2f | $%.2f | $%.2f | $%.2f | %.1f%% | %.1f%% |\n",
				getStringVal(row, "ticker", ""),
				getStringVal(row, "name", ""),
				getFloatVal(row, "units", 0),
				getFloatVal(row, "avg_cost", 0),
				getFloatVal(row, "current_price", 0),
				getFloatVal(row, "current_value", 0),
				getFloatVal(row, "cost_basis", 0),
				getFloatVal(row, "unrealized_pl", 0),
				getFloatVal(row, "return_pct", 0)*100,
				getFloatVal(row, "weight_pct", 0)*100,
			))
		}
		md.WriteString("\n")
	}

	// Total Summary (for SMSF portfolio)
	if totalSummary, ok := data["total_summary"].(map[string]interface{}); ok {
		md.WriteString("## Total Portfolio Summary\n\n")
		md.WriteString(fmt.Sprintf("- **Total Investment:** $%.2f\n", getFloatVal(totalSummary, "total_investment", 0)))
		md.WriteString(fmt.Sprintf("- **Total Value:** $%.2f\n", getFloatVal(totalSummary, "total_value", 0)))
		md.WriteString(fmt.Sprintf("- **Total P/L:** $%.2f\n", getFloatVal(totalSummary, "total_pl", 0)))
		md.WriteString(fmt.Sprintf("- **Overall Return:** %.1f%%\n\n", getFloatVal(totalSummary, "overall_return_pct", 0)*100))
	}

	// Recommendations table (for SMSF portfolio)
	if recommendations, ok := data["recommendations"].([]interface{}); ok && len(recommendations) > 0 {
		md.WriteString("## Recommendations\n\n")
		md.WriteString("| Ticker | Quality | Trader Rec | Conv | Super Rec | Conv |\n")
		md.WriteString("|--------|---------|------------|------|-----------|------|\n")

		for _, recRaw := range recommendations {
			rec, ok := recRaw.(map[string]interface{})
			if !ok {
				continue
			}

			traderAction := ""
			traderConv := 0.0
			if traderRec, ok := rec["trader_recommendation"].(map[string]interface{}); ok {
				traderAction = getStringVal(traderRec, "action", "N/A")
				traderConv = getFloatVal(traderRec, "conviction", 0)
			}

			superAction := ""
			superConv := 0.0
			if superRec, ok := rec["super_recommendation"].(map[string]interface{}); ok {
				superAction = getStringVal(superRec, "action", "N/A")
				superConv = getFloatVal(superRec, "conviction", 0)
			}

			md.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f | %s | %.0f |\n",
				getStringVal(rec, "ticker", ""),
				getStringVal(rec, "quality_rating", ""),
				traderAction,
				traderConv,
				superAction,
				superConv,
			))
		}
		md.WriteString("\n")

		// Quality reasoning per stock
		for _, recRaw := range recommendations {
			rec, ok := recRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if reasoning := getStringVal(rec, "quality_reasoning", ""); reasoning != "" {
				md.WriteString(fmt.Sprintf("**%s Quality Reasoning:** %s\n\n", getStringVal(rec, "ticker", ""), reasoning))
			}
		}
	}

	// Portfolio Mix (for SMSF portfolio)
	if portfolioMix, ok := data["portfolio_mix"].(map[string]interface{}); ok {
		md.WriteString("## Portfolio Mix\n\n")

		if riskProfile := getStringVal(portfolioMix, "risk_profile", ""); riskProfile != "" {
			md.WriteString(fmt.Sprintf("**Risk Profile:** %s\n\n", riskProfile))
		}
		if diversScore := getStringVal(portfolioMix, "diversification_score", ""); diversScore != "" {
			md.WriteString(fmt.Sprintf("**Diversification:** %s\n\n", diversScore))
		}

		defensivePct := getFloatVal(portfolioMix, "defensive_pct", 0)
		growthPct := getFloatVal(portfolioMix, "growth_pct", 0)
		incomeYield := getFloatVal(portfolioMix, "income_yield", 0)

		if defensivePct > 0 || growthPct > 0 {
			md.WriteString(fmt.Sprintf("- **Defensive:** %.1f%%\n", defensivePct*100))
			md.WriteString(fmt.Sprintf("- **Growth:** %.1f%%\n", growthPct*100))
		}
		if incomeYield > 0 {
			md.WriteString(fmt.Sprintf("- **Portfolio Yield:** %.2f%%\n", incomeYield*100))
		}
		md.WriteString("\n")

		// Industry breakdown
		if industries, ok := portfolioMix["industry_breakdown"].([]interface{}); ok && len(industries) > 0 {
			md.WriteString("### Industry Breakdown\n\n")
			md.WriteString("| Industry | Weight |\n")
			md.WriteString("|----------|--------|\n")
			for _, indRaw := range industries {
				ind, ok := indRaw.(map[string]interface{})
				if !ok {
					continue
				}
				md.WriteString(fmt.Sprintf("| %s | %.1f%% |\n",
					getStringVal(ind, "industry", ""),
					getFloatVal(ind, "weight_pct", 0)*100,
				))
			}
			md.WriteString("\n")
		}
	}

	// Priority Actions (for SMSF portfolio)
	if priorityActions, ok := data["priority_actions"].(map[string]interface{}); ok {
		md.WriteString("## Priority Actions\n\n")

		if traderOpps, ok := priorityActions["trader_opportunities"].([]interface{}); ok && len(traderOpps) > 0 {
			md.WriteString("### Trader Opportunities (1-6 weeks)\n")
			for _, opp := range traderOpps {
				if o, ok := opp.(string); ok {
					md.WriteString(fmt.Sprintf("- %s\n", o))
				}
			}
			md.WriteString("\n")
		}

		if superAcc, ok := priorityActions["super_accumulate"].([]interface{}); ok && len(superAcc) > 0 {
			md.WriteString("### Super Accumulation Targets (6-12+ months)\n")
			for _, acc := range superAcc {
				if a, ok := acc.(string); ok {
					md.WriteString(fmt.Sprintf("- %s\n", a))
				}
			}
			md.WriteString("\n")
		}

		if rebalancing, ok := priorityActions["rebalancing"].([]interface{}); ok && len(rebalancing) > 0 {
			md.WriteString("### Rebalancing Recommendations\n")
			for _, reb := range rebalancing {
				if r, ok := reb.(string); ok {
					md.WriteString(fmt.Sprintf("- %s\n", r))
				}
			}
			md.WriteString("\n")
		}
	}

	// Risk Alerts (for SMSF portfolio)
	if riskAlerts, ok := data["risk_alerts"].([]interface{}); ok && len(riskAlerts) > 0 {
		md.WriteString("## Risk Alerts\n\n")
		for _, alert := range riskAlerts {
			if a, ok := alert.(string); ok {
				md.WriteString(fmt.Sprintf("- ⚠️ %s\n", a))
			}
		}
		md.WriteString("\n")
	}

	// If we didn't generate any meaningful content, use generic JSON to markdown conversion
	if md.Len() == 0 {
		md.WriteString("# Analysis Summary\n\n")
		md.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format("January 2, 2006")))
		formatGenericJSONToMarkdown(&md, data, 0)
	}

	return md.String(), nil
}

// formatStockToMarkdown formats a single stock object to markdown
// This handles the stock-analysis.schema.json format where ticker is at root level
func (w *SummaryWorker) formatStockToMarkdown(md *strings.Builder, stock map[string]interface{}) {
	ticker := getStringVal(stock, "ticker", "Unknown")
	name := getStringVal(stock, "name", "Unknown")

	md.WriteString(fmt.Sprintf("## ASX: %s (%s)\n\n", ticker, name))

	// Industry
	if industry := getStringVal(stock, "industry", ""); industry != "" {
		md.WriteString(fmt.Sprintf("**Industry:** %s\n\n", industry))
	}

	// Stock data summary
	if summary := getStringVal(stock, "stock_data_summary", ""); summary != "" {
		md.WriteString(fmt.Sprintf("### Stock Data\n%s\n\n", summary))
	}

	// Technical analysis
	if techAnalysis, ok := stock["technical_analysis"].(map[string]interface{}); ok {
		md.WriteString("### Technical Analysis\n")
		if techSummary := getStringVal(techAnalysis, "summary", ""); techSummary != "" {
			md.WriteString(fmt.Sprintf("%s\n\n", techSummary))
		}

		// Technical metrics table
		md.WriteString("| Metric | Value |\n")
		md.WriteString("|--------|-------|\n")
		if sma20 := getFloatVal(techAnalysis, "sma_20", 0); sma20 > 0 {
			md.WriteString(fmt.Sprintf("| SMA 20 | $%.2f |\n", sma20))
		}
		if sma50 := getFloatVal(techAnalysis, "sma_50", 0); sma50 > 0 {
			md.WriteString(fmt.Sprintf("| SMA 50 | $%.2f |\n", sma50))
		}
		if sma200 := getFloatVal(techAnalysis, "sma_200", 0); sma200 > 0 {
			md.WriteString(fmt.Sprintf("| SMA 200 | $%.2f |\n", sma200))
		}
		if rsi := getFloatVal(techAnalysis, "rsi_14", 0); rsi > 0 {
			md.WriteString(fmt.Sprintf("| RSI (14) | %.1f |\n", rsi))
		}
		if support := getFloatVal(techAnalysis, "support_level", 0); support > 0 {
			md.WriteString(fmt.Sprintf("| Support | $%.2f |\n", support))
		}
		if resistance := getFloatVal(techAnalysis, "resistance_level", 0); resistance > 0 {
			md.WriteString(fmt.Sprintf("| Resistance | $%.2f |\n", resistance))
		}
		if shortTrend := getStringVal(techAnalysis, "short_term_trend", ""); shortTrend != "" {
			md.WriteString(fmt.Sprintf("| Short-term Trend | %s |\n", shortTrend))
		}
		if longTrend := getStringVal(techAnalysis, "long_term_trend", ""); longTrend != "" {
			md.WriteString(fmt.Sprintf("| Long-term Trend | %s |\n", longTrend))
		}
		md.WriteString("\n")
	}

	// Announcement analysis
	if announcements := getStringVal(stock, "announcement_analysis", ""); announcements != "" {
		md.WriteString(fmt.Sprintf("### Announcement Analysis\n%s\n\n", announcements))
	}

	// Price event analysis
	if priceEvents := getStringVal(stock, "price_event_analysis", ""); priceEvents != "" {
		md.WriteString(fmt.Sprintf("### Price Event Analysis\n%s\n\n", priceEvents))
	}

	// 5-year performance
	if fiveYear := getStringVal(stock, "five_year_performance", ""); fiveYear != "" {
		md.WriteString(fmt.Sprintf("### 5-Year Performance\n%s\n\n", fiveYear))
	}

	// Quality rating
	qualityRating := getStringVal(stock, "quality_rating", "")
	qualityReasoning := getStringVal(stock, "quality_reasoning", "")
	if qualityRating != "" {
		md.WriteString(fmt.Sprintf("### Quality Assessment\n**Rating:** %s\n\n", qualityRating))
		if qualityReasoning != "" {
			md.WriteString(fmt.Sprintf("%s\n\n", qualityReasoning))
		}
	}

	// Signal-noise ratio
	if snr := getStringVal(stock, "signal_noise_ratio", ""); snr != "" {
		md.WriteString(fmt.Sprintf("**Signal-to-Noise Ratio:** %s\n\n", snr))
		if snrReasoning := getStringVal(stock, "signal_noise_reasoning", ""); snrReasoning != "" {
			md.WriteString(fmt.Sprintf("%s\n\n", snrReasoning))
		}
	}

	// 5-year CAGR
	if cagr := getFloatVal(stock, "five_year_cagr", 0); cagr != 0 {
		md.WriteString(fmt.Sprintf("**5-Year CAGR:** %.1f%%\n\n", cagr*100))
	}

	// Key metrics table
	currentPrice := getFloatVal(stock, "current_price", 0)
	marketCap := getStringVal(stock, "market_cap", "")
	peRatio := getFloatVal(stock, "pe_ratio", 0)
	eps := getFloatVal(stock, "eps", 0)
	divYield := getFloatVal(stock, "dividend_yield", 0)

	if currentPrice > 0 || marketCap != "" {
		md.WriteString("### Key Metrics\n")
		md.WriteString("| Metric | Value |\n")
		md.WriteString("|--------|-------|\n")
		if currentPrice > 0 {
			md.WriteString(fmt.Sprintf("| Current Price | $%.2f |\n", currentPrice))
		}
		if marketCap != "" {
			md.WriteString(fmt.Sprintf("| Market Cap | %s |\n", marketCap))
		}
		if peRatio > 0 {
			md.WriteString(fmt.Sprintf("| P/E Ratio | %.1f |\n", peRatio))
		}
		if eps != 0 {
			md.WriteString(fmt.Sprintf("| EPS | $%.2f |\n", eps))
		}
		if divYield > 0 {
			md.WriteString(fmt.Sprintf("| Dividend Yield | %.2f%% |\n", divYield*100))
		}
		md.WriteString("\n")
	}

	// Trader recommendation
	if traderRec, ok := stock["trader_recommendation"].(map[string]interface{}); ok {
		md.WriteString("### Trader Recommendation (1-6 week horizon)\n")
		action := getStringVal(traderRec, "action", "N/A")
		conviction := getFloatVal(traderRec, "conviction", 0)
		triggers := getStringVal(traderRec, "triggers", "")

		md.WriteString(fmt.Sprintf("**Action:** %s | **Conviction:** %.0f/10\n\n", action, conviction))
		if triggers != "" {
			md.WriteString(fmt.Sprintf("**Key Triggers:** %s\n\n", triggers))
		}
	}

	// Super recommendation
	if superRec, ok := stock["super_recommendation"].(map[string]interface{}); ok {
		md.WriteString("### Super Recommendation (6-12+ month horizon)\n")
		action := getStringVal(superRec, "action", "N/A")
		conviction := getFloatVal(superRec, "conviction", 0)
		rationale := getStringVal(superRec, "rationale", "")

		md.WriteString(fmt.Sprintf("**Action:** %s | **Conviction:** %.0f/10\n\n", action, conviction))
		if rationale != "" {
			md.WriteString(fmt.Sprintf("**Rationale:** %s\n\n", rationale))
		}
	}

	md.WriteString("---\n\n")
}

// formatAnnouncementAnalysisToMarkdown formats announcement-analysis.schema.json output to markdown
func (w *SummaryWorker) formatAnnouncementAnalysisToMarkdown(data map[string]interface{}) (string, error) {
	var md strings.Builder

	ticker := getStringVal(data, "ticker", "Unknown")
	companyName := getStringVal(data, "company_name", "Unknown Company")
	analysisDate := getStringVal(data, "analysis_date", time.Now().Format("2006-01-02"))
	analysisPeriod := getStringVal(data, "analysis_period", "")

	// Header
	md.WriteString(fmt.Sprintf("# Announcement Signal Analysis: %s (%s)\n\n", ticker, companyName))
	md.WriteString(fmt.Sprintf("**Analysis Date:** %s\n", analysisDate))
	if analysisPeriod != "" {
		md.WriteString(fmt.Sprintf("**Period Covered:** %s\n", analysisPeriod))
	}
	md.WriteString("\n---\n\n")

	// Executive Summary
	if execSummary := getStringVal(data, "executive_summary", ""); execSummary != "" {
		md.WriteString("## Executive Summary\n\n")
		md.WriteString(execSummary)
		md.WriteString("\n\n---\n\n")
	}

	// Signal-Noise Assessment
	if assessment, ok := data["signal_noise_assessment"].(map[string]interface{}); ok {
		md.WriteString("## Signal-Noise Assessment\n\n")

		overallRating := getStringVal(assessment, "overall_rating", "N/A")
		md.WriteString(fmt.Sprintf("**Overall Rating:** %s\n\n", overallRating))

		if reasoning := getStringVal(assessment, "overall_rating_reasoning", ""); reasoning != "" {
			md.WriteString(fmt.Sprintf("%s\n\n", reasoning))
		}

		// Counts table
		md.WriteString("### Announcement Breakdown\n\n")
		md.WriteString("| Category | Count |\n")
		md.WriteString("|----------|-------|\n")

		if highCount := getFloatVal(assessment, "high_signal_count", -1); highCount >= 0 {
			md.WriteString(fmt.Sprintf("| High Signal | %.0f |\n", highCount))
		}
		if modCount := getFloatVal(assessment, "moderate_signal_count", -1); modCount >= 0 {
			md.WriteString(fmt.Sprintf("| Moderate Signal | %.0f |\n", modCount))
		}
		if lowCount := getFloatVal(assessment, "low_signal_count", -1); lowCount >= 0 {
			md.WriteString(fmt.Sprintf("| Low Signal | %.0f |\n", lowCount))
		}
		if noiseCount := getFloatVal(assessment, "noise_count", -1); noiseCount >= 0 {
			md.WriteString(fmt.Sprintf("| Noise | %.0f |\n", noiseCount))
		}
		if totalCount := getFloatVal(assessment, "total_count", -1); totalCount >= 0 {
			md.WriteString(fmt.Sprintf("| **Total** | **%.0f** |\n", totalCount))
		}
		md.WriteString("\n")

		if noiseRatio := getFloatVal(assessment, "noise_ratio", -1); noiseRatio >= 0 {
			md.WriteString(fmt.Sprintf("**Noise Ratio:** %.1f%%\n\n", noiseRatio*100))
		}
		if accuracy := getFloatVal(assessment, "price_sensitive_accuracy", -1); accuracy >= 0 {
			md.WriteString(fmt.Sprintf("**Price-Sensitive Accuracy:** %.1f%%\n\n", accuracy))
		}

		md.WriteString("---\n\n")
	}

	// High Signal Announcements
	if announcements, ok := data["high_signal_announcements"].([]interface{}); ok && len(announcements) > 0 {
		md.WriteString("## High Signal Announcements\n\n")

		for i, annRaw := range announcements {
			ann, ok := annRaw.(map[string]interface{})
			if !ok {
				continue
			}

			date := getStringVal(ann, "date", "N/A")
			headline := getStringVal(ann, "headline", "Unknown")
			signalRating := getStringVal(ann, "signal_rating", "N/A")
			annType := getStringVal(ann, "type", "")
			rationale := getStringVal(ann, "rationale", "")
			aiAnalysis := getStringVal(ann, "ai_analysis", "")

			md.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, headline))
			md.WriteString(fmt.Sprintf("**Date:** %s | **Signal:** %s", date, signalRating))
			if annType != "" {
				md.WriteString(fmt.Sprintf(" | **Type:** %s", annType))
			}

			// Price sensitive flag
			if priceSensitive, ok := ann["price_sensitive"].(bool); ok && priceSensitive {
				md.WriteString(" | **Price Sensitive**")
			}
			md.WriteString("\n\n")

			// Price impact
			if impact, ok := ann["price_impact"].(map[string]interface{}); ok {
				priceChange := getFloatVal(impact, "price_change_percent", 0)
				volRatio := getFloatVal(impact, "volume_ratio", 0)
				impactSignal := getStringVal(impact, "impact_signal", "")

				if priceChange != 0 || volRatio > 0 {
					md.WriteString("**Market Impact:** ")
					parts := []string{}
					if priceChange != 0 {
						parts = append(parts, fmt.Sprintf("%.1f%% price change", priceChange))
					}
					if volRatio > 0 {
						parts = append(parts, fmt.Sprintf("%.1fx volume", volRatio))
					}
					md.WriteString(strings.Join(parts, ", "))
					if impactSignal != "" {
						md.WriteString(fmt.Sprintf(" (%s)", impactSignal))
					}
					md.WriteString("\n\n")
				}
			}

			if rationale != "" {
				md.WriteString(fmt.Sprintf("**Rationale:** %s\n\n", rationale))
			}

			if aiAnalysis != "" {
				md.WriteString(fmt.Sprintf("**AI Analysis:** %s\n\n", aiAnalysis))
			}

			// PDF link
			if pdfURL := getStringVal(ann, "pdf_url", ""); pdfURL != "" {
				md.WriteString(fmt.Sprintf("[View ASX Announcement PDF](%s)\n\n", pdfURL))
			}
		}

		md.WriteString("---\n\n")
	}

	// Anomalies
	if anomalies, ok := data["anomalies"].([]interface{}); ok && len(anomalies) > 0 {
		md.WriteString("## Anomalies\n\n")
		md.WriteString("*Announcements with unexpected market behavior*\n\n")

		for _, anomRaw := range anomalies {
			anom, ok := anomRaw.(map[string]interface{})
			if !ok {
				continue
			}

			date := getStringVal(anom, "date", "N/A")
			headline := getStringVal(anom, "headline", "Unknown")
			anomalyType := getStringVal(anom, "anomaly_type", "")
			explanation := getStringVal(anom, "explanation", "")

			md.WriteString(fmt.Sprintf("- **%s** (%s)", headline, date))
			if anomalyType != "" {
				md.WriteString(fmt.Sprintf(" - *%s*", anomalyType))
			}
			md.WriteString("\n")
			if explanation != "" {
				md.WriteString(fmt.Sprintf("  - %s\n", explanation))
			}
		}
		md.WriteString("\n---\n\n")
	}

	// Pre-announcement movements
	if movements, ok := data["pre_announcement_movements"].([]interface{}); ok && len(movements) > 0 {
		md.WriteString("## Pre-Announcement Movements\n\n")
		md.WriteString("*Potential information leakage indicators*\n\n")

		for _, movRaw := range movements {
			mov, ok := movRaw.(map[string]interface{})
			if !ok {
				continue
			}

			date := getStringVal(mov, "date", "N/A")
			headline := getStringVal(mov, "headline", "Unknown")
			preDrift := getFloatVal(mov, "pre_drift_percent", 0)
			interpretation := getStringVal(mov, "interpretation", "")

			md.WriteString(fmt.Sprintf("- **%s** (%s) - Pre-drift: %.1f%%\n", headline, date, preDrift))
			if interpretation != "" {
				md.WriteString(fmt.Sprintf("  - %s\n", interpretation))
			}
		}
		md.WriteString("\n---\n\n")
	}

	// Dividend announcements
	if dividends, ok := data["dividend_announcements"].([]interface{}); ok && len(dividends) > 0 {
		md.WriteString("## Dividend Announcements\n\n")

		for _, divRaw := range dividends {
			div, ok := divRaw.(map[string]interface{})
			if !ok {
				continue
			}

			date := getStringVal(div, "date", "N/A")
			headline := getStringVal(div, "headline", "Unknown")
			notes := getStringVal(div, "notes", "")

			md.WriteString(fmt.Sprintf("- **%s** (%s)\n", headline, date))
			if notes != "" {
				md.WriteString(fmt.Sprintf("  - %s\n", notes))
			}
		}
		md.WriteString("\n---\n\n")
	}

	// Key Themes
	if themes, ok := data["key_themes"].([]interface{}); ok && len(themes) > 0 {
		md.WriteString("## Key Themes\n\n")
		for _, theme := range themes {
			if t, ok := theme.(string); ok {
				md.WriteString(fmt.Sprintf("- %s\n", t))
			}
		}
		md.WriteString("\n---\n\n")
	}

	// Sources
	if sources, ok := data["sources"].(map[string]interface{}); ok {
		md.WriteString("## Sources\n\n")

		// Local documents
		if local, ok := sources["local"].([]interface{}); ok && len(local) > 0 {
			md.WriteString("### Local Documents\n")
			for _, docRaw := range local {
				doc, ok := docRaw.(map[string]interface{})
				if !ok {
					continue
				}
				title := getStringVal(doc, "title", "Unknown")
				docType := getStringVal(doc, "type", "")
				md.WriteString(fmt.Sprintf("- %s", title))
				if docType != "" {
					md.WriteString(fmt.Sprintf(" (%s)", docType))
				}
				md.WriteString("\n")
			}
			md.WriteString("\n")
		}

		// Data APIs
		if dataAPIs, ok := sources["data"].([]interface{}); ok && len(dataAPIs) > 0 {
			md.WriteString("### Data Sources\n")
			for _, apiRaw := range dataAPIs {
				api, ok := apiRaw.(map[string]interface{})
				if !ok {
					continue
				}
				name := getStringVal(api, "name", "Unknown")
				url := getStringVal(api, "url", "")
				if url != "" {
					md.WriteString(fmt.Sprintf("- [%s](%s)\n", name, url))
				} else {
					md.WriteString(fmt.Sprintf("- %s\n", name))
				}
			}
			md.WriteString("\n")
		}

		// Web sources
		if web, ok := sources["web"].([]interface{}); ok && len(web) > 0 {
			md.WriteString("### Web Sources\n")
			for _, webRaw := range web {
				webSrc, ok := webRaw.(map[string]interface{})
				if !ok {
					continue
				}
				title := getStringVal(webSrc, "title", "Unknown")
				url := getStringVal(webSrc, "url", "")
				if url != "" {
					md.WriteString(fmt.Sprintf("- [%s](%s)\n", title, url))
				} else {
					md.WriteString(fmt.Sprintf("- %s\n", title))
				}
			}
			md.WriteString("\n")
		}
	}

	return md.String(), nil
}

// formatAnnouncementAnalysisReportToMarkdown formats announcement-analysis-report.schema.json output to markdown.
// This handles multi-stock announcement analysis with stocks array.
func (w *SummaryWorker) formatAnnouncementAnalysisReportToMarkdown(data map[string]interface{}) (string, error) {
	var md strings.Builder

	analysisDate := getStringVal(data, "analysis_date", time.Now().Format("2006-01-02"))
	reportSummary := getStringVal(data, "report_summary", "")

	// Header
	md.WriteString("# Announcement Signal Analysis Report\n\n")
	md.WriteString(fmt.Sprintf("**Analysis Date:** %s\n\n", analysisDate))

	// Report Summary (cross-stock insights)
	if reportSummary != "" {
		md.WriteString("## Report Summary\n\n")
		md.WriteString(reportSummary)
		md.WriteString("\n\n---\n\n")
	}

	// Process each stock in the array
	stocks, ok := data["stocks"].([]interface{})
	if !ok || len(stocks) == 0 {
		md.WriteString("*No stocks analyzed*\n")
		return md.String(), nil
	}

	for i, stockRaw := range stocks {
		stock, ok := stockRaw.(map[string]interface{})
		if !ok {
			continue
		}

		ticker := getStringVal(stock, "ticker", "Unknown")
		companyName := getStringVal(stock, "company_name", "")
		stockAnalysisDate := getStringVal(stock, "analysis_date", analysisDate)
		analysisPeriod := getStringVal(stock, "analysis_period", "")

		// Stock Header
		if companyName != "" {
			md.WriteString(fmt.Sprintf("# %d. %s (%s)\n\n", i+1, ticker, companyName))
		} else {
			md.WriteString(fmt.Sprintf("# %d. %s\n\n", i+1, ticker))
		}
		md.WriteString(fmt.Sprintf("**Analysis Date:** %s\n", stockAnalysisDate))
		if analysisPeriod != "" {
			md.WriteString(fmt.Sprintf("**Period Covered:** %s\n", analysisPeriod))
		}
		md.WriteString("\n")

		// Executive Summary
		if execSummary := getStringVal(stock, "executive_summary", ""); execSummary != "" {
			md.WriteString("## Executive Summary\n\n")
			md.WriteString(execSummary)
			md.WriteString("\n\n")
		}

		// Signal-Noise Assessment
		if assessment, ok := stock["signal_noise_assessment"].(map[string]interface{}); ok {
			md.WriteString("## Signal-Noise Assessment\n\n")

			overallRating := getStringVal(assessment, "overall_rating", "N/A")
			md.WriteString(fmt.Sprintf("**Overall Rating:** %s\n\n", overallRating))

			if reasoning := getStringVal(assessment, "overall_rating_reasoning", ""); reasoning != "" {
				md.WriteString(fmt.Sprintf("%s\n\n", reasoning))
			}

			// Counts table
			md.WriteString("### Announcement Breakdown\n\n")
			md.WriteString("| Category | Count |\n")
			md.WriteString("|----------|-------|\n")

			if highCount := getFloatVal(assessment, "high_signal_count", -1); highCount >= 0 {
				md.WriteString(fmt.Sprintf("| High Signal | %.0f |\n", highCount))
			}
			if modCount := getFloatVal(assessment, "moderate_signal_count", -1); modCount >= 0 {
				md.WriteString(fmt.Sprintf("| Moderate Signal | %.0f |\n", modCount))
			}
			if lowCount := getFloatVal(assessment, "low_signal_count", -1); lowCount >= 0 {
				md.WriteString(fmt.Sprintf("| Low Signal | %.0f |\n", lowCount))
			}
			if noiseCount := getFloatVal(assessment, "noise_count", -1); noiseCount >= 0 {
				md.WriteString(fmt.Sprintf("| Noise | %.0f |\n", noiseCount))
			}
			if totalCount := getFloatVal(assessment, "total_count", -1); totalCount >= 0 {
				md.WriteString(fmt.Sprintf("| **Total** | **%.0f** |\n", totalCount))
			}
			md.WriteString("\n")

			if noiseRatio := getFloatVal(assessment, "noise_ratio", -1); noiseRatio >= 0 {
				md.WriteString(fmt.Sprintf("**Noise Ratio:** %.1f%%\n\n", noiseRatio*100))
			}
			if accuracy := getFloatVal(assessment, "price_sensitive_accuracy", -1); accuracy >= 0 {
				md.WriteString(fmt.Sprintf("**Price-Sensitive Accuracy:** %.1f%%\n\n", accuracy))
			}
		}

		// High Signal Announcements
		if announcements, ok := stock["high_signal_announcements"].([]interface{}); ok && len(announcements) > 0 {
			md.WriteString("## High Signal Announcements\n\n")

			for j, annRaw := range announcements {
				ann, ok := annRaw.(map[string]interface{})
				if !ok {
					continue
				}

				date := getStringVal(ann, "date", "N/A")
				headline := getStringVal(ann, "headline", "Unknown")
				signalRating := getStringVal(ann, "signal_rating", "N/A")
				annType := getStringVal(ann, "type", "")
				rationale := getStringVal(ann, "rationale", "")
				aiAnalysis := getStringVal(ann, "ai_analysis", "")

				md.WriteString(fmt.Sprintf("### %d. %s\n\n", j+1, headline))
				md.WriteString(fmt.Sprintf("**Date:** %s | **Signal:** %s", date, signalRating))
				if annType != "" {
					md.WriteString(fmt.Sprintf(" | **Type:** %s", annType))
				}

				if priceSensitive, ok := ann["price_sensitive"].(bool); ok && priceSensitive {
					md.WriteString(" | **Price Sensitive**")
				}
				md.WriteString("\n\n")

				if impact, ok := ann["price_impact"].(map[string]interface{}); ok {
					priceChange := getFloatVal(impact, "price_change_percent", 0)
					volRatio := getFloatVal(impact, "volume_ratio", 0)
					impactSignal := getStringVal(impact, "impact_signal", "")

					if priceChange != 0 || volRatio > 0 {
						md.WriteString("**Market Impact:** ")
						parts := []string{}
						if priceChange != 0 {
							parts = append(parts, fmt.Sprintf("%.1f%% price change", priceChange))
						}
						if volRatio > 0 {
							parts = append(parts, fmt.Sprintf("%.1fx volume", volRatio))
						}
						md.WriteString(strings.Join(parts, ", "))
						if impactSignal != "" {
							md.WriteString(fmt.Sprintf(" (%s)", impactSignal))
						}
						md.WriteString("\n\n")
					}
				}

				if rationale != "" {
					md.WriteString(fmt.Sprintf("**Rationale:** %s\n\n", rationale))
				}

				if aiAnalysis != "" {
					md.WriteString(fmt.Sprintf("**AI Analysis:** %s\n\n", aiAnalysis))
				}

				if pdfURL := getStringVal(ann, "pdf_url", ""); pdfURL != "" {
					md.WriteString(fmt.Sprintf("[View ASX Announcement PDF](%s)\n\n", pdfURL))
				}
			}
		}

		// Anomalies
		if anomalies, ok := stock["anomalies"].([]interface{}); ok && len(anomalies) > 0 {
			md.WriteString("## Anomalies\n\n")
			for _, anomRaw := range anomalies {
				anom, ok := anomRaw.(map[string]interface{})
				if !ok {
					continue
				}
				date := getStringVal(anom, "date", "N/A")
				headline := getStringVal(anom, "headline", "Unknown")
				anomalyType := getStringVal(anom, "anomaly_type", "")
				explanation := getStringVal(anom, "explanation", "")

				md.WriteString(fmt.Sprintf("- **%s** (%s)", headline, date))
				if anomalyType != "" {
					md.WriteString(fmt.Sprintf(" - *%s*", anomalyType))
				}
				md.WriteString("\n")
				if explanation != "" {
					md.WriteString(fmt.Sprintf("  - %s\n", explanation))
				}
			}
			md.WriteString("\n")
		}

		// Pre-announcement movements
		if movements, ok := stock["pre_announcement_movements"].([]interface{}); ok && len(movements) > 0 {
			md.WriteString("## Pre-Announcement Movements\n\n")
			for _, movRaw := range movements {
				mov, ok := movRaw.(map[string]interface{})
				if !ok {
					continue
				}
				date := getStringVal(mov, "date", "N/A")
				headline := getStringVal(mov, "headline", "Unknown")
				preDrift := getFloatVal(mov, "pre_drift_percent", 0)
				interpretation := getStringVal(mov, "interpretation", "")

				md.WriteString(fmt.Sprintf("- **%s** (%s) - Pre-drift: %.1f%%\n", headline, date, preDrift))
				if interpretation != "" {
					md.WriteString(fmt.Sprintf("  - %s\n", interpretation))
				}
			}
			md.WriteString("\n")
		}

		// Key Themes
		if themes, ok := stock["key_themes"].([]interface{}); ok && len(themes) > 0 {
			md.WriteString("## Key Themes\n\n")
			for _, theme := range themes {
				if t, ok := theme.(string); ok {
					md.WriteString(fmt.Sprintf("- %s\n", t))
				}
			}
			md.WriteString("\n")
		}

		md.WriteString("\n---\n\n")
	}

	return md.String(), nil
}

// getStringVal extracts a string value from a map with a default fallback
func getStringVal(m map[string]interface{}, key, defaultVal string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

// getFloatVal extracts a numeric value from a map, handling various types
func getFloatVal(m map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

// repairTruncatedJSON attempts to fix JSON that was truncated mid-stream.
// This handles cases where LLM output was cut off before completion.
// It finds the last complete JSON element and closes any open structures.
func repairTruncatedJSON(jsonStr string) string {
	// Check if it's already valid
	var test interface{}
	if json.Unmarshal([]byte(jsonStr), &test) == nil {
		return jsonStr
	}

	// Track open structures
	var stack []rune
	inString := false
	escaped := false
	lastValidPos := 0

	for i, ch := range jsonStr {
		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
				// This is a complete object - mark as valid position
				if len(stack) > 0 || i == len(jsonStr)-1 {
					lastValidPos = i + 1
				}
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
				// This is a complete array - mark as valid position
				if len(stack) > 0 || i == len(jsonStr)-1 {
					lastValidPos = i + 1
				}
			}
		case ',':
			// After a comma is a good truncation point
			if !inString && len(stack) > 0 {
				lastValidPos = i
			}
		}
	}

	// If still valid JSON or no open structures, return as-is
	if len(stack) == 0 {
		return jsonStr
	}

	// Try to find the last complete element
	// Look for the last comma or complete value before truncation
	repaired := jsonStr
	if lastValidPos > 0 && lastValidPos < len(jsonStr) {
		// Truncate at the last comma (removing incomplete element)
		repaired = jsonStr[:lastValidPos]
		// If we ended at a comma, remove it
		repaired = strings.TrimSuffix(repaired, ",")
	}

	// Close any open structures in reverse order
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			repaired += "}"
		} else if stack[i] == '[' {
			repaired += "]"
		}
	}

	return repaired
}

// formatGenericJSONToMarkdown recursively formats any JSON structure to markdown
// This is used as a fallback when no specific format handler matches
func formatGenericJSONToMarkdown(md *strings.Builder, data interface{}, depth int) {
	switch v := data.(type) {
	case map[string]interface{}:
		// Sort keys for consistent output
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		// Process in order: prioritize certain keys
		orderedKeys := orderMarkdownKeys(keys)
		for _, key := range orderedKeys {
			val := v[key]
			formatKeyAsMarkdownHeader(md, key, depth)
			formatGenericJSONToMarkdown(md, val, depth+1)
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				md.WriteString(fmt.Sprintf("- %s\n", s))
			} else if m, ok := item.(map[string]interface{}); ok {
				// For objects in arrays, indent and format
				formatGenericJSONToMarkdown(md, m, depth)
				md.WriteString("\n")
			} else {
				md.WriteString(fmt.Sprintf("- %v\n", item))
			}
		}
		md.WriteString("\n")
	case string:
		md.WriteString(fmt.Sprintf("%s\n\n", v))
	case float64:
		md.WriteString(fmt.Sprintf("%.2f\n\n", v))
	case int:
		md.WriteString(fmt.Sprintf("%d\n\n", v))
	case bool:
		md.WriteString(fmt.Sprintf("%v\n\n", v))
	case nil:
		md.WriteString("N/A\n\n")
	}
}

// formatKeyAsMarkdownHeader formats a JSON key as a markdown header based on depth
func formatKeyAsMarkdownHeader(md *strings.Builder, key string, depth int) {
	title := formatKeyToTitle(key)

	switch depth {
	case 0:
		md.WriteString(fmt.Sprintf("## %s\n\n", title))
	case 1:
		md.WriteString(fmt.Sprintf("### %s\n\n", title))
	default:
		md.WriteString(fmt.Sprintf("**%s:** ", title))
	}
}

// formatKeyToTitle converts snake_case or camelCase to Title Case
func formatKeyToTitle(key string) string {
	// Replace underscores with spaces
	result := strings.ReplaceAll(key, "_", " ")

	// Insert space before capital letters (for camelCase)
	var titled strings.Builder
	for i, r := range result {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Check if previous char was lowercase
			prev := rune(result[i-1])
			if prev >= 'a' && prev <= 'z' {
				titled.WriteRune(' ')
			}
		}
		titled.WriteRune(r)
	}

	// Title case each word
	words := strings.Fields(titled.String())
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// orderMarkdownKeys orders keys for consistent and logical markdown output
// Priority: summary/title first, recommendation/conclusion last
func orderMarkdownKeys(keys []string) []string {
	priority := map[string]int{
		"title":           -100,
		"name":            -99,
		"summary":         -98,
		"overview":        -97,
		"description":     -96,
		"components":      0,
		"details":         10,
		"analysis":        20,
		"recommendation":  90,
		"recommendations": 91,
		"conclusion":      95,
		"warnings":        98,
		"alerts":          99,
	}

	// Sort by priority, then alphabetically
	result := make([]string, len(keys))
	copy(result, keys)

	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			pi := priority[strings.ToLower(result[i])]
			pj := priority[strings.ToLower(result[j])]

			// If both have same priority (or neither is in map), sort alphabetically
			if pi == pj {
				if result[i] > result[j] {
					result[i], result[j] = result[j], result[i]
				}
			} else if pi > pj {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// isTickerTag checks if a tag looks like a stock ticker (2-5 lowercase letters)
// Excludes known system tags that might match the pattern
func isTickerTag(tag string) bool {
	// Must be 2-5 characters
	if len(tag) < 2 || len(tag) > 5 {
		return false
	}

	// Must be lowercase
	if tag != strings.ToLower(tag) {
		return false
	}

	// Must be all letters
	for _, c := range tag {
		if c < 'a' || c > 'z' {
			return false
		}
	}

	// Exclude known system tags
	systemTags := map[string]bool{
		"date":    true,
		"email":   true,
		"smsf":    true,
		"job":     true,
		"summary": true,
		"stock":   true,
		"asx":     true,
		"test":    true,
		"deep":    true,
		"dive":    true,
		"data":    true,
		"format":  true,
		"output":  true,
		"report":  true,
		"market":  true,
		"pdf":     true,
		"html":    true,
		"body":    true,
		"kneppy":  true,
		"stocks":  true,
	}

	return !systemTags[tag]
}
