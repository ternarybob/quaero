# Feature: Screenshot Helper Consolidation
- Slug: screenshot-helper-consolidation | Type: feature | Date: 2025-12-12
- Request: "1. Update all tests which have screenshots to use the same library -> test\ui\screenshot_helper.go 2. Update the lib to screenshot the entire page, as many times the screenshot does NOT capture the test/attribution."
- Prior: none

## User Intent
1. **Consolidate screenshot code**: All tests should use the centralized `test/ui/screenshot_helper.go` library instead of having multiple implementations
2. **Full page screenshots**: Update the screenshot helper to capture the entire page by default, since current viewport screenshots often miss important content like test attribution at the bottom

Currently there are two screenshot implementations:
- `test/ui/screenshot_helper.go` - Standalone `TakeScreenshot()` function using `chromedp.CaptureScreenshot` (viewport only)
- `test/common/setup.go` - Methods on `TestEnvironment`: `TakeScreenshot()`, `TakeFullScreenshot()`, `GetScreenshotPath()`

Tests currently mix both approaches:
- Some use `env.TakeScreenshot()` / `env.TakeFullScreenshot()` from setup.go
- Some use `utc.Screenshot()` which wraps `env.TakeScreenshot()`
- The standalone `TakeScreenshot()` in screenshot_helper.go appears unused

## Success Criteria
- [ ] screenshot_helper.go updated to use `chromedp.FullScreenshot` for full page capture
- [ ] screenshot_helper.go has both `TakeScreenshot` (viewport) and `TakeFullScreenshot` (full page) functions
- [ ] TestEnvironment methods delegate to screenshot_helper.go functions
- [ ] All existing tests continue to work
- [ ] Build passes
- [ ] Tests pass

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | yes | yes | Modifying Go test infrastructure |
| frontend | .claude/skills/frontend/SKILL.md | yes | no | No frontend changes |

**Active Skills:** go
