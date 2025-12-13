# Task 1: Fix EventDocumentUpdated to Not Increment Parent Count
Depends: - | Critical: yes:data-integrity | Model: opus

## Problem
The `EventDocumentUpdated` handler in `internal/queue/state/monitor.go` (lines 517-558) increments the parent job's `document_count` when documents are updated. This causes double-counting when:
1. Step 1 (places_search/crawler) creates 20 documents → count = 20
2. Step 2 (agent) updates the same 20 documents → count += 4 (completed) = 24

The document count should reflect UNIQUE documents, not operations.

## Analysis
Looking at the two event handlers:
- `EventDocumentSaved` (line 473-513): For NEW documents - correctly increments count
- `EventDocumentUpdated` (line 517-558): For UPDATED documents - should NOT increment count

The `EventDocumentUpdated` was added for agent jobs that update existing documents (e.g., keyword extraction). But incrementing the parent's document_count for updates is incorrect - the documents already exist.

## Do
1. Modify `EventDocumentUpdated` handler in `internal/queue/state/monitor.go`
2. Remove the call to `IncrementDocumentCount()` for the parent job
3. Keep the logging for debugging purposes
4. Add a comment explaining why updates don't increment count

## Accept
- [ ] `EventDocumentUpdated` handler no longer calls `IncrementDocumentCount()` for parent job
- [ ] Comment added explaining the design decision
- [ ] Build compiles successfully
- [ ] Existing tests pass
