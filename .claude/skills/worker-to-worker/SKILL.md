# Worker-to-Worker Communication Pattern

## Overview

The Worker-to-Worker pattern enables one worker (Worker A) to request data from another worker (Worker B) without directly fetching or managing that data. This pattern maintains separation of concerns - each worker owns its domain of data.

## Pattern Flow

```
1. Worker A calls Worker B's GetDocument(ticker) or GetDocuments(tickers)
2. Worker B checks database:
   - If document NOT present -> create it
   - If document present -> check freshness
   - If NOT fresh -> recreate it
3. Worker B responds with document_id(s) or tags
4. Worker A gets document(s) from database using returned IDs
```

## Interface Definition

Workers that provide data to other workers should implement `DocumentProvider`:

```go
// Location: internal/interfaces/document_provider.go

// DocumentProvider is implemented by workers that can provision documents for other workers.
// This enables worker-to-worker communication without coupling to concrete types.
type DocumentProvider interface {
    // GetDocument ensures a document exists for a single identifier.
    // Returns the document result with ID, freshness info, and whether it was created.
    GetDocument(ctx context.Context, identifier string, opts ...DocumentOption) (*DocumentResult, error)

    // GetDocuments ensures documents exist for multiple identifiers.
    // Returns results for each identifier (may include errors for individual items).
    GetDocuments(ctx context.Context, identifiers []string, opts ...DocumentOption) ([]*DocumentResult, error)
}

// DocumentResult contains the result of document provisioning.
type DocumentResult struct {
    Identifier string   // The original identifier (e.g., "ASX:GNP")
    DocumentID string   // The database document ID
    Tags       []string // Tags applied to the document
    Fresh      bool     // True if document was already fresh (cache hit)
    Created    bool     // True if document was newly created
    Error      error    // Non-nil if provisioning failed for this identifier
}

// DocumentOption configures document provisioning behavior.
type DocumentOption func(*DocumentOptions)

// DocumentOptions holds all configurable options.
type DocumentOptions struct {
    CacheHours   int  // Freshness window for cached documents (0 = always fetch)
    ForceRefresh bool // Bypass cache and always generate fresh documents
}

// WithCacheHours sets the cache freshness window.
func WithCacheHours(hours int) DocumentOption {
    return func(o *DocumentOptions) {
        o.CacheHours = hours
    }
}

// WithForceRefresh forces document regeneration.
func WithForceRefresh(force bool) DocumentOption {
    return func(o *DocumentOptions) {
        o.ForceRefresh = force
    }
}
```

## Backward Compatibility

The existing `DocumentProvisioner` interface is maintained for backward compatibility:

```go
// DocumentProvisioner is the legacy interface (still supported).
// New code should prefer DocumentProvider.
type DocumentProvisioner interface {
    EnsureDocuments(ctx context.Context, identifiers []string, options DocumentProvisionOptions) (map[string]string, error)
}
```

Workers can implement both interfaces to support existing and new callers.

## Implementation Example

### Provider Worker (Worker B - provides data)

```go
// AnnouncementsWorker implements DocumentProvider
type AnnouncementsWorker struct {
    documentStorage interfaces.DocumentStorage
    // ... other fields
}

// Compile-time assertion
var _ interfaces.DocumentProvider = (*AnnouncementsWorker)(nil)

// GetDocument ensures announcement data exists for a ticker
func (w *AnnouncementsWorker) GetDocument(ctx context.Context, identifier string, opts ...interfaces.DocumentOption) (*interfaces.DocumentResult, error) {
    options := interfaces.ApplyDocumentOptions(opts...)

    ticker := common.ParseTicker(identifier)
    sourceType := "announcement"
    sourceID := fmt.Sprintf("%s:%s:announcement", ticker.Exchange, ticker.Code)

    result := &interfaces.DocumentResult{
        Identifier: identifier,
    }

    // Check for cached document
    if !options.ForceRefresh && options.CacheHours > 0 {
        existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
        if err == nil && existingDoc != nil && existingDoc.LastSynced != nil {
            if time.Since(*existingDoc.LastSynced) < time.Duration(options.CacheHours)*time.Hour {
                result.DocumentID = existingDoc.ID
                result.Tags = existingDoc.Tags
                result.Fresh = true
                result.Created = false
                return result, nil
            }
        }
    }

    // Cache miss or stale - fetch and create document
    doc, err := w.fetchAndCreateDocument(ctx, ticker)
    if err != nil {
        result.Error = err
        return result, err
    }

    result.DocumentID = doc.ID
    result.Tags = doc.Tags
    result.Fresh = false
    result.Created = true
    return result, nil
}

// GetDocuments handles multiple identifiers
func (w *AnnouncementsWorker) GetDocuments(ctx context.Context, identifiers []string, opts ...interfaces.DocumentOption) ([]*interfaces.DocumentResult, error) {
    results := make([]*interfaces.DocumentResult, len(identifiers))
    var lastErr error

    for i, id := range identifiers {
        result, err := w.GetDocument(ctx, id, opts...)
        results[i] = result
        if err != nil {
            lastErr = err
            // Continue processing other identifiers
        }
    }

    return results, lastErr
}
```

