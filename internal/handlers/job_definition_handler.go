package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/queue/state"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/validation"
)

var ErrJobDefinitionNotFound = errors.New("job definition not found")

// JobDefinitionHandler handles HTTP requests for job definition management
type JobDefinitionHandler struct {
	jobDefStorage      interfaces.JobDefinitionStorage
	jobStorage         interfaces.QueueStorage
	jobMgr             *queue.Manager
	orchestrator       *queue.Orchestrator
	jobMonitor         interfaces.JobMonitor
	stepMonitor        interfaces.StepMonitor
	authStorage        interfaces.AuthStorage
	kvStorage          interfaces.KeyValueStorage // For {key-name} replacement in job definitions
	storageManager     interfaces.StorageManager  // For reloading job definitions from disk
	definitionsDir     string                     // Path to job definitions directory
	validationService  *validation.TOMLValidationService
	jobService         *jobs.Service              // Business logic for job definitions
	documentService    interfaces.DocumentService // For direct document capture
	logger             arbor.ILogger
}

// NewJobDefinitionHandler creates a new job definition handler
func NewJobDefinitionHandler(
	jobDefStorage interfaces.JobDefinitionStorage,
	jobStorage interfaces.QueueStorage,
	jobMgr *queue.Manager,
	orchestrator *queue.Orchestrator,
	jobMonitor interfaces.JobMonitor,
	stepMonitor interfaces.StepMonitor,
	authStorage interfaces.AuthStorage,
	kvStorage interfaces.KeyValueStorage, // For {key-name} replacement in job definitions
	storageManager interfaces.StorageManager, // For reloading job definitions from disk
	definitionsDir string, // Path to job definitions directory
	agentService interfaces.AgentService, // Optional: can be nil if agent service unavailable
	documentService interfaces.DocumentService, // For direct document capture from extension
	logger arbor.ILogger,
) *JobDefinitionHandler {
	if jobDefStorage == nil {
		panic("jobDefStorage cannot be nil")
	}
	if jobStorage == nil {
		panic("jobStorage cannot be nil")
	}
	if jobMgr == nil {
		panic("jobMgr cannot be nil")
	}
	if orchestrator == nil {
		panic("orchestrator cannot be nil")
	}
	if authStorage == nil {
		panic("authStorage cannot be nil")
	}
	if kvStorage == nil {
		panic("kvStorage cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}

	logger.Debug().Msg("Job definition handler initialized with JobManager and JobMonitor")

	return &JobDefinitionHandler{
		jobDefStorage:      jobDefStorage,
		jobStorage:         jobStorage,
		jobMgr:             jobMgr,
		orchestrator:       orchestrator,
		jobMonitor:         jobMonitor,
		stepMonitor:        stepMonitor,
		authStorage:        authStorage,
		kvStorage:          kvStorage,
		storageManager:     storageManager,
		definitionsDir:     definitionsDir,
		validationService:  validation.NewTOMLValidationService(logger),
		jobService:         jobs.NewService(kvStorage, agentService, logger),
		documentService:    documentService,
		logger:             logger,
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
		GetJobTreeStatus(ctx context.Context, parentJobID string) (*state.JobTreeStatus, error)
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

	h.logger.Debug().
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

	ctx := r.Context()

	// Validate step actions are registered
	if err := h.jobService.ValidateStepActions(jobDef.Type, jobDef.Steps); err != nil {
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

	h.logger.Debug().Str("job_def_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition created successfully")
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
		OrderBy:  "CreatedAt",
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

	// Validate runtime dependencies for each job definition
	for _, jobDef := range jobDefs {
		h.jobService.ValidateRuntimeDependencies(jobDef)
	}

	h.logger.Debug().Int("count", len(jobDefs)).Int("total", totalCount).Msg("Listed job definitions")

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

	h.logger.Debug().Str("job_def_id", id).Msg("Retrieved job definition")
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

	ctx := r.Context()

	// Check if job definition exists and is not a system job
	existingJobDef, err := h.jobDefStorage.GetJobDefinition(ctx, id)
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

	// Prevent editing system jobs
	if existingJobDef.IsSystemJob() {
		h.logger.Warn().Str("job_def_id", id).Msg("Cannot edit system job")
		WriteError(w, http.StatusForbidden, "Cannot edit system-managed jobs")
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

	// Validate step actions are registered
	if err := h.jobService.ValidateStepActions(jobDef.Type, jobDef.Steps); err != nil {
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

	h.logger.Debug().Str("job_def_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition updated successfully")
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

	// Check if job definition exists and is not a system job
	existingJobDef, err := h.jobDefStorage.GetJobDefinition(ctx, id)
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

	// Prevent deleting system jobs
	if existingJobDef.IsSystemJob() {
		h.logger.Warn().Str("job_def_id", id).Msg("Cannot delete system job")
		WriteError(w, http.StatusForbidden, "Cannot delete system-managed jobs")
		return
	}

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

	h.logger.Debug().Str("job_def_id", id).Msg("Job definition deleted successfully")
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

	h.logger.Debug().
		Str("job_def_id", jobDef.ID).
		Str("job_name", jobDef.Name).
		Str("job_def_type", string(jobDef.Type)).
		Str("source_type", jobDef.SourceType).
		Int("step_count", len(jobDef.Steps)).
		Msg("Executing job definition")

	// Launch goroutine to execute job definition asynchronously
	go func() {
		bgCtx := context.Background()

		parentJobID, err := h.orchestrator.ExecuteJobDefinition(bgCtx, jobDef, h.jobMonitor, h.stepMonitor)
		if err != nil {
			h.logger.Error().
				Err(err).
				Str("job_def_id", jobDef.ID).
				Msg("Job definition execution failed")
			return
		}

		h.logger.Debug().
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

// extractJobDefinitionID extracts the job definition ID from the URL path
func extractJobDefinitionID(path string) string {
	// Remove trailing slash
	path = strings.TrimSuffix(path, "/")

	// Handle /execute suffix
	path = strings.TrimSuffix(path, "/execute")

	// Handle /export suffix
	path = strings.TrimSuffix(path, "/export")

	// Handle /status suffix
	path = strings.TrimSuffix(path, "/status")

	// Extract ID from path like "/api/job-definitions/{id}"
	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[1] == "api" && parts[2] == "job-definitions" {
		return parts[3]
	}

	return ""
}

// ExportJobDefinitionHandler handles GET /api/job-definitions/{id}/export
// Exports a job definition as a TOML file for download
func (h *JobDefinitionHandler) ExportJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
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
			h.logger.Warn().Str("job_def_id", id).Msg("Job definition not found for export")
			WriteError(w, http.StatusNotFound, "Job definition not found")
			return
		}
		h.logger.Error().Err(err).Str("job_def_id", id).Msg("Failed to get job definition for export")
		WriteError(w, http.StatusInternalServerError, "Failed to get job definition")
		return
	}

	// Only export crawler jobs (other types are internal)
	if jobDef.Type != models.JobDefinitionTypeCrawler {
		h.logger.Warn().Str("job_def_id", id).Str("type", string(jobDef.Type)).Msg("Cannot export non-crawler job definition")
		WriteError(w, http.StatusBadRequest, "Only crawler job definitions can be exported")
		return
	}

	// Convert to simplified TOML format
	tomlData, err := h.jobService.ConvertToTOML(jobDef)
	if err != nil {
		h.logger.Error().Err(err).Str("job_def_id", id).Msg("Failed to convert job definition to TOML")
		WriteError(w, http.StatusInternalServerError, "Failed to export job definition")
		return
	}

	// Set headers for file download
	filename := fmt.Sprintf("%s.toml", jobDef.ID)
	w.Header().Set("Content-Type", "application/toml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(tomlData)))

	h.logger.Debug().Str("job_def_id", id).Str("filename", filename).Msg("Exporting job definition as TOML")

	w.WriteHeader(http.StatusOK)
	w.Write(tomlData)
}

// ValidateJobDefinitionTOMLHandler handles POST /api/job-definitions/validate
// Validates TOML content and optionally persists validation status if job_id query param provided
func (h *JobDefinitionHandler) ValidateJobDefinitionTOMLHandler(w http.ResponseWriter, r *http.Request) {
	// Read TOML content from request body
	tomlContent, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to read request body")
		WriteError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	ctx := r.Context()

	// Validate TOML using validation service
	result := h.validationService.ValidateTOML(ctx, string(tomlContent))

	// Return validation result
	if result.Valid {
		h.logger.Debug().Msg("TOML validated successfully")
		WriteJSON(w, http.StatusOK, result)
	} else {
		h.logger.Warn().Str("error", result.Error).Msg("TOML validation failed")
		WriteJSON(w, http.StatusBadRequest, result)
	}
}

// ReloadJobDefinitionsHandler handles POST /api/job-definitions/reload
// Reloads job definitions from disk (TOML files in the definitions directory)
func (h *JobDefinitionHandler) ReloadJobDefinitionsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	ctx := r.Context()

	h.logger.Info().Str("dir", h.definitionsDir).Msg("Reloading job definitions from disk")

	// Check if storage manager and definitions dir are configured
	if h.storageManager == nil {
		h.logger.Error().Msg("Storage manager not configured for reload")
		WriteError(w, http.StatusInternalServerError, "Storage manager not configured")
		return
	}

	if h.definitionsDir == "" {
		h.logger.Error().Msg("Definitions directory not configured for reload")
		WriteError(w, http.StatusInternalServerError, "Definitions directory not configured")
		return
	}

	// Reload job definitions from disk
	if err := h.storageManager.LoadJobDefinitionsFromFiles(ctx, h.definitionsDir); err != nil {
		h.logger.Error().Err(err).Str("dir", h.definitionsDir).Msg("Failed to reload job definitions")
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reload: %s", err.Error()))
		return
	}

	// Count loaded definitions
	jobDefs, err := h.jobDefStorage.ListJobDefinitions(ctx)
	loaded := 0
	if err == nil {
		loaded = len(jobDefs)
	}

	h.logger.Info().Int("loaded", loaded).Str("dir", h.definitionsDir).Msg("Job definitions reloaded successfully")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"loaded":  loaded,
		"message": fmt.Sprintf("Reloaded %d job definitions from %s", loaded, h.definitionsDir),
	})
}

// UploadJobDefinitionTOMLHandler handles POST /api/job-definitions/upload
// Creates or updates a job definition from TOML content
func (h *JobDefinitionHandler) UploadJobDefinitionTOMLHandler(w http.ResponseWriter, r *http.Request) {
	// Read TOML content from request body
	tomlContent, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to read request body")
		WriteError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Parse TOML into generic JobDefinitionFile
	jobFile, err := jobs.ParseTOML(tomlContent)
	if err != nil {
		h.logger.Error().Err(err).Msg("Invalid TOML syntax")
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Convert to full JobDefinition model
	jobDef, err := jobFile.ToJobDefinition()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to convert TOML to JobDefinition")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("TOML conversion failed: %v", err))
		return
	}

	// Store raw TOML content
	jobDef.TOML = string(tomlContent)

	// Validate full job definition
	if err := jobDef.Validate(); err != nil {
		h.logger.Error().Err(err).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Validate step actions are registered
	if err := h.jobService.ValidateStepActions(jobDef.Type, jobDef.Steps); err != nil {
		h.logger.Error().Err(err).Msg("Action validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid action: %v", err))
		return
	}

	ctx := r.Context()

	// Check if job definition already exists
	existingJobDef, err := h.jobDefStorage.GetJobDefinition(ctx, jobDef.ID)
	isUpdate := false

	if err == nil && existingJobDef != nil {
		// Job exists - check if it's a system job
		if existingJobDef.IsSystemJob() {
			h.logger.Warn().Str("job_def_id", jobDef.ID).Msg("Cannot update system job via upload")
			WriteError(w, http.StatusForbidden, "Cannot update system-managed jobs")
			return
		}
		isUpdate = true
	}

	// Save or update job definition
	if isUpdate {
		if err := h.jobDefStorage.UpdateJobDefinition(ctx, jobDef); err != nil {
			h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to update job definition")
			WriteError(w, http.StatusInternalServerError, "Failed to update job definition")
			return
		}
		h.logger.Debug().Str("job_def_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition updated from TOML upload")
		WriteJSON(w, http.StatusOK, jobDef)
	} else {
		if err := h.jobDefStorage.SaveJobDefinition(ctx, jobDef); err != nil {
			h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to save job definition")
			WriteError(w, http.StatusInternalServerError, "Failed to save job definition")
			return
		}
		h.logger.Debug().Str("job_def_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition created from TOML upload")
		WriteJSON(w, http.StatusCreated, jobDef)
	}
}

// SaveInvalidJobDefinitionHandler handles POST /api/job-definitions/save-invalid
// Saves invalid/incomplete TOML content without validation for testing purposes
func (h *JobDefinitionHandler) SaveInvalidJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	// Read TOML content from request body
	tomlContent, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to read request body")
		WriteError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Generate unique ID with invalid- prefix
	id := fmt.Sprintf("invalid-%d", time.Now().Unix())

	// Create JobDefinition with minimal fields and raw TOML
	jobDef := &models.JobDefinition{
		ID:   id,
		Name: "Invalid",
		TOML: string(tomlContent),
	}

	// Save directly to storage without validation
	ctx := r.Context()
	if err := h.jobDefStorage.SaveJobDefinition(ctx, jobDef); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to save invalid job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to save job definition")
		return
	}

	h.logger.Debug().Str("job_def_id", jobDef.ID).Msg("Invalid job definition saved without validation")
	WriteJSON(w, http.StatusCreated, jobDef)
}

// findMatchingJobDefinition searches crawler job definitions for URL pattern matches
// Returns the MOST SPECIFIC job definition whose url_patterns match the target URL
// When multiple patterns match, the one with highest specificity wins (more literal characters)
// Patterns support wildcards: * matches any sequence of characters
// Example: "*.atlassian.net/wiki/*" is more specific than "*.*" and will be preferred
func (h *JobDefinitionHandler) findMatchingJobDefinition(ctx context.Context, targetURL string) (*models.JobDefinition, error) {
	// List all crawler-type job definitions
	opts := &interfaces.JobDefinitionListOptions{
		Type:  string(models.JobDefinitionTypeCrawler),
		Limit: 100, // Reasonable limit for job definitions
	}

	jobDefs, err := h.jobDefStorage.ListJobDefinitions(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list job definitions: %w", err)
	}

	// Parse target URL to extract host for matching
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}
	targetHost := parsedURL.Host
	targetPath := parsedURL.Path

	// Find ALL matching job definitions and track their specificity
	type matchResult struct {
		jobDef      *models.JobDefinition
		pattern     string
		specificity int // Higher = more specific (more literal characters)
	}
	var matches []matchResult

	for _, jobDef := range jobDefs {
		if len(jobDef.UrlPatterns) == 0 {
			continue
		}

		for _, pattern := range jobDef.UrlPatterns {
			if h.matchURLPattern(pattern, targetHost, targetPath, targetURL) {
				// Calculate pattern specificity: count non-wildcard characters
				specificity := h.calculatePatternSpecificity(pattern)
				matches = append(matches, matchResult{
					jobDef:      jobDef,
					pattern:     pattern,
					specificity: specificity,
				})
			}
		}
	}

	// Return the most specific match (highest specificity score)
	if len(matches) > 0 {
		bestMatch := matches[0]
		for _, m := range matches[1:] {
			if m.specificity > bestMatch.specificity {
				bestMatch = m
			}
		}
		h.logger.Debug().
			Str("job_def_id", bestMatch.jobDef.ID).
			Str("pattern", bestMatch.pattern).
			Int("specificity", bestMatch.specificity).
			Int("total_matches", len(matches)).
			Str("target_url", targetURL).
			Msg("Found best matching job definition for URL")
		return bestMatch.jobDef, nil
	}

	h.logger.Debug().
		Str("target_url", targetURL).
		Int("job_defs_checked", len(jobDefs)).
		Msg("No matching job definition found for URL")
	return nil, nil
}

