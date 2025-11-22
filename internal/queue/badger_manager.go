// -----------------------------------------------------------------------
// Last Modified: Friday, 22nd November 2025 5:40:51 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/timshannon/badgerhold/v4"
)

// BadgerManager implements a persistent message queue using Badger.
// It provides FIFO ordering, visibility timeouts, and redelivery tracking.
type BadgerManager struct {
	store             *badgerhold.Store
	queueName         string
	visibilityTimeout time.Duration
	maxReceive        int
}

// QueueMessage represents a message stored in the Badger queue
type QueueMessage struct {
	ID           string          `json:"id" badgerhold:"key"`
	Body         Message         `json:"body"`
	EnqueuedAt   time.Time       `json:"enqueued_at" badgerhold:"index"`
	VisibleAt    time.Time       `json:"visible_at" badgerhold:"index"`
	ReceiveCount int             `json:"receive_count"`
	QueueName    string          `json:"queue_name" badgerhold:"index"`
}

// NewBadgerManager creates a new Badger-backed queue manager.
func NewBadgerManager(store *badgerhold.Store, queueName string, visibilityTimeout time.Duration, maxReceive int) (*BadgerManager, error) {
	if store == nil {
		return nil, fmt.Errorf("badgerhold store is required")
	}
	if queueName == "" {
		return nil, fmt.Errorf("queue name is required")
	}
	if visibilityTimeout <= 0 {
		visibilityTimeout = 30 * time.Second
	}
	if maxReceive <= 0 {
		maxReceive = 3
	}

	return &BadgerManager{
		store:             store,
		queueName:         queueName,
		visibilityTimeout: visibilityTimeout,
		maxReceive:        maxReceive,
	}, nil
}

// Enqueue adds a message to the queue.
func (m *BadgerManager) Enqueue(ctx context.Context, msg Message) error {
	// Generate unique message ID with timestamp prefix for FIFO ordering
	now := time.Now()
	messageID := fmt.Sprintf("%019d:%s", now.UnixNano(), uuid.New().String())

	qMsg := QueueMessage{
		ID:           messageID,
		Body:         msg,
		EnqueuedAt:   now,
		VisibleAt:    now, // Immediately visible
		ReceiveCount: 0,
		QueueName:    m.queueName,
	}

	if err := m.store.Insert(messageID, &qMsg); err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	return nil
}

// Receive retrieves the next visible message from the queue.
// Returns the message and a delete function to call after processing.
func (m *BadgerManager) Receive(ctx context.Context) (*Message, func() error, error) {
	now := time.Now()

	// Query for messages in this queue that are visible and haven't exceeded max receives
	// Sort by ID (which has timestamp prefix) for FIFO ordering
	var messages []QueueMessage
	err := m.store.Find(&messages,
		badgerhold.Where("QueueName").Eq(m.queueName).
			And("VisibleAt").Le(now).
			And("ReceiveCount").Lt(m.maxReceive).
			SortBy("ID").
			Limit(1))

	if err != nil {
		return nil, nil, fmt.Errorf("failed to receive message: %w", err)
	}

	if len(messages) == 0 {
		return nil, nil, ErrNoMessage
	}

	foundMsg := messages[0]

	// Update message visibility and receive count
	foundMsg.ReceiveCount++
	foundMsg.VisibleAt = now.Add(m.visibilityTimeout)

	if err := m.store.Update(foundMsg.ID, &foundMsg); err != nil {
		return nil, nil, fmt.Errorf("failed to update message visibility: %w", err)
	}

	// Create delete function that removes the message from the queue
	messageID := foundMsg.ID
	deleteFn := func() error {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check context
		select {
		case <-deleteCtx.Done():
			return deleteCtx.Err()
		default:
		}

		return m.store.Delete(messageID, &QueueMessage{})
	}

	return &foundMsg.Body, deleteFn, nil
}

// Extend extends the visibility timeout for a message.
func (m *BadgerManager) Extend(ctx context.Context, messageID string, duration time.Duration) error {
	var qMsg QueueMessage
	if err := m.store.Get(messageID, &qMsg); err != nil {
		if err == badgerhold.ErrNotFound {
			return fmt.Errorf("message not found: %s", messageID)
		}
		return fmt.Errorf("failed to find message: %w", err)
	}

	// Extend visibility timeout
	qMsg.VisibleAt = time.Now().Add(duration)

	if err := m.store.Update(messageID, &qMsg); err != nil {
		return fmt.Errorf("failed to extend message visibility: %w", err)
	}

	return nil
}

// Close closes the queue manager.
func (m *BadgerManager) Close() error {
	// Badger DB is owned by the storage manager, don't close it here
	return nil
}
