package chat

import (
	"context"
	"fmt"
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

	// Get LLM mode from service
	mode := string(s.llmService.GetMode())
	status["mode"] = mode

	// Perform health check on LLM service
	healthErr := s.llmService.HealthCheck(ctx)
	status["healthy"] = healthErr == nil

	// Set service type to indicate Google ADK provider
	status["service_type"] = "google_adk"

	// Set timestamp of status check
	status["last_check_time"] = time.Now().Format(time.RFC3339)

	// Add offline-compatible fields for UI compatibility
	// These provide sensible defaults when mode isn't offline
	status["embed_server"] = "N/A (cloud mode)"
	status["chat_server"] = "N/A (cloud mode)"
	status["model_loaded"] = healthErr == nil // True if healthy, false otherwise

	// Log health check result at debug level
	if healthErr != nil {
		s.logger.Debug().
			Err(healthErr).
			Str("mode", mode).
			Msg("LLM service health check failed")
	} else {
		s.logger.Debug().
			Str("mode", mode).
			Bool("healthy", true).
			Msg("LLM service health check passed")
	}

	return status
}
