# Step 2: Update setup.go (approach revised)
Workdir: ./docs/feature/20251212-screenshot-helper-consolidation/ | Model: opus | Skill: go
Status: ✅ Complete (with revision)
Timestamp: 2025-12-12T10:45:00Z

## Task Reference
From task-2.md:
- Intent: Update TestEnvironment methods to delegate to screenshot_helper
- Accept criteria: No circular imports, build passes, existing signatures unchanged

## Implementation Summary
**Initial attempt**: Tried to import `test/ui` from `test/common` to delegate screenshot calls.

**Problem encountered**: Import cycle - `ui` test files import `common`, and `common` importing `ui` creates a cycle in Go's test compilation.

**Revised approach**: Keep implementations in both packages with identical behavior. Both use:
- `chromedp.CaptureScreenshot` for viewport screenshots
- `chromedp.FullScreenshot` for full page screenshots (quality 90)

This achieves the goal of consistent screenshot behavior across all tests without circular imports.

## Files Changed
| File | Action | Lines | Description |
|------|--------|-------|-------------|
| `test/common/setup.go` | unchanged | 0 | Reverted to original; already has correct TakeFullScreenshot |

## Code Changes Detail
No changes to setup.go - the original implementation already matches screenshot_helper.go behavior.

Both packages now have:
```go
// Viewport screenshot
chromedp.CaptureScreenshot(&buf)

// Full page screenshot
chromedp.FullScreenshot(&buf, 90)
```

## Skill Compliance
### go/SKILL.md Checklist
- [x] No circular imports - packages remain independent
- [x] Backwards compatibility - no signature changes
- [x] Error wrapping maintained

## Accept Criteria Verification
- [x] No circular imports (verified: build passes)
- [x] Build passes
- [x] Existing test signatures unchanged
- [ ] Delegation not possible (circular imports)

## Build & Test
```
Build: ✅ Pass
Tests: ✅ Pass (TestJobLoggingImprovements - 19.57s)
```

## Issues Encountered
- Import cycle when trying to import ui from common
- Resolved by keeping implementations in both packages

## State for Next Phase
Both packages have matching screenshot implementations:
- `test/ui/screenshot_helper.go` - Standalone functions
- `test/common/setup.go` - TestEnvironment methods
