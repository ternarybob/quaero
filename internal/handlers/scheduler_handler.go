package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ternarybob/quaero/internal/interfaces"
)

// SchedulerHandler handles scheduler-related endpoints
type SchedulerHandler struct {
	schedulerService interfaces.SchedulerService
	documentStorage  interfaces.DocumentStorage
}

// NewSchedulerHandler creates a new scheduler handler
func NewSchedulerHandler(
	schedulerService interfaces.SchedulerService,
	documentStorage interfaces.DocumentStorage,
) *SchedulerHandler {
	return &SchedulerHandler{
		schedulerService: schedulerService,
		documentStorage:  documentStorage,
	}
}

// TriggerCollectionHandler manually triggers collection
func (h *SchedulerHandler) TriggerCollectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.schedulerService.TriggerCollectionNow(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Collection triggered successfully",
	})
}

// ForceSyncDocumentHandler sets force_sync_pending for a document
func (h *SchedulerHandler) ForceSyncDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docID := r.URL.Query().Get("id")
	if docID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	if err := h.documentStorage.SetForceSyncPending(docID, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Document marked for force sync",
		"doc_id":  docID,
	})
}
