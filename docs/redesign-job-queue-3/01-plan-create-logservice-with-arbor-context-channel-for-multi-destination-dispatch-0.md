I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

The codebase already has a partial implementation of log service functionality embedded in `internal/app/app.go` (lines 239-292). This implementation:

- Creates a `logBatchChannel` to receive log event batches from Arbor
- Sets the channel on the logger with `SetContextChannel()`
- Runs a consumer goroutine that extracts jobID from `CorrelationID` and writes directly to database
- Lacks proper lifecycle management, WebSocket broadcasting, and service encapsulation

The `LogService` interface is already defined in `internal/interfaces/queue_service.go` with all required methods. The `JobLogStorage` implementation exists in `internal/storage/sqlite/job_log_storage.go` and provides database persistence.

## Key Requirements

1. **Arbor Integration**: Consume `chan []arbormodels.LogEvent` batches from Arbor's context channel mechanism
2. **Multi-Destination Dispatch**: Send logs to both database (via JobLogStorage) and WebSocket (via WSHandler) using separate goroutines
3. **Lifecycle Management**: Implement `Start()` to launch consumer goroutine and `Stop()` for graceful shutdown with context cancellation
4. **Thread Safety**: Ensure concurrent access to shared state is properly synchronized
5. **JobID Extraction**: Extract jobID from `event.CorrelationID` to associate logs with jobs
6. **Log Formatting**: Format timestamp as "15:04:05", convert level to lowercase, append fields to message

## Architecture Decision

The LogService will act as a **dispatcher** that:
- Receives log event batches from Arbor's context channel
- Filters events to only process those with a jobID (non-empty CorrelationID)
- Transforms Arbor log events into JobLogEntry format
- Dispatches to multiple destinations concurrently (database + WebSocket)
- Manages goroutine lifecycle with context cancellation

This follows the **fan-out pattern** where a single source (Arbor channel) fans out to multiple destinations (database, WebSocket) via separate goroutines for each destination type.

### Approach

Create `internal/logs/service.go` implementing the `LogService` interface with Arbor context channel integration. The service will consume log event batches from a channel, transform them into JobLogEntry format, and dispatch to database and WebSocket destinations using separate goroutines. Lifecycle management will use context cancellation for graceful shutdown. The existing inline implementation in `app.go` will be replaced with calls to this new service.

### Reasoning

I explored the codebase structure, examined the LogService interface definition in `internal/interfaces/queue_service.go`, reviewed the existing JobLogStorage implementation in `internal/storage/sqlite/job_log_storage.go`, analyzed the current inline implementation in `internal/app/app.go`, studied the service pattern used in `internal/services/events/event_service.go`, and reviewed the WebSocket handler's broadcasting capabilities in `internal/handlers/websocket.go`.

## Mermaid Diagram

sequenceDiagram
    participant App as app.go
    participant Logger as Arbor Logger
    participant LogService as LogService
    participant Channel as logBatchChannel
    participant Consumer as Consumer Goroutine
    participant DB as JobLogStorage
    participant WS as WebSocketHandler

    Note over App: Initialization Phase
    App->>LogService: NewService(storage, wsHandler, logger)
    App->>LogService: Start()
    LogService->>Channel: Create buffered channel (cap=10)
    LogService->>Consumer: Launch consumer goroutine
    Consumer->>Channel: Range over channel (blocking)
    LogService-->>App: Return channel via GetChannel()
    App->>Logger: SetContextChannel(channel)
    
    Note over Logger,Consumer: Runtime Phase
    Logger->>Channel: Send log event batch (async)
    Channel->>Consumer: Receive batch
    loop For each event in batch
        Consumer->>Consumer: Extract jobID from CorrelationID
        alt jobID is empty
            Consumer->>Consumer: Skip event
        else jobID exists
            Consumer->>Consumer: Transform to JobLogEntry
            par Dispatch to Database
                Consumer->>DB: AppendLog(jobID, entry)
            and Dispatch to WebSocket
                Consumer->>WS: BroadcastLog(entry)
            end
        end
    end
    
    Note over App,Consumer: Shutdown Phase
    App->>LogService: Stop()
    LogService->>Consumer: Cancel context
    LogService->>Channel: Close channel
    Consumer->>Consumer: Exit loop (channel closed)
    LogService->>LogService: wg.Wait() for goroutine
    LogService-->>App: Shutdown complete

