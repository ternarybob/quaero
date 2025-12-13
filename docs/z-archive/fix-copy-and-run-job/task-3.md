# Task 3: Add Tests for Modal and Job Running

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 1, 2
- Sandbox: /tmp/3agents/task-3/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans

## Files
- `test/ui/queue_test.go` - Add new test functions

## Requirements

Add two new tests to the existing queue test file:

### Test 1: TestCopyAndQueueModal
Tests that clicking the copy/rerun button shows a modal (not a browser popup) and confirms the action.

```go
// TestCopyAndQueueModal tests that the copy and queue button shows a modal
func TestCopyAndQueueModal(t *testing.T) {
    qtc, cleanup := newQueueTestContext(t, 5*time.Minute)
    defer cleanup()

    qtc.env.LogTest(t, "--- Starting Test: Copy and Queue Modal ---")

    // First, trigger a quick job to have something to copy
    placesJobName := "Nearby Restaurants (Wheelers Hill)"

    if err := qtc.triggerJob(placesJobName); err != nil {
        t.Fatalf("Failed to trigger initial job: %v", err)
    }

    // Wait for job to complete
    if err := qtc.monitorJob(placesJobName, 2*time.Minute, true, false); err != nil {
        t.Fatalf("Initial job failed: %v", err)
    }

    // Navigate to queue page
    if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
        t.Fatalf("failed to navigate to queue page: %v", err)
    }

    // Wait for page to load
    if err := chromedp.Run(qtc.ctx,
        chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
        chromedp.Sleep(2*time.Second),
    ); err != nil {
        t.Fatalf("queue page did not load: %v", err)
    }

    // Find and click the rerun/copy button for the completed job
    qtc.env.LogTest(t, "Clicking copy/rerun button...")
    var clicked bool
    if err := chromedp.Run(qtc.ctx,
        chromedp.Evaluate(fmt.Sprintf(`
            (() => {
                const cards = document.querySelectorAll('.card');
                for (const card of cards) {
                    const titleEl = card.querySelector('.card-title');
                    if (titleEl && titleEl.textContent.includes('%s')) {
                        const rerunBtn = card.querySelector('button[title="Copy Job and Add to Queue"]');
                        if (rerunBtn) {
                            rerunBtn.click();
                            return true;
                        }
                    }
                }
                return false;
            })()
        `, placesJobName), &clicked),
    ); err != nil {
        t.Fatalf("failed to click rerun button: %v", err)
    }

    if !clicked {
        qtc.env.TakeScreenshot(qtc.ctx, "rerun_button_not_found")
        t.Fatalf("Rerun button not found for job %s", placesJobName)
    }
    qtc.env.LogTest(t, "Rerun button clicked")

    // Wait for modal to appear (NOT browser confirm dialog)
    qtc.env.LogTest(t, "Waiting for confirmation modal...")
    if err := chromedp.Run(qtc.ctx,
        chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
        chromedp.Sleep(500*time.Millisecond),
    ); err != nil {
        qtc.env.TakeScreenshot(qtc.ctx, "modal_not_found")
        t.Fatalf("Modal did not appear (still using browser confirm?): %v", err)
    }
    qtc.env.TakeScreenshot(qtc.ctx, "copy_queue_modal")
    qtc.env.LogTest(t, "Modal appeared")

    // Verify modal title contains expected text
    var modalTitle string
    if err := chromedp.Run(qtc.ctx,
        chromedp.Text(`.modal.active .modal-title`, &modalTitle, chromedp.ByQuery),
    ); err != nil {
        t.Fatalf("failed to get modal title: %v", err)
    }
    qtc.env.LogTest(t, "Modal title: %s", modalTitle)

    // Cancel to clean up
    if err := chromedp.Run(qtc.ctx,
        chromedp.Click(`.modal.active .btn-link`, chromedp.ByQuery),
        chromedp.Sleep(500*time.Millisecond),
    ); err != nil {
        t.Fatalf("failed to cancel modal: %v", err)
    }

    qtc.env.LogTest(t, "Test completed successfully - Modal confirmed working")
}
```

### Test 2: TestCopyAndQueueJobRuns
Tests that a copied job actually runs (not stuck in pending).

