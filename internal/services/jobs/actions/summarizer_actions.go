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

// SummarizerActionDeps holds dependencies needed by summarizer action handlers.
type SummarizerActionDeps struct {
	DocStorage interfaces.DocumentStorage
	LLMService interfaces.LLMService
	Logger     arbor.ILogger
}

// scanAction performs scanning of documents to identify those needing summarization.
// This action is responsible for SCANNING (identifying documents), not processing them.
func scanAction(ctx context.Context, step models.JobStep, sources []*models.SourceConfig, deps *SummarizerActionDeps) error {
	// Extract configuration parameters
	batchSize := extractInt(step.Config, "batch_size", 100)
	offset := extractInt(step.Config, "offset", 0)
	maxDocuments := extractInt(step.Config, "max_documents", 0)
	filterSourceType := extractString(step.Config, "filter_source_type", "")
	skipWithSummary := extractBool(step.Config, "skip_with_summary", true)
	skipEmptyContent := extractBool(step.Config, "skip_empty_content", true)

	deps.Logger.Info().
		Str("action", "scan").
		Int("batch_size", batchSize).
		Int("offset", offset).
		Int("max_documents", maxDocuments).
		Str("filter_source_type", filterSourceType).
		Bool("skip_with_summary", skipWithSummary).
		Bool("skip_empty_content", skipEmptyContent).
		Msg("Starting scan action")

	// Initialize tracking
	processedCount := 0
	skippedCount := 0
	var errors []error

	// Batch processing loop
	for {
		// Build list options
		opts := &interfaces.ListOptions{
			Limit:    batchSize,
			Offset:   offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		// Apply source type filter if specified
		if filterSourceType != "" {
			opts.SourceType = filterSourceType
		}

		// Get batch of documents
		docs, err := deps.DocStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		// Break if no more documents
		if len(docs) == 0 {
			break
		}

		deps.Logger.Debug().
			Int("batch_size", len(docs)).
			Int("offset", offset).
			Msg("Processing document batch")

		// Process each document in batch
		for _, doc := range docs {
			// Skip if already has summary
			if skipWithSummary {
				if summary, exists := doc.Metadata["summary"]; exists && summary != "" {
					skippedCount++
					continue
				}
			}

			// Skip if no markdown content
			if skipEmptyContent && doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			processedCount++

			// Log progress every 10 documents
			if processedCount%10 == 0 {
				deps.Logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Msg("Scan progress")
			}
		}

		// Increment offset for next batch
		offset += batchSize

		// Check max documents limit
		if maxDocuments > 0 && processedCount >= maxDocuments {
			break
		}
	}

	// Return aggregated errors if any
	if len(errors) > 0 {
		return fmt.Errorf("scan action completed with %d error(s): %v", len(errors), errors)
	}

	deps.Logger.Info().
		Str("action", "scan").
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Msg("Scan action completed successfully")

	return nil
}

// summarizeAction performs summarization on documents using LLM.
func summarizeAction(ctx context.Context, step models.JobStep, sources []*models.SourceConfig, deps *SummarizerActionDeps) error {
	// Extract configuration parameters
	batchSize := extractInt(step.Config, "batch_size", 100)
	offset := extractInt(step.Config, "offset", 0)
	maxDocuments := extractInt(step.Config, "max_documents", 0)
	filterSourceType := extractString(step.Config, "filter_source_type", "")
	skipWithSummary := extractBool(step.Config, "skip_with_summary", true)
	contentLimit := extractInt(step.Config, "content_limit", 2000)
	systemPrompt := extractString(step.Config, "system_prompt", "You are a helpful assistant that generates concise summaries. Provide a 2-3 sentence summary of the following content.")
	includeKeywords := extractBool(step.Config, "include_keywords", true)
	includeWordCount := extractBool(step.Config, "include_word_count", true)
	topNKeywords := extractInt(step.Config, "top_n_keywords", 10)

	deps.Logger.Info().
		Str("action", "summarize").
		Int("batch_size", batchSize).
		Int("offset", offset).
		Int("max_documents", maxDocuments).
		Str("filter_source_type", filterSourceType).
		Bool("skip_with_summary", skipWithSummary).
		Int("content_limit", contentLimit).
		Bool("include_keywords", includeKeywords).
		Bool("include_word_count", includeWordCount).
		Int("top_n_keywords", topNKeywords).
		Msg("Starting summarize action")

	// Initialize tracking
	processedCount := 0
	skippedCount := 0
	var errors []error

	// Batch processing loop
	for {
		// Build list options
		opts := &interfaces.ListOptions{
			Limit:    batchSize,
			Offset:   offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		// Apply source type filter if specified
		if filterSourceType != "" {
			opts.SourceType = filterSourceType
		}

		// Get batch of documents
		docs, err := deps.DocStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		// Break if no more documents
		if len(docs) == 0 {
			break
		}

		deps.Logger.Debug().
			Int("batch_size", len(docs)).
			Int("offset", offset).
			Msg("Processing document batch")

		// Process each document
		for _, doc := range docs {
			// Skip if already has summary
			if skipWithSummary {
				if summary, exists := doc.Metadata["summary"]; exists && summary != "" {
					skippedCount++
					continue
				}
			}

			// Skip if no markdown content
			if doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			// Generate summary
			summary, err := generateSummary(ctx, doc.ContentMarkdown, contentLimit, systemPrompt, deps.LLMService, deps.Logger)
			if err != nil {
				deps.Logger.Warn().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to generate summary, using placeholder")
				summary = "Summary not available"
				errors = append(errors, fmt.Errorf("document %s: %w", doc.ID, err))

				// Check error strategy
				if step.OnError == models.ErrorStrategyFail {
					return fmt.Errorf("failed to generate summary for document %s: %w", doc.ID, err)
				}
			}

			// Calculate word count if enabled
			var wordCount int
			if includeWordCount {
				wordCount = calculateWordCount(doc.ContentMarkdown)
			}

			// Extract keywords if enabled
			var keywords []string
			if includeKeywords {
				keywords = extractKeywords(doc.ContentMarkdown, topNKeywords, 3, stopWords, deps.Logger)
			}

			// Initialize metadata if nil
			if doc.Metadata == nil {
				doc.Metadata = make(map[string]interface{})
			}

			// Update metadata
			doc.Metadata["summary"] = summary
			if includeWordCount {
				doc.Metadata["word_count"] = wordCount
			}
			if includeKeywords {
				doc.Metadata["keywords"] = keywords
			}
			doc.Metadata["last_summarized"] = time.Now().Format(time.RFC3339)

			// Save document
			if err := deps.DocStorage.UpdateDocument(doc); err != nil {
				errMsg := fmt.Errorf("failed to update document %s: %w", doc.ID, err)
				deps.Logger.Error().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to update document")
				errors = append(errors, errMsg)

				// Check error strategy
				if step.OnError == models.ErrorStrategyFail {
					return errMsg
				}
			}

			processedCount++

			// Log progress every 10 documents
			if processedCount%10 == 0 {
				deps.Logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Msg("Summarize progress")
			}
		}

		// Increment offset for next batch
		offset += batchSize

		// Check max documents limit
		if maxDocuments > 0 && processedCount >= maxDocuments {
			break
		}
	}

	// Return aggregated errors if any
	if len(errors) > 0 {
		deps.Logger.Warn().
			Int("error_count", len(errors)).
			Int("processed", processedCount).
			Msg("Summarize action completed with errors")
		return fmt.Errorf("summarize action completed with %d error(s): %v", len(errors), errors)
	}

	deps.Logger.Info().
		Str("action", "summarize").
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Msg("Summarize action completed successfully")

	return nil
}

// extractKeywordsAction performs keyword extraction on documents.
func extractKeywordsAction(ctx context.Context, step models.JobStep, sources []*models.SourceConfig, deps *SummarizerActionDeps) error {
	// Extract configuration parameters
	batchSize := extractInt(step.Config, "batch_size", 100)
	offset := extractInt(step.Config, "offset", 0)
	maxDocuments := extractInt(step.Config, "max_documents", 0)
	filterSourceType := extractString(step.Config, "filter_source_type", "")
	topN := extractInt(step.Config, "top_n", 10)
	minWordLength := extractInt(step.Config, "min_word_length", 3)
	skipWithKeywords := extractBool(step.Config, "skip_with_keywords", false)

	deps.Logger.Info().
		Str("action", "extract_keywords").
		Int("batch_size", batchSize).
		Int("offset", offset).
		Int("max_documents", maxDocuments).
		Str("filter_source_type", filterSourceType).
		Int("top_n", topN).
		Int("min_word_length", minWordLength).
		Bool("skip_with_keywords", skipWithKeywords).
		Msg("Starting extract keywords action")

	// Initialize tracking
	processedCount := 0
	skippedCount := 0
	var errors []error

	// Batch processing loop
	for {
		// Build list options
		opts := &interfaces.ListOptions{
			Limit:    batchSize,
			Offset:   offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		// Apply source type filter if specified
		if filterSourceType != "" {
			opts.SourceType = filterSourceType
		}

		// Get batch of documents
		docs, err := deps.DocStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		// Break if no more documents
		if len(docs) == 0 {
			break
		}

		deps.Logger.Debug().
			Int("batch_size", len(docs)).
			Int("offset", offset).
			Msg("Processing document batch")

		// Process each document
		for _, doc := range docs {
			// Skip if already has keywords
			if skipWithKeywords {
				if keywords, exists := doc.Metadata["keywords"]; exists && keywords != nil {
					if kwSlice, ok := keywords.([]interface{}); ok && len(kwSlice) > 0 {
						skippedCount++
						continue
					} else if kwArr, ok := keywords.([]string); ok && len(kwArr) > 0 {
						skippedCount++
						continue
					}
				}
			}

			// Skip if no markdown content
			if doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			// Extract keywords
			keywords := extractKeywords(doc.ContentMarkdown, topN, minWordLength, stopWords, deps.Logger)

			// Initialize metadata if nil
			if doc.Metadata == nil {
				doc.Metadata = make(map[string]interface{})
			}

			// Update metadata
			doc.Metadata["keywords"] = keywords
			doc.Metadata["last_keyword_extraction"] = time.Now().Format(time.RFC3339)

			// Save document
			if err := deps.DocStorage.UpdateDocument(doc); err != nil {
				errMsg := fmt.Errorf("failed to update document %s: %w", doc.ID, err)
				deps.Logger.Error().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to update document")
				errors = append(errors, errMsg)

				// Check error strategy
				if step.OnError == models.ErrorStrategyFail {
					return errMsg
				}
			}

			processedCount++

			// Log progress every 10 documents
			if processedCount%10 == 0 {
				deps.Logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Msg("Extract keywords progress")
			}
		}

		// Increment offset for next batch
		offset += batchSize

		// Check max documents limit
		if maxDocuments > 0 && processedCount >= maxDocuments {
			break
		}
	}

	// Return aggregated errors if any
	if len(errors) > 0 {
		deps.Logger.Warn().
			Int("error_count", len(errors)).
			Int("processed", processedCount).
			Msg("Extract keywords action completed with errors")
		return fmt.Errorf("extract keywords action completed with %d error(s): %v", len(errors), errors)
	}

	deps.Logger.Info().
		Str("action", "extract_keywords").
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Msg("Extract keywords action completed successfully")

	return nil
}

// Helper functions

// generateSummary generates a summary using the LLM service
func generateSummary(ctx context.Context, content string, contentLimit int, systemPrompt string, llmService interfaces.LLMService, logger arbor.ILogger) (string, error) {
	// Limit content to specified character limit
	summaryContent := content
	if len(content) > contentLimit {
		summaryContent = content[:contentLimit] + "..."
	}

	// Create chat messages
	userPrompt := fmt.Sprintf("Summarize this content:\n\n%s", summaryContent)

	messages := []interfaces.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Generate summary using LLM
	summary, err := llmService.Chat(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("llm chat failed: %w", err)
	}

	return strings.TrimSpace(summary), nil
}

// extractKeywords performs frequency analysis to extract keywords
func extractKeywords(content string, topN int, minWordLength int, stopWords map[string]bool, logger arbor.ILogger) []string {
	// Normalize content to lowercase
	content = strings.ToLower(content)

	// Remove common markdown syntax
	content = strings.ReplaceAll(content, "#", "")
	content = strings.ReplaceAll(content, "*", "")
	content = strings.ReplaceAll(content, "_", "")
	content = strings.ReplaceAll(content, "`", "")

	// Split into words
	words := strings.Fields(content)

	// Count word frequency
	frequency := make(map[string]int)
	for _, word := range words {
		// Clean word (remove punctuation)
		word = strings.Trim(word, ".,;:!?()[]{}\"'")

		// Skip if empty, too short, or stop word
		if len(word) < minWordLength || stopWords[word] {
			continue
		}

		frequency[word]++
	}

	// Convert to sorted slice
	type wordFreq struct {
		word  string
		count int
	}

	var freqList []wordFreq
	for word, count := range frequency {
		freqList = append(freqList, wordFreq{word, count})
	}

	// Sort by frequency (descending)
	sort.Slice(freqList, func(i, j int) bool {
		return freqList[i].count > freqList[j].count
	})

	// Extract top N keywords
	keywords := make([]string, 0, topN)
	for i := 0; i < len(freqList) && i < topN; i++ {
		keywords = append(keywords, freqList[i].word)
	}

	return keywords
}

// calculateWordCount counts words in markdown content
func calculateWordCount(content string) int {
	// Remove markdown syntax for accurate word count
	content = strings.ReplaceAll(content, "#", "")
	content = strings.ReplaceAll(content, "*", "")
	content = strings.ReplaceAll(content, "_", "")
	content = strings.ReplaceAll(content, "`", "")
	content = strings.ReplaceAll(content, "[", "")
	content = strings.ReplaceAll(content, "]", "")
	content = strings.ReplaceAll(content, "(", "")
	content = strings.ReplaceAll(content, ")", "")

	// Split into words and count
	words := strings.Fields(content)
	return len(words)
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
	scanActionHandler := func(ctx context.Context, step models.JobStep, sources []*models.SourceConfig) error {
		return scanAction(ctx, step, sources, deps)
	}

	summarizeActionHandler := func(ctx context.Context, step models.JobStep, sources []*models.SourceConfig) error {
		return summarizeAction(ctx, step, sources, deps)
	}

	extractKeywordsActionHandler := func(ctx context.Context, step models.JobStep, sources []*models.SourceConfig) error {
		return extractKeywordsAction(ctx, step, sources, deps)
	}

	// Register actions
	if err := registry.RegisterAction(models.JobTypeSummarizer, "scan", scanActionHandler); err != nil {
		return fmt.Errorf("failed to register scan action: %w", err)
	}

	if err := registry.RegisterAction(models.JobTypeSummarizer, "summarize", summarizeActionHandler); err != nil {
		return fmt.Errorf("failed to register summarize action: %w", err)
	}

	if err := registry.RegisterAction(models.JobTypeSummarizer, "extract_keywords", extractKeywordsActionHandler); err != nil {
		return fmt.Errorf("failed to register extract_keywords action: %w", err)
	}

	deps.Logger.Info().
		Str("job_type", string(models.JobTypeSummarizer)).
		Int("action_count", 3).
		Msg("Summarizer actions registered successfully")

	return nil
}
