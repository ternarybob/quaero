// Package interfaces provides service interfaces for dependency injection.
package interfaces

import "context"

// DocumentProvider is implemented by workers that can provision documents for other workers.
// This enables worker-to-worker communication without coupling to concrete types.
//
// Pattern:
//   - If document exists and is fresh (within CacheHours), returns existing document ID
//   - If document is missing or stale, fetches/generates and saves new document
//   - Returns document IDs only, NOT the data - caller retrieves via DocumentStorage
//
// Example usage:
//
//	// Worker A needs data from Worker B
//	result, err := workerB.GetDocument(ctx, "ASX:GNP",
//	    interfaces.WithCacheHours(24),
//	    interfaces.WithForceRefresh(false),
//	)
//	if err != nil {
//	    return err
//	}
//	// Use document ID to retrieve content
//	doc, _ := documentStorage.GetDocument(result.DocumentID)
type DocumentProvider interface {
	// GetDocument ensures a document exists for a single identifier.
	// Returns the document result with ID, freshness info, and whether it was created.
	// The identifier format is worker-specific (e.g., "ASX:GNP" for market data).
	GetDocument(ctx context.Context, identifier string, opts ...DocumentOption) (*DocumentResult, error)

	// GetDocuments ensures documents exist for multiple identifiers.
	// Returns results for each identifier (may include errors for individual items).
	// Processing continues even if individual identifiers fail.
	GetDocuments(ctx context.Context, identifiers []string, opts ...DocumentOption) ([]*DocumentResult, error)
}

// DocumentResult contains the result of document provisioning.
type DocumentResult struct {
	// Identifier is the original identifier requested (e.g., "ASX:GNP")
	Identifier string

	// DocumentID is the database document ID that can be used with DocumentStorage.GetDocument()
	DocumentID string

	// Tags are the tags applied to the document
	Tags []string

	// Fresh is true if the document was already fresh (cache hit)
	Fresh bool

	// Created is true if the document was newly created (not from cache)
	Created bool

	// Error is non-nil if provisioning failed for this identifier
	// When Error is set, DocumentID may be empty
	Error error
}

// DocumentOption configures document provisioning behavior.
type DocumentOption func(*DocumentOptions)

// DocumentOptions holds all configurable options for document provisioning.
type DocumentOptions struct {
	// CacheHours is the freshness window for cached documents.
	// Documents older than this will be regenerated.
	// Set to 0 to always fetch fresh data.
	CacheHours int

	// ForceRefresh bypasses cache and always generates fresh documents.
	// Takes precedence over CacheHours.
	ForceRefresh bool

	// ManagerID is the job manager ID for document isolation.
	// When set, the document will be associated with this manager.
	ManagerID string

	// OutputTags are additional tags to apply to created documents.
	OutputTags []string
}

// DefaultDocumentOptions returns the default options.
func DefaultDocumentOptions() *DocumentOptions {
	return &DocumentOptions{
		CacheHours:   24,
		ForceRefresh: false,
	}
}

// WithCacheHours sets the cache freshness window in hours.
func WithCacheHours(hours int) DocumentOption {
	return func(o *DocumentOptions) {
		o.CacheHours = hours
	}
}

// WithForceRefresh forces document regeneration regardless of cache.
func WithForceRefresh(force bool) DocumentOption {
	return func(o *DocumentOptions) {
		o.ForceRefresh = force
	}
}

// WithManagerID sets the manager ID for document isolation.
func WithManagerID(managerID string) DocumentOption {
	return func(o *DocumentOptions) {
		o.ManagerID = managerID
	}
}

// WithOutputTags sets additional output tags for created documents.
func WithOutputTags(tags []string) DocumentOption {
	return func(o *DocumentOptions) {
		o.OutputTags = tags
	}
}

// ApplyDocumentOptions applies the given options to a DocumentOptions struct.
// If no options are provided, returns default options.
func ApplyDocumentOptions(opts ...DocumentOption) *DocumentOptions {
	options := DefaultDocumentOptions()
	for _, opt := range opts {
		opt(options)
	}
	return options
}