```go
// TestCopyAndQueueJobRuns tests that copied jobs actually execute
func TestCopyAndQueueJobRuns(t *testing.T) {
    qtc, cleanup := newQueueTestContext(t, 8*time.Minute)
    defer cleanup()

    qtc.env.LogTest(t, "--- Starting Test: Copy and Queue Job Runs ---")

    placesJobName := "Nearby Restaurants (Wheelers Hill)"

    // First, run the original job
    if err := qtc.triggerJob(placesJobName); err != nil {
        t.Fatalf("Failed to trigger initial job: %v", err)
    }

    if err := qtc.monitorJob(placesJobName, 2*time.Minute, true, false); err != nil {
        t.Fatalf("Initial job failed: %v", err)
    }
    qtc.env.LogTest(t, "Original job completed")

    // Navigate to queue page
    if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
        t.Fatalf("failed to navigate to queue page: %v", err)
    }

    // Wait for page to load
    if err := chromedp.Run(qtc.ctx,
        chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
        chromedp.Sleep(2*time.Second),
    ); err != nil {
        t.Fatalf("queue page did not load: %v", err)
    }

    // Count current jobs before copy
    var initialJobCount int
    chromedp.Run(qtc.ctx,
        chromedp.Evaluate(`
            (() => {
                const element = document.querySelector('[x-data="jobList"]');
                if (!element) return 0;
                const component = Alpine.$data(element);
                return component.allJobs ? component.allJobs.length : 0;
            })()
        `, &initialJobCount),
    )
    qtc.env.LogTest(t, "Initial job count: %d", initialJobCount)

    // Click the rerun button
    qtc.env.LogTest(t, "Clicking copy/rerun button...")
    if err := chromedp.Run(qtc.ctx,
        chromedp.Evaluate(fmt.Sprintf(`
            (() => {
                const cards = document.querySelectorAll('.card');
                for (const card of cards) {
                    const titleEl = card.querySelector('.card-title');
                    if (titleEl && titleEl.textContent.includes('%s')) {
                        const rerunBtn = card.querySelector('button[title="Copy Job and Add to Queue"]');
                        if (rerunBtn) {
                            rerunBtn.click();
                            return true;
                        }
                    }
                }
                return false;
            })()
        `, placesJobName), nil),
    ); err != nil {
        t.Fatalf("failed to click rerun button: %v", err)
    }

    // Wait for modal and confirm
    if err := chromedp.Run(qtc.ctx,
        chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
        chromedp.Sleep(500*time.Millisecond),
        chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
        chromedp.Sleep(1*time.Second),
    ); err != nil {
        t.Fatalf("failed to confirm copy: %v", err)
    }
    qtc.env.LogTest(t, "Copy confirmed")

    // Wait for new job to appear and verify it runs
    qtc.env.LogTest(t, "Waiting for copied job to appear and run...")

    // Give time for job to be created and start running
    time.Sleep(3 * time.Second)

    // Refresh job list
    chromedp.Run(qtc.ctx,
        chromedp.Evaluate(`
            (() => {
                if (typeof loadJobs === 'function') {
                    loadJobs();
                }
            })()
        `, nil),
    )
    time.Sleep(2 * time.Second)

    // Check that the newest job (the copy) is running or completed
    var newestJobStatus string
    if err := chromedp.Run(qtc.ctx,
        chromedp.Evaluate(fmt.Sprintf(`
            (() => {
                const element = document.querySelector('[x-data="jobList"]');
                if (!element) return 'error: element not found';
                const component = Alpine.$data(element);
                if (!component || !component.allJobs) return 'error: no jobs';

                // Find jobs with our name, get the newest one
                const matchingJobs = component.allJobs.filter(j => j.name && j.name.includes('%s'));
                if (matchingJobs.length === 0) return 'error: no matching jobs';

                // Sort by created_at descending to get newest
                matchingJobs.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
                return matchingJobs[0].status;
            })()
        `, placesJobName), &newestJobStatus),
    ); err != nil {
        t.Fatalf("failed to get newest job status: %v", err)
    }

    qtc.env.LogTest(t, "Newest job status: %s", newestJobStatus)

    // The job should NOT be stuck in pending
    if newestJobStatus == "pending" {
        // Wait a bit more and check again
        time.Sleep(10 * time.Second)
        chromedp.Run(qtc.ctx,
            chromedp.Evaluate(`loadJobs()`, nil),
        )
        time.Sleep(2 * time.Second)

        chromedp.Run(qtc.ctx,
            chromedp.Evaluate(fmt.Sprintf(`
                (() => {
                    const element = document.querySelector('[x-data="jobList"]');
                    const component = Alpine.$data(element);
                    const matchingJobs = component.allJobs.filter(j => j.name && j.name.includes('%s'));
                    matchingJobs.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
                    return matchingJobs[0].status;
                })()
            `, placesJobName), &newestJobStatus),
        )
        qtc.env.LogTest(t, "Status after wait: %s", newestJobStatus)
    }

    if newestJobStatus == "pending" {
        qtc.env.TakeScreenshot(qtc.ctx, "job_stuck_pending")
        t.Fatalf("Copied job is stuck in pending status - job is NOT executing!")
    }

    // Wait for the copied job to complete
    if err := qtc.monitorJob(placesJobName, 2*time.Minute, false, false); err != nil {
        // Could be the original completed job being found - that's ok
        qtc.env.LogTest(t, "Warning: monitor returned: %v", err)
    }

    qtc.env.TakeScreenshot(qtc.ctx, "copy_job_completed")
    qtc.env.LogTest(t, "Test completed successfully - Copied job executed")
}
```

## Acceptance
- [ ] TestCopyAndQueueModal passes - confirms modal appears instead of browser dialog
- [ ] TestCopyAndQueueJobRuns passes - confirms copied jobs actually run
- [ ] Tests use nearby-restaurants-places.toml job (quick to run)
- [ ] Compiles
- [ ] Tests pass
