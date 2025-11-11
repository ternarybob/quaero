# Build Script Simplification - Migration Guide

**Date:** 2025-11-08
**Affected:** `scripts/build.ps1`
**Version:** Simplified from 590 lines to ~475 lines

---

## Overview

The `build.ps1` script has been simplified to support only three core operations:
1. **Default build** (no parameters) - Build executable silently
2. **-Deploy** - Build and deploy files to bin directory
3. **-Run** - Build, deploy, and run service in new terminal

The following parameters have been removed:
- `-Clean`
- `-Verbose`
- `-Release`
- `-ResetDatabase`
- `-Environment` (unused)
- `-Version` (unused)

---

## What Changed

### Removed Parameters

| Parameter | Purpose | Removal Reason |
|-----------|---------|----------------|
| `-Clean` | Remove bin/ and go.sum before building | Rarely needed; manual deletion is simpler |
| `-Verbose` | Enable verbose build output | Not commonly used; go build is fast enough |
| `-Release` | Optimized release build with stripped symbols | Standard build is sufficient for local development |
| `-ResetDatabase` | Backup and delete database before run | Dangerous operation; better done manually |
| `-Environment` | Target environment (dev/staging/prod) | Unused in actual implementation |
| `-Version` | Version override | Unused in actual implementation |

### Simplified Logic

**Before:**
- 7 parameters with complex conditional logic
- Multiple build flag configurations
- Environment variable setting for release builds
- Database backup/reset workflows
- ~590 lines of code

**After:**
- 2 parameters with straightforward behavior
- Single standard build configuration
- ~475 lines of code (19.5% reduction)

---

## Migration Path

### If you used `-Clean`

**Old command:**
```powershell
.\scripts\build.ps1 -Clean
```

**New approach:**
```powershell
# Manual cleanup (if needed)
Remove-Item -Path bin -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path go.sum -Force -ErrorAction SilentlyContinue

# Then build normally
.\scripts\build.ps1
```

**Why manual is better:**
- Explicit control over what gets deleted
- No accidental data loss
- Cleanup is rarely needed (Go handles dependencies well)

---

### If you used `-Verbose`

**Old command:**
```powershell
.\scripts\build.ps1 -Verbose
```

**New approach:**
```powershell
# Build normally (go build output is already visible)
.\scripts\build.ps1

# If you need more detail, run go build directly for testing
go build -v ./cmd/quaero
```

**Why this is better:**
- Build is fast enough that verbose output isn't needed
- Errors are still shown
- Transcript logs capture all build output in `scripts/logs/`

---

### If you used `-Release`

**Old command:**
```powershell
.\scripts\build.ps1 -Release
```

**New approach:**
Standard build is now used for all scenarios. The difference between debug and release builds is minimal for local development.

**If you really need an optimized build:**
```powershell
# Manual optimized build
$ldflags = "-w -s"  # Strip debug info
go build -ldflags="$ldflags" -o bin/quaero.exe ./cmd/quaero
```

**Why standard build is sufficient:**
- Binary size difference: ~5-10% (not significant for local use)
- Build time difference: negligible
- Debugging is easier with symbols present
- For production deployment, use Docker builds instead

---

### If you used `-ResetDatabase`

**Old command:**
```powershell
.\scripts\build.ps1 -ResetDatabase -Run
```

**New approach:**
```powershell
# Stop any running service first
Stop-Process -Name quaero -Force -ErrorAction SilentlyContinue

# Backup database manually (IMPORTANT!)
$timestamp = Get-Date -Format "yyyy-MM-dd-HH-mm-ss"
New-Item -ItemType Directory -Path bin\backups -Force | Out-Null
Copy-Item bin\data\quaero.db bin\backups\quaero-$timestamp.db

# Delete database files
Remove-Item bin\data\quaero.db -Force
Remove-Item bin\data\quaero.db-wal -Force -ErrorAction SilentlyContinue
Remove-Item bin\data\quaero.db-shm -Force -ErrorAction SilentlyContinue

# Build and run
.\scripts\build.ps1 -Run
```

**Why manual is safer:**
- Explicit backup step prevents accidental data loss
- You can verify backup before deletion
- You control the backup location
- Less risk of automation errors with critical data

**Alternative: Fresh start script**
```powershell
# Create a reusable script for database reset
# Save as: scripts/reset-database.ps1

param([switch]$NoBackup)

$dbPath = "bin\data\quaero.db"

if (Test-Path $dbPath) {
    if (-not $NoBackup) {
        $timestamp = Get-Date -Format "yyyy-MM-dd-HH-mm-ss"
        $backupDir = "bin\backups"
        New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
        Copy-Item $dbPath "$backupDir\quaero-$timestamp.db"
        Write-Host "Database backed up to: $backupDir\quaero-$timestamp.db"
    }

    Remove-Item $dbPath -Force
    Remove-Item "$dbPath-wal" -Force -ErrorAction SilentlyContinue
    Remove-Item "$dbPath-shm" -Force -ErrorAction SilentlyContinue
    Write-Host "Database deleted successfully"
} else {
    Write-Host "No database found at: $dbPath"
}
```

