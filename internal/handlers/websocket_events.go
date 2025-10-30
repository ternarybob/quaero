package handlers

import (
	"context"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"golang.org/x/time/rate"
)

// EventSubscriber manages subscriptions to job lifecycle events and broadcasts them via WebSocket
type EventSubscriber struct {
	handler       *WebSocketHandler
	eventService  interfaces.EventService
	logger        arbor.ILogger
	allowedEvents map[string]bool          // Whitelist of events to broadcast (empty = allow all)
	throttlers    map[string]*rate.Limiter // Rate limiters for high-frequency events
	config        *common.WebSocketConfig
}

// NewEventSubscriber creates and initializes an event subscriber
// Automatically subscribes to all job lifecycle events with config-driven filtering and throttling
func NewEventSubscriber(handler *WebSocketHandler, eventService interfaces.EventService, logger arbor.ILogger, config *common.WebSocketConfig) *EventSubscriber {
	s := &EventSubscriber{
		handler:      handler,
		eventService: eventService,
		logger:       logger,
		config:       config,
	}

	// Initialize allowedEvents map (whitelist pattern)
	// Empty list means allow all events (backward compatible)
	s.allowedEvents = make(map[string]bool)
	if config != nil && len(config.AllowedEvents) > 0 {
		for _, eventType := range config.AllowedEvents {
			s.allowedEvents[eventType] = true
		}
	}

	// Initialize throttlers for high-frequency events
	s.throttlers = make(map[string]*rate.Limiter)
	if config != nil && len(config.ThrottleIntervals) > 0 {
		for eventType, intervalStr := range config.ThrottleIntervals {
			if duration, err := time.ParseDuration(intervalStr); err == nil {
				// Create rate limiter: 1 event per interval (burst=1)
				limiter := rate.NewLimiter(rate.Every(duration), 1)
				s.throttlers[eventType] = limiter
				logger.Debug().
					Str("event_type", eventType).
					Str("interval", intervalStr).
					Msg("Throttler initialized for event type")
			} else {
				logger.Warn().
					Err(err).
					Str("event_type", eventType).
					Str("interval", intervalStr).
					Msg("Failed to parse throttle interval - skipping throttler")
			}
		}
	}

	// Check for nil eventService
	if eventService == nil {
		logger.Warn().Msg("EventSubscriber created with nil eventService - subscriptions will be skipped")
		return s
	}

	// Subscribe to all job lifecycle events
	s.SubscribeAll()

	return s
}

// SubscribeAll registers subscriptions for all job lifecycle events
func (s *EventSubscriber) SubscribeAll() {
	// Early return if eventService is nil
	if s.eventService == nil {
		s.logger.Warn().Msg("Cannot subscribe to events - eventService is nil")
		return
	}

	// Subscribe to job creation events
	s.eventService.Subscribe(interfaces.EventJobCreated, s.handleJobCreated)

	// Subscribe to job start events
	s.eventService.Subscribe(interfaces.EventJobStarted, s.handleJobStarted)

	// Subscribe to job completion events
	s.eventService.Subscribe(interfaces.EventJobCompleted, s.handleJobCompleted)

	// Subscribe to job failure events
	s.eventService.Subscribe(interfaces.EventJobFailed, s.handleJobFailed)

	// Subscribe to job cancellation events
	s.eventService.Subscribe(interfaces.EventJobCancelled, s.handleJobCancelled)

	// Subscribe to job spawn events
	s.eventService.Subscribe(interfaces.EventJobSpawn, s.handleJobSpawn)

	s.logger.Info().Msg("EventSubscriber registered for all job lifecycle events (created, started, completed, failed, cancelled, spawn)")
}

// handleJobSpawn bridges EventJobSpawn to WebSocket job_spawn broadcast
func (s *EventSubscriber) handleJobSpawn(ctx context.Context, event interfaces.Event) error {
	// Check if event should be broadcast (filtering + throttling)
	if !s.shouldBroadcastEvent("job_spawn") {
		return nil
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		s.logger.Warn().Msg("Invalid job spawn event payload type")
		return nil
	}

	spawn := JobSpawnUpdate{
		ParentJobID: getStringWithFallback(payload, "parent_job_id", "parentJobId"),
		ChildJobID:  getStringWithFallback(payload, "child_job_id", "childJobId"),
		JobType:     getStringWithFallback(payload, "job_type", "jobType"),
		URL:         getString(payload, "url"),
		Depth:       getIntWithFallback(payload, "depth", "depth"),
		Timestamp:   getTimestamp(payload),
	}

	s.handler.BroadcastJobSpawn(spawn)
	return nil
}

// shouldBroadcastEvent checks if an event should be broadcast based on whitelist and throttling
func (s *EventSubscriber) shouldBroadcastEvent(eventType string) bool {
	// Check whitelist (empty allowedEvents = allow all)
	if len(s.allowedEvents) > 0 && !s.allowedEvents[eventType] {
		return false
	}

	// Check throttling
	if limiter, ok := s.throttlers[eventType]; ok {
		if !limiter.Allow() {
			s.logger.Debug().
				Str("event_type", eventType).
				Msg("Event throttled - rate limit exceeded")
			return false
		}
	}

	return true
}

// getStringWithFallback tries both snake_case and camelCase field names
func getStringWithFallback(payload map[string]interface{}, snakeCase, camelCase string) string {
	if val := getString(payload, snakeCase); val != "" {
		return val
	}
	return getString(payload, camelCase)
}

// getIntWithFallback tries both snake_case and camelCase field names
func getIntWithFallback(payload map[string]interface{}, snakeCase, camelCase string) int {
	if val := getInt(payload, snakeCase); val != 0 {
		return val
	}
	return getInt(payload, camelCase)
}

