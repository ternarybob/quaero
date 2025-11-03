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
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/jobs/executor"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/sources"
)

var ErrJobDefinitionNotFound = errors.New("job definition not found")

// JobDefinitionHandler handles HTTP requests for job definition management
type JobDefinitionHandler struct {
	jobDefStorage interfaces.JobDefinitionStorage
	jobStorage    interfaces.JobStorage
	jobExecutor   *executor.JobExecutor
	sourceService *sources.Service
	logger        arbor.ILogger
}

// NewJobDefinitionHandler creates a new job definition handler
func NewJobDefinitionHandler(
	jobDefStorage interfaces.JobDefinitionStorage,
	jobStorage interfaces.JobStorage,
	jobExecutor *executor.JobExecutor,
	sourceService *sources.Service,
	logger arbor.ILogger,
) *JobDefinitionHandler {
	if jobDefStorage == nil {
		panic("jobDefStorage cannot be nil")
	}
	if jobStorage == nil {
		panic("jobStorage cannot be nil")
	}
	if jobExecutor == nil {
		panic("jobExecutor cannot be nil")
	}
	if sourceService == nil {
		panic("sourceService cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}

	logger.Info().Msg("Job definition handler initialized with job executor")

	return &JobDefinitionHandler{
		jobDefStorage: jobDefStorage,
		jobStorage:    jobStorage,
		jobExecutor:   jobExecutor,
		sourceService: sourceService,
		logger:        logger,
	}
}

// GetJobTreeStatusHandler handles GET /api/jobs/{id}/status
// Returns aggregated status for a parent job and all its children
func (h *JobDefinitionHandler) GetJobTreeStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Extract job ID from path
	jobID := extractJobID(r.URL.Path)
	if jobID == "" {
		WriteError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	ctx := r.Context()

	// Get job manager from job storage (type assertion)
	jobManager, ok := h.jobStorage.(interface {
		GetJobTreeStatus(ctx context.Context, parentJobID string) (*jobs.JobTreeStatus, error)
	})

	if !ok {
		h.logger.Error().Msg("Job storage does not implement GetJobTreeStatus")
		WriteError(w, http.StatusInternalServerError, "Status aggregation not supported")
		return
	}

	// Get aggregated status
	status, err := jobManager.GetJobTreeStatus(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job tree status")
		WriteError(w, http.StatusInternalServerError, "Failed to get job status")
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Int("total_children", status.TotalChildren).
		Int("completed", status.CompletedCount).
		Int("failed", status.FailedCount).
		Float64("progress", status.OverallProgress).
		Msg("Retrieved job tree status")

	WriteJSON(w, http.StatusOK, status)
}

// extractJobID extracts the job ID from the URL path
func extractJobID(path string) string {
	// Handle paths like "/api/jobs/{id}/status" or "/api/jobs/{id}"
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, "/status")

	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[1] == "api" && parts[2] == "jobs" {
		return parts[3]
	}

	return ""
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

	h.logger.Info().
		Str("job_def_id", jobDef.ID).
		Str("job_name", jobDef.Name).
		Int("step_count", len(jobDef.Steps)).
		Int("source_count", len(jobDef.Sources)).
		Msg("Executing job definition")

	// Launch goroutine to execute job definition asynchronously
	go func() {
		bgCtx := context.Background()

		parentJobID, err := h.jobExecutor.Execute(bgCtx, jobDef)
		if err != nil {
			h.logger.Error().
				Err(err).
				Str("job_def_id", jobDef.ID).
				Msg("Job definition execution failed")
			return
		}

		h.logger.Info().
			Str("job_def_id", jobDef.ID).
			Str("parent_job_id", parentJobID).
			Msg("Job definition execution completed successfully")
	}()

	response := map[string]interface{}{
		"job_id":   jobDef.ID,
		"job_name": jobDef.Name,
		"status":   "running",
		"message":  "Job execution started",
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
// TODO Phase 8-11: Re-enable when job registry is re-integrated
func (h *JobDefinitionHandler) validateStepActions(jobType models.JobDefinitionType, steps []models.JobStep) error {
	// Temporarily disabled during queue refactor - jobRegistry is interface{} with no methods
	_ = jobType // Suppress unused variable
	_ = steps   // Suppress unused variable
	return nil  // Skip validation during refactor

	// TODO Phase 8-11: Uncomment when job registry is available
	// for _, step := range steps {
	// 	if _, err := h.jobRegistry.GetAction(jobType, step.Action); err != nil {
	// 		return fmt.Errorf("unknown action '%s' for step '%s'", step.Action, step.Name)
	// 	}
	// }
	// return nil
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
