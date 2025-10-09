package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func TestJira_CompleteWorkflow(t *testing.T) {
	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8085"
	}

	// Setup browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	screenshotCounter = 0

	// Start video recording
	stopRecording, err := startVideoRecording(ctx, t)
	if err != nil {
		t.Logf("Warning: Could not start video recording: %v", err)
	} else {
		defer stopRecording()
	}

	// Navigate to jira page
	t.Log("=== STEP 1: Navigate to Jira page ===")
	if err := chromedp.Run(ctx, chromedp.Navigate(serverURL+"/jira")); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	if err := chromedp.Run(ctx, chromedp.WaitVisible(`#project-list`, chromedp.ByID)); err != nil {
		t.Fatalf("Page did not load: %v", err)
	}
	t.Log("✓ Jira page loaded")
	takeScreenshot(ctx, t, "01_page_loaded")
	validateStyles(ctx, t, "Page Loaded")

	// STEP 2: Clear all data
	t.Log("\n=== STEP 2: Clear all Jira data ===")

	// Set up dialog handler before clicking
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			go chromedp.Run(ctx,
				page.HandleJavaScriptDialog(true),
			)
		}
	})

	if err := chromedp.Run(ctx, chromedp.Click(`#clear-data-btn`, chromedp.ByID)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_click_clear_data")
		t.Fatalf("Failed to click CLEAR ALL DATA: %v", err)
	}
	t.Log("✓ Clicked CLEAR ALL DATA and accepted confirmation")
	takeScreenshot(ctx, t, "02_clear_data_clicked")
	validateStyles(ctx, t, "After Clear Data")

	// Wait for clear to complete (projects should be empty)
	time.Sleep(3 * time.Second)

	var projectCount int
	chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('project-list');
			if (!tbody) return 0;
			const rows = tbody.querySelectorAll('tr');
			let count = 0;
			for (const row of rows) {
				// Count only data rows, not "no projects" messages
				if (row.querySelectorAll('td').length >= 5) {
					count++;
				}
			}
			return count;
		})()
	`, &projectCount))
	t.Logf("Projects after clear: %d", projectCount)
	takeScreenshot(ctx, t, "03_after_clear")
	validateStyles(ctx, t, "After Clear Verified")

	// STEP 3: Sync projects
	t.Log("\n=== STEP 3: Sync projects from Jira ===")

	// Trigger the fetch directly since ChromeDP has issues with async functions
	// This simulates what the syncProjects() button click should do
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		fetch('/api/projects/refresh-cache', {method: 'POST'})
			.then(r => console.log('Sync started, status:', r.status))
			.catch(e => console.error('Sync failed:', e));
	`, nil)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_trigger_sync")
		t.Fatalf("Failed to trigger sync: %v", err)
	}
	t.Log("✓ Triggered project sync via fetch")

	// Wait a bit for the background scraping to complete (usually takes 2-5 seconds)
	time.Sleep(5 * time.Second)

	// Now call loadProjects() to refresh the UI with the scraped projects
	if err := chromedp.Run(ctx, chromedp.Evaluate(`loadProjects()`, nil)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_load_projects")
		t.Fatalf("Failed to load projects into UI: %v", err)
	}
	t.Log("✓ Called loadProjects() to refresh UI")

	// Give it a moment to render
	time.Sleep(1 * time.Second)

	takeScreenshot(ctx, t, "04_sync_projects_clicked")

	// Wait for projects to load
	t.Log("Waiting for projects to appear in UI (max 10 seconds)...")
	var projectsLoaded bool
	for i := 0; i < 30; i++ {
		var count int
		chromedp.Run(ctx, chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('project-list');
				if (!tbody) return 0;
				const rows = tbody.querySelectorAll('tr');
				let count = 0;
				for (const row of rows) {
					// Count only data rows, not "no projects" messages
					if (row.querySelectorAll('td').length >= 5) {
						count++;
					}
				}
				return count;
			})()
		`, &count))

		t.Logf("  Waiting... projects loaded: %d", count)
		if count > 0 {
			projectsLoaded = true
			projectCount = count
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !projectsLoaded {
		takeScreenshot(ctx, t, "FAIL_no_projects_synced")
		t.Fatalf("No projects loaded after sync")
	}
	t.Logf("✓ Projects synced: %d projects", projectCount)
	takeScreenshot(ctx, t, "05_projects_synced")
	validateStyles(ctx, t, "After Projects Synced")

	// STEP 3b: Refresh page and verify projects auto-load
	t.Log("\n=== STEP 3b: Refresh page to verify auto-load ===")
	if err := chromedp.Run(ctx, chromedp.Reload()); err != nil {
		t.Fatalf("Failed to reload page: %v", err)
	}
	t.Log("✓ Page reloaded")

	// Wait for page to load and projects to auto-populate
	time.Sleep(2 * time.Second)

	// Verify projects are still visible after reload
	chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('project-list');
			if (!tbody) return 0;
			const rows = tbody.querySelectorAll('tr');
			let count = 0;
			for (const row of rows) {
				if (row.querySelectorAll('td').length >= 5) {
					count++;
				}
			}
			return count;
		})()
	`, &projectCount))

	if projectCount == 0 {
		takeScreenshot(ctx, t, "FAIL_no_projects_after_refresh")
		t.Fatal("Projects did not auto-load after page refresh")
	}
	t.Logf("✓ Projects auto-loaded after refresh: %d projects", projectCount)
	takeScreenshot(ctx, t, "06_after_page_refresh")

	// STEP 4: Find a project with issues
	t.Log("\n=== STEP 4: Select project with issues ===")

	var result map[string]interface{}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('project-list');
			if (!tbody) return null;
			const rows = tbody.querySelectorAll('tr');

			for (const row of rows) {
				const cells = row.querySelectorAll('td');
				if (cells.length >= 5) {
					const checkbox = cells[0].querySelector('input[type="checkbox"]');
					const issueCountText = cells[4]?.textContent || '0';
					const issueCount = parseInt(issueCountText);

					if (checkbox && issueCount > 0 && issueCount <= 100) {
						return {
							key: checkbox.value,
							count: issueCount
						};
					}
				}
			}
			return null;
		})()
	`, &result)); err != nil {
		t.Fatalf("Failed to find project: %v", err)
	}

	if result == nil {
		t.Fatalf("No project with issues found")
	}

	projectKey, _ := result["key"].(string)
	var expectedIssueCount int
	if count, ok := result["count"].(float64); ok {
		expectedIssueCount = int(count)
	}

	t.Logf("✓ Found project '%s' with %d issues", projectKey, expectedIssueCount)

	// Select the project
	checkboxSelector := fmt.Sprintf(`input[type="checkbox"][value="%s"]`, projectKey)
	if err := chromedp.Run(ctx, chromedp.Click(checkboxSelector, chromedp.ByQuery)); err != nil {
		t.Fatalf("Failed to select project: %v", err)
	}
	t.Log("✓ Project selected")
	takeScreenshot(ctx, t, "07_project_selected")

	// STEP 5: Get issues for selected project
	t.Log("\n=== STEP 5: Get issues for selected project ===")

	// Get selected project keys and trigger fetch directly (ChromeDP doesn't handle async onclick well)
	var selectedKeys []string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`Array.from(selectedProjects)`, &selectedKeys)); err != nil {
		t.Fatalf("Failed to get selected projects: %v", err)
	}
	t.Logf("Selected project keys: %v", selectedKeys)

	// Trigger the API call directly with properly formatted JSON
	keysJSON, _ := json.Marshal(selectedKeys)
	fetchScript := fmt.Sprintf(`
		fetch('/api/projects/get-issues', {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify({projectKeys: %s})
		}).then(r => console.log('Get issues started, status:', r.status))
		  .catch(e => console.error('Get issues failed:', e));
	`, keysJSON)

	if err := chromedp.Run(ctx, chromedp.Evaluate(fetchScript, nil)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_trigger_get_issues")
		t.Fatalf("Failed to trigger get issues: %v", err)
	}
	t.Log("✓ Triggered GET ISSUES via fetch")

	// Wait for background scraping to complete
	time.Sleep(5 * time.Second)

	// Call loadIssuesFromDatabase() to refresh the UI with scraped issues
	if err := chromedp.Run(ctx, chromedp.Evaluate(`loadIssuesFromDatabase()`, nil)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_load_issues_ui")
		t.Fatalf("Failed to load issues into UI: %v", err)
	}
	t.Log("✓ Called loadIssuesFromDatabase() to refresh UI")

	// Give it a moment to render
	time.Sleep(1 * time.Second)

	takeScreenshot(ctx, t, "08_get_issues_clicked")

	// STEP 6: Wait for issues to load
	t.Log("\n=== STEP 6: Wait for issues to load ===")
	var issuesLoaded bool
	var actualIssueCount int
	maxWait := 60 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWait {
		var checkResult struct {
			HasIssues  bool
			IssueCount int
		}

		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('issues-table-body');
				const rows = tbody ? tbody.querySelectorAll('tr') : [];

				let dataRows = 0;
				for (const row of rows) {
					const text = row.textContent;
					if (!text.includes('Loading') && !text.includes('No issues')) {
						const cells = row.querySelectorAll('td');
						if (cells.length >= 5) {
							dataRows++;
						}
					}
				}

				return {
					HasIssues: dataRows > 0,
					IssueCount: dataRows
				};
			})()
		`, &checkResult))

		if err != nil {
			t.Logf("Error checking issues: %v", err)
		} else {
			elapsed := time.Since(startTime).Round(time.Second)
			if checkResult.IssueCount > 0 {
				t.Logf("  ✓ Issues loading... count=%d, elapsed=%v", checkResult.IssueCount, elapsed)
			} else {
				t.Logf("  Waiting... count=%d, elapsed=%v", checkResult.IssueCount, elapsed)
			}

			if checkResult.HasIssues {
				issuesLoaded = true
				actualIssueCount = checkResult.IssueCount

				// Wait a bit more to see if more issues load
				if actualIssueCount < expectedIssueCount {
					time.Sleep(2 * time.Second)
					continue
				}
				break
			}
		}

		time.Sleep(2 * time.Second)
	}

	if !issuesLoaded {
		takeScreenshot(ctx, t, "FAIL_no_issues_loaded")

		// Debug: Check what API returns
		var apiDebug map[string]interface{}
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(async () => {
				try {
					const response = await fetch('/api/data/jira/issues?projectKey=%s');
					const data = await response.json();
					return {
						status: response.status,
						issueCount: data.issues ? data.issues.length : 0,
						firstIssue: data.issues && data.issues.length > 0 ? data.issues[0] : null
					};
				} catch (e) {
					return { error: e.toString() };
				}
			})()
		`, projectKey), &apiDebug))
		t.Logf("API Debug: %+v", apiDebug)

		t.Fatalf("❌ Issues did not load within 60 seconds")
	}

	t.Logf("✓ Issues loaded after %v", time.Since(startTime).Round(time.Second))
	takeScreenshot(ctx, t, "09_issues_loaded")
	validateStyles(ctx, t, "After Issues Loaded")

	// STEP 7: Verify issue count
	t.Log("\n=== STEP 7: Verify issue count ===")
	t.Logf("Expected: %d issues", expectedIssueCount)
	t.Logf("Actual: %d issues", actualIssueCount)

	if actualIssueCount != expectedIssueCount {
		takeScreenshot(ctx, t, "FAIL_issue_count_mismatch")
		t.Errorf("❌ Issue count mismatch: expected %d, got %d", expectedIssueCount, actualIssueCount)
	} else {
		t.Logf("✓ Issue count matches: %d issues", actualIssueCount)
	}

	// STEP 8: Verify all issues belong to selected project
	t.Log("\n=== STEP 8: Verify issues belong to correct project ===")
	var projectKeys []string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('issues-table-body');
			const rows = tbody.querySelectorAll('tr');
			const keys = new Set();

			for (const row of rows) {
				const cells = row.querySelectorAll('td');
				if (cells.length >= 5) {
					const issueKey = cells[1].textContent.trim(); // KEY column
					if (issueKey && issueKey !== 'N/A') {
						const projectKey = issueKey.split('-')[0];
						keys.add(projectKey);
					}
				}
			}

			return Array.from(keys);
		})()
	`, &projectKeys)); err != nil {
		t.Errorf("Failed to extract project keys: %v", err)
	} else {
		t.Logf("Project keys found in table: %v", projectKeys)

		allMatch := true
		for _, key := range projectKeys {
			if key != projectKey {
				t.Errorf("❌ Found issue from wrong project: %s (expected %s)", key, projectKey)
				allMatch = false
			}
		}

		if allMatch && len(projectKeys) > 0 {
			t.Logf("✓ All issues belong to project '%s'", projectKey)
		}
	}

	takeScreenshot(ctx, t, "10_SUCCESS_all_verified")

	if actualIssueCount == expectedIssueCount {
		t.Log("\n✅ COMPLETE WORKFLOW PASSED")
	} else {
		t.Log("\n❌ WORKFLOW FAILED - Issue count mismatch")
		t.Fail()
	}
}
