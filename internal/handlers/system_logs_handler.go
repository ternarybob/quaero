package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/arbor/services/logviewer"
)

type SystemLogsHandler struct {
	service *logviewer.Service
	logger  arbor.ILogger
}

func NewSystemLogsHandler(service *logviewer.Service, logger arbor.ILogger) *SystemLogsHandler {
	return &SystemLogsHandler{
		service: service,
		logger:  logger,
	}
}

// ListLogFilesHandler handles GET /api/system/logs/files
func (h *SystemLogsHandler) ListLogFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files, err := h.service.ListLogFiles()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list log files")
		http.Error(w, "Failed to list log files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// GetLogContentHandler handles GET /api/system/logs/content
func (h *SystemLogsHandler) GetLogContentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 1000 // Default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	levelsStr := r.URL.Query().Get("levels")
	var levels []string
	if levelsStr != "" {
		levels = strings.Split(levelsStr, ",")
	}

	// Use arbor service to get log content with correct directory path
	entries, err := h.service.GetLogContent(filename, limit, levels)
	if err != nil {
		h.logger.Error().Err(err).Str("filename", filename).Msg("Failed to get log content")
		http.Error(w, "Failed to get log content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
