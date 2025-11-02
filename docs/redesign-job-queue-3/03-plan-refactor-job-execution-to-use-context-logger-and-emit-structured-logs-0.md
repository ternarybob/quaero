I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Logging Redundancy Pattern:**
All three job types exhibit the same anti-pattern:
1. Log an event with raw Arbor methods (`logger.Info()`, `logger.Error()`)
2. Immediately log the same event with structured helpers (`LogJobStart()`, `LogJobComplete()`)
3. Result: Duplicate log entries for the same lifecycle event

**Example from CrawlerJob:**
- Lines 99-104: Raw `Info()` logs "Processing crawler URL job"
- Lines 157-161: `LogJobStart()` logs "Job started" with structured fields
- Both represent the same "job start" event

**Missing Structured Logging:**
1. **No LogJobError Usage**: All three jobs use raw `Error()` calls instead of `LogJobError(err, context)`
2. **No LogJobProgress Usage**: CrawlerJob updates progress but doesn't log it with `LogJobProgress()`
3. **Inconsistent Context**: Some logs include URL/depth, others don't

**JobLogger Capabilities (from logger.go):**
- `LogJobStart(name, sourceType, config)` - Includes job_id, name, source_type, config
- `LogJobProgress(completed, total, message)` - Includes job_id, completed, total, progress_pct
- `LogJobComplete(duration, resultCount)` - Includes job_id, duration_sec, result_count
- `LogJobError(err, context)` - Includes job_id, error, context
- `LogJobCancelled(reason)` - Includes job_id, reason

**Key Insight:**
The structured helpers already include `job_id` automatically via CorrelationID. All logs flow through Arbor's context channel to LogService, which extracts the jobID and dispatches to database and WebSocket. The infrastructure is complete - we just need to use it consistently.

## Refactoring Strategy

**Principle: One Event, One Log Entry**
- Each lifecycle event (start, progress, complete, error) should have exactly ONE log entry
- Use structured helpers for lifecycle events
- Use raw Arbor methods ONLY for detailed operational logs (e.g., "Enqueueing child job", "Document loaded")
- Operational logs should add context, not duplicate lifecycle events

**Context Enrichment:**
- CrawlerJob: Add URL and depth to all relevant logs
- SummarizerJob: Add document_id and action to all relevant logs
- CleanupJob: Add age_threshold and status_filter to all relevant logs

**Error Handling Pattern:**
Replace:
```
logger.Error().Err(err).Msg("Failed to X")
return fmt.Errorf("failed to X: %w", err)
```

With:
```
logger.LogJobError(err, "Failed to X")
return fmt.Errorf("failed to X: %w", err)
```

**Progress Logging Pattern (CrawlerJob only):**
Add progress logs when updating job progress:
```
logger.LogJobProgress(job.Progress.CompletedURLs, job.Progress.TotalURLs, "URL processing progress")
```

### Approach

Refactor the three job type files (CrawlerJob, SummarizerJob, CleanupJob) to use JobLogger's structured helpers consistently. Remove redundant raw Arbor logging calls that duplicate structured helper functionality. Add missing structured logging for errors and progress updates. Ensure all logs include relevant context (URL, depth, status, error details) for better observability in the UI.

### Reasoning

I explored the repository structure, read the three job type files (crawler.go, summarizer.go, cleanup.go), examined the BaseJob and JobLogger implementations, and analyzed the current logging patterns. I identified redundant logging where jobs log the same event twice - once with raw Arbor methods and once with structured helpers. I confirmed that JobLogger provides all necessary structured helpers (LogJobStart, LogJobProgress, LogJobComplete, LogJobError, LogJobCancelled) and that all jobs already have access to these methods via the embedded BaseJob.

## Proposed File Changes

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\jobs\types\logger.go
- internal\jobs\types\base.go

**Remove Redundant Job Start Logging (lines 99-104):**
- Delete the raw `Info()` call at lines 99-104 that logs "Processing crawler URL job"
- Keep ONLY the `LogJobStart()` call at lines 157-161
- Rationale: Both log the same "job start" event; structured helper is more consistent

