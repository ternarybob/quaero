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
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
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

	// Parse query params
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(query.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// Parse tags (comma separated)
	var tags []string
	tagsParam := query.Get("tags")
	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
	}

	opts := &interfaces.ListOptions{
		SourceType:    query.Get("source_type"),
		Tags:          tags,
		Limit:         limit,
		Offset:        offset,
		OrderBy:       query.Get("order_by"),
		OrderDir:      query.Get("order_dir"),
		CreatedAfter:  nil,
		CreatedBefore: nil,
	}

	if ca := query.Get("created_after"); ca != "" {
		opts.CreatedAfter = &ca
	}
	if cb := query.Get("created_before"); cb != "" {
		opts.CreatedBefore = &cb
	}

	docs, err := h.documentService.List(r.Context(), opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list documents")
		http.Error(w, "Failed to list documents", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	// Note: This is an approximation if filters are applied, as Count() only filters by source type
	// Ideally we'd have a Count(opts) method
	totalCount, err := h.documentService.Count(r.Context(), opts.SourceType)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to count documents")
		// Don't fail the request, just return 0 count
		totalCount = 0
	}

	response := map[string]interface{}{
		"documents":   docs,
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDocumentHandler handles GET /api/documents/{id}
// Returns a single document by ID
func (h *DocumentHandler) GetDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Extract ID from URL path
	// Path is /api/documents/{id}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid document ID", http.StatusBadRequest)
		return
	}
	id := parts[len(parts)-1]

	if id == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	doc, err := h.documentService.GetDocument(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to get document")
		http.Error(w, "Failed to get document", http.StatusInternalServerError)
		return
	}

	if doc == nil {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
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
	h.logger.Debug().Str("doc_id", docID).Msg("Reprocess endpoint called (no-op after Phase 5 embedding removal)")

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

	h.logger.Debug().Str("doc_id", docID).Msg("Document deleted")

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

	// Extract optional tags
	var tags []string
	if tagsRaw, ok := docReq["tags"].([]interface{}); ok {
		for _, t := range tagsRaw {
			if tagStr, ok := t.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	// Create document model
	doc := &models.Document{
		ID:              id,
		SourceType:      sourceType,
		SourceID:        sourceID,
		Title:           title,
		ContentMarkdown: contentMarkdown,
		URL:             url,
		Metadata:        metadata,
		Tags:            tags,
	}

	// Save document
	if err := h.documentService.SaveDocument(ctx, doc); err != nil {
		h.logger.Error().Err(err).Str("doc_id", id).Msg("Failed to save document")
		WriteError(w, http.StatusInternalServerError, "Failed to save document")
		return
	}

	h.logger.Debug().Str("doc_id", id).Str("source_type", sourceType).Msg("Document created")

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

	h.logger.Debug().Msg("FTS5 index rebuild requested")

	// Rebuild the FTS5 index
	err := h.documentStorage.RebuildFTS5Index()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to rebuild FTS5 index")
		WriteError(w, http.StatusInternalServerError, "Failed to rebuild search index")
		return
	}

	h.logger.Debug().Msg("FTS5 index rebuilt successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Search index rebuilt successfully",
	})
}

// CaptureRequest represents a page capture request from the Chrome extension
type CaptureRequest struct {
	URL         string `json:"url"`
	HTML        string `json:"html"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
}

// CaptureHandler handles POST /api/documents/capture
// Receives HTML content directly from Chrome extension and saves as document
func (h *DocumentHandler) CaptureHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	ctx := r.Context()

	// Parse request body
	var req CaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode capture request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.URL == "" {
		WriteError(w, http.StatusBadRequest, "URL is required")
		return
	}
	if req.HTML == "" {
		WriteError(w, http.StatusBadRequest, "HTML content is required")
		return
	}

	h.logger.Debug().
		Str("url", req.URL).
		Int("html_size", len(req.HTML)).
		Str("title", req.Title).
		Msg("Processing page capture from extension")

	// Process HTML content using ContentProcessor
	contentProcessor := crawler.NewContentProcessor(h.logger)
	processedContent, err := contentProcessor.ProcessHTML(req.HTML, req.URL)
	if err != nil {
		h.logger.Error().Err(err).Str("url", req.URL).Msg("Failed to process HTML content")
		WriteError(w, http.StatusInternalServerError, "Failed to process HTML content")
		return
	}

	// Generate document ID
	docID := uuid.New().String()

	// Use title from request or extracted content
	title := req.Title
	if title == "" {
		title = processedContent.Title
	}
	if title == "" {
		title = req.URL
	}

	// Build metadata
	metadata := map[string]interface{}{
		"capture_source":    "chrome_extension",
		"capture_time":      time.Now().Format(time.RFC3339),
		"original_url":      req.URL,
		"content_size":      len(processedContent.Markdown),
		"links_found":       len(processedContent.Links),
		"description":       req.Description,
		"request_timestamp": req.Timestamp,
	}

	// Create document
	doc := &models.Document{
		ID:              docID,
		SourceType:      "web",
		SourceID:        "chrome-extension-capture",
		Title:           title,
		ContentMarkdown: processedContent.Markdown,
		URL:             req.URL,
		Metadata:        metadata,
		Tags:            []string{"captured", "chrome-extension"},
	}

	// Save document
	if err := h.documentService.SaveDocument(ctx, doc); err != nil {
		h.logger.Error().Err(err).Str("doc_id", docID).Msg("Failed to save captured document")
		WriteError(w, http.StatusInternalServerError, "Failed to save document")
		return
	}

	h.logger.Info().
		Str("doc_id", docID).
		Str("url", req.URL).
		Str("title", title).
		Int("content_size", len(processedContent.Markdown)).
		Msg("Page captured and saved from Chrome extension")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"document_id":  docID,
		"title":        title,
		"url":          req.URL,
		"content_size": len(processedContent.Markdown),
		"message":      "Page captured successfully",
	})
}
