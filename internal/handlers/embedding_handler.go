package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// EmbeddingHandler handles embedding-related HTTP requests
type EmbeddingHandler struct {
	embeddingService interfaces.EmbeddingService
	documentStorage  interfaces.DocumentStorage
	logger           arbor.ILogger
}

// NewEmbeddingHandler creates a new embedding handler
func NewEmbeddingHandler(
	embeddingService interfaces.EmbeddingService,
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
) *EmbeddingHandler {
	return &EmbeddingHandler{
		embeddingService: embeddingService,
		documentStorage:  documentStorage,
		logger:           logger,
	}
}

// EmbedRequest represents the request payload for embedding generation
type EmbedRequest struct {
	Text string `json:"text"`
}

// EmbedResponse represents the response payload for embedding generation
type EmbedResponse struct {
	Embedding []float32 `json:"embedding"`
	Dimension int       `json:"dimension"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// GenerateEmbeddingHandler handles POST /api/embeddings/generate requests
func (h *EmbeddingHandler) GenerateEmbeddingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EmbedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode embedding request")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(EmbedResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	if req.Text == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(EmbedResponse{
			Success: false,
			Error:   "Text field is required",
		})
		return
	}

	h.logger.Info().Int("text_length", len(req.Text)).Msg("Generating embedding")

	ctx := context.Background()
	embedding, err := h.embeddingService.GenerateEmbedding(ctx, req.Text)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to generate embedding")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(EmbedResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.logger.Info().Int("dimension", len(embedding)).Msg("Embedding generated successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EmbedResponse{
		Embedding: embedding,
		Dimension: len(embedding),
		Success:   true,
	})
}

// ClearResponse represents the response for clearing embeddings
type ClearResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	DocumentsAffected int    `json:"documents_affected"`
	Error             string `json:"error,omitempty"`
}

// ClearEmbeddingsHandler handles DELETE /api/embeddings requests
func (h *EmbeddingHandler) ClearEmbeddingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Clearing all documents from collection")

	// Get count before deletion
	count, err := h.documentStorage.CountDocuments()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to count documents")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ClearResponse{
			Success: false,
			Error:   "Failed to count documents",
		})
		return
	}

	// Delete all documents
	if err := h.documentStorage.ClearAll(); err != nil {
		h.logger.Error().Err(err).Msg("Failed to clear documents")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ClearResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.logger.Info().Int("count", count).Msg("All documents cleared from collection")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ClearResponse{
		Success:           true,
		Message:           "All documents have been deleted from the collection",
		DocumentsAffected: count,
	})
}
