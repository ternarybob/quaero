I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase has two separate crawler executor files that need to be merged:

1. **crawler_executor.go (1034 lines)**:
   - CrawlerExecutor struct with 8 dependencies
   - Core methods: Execute(), GetWorkerType(), Validate()
   - Helper methods: extractCrawlConfig(), renderPageWithChromeDp(), spawnChildJob()
   - Event publishing methods: publishCrawlerJobLog(), publishCrawlerProgressUpdate(), publishLinkDiscoveryEvent(), publishJobSpawnEvent()
   - Calls injectAuthCookies() at line 198 during Execute()

2. **crawler_executor_auth.go (495 lines)**:
   - Single method: injectAuthCookies() as receiver method on CrawlerExecutor
   - Already has conditional logic: checks `if e.authStorage == nil` and returns early (lines 31-34)
   - Extensive logging with 🔐 emoji for auth operations
   - Three phases: pre-injection diagnostics, network domain enablement, post-injection verification
   - Uses authStorage and jobDefStorage fields from CrawlerExecutor struct

**Key Architectural Insight:**

The separation into two files was unnecessary - the auth logic is already conditional and cleanly encapsulated in a single method. Merging into one file simplifies the architecture without losing any functionality.

**Dependencies Analysis:**

All 8 dependencies are used across both files:
- **crawlerService**: Used in Execute() for browser pool (currently bypassed)
- **jobMgr**: Used throughout for job status updates and logging
- **queueMgr**: Used in spawnChildJob() for enqueueing child jobs
- **documentStorage**: Used in Execute() for saving crawled documents
- **authStorage**: Used in injectAuthCookies() for loading credentials (conditional)
- **jobDefStorage**: Used in injectAuthCookies() for auth_id fallback lookup
- **logger**: Used throughout for structured logging
- **eventService**: Used in all event publishing methods

**Import Location:**

Only one file imports CrawlerExecutor:
- `internal/app/app.go` (line 297): Creates and registers with JobProcessor
- Log message (line 308): "Crawler URL worker registered for job type: crawler_url"

**Interface Compliance:**

Current interface (already updated in ARCH-002):
- `Execute(ctx, job) error` ✓
- `GetWorkerType() string` ✓ (returns "crawler_url")
- `Validate(job) error` ✓

Target interface (worker.JobWorker) - same methods, just different package.

**Risk Assessment:**

- **Low Risk**: File merge is mechanical (copy-paste with renames)
- **Low Risk**: Struct rename is compile-time checked
- **Low Risk**: Single import location to update
- **Medium Risk**: Large file size (1529 lines) - ensure all references updated
- **Low Risk**: Conditional auth logic already exists - no new logic needed

**Success Criteria:**

1. Single crawler_worker.go file in internal/jobs/worker/
2. CrawlerWorker struct implements worker.JobWorker interface
3. injectAuthCookies() as private method in merged file
4. app.go successfully imports and uses worker.NewCrawlerWorker()
5. Application compiles and runs successfully
6. Crawler jobs execute correctly (with and without auth)
7. Child job spawning works correctly
8. All event publishing works correctly
9. Old files remain with deprecation notices

### Approach

**File Merge and Migration Strategy**

This phase consolidates two crawler executor files into a single worker file while migrating to the new worker directory structure. The approach follows these principles:

1. **Complete File Merge**: Combine crawler_executor.go (1034 lines) and crawler_executor_auth.go (495 lines) into single crawler_worker.go
2. **Preserve All Functionality**: No logic changes - only structural refactoring (rename, reorganize, update imports)
3. **Conditional Auth Logic**: Already exists (authStorage nil check at line 31-34) - no new logic needed
4. **Clean Integration**: Move injectAuthCookies() as private method into merged file
5. **Interface Compliance**: Update to implement JobWorker interface from worker package
6. **Backward Compatibility**: Keep old files with deprecation notices until ARCH-008

**Why This Approach:**

