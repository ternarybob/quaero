package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JobMessage represents a queue message for job processing
type JobMessage struct {
	// ID is the unique message ID
	ID string `json:"id"`

	// Type is the job type: "parent", "crawler_url", "summarizer", "cleanup"
	Type string `json:"type"`

	// ParentID is the parent job ID for child jobs (empty for parent jobs)
	ParentID string `json:"parent_id"`

	// JobDefinitionID references a job definition if applicable
	JobDefinitionID string `json:"job_definition_id,omitempty"`

	// Depth is the crawl depth for URL jobs
	Depth int `json:"depth"`

	// URL is the URL for crawler_url type
	URL string `json:"url,omitempty"`

	// SourceType identifies the source type (e.g., "jira", "confluence")
	SourceType string `json:"source_type,omitempty"`

	// EntityType identifies the entity type (e.g., "projects", "issues")
	EntityType string `json:"entity_type,omitempty"`

	// Config holds job-specific configuration
	Config map[string]interface{} `json:"config,omitempty"`

	// Metadata holds additional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Status is the job status: "pending", "running", "completed", "failed"
	Status string `json:"status"`

	// CreatedAt is when the message was created
	CreatedAt time.Time `json:"created_at"`

	// StartedAt is when the job started processing
	StartedAt time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the job completed
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// NewJobMessage creates a new job message with defaults
func NewJobMessage(jobType string, parentID string) *JobMessage {
	return &JobMessage{
		ID:        uuid.New().String(),
		Type:      jobType,
		ParentID:  parentID,
		Status:    "pending",
		CreatedAt: time.Now(),
		Config:    make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
	}
}

// NewParentJobMessage creates a parent job message for coordinating child job execution.
// Deprecated: This function is no longer used. Job definitions now execute directly via JobExecutor
// without creating parent messages. This function is kept for backward compatibility only.
func NewParentJobMessage(sourceType, entityType string, config map[string]interface{}) *JobMessage {
	msg := NewJobMessage("parent", "")
	msg.SourceType = sourceType
	msg.EntityType = entityType
	msg.Config = config
	return msg
}

// NewCrawlerURLMessage creates a crawler URL job message
func NewCrawlerURLMessage(parentID string, url string, depth int, sourceType, entityType string) *JobMessage {
	msg := NewJobMessage("crawler_url", parentID)
	msg.URL = url
	msg.Depth = depth
	msg.SourceType = sourceType
	msg.EntityType = entityType
	return msg
}

// NewSummarizerMessage creates a summarizer job message
func NewSummarizerMessage(parentID string, config map[string]interface{}) *JobMessage {
	msg := NewJobMessage("summarizer", parentID)
	msg.Config = config
	return msg
}

// NewCleanupMessage creates a cleanup job message
func NewCleanupMessage(config map[string]interface{}) *JobMessage {
	msg := NewJobMessage("cleanup", "")
	msg.Config = config
	return msg
}

// NewPreValidationMessage creates a pre-validation job message
func NewPreValidationMessage(parentID string, config map[string]interface{}) *JobMessage {
	msg := NewJobMessage("pre_validation", parentID)
	msg.Config = config
	return msg
}

// NewPostSummarizationMessage creates a post-summarization job message
func NewPostSummarizationMessage(parentID string, config map[string]interface{}) *JobMessage {
	msg := NewJobMessage("post_summarization", parentID)
	msg.Config = config
	return msg
}

// NewJobDefinitionMessage creates a parent message for job definition execution.
// Deprecated: This function is no longer used. ExecuteJobDefinitionHandler now invokes JobExecutor directly
// without creating parent messages. This function is kept for backward compatibility only.
func NewJobDefinitionMessage(jobDefID string, config map[string]interface{}) *JobMessage {
	msg := NewJobMessage("parent", "")
	msg.JobDefinitionID = jobDefID
	msg.Config = config
	// Store job_type in metadata for clarity (not in SourceType which is for data sources)
	if config != nil {
		if jobType, ok := config["job_type"]; ok {
			if msg.Metadata == nil {
				msg.Metadata = make(map[string]interface{})
			}
			msg.Metadata["job_type"] = jobType
		}
	}
	return msg
}

// ToJSON serializes the message to JSON
func (m *JobMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON deserializes the message from JSON
func FromJSON(data []byte) (*JobMessage, error) {
	var msg JobMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
