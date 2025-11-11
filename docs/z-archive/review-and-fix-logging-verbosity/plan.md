---
task: "Review and fix excessive INFO/WARNING logging throughout the codebase"
complexity: medium
steps: 8
---

# Review and Fix Excessive INFO/WARNING Logging

## Problem Statement

The service currently has too many INFO and WARNING logs, with duplicates and non-essential information logged at inappropriate levels. Many logs that represent internal processing details are logged at INFO level when they should be DEBUG. Similarly, warnings are logged for normal operational conditions rather than actual business rule violations.

**Key Issues Identified:**
- Cookie injection process logs extensively at INFO level (should be DEBUG)
- Network/authentication diagnostics logged as WARNING during normal operation
- Process/debug information logged as INFO rather than DEBUG
- Duplicate log messages in different contexts

**Recent Context:**
- Enhanced crawler executor files recently had DIAGNOSTIC logs reduced to DEBUG level
- Cookie injection added for debugging auth issues, but normal auth absence shouldn't warn
- Pages can be crawled without auth - cookie injection failures are not warnings

## Logging Level Guidelines

### INFO Level - Key User-Facing Events
- Job started/completed/failed/cancelled
- Document saved (high-level summary)
- Service started/stopped
- Major configuration changes
- Authentication credentials loaded successfully

### WARNING Level - Actual Business Rule Violations
- Configuration missing but required
- Expected data not found (when it should exist)
- Non-critical errors that don't stop operation
- Deprecated API usage
- Resource limits approaching

### DEBUG Level - Internal Processing
- Detailed step-by-step execution flow
- Cookie injection diagnostics
- Network request details
- Authentication token extraction
- Browser pool operations
- Database query details
- Event publishing confirmations

### ERROR Level - Failures (No Changes)
- Operations that failed and stop execution
- Data corruption or integrity issues
- Critical service failures

## Implementation Plan

### Step 1: Fix Cookie Injection Auth Logging
**Why:** These are the most verbose and represent internal diagnostics, not user-facing info
**Depends:** none
**Validates:** code_compiles, follows_conventions
**Files:**
- `C:\development\quaero\internal\jobs\processor\enhanced_crawler_executor_auth.go`
**Risk:** low
**User decision required:** no

**Changes:**
- Lines 25-28, 86-88, 113-116, 134-150, 165-168: Change INFO to DEBUG (cookie injection process steps)
- Lines 32, 95-96, 120-121, 129, 161: Keep as DEBUG or WARN appropriately
- Line 90-93: Change WARN to DEBUG (auth_id not found is normal for non-auth jobs)
- Line 124: Change WARN to DEBUG (job_definition_id not in metadata is normal)
- Lines 227-232, 237-240, 593-595, 604-606: Change WARN to DEBUG (domain mismatch and no cookies are diagnostic info)
- Line 711-716: Change WARN to DEBUG (network request failed during normal operation)
- Lines 466-470: Change WARN to DEBUG (unexpected cookies are diagnostic)
- Lines 784-789: Keep WARN (cookies cleared during navigation is concerning)

### Step 2: Fix Enhanced Crawler Executor Logging
**Why:** Reduce INFO logs for internal processing steps
**Depends:** Step 1
**Validates:** code_compiles, follows_conventions
**Files:**
- `C:\development\quaero\internal\jobs\processor\enhanced_crawler_executor.go`
**Risk:** low
**User decision required:** no

**Changes:**
- Lines 134-143: Keep INFO (job start is user-facing)
- Lines 166, 190-195: Change INFO to DEBUG (browser instance creation is internal)
- Lines 225-231: Keep INFO (page rendering success is key milestone)
- Lines 256-262: Keep INFO (content processing success is key milestone)
- Lines 306-310: Keep INFO (document saved is key milestone)
- Lines 400-404: Keep INFO (child jobs spawned is key milestone)
- Lines 460-464: Keep INFO (job completion is user-facing)

### Step 3: Fix Auth Service Logging
**Why:** Reduce diagnostic noise during normal operation
**Depends:** Step 2
**Validates:** code_compiles, follows_conventions
**Files:**
- `C:\development\quaero\internal\services\auth\service.go`
**Risk:** low
**User decision required:** no

**Changes:**
- Lines 109, 116, 119: Change INFO/WARN to DEBUG (token extraction is internal diagnostic)

### Step 4: Fix Crawler Service Logging
**Why:** Too many INFO logs during startup and configuration
**Depends:** Step 3
**Validates:** code_compiles, follows_conventions
**Files:**
- `C:\development\quaero\internal\services\crawler\service.go`
**Risk:** low
**User decision required:** no

