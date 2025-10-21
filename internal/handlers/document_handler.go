// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:57:01 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

type DocumentHandler struct {
	documentService interfaces.DocumentService
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
}

func NewDocumentHandler(documentService interfaces.DocumentService, documentStorage interfaces.DocumentStorage, logger arbor.ILogger) *DocumentHandler {
	return &DocumentHandler{
		documentService: documentService,
		documentStorage: documentStorage,
		logger:          logger,
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
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	sourceType := r.URL.Query().Get("sourceType")

	// Set defaults
	limit := 50
	offset := 0

	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}

	opts := &interfaces.ListOptions{
		SourceType: sourceType,
		Limit:      limit,
		Offset:     offset,
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
		"documents":   documents,
		"count":       len(documents), // Number of documents in current response
		"total_count": totalCount,     // Total number of documents in database
		"limit":       limit,
		"offset":      offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

// DeleteDocumentHandler handles DELETE /api/documents/{id}
// Deletes a document from the database
func (h *DocumentHandler) DeleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract document ID from path: /api/documents/{id}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}
	docID := pathParts[2]

	if docID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	err := h.documentStorage.DeleteDocument(docID)
	if err != nil {
		h.logger.Error().Err(err).Str("doc_id", docID).Msg("Failed to delete document")
		http.Error(w, "Failed to delete document", http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("doc_id", docID).Msg("Document deleted")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"doc_id":  docID,
		"message": "Document deleted successfully",
	})
}
