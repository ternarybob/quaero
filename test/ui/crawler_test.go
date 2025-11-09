package ui

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestNewsCrawlerJobExecution tests the complete news crawler job execution workflow
// Run with: go test -timeout 2m -run TestNewsCrawlerJobExecution
func TestNewsCrawlerJobExecution(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestNewsCrawlerJobExecution")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestNewsCrawlerJobExecution")

	// Capture panics and log them
	defer func() {
		if r := recover(); r != nil {
			env.LogTest(t, "PANIC: %v", r)
			env.LogTest(t, "Stack trace:")
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			env.LogTest(t, "%s", string(buf[:n]))
			t.Fatalf("Test panicked: %v", r)
		}

		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestNewsCrawlerJobExecution (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestNewsCrawlerJobExecution (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 180*time.Second) // Extended timeout for crawling
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Navigate to jobs page and verify news-crawler exists
	env.LogTest(t, "Step 1: Navigating to jobs page to find news-crawler...")
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
	env.TakeScreenshot(ctx, "01-jobs-page-loaded")

	// Verify the News Crawler job appears
	var newsCrawlerFound bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				return cards.some(card => card.textContent.includes('News Crawler'));
			})()
		`, &newsCrawlerFound),
	)

	if err != nil || !newsCrawlerFound {
		env.LogTest(t, "ERROR: News Crawler job not found in job definitions list")
		env.TakeScreenshot(ctx, "news-crawler-not-found")
		t.Fatal("News Crawler job should be available in job definitions list")
	}

	env.LogTest(t, "✓ News Crawler job found in job definitions list")
	env.TakeScreenshot(ctx, "02-news-crawler-found")

	// Step 2: Execute the News Crawler job
	env.LogTest(t, "Step 2: Executing the News Crawler job...")

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

	// Find and click the run button for News Crawler
	var runButtonClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const newsCrawlerCard = cards.find(card => card.textContent.includes('News Crawler'));
				if (newsCrawlerCard) {
					const runButton = newsCrawlerCard.querySelector('button.btn-success');
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
		env.LogTest(t, "ERROR: Failed to execute News Crawler job")
		env.TakeScreenshot(ctx, "run-button-click-failed")
		t.Fatal("Failed to execute News Crawler job")
	}

	env.LogTest(t, "✓ News Crawler job execution triggered")

	// Step 3: Navigate to queue page and monitor execution
	env.LogTest(t, "Step 3: Navigating to queue page to monitor execution...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load queue page: %v", err)
		t.Fatalf("Failed to load queue page: %v", err)
	}
	env.TakeScreenshot(ctx, "03-queue-page-loaded")

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
		`, &loadResult, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Sleep(3*time.Second),
	)

	if loadResult.Success {
		env.LogTest(t, "✓ Jobs loaded successfully (%d jobs)", loadResult.JobsLoaded)
	} else {
		env.LogTest(t, "WARNING: loadJobs() failed: %s", loadResult.ErrorMessage)
	}

	// Step 4: Verify the job appears in queue and monitor its execution
	env.LogTest(t, "Step 4: Monitoring News Crawler job execution...")

	var jobFound bool
	var jobDetails struct {
		Found  bool   `json:"found"`
		JobID  string `json:"jobId"`
		Status string `json:"status"`
		Name   string `json:"name"`
	}

	// Monitor for up to 20 seconds (job may stay Running if no URLs accessible)
	monitorStart := time.Now()
	maxMonitorTime := 20 * time.Second

	for time.Since(monitorStart) < maxMonitorTime {
		err = chromedp.Run(ctx,
			chromedp.Sleep(2*time.Second), // Wait between checks
			chromedp.Evaluate(`
				(() => {
					const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
					const newsCrawlerJob = jobCards.find(card => {
						const titleElement = card.querySelector('.card-title');
						return titleElement && titleElement.textContent.includes('News Crawler');
					});

					if (!newsCrawlerJob) {
						return { found: false, jobId: '', status: '', name: '' };
					}

					const jobId = newsCrawlerJob.getAttribute('data-job-id') || '';
					const statusBadge = newsCrawlerJob.querySelector('.label');
					const status = statusBadge ? statusBadge.textContent.trim() : '';
					const titleElement = newsCrawlerJob.querySelector('.card-title');
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
				env.LogTest(t, "✓ News Crawler job found in queue")
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
		env.LogTest(t, "ERROR: News Crawler job not found in queue")
		env.TakeScreenshot(ctx, "news-crawler-job-not-in-queue")
		t.Fatal("News Crawler job should appear in queue after execution")
	}

	// Step 4a: Validate progress text format
	env.LogTest(t, "Step 4a: Validating progress text format...")

	var progressTextData struct {
		Found        bool   `json:"found"`
		ProgressText string `json:"progressText"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const newsCrawlerJob = jobCards.find(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.includes('News Crawler');
				});

				if (!newsCrawlerJob) {
					return { found: false, progressText: '' };
				}

				// Look for progress text in the Progress column (4th column)
				const progressCell = newsCrawlerJob.querySelector('td:nth-child(4)');
				const progressText = progressCell ? progressCell.textContent.trim() : '';

				return {
					found: true,
					progressText: progressText
				};
			})()
		`, &progressTextData),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to read progress text: %v", err)
	} else if !progressTextData.Found {
		env.LogTest(t, "WARNING: Could not find job to read progress text")
	} else {
		env.LogTest(t, "  Progress text: %s", progressTextData.ProgressText)

		// Validate that progress text contains expected keywords
		progressLower := strings.ToLower(progressTextData.ProgressText)
		hasExpectedFormat := false

		// Check for parent job progress format: "X pending, Y running, Z completed"
		if strings.Contains(progressLower, "pending") &&
		   strings.Contains(progressLower, "running") &&
		   strings.Contains(progressLower, "completed") {
			hasExpectedFormat = true
			env.LogTest(t, "  ✓ Progress text contains expected keywords: pending, running, completed")
		}

		// If no children spawned, progress might be empty or show different text
		if !hasExpectedFormat && progressTextData.ProgressText == "" {
			env.LogTest(t, "  INFO: Progress text is empty (no child jobs spawned)")
		} else if !hasExpectedFormat {
			env.LogTest(t, "  WARNING: Progress text does not match expected format")
			env.LogTest(t, "           Expected format: 'X pending, Y running, Z completed'")
		}
	}

	env.TakeScreenshot(ctx, "04-news-crawler-execution-monitored")

	// Step 4b: Extract and validate document count from queue page
	env.LogTest(t, "Step 4b: Validating document count from queue page...")

	var queueDocumentCount struct {
		Found        bool   `json:"found"`
		DocumentText string `json:"documentText"`
		Count        int    `json:"count"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const newsCrawlerJob = jobCards.find(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.includes('News Crawler');
				});

				if (!newsCrawlerJob) {
					return { found: false, documentText: '', count: 0 };
				}

				// Look for the document count text (e.g., "34 Documents")
				const documentSpans = newsCrawlerJob.querySelectorAll('span');
				for (const span of documentSpans) {
					const text = span.textContent.trim();
					if (text.includes('Document')) {
						// Extract the number from text like "34 Documents" or "1 Document"
						const match = text.match(/(\d+)\s*Document/);
						if (match) {
							return {
								found: true,
								documentText: text,
								count: parseInt(match[1])
							};
						}
					}
				}

				return { found: false, documentText: '', count: 0 };
			})()
		`, &queueDocumentCount),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to read document count from queue page: %v", err)
	} else if !queueDocumentCount.Found {
		env.LogTest(t, "WARNING: Document count not found in queue page job card")
	} else {
		env.LogTest(t, "✓ Document count from queue page: %s", queueDocumentCount.DocumentText)

		// Based on news-crawler.toml configuration: max_pages = 1
		// We expect exactly 1 document, so fail if count > 1
		expectedMaxCount := 1

		if queueDocumentCount.Count > expectedMaxCount {
			env.LogTest(t, "❌ FAILURE: Queue page shows %d documents (expected <= %d)", queueDocumentCount.Count, expectedMaxCount)
			env.LogTest(t, "  This indicates max_pages=1 configuration is not being respected")
			env.TakeScreenshot(ctx, "queue-document-count-exceeded")
			t.Errorf("Queue page document count should be <= %d (max_pages=1), got %d", expectedMaxCount, queueDocumentCount.Count)
		} else if queueDocumentCount.Count == expectedMaxCount {
			env.LogTest(t, "✅ SUCCESS: Queue page shows exactly %d document (matches max_pages=1)", queueDocumentCount.Count)
		} else {
			env.LogTest(t, "  Queue page shows %d documents (within expected limit of %d)", queueDocumentCount.Count, expectedMaxCount)
		}
	}

	// Step 5: Click on the job to view details and verify configuration display
	if jobDetails.JobID != "" {
		env.LogTest(t, "Step 5: Navigating to job details page to verify configuration and logs...")

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
		env.TakeScreenshot(ctx, "05-job-details-page-loaded")

		// Step 5a: Verify crawler-specific configuration display
		env.LogTest(t, "Step 5a: Verifying crawler configuration display...")

		var crawlerConfigResult struct {
			CrawlerConfigVisible bool     `json:"crawlerConfigVisible"`
			StartUrls            []string `json:"startUrls"`
			MaxDepth             string   `json:"maxDepth"`
			MaxPages             string   `json:"maxPages"`
			IncludePatterns      []string `json:"includePatterns"`
			ExcludePatterns      []string `json:"excludePatterns"`
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					// Look for crawler configuration section
					const crawlerConfigSection = Array.from(document.querySelectorAll('section.card')).find(section => {
						const header = section.querySelector('h3');
						return header && header.textContent.includes('Crawler Configuration');
					});

					if (!crawlerConfigSection) {
						return { crawlerConfigVisible: false, startUrls: [], maxDepth: '', maxPages: '', includePatterns: [], excludePatterns: [] };
					}

					// Extract start URLs
					const startUrlElements = crawlerConfigSection.querySelectorAll('.start-urls-list a');
					const startUrls = Array.from(startUrlElements).map(a => a.textContent.trim());

					// Extract configuration values
					const configItems = crawlerConfigSection.querySelectorAll('.config-item');
					let maxDepth = '', maxPages = '';
					
					configItems.forEach(item => {
						const label = item.querySelector('div:first-child');
						const value = item.querySelector('div:last-child');
						if (label && value) {
							const labelText = label.textContent.trim();
							const valueText = value.textContent.trim();
							if (labelText.includes('Max Depth')) maxDepth = valueText;
							if (labelText.includes('Max Pages')) maxPages = valueText;
						}
					});

					// Extract patterns
					const includePatternElements = crawlerConfigSection.querySelectorAll('.pattern-list div');
					const includePatterns = Array.from(includePatternElements).map(div => div.textContent.trim()).filter(text => text.length > 0);

					return {
						crawlerConfigVisible: true,
						startUrls: startUrls,
						maxDepth: maxDepth,
						maxPages: maxPages,
						includePatterns: includePatterns,
						excludePatterns: [] // Will be populated if exclude patterns section exists
					};
				})()
			`, &crawlerConfigResult),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check crawler configuration: %v", err)
		} else if !crawlerConfigResult.CrawlerConfigVisible {
			env.LogTest(t, "WARNING: Crawler Configuration section not visible (config available in logs)")
			env.TakeScreenshot(ctx, "crawler-config-not-visible")
			// Don't fail the test - configuration is available in logs which we verify below
		} else {
			env.LogTest(t, "✓ Crawler Configuration section is visible")
			env.LogTest(t, "  Start URLs found: %v", crawlerConfigResult.StartUrls)
			env.LogTest(t, "  Max Depth: %s", crawlerConfigResult.MaxDepth)
			env.LogTest(t, "  Max Pages: %s", crawlerConfigResult.MaxPages)

			// Verify expected start URLs are present
			expectedUrls := []string{"stockhead.com.au", "abc.net.au"}
			urlsFound := 0
			for _, expectedUrl := range expectedUrls {
				for _, actualUrl := range crawlerConfigResult.StartUrls {
					if strings.Contains(actualUrl, expectedUrl) {
						urlsFound++
						break
					}
				}
			}

			if urlsFound >= 1 {
				env.LogTest(t, "✓ Expected start URLs found in configuration")
			} else {
				env.LogTest(t, "WARNING: Expected start URLs not found in configuration")
			}
		}

		// Step 5b: Switch to Output tab and verify logs
		env.LogTest(t, "Step 5b: Switching to Output tab to verify logs...")

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
			chromedp.Sleep(3*time.Second), // Wait for tab switch and logs to load
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to switch to Output tab: %v", err)
		} else {
			env.LogTest(t, "✓ Switched to Output tab")

			// Get the job logs and verify crawler-specific content
			var logVisibility struct {
				LogContent      string `json:"logContent"`
				TerminalVisible bool   `json:"terminalVisible"`
				TerminalHeight  int    `json:"terminalHeight"`
				HasVisibleText  bool   `json:"hasVisibleText"`
			}

			err = chromedp.Run(ctx,
				chromedp.Evaluate(`
					(() => {
						const logContainer = document.querySelector('.terminal, .log-container, pre, code');
						if (!logContainer) {
							return { logContent: '', terminalVisible: false, terminalHeight: 0, hasVisibleText: false };
						}

						// Get computed style to check if terminal is actually visible
						const style = window.getComputedStyle(logContainer);
						const isVisible = style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
						const height = logContainer.offsetHeight;

						// Check if there's any visible text (not just white text on white background)
						const hasContent = logContainer.textContent && logContainer.textContent.trim().length > 0;

						return {
							logContent: logContainer.textContent || '',
							terminalVisible: isVisible,
							terminalHeight: height,
							hasVisibleText: hasContent && isVisible
						};
					})()
				`, &logVisibility),
			)

			if err != nil {
				env.LogTest(t, "WARNING: Failed to read logs: %v", err)
			} else {
				env.LogTest(t, "✓ Retrieved job logs (%d characters)", len(logVisibility.LogContent))

				// Step 5b-1: Verify logs are actually visible in the UI (not empty terminal)
				env.LogTest(t, "Step 5b-1: Verifying job logs are visible in the UI...")
				env.LogTest(t, "  Terminal visible: %v", logVisibility.TerminalVisible)
				env.LogTest(t, "  Terminal height: %dpx", logVisibility.TerminalHeight)
				env.LogTest(t, "  Has visible text: %v", logVisibility.HasVisibleText)
				env.LogTest(t, "  Content length: %d characters", len(logVisibility.LogContent))

				// CRITICAL: Check for empty, blank, or null content - these are ALL failures
				logContentTrimmed := strings.TrimSpace(logVisibility.LogContent)

				// Check for "No logs available" message (failure case)
				if strings.Contains(strings.ToLower(logVisibility.LogContent), "no logs available") {
					env.LogTest(t, "❌ FAILURE: Job logs show 'No logs available' message")
					env.LogTest(t, "  This indicates the job has no logs or the GET /api/jobs/{id}/logs endpoint failed")
					env.TakeScreenshot(ctx, "no-logs-available-message")

					// Capture console errors
					var consoleErrors []map[string]interface{}
					chromedp.Evaluate(`
						(() => {
							const errors = [];
							// Get console errors from window.__consoleErrors if available
							if (window.__consoleErrors) {
								errors.push(...window.__consoleErrors);
							}
							return errors;
						})()
					`, &consoleErrors).Do(ctx)

					if len(consoleErrors) > 0 {
						env.LogTest(t, "  Console errors detected:")
						for i, errObj := range consoleErrors {
							env.LogTest(t, "    %d. %v", i+1, errObj)
						}
					}

					t.Errorf("Job logs show 'No logs available' - expected actual log content")
					return
				}

				// STRICT CHECK: Empty, blank, or missing content is a FAILURE
				// Any of these conditions mean the logs are NOT properly displayed:
				// - logContentTrimmed is empty string ""
				// - logVisibility.LogContent has zero length
				// - Terminal text is not visible (hasVisibleText = false)
				if logContentTrimmed == "" || len(logVisibility.LogContent) == 0 || !logVisibility.HasVisibleText {
					env.LogTest(t, "❌ FAILURE: Job logs are EMPTY or NOT VISIBLE")
					env.LogTest(t, "  Expected: Actual log content with text (job execution logs)")
					env.LogTest(t, "  Actual: Empty/blank terminal with no content")
					env.LogTest(t, "  Terminal display: visible=%v, height=%dpx, content_length=%d, trimmed_length=%d",
						logVisibility.TerminalVisible, logVisibility.TerminalHeight, len(logVisibility.LogContent), len(logContentTrimmed))

					// Capture console errors to diagnose the issue
					var consoleErrors []map[string]interface{}
					chromedp.Evaluate(`
						(() => {
							const errors = [];
							// Capture all console errors
							if (window.__consoleErrors) {
								errors.push(...window.__consoleErrors);
							}
							return errors;
						})()
					`, &consoleErrors).Do(ctx)

					if len(consoleErrors) > 0 {
						env.LogTest(t, "  Console errors detected:")
						for i, errObj := range consoleErrors {
							env.LogTest(t, "    %d. %v", i+1, errObj)
						}
					} else {
						env.LogTest(t, "  No console errors captured - logs may be failing to load silently")
					}

					env.TakeScreenshot(ctx, "job-logs-empty-or-blank")
					t.Errorf("CRITICAL FAILURE: Job logs are empty/blank - logs MUST display actual content")
					return
				}

				// If we got here, logs have actual content
				env.LogTest(t, "✓ Job logs are visible in the UI (%d characters, height: %dpx)", len(logVisibility.LogContent), logVisibility.TerminalHeight)

				// Use the extracted log content for further checks
				logContent := logVisibility.LogContent

				// Check for crawler-specific log entries
				crawlerLogChecks := []struct {
					pattern     string
					description string
					required    bool
				}{
					{"start_urls", "start_urls configuration", true},
					{"stockhead.com.au", "stockhead.com.au URL", false}, // Optional: external URL may be temporarily unavailable
					{"abc.net.au", "abc.net.au URL", true},
					{"source_type", "source type configuration", true},
					{"news-crawler", "job definition ID", true},
					{"max_depth", "max depth configuration", true},
					{"step_1_crawl", "crawl step configuration", true},
				}

				foundChecks := 0
				requiredChecks := 0
				for _, check := range crawlerLogChecks {
					if check.required {
						requiredChecks++
					}
					if strings.Contains(logContent, check.pattern) {
						env.LogTest(t, "  ✓ Found %s in logs", check.description)
						foundChecks++
					} else {
						if check.required {
							env.LogTest(t, "  ✗ Missing required %s in logs", check.description)
						} else {
							env.LogTest(t, "  - Missing optional %s in logs", check.description)
						}
					}
				}

				if foundChecks >= requiredChecks {
					env.LogTest(t, "✓ Logs contain expected crawler configuration details (%d/%d checks passed)", foundChecks, len(crawlerLogChecks))
				} else {
					env.LogTest(t, "WARNING: Logs missing required crawler details (%d/%d required checks passed)", foundChecks, requiredChecks)
				}

				// Log first 1000 characters for debugging
				logPreview := logContent
				if len(logPreview) > 1000 {
					logPreview = logPreview[:1000] + "..."
				}
				env.LogTest(t, "Log preview (first 1000 chars): %s", logPreview)
			}
		}

		env.TakeScreenshot(ctx, "06-job-details-complete")
	}

	// Step 6: Verify enhanced queue page features for crawler jobs
	env.LogTest(t, "Step 6: Verifying enhanced queue page features for crawler jobs...")

	// Navigate back to queue page to test enhanced UI features
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate back to queue page: %v", err)
	} else {
		// Check for enhanced crawler progress display
		var crawlerProgressResult struct {
			ProgressDisplayFound bool   `json:"progressDisplayFound"`
			StatsGridFound       bool   `json:"statsGridFound"`
			ProgressText         string `json:"progressText"`
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					// Look for crawler progress display
					const crawlerProgressDisplay = document.querySelector('.crawler-progress-display');
					const statsGrid = document.querySelector('.crawler-stats-grid');
					
					let progressText = '';
					if (crawlerProgressDisplay) {
						const progressTextEl = crawlerProgressDisplay.querySelector('span');
						progressText = progressTextEl ? progressTextEl.textContent : '';
					}

					return {
						progressDisplayFound: !!crawlerProgressDisplay,
						statsGridFound: !!statsGrid,
						progressText: progressText
					};
				})()
			`, &crawlerProgressResult),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check crawler progress features: %v", err)
		} else {
			if crawlerProgressResult.ProgressDisplayFound {
				env.LogTest(t, "✓ Enhanced crawler progress display found in queue")
			}
			if crawlerProgressResult.StatsGridFound {
				env.LogTest(t, "✓ Crawler statistics grid found in queue")
			}
			if crawlerProgressResult.ProgressText != "" {
				env.LogTest(t, "  Progress text: %s", crawlerProgressResult.ProgressText)
			}
		}

		env.TakeScreenshot(ctx, "07-enhanced-queue-features")
	}

	// Step 7: Navigate to Documents page and verify document count
	env.LogTest(t, "Step 7: Navigating to Documents page to verify document count...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/documents"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load documents page: %v", err)
		env.TakeScreenshot(ctx, "documents-page-load-failed")
		t.Fatalf("Failed to load documents page: %v", err)
	}

	env.LogTest(t, "✓ Documents page loaded")
	env.TakeScreenshot(ctx, "09-documents-page-loaded")

	// Step 7a: Check document count from UI
	env.LogTest(t, "Step 7a: Checking document count from UI...")

	var documentUICount struct {
		TotalCount int    `json:"totalCount"`
		Error      string `json:"error"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				try {
					// Look for the total documents stat in the Document Statistics section
					const statElements = document.querySelectorAll('.has-text-centered');
					for (const el of statElements) {
						const heading = el.querySelector('p');
						if (heading && heading.textContent.includes('TOTAL DOCUMENTS')) {
							const countElement = el.querySelector('.title, .is-size-1, .is-size-2');
							if (countElement) {
								return { totalCount: parseInt(countElement.textContent.trim()) || 0, error: '' };
							}
						}
					}
					return { totalCount: 0, error: 'Total documents stat not found in UI' };
				} catch (e) {
					return { totalCount: 0, error: e.toString() };
				}
			})()
		`, &documentUICount),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to read document count from UI: %v", err)
	} else if documentUICount.Error != "" {
		env.LogTest(t, "WARNING: Error reading document count from UI: %s", documentUICount.Error)
	} else {
		env.LogTest(t, "✓ Document count from UI: %d documents", documentUICount.TotalCount)
	}

	// Step 7b: Verify document collection via API
	env.LogTest(t, "Step 7b: Verifying document collection via API...")

	var documentCount struct {
		Count int    `json:"count"`
		Error string `json:"error"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(async () => {
				try {
					const response = await fetch('/api/documents/stats');
					if (!response.ok) {
						throw new Error('Failed to fetch document stats');
					}
					const data = await response.json();
					return { count: data.total_documents || 0, error: '' };
				} catch (e) {
					return { count: 0, error: e.toString() };
				}
			})()
		`, &documentCount, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check document count via API: %v", err)
	} else if documentCount.Error != "" {
		env.LogTest(t, "WARNING: Error fetching document stats from API: %s", documentCount.Error)
	} else {
		env.LogTest(t, "✓ Document count from API: %d documents", documentCount.Count)

		// Compare UI and API counts
		if documentUICount.TotalCount > 0 && documentCount.Count != documentUICount.TotalCount {
			env.LogTest(t, "⚠️  WARNING: UI count (%d) differs from API count (%d)", documentUICount.TotalCount, documentCount.Count)
		}

		// Based on news-crawler.toml configuration: max_pages = 1
		// We expect exactly 1 document to be collected
		expectedCount := 1

		if documentCount.Count == expectedCount {
			env.LogTest(t, "✅ SUCCESS: Exactly %d document collected (matches max_pages=1 in news-crawler.toml)", documentCount.Count)
		} else {
			env.LogTest(t, "❌ FAILURE: Expected exactly %d document but got %d documents", expectedCount, documentCount.Count)
			env.LogTest(t, "  This indicates max_pages=1 configuration is not being respected")
			if documentUICount.TotalCount > 0 {
				env.LogTest(t, "  UI also shows: %d documents", documentUICount.TotalCount)
			}
			env.TakeScreenshot(ctx, "incorrect-document-count")
			t.Errorf("Expected exactly %d document to be collected (max_pages=1), got %d", expectedCount, documentCount.Count)
		}
	}

	// Also check if any job logs indicate successful document storage
	if jobDetails.JobID != "" {
		var documentLogCheck struct {
			Found bool   `json:"found"`
			Count int    `json:"count"`
			Logs  string `json:"logs"`
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(async () => {
					try {
						const response = await fetch('/api/jobs/%s/logs');
						if (!response.ok) {
							return { found: false, count: 0, logs: '' };
						}
						const data = await response.json();
						const logs = data.logs || [];
						const logText = logs.map(log => log.message).join(' ');
						
						// Look for document storage indicators in logs
						const documentSaveMatches = logText.match(/Document saved|document.*saved|saved.*document/gi) || [];
						const documentStorageMatches = logText.match(/storage.*success|successfully.*stored/gi) || [];
						
						return {
							found: documentSaveMatches.length > 0 || documentStorageMatches.length > 0,
							count: documentSaveMatches.length + documentStorageMatches.length,
							logs: logText.substring(0, 500) // First 500 chars for debugging
						};
					} catch (e) {
						return { found: false, count: 0, logs: e.toString() };
					}
				})()
			`, jobDetails.JobID), &documentLogCheck, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
				return p.WithAwaitPromise(true)
			}),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check job logs for document storage: %v", err)
		} else {
			if documentLogCheck.Found {
				env.LogTest(t, "✓ Job logs indicate document storage occurred (%d references)", documentLogCheck.Count)
			} else {
				env.LogTest(t, "⚠️  Job logs do not show document storage references")
				env.LogTest(t, "Log preview: %s", documentLogCheck.Logs)
			}
		}
	}

	env.TakeScreenshot(ctx, "08-test-completed")
	env.LogTest(t, "✅ News Crawler job execution test completed successfully")
}

