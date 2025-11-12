I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The `JobOrchestrator` component is well-isolated with clear boundaries:
- **Single implementation** in `internal/jobs/orchestrator/job_orchestrator.go` (520 lines)
- **Interface definition** in `internal/interfaces/job_interfaces.go` (lines 24-36)
- **4 files with actual usage**: app.go, job_definition_orchestrator.go, database_maintenance_manager.go, and the implementation itself
- **4 files with comment-only references**: job_processor.go, manager.go, event_service.go, places_job_document_test.go

**Naming Rationale:**

The user correctly identified that "orchestrator" is overloaded:
- `JobDefinitionOrchestrator` - Routes job definition steps to managers (true orchestration)
- `JobOrchestrator` - Monitors job progress and aggregates statistics (passive monitoring)

Using "monitor" for the latter clarifies its role as an **observer** rather than a **coordinator**.

**Architecture Impact:**

This refactoring improves the Manager/Worker/Monitor terminology:
- **Managers** create jobs and define workflows (active orchestration)
- **Workers** execute individual tasks (active execution)  
- **Monitor** tracks progress and aggregates stats (passive observation)

The new naming better reflects the reactive, event-driven nature of the monitoring component.

### Approach

This is a **systematic rename refactoring** to improve domain clarity by replacing "orchestrator" terminology with "monitor" for the job monitoring component. The refactoring follows a clear dependency order:

1. **Interface layer first** - Update the contract definition
2. **Implementation layer** - Rename folder, file, package, types
3. **Consumer layer** - Update all references in dependent code
4. **Documentation layer** - Update architecture docs and comments

**Key Decision:** Rename both the folder (`orchestrator/` → `monitor/`) and all types (`JobOrchestrator` → `JobMonitor`) for complete consistency. This eliminates confusion with the root-level `JobDefinitionOrchestrator` which handles workflow orchestration.

**Risk Mitigation:** The refactoring is low-risk because:
- The interface is internal (not exposed via API)
- Only 8 Go files reference it (4 with actual usage, 4 with comments only)
- No test files exist that would need updating
- Build verification will catch any missed references

### Reasoning

I explored the codebase systematically:
1. Listed repository structure to understand layout
2. Read the current implementation file (`job_orchestrator.go`)
3. Searched for all references using grep (found 18 results for `JobOrchestrator`, 25 for `jobOrchestrator`, 1 import path)
4. Read the interface definition in `job_interfaces.go`
5. Checked for test files (none exist in `test/unit/jobs`)
6. Searched markdown files for documentation references
7. Read portions of the architecture documentation to understand update scope

This comprehensive search ensures no references will be missed during the refactoring.

## Proposed File Changes

### internal\interfaces\job_interfaces.go(MODIFY)

**Update Interface Definition (Lines 24-36):**

1. **Interface name (Line 27):** `JobOrchestrator` → `JobMonitor`
2. **Comment (Line 24):** "JobOrchestrator monitors parent job progress" → "JobMonitor monitors parent job progress"
3. **Comment (Line 26):** "Orchestrators subscribe to child job status changes" → "Monitors subscribe to child job status changes"
4. **Method signatures:** Keep `StartMonitoring()` and `SubscribeToChildStatusChanges()` unchanged (names are already appropriate for monitoring)

**Rationale:** The interface defines the monitoring contract. Renaming to `JobMonitor` clarifies its passive observation role versus the active orchestration role of `JobDefinitionOrchestrator`.

### internal\jobs\orchestrator → internal\jobs\monitor

**Rename directory for domain clarity.**

**Rationale:** Aligns the package location with its monitoring responsibility. The `monitor/` directory name clearly indicates this component observes and tracks job progress rather than orchestrating workflows.

### internal\jobs\orchestrator\job_orchestrator.go → internal\jobs\monitor\job_monitor.go

**Rename file to match new package and type names.**