## Proposed File Changes

### internal\logs\service.go(MODIFY)

References: 

- internal\interfaces\queue_service.go(MODIFY)
- internal\storage\sqlite\job_log_storage.go
- internal\handlers\websocket.go
- internal\models\job_log.go

Create the LogService implementation with the following structure:

**Package and Imports:**
- Package `logs`
- Import: `context`, `fmt`, `sync`, `time`
- Import: `github.com/ternarybob/arbor` and `arbormodels "github.com/ternarybob/arbor/models"`
- Import: `internal/interfaces`, `internal/models`

**Service Struct:**
- Define `Service` struct with fields:
  - `storage interfaces.JobLogStorage` - database persistence
  - `wsHandler interfaces.WebSocketHandler` - WebSocket broadcasting (optional, can be nil)
  - `logger arbor.ILogger` - structured logging
  - `logBatchChannel chan []arbormodels.LogEvent` - receives batches from Arbor
  - `ctx context.Context` - for cancellation
  - `cancel context.CancelFunc` - cancellation function
  - `wg sync.WaitGroup` - tracks goroutines for graceful shutdown
  - `mu sync.RWMutex` - protects shared state

**Constructor:**
- Implement `NewService(storage interfaces.JobLogStorage, wsHandler interfaces.WebSocketHandler, logger arbor.ILogger) interfaces.LogService`
- Return `&Service` with initialized fields
- Note: wsHandler can be nil if WebSocket broadcasting is not needed

**Start Method:**
- Create cancellable context with `context.WithCancel(context.Background())`
- Create buffered channel `logBatchChannel` with capacity 10
- Launch consumer goroutine that:
  - Ranges over `logBatchChannel` to receive batches
  - For each batch, iterate through events
  - Skip events without CorrelationID (no jobID)
  - Extract jobID from `event.CorrelationID`
  - Transform event to JobLogEntry: format timestamp as "15:04:05", convert level to lowercase with `event.Level.String()`, append fields to message
  - Dispatch to database using `storage.AppendLog()` with background context
  - If wsHandler is not nil, dispatch to WebSocket using `wsHandler.BroadcastLog()` with LogEntry format
  - Handle context cancellation to exit gracefully
- Increment `wg` for the consumer goroutine
- Log "Log service started" with info level
- Return nil on success

**Stop Method:**
- Call `cancel()` to signal shutdown
- Close `logBatchChannel` to stop receiving new batches
- Call `wg.Wait()` to wait for consumer goroutine to finish
- Log "Log service stopped" with info level
- Return nil

**GetChannel Method:**
- Implement `GetChannel() chan []arbormodels.LogEvent`
- Return `logBatchChannel` so it can be set on Arbor logger
- This allows Arbor to send log batches to the service

**AppendLog Method:**
- Implement `AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error`
- Delegate directly to `storage.AppendLog(ctx, jobID, entry)`
- This provides synchronous log appending for non-Arbor sources

**AppendLogs Method:**
- Implement `AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error`
- Delegate directly to `storage.AppendLogs(ctx, jobID, entries)`
- This provides batch log appending for non-Arbor sources

**GetLogs Method:**
- Implement `GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error)`
- Delegate to `storage.GetLogs(ctx, jobID, limit)`

**GetLogsByLevel Method:**
- Implement `GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error)`
- Delegate to `storage.GetLogsByLevel(ctx, jobID, level, limit)`

**DeleteLogs Method:**
- Implement `DeleteLogs(ctx context.Context, jobID string) error`
- Delegate to `storage.DeleteLogs(ctx, jobID)`

**CountLogs Method:**
- Implement `CountLogs(ctx context.Context, jobID string) (int, error)`
- Delegate to `storage.CountLogs(ctx, jobID)`

**Error Handling:**
- Log errors from database operations with warn level (non-blocking)
- Log errors from WebSocket operations with debug level (non-critical)
- Never panic - log errors and continue processing

