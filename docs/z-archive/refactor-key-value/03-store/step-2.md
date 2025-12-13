# Step 2: Update Auth Credentials Loader

**Skill:** @go-coder
**Files:**
- `internal/storage/sqlite/load_auth_credentials.go`
- `internal/storage/sqlite/manager.go`
- `internal/models/auth.go`

---

## Iteration 1

### Agent 2 - Implementation

Updating `LoadAuthCredentialsFromFiles()` to detect API key type and load API keys into KV store instead of auth_credentials table. Cookie-based auth remains unchanged.

**Implementation details:**
- Detect API key credentials by checking `auth_type='api_key'` or presence of `api_key` field
- Load API keys into KV store using `Set()` method
- Keep cookie-based auth loading to auth_credentials unchanged
- Maintain backward compatibility with existing TOML format

**Commands run:**
```bash
go build -o bin/quaero.exe ./cmd/quaero
```

**Changes:**
1. Modified `LoadAuthCredentialsFromFiles()` in `internal/storage/sqlite/load_auth_credentials.go:102-125`
   - Changed from storing API keys in auth_credentials table
   - Now stores API keys in KV store using `m.kv.Set()`
   - Uses section name as key, api_key value as value
   - Creates description from file metadata or defaults to "API key for {service_type}"
   - Updated log message to indicate KV store storage
2. Removed conversion to AuthCredentials model (no longer needed)

**Build result:** ✅ Success

---

## Iteration 2 - Validation

### Agent 3 - Validator

**Validation steps:**
1. ✅ Code compiles successfully
2. ✅ API keys now loaded into KV store instead of auth_credentials
3. ✅ Backward compatibility maintained (TOML format unchanged)
4. ✅ Idempotent operation (KV store uses UPSERT)
5. ✅ Proper error handling and logging
6. ✅ API key masking still applied for safe logging

**Status:** APPROVED - Step 2 complete
