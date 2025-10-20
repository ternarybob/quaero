I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Existing Job Configuration:**
- `JobConfig` struct (lines 149-153 in `config.go`) has three fields: `Enabled`, `Schedule`, `Description`
- Jobs are registered in `app.go` (lines 296-358) and disabled if `!Config.Jobs.*.Enabled`
- Scheduler starts all enabled jobs when `Start()` is called (line 377 in `app.go`)
- Jobs execute only on cron schedule or manual trigger - no immediate execution on startup

**User Requirement:**
Add `auto_start` boolean field to control whether jobs execute immediately when the service starts, independent of scheduled execution. Default should be `false` to preserve current behavior (jobs only run on schedule or manual trigger).

**Key Files Identified:**
1. `internal/common/config.go` - JobConfig struct definition and defaults
2. `internal/services/scheduler/scheduler_service.go` - Job execution logic
3. `deployments/local/quaero.toml` - User-facing configuration
4. `internal/app/app.go` - Job registration and scheduler startup

**Design Decision:**
The `auto_start` flag should trigger immediate job execution AFTER the scheduler starts but BEFORE the application completes initialization. This ensures all dependencies are ready (crawler service, transformers, etc.) before jobs execute.

### Approach

Add `auto_start` boolean field to `JobConfig` struct with default value `false`. After scheduler starts and jobs are registered, iterate through all registered jobs and execute those with `auto_start=true` immediately. This preserves current behavior (no auto-start) while allowing users to opt-in for immediate execution on service startup.

### Reasoning

Explored the repository structure, read the configuration system (`config.go`), scheduler service implementation (`scheduler_service.go`), TOML config example (`quaero.toml`), and application initialization logic (`app.go`). Analyzed how jobs are currently registered, enabled/disabled, and executed. Identified that jobs currently only execute on cron schedule or manual trigger, with no immediate execution on startup.

## Mermaid Diagram

sequenceDiagram
    participant Config as config.go
    participant App as app.go
    participant Scheduler as scheduler_service.go
    participant Job as Job Handler

    Note over Config,Job: Service Startup Flow with AutoStart

    Config->>Config: Load quaero.toml
    Config->>Config: Parse jobs.crawl_and_collect.auto_start
    Config->>Config: Default: auto_start = false

    App->>Scheduler: RegisterJob("crawl_and_collect", schedule, desc, autoStart, handler)
    Scheduler->>Scheduler: Store autoStart in jobEntry struct
    Scheduler->>Scheduler: Add job to cron scheduler

    App->>Scheduler: RegisterJob("scan_and_summarize", schedule, desc, autoStart, handler)
    Scheduler->>Scheduler: Store autoStart in jobEntry struct

    App->>Scheduler: Start("*/5 * * * *")
    Scheduler->>Scheduler: cron.Start()
    Scheduler->>Scheduler: Log: "Scheduler started"

    Scheduler->>Scheduler: go executeAutoStartJobs()
    activate Scheduler

    Scheduler->>Scheduler: Lock jobMu, iterate jobs
    Scheduler->>Scheduler: Find jobs with enabled=true && autoStart=true

    alt auto_start = true
        Scheduler->>Job: go executeJob("crawl_and_collect")
        activate Job
        Job->>Job: Execute job handler immediately
        Job->>Job: Log: "Job execution started"
        Job-->>Scheduler: Complete
        deactivate Job
    else auto_start = false
        Scheduler->>Scheduler: Skip immediate execution
        Note over Scheduler: Job waits for scheduled time
    end

    deactivate Scheduler

    Note over Scheduler,Job: Later: Scheduled Execution
    Scheduler->>Job: Cron triggers job (schedule: */5 * * * *)
    Job->>Job: Execute job handler
    Job-->>Scheduler: Complete

    Note over Scheduler,Job: Manual Trigger (API/UI)
    App->>Scheduler: executeJob("crawl_and_collect")
    Scheduler->>Job: Execute job handler
    Job-->>Scheduler: Complete

