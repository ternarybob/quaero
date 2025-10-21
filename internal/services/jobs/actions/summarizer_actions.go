package actions

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/jobs"
)

// Common stop words to exclude from keyword extraction
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "but": true, "by": true, "for": true, "if": true, "in": true,
	"into": true, "is": true, "it": true, "no": true, "not": true, "of": true,
	"on": true, "or": true, "such": true, "that": true, "the": true, "their": true,
	"then": true, "there": true, "these": true, "they": true, "this": true, "to": true,
	"was": true, "will": true, "with": true,
}

// Optimized markdown syntax remover using strings.Replacer
var markdownReplacer = strings.NewReplacer(
	"#", "",
	"*", "",
	"_", "",
	"`", "",
	"[", "",
	"]", "",
	"(", "",
	")", "",
)

// SummarizerActionDeps holds dependencies needed by summarizer action handlers.
type SummarizerActionDeps struct {
	DocStorage interfaces.DocumentStorage
	LLMService interfaces.LLMService
	Logger     arbor.ILogger
}

// batchConfig holds common configuration for batch processing
type batchConfig struct {
	batchSize        int
	offset           int
	maxDocuments     int
	filterSourceType string
}

// extractBatchConfig extracts common batch processing configuration from step config
func extractBatchConfig(config map[string]interface{}) batchConfig {
	batchSize := extractInt(config, "batch_size", 100)
	offset := extractInt(config, "offset", 0)
	maxDocuments := extractInt(config, "max_documents", 0)

	// Clamp to safe minimums to prevent panics and invalid queries
	if batchSize <= 0 {
		batchSize = 100
	}
	if offset < 0 {
		offset = 0
	}
	if maxDocuments < 0 {
		maxDocuments = 0
	}

	return batchConfig{
		batchSize:        batchSize,
		offset:           offset,
		maxDocuments:     maxDocuments,
		filterSourceType: extractString(config, "filter_source_type", ""),
	}
}

// buildListOptions creates ListOptions from batch config
func buildListOptions(cfg batchConfig) *interfaces.ListOptions {
	opts := &interfaces.ListOptions{
		Limit:    cfg.batchSize,
		Offset:   cfg.offset,
		OrderBy:  "updated_at",
		OrderDir: "desc",
	}
	if cfg.filterSourceType != "" {
		opts.SourceType = cfg.filterSourceType
	}
	return opts
}

// scanAction performs scanning of documents to identify those needing summarization.
func scanAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *SummarizerActionDeps) error {
	cfg := extractBatchConfig(step.Config)
	skipWithSummary := extractBool(step.Config, "skip_with_summary", true)
	skipEmptyContent := extractBool(step.Config, "skip_empty_content", true)

	// Build set of allowed source IDs
	allowedSources := make(map[string]string) // sourceID -> sourceType
	if len(sources) > 0 {
		for _, src := range sources {
			allowedSources[src.ID] = src.Type
		}
	}

	deps.Logger.Info().
		Str("action", "scan").
		Int("batch_size", cfg.batchSize).
		Int("offset", cfg.offset).
		Int("max_documents", cfg.maxDocuments).
		Str("filter_source_type", cfg.filterSourceType).
		Bool("skip_with_summary", skipWithSummary).
		Bool("skip_empty_content", skipEmptyContent).
		Int("selected_sources", len(allowedSources)).
		Msg("Starting scan action")

	processedCount := 0
	skippedCount := 0
	skippedSourceCount := 0

	for {
		opts := buildListOptions(cfg)
		docs, err := deps.DocStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		if len(docs) == 0 {
			break
		}

		deps.Logger.Debug().
			Int("batch_size", len(docs)).
			Int("offset", cfg.offset).
			Msg("Processing document batch")

		for _, doc := range docs {
			// Skip documents not in selected sources
			if len(allowedSources) > 0 {
				expectedType, inSources := allowedSources[doc.SourceID]
				if !inSources || (expectedType != "" && doc.SourceType != expectedType) {
					skippedSourceCount++
					continue
				}
			}

			// Skip documents based on criteria
			if skipWithSummary && hasNonEmptyMetadata(doc, "summary") {
				skippedCount++
				continue
			}
			if skipEmptyContent && doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			processedCount++

			if processedCount%10 == 0 {
				deps.Logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Int("skipped_source", skippedSourceCount).
					Msg("Scan progress")
			}
		}

		cfg.offset += cfg.batchSize

		if cfg.maxDocuments > 0 && processedCount >= cfg.maxDocuments {
			break
		}
	}

	deps.Logger.Info().
		Str("action", "scan").
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Int("skipped_source", skippedSourceCount).
		Msg("Scan action completed successfully")

	return nil
}

