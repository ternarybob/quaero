package offline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// OfflineLLMService provides local LLM operations using llama-cli binary
// SECURITY: Guarantees 100% local operation with NO network calls
type OfflineLLMService struct {
	modelManager *ModelManager
	contextSize  int
	threadCount  int
	gpuLayers    int
	logger       arbor.ILogger
	llamaCLIPath string
	mockMode     bool
}

// llamaEmbeddingResponse represents the JSON output from llama-cli --embedding
type llamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// NewOfflineLLMService creates a new offline LLM service instance
// Returns error if llama-cli binary not found or models missing
func NewOfflineLLMService(
	llamaDir string,
	modelDir string,
	embedModel string,
	chatModel string,
	contextSize int,
	threadCount int,
	gpuLayers int,
	logger arbor.ILogger,
) (*OfflineLLMService, error) {
	// Find llama-cli binary
	llamaCLIPath, err := findLlamaCLI(llamaDir, logger)
	if err != nil {
		return nil, fmt.Errorf("llama-cli binary not found: %w", err)
	}

	logger.Info().
		Str("binary_path", llamaCLIPath).
		Msg("Found llama-cli binary")

	// Create model manager
	modelManager := NewModelManager(modelDir, embedModel, chatModel, logger)

	// Verify models exist
	if err := modelManager.VerifyModels(); err != nil {
		return nil, fmt.Errorf("model verification failed: %w", err)
	}

	service := &OfflineLLMService{
		modelManager: modelManager,
		contextSize:  contextSize,
		threadCount:  threadCount,
		gpuLayers:    gpuLayers,
		logger:       logger,
		llamaCLIPath: llamaCLIPath,
		mockMode:     false,
	}

	logger.Info().
		Str("mode", "offline").
		Int("context_size", contextSize).
		Int("threads", threadCount).
		Int("gpu_layers", gpuLayers).
		Msg("Offline LLM service initialized")

	return service, nil
}

// NewMockOfflineLLMService creates an offline LLM service in mock mode for testing
// This bypasses llama-cli binary and model file requirements
func NewMockOfflineLLMService(logger arbor.ILogger) *OfflineLLMService {
	service := &OfflineLLMService{
		modelManager: nil, // Not needed in mock mode
		contextSize:  2048,
		threadCount:  4,
		gpuLayers:    0,
		logger:       logger,
		llamaCLIPath: "",
		mockMode:     true,
	}

	logger.Warn().Msg("Created offline LLM service in MOCK mode - using fake responses")

	return service
}

// findLlamaCLI locates the llama-cli binary in configured directory or standard locations
func findLlamaCLI(llamaDir string, logger arbor.ILogger) (string, error) {
	// Build search locations in order of preference
	locations := []string{}

	// 1. Configured llama directory (highest priority)
	if llamaDir != "" {
		locations = append(locations, llamaDir+"/llama-cli")
		locations = append(locations, llamaDir+"/llama-cli.exe")
	}

	// 2. Legacy fallback locations (for backwards compatibility)
	locations = append(locations,
		"./bin/llama-cli",
		"./bin/llama-cli.exe",
		"./llama-cli",
		"./llama-cli.exe",
		"llama-cli", // Will search PATH
	)

	for _, location := range locations {
		path, err := exec.LookPath(location)
		if err == nil {
			// Verify file exists and is executable
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				logger.Debug().
					Str("location", location).
					Str("resolved_path", path).
					Msg("Found llama-cli binary")
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("llama-cli not found in: %v", locations)
}

// Embed generates a 768-dimension embedding vector for the given text
// SECURITY: Uses only local binary execution, no network calls
func (s *OfflineLLMService) Embed(ctx context.Context, text string) ([]float32, error) {
	if s.mockMode {
		return s.generateMockEmbedding(text), nil
	}

	s.logger.Debug().
		Int("text_length", len(text)).
		Msg("Generating embedding")

	// Build command: llama-cli -m <model> --embedding -p "<text>"
	args := []string{
		"-m", s.modelManager.GetEmbedModelPath(),
		"--embedding",
		"-p", text,
		"--json",
	}

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, s.llamaCLIPath, args...)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("output", string(output)).
			Msg("Embedding generation failed")
		return nil, fmt.Errorf("llama-cli execution failed: %w", err)
	}

	// Parse JSON response
	var response llamaEmbeddingResponse
	if err := json.Unmarshal(output, &response); err != nil {
		s.logger.Error().
			Err(err).
			Str("output", string(output)).
			Msg("Failed to parse embedding response")
		return nil, fmt.Errorf("failed to parse embedding JSON: %w", err)
	}

	if len(response.Embedding) == 0 {
		return nil, fmt.Errorf("embedding vector is empty")
	}

	s.logger.Debug().
		Int("dimension", len(response.Embedding)).
		Msg("Embedding generated successfully")

	return response.Embedding, nil
}

