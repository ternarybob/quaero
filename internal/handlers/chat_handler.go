package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// ChatHandler handles chat-related HTTP requests
type ChatHandler struct {
	chatService interfaces.ChatService
	logger      arbor.ILogger
}

// NewChatHandler creates a new chat handler
func NewChatHandler(
	chatService interfaces.ChatService,
	logger arbor.ILogger,
) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		logger:      logger,
	}
}

// ChatHandler handles POST /api/chat requests
func (h *ChatHandler) ChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req interfaces.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode chat request")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Message field is required",
		})
		return
	}

	hasHistory := len(req.History) > 0
	ragConfigPresent := req.RAGConfig != nil
	ragEnabled := false
	if req.RAGConfig != nil {
		ragEnabled = req.RAGConfig.Enabled
	}
	h.logger.Info().
		Int("message_length", len(req.Message)).
		Str("has_history", fmt.Sprintf("%v", hasHistory)).
		Str("rag_config_present", fmt.Sprintf("%v", ragConfigPresent)).
		Str("rag_enabled", fmt.Sprintf("%v", ragEnabled)).
		Msg("Processing chat request")

	ctx := context.Background()
	response, err := h.chatService.Chat(ctx, &req)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to generate chat response")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to generate response: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"message":      response.Message,
		"context_docs": response.ContextDocs,
		"token_usage":  response.TokenUsage,
		"model":        response.Model,
		"mode":         response.Mode,
	})
}

// HealthHandler handles GET /api/chat/health requests
// Returns detailed service status including LLM mode and server states
func (h *ChatHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()

	// Perform single health check
	err := h.chatService.HealthCheck(ctx)
	healthy := err == nil

	// Get detailed status (does NOT perform additional health checks)
	status := h.chatService.GetServiceStatus(ctx)

	w.Header().Set("Content-Type", "application/json")

	// Build response
	response := map[string]interface{}{
		"healthy":      healthy,
		"mode":         status["mode"],
		"embed_server": status["embed_server"],
		"chat_server":  status["chat_server"],
		"model_loaded": status["model_loaded"],
	}

	if err != nil {
		h.logger.Warn().Err(err).Msg("Chat service health check failed")
		response["error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(response)
}
