# Task 2: Update setup.go to delegate to screenshot_helper
Workdir: ./docs/feature/20251212-screenshot-helper-consolidation/ | Depends: 1 | Critical: no
Model: opus | Skill: go

## Context
This task is part of: Screenshot helper consolidation - centralizing screenshot code
Prior tasks completed: Task 1 - Enhanced screenshot_helper.go with full page capture

## User Intent Addressed
Update all tests which have screenshots to use the same library -> test/ui/screenshot_helper.go

## Input State
Files that exist before this task:
- `test/ui/screenshot_helper.go` - Enhanced with TakeFullScreenshot, TakeScreenshotToPath, TakeFullScreenshotToPath
- `test/common/setup.go` - Has TakeScreenshot, TakeFullScreenshot, GetScreenshotPath methods on TestEnvironment

## Output State
Files after this task completes:
- `test/common/setup.go` - Methods delegate to screenshot_helper.go functions

## Skill Patterns to Apply
### From go/SKILL.md:
- **DO:** Keep backwards compatibility for existing code
- **DO:** Wrap errors with context
- **DON'T:** Break existing API signatures

## Implementation Steps
1. Read current setup.go screenshot methods
2. Import the ui package (check for circular imports)
3. Update TakeScreenshot to delegate to ui.TakeScreenshotToPath
4. Update TakeFullScreenshot to delegate to ui.TakeFullScreenshotToPath
5. Keep GetScreenshotPath as-is (generates paths)
6. Verify no circular imports
7. Build passes

## Code Specifications
Updated methods should look like:
```go
func (env *TestEnvironment) TakeScreenshot(ctx context.Context, name string) error {
    path := env.GetScreenshotPath(name)
    return ui.TakeScreenshotToPath(ctx, path)
}

func (env *TestEnvironment) TakeFullScreenshot(ctx context.Context, name string) error {
    path := env.GetScreenshotPath(name)
    return ui.TakeFullScreenshotToPath(ctx, path)
}
```

## Accept Criteria
- [ ] TakeScreenshot delegates to ui.TakeScreenshotToPath
- [ ] TakeFullScreenshot delegates to ui.TakeFullScreenshotToPath
- [ ] No circular imports
- [ ] Build passes
- [ ] Existing test signatures unchanged

## Handoff
After completion, next task(s): Task 3 - Verify build and run tests
