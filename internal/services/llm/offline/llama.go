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
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// OfflineLLMService provides local LLM operations using llama binaries
// SECURITY: Guarantees 100% local operation with NO external network calls
// Uses llama-server for both embeddings and chat (localhost HTTP)
type OfflineLLMService struct {
	modelManager       *ModelManager
	contextSize        int
	threadCount        int
	gpuLayers          int
	logger             arbor.ILogger
	llamaServerPath    string
	embedServerCmd     *exec.Cmd
	embedServerURL     string
	embedServerReady   bool
	chatServerCmd      *exec.Cmd
	chatServerURL      string
	chatServerReady    bool
	mockMode           bool
	cachedHealthStatus error         // Cached health check result
	healthCheckTime    time.Time     // Last health check time
	healthCheckMutex   *sync.RWMutex // Mutex for health check cache
}

// llamaServerEmbeddingRequest represents embedding request to llama-server
type llamaServerEmbeddingRequest struct {
	Content string `json:"content"`
}

// llamaServerEmbeddingResponse represents embedding response from llama-server
type llamaServerEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

type llamaServerBatchEmbeddingResponse struct {
	Index     int         `json:"index"`
	Embedding [][]float32 `json:"embedding"` // Nested array format
}

// llamaServerChatRequest represents chat request to llama-server
type llamaServerChatRequest struct {
	Messages    []llamaServerMessage `json:"messages"`
	Temperature float32              `json:"temperature,omitempty"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Stream      bool                 `json:"stream"`
}

// llamaServerMessage represents a single message in chat request
type llamaServerMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// llamaServerChatResponse represents chat response from llama-server
type llamaServerChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewOfflineLLMService creates a new offline LLM service instance
// Returns error if llama-server binary not found or models missing
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
	// Find llama-server binary (for both embeddings and chat)
	llamaServerPath, err := findLlamaBinary(llamaDir, "llama-server", logger)
	if err != nil {
		return nil, fmt.Errorf("llama-server binary not found: %w", err)
	}

	logger.Info().
		Str("server_path", llamaServerPath).
		Msg("Found llama-server binary")

	// Create model manager
	modelManager := NewModelManager(modelDir, embedModel, chatModel, logger)

	// Verify models exist
	if err := modelManager.VerifyModels(); err != nil {
		return nil, fmt.Errorf("model verification failed: %w", err)
	}

	service := &OfflineLLMService{
		modelManager:       modelManager,
		contextSize:        contextSize,
		threadCount:        threadCount,
		gpuLayers:          gpuLayers,
		logger:             logger,
		llamaServerPath:    llamaServerPath,
		embedServerURL:     "http://127.0.0.1:8086", // Local-only, for embeddings
		embedServerReady:   false,
		chatServerURL:      "http://127.0.0.1:8087", // Local-only, for chat
		chatServerReady:    false,
		mockMode:           false,
		cachedHealthStatus: nil,
		healthCheckTime:    time.Time{},
		healthCheckMutex:   &sync.RWMutex{},
	}

	// Clean up any orphaned llama-server processes from previous runs
	logger.Info().Msg("Checking for orphaned llama-server processes")
	if err := service.cleanupOrphanedProcesses(); err != nil {
		logger.Warn().Err(err).Msg("Failed to cleanup orphaned processes (non-critical)")
	}

	// Start llama-server for embeddings
	if err := service.startEmbeddingServer(); err != nil {
		return nil, fmt.Errorf("failed to start embedding server: %w", err)
	}

	// Start llama-server for chat
	if err := service.startChatServer(); err != nil {
		service.stopEmbeddingServer() // Clean up embedding server on failure
		return nil, fmt.Errorf("failed to start chat server: %w", err)
	}

	// Perform initial health check
	service.refreshHealthCheck(context.Background())

	// Start background health check updater (refreshes every 60 seconds)
	go service.healthCheckUpdater()

	logger.Info().
		Str("mode", "offline").
		Int("context_size", contextSize).
		Int("threads", threadCount).
		Int("gpu_layers", gpuLayers).
		Str("embed_server_url", service.embedServerURL).
		Str("chat_server_url", service.chatServerURL).
		Msg("Offline LLM service initialized")

	return service, nil
}

// NewMockOfflineLLMService creates an offline LLM service in mock mode for testing
// This bypasses llama-server binary and model file requirements
func NewMockOfflineLLMService(logger arbor.ILogger) *OfflineLLMService {
	service := &OfflineLLMService{
		modelManager:       nil, // Not needed in mock mode
		contextSize:        2048,
		threadCount:        4,
		gpuLayers:          0,
		logger:             logger,
		llamaServerPath:    "",
		embedServerURL:     "",
		embedServerReady:   false,
		chatServerURL:      "",
		chatServerReady:    false,
		mockMode:           true,
		cachedHealthStatus: nil,
		healthCheckTime:    time.Now(),
		healthCheckMutex:   &sync.RWMutex{},
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
		"-b", "4096", // Increase physical batch size to handle larger inputs (was 2048)
		"-ub", "4096", // Increase batch size for embeddings specifically
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
func (s *OfflineLLMService) stopEmbeddingServer() error {
	if s.embedServerCmd == nil || s.embedServerCmd.Process == nil {
		s.logger.Debug().Msg("Embedding server not running, nothing to stop")
		return nil
	}

	pid := s.embedServerCmd.Process.Pid
	s.logger.Info().
		Int("pid", pid).
		Msg("Stopping embedding server")

	// Try graceful shutdown first (Unix-like systems only)
	if !isWindows() {
		if err := s.embedServerCmd.Process.Signal(os.Interrupt); err != nil {
			s.logger.Debug().
				Err(err).
				Int("pid", pid).
				Msg("Failed to send interrupt signal (expected on some platforms)")
		}
	}

	// Wait up to 2 seconds for graceful shutdown (Windows doesn't support signals, so skip to kill)
	timeout := 2 * time.Second
	if isWindows() {
		timeout = 500 * time.Millisecond // Shorter timeout on Windows since graceful shutdown isn't supported
	}

	done := make(chan error, 1)
	go func() {
		done <- s.embedServerCmd.Wait()
	}()

	var shutdownErr error
	select {
	case <-time.After(timeout):
		// Force kill
		s.logger.Info().
			Int("pid", pid).
			Msg("Terminating embedding server")
		if err := s.embedServerCmd.Process.Kill(); err != nil {
			s.logger.Error().
				Err(err).
				Int("pid", pid).
				Msg("Failed to kill embedding server")
			shutdownErr = fmt.Errorf("failed to kill embedding server (pid %d): %w", pid, err)
		} else {
			s.logger.Info().
				Int("pid", pid).
				Msg("Embedding server terminated successfully")
		}
	case err := <-done:
		if err != nil && !isProcessExitError(err) {
			s.logger.Warn().
				Err(err).
				Int("pid", pid).
				Msg("Embedding server exited with error")
			shutdownErr = fmt.Errorf("embedding server exit error (pid %d): %w", pid, err)
		} else {
			s.logger.Info().
				Int("pid", pid).
				Msg("Embedding server stopped")
		}
	}

	s.embedServerReady = false
	return shutdownErr
}

// startChatServer starts llama-server in chat mode
// SECURITY: Server binds to 127.0.0.1 only - no external access possible
func (s *OfflineLLMService) startChatServer() error {
	s.logger.Info().
		Str("model", s.modelManager.GetChatModelPath()).
		Str("url", s.chatServerURL).
		Msg("Starting chat server")

	// Build command: llama-server -m <model> --host 127.0.0.1 --port 8087
	args := []string{
		"-m", s.modelManager.GetChatModelPath(),
		"--host", "127.0.0.1", // SECURITY: localhost only
		"--port", "8087",
		"-c", strconv.Itoa(s.contextSize),
		"-t", strconv.Itoa(s.threadCount),
		"-ngl", strconv.Itoa(s.gpuLayers),
		"-b", "2048", // Batch size for chat processing
		"--log-disable", // Disable server logs to reduce noise
	}

	// Create command
	cmd := exec.Command(s.llamaServerPath, args...)

	// Start server in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start chat server: %w", err)
	}

	s.chatServerCmd = cmd
	s.logger.Info().
		Int("pid", cmd.Process.Pid).
		Msg("Chat server started, waiting for ready")

	// Wait for server to be ready (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.stopChatServer()
			return fmt.Errorf("chat server did not become ready within 60 seconds")
		case <-ticker.C:
			if s.checkChatServerHealth() {
				s.chatServerReady = true
				s.logger.Info().Msg("Chat server is ready")
				return nil
			}
		}
	}
}

// checkChatServerHealth checks if chat server is responding
func (s *OfflineLLMService) checkChatServerHealth() bool {
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

	resp, err := client.Get(s.chatServerURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// stopChatServer stops the background chat server
func (s *OfflineLLMService) stopChatServer() error {
	if s.chatServerCmd == nil || s.chatServerCmd.Process == nil {
		s.logger.Debug().Msg("Chat server not running, nothing to stop")
		return nil
	}

	pid := s.chatServerCmd.Process.Pid
	s.logger.Info().
		Int("pid", pid).
		Msg("Stopping chat server")

	// Try graceful shutdown first (Unix-like systems only)
	if !isWindows() {
		if err := s.chatServerCmd.Process.Signal(os.Interrupt); err != nil {
			s.logger.Debug().
				Err(err).
				Int("pid", pid).
				Msg("Failed to send interrupt signal (expected on some platforms)")
		}
	}

	// Wait up to 2 seconds for graceful shutdown (Windows doesn't support signals, so skip to kill)
	timeout := 2 * time.Second
	if isWindows() {
		timeout = 500 * time.Millisecond // Shorter timeout on Windows since graceful shutdown isn't supported
	}

	done := make(chan error, 1)
	go func() {
		done <- s.chatServerCmd.Wait()
	}()

	var shutdownErr error
	select {
	case <-time.After(timeout):
		// Force kill
		s.logger.Info().
			Int("pid", pid).
			Msg("Terminating chat server")
		if err := s.chatServerCmd.Process.Kill(); err != nil {
			s.logger.Error().
				Err(err).
				Int("pid", pid).
				Msg("Failed to kill chat server")
			shutdownErr = fmt.Errorf("failed to kill chat server (pid %d): %w", pid, err)
		} else {
			s.logger.Info().
				Int("pid", pid).
				Msg("Chat server terminated successfully")
		}
	case err := <-done:
		if err != nil && !isProcessExitError(err) {
			s.logger.Warn().
				Err(err).
				Int("pid", pid).
				Msg("Chat server exited with error")
			shutdownErr = fmt.Errorf("chat server exit error (pid %d): %w", pid, err)
		} else {
			s.logger.Info().
				Int("pid", pid).
				Msg("Chat server stopped")
		}
	}

	s.chatServerReady = false
	return shutdownErr
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
	// Try to parse as object first {"embedding": [...]}
	// If that fails, try parsing as array directly [...]
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error().
			Err(err).
			Msg("Failed to read embedding response body")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var embedding []float32

	// Try parsing as object first: {"embedding": [...]}
	var objResponse llamaServerEmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &objResponse); err == nil && len(objResponse.Embedding) > 0 {
		embedding = objResponse.Embedding
	} else {
		// Try parsing as array directly: [...]
		if err := json.Unmarshal(bodyBytes, &embedding); err == nil && len(embedding) > 0 {
			// Successfully parsed as flat array
		} else {
			// Try parsing as batch response: [{"index":0,"embedding":[[...]]}]
			var batchResponse []llamaServerBatchEmbeddingResponse
			if err := json.Unmarshal(bodyBytes, &batchResponse); err == nil && len(batchResponse) > 0 {
				// Extract first embedding from batch (flatten nested array)
				if len(batchResponse[0].Embedding) > 0 && len(batchResponse[0].Embedding[0]) > 0 {
					embedding = batchResponse[0].Embedding[0]
				} else {
					return nil, fmt.Errorf("batch embedding response has empty embedding array")
				}
			} else {
				// Preview first 200 bytes of response for debugging
				previewLen := 200
				if len(bodyBytes) < previewLen {
					previewLen = len(bodyBytes)
				}
				s.logger.Error().
					Err(err).
					Str("response_preview", string(bodyBytes[:previewLen])).
					Msg("Failed to parse embedding response in any known format")
				return nil, fmt.Errorf("failed to parse embedding JSON: %w", err)
			}
		}
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding vector is empty")
	}

	s.logger.Debug().
		Int("dimension", len(embedding)).
		Msg("Embedding generated successfully")

	return embedding, nil
}

// Chat generates a completion response based on conversation history
// SECURITY: Uses llama-server on localhost:8087 ONLY - no external network access
func (s *OfflineLLMService) Chat(ctx context.Context, messages []interfaces.Message) (string, error) {
	if s.mockMode {
		return s.generateMockResponse(messages), nil
	}

	if !s.chatServerReady {
		return "", fmt.Errorf("chat server not ready")
	}

	s.logger.Debug().
		Int("message_count", len(messages)).
		Msg("Generating chat completion")

	// Convert messages to llama-server format
	llamaMessages := make([]llamaServerMessage, len(messages))
	for i, msg := range messages {
		llamaMessages[i] = llamaServerMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Create request body
	reqBody := llamaServerChatRequest{
		Messages:    llamaMessages,
		Temperature: 0.8,
		MaxTokens:   512, // Reduced from 2048 to prevent context overflow
		Stream:      false,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP client with localhost-only transport
	// SECURITY: Transport enforces localhost-only connections
	client := &http.Client{
		Timeout: 240 * time.Second, // Extended timeout for longer chat completions (was 120s)
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

	// Make request to local chat server
	req, err := http.NewRequestWithContext(ctx, "POST", s.chatServerURL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error().
			Err(err).
			Msg("Chat completion failed")
		return "", fmt.Errorf("llama-server request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Error().
			Int("status_code", resp.StatusCode).
			Str("response", string(body)).
			Msg("Chat server returned error")
		return "", fmt.Errorf("llama-server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error().
			Err(err).
			Msg("Failed to read chat response body")
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var chatResponse llamaServerChatResponse
	if err := json.Unmarshal(bodyBytes, &chatResponse); err != nil {
		s.logger.Error().
			Err(err).
			Str("response", string(bodyBytes[:min(200, len(bodyBytes))])).
			Msg("Failed to parse chat response")
		return "", fmt.Errorf("failed to parse chat JSON: %w", err)
	}

	if len(chatResponse.Choices) == 0 {
		return "", fmt.Errorf("no choices in chat response")
	}

	response := chatResponse.Choices[0].Message.Content

	s.logger.Debug().
		Int("response_length", len(response)).
		Msg("Chat completion generated")

	return response, nil
}

// HealthCheck verifies the LLM service is operational
// Returns cached health status to avoid expensive checks on every request
// Background goroutine updates cache every 60 seconds
func (s *OfflineLLMService) HealthCheck(ctx context.Context) error {
	// Mock mode always healthy
	if s.mockMode {
		return nil
	}

	// Return cached health status
	s.healthCheckMutex.RLock()
	defer s.healthCheckMutex.RUnlock()

	return s.cachedHealthStatus
}

// refreshHealthCheck performs the actual health check and updates cache
// This is called by the background updater goroutine
func (s *OfflineLLMService) refreshHealthCheck(ctx context.Context) {
	// Verbose-level logging for routine health checks
	s.logger.Trace().Msg("Refreshing health check cache")

	var err error

	// Check llama-server binary exists and is executable
	info, statErr := os.Stat(s.llamaServerPath)
	if statErr != nil {
		err = fmt.Errorf("llama-server binary not accessible: %w", statErr)
	} else if info.IsDir() {
		err = fmt.Errorf("llama-server path is a directory: %s", s.llamaServerPath)
	}

	// Verify models exist (only if binary check passed)
	if err == nil {
		if verifyErr := s.modelManager.VerifyModels(); verifyErr != nil {
			err = fmt.Errorf("model verification failed: %w", verifyErr)
		}
	}

	// Try running llama-server --version to confirm it works (only if previous checks passed)
	if err == nil {
		cmd := exec.CommandContext(ctx, s.llamaServerPath, "--version")
		output, versionErr := cmd.CombinedOutput()
		if versionErr != nil {
			// Only log version failures at verbose level
			s.logger.Debug().
				Err(versionErr).
				Str("output", string(output)).
				Msg("Failed to get llama-server version")
			err = fmt.Errorf("llama-server binary not functional: %w", versionErr)
		} else {
			// Successful health checks at verbose level
			version := strings.TrimSpace(string(output))
			s.logger.Trace().
				Str("version", version).
				Msg("Health check passed")
		}
	}

	// Update cached status
	s.healthCheckMutex.Lock()
	s.cachedHealthStatus = err
	s.healthCheckTime = time.Now()
	s.healthCheckMutex.Unlock()

	// Only log failures at info level for visibility
	if err != nil {
		s.logger.Info().Err(err).Msg("LLM service health check failed")
	}
}

// healthCheckUpdater runs in background and refreshes health check cache periodically
func (s *OfflineLLMService) healthCheckUpdater() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		s.refreshHealthCheck(ctx)
		cancel()
	}
}

// GetMode returns the operational mode (always offline)
func (s *OfflineLLMService) GetMode() interfaces.LLMMode {
	return interfaces.LLMModeOffline
}

// Close releases resources and stops both servers
func (s *OfflineLLMService) Close() error {
	s.logger.Info().Msg("Closing offline LLM service")

	var errors []error

	// Stop embedding server
	if err := s.stopEmbeddingServer(); err != nil {
		s.logger.Error().Err(err).Msg("Error stopping embedding server")
		errors = append(errors, fmt.Errorf("embedding server shutdown: %w", err))
	}

	// Stop chat server
	if err := s.stopChatServer(); err != nil {
		s.logger.Error().Err(err).Msg("Error stopping chat server")
		errors = append(errors, fmt.Errorf("chat server shutdown: %w", err))
	}

	// Belt-and-suspenders: cleanup any remaining llama-server processes
	s.logger.Info().Msg("Performing final llama-server process cleanup")
	if err := s.cleanupOrphanedProcesses(); err != nil {
		s.logger.Warn().Err(err).Msg("Error during final cleanup (non-critical)")
	}

	if len(errors) > 0 {
		s.logger.Error().
			Int("error_count", len(errors)).
			Msg("LLM service closed with errors")
		return fmt.Errorf("llm service shutdown had %d errors: %v", len(errors), errors)
	}

	s.logger.Info().Msg("Offline LLM service closed successfully")
	return nil
}

// SetMockMode enables or disables mock mode for testing without binary
func (s *OfflineLLMService) SetMockMode(enabled bool) {
	s.mockMode = enabled
	if enabled {
		s.logger.Warn().Msg("Mock mode enabled - using fake responses")
	} else {
		s.logger.Info().Msg("Mock mode disabled - using real llama-server")
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

// cleanupOrphanedProcesses finds and kills any orphaned llama-server processes
// This ensures a clean environment before starting new servers
// Excludes processes that are currently being managed by this service
func (s *OfflineLLMService) cleanupOrphanedProcesses() error {
	s.logger.Debug().Msg("Searching for orphaned llama-server processes")

	// Get PIDs of processes we're currently managing (so we don't try to kill them)
	managedPIDs := make(map[int]bool)
	if s.embedServerCmd != nil && s.embedServerCmd.Process != nil {
		managedPIDs[s.embedServerCmd.Process.Pid] = true
	}
	if s.chatServerCmd != nil && s.chatServerCmd.Process != nil {
		managedPIDs[s.chatServerCmd.Process.Pid] = true
	}

	// Platform-specific process detection
	var cmd *exec.Cmd
	var processList string

	// Windows: Use tasklist to find llama-server processes
	if isWindows() {
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq llama-server.exe", "/NH")
		output, err := cmd.Output()
		if err != nil {
			s.logger.Debug().Err(err).Msg("Failed to list processes (non-critical)")
			return nil
		}
		processList = string(output)

		// Parse tasklist output and kill processes
		lines := strings.Split(processList, "\n")
		killedCount := 0
		for _, line := range lines {
			if strings.Contains(line, "llama-server.exe") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					pidStr := fields[1]
					pid, err := strconv.Atoi(pidStr)
					if err != nil {
						continue
					}

					// Skip processes we're currently managing
					if managedPIDs[pid] {
						s.logger.Debug().
							Int("pid", pid).
							Msg("Skipping managed process")
						continue
					}

					s.logger.Warn().
						Int("pid", pid).
						Msg("Found orphaned llama-server process, killing")

					killCmd := exec.Command("taskkill", "/F", "/PID", pidStr)
					if err := killCmd.Run(); err != nil {
						// Exit status 128 means process doesn't exist (already killed)
						if !strings.Contains(err.Error(), "exit status 128") {
							s.logger.Debug().
								Err(err).
								Int("pid", pid).
								Msg("Failed to kill orphaned process (may have already exited)")
						}
					} else {
						killedCount++
					}
				}
			}
		}

		if killedCount > 0 {
			s.logger.Info().
				Int("count", killedCount).
				Msg("Cleaned up orphaned llama-server processes")
		} else {
			s.logger.Debug().Msg("No orphaned llama-server processes found")
		}
	} else {
		// Unix-like: Use pgrep to find PIDs, then kill only orphaned ones
		cmd = exec.Command("pgrep", "llama-server")
		output, err := cmd.Output()
		if err != nil {
			// pgrep returns error if no processes found (exit code 1)
			s.logger.Debug().Msg("No orphaned llama-server processes found")
			return nil
		}

		// Parse PIDs and kill orphaned ones
		pidStrs := strings.Fields(string(output))
		killedCount := 0
		for _, pidStr := range pidStrs {
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}

			// Skip processes we're currently managing
			if managedPIDs[pid] {
				s.logger.Debug().
					Int("pid", pid).
					Msg("Skipping managed process")
				continue
			}

			s.logger.Warn().
				Int("pid", pid).
				Msg("Found orphaned llama-server process, killing")

			killCmd := exec.Command("kill", "-9", strconv.Itoa(pid))
			if err := killCmd.Run(); err != nil {
				s.logger.Debug().
					Err(err).
					Int("pid", pid).
					Msg("Failed to kill orphaned process (may have already exited)")
			} else {
				killedCount++
			}
		}

		if killedCount > 0 {
			s.logger.Info().
				Int("count", killedCount).
				Msg("Cleaned up orphaned llama-server processes")
		} else {
			s.logger.Debug().Msg("No orphaned llama-server processes found")
		}
	}

	return nil
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

// isProcessExitError returns true if the error is a normal process exit (not a real error)
func isProcessExitError(err error) bool {
	if err == nil {
		return false
	}
	// Check for "signal: killed" or exit code 0
	errStr := err.Error()
	return strings.Contains(errStr, "signal: killed") ||
		strings.Contains(errStr, "exit status 0")
}