// matchURLPattern checks if a URL matches a wildcard pattern
// Pattern format: "*.domain.com/path/*" where * matches any characters
func (h *JobDefinitionHandler) matchURLPattern(pattern, targetHost, targetPath, fullURL string) bool {
	// Convert wildcard pattern to regex
	// Escape special regex characters except *
	escaped := regexp.QuoteMeta(pattern)
	// Replace escaped \* with regex .*
	regexPattern := strings.ReplaceAll(escaped, `\*`, `.*`)
	// Anchor the pattern
	regexPattern = "^" + regexPattern + "$"

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		h.logger.Warn().
			Str("pattern", pattern).
			Err(err).
			Msg("Invalid URL pattern, skipping")
		return false
	}

	// Try matching against host+path (without scheme)
	hostPath := targetHost + targetPath
	if re.MatchString(hostPath) {
		return true
	}

	// Also try matching against full URL (with scheme)
	if re.MatchString(fullURL) {
		return true
	}

	return false
}

// calculatePatternSpecificity calculates how specific a URL pattern is
// Higher scores indicate more specific patterns (preferred over generic ones)
// Score is based on:
// - Number of literal (non-wildcard) characters
// - Patterns with more literal content are more specific
// Example: "*.atlassian.net/wiki/*" is more specific than "*.*"
func (h *JobDefinitionHandler) calculatePatternSpecificity(pattern string) int {
	specificity := 0

	// Count literal characters (non-wildcards)
	for _, char := range pattern {
		if char != '*' {
			specificity++
		}
	}

	// Bonus for patterns with more path segments (more '/')
	specificity += strings.Count(pattern, "/") * 2

	// Bonus for patterns with specific domain parts (more '.')
	specificity += strings.Count(pattern, ".") * 2

	// Penalty for patterns that are mostly wildcards
	wildcardCount := strings.Count(pattern, "*")
	if wildcardCount > 0 && len(pattern) > 0 {
		wildcardRatio := float64(wildcardCount) / float64(len(pattern))
		if wildcardRatio > 0.5 {
			specificity = specificity / 2 // Heavy penalty for mostly-wildcard patterns
		}
	}

	return specificity
}

