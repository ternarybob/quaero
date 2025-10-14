package status

import (
	"context"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// AppState represents the application state
type AppState string

const (
	StateIdle     AppState = "idle"
	StateCrawling AppState = "crawling"
	StateOffline  AppState = "offline"
)

// Service manages application status
type Service struct {
	state        AppState
	mu           sync.RWMutex
	eventService interfaces.EventService
	logger       arbor.ILogger
	metadata     map[string]interface{}
}

// NewService creates a new StatusService
func NewService(eventService interfaces.EventService, logger arbor.ILogger) *Service {
	return &Service{
		state:        StateIdle,
		eventService: eventService,
		logger:       logger,
		metadata:     make(map[string]interface{}),
	}
}

// GetState returns the current application state (thread-safe)
func (s *Service) GetState() AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState updates the application state and broadcasts the change
func (s *Service) SetState(state AppState, metadata map[string]interface{}) {
	s.mu.Lock()
	oldState := s.state
	s.state = state
	if metadata != nil {
		s.metadata = metadata
	} else {
		s.metadata = make(map[string]interface{})
	}
	s.mu.Unlock()

	s.logger.Info().
		Str("old_state", string(oldState)).
		Str("new_state", string(state)).
		Msg("Application state changed")

	// Publish state change event
	payload := map[string]interface{}{
		"state":     string(state),
		"metadata":  metadata,
		"timestamp": time.Now(),
	}
	event := interfaces.Event{
		Type:    interfaces.EventStatusChanged,
		Payload: payload,
	}
	s.eventService.Publish(context.Background(), event)
}

// GetStatus returns the full status including state, metadata, and timestamp
func (s *Service) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Deep copy metadata to avoid concurrent modification
	metadataCopy := make(map[string]interface{})
	for k, v := range s.metadata {
		metadataCopy[k] = v
	}

	return map[string]interface{}{
		"state":     string(s.state),
		"metadata":  metadataCopy,
		"timestamp": time.Now(),
	}
}

// SubscribeToCrawlerEvents subscribes to crawler events to automatically update state
func (s *Service) SubscribeToCrawlerEvents() {
	// Subscribe to crawler progress events
	s.eventService.Subscribe(interfaces.EventCrawlProgress, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			return nil
		}

		status, ok := payload["status"].(string)
		if !ok {
			return nil
		}

		switch status {
		case "started", "running":
			// Extract job information
			metadata := map[string]interface{}{}
			if jobID, ok := payload["job_id"].(string); ok {
				metadata["active_job_id"] = jobID
			}
			if sourceType, ok := payload["source_type"].(string); ok {
				metadata["source_type"] = sourceType
			}
			if progress, ok := payload["progress"].(float64); ok {
				metadata["progress"] = progress
			}
			s.SetState(StateCrawling, metadata)

		case "completed", "failed", "cancelled":
			// Clear metadata and return to idle
			s.SetState(StateIdle, nil)
		}

		return nil
	})

	s.logger.Info().Msg("StatusService subscribed to crawler events")
}
