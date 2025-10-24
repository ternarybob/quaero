package queue

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"maragu.dev/goqite"
)

// Manager manages the goqite-backed job queue
type Manager struct {
	queue  *goqite.Queue
	config Config
	logger arbor.ILogger
	ctx    context.Context
	cancel context.CancelFunc
	db     *sql.DB
}

// NewManager creates a new queue manager
func NewManager(db *sql.DB, config Config, logger arbor.ILogger) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create goqite queue instance
	queue := goqite.New(goqite.NewOpts{
		DB:         db,
		Name:       config.QueueName,
		Timeout:    config.VisibilityTimeout,
		MaxReceive: config.MaxReceive,
	})

	m := &Manager{
		queue:  queue,
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		db:     db,
	}

	logger.Info().
		Str("queue_name", config.QueueName).
		Int("concurrency", config.Concurrency).
		Dur("visibility_timeout", config.VisibilityTimeout).
		Msg("Queue manager initialized")

	return m, nil
}

// Start starts the queue manager
func (m *Manager) Start() error {
	// Create new context if current one is done (e.g., after Stop() was called)
	if m.ctx == nil || m.ctx.Err() != nil {
		m.ctx, m.cancel = context.WithCancel(context.Background())
		m.logger.Debug().Msg("Recreated queue manager context")
	}
	m.logger.Info().Msg("Queue manager started")
	return nil
}

// Stop gracefully stops the queue manager
func (m *Manager) Stop() error {
	m.logger.Info().Msg("Stopping queue manager")
	m.cancel()
	return nil
}

// Restart restarts the queue manager
func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return fmt.Errorf("failed to stop queue manager: %w", err)
	}
	return m.Start()
}

// Enqueue sends a message to the queue
func (m *Manager) Enqueue(ctx context.Context, msg *JobMessage) error {
	// Serialize message to JSON
	body, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Send to queue
	if err := m.queue.Send(ctx, goqite.Message{
		ID:   goqite.ID(msg.ID),
		Body: body,
	}); err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	m.logger.Debug().
		Str("message_id", msg.ID).
		Str("type", msg.Type).
		Str("parent_id", msg.ParentID).
		Msg("Message enqueued")

	return nil
}

// EnqueueWithDelay sends a delayed message to the queue
func (m *Manager) EnqueueWithDelay(ctx context.Context, msg *JobMessage, delay time.Duration) error {
	// Serialize message to JSON
	body, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Send to queue with delay
	if err := m.queue.Send(ctx, goqite.Message{
		ID:    goqite.ID(msg.ID),
		Body:  body,
		Delay: delay,
	}); err != nil {
		return fmt.Errorf("failed to enqueue delayed message: %w", err)
	}

	m.logger.Debug().
		Str("message_id", msg.ID).
		Str("type", msg.Type).
		Dur("delay", delay).
		Msg("Delayed message enqueued")

	return nil
}

// Receive retrieves a message from the queue
func (m *Manager) Receive(ctx context.Context) (*goqite.Message, error) {
	msg, err := m.queue.Receive(ctx)
	if err != nil {
		return nil, err
	}
	// goqite can return (nil, nil) when no message is available
	if msg == nil {
		return nil, fmt.Errorf("no message")
	}
	return msg, nil
}

// Delete removes a message from the queue
func (m *Manager) Delete(ctx context.Context, msg goqite.Message) error {
	return m.queue.Delete(ctx, msg.ID)
}

// Extend extends the visibility timeout of a message
func (m *Manager) Extend(ctx context.Context, msg goqite.Message, duration time.Duration) error {
	return m.queue.Extend(ctx, msg.ID, duration)
}

// GetQueueLength returns the current queue length
func (m *Manager) GetQueueLength(ctx context.Context) (int, error) {
	// Query the queue table for pending messages (timeout <= now means ready to receive)
	var count int
	query := `SELECT COUNT(*) FROM goqite WHERE queue = ? AND timeout <= strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`
	err := m.db.QueryRowContext(ctx, query, m.config.QueueName).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}
	return count, nil
}

// GetQueueStats returns queue statistics
func (m *Manager) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get total messages
	var total int
	query := `SELECT COUNT(*) FROM goqite WHERE queue = ?`
	if err := m.db.QueryRowContext(ctx, query, m.config.QueueName).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to get total messages: %w", err)
	}
	stats["total_messages"] = total

	// Get pending messages
	pending, err := m.GetQueueLength(ctx)
	if err != nil {
		return nil, err
	}
	stats["pending_messages"] = pending

	// Get in-flight messages (timeout > now and received > 0 means message is being processed)
	var inFlight int
	query = `SELECT COUNT(*) FROM goqite WHERE queue = ? AND timeout > strftime('%Y-%m-%dT%H:%M:%fZ', 'now') AND received > 0`
	if err := m.db.QueryRowContext(ctx, query, m.config.QueueName).Scan(&inFlight); err != nil {
		return nil, fmt.Errorf("failed to get in-flight messages: %w", err)
	}
	stats["in_flight_messages"] = inFlight

	// Add configuration
	stats["queue_name"] = m.config.QueueName
	stats["concurrency"] = m.config.Concurrency
	stats["visibility_timeout"] = m.config.VisibilityTimeout.String()

	return stats, nil
}

// Queue returns the underlying goqite queue
func (m *Manager) Queue() *goqite.Queue {
	return m.queue
}
