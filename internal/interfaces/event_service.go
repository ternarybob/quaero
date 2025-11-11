package interfaces

import "context"

// EventType represents different event types in the system
type EventType string

const (
	EventCollectionTriggered EventType = "collection_triggered"
	EventEmbeddingTriggered  EventType = "embedding_triggered"
	EventDocumentForceSync   EventType = "document_force_sync"
	// EventCrawlProgress is published periodically during crawl jobs with detailed progress updates.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string
	//   - source_type: string ("jira", "confluence", "github")
	//   - entity_type: string ("project", "issue", "space", "page")
	//   - status: string ("pending", "running", "completed", "failed", "cancelled")
	//   - total_urls: int
	//   - completed_urls: int
	//   - failed_urls: int
	//   - pending_urls: int
	//   - current_url: string
	//   - percentage: float64
	//   - estimated_completion: time.Time
	EventCrawlProgress EventType = "crawl_progress"

	// EventStatusChanged is published when application state changes (Idle, Crawling, Offline)
	// Payload structure: map[string]interface{} with keys:
	//   - state: string ("idle", "crawling", "offline")
	//   - metadata: map[string]interface{} (additional context)
	//   - timestamp: time.Time
	EventStatusChanged EventType = "status_changed"

	// EventSourceCreated is published when a new source is created
	// Payload structure: map[string]interface{} with keys:
	//   - source_id: string
	//   - source_type: string ("jira", "confluence", "github")
	//   - source_name: string
	//   - timestamp: time.Time
	EventSourceCreated EventType = "source_created"

	// EventSourceUpdated is published when a source is updated
	// Payload structure: map[string]interface{} with keys:
	//   - source_id: string
	//   - source_type: string ("jira", "confluence", "github")
	//   - source_name: string
	//   - timestamp: time.Time
	EventSourceUpdated EventType = "source_updated"

	// EventSourceDeleted is published when a source is deleted
	// Payload structure: map[string]interface{} with keys:
	//   - source_id: string
	//   - source_type: string ("jira", "confluence", "github")
	//   - source_name: string
	//   - timestamp: time.Time
	EventSourceDeleted EventType = "source_deleted"

	// EventJobProgress is published during job definition execution with progress updates.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string (job definition ID)
	//   - job_name: string (job definition name)
	//   - job_type: string ("crawler", "summarizer", "custom")
	//   - step_index: int (current step index, 0-based)
	//   - step_name: string (current step name)
	//   - step_action: string (current step action)
	//   - total_steps: int (total number of steps)
	//   - status: string ("running", "completed", "failed")
	//   - error: string (error message if status is "failed")
	//   - timestamp: time.Time
	//
	// For crawl actions, additional fields may be included:
	//   - crawl_job_id: string (crawler service job ID)
	//   - source_id: string (source being crawled)
	//   - source_type: string (jira, confluence, github)
	//   - total_urls: int (total URLs discovered)
	//   - completed_urls: int (URLs successfully crawled)
	//   - failed_urls: int (URLs that failed)
	//   - pending_urls: int (URLs in queue)
	//   - percentage: float64 (completion percentage)
	//   - current_url: string (currently processing URL)
	//
	// These fields are populated during async polling of crawl jobs
	EventJobProgress EventType = "job_progress"

	// EventJobSpawn is published when a job spawns child jobs
	// Payload structure: map[string]interface{} with keys:
	//   - parent_job_id: string (parent job ID)
	//   - child_job_id: string (newly spawned child job ID)
	//   - job_type: string (type of job, e.g., "crawler_url")
	//   - url: string (URL being crawled, if applicable)
	//   - depth: int (crawl depth)
	//   - timestamp: time.Time
	EventJobSpawn EventType = "job_spawn"

	// EventJobCreated is published when a new crawl job is created and persisted.
	// Published after successful database persistence in StartCrawl.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string
	//   - status: string ("pending")
	//   - source_type: string ("jira", "confluence", "github")
	//   - entity_type: string ("project", "issue", "space", "page")
	//   - seed_url_count: int
	//   - max_depth: int
	//   - max_pages: int
	//   - follow_links: bool
	//   - timestamp: time.Time
	EventJobCreated EventType = "job_created"

	// EventJobStarted is published when a job begins processing its first URL.
	// Published when job transitions from pending to running (first URL processed).
	// Published at the start of CrawlerJob.Execute for the first URL.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string
	//   - status: string ("running")
	//   - source_type: string ("jira", "confluence", "github")
	//   - entity_type: string ("project", "issue", "space", "page")
	//   - url: string (first URL being processed)
	//   - depth: int
	//   - timestamp: time.Time
	EventJobStarted EventType = "job_started"

	// EventJobCompleted is published when a job successfully completes all URLs.
	// Published after grace period verification in ExecuteCompletionProbe.
	// Published after marking job complete and persisting to database.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string
	//   - status: string ("completed")
	//   - source_type: string ("jira", "confluence", "github")
	//   - entity_type: string ("project", "issue", "space", "page")
	//   - result_count: int
	//   - failed_count: int
	//   - total_urls: int
	//   - duration_seconds: float64
	//   - timestamp: time.Time
	EventJobCompleted EventType = "job_completed"

	// EventJobFailed is published when a job fails due to system errors or timeout.
	// Published when job is marked as failed (stale job detection, system errors).
	// Published after marking job failed in Service.FailJob.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string
	//   - status: string ("failed")
	//   - source_type: string ("jira", "confluence", "github")
	//   - entity_type: string ("project", "issue", "space", "page")
	//   - result_count: int
	//   - failed_count: int
	//   - error: string
	//   - timestamp: time.Time
	EventJobFailed EventType = "job_failed"

	// EventJobCancelled is published when a user cancels a running job.
	// Published when user cancels a running job via API.
	// Published after marking job cancelled in Service.CancelJob.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string
	//   - status: string ("cancelled")
	//   - source_type: string ("jira", "confluence", "github")
	//   - entity_type: string ("project", "issue", "space", "page")
	//   - result_count: int
	//   - failed_count: int
	//   - timestamp: time.Time
	EventJobCancelled EventType = "job_cancelled"

	// EventJobStatusChange is published when any job changes status (pending → running → completed/failed/cancelled).
	// Published from Manager.UpdateJobStatus after successful database update.
	// Used by ParentJobOrchestrator to track child job progress in real-time.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string (ID of the job that changed status)
	//   - status: string (new status: "pending", "running", "completed", "failed", "cancelled")
	//   - job_type: string (type of job)
	//   - parent_id: string (optional - only present if this is a child job)
	//   - timestamp: string (RFC3339 formatted timestamp)
	EventJobStatusChange EventType = "job_status_change"

	// EventDocumentSaved is published when a child job successfully saves a document.
	// Published from DocumentPersister.SaveCrawledDocument after successful document persistence.
	// Used by ParentJobOrchestrator to track document count for parent jobs in real-time.
	// Payload structure: map[string]interface{} with keys:
	//   - job_id: string (child job ID that saved the document)
	//   - parent_job_id: string (parent job ID to update)
	//   - document_id: string (saved document ID)
	//   - source_url: string (document URL)
	//   - timestamp: string (RFC3339 formatted timestamp)
	EventDocumentSaved EventType = "document_saved"
)

// Event represents a system event
type Event struct {
	Type    EventType
	Payload interface{}
}

// EventHandler is a function that handles events
type EventHandler func(ctx context.Context, event Event) error

// EventService manages pub/sub event bus
type EventService interface {
	// Subscribe to an event type
	Subscribe(eventType EventType, handler EventHandler) error

	// Unsubscribe from an event type
	Unsubscribe(eventType EventType, handler EventHandler) error

	// Publish an event to all subscribers
	Publish(ctx context.Context, event Event) error

	// PublishSync publishes event and waits for all handlers to complete
	PublishSync(ctx context.Context, event Event) error

	// Close shuts down the event service
	Close() error
}