- **Single Source of Truth**: One file for all crawler worker logic eliminates confusion
- **Conditional Auth**: Existing nil check makes auth optional without separate files
- **Maintainability**: Easier to understand and modify when all logic is together
- **Consistency**: Aligns with manager pattern (single file per component)
- **Low Risk**: Mechanical transformation with no functional changes

**Merge Strategy:**

The merged file will have this structure:
1. Package declaration and imports (combined from both files)
2. CrawlerWorker struct definition (renamed from CrawlerExecutor)
3. NewCrawlerWorker constructor (renamed from NewCrawlerExecutor)
4. Interface methods: GetWorkerType(), Validate(), Execute()
5. Private helper methods: extractCrawlConfig(), renderPageWithChromeDp(), spawnChildJob()
6. **Private auth method**: injectAuthCookies() (moved from auth file)
7. Event publishing methods: publishCrawlerJobLog(), publishCrawlerProgressUpdate(), etc.

**Key Transformations:**

- **Package**: `processor` → `worker`
- **Struct**: `CrawlerExecutor` → `CrawlerWorker`
- **Constructor**: `NewCrawlerExecutor()` → `NewCrawlerWorker()`
- **Receiver**: `func (e *CrawlerExecutor)` → `func (w *CrawlerWorker)` (use `w` for worker convention)
- **Interface**: Implement `worker.JobWorker` instead of `interfaces.JobExecutor`
- **Method**: `GetJobType()` → `GetWorkerType()` (already done in ARCH-002, verify in merged file)

**Import Updates Required:**

Only one file needs import updates:
- `internal/app/app.go` (line 297) - Change from `processor.NewCrawlerExecutor()` to `worker.NewCrawlerWorker()`

**File Organization:**

The merged file will be well-organized with clear sections:
1. Core interface methods (Execute, GetWorkerType, Validate)
2. Main workflow helpers (extractCrawlConfig, renderPageWithChromeDp, spawnChildJob)
3. Authentication helper (injectAuthCookies) - clearly marked as auth-related
4. Event publishing helpers (grouped together at end)

**Validation Strategy:**

After merge:
1. Verify file compiles independently
2. Verify implements JobWorker interface correctly
3. Update app.go registration
4. Build application successfully
5. Run crawler tests end-to-end
6. Verify auth injection works (with and without authStorage)
7. Verify child job spawning works correctly

### Reasoning

I systematically explored the codebase to understand the merge requirements:

1. **Read both crawler files** - Analyzed crawler_executor.go (1034 lines) and crawler_executor_auth.go (495 lines) to understand structure and dependencies
2. **Identified auth method call** - Found injectAuthCookies() called at line 198 in Execute() method
3. **Verified conditional logic** - Confirmed authStorage nil check exists at lines 31-34 of auth file
4. **Searched for imports** - Found only app.go imports and uses CrawlerExecutor (line 297)
5. **Read worker interface** - Examined JobWorker interface in worker/interfaces.go to understand requirements
6. **Analyzed registration** - Reviewed app.go registration code to understand initialization pattern
7. **Checked struct dependencies** - Identified 8 dependencies: CrawlerService, JobManager, QueueManager, DocumentStorage, AuthStorage, JobDefinitionStorage, Logger, EventService

