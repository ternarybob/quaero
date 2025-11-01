package types

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// PostSummarizationJobDeps holds dependencies for post-summarization jobs
type PostSummarizationJobDeps struct {
	LLMService      interfaces.LLMService
	DocumentStorage interfaces.DocumentStorage
	JobStorage      interfaces.JobStorage
}

// PostSummarizationJob handles corpus-level summarization jobs
type PostSummarizationJob struct {
	*BaseJob
	deps *PostSummarizationJobDeps
}

// NewPostSummarizationJob creates a new post-summarization job
func NewPostSummarizationJob(base *BaseJob, deps *PostSummarizationJobDeps) *PostSummarizationJob {
	return &PostSummarizationJob{
		BaseJob: base,
		deps:    deps,
	}
}

// Execute processes a post-summarization job
func (p *PostSummarizationJob) Execute(ctx context.Context, msg *queue.JobMessage) error {
	p.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", msg.ParentID).
		Msg("Processing post-summarization job")

	// Validate message
	if err := p.Validate(msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Extract configuration
	parentID := msg.ParentID

	sourceType := ""
	if st, ok := msg.Config["source_type"].(string); ok {
		sourceType = st
	}

	// Note: entityType extracted from config but not currently used in post-summarization logic
	// Preserved for future use cases where entity-specific summarization may be needed
	_ = msg.Config["entity_type"] // Acknowledge variable for future use

	// Log job start
	if err := p.LogJobEvent(ctx, parentID, "info", "Starting post-summarization"); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to log job start event")
	}

	// Load Parent Job
	jobInterface, err := p.deps.JobStorage.GetJob(ctx, parentID)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("parent_id", parentID).
			Msg("Failed to load parent job")
		return fmt.Errorf("failed to load parent job: %w", err)
	}

	job, ok := jobInterface.(*models.CrawlJob)
	if !ok {
		p.logger.Error().
			Str("parent_id", parentID).
			Msg("Parent job is not a CrawlJob")
		return fmt.Errorf("parent job is not a CrawlJob")
	}

	// Query Documents Created During Job
	p.logger.Info().
		Str("parent_id", parentID).
		Str("created_at", job.CreatedAt.Format(time.RFC3339)).
		Str("completed_at", job.CompletedAt.Format(time.RFC3339)).
		Msg("Querying documents created during job")

	// Build ListOptions with database-level filters
	createdAfter := job.CreatedAt.Format(time.RFC3339)
	createdBefore := job.CompletedAt.Format(time.RFC3339)

	opts := &interfaces.ListOptions{
		SourceType:    sourceType, // Filter by source_type if provided (empty string = no filter)
		OrderBy:       "created_at",
		OrderDir:      "desc",
		Limit:         1000,                // Reasonable corpus size
		CreatedAfter:  &createdAfter,       // Documents created at or after job start
		CreatedBefore: &createdBefore,      // Documents created at or before job completion
	}

	filteredDocs, err := p.deps.DocumentStorage.ListDocuments(opts)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("parent_id", parentID).
			Msg("Failed to list documents")
		return fmt.Errorf("failed to list documents: %w", err)
	}

	if len(filteredDocs) == 0 {
		p.logger.Info().
			Str("parent_id", parentID).
			Msg("No documents found for summarization")
		if err := p.LogJobEvent(ctx, parentID, "info", "No documents found for summarization"); err != nil {
			p.logger.Warn().Err(err).Msg("Failed to log event")
		}
		return nil
	}

	p.logger.Info().
		Str("parent_id", parentID).
		Int("document_count", len(filteredDocs)).
		Msg("Found documents for summarization")

	// Generate Corpus Summary
	corpusText := ""
	for i, doc := range filteredDocs {
		// Limit to first 500 chars per doc to avoid token limits
		content := doc.ContentMarkdown
		runes := []rune(content)
		if len(runes) > 500 {
			content = string(runes[:500]) + "..."
		}

		corpusText += fmt.Sprintf("Document %d: %s\n%s\n---\n", i+1, doc.Title, content)

		// Truncate corpus to max 10,000 characters
		if len(corpusText) > 10000 {
			corpusText = corpusText[:10000] + "\n...(truncated)"
			break
		}
	}

	// Create LLM messages
	systemPrompt := "You are a helpful assistant that generates concise corpus-level summaries. Analyze the collection of documents and provide: 1) Overall theme/purpose, 2) Key topics covered, 3) Notable patterns or insights."
	userPrompt := fmt.Sprintf("Summarize this collection of %d documents:\n\n%s", len(filteredDocs), corpusText)

	messages := []interfaces.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	summary, err := p.deps.LLMService.Chat(ctx, messages)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("parent_id", parentID).
			Msg("Failed to generate corpus summary")
		summary = fmt.Sprintf("Summary generation failed: %s", err.Error())
	}

	p.logger.Info().
		Str("parent_id", parentID).
		Int("summary_length", len(summary)).
		Msg("Generated corpus summary")

	// Extract Keywords
	p.logger.Info().
		Str("parent_id", parentID).
		Msg("Extracting keywords from corpus")

	// Common English stop words
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
		"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
		"that": true, "the": true, "to": true, "was": true, "will": true, "with": true,
		"this": true, "but": true, "they": true, "have": true, "had": true, "what": true,
		"when": true, "where": true, "who": true, "which": true, "why": true, "how": true,
	}

	// Aggregate all content for keyword extraction
	wordFreq := make(map[string]int)
	for _, doc := range filteredDocs {
		content := strings.ToLower(doc.Title + " " + doc.ContentMarkdown)

		// Split into words (alphanumeric sequences)
		words := strings.FieldsFunc(content, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
		})

		// Count word frequencies with filtering
		for _, word := range words {
			// Filter: minimum length 4 chars and not a stop word
			if len(word) >= 4 && !stopWords[word] {
				wordFreq[word]++
			}
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

	sort.Slice(wordCounts, func(i, j int) bool {
		return wordCounts[i].count > wordCounts[j].count
	})

	// Take top 20 keywords
	keywords := []string{}
	for i := 0; i < len(wordCounts) && i < 20; i++ {
		keywords = append(keywords, wordCounts[i].word)
	}

	p.logger.Info().
		Str("parent_id", parentID).
		Int("keyword_count", len(keywords)).
		Msg("Extracted keywords")

	// Update Parent Job Metadata
	p.logger.Info().
		Str("parent_id", parentID).
		Msg("Updating parent job with corpus summary")

	// Reload parent job to ensure we have latest state
	jobInterface, err = p.deps.JobStorage.GetJob(ctx, parentID)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("parent_id", parentID).
			Msg("Failed to reload parent job")
		// Don't fail - summary was generated successfully
		if logErr := p.LogJobEvent(ctx, parentID, "warning",
			"Failed to reload parent job for metadata update"); logErr != nil {
			p.logger.Warn().Err(logErr).Msg("Failed to log warning event")
		}
	} else {
		job, ok = jobInterface.(*models.CrawlJob)
		if !ok {
			p.logger.Error().
				Str("parent_id", parentID).
				Msg("Parent job type assertion failed after reload")
		} else {
			// Initialize metadata if nil
			if job.Metadata == nil {
				job.Metadata = make(map[string]interface{})
			}

			// Update metadata
			job.Metadata["corpus_summary"] = summary
			job.Metadata["corpus_keywords"] = keywords
			job.Metadata["corpus_document_count"] = len(filteredDocs)
			job.Metadata["summarized_at"] = time.Now().Format(time.RFC3339)

			// Save updated job
			if err := p.deps.JobStorage.SaveJob(ctx, job); err != nil {
				p.logger.Error().
					Err(err).
					Str("parent_id", parentID).
					Msg("Failed to update parent job with metadata")
				// Don't fail - summary was generated successfully
				if logErr := p.LogJobEvent(ctx, parentID, "warning",
					"Failed to update parent job metadata"); logErr != nil {
					p.logger.Warn().Err(logErr).Msg("Failed to log warning event")
				}
			} else {
				p.logger.Info().
					Str("parent_id", parentID).
					Msg("Parent job updated with corpus summary")
			}
		}
	}

	// Log completion
	completionMsg := fmt.Sprintf("Post-summarization completed: %d documents, %d keywords",
		len(filteredDocs), len(keywords))
	if err := p.LogJobEvent(ctx, parentID, "info", completionMsg); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to log completion event")
	}

	p.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", parentID).
		Int("document_count", len(filteredDocs)).
		Int("keyword_count", len(keywords)).
		Msg("Post-summarization job completed successfully")

	return nil
}

// Validate validates the post-summarization message
func (p *PostSummarizationJob) Validate(msg *queue.JobMessage) error {
	if msg.ParentID == "" {
		return fmt.Errorf("parent_id is required")
	}

	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	return nil
}

// GetType returns the job type
func (p *PostSummarizationJob) GetType() string {
	return "post_summarization"
}
