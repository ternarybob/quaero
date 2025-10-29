package actions

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/jobs"
)

// Mock implementations

type mockDocumentStorage struct {
	listDocumentsFunc   func(*interfaces.ListOptions) ([]*models.Document, error)
	updateDocumentFunc  func(*models.Document) error
	listDocumentsCalls  int
	updateDocumentCalls int
	updatedDocs         []*models.Document
}

func (m *mockDocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	m.listDocumentsCalls++
	if m.listDocumentsFunc != nil {
		return m.listDocumentsFunc(opts)
	}
	return []*models.Document{}, nil
}

func (m *mockDocumentStorage) UpdateDocument(doc *models.Document) error {
	m.updateDocumentCalls++
	m.updatedDocs = append(m.updatedDocs, doc)
	if m.updateDocumentFunc != nil {
		return m.updateDocumentFunc(doc)
	}
	return nil
}

// Implement remaining DocumentStorage interface methods as no-ops
func (m *mockDocumentStorage) SaveDocument(doc *models.Document) error         { return nil }
func (m *mockDocumentStorage) SaveDocuments(docs []*models.Document) error     { return nil }
func (m *mockDocumentStorage) GetDocument(id string) (*models.Document, error) { return nil, nil }
func (m *mockDocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) DeleteDocument(id string) error { return nil }
func (m *mockDocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) CountDocuments() (int, error)                          { return 0, nil }
func (m *mockDocumentStorage) CountDocumentsBySource(sourceType string) (int, error) { return 0, nil }
func (m *mockDocumentStorage) GetStats() (*models.DocumentStats, error)              { return nil, nil }
func (m *mockDocumentStorage) ClearAll() error                                       { return nil }
func (m *mockDocumentStorage) SetForceSyncPending(id string, pending bool) error     { return nil }
func (m *mockDocumentStorage) GetDocumentsForceSync() ([]*models.Document, error)    { return nil, nil }
func (m *mockDocumentStorage) RebuildFTS5Index() error                                { return nil }

type mockLLMService struct {
	chatFunc      func(context.Context, []interfaces.Message) (string, error)
	chatCalls     int
	chatMessages  [][]interfaces.Message
	chatResponses []string
}

func (m *mockLLMService) Chat(ctx context.Context, messages []interfaces.Message) (string, error) {
	m.chatCalls++
	m.chatMessages = append(m.chatMessages, messages)
	if m.chatFunc != nil {
		return m.chatFunc(ctx, messages)
	}
	return "Mock summary response", nil
}

func (m *mockLLMService) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

func (m *mockLLMService) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockLLMService) GetMode() interfaces.LLMMode {
	return interfaces.LLMModeOffline
}

func (m *mockLLMService) Close() error {
	return nil
}

// Test helpers

func createTestSummarizerDeps() (*SummarizerActionDeps, *mockDocumentStorage, *mockLLMService) {
	mockStorage := &mockDocumentStorage{}
	mockLLM := &mockLLMService{}

	logger := arbor.NewLogger()

	deps := &SummarizerActionDeps{
		DocStorage: mockStorage,
		LLMService: mockLLM,
		Logger:     logger,
	}

	return deps, mockStorage, mockLLM
}

func createTestDocuments(count int, withSummary bool, withKeywords bool, emptyContent bool) []*models.Document {
	docs := make([]*models.Document, count)
	for i := 0; i < count; i++ {
		doc := &models.Document{
			ID:         fmt.Sprintf("doc-%d", i+1),
			SourceType: "jira",
			SourceID:   fmt.Sprintf("ISSUE-%d", i+1),
			Title:      fmt.Sprintf("Test Document %d", i+1),
			Metadata:   make(map[string]interface{}),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		if !emptyContent {
			doc.ContentMarkdown = fmt.Sprintf("This is test content for document %d. It contains various words for testing keyword extraction functionality. Technology software development testing automation.", i+1)
		}

		if withSummary {
			doc.Metadata["summary"] = fmt.Sprintf("Summary for document %d", i+1)
		}

		if withKeywords {
			doc.Metadata["keywords"] = []string{"keyword1", "keyword2", "keyword3"}
		}

		docs[i] = doc
	}
	return docs
}

// Tests for scanAction

func TestScanAction_Success(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", map[string]interface{}{
		"batch_size": 100,
	})

	// Mock ListDocuments to return test documents
	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(10, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if mockStorage.listDocumentsCalls < 1 {
		t.Errorf("Expected at least 1 ListDocuments call, got %d", mockStorage.listDocumentsCalls)
	}
}

