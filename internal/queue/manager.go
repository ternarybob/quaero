package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"maragu.dev/goqite"
)

// Manager is a thin wrapper around goqite.
// It provides ONLY queue operations, no business logic.
type Manager struct {
	q *goqite.Queue
}

// NewManager creates a new queue manager.
func NewManager(db *sql.DB, queueName string) (*Manager, error) {
	// Setup creates the goqite tables if they don't exist
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := goqite.Setup(ctx, db); err != nil {
		return nil, err
	}

	q := goqite.New(goqite.NewOpts{
		DB:   db,
		Name: queueName,
	})

	return &Manager{q: q}, nil
}

// Enqueue adds a message to the queue.
// This is the ONLY way to add jobs to the queue.
func (m *Manager) Enqueue(ctx context.Context, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return m.q.Send(ctx, goqite.Message{
		Body: data,
	})
}

// Receive pulls the next message from the queue.
// Returns the message and a delete function to call after processing.
func (m *Manager) Receive(ctx context.Context) (*Message, func() error, error) {
	gMsg, err := m.q.Receive(ctx)
	if err != nil {
		return nil, nil, err
	}

	var msg Message
	if err := json.Unmarshal(gMsg.Body, &msg); err != nil {
		return nil, nil, err
	}

	// Return delete function for worker to call after successful processing
	deleteFn := func() error {
		return m.q.Delete(ctx, gMsg.ID)
	}

	return &msg, deleteFn, nil
}

// Extend extends the visibility timeout for a long-running job.
// Call this periodically during job execution to prevent re-delivery.
func (m *Manager) Extend(ctx context.Context, messageID goqite.ID, duration time.Duration) error {
	return m.q.Extend(ctx, messageID, duration)
}

// Close closes the queue manager.
func (m *Manager) Close() error {
	// goqite doesn't require explicit close, but we provide it for consistency
	return nil
}
