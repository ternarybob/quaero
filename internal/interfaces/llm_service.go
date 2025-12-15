package interfaces

import (
	"context"
)

// LLMMode represents the operational mode of the LLM service
type LLMMode string

const (
	// LLMModeCloud indicates the service uses cloud-based LLM APIs (Google ADK)
	LLMModeCloud LLMMode = "cloud"
)

// Message represents a single message in a chat conversation
type Message struct {
	// Role identifies the message sender: "user", "assistant", or "system"
	Role string

	// Content contains the text content of the message
	Content string
}

// LLMService defines the interface for language model operations including
// chat completions. Current implementation uses Google ADK (cloud-based) via Gemini API.
type LLMService interface {
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
	// Current implementation returns LLMModeCloud (Google ADK).
	//
	// Returns:
	//   - LLMMode: Current mode (LLMModeCloud)
	GetMode() LLMMode

	// Close releases resources and performs cleanup operations.
	// For cloud services, this may close HTTP connections.
	// For offline services, this may unload models and free memory.
	//
	// Returns:
	//   - error: Error if cleanup fails
	Close() error
}
