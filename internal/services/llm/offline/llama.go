// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 6:18:01 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package offline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// OfflineLLMService provides local LLM operations using llama binaries
// SECURITY: Guarantees 100% local operation with NO external network calls
// Uses llama-server for embeddings (localhost HTTP) and llama-cli for chat
type OfflineLLMService struct {
	modelManager     *ModelManager
	contextSize      int
	threadCount      int
	gpuLayers        int
	logger           arbor.ILogger
	llamaCLIPath     string
	llamaServerPath  string
	embedServerCmd   *exec.Cmd
	embedServerURL   string
	embedServerReady bool
	mockMode         bool
}

// llamaServerEmbeddingRequest represents embedding request to llama-server
type llamaServerEmbeddingRequest struct {
	Content string `json:"content"`
}

// llamaServerEmbeddingResponse represents the JSON output from llama-server /embedding
type llamaServerEmbeddingResponse struct {
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
	// Find llama-cli binary (for chat)
	llamaCLIPath, err := findLlamaBinary(llamaDir, "llama-cli", logger)
	if err != nil {
		return nil, fmt.Errorf("llama-cli binary not found: %w", err)
	}

	// Find llama-server binary (for embeddings)
	llamaServerPath, err := findLlamaBinary(llamaDir, "llama-server", logger)
	if err != nil {
		return nil, fmt.Errorf("llama-server binary not found: %w", err)
	}

	logger.Info().
		Str("cli_path", llamaCLIPath).
		Str("server_path", llamaServerPath).
		Msg("Found llama binaries")

	// Create model manager
	modelManager := NewModelManager(modelDir, embedModel, chatModel, logger)

	// Verify models exist
	if err := modelManager.VerifyModels(); err != nil {
		return nil, fmt.Errorf("model verification failed: %w", err)
	}

	service := &OfflineLLMService{
		modelManager:     modelManager,
		contextSize:      contextSize,
		threadCount:      threadCount,
		gpuLayers:        gpuLayers,
		logger:           logger,
		llamaCLIPath:     llamaCLIPath,
		llamaServerPath:  llamaServerPath,
		embedServerURL:   "http://127.0.0.1:8086", // Local-only, different from main server
		embedServerReady: false,
		mockMode:         false,
	}

	// Start llama-server for embeddings
	if err := service.startEmbeddingServer(); err != nil {
		return nil, fmt.Errorf("failed to start embedding server: %w", err)
	}

	logger.Info().
		Str("mode", "offline").
		Int("context_size", contextSize).
		Int("threads", threadCount).
		Int("gpu_layers", gpuLayers).
		Str("embed_server_url", service.embedServerURL).
		Msg("Offline LLM service initialized")

	return service, nil
}

// NewMockOfflineLLMService creates an offline LLM service in mock mode for testing
// This bypasses llama-cli binary and model file requirements
func NewMockOfflineLLMService(logger arbor.ILogger) *OfflineLLMService {
	service := &OfflineLLMService{
		modelManager:     nil, // Not needed in mock mode
		contextSize:      2048,
		threadCount:      4,
		gpuLayers:        0,
		logger:           logger,
		llamaCLIPath:     "",
		llamaServerPath:  "",
		embedServerURL:   "",
		embedServerReady: false,
		mockMode:         true,
	}

	logger.Warn().Msg("Created offline LLM service in MOCK mode - using fake responses")

	return service
}

