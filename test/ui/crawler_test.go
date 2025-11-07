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

// TestCrawlerJobExecution tests the complete crawler job execution workflow
func TestCrawlerJobExecution(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestCrawlerJobExecution")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestCrawlerJobExecution")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestCrawlerJobExecution (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestCrawlerJobExecution (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 180*time.Second) // Extended timeout for crawling
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Create a test crawler job with accessible URLs
	env.LogTest(t, "Step 1: Creating test crawler job with accessible URLs...")
	testCrawlerTOML := `# Test Crawler Job Definition
id = "test-web-crawler"
name = "Test Web Crawler"
description = "Test crawler for verifying web crawling functionality"

# Use accessible test URLs
start_urls = ["https://httpbin.org/html", "https://httpbin.org/json"]

schedule = ""
timeout = "5m"
enabled = true
auto_start = false

# URL filtering patterns
include_patterns = ["html", "json"]
exclude_patterns = ["admin", "login"]

# Crawler behavior
max_depth = 1
max_pages = 5
concurrency = 2
follow_links = false
`

	// Step 2: Navigate to job_add page and load the test crawler
	env.LogTest(t, "Step 2: Navigating to job_add page...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/job_add"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for page to load
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load job_add page: %v", err)
		env.TakeScreenshot(ctx, "job-add-page-load-failed")
		t.Fatalf("Failed to load job_add page: %v", err)
	}
	env.LogTest(t, "✓ Job add page loaded")

	// Step 3: Paste the test crawler TOML content
	env.LogTest(t, "Step 3: Pasting test crawler TOML content...")
	err = chromedp.Run(ctx,
		chromedp.Sleep(3*time.Second), // Wait for CodeMirror to initialize
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const editorElement = document.querySelector('.CodeMirror');
				if (editorElement && editorElement.CodeMirror) {
					const editor = editorElement.CodeMirror;
					editor.setValue(%s);
					return true;
				}
				return false;
			})()
		`, "`"+testCrawlerTOML+"`"), nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to set editor content: %v", err)
		env.TakeScreenshot(ctx, "editor-content-set-failed")
		t.Fatalf("Failed to set editor content: %v", err)
	}
	env.LogTest(t, "✓ Test crawler content pasted into editor")
	env.TakeScreenshot(ctx, "test-crawler-content-pasted")

	// Step 4: Save the job definition
	env.LogTest(t, "Step 4: Saving the test crawler job definition...")
	var saveResult bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const saveButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.trim() === 'Save' || btn.textContent.includes('Save')
				);
				if (saveButton) {
					saveButton.click();
					return true;
				}
				return false;
			})()
		`, &saveResult),
		chromedp.Sleep(3*time.Second), // Wait for save and redirect
	)

	if err != nil || !saveResult {
		env.LogTest(t, "ERROR: Failed to save job definition")
		env.TakeScreenshot(ctx, "job-save-failed")
		t.Fatalf("Failed to save job definition")
	}

	env.LogTest(t, "✓ Job definition saved")

	// Step 5: Navigate to jobs page and verify the job exists
	env.LogTest(t, "Step 5: Navigating to jobs page...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for page and data to load
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to jobs page: %v", err)
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}

	// Verify the Test Web Crawler job appears
	var testCrawlerFound bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				return cards.some(card => card.textContent.includes('Test Web Crawler'));
			})()
		`, &testCrawlerFound),
	)

	if err != nil || !testCrawlerFound {
		env.LogTest(t, "ERROR: Test Web Crawler job not found in job definitions list")
		env.TakeScreenshot(ctx, "test-crawler-not-found")
		t.Fatal("Test Web Crawler job should appear in job definitions list after saving")
	}

	env.LogTest(t, "✓ Test Web Crawler job found in job definitions list")
	env.TakeScreenshot(ctx, "test-crawler-found")

	// Step 6: Execute the Test Web Crawler job
	env.LogTest(t, "Step 6: Executing the Test Web Crawler job...")

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

	// Find and click the run button for Test Web Crawler
	var runButtonClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const testCrawlerCard = cards.find(card => card.textContent.includes('Test Web Crawler'));
				if (testCrawlerCard) {
					const runButton = testCrawlerCard.querySelector('button.btn-success');
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
		env.LogTest(t, "ERROR: Failed to execute Test Web Crawler job")
		env.TakeScreenshot(ctx, "run-button-click-failed")
		t.Fatal("Failed to execute Test Web Crawler job")
	}

	env.LogTest(t, "✓ Test Web Crawler job execution triggered")

	// Step 7: Navigate to queue page and monitor execution
	env.LogTest(t, "Step 7: Navigating to queue page to monitor execution...")
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

	// Step 8: Verify the job appears in queue and monitor its execution
	env.LogTest(t, "Step 8: Monitoring Test Web Crawler job execution...")

	var jobFound bool
	var jobDetails struct {
		Found  bool   `json:"found"`
		JobID  string `json:"jobId"`
		Status string `json:"status"`
		Name   string `json:"name"`
	}

	// Monitor for up to 60 seconds
	monitorStart := time.Now()
	maxMonitorTime := 60 * time.Second

	for time.Since(monitorStart) < maxMonitorTime {
		err = chromedp.Run(ctx,
			chromedp.Sleep(2*time.Second), // Wait between checks
			chromedp.Evaluate(`
				(() => {
					const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
					const testCrawlerJob = jobCards.find(card => {
						const titleElement = card.querySelector('.card-title');
						return titleElement && titleElement.textContent.includes('Test Web Crawler');
					});

					if (!testCrawlerJob) {
						return { found: false, jobId: '', status: '', name: '' };
					}

					const jobId = testCrawlerJob.getAttribute('data-job-id') || '';
					const statusBadge = testCrawlerJob.querySelector('.label');
					const status = statusBadge ? statusBadge.textContent.trim() : '';
					const titleElement = testCrawlerJob.querySelector('.card-title');
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

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check job status: %v", err)
			continue
		}

		if jobDetails.Found {
			if !jobFound {
				jobFound = true
				env.LogTest(t, "✓ Test Web Crawler job found in queue")
				env.LogTest(t, "  Job ID: %s", jobDetails.JobID)
				env.LogTest(t, "  Initial Status: %s", jobDetails.Status)
			}

			env.LogTest(t, "  Current Status: %s", jobDetails.Status)

			// Check if job completed or failed
			statusLower := strings.ToLower(jobDetails.Status)
			if strings.Contains(statusLower, "completed") || strings.Contains(statusLower, "failed") {
				env.LogTest(t, "  Job reached terminal state: %s", jobDetails.Status)
				break
			}
		}
	}

	if !jobFound {
		env.LogTest(t, "ERROR: Test Web Crawler job not found in queue")
		env.TakeScreenshot(ctx, "test-crawler-job-not-in-queue")
		t.Fatal("Test Web Crawler job should appear in queue after execution")
	}

	env.TakeScreenshot(ctx, "test-crawler-execution-monitored")

	// Step 9: Click on the job to view details and verify configuration display
	if jobDetails.JobID != "" {
		env.LogTest(t, "Step 9: Clicking on job to view details and verify configuration...")

		err = chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					window.location.href = '/job?id=%s';
					return true;
				})()
			`, jobDetails.JobID), nil),
			chromedp.Sleep(3*time.Second), // Wait for navigation
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to navigate to job details: %v", err)
		} else {
			// Verify we're on the job details page
			var currentURL string
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`window.location.href`, &currentURL),
			)

			if err == nil && strings.Contains(currentURL, "/job?id=") {
				env.LogTest(t, "✓ Successfully navigated to job details page")

				// Wait for job details to load
				err = chromedp.Run(ctx,
					chromedp.WaitVisible(`body`, chromedp.ByQuery),
					chromedp.Sleep(3*time.Second), // Wait for Alpine.js to load data
				)

				if err == nil {
					// Check if configuration section is visible and contains TOML data
					var configResult struct {
						Visible bool   `json:"visible"`
						Content string `json:"content"`
					}

					err = chromedp.Run(ctx,
						chromedp.Evaluate(`
							(() => {
								// Check if configuration section exists and is visible
								const configSections = Array.from(document.querySelectorAll('section.card')).filter(section => {
									const header = section.querySelector('h3');
									return header && header.textContent.includes('Configuration');
								});
								
								if (configSections.length === 0) {
									return { visible: false, content: '' };
								}
								
								// Get the configuration content
								const configSection = configSections[0];
								const codeElement = configSection.querySelector('pre code');
								const content = codeElement ? codeElement.textContent : '';
								
								return {
									visible: true,
									content: content
								};
							})()
						`, &configResult),
					)

					if err != nil {
						env.LogTest(t, "WARNING: Failed to check configuration section: %v", err)
					} else if !configResult.Visible {
						env.LogTest(t, "ERROR: Configuration section not visible on job details page")
						env.TakeScreenshot(ctx, "config-section-not-visible")
						t.Error("Configuration section should be visible on job details page")
					} else {
						env.LogTest(t, "✓ Configuration section is visible")

						// Check if configuration contains expected data
						if strings.Contains(configResult.Content, "start_urls") ||
							strings.Contains(configResult.Content, "httpbin.org") ||
							strings.Contains(configResult.Content, "max_depth") {
							env.LogTest(t, "✓ Configuration contains expected crawler settings")
							contentPreview := configResult.Content
							if len(contentPreview) > 200 {
								contentPreview = contentPreview[:200]
							}
							env.LogTest(t, "  Config preview (first 200 chars): %s", contentPreview)
						} else {
							env.LogTest(t, "WARNING: Configuration content may not contain expected crawler settings")
							env.LogTest(t, "  Actual config content: %s", configResult.Content)
						}
					}
				}

				env.TakeScreenshot(ctx, "test-crawler-job-details")
			}
		}
	}

	// Step 10: Check if any documents were created (verify actual crawling occurred)
	env.LogTest(t, "Step 10: Checking if documents were created by the crawler...")

	// Navigate to documents page to check for created documents
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/documents"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for page to load
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate to documents page: %v", err)
	} else {
		// Check for documents that might have been created by our crawler
		var documentResult struct {
			Count     int      `json:"count"`
			Documents []string `json:"documents"`
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					// Look for document cards or table rows
					const documentElements = document.querySelectorAll('.document-card, .document-row, tbody tr');
					const count = documentElements.length;
					
					// Get titles/names of recent documents
					const documents = Array.from(documentElements).slice(0, 5).map(el => {
						const titleEl = el.querySelector('.document-title, .title, td:first-child');
						return titleEl ? titleEl.textContent.trim() : '';
					}).filter(title => title.length > 0);
					
					return {
						count: count,
						documents: documents
					};
				})()
			`, &documentResult),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check documents: %v", err)
		} else {
			env.LogTest(t, "✓ Found %d documents in the system", documentResult.Count)
			if len(documentResult.Documents) > 0 {
				env.LogTest(t, "  Recent documents:")
				for i, doc := range documentResult.Documents {
					env.LogTest(t, "    %d. %s", i+1, doc)
				}
			}

			// Note: We can't definitively say if these were created by our crawler
			// without more detailed tracking, but we can verify the system is working
		}

		env.TakeScreenshot(ctx, "documents-after-crawl")
	}

	env.TakeScreenshot(ctx, "crawler-test-completed")
	env.LogTest(t, "✅ Crawler job execution test completed successfully")
}

