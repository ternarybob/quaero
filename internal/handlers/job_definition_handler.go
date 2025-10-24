package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/jobs"
	"github.com/ternarybob/quaero/internal/services/sources"
)

var ErrJobDefinitionNotFound = errors.New("job definition not found")

// JobDefinitionHandler handles HTTP requests for job definition management
type JobDefinitionHandler struct {
	jobDefStorage interfaces.JobDefinitionStorage
	jobExecutor   *jobs.JobExecutor
	sourceService *sources.Service
	jobRegistry   *jobs.JobTypeRegistry
	queueManager  interfaces.QueueManager
	logger        arbor.ILogger
}

// NewJobDefinitionHandler creates a new job definition handler
func NewJobDefinitionHandler(
	jobDefStorage interfaces.JobDefinitionStorage,
	jobExecutor *jobs.JobExecutor,
	sourceService *sources.Service,
	jobRegistry *jobs.JobTypeRegistry,
	queueManager interfaces.QueueManager,
	logger arbor.ILogger,
) *JobDefinitionHandler {
	if jobDefStorage == nil {
		panic("jobDefStorage cannot be nil")
	}
	if jobExecutor == nil {
		panic("jobExecutor cannot be nil")
	}
	if sourceService == nil {
		panic("sourceService cannot be nil")
	}
	if jobRegistry == nil {
		panic("jobRegistry cannot be nil")
	}
	if queueManager == nil {
		panic("queueManager cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}

	logger.Info().Msg("Job definition handler initialized")

	return &JobDefinitionHandler{
		jobDefStorage: jobDefStorage,
		jobExecutor:   jobExecutor,
		sourceService: sourceService,
		jobRegistry:   jobRegistry,
		queueManager:  queueManager,
		logger:        logger,
	}
}

// CreateJobDefinitionHandler handles POST /api/job-definitions
func (h *JobDefinitionHandler) CreateJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	var jobDef models.JobDefinition
	if err := json.NewDecoder(r.Body).Decode(&jobDef); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode job definition")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if jobDef.ID == "" {
		WriteError(w, http.StatusBadRequest, "Job definition ID is required")
		return
	}
	if jobDef.Name == "" {
		WriteError(w, http.StatusBadRequest, "Job definition name is required")
		return
	}
	if jobDef.Type == "" {
		WriteError(w, http.StatusBadRequest, "Job definition type is required")
		return
	}
	if len(jobDef.Steps) == 0 {
		WriteError(w, http.StatusBadRequest, "Job definition must have at least one step")
		return
	}

	// Validate job definition
	if err := jobDef.Validate(); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid job definition: %v", err))
		return
	}

	// Validate source IDs exist
	ctx := r.Context()
	if err := h.validateSourceIDs(ctx, jobDef.Sources); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Source validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid source: %v", err))
		return
	}

	// Validate step actions are registered
	if err := h.validateStepActions(jobDef.Type, jobDef.Steps); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Action validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid action: %v", err))
		return
	}

	// Save job definition
	if err := h.jobDefStorage.SaveJobDefinition(ctx, &jobDef); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to save job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to save job definition")
		return
	}

	h.logger.Info().Str("job_def_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition created successfully")
	WriteJSON(w, http.StatusCreated, jobDef)
}

// ListJobDefinitionsHandler handles GET /api/job-definitions
func (h *JobDefinitionHandler) ListJobDefinitionsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	ctx := r.Context()
	query := r.URL.Query()

	// Parse query parameters
	opts := interfaces.JobDefinitionListOptions{
		Limit:    50,
		Offset:   0,
		OrderBy:  "created_at",
		OrderDir: "DESC",
	}

	// Parse type filter
	if typeStr := query.Get("type"); typeStr != "" {
		opts.Type = typeStr
	}

	// Parse enabled filter
	if enabledStr := query.Get("enabled"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			opts.Enabled = &enabled
		}
	}

	// Parse limit
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}

	// Parse offset
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	// Parse order_by
	if orderBy := query.Get("order_by"); orderBy != "" {
		opts.OrderBy = orderBy
	}

	// Parse order_dir
	if orderDir := query.Get("order_dir"); orderDir != "" {
		opts.OrderDir = strings.ToUpper(orderDir)
	}

	// Fetch job definitions
	jobDefs, err := h.jobDefStorage.ListJobDefinitions(ctx, &opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list job definitions")
		WriteError(w, http.StatusInternalServerError, "Failed to list job definitions")
		return
	}

	// Get total count
	totalCount, err := h.jobDefStorage.CountJobDefinitions(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to count job definitions")
		WriteError(w, http.StatusInternalServerError, "Failed to count job definitions")
		return
	}

	// Ensure we return an empty array instead of null
	if jobDefs == nil {
		jobDefs = []*models.JobDefinition{}
	}

	h.logger.Info().Int("count", len(jobDefs)).Int("total", totalCount).Msg("Listed job definitions")

	response := map[string]interface{}{
		"job_definitions": jobDefs,
		"total_count":     totalCount,
		"limit":           opts.Limit,
		"offset":          opts.Offset,
	}

	WriteJSON(w, http.StatusOK, response)
}

// GetJobDefinitionHandler handles GET /api/job-definitions/{id}
func (h *JobDefinitionHandler) GetJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	id := extractJobDefinitionID(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Job definition ID is required")
		return
	}

	ctx := r.Context()
	jobDef, err := h.jobDefStorage.GetJobDefinition(ctx, id)
	if err != nil {
		if err == ErrJobDefinitionNotFound {
			h.logger.Warn().Str("job_def_id", id).Msg("Job definition not found")
			WriteError(w, http.StatusNotFound, "Job definition not found")
			return
		}
		h.logger.Error().Err(err).Str("job_def_id", id).Msg("Failed to get job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to get job definition")
		return
	}

	h.logger.Info().Str("job_def_id", id).Msg("Retrieved job definition")
	WriteJSON(w, http.StatusOK, jobDef)
}

