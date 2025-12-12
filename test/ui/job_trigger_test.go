// job_trigger_test.go - Job triggering and cancellation tests
// Tests the core job workflow: trigger, monitor, cancel

package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJobTrigger tests basic job triggering via the UI
func TestJobTrigger(t *testing.T) {
	utc := NewUITestContext(t, 5*time.Minute)
	defer utc.Cleanup()

	jobName := "News Crawler"

	utc.Log("--- Testing Job Trigger ---")

	// Trigger the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Navigate to queue and verify job appeared
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to queue: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Check job exists in queue
	var jobFound bool
	err := chromedp.Run(utc.Ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return false;
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return false;
					return component.allJobs.some(j => j.name && j.name.includes('%s'));
				})()
			`, jobName),
			&jobFound,
			chromedp.WithPollingTimeout(10*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)

	if err != nil || !jobFound {
		t.Fatalf("Job %s not found in queue after triggering", jobName)
	}

	utc.Screenshot("job_triggered_in_queue")
	utc.Log("✓ Job triggered and appeared in queue")
}

// TestJobCancel tests cancelling a running job via the Queue UI
func TestJobCancel(t *testing.T) {
	utc := NewUITestContext(t, 5*time.Minute)
	defer utc.Cleanup()

	jobName := "News Crawler"

	utc.Log("--- Testing Job Cancel ---")

	// Trigger the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Navigate to Queue page
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to queue: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Wait for job to start running
	utc.Log("Waiting for job to start running...")
	var jobRunning bool
	runningErr := chromedp.Run(utc.Ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge && statusBadge.getAttribute('data-status') === 'running') {
								return true;
							}
						}
					}
					return false;
				})()
			`, jobName),
			&jobRunning,
			chromedp.WithPollingTimeout(30*time.Second),
			chromedp.WithPollingInterval(500*time.Millisecond),
		),
	)
	if runningErr != nil {
		utc.Screenshot("job_not_running")
		t.Fatalf("Job did not start running within 30s: %v", runningErr)
	}
	utc.Log("✓ Job is running")
	utc.Screenshot("job_running")

	// Find and click the cancel button
	utc.Log("Clicking cancel button...")
	cancelSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]/ancestor::div[contains(@class, "card")]//button[contains(@class, "btn-error") or contains(text(), "Cancel")]`, jobName)
	if err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(cancelSelector),
		chromedp.Click(cancelSelector),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		utc.Screenshot("cancel_button_not_found")
		t.Fatalf("Failed to click cancel button: %v", err)
	}
	utc.Screenshot("cancel_clicked")

	// Wait for job to be cancelled
	utc.Log("Waiting for job to be cancelled...")
	var jobCancelled bool
	cancelledErr := chromedp.Run(utc.Ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge && statusBadge.getAttribute('data-status') === 'cancelled') {
								return true;
							}
						}
					}
					return false;
				})()
			`, jobName),
			&jobCancelled,
			chromedp.WithPollingTimeout(30*time.Second),
			chromedp.WithPollingInterval(500*time.Millisecond),
		),
	)

	if cancelledErr != nil || !jobCancelled {
		utc.Screenshot("job_not_cancelled")
		t.Fatalf("Job was not cancelled within 30s")
	}

	utc.Screenshot("job_cancelled")
	utc.Log("✓ Job cancelled successfully")
}

