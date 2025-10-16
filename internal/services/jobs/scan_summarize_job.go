package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
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

// ScanSummarizeJob implements the scan and summarize default job
type ScanSummarizeJob struct {
	docStorage interfaces.DocumentStorage
	llmService interfaces.LLMService
	logger     arbor.ILogger
}

// NewScanSummarizeJob creates a new scan and summarize job
func NewScanSummarizeJob(
	docStorage interfaces.DocumentStorage,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
) *ScanSummarizeJob {
	return &ScanSummarizeJob{
		docStorage: docStorage,
		llmService: llmService,
		logger:     logger,
	}
}

// Execute runs the scan and summarize job
func (j *ScanSummarizeJob) Execute() error {
	ctx := context.Background()

	j.logger.Info().Msg("Starting scan and summarize job")

	// Query documents in batches
	batchSize := 100
	offset := 0
	processedCount := 0
	var errors []error

	for {
		// Get batch of documents
		opts := &interfaces.ListOptions{
			Limit:    batchSize,
			Offset:   offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		docs, err := j.docStorage.ListDocuments(opts)
		if err != nil {
			return fmt.Errorf("failed to list documents: %w", err)
		}

		// Break if no more documents
		if len(docs) == 0 {
			break
		}

		j.logger.Debug().
			Int("batch_size", len(docs)).
			Int("offset", offset).
			Msg("Processing document batch")

		// Process each document
		for _, doc := range docs {
			// Skip if already has summary
			if summary, exists := doc.Metadata["summary"]; exists && summary != "" {
				continue
			}

			// Skip if no markdown content
			if doc.ContentMarkdown == "" {
				continue
			}

			if err := j.processDocument(ctx, doc); err != nil {
				j.logger.Error().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to process document")
				errors = append(errors, fmt.Errorf("document %s: %w", doc.ID, err))
			}

			processedCount++

			// Log progress every 10 documents
			if processedCount%10 == 0 {
				j.logger.Info().
					Int("processed", processedCount).
					Msg("Scan and summarize progress")
			}
		}

		// Move to next batch
		offset += batchSize
	}

	// Return aggregated errors if any
	if len(errors) > 0 {
		j.logger.Warn().
			Int("error_count", len(errors)).
			Int("processed", processedCount).
			Msg("Scan and summarize job completed with errors")
		return fmt.Errorf("scan job completed with %d error(s): %v", len(errors), errors)
	}

	j.logger.Info().
		Int("processed", processedCount).
		Msg("Scan and summarize job completed successfully")
	return nil
}

// processDocument processes a single document
func (j *ScanSummarizeJob) processDocument(ctx context.Context, doc *models.Document) error {
	// Generate summary
	summary, err := j.generateSummary(ctx, doc.ContentMarkdown)
	if err != nil {
		j.logger.Warn().
			Err(err).
			Str("doc_id", doc.ID).
			Msg("Failed to generate summary, using placeholder")
		summary = "Summary not available"
	}

	// Calculate word count
	wordCount := j.calculateWordCount(doc.ContentMarkdown)

	// Extract keywords
	keywords := j.extractKeywords(doc.ContentMarkdown, 10)

	// Initialize metadata if nil
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	// Update metadata
	doc.Metadata["summary"] = summary
	doc.Metadata["word_count"] = wordCount
	doc.Metadata["keywords"] = keywords
	doc.Metadata["last_summarized"] = time.Now().Format(time.RFC3339)

	// Save document
	if err := j.docStorage.UpdateDocument(doc); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	j.logger.Debug().
		Str("doc_id", doc.ID).
		Int("word_count", wordCount).
		Int("keyword_count", len(keywords)).
		Msg("Document processed successfully")

	return nil
}

// generateSummary generates a summary using the LLM service
func (j *ScanSummarizeJob) generateSummary(ctx context.Context, content string) (string, error) {
	// Limit content to first 2000 characters for summary generation
	summaryContent := content
	if len(content) > 2000 {
		summaryContent = content[:2000] + "..."
	}

	// Create chat request
	systemPrompt := "You are a helpful assistant that generates concise summaries. Provide a 2-3 sentence summary of the following content."
	userPrompt := fmt.Sprintf("Summarize this content:\n\n%s", summaryContent)

	messages := []interfaces.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	req := &interfaces.ChatRequest{
		Messages:    messages,
		Temperature: 0.3, // Lower temperature for more focused summaries
		MaxTokens:   150, // Limit to about 2-3 sentences
	}

	// Generate summary
	resp, err := j.llmService.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("llm chat failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

// extractKeywords performs frequency analysis to extract keywords
func (j *ScanSummarizeJob) extractKeywords(content string, topN int) []string {
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
		if len(word) < 3 || stopWords[word] {
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
func (j *ScanSummarizeJob) calculateWordCount(content string) int {
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

// findSimilarSources groups documents by metadata fields (future enhancement)
// Currently not implemented but placeholder for Phase 4 MCP enhancement
func (j *ScanSummarizeJob) findSimilarSources(doc *models.Document, allDocs []*models.Document) []string {
	// TODO: Implement similar source grouping by project_key, space_key, etc.
	// This will be used by MCP tools in Phase 4
	return []string{}
}
