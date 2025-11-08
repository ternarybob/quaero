---
task: "Add document count tracking to parent jobs with real-time WebSocket updates"
folder: parent-job-document-count
complexity: medium
estimated_steps: 6
---

# Implementation Plan: Parent Job Document Count Tracking

## Problem Statement

From test screenshot `test\results\ui\news-20251108-202129\TestNewsCrawlerJobExecution\08-test-completed.png`, the parent job "News Crawler" shows "0 Documents" even though service logs confirm "Successfully saved created document". The document count is not being tracked or displayed in real-time.

## Architecture Analysis

**Current Flow:**
1. Child jobs (crawler_url) execute via `CrawlerStepExecutor`
2. Documents are saved via `DocumentPersister.SaveCrawledDocument()`
3. Parent job monitors child completion via `ParentJobExecutor.checkChildJobProgress()`
4. WebSocket broadcasts job updates via EventService pub/sub

**Key Components:**
- `EventService` (internal/services/events/) - Pub/sub event bus
- `ParentJobExecutor` (internal/jobs/processor/parent_job_executor.go) - Parent job monitoring
- `DocumentPersister` (internal/services/crawler/document_persister.go) - Document saving
- `WebSocketHandler` (internal/handlers/websocket.go) - Real-time UI updates
- `Manager` (internal/jobs/manager.go) - Job metadata storage

## Implementation Steps

### Step 1: Add document_saved event type to EventService

**Why:** Need a new event type that child jobs emit when saving documents to trigger parent job updates

**Depends on:** none

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\interfaces\event_service.go` (add EventDocumentSaved constant)

**Risk:** low

**Implementation Details:**
- Add new event constant: `EventDocumentSaved EventType = "document_saved"`
- Document payload structure in comments:
  ```
  Payload: map[string]interface{}{
    "job_id": string (child job ID that saved the document),
    "parent_job_id": string (parent job ID to update),
    "document_id": string (saved document ID),
    "source_url": string (document URL),
    "timestamp": time.RFC3339
  }
  ```

---

### Step 2: Publish document_saved event when documents are saved

**Why:** Child jobs need to notify parent jobs when documents are successfully persisted

**Depends on:** Step 1

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\services\crawler\document_persister.go` (emit event after save)

**Risk:** medium (adds event publishing to critical save path)

**Implementation Details:**
- Add `eventService interfaces.EventService` field to `DocumentPersister` struct
- Update `NewDocumentPersister()` constructor to accept EventService parameter
- In `SaveCrawledDocument()` method, after successful save/update (line 75):
  ```go
  // Publish document_saved event for parent job tracking
  if dp.eventService != nil && crawledDoc.ParentJobID != "" {
      payload := map[string]interface{}{
          "job_id": crawledDoc.JobID,
          "parent_job_id": crawledDoc.ParentJobID,
          "document_id": doc.ID,
          "source_url": crawledDoc.SourceURL,
          "timestamp": time.Now().Format(time.RFC3339),
      }
      event := interfaces.Event{
          Type: interfaces.EventDocumentSaved,
          Payload: payload,
      }
      // Publish asynchronously to not block document save
      go dp.eventService.Publish(context.Background(), event)
  }
  ```
- Update all places where `NewDocumentPersister()` is called to pass EventService

**Testing:**
- Verify event is published only on successful save/update
- Verify event is NOT published if document save fails
- Verify async publish doesn't block save operation

---

### Step 3: Add document count tracking to job metadata

**Why:** Parent jobs need persistent storage for document count that survives process restarts

