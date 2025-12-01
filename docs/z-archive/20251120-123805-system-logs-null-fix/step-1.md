# Step 1: Investigate arbor service log directory configuration

**Skill:** @none
**Files:** Arbor service logviewer package

---

## Iteration 1

### Agent 2 - Implementation

Investigated the arbor logviewer service to understand how it handles log directory paths.

**Findings:**

1. **Arbor Service Configuration (service.go:18-42):**
   - Service has `LogDirectory` field that stores the correct path
   - Initialized from `WriterConfiguration.FileName` via `filepath.Dir()`
   - Has `GetLogContent()` method that uses `s.LogDirectory` correctly (line 87)

2. **Handler Initialization (app.go:351-371):**
   - `logsDir` calculated as `filepath.Join(filepath.Dir(execPath), "logs")`
   - Which resolves to `bin/logs/` when running from bin
   - Service properly initialized with this directory

3. **Problem in Handler (system_logs_handler.go:86-95):**
   - Handler's `GetLogContentHandler` calls custom `readLogFile()` instead of service method
   - Custom `readLogFile()` hardcodes `filepath.Join("logs", filename)` at line 102
   - This bypasses the arbor service's LogDirectory configuration

**Root Cause:**
The handler implements its own file reading logic instead of using the arbor service's `GetLogContent()` method, which has the correct directory path.

**Solution:**
Replace handler's custom `readLogFile()` with the arbor service's `GetLogContent()` method.

### Agent 3 - Validation

**Investigation Quality:**
✅ Correctly identified the root cause
✅ Found the arbor service already has the needed functionality
✅ Identified clean solution without custom code

**Quality Score:** 10/10

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
The arbor service already provides the correct functionality. The handler just needs to use it instead of reimplementing file reading with hardcoded paths.

**→ Continuing to Step 2**
