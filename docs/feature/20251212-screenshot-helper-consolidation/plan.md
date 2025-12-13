# Plan: Screenshot Helper Consolidation
Type: feature | Workdir: ./docs/feature/20251212-screenshot-helper-consolidation/ | Date: 2025-12-12

## Context
Project: Quaero
Related files:
- `test/ui/screenshot_helper.go` - Standalone screenshot functions (to be enhanced)
- `test/common/setup.go` - TestEnvironment screenshot methods (to delegate to helper)

## User Intent (from manifest)
1. Consolidate screenshot code: All tests should use the centralized `test/ui/screenshot_helper.go` library
2. Full page screenshots: Update to capture entire page by default (fixes missing test/attribution content)

## Success Criteria (from manifest)
- [ ] screenshot_helper.go updated to use `chromedp.FullScreenshot` for full page capture
- [ ] screenshot_helper.go has both `TakeScreenshot` (viewport) and `TakeFullScreenshot` (full page) functions
- [ ] TestEnvironment methods delegate to screenshot_helper.go functions
- [ ] All existing tests continue to work
- [ ] Build passes
- [ ] Tests pass

## Active Skills
| Skill | Key Patterns to Apply |
|-------|----------------------|
| go | Error wrapping with context, no global state mutation, functions return (result, error) |

## Technical Approach
1. Enhance `screenshot_helper.go` with:
   - `TakeFullScreenshot(ctx, name)` - captures full page using `chromedp.FullScreenshot`
   - Keep existing `TakeScreenshot(ctx, name)` for viewport capture
   - Shared helper for path generation

2. Update `setup.go` TestEnvironment methods to:
   - Import and call screenshot_helper functions
   - Maintain existing API for backwards compatibility
   - Pass the results directory for path generation

3. No changes needed to existing tests - they use TestEnvironment methods which will delegate internally

## Files to Change
| File | Action | Purpose |
|------|--------|---------|
| test/ui/screenshot_helper.go | modify | Add TakeFullScreenshot, refactor for configurability |
| test/common/setup.go | modify | Delegate to screenshot_helper.go functions |

## Tasks
| # | Desc | Depends | Critical | Model | Skill | Est. Files |
|---|------|---------|----------|-------|-------|------------|
| 1 | Enhance screenshot_helper.go with full page capture | - | no | opus | go | 1 |
| 2 | Update setup.go to delegate to screenshot_helper | 1 | no | opus | go | 1 |
| 3 | Verify build and run tests | 2 | no | opus | go | 0 |

## Execution Order
[1] → [2] → [3]

## Risks/Decisions
- Keep backwards compatibility - existing test code should not need changes
- Full page screenshot quality set to 90 (matches existing TakeFullScreenshot in setup.go)
- screenshot_helper.go needs to be importable from test/common/setup.go without circular imports
