package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// GitHubJobsHandler handles GitHub job-related API endpoints
type GitHubJobsHandler struct {
	connectorService interfaces.ConnectorService
	jobMgr           *queue.Manager
	queueMgr         interfaces.QueueManager
	jobMonitor       interfaces.JobMonitor
	stepMonitor      interfaces.StepMonitor
	logger           arbor.ILogger
}

// NewGitHubJobsHandler creates a new GitHub jobs handler
func NewGitHubJobsHandler(
	connectorService interfaces.ConnectorService,
	jobMgr *queue.Manager,
	queueMgr interfaces.QueueManager,
	jobMonitor interfaces.JobMonitor,
	stepMonitor interfaces.StepMonitor,
	logger arbor.ILogger,
) *GitHubJobsHandler {
	return &GitHubJobsHandler{
		connectorService: connectorService,
		jobMgr:           jobMgr,
		queueMgr:         queueMgr,
		jobMonitor:       jobMonitor,
		stepMonitor:      stepMonitor,
		logger:           logger,
	}
}

// resolveConnector resolves a connector by ID or name
// connector_id takes precedence if both are provided
func (h *GitHubJobsHandler) resolveConnector(ctx context.Context, connectorID, connectorName string) (*models.Connector, error) {
	// connector_id takes precedence
	if connectorID != "" {
		return h.connectorService.GetConnector(ctx, connectorID)
	}

	// Fall back to connector_name
	if connectorName != "" {
		return h.connectorService.GetConnectorByName(ctx, connectorName)
	}

	return nil, fmt.Errorf("either connector_id or connector_name is required")
}

// Request/Response Types

// PreviewRepoRequest - request to preview repo files
type PreviewRepoRequest struct {
	ConnectorID   string   `json:"connector_id"`
	ConnectorName string   `json:"connector_name"`
	Owner         string   `json:"owner"`
	Repo          string   `json:"repo"`
	Branches      []string `json:"branches"`
	Extensions    []string `json:"extensions"`
	ExcludePaths  []string `json:"exclude_paths"`
}

// PreviewRepoResponse - response with file list
type PreviewRepoResponse struct {
	Files      []RepoFilePreview `json:"files"`
	TotalCount int               `json:"total_count"`
	Branches   []string          `json:"branches"`
}

// RepoFilePreview represents a file in the preview
type RepoFilePreview struct {
	Path   string `json:"path"`
	Folder string `json:"folder"`
	Size   int    `json:"size"`
	Branch string `json:"branch"`
}

// PreviewActionsRequest - request to preview action runs
type PreviewActionsRequest struct {
	ConnectorID   string `json:"connector_id"`
	ConnectorName string `json:"connector_name"`
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	Limit         int    `json:"limit"`
	StatusFilter  string `json:"status_filter"`
	BranchFilter  string `json:"branch_filter"`
}

// PreviewActionsResponse - response with run list
type PreviewActionsResponse struct {
	Runs       []ActionRunPreview `json:"runs"`
	TotalCount int                `json:"total_count"`
}

// ActionRunPreview represents a workflow run in the preview
type ActionRunPreview struct {
	ID           int64  `json:"id"`
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	Branch       string `json:"branch"`
	StartedAt    string `json:"started_at"`
}

// StartRepoJobRequest - request to start a repo collector job
type StartRepoJobRequest struct {
	ConnectorID   string   `json:"connector_id"`
	ConnectorName string   `json:"connector_name"`
	Owner         string   `json:"owner"`
	Repo          string   `json:"repo"`
	Tags          []string `json:"tags"`
	Branches      []string `json:"branches,omitempty"`
	Extensions    []string `json:"extensions,omitempty"`
	ExcludePaths  []string `json:"exclude_paths,omitempty"`
	MaxFiles      int      `json:"max_files,omitempty"`
}

// StartActionsJobRequest - request to start an actions log collector job
type StartActionsJobRequest struct {
	ConnectorID   string   `json:"connector_id"`
	ConnectorName string   `json:"connector_name"`
	Owner         string   `json:"owner"`
	Repo          string   `json:"repo"`
	Tags          []string `json:"tags"`
	Limit         int      `json:"limit,omitempty"`
	StatusFilter  string   `json:"status_filter,omitempty"`
	BranchFilter  string   `json:"branch_filter,omitempty"`
}

