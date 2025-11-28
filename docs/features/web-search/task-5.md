# Task 5: Create UI Test

- Group: 5 | Mode: sequential | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 4
- Sandbox: /tmp/3agents/task-5/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans\web-search

## Files
- `test/ui/queue_test.go` - Add TestWebSearchJob function

## Requirements

Add a new test function to test the web search job:

```go
// TestWebSearchJob tests that the web search job executes and produces results
func TestWebSearchJob(t *testing.T) {
    qtc, cleanup := newQueueTestContext(t, 10*time.Minute)
    defer cleanup()

    qtc.env.LogTest(t, "--- Starting Test: Web Search Job ---")

    jobName := "Web Search: ASX:GNP Company Info"

    // Trigger the job
    if err := qtc.triggerJob(jobName); err != nil {
        t.Fatalf("Failed to trigger web search job: %v", err)
    }

    // Monitor job completion (allow 5 minutes for web search)
    if err := qtc.monitorJob(jobName, 5*time.Minute, true, false); err != nil {
        t.Fatalf("Web search job failed: %v", err)
    }
    qtc.env.LogTest(t, "✓ Web search job completed")

    // Verify a document was created
    // Navigate to documents page and search for results
    if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.documentsURL)); err != nil {
        t.Fatalf("failed to navigate to documents page: %v", err)
    }

    // Wait for page and search for web_search documents
    if err := chromedp.Run(qtc.ctx,
        chromedp.WaitVisible(`#search-input`, chromedp.ByQuery),
        chromedp.SetValue(`#search-input`, "ASX:GNP", chromedp.ByQuery),
        chromedp.Click(`#search-button`, chromedp.ByQuery),
        chromedp.Sleep(2*time.Second),
    ); err != nil {
        t.Fatalf("failed to search for documents: %v", err)
    }

    // Check if results exist
    var resultCount int
    if err := chromedp.Run(qtc.ctx,
        chromedp.Evaluate(`document.querySelectorAll('.document-card').length`, &resultCount),
    ); err != nil {
        t.Fatalf("failed to count results: %v", err)
    }

    if resultCount == 0 {
        qtc.env.TakeScreenshot(qtc.ctx, "web_search_no_results")
        t.Fatalf("Expected at least one document from web search, got 0")
    }

    qtc.env.TakeScreenshot(qtc.ctx, "web_search_results")
    qtc.env.LogTest(t, "✓ Test completed successfully - Found %d documents", resultCount)
}
```

## Acceptance
- [ ] Test function compiles
- [ ] Triggers web search job
- [ ] Monitors job for up to 5 minutes
- [ ] Verifies document was created
- [ ] Takes screenshots for debugging