// prepareJobDefForExecution creates an in-memory copy of the job definition with runtime overrides.
// The returned copy has start_urls and auth_id modified, but retains the original ID.
// This is used when executing an existing job definition without creating a new one.
// For quick crawl from extension: limits to SINGLE PAGE only (max_depth=0, max_pages=1, follow_links=false)
func (h *JobDefinitionHandler) prepareJobDefForExecution(template *models.JobDefinition, targetURL string, authID string) *models.JobDefinition {
	// Create an in-memory copy - keep original ID so it references the existing job definition
	jobDef := &models.JobDefinition{
		ID:          template.ID, // Keep original ID
		Name:        template.Name,
		Type:        template.Type,
		JobType:     template.JobType,
		Schedule:    template.Schedule,
		Timeout:     template.Timeout,
		Enabled:     template.Enabled,
		AutoStart:   template.AutoStart,
		AuthID:      authID, // Override with fresh auth from extension
		Tags:        template.Tags,
		UrlPatterns: template.UrlPatterns,
		Config:      make(map[string]interface{}),
		Description: template.Description,
	}

	// Copy the original config
	for k, v := range template.Config {
		jobDef.Config[k] = v
	}

	// Use default timeout if not set
	if jobDef.Timeout == "" {
		jobDef.Timeout = "30m"
	}

	// Copy and modify steps - override for SINGLE PAGE quick crawl
	for _, step := range template.Steps {
		newStep := models.JobStep{
			Name:        step.Name,
			Type:        step.Type,
			Description: step.Description,
			OnError:     step.OnError,
			Depends:     step.Depends,
			Condition:   step.Condition,
		}

		// Copy config and override for single-page quick crawl
		newConfig := make(map[string]interface{})
		for k, v := range step.Config {
			newConfig[k] = v
		}
		// Override start_urls with the requested URL
		newConfig["start_urls"] = []interface{}{targetURL}

		// QUICK CRAWL OVERRIDES: Limit to single target page only
		// This prevents opening multiple browsers and crawling irrelevant pages
		newConfig["max_depth"] = 0        // Don't follow links to other pages
		newConfig["max_pages"] = 1        // Only crawl the single target page
		newConfig["follow_links"] = false // Don't follow any links
		newConfig["concurrency"] = 1      // Single browser instance

		newStep.Config = newConfig

		jobDef.Steps = append(jobDef.Steps, newStep)
	}

	return jobDef
}