**Thread Safety:**
- Use `mu.RLock()/mu.RUnlock()` when reading shared state
- Use `mu.Lock()/mu.Unlock()` when modifying shared state
- Channel operations are inherently thread-safe

**Performance Considerations:**
- Use buffered channel (capacity 10) to prevent blocking Arbor
- Process batches asynchronously in goroutine
- Database writes use background context to avoid blocking
- WebSocket broadcasts are fire-and-forget (non-blocking)

### internal\interfaces\queue_service.go(MODIFY)

References: 

- internal\app\app.go(MODIFY)
- internal\handlers\handler_interfaces.go
- internal\handlers\websocket.go

Add a new method to the `LogService` interface:

**GetChannel Method:**
- Add `GetChannel() chan []arbormodels.LogEvent` to the interface
- This method returns the channel that Arbor should send log batches to
- Place it after the `Stop()` method and before `AppendLog()`
- Add import for `arbormodels "github.com/ternarybob/arbor/models"` at the top of the file

**Rationale:**
- The channel needs to be accessible to `app.go` so it can be set on the Arbor logger with `logger.SetContextChannel()`
- This decouples the service from knowing about Arbor's logger configuration
- Follows the dependency inversion principle - the service provides the channel, the app wires it up
Add WebSocketHandler interface definition to support LogService WebSocket broadcasting:

**Add WebSocketHandler Interface:**
- Define `WebSocketHandler` interface with method: `BroadcastLog(entry LogEntry)`
- Place it after the `WorkerPool` interface definition
- Add `LogEntry` struct definition with fields: `Timestamp string`, `Level string`, `Message string`
- This allows LogService to broadcast logs without depending on the concrete handlers package

**Alternative Approach:**
- If the WebSocketHandler interface already exists in `internal/handlers/handler_interfaces.go`, import it instead
- Check if `handlers.LogEntry` struct exists and can be reused
- Prefer reusing existing types over creating new ones

**Rationale:**
- LogService needs to call WebSocket broadcasting but should not depend on concrete handler implementations
- Using an interface allows for testing with mocks and maintains clean architecture
- The interface can be nil-checked in LogService to make WebSocket broadcasting optional

### internal\app\app.go(MODIFY)

References: 

- internal\logs\service.go(MODIFY)
- internal\handlers\websocket.go

Refactor the log service initialization to use the new LogService implementation:

**Remove Inline Implementation (lines 239-292):**
- Delete the inline log service initialization code
- Delete the `logBatchChannel` creation
- Delete the consumer goroutine that processes log batches
- Keep the `LogService` field in the `App` struct

**Replace with Service Initialization (around line 240):**
- Create LogService with `logs.NewService(a.StorageManager.JobLogStorage(), a.WSHandler, a.Logger)`
- Note: Pass `a.WSHandler` for WebSocket broadcasting capability
- Call `logService.Start()` to launch the consumer goroutine
- Store in `a.LogService`
- Get the channel with `logBatchChannel := a.LogService.GetChannel()`
- Configure Arbor with `a.Logger.SetContextChannel(logBatchChannel)`
- Log "Log service initialized with Arbor context channel" with info level

**Update Close Method (around line 856):**
- The existing `Close()` method already calls `a.LogService.Stop()` at line 879-885
- Verify the stop sequence is correct: close log batch channel before stopping queue manager
- The channel closing is now handled inside `LogService.Stop()`, so remove the explicit `close(a.logBatchChannel)` call at line 856
- Remove the sleep at line 858 - the service's `wg.Wait()` handles synchronization

**Remove logBatchChannel Field:**
- Remove `logBatchChannel chan []arbormodels.LogEvent` from the `App` struct (line 51)
- The channel is now encapsulated inside the LogService

**Dependency Order:**
- Ensure LogService is initialized BEFORE setting it on the logger
- Ensure WSHandler is initialized BEFORE creating LogService (already correct at line 642)
- The initialization order should be: WSHandler → LogService → Arbor channel configuration

**Error Handling:**
- Check error from `logService.Start()` and return if it fails
- Log the error with context: "failed to start log service"