# Step 2: Add PublishToUI option to JobLogOptions

Model: sonnet | Status: ✅

## Done

- Added `PublishToUI *bool` field to JobLogOptions struct
- nil = auto filter by level (default behavior)
- true = force publish to UI regardless of level
- false = skip UI publish regardless of level
- Workers can override the default behavior when needed

## Files Changed

- `internal/queue/manager.go` - Added PublishToUI field to JobLogOptions

## Verify

Build: ✅ | Tests: ⏭️
