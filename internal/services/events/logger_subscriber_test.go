package events

import (
	"context"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// TestNewLoggerSubscriber verifies that the logger subscriber logs events
func TestNewLoggerSubscriber(t *testing.T) {
	// Create a test logger
	logger := arbor.NewLogger()
	defer common.Stop()

	// Create logger subscriber
	subscriber := NewLoggerSubscriber(logger)

	// Test with event containing payload
	ctx := context.Background()
	event := interfaces.Event{
		Type: interfaces.EventJobCreated,
		Payload: map[string]interface{}{
			"job_id":      "test-job-123",
			"source_type": "jira",
			"status":      "pending",
		},
	}

	// Call the subscriber
	err := subscriber(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test with event without payload
	event2 := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: nil,
	}

	err = subscriber(ctx, event2)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestSubscribeLoggerToAllEvents verifies logger is subscribed to all event types
func TestSubscribeLoggerToAllEvents(t *testing.T) {
	// Create a test logger
	logger := arbor.NewLogger()
	defer common.Stop()

	// Create event service
	eventService := NewService(logger)
	defer eventService.Close()

	// Note: SubscribeLoggerToAllEvents is called automatically in NewService
	// We're testing that it was successful by publishing an event

	ctx := context.Background()
	event := interfaces.Event{
		Type: interfaces.EventJobCreated,
		Payload: map[string]interface{}{
			"job_id": "test-job",
		},
	}

	// Publish event - should not error
	err := eventService.Publish(ctx, event)
	if err != nil {
		t.Errorf("Expected no error publishing event, got: %v", err)
	}

	// Test all event types
	eventTypes := []interfaces.EventType{
		interfaces.EventCollectionTriggered,
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
		event := interfaces.Event{
			Type:    eventType,
			Payload: map[string]interface{}{"test": "data"},
		}

		err := eventService.Publish(ctx, event)
		if err != nil {
			t.Errorf("Expected no error publishing %s event, got: %v", eventType, err)
		}
	}
}

// TestLoggerSubscriberDoesNotInterfere verifies logger subscriber doesn't interfere with other handlers
func TestLoggerSubscriberDoesNotInterfere(t *testing.T) {
	// Create a test logger
	logger := arbor.NewLogger()
	defer common.Stop()

	// Create event service (logger subscriber auto-registered)
	eventService := NewService(logger)
	defer eventService.Close()

	// Add a custom handler that tracks calls
	callCount := 0
	customHandler := func(ctx context.Context, event interfaces.Event) error {
		callCount++
		return nil
	}

	// Subscribe custom handler
	err := eventService.Subscribe(interfaces.EventJobCreated, customHandler)
	if err != nil {
		t.Fatalf("Failed to subscribe custom handler: %v", err)
	}

	// Publish event
	ctx := context.Background()
	event := interfaces.Event{
		Type: interfaces.EventJobCreated,
		Payload: map[string]interface{}{
			"job_id": "test-job",
		},
	}

	err = eventService.PublishSync(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify custom handler was called
	if callCount != 1 {
		t.Errorf("Expected custom handler to be called once, got: %d", callCount)
	}
}
