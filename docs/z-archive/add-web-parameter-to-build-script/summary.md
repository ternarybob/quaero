# Add -Web Parameter to Build Script

## Overview
Added `-Web` parameter to `scripts/build.ps1` for rapid frontend development. This parameter deploys only the `pages` directory and restarts the application without building or updating the version.

## Changes Made

### Modified Files
- `scripts/build.ps1` - Added `-Web` parameter and implementation

## Implementation Details

### New Parameter
```powershell
.\build.ps1 -Web
```

**Behavior:**
1. Verifies `bin/quaero.exe` exists (requires initial build)
2. Stops running Quaero service (graceful shutdown if possible)
3. Deploys only the `pages` directory to `bin/pages`
4. Restarts the application
5. Exits without building or updating version

**Use Case:** Rapid frontend iteration - modify HTML, CSS, or JavaScript in `pages/` and quickly reload without waiting for a full build.

### Script Flow

**With `-Web` parameter:**
```
1. Start transcript logging
2. Setup paths and verify executable exists
3. Stop Quaero service (HTTP graceful shutdown → force if needed)
4. Remove old pages directory from bin/
5. Copy pages/ to bin/pages/
6. Restart application in new terminal
7. Exit (skip all build steps)
```

**Without `-Web` parameter:**
```
(Standard build flow continues as before)
```

### Code Structure

The `-Web` logic is placed at the beginning of the script (after transcript start) to:
- Exit early and skip expensive build operations
- Provide fast feedback for frontend developers
- Reuse minimal helper code (inline port detection and service stop)

## Usage Examples

```powershell
# Initial setup (one time)
.\build.ps1 -Run

# Frontend development workflow (rapid iteration)
# Edit files in pages/
.\build.ps1 -Web
# Repeat as needed

# Full rebuild when needed
.\build.ps1 -Run
```

## Benefits

1. **Faster Frontend Iteration**
   - No Go compilation (saves ~10-30 seconds)
   - No version file updates
   - Only deploys changed files (pages directory)

2. **Simplified Workflow**
   - Single command for frontend-only changes
   - Automatic service restart
   - No manual copy/paste of files

3. **Maintains Consistency**
   - Uses same deployment logic as full build
   - Consistent service stop/start behavior
   - Transcript logging preserved

## Testing

**Manual verification steps:**
1. Run `.\build.ps1` to create initial executable
2. Verify application starts correctly
3. Modify a file in `pages/` (e.g., add a comment to HTML)
4. Run `.\build.ps1 -Web`
5. Verify:
   - Service stops gracefully
   - Pages directory updated in bin/
   - Service restarts automatically
   - Changes visible in browser (after refresh)
   - No version update in `.version` file

## Documentation Updates

Updated help documentation in `build.ps1`:
- Added parameter description for `-Web`
- Added usage example
- Updated operation count (3 → 4)
- Added notes about rapid frontend development

**Completed:** 2025-12-11