// summarizeAction performs summarization on documents using LLM.
func summarizeAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *SummarizerActionDeps) error {
	cfg := extractBatchConfig(step.Config)
	skipWithSummary := extractBool(step.Config, "skip_with_summary", true)
	contentLimit := extractInt(step.Config, "content_limit", 2000)
	systemPrompt := extractString(step.Config, "system_prompt", "You are a helpful assistant that generates concise summaries. Provide a 2-3 sentence summary of the following content.")
	includeKeywords := extractBool(step.Config, "include_keywords", true)
	includeWordCount := extractBool(step.Config, "include_word_count", true)
	topNKeywords := extractInt(step.Config, "top_n_keywords", 10)

	// Clamp to safe minimum
	if topNKeywords < 0 {
		topNKeywords = 0
	}

	// Build set of allowed source IDs
	allowedSources := make(map[string]string) // sourceID -> sourceType
	if len(sources) > 0 {
		for _, src := range sources {
			allowedSources[src.ID] = src.Type
		}
	}

	deps.Logger.Info().
		Str("action", "summarize").
		Int("batch_size", cfg.batchSize).
		Int("content_limit", contentLimit).
		Bool("include_keywords", includeKeywords).
		Bool("include_word_count", includeWordCount).
		Int("selected_sources", len(allowedSources)).
		Msg("Starting summarize action")

	processedCount := 0
	skippedCount := 0
	skippedSourceCount := 0
	errors := make([]error, 0)

	for {
		opts := buildListOptions(cfg)
		docs, err := deps.DocStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		if len(docs) == 0 {
			break
		}

		for _, doc := range docs {
			// Skip documents not in selected sources
			if len(allowedSources) > 0 {
				expectedType, inSources := allowedSources[doc.SourceID]
				if !inSources || (expectedType != "" && doc.SourceType != expectedType) {
					skippedSourceCount++
					continue
				}
			}

			if skipWithSummary && hasNonEmptyMetadata(doc, "summary") {
				skippedCount++
				continue
			}
			if doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			// Generate summary
			summary, err := generateSummary(ctx, doc.ContentMarkdown, contentLimit, systemPrompt, deps.LLMService, deps.Logger)
			if err != nil {
				deps.Logger.Warn().Err(err).Str("doc_id", doc.ID).Msg("Failed to generate summary")
				summary = "Summary not available"
				errors = append(errors, fmt.Errorf("document %s: %w", doc.ID, err))

				if step.OnError == models.ErrorStrategyFail {
					return fmt.Errorf("failed to generate summary for document %s: %w", doc.ID, err)
				}
			}

			// Initialize metadata if needed
			if doc.Metadata == nil {
				doc.Metadata = make(map[string]interface{})
			}

			// Update metadata
			doc.Metadata["summary"] = summary
			if includeWordCount {
				doc.Metadata["word_count"] = calculateWordCount(doc.ContentMarkdown)
			}
			if includeKeywords {
				doc.Metadata["keywords"] = extractKeywords(doc.ContentMarkdown, topNKeywords, 3, stopWords)
			}
			doc.Metadata["last_summarized"] = time.Now().Format(time.RFC3339)

			// Save document
			if err := deps.DocStorage.UpdateDocument(doc); err != nil {
				errMsg := fmt.Errorf("failed to update document %s: %w", doc.ID, err)
				deps.Logger.Error().Err(err).Str("doc_id", doc.ID).Msg("Failed to update document")
				errors = append(errors, errMsg)

				if step.OnError == models.ErrorStrategyFail {
					return errMsg
				}
			}

			processedCount++

			if processedCount%10 == 0 {
				deps.Logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Int("skipped_source", skippedSourceCount).
					Msg("Summarize progress")
			}
		}

		cfg.offset += cfg.batchSize

		if cfg.maxDocuments > 0 && processedCount >= cfg.maxDocuments {
			break
		}
	}

	if len(errors) > 0 {
		deps.Logger.Warn().
			Int("error_count", len(errors)).
			Int("processed", processedCount).
			Int("skipped_source", skippedSourceCount).
			Msg("Summarize action completed with errors")
		return fmt.Errorf("summarize action completed with %d error(s): %v", len(errors), errors)
	}

	deps.Logger.Info().
		Str("action", "summarize").
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Int("skipped_source", skippedSourceCount).
		Msg("Summarize action completed successfully")

	return nil
}