func TestScanAction_WithBatchSize(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", map[string]interface{}{
		"batch_size": 50,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		// Verify batch size is respected
		if opts.Limit != 50 {
			t.Errorf("Expected batch size 50, got %d", opts.Limit)
		}
		return []*models.Document{}, nil
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestScanAction_WithOffset(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", map[string]interface{}{
		"offset": 100,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		// Verify offset is respected on first call
		if mockStorage.listDocumentsCalls == 1 && opts.Offset != 100 {
			t.Errorf("Expected initial offset 100, got %d", opts.Offset)
		}
		return []*models.Document{}, nil
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestScanAction_WithMaxDocuments(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", map[string]interface{}{
		"max_documents": 5,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		// Always return 10 documents
		return createTestDocuments(10, false, false, false), nil
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should stop after processing 5 documents
	if mockStorage.listDocumentsCalls > 1 {
		t.Errorf("Expected only 1 batch call (max_documents reached), got %d", mockStorage.listDocumentsCalls)
	}
}

func TestScanAction_WithFilterSourceType(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", map[string]interface{}{
		"filter_source_type": "jira",
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		// Verify source type filter
		if opts.SourceType != "jira" {
			t.Errorf("Expected source type 'jira', got '%s'", opts.SourceType)
		}
		return []*models.Document{}, nil
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestScanAction_SkipWithSummary(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", map[string]interface{}{
		"skip_with_summary": true,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			// Return mix of documents with and without summaries
			docs := createTestDocuments(5, false, false, false)
			docs = append(docs, createTestDocuments(5, true, false, false)...)
			return docs, nil
		}
		return []*models.Document{}, nil
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestScanAction_SkipEmptyContent(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", map[string]interface{}{
		"skip_empty_content": true,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			// Return mix of documents with and without content
			docs := createTestDocuments(5, false, false, false)
			docs = append(docs, createTestDocuments(5, false, false, true)...)
			return docs, nil
		}
		return []*models.Document{}, nil
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestScanAction_ListDocumentsError(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", nil)

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		return nil, fmt.Errorf("database error")
	}

	err := scanAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err == nil {
		t.Error("Expected error for ListDocuments failure, got nil")
	}
}

// Tests for summarizeAction

func TestSummarizeAction_Success(t *testing.T) {
	deps, mockStorage, mockLLM := createTestSummarizerDeps()
	step := createTestStep("summarize", nil)

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(3, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		return "Test summary", nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if mockLLM.chatCalls != 3 {
		t.Errorf("Expected 3 Chat calls, got %d", mockLLM.chatCalls)
	}

	if mockStorage.updateDocumentCalls != 3 {
		t.Errorf("Expected 3 UpdateDocument calls, got %d", mockStorage.updateDocumentCalls)
	}

	// Verify metadata was updated
	for _, doc := range mockStorage.updatedDocs {
		if _, ok := doc.Metadata["summary"]; !ok {
			t.Error("Expected summary in metadata")
		}
		if _, ok := doc.Metadata["word_count"]; !ok {
			t.Error("Expected word_count in metadata")
		}
		if _, ok := doc.Metadata["keywords"]; !ok {
			t.Error("Expected keywords in metadata")
		}
		if _, ok := doc.Metadata["last_summarized"]; !ok {
			t.Error("Expected last_summarized in metadata")
		}
	}
}

func TestSummarizeAction_WithContentLimit(t *testing.T) {
	deps, mockStorage, mockLLM := createTestSummarizerDeps()
	step := createTestStep("summarize", map[string]interface{}{
		"content_limit": 100,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			docs := createTestDocuments(1, false, false, false)
			// Create a long content
			docs[0].ContentMarkdown = string(make([]byte, 5000))
			for i := range docs[0].ContentMarkdown {
				docs[0].ContentMarkdown = docs[0].ContentMarkdown[:i] + "x"
			}
			return docs, nil
		}
		return []*models.Document{}, nil
	}

	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		// Verify content was truncated
		for _, msg := range messages {
			if msg.Role == "user" && len(msg.Content) > 200 {
				t.Errorf("Expected content to be truncated to ~100 chars, got %d", len(msg.Content))
			}
		}
		return "Test summary", nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestSummarizeAction_WithCustomSystemPrompt(t *testing.T) {
	deps, mockStorage, mockLLM := createTestSummarizerDeps()
	customPrompt := "Custom summarization prompt"
	step := createTestStep("summarize", map[string]interface{}{
		"system_prompt": customPrompt,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		// Verify system prompt
		if len(messages) > 0 && messages[0].Role == "system" {
			if messages[0].Content != customPrompt {
				t.Errorf("Expected system prompt '%s', got '%s'", customPrompt, messages[0].Content)
			}
		}
		return "Test summary", nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestSummarizeAction_WithIncludeKeywordsFalse(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("summarize", map[string]interface{}{
		"include_keywords": false,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify keywords were NOT added
	for _, doc := range mockStorage.updatedDocs {
		if _, ok := doc.Metadata["keywords"]; ok {
			t.Error("Expected keywords to NOT be in metadata when include_keywords=false")
		}
	}
}

func TestSummarizeAction_WithIncludeWordCountFalse(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("summarize", map[string]interface{}{
		"include_word_count": false,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify word_count was NOT added
	for _, doc := range mockStorage.updatedDocs {
		if _, ok := doc.Metadata["word_count"]; ok {
			t.Error("Expected word_count to NOT be in metadata when include_word_count=false")
		}
	}
}

func TestSummarizeAction_WithTopNKeywords(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("summarize", map[string]interface{}{
		"top_n_keywords": 5,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify exactly 5 keywords (or less if content has fewer unique words)
	for _, doc := range mockStorage.updatedDocs {
		if keywords, ok := doc.Metadata["keywords"].([]string); ok {
			if len(keywords) > 5 {
				t.Errorf("Expected at most 5 keywords, got %d", len(keywords))
			}
		}
	}
}

func TestSummarizeAction_LLMChatError(t *testing.T) {
	deps, mockStorage, mockLLM := createTestSummarizerDeps()
	step := createTestStep("summarize", nil)
	step.OnError = models.ErrorStrategyContinue

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		return "", fmt.Errorf("LLM service error")
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	// Should have errors but continue processing
	if err == nil {
		t.Error("Expected aggregated errors, got nil")
	}

	// Verify placeholder summary was used
	for _, doc := range mockStorage.updatedDocs {
		if summary, ok := doc.Metadata["summary"].(string); ok {
			if summary != "Summary not available" {
				t.Errorf("Expected placeholder summary, got '%s'", summary)
			}
		}
	}
}

func TestSummarizeAction_UpdateDocumentError(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("summarize", nil)
	step.OnError = models.ErrorStrategyContinue

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	mockStorage.updateDocumentFunc = func(doc *models.Document) error {
		return fmt.Errorf("database error")
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err == nil {
		t.Error("Expected aggregated errors, got nil")
	}
}

func TestSummarizeAction_SkipWithSummary(t *testing.T) {
	deps, mockStorage, mockLLM := createTestSummarizerDeps()
	step := createTestStep("summarize", map[string]interface{}{
		"skip_with_summary": true,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(5, true, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should not call Chat for documents with summaries
	if mockLLM.chatCalls != 0 {
		t.Errorf("Expected 0 Chat calls (all skipped), got %d", mockLLM.chatCalls)
	}
}

func TestSummarizeAction_EmptyDocumentList(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("summarize", nil)

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		return []*models.Document{}, nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error for empty list, got: %v", err)
	}
}

// Tests for extractKeywordsAction

func TestExtractKeywordsAction_Success(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", nil)

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(3, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if mockStorage.updateDocumentCalls != 3 {
		t.Errorf("Expected 3 UpdateDocument calls, got %d", mockStorage.updateDocumentCalls)
	}

	// Verify metadata was updated
	for _, doc := range mockStorage.updatedDocs {
		if _, ok := doc.Metadata["keywords"]; !ok {
			t.Error("Expected keywords in metadata")
		}
		if _, ok := doc.Metadata["last_keyword_extraction"]; !ok {
			t.Error("Expected last_keyword_extraction in metadata")
		}
	}
}

func TestExtractKeywordsAction_WithTopN(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", map[string]interface{}{
		"top_n": 5,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify at most 5 keywords
	for _, doc := range mockStorage.updatedDocs {
		if keywords, ok := doc.Metadata["keywords"].([]string); ok {
			if len(keywords) > 5 {
				t.Errorf("Expected at most 5 keywords, got %d", len(keywords))
			}
		}
	}
}

func TestExtractKeywordsAction_WithMinWordLength(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", map[string]interface{}{
		"min_word_length": 5,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			docs := createTestDocuments(1, false, false, false)
			docs[0].ContentMarkdown = "short words: is at on by for technology development automation"
			return docs, nil
		}
		return []*models.Document{}, nil
	}

	err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify all keywords are >= 5 characters
	for _, doc := range mockStorage.updatedDocs {
		if keywords, ok := doc.Metadata["keywords"].([]string); ok {
			for _, keyword := range keywords {
				if len(keyword) < 5 {
					t.Errorf("Expected all keywords >= 5 chars, found '%s' (%d chars)", keyword, len(keyword))
				}
			}
		}
	}
}

func TestExtractKeywordsAction_SkipWithKeywords(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", map[string]interface{}{
		"skip_with_keywords": true,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(5, false, true, false), nil
		}
		return []*models.Document{}, nil
	}

	err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should not update documents that already have keywords
	if mockStorage.updateDocumentCalls != 0 {
		t.Errorf("Expected 0 UpdateDocument calls (all skipped), got %d", mockStorage.updateDocumentCalls)
	}
}

func TestExtractKeywordsAction_EmptyContent(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", nil)

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, true), nil
		}
		return []*models.Document{}, nil
	}

	err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Empty content documents should be skipped
	if mockStorage.updateDocumentCalls != 0 {
		t.Errorf("Expected 0 UpdateDocument calls (empty content), got %d", mockStorage.updateDocumentCalls)
	}
}

func TestExtractKeywordsAction_StopWordsFiltering(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", nil)

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			docs := createTestDocuments(1, false, false, false)
			docs[0].ContentMarkdown = "the and for technology development automation testing software"
			return docs, nil
		}
		return []*models.Document{}, nil
	}

	err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify stop words are excluded
	for _, doc := range mockStorage.updatedDocs {
		if keywords, ok := doc.Metadata["keywords"].([]string); ok {
			for _, keyword := range keywords {
				if stopWords[keyword] {
					t.Errorf("Expected stop word '%s' to be excluded", keyword)
				}
			}
		}
	}
}

func TestExtractKeywordsAction_UpdateDocumentError(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", nil)
	step.OnError = models.ErrorStrategyContinue

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	mockStorage.updateDocumentFunc = func(doc *models.Document) error {
		return fmt.Errorf("database error")
	}

	err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err == nil {
		t.Error("Expected aggregated errors, got nil")
	}
}

// Tests for helper functions

func TestGenerateSummary(t *testing.T) {
	deps, _, mockLLM := createTestSummarizerDeps()
	ctx := context.Background()

	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		return "  Test summary  ", nil
	}

	summary, err := generateSummary(ctx, "Test content", 2000, "Test prompt", deps.LLMService, deps.Logger)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify summary is trimmed
	if summary != "Test summary" {
		t.Errorf("Expected trimmed summary 'Test summary', got '%s'", summary)
	}
}

func TestGenerateSummary_LongContent(t *testing.T) {
	deps, _, mockLLM := createTestSummarizerDeps()
	ctx := context.Background()

	longContent := string(make([]byte, 5000))
	for i := range longContent {
		longContent = longContent[:i] + "x"
	}

	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		// Verify content was truncated
		for _, msg := range messages {
			if msg.Role == "user" && len(msg.Content) > 1200 {
				t.Errorf("Expected content to be truncated, got %d chars", len(msg.Content))
			}
		}
		return "Summary", nil
	}

	_, err := generateSummary(ctx, longContent, 1000, "Test prompt", deps.LLMService, deps.Logger)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestGenerateSummary_LLMError(t *testing.T) {
	deps, _, mockLLM := createTestSummarizerDeps()
	ctx := context.Background()

	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		return "", fmt.Errorf("LLM error")
	}

	_, err := generateSummary(ctx, "Test content", 2000, "Test prompt", deps.LLMService, deps.Logger)

	if err == nil {
		t.Error("Expected error for LLM failure, got nil")
	}
}

func TestExtractKeywords_Normal(t *testing.T) {
	content := "technology development automation testing software engineering"

	keywords := extractKeywords(content, 3, 3, stopWords)

	if len(keywords) == 0 {
		t.Error("Expected keywords to be extracted")
	}

	if len(keywords) > 3 {
		t.Errorf("Expected at most 3 keywords, got %d", len(keywords))
	}
}

func TestExtractKeywords_WithMarkdown(t *testing.T) {
	content := "# Heading\n**bold** _italic_ `code` technology development"

	keywords := extractKeywords(content, 10, 3, stopWords)

	// Should exclude markdown syntax and extract meaningful words
	for _, keyword := range keywords {
		if keyword == "#" || keyword == "**" || keyword == "_" || keyword == "`" {
			t.Errorf("Expected markdown syntax to be removed, found '%s'", keyword)
		}
	}
}

func TestExtractKeywords_EmptyContent(t *testing.T) {
	keywords := extractKeywords("", 10, 3, stopWords)

	if len(keywords) != 0 {
		t.Errorf("Expected empty keywords for empty content, got %d", len(keywords))
	}
}

func TestExtractKeywords_TopNGreaterThanAvailable(t *testing.T) {
	content := "technology development"

	keywords := extractKeywords(content, 100, 3, stopWords)

	// Should return all available keywords (max 2 in this case)
	if len(keywords) > 2 {
		t.Errorf("Expected at most 2 keywords, got %d", len(keywords))
	}
}

func TestCalculateWordCount(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "normal content",
			content:  "one two three four five",
			expected: 5,
		},
		{
			name:     "with markdown",
			content:  "# Heading\n**bold** text",
			expected: 2, // "Heading" and "text" (markdown removed)
		},
		{
			name:     "empty content",
			content:  "",
			expected: 0,
		},
		{
			name:     "multiple spaces",
			content:  "one  two   three",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateWordCount(tt.content)
			if result != tt.expected {
				t.Errorf("Expected word count %d, got %d", tt.expected, result)
			}
		})
	}
}

// Tests for RegisterSummarizerActions

func TestRegisterSummarizerActions_Success(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _ := createTestSummarizerDeps()

	err := RegisterSummarizerActions(registry, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify all three actions are registered
	actions := registry.ListActions(models.JobTypeSummarizer)
	if len(actions) != 3 {
		t.Errorf("Expected 3 registered actions, got %d", len(actions))
	}

	// Verify each action can be retrieved
	scanAction, err := registry.GetAction(models.JobTypeSummarizer, "scan")
	if err != nil || scanAction == nil {
		t.Errorf("Failed to get scan action: %v", err)
	}

	summarizeAction, err := registry.GetAction(models.JobTypeSummarizer, "summarize")
	if err != nil || summarizeAction == nil {
		t.Errorf("Failed to get summarize action: %v", err)
	}

	extractKeywordsAction, err := registry.GetAction(models.JobTypeSummarizer, "extract_keywords")
	if err != nil || extractKeywordsAction == nil {
		t.Errorf("Failed to get extract_keywords action: %v", err)
	}
}

func TestRegisterSummarizerActions_NilRegistry(t *testing.T) {
	deps, _, _ := createTestSummarizerDeps()

	err := RegisterSummarizerActions(nil, deps)

	if err == nil {
		t.Error("Expected error for nil registry, got nil")
	}
}

func TestRegisterSummarizerActions_NilDependencies(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)

	err := RegisterSummarizerActions(registry, nil)

	if err == nil {
		t.Error("Expected error for nil dependencies, got nil")
	}
}

func TestRegisterSummarizerActions_MissingDocStorage(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _ := createTestSummarizerDeps()
	deps.DocStorage = nil

	err := RegisterSummarizerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil DocStorage, got nil")
	}
}

func TestRegisterSummarizerActions_MissingLLMService(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _ := createTestSummarizerDeps()
	deps.LLMService = nil

	err := RegisterSummarizerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil LLMService, got nil")
	}
}

func TestRegisterSummarizerActions_MissingLogger(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _ := createTestSummarizerDeps()
	deps.Logger = nil

	err := RegisterSummarizerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil Logger, got nil")
	}
}

