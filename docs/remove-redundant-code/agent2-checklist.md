# Agent 2 Implementation Checklist

This checklist ensures Agent 2 (Implementation) can verify the analysis and execute the plan safely.

## Pre-Implementation Verification

Before starting ANY deletions, Agent 2 should verify these facts:

### ✅ Verification 1: log_consumer.go is empty stub
```bash
cat internal/common/log_consumer.go
# Expected output: Only redirect comment (3 lines)
# Verify: No functional code

grep -r "internal/common.*Consumer\|log_consumer" internal/
# Expected output: No imports found
```

### ✅ Verification 2: ConfigService is never used
```bash
# Check for any ConfigService method calls
grep -r "\.ConfigService\." internal/

# Expected output: 0 matches (or only in comments)
```

```bash
# Check where ConfigService is created
grep -n "config.NewService\|ConfigService:" internal/app/app.go

# Expected output:
# - Line 106: configService := config.NewService(cfg)
# - Line 110: ConfigService: configService
# - IMPORTANT: Verify no other code accesses app.ConfigService
```

```bash
# Check actual config access pattern
grep -r "app\.Config\." internal/

# Expected output: Should find uses in app.go and server.go
# This confirms direct config access is the current pattern
```

### ✅ Verification 3: config package imports
```bash
# Verify what imports the config service package
grep -r "github.com/ternarybob/quaero/internal/services/config" internal/

# Expected output: Only internal/app/app.go
# This confirms only one place to update
```

## Implementation Steps (IN ORDER)

### Step 1: Remove log_consumer.go stub
**Pre-check:**
```bash
# Verify file exists and is stub
cat internal/common/log_consumer.go

# Verify no imports
grep -r "log_consumer" internal/
```

**Action:**
```powershell
Remove-Item "C:\development\quaero\internal\common\log_consumer.go"
```

**Post-check:**
```bash
go build ./...
# Expected: Success (no errors)
```

---

### Step 2: Remove ConfigService interface
**Pre-check:**
```bash
# Verify interface file exists
cat internal/interfaces/config_service.go

# Verify no usage besides app.go
grep -r "ConfigService" internal/ --exclude-dir=app
```

**Action:**
```powershell
Remove-Item "C:\development\quaero\internal\interfaces\config_service.go"
```

**Post-check:**
```bash
go build ./...
# Expected: Build error in app.go (expected, will fix in step 3)
```

---

### Step 3: Remove config service package and update app.go

**Pre-check:**
```bash
# Verify package exists
ls internal/services/config/

# Verify app.go imports it
grep "services/config" internal/app/app.go
```

**Action 3a: Delete config service package**
```powershell
Remove-Item -Recurse "C:\development\quaero\internal\services\config"
```

**Action 3b: Edit app.go**

Open `internal/app/app.go` and make these changes:

1. **Remove import (line ~26):**
   ```diff
   - "github.com/ternarybob/quaero/internal/services/config"
   ```

2. **Remove struct field (line ~46):**
   ```diff
   - ConfigService  interfaces.ConfigService
   ```

3. **Update Config field comment (line ~45):**
   ```diff
   - Config         *common.Config // Deprecated: Use ConfigService instead
   + Config         *common.Config
   ```

4. **Remove ConfigService initialization (lines ~105-110):**
   ```diff
   - // Create ConfigService for dependency injection
   - configService := config.NewService(cfg)
   -
   ```

5. **Remove from App struct initialization (line ~110):**
   ```diff
   app := &App{
   -     Config:        cfg,           // Deprecated: kept for backward compatibility
   -     ConfigService: configService, // Use this for new code
   +     Config:        cfg,
        Logger:        logger,
   }
   ```

**Post-check:**
```bash
# Verify build succeeds
go build ./...
# Expected: Success

# Verify no references remain
grep -r "ConfigService" internal/
# Expected: 0 matches (or only in unrelated comments)

# Verify config access still works
grep "app.Config\." internal/app/app.go
# Expected: Still shows config being used
```

---

### Step 4: Clean up empty directories

**Pre-check:**
```powershell
# Check for empty directories
Get-ChildItem -Path C:\development\quaero\internal -Directory -Recurse |
Where-Object { (Get-ChildItem $_.FullName -File -Recurse).Count -eq 0 }
```

**Action:**
```powershell
# If internal/services/config still exists and is empty:
if (Test-Path "C:\development\quaero\internal\services\config") {
    if ((Get-ChildItem "C:\development\quaero\internal\services\config" -Recurse).Count -eq 0) {
        Remove-Item -Recurse "C:\development\quaero\internal\services\config"
    }
}
```

**Post-check:**
```bash
# Verify no empty directories remain
find internal/ -type d -empty
# Expected: No output (or only test result directories)
```

---

## Final Validation

### Build Verification
```bash
# Full build
cd C:\development\quaero
go build ./...

# Expected: Success with no errors
```

### Test Verification
```bash
# UI Tests
cd test/ui
go test -timeout 20m -v -run TestHomepage
# Expected: PASS

# API Tests
cd test/api
go test -v -run TestConfigAPI
# Expected: PASS
```

### Production Build
```powershell
# Full production build
.\scripts\build.ps1

# Expected: Build succeeds, binary created
```

### Code Search Verification
```bash
# Verify no references to removed code
grep -r "log_consumer" internal/
grep -r "ConfigService" internal/
grep -r "services/config" internal/

# Expected: All return 0 matches (or only in unrelated comments)
```

---

## Rollback Plan (If Needed)

If any step fails, rollback using git:

```bash
# Revert all changes
git checkout internal/common/log_consumer.go
git checkout internal/interfaces/config_service.go
git checkout internal/services/config/
git checkout internal/app/app.go

# Verify build works again
go build ./...
```

---

## Success Criteria

All of these must be true:
- ✅ `internal/common/log_consumer.go` deleted
- ✅ `internal/interfaces/config_service.go` deleted
- ✅ `internal/services/config/` directory deleted
- ✅ `internal/app/app.go` modified (ConfigService references removed)
- ✅ `go build ./...` succeeds
- ✅ UI tests pass
- ✅ API tests pass
- ✅ Production build succeeds
- ✅ No grep matches for removed code

---

## Expected Changes Summary

**Files Deleted:** 3
- internal/common/log_consumer.go
- internal/interfaces/config_service.go
- internal/services/config/service.go

**Directories Deleted:** 1
- internal/services/config/

**Files Modified:** 1
- internal/app/app.go (~10 lines removed)

**Total Lines Removed:** ~122 lines

**Functional Impact:** NONE (all removed code was unused)

---

## Notes for Agent 2

1. **Execute steps in order** - Steps 2-3 will cause temporary build errors (expected)
2. **Verify at each step** - Run the post-checks to confirm success
3. **Don't skip verification** - The grep commands confirm the analysis is correct
4. **Use git safety** - Commit after each major step for easy rollback
5. **Test before final commit** - Run full test suite before declaring success

## Questions for Agent 2 to Answer

Before starting, verify you can answer YES to all:
1. Have you read the full plan in `plan.md`?
2. Have you verified the analysis in `analysis-summary.md`?
3. Do you understand why each piece of code is redundant?
4. Do you have a rollback plan if something fails?
5. Will you run tests after each major change?

If YES to all → Proceed with implementation
If NO to any → Review documentation before starting
