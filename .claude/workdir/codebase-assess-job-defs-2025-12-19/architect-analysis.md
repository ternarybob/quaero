# ARCHITECT ANALYSIS - Codebase Assess Job Definitions

## Task
Create 2 new codebase assess job definitions in `bin/job-definitions/`:
1. `codebase_assess_fast.toml` - Fast/lightweight version
2. `codebase_assess.toml` - Full comprehensive version

Remove the old single job definition and replace with both versions.

## Source Files

### Templates (from deployments/local/job-definitions/)
- `codebase_assess_fast.toml` - Fast version with only code_map + summaries (no file import)
- `codebase_assess.toml` - Full version with file import, classification, agents, graphs

### Current (bin/job-definitions/)
- `codebase_assess.toml` - Has correct directory path: `C:\development\quaero`

## Analysis

### Key Differences Between Templates

| Aspect | Fast Version | Full Version |
|--------|--------------|--------------|
| Timeout | 30m | 4h |
| Steps | 4 | 10 |
| File Import | No (code_map only) | Yes (local_dir) |
| Classification | No | Yes (agent) |
| Dependency Graph | No | Yes |
| Use Case | Quick overview | Deep analysis |

### Directory Path
Both new job definitions must use: `C:\development\quaero`

### Project Name
Current uses: `test-project` - keep consistent

## Files to Create/Modify

| File | Action |
|------|--------|
| `bin/job-definitions/codebase_assess.toml` | Replace with full template + correct dir_path |
| `bin/job-definitions/codebase_assess_fast.toml` | Create new from fast template + correct dir_path |

## Changes Required

### 1. codebase_assess_fast.toml (NEW)
- Copy from `deployments/local/job-definitions/codebase_assess_fast.toml`
- Update `dir_path` to `C:\development\quaero`
- Update `project_name` to `test-project` for consistency

### 2. codebase_assess.toml (REPLACE)
- Copy from `deployments/local/job-definitions/codebase_assess.toml`
- Keep existing `dir_path = "C:\development\quaero"` (already correct)
- Keep existing `project_name = "test-project"` (already correct)

## Anti-Creation Compliance

- **EXTEND > MODIFY > CREATE**: One modification, one creation (both necessary per user request)
- **Templates used**: Following existing patterns from deployments/local/
- **No over-engineering**: Direct copy with path updates only