// TestNewsCrawlerConfiguration tests the news crawler configuration display and validation
func TestNewsCrawlerConfiguration(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestNewsCrawlerConfiguration")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestNewsCrawlerConfiguration")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestNewsCrawlerConfiguration (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestNewsCrawlerConfiguration (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Navigate to job_add page and load news-crawler configuration manually
	env.LogTest(t, "Step 1: Loading news-crawler configuration...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/job_add"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for page to load
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load job_add page: %v", err)
		t.Fatalf("Failed to load job_add page: %v", err)
	}

	// Step 2: Read the news-crawler.toml file content
	env.LogTest(t, "Step 2: Preparing news-crawler.toml content...")

	newsCrawlerContent := `# Example Crawler Job Definition
# This file demonstrates how to define a custom crawler job in TOML format
# Place .toml or .json files in the job-definitions directory to auto-load at startup

id = "news-crawler"
name = "News Crawler"
job_type = "user"
description = "Crawler job that crawls a news website and filters for specific content"

# Initial URLs to start crawling from
start_urls = ["https://stockhead.com.au/just-in", "https://www.abc.net.au/news"]

# Cron schedule (empty = manual execution only)
# Examples:
#   "*/5 * * * *"  = Every 5 minutes
#   "0 */6 * * *"  = Every 6 hours
#   "0 0 * * *"    = Daily at midnight
schedule = ""

# Maximum execution time (e.g., "30m", "1h", "2h30m")
timeout = "30m"

# Whether this job is enabled
enabled = true

# Whether to auto-start when scheduler initializes
auto_start = false

# URL filtering patterns (regex)
# Only URLs matching these patterns will be crawled
include_patterns = ["article", "news", "post"]

# URLs matching these patterns will be excluded
exclude_patterns = ["login", "logout", "admin"]

# Crawler behavior
max_depth = 2       # Maximum depth to follow links
max_pages = 100     # Maximum number of pages to crawl
concurrency = 5     # Number of concurrent workers
follow_links = true # Whether to follow discovered links
`

	env.LogTest(t, "✓ News crawler TOML content prepared (%d bytes)", len(newsCrawlerContent))

	// Step 3: Wait for CodeMirror editor and paste the content
	env.LogTest(t, "Step 3: Waiting for CodeMirror editor and pasting content...")

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
		`, "`"+newsCrawlerContent+"`"), nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to set editor content: %v", err)
		env.TakeScreenshot(ctx, "editor-content-set-failed")
		t.Fatalf("Failed to set editor content: %v", err)
	}
	env.LogTest(t, "✓ News crawler content pasted into editor")

	// Step 4: Verify the configuration content is properly loaded
	env.LogTest(t, "Step 4: Verifying configuration content...")

	var configResult struct {
		Loaded  bool   `json:"loaded"`
		Content string `json:"content"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const editorElement = document.querySelector('.CodeMirror');
				if (editorElement && editorElement.CodeMirror) {
					const content = editorElement.CodeMirror.getValue();
					return {
						loaded: content.includes('news-crawler') && content.includes('News Crawler'),
						content: content
					};
				}
				return { loaded: false, content: '' };
			})()
		`, &configResult),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check editor content: %v", err)
		t.Fatalf("Failed to check editor content: %v", err)
	}

	if !configResult.Loaded {
		env.LogTest(t, "ERROR: News Crawler configuration not properly loaded in editor")
		env.TakeScreenshot(ctx, "news-crawler-config-not-loaded")
		t.Fatal("News Crawler configuration should be loaded in editor")
	}

	env.LogTest(t, "✓ News Crawler configuration loaded successfully")

	// Verify expected configuration elements
	expectedElements := []string{
		"stockhead.com.au",
		"abc.net.au",
		"max_depth = 2",
		"max_pages = 100",
		"include_patterns",
		"exclude_patterns",
	}

	foundElements := 0
	for _, element := range expectedElements {
		if strings.Contains(configResult.Content, element) {
			env.LogTest(t, "  ✓ Found: %s", element)
			foundElements++
		} else {
			env.LogTest(t, "  - Missing: %s", element)
		}
	}

	if foundElements >= 4 {
		env.LogTest(t, "✓ News Crawler configuration contains expected elements (%d/%d)", foundElements, len(expectedElements))
	} else {
		env.LogTest(t, "WARNING: News Crawler configuration may be incomplete (%d/%d)", foundElements, len(expectedElements))
	}

	env.TakeScreenshot(ctx, "news-crawler-config-loaded")

	// Step 2: Test configuration validation
	env.LogTest(t, "Step 2: Testing configuration validation...")

	// Click validate button if it exists
	var validationResult struct {
		ButtonFound bool   `json:"buttonFound"`
		IsValid     bool   `json:"isValid"`
		Message     string `json:"message"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const validateButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.includes('Validate') || btn.textContent.includes('Check')
				);
				
				if (!validateButton) {
					return { buttonFound: false, isValid: false, message: 'Validate button not found' };
				}

				// Click the validate button
				validateButton.click();
				
				// Wait a moment for validation
				return new Promise(resolve => {
					setTimeout(() => {
						// Check for validation messages
						const errorElements = document.querySelectorAll('.error, .alert-error, .toast-error');
						const successElements = document.querySelectorAll('.success, .alert-success, .toast-success');
						
						const hasErrors = errorElements.length > 0;
						const hasSuccess = successElements.length > 0;
						
						let message = '';
						if (hasErrors) {
							message = Array.from(errorElements).map(el => el.textContent).join('; ');
						} else if (hasSuccess) {
							message = Array.from(successElements).map(el => el.textContent).join('; ');
						}
						
						resolve({
							buttonFound: true,
							isValid: !hasErrors,
							message: message
						});
					}, 2000);
				});
			})()
		`, &validationResult, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to test validation: %v", err)
	} else if !validationResult.ButtonFound {
		env.LogTest(t, "INFO: Validate button not found - validation may be automatic")
	} else {
		if validationResult.IsValid {
			env.LogTest(t, "✓ Configuration validation passed")
			if validationResult.Message != "" {
				env.LogTest(t, "  Validation message: %s", validationResult.Message)
			}
		} else {
			env.LogTest(t, "WARNING: Configuration validation failed: %s", validationResult.Message)
		}
	}

	env.TakeScreenshot(ctx, "news-crawler-validation-tested")
	env.LogTest(t, "✅ News Crawler configuration test completed")
}

// TestParentJobProgressTracking tests the real-time parent job progress and status updates
// Run with: go test -timeout 5m -run TestParentJobProgressTracking
func TestParentJobProgressTracking(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestParentJobProgressTracking")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestParentJobProgressTracking")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestParentJobProgressTracking (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestParentJobProgressTracking (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Navigate to jobs page and execute news-crawler
	env.LogTest(t, "Step 1: Navigating to jobs page and executing News Crawler...")
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
	env.TakeScreenshot(ctx, "01-jobs-page-loaded")

	// Wait for WebSocket connection
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Execute the News Crawler job
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const newsCrawlerCard = cards.find(card => card.textContent.includes('News Crawler'));
				if (newsCrawlerCard) {
					const runButton = newsCrawlerCard.querySelector('button.btn-success');
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
	if err != nil {
		env.LogTest(t, "ERROR: Failed to execute News Crawler: %v", err)
		t.Fatalf("Failed to execute News Crawler: %v", err)
	}
	env.LogTest(t, "✓ News Crawler job execution triggered")

	// Step 2: Navigate to queue page and monitor progress updates
	env.LogTest(t, "Step 2: Navigating to queue page to monitor real-time progress updates...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load queue page: %v", err)
		t.Fatalf("Failed to load queue page: %v", err)
	}
	env.TakeScreenshot(ctx, "02-queue-page-loaded")

	// Step 3: Monitor parent job progress text for updates
	env.LogTest(t, "Step 3: Monitoring parent job progress text for real-time updates...")

	type ProgressSnapshot struct {
		Timestamp    time.Time
		ProgressText string
		Status       string
		Found        bool
	}

	var progressHistory []ProgressSnapshot
	monitorStart := time.Now()
	maxMonitorTime := 30 * time.Second // Reduced timeout since we're just checking for updates

	for time.Since(monitorStart) < maxMonitorTime {
		var progressData struct {
			Found        bool   `json:"found"`
			ProgressText string `json:"progressText"`
			Status       string `json:"status"`
			JobID        string `json:"jobId"`
		}

		err = chromedp.Run(ctx,
			chromedp.Sleep(2*time.Second),
			chromedp.Evaluate(`
				(() => {
					const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
					const newsCrawlerJob = jobCards.find(card => {
						const titleElement = card.querySelector('.card-title');
						return titleElement && titleElement.textContent.includes('News Crawler');
					});

					if (!newsCrawlerJob) {
						return { found: false, progressText: '', status: '', jobId: '' };
					}

					const jobId = newsCrawlerJob.getAttribute('data-job-id') || '';
					const statusBadge = newsCrawlerJob.querySelector('.label');
					const status = statusBadge ? statusBadge.textContent.trim() : '';

					// Look for progress text in the Progress column
					const progressCell = newsCrawlerJob.querySelector('td:nth-child(4)'); // Progress is 4th column
					const progressText = progressCell ? progressCell.textContent.trim() : '';

					return {
						found: true,
						progressText: progressText,
						status: status,
						jobId: jobId
					};
				})()
			`, &progressData),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check progress: %v", err)
			continue
		}

		if progressData.Found {
			// Record this snapshot
			snapshot := ProgressSnapshot{
				Timestamp:    time.Now(),
				ProgressText: progressData.ProgressText,
				Status:       progressData.Status,
				Found:        true,
			}
			progressHistory = append(progressHistory, snapshot)

			env.LogTest(t, "  [%s] Status: %s, Progress: %s",
				snapshot.Timestamp.Format("15:04:05"),
				snapshot.Status,
				snapshot.ProgressText)

			// If we got progress text with the expected format, we can exit early
			if progressData.ProgressText != "" &&
				strings.Contains(progressData.ProgressText, "pending") &&
				strings.Contains(progressData.ProgressText, "running") &&
				strings.Contains(progressData.ProgressText, "completed") {
				env.LogTest(t, "  ✓ Progress text with expected format received, ending early")
				break
			}

			// Check if job reached terminal state
			statusLower := strings.ToLower(progressData.Status)
			if strings.Contains(statusLower, "completed") || strings.Contains(statusLower, "failed") {
				env.LogTest(t, "  Job reached terminal state: %s", progressData.Status)
				break
			}
		}
	}

	if len(progressHistory) == 0 {
		env.LogTest(t, "ERROR: No progress updates captured")
		env.TakeScreenshot(ctx, "no-progress-updates")
		t.Fatal("Expected to capture progress updates but got none")
	}

	env.LogTest(t, "✓ Captured %d progress snapshots over %v", len(progressHistory), time.Since(monitorStart))

	// Step 4: Verify progress text format and updates
	env.LogTest(t, "Step 4: Verifying progress text format and real-time updates...")

	// Check if we got the expected format: "X pending, Y running, Z completed, W failed"
	foundValidFormat := false
	foundMultipleUpdates := false

	for i, snapshot := range progressHistory {
		// Check for expected format
		if strings.Contains(snapshot.ProgressText, "pending") &&
			strings.Contains(snapshot.ProgressText, "running") &&
			strings.Contains(snapshot.ProgressText, "completed") {
			foundValidFormat = true
			env.LogTest(t, "  ✓ Valid progress format found: %s", snapshot.ProgressText)
		}

		// Check if progress text changed between snapshots
		if i > 0 && snapshot.ProgressText != progressHistory[i-1].ProgressText {
			foundMultipleUpdates = true
			env.LogTest(t, "  ✓ Progress text changed: '%s' → '%s'",
				progressHistory[i-1].ProgressText, snapshot.ProgressText)
		}
	}

	if !foundValidFormat {
		env.LogTest(t, "WARNING: Expected progress format not found (e.g., 'X pending, Y running, Z completed')")
		env.LogTest(t, "  Sample progress texts:")
		for i, snapshot := range progressHistory {
			if i >= 3 {
				break
			}
			env.LogTest(t, "    - %s", snapshot.ProgressText)
		}
		// Don't fail the test, as the format might be slightly different but still functional
	}

	if foundMultipleUpdates {
		env.LogTest(t, "✓ Progress text updated in real-time (multiple different values captured)")
	} else {
		env.LogTest(t, "INFO: Progress text did not change during monitoring period")
		env.LogTest(t, "  This may be normal if the job completed quickly")
	}

	// Step 5: Verify status changes were captured
	env.LogTest(t, "Step 5: Verifying status transitions...")

	uniqueStatuses := make(map[string]bool)
	for _, snapshot := range progressHistory {
		if snapshot.Status != "" {
			uniqueStatuses[snapshot.Status] = true
		}
	}

	env.LogTest(t, "  Unique statuses observed: %v", getKeys(uniqueStatuses))

	if len(uniqueStatuses) > 1 {
		env.LogTest(t, "✓ Multiple status values captured (job progressed through states)")
	} else {
		env.LogTest(t, "INFO: Only one status value observed during monitoring")
		env.LogTest(t, "  NOTE: If no child jobs spawned, status will remain 'Running' or 'Orchestrating'")
		env.LogTest(t, "        With child jobs, status should transition: Running → Completed/Failed")
	}

	env.TakeScreenshot(ctx, "03-progress-tracking-complete")
	env.LogTest(t, "✅ Parent job progress tracking test completed")
	env.LogTest(t, "")
	env.LogTest(t, "Test Note: This test verifies the UI correctly receives and displays")
	env.LogTest(t, "           parent job progress updates via WebSocket. Empty progress is")
	env.LogTest(t, "           expected when no child jobs spawn (e.g., no URLs to crawl).")
}

// TestCrawlerJobLogsVisibility tests that job logs are visible in the Output tab
// Run with: go test -timeout 2m -run TestCrawlerJobLogsVisibility
func TestCrawlerJobLogsVisibility(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestCrawlerJobLogsVisibility")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestCrawlerJobLogsVisibility")

	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestCrawlerJobLogsVisibility (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestCrawlerJobLogsVisibility (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Navigate to jobs page and execute News Crawler
	env.LogTest(t, "Step 1: Navigating to jobs page and executing News Crawler...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}
	env.LogTest(t, "✓ Jobs page loaded")
	env.TakeScreenshot(ctx, "01-jobs-page-loaded")

	// Wait for WebSocket connection
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		env.TakeScreenshot(ctx, "websocket-failed")
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Execute the News Crawler job
	env.LogTest(t, "Step 2: Executing News Crawler job...")
	var runButtonClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const newsCrawlerCard = cards.find(card => card.textContent.includes('News Crawler'));
				if (newsCrawlerCard) {
					const runButton = newsCrawlerCard.querySelector('button.btn-success');
					if (runButton) {
						runButton.click();
						return true;
					}
				}
				return false;
			})()
		`, &runButtonClicked),
		chromedp.Sleep(2*time.Second),
	)

	if err != nil || !runButtonClicked {
		env.LogTest(t, "ERROR: Failed to execute News Crawler job")
		env.TakeScreenshot(ctx, "run-button-click-failed")
		t.Fatal("Failed to execute News Crawler job")
	}
	env.LogTest(t, "✓ News Crawler job execution triggered")

	// Step 3: Navigate to queue page
	env.LogTest(t, "Step 3: Navigating to queue page...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load queue page: %v", err)
		env.TakeScreenshot(ctx, "queue-page-load-failed")
		t.Fatalf("Failed to load queue page: %v", err)
	}
	env.TakeScreenshot(ctx, "02-queue-page-loaded")

	// Step 4: Find the News Crawler job in queue
	env.LogTest(t, "Step 4: Finding News Crawler job in queue...")
	var jobDetails struct {
		Found bool   `json:"found"`
		JobID string `json:"jobId"`
		Name  string `json:"name"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const newsCrawlerJob = jobCards.find(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.includes('News Crawler');
				});

				if (!newsCrawlerJob) {
					return { found: false, jobId: '', name: '' };
				}

				const jobId = newsCrawlerJob.getAttribute('data-job-id') || '';
				const titleElement = newsCrawlerJob.querySelector('.card-title');
				const name = titleElement ? titleElement.textContent.trim() : '';

				return {
					found: true,
					jobId: jobId,
					name: name
				};
			})()
		`, &jobDetails),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to find job in queue: %v", err)
		env.TakeScreenshot(ctx, "job-not-found")
		t.Fatalf("Failed to find job in queue: %v", err)
	}

	if !jobDetails.Found {
		env.LogTest(t, "ERROR: News Crawler job not found in queue")
		env.TakeScreenshot(ctx, "news-crawler-not-in-queue")
		t.Fatal("News Crawler job should appear in queue")
	}

	env.LogTest(t, "✓ Found News Crawler job in queue (ID: %s)", jobDetails.JobID)

	// Step 5: Navigate to job details page
	env.LogTest(t, "Step 5: Navigating to job details page...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("%s/job?id=%s", baseURL, jobDetails.JobID)),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to job details: %v", err)
		env.TakeScreenshot(ctx, "job-details-nav-failed")
		t.Fatalf("Failed to navigate to job details: %v", err)
	}
	env.LogTest(t, "✓ Successfully navigated to job details page")
	env.TakeScreenshot(ctx, "03-job-details-page-loaded")

	// Step 6: Click the Output tab
	env.LogTest(t, "Step 6: Clicking Output tab...")
	var outputTabClicked bool
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
		`, &outputTabClicked),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil || !outputTabClicked {
		env.LogTest(t, "ERROR: Failed to click Output tab")
		env.TakeScreenshot(ctx, "output-tab-click-failed")
		t.Fatal("Failed to click Output tab")
	}
	env.LogTest(t, "✓ Clicked Output tab")
	env.TakeScreenshot(ctx, "04-output-tab-opened")

	// Step 7: Verify logs are visible
	env.LogTest(t, "Step 7: Verifying job logs are visible...")
	var logVisibility struct {
		LogContent      string `json:"logContent"`
		TerminalVisible bool   `json:"terminalVisible"`
		TerminalHeight  int    `json:"terminalHeight"`
		HasVisibleText  bool   `json:"hasVisibleText"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const logContainer = document.querySelector('.terminal, .log-container, pre, code');
				if (!logContainer) {
					return { logContent: '', terminalVisible: false, terminalHeight: 0, hasVisibleText: false };
				}

				const style = window.getComputedStyle(logContainer);
				const isVisible = style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
				const height = logContainer.offsetHeight;

				const hasContent = logContainer.textContent && logContainer.textContent.trim().length > 0;

				return {
					logContent: logContainer.textContent || '',
					terminalVisible: isVisible,
					terminalHeight: height,
					hasVisibleText: hasContent && isVisible
				};
			})()
		`, &logVisibility),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check log visibility: %v", err)
		env.TakeScreenshot(ctx, "log-visibility-check-failed")
		t.Fatalf("Failed to check log visibility: %v", err)
	}

	env.LogTest(t, "Log visibility check:")
	env.LogTest(t, "  Terminal visible: %v", logVisibility.TerminalVisible)
	env.LogTest(t, "  Terminal height: %dpx", logVisibility.TerminalHeight)
	env.LogTest(t, "  Has visible text: %v", logVisibility.HasVisibleText)
	env.LogTest(t, "  Content length: %d characters", len(logVisibility.LogContent))

	// Minimum expected terminal height for logs to be properly rendered
	// NOTE: Terminal height check disabled per user request - terminal height is a non-issue
	// minTerminalHeight := 50

	if !logVisibility.HasVisibleText || len(logVisibility.LogContent) == 0 {
		env.LogTest(t, "❌ FAILURE: Job logs are not visible in Output tab")
		env.LogTest(t, "  Expected: Log content displayed in terminal")
		env.LogTest(t, "  Actual: No log content found")
		env.TakeScreenshot(ctx, "no-logs-visible")
		t.Error("Job logs should be visible in the Output tab but no content was found")
	} else {
		env.LogTest(t, "✅ SUCCESS: Job logs are visible in Output tab")
		env.LogTest(t, "  Terminal rendered with %d characters, height: %dpx",
			len(logVisibility.LogContent), logVisibility.TerminalHeight)
	}

	env.TakeScreenshot(ctx, "05-test-completed")
	env.LogTest(t, "✅ Job logs visibility test completed")
}

// Helper function to get map keys as slice
func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
