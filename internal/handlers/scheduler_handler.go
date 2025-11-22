package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ternarybob/quaero/internal/interfaces"
)

// SchedulerHandler handles scheduler-related endpoints
type SchedulerHandler struct {
	schedulerService interfaces.SchedulerService
}

// NewSchedulerHandler creates a new scheduler handler
func NewSchedulerHandler(
	schedulerService interfaces.SchedulerService,
) *SchedulerHandler {
	return &SchedulerHandler{
		schedulerService: schedulerService,
	}
}

// TriggerCollectionHandler manually triggers collection
func (h *SchedulerHandler) TriggerCollectionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
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