// StartJobResponse - response with job ID
type StartJobResponse struct {
	JobID   string `json:"job_id"`
	Message string `json:"message"`
}

// PreviewRepoFilesHandler handles POST /api/github/repo/preview
func (h *GitHubJobsHandler) PreviewRepoFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PreviewRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ConnectorID == "" && req.ConnectorName == "" {
		http.Error(w, "either connector_id or connector_name is required", http.StatusBadRequest)
		return
	}
	if req.Owner == "" {
		http.Error(w, "owner is required", http.StatusBadRequest)
		return
	}
	if req.Repo == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if len(req.Branches) == 0 {
		req.Branches = []string{"main"}
	}
	if len(req.Extensions) == 0 {
		req.Extensions = []string{".go", ".ts", ".tsx", ".js", ".jsx", ".md", ".yaml", ".yml", ".toml", ".json"}
	}
	if len(req.ExcludePaths) == 0 {
		req.ExcludePaths = []string{"vendor/", "node_modules/", ".git/", "dist/", "build/"}
	}

	// Get connector (connector_id takes precedence over connector_name)
	connector, err := h.resolveConnector(r.Context(), req.ConnectorID, req.ConnectorName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Connector not found: %v", err), http.StatusNotFound)
		return
	}

	ghConnector, err := github.NewConnector(connector)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create GitHub connector: %v", err), http.StatusInternalServerError)
		return
	}

	// List files for each branch
	var allFiles []RepoFilePreview
	for _, branch := range req.Branches {
		files, err := ghConnector.ListFiles(r.Context(), req.Owner, req.Repo, branch, req.Extensions, req.ExcludePaths)
		if err != nil {
			h.logger.Warn().Err(err).Str("branch", branch).Msg("Failed to list files for branch")
			continue
		}

		for _, f := range files {
			allFiles = append(allFiles, RepoFilePreview{
				Path:   f.Path,
				Folder: f.Folder,
				Size:   f.Size,
				Branch: branch,
			})
		}
	}

	resp := PreviewRepoResponse{
		Files:      allFiles,
		TotalCount: len(allFiles),
		Branches:   req.Branches,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PreviewActionRunsHandler handles POST /api/github/actions/preview
func (h *GitHubJobsHandler) PreviewActionRunsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PreviewActionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ConnectorID == "" && req.ConnectorName == "" {
		http.Error(w, "either connector_id or connector_name is required", http.StatusBadRequest)
		return
	}
	if req.Owner == "" {
		http.Error(w, "owner is required", http.StatusBadRequest)
		return
	}
	if req.Repo == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	// Get connector (connector_id takes precedence over connector_name)
	connector, err := h.resolveConnector(r.Context(), req.ConnectorID, req.ConnectorName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Connector not found: %v", err), http.StatusNotFound)
		return
	}

	ghConnector, err := github.NewConnector(connector)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create GitHub connector: %v", err), http.StatusInternalServerError)
		return
	}

	// List workflow runs
	runs, err := ghConnector.ListWorkflowRuns(r.Context(), req.Owner, req.Repo, req.Limit, req.StatusFilter, req.BranchFilter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list workflow runs: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to preview response
	var previewRuns []ActionRunPreview
	for _, run := range runs {
		previewRuns = append(previewRuns, ActionRunPreview{
			ID:           run.ID,
			WorkflowName: run.WorkflowName,
			Status:       run.Status,
			Conclusion:   run.Conclusion,
			Branch:       run.Branch,
			StartedAt:    run.RunStartedAt.Format(time.RFC3339),
		})
	}

	resp := PreviewActionsResponse{
		Runs:       previewRuns,
		TotalCount: len(previewRuns),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// StartRepoCollectorHandler handles POST /api/github/repo/start
func (h *GitHubJobsHandler) StartRepoCollectorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartRepoJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ConnectorID == "" && req.ConnectorName == "" {
		http.Error(w, "either connector_id or connector_name is required", http.StatusBadRequest)
		return
	}
	if req.Owner == "" {
		http.Error(w, "owner is required", http.StatusBadRequest)
		return
	}
	if req.Repo == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if len(req.Branches) == 0 {
		req.Branches = []string{"main"}
	}
	if len(req.Extensions) == 0 {
		req.Extensions = []string{".go", ".ts", ".tsx", ".js", ".jsx", ".md", ".yaml", ".yml", ".toml", ".json"}
	}
	if len(req.ExcludePaths) == 0 {
		req.ExcludePaths = []string{"vendor/", "node_modules/", ".git/", "dist/", "build/"}
	}
	if req.MaxFiles == 0 {
		req.MaxFiles = 1000
	}

	// Resolve connector (connector_id takes precedence over connector_name)
	connector, err := h.resolveConnector(r.Context(), req.ConnectorID, req.ConnectorName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Connector not found: %v", err), http.StatusNotFound)
		return
	}

	// Create job definition for this run (always use connector ID in job config)
	jobDef := &models.JobDefinition{
		ID:          fmt.Sprintf("github-repo-%s", uuid.New().String()[:8]),
		Name:        fmt.Sprintf("GitHub Repo: %s/%s", req.Owner, req.Repo),
		Type:        "custom",
		JobType:     "user",
		SourceType:  "github",
		Description: fmt.Sprintf("Fetch repository content from %s/%s", req.Owner, req.Repo),
		Tags:        req.Tags,
		Timeout:     "30m",
		Enabled:     true,
		Steps: []models.JobStep{
			{
				Name:    "fetch_repo_content",
				Type:    models.WorkerTypeGitHubRepo,
				OnError: "continue",
				Config: map[string]interface{}{
					"connector_id":  connector.ID,
					"owner":         req.Owner,
					"repo":          req.Repo,
					"branches":      req.Branches,
					"extensions":    req.Extensions,
					"exclude_paths": req.ExcludePaths,
					"max_files":     req.MaxFiles,
				},
			},
		},
	}

	// Execute the job definition via JobManager
	jobID, err := h.jobMgr.ExecuteJobDefinition(r.Context(), jobDef, h.jobMonitor, h.stepMonitor)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to execute GitHub repo collector job")
		http.Error(w, fmt.Sprintf("Failed to start job: %v", err), http.StatusInternalServerError)
		return
	}

	resp := StartJobResponse{
		JobID:   jobID,
		Message: fmt.Sprintf("GitHub repo collector job started for %s/%s", req.Owner, req.Repo),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// StartActionsCollectorHandler handles POST /api/github/actions/start
func (h *GitHubJobsHandler) StartActionsCollectorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartActionsJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ConnectorID == "" && req.ConnectorName == "" {
		http.Error(w, "either connector_id or connector_name is required", http.StatusBadRequest)
		return
	}
	if req.Owner == "" {
		http.Error(w, "owner is required", http.StatusBadRequest)
		return
	}
	if req.Repo == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	// Resolve connector (connector_id takes precedence over connector_name)
	connector, err := h.resolveConnector(r.Context(), req.ConnectorID, req.ConnectorName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Connector not found: %v", err), http.StatusNotFound)
		return
	}

	// Create job definition for this run (always use connector ID in job config)
	jobDef := &models.JobDefinition{
		ID:          fmt.Sprintf("github-actions-%s", uuid.New().String()[:8]),
		Name:        fmt.Sprintf("GitHub Actions: %s/%s", req.Owner, req.Repo),
		Type:        "custom",
		JobType:     "user",
		SourceType:  "github",
		Description: fmt.Sprintf("Fetch GitHub Actions logs from %s/%s", req.Owner, req.Repo),
		Tags:        req.Tags,
		Timeout:     "15m",
		Enabled:     true,
		Steps: []models.JobStep{
			{
				Name:    "fetch_action_logs",
				Type:    models.WorkerTypeGitHubActions,
				OnError: "continue",
				Config: map[string]interface{}{
					"connector_id":  connector.ID,
					"owner":         req.Owner,
					"repo":          req.Repo,
					"limit":         req.Limit,
					"status_filter": req.StatusFilter,
					"branch_filter": req.BranchFilter,
				},
			},
		},
	}

	// Execute the job definition via JobManager
	jobID, err := h.jobMgr.ExecuteJobDefinition(r.Context(), jobDef, h.jobMonitor, h.stepMonitor)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to execute GitHub actions collector job")
		http.Error(w, fmt.Sprintf("Failed to start job: %v", err), http.StatusInternalServerError)
		return
	}

	resp := StartJobResponse{
		JobID:   jobID,
		Message: fmt.Sprintf("GitHub actions collector job started for %s/%s", req.Owner, req.Repo),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}