**Usage:**
```powershell
# With backup (recommended)
.\scripts\reset-database.ps1

# Without backup (use with caution!)
.\scripts\reset-database.ps1 -NoBackup
```

---

### If you used `-Environment` or `-Version`

**Old command:**
```powershell
.\scripts\build.ps1 -Environment "prod" -Version "2.0.0"
```

**New approach:**
These parameters were not actually used in the implementation. Version is managed via the `.version` file:

```powershell
# Update version manually in .version file
$versionContent = @"
version: 2.0.0
build: $(Get-Date -Format "MM-dd-HH-mm-ss")
"@
Set-Content -Path .version -Value $versionContent

# Then build normally
.\scripts\build.ps1
```

**Why manual is better:**
- Explicit version control
- No confusion about where version comes from
- Git tracks version changes properly

---

## Testing the Migration

### Step 1: Test default build
```powershell
.\scripts\build.ps1
```

**Expected result:**
- ✅ Executable created at `bin/quaero.exe`
- ✅ `.version` file updated with new build timestamp
- ✅ No deployment (pages/, config not copied)
- ✅ Service not started

---

### Step 2: Test deployment
```powershell
.\scripts\build.ps1 -Deploy
```

**Expected result:**
- ✅ Executable built
- ✅ Running service stopped
- ✅ Files deployed to `bin/`:
  - `quaero.toml` (if not exists)
  - `pages/` directory
  - `quaero-chrome-extension/` directory
  - `job-definitions/` directory (new files only)
- ✅ Service NOT started

---

### Step 3: Test build and run
```powershell
.\scripts\build.ps1 -Run
```

**Expected result:**
- ✅ Executable built
- ✅ Files deployed
- ✅ New terminal window opened with service running
- ✅ Service accessible on configured port (default: 8085)

---

## Frequently Asked Questions

### Q: Can I still use the old parameters?

**A:** The old parameters will be silently ignored due to PowerShell's default behavior. However, they will NOT execute their previous functionality. It's recommended to update your scripts and workflows to use the new approach.

### Q: What if I have automated scripts using the old parameters?

**A:** Update your automation scripts to use the alternatives documented above. The silent parameter ignoring provides a grace period, but explicit migration is recommended.

### Q: Will this break my CI/CD pipeline?

**A:** Depends on what your pipeline does:
- If it uses `.\scripts\build.ps1` only → **No impact**
- If it uses `-Deploy` or `-Run` → **No impact**
- If it uses removed parameters → **Update required** (but scripts won't fail, just won't execute removed functionality)

### Q: How do I clean build if I encounter issues?

**A:** Manual cleanup is more reliable:
```powershell
# Full cleanup
Remove-Item bin -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item go.sum -Force -ErrorAction SilentlyContinue
go clean -cache -modcache -testcache

# Then rebuild
.\scripts\build.ps1
```

### Q: What about production deployments?

**A:** For production, use Docker builds (see README.md):
```bash
docker build -f deployments/docker/Dockerfile -t quaero:latest .
```

Docker builds are optimized and platform-independent.

### Q: Can I get the old parameters back?

**A:** No, they have been permanently removed for simplification. However, all functionality is still available through manual commands documented in this guide.

---

## Benefits of Simplification

### For Developers
- ✅ Fewer build options to remember
- ✅ Faster script execution (less conditional logic)
- ✅ Clearer intent (build, deploy, or run)
- ✅ Less confusion about which flags to use

### For CI/CD
- ✅ Simpler automation scripts
- ✅ Fewer failure modes
- ✅ Explicit control over operations

### For Maintenance
- ✅ 19.5% code reduction (115 fewer lines)
- ✅ 75% fewer parameters
- ✅ Easier to understand and modify
- ✅ Less documentation to maintain

---

## Rollback Plan

If you need to rollback to the old script:

```powershell
# Check out previous version from git
git log --oneline scripts/build.ps1  # Find commit before simplification
git checkout <commit-hash> -- scripts/build.ps1
```

**Note:** Rollback is not recommended. The migration path above provides equivalent functionality with better explicitness.

---

## Support

For issues or questions:
1. Check this migration guide
2. Review `docs/simplify-build-script/test-results.md`
3. See `README.md` for general build instructions
4. Check `CLAUDE.md` or `AGENTS.md` for AI agent guidelines

---

## Summary

| Operation | Old Way | New Way |
|-----------|---------|---------|
| **Clean build** | `.\scripts\build.ps1 -Clean` | Manual `Remove-Item bin/` |
| **Verbose build** | `.\scripts\build.ps1 -Verbose` | Build output already visible |
| **Release build** | `.\scripts\build.ps1 -Release` | Standard build (or manual `go build -ldflags="-w -s"`) |
| **Database reset** | `.\scripts\build.ps1 -ResetDatabase -Run` | Manual backup and delete |
| **Environment override** | `.\scripts\build.ps1 -Environment prod` | Not needed (use config file) |
| **Version override** | `.\scripts\build.ps1 -Version 2.0.0` | Edit `.version` file manually |

**Bottom line:** Manual control over advanced operations is safer and more explicit than automated flags.
