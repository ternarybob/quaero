package ui

import (
	"context"
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
	if err := chromedp.Run(ctx, chromedp.Click(`#sync-btn`, chromedp.ByID)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_click_sync_projects")
		t.Fatalf("Failed to click GET PROJECTS: %v", err)
	}
	t.Log("✓ Clicked GET PROJECTS")
	takeScreenshot(ctx, t, "04_sync_projects_clicked")

	// Wait for projects to load
	t.Log("Waiting for projects to sync (max 30 seconds)...")
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
	takeScreenshot(ctx, t, "06_project_selected")

	// STEP 5: Get issues for selected project
	t.Log("\n=== STEP 5: Get issues for selected project ===")
	if err := chromedp.Run(ctx, chromedp.Click(`#get-issues-menu-btn`, chromedp.ByID)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_click_get_issues")
		t.Fatalf("Failed to click GET ISSUES: %v", err)
	}
	t.Log("✓ Clicked GET ISSUES")
	takeScreenshot(ctx, t, "07_get_issues_clicked")

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
	takeScreenshot(ctx, t, "08_issues_loaded")
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

	takeScreenshot(ctx, t, "09_SUCCESS_all_verified")

	if actualIssueCount == expectedIssueCount {
		t.Log("\n✅ COMPLETE WORKFLOW PASSED")
	} else {
		t.Log("\n❌ WORKFLOW FAILED - Issue count mismatch")
		t.Fail()
	}
}
