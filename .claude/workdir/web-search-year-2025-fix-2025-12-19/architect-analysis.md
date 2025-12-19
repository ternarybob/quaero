# ARCHITECT ANALYSIS - Update Year References in Web Search Jobs

## Task
Update all web-search job definitions to use 2025 instead of limiting to 2024.

## Files with 2024 References Found

| File | Line | Query Contains |
|------|------|----------------|
| `web-search-asx.toml` | 55 | `outlook 2024 2025` |
| `web-search-asx-wes.toml` | 56 | `outlook 2024 2025` |
| `web-search-asx-wes.toml` | 68 | `performance 2024` |
| `web-search-asx-cba.toml` | 55 | `outlook 2024 2025` |
| `web-search-asx-cba.toml` | 67 | `performance 2024` |
| `web-search-asx-exr.toml` | 55 | `outlook 2024 2025` |

## Analysis

### Pattern 1: "outlook 2024 2025"
These queries include both years to get recent data. Should be updated to `2025 2026` to cover current year and forward-looking data.

### Pattern 2: "performance 2024"
These queries specifically reference 2024 data. Should be updated to `2025` to get current year data.

## Recommendation

**MODIFY** the TOML files directly - no Go code changes needed.

Changes:
1. `outlook 2024 2025` → `outlook 2025 2026`
2. `performance 2024` → `performance 2025`

## Files to Modify
1. `bin/job-definitions/web-search-asx.toml` - 1 change
2. `bin/job-definitions/web-search-asx-wes.toml` - 2 changes
3. `bin/job-definitions/web-search-asx-cba.toml` - 2 changes
4. `bin/job-definitions/web-search-asx-exr.toml` - 1 change

## Anti-Creation Compliance
- No new files created
- No Go code changes
- Simple text replacement in TOML config files

## Build Verification
Build script should still pass (TOML files loaded at runtime).
