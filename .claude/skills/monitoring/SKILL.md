# UI Testing & Monitoring Skill for Quaero

**Prerequisite:** Read `.claude/skills/refactoring/SKILL.md` first.

## BEFORE WRITING ANY TEST

**MANDATORY:**
1. Read `docs/TEST_ARCHITECTURE.md`
2. Read 2-3 existing tests in `test/ui/`
3. Use patterns from `test/ui/uitest_context.go`

## BROWSER AUTOMATION STANDARD
```
┌─────────────────────────────────────────────────────────────────┐
│ CHROMEDP IS THE ONLY ALLOWED OPTION                             │
│                                                                  │
│ ✓ github.com/chromedp/chromedp                                  │
│                                                                  │
│ ✗ FORBIDDEN: selenium, playwright, puppeteer, rod               │
└─────────────────────────────────────────────────────────────────┘
```

## TEST CONTEXT (MANDATORY)
```go
func TestMyFeature(t *testing.T) {
    utc := NewUITestContext(t, 5*time.Minute)
    defer utc.Cleanup()  // ALWAYS

    utc.Log("Starting test")
    utc.Screenshot("initial")
    // ...
}
```

## Available Helpers

| Method | Purpose |
|--------|---------|
| `utc.Navigate(url)` | Navigate to page |
| `utc.Screenshot(name)` | Auto-numbered screenshot |
| `utc.Log(fmt, args...)` | Structured logging |
| `utc.Click(selector)` | Click element |
| `utc.GetText(selector)` | Get element text |
| `utc.TriggerJob(name)` | Trigger job via UI |
| `utc.MonitorJob(name, opts)` | Monitor job status |
| `utc.SaveToResults(file, data)` | Save data to results |

## Anti-Patterns (AUTO-FAIL)
```go
// ❌ Not using UITestContext
ctx, cancel := chromedp.NewContext(...)  // Use NewUITestContext!

// ❌ Alternative browser automation
import "github.com/tebeka/selenium"       // FORBIDDEN
import "github.com/playwright-community/playwright-go"  // FORBIDDEN

// ❌ Custom infrastructure
type MyTestContext struct { }  // Use existing UITestContext!

// ❌ Missing cleanup
utc := NewUITestContext(t, timeout)
// Missing: defer utc.Cleanup()

// ❌ Custom logging
log.Printf(...)  // Use utc.Log()
fmt.Printf(...)  // Use utc.Log()

// ❌ No error checks
utc.Navigate(url)  // Check error!
```

## MISALIGNED TEST HANDLING

If a test expects **deprecated behavior** (not a code bug):

1. **DO NOT** add backward compatibility to make test pass
2. **DO NOT** modify the test
3. **Document** in `$WORKDIR/test_issues.md`:
   - What test expects (deprecated)
   - What test SHOULD expect (current)
   - Suggested test change
4. **Continue** with remaining tests

See `3agents-tdd` workflow for full protocol.

## Validation Checklist

- [ ] Uses `NewUITestContext(t, timeout)`
- [ ] Has `defer utc.Cleanup()`
- [ ] Uses `utc.Log()` not log/fmt
- [ ] Uses `utc.Screenshot()` at key moments
- [ ] Error checks on chromedp operations
- [ ] Follows patterns from existing tests
- [ ] Uses testify (assert/require)
- [ ] No parallel test infrastructure
- [ ] No dead test code left behind