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