### Consumer Worker (Worker A - uses data)

```go
// AnnouncementDownloadWorker uses announcements from AnnouncementsWorker
type AnnouncementDownloadWorker struct {
    documentStorage      interfaces.DocumentStorage
    announcementProvider interfaces.DocumentProvider // Worker-to-worker
    // ... other fields
}

func (w *AnnouncementDownloadWorker) processOneTicker(ctx context.Context, ticker string) error {
    // Step 1: Request document from provider worker
    result, err := w.announcementProvider.GetDocument(ctx, ticker,
        interfaces.WithCacheHours(24),
        interfaces.WithForceRefresh(false),
    )
    if err != nil {
        return fmt.Errorf("failed to get announcements: %w", err)
    }

    // Step 2: Retrieve document content using returned ID
    doc, err := w.documentStorage.GetDocument(result.DocumentID)
    if err != nil {
        return fmt.Errorf("failed to retrieve document: %w", err)
    }

    // Step 3: Process document content
    announcements := w.extractAnnouncements(doc)
    // ... continue processing

    return nil
}
```

## When to Use This Pattern

### USE when:
- Worker A needs data that Worker B specializes in fetching/processing
- The data has caching requirements (freshness checks)
- You want to avoid duplicate data fetching logic
- Multiple workers need the same underlying data
- Data ownership should remain with one worker

### DO NOT USE when:
- Simple one-time data lookups (just query the database directly)
- No caching or freshness requirements
- Workers are in different services/processes
- The "provider" worker doesn't already exist

## Integration with Dependency Injection

Register providers in `app/app.go`:

```go
func (a *App) initWorkers() {
    // Create the provider worker
    announcementsWorker := market.NewAnnouncementsWorker(
        a.storageManager.DocumentStorage(),
        a.kvStorage,
        a.logger,
        a.jobMgr,
        a.dataWorker, // DataWorker as DocumentProvider
        a.debugEnabled,
    )

    // Create consumer worker with provider injected
    announcementDownloadWorker := market.NewAnnouncementDownloadWorker(
        a.storageManager.DocumentStorage(),
        a.searchService,
        a.kvStorage,
        a.logger,
        a.jobMgr,
        announcementsWorker, // Pass as DocumentProvider
        a.debugEnabled,
    )
}
```

## Best Practices

1. **Always return document IDs, not content** - Let callers retrieve what they need
2. **Handle partial failures gracefully** - Continue processing other identifiers if one fails
3. **Log cache hits/misses** - Helps debug performance issues
4. **Use options pattern** - Flexible configuration without breaking signatures
5. **Implement both interfaces** - Support legacy `DocumentProvisioner` and new `DocumentProvider`

## Related Files

- `internal/interfaces/document_provider.go` - Interface definitions
- `internal/interfaces/document_provisioner.go` - Legacy interface (backward compatibility)
- `internal/workers/market/data_worker.go` - Example implementation
- `internal/workers/market/announcements_worker.go` - Example implementation
- `internal/workers/market/announcement_download_worker.go` - Example consumer
