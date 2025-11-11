package interfaces

import "context"

// AgentService provides a unified API for executing different types of AI agents.
// Agents are powered by Google ADK with Gemini models and handle document processing
// tasks such as keyword extraction, summarization, classification, etc.
//
// Design Principles:
// - No offline fallback - requires valid Google API key
// - Agent types are registered at service initialization
// - Input/output formats are agent-specific and documented per agent type
// - All agents use the same underlying Gemini model but with different instructions
//
// Example Usage:
//
//	// Execute keyword extraction agent
//	input := map[string]interface{}{
//	    "document_id": "doc_123",
//	    "content": "Document content here...",
//	    "max_keywords": 10,
//	}
//	output, err := agentService.Execute(ctx, "keyword_extractor", input)
//	if err != nil {
//	    // Handle error
//	}
//	keywords := output["keywords"].([]string)
//	confidence := output["confidence"].(map[string]float64)
type AgentService interface {
	// Execute runs an agent of the specified type with the given input.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - agentType: Agent identifier (e.g., "keyword_extractor", "summarizer")
	//   - input: Agent-specific input data (structure varies by agent type)
	//
	// Returns:
	//   - output: Agent output as a map (structure varies by agent type)
	//   - error: nil on success, error with details on failure
	//
	// Common input fields (agent-specific):
	//   - document_id (string): Document ID being processed
	//   - content (string): Document content to analyze
	//   - Additional agent-specific parameters
	//
	// Common output fields (agent-specific):
	//   - Agent-specific results (e.g., keywords, summary, classification)
	//   - confidence scores (if applicable)
	//
	// Errors:
	//   - Unknown agent type
	//   - Invalid input structure
	//   - Google API communication failure
	//   - Malformed agent response
	//   - Timeout exceeded
	Execute(ctx context.Context, agentType string, input map[string]interface{}) (map[string]interface{}, error)

	// HealthCheck verifies the agent service is operational and can communicate
	// with Google ADK. This should be called during service initialization to
	// fail fast if the API key is invalid or the service is unavailable.
	//
	// Returns:
	//   - nil if healthy (can create agents and communicate with Google)
	//   - error with details if unhealthy (invalid API key, network issues, etc.)
	HealthCheck(ctx context.Context) error

	// Close releases resources and performs cleanup. Should be called during
	// application shutdown to gracefully terminate agent operations.
	//
	// Returns:
	//   - nil on successful cleanup
	//   - error if cleanup fails (resources may not be fully released)
	Close() error
}
