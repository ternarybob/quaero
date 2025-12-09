# Step 2: Add filter_category support to agent_worker queryDocuments

- **Status:** complete
- **Skill:** go
- **Duration:** ~2min

## Files Modified
- `internal/queue/workers/agent_worker.go` - Added category filter support

## Skill Compliance
- [x] Error wrapping with context - N/A
- [x] Structured logging (arbor) - Added debug log for category filtering
- [x] Interface-based DI - N/A
- [x] Constructor injection - N/A

## Changes
- Added `extractCategoryFilter()` helper function
- Modified `queryDocuments()` to convert `filter_category` to MetadataFilters
- Supports array format: `filter_category = ["source", "build"]`
- Converts to nested key format: `rule_classifier.category=source,build`

## Config Format
```toml
[step.extract_build_info]
filter_category = ["build", "config", "docs"]
```

## Notes
- The `filter_` prefix is automatically stripped by existing logic in Init()
- Category filter uses MetadataFilters which supports nested dot-notation
