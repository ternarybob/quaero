package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// TimingHandler handles timing record HTTP requests
type TimingHandler struct {
	kvStorage interfaces.KeyValueStorage
	logger    arbor.ILogger
}

// NewTimingHandler creates a new timing handler
func NewTimingHandler(kvStorage interfaces.KeyValueStorage, logger arbor.ILogger) *TimingHandler {
	return &TimingHandler{
		kvStorage: kvStorage,
		logger:    logger,
	}
}

// HandleGetTimingRecords handles GET /api/timing - lists timing records with filters
func (h *TimingHandler) HandleGetTimingRecords(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	workerType := query.Get("worker_type")
	jobID := query.Get("job_id")
	status := query.Get("status")
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	limit := 100
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get all timing records from KV storage
	pairs, err := h.kvStorage.ListByPrefix(r.Context(), "timing:")
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list timing records")
		WriteError(w, http.StatusInternalServerError, "Failed to list timing records")
		return
	}

	// Parse and filter records
	var records []*models.TimingRecord
	for _, pair := range pairs {
		var record models.TimingRecord
		if err := json.Unmarshal([]byte(pair.Value), &record); err != nil {
			h.logger.Warn().Err(err).Str("key", pair.Key).Msg("Failed to parse timing record")
			continue
		}

		// Apply filters
		if workerType != "" && record.WorkerType != workerType {
			continue
		}
		if jobID != "" && record.JobID != jobID {
			continue
		}
		if status != "" && record.Status != status {
			continue
		}

		records = append(records, &record)
	}

	// Sort by completed_at DESC
	sort.Slice(records, func(i, j int) bool {
		return records[i].CompletedAt.After(records[j].CompletedAt)
	})

	// Apply pagination
	total := len(records)
	if offset >= len(records) {
		records = []*models.TimingRecord{}
	} else {
		end := offset + limit
		if end > len(records) {
			end = len(records)
		}
		records = records[offset:end]
	}

	response := map[string]interface{}{
		"records": records,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}

	WriteJSON(w, http.StatusOK, response)
}

// HandleGetTimingStats handles GET /api/timing/stats - returns aggregated timing statistics
func (h *TimingHandler) HandleGetTimingStats(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Get all timing records
	pairs, err := h.kvStorage.ListByPrefix(r.Context(), "timing:")
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list timing records for stats")
		WriteError(w, http.StatusInternalServerError, "Failed to get timing stats")
		return
	}

	// Parse records and aggregate by worker type
	type workerStats struct {
		Count       int     `json:"count"`
		AvgMs       int64   `json:"avg_ms"`
		MinMs       int64   `json:"min_ms"`
		MaxMs       int64   `json:"max_ms"`
		SuccessRate float64 `json:"success_rate"`
		totalMs     int64
		successCnt  int
	}

	byWorker := make(map[string]*workerStats)
	var minTime, maxTime time.Time
	totalCount := 0

	for _, pair := range pairs {
		var record models.TimingRecord
		if err := json.Unmarshal([]byte(pair.Value), &record); err != nil {
			continue
		}

		totalCount++

		// Update time range
		if minTime.IsZero() || record.StartedAt.Before(minTime) {
			minTime = record.StartedAt
		}
		if maxTime.IsZero() || record.CompletedAt.After(maxTime) {
			maxTime = record.CompletedAt
		}

		// Aggregate by worker type
		stats, ok := byWorker[record.WorkerType]
		if !ok {
			stats = &workerStats{
				MinMs: record.TotalMs,
				MaxMs: record.TotalMs,
			}
			byWorker[record.WorkerType] = stats
		}

		stats.Count++
		stats.totalMs += record.TotalMs
		if record.TotalMs < stats.MinMs {
			stats.MinMs = record.TotalMs
		}
		if record.TotalMs > stats.MaxMs {
			stats.MaxMs = record.TotalMs
		}
		if record.Status == "success" {
			stats.successCnt++
		}
	}

	// Calculate averages and success rates
	byWorkerResponse := make(map[string]interface{})
	for workerType, stats := range byWorker {
		if stats.Count > 0 {
			stats.AvgMs = stats.totalMs / int64(stats.Count)
			stats.SuccessRate = float64(stats.successCnt) / float64(stats.Count)
		}
		byWorkerResponse[workerType] = map[string]interface{}{
			"count":        stats.Count,
			"avg_ms":       stats.AvgMs,
			"min_ms":       stats.MinMs,
			"max_ms":       stats.MaxMs,
			"success_rate": stats.SuccessRate,
		}
	}

	response := map[string]interface{}{
		"by_worker":   byWorkerResponse,
		"total_count": totalCount,
	}

	if !minTime.IsZero() {
		response["time_range"] = map[string]string{
			"from": minTime.Format(time.RFC3339),
			"to":   maxTime.Format(time.RFC3339),
		}
	}

	WriteJSON(w, http.StatusOK, response)
}
