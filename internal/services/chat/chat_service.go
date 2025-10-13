package chat

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/mcp"
)

// ChatService implements agent-based chat functionality
type ChatService struct {
	llmService interfaces.LLMService
	logger     arbor.ILogger
	toolRouter *mcp.ToolRouter
	agentLoop  *AgentLoop
}

// NewChatService creates a new agent-based chat service
func NewChatService(
	llmService interfaces.LLMService,
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
) *ChatService {
	// Initialize agent components
	toolRouter := mcp.NewToolRouter(documentStorage, searchService, logger)
	agentLoop := NewAgentLoop(toolRouter, llmService, logger, DefaultAgentConfig())

	return &ChatService{
		llmService: llmService,
		logger:     logger,
		toolRouter: toolRouter,
		agentLoop:  agentLoop,
	}
}

// Chat implements the ChatService interface using agent mode
func (s *ChatService) Chat(ctx context.Context, req *interfaces.ChatRequest) (*interfaces.ChatResponse, error) {
	s.logger.Info().
		Str("message", req.Message).
		Msg("Processing chat request (agent mode)")

	// Call agent loop
	answer, err := s.agentLoop.Execute(ctx, req.Message, nil)
	if err != nil {
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	// Return as ChatResponse
	return &interfaces.ChatResponse{
		Message: answer,
		Model:   "agent-mode",
		Mode:    s.llmService.GetMode(),
	}, nil
}

// GetMode returns the current LLM mode
func (s *ChatService) GetMode() interfaces.LLMMode {
	return s.llmService.GetMode()
}

// HealthCheck verifies the chat service is operational
func (s *ChatService) HealthCheck(ctx context.Context) error {
	// Check LLM service
	if err := s.llmService.HealthCheck(ctx); err != nil {
		return fmt.Errorf("LLM service unhealthy: %w", err)
	}

	return nil
}

// GetServiceStatus returns detailed service status information
func (s *ChatService) GetServiceStatus(ctx context.Context) map[string]interface{} {
	status := make(map[string]interface{})

	// Get LLM mode
	mode := string(s.llmService.GetMode())
	status["mode"] = mode

	// Default values for mock mode
	status["embed_server"] = "N/A (mock mode)"
	status["chat_server"] = "N/A (mock mode)"
	status["model_loaded"] = true
	status["last_check_time"] = "N/A"

	// For offline mode, check if servers are running
	if mode == "offline" {
		// Check embed server (port 8086)
		embedStatus := checkServerHealth("http://127.0.0.1:8086/health")
		if embedStatus {
			status["embed_server"] = "active"
		} else {
			status["embed_server"] = "inactive"
		}

		// Check chat server (port 8087)
		chatStatus := checkServerHealth("http://127.0.0.1:8087/health")
		if chatStatus {
			status["chat_server"] = "active"
		} else {
			status["chat_server"] = "inactive"
		}

		status["model_loaded"] = embedStatus && chatStatus
	}

	return status
}

// checkServerHealth checks if a server port is listening
func checkServerHealth(url string) bool {
	// Extract host:port from URL
	var address string
	if strings.HasPrefix(url, "http://127.0.0.1:8086") {
		address = "127.0.0.1:8086"
	} else if strings.HasPrefix(url, "http://127.0.0.1:8087") {
		address = "127.0.0.1:8087"
	} else {
		return false
	}

	// Simple TCP connection check with 500ms timeout
	conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
