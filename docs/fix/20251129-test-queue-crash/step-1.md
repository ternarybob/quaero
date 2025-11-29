# Step 1: Remove chromedp.Cancel call in verifyServiceConnectivity

- Task: task-1.md | Group: 1 | Model: opus

## Actions
1. Identified root cause: `chromedp.Cancel(browserCtx)` at line 99 causes deadlock when browser context is already cancelled
2. Removed the explicit `chromedp.Cancel()` call from the defer block
3. Kept `cancelBrowser()` which properly cleans up the context

## Files
- `test/ui/main_test.go` - Removed lines 98-101 (chromedp.Cancel in defer), simplified to just `defer cancelBrowser()`

## Decisions
- Removed explicit chromedp.Cancel: The cancel function returned by `chromedp.NewContext` already handles proper cleanup. Calling `chromedp.Cancel()` explicitly is redundant and causes deadlock when the context is already done.

## Verify
Compile: ✅ | Tests: ⚙️ (pending)

## Status: ✅ COMPLETE