## Proposed File Changes

### internal\common\config.go(MODIFY)

References: 

- deployments\local\quaero.toml(MODIFY)

## Add AutoStart Field to JobConfig Struct

**Location: Lines 149-153 (JobConfig struct definition)**

1. **Add `AutoStart` field** after the `Enabled` field:
   - Field name: `AutoStart`
   - Type: `bool`
   - TOML tag: `toml:"auto_start"`
   - Purpose: Controls whether job executes immediately on service startup
   - Position: Between `Enabled` and `Schedule` fields for logical grouping

**Location: Lines 246-257 (NewDefaultConfig function - Jobs section)**

2. **Set default value for CrawlAndCollect job**:
   - Add `AutoStart: false` to `JobConfig` initialization (line 247-251)
   - This preserves current behavior (no immediate execution on startup)
   - Users must explicitly set `auto_start = true` in TOML to enable

3. **Set default value for ScanAndSummarize job**:
   - Add `AutoStart: false` to `JobConfig` initialization (line 252-256)
   - Consistent with crawl_and_collect default

**Expected Result:**
- `JobConfig` struct has four fields: `Enabled`, `AutoStart`, `Schedule`, `Description`
- Default configuration has `auto_start = false` for both jobs
- Backward compatible: existing configs without `auto_start` field will use default `false`

### internal\services\scheduler\scheduler_service.go(MODIFY)

References: 

- internal\interfaces\scheduler_service.go(MODIFY)
- internal\app\app.go(MODIFY)

## Add Auto-Start Execution Logic

**Location: Lines 17-29 (jobEntry struct definition)**

1. **Add `autoStart` field to jobEntry struct**:
   - Field name: `autoStart`
   - Type: `bool`
   - Purpose: Store auto-start flag from JobConfig for later execution
   - Position: After `enabled` field (line 23)

**Location: Lines 190-230 (RegisterJob method)**

2. **Accept autoStart parameter in RegisterJob signature**:
   - Update method signature: `RegisterJob(name string, schedule string, description string, autoStart bool, handler func() error) error`
   - Store `autoStart` value in `jobEntry` struct (line 205-211)
   - This allows jobs to declare their auto-start preference during registration

**Location: Lines 362-394 (GetJobStatus method)**

3. **Include AutoStart in JobStatus response**:
   - Add `AutoStart` field to returned `interfaces.JobStatus` struct (line 384-393)
   - This allows UI to display whether a job is configured for auto-start

**Location: After line 113 (end of Start method)**

4. **Create new `executeAutoStartJobs()` method** (new method after `Start()`):
   - Method signature: `func (s *Service) executeAutoStartJobs()`
   - Purpose: Execute all jobs with `autoStart=true` immediately after scheduler starts
   - Logic:
     - Acquire `s.jobMu.Lock()` to safely iterate jobs map
     - Collect names of jobs where `entry.enabled && entry.autoStart`
     - Release lock before executing (avoid holding lock during execution)
     - For each auto-start job, call `s.executeJob(name)` in a goroutine
     - Log: "Executing auto-start job: {name}"
   - Error handling: Log errors but don't fail scheduler startup

5. **Call executeAutoStartJobs() from Start() method**:
   - Location: After line 110 (after "Scheduler started" log)
   - Add: `go s.executeAutoStartJobs()` to execute in background
   - Use goroutine to avoid blocking scheduler startup
   - This ensures scheduler is fully started before jobs execute

**Expected Result:**
- Jobs with `auto_start=true` execute immediately after scheduler starts
- Execution happens in background (non-blocking)
- Jobs with `auto_start=false` only execute on schedule or manual trigger
- Manual job triggering via API/UI works regardless of auto-start setting

### internal\interfaces\scheduler_service.go(MODIFY)

References: 

- internal\services\scheduler\scheduler_service.go(MODIFY)

## Update SchedulerService Interface