This exploration revealed that the merge is straightforward: combine files, rename struct/methods, move injectAuthCookies as private method, update single import location.

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant Old1 as processor/crawler_executor.go
    participant Old2 as processor/crawler_executor_auth.go
    participant New as worker/crawler_worker.go
    participant App as internal/app/app.go
    participant Build as Go Build System
    
    Note over Dev,Build: Phase 1: Create Merged Worker File
    
    Dev->>New: Create crawler_worker.go
    Note right of New: Merge both source files<br/>Package: processor → worker<br/>Struct: CrawlerExecutor → CrawlerWorker<br/>Constructor: NewCrawlerExecutor → NewCrawlerWorker<br/>Receiver: (e *CrawlerExecutor) → (w *CrawlerWorker)
    
    Dev->>New: Copy all methods from crawler_executor.go
    Note right of New: Interface methods: Execute, GetWorkerType, Validate<br/>Helpers: extractCrawlConfig, renderPageWithChromeDp, spawnChildJob<br/>Event publishers: publishCrawlerJobLog, etc.
    
    Dev->>New: Copy injectAuthCookies from crawler_executor_auth.go
    Note right of New: Add as private method in merged file<br/>Keep conditional logic: if w.authStorage == nil<br/>Keep all 🔐 emoji logging<br/>Keep three-phase injection process
    
    Dev->>New: Organize file sections
    Note right of New: 1. Interface methods<br/>2. Config/rendering helpers<br/>3. Authentication helpers<br/>4. Child job management<br/>5. Event publishing
    
    Dev->>Build: Compile new worker file
    Build-->>Dev: ✓ crawler_worker.go compiles successfully
    
    Note over Dev,Build: Phase 2: Update Import Paths
    
    Dev->>App: Add worker import
    Note right of App: import "internal/jobs/worker"<br/>Keep processor import for other workers
    
    Dev->>App: Update CrawlerWorker registration
    Note right of App: processor.NewCrawlerExecutor()<br/>→ worker.NewCrawlerWorker()<br/>Variable: crawlerExecutor → crawlerWorker
    
    Dev->>Build: Build application
    Build-->>Dev: ✓ Application compiles successfully
    
    Note over Dev,Build: Phase 3: Add Deprecation Notices
    
    Dev->>Old1: Add deprecation comment to crawler_executor.go
    Note right of Old1: "DEPRECATED: Merged and migrated to<br/>internal/jobs/worker/crawler_worker.go"
    
    Dev->>Old2: Add deprecation comment to crawler_executor_auth.go
    Note right of Old2: "DEPRECATED: Merged into<br/>internal/jobs/worker/crawler_worker.go"
    
    Note over Old1,Old2: Files remain functional<br/>for backward compatibility<br/>Will be deleted in ARCH-008
    
    Note over Dev,Build: Phase 4: Validation
    
    Dev->>Build: Run test suite
    Build-->>Dev: ✓ All tests pass
    
    Dev->>App: Start application
    App->>New: Register CrawlerWorker with JobProcessor
    App-->>Dev: ✓ "Crawler URL worker registered for job type: crawler_url"
    
    Dev->>App: Trigger crawler job via UI
    App->>New: Execute crawler job
    New->>New: Check if authStorage != nil
    alt Auth Available
        New->>New: injectAuthCookies() executes
        Note right of New: Load credentials<br/>Inject cookies<br/>Verify injection
    else No Auth
        New->>New: Skip auth injection
        Note right of New: Continue without authentication
    end
    New->>New: Render page with ChromeDP
    New->>New: Process content to markdown
    New->>New: Save document
    New->>New: Discover and filter links
    New->>New: Spawn child jobs
    New-->>App: ✓ Job completed successfully
    
    Note over Dev,Build: Migration Complete<br/>2 files merged into 1<br/>Old files deprecated<br/>Backward compatible<br/>Auth logic conditional

## Proposed File Changes

### internal\jobs\worker\crawler_worker.go(NEW)

References: 

- internal\jobs\processor\crawler_executor.go(MODIFY)
- internal\jobs\processor\crawler_executor_auth.go(MODIFY)
- internal\jobs\worker\interfaces.go

Create new CrawlerWorker file by merging crawler_executor.go and crawler_executor_auth.go with the following transformations:

**Package Declaration:**
- Change: `package processor` → `package worker`

**File Header Comment:**
- Update to: "Crawler Worker - Processes individual crawler jobs from the queue with ChromeDP rendering, content processing, and child job spawning"

**Imports (Combined from both files):**
- Keep all imports from crawler_executor.go
- Add auth-specific imports from crawler_executor_auth.go:
  - `encoding/json` (for cookie unmarshaling)
  - `net/url` (for URL parsing)
  - `github.com/chromedp/cdproto/cdp` (for cookie types)
  - `github.com/chromedp/cdproto/network` (for network operations)