// Chat generates a completion response based on conversation history
// SECURITY: Uses only local binary execution, no network calls
func (s *OfflineLLMService) Chat(ctx context.Context, messages []interfaces.Message) (string, error) {
	if s.mockMode {
		return s.generateMockResponse(messages), nil
	}

	s.logger.Debug().
		Int("message_count", len(messages)).
		Msg("Generating chat completion")

	// Convert messages to Qwen 2.5 prompt format
	prompt := s.formatPrompt(messages)

	// Build command
	args := []string{
		"-m", s.modelManager.GetChatModelPath(),
		"-p", prompt,
		"-n", "2048", // Max tokens to generate
		"-c", strconv.Itoa(s.contextSize),
		"-t", strconv.Itoa(s.threadCount),
		"-ngl", strconv.Itoa(s.gpuLayers),
		"--no-display-prompt", // Don't echo the prompt
	}

	// Create command with context
	cmd := exec.CommandContext(ctx, s.llamaCLIPath, args...)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("output", string(output)).
			Msg("Chat completion failed")
		return "", fmt.Errorf("llama-cli execution failed: %w", err)
	}

	response := s.extractResponse(string(output))

	s.logger.Debug().
		Int("response_length", len(response)).
		Msg("Chat completion generated")

	return response, nil
}

// formatPrompt converts messages to Qwen 2.5 chat template format
func (s *OfflineLLMService) formatPrompt(messages []interfaces.Message) string {
	var builder strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			builder.WriteString("<|im_start|>system\n")
			builder.WriteString(msg.Content)
			builder.WriteString("<|im_end|>\n")
		case "user":
			builder.WriteString("<|im_start|>user\n")
			builder.WriteString(msg.Content)
			builder.WriteString("<|im_end|>\n")
		case "assistant":
			builder.WriteString("<|im_start|>assistant\n")
			builder.WriteString(msg.Content)
			builder.WriteString("<|im_end|>\n")
		}
	}

	// Add final assistant prompt
	builder.WriteString("<|im_start|>assistant\n")

	return builder.String()
}

// extractResponse extracts the generated text from llama-cli output
func (s *OfflineLLMService) extractResponse(output string) string {
	// llama-cli outputs the generated text directly
	// Clean up any control sequences or extra whitespace
	lines := strings.Split(output, "\n")
	var response strings.Builder

	for _, line := range lines {
		// Skip empty lines and debug output
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip lines that look like debug output
		if strings.HasPrefix(trimmed, "llama_") || strings.HasPrefix(trimmed, "ggml_") {
			continue
		}
		if strings.Contains(trimmed, "perplexity:") || strings.Contains(trimmed, "tokens per second") {
			continue
		}

		response.WriteString(trimmed)
		response.WriteString("\n")
	}

	return strings.TrimSpace(response.String())
}

// HealthCheck verifies the LLM service is operational
// SECURITY: Only checks local file system, no network calls
func (s *OfflineLLMService) HealthCheck(ctx context.Context) error {
	s.logger.Debug().Msg("Running health check")

	// Check llama-cli binary exists and is executable
	info, err := os.Stat(s.llamaCLIPath)
	if err != nil {
		return fmt.Errorf("llama-cli binary not accessible: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("llama-cli path is a directory: %s", s.llamaCLIPath)
	}

	// Verify models exist
	if err := s.modelManager.VerifyModels(); err != nil {
		return fmt.Errorf("model verification failed: %w", err)
	}

	// Try running llama-cli --version to confirm it works
	cmd := exec.CommandContext(ctx, s.llamaCLIPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("output", string(output)).
			Msg("Failed to get llama-cli version")
		return fmt.Errorf("llama-cli binary not functional: %w", err)
	}

	version := strings.TrimSpace(string(output))
	s.logger.Info().
		Str("version", version).
		Msg("Health check passed")

	return nil
}

// GetMode returns the operational mode (always offline)
func (s *OfflineLLMService) GetMode() interfaces.LLMMode {
	return interfaces.LLMModeOffline
}

// Close releases resources (no cleanup needed for binary execution)
func (s *OfflineLLMService) Close() error {
	s.logger.Info().Msg("Closing offline LLM service")
	return nil
}

// SetMockMode enables or disables mock mode for testing without binary
func (s *OfflineLLMService) SetMockMode(enabled bool) {
	s.mockMode = enabled
	if enabled {
		s.logger.Warn().Msg("Mock mode enabled - using fake responses")
	} else {
		s.logger.Info().Msg("Mock mode disabled - using real llama-cli")
	}
}

// generateMockEmbedding creates a fake embedding for testing
func (s *OfflineLLMService) generateMockEmbedding(text string) []float32 {
	// Generate deterministic 768-dimension vector based on text
	embedding := make([]float32, 768)
	seed := 0
	for _, c := range text {
		seed += int(c)
	}

	for i := range embedding {
		// Simple deterministic generation
		embedding[i] = float32((seed+i)%100) / 100.0
	}

	return embedding
}

// generateMockResponse creates a fake chat response for testing
func (s *OfflineLLMService) generateMockResponse(messages []interfaces.Message) string {
	if len(messages) == 0 {
		return "Mock response: No messages provided"
	}

	lastMsg := messages[len(messages)-1]
	return fmt.Sprintf("Mock response to: %s", lastMsg.Content)
}
