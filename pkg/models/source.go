package models

import "context"

// Source defines the interface for data collection sources
type Source interface {
	// Name returns the name of the source (e.g., "confluence", "jira")
	Name() string
	
	// Collect retrieves documents from the source
	Collect(ctx context.Context) ([]*Document, error)
	
	// SupportsImages indicates if the source can have images
	SupportsImages() bool
}
