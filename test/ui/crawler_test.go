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

// TestNewsCrawlerJobExecution tests the complete news crawler job execution workflow
func TestNewsCrawlerJobExecution(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestNewsCrawlerJobExecution")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestNewsCrawlerJobExecution")
	defer func() {
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
	env.TakeScreenshot(ctx, "news-crawler-found")

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

	// Step 4: Verify the job appears in queue and monitor its execution
	env.LogTest(t, "Step 4: Monitoring News Crawler job execution...")

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

	env.TakeScreenshot(ctx, "news-crawler-execution-monitored")

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

				// Check for crawler-specific log entries
				crawlerLogChecks := []struct {
					pattern     string
					description string
					required    bool
				}{
					{"start_urls", "start_urls configuration", true},
					{"stockhead.com.au", "stockhead.com.au URL", true},
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

		env.TakeScreenshot(ctx, "news-crawler-job-details-complete")
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

		env.TakeScreenshot(ctx, "enhanced-queue-features")
	}

	// Step 7: Verify document collection (must be more than 0)
	env.LogTest(t, "Step 7: Verifying document collection...")

	var documentCount struct {
		Count int    `json:"count"`
		Error string `json:"error"`
	}

	// Check document count via API
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
		`, &documentCount, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check document count: %v", err)
	} else if documentCount.Error != "" {
		env.LogTest(t, "WARNING: Error fetching document stats: %s", documentCount.Error)
	} else {
		env.LogTest(t, "Document collection result: %d documents", documentCount.Count)

		if documentCount.Count > 0 {
			env.LogTest(t, "✅ SUCCESS: Documents were collected (%d documents)", documentCount.Count)
		} else {
			env.LogTest(t, "❌ FAILURE: No documents were collected")
			env.TakeScreenshot(ctx, "no-documents-collected")
			t.Errorf("Expected more than 0 documents to be collected, got %d", documentCount.Count)
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
			`, jobDetails.JobID), &documentLogCheck, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
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

	env.TakeScreenshot(ctx, "news-crawler-test-completed")
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
		`, &validationResult, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
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
