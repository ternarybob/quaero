package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Service implements EventService interface with pub/sub pattern
type Service struct {
	subscribers map[interfaces.EventType][]interfaces.EventHandler
	mu          sync.RWMutex
	logger      arbor.ILogger
}

// NewService creates a new event service
func NewService(logger arbor.ILogger) interfaces.EventService {
	return &Service{
		subscribers: make(map[interfaces.EventType][]interfaces.EventHandler),
		logger:      logger,
	}
}

// Subscribe registers a handler for an event type
func (s *Service) Subscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribers[eventType] = append(s.subscribers[eventType], handler)

	s.logger.Debug().
		Str("event_type", string(eventType)).
		Int("subscriber_count", len(s.subscribers[eventType])).
		Msg("Event handler subscribed")

	return nil
}

// Unsubscribe removes a handler from an event type
func (s *Service) Unsubscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	handlers := s.subscribers[eventType]
	for i, h := range handlers {
		if &h == &handler {
			s.subscribers[eventType] = append(handlers[:i], handlers[i+1:]...)
			s.logger.Debug().
				Str("event_type", string(eventType)).
				Msg("Event handler unsubscribed")
			return nil
		}
	}

	return fmt.Errorf("handler not found for event type: %s", eventType)
}

// Publish sends an event to all subscribers asynchronously
func (s *Service) Publish(ctx context.Context, event interfaces.Event) error {
	s.mu.RLock()
	handlers := s.subscribers[event.Type]
	s.mu.RUnlock()

	if len(handlers) == 0 {
		s.logger.Debug().
			Str("event_type", string(event.Type)).
			Msg("No subscribers for event")
		return nil
	}

	s.logger.Info().
		Str("event_type", string(event.Type)).
		Int("subscriber_count", len(handlers)).
		Msg("Publishing event")

	for _, handler := range handlers {
		go func(h interfaces.EventHandler) {
			if err := h(ctx, event); err != nil {
				s.logger.Error().
					Err(err).
					Str("event_type", string(event.Type)).
					Msg("Event handler failed")
			}
		}(handler)
	}

	return nil
}

// PublishSync sends an event to all subscribers synchronously
func (s *Service) PublishSync(ctx context.Context, event interfaces.Event) error {
	s.mu.RLock()
	handlers := s.subscribers[event.Type]
	s.mu.RUnlock()

	if len(handlers) == 0 {
		s.logger.Debug().
			Str("event_type", string(event.Type)).
			Msg("No subscribers for event")
		return nil
	}

	s.logger.Info().
		Str("event_type", string(event.Type)).
		Int("subscriber_count", len(handlers)).
		Msg("Publishing event synchronously")

	var wg sync.WaitGroup
	errChan := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h interfaces.EventHandler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				s.logger.Error().
					Err(err).
					Str("event_type", string(event.Type)).
					Msg("Event handler failed")
				errChan <- err
			}
		}(handler)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("event handlers failed: %d errors", len(errs))
	}

	return nil
}

// Close shuts down the event service
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribers = make(map[interfaces.EventType][]interfaces.EventHandler)
	s.logger.Info().Msg("Event service closed")

	return nil
}