**Location: RegisterJob method signature**

1. **Update RegisterJob method signature** to include `autoStart` parameter:
   - Current: `RegisterJob(name string, schedule string, description string, handler func() error) error`
   - Updated: `RegisterJob(name string, schedule string, description string, autoStart bool, handler func() error) error`
   - This ensures all implementations accept the auto-start flag

**Location: JobStatus struct definition**

2. **Add AutoStart field to JobStatus struct**:
   - Field name: `AutoStart`
   - Type: `bool`
   - Purpose: Expose auto-start configuration to API/UI
   - Position: After `Enabled` field for logical grouping

**Expected Result:**
- Interface signature matches scheduler service implementation
- JobStatus includes auto-start information for API responses

### internal\app\app.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\services\scheduler\scheduler_service.go(MODIFY)

## Update Job Registration to Pass AutoStart Flag

**Location: Lines 308-329 (crawl_and_collect job registration)**

1. **Pass AutoStart flag to RegisterJob call** (line 308-313):
   - Update `RegisterJob()` call to include `a.Config.Jobs.CrawlAndCollect.AutoStart` parameter
   - Position: After `description` parameter, before `handler` parameter
   - Example: `a.SchedulerService.RegisterJob("crawl_and_collect", schedule, description, autoStart, handler)`

2. **Update logging to include auto-start status** (line 325-328):
   - Add `.Bool("auto_start", a.Config.Jobs.CrawlAndCollect.AutoStart)` to log statement
   - This helps diagnose whether jobs will execute immediately on startup

**Location: Lines 337-358 (scan_and_summarize job registration)**

3. **Pass AutoStart flag to RegisterJob call** (line 337-342):
   - Update `RegisterJob()` call to include `a.Config.Jobs.ScanAndSummarize.AutoStart` parameter
   - Consistent with crawl_and_collect registration

4. **Update logging to include auto-start status** (line 354-357):
   - Add `.Bool("auto_start", a.Config.Jobs.ScanAndSummarize.AutoStart)` to log statement

**Expected Result:**
- Job registration passes auto-start configuration from TOML to scheduler
- Logs clearly indicate which jobs are configured for auto-start
- Jobs execute immediately on startup if `auto_start=true` in config

### deployments\local\quaero.toml(MODIFY)

References: 

- internal\common\config.go(MODIFY)

## Document AutoStart Configuration Option

**Location: Lines 100-107 (Default Jobs Configuration section header)**

1. **Update section documentation** to explain auto-start behavior:
   - Add explanation: "The `auto_start` option controls whether jobs execute immediately when the service starts."
   - Add note: "Set `auto_start = true` to run jobs on startup (useful for initial data collection)."
   - Add note: "Set `auto_start = false` (default) to only run jobs on schedule or manual trigger."
   - Clarify that auto-start is independent of the cron schedule

**Location: Lines 108-111 (crawl_and_collect job configuration)**

2. **Add `auto_start` field** to crawl_and_collect configuration:
   - Add line: `auto_start = false`
   - Position: After `enabled` field, before `schedule` field
   - Comment: `# Execute immediately on service startup (default: false)`

**Location: Lines 113-116 (scan_and_summarize job configuration)**

3. **Add `auto_start` field** to scan_and_summarize configuration:
   - Add line: `auto_start = false`
   - Position: After `enabled` field, before `schedule` field
   - Comment: `# Execute immediately on service startup (default: false)`

**Location: Lines 100-107 (section header)**

4. **Add usage example in comments**:
   - Example: "To collect data immediately on startup, set `auto_start = true` for crawl_and_collect."
   - Example: "Jobs with `auto_start = false` will wait for their first scheduled run."
   - Clarify: "Manual job triggering via API/UI works regardless of auto_start setting."

**Expected Result:**
- Users understand the purpose of `auto_start` field
- Clear examples show how to enable immediate execution on startup
- Default value (`false`) is documented and visible
- Backward compatible: existing configs work without modification