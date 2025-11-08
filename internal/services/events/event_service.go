package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// nonLoggableEvents defines event types that should NOT be logged by EventService
// to prevent circular logging conditions (e.g., log_event triggering more log_event)
var nonLoggableEvents = map[interfaces.EventType]bool{
	"log_event": true, // Log events are published by LogConsumer - don't log them
}

// Service implements EventService interface with pub/sub pattern
type Service struct {
	subscribers map[interfaces.EventType][]interfaces.EventHandler
	mu          sync.RWMutex
	logger      arbor.ILogger
}

// NewService creates a new event service
func NewService(logger arbor.ILogger) interfaces.EventService {
	service := &Service{
		subscribers: make(map[interfaces.EventType][]interfaces.EventHandler),
		logger:      logger,
	}

	// Subscribe logger to all event types for centralized event logging
	if err := SubscribeLoggerToAllEvents(service, logger); err != nil {
		logger.Warn().Err(err).Msg("Failed to subscribe logger to all events")
	}

	return service
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

	// Only log event publication if not in blacklist (prevents circular logging)
	if !nonLoggableEvents[event.Type] {
		s.logger.Info().
			Str("event_type", string(event.Type)).
			Int("subscriber_count", len(handlers)).
			Msg("Publishing event")
	}

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
	s.logger.Debug().
		Str("event_type", string(event.Type)).
		Msg("*** EVENT SERVICE: PublishSync START")

	s.logger.Debug().Msg("*** EVENT SERVICE: Acquiring read lock for subscribers")
	s.mu.RLock()
	handlers := s.subscribers[event.Type]
	s.mu.RUnlock()
	s.logger.Debug().
		Int("handler_count", len(handlers)).
		Msg("*** EVENT SERVICE: Read lock released, handlers retrieved")

	if len(handlers) == 0 {
		s.logger.Debug().
			Str("event_type", string(event.Type)).
			Msg("*** EVENT SERVICE: No subscribers for event - returning")
		return nil
	}

	// Only log event publication if not in blacklist (prevents circular logging)
	if !nonLoggableEvents[event.Type] {
		s.logger.Info().
			Str("event_type", string(event.Type)).
			Int("subscriber_count", len(handlers)).
			Msg("*** EVENT SERVICE: Publishing event synchronously to all handlers")
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(handlers))
	panicChan := make(chan interface{}, len(handlers))

	s.logger.Debug().
		Int("handler_count", len(handlers)).
		Msg("*** EVENT SERVICE: Starting goroutines for all handlers")

	for i, handler := range handlers {
		wg.Add(1)
		handlerIndex := i
		s.logger.Debug().
			Int("handler_index", handlerIndex).
			Str("event_type", string(event.Type)).
			Msg("*** EVENT SERVICE: Launching handler goroutine")

		go func(h interfaces.EventHandler, idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error().
						Str("panic", fmt.Sprintf("%v", r)).
						Str("event_type", string(event.Type)).
						Int("handler_index", idx).
						Msg("*** EVENT SERVICE: PANIC RECOVERED in event handler")
					panicChan <- r
				}
			}()

			s.logger.Debug().
				Int("handler_index", idx).
				Str("event_type", string(event.Type)).
				Msg("*** EVENT SERVICE: Calling handler")

			if err := h(ctx, event); err != nil {
				s.logger.Error().
					Err(err).
					Str("event_type", string(event.Type)).
					Int("handler_index", idx).
					Msg("*** EVENT SERVICE: Handler returned error")
				errChan <- err
			} else {
				s.logger.Debug().
					Int("handler_index", idx).
					Str("event_type", string(event.Type)).
					Msg("*** EVENT SERVICE: Handler completed successfully")
			}
		}(handler, handlerIndex)
	}

	s.logger.Debug().Msg("*** EVENT SERVICE: Waiting for all handlers to complete")
	wg.Wait()
	s.logger.Debug().Msg("*** EVENT SERVICE: All handlers completed")

	close(errChan)
	close(panicChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// Check for panics
	var panics []interface{}
	for p := range panicChan {
		panics = append(panics, p)
	}

	s.logger.Debug().
		Int("panic_count", len(panics)).
		Int("error_count", len(errs)).
		Msg("*** EVENT SERVICE: Checked channels for panics and errors")

	if len(panics) > 0 {
		s.logger.Error().
			Int("panic_count", len(panics)).
			Msg("*** EVENT SERVICE: FAILED - Handlers panicked")
		return fmt.Errorf("event handlers panicked: %d panics", len(panics))
	}

	if len(errs) > 0 {
		s.logger.Error().
			Int("error_count", len(errs)).
			Msg("*** EVENT SERVICE: FAILED - Handlers returned errors")
		for i, err := range errs {
			s.logger.Error().
				Int("error_index", i).
				Err(err).
				Msg("*** EVENT SERVICE: Handler error detail")
		}
		return fmt.Errorf("event handlers failed: %d errors", len(errs))
	}

	s.logger.Debug().
		Str("event_type", string(event.Type)).
		Msg("*** EVENT SERVICE: PublishSync END - Success")
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
