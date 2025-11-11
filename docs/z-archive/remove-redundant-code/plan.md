---
task: "Remove redundant and unnecessary code from codebase"
folder: remove-redundant-code
complexity: low
estimated_steps: 4
---

# Implementation Plan: Remove Redundant Code

## Executive Summary

A systematic scan of the codebase has identified multiple categories of redundant and unnecessary code:

1. **Empty/Stub Files** - Files that exist but contain no functional code
2. **Unused Services** - Complete service implementations that are created but never used
3. **Deprecated Files** - Files explicitly marked as moved/deprecated with redirect comments

**Total Impact:**
- 3 files to remove completely
- 1 service package to remove (internal/services/config/)
- 1 interface to remove (ConfigService)
- ~150 lines of dead code eliminated
- Simplified dependency graph in app initialization

## Current State Analysis

### Category 1: Empty/Stub Files (CONFIRMED)

**File:** `internal/common/log_consumer.go`
- **Size:** 3 lines
- **Content:** Redirect comment only
- **Status:** Explicitly marked as moved to `internal/logs/consumer.go`
- **Current imports:** 0 (grep confirms no imports)
- **Action:** SAFE TO DELETE

```go
// This file has been moved to internal/logs/consumer.go
// Kept here temporarily to avoid breaking imports during refactor
package common
```

### Category 2: Unused Service - ConfigService (CONFIRMED)

**Package:** `internal/services/config/`
**File:** `internal/services/config/service.go` (76 lines)

**Evidence of Non-Usage:**
1. Created in `app.New()` at line 106:
   ```go
   configService := config.NewService(cfg)
   app.ConfigService = configService
   ```

2. NEVER accessed via `app.ConfigService.` (grep found 0 matches)

3. App struct has BOTH fields (line 45-46):
   ```go
   Config         *common.Config           // Deprecated: Use ConfigService instead
   ConfigService  interfaces.ConfigService  // ← NEVER USED
   ```

4. All actual config access uses `app.Config.` directly (4 occurrences found)

**Why it exists:** Appears to be an incomplete refactoring attempt to introduce interface-based config access, but was never completed.

**Related files to remove:**
- `internal/services/config/service.go` (implementation)
- `internal/interfaces/config_service.go` (interface definition - 33 lines)

### Category 3: Version Duplication (POTENTIAL)

**Files:**
- `cmd/quaero/version.go` (17 lines) - CLI command for version display
- `internal/common/version.go` (59 lines) - Version data and utilities

**Analysis:** These serve DIFFERENT purposes:
- `cmd/quaero/version.go` - Cobra CLI command implementation
- `internal/common/version.go` - Version data storage and utilities (GetVersion, LoadVersionFromFile, etc.)

**Decision:** KEEP BOTH - Not duplicates, complementary functionality

### Category 4: Document Services (ANALYSIS)

**Files:**
- `internal/services/documents/document_service.go` (210 lines)
- `internal/services/mcp/document_service.go` (765 lines)

**Analysis:** These are NOT duplicates:
- `documents/document_service.go` - Core document CRUD operations
- `mcp/document_service.go` - MCP protocol adapter for document operations (exposes documents via MCP API)

**Decision:** KEEP BOTH - Different architectural layers

## Step-by-Step Implementation Plan

### Step 1: Remove Empty Stub File
**Why:** File explicitly marked as moved, contains only redirect comment
**Depends on:** none
**Validation:** code_compiles, follows_conventions
**Creates/Modifies:**
- DELETE: `C:\development\quaero\internal\common\log_consumer.go`

**Actions:**
1. Verify no imports of `internal/common.Consumer` or similar (already confirmed via grep)
2. Delete `internal/common/log_consumer.go`
3. Run `go build ./...` to verify no broken imports

**Risk:** low (file contains no code, already verified no imports)

---

### Step 2: Remove Unused ConfigService Interface
**Why:** Interface defined but implementation is never accessed
**Depends on:** none
**Validation:** code_compiles, follows_conventions
**Creates/Modifies:**
- DELETE: `C:\development\quaero\internal\interfaces\config_service.go`

**Actions:**
1. Delete `internal/interfaces/config_service.go`
2. Run `go build ./...` to verify compilation
3. Grep for any remaining references: `grep -r "ConfigService" internal/`

**Risk:** low (grep confirms no usage of ConfigService methods)

