package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// ClaudeService implements the LLMService interface using Anthropic Claude API.
// It provides chat completions using Claude models.
type ClaudeService struct {
	config    *common.ClaudeConfig
	logger    arbor.ILogger
	client    *anthropic.Client
	timeout   time.Duration
	maxTokens int
}

// convertMessagesToClaude converts []interfaces.Message to Claude MessageParam format.
// Maps Role values to provider's expected values and maintains chronological ordering.
// Extracts system messages separately for use with System parameter.
// Returns the user/assistant messages, the first system message content (if any), and an error.
func convertMessagesToClaude(messages []interfaces.Message) ([]anthropic.MessageParam, string, error) {
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

	// Convert messages to Claude format, excluding system messages
	claudeMessages := make([]anthropic.MessageParam, 0, len(messages))
	var systemText string
	for _, msg := range messages {
		// Handle system messages separately
		if msg.Role == "system" {
			if systemText == "" {
				systemText = msg.Content
			}
			continue // Don't add system messages to messages array
		}

		// Create message based on role
		switch msg.Role {
		case "assistant":
			claudeMessages = append(claudeMessages, anthropic.NewAssistantMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		case "user":
			claudeMessages = append(claudeMessages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		default:
			// Default to user for unknown roles
			claudeMessages = append(claudeMessages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		}
	}

	return claudeMessages, systemText, nil
}

// NewClaudeService creates a new Claude LLM service instance.
//
// The service initialization includes:
//  1. Resolving Anthropic API key from KV store with config fallback
//  2. Setting default model name if not specified
//  3. Parsing timeout duration from configuration
//  4. Initializing Claude client
//
// Parameters:
//   - claudeConfig: Claude configuration with API key and model settings
//   - storageManager: Storage manager interface for KV and auth storage access
//   - logger: Structured logger for service operations
//
// Returns:
//   - *ClaudeService: Initialized service ready for use
//   - error: nil on success, error with details on failure
func NewClaudeService(claudeConfig *common.ClaudeConfig, storageManager interfaces.StorageManager, logger arbor.ILogger) (*ClaudeService, error) {
	// Resolve API key with KV-first resolution order: KV store → config fallback
	ctx := context.Background()
	apiKey, err := common.ResolveAPIKey(ctx, storageManager.KeyValueStorage(), "anthropic_api_key", claudeConfig.APIKey)
	if err != nil {
		return nil, fmt.Errorf("Anthropic API key is required for Claude service (set via ANTHROPIC_API_KEY, QUAERO_CLAUDE_API_KEY, or claude.api_key in config): %w", err)
	}

	// Set default model name if not specified
	if claudeConfig.Model == "" {
		claudeConfig.Model = "claude-sonnet-4-20250514"
	}

	// Parse timeout duration
	timeout, err := time.ParseDuration(claudeConfig.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration '%s': %w", claudeConfig.Timeout, err)
	}

	// Set default max tokens
	maxTokens := claudeConfig.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	// Initialize Claude client
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	// Create service instance
	service := &ClaudeService{
		config:    claudeConfig,
		logger:    logger,
		client:    client,
		timeout:   timeout,
		maxTokens: maxTokens,
	}

	logger.Debug().
		Str("model", claudeConfig.Model).
		Dur("timeout", timeout).
		Float32("temperature", claudeConfig.Temperature).
		Int("max_tokens", maxTokens).
		Msg("Claude LLM service initialized successfully")

	return service, nil
}

// Chat generates a completion response based on the conversation history.
//
// This method uses the configured Claude model for high-quality
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
func (s *ClaudeService) Chat(ctx context.Context, messages []interfaces.Message) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("messages cannot be empty for chat completion")
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	startTime := time.Now()
	s.logger.Debug().
		Int("message_count", len(messages)).
		Msg("Starting Claude chat completion")

	// Generate completion
	response, err := s.generateCompletion(timeoutCtx, messages)
	if err != nil {
		s.logger.Error().
			Err(err).
			Int("message_count", len(messages)).
			Msg("Claude chat completion failed")
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	duration := time.Since(startTime)
	s.logger.Debug().
		Int("message_count", len(messages)).
		Int("response_length", len(response)).
		Dur("duration", duration).
		Msg("Claude chat completion completed successfully")

	return response, nil
}

// HealthCheck verifies the Claude service is operational and can handle requests.
//
// The health check validates that the Claude client is properly initialized
// and accessible. This includes a lightweight connectivity probe.
//
// Parameters:
//   - ctx: Context for cancellation control
//
// Returns:
//   - nil if service is healthy (operational)
//   - error with details if service is unhealthy
func (s *ClaudeService) HealthCheck(ctx context.Context) error {
	s.logger.Debug().Msg("Running Claude LLM service health check")

	// Verify client is initialized
	if s.client == nil {
		return fmt.Errorf("Claude client is not initialized")
	}

	// Perform lightweight connectivity probe with short timeout
	if err := s.performHealthCheck(ctx); err != nil {
		return fmt.Errorf("Claude health check failed: %w", err)
	}

	s.logger.Debug().
		Str("model", s.config.Model).
		Msg("Claude LLM service health check passed")

	return nil
}

// performHealthCheck exercises the Claude API with a minimal probe.
func (s *ClaudeService) performHealthCheck(ctx context.Context) error {
	// Create timeout context for health check
	healthCheckCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
		return fmt.Errorf("Claude probe failed: %w", err)
	}

	// Validate that we got a non-empty response
	if len(strings.TrimSpace(response)) == 0 {
		return fmt.Errorf("Claude probe returned empty response")
	}

	s.logger.Debug().
		Int("response_length", len(response)).
		Msg("Claude health check passed")

	return nil
}

// GetMode returns the current operational mode of the LLM service.
//
// Since this implementation uses Anthropic cloud APIs, it returns
// LLMModeCloud to indicate cloud-based service usage.
//
// Returns:
//   - interfaces.LLMModeCloud: Indicating cloud-based service
func (s *ClaudeService) GetMode() interfaces.LLMMode {
	return interfaces.LLMModeCloud
}

// Close releases resources and performs cleanup operations.
//
// Returns:
//   - nil: Always returns nil as no cleanup errors are expected
func (s *ClaudeService) Close() error {
	s.logger.Debug().Msg("Closing Claude LLM service")
	// Claude client doesn't require explicit cleanup
	s.client = nil
	return nil
}

// generateCompletion is a helper method that encapsulates the Claude API
// chat completion logic.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - messages: Conversation history to process
//
// Returns:
//   - string: Generated response text
//   - error: nil on success, error on failure
func (s *ClaudeService) generateCompletion(ctx context.Context, messages []interfaces.Message) (string, error) {
	// Convert interfaces.Message to Claude format
	claudeMessages, systemText, err := convertMessagesToClaude(messages)
	if err != nil {
		return "", fmt.Errorf("failed to convert messages to Claude format: %w", err)
	}

	// Build request parameters
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(s.config.Model),
		MaxTokens: int64(s.maxTokens),
		Messages:  claudeMessages,
	}

	// Set temperature if configured
	if s.config.Temperature > 0 {
		params.Temperature = anthropic.Float(float64(s.config.Temperature))
	}

	// Set system message if present
	if systemText != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemText},
		}
	}

	// Make the API call
	resp, err := s.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("Claude API call failed: %w", err)
	}

	// Extract text from response
	var response strings.Builder
	for _, block := range resp.Content {
		if block.Type == anthropic.ContentBlockTypeText {
			response.WriteString(block.Text)
		}
	}

	if response.Len() == 0 {
		return "", fmt.Errorf("no response generated from Claude API")
	}

	return response.String(), nil
}

// GetClient returns the underlying Anthropic client for direct API access.
// This is useful for workers that need extended thinking or other advanced features.
func (s *ClaudeService) GetClient() *anthropic.Client {
	return s.client
}

// GetConfig returns the Claude configuration.
func (s *ClaudeService) GetConfig() *common.ClaudeConfig {
	return s.config
}
