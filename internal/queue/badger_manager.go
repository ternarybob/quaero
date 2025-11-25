package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

// ErrNoMessage is returned when the queue is empty
// var ErrNoMessage = errors.New("no messages in queue") // Already defined in manager.go, but we'll redefine if we delete manager.go first or just use it.
// Since we are replacing manager.go, we should ensure this is available.
// If manager.go is deleted, we need to define it here or in types.go.
// Let's assume types.go is a better place for shared errors if we had one, but for now we'll define it here if not present.
// Actually, manager.go defines it. If we delete manager.go, we need it here.

// QueueMessage represents the internal structure stored in Badger
type QueueMessage struct {
	ID           string    `json:"id"`
	Body         Message   `json:"body"`
	EnqueuedAt   time.Time `json:"enqueued_at"`
	VisibleAt    time.Time `json:"visible_at"`
	ReceiveCount int       `json:"receive_count"`
	DedupID      string    `json:"dedup_id,omitempty"` // Optional deduplication ID
}

// BadgerManager implements a persistent queue using BadgerDB
type BadgerManager struct {
	db                *badger.DB
	queueName         string
	visibilityTimeout time.Duration
	maxReceive        int
}

// NewBadgerManager creates a new Badger-backed queue manager
func NewBadgerManager(db *badger.DB, queueName string, visibilityTimeout time.Duration, maxReceive int) (*BadgerManager, error) {
	if db == nil {
		return nil, errors.New("badger db is required")
	}
	if queueName == "" {
		return nil, errors.New("queue name is required")
	}
	if visibilityTimeout <= 0 {
		visibilityTimeout = 5 * time.Minute // Default
	}
	if maxReceive <= 0 {
		maxReceive = 3 // Default
	}

	return &BadgerManager{
		db:                db,
		queueName:         queueName,
		visibilityTimeout: visibilityTimeout,
		maxReceive:        maxReceive,
	}, nil
}

// Enqueue adds a message to the queue
func (m *BadgerManager) Enqueue(ctx context.Context, msg Message) error {
	// Generate a unique ID for the message
	id := uuid.New().String()

	// Create internal message wrapper
	qMsg := QueueMessage{
		ID:           id,
		Body:         msg,
		EnqueuedAt:   time.Now(),
		VisibleAt:    time.Now(), // Immediately visible
		ReceiveCount: 0,
	}

	// Serialize
	data, err := json.Marshal(qMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal queue message: %w", err)
	}

	// Store in Badger
	// Key format: queue:{queueName}:msg:{visibleAt}:{id}
	// We use visibleAt in the key to allow efficient scanning for ready messages
	// However, if we update visibility, we need to move the key.
	// Alternative: Key is ID, and we use an index or scan.
	// Scanning all keys is expensive.
	// Badger supports iteration.
	// Let's use a composite key for ordering: queue:{queueName}:visible:{timestamp}:{id} -> ID
	// And the data stored at: queue:{queueName}:data:{id} -> JSON

	// Actually, simpler approach for now:
	// Store data at: queue:{queueName}:msg:{id}
	// Maintain an index for visibility: queue:{queueName}:index:{visibleAt}:{id} -> empty

	return m.db.Update(func(txn *badger.Txn) error {
		// 1. Store message data
		msgKey := m.msgKey(id)
		if err := txn.Set(msgKey, data); err != nil {
			return err
		}

		// 2. Add to visibility index
		indexKey := m.indexKey(qMsg.VisibleAt, id)
		if err := txn.Set(indexKey, []byte{}); err != nil {
			return err
		}

		return nil
	})
}

