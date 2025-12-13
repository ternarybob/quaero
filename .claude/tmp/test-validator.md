# CLAUDE COMMAND: Validate UI Tests

You are reviewing Go UI test files in `./test/ui` for template compliance and code alignment.

## Step 1: Read All Test Files

Read all `*_test.go` files in the `./test/ui` directory.

## Step 2: Validate Against Template

Each test MUST follow this pattern:
```go
func TestExample(t *testing.T) {
    // 1. Setup test environment with test name
    env, err := common.SetupTestEnvironment("TestName")
    if err != nil {
        t.Fatalf("Failed to setup test environment: %v", err)
    }
    defer env.Cleanup()

    // 2. Timing and status logging
    startTime := time.Now()
    env.LogTest(t, "=== RUN TestExample")
    defer func() {
        elapsed := time.Since(startTime)
        if t.Failed() {
            env.LogTest(t, "--- FAIL: TestExample (%.2fs)", elapsed.Seconds())
        } else {
            env.LogTest(t, "--- PASS: TestExample (%.2fs)", elapsed.Seconds())
        }
    }()

    // 3. Setup chromedp context
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    // 4. Navigate and take START screenshot
    err = chromedp.Run(ctx,
        chromedp.Navigate(url),
        chromedp.WaitVisible(`body`, chromedp.ByQuery),
    )
    
    if err := env.TakeScreenshot(ctx, "start-state"); err != nil {
        t.Fatalf("Failed to take screenshot: %v", err)
    }

    // 5. Perform test actions with screenshots at key points
    // ... test logic ...

    // 6. Take END screenshot
    if err := env.TakeScreenshot(ctx, "final-state"); err != nil {
        t.Fatalf("Failed to take screenshot: %v", err)
    }

    // 7. Assertions
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

## Step 3: Check Template Compliance

For each test function, verify:

1. ✅ **Environment Setup**: Has `common.SetupTestEnvironment("TestName")`
2. ✅ **Cleanup**: Has `defer env.Cleanup()`
3. ✅ **Timing Setup**: Has `startTime := time.Now()`
4. ✅ **Status Logging**: Has defer func with `t.Failed()` check and PASS/FAIL logging
5. ✅ **Screenshots**: Has at least one `env.TakeScreenshot()` call
6. ✅ **Start Screenshot**: Has screenshot near beginning (initial state)
7. ✅ **End Screenshot**: Has screenshot near end (final state)
8. ✅ **Context Setup**: Has chromedp context with defer cancel
9. ✅ **Logging**: Uses `env.LogTest()` for test progress

## Step 4: Check Code Alignment

For each test, verify the code being tested exists:

- **Routes/Paths**: If test navigates to `/jobs`, does that route exist?
- **HTML Elements**: If test checks for `.card` selector, does that element exist in the templates?
- **Functionality**: Is the tested functionality actually implemented?
- **Alignment Status**: ✅ Aligned / ⚠️ Needs verification / ❌ Misaligned

You should look at route handlers, HTML templates, and related code to verify alignment.

## Step 5: Generate Report

Create a Markdown report saved to `./docs/tests/test-validation-<date>.md`

### Report Format:
```markdown
# UI Test Validation Report

**Generated:** YYYY-MM-DD HH:MM:SS

## Summary

| Metric | Count |
|--------|-------|
| Total Tests | X |
| ✅ Passed | X |
| ❌ Failed | X |
| Compliance | X% |

## Detailed Results

### filename_test.go

#### [✅ PASS / ❌ FAIL] TestName

**Template Compliance:**
- [x] or [ ] Environment setup
- [x] or [ ] Defer cleanup
- [x] or [ ] Timing setup
- [x] or [ ] Status logging
- [x] or [ ] Screenshots (count: X)
- [x] or [ ] Start screenshot
- [x] or [ ] End screenshot
- [x] or [ ] Context setup
- [x] or [ ] Progress logging

**Screenshots Taken:**
1. `screenshot-name` - start
2. `screenshot-name` - middle
3. `screenshot-name` - end

**Code Alignment:**
- Routes tested: /path1, /path2
- Elements validated: header.app-header, .card, #element-id
- Functionality tested: Navigation, form submission, data display
- Alignment status: ✅ Aligned / ⚠️ Needs verification / ❌ Misaligned
- Notes: [Any specific alignment concerns]

**Issues:**
- Issue description 1
- Issue description 2

**Proposed Actions:**
- Specific action to fix issue 1
- Specific action to fix issue 2

---

(Repeat for each test)

## Recommendations

### High Priority
- [Number] tests missing environment setup
- [Number] tests missing screenshots
- [Number] tests with code alignment issues

### Code Alignment Issues
- Route X tested but not found in handlers
- Selector Y tested but not found in templates

### Best Practices
- All tests should have start + end screenshots
- Screenshot names should be descriptive
- Test names should clearly indicate what is tested
```

## Step 6: Be Direct and Specific

- No verbose explanations
- Be direct about issues
- Provide specific, actionable fixes
- Don't sugarcoat problems
- Focus on facts and compliance

## Step 7: Execute

Now perform the validation:
1. Read all test files in `./test/ui`
2. Validate each test against template
3. Check code alignment
4. Generate the complete report
5. Save to `./docs/tests/test-validation-<today's date>.md`

Begin the validation now.