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
	"github.com/ternarybob/quaero/internal/models"
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
	if !RequireMethod(w, r, http.MethodGet) {
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

// TagsHandler returns all unique tags across all documents
func (h *DocumentHandler) TagsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	tags, err := h.documentStorage.GetAllTags()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get document tags")
		http.Error(w, "Failed to get tags", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tags": tags,
	})
}

// ListHandler returns paginated list of documents with filtering
func (h *DocumentHandler) ListHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	ctx := r.Context()

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	sourceType := r.URL.Query().Get("sourceType")
	tagsParam := r.URL.Query().Get("tags")

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

	// Parse tags (comma-separated list)
	var tags []string
	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
		// Trim whitespace from each tag
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	opts := &interfaces.ListOptions{
		SourceType: sourceType,
		Tags:       tags,
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
	if !RequireMethod(w, r, http.MethodPost) {
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
	if !RequireMethod(w, r, http.MethodDelete) {
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

// DeleteAllDocumentsHandler handles DELETE /api/documents/clear-all
// Deletes ALL documents from the database (danger zone operation)
func (h *DocumentHandler) DeleteAllDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodDelete) {
		return
	}

	h.logger.Warn().Msg("Delete all documents requested (danger zone operation)")

	// Get count before deletion for response
	ctx := r.Context()
	stats, err := h.documentService.GetStats(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get document stats before deletion")
		WriteError(w, http.StatusInternalServerError, "Failed to get document count")
		return
	}
	documentsAffected := stats.TotalDocuments

	// Clear all documents
	err = h.documentStorage.ClearAll()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to clear all documents")
		WriteError(w, http.StatusInternalServerError, "Failed to clear all documents")
		return
	}

	h.logger.Warn().
		Int("documents_deleted", documentsAffected).
		Msg("All documents deleted successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":            "All documents deleted successfully",
		"documents_affected": documentsAffected,
	})
}

// CreateDocumentHandler handles POST /api/documents
// Creates a new document in the database
func (h *DocumentHandler) CreateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	ctx := r.Context()

	// Parse request body
	var docReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&docReq); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode document request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Extract required fields
	id, _ := docReq["id"].(string)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Document ID is required")
		return
	}

	sourceType, _ := docReq["source_type"].(string)
	if sourceType == "" {
		WriteError(w, http.StatusBadRequest, "Source type is required")
		return
	}

	title, _ := docReq["title"].(string)
	contentMarkdown, _ := docReq["content_markdown"].(string)
	url, _ := docReq["url"].(string)
	sourceID, _ := docReq["source_id"].(string)

	// Extract optional metadata
	metadata, _ := docReq["metadata"].(map[string]interface{})

	// Create document model
	doc := &models.Document{
		ID:              id,
		SourceType:      sourceType,
		SourceID:        sourceID,
		Title:           title,
		ContentMarkdown: contentMarkdown,
		URL:             url,
		Metadata:        metadata,
	}

	// Save document
	if err := h.documentService.SaveDocument(ctx, doc); err != nil {
		h.logger.Error().Err(err).Str("doc_id", id).Msg("Failed to save document")
		WriteError(w, http.StatusInternalServerError, "Failed to save document")
		return
	}

	h.logger.Info().Str("doc_id", id).Str("source_type", sourceType).Msg("Document created")

	// Return created document
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          doc.ID,
		"source_type": doc.SourceType,
		"title":       doc.Title,
	})
}

// RebuildIndexHandler handles POST /api/documents/rebuild-index
// Rebuilds the FTS5 full-text search index
func (h *DocumentHandler) RebuildIndexHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.logger.Info().Msg("FTS5 index rebuild requested")

	// Rebuild the FTS5 index
	err := h.documentStorage.RebuildFTS5Index()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to rebuild FTS5 index")
		WriteError(w, http.StatusInternalServerError, "Failed to rebuild search index")
		return
	}

	h.logger.Info().Msg("FTS5 index rebuilt successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Search index rebuilt successfully",
	})
}
