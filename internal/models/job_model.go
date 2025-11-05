// -----------------------------------------------------------------------
// Job Model - Common immutable job structure for queue persistence
// -----------------------------------------------------------------------

package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JobModel represents the immutable job definition stored in the queue and database.
// Once created and enqueued, this model should not be modified.
// All job types (parent, child, crawler, summarizer, etc.) use this common structure.
type JobModel struct {
	// Core identification
	ID       string  `json:"id"`        // Unique job ID (UUID)
	ParentID *string `json:"parent_id"` // Parent job ID for child jobs (nil for root jobs)

	// Job classification
	Type string `json:"type"` // Job type: "database_maintenance", "crawler", "summarizer", etc.
	Name string `json:"name"` // Human-readable job name

	// Configuration (immutable snapshot at creation time)
	Config map[string]interface{} `json:"config"` // Job-specific configuration

	// Metadata
	Metadata map[string]interface{} `json:"metadata"` // Additional metadata (job_definition_id, etc.)

	// Timestamps
	CreatedAt time.Time `json:"created_at"` // Job creation timestamp

	// Hierarchy tracking
	Depth int `json:"depth"` // Depth in job tree (0 for root, 1 for direct children, etc.)
}

// NewJobModel creates a new root job model
func NewJobModel(jobType, name string, config, metadata map[string]interface{}) *JobModel {
	return &JobModel{
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

// NewChildJobModel creates a new child job model
func NewChildJobModel(parentID, jobType, name string, config, metadata map[string]interface{}, depth int) *JobModel {
	return &JobModel{
		ID:        uuid.New().String(),
		ParentID:  &parentID,
		Type:      jobType,
		Name:      name,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		Depth:     depth,
	}
}

// IsRootJob returns true if this is a root job (no parent)
func (j *JobModel) IsRootJob() bool {
	return j.ParentID == nil
}

// GetParentID returns the parent ID or empty string if root job
func (j *JobModel) GetParentID() string {
	if j.ParentID == nil {
		return ""
	}
	return *j.ParentID
}

// ToJSON serializes the job model to JSON for queue storage
func (j *JobModel) ToJSON() ([]byte, error) {
	data, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job model: %w", err)
	}
	return data, nil
}

// FromJSON deserializes a job model from JSON
func FromJSON(data []byte) (*JobModel, error) {
	var model JobModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job model: %w", err)
	}
	return &model, nil
}

// Validate validates the job model
func (j *JobModel) Validate() error {
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

// Clone creates a deep copy of the job model (useful for creating child jobs)
func (j *JobModel) Clone() *JobModel {
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

	clone := &JobModel{
		ID:        j.ID,
		ParentID:  j.ParentID,
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
func (j *JobModel) GetConfigString(key string) (string, bool) {
	val, ok := j.Config[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetConfigInt retrieves an int value from config
func (j *JobModel) GetConfigInt(key string) (int, bool) {
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
func (j *JobModel) GetConfigBool(key string) (bool, bool) {
	val, ok := j.Config[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetConfigStringSlice retrieves a string slice from config
func (j *JobModel) GetConfigStringSlice(key string) ([]string, bool) {
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
func (j *JobModel) GetMetadataString(key string) (string, bool) {
	val, ok := j.Metadata[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// SetMetadata sets a metadata value (use sparingly - model should be immutable after creation)
func (j *JobModel) SetMetadata(key string, value interface{}) {
	if j.Metadata == nil {
		j.Metadata = make(map[string]interface{})
	}
	j.Metadata[key] = value
}

// -----------------------------------------------------------------------
// Job - Runtime job state (combines JobModel with execution state)
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

// Job represents a job with runtime execution state
// This combines the immutable JobModel with mutable runtime state
type Job struct {
	// Immutable job definition
	*JobModel

	// Mutable runtime state
	Status        JobStatus    `json:"status"`
	Progress      *JobProgress `json:"progress,omitempty"`
	StartedAt     *time.Time   `json:"started_at,omitempty"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
	FinishedAt    *time.Time   `json:"finished_at,omitempty"`
	LastHeartbeat *time.Time   `json:"last_heartbeat,omitempty"`
	Error         string       `json:"error,omitempty"`
	ResultCount   int          `json:"result_count"`
	FailedCount   int          `json:"failed_count"`
}

// NewJob creates a new job from a JobModel
func NewJob(model *JobModel) *Job {
	return &Job{
		JobModel:    model,
		Status:      JobStatusPending,
		Progress:    &JobProgress{},
		ResultCount: 0,
		FailedCount: 0,
	}
}

// ToJobModel extracts the immutable JobModel from a Job
func (j *Job) ToJobModel() *JobModel {
	return j.JobModel
}

// UpdateProgress updates the job progress and percentage
func (j *Job) UpdateProgress(completed, failed, pending, total int) {
	if j.Progress == nil {
		j.Progress = &JobProgress{}
	}
	j.Progress.CompletedURLs = completed
	j.Progress.FailedURLs = failed
	j.Progress.PendingURLs = pending
	j.Progress.TotalURLs = total

	if total > 0 {
		j.Progress.Percentage = float64(completed+failed) / float64(total) * 100
	}
}

// MarkStarted marks the job as started
func (j *Job) MarkStarted() {
	j.Status = JobStatusRunning
	now := time.Now()
	j.StartedAt = &now
}

// MarkCompleted marks the job as completed
func (j *Job) MarkCompleted() {
	j.Status = JobStatusCompleted
	now := time.Now()
	j.CompletedAt = &now
	if j.Progress != nil {
		j.ResultCount = j.Progress.CompletedURLs
		j.FailedCount = j.Progress.FailedURLs
	}
}

// MarkFailed marks the job as failed with an error message
func (j *Job) MarkFailed(errorMsg string) {
	j.Status = JobStatusFailed
	j.Error = errorMsg
	now := time.Now()
	j.CompletedAt = &now
	if j.Progress != nil {
		j.ResultCount = j.Progress.CompletedURLs
		j.FailedCount = j.Progress.FailedURLs
	}
}

// MarkCancelled marks the job as cancelled
func (j *Job) MarkCancelled() {
	j.Status = JobStatusCancelled
	now := time.Now()
	j.CompletedAt = &now
}

// UpdateHeartbeat updates the last heartbeat timestamp
func (j *Job) UpdateHeartbeat() {
	now := time.Now()
	j.LastHeartbeat = &now
}

// IsTerminal returns true if the job is in a terminal state
func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusFailed ||
		j.Status == JobStatusCancelled
}
