package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"google.golang.org/genai"
)

// GeminiService implements the LLMService interface using Google ADK.
// It provides chat completions using Gemini models.
type GeminiService struct {
	config  *common.GeminiConfig
	logger  arbor.ILogger
	client  *genai.Client
	timeout time.Duration
}

// convertMessagesToGemini converts []interfaces.Message to Gemini Content format.
// Maps Role values to provider's expected values and maintains chronological ordering.
// Extracts system messages separately for use with SystemInstruction.
// Returns the user/model messages, the first system message content (if any), and an error.
func convertMessagesToGemini(messages []interfaces.Message) ([]*genai.Content, string, error) {
	if len(messages) == 0 {
		return nil, "", fmt.Errorf("messages cannot be empty")
	}

	// Check that at least one message has role "user"
	hasUserMessage := false
	for _, msg := range messages {
		if msg.Role == "user" {
			hasUserMessage = true
			break
		}
	}
	if !hasUserMessage {
		return nil, "", fmt.Errorf("at least one message must have role 'user'")
	}

	// Convert messages to Gemini format, excluding system messages
	contents := make([]*genai.Content, 0, len(messages))
	var systemText string
	for _, msg := range messages {
		// Handle system messages separately
		if msg.Role == "system" {
			if systemText == "" {
				systemText = msg.Content
			}
			continue // Don't add system messages to contents
		}

		// Map Role values to Gemini expected values
		var geminiRole string
		switch msg.Role {
		case "assistant":
			geminiRole = genai.RoleModel
		case "user":
			geminiRole = genai.RoleUser
		default:
			geminiRole = genai.RoleUser // Default to user for unknown roles
		}

		// Create content part from text
		part := genai.NewPartFromText(msg.Content)
		content := &genai.Content{
			Role:  geminiRole,
			Parts: []*genai.Part{part},
		}

		contents = append(contents, content)
	}

	return contents, systemText, nil
}

// NewGeminiService creates a new Gemini LLM service instance.
//
// The service initialization includes:
//  1. Resolving Google API key from KV store with config fallback
//  2. Setting default model name if not specified
//  3. Parsing timeout duration from configuration
//  4. Initializing chat model with Google ADK
//
// Parameters:
//   - geminiConfig: Gemini configuration with API key and model settings
//   - storageManager: Storage manager interface for KV and auth storage access
//   - logger: Structured logger for service operations
//
// Returns:
//   - *GeminiService: Initialized service ready for use
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Missing or empty Google API key (from KV store or config)
//   - Invalid model name or timeout duration
//   - Failed to initialize ADK models (network, auth, etc.)
func NewGeminiService(geminiConfig *common.GeminiConfig, storageManager interfaces.StorageManager, logger arbor.ILogger) (*GeminiService, error) {
	// Resolve API key with KV-first resolution order: KV store → config fallback
	ctx := context.Background()
	apiKey, err := common.ResolveAPIKey(ctx, storageManager.KeyValueStorage(), "google_api_key", geminiConfig.GoogleAPIKey)
	if err != nil {
		return nil, fmt.Errorf("Google API key is required for LLM service (set via KV store, QUAERO_GEMINI_GOOGLE_API_KEY, or gemini.google_api_key in config): %w", err)
	}

	// Set default model name if not specified
	if geminiConfig.ChatModel == "" {
		geminiConfig.ChatModel = "gemini-2.0-flash"
	}

	// Parse timeout duration
	timeout, err := time.ParseDuration(geminiConfig.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration '%s': %w", geminiConfig.Timeout, err)
	}

	// Initialize genai client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize genai client: %w", err)
	}

	// Create service instance
	service := &GeminiService{
		config:  geminiConfig,
		logger:  logger,
		client:  client,
		timeout: timeout,
	}

	logger.Info().
		Str("chat_model", geminiConfig.ChatModel).
		Dur("timeout", timeout).
		Float32("temperature", geminiConfig.Temperature).
		Msg("Gemini LLM service initialized successfully")

	return service, nil
}

// Chat generates a completion response based on the conversation history.
//
// This method uses the gemini-2.0-flash model for fast and cost-effective
// chat completions. The messages slice should contain the full conversation
// context in chronological order, including system prompts, user messages,
// and previous assistant responses.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - messages: Conversation history in chronological order
//
// Returns:
//   - string: Generated assistant response
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Context cancellation or timeout
//   - Empty or invalid message history
//   - API communication errors
//   - Invalid response format from Google ADK
func (s *GeminiService) Chat(ctx context.Context, messages []interfaces.Message) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("messages cannot be empty for chat completion")
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	startTime := time.Now()
	s.logger.Debug().
		Int("message_count", len(messages)).
		Msg("Starting chat completion")

	// Generate completion using Google ADK
	response, err := s.generateCompletion(timeoutCtx, messages)
	if err != nil {
		s.logger.Error().
			Err(err).
			Int("message_count", len(messages)).
			Msg("Chat completion failed")
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	duration := time.Since(startTime)
	s.logger.Info().
		Int("message_count", len(messages)).
		Int("response_length", len(response)).
		Dur("duration", duration).
		Msg("Chat completion completed successfully")

	return response, nil
}