- Update interface import: `github.com/ternarybob/quaero/internal/interfaces` (keep for other interfaces used)
- Group imports logically: standard library, external packages, internal packages

**Struct Rename:**
- Change: `type CrawlerExecutor struct` → `type CrawlerWorker struct`
- Keep all 8 fields unchanged:
  - crawlerService *crawler.Service
  - jobMgr *jobs.Manager
  - queueMgr *queue.Manager
  - documentStorage interfaces.DocumentStorage
  - authStorage interfaces.AuthStorage
  - jobDefStorage interfaces.JobDefinitionStorage
  - logger arbor.ILogger
  - eventService interfaces.EventService
  - contentProcessor *crawler.ContentProcessor
- Update struct comment: "CrawlerWorker processes individual crawler jobs from the queue, rendering pages with ChromeDP, extracting content, and spawning child jobs for discovered links"

**Constructor Rename:**
- Change: `func NewCrawlerExecutor(...)` → `func NewCrawlerWorker(...)`
- Change return type: `*CrawlerExecutor` → `*CrawlerWorker`
- Update struct initialization: `return &CrawlerExecutor{...}` → `return &CrawlerWorker{...}`
- Update comment: "NewCrawlerWorker creates a new crawler worker for processing individual crawler jobs from the queue"
- Keep all 9 parameters unchanged (same order, same types)

**Method Receivers (All Methods):**
- Change all method receivers: `func (e *CrawlerExecutor)` → `func (w *CrawlerWorker)`
- Rename receiver variable from `e` to `w` for consistency (worker convention)
- Update all references to `e.` → `w.` within all method bodies
- This applies to:
  - GetWorkerType() - interface method
  - Validate() - interface method
  - Execute() - interface method
  - extractCrawlConfig() - private helper
  - renderPageWithChromeDp() - private helper
  - spawnChildJob() - private helper
  - injectAuthCookies() - private auth helper (moved from auth file)
  - publishCrawlerJobLog() - private event helper
  - publishCrawlerProgressUpdate() - private event helper
  - publishLinkDiscoveryEvent() - private event helper
  - publishJobSpawnEvent() - private event helper

**Interface Methods (Keep at top after constructor):**
1. GetWorkerType() - Already returns "crawler_url" (correct)
2. Validate() - Already validates job type (correct)
3. Execute() - Main workflow method (keep all logic unchanged)

**Private Helper Methods (Organize logically):**

**Section 1: Configuration and Rendering**
- extractCrawlConfig() - Parse crawl configuration from job config
- renderPageWithChromeDp() - ChromeDP page rendering with network/log domain enabling

**Section 2: Authentication (Moved from crawler_executor_auth.go)**
- injectAuthCookies() - Load auth credentials and inject cookies into browser
  - Add section comment: "// ============================================================================\n// AUTHENTICATION HELPERS\n// ============================================================================"
  - Keep all logic unchanged (already has conditional authStorage nil check)
  - Keep all three phases: pre-injection diagnostics, network enablement, post-injection verification
  - Keep all 🔐 emoji logging for auth operations
  - Method signature: `func (w *CrawlerWorker) injectAuthCookies(ctx context.Context, browserCtx context.Context, parentJobID, targetURL string, logger arbor.ILogger) error`

**Section 3: Child Job Management**
- spawnChildJob() - Create and enqueue child jobs for discovered links

**Section 4: Event Publishing (Group at end)**
- Add section comment: "// ============================================================================\n// REAL-TIME EVENT PUBLISHING\n// ============================================================================"
- publishCrawlerJobLog() - Publish job log events
- publishCrawlerProgressUpdate() - Publish progress updates
- publishLinkDiscoveryEvent() - Publish link discovery statistics
- publishJobSpawnEvent() - Publish child job spawn events

**Comments:**
- Update all comments referencing "executor" → "worker"
- Update all comments referencing "CrawlerExecutor" → "CrawlerWorker"
- Keep all existing detailed comments (especially in Execute() method)
- Keep all 🔐 emoji comments in injectAuthCookies() method
- Add clear section dividers for organization

