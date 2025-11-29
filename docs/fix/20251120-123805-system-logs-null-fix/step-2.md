# Step 2: Fix handler to use correct log directory path

**Skill:** @go-coder
**Files:** `internal/handlers/system_logs_handler.go`

---

## Iteration 1

### Agent 2 - Implementation

Replacing the handler's custom `readLogFile()` implementation with the arbor service's `GetLogContent()` method, which already has the correct log directory path.

**Changes made:**
- `internal/handlers/system_logs_handler.go`:
  - Removed custom `LogEntry` struct (lines 24-32) - using arbor's logviewer.LogEntry instead
  - Replaced call to custom `readLogFile()` with `h.service.GetLogContent()` at line 76
  - Removed entire custom `readLogFile()` method (lines 98-178) - arbor service provides this
  - Cleaned up unused imports (bufio, fmt, io, os, path/filepath)

**Commands run:**
```bash
go build -o /tmp/quaero ./cmd/quaero
```

**Result:** ✅ Compiles cleanly

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly

**Code Quality:**
✅ Follows Go patterns - delegates to service layer instead of reimplementing
✅ Matches existing code style
✅ Proper error handling maintained
✅ Removed unnecessary code duplication
✅ Clean approach without custom path handling (as requested)
✅ Uses arbor service's existing functionality

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
- Simplified handler by removing 80+ lines of duplicate code
- Now uses arbor service's GetLogContent() which has correct directory path
- Clean solution that follows single responsibility principle

**→ Continuing to Step 3**