**Enhance LogJobStart Call (lines 157-161):**
- Change the `name` parameter to include URL and depth: `fmt.Sprintf("Crawl URL: %s (depth=%d)", msg.URL, msg.Depth)`
- This provides better context in the UI without needing a separate log entry
- The structured helper already includes job_id, source_type, and config

**Replace Error Logging with LogJobError (lines 109-127):**
- Replace the raw `Warn()` call at line 117 with `LogJobError(saveErr, "Failed to save job with validation error")`
- Replace the raw `Info()` call at lines 119-123 with a single structured log that includes the error context
- Rationale: Errors should use structured helper for consistency and better UI display

**Add Progress Logging (lines 178-188):**
- After updating job progress at line 181-182, add: `c.logger.LogJobProgress(job.Progress.CompletedURLs, job.Progress.TotalURLs, fmt.Sprintf("Processed URL: %s", msg.URL))`
- This provides real-time progress updates that flow to the UI via WebSocket
- Include URL in the message for context

**Remove Redundant Completion Logging (lines 228-233):**
- Delete the raw `Info()` call at lines 228-233 that logs "Crawler URL job completed successfully"
- Keep ONLY the `LogJobComplete()` call at line 226
- Rationale: Both log the same "job complete" event; structured helper is sufficient

**Enhance LogJobComplete Call (line 226):**
- Change `resultCount` parameter from `0` to `1` (one URL processed)
- This provides accurate result count in the structured log

**Replace Error Logging in ExecuteCompletionProbe (lines 248-252):**
- Replace raw `Error()` call at lines 248-252 with `LogJobError(err, fmt.Sprintf("Failed to load parent job: %s", msg.ParentID))`
- Include parent_id in the context for better debugging

**Replace Error Logging in ExecuteCompletionProbe (lines 257-260):**
- Replace raw `Error()` call at lines 257-260 with `LogJobError(fmt.Errorf("parent job is not a CrawlJob"), fmt.Sprintf("Invalid job type for parent: %s", msg.ParentID))`
- Use structured helper for consistency

**Replace Error Logging in ExecuteCompletionProbe (lines 270-274):**
- Replace raw `Error()` call at lines 270-274 with `LogJobError(err, fmt.Sprintf("Failed to update parent job status: %s", msg.ParentID))`
- Use structured helper for consistency

**Keep Operational Logs:**
- Keep the raw `Debug()` call at lines 142-146 (depth limit check) - this is operational detail, not a lifecycle event
- Keep the raw `Info()` call at lines 170-173 (URL processing simulation) - this is operational detail
- Keep the raw `Info()` call at lines 199-202 (enqueueing child jobs) - this is operational detail
- Keep the raw `Warn()` call at lines 217-220 (failed to enqueue child) - this is operational detail, not a job failure
- Keep the raw `Info()` call at lines 277-281 (parent job completion) - this is operational detail

**Rationale for Keeping Operational Logs:**
- These logs provide detailed operational context that supplements lifecycle events
- They don't duplicate lifecycle events
- They help with debugging and understanding job execution flow
- They will still flow through Arbor's context channel with the correct CorrelationID

### internal\jobs\types\summarizer.go(MODIFY)

References: 

- internal\jobs\types\logger.go
- internal\jobs\types\base.go

**Remove Redundant Job Start Logging (lines 37-40):**
- Delete the raw `Info()` call at lines 37-40 that logs "Processing summarizer job"
- Keep ONLY the `LogJobStart()` call at line 54
- Rationale: Both log the same "job start" event; structured helper is more consistent

**Enhance LogJobStart Call (line 54):**
- Change the `name` parameter to include action and document_id: `fmt.Sprintf("Summarize document: %s (action=%s)", documentID, action)`
- Move the `LogJobStart()` call to AFTER extracting documentID (after line 64) so we have the document_id available
- Change `sourceType` parameter from empty string to `"document"` for consistency
- This provides better context in the UI without needing a separate log entry

