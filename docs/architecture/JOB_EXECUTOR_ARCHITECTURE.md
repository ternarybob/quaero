# Job Executor Architecture

## Overview

The Job Executor provides a generic, job-agnostic system for executing JobDefinitions with sequential steps, parent-child hierarchy tracking, and efficient status aggregation for UI reporting.

## Key Requirements

1. **Job Agnostic**: Works with any job type through interfaces
2. **Sequential Execution**: Steps execute in order (pre → core → post)
3. **Hierarchy Tracking**: Parent jobs track all child jobs
4. **Status Aggregation**: Efficient UI reporting of job tree status
5. **Error Handling**: Proper error tolerance and recovery

## Architecture Components

### 1. StepExecutor Interface

```go
// StepExecutor executes a single step of a job definition
type StepExecutor interface {
    // ExecuteStep executes a step and returns the job ID created
    // The jobID is used to track the parent-child hierarchy
    ExecuteStep(ctx context.Context, step models.JobStep, sources []string, config map[string]interface{}) (jobID string, err error)

    // GetStepType returns the action type this executor handles
    GetStepType() string
}
```

### 2. JobExecutor

```go
// JobExecutor orchestrates job definition execution
type JobExecutor struct {
    stepExecutors map[string]StepExecutor
    jobManager    *jobs.Manager
    logger        arbor.ILogger
}

// Execute executes a job definition
// Returns the parent job ID
func (e *JobExecutor) Execute(ctx context.Context, jobDef *models.JobDefinition) (string, error) {
    // 1. Create parent job record
    parentJobID := uuid.New().String()

    // 2. Execute pre-jobs (if any)
    for _, preJobID := range jobDef.PreJobs {
        // Load and execute pre-job definition
    }

    // 3. Execute steps sequentially
    for _, step := range jobDef.Steps {
        executor, exists := e.stepExecutors[step.Action]
        if !exists {
            return "", fmt.Errorf("no executor for action: %s", step.Action)
        }

        // Execute step - returns child job ID
        childJobID, err := executor.ExecuteStep(ctx, step, jobDef.Sources, step.Config)
        if err != nil {
            // Handle based on step.OnError strategy
            if step.OnError == "fail" {
                return parentJobID, err
            }
            // Log and continue for "continue" strategy
        }

        // Track child job under parent
        // jobManager.LinkChild(parentJobID, childJobID)
    }

    // 4. Execute post-jobs (if any)
    for _, postJobID := range jobDef.PostJobs {
        // Load and execute post-job definition
    }

    return parentJobID, nil
}
```

### 3. CrawlerStepExecutor

```go
// CrawlerStepExecutor executes "crawl" action steps
type CrawlerStepExecutor struct {
    crawlerService interfaces.CrawlerService
    sourceService  *sources.Service
    logger         arbor.ILogger
}

func (e *CrawlerStepExecutor) ExecuteStep(ctx context.Context, step models.JobStep, sources []string, config map[string]interface{}) (string, error) {
    // Parse step config
    crawlConfig := parseCrawlConfig(step.Config)

    // Get source details
    source, err := e.sourceService.GetSource(ctx, sources[0])
    if err != nil {
        return "", err
    }

    // Build seed URLs based on source type
    seedURLs := buildSeedURLs(source, crawlConfig)

    // Start crawl job
    jobID, err := e.crawlerService.StartCrawl(
        string(source.Type),
        crawlConfig.EntityType,
        seedURLs,
        crawlConfig,
        source.ID,
        false, // refreshSource
        nil,   // sourceConfigSnapshot
        nil,   // authSnapshot
        "",    // jobDefinitionID (set by JobExecutor)
    )

    return jobID, err
}

func (e *CrawlerStepExecutor) GetStepType() string {
    return "crawl"
}
```

## Parent-Child Hierarchy

### Database Schema

The existing `crawl_jobs` table already supports hierarchy via `parent_id`:

```sql
CREATE TABLE crawl_jobs (
    id TEXT PRIMARY KEY,
    parent_id TEXT,  -- Links to parent job
    job_type TEXT,   -- parent, crawler_url, pre_validation, post_summary
    ...
)
```

### Hierarchy Tracking

```
Parent Job (job_def_123)
├─ Pre-Job (optional)
├─ Step 1 Job (crawl_abc)
│  ├─ URL Job 1 (seed-1)
│  ├─ URL Job 2 (seed-2)
│  └─ URL Job 3 (discovered link)
├─ Step 2 Job (transform_xyz)
└─ Post-Job (optional)
```

### Status Aggregation

```go
// GetJobTreeStatus aggregates status from parent and all descendants
type JobTreeStatus struct {
    JobID      string
    Name       string
    Status     string // pending, running, completed, failed
    Progress   JobProgress
    Children   []JobTreeStatus
    StartTime  time.Time
    EndTime    *time.Time
}

func (jm *JobManager) GetJobTreeStatus(ctx context.Context, parentID string) (*JobTreeStatus, error) {
    // 1. Get parent job
    parent, err := jm.GetJob(ctx, parentID)

    // 2. Get all children recursively
    children, err := jm.GetChildrenRecursive(ctx, parentID)

    // 3. Aggregate status
    status := aggregateStatus(parent, children)

    return status, nil
}
```

## Sequential Execution Flow

1. User clicks "Execute" on job definition
2. JobExecutor creates parent job record
3. For each step in jobDef.Steps (sequential):
   a. Get StepExecutor for step.Action
   b. Execute step → returns child job ID
   c. Link child to parent in database
   d. Wait for step completion (or continue based on OnError)
4. All steps complete → mark parent as completed
5. UI polls GetJobTreeStatus for real-time updates

## Error Handling

### ErrorTolerance Configuration

```go
type ErrorTolerance struct {
    MaxChildFailures int    // Max child failures before stopping
    FailureAction    string // "stop_all", "continue", "mark_warning"
}
```

### Per-Step Error Strategy

```go
type ErrorStrategy string

const (
    ErrorStrategyFail     ErrorStrategy = "fail"     // Stop entire job
    ErrorStrategyContinue ErrorStrategy = "continue" // Log and continue
    ErrorStrategyRetry    ErrorStrategy = "retry"    // Retry with backoff
)
```

## Implementation Plan

### Phase 1: Core Interfaces (This Session)
- [ ] Create `internal/executor/interfaces.go` with StepExecutor interface
- [ ] Create `internal/executor/job_executor.go` with JobExecutor implementation
- [ ] Create `internal/executor/crawler_step_executor.go`

### Phase 2: Integration
- [ ] Update JobDefinitionHandler to use JobExecutor
- [ ] Register executors in app initialization
- [ ] Test sequential execution

### Phase 3: Status Aggregation
- [ ] Implement GetJobTreeStatus in JobManager
- [ ] Add UI endpoint for job tree status
- [ ] Test parent-child status reporting

### Phase 4: Error Handling & Concurrency
- [ ] Implement error tolerance checks
- [ ] Add concurrent step execution (optional)
- [ ] Implement retry logic

## Benefits

1. **Separation of Concerns**: JobDefinitionHandler doesn't know about CrawlerService
2. **Extensibility**: Easy to add new step types (summarize, transform, etc.)
3. **Testability**: Each component can be tested in isolation
4. **Maintainability**: Clear boundaries and responsibilities

## Migration Path

1. Current: Job definitions temporarily disabled
2. Phase 1: Basic JobExecutor with crawler steps only
3. Phase 2: Full step execution with error handling
4. Phase 3: Advanced features (concurrency, retry, etc.)
