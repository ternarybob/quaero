package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestCrawlerJobDeletion tests the complete crawler job deletion workflow
func TestCrawlerJobDeletion(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestCrawlerJobDeletion")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestCrawlerJobDeletion")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestCrawlerJobDeletion (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestCrawlerJobDeletion (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second) // Shorter timeout for deletion test
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Navigate to jobs page and find any crawler job
	env.LogTest(t, "Step 1: Navigating to jobs page to find a crawler job...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for page and data to load
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}
	env.LogTest(t, "✓ Jobs page loaded")

	// Find any crawler job (look for jobs with "Crawler" in the name)
	var crawlerJobFound struct {
		Found bool   `json:"found"`
		Name  string `json:"name"`
	}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const crawlerCard = cards.find(card => card.textContent.toLowerCase().includes('crawler'));
				if (crawlerCard) {
					const titleElement = crawlerCard.querySelector('.card-title, h3, h4, .title');
					const name = titleElement ? titleElement.textContent.trim() : 'Unknown Crawler';
					return { found: true, name: name };
				}
				return { found: false, name: '' };
			})()
		`, &crawlerJobFound),
	)

	if err != nil || !crawlerJobFound.Found {
		env.LogTest(t, "ERROR: No crawler job found in job definitions list")
		env.TakeScreenshot(ctx, "crawler-job-not-found")
		t.Fatal("A crawler job should be available in job definitions list")
	}

	env.LogTest(t, "✓ Crawler job found: %s", crawlerJobFound.Name)
	env.TakeScreenshot(ctx, "crawler-job-found-for-deletion")

	// Step 2: Execute the crawler job first (to have something to delete)
	env.LogTest(t, "Step 2: Executing the crawler job to create a job instance...")

	// Wait for WebSocket connection
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		env.TakeScreenshot(ctx, "websocket-failed")
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Override confirm dialog and execute
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to override confirm dialog: %v", err)
		t.Fatalf("Failed to override confirm dialog: %v", err)
	}

	// Find and click the run button for the crawler job
	var runButtonClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const crawlerCard = cards.find(card => card.textContent.toLowerCase().includes('crawler'));
				if (crawlerCard) {
					const runButton = crawlerCard.querySelector('button.btn-success');
					if (runButton) {
						runButton.click();
						return true;
					}
				}
				return false;
			})()
		`, &runButtonClicked),
		chromedp.Sleep(2*time.Second), // Wait for job to be triggered
	)

	if err != nil || !runButtonClicked {
		env.LogTest(t, "ERROR: Failed to execute crawler job")
		env.TakeScreenshot(ctx, "run-button-click-failed")
		t.Fatal("Failed to execute crawler job")
	}

	env.LogTest(t, "✓ Crawler job execution triggered")

	// Step 3: Navigate to queue page and find the running job
	env.LogTest(t, "Step 3: Navigating to queue page to find the running job...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load queue page: %v", err)
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Initialize filters and load jobs
	var loadResult struct {
		Success      bool   `json:"success"`
		ErrorMessage string `json:"errorMessage"`
		JobsLoaded   int    `json:"jobsLoaded"`
	}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(async () => {
				try {
					if (!window.activeFilters) {
						window.activeFilters = {
							status: new Set(['pending', 'running', 'completed', 'failed', 'cancelled']),
							source: new Set(),
							entity: new Set()
						};
					}
					const jobListEl = document.querySelector('[x-data="jobList"]');
					if (!jobListEl) {
						return { success: false, errorMessage: 'jobList element not found', jobsLoaded: 0 };
					}
					const alpineData = Alpine.$data(jobListEl);
					if (!alpineData || !alpineData.loadJobs) {
						return { success: false, errorMessage: 'loadJobs method not found', jobsLoaded: 0 };
					}
					await alpineData.loadJobs();
					return { success: true, errorMessage: '', jobsLoaded: alpineData.allJobs ? alpineData.allJobs.length : 0 };
				} catch (e) {
					return { success: false, errorMessage: e.toString(), jobsLoaded: 0 };
				}
			})()
		`, &loadResult, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Sleep(3*time.Second),
	)

	if loadResult.Success {
		env.LogTest(t, "✓ Jobs loaded successfully (%d jobs)", loadResult.JobsLoaded)
	} else {
		env.LogTest(t, "WARNING: loadJobs() failed: %s", loadResult.ErrorMessage)
	}

	// Step 4: Find the crawler job in the queue
	env.LogTest(t, "Step 4: Finding crawler job in queue...")

	var jobDetails struct {
		Found  bool   `json:"found"`
		JobID  string `json:"jobId"`
		Status string `json:"status"`
		Name   string `json:"name"`
	}

	// Look for the job (it might be running or completed)
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for jobs to load
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const crawlerJob = jobCards.find(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.toLowerCase().includes('crawler');
				});

				if (!crawlerJob) {
					return { found: false, jobId: '', status: '', name: '' };
				}

				const jobId = crawlerJob.getAttribute('data-job-id') || '';
				const statusBadge = crawlerJob.querySelector('.label');
				const status = statusBadge ? statusBadge.textContent.trim() : '';
				const titleElement = crawlerJob.querySelector('.card-title');
				const name = titleElement ? titleElement.textContent.trim() : '';

				return {
					found: true,
					jobId: jobId,
					status: status,
					name: name
				};
			})()
		`, &jobDetails),
	)

	if err != nil || !jobDetails.Found {
		env.LogTest(t, "ERROR: Crawler job not found in queue")
		env.TakeScreenshot(ctx, "crawler-job-not-found-for-deletion")
		t.Fatal("Crawler job should appear in queue after execution")
	}

	env.LogTest(t, "✓ Crawler job found in queue: %s", jobDetails.Name)
	env.LogTest(t, "  Job ID: %s", jobDetails.JobID)
	env.LogTest(t, "  Status: %s", jobDetails.Status)
	env.TakeScreenshot(ctx, "crawler-job-found-for-deletion")

	// Step 5: Navigate to job details page to delete the job
	env.LogTest(t, "Step 5: Navigating to job details page to delete the job...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("%s/job?id=%s", baseURL, jobDetails.JobID)),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for Alpine.js to load data
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to job details: %v", err)
		t.Fatalf("Failed to navigate to job details: %v", err)
	}

	env.LogTest(t, "✓ Successfully navigated to job details page")

	// Step 6: Check if job is running and cancel it first if needed
	env.LogTest(t, "Step 6: Checking job status and canceling if running...")

	var jobStatusResult struct {
		IsRunning     bool `json:"isRunning"`
		CancelClicked bool `json:"cancelClicked"`
		DeleteFound   bool `json:"deleteFound"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Check if job is running by looking for cancel button
				let cancelButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.includes('Cancel Job') && btn.classList.contains('btn-error')
				);
				
				let deleteButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.includes('Delete Job') && btn.classList.contains('btn-error')
				);

				let cancelClicked = false;
				if (cancelButton && cancelButton.offsetParent !== null) {
					// Job is running, cancel it first
					cancelButton.click();
					cancelClicked = true;
				}

				return {
					isRunning: !!cancelButton && cancelButton.offsetParent !== null,
					cancelClicked: cancelClicked,
					deleteFound: !!deleteButton && deleteButton.offsetParent !== null
				};
			})()
		`, &jobStatusResult),
		chromedp.Sleep(3*time.Second), // Wait for cancel operation if performed
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check job status: %v", err)
	} else {
		if jobStatusResult.IsRunning {
			env.LogTest(t, "✓ Job was running, cancel button clicked")
			if jobStatusResult.CancelClicked {
				env.LogTest(t, "✓ Job cancellation triggered")
				// Wait a bit more for the job to be cancelled and page to update
				err = chromedp.Run(ctx,
					chromedp.Sleep(5*time.Second),
					chromedp.Evaluate(`location.reload()`, nil), // Refresh to get updated status
					chromedp.Sleep(3*time.Second),
				)
				if err != nil {
					env.LogTest(t, "WARNING: Failed to refresh page after cancel: %v", err)
				}
			}
		} else {
			env.LogTest(t, "✓ Job was not running (completed or failed)")
		}

		if jobStatusResult.DeleteFound {
			env.LogTest(t, "✓ Delete button found")
		} else {
			env.LogTest(t, "WARNING: Delete button not immediately visible")
		}
	}

	env.TakeScreenshot(ctx, "job-status-checked")

	// Step 7: Delete the job
	env.LogTest(t, "Step 7: Deleting the crawler job...")

	var deleteResult struct {
		DeleteButtonFound bool `json:"deleteButtonFound"`
		DeleteClicked     bool `json:"deleteClicked"`
	}

	// First, override any confirmation dialogs
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Override confirm dialog to always return true
			window.confirm = function(message) { 
				console.log('Confirm dialog intercepted:', message); 
				return true; 
			};
		`, nil),
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to override confirm dialog: %v", err)
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for delete button by text and class
				let deleteButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.includes('Delete Job') && btn.classList.contains('btn-error')
				);

				if (!deleteButton) {
					// Debug: log all available buttons
					const allButtons = Array.from(document.querySelectorAll('button')).map(btn => ({
						id: btn.id || 'no-id',
						text: btn.textContent.trim(),
						classes: Array.from(btn.classList).join(' '),
						visible: btn.offsetParent !== null
					}));
					console.log('Available buttons:', allButtons);
					return { deleteButtonFound: false, deleteClicked: false };
				}

				// Check if button is visible
				if (deleteButton.offsetParent === null) {
					console.log('Delete button found but not visible');
					return { deleteButtonFound: false, deleteClicked: false };
				}

				// Click delete button
				console.log('Clicking delete button with ID:', deleteButton.id);
				deleteButton.click();
				return { deleteButtonFound: true, deleteClicked: true };
			})()
		`, &deleteResult),
		chromedp.Sleep(5*time.Second), // Wait longer for delete operation
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to execute delete button check: %v", err)
		env.TakeScreenshot(ctx, "delete-button-check-failed")
		t.Fatalf("Failed to execute delete button check: %v", err)
	}

	if !deleteResult.DeleteButtonFound {
		env.LogTest(t, "ERROR: Delete button not found")

		// Get console logs to see what buttons were found
		var consoleOutput string
		chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const allButtons = Array.from(document.querySelectorAll('button')).map(btn => ({
						text: btn.textContent.trim(),
						classes: Array.from(btn.classList).join(' '),
						visible: btn.offsetParent !== null,
						onclick: btn.getAttribute('x-on:click') || btn.getAttribute('@click') || 'none'
					}));
					return JSON.stringify(allButtons, null, 2);
				})()
			`, &consoleOutput),
		)
		env.LogTest(t, "Available buttons: %s", consoleOutput)

		env.TakeScreenshot(ctx, "delete-button-not-found")
		t.Fatal("Delete button should be available for completed/cancelled jobs")
	}

	if !deleteResult.DeleteClicked {
		env.LogTest(t, "ERROR: Failed to click delete button")
		env.TakeScreenshot(ctx, "delete-button-click-failed")
		t.Fatal("Delete button should be clickable")
	}

	env.LogTest(t, "✓ Delete button clicked successfully")

	// Step 8: Verify job deletion by checking if we're redirected or if job is gone
	env.LogTest(t, "Step 8: Verifying job deletion...")

	// Wait for potential redirect or page update
	err = chromedp.Run(ctx,
		chromedp.Sleep(3*time.Second),
	)

	// Check current URL to see if we were redirected
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to get current URL: %v", err)
	} else {
		env.LogTest(t, "Current URL after deletion: %s", currentURL)
		if strings.Contains(currentURL, "/queue") {
			env.LogTest(t, "✓ Redirected to queue page after deletion")
		} else if strings.Contains(currentURL, "/job?id=") {
			env.LogTest(t, "Job details page still showing - checking for error message or job status")
		}
	}

	env.TakeScreenshot(ctx, "after-job-deletion")

	// Step 9: Navigate to queue page and verify job is no longer listed
	env.LogTest(t, "Step 9: Verifying job is removed from queue listing...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to queue page: %v", err)
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Reload jobs to get fresh data
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(async () => {
				try {
					const jobListEl = document.querySelector('[x-data="jobList"]');
					if (jobListEl) {
						const alpineData = Alpine.$data(jobListEl);
						if (alpineData && alpineData.loadJobs) {
							await alpineData.loadJobs();
						}
					}
				} catch (e) {
					console.error('Failed to reload jobs:', e);
				}
			})()
		`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Sleep(3*time.Second),
	)

	// Check if the deleted job still appears in the queue
	var jobStillExists struct {
		Found bool   `json:"found"`
		JobID string `json:"jobId"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const deletedJob = jobCards.find(card => {
					const jobId = card.getAttribute('data-job-id');
					return jobId === '%s';
				});

				return {
					found: !!deletedJob,
					jobId: deletedJob ? deletedJob.getAttribute('data-job-id') : ''
				};
			})()
		`, jobDetails.JobID), &jobStillExists),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check if job still exists: %v", err)
	} else {
		if jobStillExists.Found {
			env.LogTest(t, "❌ FAILURE: Deleted job still appears in queue listing")
			env.LogTest(t, "  This indicates the delete operation failed")
			env.TakeScreenshot(ctx, "deleted-job-still-exists")
			t.Errorf("Job deletion failed - job %s still exists in queue", jobDetails.JobID)
		} else {
			env.LogTest(t, "✓ Deleted job no longer appears in queue listing")
		}
	}

	// Step 10: Verify job definition still exists (only the job instance should be deleted)
	env.LogTest(t, "Step 10: Verifying job definition still exists...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to jobs page: %v", err)
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}

	// Check if crawler job definition still exists
	var jobDefinitionExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				return cards.some(card => card.textContent.toLowerCase().includes('crawler'));
			})()
		`, &jobDefinitionExists),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check job definition existence: %v", err)
	} else {
		if jobDefinitionExists {
			env.LogTest(t, "✓ Crawler job definition still exists (correct behavior)")
		} else {
			env.LogTest(t, "ERROR: Crawler job definition was incorrectly deleted")
			t.Error("Job definition should not be deleted when deleting a job instance")
		}
	}

	env.TakeScreenshot(ctx, "job-definition-after-deletion")
	env.TakeScreenshot(ctx, "crawler-deletion-test-completed")
	env.LogTest(t, "✅ Crawler job deletion test completed")
}

// TestMultipleCrawlerJobDeletion tests deletion of multiple crawler jobs
func TestMultipleCrawlerJobDeletion(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestMultipleCrawlerJobDeletion")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestMultipleCrawlerJobDeletion")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestMultipleCrawlerJobDeletion (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestMultipleCrawlerJobDeletion (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 180*time.Second) // Extended timeout for multiple jobs
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Create multiple crawler job instances
	env.LogTest(t, "Step 1: Creating multiple crawler job instances...")

	// Navigate to jobs page
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to jobs page: %v", err)
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}

	// Wait for WebSocket connection
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Override confirm dialog
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to override confirm dialog: %v", err)
		t.Fatalf("Failed to override confirm dialog: %v", err)
	}

	// Execute crawler job multiple times (3 times)
	jobCount := 3
	for i := 0; i < jobCount; i++ {
		env.LogTest(t, "Creating job instance %d/%d...", i+1, jobCount)

		var runButtonClicked bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
					const crawlerCard = cards.find(card => card.textContent.toLowerCase().includes('crawler'));
					if (crawlerCard) {
						const runButton = crawlerCard.querySelector('button.btn-success');
						if (runButton) {
							runButton.click();
							return true;
						}
					}
					return false;
				})()
			`, &runButtonClicked),
			chromedp.Sleep(2*time.Second), // Wait between job executions
		)

		if err != nil || !runButtonClicked {
			env.LogTest(t, "WARNING: Failed to execute crawler job instance %d", i+1)
		} else {
			env.LogTest(t, "✓ Crawler job instance %d triggered", i+1)
		}
	}

	// Step 2: Navigate to queue and verify multiple jobs exist
	env.LogTest(t, "Step 2: Verifying multiple crawler jobs in queue...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load queue page: %v", err)
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Load jobs
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(async () => {
				try {
					if (!window.activeFilters) {
						window.activeFilters = {
							status: new Set(['pending', 'running', 'completed', 'failed', 'cancelled']),
							source: new Set(),
							entity: new Set()
						};
					}
					const jobListEl = document.querySelector('[x-data="jobList"]');
					if (jobListEl) {
						const alpineData = Alpine.$data(jobListEl);
						if (alpineData && alpineData.loadJobs) {
							await alpineData.loadJobs();
						}
					}
				} catch (e) {
					console.error('Failed to load jobs:', e);
				}
			})()
		`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Sleep(3*time.Second),
	)

	// Count crawler jobs
	var crawlerJobsCount struct {
		Count  int      `json:"count"`
		JobIDs []string `json:"jobIds"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const crawlerJobs = jobCards.filter(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.toLowerCase().includes('crawler');
				});

				const jobIds = crawlerJobs.map(card => card.getAttribute('data-job-id')).filter(id => id);

				return {
					count: crawlerJobs.length,
					jobIds: jobIds
				};
			})()
		`, &crawlerJobsCount),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to count crawler jobs: %v", err)
	} else {
		env.LogTest(t, "✓ Found %d crawler jobs in queue", crawlerJobsCount.Count)
		if crawlerJobsCount.Count < 2 {
			env.LogTest(t, "WARNING: Expected at least 2 crawler jobs, found %d", crawlerJobsCount.Count)
		}
	}

	env.TakeScreenshot(ctx, "multiple-crawler-jobs-created")

	// Step 3: Delete each crawler job one by one
	env.LogTest(t, "Step 3: Deleting crawler jobs one by one...")

	deletedCount := 0
	for i, jobID := range crawlerJobsCount.JobIDs {
		if jobID == "" {
			continue
		}

		env.LogTest(t, "Deleting job %d/%d (ID: %s)...", i+1, len(crawlerJobsCount.JobIDs), jobID)

		// Navigate to job details page
		err = chromedp.Run(ctx,
			chromedp.Navigate(fmt.Sprintf("%s/job?id=%s", baseURL, jobID)),
			chromedp.WaitVisible(`body`, chromedp.ByQuery),
			chromedp.Sleep(3*time.Second),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to navigate to job %s details: %v", jobID, err)
			continue
		}

		// Try to delete the job
		var deleteResult struct {
			DeleteButtonFound bool `json:"deleteButtonFound"`
			DeleteClicked     bool `json:"deleteClicked"`
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					// Look for delete button
					let deleteButton = Array.from(document.querySelectorAll('button')).find(btn =>
						btn.textContent.includes('Delete Job') && btn.classList.contains('btn-error')
					);

					if (!deleteButton || deleteButton.offsetParent === null) {
						return { deleteButtonFound: false, deleteClicked: false };
					}

					// Click delete button
					deleteButton.click();
					return { deleteButtonFound: true, deleteClicked: true };
				})()
			`, &deleteResult),
			chromedp.Sleep(3*time.Second),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to delete job %s: %v", jobID, err)
		} else if deleteResult.DeleteButtonFound && deleteResult.DeleteClicked {
			env.LogTest(t, "✓ Delete button clicked for job %s", jobID)
			deletedCount++
		} else {
			env.LogTest(t, "WARNING: Delete button not found or not clickable for job %s", jobID)
		}
	}

	env.LogTest(t, "✓ Attempted to delete %d jobs", deletedCount)

	// Step 4: Verify all jobs are deleted from queue
	env.LogTest(t, "Step 4: Verifying jobs are removed from queue...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to queue page: %v", err)
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Reload jobs
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(async () => {
				try {
					const jobListEl = document.querySelector('[x-data="jobList"]');
					if (jobListEl) {
						const alpineData = Alpine.$data(jobListEl);
						if (alpineData && alpineData.loadJobs) {
							await alpineData.loadJobs();
						}
					}
				} catch (e) {
					console.error('Failed to reload jobs:', e);
				}
			})()
		`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Sleep(3*time.Second),
	)

	// Check remaining crawler jobs
	var remainingJobs struct {
		Count           int      `json:"count"`
		RemainingJobIDs []string `json:"remainingJobIds"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const crawlerJobs = jobCards.filter(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.toLowerCase().includes('crawler');
				});

				const jobIds = crawlerJobs.map(card => card.getAttribute('data-job-id')).filter(id => id);

				return {
					count: crawlerJobs.length,
					remainingJobIds: jobIds
				};
			})()
		`, &remainingJobs),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check remaining jobs: %v", err)
	} else {
		env.LogTest(t, "Remaining crawler jobs in queue: %d", remainingJobs.Count)
		if remainingJobs.Count > 0 {
			env.LogTest(t, "❌ FAILURE: %d crawler jobs still exist after deletion", remainingJobs.Count)
			env.LogTest(t, "  Remaining job IDs: %v", remainingJobs.RemainingJobIDs)
			env.TakeScreenshot(ctx, "jobs-not-deleted")
			t.Errorf("Job deletion failed - %d jobs still exist in queue", remainingJobs.Count)
		} else {
			env.LogTest(t, "✓ All crawler jobs successfully deleted from queue")
		}
	}

	env.TakeScreenshot(ctx, "multiple-crawler-deletion-completed")
	env.LogTest(t, "✅ Multiple crawler job deletion test completed")
}