**Depends on:** none

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\jobs\manager.go` (add IncrementDocumentCount method)

**Risk:** low (adds new method, doesn't modify existing)

**Implementation Details:**
- Add new method to Manager:
  ```go
  // IncrementDocumentCount increments the document_count in job metadata
  func (m *Manager) IncrementDocumentCount(ctx context.Context, jobID string) error {
      // Read current metadata
      var metadataStr string
      err := m.db.QueryRowContext(ctx, `
          SELECT metadata_json FROM jobs WHERE id = ?
      `, jobID).Scan(&metadataStr)
      if err != nil {
          return fmt.Errorf("failed to get job metadata: %w", err)
      }

      // Parse metadata
      var metadata map[string]interface{}
      if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
          return fmt.Errorf("failed to parse metadata: %w", err)
      }

      // Increment document_count (default 0 if not exists)
      currentCount := 0
      if count, ok := metadata["document_count"].(float64); ok {
          currentCount = int(count)
      } else if count, ok := metadata["document_count"].(int); ok {
          currentCount = count
      }
      metadata["document_count"] = currentCount + 1

      // Save updated metadata
      updatedMetadata, err := json.Marshal(metadata)
      if err != nil {
          return fmt.Errorf("failed to marshal metadata: %w", err)
      }

      // Use retry logic for write contention
      err = retryOnBusy(ctx, func() error {
          _, err := m.db.ExecContext(ctx, `
              UPDATE jobs SET metadata_json = ? WHERE id = ?
          `, string(updatedMetadata), jobID)
          return err
      })

      return err
  }
  ```

**Testing:**
- Verify count starts at 0 and increments correctly
- Verify concurrent increments don't lose counts (retry logic)
- Verify persists across service restarts

---

### Step 4: Subscribe parent job executor to document_saved events

**Why:** Parent jobs must listen for document_saved events and update their metadata

**Depends on:** Step 1, Step 3

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\jobs\processor\parent_job_executor.go` (add subscription)

**Risk:** low (adds new subscription alongside existing ones)

**Implementation Details:**
- Add `jobMgr *jobs.Manager` reference to ParentJobExecutor (already exists)
- In `SubscribeToChildStatusChanges()` method, add new subscription:
  ```go
  // Subscribe to document_saved events for real-time document count tracking
  e.eventService.Subscribe(interfaces.EventDocumentSaved, func(ctx context.Context, event interfaces.Event) error {
      payload, ok := event.Payload.(map[string]interface{})
      if !ok {
          e.logger.Warn().Msg("Invalid document_saved payload type")
          return nil
      }

      parentJobID := getStringFromPayload(payload, "parent_job_id")
      if parentJobID == "" {
          return nil // No parent job, ignore
      }

      // Increment document count in parent job metadata
      if err := e.jobMgr.IncrementDocumentCount(ctx, parentJobID); err != nil {
          e.logger.Error().Err(err).
              Str("parent_job_id", parentJobID).
              Msg("Failed to increment document count for parent job")
          return nil // Don't fail the event handler
      }

      e.logger.Debug().
          Str("parent_job_id", parentJobID).
          Str("document_id", getStringFromPayload(payload, "document_id")).
          Msg("Incremented document count for parent job")

      return nil
  })
  ```

**Testing:**
- Verify subscription happens on executor initialization
- Verify document count increments when events received
- Verify errors don't crash event handler

---

### Step 5: Include document count in parent_job_progress WebSocket events

**Why:** UI needs document count in real-time updates to display current progress

