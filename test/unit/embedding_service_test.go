package unit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/embeddings"
)

// MockLLMService is a mock implementation of LLMService for testing
type MockLLMService struct {
	embedFunc     func(ctx context.Context, text string) ([]float32, error)
	mode          interfaces.LLMMode
	healthCheckOK bool
}

func (m *MockLLMService) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, text)
	}
	// Default: return mock embedding
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockLLMService) Chat(ctx context.Context, messages []interfaces.Message) (string, error) {
	return "mock response", nil
}

func (m *MockLLMService) GetMode() interfaces.LLMMode {
	if m.mode == "" {
		return interfaces.LLMModeOffline
	}
	return m.mode
}

func (m *MockLLMService) HealthCheck(ctx context.Context) error {
	if !m.healthCheckOK {
		return assert.AnError
	}
	return nil
}

func (m *MockLLMService) Close() error {
	return nil
}

// TestGenerateEmbedding_Success tests successful embedding generation
func TestGenerateEmbedding_Success(t *testing.T) {
	t.Log("=== Testing Embedding Generation - Success ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		embedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4, 0.5}, nil
		},
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}

	service := embeddings.NewService(mockLLM, nil, 5, logger)

	// Test
	ctx := context.Background()
	embedding, err := service.GenerateEmbedding(ctx, "test text")

	// Verify
	require.NoError(t, err, "Should generate embedding without error")
	require.NotNil(t, embedding, "Embedding should not be nil")
	assert.Equal(t, 5, len(embedding), "Embedding should have expected dimension")
	assert.Equal(t, []float32{0.1, 0.2, 0.3, 0.4, 0.5}, embedding, "Embedding values should match")

	t.Log("✅ SUCCESS: Embedding generated correctly")
}

// TestGenerateEmbedding_EmptyText tests error handling for empty text
func TestGenerateEmbedding_EmptyText(t *testing.T) {
	t.Log("=== Testing Embedding Generation - Empty Text ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{mode: interfaces.LLMModeOffline, healthCheckOK: true}
	service := embeddings.NewService(mockLLM, nil, 5, logger)

	// Test
	ctx := context.Background()
	embedding, err := service.GenerateEmbedding(ctx, "")

	// Verify
	require.Error(t, err, "Should return error for empty text")
	assert.Nil(t, embedding, "Embedding should be nil")
	assert.Contains(t, err.Error(), "text cannot be empty", "Error message should indicate empty text")

	t.Log("✅ SUCCESS: Empty text error handled correctly")
}

// TestEmbedDocument_Success tests successful document embedding
func TestEmbedDocument_Success(t *testing.T) {
	t.Log("=== Testing Document Embedding - Success ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		embedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3}, nil
		},
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	service := embeddings.NewService(mockLLM, nil, 3, logger)

	doc := &models.Document{
		ID:      "doc1",
		Title:   "Test Document",
		Content: "This is test content",
	}

	// Test
	ctx := context.Background()
	err := service.EmbedDocument(ctx, doc)

	// Verify
	require.NoError(t, err, "Should embed document without error")
	require.NotNil(t, doc.Embedding, "Document should have embedding")
	assert.Equal(t, 3, len(doc.Embedding), "Embedding should have expected dimension")
	assert.Equal(t, "offline", doc.EmbeddingModel, "Embedding model should be set")

	t.Log("✅ SUCCESS: Document embedded correctly")
}

// TestEmbedDocuments_Success tests batch document embedding
func TestEmbedDocuments_Success(t *testing.T) {
	t.Log("=== Testing Batch Document Embedding - Success ===")

	// Setup
	logger := arbor.NewLogger()
	callCount := 0
	mockLLM := &MockLLMService{
		embedFunc: func(ctx context.Context, text string) ([]float32, error) {
			callCount++
			return []float32{0.1, 0.2, 0.3}, nil
		},
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	service := embeddings.NewService(mockLLM, nil, 3, logger)

	docs := []*models.Document{
		{ID: "doc1", Title: "Doc 1", Content: "Content 1"},
		{ID: "doc2", Title: "Doc 2", Content: "Content 2"},
		{ID: "doc3", Title: "Doc 3", Content: "Content 3"},
	}

	// Test
	ctx := context.Background()
	err := service.EmbedDocuments(ctx, docs)

	// Verify
	require.NoError(t, err, "Should embed all documents without error")
	assert.Equal(t, 3, callCount, "Should call LLM embed 3 times")

	for i, doc := range docs {
		assert.NotNil(t, doc.Embedding, "Document %d should have embedding", i)
		assert.Equal(t, 3, len(doc.Embedding), "Document %d should have expected dimension", i)
	}

	t.Log("✅ SUCCESS: Batch documents embedded correctly")
}

// TestGenerateQueryEmbedding_Success tests query embedding generation
func TestGenerateQueryEmbedding_Success(t *testing.T) {
	t.Log("=== Testing Query Embedding Generation - Success ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		embedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.5, 0.6, 0.7}, nil
		},
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	service := embeddings.NewService(mockLLM, nil, 3, logger)

	// Test
	ctx := context.Background()
	embedding, err := service.GenerateQueryEmbedding(ctx, "search query")

	// Verify
	require.NoError(t, err, "Should generate query embedding without error")
	require.NotNil(t, embedding, "Query embedding should not be nil")
	assert.Equal(t, 3, len(embedding), "Query embedding should have expected dimension")

	t.Log("✅ SUCCESS: Query embedding generated correctly")
}

// TestModelName tests ModelName method
func TestModelName(t *testing.T) {
	t.Log("=== Testing ModelName ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{mode: interfaces.LLMModeOffline}
	service := embeddings.NewService(mockLLM, nil, 3, logger)

	// Test
	name := service.ModelName()

	// Verify
	assert.Equal(t, "offline", name, "Model name should match LLM mode")

	t.Log("✅ SUCCESS: Model name correct")
}

// TestDimension tests Dimension method
func TestDimension(t *testing.T) {
	t.Log("=== Testing Dimension ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{mode: interfaces.LLMModeOffline}
	service := embeddings.NewService(mockLLM, nil, 768, logger)

	// Test
	dimension := service.Dimension()

	// Verify
	assert.Equal(t, 768, dimension, "Dimension should match configured value")

	t.Log("✅ SUCCESS: Dimension correct")
}

// TestIsAvailable_Success tests service availability check
func TestIsAvailable_Success(t *testing.T) {
	t.Log("=== Testing IsAvailable - Success ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: true,
	}
	service := embeddings.NewService(mockLLM, nil, 3, logger)

	// Test
	ctx := context.Background()
	available := service.IsAvailable(ctx)

	// Verify
	assert.True(t, available, "Service should be available when health check passes")

	t.Log("✅ SUCCESS: Service availability check correct")
}

// TestIsAvailable_Failure tests service availability check failure
func TestIsAvailable_Failure(t *testing.T) {
	t.Log("=== Testing IsAvailable - Failure ===")

	// Setup
	logger := arbor.NewLogger()
	mockLLM := &MockLLMService{
		mode:          interfaces.LLMModeOffline,
		healthCheckOK: false,
	}
	service := embeddings.NewService(mockLLM, nil, 3, logger)

	// Test
	ctx := context.Background()
	available := service.IsAvailable(ctx)

	// Verify
	assert.False(t, available, "Service should not be available when health check fails")

	t.Log("✅ SUCCESS: Service unavailability detected correctly")
}