**Replace Error Logging with LogJobError (lines 69-73):**
- Replace the raw `Error()` call at lines 69-73 with `LogJobError(err, fmt.Sprintf("Failed to load document: %s", documentID))`
- Include document_id in the context for better debugging
- Rationale: Errors should use structured helper for consistency and better UI display

**Remove Redundant Document Loaded Log (lines 76-79):**
- Delete the raw `Info()` call at lines 76-79 that logs "Document loaded"
- This is redundant with the job start log that now includes the document_id
- Rationale: Reduce log noise; document_id is already in the job start log

**Replace Error Logging in summarizeDocument (lines 105-109):**
- Replace the raw `Error()` call at lines 105-109 with `LogJobError(err, fmt.Sprintf("Failed to generate summary for document: %s", document.ID))`
- Include document_id in the context for better debugging

**Remove Redundant Summary Generated Log (lines 112-115):**
- Delete the raw `Info()` call at lines 112-115 that logs "Summary generated"
- This is redundant with the job complete log
- Rationale: The completion log already indicates success; no need for intermediate log

**Remove Redundant Completion Logging (lines 124-126):**
- Delete the raw `Info()` call at lines 124-126 that logs "Summarization completed successfully"
- Keep ONLY the `LogJobComplete()` call at line 122
- Rationale: Both log the same "job complete" event; structured helper is sufficient

**Fix LogJobComplete Duration (line 122):**
- Change `time.Since(time.Now())` to use a proper start time variable
- Add `startTime := time.Now()` at the beginning of `summarizeDocument()` (after line 93)
- Change line 122 to: `s.logger.LogJobComplete(time.Since(startTime), len(summary))`
- Rationale: Current code calculates duration as zero; need to track actual start time

**Remove Redundant Keywords Extracted Log (lines 180-183):**
- Delete the raw `Info()` call at lines 180-183 that logs "Keywords extracted"
- This is redundant with the job complete log
- Rationale: The completion log already indicates success; no need for intermediate log

**Remove Redundant Completion Logging (lines 188-190):**
- Delete the raw `Info()` call at lines 188-190 that logs "Keyword extraction completed successfully"
- Keep ONLY the `LogJobComplete()` call at line 186
- Rationale: Both log the same "job complete" event; structured helper is sufficient

**Fix LogJobComplete Duration (line 186):**
- Change `time.Since(time.Now())` to use a proper start time variable
- Add `startTime := time.Now()` at the beginning of `extractKeywords()` (after line 132)
- Change line 186 to: `s.logger.LogJobComplete(time.Since(startTime), len(keywords))`
- Rationale: Current code calculates duration as zero; need to track actual start time

**Keep Operational Logs:**
- All raw Arbor logs have been removed or replaced with structured helpers
- No operational logs need to be kept in this job type
- The structured helpers provide sufficient context for the UI

### internal\jobs\types\cleanup.go(MODIFY)

References: 

- internal\jobs\types\logger.go
- internal\jobs\types\base.go

**Remove Redundant Job Start Logging (lines 34-36):**
- Delete the raw `Info()` call at lines 34-36 that logs "Processing cleanup job"
- Keep ONLY the `LogJobStart()` call at lines 70-74
- Rationale: Both log the same "job start" event; structured helper is more consistent

**Enhance LogJobStart Call (lines 70-74):**
- Change the `name` parameter to: `fmt.Sprintf("Cleanup jobs (age>%d days, status=%s)", ageThreshold, statusFilter)`
- Change the `sourceType` parameter from `"system"` to `"maintenance"`
- Change the `config` parameter from a formatted string to the actual `msg.Config` map
- This provides better context in the UI and maintains consistency with other job types

**Remove Redundant Criteria Log (lines 76-80):**
- Delete the raw `Info()` call at lines 76-80 that logs "Starting cleanup with criteria"
- This is redundant with the enhanced `LogJobStart()` call
- Rationale: The job start log now includes all criteria; no need for duplicate log

