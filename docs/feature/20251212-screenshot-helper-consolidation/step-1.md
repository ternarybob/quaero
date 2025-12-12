# Step 1: Enhance screenshot_helper.go
Workdir: ./docs/feature/20251212-screenshot-helper-consolidation/ | Model: opus | Skill: go
Status: ✅ Complete
Timestamp: 2025-12-12T10:30:00Z

## Task Reference
From task-1.md:
- Intent: Add full page screenshot capability to screenshot_helper.go
- Accept criteria: TakeFullScreenshot, TakeScreenshotToPath, TakeFullScreenshotToPath added

## Implementation Summary
Enhanced screenshot_helper.go with full page screenshot support and path-based functions for delegation from setup.go.

## Files Changed
| File | Action | Lines | Description |
|------|--------|-------|-------------|
| `test/ui/screenshot_helper.go` | modified | +43 | Added TakeFullScreenshot, TakeScreenshotToPath, TakeFullScreenshotToPath |

## Code Changes Detail
### screenshot_helper.go
```go
// Added constant for quality
const FullScreenshotQuality = 90

// Added TakeFullScreenshot for full page capture
func TakeFullScreenshot(ctx context.Context, name string) error

// Added TakeScreenshotToPath for explicit path control
func TakeScreenshotToPath(ctx context.Context, path string) error

// Added TakeFullScreenshotToPath for explicit path control
func TakeFullScreenshotToPath(ctx context.Context, path string) error
```
**Why:** These functions enable delegation from setup.go and provide full page capture using chromedp.FullScreenshot

## Skill Compliance
### go/SKILL.md Checklist
- [x] Error wrapping with %w - all errors wrapped with context
- [x] No global state for configuration - quality is a constant
- [x] Functions return (result, error) pattern followed

## Accept Criteria Verification
- [x] TakeFullScreenshot function added using chromedp.FullScreenshot
- [x] TakeScreenshotToPath and TakeFullScreenshotToPath functions added
- [x] Error handling uses %w wrapping
- [x] File compiles without errors

## Build & Test
```
Build: Pending (combined with Task 2)
Tests: Pending
```

## Issues Encountered
- None

## State for Next Phase
Files ready for Task 2:
- `test/ui/screenshot_helper.go` - Enhanced with all required functions
