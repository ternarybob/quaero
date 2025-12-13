# Step 2: Fix bin/keys/example-keys.toml format

**Skill:** @none
**Files:** `bin/keys/example-keys.toml`

---

## Iteration 1

### Agent 2 - Implementation

Fixed the example keys file to use the correct format expected by the key/value loader (`internal/storage/sqlite/load_keys.go:159`).

**Problem:** The file used a legacy format with `api_key` and `service_type` fields, but the loader expects `value` and optional `description` fields.

**Changes made:**
- `bin/keys/example-keys.toml:2`: Changed `api_key = "..."` to `value = "..."`
- `bin/keys/example-keys.toml:3`: Removed `service_type = "google-places"` line (not used by KV store)
- `bin/keys/example-keys.toml:4`: Kept `description` field (optional, supported)

**Before:**
```toml
[google-places-key]
api_key = "AIzaSyCwXVa0E5aCDmCg9FlhPeX8ct83E9EADFg"
service_type = "google-places"
description = "Google Places API key for location search functionality"
```

**After:**
```toml
[google-places-key]
value = "AIzaSyCwXVa0E5aCDmCg9FlhPeX8ct83E9EADFg"
description = "Google Places API key for location search functionality"
```

**Commands run:**
```bash
go build -o /tmp/quaero-test ./cmd/quaero
```

**Result:** Compilation successful

### Agent 3 - Validation

**Skill:** @none

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ No tests for TOML file format (integration test in Step 4)

**Code Quality:**
✅ Matches expected format in `load_keys.go:34-36`
✅ Required field `value` is present (validated at line 159)
✅ Optional field `description` is present (line 162)
✅ Removed unused `service_type` field
✅ Clean, minimal format

**Quality Score:** 10/10

**Issues Found:**
None. The file now matches the loader's expected format and will be successfully loaded at startup.

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
The example keys file now uses the correct format. When the service starts, the loader will:
1. Read `bin/keys/example-keys.toml`
2. Parse the `[google-places-key]` section
3. Validate that `value` field is present (line 159)
4. Store the key/value pair in the KV store using `Set()` (line 104)
5. Log success: "Loaded key/value pair from file" (line 113)

This fixes the log warning: `WRN > error=value is required Key/value validation failed`

**→ Continuing to Step 3**
