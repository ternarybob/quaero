// Package interfaces provides service interfaces for dependency injection.
package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// CacheService provides document cache freshness checking and management.
// Used by the queue system to determine if cached documents can be reused.
type CacheService interface {
	// IsFresh checks if a document is still fresh based on cache config.
	// Returns true if the document can be reused from cache.
	IsFresh(doc *models.Document, config models.CacheConfig) bool

	// GetFreshDocument retrieves the most recent fresh document matching cache tags.
	// Returns the document and true if a fresh cached document exists.
	// Returns nil and false if no fresh cached document is available.
	GetFreshDocument(ctx context.Context, tags []string, config models.CacheConfig) (*models.Document, bool)

	// CleanupRevisions removes excess revisions beyond the configured limit.
	// Used to maintain the revision count for a job/step combination.
	CleanupRevisions(ctx context.Context, jobDefID, stepName string, keepCount int) error

	// GetCurrentRevision returns the current revision number for a job/step.
	// Returns 0 if no documents exist for this job/step.
	GetCurrentRevision(ctx context.Context, jobDefID, stepName string) (int, error)
}
