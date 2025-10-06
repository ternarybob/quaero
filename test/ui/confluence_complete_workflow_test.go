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

func TestConfluence_CompleteWorkflow(t *testing.T) {
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

	// Navigate to confluence page
	t.Log("=== STEP 1: Navigate to Confluence page ===")
	if err := chromedp.Run(ctx, chromedp.Navigate(serverURL+"/confluence")); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	if err := chromedp.Run(ctx, chromedp.WaitVisible(`#space-list`, chromedp.ByID)); err != nil {
		t.Fatalf("Page did not load: %v", err)
	}
	t.Log("✓ Confluence page loaded")
	takeScreenshot(ctx, t, "01_page_loaded")

	// STEP 2: Clear all data
	t.Log("\n=== STEP 2: Clear all Confluence data ===")

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

	// Wait for clear to complete (spaces should be empty)
	time.Sleep(3 * time.Second)

	var spaceCount int
	chromedp.Run(ctx, chromedp.Evaluate(`
		document.querySelectorAll('.project-item').length
	`, &spaceCount))
	t.Logf("Spaces after clear: %d", spaceCount)
	takeScreenshot(ctx, t, "03_after_clear")

	// STEP 3: Sync spaces
	t.Log("\n=== STEP 3: Sync spaces from Confluence ===")
	if err := chromedp.Run(ctx, chromedp.Click(`#sync-btn`, chromedp.ByID)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_click_sync_spaces")
		t.Fatalf("Failed to click GET SPACES: %v", err)
	}
	t.Log("✓ Clicked GET SPACES")
	takeScreenshot(ctx, t, "04_sync_spaces_clicked")

	// Wait for spaces to load
	t.Log("Waiting for spaces to sync (max 30 seconds)...")
	var spacesLoaded bool
	for i := 0; i < 30; i++ {
		var count int
		chromedp.Run(ctx, chromedp.Evaluate(`
			document.querySelectorAll('.project-item').length
		`, &count))

		t.Logf("  Waiting... spaces loaded: %d", count)
		if count > 0 {
			spacesLoaded = true
			spaceCount = count
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !spacesLoaded {
		takeScreenshot(ctx, t, "FAIL_no_spaces_synced")
		t.Fatalf("No spaces loaded after sync")
	}
	t.Logf("✓ Spaces synced: %d spaces", spaceCount)
	takeScreenshot(ctx, t, "05_spaces_synced")

	// STEP 4: Find a space with pages
	t.Log("\n=== STEP 4: Select space with pages ===")

	var result map[string]interface{}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const spaces = Array.from(document.querySelectorAll('.project-item'));
			for (const space of spaces) {
				const checkbox = space.querySelector('input[type="checkbox"]');
				const pageCountText = space.querySelector('.project-issues')?.textContent || '0 pages';
				const pageCount = parseInt(pageCountText);

				if (pageCount > 0 && pageCount <= 100) {
					return {
						key: checkbox.value,
						count: pageCount
					};
				}
			}
			return null;
		})()
	`, &result)); err != nil {
		t.Fatalf("Failed to find space: %v", err)
	}

	if result == nil {
		t.Fatalf("No space with pages found")
	}

	spaceKey, _ := result["key"].(string)
	var expectedPageCount int
	if count, ok := result["count"].(float64); ok {
		expectedPageCount = int(count)
	}

	t.Logf("✓ Found space '%s' with %d pages", spaceKey, expectedPageCount)

	// Select the space
	checkboxSelector := fmt.Sprintf(`input[type="checkbox"][value="%s"]`, spaceKey)
	if err := chromedp.Run(ctx, chromedp.Click(checkboxSelector, chromedp.ByQuery)); err != nil {
		t.Fatalf("Failed to select space: %v", err)
	}
	t.Log("✓ Space selected")
	takeScreenshot(ctx, t, "06_space_selected")

	// STEP 5: Get pages for selected space
	t.Log("\n=== STEP 5: Get pages for selected space ===")
	if err := chromedp.Run(ctx, chromedp.Click(`#get-pages-menu-btn`, chromedp.ByID)); err != nil {
		takeScreenshot(ctx, t, "FAIL_cannot_click_get_pages")
		t.Fatalf("Failed to click GET PAGES: %v", err)
	}
	t.Log("✓ Clicked GET PAGES")
	takeScreenshot(ctx, t, "07_get_pages_clicked")

	// STEP 6: Wait for pages to load
	t.Log("\n=== STEP 6: Wait for pages to load ===")
	var pagesLoaded bool
	var actualPageCount int
	maxWait := 60 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWait {
		var checkResult struct {
			HasPages  bool
			PageCount int
		}

		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('pages-table-body');
				const rows = tbody ? tbody.querySelectorAll('tr') : [];

				let dataRows = 0;
				for (const row of rows) {
					const text = row.textContent;
					if (!text.includes('Loading') && !text.includes('No pages')) {
						const cells = row.querySelectorAll('td');
						if (cells.length >= 4) {
							dataRows++;
						}
					}
				}

				return {
					HasPages: dataRows > 0,
					PageCount: dataRows
				};
			})()
		`, &checkResult))

		if err != nil {
			t.Logf("Error checking pages: %v", err)
		} else {
			elapsed := time.Since(startTime).Round(time.Second)
			if checkResult.PageCount > 0 {
				t.Logf("  ✓ Pages loading... count=%d, elapsed=%v", checkResult.PageCount, elapsed)
			} else {
				t.Logf("  Waiting... count=%d, elapsed=%v", checkResult.PageCount, elapsed)
			}

			if checkResult.HasPages {
				pagesLoaded = true
				actualPageCount = checkResult.PageCount

				// Wait a bit more to see if more pages load
				if actualPageCount < expectedPageCount {
					time.Sleep(2 * time.Second)
					continue
				}
				break
			}
		}

		time.Sleep(2 * time.Second)
	}

	if !pagesLoaded {
		takeScreenshot(ctx, t, "FAIL_no_pages_loaded")

		// Debug: Check what API returns
		var apiDebug map[string]interface{}
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(async () => {
				try {
					const response = await fetch('/api/data/confluence/pages?spaceKey=%s');
					const data = await response.json();
					return {
						status: response.status,
						pageCount: data.pages ? data.pages.length : 0,
						firstPage: data.pages && data.pages.length > 0 ? data.pages[0] : null
					};
				} catch (e) {
					return { error: e.toString() };
				}
			})()
		`, spaceKey), &apiDebug))
		t.Logf("API Debug: %+v", apiDebug)

		t.Fatalf("❌ Pages did not load within 60 seconds")
	}

	t.Logf("✓ Pages loaded after %v", time.Since(startTime).Round(time.Second))
	takeScreenshot(ctx, t, "08_pages_loaded")

	// STEP 7: Verify page count
	t.Log("\n=== STEP 7: Verify page count ===")
	t.Logf("Expected: %d pages", expectedPageCount)
	t.Logf("Actual: %d pages", actualPageCount)

	if actualPageCount != expectedPageCount {
		takeScreenshot(ctx, t, "FAIL_page_count_mismatch")
		t.Errorf("❌ Page count mismatch: expected %d, got %d", expectedPageCount, actualPageCount)
	} else {
		t.Logf("✓ Page count matches: %d pages", actualPageCount)
	}

	// STEP 8: Verify all pages belong to selected space
	t.Log("\n=== STEP 8: Verify pages belong to correct space ===")
	var spaceKeys []string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('pages-table-body');
			const rows = tbody.querySelectorAll('tr');
			const keys = new Set();

			for (const row of rows) {
				const cells = row.querySelectorAll('td');
				if (cells.length >= 4) {
					const spaceKey = cells[0].textContent.trim(); // Space column
					if (spaceKey && spaceKey !== 'N/A') {
						keys.add(spaceKey);
					}
				}
			}

			return Array.from(keys);
		})()
	`, &spaceKeys)); err != nil {
		t.Errorf("Failed to extract space keys: %v", err)
	} else {
		t.Logf("Space keys found in table: %v", spaceKeys)

		allMatch := true
		for _, key := range spaceKeys {
			if key != spaceKey {
				t.Errorf("❌ Found page from wrong space: %s (expected %s)", key, spaceKey)
				allMatch = false
			}
		}

		if allMatch && len(spaceKeys) > 0 {
			t.Logf("✓ All pages belong to space '%s'", spaceKey)
		}
	}

	takeScreenshot(ctx, t, "09_SUCCESS_all_verified")

	if actualPageCount == expectedPageCount {
		t.Log("\n✅ COMPLETE WORKFLOW PASSED")
	} else {
		t.Log("\n❌ WORKFLOW FAILED - Page count mismatch")
		t.Fail()
	}
}
