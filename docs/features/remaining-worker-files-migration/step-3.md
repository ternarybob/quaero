# Step 3: Update App Registration

**Skill:** @go-coder
**Files:** `internal/app/app.go`

---

## Iteration 1

### Agent 2 - Implementation

Updated app.go to use worker package for JobProcessor and AgentWorker.

**Changes made:**
- `internal/app/app.go` - Updated 3 locations:
  - Line 67: Field declaration `*processor.JobProcessor` → `*worker.JobProcessor`
  - Line 271: Constructor call `processor.NewJobProcessor()` → `worker.NewJobProcessor()`
  - Line 323: Constructor call `processor.NewAgentExecutor()` → `worker.NewAgentWorker()`
  - Variable renamed: `agentExecutor` → `agentWorker`
  - Comment updated: "Register agent executor" → "Register agent worker"
  - Import for worker package already exists (added in ARCH-005)
  - ParentJobExecutor unchanged (line 314 - migrates in ARCH-007)

**Commands run:**
```bash
# Build full application
powershell -File scripts/build.ps1
# ✅ Build successful
# Version: 0.1.1969, Build: 11-11-17-55-12
# Both quaero.exe and quaero-mcp.exe generated
```

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ Application builds successfully
✅ Both executables generated (quaero.exe + quaero-mcp.exe)

**Tests:**
⚙️ Not applicable - Build verification only, full testing in Step 6

**Code Quality:**
✅ Import already exists - worker package import added in ARCH-005
✅ Field type updated correctly - JobProcessor type changed to worker package
✅ Constructor calls updated - Both NewJobProcessor() and NewAgentWorker() use worker package
✅ Variable naming consistent - agentExecutor renamed to agentWorker
✅ Comments updated - Reflects worker terminology
✅ ParentJobExecutor unchanged - Remains in processor package (migrates in ARCH-007)

**Quality Score:** 10/10

**Issues Found:**
None - All updates applied successfully, application builds cleanly

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
App registration updated successfully. All three locations changed to use worker package (field declaration, JobProcessor initialization, AgentWorker registration). Application builds successfully with both executables generated.

**→ Continuing to Step 4**
