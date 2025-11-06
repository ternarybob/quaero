package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/jobs/executor"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/storage/sqlite"
)

var ErrJobDefinitionNotFound = errors.New("job definition not found")

// JobDefinitionHandler handles HTTP requests for job definition management
type JobDefinitionHandler struct {
	jobDefStorage interfaces.JobDefinitionStorage
	jobStorage    interfaces.JobStorage
	jobExecutor   *executor.JobExecutor
	logger        arbor.ILogger
}

// NewJobDefinitionHandler creates a new job definition handler
func NewJobDefinitionHandler(
	jobDefStorage interfaces.JobDefinitionStorage,
	jobStorage interfaces.JobStorage,
	jobExecutor *executor.JobExecutor,
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
	if logger == nil {
		panic("logger cannot be nil")
	}

	logger.Info().Msg("Job definition handler initialized with job executor")

	return &JobDefinitionHandler{
		jobDefStorage: jobDefStorage,
		jobStorage:    jobStorage,
		jobExecutor:   jobExecutor,
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

	ctx := r.Context()

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

	ctx := r.Context()

	// Check if job definition exists and is not a system job
	existingJobDef, err := h.jobDefStorage.GetJobDefinition(ctx, id)
	if err != nil {
		if err == sqlite.ErrJobDefinitionNotFound {
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

	// Check if job definition exists and is not a system job
	existingJobDef, err := h.jobDefStorage.GetJobDefinition(ctx, id)
	if err != nil {
		if err == sqlite.ErrJobDefinitionNotFound {
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
		if err == sqlite.ErrJobDefinitionNotFound {
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
	tomlData, err := h.convertJobDefinitionToTOML(jobDef)
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

	h.logger.Info().Str("job_def_id", id).Str("filename", filename).Msg("Exporting job definition as TOML")

	w.WriteHeader(http.StatusOK)
	w.Write(tomlData)
}

// convertJobDefinitionToTOML converts a JobDefinition to simplified TOML format
func (h *JobDefinitionHandler) convertJobDefinitionToTOML(jobDef *models.JobDefinition) ([]byte, error) {
	// Extract crawler configuration from first step
	var crawlConfig map[string]interface{}
	if len(jobDef.Steps) > 0 && jobDef.Steps[0].Action == "crawl" {
		crawlConfig = jobDef.Steps[0].Config
	} else {
		crawlConfig = make(map[string]interface{})
	}

	// Build simplified structure matching the file format
	simplified := map[string]interface{}{
		"id":          jobDef.ID,
		"name":        jobDef.Name,
		"description": jobDef.Description,
		"schedule":    jobDef.Schedule,
		"timeout":     jobDef.Timeout,
		"enabled":     jobDef.Enabled,
		"auto_start":  jobDef.AutoStart,
	}

	// Extract crawler-specific fields from config
	if startURLs, ok := crawlConfig["start_urls"].([]interface{}); ok {
		urls := make([]string, 0, len(startURLs))
		for _, url := range startURLs {
			if urlStr, ok := url.(string); ok {
				urls = append(urls, urlStr)
			}
		}
		simplified["start_urls"] = urls
	} else {
		simplified["start_urls"] = []string{}
	}

	if includePatterns, ok := crawlConfig["include_patterns"].([]interface{}); ok {
		patterns := make([]string, 0, len(includePatterns))
		for _, pattern := range includePatterns {
			if patternStr, ok := pattern.(string); ok {
				patterns = append(patterns, patternStr)
			}
		}
		simplified["include_patterns"] = patterns
	} else {
		simplified["include_patterns"] = []string{}
	}

	if excludePatterns, ok := crawlConfig["exclude_patterns"].([]interface{}); ok {
		patterns := make([]string, 0, len(excludePatterns))
		for _, pattern := range excludePatterns {
			if patternStr, ok := pattern.(string); ok {
				patterns = append(patterns, patternStr)
			}
		}
		simplified["exclude_patterns"] = patterns
	} else {
		simplified["exclude_patterns"] = []string{}
	}

	// Extract numeric fields with defaults
	if maxDepth, ok := crawlConfig["max_depth"].(float64); ok {
		simplified["max_depth"] = int(maxDepth)
	} else {
		simplified["max_depth"] = 2
	}

	if maxPages, ok := crawlConfig["max_pages"].(float64); ok {
		simplified["max_pages"] = int(maxPages)
	} else {
		simplified["max_pages"] = 100
	}

	if concurrency, ok := crawlConfig["concurrency"].(float64); ok {
		simplified["concurrency"] = int(concurrency)
	} else {
		simplified["concurrency"] = 5
	}

	if followLinks, ok := crawlConfig["follow_links"].(bool); ok {
		simplified["follow_links"] = followLinks
	} else {
		simplified["follow_links"] = true
	}

	// Marshal to TOML
	tomlData, err := toml.Marshal(simplified)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to TOML: %w", err)
	}

	return tomlData, nil
}

// ValidateJobDefinitionTOMLHandler handles POST /api/job-definitions/validate
// Validates TOML content without creating the job definition
func (h *JobDefinitionHandler) ValidateJobDefinitionTOMLHandler(w http.ResponseWriter, r *http.Request) {
	// Read TOML content from request body
	tomlContent, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to read request body")
		WriteError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Parse TOML into CrawlerJobDefinitionFile
	var crawlerJob sqlite.CrawlerJobDefinitionFile
	if err := toml.Unmarshal(tomlContent, &crawlerJob); err != nil {
		h.logger.Error().Err(err).Msg("Invalid TOML syntax")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid TOML syntax: %v", err))
		return
	}

	// Validate crawler job file
	if err := crawlerJob.Validate(); err != nil {
		h.logger.Error().Err(err).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Convert to full JobDefinition model
	jobDef := crawlerJob.ToJobDefinition()

	// Validate full job definition
	if err := jobDef.Validate(); err != nil {
		h.logger.Error().Err(err).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Validate step actions are registered
	if err := h.validateStepActions(jobDef.Type, jobDef.Steps); err != nil {
		h.logger.Error().Err(err).Msg("Action validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid action: %v", err))
		return
	}

	h.logger.Info().Str("job_def_id", jobDef.ID).Msg("Job definition TOML validated successfully")
	WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "valid",
		"message": "Job definition is valid and ready to create",
	})
}

// UploadJobDefinitionTOMLHandler handles POST /api/job-definitions/upload
// Creates a job definition from TOML content
func (h *JobDefinitionHandler) UploadJobDefinitionTOMLHandler(w http.ResponseWriter, r *http.Request) {
	// Read TOML content from request body
	tomlContent, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to read request body")
		WriteError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Parse TOML into CrawlerJobDefinitionFile
	var crawlerJob sqlite.CrawlerJobDefinitionFile
	if err := toml.Unmarshal(tomlContent, &crawlerJob); err != nil {
		h.logger.Error().Err(err).Msg("Invalid TOML syntax")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid TOML syntax: %v", err))
		return
	}

	// Validate crawler job file
	if err := crawlerJob.Validate(); err != nil {
		h.logger.Error().Err(err).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Convert to full JobDefinition model
	jobDef := crawlerJob.ToJobDefinition()

	// Validate full job definition
	if err := jobDef.Validate(); err != nil {
		h.logger.Error().Err(err).Msg("Job definition validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Validate step actions are registered
	if err := h.validateStepActions(jobDef.Type, jobDef.Steps); err != nil {
		h.logger.Error().Err(err).Msg("Action validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid action: %v", err))
		return
	}

	// Save job definition
	ctx := r.Context()
	if err := h.jobDefStorage.SaveJobDefinition(ctx, jobDef); err != nil {
		h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to save job definition")
		WriteError(w, http.StatusInternalServerError, "Failed to save job definition")
		return
	}

	h.logger.Info().Str("job_def_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition created from TOML upload")
	WriteJSON(w, http.StatusCreated, jobDef)
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

	h.logger.Info().Str("job_def_id", jobDef.ID).Msg("Invalid job definition saved without validation")
	WriteJSON(w, http.StatusCreated, jobDef)
}