**Changes:**
- Lines 191-194, 196-198: Keep INFO (service startup is user-facing)
- Lines 260-263: Change INFO to DEBUG (loading auth credentials is internal)
- Lines 364: Change INFO to DEBUG (source type logging is diagnostic)
- Lines 454, 470: Change INFO to DEBUG (auth snapshot details are diagnostic)
- Lines 465-466: Change logger.Debug to logger.Debug (already correct)

### Step 5: Fix Parent Job Executor Logging
**Why:** Reduce noise during job monitoring
**Depends:** Step 4
**Validates:** code_compiles, follows_conventions
**Files:**
- `C:\development\quaero\internal\jobs\processor\parent_job_executor.go`
**Risk:** low
**User decision required:** no

**Changes:**
- Line 69-71: Keep INFO (parent job monitoring start is user-facing)
- Lines 108-112: Keep INFO (parent job execution start is user-facing)
- Line 402: Keep INFO (subscription confirmation is configuration info)

### Step 6: Fix Job Handler Logging
**Why:** Too many DEBUG logs in HTTP handlers that aren't needed
**Depends:** Step 5
**Validates:** code_compiles, follows_conventions
**Files:**
- `C:\development\quaero\internal\handlers\job_handler.go`
**Risk:** low
**User decision required:** no

**Changes:**
- Lines 114, 316, 461, 496, 667-693: Review and keep as-is (error handling logs)
- Lines 152-161: Keep DEBUG (child statistics are diagnostic)
- Lines 745, 779, 842, 929-932, 951-955: Keep INFO (job operations are user-facing)

### Step 7: Fix Other Service Logging
**Why:** Clean up remaining verbose logging across services
**Depends:** Step 6
**Validates:** code_compiles, follows_conventions
**Files:**
- `C:\development\quaero\internal\services\scheduler\scheduler_service.go`
- `C:\development\quaero\internal\services\llm\offline\llama.go`
- `C:\development\quaero\internal\storage\sqlite\connection.go`
- `C:\development\quaero\internal\storage\sqlite\job_definition_storage.go`
**Risk:** low
**User decision required:** no

**Changes:**
- Review each file's INFO/WARN usage
- Downgrade process/diagnostic logs to DEBUG
- Keep configuration/startup INFO logs
- Keep business rule violation WARNs

### Step 8: Test and Validation
**Why:** Ensure changes compile and logging behavior is correct
**Depends:** Step 7
**Validates:** code_compiles, tests_pass
**Files:** All modified files
**Risk:** low
**User decision required:** no

**Actions:**
1. Build application using `.\scripts\build.ps1`
2. Run sample crawl job and review logs
3. Verify INFO logs show only key milestones
4. Verify DEBUG logs show detailed diagnostics
5. Verify WARNING logs show only actual issues
6. Confirm no duplicate messages

## User Decision Points

None - all changes follow established logging conventions and guidelines from CLAUDE.md

## Constraints

- Do not change ERROR level logs
- Keep INFO for user-facing key events (job started, job completed, document saved)
- Keep WARNING for actual business rule violations or unexpected states
- Downgrade process/debug logs to DEBUG level
- Ensure consistent logging conventions across codebase
- Maintain structured logging with arbor logger (Str, Int, Bool fields)

## Success Criteria

- All diagnostic/process logs at DEBUG level
- INFO logs only for key user-facing events
- WARNING logs only for actual issues
- Code compiles successfully
- No duplicate log messages
- Logging follows conventions in CLAUDE.md:
  - Use `github.com/ternarybob/arbor` for all logging
  - Structured logging with fields: `logger.Info().Str("field", value).Msg("Message")`
  - No `fmt.Println()` or `log.Printf()` in production code

## Estimated Impact

**Before Changes:**
- ~427 INFO/WARN log statements across 63 files
- Heavy INFO logging during cookie injection (18 INFO statements in auth file)
- Heavy WARN logging for normal operational conditions (9 WARN in auth file)
- Duplicate messages in event publishing and job logs

**After Changes:**
- ~60-70% reduction in INFO logs during normal operation
- ~80% reduction in WARN logs during normal operation
- Clear separation: INFO = user milestones, DEBUG = diagnostics, WARN = actual issues
- Cleaner console output focusing on job progress, not internal details

## References

- **CLAUDE.md Logging Guidelines:** All logging must use `github.com/ternarybob/arbor`
- **Recent Changes:** Enhanced crawler executors recently updated diagnostic logs to DEBUG
- **Cookie Injection Context:** Added for debugging - normal operation shouldn't warn about missing auth