**Log Messages:**
- Update log messages: "executor" → "worker" where referring to this component
- Keep all other log messages unchanged (e.g., "Starting crawl of URL", "Document saved")
- Keep all 🔐 emoji log messages in auth method unchanged

**Validation:**
- Verify file compiles independently: `go build internal/jobs/worker/crawler_worker.go`
- Verify implements worker.JobWorker interface
- Verify all method signatures match interface
- Total lines: ~1529 (1034 + 495 from both source files)
- Verify injectAuthCookies() is called correctly at line ~198 in Execute() method
- Verify conditional auth logic: `if w.authStorage == nil { return nil }`

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\worker\crawler_worker.go(NEW)
- internal\jobs\processor\processor.go

Update app.go to import and use the new worker package for CrawlerWorker.

**Import Section Updates (around line 20):**

Add new import for worker package:
```
import (
    // ... existing imports ...
    "github.com/ternarybob/quaero/internal/jobs/processor"  // OLD - Keep for ParentJobExecutor and AgentExecutor (not migrated yet)
    "github.com/ternarybob/quaero/internal/jobs/worker"     // NEW - For CrawlerWorker
    // ... rest of imports ...
)
```

**CrawlerWorker Registration (around line 297):**

Replace:
```
crawlerExecutor := processor.NewCrawlerExecutor(
    a.CrawlerService,
    jobMgr,
    queueMgr,
    a.StorageManager.DocumentStorage(),
    a.StorageManager.AuthStorage(),
    a.StorageManager.JobDefinitionStorage(),
    a.Logger,
    a.EventService,
)
jobProcessor.RegisterExecutor(crawlerExecutor)
a.Logger.Info().Msg("Crawler URL worker registered for job type: crawler_url")
```

With:
```
crawlerWorker := worker.NewCrawlerWorker(
    a.CrawlerService,
    jobMgr,
    queueMgr,
    a.StorageManager.DocumentStorage(),
    a.StorageManager.AuthStorage(),
    a.StorageManager.JobDefinitionStorage(),
    a.Logger,
    a.EventService,
)
jobProcessor.RegisterExecutor(crawlerWorker)
a.Logger.Info().Msg("Crawler URL worker registered for job type: crawler_url")
```

**Variable Naming:**
- Changed from `crawlerExecutor` to `crawlerWorker` for clarity and consistency
- Constructor call: `processor.NewCrawlerExecutor()` → `worker.NewCrawlerWorker()`
- All 8 parameters remain in same order (no changes to parameter list)

**Keep Unchanged:**
- ParentJobExecutor registration (line 313) still uses `processor` package (not migrated in this phase)
- AgentExecutor registration (line 322) still uses `processor` package (migrating in ARCH-006)
- DatabaseMaintenanceExecutor registration (line 334) still uses `executor` package (migrating in ARCH-007)
- All other initialization code remains unchanged

**Log Message:**
- Already uses "worker" terminology (line 308) - no changes needed
- Message: "Crawler URL worker registered for job type: crawler_url"

**Validation:**
- Verify application compiles successfully
- Verify CrawlerWorker is registered correctly with JobProcessor
- Run application and check startup logs for "Crawler URL worker registered for job type: crawler_url"
- Verify crawler jobs execute correctly via UI or API

### AGENTS.md(MODIFY)

References: 

- docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update AGENTS.md to document the progress of the crawler worker migration (ARCH-005 completion).

**Section to Update: "Directory Structure (In Transition - ARCH-004)"**

Update the migration status to reflect ARCH-005 completion:

Change from:
```markdown
### Directory Structure (In Transition - ARCH-004)

Quaero is migrating to a Manager/Worker/Orchestrator architecture. The new structure is:

**New Directories:**
- `internal/jobs/manager/` - Job managers (orchestration layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_manager.go` (ARCH-004)
  - ✅ `database_maintenance_manager.go` (ARCH-004)
  - ✅ `agent_manager.go` (ARCH-004)
  - ⏳ `transform_manager.go` (pending)
  - ⏳ `reindex_manager.go` (pending)
  - ⏳ `places_search_manager.go` (pending)