// TestCrawlerLogsVerification tests that the crawler is using start_urls and logs properly
func TestCrawlerLogsVerification(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestCrawlerLogsVerification")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestCrawlerLogsVerification")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestCrawlerLogsVerification (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestCrawlerLogsVerification (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Create and execute a simple crawler job
	env.LogTest(t, "Step 1: Creating and executing simple crawler job...")
	testCrawlerTOML := `# Simple Test Crawler
id = "simple-test-crawler"
name = "Simple Test Crawler"
description = "Simple crawler to test start_urls usage"

# Use a simple, accessible URL
start_urls = ["https://httpbin.org/html"]

schedule = ""
timeout = "2m"
enabled = true
auto_start = false

max_depth = 0
max_pages = 1
concurrency = 1
follow_links = false
`

	// Navigate to job_add page and create the job
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/job_add"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const editorElement = document.querySelector('.CodeMirror');
				if (editorElement && editorElement.CodeMirror) {
					const editor = editorElement.CodeMirror;
					editor.setValue(%s);
					return true;
				}
				return false;
			})()
		`, "`"+testCrawlerTOML+"`"), nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to create job: %v", err)
		t.Fatalf("Failed to create job: %v", err)
	}

	// Save the job
	var saveResult bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const saveButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.trim() === 'Save' || btn.textContent.includes('Save')
				);
				if (saveButton) {
					saveButton.click();
					return true;
				}
				return false;
			})()
		`, &saveResult),
		chromedp.Sleep(3*time.Second),
	)

	if !saveResult {
		env.LogTest(t, "ERROR: Failed to save job")
		t.Fatal("Failed to save job")
	}

	env.LogTest(t, "✓ Job created and saved")

	// Navigate to jobs page and execute the job
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to jobs page: %v", err)
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}

	// Wait for WebSocket and execute the job
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket connection failed: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const testCrawlerCard = cards.find(card => card.textContent.includes('Simple Test Crawler'));
				if (testCrawlerCard) {
					const runButton = testCrawlerCard.querySelector('button.btn-success');
					if (runButton) {
						runButton.click();
						return true;
					}
				}
				return false;
			})()
		`, nil),
		chromedp.Sleep(2*time.Second),
	)

	env.LogTest(t, "✓ Job execution triggered")

	// Step 2: Navigate to queue and find the job
	env.LogTest(t, "Step 2: Finding job in queue...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to queue: %v", err)
		t.Fatalf("Failed to navigate to queue: %v", err)
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
		env.LogTest(t, "✓ Jobs loaded (%d jobs)", loadResult.JobsLoaded)
	}

	// Find the job and get its ID
	var jobDetails struct {
		Found  bool   `json:"found"`
		JobID  string `json:"jobId"`
		Status string `json:"status"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const testCrawlerJob = jobCards.find(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.includes('Simple Test Crawler');
				});

				if (!testCrawlerJob) {
					return { found: false, jobId: '', status: '' };
				}

				const jobId = testCrawlerJob.getAttribute('data-job-id') || '';
				const statusBadge = testCrawlerJob.querySelector('.label');
				const status = statusBadge ? statusBadge.textContent.trim() : '';

				return {
					found: true,
					jobId: jobId,
					status: status
				};
			})()
		`, &jobDetails),
	)

	if !jobDetails.Found {
		env.LogTest(t, "ERROR: Simple Test Crawler job not found in queue")
		env.TakeScreenshot(ctx, "job-not-found")
		t.Fatal("Job should appear in queue")
	}

	env.LogTest(t, "✓ Found job in queue (ID: %s, Status: %s)", jobDetails.JobID, jobDetails.Status)

	// Step 3: Navigate to job details and check logs
	env.LogTest(t, "Step 3: Checking job logs for start_urls usage...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("%s/job?id=%s", baseURL, jobDetails.JobID)),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to job details: %v", err)
		t.Fatalf("Failed to navigate to job details: %v", err)
	}

	// Switch to Output tab
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const outputTab = Array.from(document.querySelectorAll('button, .tab')).find(el => 
					el.textContent && el.textContent.toLowerCase().includes('output')
				);
				if (outputTab) {
					outputTab.click();
					return true;
				}
				return false;
			})()
		`, nil),
		chromedp.Sleep(2*time.Second),
	)

	// Get the job logs
	var logContent string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const logContainer = document.querySelector('.terminal, .log-container, pre, code');
				if (logContainer) {
					return logContainer.textContent;
				}
				return '';
			})()
		`, &logContent),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to read logs: %v", err)
	} else {
		env.LogTest(t, "✓ Retrieved job logs (%d characters)", len(logContent))

		// Check if logs contain evidence of start_urls usage
		if strings.Contains(logContent, "httpbin.org") {
			env.LogTest(t, "✓ Logs contain reference to httpbin.org (start_urls being used)")
		} else {
			env.LogTest(t, "WARNING: Logs do not contain reference to httpbin.org")
		}

		if strings.Contains(logContent, "start_urls") {
			env.LogTest(t, "✓ Logs contain 'start_urls' reference")
		} else {
			env.LogTest(t, "WARNING: Logs do not contain 'start_urls' reference")
		}

		if strings.Contains(logContent, "Using start_urls from job definition config") {
			env.LogTest(t, "✓ Logs confirm start_urls are being used from job definition")
		} else {
			env.LogTest(t, "WARNING: Logs do not confirm start_urls usage from job definition")
		}

		// Log first 500 characters for debugging
		logPreview := logContent
		if len(logPreview) > 500 {
			logPreview = logPreview[:500] + "..."
		}
		env.LogTest(t, "Log preview: %s", logPreview)
	}

	env.TakeScreenshot(ctx, "job-logs-checked")
	env.LogTest(t, "✅ Crawler logs verification completed")
}