---

### Step 3: Remove Unused ConfigService Implementation
**Why:** Service is created but never used, all config access goes through app.Config directly
**Depends on:** Step 2 (interface removal)
**Validation:** code_compiles, follows_conventions
**Creates/Modifies:**
- DELETE: `C:\development\quaero\internal\services\config\service.go`
- DELETE: `C:\development\quaero\internal\services\config\` directory (if empty after file removal)
- MODIFY: `C:\development\quaero\internal\app\app.go` (remove ConfigService initialization)

**Actions:**
1. Delete `internal/services/config/service.go`
2. Check if `internal/services/config/` directory is empty, delete if so
3. Edit `internal/app/app.go`:
   - Remove import: `"github.com/ternarybob/quaero/internal/services/config"`
   - Remove struct field (line 46): `ConfigService  interfaces.ConfigService`
   - Remove initialization (lines 105-110):
     ```go
     // Create ConfigService for dependency injection
     configService := config.NewService(cfg)
     ```
   - Remove assignment (line 110):
     ```go
     ConfigService: configService, // Use this for new code
     ```
   - Update comment on line 109 (remove "// Deprecated: kept for backward compatibility")
4. Run `go build ./...` to verify compilation
5. Run tests: `cd test/ui && go test -v ./...`

**Risk:** low (ConfigService never accessed, only created and stored)

---

### Step 4: Verify and Clean Up Empty Directories
**Why:** Ensure no orphaned empty directories remain after deletions
**Depends on:** Steps 1-3
**Validation:** code_compiles, directory_structure_clean
**Creates/Modifies:**
- DELETE: Any empty directories created by previous steps

**Actions:**
1. Check for empty directories:
   ```powershell
   Get-ChildItem -Path C:\development\quaero\internal -Directory -Recurse |
   Where-Object { (Get-ChildItem $_.FullName -File -Recurse).Count -eq 0 }
   ```
2. Delete identified empty directories:
   - `internal/services/config/` (if not already removed)
3. Verify build: `go build ./...`
4. Verify tests pass: `cd test/ui && go test -v -run TestHomepage`
5. Commit changes with descriptive message

**Risk:** low (only removing empty directories)

---

## Constraints

- ✅ Breaking changes acceptable (per requirements)
- ✅ Must maintain functionality (redundant code performs no unique function)
- ✅ Remove empty directories after file removal
- ✅ All changes must compile without errors

## Success Criteria

1. ✅ All identified redundant files removed:
   - `internal/common/log_consumer.go` (empty stub)
   - `internal/interfaces/config_service.go` (unused interface)
   - `internal/services/config/service.go` (unused implementation)
   - `internal/services/config/` directory (empty after file removal)

2. ✅ Code compiles without errors: `go build ./...`

3. ✅ No empty directories remain in codebase

4. ✅ App initialization simplified (ConfigService creation removed)

5. ✅ Test suite passes: `cd test/ui && go test -v ./...`

## Code Impact Summary

**Files Deleted:** 3
- `internal/common/log_consumer.go` (3 lines)
- `internal/interfaces/config_service.go` (33 lines)
- `internal/services/config/service.go` (76 lines)

**Files Modified:** 1
- `internal/app/app.go` (remove ~10 lines for ConfigService initialization)

**Directories Deleted:** 1
- `internal/services/config/`

**Total Lines Removed:** ~122 lines of dead code

**Benefit:** Cleaner codebase, reduced cognitive load, simplified dependency graph

## Validation Strategy

Each step must pass:
1. **code_compiles**: `go build ./...` exits with code 0
2. **follows_conventions**: No receiver methods in common/, services use interfaces
3. **directory_structure_clean**: No empty directories remain

Final validation:
1. Full build: `./scripts/build.ps1`
2. UI test suite: `cd test/ui && go test -timeout 20m -v ./...`
3. API test suite: `cd test/api && go test -v ./...`

## Notes

- ConfigService was likely an incomplete refactoring that was started but never finished
- The pattern of having both `app.Config` and `app.ConfigService` indicates mid-refactor abandonment
- All current code uses `app.Config` directly, suggesting ConfigService abstraction was unnecessary
- Version files are NOT duplicates - they serve complementary purposes (CLI vs utilities)
- Document services are NOT duplicates - different architectural layers (core vs MCP adapter)
