// Package interfaces provides service interfaces for dependency injection.
package interfaces

import "context"

// DocumentProvisionOptions configures document provisioning behavior.
type DocumentProvisionOptions struct {
	// CacheHours is the freshness window for cached documents (0 = always fetch)
	CacheHours int
	// ForceRefresh bypasses cache and always generates fresh documents
	ForceRefresh bool
}

// DocumentProvisioner is implemented by workers that can provision documents for other workers.
// This interface enables worker-to-worker communication without coupling to concrete types.
//
// Pattern:
//   - If document exists and is fresh (within CacheHours), returns existing document ID
//   - If document is missing or stale, fetches/generates and saves new document
//   - Returns document IDs only, NOT the data - caller retrieves via DocumentStorage
//
// Example usage:
//
//	// Worker A needs data from Worker B
//	docIDs, err := workerB.EnsureDocuments(ctx, []string{"ASX:GNP", "ASX:BHP"}, interfaces.DocumentProvisionOptions{
//	    CacheHours: 24,
//	    ForceRefresh: false,
//	})
//	// Use document IDs to retrieve content
//	for id, docID := range docIDs {
//	    doc, _ := documentStorage.GetDocument(docID)
//	    // Process doc...
//	}
type DocumentProvisioner interface {
	// EnsureDocuments checks cache and creates/updates if needed.
	// Returns a map of identifier -> document ID.
	// The identifier format is worker-specific (e.g., "ASX:GNP" for market data).
	EnsureDocuments(ctx context.Context, identifiers []string, options DocumentProvisionOptions) (map[string]string, error)
}
