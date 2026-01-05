// -----------------------------------------------------------------------
// Last Modified: Thursday, 14th November 2025 12:00:00 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package interfaces

import (
	"context"
	"errors"
	"time"
)

// ErrKeyNotFound is returned when a key is not found in the key/value store
var ErrKeyNotFound = errors.New("key not found")

// KeyValuePair represents a single key/value pair with metadata
type KeyValuePair struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// KeyValueStorage defines operations for generic key/value storage
type KeyValueStorage interface {
	// Get retrieves a value by key, returns error if not found
	Get(ctx context.Context, key string) (string, error)

	// GetPair retrieves a full KeyValuePair by key, returns error if not found
	GetPair(ctx context.Context, key string) (*KeyValuePair, error)

	// Set inserts or updates a key/value pair with optional description
	Set(ctx context.Context, key string, value string, description string) error

	// Upsert inserts or updates a key/value pair with explicit logging of the operation
	// Returns true if a new key was created, false if an existing key was updated
	Upsert(ctx context.Context, key string, value string, description string) (bool, error)

	// Delete removes a key/value pair, returns error if not found
	Delete(ctx context.Context, key string) error

	// DeleteAll removes all key/value pairs from storage
	DeleteAll(ctx context.Context) error

	// List returns all key/value pairs ordered by updated_at DESC
	List(ctx context.Context) ([]KeyValuePair, error)

	// GetAll returns all key/value pairs as a map (useful for bulk operations)
	GetAll(ctx context.Context) (map[string]string, error)

	// ListByPrefix returns all key/value pairs with keys starting with the given prefix
	ListByPrefix(ctx context.Context, prefix string) ([]KeyValuePair, error)
}
