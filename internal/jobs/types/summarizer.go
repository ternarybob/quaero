package types

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/queue"
)

// SummarizerJobDeps holds dependencies for summarizer jobs
type SummarizerJobDeps struct {
	LLMService      interfaces.LLMService
	DocumentStorage interfaces.DocumentStorage
}

// SummarizerJob handles document summarization jobs
type SummarizerJob struct {
	*BaseJob
	deps *SummarizerJobDeps
}

// NewSummarizerJob creates a new summarizer job
func NewSummarizerJob(base *BaseJob, deps *SummarizerJobDeps) *SummarizerJob {
	return &SummarizerJob{
		BaseJob: base,
		deps:    deps,
	}
}

// Execute processes a summarizer job
func (s *SummarizerJob) Execute(ctx context.Context, msg *queue.JobMessage) error {
	s.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", msg.ParentID).
		Msg("Processing summarizer job")

	// Validate message
	if err := s.Validate(msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Extract action from config
	action := "summarize" // Default action
	if act, ok := msg.Config["action"].(string); ok {
		action = act
	}

	// Log job start
	if err := s.LogJobEvent(ctx, msg.ParentID, "info",
		fmt.Sprintf("Starting summarizer action: %s", action)); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to log job start event")
	}

	s.logger.Info().
		Str("action", action).
		Str("message_id", msg.ID).
		Msg("Executing summarizer action")

	// Execute appropriate action based on config
	switch action {
	case "scan":
		// Scan documents to identify those needing summarization
		if err := s.executeScanAction(ctx, msg); err != nil {
			return fmt.Errorf("scan action failed: %w", err)
		}

	case "summarize":
		// Generate summaries for documents using LLM
		if err := s.executeSummarizeAction(ctx, msg); err != nil {
			return fmt.Errorf("summarize action failed: %w", err)
		}

	case "extract_keywords":
		// Extract keywords from document content
		if err := s.executeExtractKeywordsAction(ctx, msg); err != nil {
			return fmt.Errorf("extract_keywords action failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	// Log job completion
	if err := s.LogJobEvent(ctx, msg.ParentID, "info",
		fmt.Sprintf("Completed summarizer action: %s", action)); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to log job completion event")
	}

	s.logger.Info().
		Str("message_id", msg.ID).
		Str("action", action).
		Msg("Summarizer job completed successfully")

	return nil
}

// executeScanAction scans documents to identify those needing processing
func (s *SummarizerJob) executeScanAction(ctx context.Context, msg *queue.JobMessage) error {
	// Extract batch config
	batchSize := 100
	if bs, ok := msg.Config["batch_size"].(float64); ok {
		batchSize = int(bs)
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	offset := 0
	if off, ok := msg.Config["offset"].(float64); ok {
		offset = int(off)
	}

	maxDocuments := 0 // 0 means unlimited
	if max, ok := msg.Config["max_documents"].(float64); ok {
		maxDocuments = int(max)
	}

	skipWithSummary := true
	if skip, ok := msg.Config["skip_with_summary"].(bool); ok {
		skipWithSummary = skip
	}

	skipEmptyContent := true
	if skip, ok := msg.Config["skip_empty_content"].(bool); ok {
		skipEmptyContent = skip
	}

	s.logger.Info().
		Str("message_id", msg.ID).
		Int("batch_size", batchSize).
		Int("offset", offset).
		Int("max_documents", maxDocuments).
		Bool("skip_with_summary", skipWithSummary).
		Bool("skip_empty_content", skipEmptyContent).
		Msg("Starting scan action")

	processedCount := 0
	skippedCount := 0

	for {
		// Query documents with pagination
		opts := &interfaces.ListOptions{
			Limit:    batchSize,
			Offset:   offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		docs, err := s.deps.DocumentStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		if len(docs) == 0 {
			break
		}

		// Filter documents
		for _, doc := range docs {
			if skipWithSummary && doc.Metadata != nil {
				if summary, ok := doc.Metadata["summary"].(string); ok && summary != "" {
					skippedCount++
					continue
				}
			}

			if skipEmptyContent && doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			processedCount++

			if processedCount%10 == 0 {
				s.logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Msg("Scan progress")
			}
		}

		offset += batchSize

		if maxDocuments > 0 && processedCount >= maxDocuments {
			break
		}
	}

	s.logger.Info().
		Str("message_id", msg.ID).
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Msg("Scan action completed")

	return nil
}

// executeSummarizeAction generates summaries for documents using LLM
func (s *SummarizerJob) executeSummarizeAction(ctx context.Context, msg *queue.JobMessage) error {
	// Extract batch config
	batchSize := 100
	if bs, ok := msg.Config["batch_size"].(float64); ok {
		batchSize = int(bs)
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	offset := 0
	if off, ok := msg.Config["offset"].(float64); ok {
		offset = int(off)
	}

	maxDocuments := 0
	if max, ok := msg.Config["max_documents"].(float64); ok {
		maxDocuments = int(max)
	}

	skipWithSummary := true
	if skip, ok := msg.Config["skip_with_summary"].(bool); ok {
		skipWithSummary = skip
	}

	contentLimit := 2000
	if limit, ok := msg.Config["content_limit"].(float64); ok {
		contentLimit = int(limit)
	}

	systemPrompt := "You are a helpful assistant that generates concise summaries. Provide a 2-3 sentence summary of the following content."
	if prompt, ok := msg.Config["system_prompt"].(string); ok && prompt != "" {
		systemPrompt = prompt
	}

	errorStrategy := "continue" // Default: continue on error
	if strategy, ok := msg.Config["on_error"].(string); ok {
		errorStrategy = strategy
	}

	s.logger.Info().
		Str("message_id", msg.ID).
		Int("batch_size", batchSize).
		Int("content_limit", contentLimit).
		Str("error_strategy", errorStrategy).
		Msg("Starting summarize action")

	processedCount := 0
	skippedCount := 0
	errorCount := 0

	for {
		opts := &interfaces.ListOptions{
			Limit:    batchSize,
			Offset:   offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		docs, err := s.deps.DocumentStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		if len(docs) == 0 {
			break
		}

		for _, doc := range docs {
			if skipWithSummary && doc.Metadata != nil {
				if summary, ok := doc.Metadata["summary"].(string); ok && summary != "" {
					skippedCount++
					continue
				}
			}

			if doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			// Limit content for summarization
			content := doc.ContentMarkdown
			runes := []rune(content)
			if len(runes) > contentLimit {
				content = string(runes[:contentLimit]) + "..."
			}

			// Generate summary using LLM
			messages := []interfaces.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: "Summarize this content:\n\n" + content},
			}

			summary, err := s.deps.LLMService.Chat(ctx, messages)
			if err != nil {
				s.logger.Warn().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to generate summary")

				errorCount++
				if errorStrategy == "fail" {
					return fmt.Errorf("failed to generate summary for document %s: %w", doc.ID, err)
				}
				summary = "Summary not available"
			}

			// Initialize metadata if needed
			if doc.Metadata == nil {
				doc.Metadata = make(map[string]interface{})
			}

			// Update document metadata
			doc.Metadata["summary"] = summary
			doc.Metadata["last_summarized"] = time.Now().Format(time.RFC3339)

			// Save updated document
			if err := s.deps.DocumentStorage.UpdateDocument(doc); err != nil {
				s.logger.Error().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to update document")

				errorCount++
				if errorStrategy == "fail" {
					return fmt.Errorf("failed to update document %s: %w", doc.ID, err)
				}
				continue
			}

			processedCount++

			if processedCount%10 == 0 {
				s.logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Int("errors", errorCount).
					Msg("Summarize progress")
			}
		}

		offset += batchSize

		if maxDocuments > 0 && processedCount >= maxDocuments {
			break
		}
	}

	s.logger.Info().
		Str("message_id", msg.ID).
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Int("errors", errorCount).
		Msg("Summarize action completed")

	if errorCount > 0 && errorStrategy == "continue" {
		s.logger.Warn().
			Int("error_count", errorCount).
			Msg("Summarize action completed with errors")
	}

	return nil
}

// executeExtractKeywordsAction extracts keywords from document content
func (s *SummarizerJob) executeExtractKeywordsAction(ctx context.Context, msg *queue.JobMessage) error {
	// Extract batch config
	batchSize := 100
	if bs, ok := msg.Config["batch_size"].(float64); ok {
		batchSize = int(bs)
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	offset := 0
	if off, ok := msg.Config["offset"].(float64); ok {
		offset = int(off)
	}

	maxDocuments := 0
	if max, ok := msg.Config["max_documents"].(float64); ok {
		maxDocuments = int(max)
	}

	// Extract keyword parameters
	topN := 10 // Default: top 10 keywords
	if n, ok := msg.Config["top_n"].(float64); ok {
		topN = int(n)
	}
	if topN <= 0 {
		topN = 10
	}

	minWordLength := 3 // Default: minimum 3 characters
	if minLen, ok := msg.Config["min_word_length"].(float64); ok {
		minWordLength = int(minLen)
	}
	if minWordLength < 1 {
		minWordLength = 3
	}

	skipWithKeywords := true
	if skip, ok := msg.Config["skip_with_keywords"].(bool); ok {
		skipWithKeywords = skip
	}

	errorStrategy := "continue"
	if strategy, ok := msg.Config["on_error"].(string); ok {
		errorStrategy = strategy
	}

	s.logger.Info().
		Str("message_id", msg.ID).
		Int("batch_size", batchSize).
		Int("top_n", topN).
		Int("min_word_length", minWordLength).
		Str("error_strategy", errorStrategy).
		Msg("Starting extract_keywords action")

	processedCount := 0
	skippedCount := 0
	errorCount := 0

	// Common English stop words
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
		"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
		"that": true, "the": true, "to": true, "was": true, "will": true, "with": true,
		"this": true, "but": true, "they": true, "have": true, "had": true, "what": true,
		"when": true, "where": true, "who": true, "which": true, "why": true, "how": true,
	}

	for {
		opts := &interfaces.ListOptions{
			Limit:    batchSize,
			Offset:   offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		docs, err := s.deps.DocumentStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		if len(docs) == 0 {
			break
		}

		for _, doc := range docs {
			if skipWithKeywords && doc.Metadata != nil {
				if keywords, ok := doc.Metadata["keywords"].([]interface{}); ok && len(keywords) > 0 {
					skippedCount++
					continue
				}
			}

			if doc.ContentMarkdown == "" {
				skippedCount++
				continue
			}

			// Perform frequency analysis
			wordFreq := make(map[string]int)
			content := strings.ToLower(doc.ContentMarkdown)

			// Split into words (alphanumeric sequences)
			words := strings.FieldsFunc(content, func(r rune) bool {
				return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
			})

			// Count word frequencies with filtering
			for _, word := range words {
				// Filter: minimum length and not a stop word
				if len(word) >= minWordLength && !stopWords[word] {
					wordFreq[word]++
				}
			}

			// Sort by frequency (descending)
			type wordCount struct {
				word  string
				count int
			}
			var wordCounts []wordCount
			for word, count := range wordFreq {
				wordCounts = append(wordCounts, wordCount{word, count})
			}

			// Sort descending by count
			sort.Slice(wordCounts, func(i, j int) bool {
				return wordCounts[i].count > wordCounts[j].count
			})

			// Take top N keywords
			keywords := []string{}
			for i := 0; i < len(wordCounts) && i < topN; i++ {
				keywords = append(keywords, wordCounts[i].word)
			}

			// Initialize metadata if needed
			if doc.Metadata == nil {
				doc.Metadata = make(map[string]interface{})
			}

			// Update document metadata
			doc.Metadata["keywords"] = keywords
			doc.Metadata["last_keyword_extraction"] = time.Now().Format(time.RFC3339)

			// Save updated document
			if err := s.deps.DocumentStorage.UpdateDocument(doc); err != nil {
				s.logger.Error().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to update document")

				errorCount++
				if errorStrategy == "fail" {
					return fmt.Errorf("failed to update document %s: %w", doc.ID, err)
				}
				continue
			}

			processedCount++

			if processedCount%10 == 0 {
				s.logger.Info().
					Int("processed", processedCount).
					Int("skipped", skippedCount).
					Int("errors", errorCount).
					Msg("Extract keywords progress")
			}
		}

		offset += batchSize

		if maxDocuments > 0 && processedCount >= maxDocuments {
			break
		}
	}

	s.logger.Info().
		Str("message_id", msg.ID).
		Int("processed", processedCount).
		Int("skipped", skippedCount).
		Int("errors", errorCount).
		Msg("Extract keywords action completed")

	if errorCount > 0 && errorStrategy == "continue" {
		s.logger.Warn().
			Int("error_count", errorCount).
			Msg("Extract keywords action completed with errors")
	}

	return nil
}

// Validate validates the summarizer message
func (s *SummarizerJob) Validate(msg *queue.JobMessage) error {
	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	// Validate ParentID is present (required for logging)
	if msg.ParentID == "" {
		return fmt.Errorf("parent_id is required for logging job events")
	}

	// Validate action if present
	if action, ok := msg.Config["action"].(string); ok {
		validActions := map[string]bool{
			"scan":             true,
			"summarize":        true,
			"extract_keywords": true,
		}
		if !validActions[action] {
			return fmt.Errorf("invalid action: %s (must be scan, summarize, or extract_keywords)", action)
		}
	}

	return nil
}

// GetType returns the job type
func (s *SummarizerJob) GetType() string {
	return "summarizer"
}