- `internal/jobs/worker/` - Job workers (execution layer) with `interfaces.go`
- `internal/jobs/orchestrator/` - Parent job orchestrator (monitoring layer) with `interfaces.go`

**Old Directories (Still Active - Will be removed in ARCH-008):**
- `internal/jobs/executor/` - Old manager implementations (6 remaining files)
- `internal/jobs/processor/` - Old worker implementations (5 files, migrating in ARCH-005/ARCH-006)

**Migration Progress:**
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ⏳ Crawler worker migration (pending)
- Phase ARCH-006: ⏳ Remaining worker files migration (pending)
```

To:
```markdown
### Directory Structure (In Transition - ARCH-005)

Quaero is migrating to a Manager/Worker/Orchestrator architecture. The new structure is:

**New Directories:**
- `internal/jobs/manager/` - Job managers (orchestration layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_manager.go` (ARCH-004)
  - ✅ `database_maintenance_manager.go` (ARCH-004)
  - ✅ `agent_manager.go` (ARCH-004)
  - ⏳ `transform_manager.go` (pending)
  - ⏳ `reindex_manager.go` (pending)
  - ⏳ `places_search_manager.go` (pending)
- `internal/jobs/worker/` - Job workers (execution layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_worker.go` (ARCH-005) - Merged from crawler_executor.go + crawler_executor_auth.go
  - ⏳ `agent_worker.go` (pending - ARCH-006)
  - ⏳ `database_maintenance_worker.go` (pending - ARCH-007)
  - ⏳ `job_processor.go` (pending - ARCH-006)
- `internal/jobs/orchestrator/` - Parent job orchestrator (monitoring layer) with `interfaces.go`

**Old Directories (Still Active - Will be removed in ARCH-008):**
- `internal/jobs/executor/` - Old manager implementations (6 remaining files)
- `internal/jobs/processor/` - Old worker implementations (4 remaining files: agent_executor.go, processor.go, parent_job_executor.go, database_maintenance_executor.go)

**Migration Progress:**
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated (merged crawler_executor.go + crawler_executor_auth.go) (YOU ARE HERE)
- Phase ARCH-006: ⏳ Remaining worker files migration (pending)
```

**Section to Update: "Interfaces"**

Update to reflect that CrawlerWorker is now in worker/ package:

```markdown
### Interfaces

**New Architecture (ARCH-003+):**
- `JobManager` interface - `internal/jobs/manager/interfaces.go`
  - Implementations: `CrawlerManager`, `DatabaseMaintenanceManager`, `AgentManager` (ARCH-004)
- `JobWorker` interface - `internal/jobs/worker/interfaces.go`
  - Implementations: `CrawlerWorker` (ARCH-005)
- `ParentJobOrchestrator` interface - `internal/jobs/orchestrator/interfaces.go`

**Old Architecture (deprecated, will be removed in ARCH-008):**
- `JobManager` interface - `internal/jobs/executor/interfaces.go` (duplicate)
  - Remaining implementations: `TransformStepExecutor`, `ReindexStepExecutor`, `PlacesSearchStepExecutor`
- `JobWorker` interface - `internal/interfaces/job_executor.go` (duplicate)
  - Remaining implementations: `AgentExecutor`, `DatabaseMaintenanceExecutor` (in processor/executor directories)
```

**Implementation Notes:**
- Update migration status to show ARCH-005 complete
- Add checkmark (✅) for crawler_worker.go
- Add note about file merge (crawler_executor.go + crawler_executor_auth.go)
- Update remaining file count in processor/ directory (4 files remaining)
- Clarify which workers are migrated vs remaining in old location

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update MANAGER_WORKER_ARCHITECTURE.md to document the completion of ARCH-005 (crawler worker migration and file merge).

**Section to Update: "Current Status (After ARCH-004)"**

Change the section title and content to reflect ARCH-005 completion:

```markdown
### Current Status (After ARCH-005)

**New Directories Created:**
- ✅ `internal/jobs/manager/` - Created with interfaces.go (ARCH-003)
  - ✅ `crawler_manager.go` - Migrated from executor/ (ARCH-004)
  - ✅ `database_maintenance_manager.go` - Migrated from executor/ (ARCH-004)
  - ✅ `agent_manager.go` - Migrated from executor/ (ARCH-004)
- ✅ `internal/jobs/worker/` - Created with interfaces.go (ARCH-003)
  - ✅ `crawler_worker.go` - Merged and migrated from processor/ (ARCH-005)
- ✅ `internal/jobs/orchestrator/` - Created with interfaces.go (ARCH-003)

**Old Directories (Still Active):**
- `internal/jobs/executor/` - Contains 6 remaining implementation files:
  - `transform_step_executor.go` (pending migration)
  - `reindex_step_executor.go` (pending migration)
  - `places_search_step_executor.go` (pending migration)
  - `job_executor.go` (orchestrator - will be refactored separately)
  - `base_executor.go` (shared utilities - will be refactored separately)
  - `database_maintenance_executor.go` (old worker - will be deleted in ARCH-007)
- `internal/jobs/processor/` - Contains 4 remaining implementation files:
  - `agent_executor.go` (migrating in ARCH-006)
  - `processor.go` (migrating in ARCH-006)
  - `parent_job_executor.go` (migrating in ARCH-006)
  - `database_maintenance_executor.go` (will be deleted in ARCH-007)

**Migration Status:**
- Phase ARCH-001: ✅ Documentation created
- Phase ARCH-002: ✅ Interfaces renamed
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ Manager files migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated and merged (YOU ARE HERE)
- Phase ARCH-006: ⏳ Remaining worker files migration (pending)
- Phase ARCH-007: ⏳ Database maintenance worker split (pending)
- Phase ARCH-008: ⏳ Import path updates and cleanup (pending)
- Phase ARCH-009: ⏳ End-to-end validation (pending)
```

**Section to Add: "Crawler Worker File Merge (ARCH-005)"**

Add a new subsection after "Manager Files Migrated (ARCH-004)":

```markdown
### Crawler Worker File Merge (ARCH-005)

**Files Merged:**

1. **Source Files (Deleted after merge):**
   - `internal/jobs/processor/crawler_executor.go` (1034 lines)
   - `internal/jobs/processor/crawler_executor_auth.go` (495 lines)

2. **Target File:**
   - `internal/jobs/worker/crawler_worker.go` (~1529 lines)

**Transformations Applied:**

- **Package**: `processor` → `worker`
- **Struct**: `CrawlerExecutor` → `CrawlerWorker`
- **Constructor**: `NewCrawlerExecutor()` → `NewCrawlerWorker()`
- **Receiver**: `func (e *CrawlerExecutor)` → `func (w *CrawlerWorker)`
- **Interface**: Implements `worker.JobWorker` (Execute, GetWorkerType, Validate)

**File Organization:**

1. **Interface Methods** (top of file):
   - GetWorkerType() - Returns "crawler_url"
   - Validate() - Validates job type and config
   - Execute() - Main workflow (ChromeDP rendering, content processing, child job spawning)

2. **Configuration and Rendering Helpers**:
   - extractCrawlConfig() - Parse crawl configuration
   - renderPageWithChromeDp() - ChromeDP page rendering

3. **Authentication Helpers** (moved from crawler_executor_auth.go):
   - injectAuthCookies() - Load auth credentials and inject cookies
   - Conditional logic: checks `if w.authStorage == nil` and returns early
   - Three phases: pre-injection diagnostics, network enablement, post-injection verification
   - Extensive logging with 🔐 emoji for auth operations

4. **Child Job Management**:
   - spawnChildJob() - Create and enqueue child jobs for discovered links

5. **Event Publishing Helpers** (grouped at end):
   - publishCrawlerJobLog() - Publish job log events
   - publishCrawlerProgressUpdate() - Publish progress updates
   - publishLinkDiscoveryEvent() - Publish link discovery statistics
   - publishJobSpawnEvent() - Publish child job spawn events

**Key Features Preserved:**

- **Conditional Authentication**: Auth logic only executes if authStorage is not nil
- **ChromeDP Rendering**: Full JavaScript execution with network/log domain enabling
- **Content Processing**: HTML to markdown conversion with metadata extraction
- **Link Discovery**: Pattern-based filtering with include/exclude rules
- **Child Job Spawning**: Respects depth limits and max pages configuration
- **Real-Time Events**: WebSocket events for progress monitoring and log streaming
- **Comprehensive Logging**: Structured logging with correlation IDs for parent job aggregation

**Import Path Updates:**

- `internal/app/app.go` (line 297) - Updated to import and use `worker.NewCrawlerWorker()`
- Variable renamed: `crawlerExecutor` → `crawlerWorker`

**Backward Compatibility:**

- Old files remain in `internal/jobs/processor/` with deprecation notices until ARCH-008
- Dual import strategy allows gradual transition
- No breaking changes to external APIs or job execution behavior
```

**Implementation Notes:**
- Update migration status to show ARCH-005 complete
- Add detailed documentation of file merge process
- Document file organization and structure
- Clarify conditional auth logic and key features preserved
- Provide context for developers working during transition

### internal\jobs\processor\crawler_executor.go(MODIFY)

References: 

- internal\jobs\worker\crawler_worker.go(NEW)

Add deprecation notice to the old crawler_executor.go file to indicate it has been merged and migrated.

**Add Deprecation Comment at Top of File (after existing header comment):**

```
// -----------------------------------------------------------------------
// Crawler Executor - Individual URL Crawling with ChromeDP and Content Processing
// -----------------------------------------------------------------------

// DEPRECATED: This file has been merged with crawler_executor_auth.go and migrated to
// internal/jobs/worker/crawler_worker.go (ARCH-005).
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/worker and use CrawlerWorker instead.
//
// Migration Details:
// - Struct renamed: CrawlerExecutor → CrawlerWorker
// - Constructor renamed: NewCrawlerExecutor() → NewCrawlerWorker()
// - Auth logic merged: injectAuthCookies() now in crawler_worker.go
// - Package changed: processor → worker
```

**No Other Changes:**
- Keep all existing code unchanged
- File remains functional for backward compatibility
- Will be deleted in ARCH-008 when all imports are updated

**Purpose:**
- Clearly communicate to developers that this file is deprecated
- Provide guidance on where to find the new implementation
- Document the migration timeline (removal in ARCH-008)
- Explain the merge with crawler_executor_auth.go
- Prevent new code from using the old location

### internal\jobs\processor\crawler_executor_auth.go(MODIFY)

References: 

- internal\jobs\worker\crawler_worker.go(NEW)

Add deprecation notice to the old crawler_executor_auth.go file to indicate it has been merged into crawler_worker.go.

**Add Deprecation Comment at Top of File (after existing header comment):**

```
// -----------------------------------------------------------------------
// Crawler Executor - Authentication Cookie Injection
// -----------------------------------------------------------------------

// DEPRECATED: This file has been merged into internal/jobs/worker/crawler_worker.go (ARCH-005).
// The injectAuthCookies() method is now a private method in CrawlerWorker.
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/worker and use CrawlerWorker instead.
//
// Migration Details:
// - Method moved: injectAuthCookies() now in crawler_worker.go as private method
// - Receiver updated: func (e *CrawlerExecutor) → func (w *CrawlerWorker)
// - All auth logic preserved: conditional authStorage check, three-phase injection, extensive logging
// - Package changed: processor → worker
```

**No Other Changes:**
- Keep all existing code unchanged
- File remains functional for backward compatibility (method still exists on CrawlerExecutor)
- Will be deleted in ARCH-008 when crawler_executor.go is deleted

**Purpose:**
- Clearly communicate to developers that this file is deprecated
- Provide guidance on where to find the new implementation
- Document the migration timeline (removal in ARCH-008)
- Explain the merge into crawler_worker.go
- Prevent new code from using the old location