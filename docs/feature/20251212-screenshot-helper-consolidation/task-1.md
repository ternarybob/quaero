# Task 1: Enhance screenshot_helper.go with full page capture
Workdir: ./docs/feature/20251212-screenshot-helper-consolidation/ | Depends: none | Critical: no
Model: opus | Skill: go

## Context
This task is part of: Screenshot helper consolidation - centralizing screenshot code and adding full page capture
Prior tasks completed: none - this is first

## User Intent Addressed
Update the lib to screenshot the entire page, as many times the screenshot does NOT capture the test/attribution.

## Input State
Files that exist before this task:
- `test/ui/screenshot_helper.go` - Has TakeScreenshot (viewport) and GetScreenshotsDir functions

## Output State
Files after this task completes:
- `test/ui/screenshot_helper.go` - Enhanced with TakeFullScreenshot and TakeScreenshotToPath functions

## Skill Patterns to Apply
### From go/SKILL.md:
- **DO:** Wrap errors with context using %w
- **DO:** Use existing patterns from codebase
- **DON'T:** Use global state for configuration
- **DON'T:** Panic on errors

## Implementation Steps
1. Read current screenshot_helper.go
2. Add TakeFullScreenshot function using chromedp.FullScreenshot(&buf, 90)
3. Add TakeScreenshotToPath and TakeFullScreenshotToPath for explicit path control
4. Ensure error wrapping follows patterns
5. Verify file compiles

## Code Specifications
Functions to add:
```go
// TakeFullScreenshot captures a full page screenshot with scrolling
func TakeFullScreenshot(ctx context.Context, name string) error

// TakeScreenshotToPath captures viewport screenshot to explicit path
func TakeScreenshotToPath(ctx context.Context, filepath string) error

// TakeFullScreenshotToPath captures full page screenshot to explicit path
func TakeFullScreenshotToPath(ctx context.Context, filepath string) error
```

## Accept Criteria
- [ ] TakeFullScreenshot function added using chromedp.FullScreenshot
- [ ] TakeScreenshotToPath and TakeFullScreenshotToPath functions added
- [ ] Error handling uses %w wrapping
- [ ] File compiles without errors
- [ ] Build passes

## Handoff
After completion, next task(s): Task 2 - Update setup.go to delegate
