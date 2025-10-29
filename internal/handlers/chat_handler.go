package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
	if !RequireMethod(w, r, http.MethodPost) {
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
		Msg("Processing chat request (agent mode)")

	// Track thinking time
	startTime := time.Now()

	// Create context with 5-minute timeout to match client timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	response, err := h.chatService.Chat(ctx, &req)

	thinkingTime := time.Since(startTime)

	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to generate chat response")

		// Check if error was due to timeout
		if ctx.Err() == context.DeadlineExceeded {
			h.logger.Warn().
				Dur("thinking_time", thinkingTime).
				Msg("Chat request exceeded 5-minute timeout")
			w.WriteHeader(http.StatusRequestTimeout)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Request timed out after 5 minutes. The LLM may need more time to process your request.",
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to generate response: " + err.Error(),
		})
		return
	}

	// Build technical metadata
	docCount := len(response.ContextDocs)
	references := []string{}
	for _, doc := range response.ContextDocs {
		ref := fmt.Sprintf("%s", doc.SourceType)
		if doc.Title != "" {
			ref = fmt.Sprintf("%s: %s", doc.SourceType, doc.Title)
		}
		if doc.URL != "" {
			ref = fmt.Sprintf("%s (%s)", ref, doc.URL)
		}
		references = append(references, ref)
	}

	metadata := map[string]interface{}{
		"document_count": docCount,
		"references":     references,
		"thinking_time":  fmt.Sprintf("%.2fs", thinkingTime.Seconds()),
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
		"metadata":     metadata,
	})
}

// HealthHandler handles GET /api/chat/health requests
// Returns detailed service status including LLM mode and server states
func (h *ChatHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
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
