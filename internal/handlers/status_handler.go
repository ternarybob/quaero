package handlers

import (
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/services/status"
)

// StatusHandler handles HTTP requests for application status
type StatusHandler struct {
	statusService *status.Service
	logger        arbor.ILogger
}

// NewStatusHandler creates a new StatusHandler
func NewStatusHandler(statusService *status.Service, logger arbor.ILogger) *StatusHandler {
	return &StatusHandler{
		statusService: statusService,
		logger:        logger,
	}
}

// GetStatusHandler handles GET /api/status
func (h *StatusHandler) GetStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	status := h.statusService.GetStatus()
	WriteJSON(w, http.StatusOK, status)
}