**Rationale:** Maintains Go convention of matching filename to primary type name. After this rename, the file will be at `internal/jobs/monitor/job_monitor.go`.

### internal\jobs\monitor\job_monitor.go(NEW)

References: 

- internal\interfaces\job_interfaces.go(MODIFY)

**Update Package and Type Names (After File Rename):**

1. **Package declaration (Line 1):** `package orchestrator` → `package monitor`

2. **Struct name (Line 18):** `jobOrchestrator` → `jobMonitor` (unexported, lowercase)

3. **Constructor name (Line 24):** `NewJobOrchestrator` → `NewJobMonitor`

4. **Constructor return type (Line 29):** `interfaces.JobOrchestrator` → `interfaces.JobMonitor`

5. **Constructor variable (Line 30):** `orchestrator := &jobOrchestrator{` → `monitor := &jobMonitor{`

6. **Constructor return (Line 39):** `return orchestrator` → `return monitor`

7. **All receiver declarations (~15 methods):** `(o *jobOrchestrator)` → `(m *jobMonitor)`
   - Affects methods: `StartMonitoring`, `validate`, `monitorChildJobs`, `checkChildJobProgress`, `publishParentJobProgress`, `publishChildJobStats`, `SubscribeToChildStatusChanges`, `formatProgressText`, `publishParentJobProgressUpdate`, `calculateOverallStatus`

**Comment Updates:**

- Line 14: "jobOrchestrator monitors" → "jobMonitor monitors"
- Line 24: "NewJobOrchestrator creates" → "NewJobMonitor creates"
- Line 24: "parent job orchestrator" → "job monitor"
- Line 42: "parent job orchestration" → "job monitoring"
- Line 51: "orchestrator" → "monitor" (in error message context)
- Line 73: "Parent job monitoring started" (keep as-is, refers to activity not component)
- Line 79: "this orchestrator" → "this monitor"
- Line 412: "JobOrchestrator subscribed" → "JobMonitor subscribed"

**Variable Naming:**
- Keep `orchestrator` variable name in constructor (Line 30) → change to `monitor` for consistency
- Receiver variable `o` → `m` throughout (standard Go convention for monitor)

**No logic changes** - all method implementations remain identical. This is purely a naming refactoring.

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\monitor\job_monitor.go(NEW)
- internal\jobs\job_definition_orchestrator.go(MODIFY)
- internal\jobs\manager\database_maintenance_manager.go(MODIFY)

**Update Import and Usage (4 locations):**

1. **Import statement (Line 21):** `"github.com/ternarybob/quaero/internal/jobs/orchestrator"` → `"github.com/ternarybob/quaero/internal/jobs/monitor"`

2. **Variable declaration (Line 313):** `jobOrchestrator := orchestrator.NewJobOrchestrator(` → `jobMonitor := monitor.NewJobMonitor(`

3. **Comment (Line 310):** "Create job orchestrator" → "Create job monitor"

4. **Comment (Line 311):** "Parent jobs are NOT registered with JobProcessor" - Keep as-is (refers to job type, not component)

5. **Log message (Line 318):** "Job orchestrator created (runs in background goroutines, not via queue)" → "Job monitor created (runs in background goroutines, not via queue)"

6. **Variable reference (Line 373):** `jobs.NewJobDefinitionOrchestrator(jobMgr, jobOrchestrator, a.Logger)` → use `jobMonitor`

7. **Comment (Line 372):** "Pass jobOrchestrator so it can start monitoring" → "Pass jobMonitor so it can start monitoring"

8. **Variable reference (Line 388):** `manager.NewDatabaseMaintenanceManager(a.JobManager, queueMgr, jobOrchestrator, a.Logger)` → use `jobMonitor`

**Rationale:** Updates all references to use the new package name and variable naming. The import path changes from `orchestrator` to `monitor`, and the constructor call changes accordingly.

### internal\jobs\job_definition_orchestrator.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(MODIFY)

