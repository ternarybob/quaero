package unit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/chat"
)

// MockDocumentService is a mock implementation of DocumentService for testing
type MockDocumentService struct {
	searchFunc func(ctx context.Context, query *interfaces.SearchQuery) ([]*models.Document, error)
}

func (m *MockDocumentService) SaveDocument(ctx context.Context, doc *models.Document) error {
	return nil
}

func (m *MockDocumentService) SaveDocuments(ctx context.Context, docs []*models.Document) error {
	return nil
}

func (m *MockDocumentService) UpdateDocument(ctx context.Context, doc *models.Document) error {
	return nil
}

func (m *MockDocumentService) GetDocument(ctx context.Context, id string) (*models.Document, error) {
	return nil, nil
}

func (m *MockDocumentService) GetBySource(ctx context.Context, sourceType, sourceID string) (*models.Document, error) {
	return nil, nil
}

func (m *MockDocumentService) DeleteDocument(ctx context.Context, id string) error {
	return nil
}

func (m *MockDocumentService) Search(ctx context.Context, query *interfaces.SearchQuery) ([]*models.Document, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query)
	}
	return []*models.Document{}, nil
}

func (m *MockDocumentService) GetStats(ctx context.Context) (*models.DocumentStats, error) {
	return nil, nil
}

func (m *MockDocumentService) Count(ctx context.Context, sourceType string) (int, error) {
	return 0, nil
}

func (m *MockDocumentService) List(ctx context.Context, opts *interfaces.ListOptions) ([]*models.Document, error) {
	return []*models.Document{}, nil
}

// MockEmbeddingService is a mock implementation of EmbeddingService for testing
type MockEmbeddingService struct {
	generateQueryEmbeddingFunc func(ctx context.Context, query string) ([]float32, error)
}

func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockEmbeddingService) EmbedDocument(ctx context.Context, doc *models.Document) error {
	return nil
}

func (m *MockEmbeddingService) EmbedDocuments(ctx context.Context, docs []*models.Document) error {
	return nil
}

func (m *MockEmbeddingService) GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	if m.generateQueryEmbeddingFunc != nil {
		return m.generateQueryEmbeddingFunc(ctx, query)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockEmbeddingService) ModelName() string {
	return "mock-model"
}

func (m *MockEmbeddingService) Dimension() int {
	return 768
}

func (m *MockEmbeddingService) IsAvailable(ctx context.Context) bool {
	return true
}

