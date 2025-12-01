# Task 1: Remove chromedp.Cancel call in verifyServiceConnectivity

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: ./docs/fix/20251129-test-queue-crash/

## Files
- `test/ui/main_test.go` - Remove problematic chromedp.Cancel call

## Requirements
1. Remove the explicit `chromedp.Cancel(browserCtx)` call in the defer block at line 98-101
2. Keep the `cancelBrowser()` call which properly cleans up the context

## Acceptance
- [ ] chromedp.Cancel call removed from verifyServiceConnectivity
- [ ] cancelBrowser() still called for proper cleanup
- [ ] Code compiles without errors