// createAdHocJobDef creates a new ad-hoc job definition with default crawler settings
// Used when no matching job definition is found for the URL
func (h *JobDefinitionHandler) createAdHocJobDef(targetURL, name string, maxDepthPtr, maxPagesPtr *int, includePatterns, excludePatterns []string, authID string) *models.JobDefinition {
	// Default values
	maxDepth := 2
	maxPages := 10
	if maxDepthPtr != nil {
		maxDepth = *maxDepthPtr
	}
	if maxPagesPtr != nil {
		maxPages = *maxPagesPtr
	}

	// Generate name if not provided
	if name == "" {
		name = fmt.Sprintf("Capture & Crawl: %s", targetURL)
	}

	// Build crawler step config
	crawlStepConfig := map[string]interface{}{
		"start_urls":   []interface{}{targetURL},
		"max_depth":    maxDepth,
		"max_pages":    maxPages,
		"concurrency":  5,
		"follow_links": true,
	}

	// Add optional patterns if provided
	if len(includePatterns) > 0 {
		patterns := make([]interface{}, len(includePatterns))
		for i, p := range includePatterns {
			patterns[i] = p
		}
		crawlStepConfig["include_patterns"] = patterns
	}
	if len(excludePatterns) > 0 {
		patterns := make([]interface{}, len(excludePatterns))
		for i, p := range excludePatterns {
			patterns[i] = p
		}
		crawlStepConfig["exclude_patterns"] = patterns
	}

	return &models.JobDefinition{
		ID:        fmt.Sprintf("capture-crawl-%d", time.Now().UnixNano()),
		Name:      name,
		Type:      models.JobDefinitionTypeCrawler,
		JobType:   models.JobOwnerTypeUser,
		Schedule:  "", // Manual execution only
		Timeout:   "30m",
		Enabled:   true,
		AutoStart: false,
		AuthID:    authID,
		Steps: []models.JobStep{
			{
				Name:    "crawl",
				Type:    models.WorkerTypeCrawler,
				Config:  crawlStepConfig,
				OnError: models.ErrorStrategyFail,
			},
		},
		Config:    make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// CreateAndExecuteQuickCrawlHandler handles POST /api/job-definitions/quick-crawl
// Creates a temporary crawler job definition from the current page URL and executes it immediately
// This endpoint is designed for the Chrome extension's "Capture & Crawl" button
func (h *JobDefinitionHandler) CreateAndExecuteQuickCrawlHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	// Parse request body
	var req struct {
		URL             string                   `json:"url"`                         // Current page URL (required)
		Name            string                   `json:"name,omitempty"`              // Optional custom name
		MaxDepth        *int                     `json:"max_depth,omitempty"`         // Optional override (defaults to config value)
		MaxPages        *int                     `json:"max_pages,omitempty"`         // Optional override (defaults to config value)
		IncludePatterns []string                 `json:"include_patterns,omitempty"`  // Optional URL patterns to include
		ExcludePatterns []string                 `json:"exclude_patterns,omitempty"`  // Optional URL patterns to exclude
		Cookies         []map[string]interface{} `json:"cookies,omitempty"`           // Optional auth cookies from extension
		HTML            string                   `json:"html,omitempty"`              // Captured HTML from extension (no browser needed)
		Title           string                   `json:"title,omitempty"`             // Page title from extension
		UseCapturedHTML bool                     `json:"use_captured_html,omitempty"` // Use captured HTML instead of crawler
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode quick crawl request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Validate required fields
	if req.URL == "" {
		WriteError(w, http.StatusBadRequest, "URL is required")
		return
	}

	ctx := r.Context()

	// If using captured HTML from extension, save document and optionally crawl links
	if req.UseCapturedHTML && req.HTML != "" {
		h.logger.Info().
			Str("url", req.URL).
			Int("html_size", len(req.HTML)).
			Msg("Quick crawl using captured HTML from extension")

		// Process HTML content using ContentProcessor
		contentProcessor := crawler.NewContentProcessor(h.logger)
		processedContent, err := contentProcessor.ProcessHTML(req.HTML, req.URL)
		if err != nil {
			h.logger.Error().Err(err).Str("url", req.URL).Msg("Failed to process captured HTML")
			WriteError(w, http.StatusInternalServerError, "Failed to process HTML content")
			return
		}

		// Generate crawl job ID
		crawlJobID := uuid.New().String()

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
			"capture_source": "chrome_extension_crawl",
			"capture_time":   time.Now().Format(time.RFC3339),
			"original_url":   req.URL,
			"content_size":   len(processedContent.Markdown),
			"links_found":    len(processedContent.Links),
			"crawl_job_id":   crawlJobID,
		}

		// Create document for the first page
		doc := &models.Document{
			ID:              uuid.New().String(),
			SourceType:      "web",
			SourceID:        crawlJobID,
			Title:           title,
			ContentMarkdown: processedContent.Markdown,
			URL:             req.URL,
			Metadata:        metadata,
			Tags:            []string{"captured", "chrome-extension", "crawl"},
		}

		// Save first document
		if err := h.documentService.SaveDocument(ctx, doc); err != nil {
			h.logger.Error().Err(err).Str("doc_id", doc.ID).Msg("Failed to save captured document")
			WriteError(w, http.StatusInternalServerError, "Failed to save document")
			return
		}

		h.logger.Info().
			Str("doc_id", doc.ID).
			Str("url", req.URL).
			Str("title", title).
			Int("links_found", len(processedContent.Links)).
			Msg("First page saved, starting background crawl of links")

		// Get crawl settings from matching job definition - required for link crawling
		matchedJobDef, _ := h.findMatchingJobDefinition(ctx, req.URL)

		// Only start background crawl if we have a matching job definition
		maxPages := 10
		includePatterns := []string{}
		excludePatterns := []string{}
		downloadImages := false
		crawlTags := []string{"extension", "crawl", "headless"} // Default tags
		canCrawlLinks := matchedJobDef != nil

		if matchedJobDef != nil {
			// Use tags from matched job definition if available
			if len(matchedJobDef.Tags) > 0 {
				crawlTags = matchedJobDef.Tags
			}
			if len(matchedJobDef.Steps) > 0 {
				stepConfig := matchedJobDef.Steps[0].Config
				if mp, ok := stepConfig["max_pages"].(int); ok {
					maxPages = mp
				}
				if di, ok := stepConfig["download_images"].(bool); ok {
					downloadImages = di
				}
				if inc, ok := stepConfig["include_patterns"].([]interface{}); ok {
					for _, p := range inc {
						if ps, ok := p.(string); ok {
							includePatterns = append(includePatterns, ps)
						}
					}
				}
				if exc, ok := stepConfig["exclude_patterns"].([]interface{}); ok {
					for _, p := range exc {
						if ps, ok := p.(string); ok {
							excludePatterns = append(excludePatterns, ps)
						}
					}
				}
			}
		}

		// Start background crawl of discovered links using headless chromedp via orchestrator
		// This ensures proper Job Manager → Step → Worker hierarchy
		// Only crawl if we have a matching job definition (canCrawlLinks)
		if len(processedContent.Links) > 0 && len(req.Cookies) > 0 && canCrawlLinks {
			// Parse URL for auth storage
			parsedURL, _ := url.Parse(req.URL)
			baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

			// Store cookies as auth credentials
			authID := fmt.Sprintf("ext_%s_%d", parsedURL.Host, time.Now().UnixNano())
			cookiesJSON, _ := json.Marshal(req.Cookies)
			authCreds := &models.AuthCredentials{
				ID:          authID,
				Name:        fmt.Sprintf("Extension: %s", parsedURL.Host),
				SiteDomain:  parsedURL.Host,
				ServiceType: "generic",
				BaseURL:     baseURL,
				Cookies:     cookiesJSON,
				Tokens:      make(map[string]string),
				Data:        make(map[string]interface{}),
				CreatedAt:   time.Now().Unix(),
				UpdatedAt:   time.Now().Unix(),
			}
			if err := h.authStorage.StoreCredentials(ctx, authCreds); err != nil {
				h.logger.Warn().Err(err).Msg("Failed to store auth credentials for crawl")
			}

			// Limit links to maxPages
			linksToUse := processedContent.Links
			if len(linksToUse) > maxPages-1 {
				linksToUse = linksToUse[:maxPages-1]
			}

			// Create ephemeral job definition for orchestrator (headless chromedp)
			ephemeralJobDef := &models.JobDefinition{
				ID:          crawlJobID,
				Name:        fmt.Sprintf("Crawl: %s", parsedURL.Host),
				Type:        models.JobDefinitionTypeCrawler,
				Description: fmt.Sprintf("Extension-initiated crawl of %s", parsedURL.Host),
				BaseURL:     baseURL,
				SourceType:  "web",
				AuthID:      authID,
				Enabled:     true,
				Timeout:     "30m",
				Tags:        crawlTags,
				Steps: []models.JobStep{
					{
						Name:        "crawl_pages",
						Type:        models.WorkerTypeCrawler,
						Description: fmt.Sprintf("Crawl pages from %s using headless browser", parsedURL.Host),
						OnError:     models.ErrorStrategyContinue,
						Config: map[string]interface{}{
							"start_urls":       linksToUse,
							"max_depth":        0,     // Don't follow links from these pages
							"max_pages":        len(linksToUse),
							"concurrency":      3,
							"follow_links":     false, // Only crawl the provided links
							"download_images":  downloadImages,
							"include_patterns": includePatterns,
							"exclude_patterns": excludePatterns,
						},
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Execute via orchestrator (headless chromedp with proper job hierarchy)
			go func() {
				bgCtx := context.Background()
				parentJobID, err := h.orchestrator.ExecuteJobDefinition(bgCtx, ephemeralJobDef, h.jobMonitor, h.stepMonitor)
				if err != nil {
					h.logger.Error().Err(err).Str("crawl_job_id", crawlJobID).Msg("Headless crawl execution failed")
					return
				}
				h.logger.Info().
					Str("crawl_job_id", crawlJobID).
					Str("parent_job_id", parentJobID).
					Int("links_count", len(linksToUse)).
					Msg("Headless crawl started via orchestrator")
			}()
		}

		// Return success response immediately
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":      crawlJobID,
			"job_name":    title,
			"status":      "running",
			"message":     fmt.Sprintf("Crawl started: 1 page saved, crawling up to %d more links (headless)", maxPages-1),
			"url":         req.URL,
			"document_id": doc.ID,
			"links_found": len(processedContent.Links),
			"max_pages":   maxPages,
			"mode":        "headless",
		})
		return
	}

	// Parse URL to extract domain
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		h.logger.Error().Err(err).Str("url", req.URL).Msg("Failed to parse URL")
		WriteError(w, http.StatusBadRequest, "Invalid URL")
		return
	}
	siteDomain := parsedURL.Host
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Store authentication cookies if provided and get the auth ID
	var authID string
	if len(req.Cookies) > 0 {
		// Marshal cookies to JSON
		cookiesJSON, err := json.Marshal(req.Cookies)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to marshal cookies")
			WriteError(w, http.StatusInternalServerError, "Failed to store authentication")
			return
		}

		// Create auth credentials with deterministic ID for upsert behavior
		authCreds := &models.AuthCredentials{
			ID:          fmt.Sprintf("auth:generic:%s", siteDomain),
			SiteDomain:  siteDomain,
			ServiceType: "generic", // Generic web authentication
			BaseURL:     baseURL,
			Cookies:     cookiesJSON,
			Tokens:      make(map[string]string),
			Data:        make(map[string]interface{}),
		}

		// Store credentials (will update if exists for same site_domain)
		if err := h.authStorage.StoreCredentials(ctx, authCreds); err != nil {
			h.logger.Error().Err(err).Str("site_domain", siteDomain).Msg("Failed to store auth credentials")
			WriteError(w, http.StatusInternalServerError, "Failed to store authentication")
			return
		}

		h.logger.Debug().
			Str("site_domain", siteDomain).
			Int("cookies_count", len(req.Cookies)).
			Msg("Auth credentials stored for quick crawl")

		// Retrieve the stored credentials to get the ID
		storedCreds, err := h.authStorage.GetCredentialsBySiteDomain(ctx, siteDomain)
		if err != nil {
			h.logger.Error().Err(err).Str("site_domain", siteDomain).Msg("Failed to retrieve stored auth credentials")
			WriteError(w, http.StatusInternalServerError, "Failed to retrieve authentication")
			return
		}
		if storedCreds != nil {
			authID = storedCreds.ID
			h.logger.Debug().
				Str("auth_id", authID).
				Str("site_domain", siteDomain).
				Msg("Retrieved auth ID for job definition")
		}
	}

	// Try to find a matching job definition based on URL patterns
	matchedJobDef, err := h.findMatchingJobDefinition(ctx, req.URL)
	if err != nil {
		h.logger.Warn().Err(err).Str("url", req.URL).Msg("Error searching for matching job definition, using defaults")
		// Continue with ad-hoc job creation
	}

	// If a matching job definition is found, use it directly (don't create a new job definition)
	// Only create a new job definition for ad-hoc (no match) cases
	if matchedJobDef != nil {
		// Create an in-memory copy with runtime overrides for start_urls and auth_id
		// This does NOT create a new job definition in storage
		execJobDef := h.prepareJobDefForExecution(matchedJobDef, req.URL, authID)

		h.logger.Info().
			Str("job_def_id", matchedJobDef.ID).
			Str("job_def_name", matchedJobDef.Name).
			Str("url", req.URL).
			Str("auth_id", authID).
			Msg("Using existing job definition for quick crawl")

		// Extract config values for logging
		var maxDepth, maxPages int
		if len(execJobDef.Steps) > 0 {
			if md, ok := execJobDef.Steps[0].Config["max_depth"].(int); ok {
				maxDepth = md
			}
			if mp, ok := execJobDef.Steps[0].Config["max_pages"].(int); ok {
				maxPages = mp
			}
		}

		// Execute the existing job definition with overrides asynchronously
		go func() {
			bgCtx := context.Background()

			parentJobID, err := h.orchestrator.ExecuteJobDefinition(bgCtx, execJobDef, h.jobMonitor, h.stepMonitor)
			if err != nil {
				h.logger.Error().
					Err(err).
					Str("job_def_id", matchedJobDef.ID).
					Msg("Quick crawl job execution failed")
				return
			}

			h.logger.Debug().
				Str("job_def_id", matchedJobDef.ID).
				Str("parent_job_id", parentJobID).
				Msg("Quick crawl job execution started successfully")
		}()

		// Return response - use the existing job definition ID
		response := map[string]interface{}{
			"job_id":    matchedJobDef.ID,
			"job_name":  matchedJobDef.Name,
			"status":    "running",
			"message":   fmt.Sprintf("Quick crawl started using '%s' job definition", matchedJobDef.Name),
			"url":       req.URL,
			"max_depth": maxDepth,
			"max_pages": maxPages,
		}

		WriteJSON(w, http.StatusAccepted, response)
		return
	}

	// No matching job definition found - create ad-hoc job definition
	jobDef := h.createAdHocJobDef(req.URL, req.Name, req.MaxDepth, req.MaxPages, req.IncludePatterns, req.ExcludePatterns, authID)
	h.logger.Debug().
		Str("url", req.URL).
		Msg("No matching job definition found, creating ad-hoc quick crawl job")

	// Ensure job definition has required fields
	if jobDef.ID == "" {
		jobDef.ID = fmt.Sprintf("capture-crawl-%d", time.Now().UnixNano())
	}
	if jobDef.Name == "" {
		jobDef.Name = fmt.Sprintf("Capture & Crawl: %s", req.URL)
	}
	jobDef.Description = fmt.Sprintf("Capture & Crawl initiated from Chrome extension for %s", req.URL)

	// Validate full job definition
	if err := jobDef.Validate(); err != nil {
		h.logger.Error().Err(err).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid job definition: %v", err))
		return
	}

	// Save ad-hoc job definition
	if err := h.jobDefStorage.SaveJobDefinition(ctx, jobDef); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to save quick crawl job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to save job definition")
		return
	}

	// Extract config values for logging
	var maxDepth, maxPages int
	if len(jobDef.Steps) > 0 {
		if md, ok := jobDef.Steps[0].Config["max_depth"].(int); ok {
			maxDepth = md
		}
		if mp, ok := jobDef.Steps[0].Config["max_pages"].(int); ok {
			maxPages = mp
		}
	}

	h.logger.Debug().
		Str("job_def_id", jobDef.ID).
		Str("job_name", jobDef.Name).
		Str("url", req.URL).
		Int("max_depth", maxDepth).
		Int("max_pages", maxPages).
		Msg("Ad-hoc quick crawl job definition created")

	// Execute job definition asynchronously
	go func() {
		bgCtx := context.Background()

		parentJobID, err := h.orchestrator.ExecuteJobDefinition(bgCtx, jobDef, h.jobMonitor, h.stepMonitor)
		if err != nil {
			h.logger.Error().
				Err(err).
				Str("job_def_id", jobDef.ID).
				Msg("Quick crawl job execution failed")
			return
		}

		h.logger.Debug().
			Str("job_def_id", jobDef.ID).
			Str("parent_job_id", parentJobID).
			Msg("Quick crawl job execution started successfully")
	}()

	// Return response
	response := map[string]interface{}{
		"job_id":    jobDef.ID,
		"job_name":  jobDef.Name,
		"status":    "running",
		"message":   "Ad-hoc quick crawl job created and started",
		"url":       req.URL,
		"max_depth": maxDepth,
		"max_pages": maxPages,
	}

	WriteJSON(w, http.StatusAccepted, response)
}