// Receive pulls the next visible message from the queue
func (m *BadgerManager) Receive(ctx context.Context) (*Message, func() error, error) {
	var qMsg QueueMessage
	var msgID string
	var oldIndexKey []byte

	err := m.db.Update(func(txn *badger.Txn) error {
		// Iterate over visibility index to find a ready message
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		prefix := []byte(fmt.Sprintf("queue:%s:index:", m.queueName))
		it := txn.NewIterator(opts)
		defer it.Close()

		now := time.Now()
		found := false

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

			// Parse timestamp from key
			// Key format: queue:{queueName}:index:{timestamp}:{id}
			// We need to extract timestamp and compare with now

			// Helper to parse key
			ts, id, err := m.parseIndexKey(key)
			if err != nil {
				continue // Skip invalid keys
			}

			if ts.After(now) {
				// Since keys are sorted by timestamp, if we hit a future timestamp,
				// no subsequent messages are ready either.
				break
			}

			// Found a candidate!
			// Get the actual message data
			msgKey := m.msgKey(id)
			itemMsg, err := txn.Get(msgKey)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					// Index exists but message doesn't? Inconsistent state, clean up index
					if err := txn.Delete(key); err != nil {
						return err
					}
					continue
				}
				return err
			}

			if err := itemMsg.Value(func(val []byte) error {
				return json.Unmarshal(val, &qMsg)
			}); err != nil {
				return err
			}

			// Check max receive count
			if qMsg.ReceiveCount >= m.maxReceive {
				// Move to DLQ or delete? For now, just delete/log and skip
				// In a real system, we'd move to DLQ.
				// Here we'll just delete it to prevent poison pill loops
				if err := txn.Delete(key); err != nil {
					return err
				}
				if err := txn.Delete(msgKey); err != nil {
					return err
				}
				continue
			}

			// Claim this message
			found = true
			msgID = id
			oldIndexKey = key // Copy key bytes
			break
		}

		if !found {
			return ErrNoMessage
		}

		// Update message: increment receive count, update visibility
		qMsg.ReceiveCount++
		qMsg.VisibleAt = time.Now().Add(m.visibilityTimeout)

		// 1. Update message data
		newData, err := json.Marshal(qMsg)
		if err != nil {
			return err
		}
		if err := txn.Set(m.msgKey(msgID), newData); err != nil {
			return err
		}

		// 2. Update index: delete old key, add new key
		if err := txn.Delete(oldIndexKey); err != nil {
			return err
		}
		newIndexKey := m.indexKey(qMsg.VisibleAt, msgID)
		if err := txn.Set(newIndexKey, []byte{}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	// Return message and delete function
	deleteFn := func() error {
		return m.db.Update(func(txn *badger.Txn) error {
			// To delete, we need to find the current index key.
			// Since visibility might have changed (if extended), or we just know the ID.
			// We can look up the message to get the current VisibleAt.

			msgKey := m.msgKey(msgID)
			item, err := txn.Get(msgKey)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					return nil // Already deleted
				}
				return err
			}

			var currentMsg QueueMessage
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &currentMsg)
			}); err != nil {
				return err
			}

			// Delete index
			idxKey := m.indexKey(currentMsg.VisibleAt, msgID)
			if err := txn.Delete(idxKey); err != nil {
				// If not found, maybe it was moved/updated?
				// Ignore not found for index deletion to be safe
				if err != badger.ErrKeyNotFound {
					return err
				}
			}

			// Delete data
			return txn.Delete(msgKey)
		})
	}

	return &qMsg.Body, deleteFn, nil
}

// Extend extends the visibility timeout for a message
func (m *BadgerManager) Extend(ctx context.Context, messageID string, duration time.Duration) error {
	return m.db.Update(func(txn *badger.Txn) error {
		msgKey := m.msgKey(messageID)
		item, err := txn.Get(msgKey)
		if err != nil {
			return err
		}

		var qMsg QueueMessage
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &qMsg)
		}); err != nil {
			return err
		}

		// Calculate new visibility
		oldVisibleAt := qMsg.VisibleAt
		qMsg.VisibleAt = time.Now().Add(duration)

		// Update data
		newData, err := json.Marshal(qMsg)
		if err != nil {
			return err
		}
		if err := txn.Set(msgKey, newData); err != nil {
			return err
		}

		// Update index
		oldIndexKey := m.indexKey(oldVisibleAt, messageID)
		if err := txn.Delete(oldIndexKey); err != nil {
			// If old index key not found, it's weird but proceed
			if err != badger.ErrKeyNotFound {
				return err
			}
		}

		newIndexKey := m.indexKey(qMsg.VisibleAt, messageID)
		if err := txn.Set(newIndexKey, []byte{}); err != nil {
			return err
		}

		return nil
	})
}

// Close closes the queue manager (no-op for BadgerManager as DB is managed externally)
func (m *BadgerManager) Close() error {
	return nil
}

// Helpers

func (m *BadgerManager) msgKey(id string) []byte {
	return []byte(fmt.Sprintf("queue:%s:msg:%s", m.queueName, id))
}

func (m *BadgerManager) indexKey(visibleAt time.Time, id string) []byte {
	ts := visibleAt.UnixNano()
	// Zero pad to 20 digits to ensure string sorting works like number sorting
	return []byte(fmt.Sprintf("queue:%s:index:%020d:%s", m.queueName, ts, id))
}

func (m *BadgerManager) parseIndexKey(key []byte) (time.Time, string, error) {
	prefixStr := fmt.Sprintf("queue:%s:index:", m.queueName)
	if len(key) <= len(prefixStr) {
		return time.Time{}, "", fmt.Errorf("invalid key length")
	}

	suffix := string(key[len(prefixStr):])
	// Suffix is "{20-digit-ts}:{id}"

	if len(suffix) < 21 { // 20 digits + 1 colon
		return time.Time{}, "", fmt.Errorf("invalid suffix length")
	}

	tsStr := suffix[:20]
	id := suffix[21:]

	var ts int64
	_, err := fmt.Sscanf(tsStr, "%d", &ts)
	if err != nil {
		return time.Time{}, "", err
	}

	return time.Unix(0, ts), id, nil
}
