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
	// Validate message
	if err := s.Validate(msg); err != nil {
		s.logger.LogJobError(err, fmt.Sprintf("Validation failed for action=%s, document_id=%s", msg.Config["action"], msg.Config["document_id"]))
		// Note: SummarizerJob doesn't have JobStorage dependency to update status
		// Status update would require adding JobStorage to SummarizerJobDeps
		return fmt.Errorf("invalid message: %w", err)
	}

	// TODO: Add JobStorage to SummarizerJobDeps to enable status updates on validation failure
	// This would allow consistent error handling across all job types

	// NOTE: Unlike CrawlerJob, SummarizerJob does not update job status on validation failure
	// because it lacks JobStorage dependency. This is acceptable because:
	//   1. Validation errors are rare (message structure is controlled by system)
	//   2. Worker logs the error and deletes the message
	//   3. Adding JobStorage dependency would increase coupling
	//
	// If status updates are needed in future, add JobStorage to SummarizerJobDeps.

	// Extract action from config
	action := "summarize" // Default action
	if act, ok := msg.Config["action"].(string); ok {
		action = act
	}

	// Extract document ID
	documentID := ""
	if id, ok := msg.Config["document_id"].(string); ok {
		documentID = id
	}

	if documentID == "" {
		return fmt.Errorf("document_id is required")
	}

	// Log job start
	s.logger.LogJobStart(fmt.Sprintf("Summarize document_id=%s, action=%s", documentID, action), "document", msg.Config)

	// Load document
	document, err := s.deps.DocumentStorage.GetDocument(documentID)
	if err != nil {
		s.logger.LogJobError(err, fmt.Sprintf("Failed to load document: document_id=%s", documentID))
		return fmt.Errorf("failed to load document: %w", err)
	}

	// Perform action based on type
	switch action {
	case "summarize":
		return s.summarizeDocument(ctx, document)
	case "extract_keywords":
		return s.extractKeywords(ctx, document)
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

// summarizeDocument generates a summary for a document
func (s *SummarizerJob) summarizeDocument(ctx context.Context, document *models.Document) error {
	startTime := time.Now()

	// Create LLM prompt
	systemPrompt := "You are a helpful assistant that generates concise, informative summaries of documents. Provide a clear, objective summary that captures the key points."
	userPrompt := fmt.Sprintf("Summarize the following document:\n\nTitle: %s\n\nContent:\n%s", document.Title, document.ContentMarkdown)

	messages := []interfaces.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	summary, err := s.deps.LLMService.Chat(ctx, messages)
	if err != nil {
		s.logger.LogJobError(err, fmt.Sprintf("Failed to summarize document: document_id=%s", document.ID))
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Update document with summary
	// Note: In a real implementation, you would update the document in storage
	// For now, we'll just log that it would be updated

	// Log completion
	s.logger.LogJobComplete(time.Since(startTime), len(summary))

	return nil
}

// extractKeywords extracts keywords from a document
func (s *SummarizerJob) extractKeywords(ctx context.Context, document *models.Document) error {
	startTime := time.Now()

	// Common English stop words
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
		"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
		"that": true, "the": true, "to": true, "was": true, "will": true, "with": true,
		"this": true, "but": true, "they": true, "have": true, "had": true, "what": true,
		"when": true, "where": true, "who": true, "which": true, "why": true, "how": true,
	}

	// Aggregate content
	content := strings.ToLower(document.Title + " " + document.ContentMarkdown)

	// Split into words (alphanumeric sequences)
	words := strings.FieldsFunc(content, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})

	// Count word frequencies with filtering
	wordFreq := make(map[string]int)
	for _, word := range words {
		// Filter: minimum length 4 chars and not a stop word
		if len(word) >= 4 && !stopWords[word] {
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

	sort.Slice(wordCounts, func(i, j int) bool {
		return wordCounts[i].count > wordCounts[j].count
	})

	// Take top 20 keywords
	keywords := []string{}
	for i := 0; i < len(wordCounts) && i < 20; i++ {
		keywords = append(keywords, wordCounts[i].word)
	}

	// Log completion
	s.logger.LogJobComplete(time.Since(startTime), len(keywords))

	return nil
}

// Validate validates the summarizer message
func (s *SummarizerJob) Validate(msg *queue.JobMessage) error {
	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	// Validate document_id
	if documentID, ok := msg.Config["document_id"].(string); ok && documentID == "" {
		return fmt.Errorf("document_id cannot be empty")
	}

	// Validate action if present
	if action, ok := msg.Config["action"].(string); ok {
		validActions := map[string]bool{
			"summarize":      true,
			"extract_keywords": true,
		}
		if !validActions[action] {
			return fmt.Errorf("invalid action: %s (must be summarize or extract_keywords)", action)
		}
	}

	return nil
}

// GetType returns the job type
func (s *SummarizerJob) GetType() string {
	return "summarizer"
}