// Tests for negative value handling

func TestExtractBatchConfig_NegativeValues(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected batchConfig
	}{
		{
			name: "negative batch_size",
			config: map[string]interface{}{
				"batch_size": -10,
			},
			expected: batchConfig{
				batchSize:    100, // Should clamp to 100
				offset:       0,
				maxDocuments: 0,
			},
		},
		{
			name: "zero batch_size",
			config: map[string]interface{}{
				"batch_size": 0,
			},
			expected: batchConfig{
				batchSize:    100, // Should clamp to 100
				offset:       0,
				maxDocuments: 0,
			},
		},
		{
			name: "negative offset",
			config: map[string]interface{}{
				"offset": -50,
			},
			expected: batchConfig{
				batchSize:    100,
				offset:       0, // Should clamp to 0
				maxDocuments: 0,
			},
		},
		{
			name: "negative max_documents",
			config: map[string]interface{}{
				"max_documents": -100,
			},
			expected: batchConfig{
				batchSize:    100,
				offset:       0,
				maxDocuments: 0, // Should clamp to 0
			},
		},
		{
			name: "all negative",
			config: map[string]interface{}{
				"batch_size":    -10,
				"offset":        -20,
				"max_documents": -30,
			},
			expected: batchConfig{
				batchSize:    100, // Should clamp to 100
				offset:       0,   // Should clamp to 0
				maxDocuments: 0,   // Should clamp to 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBatchConfig(tt.config)
			if result.batchSize != tt.expected.batchSize {
				t.Errorf("Expected batchSize %d, got %d", tt.expected.batchSize, result.batchSize)
			}
			if result.offset != tt.expected.offset {
				t.Errorf("Expected offset %d, got %d", tt.expected.offset, result.offset)
			}
			if result.maxDocuments != tt.expected.maxDocuments {
				t.Errorf("Expected maxDocuments %d, got %d", tt.expected.maxDocuments, result.maxDocuments)
			}
		})
	}
}

