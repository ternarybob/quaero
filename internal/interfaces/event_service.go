package interfaces

import "context"

// EventType represents different event types in the system
type EventType string

const (
	EventCollectionTriggered EventType = "collection_triggered"
	EventEmbeddingTriggered  EventType = "embedding_triggered"
	EventDocumentForceSync   EventType = "document_force_sync"
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
