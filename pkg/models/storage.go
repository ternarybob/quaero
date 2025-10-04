package models

import "context"

// Storage defines the interface for document storage
type Storage interface {
	// Store saves a document
	Store(ctx context.Context, doc *Document) error

	// StoreBatch saves multiple documents
	StoreBatch(ctx context.Context, docs []*Document) error

	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*Document, error)

	// Search performs full-text search
	Search(ctx context.Context, query string, limit int) ([]*Document, error)

	// VectorSearch performs vector similarity search
	VectorSearch(ctx context.Context, embedding []float64, limit int) ([]*Document, error)

	// Delete removes a document
	Delete(ctx context.Context, id string) error

	// DeleteBySource removes all documents from a source
	DeleteBySource(ctx context.Context, source string) error
}