// DEPRECATED: crawlLinksWithHTTP - Non-headless HTTP crawling has been replaced with
// headless chromedp via orchestrator for better JavaScript rendering and authentication support.
// All capture processes now use the orchestrator pattern (Job Manager → Step → Worker).
// This function is commented out pending testing completion.
/*
func (h *JobDefinitionHandler) crawlLinksWithHTTP(
	crawlJobID string,
	sourceURL string,
	links []string,
	cookies []map[string]interface{},
	maxPages int,
	includePatterns []string,
	excludePatterns []string,
) {
	ctx := context.Background()
	startTime := time.Now()

	h.logger.Info().
		Str("crawl_job_id", crawlJobID).
		Str("source_url", sourceURL).
		Int("links_count", len(links)).
		Int("max_pages", maxPages).
		Msg("Starting HTTP crawl of discovered links")

	// Convert extension cookies to http.Cookie format
	httpCookies := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		name, _ := c["name"].(string)
		value, _ := c["value"].(string)
		domain, _ := c["domain"].(string)
		path, _ := c["path"].(string)
		if name != "" && value != "" {
			httpCookies = append(httpCookies, &http.Cookie{
				Name:   name,
				Value:  value,
				Domain: domain,
				Path:   path,
			})
		}
	}

	// Create HTTP client with cookie jar
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// Parse source URL for cookie domain
	parsedSource, err := url.Parse(sourceURL)
	if err != nil {
		h.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Failed to parse source URL")
		return
	}

	// Set cookies on the jar
	jar.SetCookies(parsedSource, httpCookies)

	// Create link extractor for filtering
	linkExtractor := crawler.NewLinkExtractor(h.logger)

	// Filter links using patterns
	filteredLinks := links
	if len(includePatterns) > 0 || len(excludePatterns) > 0 {
		filterResult := linkExtractor.FilterLinks(links, includePatterns, excludePatterns)
		filteredLinks = filterResult.FilteredLinks
		h.logger.Debug().
			Int("original", len(links)).
			Int("filtered", len(filteredLinks)).
			Msg("Links filtered by patterns")
	}

	// Limit to maxPages
	if len(filteredLinks) > maxPages {
		filteredLinks = filteredLinks[:maxPages]
	}

	// Track crawled URLs to avoid duplicates
	crawledURLs := make(map[string]bool)
	crawledURLs[sourceURL] = true

	contentProcessor := crawler.NewContentProcessor(h.logger)
	successCount := 0
	failCount := 0

	for _, link := range filteredLinks {
		// Skip already crawled
		if crawledURLs[link] {
			continue
		}
		crawledURLs[link] = true

		// Fetch the page
		resp, err := client.Get(link)
		if err != nil {
			h.logger.Debug().Err(err).Str("url", link).Msg("Failed to fetch link")
			failCount++
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			h.logger.Debug().Int("status", resp.StatusCode).Str("url", link).Msg("Non-200 response")
			failCount++
			continue
		}

		// Read body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			h.logger.Debug().Err(err).Str("url", link).Msg("Failed to read response body")
			failCount++
			continue
		}

		// Process HTML
		processedContent, err := contentProcessor.ProcessHTML(string(body), link)
		if err != nil {
			h.logger.Debug().Err(err).Str("url", link).Msg("Failed to process HTML")
			failCount++
			continue
		}

		// Create and save document
		doc := &models.Document{
			ID:              uuid.New().String(),
			SourceType:      "web",
			SourceID:        crawlJobID,
			Title:           processedContent.Title,
			ContentMarkdown: processedContent.Markdown,
			URL:             link,
			Metadata: map[string]interface{}{
				"capture_source": "http_crawl",
				"capture_time":   time.Now().Format(time.RFC3339),
				"crawl_job_id":   crawlJobID,
				"content_size":   len(processedContent.Markdown),
			},
			Tags: []string{"captured", "http-crawl"},
		}

		if err := h.documentService.SaveDocument(ctx, doc); err != nil {
			h.logger.Debug().Err(err).Str("url", link).Msg("Failed to save document")
			failCount++
			continue
		}

		successCount++
		h.logger.Debug().
			Str("url", link).
			Str("title", processedContent.Title).
			Int("size", len(processedContent.Markdown)).
			Msg("Crawled and saved page")
	}

	duration := time.Since(startTime)
	h.logger.Info().
		Str("crawl_job_id", crawlJobID).
		Int("success", successCount).
		Int("failed", failCount).
		Int("total_links", len(filteredLinks)).
		Dur("duration", duration).
		Msg("HTTP crawl completed")
}
*/

