package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// ChatRequest represents a chat request
type ChatRequest struct {
	// User's message
	Message string `json:"message"`

	// Conversation history (optional)
	History []Message `json:"history,omitempty"`

	// System prompt (optional, defaults will be used if not provided)
	SystemPrompt string `json:"system_prompt,omitempty"`
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

// ChatService provides agent-based chat functionality
type ChatService interface {
	// Chat sends a message and receives a response using the agent
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// GetMode returns the current LLM mode (online/offline)
	GetMode() LLMMode

	// HealthCheck verifies the chat service is operational
	HealthCheck(ctx context.Context) error

	// GetServiceStatus returns detailed service status information
	GetServiceStatus(ctx context.Context) map[string]interface{}
}
