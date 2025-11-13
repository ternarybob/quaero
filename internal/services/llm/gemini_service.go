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
// It provides embedding and chat completions using Gemini models.
type GeminiService struct {
	config   *common.LLMConfig
	logger   arbor.ILogger
	client   *genai.Client
	timeout  time.Duration
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
//  1. Resolving Google API key from auth storage with config fallback
//  2. Setting default model names if not specified
//  3. Validating that EmbedDimension matches SQLite.EmbeddingDimension
//  4. Parsing timeout duration from configuration
//  5. Initializing both embedding and chat models with Google ADK
//
// Parameters:
//   - config: Full application configuration to access storage settings
//   - authStorage: Auth storage interface for API key resolution
//   - logger: Structured logger for service operations
//
// Returns:
//   - *GeminiService: Initialized service ready for use
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Missing or empty Google API key (from storage or config)
//   - EmbedDimension mismatch with SQLite configuration
//   - Invalid model names or timeout duration
//   - Failed to initialize ADK models (network, auth, etc.)
func NewGeminiService(config *common.Config, authStorage interfaces.AuthStorage, logger arbor.ILogger) (*GeminiService, error) {
	// Resolve API key from auth storage with config fallback
	ctx := context.Background()
	apiKey, err := common.ResolveAPIKey(ctx, authStorage, "gemini-llm", config.LLM.GoogleAPIKey)
	if err != nil {
		return nil, fmt.Errorf("Google API key is required for LLM service (set via auth storage, QUAERO_LLM_GOOGLE_API_KEY, or llm.google_api_key in config): %w", err)
	}

	// Validate that EmbedDimension matches SQLite.EmbeddingDimension
	if config.LLM.EmbedDimension != config.Storage.SQLite.EmbeddingDimension {
		return nil, fmt.Errorf("LLM.EmbedDimension (%d) must match SQLite.EmbeddingDimension (%d): embedding dimension mismatch will cause database storage errors", config.LLM.EmbedDimension, config.Storage.SQLite.EmbeddingDimension)
	}

	// Set default model names if not specified
	if config.LLM.EmbedModelName == "" {
		config.LLM.EmbedModelName = "gemini-embedding-001"
	}
	if config.LLM.ChatModelName == "" {
		config.LLM.ChatModelName = "gemini-2.0-flash"
	}

	// Parse timeout duration
	timeout, err := time.ParseDuration(config.LLM.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration '%s': %w", config.LLM.Timeout, err)
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
		config:  &config.LLM, // Store only the LLM config part
		logger:  logger,
		client:  client,
		timeout: timeout,
	}

	logger.Info().
		Str("embed_model", config.LLM.EmbedModelName).
		Str("chat_model", config.LLM.ChatModelName).
		Int("embed_dimension", config.LLM.EmbedDimension).
		Int("sqlite_embedding_dimension", config.Storage.SQLite.EmbeddingDimension).
		Dur("timeout", timeout).
		Msg("Gemini LLM service initialized successfully")

	return service, nil
}

// Embed generates a 768-dimension embedding vector for the given text.
//
// This method uses the gemini-embedding-001 model with 768 output dimensionality
// to maintain compatibility with the existing database schema. The embedding
// vector can be used for semantic search, similarity comparison, and vector
// storage operations.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - text: Input text to generate embedding for
//
// Returns:
//   - []float32: 768-dimension embedding vector
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Context cancellation or timeout
//   - Empty or invalid input text
//   - API communication errors
//   - Invalid response format from Google ADK
func (s *GeminiService) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty for embedding generation")
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	startTime := time.Now()
	s.logger.Debug().
		Int("text_length", len(text)).
		Msg("Starting embedding generation")

	// Generate embedding using Google ADK
	// Note: The exact method name may vary based on the Google ADK API
	// This is a placeholder implementation following the established pattern
	embedding, err := s.generateEmbedding(timeoutCtx, text)
	if err != nil {
		s.logger.Error().
			Err(err).
			Int("text_length", len(text)).
			Msg("Embedding generation failed")
		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}

	duration := time.Since(startTime)
	s.logger.Info().
		Int("text_length", len(text)).
		Int("embedding_dim", len(embedding)).
		Dur("duration", duration).
		Msg("Embedding generation completed successfully")

	return embedding, nil
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
// probes to exercise both models with short timeouts.
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

	// Perform lightweight connectivity probes with short timeouts
	if err := s.performEmbeddingHealthCheck(ctx); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Embedding model health check failed")
		return fmt.Errorf("embedding model health check failed: %w", err)
	}

	if err := s.performChatHealthCheck(ctx); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Chat model health check failed")
		return fmt.Errorf("chat model health check failed: %w", err)
	}

	s.logger.Info().
		Str("embed_model", s.config.EmbedModelName).
		Str("chat_model", s.config.ChatModelName).
		Msg("Gemini LLM service health check passed")

	return nil
}

// performEmbeddingHealthCheck exercises the embedding model with a lightweight probe.
// Uses a longer timeout to avoid false negatives and logs detailed failures.
func (s *GeminiService) performEmbeddingHealthCheck(ctx context.Context) error {
	// Create timeout context for health check (increased to 5s to avoid false negatives)
	healthCheckCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use a simple, static string for the probe
	testText := "health check probe"

	// Generate embedding and immediately discard the result
	embedding, err := s.generateEmbedding(healthCheckCtx, testText)
	if err != nil {
		return fmt.Errorf("embedding probe failed: %w", err)
	}

	// Validate that we got a non-empty embedding vector
	if len(embedding) == 0 {
		return fmt.Errorf("embedding probe returned empty vector")
	}

	s.logger.Debug().
		Int("embedding_dim", len(embedding)).
		Msg("Embedding model health check passed")

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

// generateEmbedding is a helper method that encapsulates the Google ADK
// embedding generation logic using gemini-embedding-001 with the specified
// output dimensionality.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - text: Text to generate embedding for
//
// Returns:
//   - []float32: embedding vector with configured dimensionality
//   - error: nil on success, error on failure
func (s *GeminiService) generateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Configure embedding with output dimensionality
	outputDim := int32(s.config.EmbedDimension)
	embeddingConfig := &genai.EmbedContentConfig{
		OutputDimensionality: &outputDim,
	}

	// Generate embedding using the genai client
	result, err := s.client.Models.EmbedContent(ctx, s.config.EmbedModelName, []*genai.Content{genai.NewContentFromText(text, genai.RoleUser)}, embeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}

	// Extract embedding vector from response
	var embedding []float32
	if result != nil && len(result.Embeddings) > 0 {
		embedding = result.Embeddings[0].Values
	}

	if embedding == nil {
		return nil, fmt.Errorf("no embedding returned from API")
	}

	// Validate embedding dimension
	if len(embedding) != s.config.EmbedDimension {
		return nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", s.config.EmbedDimension, len(embedding))
	}

	return embedding, nil
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
	resp, err := s.client.Models.GenerateContent(ctx, s.config.ChatModelName, geminiContents, config)
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