I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires removing event-based and startup triggers for corpus summary generation. Currently, the `SummaryService` subscribes to `EventEmbeddingTriggered` and generates a summary document at application startup. These automatic triggers need to be removed so that summary generation will only occur via scheduled jobs (to be implemented in subsequent phases).

**Current Implementation:**
- `summary_service.go` line 44: Subscribes to `EventEmbeddingTriggered` in the `NewService` constructor
- `summary_service.go` lines 50-62: Handler method `handleEmbeddingEvent` that responds to the event
- `app.go` lines 598-601: Startup call to `GenerateSummaryDocument` immediately after service initialization

**Key Constraint:** The `GenerateSummaryDocument` method must remain intact as it will be invoked by job actions in subsequent phases.

### Approach

Remove the event subscription and handler from `SummaryService`, and remove the startup call from `App.initServices`. This decouples summary generation from automatic triggers, preparing it for job-based execution.

### Reasoning

I examined the repository structure, read the relevant files (`internal/services/summary/summary_service.go` and `internal/app/app.go`), and identified the exact locations where event subscription and startup generation occur.

## Mermaid Diagram

sequenceDiagram
    participant App as App.initServices
    participant SS as SummaryService
    participant ES as EventService
    
    Note over App,ES: BEFORE (Current State)
    App->>SS: NewService(deps...)
    SS->>ES: Subscribe(EventEmbeddingTriggered)
    ES-->>SS: Subscription registered
    App->>SS: GenerateSummaryDocument()
    SS-->>App: Summary generated at startup
    
    Note over App,ES: AFTER (Desired State)
    App->>SS: NewService(deps...)
    Note over SS: No event subscription
    Note over App: No startup generation
    Note over SS: GenerateSummaryDocument() kept for job-based calls

## Proposed File Changes

### internal\services\summary\summary_service.go(MODIFY)

References: 

- internal\interfaces\event_service.go

**Remove Event Subscription (Line 44):**

Remove the line that subscribes to `EventEmbeddingTriggered` in the `NewService` constructor. This line calls `s.eventService.Subscribe(interfaces.EventEmbeddingTriggered, s.handleEmbeddingEvent)`.

**Remove Event Handler Method (Lines 49-62):**

Delete the entire `handleEmbeddingEvent` method as it will no longer be needed. This method currently responds to embedding events by calling `GenerateSummaryDocument`.

**Update Constructor Documentation (Lines 28-29):**

Update the comment above `NewService` to remove the mention of "automatically subscribes to embedding events to update summaries" since this behavior is being removed.

**Remove EventService Dependency (Optional Cleanup):**

Since the service no longer subscribes to events, consider removing the `eventService` field from the `Service` struct (line 24) and from the constructor parameters (line 33). However, verify that no other methods use this field before removal.

**Keep GenerateSummaryDocument Intact:**

Ensure the `GenerateSummaryDocument` method (lines 64-150) remains unchanged as it will be called by job actions in subsequent phases.

### internal\app\app.go(MODIFY)

References: 

- internal\services\summary\summary_service.go(MODIFY)

**Remove Startup Summary Generation (Lines 597-601):**

Delete the block of code that generates the initial corpus summary document at startup. This includes:
- The log message: `a.Logger.Info().Msg("Generating initial corpus summary document at startup")`
- The call to `a.SummaryService.GenerateSummaryDocument(context.Background())`
- The error handling: `if err != nil { a.Logger.Warn().Err(err).Msg("Failed to generate initial summary document (non-critical)") }`

**Update Initialization Comment (Line 590):**

Update the comment from "Initialize summary service (subscribes to embedding events)" to "Initialize summary service" since it no longer subscribes to events.

**Verify Service Initialization Order:**

Ensure the `SummaryService` initialization (lines 591-596) remains in place, as the service itself is still needed for job-based execution in subsequent phases. Only the startup call to `GenerateSummaryDocument` should be removed.