// getFloat64WithFallback tries both snake_case and camelCase field names
func getFloat64WithFallback(payload map[string]interface{}, snakeCase, camelCase string) float64 {
	if val := getFloat64(payload, snakeCase); val != 0.0 {
		return val
	}
	return getFloat64(payload, camelCase)
}

// getTimestamp attempts to parse a timestamp from the payload, falls back to time.Now()
func getTimestamp(payload map[string]interface{}) time.Time {
	if tsStr := getString(payload, "timestamp"); tsStr != "" {
		if ts, err := time.Parse(time.RFC3339, tsStr); err == nil {
			return ts
		}
	}
	return time.Now()
}

func (s *EventSubscriber) handleJobCreated(ctx context.Context, event interfaces.Event) error {
	// Check if event should be broadcast (filtering + throttling)
	if !s.shouldBroadcastEvent("job_created") {
		return nil
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		s.logger.Warn().Msg("Invalid job created event payload type")
		return nil
	}

	update := JobStatusUpdate{
		JobID:      getStringWithFallback(payload, "job_id", "jobId"),
		Status:     getString(payload, "status"),
		SourceType: getStringWithFallback(payload, "source_type", "sourceType"),
		EntityType: getStringWithFallback(payload, "entity_type", "entityType"),
		Timestamp:  getTimestamp(payload),
	}

	s.handler.BroadcastJobStatusChange(update)
	return nil
}

func (s *EventSubscriber) handleJobStarted(ctx context.Context, event interfaces.Event) error {
	// Check if event should be broadcast (filtering + throttling)
	if !s.shouldBroadcastEvent("job_started") {
		return nil
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		s.logger.Warn().Msg("Invalid job started event payload type")
		return nil
	}

	update := JobStatusUpdate{
		JobID:      getStringWithFallback(payload, "job_id", "jobId"),
		Status:     getString(payload, "status"),
		SourceType: getStringWithFallback(payload, "source_type", "sourceType"),
		EntityType: getStringWithFallback(payload, "entity_type", "entityType"),
		Timestamp:  getTimestamp(payload),
	}

	s.handler.BroadcastJobStatusChange(update)
	return nil
}

func (s *EventSubscriber) handleJobCompleted(ctx context.Context, event interfaces.Event) error {
	// Check if event should be broadcast (filtering + throttling)
	if !s.shouldBroadcastEvent("job_completed") {
		return nil
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		s.logger.Warn().Msg("Invalid job completed event payload type")
		return nil
	}

	totalURLs := getIntWithFallback(payload, "total_urls", "totalUrls")

	update := JobStatusUpdate{
		JobID:         getStringWithFallback(payload, "job_id", "jobId"),
		Status:        getString(payload, "status"),
		SourceType:    getStringWithFallback(payload, "source_type", "sourceType"),
		EntityType:    getStringWithFallback(payload, "entity_type", "entityType"),
		ResultCount:   getIntWithFallback(payload, "result_count", "resultCount"),
		FailedCount:   getIntWithFallback(payload, "failed_count", "failedCount"),
		TotalURLs:     totalURLs,
		CompletedURLs: totalURLs, // All URLs completed
		PendingURLs:   0,         // No pending URLs
		Duration:      getFloat64WithFallback(payload, "duration_seconds", "durationSeconds"),
		Timestamp:     getTimestamp(payload),
	}

	s.handler.BroadcastJobStatusChange(update)
	return nil
}

func (s *EventSubscriber) handleJobFailed(ctx context.Context, event interfaces.Event) error {
	// Check if event should be broadcast (filtering + throttling)
	if !s.shouldBroadcastEvent("job_failed") {
		return nil
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		s.logger.Warn().Msg("Invalid job failed event payload type")
		return nil
	}

	update := JobStatusUpdate{
		JobID:         getStringWithFallback(payload, "job_id", "jobId"),
		Status:        getString(payload, "status"),
		SourceType:    getStringWithFallback(payload, "source_type", "sourceType"),
		EntityType:    getStringWithFallback(payload, "entity_type", "entityType"),
		ResultCount:   getIntWithFallback(payload, "result_count", "resultCount"),
		FailedCount:   getIntWithFallback(payload, "failed_count", "failedCount"),
		CompletedURLs: getIntWithFallback(payload, "completed_urls", "completedUrls"),
		PendingURLs:   getIntWithFallback(payload, "pending_urls", "pendingUrls"),
		Error:         getString(payload, "error"),
		Timestamp:     getTimestamp(payload),
	}

	s.handler.BroadcastJobStatusChange(update)
	return nil
}

func (s *EventSubscriber) handleJobCancelled(ctx context.Context, event interfaces.Event) error {
	// Check if event should be broadcast (filtering + throttling)
	if !s.shouldBroadcastEvent("job_cancelled") {
		return nil
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		s.logger.Warn().Msg("Invalid job cancelled event payload type")
		return nil
	}

	update := JobStatusUpdate{
		JobID:         getStringWithFallback(payload, "job_id", "jobId"),
		Status:        getString(payload, "status"),
		SourceType:    getStringWithFallback(payload, "source_type", "sourceType"),
		EntityType:    getStringWithFallback(payload, "entity_type", "entityType"),
		ResultCount:   getIntWithFallback(payload, "result_count", "resultCount"),
		FailedCount:   getIntWithFallback(payload, "failed_count", "failedCount"),
		CompletedURLs: getIntWithFallback(payload, "completed_urls", "completedUrls"),
		PendingURLs:   getIntWithFallback(payload, "pending_urls", "pendingUrls"),
		Timestamp:     getTimestamp(payload),
	}

	s.handler.BroadcastJobStatusChange(update)
	return nil
}
