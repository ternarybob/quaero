package unit

import (
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	arbormodels "github.com/ternarybob/arbor/models"
)

// TestArborContextChannelInheritance verifies that WithCorrelationId() inherits context channel
func TestArborContextChannelInheritance(t *testing.T) {
	// Create a channel to receive log batches
	logChannel := make(chan []arbormodels.LogEvent, 10)

	// Create root logger
	rootLogger := arbor.NewLogger()

	// Set context channel on root logger (using new API)
	rootLogger.SetChannel("context", logChannel)

	// Create derived logger with correlation ID
	derivedLogger := rootLogger.WithCorrelationId("test-job-123")

	// Send a log message
	derivedLogger.Info().Msg("Test message from derived logger")

	// Wait for log to be sent to channel
	select {
	case batch := <-logChannel:
		if len(batch) == 0 {
			t.Fatal("Received empty batch")
		}

		// Verify the log event
		event := batch[0]
		if event.CorrelationID != "test-job-123" {
			t.Errorf("Expected correlation ID 'test-job-123', got '%s'", event.CorrelationID)
		}

		if event.Message != "Test message from derived logger" {
			t.Errorf("Expected message 'Test message from derived logger', got '%s'", event.Message)
		}

		t.Logf("✓ Successfully received log via context channel")
		t.Logf("  Correlation ID: %s", event.CorrelationID)
		t.Logf("  Message: %s", event.Message)
		t.Logf("  Level: %s", event.Level)

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for log event on channel - context channel inheritance may be broken")
	}
}

// TestArborContextChannelMultipleLogs verifies batching behavior
func TestArborContextChannelMultipleLogs(t *testing.T) {
	// Create a channel to receive log batches
	logChannel := make(chan []arbormodels.LogEvent, 10)

	// Create root logger
	rootLogger := arbor.NewLogger()

	// Set context channel on root logger (using new API)
	rootLogger.SetChannel("context", logChannel)

	// Create derived logger with correlation ID
	derivedLogger := rootLogger.WithCorrelationId("batch-test-456")

	// Send multiple log messages
	derivedLogger.Info().Msg("Message 1")
	derivedLogger.Warn().Msg("Message 2")
	derivedLogger.Error().Msg("Message 3")

	// Wait for logs to be sent to channel
	receivedCount := 0
	timeout := time.After(2 * time.Second)

	for receivedCount < 3 {
		select {
		case batch := <-logChannel:
			receivedCount += len(batch)
			t.Logf("Received batch with %d events (total: %d)", len(batch), receivedCount)

			for _, event := range batch {
				if event.CorrelationID != "batch-test-456" {
					t.Errorf("Expected correlation ID 'batch-test-456', got '%s'", event.CorrelationID)
				}
			}

		case <-timeout:
			t.Fatalf("Timeout waiting for all log events - received %d out of 3", receivedCount)
		}
	}

	t.Logf("✓ Successfully received all %d logs via context channel", receivedCount)
}
