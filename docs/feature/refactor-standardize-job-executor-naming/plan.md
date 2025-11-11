# Plan: Refactor - Standardize Job Executor Naming Convention

## Overview
Standardize the crawler executor naming by removing the "Enhanced" prefix and deleting the stub implementation. This enforces the `{Type}Executor` naming convention used by other executors (`ParentJobExecutor`, `DatabaseMaintenanceExecutor`).

## Current State
- Two crawler executor implementations exist in `internal/jobs/processor/`:
  - `crawler_executor.go` - Stub with TODO comments (110 lines)
  - `enhanced_crawler_executor.go` - Production implementation (1034 lines)
  - `enhanced_crawler_executor_auth.go` - Auth logic (495 lines)
- The "Enhanced" prefix violates naming convention
- The stub was never completed and creates confusion

## Steps

### 1. Delete stub crawler_executor.go
- Skill: @code-architect
- Files: `internal/jobs/processor/crawler_executor.go`
- User decision: no
- Action: Remove the incomplete stub implementation

### 2. Rename enhanced_crawler_executor.go to crawler_executor.go
- Skill: @go-coder
- Files: `internal/jobs/processor/enhanced_crawler_executor.go`
- User decision: no
- Actions:
  - Rename file
  - Rename `EnhancedCrawlerExecutor` → `CrawlerExecutor`
  - Rename `NewEnhancedCrawlerExecutor` → `NewCrawlerExecutor`
  - Update all method receivers
  - Update comments

### 3. Rename enhanced_crawler_executor_auth.go to crawler_executor_auth.go
- Skill: @go-coder
- Files: `internal/jobs/processor/enhanced_crawler_executor_auth.go`
- User decision: no
- Actions:
  - Rename file
  - Update method receiver to `CrawlerExecutor`
  - Update comments

### 4. Update references in app.go
- Skill: @go-coder
- Files: `internal/app/app.go`
- User decision: no
- Actions:
  - Rename variable `enhancedCrawlerExecutor` → `crawlerExecutor`
  - Update constructor call to `processor.NewCrawlerExecutor()`
  - Update registration call
  - Update comments and log messages

### 5. Verify compilation
- Skill: @go-coder
- Files: All modified files
- User decision: no
- Actions:
  - Run `go build -o /tmp/quaero`
  - Verify no compilation errors
  - Confirm all references resolved

### 6. Verify processor.go requires no changes
- Skill: @none
- Files: `internal/jobs/processor/processor.go`
- User decision: no
- Actions:
  - Verify interface-based design supports refactoring
  - Document that no changes needed due to dynamic type resolution

## Success Criteria
- ✅ Stub `crawler_executor.go` deleted
- ✅ Production files renamed without "Enhanced" prefix
- ✅ All type names follow `{Type}Executor` convention
- ✅ All references in `app.go` updated
- ✅ Code compiles cleanly
- ✅ No test failures (no existing tests for this executor)
- ✅ Naming consistent with `ParentJobExecutor` and `DatabaseMaintenanceExecutor`