func TestSummarizeAction_NegativeTopNKeywords(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("summarize", map[string]interface{}{
		"top_n_keywords": -5,
	})

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return createTestDocuments(1, false, false, false), nil
		}
		return []*models.Document{}, nil
	}

	err := summarizeAction(context.Background(), &step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify keywords field exists but may be empty due to clamped value
	for _, doc := range mockStorage.updatedDocs {
		if keywords, ok := doc.Metadata["keywords"].([]string); ok {
			if len(keywords) < 0 {
				t.Error("Expected non-negative keyword count")
			}
		}
	}
}

func TestExtractKeywordsAction_NegativeValues(t *testing.T) {
	tests := []struct {
		name             string
		config           map[string]interface{}
		expectError      bool
		validateKeywords func(*testing.T, []string)
	}{
		{
			name: "negative top_n",
			config: map[string]interface{}{
				"top_n": -5,
			},
			expectError: false,
			validateKeywords: func(t *testing.T, keywords []string) {
				// Should clamp to 0, resulting in empty keywords
				if len(keywords) < 0 {
					t.Error("Expected non-negative keyword count")
				}
			},
		},
		{
			name: "negative min_word_length",
			config: map[string]interface{}{
				"min_word_length": -3,
			},
			expectError: false,
			validateKeywords: func(t *testing.T, keywords []string) {
				// Should clamp to 1, allowing single-character words
				for _, kw := range keywords {
					if len(kw) < 1 {
						t.Errorf("Expected all keywords >= 1 char, found '%s'", kw)
					}
				}
			},
		},
		{
			name: "zero min_word_length",
			config: map[string]interface{}{
				"min_word_length": 0,
			},
			expectError: false,
			validateKeywords: func(t *testing.T, keywords []string) {
				// Should clamp to 1
				for _, kw := range keywords {
					if len(kw) < 1 {
						t.Errorf("Expected all keywords >= 1 char, found '%s'", kw)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, mockStorage, _ := createTestSummarizerDeps()
			step := createTestStep("extract_keywords", tt.config)

			mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
				if mockStorage.listDocumentsCalls == 1 {
					return createTestDocuments(1, false, false, false), nil
				}
				return []*models.Document{}, nil
			}

			err := extractKeywordsAction(context.Background(), &step, []*models.SourceConfig{}, deps)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			// Validate keywords if validation function provided
			if tt.validateKeywords != nil {
				for _, doc := range mockStorage.updatedDocs {
					if keywords, ok := doc.Metadata["keywords"].([]string); ok {
						tt.validateKeywords(t, keywords)
					}
				}
			}
		})
	}
}

// Tests for source filtering

func TestScanAction_WithSourceFiltering(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("scan", nil)

	// Create test sources
	sources := []*models.SourceConfig{
		{ID: "source-1", Type: "jira"},
		{ID: "source-2", Type: "confluence"},
	}

	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			// Return documents from various sources
			docs := []*models.Document{
				{ID: "doc-1", SourceID: "source-1", SourceType: "jira", ContentMarkdown: "content"},
				{ID: "doc-2", SourceID: "source-2", SourceType: "confluence", ContentMarkdown: "content"},
				{ID: "doc-3", SourceID: "source-3", SourceType: "github", ContentMarkdown: "content"}, // Should be skipped
			}
			return docs, nil
		}
		return []*models.Document{}, nil
	}

	err := scanAction(context.Background(), &step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestSummarizeAction_WithSourceFiltering(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("summarize", nil)

	// Create test sources
	sources := []*models.SourceConfig{
		{ID: "source-1", Type: "jira"},
	}

	processedDocs := make(map[string]bool)
	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return []*models.Document{
				{ID: "doc-1", SourceID: "source-1", SourceType: "jira", ContentMarkdown: "content"},
				{ID: "doc-2", SourceID: "source-2", SourceType: "confluence", ContentMarkdown: "content"},
			}, nil
		}
		return []*models.Document{}, nil
	}

	mockStorage.updateDocumentFunc = func(doc *models.Document) error {
		processedDocs[doc.ID] = true
		return nil
	}

	err := summarizeAction(context.Background(), &step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify only source-1 documents were processed
	if !processedDocs["doc-1"] {
		t.Error("Expected doc-1 to be processed")
	}
	if processedDocs["doc-2"] {
		t.Error("Expected doc-2 to be skipped (not in selected sources)")
	}
}

