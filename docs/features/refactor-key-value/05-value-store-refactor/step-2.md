# Step 2: Update main Config to support keys directory

**Skill:** @go-coder
**Files:** `internal/common/config.go`

---

## Iteration 1

### Agent 2 - Implementation
Updated main Config struct to add support for keys directory configuration. Added new Keys field, default value, and environment variable override following the same pattern as Auth configuration.

**Changes made:**
- `internal/common/config.go`:
  - Line 28: Added `Keys KeysDirConfig` field to Config struct with TOML tag and comment
  - Lines 223-225: Added default initialization in `NewDefaultConfig()` with `Dir: "./keys"`
  - Lines 616-619: Added environment variable override `QUAERO_KEYS_DIR` in `applyEnvOverrides()`
  - All changes follow the exact pattern of `Auth AuthDirConfig`

**Commands run:**
```bash
go build ./internal/common/...
```

### Agent 3 - Validation
**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ No tests applicable (config additions will be tested via integration in Step 5)

**Code Quality:**
✅ Follows Go patterns (struct field with TOML tag)
✅ Matches existing code style (mirrors Auth field pattern exactly)
✅ Proper placement in struct (after Auth, before Crawler)
✅ Default value matches requirement (`./keys`)
✅ Environment variable follows naming convention (`QUAERO_KEYS_DIR`)
✅ Backward compatible (new field is optional with default)

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Config successfully updated with Keys field. All patterns match existing Auth configuration. Ready for loader implementation.

**→ Continuing to Step 3**
