# Step 5: Add deprecation notices to old executor files

**Skill:** @go-coder
**Files:** internal/jobs/executor/crawler_step_executor.go, internal/jobs/executor/database_maintenance_step_executor.go, internal/jobs/executor/agent_step_executor.go

---

## Iteration 1

### Agent 2 - Implementation

Added deprecation notices to the 3 old executor files to indicate migration status.

**Changes made:**
- `internal/jobs/executor/crawler_step_executor.go`: Added deprecation comment after package declaration:
  ```go
  // DEPRECATED: This file has been migrated to internal/jobs/manager/crawler_manager.go (ARCH-004).
  // This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
  // New code should import from internal/jobs/manager and use CrawlerManager instead.
  ```

- `internal/jobs/executor/database_maintenance_step_executor.go`: Added deprecation comment after file header:
  ```go
  // DEPRECATED: This file has been migrated to internal/jobs/manager/database_maintenance_manager.go (ARCH-004).
  // This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
  // New code should import from internal/jobs/manager and use DatabaseMaintenanceManager instead.
  ```

- `internal/jobs/executor/agent_step_executor.go`: Added deprecation comment after package declaration:
  ```go
  // DEPRECATED: This file has been migrated to internal/jobs/manager/agent_manager.go (ARCH-004).
  // This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
  // New code should import from internal/jobs/manager and use AgentManager instead.
  ```

**Commands run:**
```bash
go build -o /tmp/test_old_files internal/jobs/executor/crawler_step_executor.go internal/jobs/executor/database_maintenance_step_executor.go internal/jobs/executor/agent_step_executor.go internal/jobs/executor/interfaces.go
```

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly - No errors or warnings from deprecation comments

**Tests:**
⚙️ No tests applicable - Comment additions only

**Code Quality:**
✅ Follows Go patterns - Standard deprecation comment format
✅ Matches existing code style - Consistent placement and wording across all 3 files
✅ Clear migration guidance - Each comment specifies new location and removal timeline
✅ Backward compatibility maintained - No functional changes, files remain operational

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Deprecation notices added to all 3 migrated files. Comments clearly communicate migration status, new location, and removal timeline (ARCH-008). Files remain fully functional for backward compatibility. Developers are guided to use new manager package for new code.

**→ Continuing to Step 6**
