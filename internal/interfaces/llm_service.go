package interfaces

import (
	"context"
)

// LLMMode represents the operational mode of the LLM service
type LLMMode string

const (
	// LLMModeCloud indicates the service uses cloud-based LLM APIs
	LLMModeCloud LLMMode = "cloud"

	// LLMModeOffline indicates the service uses local/offline LLM models
	LLMModeOffline LLMMode = "offline"
)

// Message represents a single message in a chat conversation
type Message struct {
	// Role identifies the message sender: "user", "assistant", or "system"
	Role string

	// Content contains the text content of the message
	Content string
}

// LLMService defines the interface for language model operations including
// embeddings generation and chat completions. Implementations may use either
// cloud-based APIs (OpenAI, Anthropic) or offline models (Ollama, local).
type LLMService interface {
	// Embed generates a 768-dimension embedding vector for the given text.
	// The embedding can be used for semantic search, similarity comparison,
	// and vector storage operations.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - text: Input text to generate embedding for
	//
	// Returns:
	//   - []float32: 768-dimension embedding vector
	//   - error: Error if embedding generation fails
	Embed(ctx context.Context, text string) ([]float32, error)

	// Chat generates a completion response based on the conversation history.
	// The messages slice should contain the full conversation context including
	// system prompts, user messages, and previous assistant responses.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - messages: Conversation history in chronological order
	//
	// Returns:
	//   - string: Generated assistant response
	//   - error: Error if chat completion fails
	Chat(ctx context.Context, messages []Message) (string, error)

	// HealthCheck verifies the LLM service is operational and can handle requests.
	// For cloud services, this may check API connectivity and authentication.
	// For offline services, this may verify model availability and loading.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//
	// Returns:
	//   - error: Error if service is not healthy or unreachable
	HealthCheck(ctx context.Context) error

	// GetMode returns the current operational mode of the LLM service.
	// This indicates whether the service is using cloud APIs or offline models.
	//
	// Returns:
	//   - LLMMode: Current mode (LLMModeCloud or LLMModeOffline)
	GetMode() LLMMode

	// Close releases resources and performs cleanup operations.
	// For cloud services, this may close HTTP connections.
	// For offline services, this may unload models and free memory.
	//
	// Returns:
	//   - error: Error if cleanup fails
	Close() error
}