// UpdateJobDefinitionHandler handles PUT /api/job-definitions/{id}
func (h *JobDefinitionHandler) UpdateJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "PUT") {
		return
	}

	id := extractJobDefinitionID(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Job definition ID is required")
		return
	}

	var jobDef models.JobDefinition
	if err := json.NewDecoder(r.Body).Decode(&jobDef); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode job definition")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Override ID from path to prevent ID mismatch
	jobDef.ID = id

	// Validate job definition
	if err := jobDef.Validate(); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid job definition: %v", err))
		return
	}

	// Validate source IDs exist
	ctx := r.Context()
	if err := h.validateSourceIDs(ctx, jobDef.Sources); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Source validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid source: %v", err))
		return
	}

	// Validate step actions are registered
	if err := h.validateStepActions(jobDef.Type, jobDef.Steps); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Action validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid action: %v", err))
		return
	}

	// Update job definition
	if err := h.jobDefStorage.UpdateJobDefinition(ctx, &jobDef); err != nil {
		if err == ErrJobDefinitionNotFound {
			h.logger.Warn().Str("job_def_id", id).Msg("Job definition not found")
			WriteError(w, http.StatusNotFound, "Job definition not found")
			return
		}
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to update job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to update job definition")
		return
	}

	h.logger.Info().Str("job_def_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition updated successfully")
	WriteJSON(w, http.StatusOK, jobDef)
}

// DeleteJobDefinitionHandler handles DELETE /api/job-definitions/{id}
func (h *JobDefinitionHandler) DeleteJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "DELETE") {
		return
	}

	id := extractJobDefinitionID(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Job definition ID is required")
		return
	}

	ctx := r.Context()
	if err := h.jobDefStorage.DeleteJobDefinition(ctx, id); err != nil {
		if err == ErrJobDefinitionNotFound {
			h.logger.Warn().Str("job_def_id", id).Msg("Job definition not found")
			WriteError(w, http.StatusNotFound, "Job definition not found")
			return
		}
		h.logger.Error().Err(err).Str("job_def_id", id).Msg("Failed to delete job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to delete job definition")
		return
	}

	h.logger.Info().Str("job_def_id", id).Msg("Job definition deleted successfully")
	w.WriteHeader(http.StatusNoContent)
}

// ExecuteJobDefinitionHandler handles POST /api/job-definitions/{id}/execute
func (h *JobDefinitionHandler) ExecuteJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	id := extractJobDefinitionID(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Job definition ID is required")
		return
	}

	ctx := r.Context()
	jobDef, err := h.jobDefStorage.GetJobDefinition(ctx, id)
	if err != nil {
		if err == ErrJobDefinitionNotFound {
			h.logger.Warn().Str("job_def_id", id).Msg("Job definition not found")
			WriteError(w, http.StatusNotFound, "Job definition not found")
			return
		}
		h.logger.Error().Err(err).Str("job_def_id", id).Msg("Failed to get job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to get job definition")
		return
	}

	// Pre-execution validation
	if !jobDef.Enabled {
		h.logger.Warn().Str("job_def_id", id).Msg("Job definition is disabled")
		WriteError(w, http.StatusBadRequest, "Job definition is disabled")
		return
	}

	if err := jobDef.Validate(); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", id).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid job definition: %v", err))
		return
	}

	// Create parent job message for job definition execution using dedicated constructor
	parentMsg := queue.NewJobDefinitionMessage(
		jobDef.ID,
		map[string]interface{}{
			"job_definition_id": jobDef.ID,
			"job_name":          jobDef.Name,
			"job_type":          string(jobDef.Type),
			"sources":           jobDef.Sources,
			"steps":             jobDef.Steps,
			"timeout":           jobDef.Timeout,
		},
	)

	// Enqueue parent message
	if err := h.queueManager.Enqueue(ctx, parentMsg); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to enqueue job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to start job execution")
		return
	}

	h.logger.Info().Str("job_def_id", id).Str("message_id", parentMsg.ID).Msg("Job definition enqueued")

	response := map[string]interface{}{
		"job_id":   parentMsg.ID,
		"job_name": jobDef.Name,
		"status":   "queued",
		"message":  "Job execution queued successfully",
	}

	WriteJSON(w, http.StatusAccepted, response)
}

// validateSourceIDs validates that all source IDs exist
func (h *JobDefinitionHandler) validateSourceIDs(ctx context.Context, sourceIDs []string) error {
	for _, sourceID := range sourceIDs {
		if _, err := h.sourceService.GetSource(ctx, sourceID); err != nil {
			return fmt.Errorf("source not found: %s", sourceID)
		}
	}
	return nil
}

// validateStepActions validates that all step actions are registered
func (h *JobDefinitionHandler) validateStepActions(jobType models.JobType, steps []models.JobStep) error {
	for _, step := range steps {
		if _, err := h.jobRegistry.GetAction(jobType, step.Action); err != nil {
			return fmt.Errorf("unknown action '%s' for step '%s'", step.Action, step.Name)
		}
	}
	return nil
}

// extractJobDefinitionID extracts the job definition ID from the URL path
func extractJobDefinitionID(path string) string {
	// Remove trailing slash
	path = strings.TrimSuffix(path, "/")

	// Handle /execute suffix
	path = strings.TrimSuffix(path, "/execute")

	// Extract ID from path like "/api/job-definitions/{id}"
	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[1] == "api" && parts[2] == "job-definitions" {
		return parts[3]
	}

	return ""
}