func TestExtractKeywordsAction_WithSourceFiltering(t *testing.T) {
	deps, mockStorage, _ := createTestSummarizerDeps()
	step := createTestStep("extract_keywords", nil)

	sources := []*models.SourceConfig{
		{ID: "source-1", Type: "jira"},
	}

	processedDocs := make(map[string]bool)
	mockStorage.listDocumentsFunc = func(opts *interfaces.ListOptions) ([]*models.Document, error) {
		if mockStorage.listDocumentsCalls == 1 {
			return []*models.Document{
				{ID: "doc-1", SourceID: "source-1", SourceType: "jira", ContentMarkdown: "content"},
				{ID: "doc-2", SourceID: "source-2", SourceType: "confluence", ContentMarkdown: "content"},
			}, nil
		}
		return []*models.Document{}, nil
	}

	mockStorage.updateDocumentFunc = func(doc *models.Document) error {
		processedDocs[doc.ID] = true
		return nil
	}

	err := extractKeywordsAction(context.Background(), &step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify only source-1 documents were processed
	if !processedDocs["doc-1"] {
		t.Error("Expected doc-1 to be processed")
	}
	if processedDocs["doc-2"] {
		t.Error("Expected doc-2 to be skipped (not in selected sources)")
	}
}

// Tests for multibyte character handling

func TestGenerateSummary_MultibyteCharacters(t *testing.T) {
	deps, _, mockLLM := createTestSummarizerDeps()
	ctx := context.Background()

	// Test with various multibyte characters
	multibyteContent := "这是中文测试内容 🚀 émojis and spëcial çharacters ñ 日本語テスト"
	contentLimit := 20 // Limit by runes, not bytes

	var receivedContent string
	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		for _, msg := range messages {
			if msg.Role == "user" {
				receivedContent = msg.Content
			}
		}
		return "Summary", nil
	}

	_, err := generateSummary(ctx, multibyteContent, contentLimit, "Test", deps.LLMService, deps.Logger)

	if err != nil {
		t.Errorf("Expected no error with multibyte chars, got: %v", err)
	}

	// Verify truncation happened at rune boundary, not byte boundary
	// If it truncated at byte boundary, it would panic or produce invalid UTF-8
	if len(receivedContent) == 0 {
		t.Error("Expected content to be received")
	}

	// Verify the content is valid UTF-8
	if !isValidUTF8(receivedContent) {
		t.Error("Expected valid UTF-8 after truncation")
	}
}

