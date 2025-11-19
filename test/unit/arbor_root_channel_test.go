package unit

import (
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	arbormodels "github.com/ternarybob/arbor/models"
)

// TestArborRootChannel verifies if root logger sends logs to "context" channel
func TestArborRootChannel(t *testing.T) {
	// Create a channel to receive log batches
	logChannel := make(chan []arbormodels.LogEvent, 10)

	// Create root logger
	rootLogger := arbor.NewLogger()

	// Set context channel on root logger
	rootLogger.SetChannel("context", logChannel)

	// Send a log message directly from root logger (no correlation ID)
	rootLogger.Info().Msg("Test message from root logger")

	// Wait for log to be sent to channel
	select {
	case batch := <-logChannel:
		if len(batch) == 0 {
			t.Fatal("Received empty batch")
		}

		// Verify the log event
		event := batch[0]
		if event.Message != "Test message from root logger" {
			t.Errorf("Expected message 'Test message from root logger', got '%s'", event.Message)
		}
		t.Logf("✓ Successfully received root log via context channel")

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for log event on channel - root logger logs may not be going to 'context' channel")
	}
}
