I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

Analyzed the codebase to identify unused API endpoints and their supporting handlers. Found 20 unused endpoints across collection, scheduler, MCP, and system categories. The CollectionHandler is completely unused after removing its three routes, while other handlers (SchedulerHandler, MCPHandler) have mixed usage requiring selective removal.

**Key Findings:**
- CollectionHandler: All 3 methods unused → entire handler can be removed
- SchedulerHandler: 1 of 2 methods unused → remove `ForceSyncDocumentHandler` only
- MCPHandler: Routes unused but handler kept for external API integration
- No test files reference any of the handlers being removed
- App initialization in `app.go` needs cleanup for CollectionHandler removal


### Approach

Remove unused endpoints in a single breaking change phase. Delete routes from `routes.go`, remove handler methods from `collection_handler.go` and `scheduler_handler.go`, and clean up app initialization. Keep MCPHandler intact but remove its routes (external API). Document all removals in commit message for future reference.


### Reasoning

Listed workspace directory structure, read the three key files (`routes.go`, `collection_handler.go`, `scheduler_handler.go`), searched for handler references across the codebase using grep, verified no test dependencies exist, read `app.go` initialization code to understand handler lifecycle, and examined `mcp.go` to confirm handler structure for external API preservation.


## Proposed File Changes

### internal\server\routes.go(MODIFY)

Remove 8 unused route registrations:

**Collection routes (lines 51-54):**
- Delete comment line 51: `// API routes - Collection (manual data sync)`
- Delete line 52: `mux.HandleFunc("/api/collection/jira/sync", s.app.CollectionHandler.SyncJiraHandler)`
- Delete line 53: `mux.HandleFunc("/api/collection/confluence/sync", s.app.CollectionHandler.SyncConfluenceHandler)`
- Delete line 54: `mux.HandleFunc("/api/collection/sync-all", s.app.CollectionHandler.SyncAllHandler)`

**Documents force-sync route (line 60):**
- Delete line 60: `mux.HandleFunc("/api/documents/force-sync", s.app.SchedulerHandler.ForceSyncDocumentHandler)`

**MCP routes (lines 67-69):**
- Delete comment line 67: `// MCP (Model Context Protocol) endpoints`
- Delete line 68: `mux.HandleFunc("/mcp", s.app.MCPHandler.HandleRPC)`
- Delete line 69: `mux.HandleFunc("/mcp/info", s.app.MCPHandler.InfoHandler)`
- Add comment: `// NOTE: MCP endpoints removed from public routes - MCPHandler kept for external API integration`

**Scheduler route (line 74):**
- Delete line 74: `mux.HandleFunc("/api/scheduler/trigger-collection", s.app.SchedulerHandler.TriggerCollectionHandler)`
- Add comment: `// NOTE: Scheduler trigger-collection endpoint removed - automatic scheduling via cron (every 5 minutes)`

**System shutdown route (line 100):**
- Update comment on line 100 to: `// Graceful shutdown endpoint (internal-only, dev mode)`
- Keep the route registration unchanged

Ensure proper spacing and alignment after deletions.

### internal\handlers\collection_handler.go(DELETE)

Delete entire file - all three handler methods (`SyncJiraHandler`, `SyncConfluenceHandler`, `SyncAllHandler`) are unused after route removal. The handler only publishes events via EventService, which is now handled directly by the scheduler service. No other code depends on this handler.

### internal\handlers\scheduler_handler.go(MODIFY)

Remove the `ForceSyncDocumentHandler` method (lines 45-68) as it's unused after route removal. Keep the `TriggerCollectionHandler` method (lines 27-43) as it may be used internally or for testing.

**Delete lines 45-68:**
- Remove entire `ForceSyncDocumentHandler` method including comments
- Remove blank line after method

**Keep:**
- `SchedulerHandler` struct (lines 11-14)
- `NewSchedulerHandler` constructor (lines 16-25)
- `TriggerCollectionHandler` method (lines 27-43)

Note: `documentStorage` field in struct is only used by `ForceSyncDocumentHandler`, but keep it for potential future use. If desired, can be removed along with the constructor parameter.

### internal\app\app.go(MODIFY)

References: 

- internal\handlers\collection_handler.go(DELETE)

Remove CollectionHandler from app initialization since the entire handler is being deleted.

**In App struct (around line 121):**
- Delete line 121: `CollectionHandler    *handlers.CollectionHandler`

**In initHandlers() method (around lines 688-691):**
- Delete lines 688-691:
  ```go
  a.CollectionHandler = handlers.NewCollectionHandler(
      a.EventService,
      a.Logger,
  )
  ```
- Remove blank line after deletion

**Update imports:**
- No import changes needed - `handlers` package still used by other handlers

Ensure proper alignment and spacing after deletions. The CollectionHandler field and initialization are the only references to the deleted handler file.