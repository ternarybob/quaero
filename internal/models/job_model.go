// -----------------------------------------------------------------------
// Queue Job - Immutable job structure for queue persistence
// -----------------------------------------------------------------------

package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// QueueJob represents the immutable job sent to the queue and stored in the database.
// Once created and enqueued, this job should not be modified.
// All job types (manager, step, job, crawler, summarizer, etc.) use this common structure.
//
// Job Hierarchy (3-level):
//   - Manager (type="manager"): Top-level orchestrator, monitors steps
//   - Step (type="step"): Step container, monitors its spawned jobs
//   - Job (type=various): Individual work units, children of steps
//
// Job State Lifecycle:
//  1. Job/JobDefinition (jobs page) - User-defined workflow
//  2. QueueJob (this struct) - Immutable job sent to queue for execution
//  3. QueueJobState - In-memory runtime state during execution (Status, Progress)
//  4. Job logs/events - Runtime state changes tracked via JobMonitor/StepMonitor
type QueueJob struct {
	// Core identification
	ID        string  `json:"id"`                   // Unique job ID (UUID)
	ParentID  *string `json:"parent_id"`            // Parent job ID (step for jobs, manager for steps, nil for managers)
	ManagerID *string `json:"manager_id,omitempty"` // Top-level manager ID (allows jobs to reference manager directly)

	// Job classification
	Type string `json:"type"` // Job type: "manager", "step", "crawler", "summarizer", etc.
	Name string `json:"name"` // Human-readable job name

	// Configuration (immutable snapshot at creation time)
	Config map[string]interface{} `json:"config"` // Job-specific configuration

	// Metadata
	Metadata map[string]interface{} `json:"metadata"` // Additional metadata (job_definition_id, step_name, etc.)

	// Timestamps
	CreatedAt time.Time `json:"created_at"` // Job creation timestamp

	// Hierarchy tracking
	Depth int `json:"depth"` // Depth in job tree (0=manager, 1=step, 2=job)
}

const (
	// GitHub Action Log types
	SourceTypeGitHubActionLog = "github_action_log"
	JobTypeGitHubActionLog    = "github_action_log"

	// GitHub Repository types (API-based)
	SourceTypeGitHubRepo  = "github_repo"
	JobTypeGitHubRepoFile = "github_repo_file"

	// GitHub Git types (git clone-based, faster for full repo downloads)
	SourceTypeGitHubGit   = "github_git"
	JobTypeGitHubGitFile  = "github_git_file"
	JobTypeGitHubGitBatch = "github_git_batch" // Batch job containing multiple files

	// Local directory types (local filesystem indexing)
	SourceTypeLocalDir   = "local_dir"
	JobTypeLocalDirBatch = "local_dir_batch" // Batch job containing multiple local files

	// Code map types (hierarchical code structure analysis)
	SourceTypeCodeMap       = "code_map"
	JobTypeCodeMapStructure = "code_map_structure" // Directory structure extraction job
	JobTypeCodeMapSummary   = "code_map_summary"   // AI summarization job for directories/files
)

// NewQueueJob creates a new root queued job
func NewQueueJob(jobType, name string, config, metadata map[string]interface{}) *QueueJob {
	return &QueueJob{
		ID:        uuid.New().String(),
		ParentID:  nil,
		Type:      jobType,
		Name:      name,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		Depth:     0,
	}
}

// NewQueueJobChild creates a new child queued job (legacy - use NewQueueJobForStep for new code)
func NewQueueJobChild(parentID string, jobType JobType, name string, config, metadata map[string]interface{}, depth int) *QueueJob {
	return &QueueJob{
		ID:        uuid.New().String(),
		ParentID:  &parentID,
		Type:      string(jobType),
		Name:      name,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		Depth:     depth,
	}
}

// NewQueueManager creates a new manager job (top-level orchestrator)
func NewQueueManager(name string, config, metadata map[string]interface{}) *QueueJob {
	return &QueueJob{
		ID:        uuid.New().String(),
		ParentID:  nil,
		ManagerID: nil, // Manager has no manager
		Type:      string(JobTypeManager),
		Name:      name,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		Depth:     0, // Manager is at depth 0
	}
}

// NewQueueStep creates a new step job (child of manager, parent of jobs)
func NewQueueStep(managerID, stepName string, config, metadata map[string]interface{}) *QueueJob {
	return &QueueJob{
		ID:        uuid.New().String(),
		ParentID:  &managerID,
		ManagerID: &managerID,
		Type:      string(JobTypeStep),
		Name:      stepName,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		Depth:     1, // Step is at depth 1
	}
}

// NewQueueJobForStep creates a new job under a step
func NewQueueJobForStep(stepID, managerID string, jobType JobType, name string, config, metadata map[string]interface{}) *QueueJob {
	return &QueueJob{
		ID:        uuid.New().String(),
		ParentID:  &stepID,
		ManagerID: &managerID,
		Type:      string(jobType),
		Name:      name,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		Depth:     2, // Job is at depth 2
	}
}

// IsRootJob returns true if this is a root job (no parent) - same as IsManager
func (j *QueueJob) IsRootJob() bool {
	return j.ParentID == nil
}

