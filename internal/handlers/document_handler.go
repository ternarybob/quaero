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

	response := map[string]interface{}{
		"documents": documents,
		"count":     len(documents),
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

	h.logger.Info().Str("doc_id", docID).Msg("Marking document for reprocessing")

	// Mark document for force embed (re-vectorization)
	if err := h.documentStorage.SetForceEmbedPending(docID, true); err != nil {
		h.logger.Error().Err(err).Str("doc_id", docID).Msg("Failed to mark document for reprocessing")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("doc_id", docID).Msg("Document marked for reprocessing")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Document marked for reprocessing",
		"doc_id":  docID,
	})
}
