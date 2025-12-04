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

// convertTo3Letter converts full level names to 3-letter codes
func convertTo3Letter(level string) string {
	switch strings.ToUpper(level) {
	case "INFO":
		return "INF"
	case "WARN", "WARNING":
		return "WRN"
	case "ERROR":
		return "ERR"
	case "DEBUG":
		return "DBG"
	default:
		// If already 3 letters, return as-is (uppercase)
		if len(level) == 3 {
			return strings.ToUpper(level)
		}
		return "INF"
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
			// Use logger without correlation ID to avoid recursive channel processing
			c.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Msg("LogConsumer panic recovered")
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
				// Skip HTTP request logs - these are not job-specific logs
				// HTTP middleware generates correlation IDs for all requests, but these
				// should not be stored in job_logs table (they're for request tracing only)
				if event.Message == "HTTP request" ||
					event.Message == "HTTP request - client error" ||
					event.Message == "HTTP request - server error" ||
					strings.Contains(event.Message, "WebSocket client") {
					continue
				}

				// Transform arbor log event to JobLogEntry
				logEntry := transformEvent(event)

				// Group by jobID for batch database writes ONLY if jobID is present
				jobID := event.CorrelationID
				if jobID != "" {
					entriesByJob[jobID] = append(entriesByJob[jobID], logEntry)
				}

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
						// Use logger without correlation ID to avoid recursive channel processing
						// Logs without correlation ID won't be stored in job logs or re-published
						c.logger.Warn().
							Err(err).
							Str("job_id", jid).
							Int("log_count", len(logs)).
							Msg("Failed to write batch logs to database")
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
// Includes structured context fields (phase, originator, step_name) for UI rendering
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
		// Build payload with structured context fields for UI rendering
		payload := map[string]interface{}{
			"job_id":    event.CorrelationID,
			"level":     logEntry.Level,
			"message":   logEntry.Message,
			"timestamp": logEntry.Timestamp,
		}

		// Include structured context fields if present (UI uses these for rendering)
		if logEntry.Phase != "" {
			payload["phase"] = logEntry.Phase
		}
		if logEntry.Originator != "" {
			payload["originator"] = logEntry.Originator
		}
		if logEntry.StepName != "" {
			payload["step_name"] = logEntry.StepName
		}
		if logEntry.SourceType != "" {
			payload["source_type"] = logEntry.SourceType
		}

		// Non-blocking publish in goroutine
		err := c.eventService.Publish(c.ctx, interfaces.Event{
			Type:    "log_event", // Event type for log streaming
			Payload: payload,
		})
		if err != nil {
			// Use logger without correlation ID to avoid recursive channel processing
			c.logger.Warn().
				Err(err).
				Str("job_id", event.CorrelationID).
				Msg("Failed to publish log event")
		}
	}()
}

// transformEvent converts arbor LogEvent to JobLogEntry format
// Extracts structured fields (phase, originator, step_name) for UI rendering
func transformEvent(event arbormodels.LogEvent) models.JobLogEntry {
	// Format timestamp as "15:04:05" for display
	formattedTime := event.Timestamp.Format("15:04:05")

	// Also store RFC3339 format for accurate sorting
	fullTimestamp := event.Timestamp.Format(time.RFC3339)

	// Convert level to 3-letter format for consistent display
	levelStr := convertTo3Letter(event.Level.String())

	// Extract structured context fields for UI rendering
	// These fields are set by workers via log.Str("phase", "step").Msg(...)
	var phase, originator, stepName, sourceType string

	// Build message, excluding context fields that are stored separately
	message := event.Message
	if len(event.Fields) > 0 {
		var extraFields []string
		for key, value := range event.Fields {
			switch key {
			case "phase":
				phase = fmt.Sprintf("%v", value)
			case "originator":
				originator = fmt.Sprintf("%v", value)
			case "step_name":
				stepName = fmt.Sprintf("%v", value)
			case "source_type":
				sourceType = fmt.Sprintf("%v", value)
			default:
				// Append non-context fields to message for persistence
				extraFields = append(extraFields, fmt.Sprintf("%s=%v", key, value))
			}
		}
		// Append extra fields to message if any
		for _, field := range extraFields {
			message += " " + field
		}
	}

	return models.JobLogEntry{
		Timestamp:       formattedTime,
		FullTimestamp:   fullTimestamp,
		Level:           levelStr,
		Message:         message,
		AssociatedJobID: event.CorrelationID,
		Phase:           phase,
		Originator:      originator,
		StepName:        stepName,
		SourceType:      sourceType,
	}
}