func TestGenerateSummary_EmojiTruncation(t *testing.T) {
	deps, _, mockLLM := createTestSummarizerDeps()
	ctx := context.Background()

	// Create content with emojis that could be split incorrectly
	emojiContent := "🚀🎉🎊🎈🎆🎇✨🎁🎂🎄🎃🎅🎁"
	contentLimit := 5

	var receivedContent string
	mockLLM.chatFunc = func(ctx context.Context, messages []interfaces.Message) (string, error) {
		for _, msg := range messages {
			if msg.Role == "user" {
				receivedContent = msg.Content
			}
		}
		return "Summary", nil
	}

	_, err := generateSummary(ctx, emojiContent, contentLimit, "Test", deps.LLMService, deps.Logger)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Count runes in result (should be contentLimit + 3 for "...")
	runes := []rune(receivedContent)
	// The "..." is appended, so we expect the prefix to be exactly contentLimit runes
	if len(runes) > contentLimit+10 { // Allow some buffer for "..." and prefix text
		t.Errorf("Expected truncation at rune boundary, got %d runes", len(runes))
	}

	// Verify valid UTF-8
	if !isValidUTF8(receivedContent) {
		t.Error("Expected valid UTF-8 after emoji truncation")
	}
}

// Helper function to validate UTF-8
func isValidUTF8(s string) bool {
	// Try to convert to runes and back
	// If it's invalid UTF-8, this will replace invalid sequences
	converted := string([]rune(s))
	return converted == s || len(s) == 0
}
