---
task: "Propagate auth_id to parent job metadata for child job cookie injection"
complexity: low
steps: 3
---

# Plan

## Step 1: Add UpdateJobMetadata method to JobManager
**Why:** Create persistence layer for job metadata following existing UpdateJobConfig pattern. This enables auth_id to be saved to database's metadata_json column.

**Depends:** none

**Validates:**
- code_compiles
- follows_conventions

**Files:**
- internal/jobs/manager.go

**Risk:** low

**User decision required:** no

**Implementation:**
- Add new method after UpdateJobConfig (after line 683)
- Method signature: `func (m *Manager) UpdateJobMetadata(ctx context.Context, jobID string, metadata map[string]interface{}) error`
- Marshal metadata to JSON using json.Marshal
- Execute SQL: `UPDATE jobs SET metadata_json = ? WHERE id = ?`
- Follow exact pattern of UpdateJobConfig for consistency

## Step 2: Call UpdateJobMetadata in JobExecutor
**Why:** Persist auth_id from parentMetadata to database so child jobs can retrieve it via GetJob. This fixes the broken flow where auth_id was only in memory.

**Depends:** Step 1

**Validates:**
- code_compiles
- follows_conventions

**Files:**
- internal/jobs/executor/job_executor.go

**Risk:** low

**User decision required:** no

**Implementation:**
- After line 324 (after parentMetadata is populated)
- Before line 326 (before creating parentJobModel)
- Call: `e.jobManager.UpdateJobMetadata(ctx, parentJobID, parentMetadata)`
- Log warning on error (non-fatal - job continues)
- Log debug on success with metadata_keys count

**Error Handling:**
- Non-fatal warning - job execution continues
- Fallback to job_definition_id lookup still works
- Maintains backward compatibility

## Step 3: Build and verify compilation
**Why:** Ensure all changes compile correctly and no syntax errors introduced.

**Depends:** Steps 1, 2

**Validates:**
- code_compiles
- use_build_script

**Files:**
- All modified files

**Risk:** low

**User decision required:** no

**Implementation:**
- Use `./scripts/build.ps1` for final build
- Verify both quaero.exe and quaero-mcp.exe build successfully
- No test execution required (behavioral changes verified through existing diagnostic logs)

## User Decision Points
None - implementation is straightforward following existing patterns.

## Constraints
- Must follow exact pattern of UpdateJobConfig (lines 672-683) for consistency
- Error handling must be non-fatal to maintain backward compatibility
- Placement between lines 324-326 is critical for correct timing
- No breaking changes to existing code

## Success Criteria
- UpdateJobMetadata method added to JobManager
- UpdateJobMetadata called in JobExecutor with proper error handling
- Code compiles successfully with build script
- Both binaries created (quaero.exe, quaero-mcp.exe)
- auth_id will be persisted to database for child job retrieval