// extractKeywordsAction performs keyword extraction on documents.
func extractKeywordsAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *SummarizerActionDeps) error {
	cfg := extractBatchConfig(step.Config)
	topN := extractInt(step.Config, "top_n", 10)
	minWordLength := extractInt(step.Config, "min_word_length", 3)
	skipWithKeywords := extractBool(step.Config, "skip_with_keywords", false)

	// Clamp to safe minimums
	if topN < 0 {
		topN = 0
	}
	if minWordLength < 1 {
		minWordLength = 1
	}

	// Build set of allowed source IDs
	allowedSources := make(map[string]string) // sourceID -> sourceType
	if len(sources) > 0 {
		for _, src := range sources {
			allowedSources[src.ID] = src.Type
		}
	}

	deps.Logger.Info().
		Str("action", "extract_keywords").
		Int("batch_size", cfg.batchSize).
		Int("top_n", topN).
		Int("min_word_length", minWordLength).
		Bool("skip_with_keywords", skipWithKeywords).
		Int("selected_sources", len(allowedSources)).
		Msg("Starting extract keywords action")

	processedCount := 0
	skippedCount := 0
	skippedSourceCount := 0
	errors := make([]error, 0)

	for {
		opts := buildListOptions(cfg)
		docs, err := deps.DocStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		if len(docs) == 0 {
			break
		}

		for _, doc := range docs {
			// Skip documents not in selected sources
			if len(allowedSources) > 0 {
				expectedType, inSources := allowedSources[doc.SourceID]
				if !inSources || (expectedType != "" && doc.SourceType != expectedType) {
					skippedSourceCount++
					continue
				}
			}

			if skipWithKeywords && hasNonEmptyKeywords(doc) {
				skippedCount++
				continue
			}
			if doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			keywords := extractKeywords(doc.ContentMarkdown, topN, minWordLength, stopWords)

			if doc.Metadata == nil {
				doc.Metadata = make(map[string]interface{})
			}

			doc.Metadata["keywords"] = keywords
			doc.Metadata["last_keyword_extraction"] = time.Now().Format(time.RFC3339)

			if err := deps.DocStorage.UpdateDocument(doc); err != nil {
				errMsg := fmt.Errorf("failed to update document %s: %w", doc.ID, err)
				deps.Logger.Error().Err(err).Str("doc_id", doc.ID).Msg("Failed to update document")
				errors = append(errors, errMsg)

				if step.OnError == models.ErrorStrategyFail {
					return errMsg
				}
			}

			processedCount++

			if processedCount%10 == 0 {
				deps.Logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Int("skipped_source", skippedSourceCount).
					Msg("Extract keywords progress")
			}
		}

		cfg.offset += cfg.batchSize

		if cfg.maxDocuments > 0 && processedCount >= cfg.maxDocuments {
			break
		}
	}

	if len(errors) > 0 {
		deps.Logger.Warn().
			Int("error_count", len(errors)).
			Int("processed", processedCount).
			Int("skipped_source", skippedSourceCount).
			Msg("Extract keywords action completed with errors")
		return fmt.Errorf("extract keywords action completed with %d error(s): %v", len(errors), errors)
	}

	deps.Logger.Info().
		Str("action", "extract_keywords").
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Int("skipped_source", skippedSourceCount).
		Msg("Extract keywords action completed successfully")

	return nil
}

// Helper functions

// hasNonEmptyMetadata checks if document has non-empty metadata value for a given key
func hasNonEmptyMetadata(doc *models.Document, key string) bool {
	if doc.Metadata == nil {
		return false
	}
	value, exists := doc.Metadata[key]
	if !exists || value == nil {
		return false
	}
	if str, ok := value.(string); ok {
		return str != ""
	}
	return true
}