**Update Field and Parameter Names (3 locations):**

1. **Struct field (Line 18):** `jobOrchestrator interfaces.JobOrchestrator` → `jobMonitor interfaces.JobMonitor`

2. **Constructor parameter (Line 23):** `jobOrchestrator interfaces.JobOrchestrator` → `jobMonitor interfaces.JobMonitor`

3. **Constructor assignment (Line 27):** `jobOrchestrator: jobOrchestrator,` → `jobMonitor: jobMonitor,`

4. **Method call (Line 368):** `o.jobOrchestrator.StartMonitoring(ctx, parentJobModel)` → `o.jobMonitor.StartMonitoring(ctx, parentJobModel)`

**Comment Updates:**

- Line 292: "let JobOrchestrator handle completion" → "let JobMonitor handle completion"
- Line 333: "leaving in running state for JobOrchestrator to monitor" → "leaving in running state for JobMonitor to monitor"
- Line 373: "NOTE: Do NOT set finished_at for crawler jobs - JobOrchestrator will handle this" → use "JobMonitor"

**Rationale:** The JobDefinitionOrchestrator stores a reference to the monitor for starting background monitoring goroutines. Field and parameter names must match the new interface name.

### internal\jobs\manager\database_maintenance_manager.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(MODIFY)

**Update Field and Parameter Names (4 locations):**

1. **Struct field (Line 25):** `jobOrchestrator interfaces.JobOrchestrator` → `jobMonitor interfaces.JobMonitor`

2. **Constructor parameter (Line 33):** `jobOrchestrator interfaces.JobOrchestrator` → `jobMonitor interfaces.JobMonitor`

3. **Constructor assignment (Line 37):** `jobOrchestrator: jobOrchestrator,` → `jobMonitor: jobMonitor,`

4. **Method call (Line 166):** `m.jobOrchestrator.StartMonitoring(ctx, parentJobModel)` → `m.jobMonitor.StartMonitoring(ctx, parentJobModel)`

**Comment Update:**

- Line 150: "Start JobOrchestrator monitoring" → "Start JobMonitor monitoring"

**Rationale:** DatabaseMaintenanceManager stores a reference to the monitor for tracking database maintenance job progress. All references must use the new interface and field names.

### internal\jobs\worker\job_processor.go(MODIFY)

**Update Comments Only (2 locations - No Code Changes):**

1. **Line 221:** "For parent jobs, do NOT mark as completed here - JobOrchestrator will handle completion" → "For parent jobs, do NOT mark as completed here - JobMonitor will handle completion"

2. **Line 227:** "Parent job remains in 'running' state and will be re-enqueued by JobOrchestrator" → "Parent job remains in 'running' state and will be monitored by JobMonitor"

**Rationale:** These are documentation comments explaining the parent job lifecycle. Update for consistency with the renamed component. Note: Line 227's comment about "re-enqueued" is slightly misleading - the monitor doesn't re-enqueue, it monitors in a goroutine. Consider clarifying this during the update.

### internal\jobs\manager.go(MODIFY)

**Update Comment Only (1 location - No Code Changes):**

**Line 1687:** "This is used by the JobOrchestrator to monitor child job progress" → "This is used by the JobMonitor to monitor child job progress"

**Rationale:** Documentation comment for the `GetChildJobStats()` method. Update for consistency with the renamed component.

### internal\interfaces\event_service.go(MODIFY)

**Update Comments Only (2 locations - No Code Changes):**

1. **Line 166:** "Used by JobOrchestrator to track child job progress" → "Used by JobMonitor to track child job progress"

2. **Line 177:** "Used by JobOrchestrator to track document count" → "Used by JobMonitor to track document count"

**Rationale:** Event documentation comments describing which component subscribes to these events. Update for consistency.

### test\api\places_job_document_test.go(MODIFY)

**Update Comment Only (1 location - No Code Changes):**