// TestChatService_ChatWithoutRAG tests chat without RAG enabled
func TestChatService_ChatWithoutRAG(t *testing.T) {
	t.Log("=== Testing Chat Service - Without RAG ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	mockDoc := &MockDocumentService{}
	mockEmbed := &MockEmbeddingService{}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test
	ctx := context.Background()
	req := &interfaces.ChatRequest{
		Message: "Hello, how are you?",
		RAGConfig: &interfaces.RAGConfig{
			Enabled: false,
		},
	}

	response, err := service.Chat(ctx, req)

	// Verify
	require.NoError(t, err, "Chat should succeed without error")
	require.NotNil(t, response, "Response should not be nil")
	assert.Equal(t, "mock response", response.Message, "Response message should match")
	assert.Equal(t, interfaces.LLMModeOffline, response.Mode, "Mode should be offline")
	assert.Nil(t, response.ContextDocs, "Context docs should be nil when RAG disabled")

	t.Log("✅ SUCCESS: Chat without RAG works correctly")
}

// TestChatService_ChatWithRAG tests chat with RAG enabled
func TestChatService_ChatWithRAG(t *testing.T) {
	t.Log("=== Testing Chat Service - With RAG ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}

	// Mock document service that returns test documents
	mockDoc := &MockDocumentService{
		searchFunc: func(ctx context.Context, query *interfaces.SearchQuery) ([]*models.Document, error) {
			return []*models.Document{
				{
					ID:         "doc1",
					Title:      "Test Document",
					Content:    "This is test content about the system",
					SourceType: "jira",
				},
			}, nil
		},
	}

	mockEmbed := &MockEmbeddingService{
		generateQueryEmbeddingFunc: func(ctx context.Context, query string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3}, nil
		},
	}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test
	ctx := context.Background()
	req := &interfaces.ChatRequest{
		Message: "What is the system?",
		RAGConfig: &interfaces.RAGConfig{
			Enabled:       true,
			MaxDocuments:  5,
			MinSimilarity: 0.7,
			SearchMode:    interfaces.SearchModeVector,
		},
	}

	response, err := service.Chat(ctx, req)

	// Verify
	require.NoError(t, err, "Chat should succeed without error")
	require.NotNil(t, response, "Response should not be nil")
	assert.Equal(t, "mock response", response.Message, "Response message should match")
	assert.Equal(t, interfaces.LLMModeOffline, response.Mode, "Mode should be offline")
	assert.NotNil(t, response.ContextDocs, "Context docs should not be nil when RAG enabled")
	assert.Equal(t, 1, len(response.ContextDocs), "Should have 1 context document")
	assert.Equal(t, "Test Document", response.ContextDocs[0].Title, "Document title should match")

	t.Log("✅ SUCCESS: Chat with RAG works correctly")
}

// TestChatService_ChatWithHistory tests chat with conversation history
func TestChatService_ChatWithHistory(t *testing.T) {
	t.Log("=== Testing Chat Service - With History ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	mockDoc := &MockDocumentService{}
	mockEmbed := &MockEmbeddingService{}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test
	ctx := context.Background()
	req := &interfaces.ChatRequest{
		Message: "What about now?",
		History: []interfaces.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi, how can I help?"},
			{Role: "user", Content: "Tell me about the weather"},
			{Role: "assistant", Content: "It's sunny today"},
		},
		RAGConfig: &interfaces.RAGConfig{
			Enabled: false,
		},
	}

	response, err := service.Chat(ctx, req)

	// Verify
	require.NoError(t, err, "Chat should succeed without error")
	require.NotNil(t, response, "Response should not be nil")
	assert.Equal(t, "mock response", response.Message, "Response message should match")

	t.Log("✅ SUCCESS: Chat with history works correctly")
}

// TestChatService_ChatWithCustomSystemPrompt tests chat with custom system prompt
func TestChatService_ChatWithCustomSystemPrompt(t *testing.T) {
	t.Log("=== Testing Chat Service - Custom System Prompt ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	mockDoc := &MockDocumentService{}
	mockEmbed := &MockEmbeddingService{}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test
	ctx := context.Background()
	req := &interfaces.ChatRequest{
		Message:      "Hello",
		SystemPrompt: "You are a helpful coding assistant specialized in Go programming.",
		RAGConfig: &interfaces.RAGConfig{
			Enabled: false,
		},
	}

	response, err := service.Chat(ctx, req)

	// Verify
	require.NoError(t, err, "Chat should succeed without error")
	require.NotNil(t, response, "Response should not be nil")
	assert.Equal(t, "mock response", response.Message, "Response message should match")

	t.Log("✅ SUCCESS: Chat with custom system prompt works correctly")
}

// TestChatService_GetMode tests getting the LLM mode
func TestChatService_GetMode(t *testing.T) {
	t.Log("=== Testing Chat Service - Get Mode ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	mockDoc := &MockDocumentService{}
	mockEmbed := &MockEmbeddingService{}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test
	mode := service.GetMode()

	// Verify
	assert.Equal(t, interfaces.LLMModeOffline, mode, "Mode should be offline")

	t.Log("✅ SUCCESS: GetMode returns correct mode")
}

// TestChatService_HealthCheck tests health check functionality
func TestChatService_HealthCheck(t *testing.T) {
	t.Log("=== Testing Chat Service - Health Check ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	mockDoc := &MockDocumentService{}
	mockEmbed := &MockEmbeddingService{}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test
	ctx := context.Background()
	err := service.HealthCheck(ctx)

	// Verify
	require.NoError(t, err, "Health check should pass")

	t.Log("✅ SUCCESS: Health check passed")
}

// TestChatService_HealthCheckFail tests health check failure
func TestChatService_HealthCheckFail(t *testing.T) {
	t.Log("=== Testing Chat Service - Health Check Failure ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: false,
	}
	mockDoc := &MockDocumentService{}
	mockEmbed := &MockEmbeddingService{}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test
	ctx := context.Background()
	err := service.HealthCheck(ctx)

	// Verify
	require.Error(t, err, "Health check should fail")
	assert.Contains(t, err.Error(), "LLM service unhealthy", "Error should mention LLM service")

	t.Log("✅ SUCCESS: Health check failure detected correctly")
}

// TestChatService_DefaultRAGConfig tests default RAG configuration
func TestChatService_DefaultRAGConfig(t *testing.T) {
	t.Log("=== Testing Chat Service - Default RAG Config ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	mockDoc := &MockDocumentService{}
	mockEmbed := &MockEmbeddingService{}

	service := chat.NewChatService(mockLLM, mockDoc, mockEmbed, logger)

	// Test - request without RAG config should use defaults
	ctx := context.Background()
	req := &interfaces.ChatRequest{
		Message: "Hello",
		// No RAGConfig specified
	}

	response, err := service.Chat(ctx, req)

	// Verify - should use default RAG config (enabled=true)
	require.NoError(t, err, "Chat should succeed without error")
	require.NotNil(t, response, "Response should not be nil")

	t.Log("✅ SUCCESS: Default RAG config applied correctly")
}