// hasNonEmptyKeywords checks if document has non-empty keywords array
func hasNonEmptyKeywords(doc *models.Document) bool {
	if doc.Metadata == nil {
		return false
	}
	keywords, exists := doc.Metadata["keywords"]
	if !exists || keywords == nil {
		return false
	}
	// Check both []interface{} and []string types
	switch kw := keywords.(type) {
	case []interface{}:
		return len(kw) > 0
	case []string:
		return len(kw) > 0
	default:
		return false
	}
}

// generateSummary generates a summary using the LLM service
func generateSummary(ctx context.Context, content string, contentLimit int, systemPrompt string, llmService interfaces.LLMService, logger arbor.ILogger) (string, error) {
	// Limit content to specified rune limit (UTF-8 safe)
	summaryContent := content
	runes := []rune(content)
	if len(runes) > contentLimit {
		summaryContent = string(runes[:contentLimit]) + "..."
	}

	messages := []interfaces.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Summarize this content:\n\n" + summaryContent},
	}

	summary, err := llmService.Chat(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("llm chat failed: %w", err)
	}

	return strings.TrimSpace(summary), nil
}

// extractKeywords performs frequency analysis to extract keywords
func extractKeywords(content string, topN int, minWordLength int, stopWords map[string]bool) []string {
	// Normalize and clean content
	content = strings.ToLower(content)
	content = strings.ReplaceAll(content, "#", " ")
	content = strings.ReplaceAll(content, "*", " ")
	content = strings.ReplaceAll(content, "_", " ")
	content = strings.ReplaceAll(content, "`", " ")

	// Split into words and count frequency
	words := strings.Fields(content)
	frequency := make(map[string]int, len(words)/2) // Pre-allocate with estimated capacity

	for _, word := range words {
		// Clean word (remove punctuation)
		word = strings.Trim(word, ".,;:!?()[]{}\"'")

		// Skip if empty, too short, or stop word
		if len(word) < minWordLength || stopWords[word] {
			continue
		}

		frequency[word]++
	}

	if len(frequency) == 0 {
		return []string{}
	}

	// Convert to sorted slice with pre-allocated capacity
	type wordFreq struct {
		word  string
		count int
	}
	freqList := make([]wordFreq, 0, len(frequency))
	for word, count := range frequency {
		freqList = append(freqList, wordFreq{word, count})
	}

	// Sort by frequency (descending)
	sort.Slice(freqList, func(i, j int) bool {
		return freqList[i].count > freqList[j].count
	})

	// Extract top N keywords
	limit := topN
	if limit > len(freqList) {
		limit = len(freqList)
	}

	keywords := make([]string, limit)
	for i := 0; i < limit; i++ {
		keywords[i] = freqList[i].word
	}

	return keywords
}

// calculateWordCount counts words in markdown content
func calculateWordCount(content string) int {
	// Remove markdown syntax using optimized replacer
	content = markdownReplacer.Replace(content)

	// Split into words and count
	return len(strings.Fields(content))
}

// RegisterSummarizerActions registers all summarizer-related actions with the job type registry.
func RegisterSummarizerActions(registry *jobs.JobTypeRegistry, deps *SummarizerActionDeps) error {
	// Validate inputs
	if registry == nil {
		return fmt.Errorf("registry cannot be nil")
	}
	if deps == nil {
		return fmt.Errorf("dependencies cannot be nil")
	}
	if deps.DocStorage == nil {
		return fmt.Errorf("DocStorage dependency cannot be nil")
	}
	if deps.LLMService == nil {
		return fmt.Errorf("LLMService dependency cannot be nil")
	}
	if deps.Logger == nil {
		return fmt.Errorf("Logger dependency cannot be nil")
	}

	// Create closure functions that capture dependencies
	actions := map[string]func(context.Context, *models.JobStep, []*models.SourceConfig) error{
		"scan": func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
			return scanAction(ctx, step, sources, deps)
		},
		"summarize": func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
			return summarizeAction(ctx, step, sources, deps)
		},
		"extract_keywords": func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
			return extractKeywordsAction(ctx, step, sources, deps)
		},
	}

	// Register all actions
	for name, handler := range actions {
		if err := registry.RegisterAction(models.JobTypeSummarizer, name, handler); err != nil {
			return fmt.Errorf("failed to register %s action: %w", name, err)
		}
	}

	deps.Logger.Info().
		Str("job_type", string(models.JobTypeSummarizer)).
		Int("action_count", len(actions)).
		Msg("Summarizer actions registered successfully")

	return nil
}