**Line 125:** "This is set by the event-driven JobOrchestrator when EventDocumentSaved is published" → "This is set by the event-driven JobMonitor when EventDocumentSaved is published"

**Rationale:** Test documentation comment explaining how document counts are tracked. Update for consistency with the renamed component.

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

**Comprehensive Documentation Update:**

Search and replace all occurrences throughout the document (946 lines):

**Primary Replacements:**
- "JobOrchestrator" → "JobMonitor" (interface/type name)
- "Job Orchestrator" → "Job Monitor" (prose references)
- "job orchestrator" → "job monitor" (lowercase prose)
- "orchestrator/job_orchestrator.go" → "monitor/job_monitor.go" (file path)
- "internal/jobs/orchestrator/" → "internal/jobs/monitor/" (directory path)

**Key Sections to Update:**

1. **Executive Summary (Line 13):** "Orchestrator (JobOrchestrator)" → "Monitor (JobMonitor)"

2. **Architecture Diagram (Line 36):** `Orchestrator[Job Orchestrator]` → `Monitor[Job Monitor]`

3. **Component Responsibilities Table (Line 50):** "JobOrchestrator" → "JobMonitor"

4. **Section Header (Line 184):** "## Orchestrator Responsibilities" → "## Monitor Responsibilities"

5. **Subsection Header (Line 186):** "### JobOrchestrator" → "### JobMonitor"

6. **File Path (Line 188):** Update to `internal/jobs/monitor/job_monitor.go`

7. **Sequence Diagram (Line 256):** `participant Orchestrator as Job Orchestrator` → `participant Monitor as Job Monitor`

8. **Flow Phases (Lines 295-315):** Update all references in phase descriptions

9. **Interface Definitions (Lines 323-356):** Update interface name and comments

10. **Implementations Section (Lines 357-432):** Update orchestrator references

11. **Best Practices (Lines 785-791):** "Orchestrator Design Guidelines" → "Monitor Design Guidelines"

12. **Troubleshooting (Lines 816-835):** Update section titles and diagnostic commands

**Estimated Changes:** ~40-50 occurrences across the document.

**Rationale:** This is the primary architecture documentation. All references must be updated to reflect the new Monitor terminology for consistency and clarity.

### AGENTS.md(MODIFY)

**Update Architecture Documentation:**

Search and replace all occurrences in the Job System Architecture section:

**Primary Replacements:**
- "JobOrchestrator" → "JobMonitor"
- "jobOrchestrator" → "jobMonitor"
- "job_orchestrator.go" → "job_monitor.go"
- "internal/jobs/orchestrator/" → "internal/jobs/monitor/"

**Key Sections to Update:**

1. **Job System Architecture** section describing Manager/Worker/Monitor pattern
2. **Directory Structure** listings showing `internal/jobs/orchestrator/` → `internal/jobs/monitor/`
3. **Interfaces** section describing the monitor interface
4. **Core Components** descriptions
5. **Job Execution Flow** diagrams and explanations
6. **Service Initialization Flow** (Line ~310-318 in app.go context)

**Estimated Changes:** ~15-20 occurrences based on grep results.

**Rationale:** AGENTS.md is the primary architectural documentation for AI assistants working on the codebase. Must be updated to reflect the simplified naming convention.

### docs\architecture\README.md(MODIFY)

**Update Architecture Overview (Minor Changes):**

Search and replace in the MANAGER_WORKER_ARCHITECTURE.md description section:

**Replacements:**
- Line 16: "Orchestrator responsibilities" → "Monitor responsibilities"
- Line 18: "Interface definitions (JobManager, JobWorker, JobOrchestrator)" → "Interface definitions (JobManager, JobWorker, JobMonitor)"
- Line 19: "File structure organization (manager/, worker/, orchestrator/)" → "File structure organization (manager/, worker/, monitor/)"

**Rationale:** This README provides an overview of architecture documentation. Update references to match the new terminology.