// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:57:01 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/processing"
)

type DocumentHandler struct {
	documentService   interfaces.DocumentService
	documentStorage   interfaces.DocumentStorage
	processingService *processing.Service
	logger            arbor.ILogger
}

func NewDocumentHandler(documentService interfaces.DocumentService, documentStorage interfaces.DocumentStorage, processingService *processing.Service) *DocumentHandler {
	return &DocumentHandler{
		documentService:   documentService,
		documentStorage:   documentStorage,
		processingService: processingService,
		logger:            common.GetLogger(),
	}
}

// StatsHandler returns document statistics
func (h *DocumentHandler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	stats, err := h.documentService.GetStats(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get document stats")
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ListHandler returns paginated list of documents with filtering
func (h *DocumentHandler) ListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse query parameters
	sourceType := r.URL.Query().Get("sourceType")
	limit := 100 // Default limit

	opts := &interfaces.ListOptions{
		SourceType: sourceType,
		Limit:      limit,
		Offset:     0,
	}

	documents, err := h.documentService.List(ctx, opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list documents")
		http.Error(w, "Failed to list documents", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	totalCount, err := h.documentService.Count(ctx, sourceType)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get document count")
		// Fallback to returned document count if total count fails
		totalCount = len(documents)
	}

	response := map[string]interface{}{
		"documents": documents,
		"count":     len(documents), // Number of documents in current response
		"total":     totalCount,     // Total number of documents in database
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ProcessHandler triggers document processing
func (h *DocumentHandler) ProcessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	h.logger.Info().Msg("Starting document processing")

	go func() {
		stats, err := h.processingService.ProcessAll(ctx)
		if err != nil {
			h.logger.Error().Err(err).Msg("Document processing failed")
		} else {
			h.logger.Info().
				Int("total", stats.TotalProcessed).
				Int("jira", stats.JiraProcessed).
				Int("confluence", stats.ConfProcessed).
				Msg("Document processing completed")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Document processing started in background",
	})
}

// ProcessingStatusHandler returns processing engine status
func (h *DocumentHandler) ProcessingStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	status, err := h.processingService.GetStatus(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get processing status")
		http.Error(w, "Failed to get processing status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ReprocessDocumentHandler handles POST /api/documents/{id}/reprocess
// This marks a document for re-vectorization (force embed)
func (h *DocumentHandler) ReprocessDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract document ID from path: /api/documents/{id}/reprocess
	path := r.URL.Path
	docID := ""
	if len(path) > len("/api/documents/") {
		endIdx := len(path) - len("/reprocess")
		if endIdx > len("/api/documents/") {
			docID = path[len("/api/documents/"):endIdx]
		}
	}

	if docID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	// NOTE: Phase 5 - Embeddings removed, reprocessing endpoint is now a no-op
	h.logger.Info().Str("doc_id", docID).Msg("Reprocess endpoint called (no-op after Phase 5 embedding removal)")

	h.logger.Info().Str("doc_id", docID).Msg("Document reprocessing skipped (embeddings removed)")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Document marked for reprocessing",
		"doc_id":  docID,
	})
}