// IsManager returns true if this is a manager job (top-level orchestrator)
func (j *QueueJob) IsManager() bool {
	return j.Type == string(JobTypeManager)
}

// IsStep returns true if this is a step job
func (j *QueueJob) IsStep() bool {
	return j.Type == string(JobTypeStep)
}

// IsJob returns true if this is a work job (not manager or step)
func (j *QueueJob) IsJob() bool {
	return !j.IsManager() && !j.IsStep()
}

// GetParentID returns the parent ID or empty string if root job
func (j *QueueJob) GetParentID() string {
	if j.ParentID == nil {
		return ""
	}
	return *j.ParentID
}

// GetManagerID returns the manager ID or empty string if not set
func (j *QueueJob) GetManagerID() string {
	if j.ManagerID == nil {
		return ""
	}
	return *j.ManagerID
}

// ToJSON serializes the queued job to JSON for queue storage
func (j *QueueJob) ToJSON() ([]byte, error) {
	data, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal queued job: %w", err)
	}
	return data, nil
}

// QueueJobFromJSON deserializes a queued job from JSON
func QueueJobFromJSON(data []byte) (*QueueJob, error) {
	var job QueueJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queued job: %w", err)
	}
	return &job, nil
}

// Validate validates the queued job
func (j *QueueJob) Validate() error {
	if j.ID == "" {
		return fmt.Errorf("job ID is required")
	}
	if j.Type == "" {
		return fmt.Errorf("job type is required")
	}
	if j.Name == "" {
		return fmt.Errorf("job name is required")
	}
	if j.Config == nil {
		return fmt.Errorf("job config cannot be nil")
	}
	if j.Metadata == nil {
		return fmt.Errorf("job metadata cannot be nil")
	}
	if j.Depth < 0 {
		return fmt.Errorf("job depth cannot be negative")
	}
	return nil
}

// Clone creates a deep copy of the queued job (useful for creating child jobs)
func (j *QueueJob) Clone() *QueueJob {
	// Deep copy config
	configCopy := make(map[string]interface{})
	for k, v := range j.Config {
		configCopy[k] = v
	}

	// Deep copy metadata
	metadataCopy := make(map[string]interface{})
	for k, v := range j.Metadata {
		metadataCopy[k] = v
	}

	clone := &QueueJob{
		ID:        j.ID,
		ParentID:  j.ParentID,
		ManagerID: j.ManagerID,
		Type:      j.Type,
		Name:      j.Name,
		Config:    configCopy,
		Metadata:  metadataCopy,
		CreatedAt: j.CreatedAt,
		Depth:     j.Depth,
	}

	return clone
}

