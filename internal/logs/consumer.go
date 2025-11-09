package logs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/phuslu/log"
	"github.com/ternarybob/arbor"
	arborlevels "github.com/ternarybob/arbor/levels"
	arbormodels "github.com/ternarybob/arbor/models"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Consumer consumes log batches from arbor's context channel and dispatches to storage and events
type Consumer struct {
	storage       interfaces.JobLogStorage
	eventService  interfaces.EventService
	logger        arbor.ILogger
	channel       chan []arbormodels.LogEvent
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	minEventLevel arbor.LogLevel // Minimum log level to publish as events
	publishing    sync.Map       // Track events being published to prevent recursion
}

// NewConsumer creates a new log consumer
func NewConsumer(storage interfaces.JobLogStorage, eventService interfaces.EventService, logger arbor.ILogger, minEventLevel string) *Consumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Consumer{
		storage:       storage,
		eventService:  eventService,
		logger:        logger,
		channel:       make(chan []arbormodels.LogEvent, 10),
		ctx:           ctx,
		cancel:        cancel,
		minEventLevel: parseLogLevel(minEventLevel),
	}
}

// parseLogLevel converts string log level to arbor.LogLevel
func parseLogLevel(levelStr string) arbor.LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return arbor.DebugLevel
	case "info":
		return arbor.InfoLevel
	case "warn", "warning":
		return arbor.WarnLevel
	case "error":
		return arbor.ErrorLevel
	default:
		return arbor.InfoLevel // Default to Info
	}
}

// GetChannel returns the channel for arbor to send log batches to
func (c *Consumer) GetChannel() chan []arbormodels.LogEvent {
	return c.channel
}

// Start launches the consumer goroutine
func (c *Consumer) Start() error {
	c.wg.Add(1)
	go c.consumer()
	return nil
}

// Stop gracefully shuts down the consumer
func (c *Consumer) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.logger.Info().Msg("Log consumer stopped")
	return nil
}

// consumer processes log batches from arbor and dispatches to destinations
func (c *Consumer) consumer() {
	defer c.wg.Done()

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ERROR: LogConsumer panic recovered: %v\n", r)
		}
	}()

	// Process batches with graceful shutdown support
	for {
		select {
		case batch, ok := <-c.channel:
			if !ok {
				// Channel closed, exit gracefully
				return
			}

			// Group entries by jobID for batch writes
			entriesByJob := make(map[string][]models.JobLogEntry)

			// Process each event in the batch
			for _, event := range batch {
				// Skip events without CorrelationID (no jobID)
				jobID := event.CorrelationID
				if jobID == "" {
					continue
				}

				// Transform arbor log event to JobLogEntry
				logEntry := transformEvent(event)

				// Group by jobID for batch database writes
				entriesByJob[jobID] = append(entriesByJob[jobID], logEntry)

				// Publish as event if level >= threshold (for UI real-time updates)
				if c.eventService != nil && c.shouldPublishEvent(event.Level) {
					c.publishLogEvent(event, logEntry)
				}
			}

			// Batch write to database by jobID with concurrent goroutines
			var wg sync.WaitGroup
			for jobID, entries := range entriesByJob {
				wg.Add(1)
				go func(jid string, logs []models.JobLogEntry) {
					defer wg.Done()

					// Dispatch to database with proper context (cancellable during shutdown)
					if err := c.storage.AppendLogs(c.ctx, jid, logs); err != nil {
						// Use fmt.Printf to avoid deadlock with logger
						fmt.Printf("WARN: Failed to write batch logs to database for job %s: %v\n", jid, err)
					}
				}(jobID, entries)
			}

			// Wait for all database writes to complete for this batch
			wg.Wait()

		case <-c.ctx.Done():
			// Context cancelled, exit gracefully
			return
		}
	}
}

// shouldPublishEvent checks if a log event should be published based on level threshold
func (c *Consumer) shouldPublishEvent(level log.Level) bool {
	eventLevel := arborlevels.FromLogLevel(level)
	return eventLevel >= c.minEventLevel
}

// publishLogEvent publishes a log entry as an event for UI consumption
func (c *Consumer) publishLogEvent(event arbormodels.LogEvent, logEntry models.JobLogEntry) {
	// Circuit breaker: Check if we're already publishing an event for this correlation ID + message
	// This prevents recursive event publishing (defense in depth)
	key := fmt.Sprintf("%s:%s", event.CorrelationID, logEntry.Message)
	if _, loaded := c.publishing.LoadOrStore(key, true); loaded {
		// Already publishing this event - skip to prevent recursion
		return
	}
	defer c.publishing.Delete(key)

	// Publish to EventService (WebSocket will subscribe to this event type)
	go func() {
		// Non-blocking publish in goroutine
		err := c.eventService.Publish(c.ctx, interfaces.Event{
			Type: "log_event", // Event type for log streaming
			Payload: map[string]interface{}{
				"job_id":    event.CorrelationID,
				"level":     logEntry.Level,
				"message":   logEntry.Message,
				"timestamp": logEntry.Timestamp,
			},
		})
		if err != nil {
			// Use fmt.Printf to avoid deadlock with logger
			fmt.Printf("WARN: Failed to publish log event for job %s: %v\n", event.CorrelationID, err)
		}
	}()
}

// transformEvent converts arbor LogEvent to JobLogEntry format
func transformEvent(event arbormodels.LogEvent) models.JobLogEntry {
	// Format timestamp as "15:04:05" for display
	formattedTime := event.Timestamp.Format("15:04:05")

	// Also store RFC3339 format for accurate sorting
	fullTimestamp := event.Timestamp.Format(time.RFC3339)

	// Convert level to lowercase for consistent storage and filtering
	levelStr := strings.ToLower(event.Level.String())

	// Build message with fields if present
	message := event.Message
	if len(event.Fields) > 0 {
		// Append fields to message for database persistence
		for key, value := range event.Fields {
			message += fmt.Sprintf(" %s=%v", key, value)
		}
	}

	return models.JobLogEntry{
		Timestamp:       formattedTime,
		FullTimestamp:   fullTimestamp,
		Level:           levelStr,
		Message:         message,
		AssociatedJobID: event.CorrelationID,
	}
}
