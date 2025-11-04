package ui

import (
	"github.com/ternarybob/quaero/test/common"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pelletier/go-toml/v2"
	appcommon "github.com/ternarybob/quaero/internal/common"
	_ "modernc.org/sqlite"
)

// TestJobDeletion_BulkDeletion tests bulk deletion of jobs via the "Delete Selected" modal
func TestJobDeletion_BulkDeletion(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("BulkDeletion")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	t.Logf("Test environment ready, service running at: %s", env.GetBaseURL())
	t.Logf("Results directory: %s", env.GetResultsDir())

	// Get database path from config
	dbPath, err := getDatabasePath()
	if err != nil {
		t.Fatalf("Failed to get database path: %v", err)
	}
	t.Logf("Using database: %s", dbPath)

	// Connect to database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Get initial job count
	initialCount, err := getJobCount(db)
	if err != nil {
		t.Fatalf("Failed to get initial job count: %v", err)
	}
	t.Logf("Initial job count: %d", initialCount)

	if initialCount == 0 {
		t.Skip("No jobs in database - cannot test deletion (create some jobs first)")
	}

	// Set up browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Navigate to queue page
	queueURL := env.GetBaseURL() + "/queue"

	var jobCount int
	var modalVisible bool
	var debugInfo map[string]interface{}

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(queueURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for jobs to load

		// Debug: Check Alpine.js state
		chromedp.Evaluate(`
			(() => {
				const jobListEl = document.querySelector('[x-data="jobList"]');
				if (jobListEl && jobListEl.__x) {
					const data = jobListEl.__x.$data;
					return {
						allJobsCount: data.allJobs ? data.allJobs.length : 0,
						filteredJobsCount: data.filteredJobs ? data.filteredJobs.length : 0,
						renderedRowCount: document.querySelectorAll('[x-data="jobList"] .table tbody tr').length
					};
				}
				return {allJobsCount: 0, filteredJobsCount: 0, renderedRowCount: 0};
			})()
		`, &debugInfo),

		// Get actual row count
		chromedp.Evaluate(`
			document.querySelectorAll('[x-data="jobList"] .table tbody tr').length
		`, &jobCount),
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	t.Logf("Alpine.js state: %+v", debugInfo)
	t.Logf("Jobs displayed in UI: %d", jobCount)

	if jobCount == 0 {
		t.Skip("No jobs displayed in UI - cannot test deletion")
	}

	// Select the first job and delete it
	var confirmationMessage string

	err = chromedp.Run(ctx,
		// Select first job checkbox
		chromedp.Click(`[x-data="jobList"] .table tbody tr:first-child input[type="checkbox"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Click Delete Selected button
		chromedp.WaitVisible(`#delete-selected-btn`, chromedp.ByQuery),
		chromedp.Click(`#delete-selected-btn`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// Wait for modal to appear
		chromedp.WaitVisible(`#delete-confirm-modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Check if modal is visible
		chromedp.Evaluate(`
			document.getElementById('delete-confirm-modal').classList.contains('active')
		`, &modalVisible),

		// Get the confirmation message
		chromedp.Text(`.toast-warning span`, &confirmationMessage, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to open deletion modal: %v", err)
	}

	if !modalVisible {
		t.Fatal("Deletion modal did not appear")
	}

	t.Logf("Modal confirmation message: '%s'", confirmationMessage)

	// Verify the message shows the correct count
	if confirmationMessage == "" {
		t.Error("Modal message is empty")
	}
	if confirmationMessage == "You are about to delete 0 job. This action cannot be undone." {
		t.Fatal("BUG: Modal shows 'delete 0 job' - jobs array was not passed correctly")
	}
	if confirmationMessage != "You are about to delete 1 job. This action cannot be undone." {
		t.Logf("Warning: Expected 'delete 1 job', got: '%s'", confirmationMessage)
	}

	// Check the confirmation checkbox and click Delete
	err = chromedp.Run(ctx,
		// Click confirmation checkbox
		chromedp.Click(`#delete-confirm-modal input[type="checkbox"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Click Delete button
		chromedp.Click(`#delete-confirm-modal button.btn-error`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for deletion to complete

		// Wait for modal to close
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to complete deletion: %v", err)
	}

	// Verify job count in database decreased
	finalCount, err := getJobCount(db)
	if err != nil {
		t.Fatalf("Failed to get final job count: %v", err)
	}
	t.Logf("Final job count: %d", finalCount)

	expectedCount := initialCount - 1
	if finalCount != expectedCount {
		t.Errorf("Expected job count to decrease by 1. Initial: %d, Final: %d, Expected: %d",
			initialCount, finalCount, expectedCount)

		// Additional debugging
		t.Logf("Checking if job still exists in database...")
		time.Sleep(1 * time.Second)
		recheckCount, _ := getJobCount(db)
		t.Logf("Recheck count: %d", recheckCount)

		if recheckCount == finalCount {
			t.Fatal("Job was NOT deleted from database")
		}
	} else {
		t.Log("✓ Job count decreased by 1 in database (bulk deletion)")
	}

	// Verify UI also shows one less job
	var finalUIJobCount int
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for UI refresh
		chromedp.Evaluate(`
			document.querySelectorAll('[x-data="jobList"] .table tbody tr').length
		`, &finalUIJobCount),
	)

	if err != nil {
		t.Logf("Warning: Failed to get final UI job count: %v", err)
	} else {
		t.Logf("Final jobs displayed in UI: %d", finalUIJobCount)
		if finalUIJobCount != jobCount-1 {
			t.Errorf("UI should show %d jobs, but shows %d", jobCount-1, finalUIJobCount)
		} else {
			t.Log("✓ UI job count decreased by 1 (bulk deletion)")
		}
	}

	t.Log("✓ Bulk deletion test completed successfully")
}

// TestJobDeletion_IndividualDeletion tests deleting a single job directly from the list
func TestJobDeletion_IndividualDeletion(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("IndividualDeletion")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	t.Logf("Test environment ready, service running at: %s", env.GetBaseURL())
	t.Logf("Results directory: %s", env.GetResultsDir())

	// Get database path from config
	dbPath, err := getDatabasePath()
	if err != nil {
		t.Fatalf("Failed to get database path: %v", err)
	}
	t.Logf("Using database: %s", dbPath)

	// Connect to database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Get initial job count
	initialCount, err := getJobCount(db)
	if err != nil {
		t.Fatalf("Failed to get initial job count: %v", err)
	}
	t.Logf("Initial job count: %d", initialCount)

	if initialCount == 0 {
		t.Skip("No jobs in database - cannot test deletion (create some jobs first)")
	}

	// Set up browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Navigate to queue page
	queueURL := env.GetBaseURL() + "/queue"

	var jobCount int

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(queueURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for jobs to load

		// Get row count
		chromedp.Evaluate(`
			document.querySelectorAll('[x-data="jobList"] .table tbody tr').length
		`, &jobCount),
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	t.Logf("Jobs displayed in UI: %d", jobCount)

	if jobCount == 0 {
		t.Skip("No jobs displayed in UI - cannot test deletion")
	}

	// Find and click the individual delete button for the first non-running job
	var deleteButtonExists bool
	err = chromedp.Run(ctx,
		// Check if delete button exists for first job
		chromedp.Evaluate(`
			(() => {
				const firstRow = document.querySelector('[x-data="jobList"] .table tbody tr:first-child');
				if (firstRow) {
					const deleteBtn = firstRow.querySelector('button.btn-error[title="Delete Job"]');
					return deleteBtn !== null;
				}
				return false;
			})()
		`, &deleteButtonExists),
	)

	if err != nil {
		t.Fatalf("Failed to check for delete button: %v", err)
	}

	if !deleteButtonExists {
		t.Skip("First job is running - cannot test individual deletion (delete button not shown for running jobs)")
	}

	t.Log("Found delete button on first job, proceeding with individual deletion")

	// Capture any notifications that appear
	var notificationText string
	err = chromedp.Run(ctx,
		// Override window.confirm to auto-accept before clicking
		chromedp.Evaluate(`
			window.originalConfirm = window.confirm;
			window.confirm = function() { return true; };
		`, nil),
		chromedp.Sleep(200*time.Millisecond),

		// Click the individual delete button - will auto-accept the confirmation
		chromedp.Click(`[x-data="jobList"] .table tbody tr:first-child button.btn-error[title="Delete Job"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for deletion to complete

		// Try to capture notification
		chromedp.Evaluate(`
			(() => {
				const notification = document.querySelector('.notification, .toast');
				if (notification) {
					return notification.textContent.trim();
				}
				return '';
			})()
		`, &notificationText),
	)

	if err != nil {
		t.Logf("Warning during individual deletion: %v", err)
	}

	if notificationText != "" {
		t.Logf("Notification: %s", notificationText)
	}

	// Verify job count in database decreased
	finalCount, err := getJobCount(db)
	if err != nil {
		t.Fatalf("Failed to get final job count: %v", err)
	}
	t.Logf("Final job count: %d", finalCount)

	// Allow deletion to succeed OR stay the same (if there was an error)
	if finalCount == initialCount {
		t.Log("⚠ Job count did NOT decrease - individual deletion may have failed")
		if notificationText != "" {
			t.Logf("Notification text suggests: %s", notificationText)
		}
		t.Fatal("Individual deletion failed - job was not deleted from database")
	} else if finalCount == initialCount-1 {
		t.Log("✓ Job count decreased by 1 in database (individual deletion)")
	} else {
		t.Errorf("Unexpected job count change. Initial: %d, Final: %d", initialCount, finalCount)
	}

	// Verify UI also shows one less job
	var finalUIJobCount int
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for UI refresh
		chromedp.Evaluate(`
			document.querySelectorAll('[x-data="jobList"] .table tbody tr').length
		`, &finalUIJobCount),
	)

	if err != nil {
		t.Logf("Warning: Failed to get final UI job count: %v", err)
	} else {
		t.Logf("Final jobs displayed in UI: %d", finalUIJobCount)
		if finalUIJobCount != jobCount-1 {
			t.Logf("Warning: UI should show %d jobs, but shows %d", jobCount-1, finalUIJobCount)
		} else {
			t.Log("✓ UI job count decreased by 1 (individual deletion)")
		}
	}

	t.Log("✓ Individual deletion test completed")
}

// getDatabasePath reads the database path from the quaero.toml config
func getDatabasePath() (string, error) {
	// Try different config locations
	configPaths := []string{
		filepath.Join("..", "bin", "quaero.toml"),
		filepath.Join("bin", "quaero.toml"),
		"bin/quaero.toml",
		"../bin/quaero.toml",
		"../../bin/quaero.toml",
	}

	var configPath string
	var data []byte
	var err error

	for _, path := range configPaths {
		data, err = os.ReadFile(path)
		if err == nil {
			configPath = path
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to read config file (tried %v): %w", configPaths, err)
	}

	var config appcommon.Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	dbPath := config.Storage.SQLite.Path
	if dbPath == "" {
		// Use default path
		dbPath = "./data/quaero.db"
	}

	// Make path absolute if relative
	if !filepath.IsAbs(dbPath) {
		// Resolve relative to the config file location
		configDir := filepath.Dir(configPath)
		dbPath = filepath.Join(configDir, dbPath)
	}

	return dbPath, nil
}

// getJobCount returns the total count of jobs in the crawl_jobs table
func getJobCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM crawl_jobs").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to query job count: %w", err)
	}
	return count, nil
}