// findLlamaBinary locates a llama binary in configured directory or standard locations
func findLlamaBinary(llamaDir string, binaryName string, logger arbor.ILogger) (string, error) {
	// Build search locations in order of preference
	locations := []string{}

	// 1. Configured llama directory (highest priority)
	if llamaDir != "" {
		locations = append(locations, llamaDir+"/"+binaryName)
		locations = append(locations, llamaDir+"/"+binaryName+".exe")
	}

	// 2. Legacy fallback locations (for backwards compatibility)
	locations = append(locations,
		"./bin/"+binaryName,
		"./bin/"+binaryName+".exe",
		"./"+binaryName,
		"./"+binaryName+".exe",
		binaryName, // Will search PATH
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
					Str("binary", binaryName).
					Msg("Found llama binary")
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("%s not found in: %v", binaryName, locations)
}

// startEmbeddingServer starts llama-server in embedding mode
// SECURITY: Server binds to 127.0.0.1 only - no external access possible
func (s *OfflineLLMService) startEmbeddingServer() error {
	s.logger.Info().
		Str("model", s.modelManager.GetEmbedModelPath()).
		Str("url", s.embedServerURL).
		Msg("Starting embedding server")

	// Build command: llama-server -m <model> --embedding --host 127.0.0.1 --port 8086
	args := []string{
		"-m", s.modelManager.GetEmbedModelPath(),
		"--embedding",
		"--host", "127.0.0.1", // SECURITY: localhost only
		"--port", "8086",
		"-t", strconv.Itoa(s.threadCount),
		"-ngl", strconv.Itoa(s.gpuLayers),
		"-b", "2048", // Increase physical batch size to handle larger inputs
		"--log-disable", // Disable server logs to reduce noise
	}

	// Create command
	cmd := exec.Command(s.llamaServerPath, args...)

	// Start server in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	s.embedServerCmd = cmd
	s.logger.Info().
		Int("pid", cmd.Process.Pid).
		Msg("Embedding server started, waiting for ready")

	// Wait for server to be ready (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.stopEmbeddingServer()
			return fmt.Errorf("embedding server did not become ready within 30 seconds")
		case <-ticker.C:
			if s.checkEmbeddingServerHealth() {
				s.embedServerReady = true
				s.logger.Info().Msg("Embedding server is ready")
				return nil
			}
		}
	}
}

// checkEmbeddingServerHealth checks if embedding server is responding
func (s *OfflineLLMService) checkEmbeddingServerHealth() bool {
	client := &http.Client{
		Timeout: 1 * time.Second,
		Transport: &http.Transport{
			// SECURITY: Only allow connections to localhost
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "localhost:") {
					return nil, fmt.Errorf("security violation: attempt to connect to non-localhost address: %s", addr)
				}
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}

	resp, err := client.Get(s.embedServerURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// stopEmbeddingServer stops the background embedding server
func (s *OfflineLLMService) stopEmbeddingServer() {
	if s.embedServerCmd != nil && s.embedServerCmd.Process != nil {
		s.logger.Info().
			Int("pid", s.embedServerCmd.Process.Pid).
			Msg("Stopping embedding server")

		// Try graceful shutdown first
		s.embedServerCmd.Process.Signal(os.Interrupt)

		// Wait up to 5 seconds for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- s.embedServerCmd.Wait()
		}()

		select {
		case <-time.After(5 * time.Second):
			// Force kill if not stopped
			s.logger.Warn().Msg("Embedding server did not stop gracefully, forcing kill")
			s.embedServerCmd.Process.Kill()
		case <-done:
			s.logger.Info().Msg("Embedding server stopped gracefully")
		}

		s.embedServerReady = false
	}
}

// Embed generates a 768-dimension embedding vector for the given text
// SECURITY: Uses llama-server on localhost:8086 ONLY - no external network access
func (s *OfflineLLMService) Embed(ctx context.Context, text string) ([]float32, error) {
	if s.mockMode {
		return s.generateMockEmbedding(text), nil
	}

	if !s.embedServerReady {
		return nil, fmt.Errorf("embedding server not ready")
	}

	s.logger.Debug().
		Int("text_length", len(text)).
		Msg("Generating embedding")

	// Create request body
	reqBody := llamaServerEmbeddingRequest{
		Content: text,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP client with localhost-only transport
	// SECURITY: Transport enforces localhost-only connections
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// SECURITY: Reject any non-localhost connections
				if !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "localhost:") {
					return nil, fmt.Errorf("security violation: attempt to connect to non-localhost address: %s", addr)
				}
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}

	// Make request to local embedding server
	req, err := http.NewRequestWithContext(ctx, "POST", s.embedServerURL+"/embedding", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error().
			Err(err).
			Msg("Embedding generation failed")
		return nil, fmt.Errorf("llama-server request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Error().
			Int("status_code", resp.StatusCode).
			Str("response", string(body)).
			Msg("Embedding server returned error")
		return nil, fmt.Errorf("llama-server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	// The llama-server /embedding endpoint returns a JSON object with a single "embedding" field,
	// which is an array of floats.
	var response llamaServerEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		s.logger.Error().
			Err(err).
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

	// Mock mode always healthy
	if s.mockMode {
		s.logger.Debug().Msg("Mock mode - health check passed")
		return nil
	}

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

// Close releases resources and stops embedding server
func (s *OfflineLLMService) Close() error {
	s.logger.Info().Msg("Closing offline LLM service")
	s.stopEmbeddingServer()
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
