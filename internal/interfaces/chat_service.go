package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// ChatRequest represents a chat request with context retrieval
type ChatRequest struct {
	// User's message
	Message string `json:"message"`

	// Conversation history (optional)
	History []Message `json:"history,omitempty"`

	// System prompt (optional, defaults will be used if not provided)
	SystemPrompt string `json:"system_prompt,omitempty"`

	// RAG Configuration
	RAGConfig *RAGConfig `json:"rag_config,omitempty"`
}

// RAGConfig configures retrieval-augmented generation
type RAGConfig struct {
	// Enable RAG (default: true)
	Enabled bool `json:"enabled"`

	// Maximum number of documents to retrieve (default: 5)
	MaxDocuments int `json:"max_documents"`

	// Minimum similarity score (0.0-1.0, default: 0.7)
	MinSimilarity float32 `json:"min_similarity"`

	// Filter by source types
	SourceTypes []string `json:"source_types,omitempty"`

	// Search mode (vector, keyword, hybrid)
	SearchMode SearchMode `json:"search_mode,omitempty"`
}

// ChatResponse represents the response from a chat request
type ChatResponse struct {
	// Generated response
	Message string `json:"message"`

	// Retrieved context documents used for response
	ContextDocs []*models.Document `json:"context_docs,omitempty"`

	// Token usage information
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`

	// Model used
	Model string `json:"model"`

	// Mode (online/offline)
	Mode LLMMode `json:"mode"`
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatService provides RAG-enabled chat functionality
type ChatService interface {
	// Chat sends a message and receives a response with optional context retrieval
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChat sends a message and streams the response (future enhancement)
	// StreamChat(ctx context.Context, req *ChatRequest) (<-chan string, error)

	// GetMode returns the current LLM mode (online/offline)
	GetMode() LLMMode

	// HealthCheck verifies the chat service is operational
	HealthCheck(ctx context.Context) error
}