// GetMatchingConfigHandler handles GET /api/job-definitions/match-config
// Returns the matching job definition config for a given URL
// Used by Chrome extension to find crawl settings before starting a crawl
func (h *JobDefinitionHandler) GetMatchingConfigHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		WriteError(w, http.StatusBadRequest, "URL parameter is required")
		return
	}

	ctx := r.Context()

	// Find matching job definition
	matchedJobDef, err := h.findMatchingJobDefinition(ctx, targetURL)
	if err != nil {
		h.logger.Error().Err(err).Str("url", targetURL).Msg("Error finding matching job definition")
		WriteError(w, http.StatusInternalServerError, "Error searching job definitions")
		return
	}

	// Build response
	response := map[string]interface{}{
		"url":     targetURL,
		"matched": matchedJobDef != nil,
	}

	if matchedJobDef != nil {
		response["job_definition"] = map[string]interface{}{
			"id":           matchedJobDef.ID,
			"name":         matchedJobDef.Name,
			"description":  matchedJobDef.Description,
			"url_patterns": matchedJobDef.UrlPatterns,
			"tags":         matchedJobDef.Tags,
		}

		// Extract crawler config from first step
		if len(matchedJobDef.Steps) > 0 {
			stepConfig := matchedJobDef.Steps[0].Config

			crawlConfig := map[string]interface{}{}

			// Extract relevant settings
			if md, ok := stepConfig["max_depth"]; ok {
				crawlConfig["max_depth"] = md
			}
			if mp, ok := stepConfig["max_pages"]; ok {
				crawlConfig["max_pages"] = mp
			}
			if fl, ok := stepConfig["follow_links"]; ok {
				crawlConfig["follow_links"] = fl
			}
			if c, ok := stepConfig["concurrency"]; ok {
				crawlConfig["concurrency"] = c
			}

			// Extract patterns
			includePatterns := []string{}
			excludePatterns := []string{}

			if inc, ok := stepConfig["include_patterns"].([]interface{}); ok {
				for _, p := range inc {
					if ps, ok := p.(string); ok {
						includePatterns = append(includePatterns, ps)
					}
				}
			}
			if exc, ok := stepConfig["exclude_patterns"].([]interface{}); ok {
				for _, p := range exc {
					if ps, ok := p.(string); ok {
						excludePatterns = append(excludePatterns, ps)
					}
				}
			}

			crawlConfig["include_patterns"] = includePatterns
			crawlConfig["exclude_patterns"] = excludePatterns

			response["crawl_config"] = crawlConfig
		}

		h.logger.Debug().
			Str("url", targetURL).
			Str("job_def_id", matchedJobDef.ID).
			Str("job_def_name", matchedJobDef.Name).
			Msg("Found matching job definition for URL")
	} else {
		// Return default config when no match
		response["crawl_config"] = map[string]interface{}{
			"max_depth":        2,
			"max_pages":        10,
			"follow_links":     true,
			"concurrency":      5,
			"include_patterns": []string{},
			"exclude_patterns": []string{},
		}

		h.logger.Debug().
			Str("url", targetURL).
			Msg("No matching job definition found, returning defaults")
	}

	WriteJSON(w, http.StatusOK, response)
}

