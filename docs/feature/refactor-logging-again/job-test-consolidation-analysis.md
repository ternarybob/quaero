# ARCHITECT ANALYSIS: Job Test File Consolidation

## Task
Review and combine the following test files into `job_core_test.go`:
- `test/ui/job_core_test.go` - Core job page tests
- `test/ui/job_framework_test.go` - UITestContext framework
- `test/ui/job_logging_improvements_test.go` - Job logging UI features
- `test/ui/job_trigger_test.go` - Job trigger/cancel tests
- `test/ui/job_types_test.go` - Specific job type tests
- `test/ui/local_dir_jobs_test.go` - Local dir job tests

**CONSTRAINT:** Only test job PAGE functionality, NOT job execution.

---

## Analysis Summary

| File | Lines | Tests | Category |
|------|-------|-------|----------|
| `job_core_test.go` | 152 | 4 | **PAGE UI** (keep) |
| `job_framework_test.go` | 593 | 0 | **FRAMEWORK** (keep separate) |
| `job_logging_improvements_test.go` | 534 | 1 | **JOB EXECUTION** (delete) |
| `job_trigger_test.go` | 160 | 2 | **JOB EXECUTION** (delete) |
| `job_types_test.go` | 151 | 4 | **JOB EXECUTION** (delete) |
| `local_dir_jobs_test.go` | 852 | 5 | **JOB EXECUTION** (delete) |

---

## Classification

### KEEP: Page UI Tests (job_core_test.go)
These test page functionality WITHOUT running actual jobs:

1. `TestJobRelatedPagesLoad` - Verifies pages load (Jobs, Queue, Documents, Settings)
2. `TestJobsPageShowsJobs` - Verifies job cards display on Jobs page
3. `TestQueuePageShowsQueue` - Verifies jobList component exists on Queue page
4. `TestNavigationBetweenPages` - Verifies nav links work

### KEEP: Framework (job_framework_test.go)
This is NOT a test file - it's shared infrastructure:
- `UITestContext` struct and methods
- `TriggerJob`, `MonitorJob` helpers
- `JobDefinitionTestConfig`
- Used by ALL other test files - CANNOT be merged

### DELETE: Job Execution Tests

**`job_logging_improvements_test.go`** - Tests actual job execution:
- `TestJobLoggingImprovements` - Creates and RUNS a local_dir job
- Monitors job logs while job is RUNNING

**`job_trigger_test.go`** - Tests actual job execution:
- `TestJobTrigger` - RUNS "News Crawler" job
- `TestJobCancel` - RUNS and CANCELS a job

**`job_types_test.go`** - Tests actual job execution:
- `TestPlacesJob` - RUNS Google Places API job
- `TestNewsCrawlerJob` - RUNS News Crawler job
- `TestKeywordExtractionJob` - RUNS agent job
- `TestMultiStepJob` - RUNS multi-step job

**`local_dir_jobs_test.go`** - Tests actual job execution:
- `TestLocalDirJobAddPage` - OK (page test)
- `TestLocalDirJobExecution` - RUNS local_dir job
- `TestLocalDirJobWithEmptyDirectory` - RUNS job
- `TestSummaryAgentWithDependency` - RUNS multi-step job
- `TestSummaryAgentPlainRequest` - RUNS two jobs

---

## Recommendation

### Action 1: DELETE job execution test files
The following files test JOB EXECUTION, not page UI:
- `job_logging_improvements_test.go` - DELETE
- `job_trigger_test.go` - DELETE
- `job_types_test.go` - DELETE
- `local_dir_jobs_test.go` - DELETE (but extract one test)

### Action 2: KEEP job_framework_test.go SEPARATE
This is shared infrastructure used by all tests. Cannot be merged.

### Action 3: EXTEND job_core_test.go with one test
Move `TestLocalDirJobAddPage` from `local_dir_jobs_test.go` to `job_core_test.go` since it tests PAGE UI, not job execution.

### Action 4: Simplify job_core_test.go
The `getCurrentURL` helper method (lines 147-151) should stay since it's a utility.

---

## Final Structure

After consolidation:

| File | Purpose |
|------|---------|
| `job_framework_test.go` | Shared UITestContext framework (unchanged) |
| `job_core_test.go` | All job PAGE UI tests |

Tests in `job_core_test.go`:
1. `TestJobRelatedPagesLoad` - Pages load correctly
2. `TestJobsPageShowsJobs` - Job cards display
3. `TestQueuePageShowsQueue` - Queue component exists
4. `TestNavigationBetweenPages` - Navigation works
5. `TestJobAddPage` - Job add page has TOML editor (moved from local_dir_jobs_test.go)

---

## Files to Delete

```
test/ui/job_logging_improvements_test.go
test/ui/job_trigger_test.go
test/ui/job_types_test.go
test/ui/local_dir_jobs_test.go
```

These test job EXECUTION which is out of scope for "page functionality" tests.
