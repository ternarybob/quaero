# Step 3: Update codebase_assess.toml pipeline with category filters

- **Status:** complete
- **Skill:** go
- **Duration:** ~1min

## Files Modified
- `bin/job-definitions/codebase_assess.toml` - Added filter_category to agent steps

## Changes

### classify_files
```toml
filter_category = ["unknown"]
```
Only LLM-classify files that rule_classifier couldn't categorize.

### extract_build_info
```toml
depends = "rule_classify_files"  # Changed from import_files
filter_category = ["build", "config", "docs"]
```
Only extract build info from build/config/docs files.

### identify_components
```toml
depends = "rule_classify_files"  # Changed from classify_files
filter_category = ["source"]
```
Only identify components in source code files.

## Expected Impact
Before: ~3000 LLM calls (1000 files x 3 agent steps)
After: ~300 LLM calls estimated:
- classify_files: ~100 unknown files
- extract_build_info: ~50 build/config/docs files
- identify_components: ~150 source files

## Notes
- Changed dependencies to require rule_classify_files completion first
- All agent steps now filtered by category after rule classification