// CrawlWithLinksHandler handles POST /api/job-definitions/crawl-links
// Starts a crawl job using the proper Job Manager → Step → Worker pattern
// This ensures jobs appear in the queue with proper hierarchy and tracking
func (h *JobDefinitionHandler) CrawlWithLinksHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	var req struct {
		StartURL           string                   `json:"start_url"`                   // Current page URL
		Links              []string                 `json:"links"`                       // Pre-filtered links to crawl
		JobDefinitionID    string                   `json:"job_definition_id,omitempty"` // Optional: use existing job def
		Cookies            []map[string]interface{} `json:"cookies,omitempty"`           // Auth cookies from extension
		HTML               string                   `json:"html,omitempty"`              // Current page HTML
		Title              string                   `json:"title,omitempty"`             // Current page title
		IncludeCurrentPage bool                     `json:"include_current_page"`        // Save current page too
		DownloadImages     bool                     `json:"download_images"`             // Download and store images locally
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode crawl-links request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.StartURL == "" {
		WriteError(w, http.StatusBadRequest, "start_url is required")
		return
	}

	ctx := r.Context()

	// Parse URL for job naming and base URL
	parsedURL, err := url.Parse(req.StartURL)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid start_url")
		return
	}
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Try to find or create a job definition
	var jobDef *models.JobDefinition

	if req.JobDefinitionID != "" {
		// Use specified job definition
		jobDef, err = h.jobDefStorage.GetJobDefinition(ctx, req.JobDefinitionID)
		if err != nil {
			h.logger.Warn().Err(err).Str("job_def_id", req.JobDefinitionID).Msg("Failed to load job definition, using defaults")
		}
	}

	if jobDef == nil {
		// Try to find matching job definition
		jobDef, _ = h.findMatchingJobDefinition(ctx, req.StartURL)
	}

	// Require a matching job definition - don't allow crawling without config
	if jobDef == nil {
		h.logger.Warn().
			Str("url", req.StartURL).
			Msg("No matching job definition found for URL - crawling disabled")
		WriteError(w, http.StatusBadRequest, "No matching job definition found for this URL. Create a job definition with matching url_patterns to enable crawling.")
		return
	}

	// Store cookies as authentication if provided
	var authID string
	if len(req.Cookies) > 0 {
		// Generate auth ID based on domain
		authID = fmt.Sprintf("ext_%s_%d", parsedURL.Host, time.Now().UnixNano())

		// Marshal cookies to JSON
		cookiesJSON, err := json.Marshal(req.Cookies)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to marshal cookies")
			WriteError(w, http.StatusInternalServerError, "Failed to store authentication")
			return
		}

		// Create auth credentials
		authCreds := &models.AuthCredentials{
			ID:          authID,
			Name:        fmt.Sprintf("Extension: %s", parsedURL.Host),
			SiteDomain:  parsedURL.Host,
			ServiceType: "generic",
			BaseURL:     baseURL,
			Cookies:     cookiesJSON,
			Tokens:      make(map[string]string),
			Data:        make(map[string]interface{}),
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		}

		if err := h.authStorage.StoreCredentials(ctx, authCreds); err != nil {
			h.logger.Error().Err(err).Msg("Failed to save auth credentials")
			WriteError(w, http.StatusInternalServerError, "Failed to store authentication")
			return
		}

		h.logger.Debug().
			Str("auth_id", authID).
			Str("domain", parsedURL.Host).
			Int("cookies_count", len(req.Cookies)).
			Msg("Stored extension cookies as auth credentials")
	}

	// Build crawl config from job definition (guaranteed to have match at this point)
	maxPages := 50
	maxDepth := 2
	concurrency := 3
	followLinks := true
	downloadImages := req.DownloadImages // Use request value as default, can be overridden by config
	var includePatterns, excludePatterns []string
	tags := jobDef.Tags

	if len(jobDef.Steps) > 0 {
		stepConfig := jobDef.Steps[0].Config
		if mp, ok := stepConfig["max_pages"].(int); ok {
			maxPages = mp
		}
		if md, ok := stepConfig["max_depth"].(int); ok {
			maxDepth = md
		}
		if cc, ok := stepConfig["concurrency"].(int); ok {
			concurrency = cc
		}
		if fl, ok := stepConfig["follow_links"].(bool); ok {
			followLinks = fl
		}
		if di, ok := stepConfig["download_images"].(bool); ok {
			downloadImages = di
		}
		if inc, ok := stepConfig["include_patterns"].([]interface{}); ok {
			for _, p := range inc {
				if ps, ok := p.(string); ok {
					includePatterns = append(includePatterns, ps)
				}
			}
		}
		if exc, ok := stepConfig["exclude_patterns"].([]interface{}); ok {
			for _, p := range exc {
				if ps, ok := p.(string); ok {
					excludePatterns = append(excludePatterns, ps)
				}
			}
		}
	}

	// Default tags if empty
	if len(tags) == 0 {
		tags = []string{"extension", "crawl"}
	}

	// Create an ephemeral job definition for the orchestrator
	ephemeralJobDef := &models.JobDefinition{
		ID:          fmt.Sprintf("ext_crawl_%d", time.Now().UnixNano()),
		Name:        fmt.Sprintf("Crawl: %s", parsedURL.Host),
		Type:        models.JobDefinitionTypeCrawler,
		Description: fmt.Sprintf("Extension-initiated crawl of %s", parsedURL.Host),
		BaseURL:     baseURL,
		SourceType:  "web",
		AuthID:      authID,
		Enabled:     true,
		Timeout:     "30m",
		Tags:        tags,
		Steps: []models.JobStep{
			{
				Name:        "crawl_pages",
				Type:        models.WorkerTypeCrawler,
				Description: fmt.Sprintf("Crawl pages from %s", parsedURL.Host),
				OnError:     models.ErrorStrategyContinue,
				Config: map[string]interface{}{
					"start_urls":       []string{req.StartURL},
					"max_depth":        maxDepth,
					"max_pages":        maxPages,
					"concurrency":      concurrency,
					"follow_links":     followLinks,
					"download_images":  downloadImages,
					"include_patterns": includePatterns,
					"exclude_patterns": excludePatterns,
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// If we have pre-filtered links, use them as seed URLs instead of discovery
	if len(req.Links) > 0 {
		// Limit links to maxPages
		linksToUse := req.Links
		if len(linksToUse) > maxPages {
			linksToUse = linksToUse[:maxPages]
		}

		// Include start URL if requested
		seedURLs := linksToUse
		if req.IncludeCurrentPage && req.StartURL != "" {
			// Prepend start URL
			seedURLs = append([]string{req.StartURL}, linksToUse...)
		}

		ephemeralJobDef.Steps[0].Config["start_urls"] = seedURLs
		ephemeralJobDef.Steps[0].Config["follow_links"] = false // Don't discover more links
		ephemeralJobDef.Steps[0].Config["max_depth"] = 1        // Flat crawl of provided links
	}

	h.logger.Info().
		Str("job_def_id", ephemeralJobDef.ID).
		Str("job_name", ephemeralJobDef.Name).
		Str("start_url", req.StartURL).
		Int("links_count", len(req.Links)).
		Bool("include_current", req.IncludeCurrentPage).
		Str("auth_id", authID).
		Msg("Starting extension crawl via orchestrator")

	// Execute via orchestrator (proper Manager → Step → Worker pattern)
	go func() {
		bgCtx := context.Background()

		parentJobID, err := h.orchestrator.ExecuteJobDefinition(bgCtx, ephemeralJobDef, h.jobMonitor, h.stepMonitor)
		if err != nil {
			h.logger.Error().
				Err(err).
				Str("job_def_id", ephemeralJobDef.ID).
				Msg("Extension crawl execution failed")
			return
		}

		h.logger.Info().
			Str("job_def_id", ephemeralJobDef.ID).
			Str("parent_job_id", parentJobID).
			Msg("Extension crawl started successfully")
	}()

	// Calculate total pages
	totalPages := len(req.Links)
	if req.IncludeCurrentPage {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1 // At least start URL
	}

	response := map[string]interface{}{
		"job_id":         ephemeralJobDef.ID,
		"job_name":       ephemeralJobDef.Name,
		"status":         "running",
		"start_url":      req.StartURL,
		"links_to_crawl": totalPages,
		"max_pages":      maxPages,
		"message":        fmt.Sprintf("Crawl job started via orchestrator: %d pages to process", totalPages),
	}

	if jobDef != nil {
		response["matched_config"] = jobDef.Name
	}
	if authID != "" {
		response["auth_id"] = authID
	}

	WriteJSON(w, http.StatusAccepted, response)
}

