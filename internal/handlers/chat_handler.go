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
	h.logger.Info().
		Int("message_length", len(req.Message)).
		Str("has_history", fmt.Sprintf("%v", hasHistory)).
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
func (h *ChatHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	err := h.chatService.HealthCheck(ctx)

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		h.logger.Warn().Err(err).Msg("Chat service health check failed")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"healthy": false,
			"mode":    h.chatService.GetMode(),
			"error":   err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"healthy": true,
		"mode":    h.chatService.GetMode(),
	})
}