// GetConfigString retrieves a string value from config
func (j *QueueJob) GetConfigString(key string) (string, bool) {
	val, ok := j.Config[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetConfigInt retrieves an int value from config
func (j *QueueJob) GetConfigInt(key string) (int, bool) {
	val, ok := j.Config[key]
	if !ok {
		return 0, false
	}

	// Handle both int and float64 (JSON unmarshaling converts numbers to float64)
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetConfigBool retrieves a bool value from config
func (j *QueueJob) GetConfigBool(key string) (bool, bool) {
	val, ok := j.Config[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetConfigStringSlice retrieves a string slice from config
func (j *QueueJob) GetConfigStringSlice(key string) ([]string, bool) {
	val, ok := j.Config[key]
	if !ok {
		return nil, false
	}

	// Handle []interface{} from JSON unmarshaling
	switch v := val.(type) {
	case []string:
		return v, true
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, false
			}
			result[i] = str
		}
		return result, true
	default:
		return nil, false
	}
}

// GetMetadataString retrieves a string value from metadata
func (j *QueueJob) GetMetadataString(key string) (string, bool) {
	val, ok := j.Metadata[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// SetMetadata sets a metadata value (use sparingly - queued job should be immutable after creation)
func (j *QueueJob) SetMetadata(key string, value interface{}) {
	if j.Metadata == nil {
		j.Metadata = make(map[string]interface{})
	}
	j.Metadata[key] = value
}

// -----------------------------------------------------------------------
// Queue Job State - Runtime job state (combines QueueJob with execution state)
// -----------------------------------------------------------------------

// JobProgress tracks job execution progress
type JobProgress struct {
	TotalURLs     int     `json:"total_urls"`
	CompletedURLs int     `json:"completed_urls"`
	FailedURLs    int     `json:"failed_urls"`
	PendingURLs   int     `json:"pending_urls"`
	CurrentURL    string  `json:"current_url,omitempty"`
	Percentage    float64 `json:"percentage"`
}

// QueueJobState represents a job with runtime execution state (in-memory only)
// This combines the immutable QueueJob fields with mutable runtime state
// Runtime state (Status, Progress) should be tracked via job logs/events, not stored in database
//
// Job Hierarchy (3-level):
//   - Manager (type="manager"): Top-level orchestrator, monitors steps
//   - Step (type="step"): Step container, monitors its spawned jobs
//   - Job (type=various): Individual work units, children of steps
//
// Job State Lifecycle:
//  1. Job/JobDefinition (jobs page) - User-defined workflow
//  2. QueueJob - Immutable job sent to queue for execution (stored in database)
//  3. QueueJobState (this struct) - In-memory runtime state during execution
//  4. Job logs/events - Runtime state changes tracked via JobMonitor/StepMonitor
type QueueJobState struct {
	// Core identification (from QueueJob)
	ID        string  `json:"id"`                   // Unique job ID (UUID)
	ParentID  *string `json:"parent_id"`            // Parent job ID (step for jobs, manager for steps, nil for managers)
	ManagerID *string `json:"manager_id,omitempty"` // Top-level manager ID

	// Job classification (from QueueJob)
	Type string `json:"type"` // Job type: "manager", "step", "crawler", "summarizer", etc.
	Name string `json:"name"` // Human-readable job name

	// Configuration (from QueueJob)
	Config   map[string]interface{} `json:"config"`   // Job-specific configuration
	Metadata map[string]interface{} `json:"metadata"` // Additional metadata

	// Timestamps (from QueueJob)
	CreatedAt time.Time `json:"created_at"` // Job creation timestamp

	// Hierarchy tracking (from QueueJob)
	Depth int `json:"depth"` // Depth in job tree (0=manager, 1=step, 2=job)

	// Mutable runtime state (tracked via job logs/events)
	Status        JobStatus   `json:"status"`
	Progress      JobProgress `json:"progress"` // Value type (not pointer)
	StartedAt     *time.Time  `json:"started_at,omitempty"`
	CompletedAt   *time.Time  `json:"completed_at,omitempty"`
	FinishedAt    *time.Time  `json:"finished_at,omitempty"`
	LastHeartbeat *time.Time  `json:"last_heartbeat,omitempty"`
	Error         string      `json:"error,omitempty"`
	ResultCount   int         `json:"result_count"`
	FailedCount   int         `json:"failed_count"`
}

// NewQueueJobState creates a new job execution state from a QueueJob
func NewQueueJobState(queued *QueueJob) *QueueJobState {
	// Ensure Config and Metadata are never nil
	config := queued.Config
	if config == nil {
		config = make(map[string]interface{})
	}
	metadata := queued.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &QueueJobState{
		// Copy fields from QueueJob
		ID:        queued.ID,
		ParentID:  queued.ParentID,
		ManagerID: queued.ManagerID,
		Type:      queued.Type,
		Name:      queued.Name,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: queued.CreatedAt,
		Depth:     queued.Depth,
		// Initialize runtime state
		Status:      JobStatusPending,
		Progress:    JobProgress{}, // Initialize to empty struct (not pointer)
		ResultCount: 0,
		FailedCount: 0,
	}
}

// ToQueueJob extracts the immutable QueueJob from a QueueJobState
func (j *QueueJobState) ToQueueJob() *QueueJob {
	return &QueueJob{
		ID:        j.ID,
		ParentID:  j.ParentID,
		ManagerID: j.ManagerID,
		Type:      j.Type,
		Name:      j.Name,
		Config:    j.Config,
		Metadata:  j.Metadata,
		CreatedAt: j.CreatedAt,
		Depth:     j.Depth,
	}
}

// UpdateProgress updates the job progress and percentage
func (j *QueueJobState) UpdateProgress(completed, failed, pending, total int) {
	j.Progress.CompletedURLs = completed
	j.Progress.FailedURLs = failed
	j.Progress.PendingURLs = pending
	j.Progress.TotalURLs = total

	if total > 0 {
		j.Progress.Percentage = float64(completed+failed) / float64(total) * 100
	}
}

// MarkStarted marks the job as started
func (j *QueueJobState) MarkStarted() {
	j.Status = JobStatusRunning
	now := time.Now()
	j.StartedAt = &now
}

// MarkCompleted marks the job as completed
func (j *QueueJobState) MarkCompleted() {
	j.Status = JobStatusCompleted
	now := time.Now()
	j.CompletedAt = &now
	// Note: ResultCount is managed via event-driven metadata updates (EventDocumentSaved)
	// Do not overwrite with progress.completed_urls as it causes double counting
	j.FailedCount = j.Progress.FailedURLs
}

// MarkFailed marks the job as failed with an error message
func (j *QueueJobState) MarkFailed(errorMsg string) {
	j.Status = JobStatusFailed
	j.Error = errorMsg
	now := time.Now()
	j.CompletedAt = &now
	// Note: ResultCount is managed via event-driven metadata updates (EventDocumentSaved)
	// Do not overwrite with progress.completed_urls as it causes double counting
	j.FailedCount = j.Progress.FailedURLs
}

// MarkCancelled marks the job as cancelled
func (j *QueueJobState) MarkCancelled() {
	j.Status = JobStatusCancelled
	now := time.Now()
	j.CompletedAt = &now
}

// UpdateHeartbeat updates the last heartbeat timestamp
func (j *QueueJobState) UpdateHeartbeat() {
	now := time.Now()
	j.LastHeartbeat = &now
}

// IsTerminal returns true if the job is in a terminal state
func (j *QueueJobState) IsTerminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusFailed ||
		j.Status == JobStatusCancelled
}