// HealthCheck verifies the LLM service is operational and can handle requests.
//
// The health check validates that the genai client is properly initialized
// and accessible. For cloud services, this includes lightweight connectivity
// probes to exercise the chat model with short timeouts.
//
// Parameters:
//   - ctx: Context for cancellation control
//
// Returns:
//   - nil if service is healthy (operational)
//   - error with details if service is unhealthy (API issues, auth problems, etc.)
func (s *GeminiService) HealthCheck(ctx context.Context) error {
	s.logger.Debug().Msg("Running Gemini LLM service health check")

	// Verify client is initialized
	if s.client == nil {
		return fmt.Errorf("genai client is not initialized")
	}

	// Perform lightweight connectivity probe with short timeout
	if err := s.performChatHealthCheck(ctx); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Chat model health check failed")
		return fmt.Errorf("chat model health check failed: %w", err)
	}

	s.logger.Info().
		Str("chat_model", s.config.ChatModel).
		Msg("Gemini LLM service health check passed")

	return nil
}

// performChatHealthCheck exercises the chat model with a minimal probe.
// Uses a longer timeout to avoid false negatives and logs detailed failures.
func (s *GeminiService) performChatHealthCheck(ctx context.Context) error {
	// Create timeout context for health check (increased to 5s to avoid false negatives)
	healthCheckCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Create minimal test message for the probe
	testMessages := []interfaces.Message{
		{
			Role:    "user",
			Content: "ping",
		},
	}

	// Generate response and ensure it's non-empty
	response, err := s.generateCompletion(healthCheckCtx, testMessages)
	if err != nil {
		return fmt.Errorf("chat probe failed: %w", err)
	}

	// Validate that we got a non-empty response
	if len(strings.TrimSpace(response)) == 0 {
		return fmt.Errorf("chat probe returned empty response")
	}

	s.logger.Debug().
		Int("response_length", len(response)).
		Msg("Chat model health check passed")

	return nil
}

// GetMode returns the current operational mode of the LLM service.
//
// Since this implementation uses Google ADK cloud APIs, it returns
// LLMModeCloud to indicate cloud-based service usage.
//
// Returns:
//   - interfaces.LLMModeCloud: Indicating cloud-based service
func (s *GeminiService) GetMode() interfaces.LLMMode {
	return interfaces.LLMModeCloud
}

// Close releases resources and performs cleanup operations.
//
// This method sets the client reference to nil, allowing the garbage
// collector to reclaim memory. The genai.Client doesn't require
// explicit resource cleanup beyond this reference clearing.
//
// Returns:
//   - nil: Always returns nil as no cleanup errors are expected
func (s *GeminiService) Close() error {
	s.logger.Info().Msg("Closing Gemini LLM service")

	// Clear client reference (genai.Client doesn't require explicit Close)
	s.client = nil

	return nil
}

// generateCompletion is a helper method that encapsulates the Google ADK
// chat completion logic using the agent/runner pattern with Gemini models.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - messages: Conversation history to process
//
// Returns:
//   - string: Generated response text
//   - error: nil on success, error on failure
func (s *GeminiService) generateCompletion(ctx context.Context, messages []interfaces.Message) (string, error) {
	// Convert interfaces.Message to Gemini format
	geminiContents, systemText, err := convertMessagesToGemini(messages)
	if err != nil {
		return "", fmt.Errorf("failed to convert messages to Gemini format: %w", err)
	}

	// Create GenerateContentConfig with temperature and system instruction
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(s.config.Temperature),
	}

	// Set SystemInstruction if system message exists
	if systemText != "" {
		config.SystemInstruction = genai.NewContentFromText(systemText, genai.RoleUser)
	}

	// Generate completion using direct GenerateContent call
	resp, err := s.client.Models.GenerateContent(ctx, s.config.ChatModel, geminiContents, config)
	if err != nil {
		return "", fmt.Errorf("chat generation failed: %w", err)
	}

	// Extract text from response - iterate candidates until non-empty text is found
	var response strings.Builder
	if resp != nil && len(resp.Candidates) > 0 {
		// Try each candidate until we find one with non-empty text
		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					response.WriteString(part.Text)
				}
			}
			// If we found text in this candidate, use it
			if response.Len() > 0 {
				break
			}
		}
	}

	if response.Len() == 0 {
		return "", fmt.Errorf("no response generated from chat model")
	}

	return response.String(), nil
}