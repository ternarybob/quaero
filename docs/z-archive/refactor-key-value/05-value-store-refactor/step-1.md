# Step 1: Create KeysDirConfig struct

**Skill:** @go-coder
**Files:** `internal/common/keys_config.go` (new)

---

## Iteration 1

### Agent 2 - Implementation
Created new configuration struct for key/value directory settings. This struct is separate from `AuthDirConfig` to maintain clean separation between authentication concerns (cookies for web scraping) and generic key/value storage.

**Changes made:**
- `internal/common/keys_config.go`: Created new file with `KeysDirConfig` struct
  - Single field: `Dir string` with TOML tag for unmarshaling
  - Package-level documentation explaining purpose and separation from auth
  - Field documentation with TOML format details and default value
  - Follows same pattern as `AuthDirConfig`

**Commands run:**
```bash
go build ./internal/common/...
```

### Agent 3 - Validation
**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ No tests applicable (config struct only, will be tested via integration)

**Code Quality:**
✅ Follows Go patterns (struct with TOML tags)
✅ Matches existing code style (mirrors `AuthDirConfig`)
✅ Proper documentation comments
✅ Clear separation of concerns documented
✅ Simple, focused struct definition

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Clean, simple config struct following established patterns. Ready for integration in main Config struct.

**→ Continuing to Step 2**
