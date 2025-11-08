package events

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// NewLoggerSubscriber creates an event handler that logs all events
func NewLoggerSubscriber(logger arbor.ILogger) interfaces.EventHandler {
	return func(ctx context.Context, event interfaces.Event) error {
		// Extract common fields from payload if available
		var jobID, sourceType, status string
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			if id, ok := payload["job_id"].(string); ok {
				jobID = id
			}
			if st, ok := payload["source_type"].(string); ok {
				sourceType = st
			}
			if s, ok := payload["status"].(string); ok {
				status = s
			}
		}

		// Log event with structured fields
		logEvent := logger.Debug().
			Str("event_type", string(event.Type))

		if jobID != "" {
			logEvent = logEvent.Str("job_id", jobID)
		}
		if sourceType != "" {
			logEvent = logEvent.Str("source_type", sourceType)
		}
		if status != "" {
			logEvent = logEvent.Str("status", status)
		}

		logEvent.Msg("Event published")

		return nil
	}
}

// SubscribeLoggerToAllEvents subscribes the logger to all known event types
func SubscribeLoggerToAllEvents(eventService interfaces.EventService, logger arbor.ILogger) error {
	subscriber := NewLoggerSubscriber(logger)

	// Subscribe to all event types
	eventTypes := []interfaces.EventType{
		interfaces.EventCollectionTriggered,
		interfaces.EventEmbeddingTriggered,
		interfaces.EventDocumentForceSync,
		interfaces.EventCrawlProgress,
		interfaces.EventStatusChanged,
		interfaces.EventSourceCreated,
		interfaces.EventSourceUpdated,
		interfaces.EventSourceDeleted,
		interfaces.EventJobProgress,
		interfaces.EventJobSpawn,
		interfaces.EventJobCreated,
		interfaces.EventJobStarted,
		interfaces.EventJobCompleted,
		interfaces.EventJobFailed,
		interfaces.EventJobCancelled,
	}

	for _, eventType := range eventTypes {
		if err := eventService.Subscribe(eventType, subscriber); err != nil {
			return fmt.Errorf("failed to subscribe logger to event type %s: %w", eventType, err)
		}
	}

	logger.Info().
		Int("event_type_count", len(eventTypes)).
		Msg("Logger subscribed to all event types")

	return nil
}