**Remove Redundant Cutoff Time Log (lines 84-86):**
- Delete the raw `Info()` call at lines 84-86 that logs "Cleanup cutoff time calculated"
- This is operational detail that doesn't need to be logged
- Rationale: The cutoff time is implicit from the age threshold; reduces log noise

**Remove Redundant Query Log (lines 97-100):**
- Delete the raw `Info()` call at lines 97-100 that logs "Querying jobs for cleanup"
- This is operational detail that doesn't need to be logged
- Rationale: The job start log already indicates cleanup is starting; reduces log noise

**Replace Error Logging with LogJobError (lines 115-119):**
- Replace the raw `Error()` call at lines 115-119 with `LogJobError(err, fmt.Sprintf("Failed to list jobs with status: %s", status))`
- Include status in the context for better debugging
- Rationale: Errors should use structured helper for consistency and better UI display

**Add Progress Logging (after line 150):**
- After the inner loop that collects jobs to clean (after line 150), add: `c.logger.LogJobProgress(len(jobsToClean), len(jobsToClean), fmt.Sprintf("Found %d jobs to clean", len(jobsToClean)))`
- This provides progress update after job collection phase
- Rationale: Gives users visibility into cleanup progress

**Remove Redundant Jobs Found Log (lines 152-155):**
- Delete the raw `Info()` call at lines 152-155 that logs "Jobs identified for cleanup"
- This is now covered by the progress log added above
- Rationale: Progress log provides the same information in structured format

**Remove Redundant Deletion Start Log (lines 162-164):**
- Delete the raw `Info()` call at lines 162-164 that logs "Starting job deletion"
- This is operational detail that doesn't need to be logged
- Rationale: The progress logs will show deletion progress; reduces log noise

**Replace Error Logging with LogJobError (lines 169-173):**
- Replace the raw `Error()` call at lines 169-173 with `LogJobError(err, fmt.Sprintf("Failed to delete job: %s", jobID))`
- Include job_id in the context for better debugging
- Rationale: Errors should use structured helper for consistency and better UI display

**Enhance Progress Logging (lines 180-184):**
- Keep the progress log at lines 180-184 but change it to use `LogJobProgress()`
- Replace with: `c.logger.LogJobProgress(i+1, len(jobsToClean), fmt.Sprintf("Deleted %d/%d jobs", i+1, len(jobsToClean)))`
- This provides structured progress updates every 10 deletions
- Rationale: Structured progress logs flow to UI via WebSocket for real-time updates

**Remove Redundant Deletion Complete Log (lines 188-190):**
- Delete the raw `Info()` call at lines 188-190 that logs "Job deletion completed"
- This is redundant with the job complete log
- Rationale: The completion log already indicates success; no need for intermediate log

**Remove Redundant Dry Run Log (lines 192-194):**
- Delete the raw `Info()` call at lines 192-194 that logs "Dry run mode - no actual deletion performed"
- Add this information to the job complete log instead
- Rationale: Consolidate completion information into structured log

**Fix LogJobComplete Duration (lines 198-201):**
- Change the duration calculation from `time.Since(time.Now().Add(-time.Duration(ageThreshold) * 24 * time.Hour))` to use a proper start time variable
- Add `startTime := time.Now()` at the beginning of `Execute()` (after line 33)
- Change lines 198-201 to: `c.logger.LogJobComplete(time.Since(startTime), jobsDeleted)`
- Rationale: Current code calculates incorrect duration; need to track actual start time

**Remove Redundant Completion Logging (lines 203-208):**
- Delete the raw `Info()` call at lines 203-208 that logs "Cleanup job completed successfully"
- Keep ONLY the `LogJobComplete()` call at lines 198-201
- Rationale: Both log the same "job complete" event; structured helper is sufficient

**Keep Operational Logs:**
- Keep the raw `Warn()` call at lines 52-56 (age threshold enforcement) - this is important operational detail for safety
- This log warns users when their requested age threshold is too low and has been adjusted
- Rationale: Safety-related operational detail that should be visible to users