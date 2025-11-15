# Step 1: Validate and clean up deployments/local/quaero.toml

**Skill:** @none
**Files:** `deployments/local/quaero.toml`

---

## Iteration 1

### Agent 2 - Implementation

Reviewed the template TOML file and identified a documentation issue. The original documentation confused two separate systems:
1. `[auth]` section: Cookie-based authentication credentials (for browser tokens)
2. `[keys]` section: Generic key/value pairs (for API keys, secrets, configuration)

The original documentation (lines 118-135) mixed these concepts, describing "API keys" under the `[auth]` section, which was incorrect.

**Changes made:**
- `deployments/local/quaero.toml:115-149`: Separated and clarified two distinct sections:
  - `[auth]` section (lines 115-125): Cookie-based authentication storage
  - `[keys]` section (lines 127-149): Key/value storage with correct TOML format documentation

**Documentation improvements:**
1. Clear separation between authentication (cookies) and key/value storage
2. Documented correct TOML format for keys:
   ```toml
   [key-name]
   value = "your-secret-value"      # Required
   description = "Optional description"  # Optional
   ```
3. Correct UI reference: `/settings?a=auth-apikeys` (not `/auth`)
4. Clear example file reference: `bin/keys/example-keys.toml`
5. Proper environment variable: `QUAERO_KEYS_DIR`

**No compilation needed** - documentation file only.

### Agent 3 - Validation

**Skill:** @none

**Compilation:**
⚙️ Not applicable (TOML configuration file)

**Tests:**
⚙️ Not applicable (documentation change)

**Code Quality:**
✅ Clear separation of concerns (auth vs keys)
✅ Accurate documentation matching code implementation
✅ Correct TOML format documented
✅ Proper defaults and environment variables listed
✅ User-friendly examples and references

**Quality Score:** 10/10

**Issues Found:**
None. The documentation is now accurate and clearly distinguishes between:
- Cookie-based authentication (`[auth]` section, `./auth` directory)
- Key/value storage (`[keys]` section, `./keys` directory)

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
The configuration template now correctly documents the two distinct storage systems. The `[keys]` section documentation matches the loader implementation in `load_keys.go` and will help users understand the correct format for their key/value files.

**→ Continuing to Step 2**