**Depends on:** Step 3, Step 4

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\jobs\processor\parent_job_executor.go` (modify publishParentJobProgressUpdate)

**Risk:** low (modifies existing payload, backward compatible)

**Implementation Details:**
- In `publishParentJobProgressUpdate()` method (line 363), retrieve document count from metadata:
  ```go
  // Get document count from job metadata
  documentCount := 0
  jobInternal, err := e.jobMgr.GetJobInternal(ctx, parentJobID)
  if err == nil && jobInternal != nil {
      // Parse metadata to get document_count
      var metadata map[string]interface{}
      if jobInternal.Payload != "" {
          if err := json.Unmarshal([]byte(jobInternal.Payload), &metadata); err == nil {
              if count, ok := metadata["document_count"].(float64); ok {
                  documentCount = int(count)
              } else if count, ok := metadata["document_count"].(int); ok {
                  documentCount = count
              }
          }
      }
  }

  payload := map[string]interface{}{
      // ... existing fields ...
      "document_count": documentCount, // ADD THIS LINE
      "timestamp": time.Now().Format(time.RFC3339),
  }
  ```
- Note: Metadata is stored in `metadata_json` column, but JobModel stores it in `Config` field when retrieved via GetJobInternal

**Alternative Implementation (cleaner):**
- Modify `GetJobInternal()` to properly parse metadata_json into a separate field
- Access `jobInternal.Metadata["document_count"]` directly

**Testing:**
- Verify document_count appears in WebSocket payload
- Verify count updates in real-time as documents are saved
- Verify backward compatibility (existing clients ignore new field)

---

### Step 6: Update WebSocket handler to broadcast document count

**Why:** Ensure document_count is properly formatted and sent to UI clients

**Depends on:** Step 5

**Validation:** code_compiles, follows_conventions, ui_displays_count

**Creates/Modifies:**
- `C:\development\quaero\internal\handlers\websocket.go` (verify parent_job_progress handler)

**Risk:** low (no changes needed, verification only)

**Implementation Details:**
- Verify existing `parent_job_progress` event handler (line 998-1065) correctly passes through payload
- The handler already extracts all fields from payload via `getString()` and `getInt()`
- Add document_count extraction to payload:
  ```go
  wsPayload := map[string]interface{}{
      "job_id":             jobID,
      "progress_text":      progressText,
      "status":             status,
      "timestamp":          getString(payload, "timestamp"),
      "document_count":     getInt(payload, "document_count"), // ADD THIS LINE
      // ... other fields ...
  }
  ```

**Testing:**
- Use browser DevTools to verify WebSocket message contains `document_count`
- Verify UI receives updates when documents are saved
- Test with UI test: verify document count increments during crawler execution

---

## Constraints

- Must use existing EventService for pub/sub (no new infrastructure)
- Must follow dependency injection pattern (pass EventService to DocumentPersister)
- No modifications to database schema (use existing metadata_json column)
- WebSocket updates must be real-time (async event publishing)
- Must work with existing parent/child job architecture
- Must handle concurrent document saves without losing counts (retry logic)

## Success Criteria

1. **Event Publishing:**
   - Child jobs emit `document_saved` event when document is saved
   - Event includes job_id, parent_job_id, document_id, source_url

2. **Parent Job Tracking:**
   - Parent job receives events and updates document count in metadata
   - Document count persisted in job metadata_json column
   - Concurrent saves handled correctly (no lost counts)

3. **WebSocket Updates:**
   - Document count included in `parent_job_progress` WebSocket events
   - UI receives real-time updates showing document count
   - Backward compatible with existing WebSocket clients

4. **Testing:**
   - Test verifies document count updates in real-time during crawler execution
   - Screenshot shows non-zero document count for completed jobs
   - Service logs confirm event publishing and count increments

## Testing Strategy

**Unit Tests:**
- Test `Manager.IncrementDocumentCount()` with concurrent calls
- Test event payload serialization/deserialization
- Test error handling when EventService is nil

**Integration Tests:**
- Test end-to-end flow: save document → emit event → increment count
- Test parent job monitoring with multiple child jobs saving documents
- Test WebSocket message format includes document_count

**UI Tests:**
- Modify `TestNewsCrawlerJobExecution` to verify document count > 0
- Add WebSocket listener to capture parent_job_progress events
- Verify screenshot shows document count in UI

## Rollback Plan

If issues occur:
1. Remove EventService from DocumentPersister constructor (backward compatible)
2. Remove document_saved event subscription from ParentJobExecutor
3. Remove document_count from WebSocket payload (ignored by UI)
4. All changes are additive and can be removed without breaking existing functionality

## Performance Considerations

- **Async Event Publishing:** Events published in goroutines to avoid blocking document saves
- **Database Retry Logic:** Uses existing `retryOnBusy()` for write contention
- **WebSocket Throttling:** parent_job_progress events not currently throttled (may need throttling if too frequent)
- **Memory:** Minimal overhead (one int per parent job in metadata)

## Dependencies

**Modified Components:**
- EventService interface (add event type constant)
- DocumentPersister (add EventService dependency)
- ParentJobExecutor (add subscription)
- Manager (add IncrementDocumentCount method)
- WebSocketHandler (add document_count to payload)

**Initialization Order:**
1. EventService (already initialized early)
2. DocumentStorage (already initialized)
3. DocumentPersister (needs EventService passed in)
4. JobManager (already initialized)
5. ParentJobExecutor (already subscribes to events)

**No changes needed to:**
- Database schema
- Queue infrastructure
- Job execution flow
- UI code (gracefully handles new field)
