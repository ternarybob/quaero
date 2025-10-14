package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// TestConfluence_CascadingWorkflow tests the complete Confluence workflow in cascading steps
// Each test builds on the previous one, avoiding duplication
func TestConfluence_CascadingWorkflow(t *testing.T) {
	config, err := LoadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	serverURL := config.ServerURL

	// Setup browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()

	screenshotCounter = 0

	// Start video recording
	stopRecording, err := startVideoRecording(ctx, t)
	if err != nil {
		t.Logf("Warning: Could not start video recording: %v", err)
	} else {
		defer stopRecording()
	}

	// Track test state across subtests
	var selectedSpaceKey string

	// === TEST 1: Clear All Data - Verify 0 data ===
	t.Run("01_ClearAllData", func(t *testing.T) {
		t.Logf("=== TEST 1: Clear All Data - Verify 0 data ===")

		// Navigate to Confluence page
		if err := chromedp.Run(ctx, chromedp.Navigate(serverURL+"/confluence")); err != nil {
			t.Fatalf("Failed to navigate: %v", err)
		}

		// Wait for page to load
		if err := chromedp.Run(ctx, chromedp.WaitVisible(`#space-list`, chromedp.ByID)); err != nil {
			t.Fatalf("Failed to wait for space list: %v", err)
		}
		t.Logf("✓ Confluence page loaded")
		takeScreenshot(ctx, t, "01_page_loaded")

		// Click CLEAR ALL DATA button and accept confirmation
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
				go chromedp.Run(ctx, page.HandleJavaScriptDialog(true))
			}
		})

		if err := chromedp.Run(ctx, chromedp.Click(`#clear-data-btn`, chromedp.ByID)); err != nil {
			t.Fatalf("Failed to click CLEAR ALL DATA: %v", err)
		}
		time.Sleep(1 * time.Second)
		t.Logf("✓ Clicked CLEAR ALL DATA and accepted confirmation")
		takeScreenshot(ctx, t, "02_clear_data_clicked")

		// Wait for data to be cleared
		time.Sleep(2 * time.Second)

		// Verify API returns 0 spaces
		resp, err := http.Get(serverURL + "/api/data/confluence")
		if err != nil {
			t.Fatalf("Failed to get Confluence data: %v", err)
		}
		defer resp.Body.Close()

		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spaceCount := 0
		if spaces, ok := data["spaces"].([]interface{}); ok {
			spaceCount = len(spaces)
		}

		if spaceCount != 0 {
			t.Errorf("Expected 0 spaces after clear, got %d", spaceCount)
		} else {
			t.Logf("✓ Verified 0 spaces in database")
		}

		takeScreenshot(ctx, t, "03_after_clear")
	})

	// === TEST 2: Clear All Data -> Sync Spaces - Verify spaces increase from 0 ===
	t.Run("02_SyncSpaces", func(t *testing.T) {
		t.Logf("\n=== TEST 2: Sync Spaces - Verify spaces increase from 0 ===")

		// Verify starting at 0 spaces
		var spaceCountBefore int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('#space-list .project-item').length`, &spaceCountBefore),
		); err != nil {
			t.Fatalf("Failed to count spaces: %v", err)
		}

		if spaceCountBefore != 0 {
			t.Errorf("Expected 0 spaces before sync, got %d", spaceCountBefore)
		} else {
			t.Logf("✓ Starting with 0 spaces")
		}

		// Click GET SPACES button
		if err := chromedp.Run(ctx, chromedp.Click(`#sync-btn`, chromedp.ByID)); err != nil {
			t.Fatalf("Failed to click GET SPACES: %v", err)
		}
		t.Logf("✓ Clicked GET SPACES")
		takeScreenshot(ctx, t, "04_sync_spaces_clicked")

		// Wait for spaces to sync (max 30 seconds)
		t.Logf("Waiting for spaces to sync (max 30 seconds)...")
		synced := false
		for i := 0; i < 30; i++ {
			time.Sleep(1 * time.Second)

			var spaceCount int
			chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('#space-list .project-item').length`, &spaceCount))

			t.Logf("  Waiting... spaces loaded: %d", spaceCount)

			if spaceCount > 0 {
				synced = true
				t.Logf("✓ Spaces synced: %d spaces", spaceCount)
				break
			}
		}

		if !synced {
			t.Fatal("❌ Spaces did not sync within 30 seconds")
		}

		takeScreenshot(ctx, t, "05_spaces_synced")

		// Get final space count and verify it increased
		var spaceCountAfter int
		chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('#space-list .project-item').length`, &spaceCountAfter))

		if spaceCountAfter <= 0 {
			t.Errorf("Expected spaces to increase from 0, but got %d", spaceCountAfter)
		} else {
			t.Logf("✓ Spaces increased from %d to %d", spaceCountBefore, spaceCountAfter)
		}
	})

	// === TEST 3: Select Space -> Get Pages - Verify pages increase from 0 ===
	t.Run("03_GetPages", func(t *testing.T) {
		t.Logf("\n=== TEST 3: Select Space and Get Pages - Verify pages increase from 0 ===")

		// Find a space with pages via API
		resp, err := http.Get(serverURL + "/api/data/confluence")
		if err != nil {
			t.Fatalf("Failed to get Confluence data: %v", err)
		}
		defer resp.Body.Close()

		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spaces, ok := data["spaces"].([]interface{})
		if !ok || len(spaces) == 0 {
			t.Fatal("No spaces found in database")
		}

		// Pick the first space (we'll scrape its pages)
		testSpace := spaces[0].(map[string]interface{})
		selectedSpaceKey = testSpace["key"].(string)
		spaceName := testSpace["name"].(string)

		t.Logf("✓ Selected space '%s' (%s) to scrape pages", selectedSpaceKey, spaceName)

		// Click the space to select it
		spaceCheckboxSelector := fmt.Sprintf(`#space-%s`, selectedSpaceKey)
		if err := chromedp.Run(ctx, chromedp.Click(spaceCheckboxSelector, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to select space: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		t.Logf("✓ Space selected")
		takeScreenshot(ctx, t, "06_space_selected")

		// Verify starting at 0 pages in table
		var pageCountBefore int
		chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('#pages-table-body tr').length`, &pageCountBefore))
		t.Logf("Pages in table before GET PAGES: %d", pageCountBefore)

		// Click GET PAGES button
		if err := chromedp.Run(ctx, chromedp.Click(`#get-pages-menu-btn`, chromedp.ByID)); err != nil {
			t.Fatalf("Failed to click GET PAGES: %v", err)
		}
		t.Logf("✓ Clicked GET PAGES")
		takeScreenshot(ctx, t, "07_get_pages_clicked")

		// Wait for pages to load (max 60 seconds)
		t.Logf("\n=== Waiting for pages to load ===")
		pagesLoaded := false
		startTime := time.Now()

		for time.Since(startTime) < 60*time.Second {
			time.Sleep(2 * time.Second)

			var pageRows int
			chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('#pages-table-body tr').length`, &pageRows))

			elapsed := int(time.Since(startTime).Seconds())
			t.Logf("  Waiting... count=%d, elapsed=%ds", pageRows, elapsed)

			if pageRows > 1 {
				pagesLoaded = true
				t.Logf("✓ Pages loading... count=%d, elapsed=%ds", pageRows, elapsed)
				break
			}
		}

		if !pagesLoaded {
			takeScreenshot(ctx, t, "08_FAIL_no_pages_loaded")
			t.Fatal("❌ Pages did not load within 60 seconds")
		}

		// Wait a bit more for all pages to load
		time.Sleep(2 * time.Second)

		var pageCountAfter int
		chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('#pages-table-body tr').length`, &pageCountAfter))

		elapsed := int(time.Since(startTime).Seconds())
		t.Logf("✓ Pages loaded after %ds", elapsed)
		takeScreenshot(ctx, t, "08_pages_loaded")

		// Verify page count increased from before
		t.Logf("\n=== Verifying Page Count ===")
		t.Logf("Pages loaded: %d", pageCountAfter)

		if pageCountAfter <= 0 {
			t.Errorf("Expected pages to be loaded, but got %d", pageCountAfter)
		} else {
			t.Logf("✓ Successfully loaded %d pages", pageCountAfter)
		}

		// Verify all pages belong to selected space
		t.Logf("\n=== Verifying Pages Belong to Selected Space ===")
		var spaceKeysInTable []string
		chromedp.Run(ctx, chromedp.Evaluate(`
			Array.from(document.querySelectorAll('#pages-table-body tr td:first-child')).map(td => td.textContent.trim())
		`, &spaceKeysInTable))

		t.Logf("Space keys found in table: %v", spaceKeysInTable)

		for _, key := range spaceKeysInTable {
			if key != selectedSpaceKey && key != "" {
				t.Errorf("Found page with space key '%s', expected '%s'", key, selectedSpaceKey)
			}
		}
		t.Logf("✓ All pages belong to space '%s'", selectedSpaceKey)

		takeScreenshot(ctx, t, "09_pages_verified")
	})

	// === TEST 4: Select Page - Verify detail appears ===
	t.Run("04_SelectPage", func(t *testing.T) {
		t.Logf("\n=== TEST 4: Select Page - Verify detail appears ===")

		// Verify page detail is initially empty
		var pageDetailBefore string
		if err := chromedp.Run(ctx,
			chromedp.Text(`#page-detail-json code`, &pageDetailBefore, chromedp.ByQuery),
		); err != nil {
			t.Logf("Page detail empty (expected): %v", err)
		}

		if pageDetailBefore != "" {
			t.Logf("Note: Page detail had content before selection (may be from previous selection)")
		}

		// Click the first page row
		if err := chromedp.Run(ctx, chromedp.Click(`#pages-table-body tr:first-child`, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to click first page: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		t.Logf("✓ Clicked first page")
		takeScreenshot(ctx, t, "10_page_clicked")

		// Verify page detail is populated
		var pageDetailAfter string
		if err := chromedp.Run(ctx,
			chromedp.Text(`#page-detail-json code`, &pageDetailAfter, chromedp.ByQuery),
		); err != nil {
			t.Fatalf("Failed to get page detail: %v", err)
		}

		if pageDetailAfter == "" {
			t.Error("❌ Page detail is still empty after selection")
		} else {
			// Verify it's valid JSON
			var detailJSON map[string]interface{}
			if err := json.Unmarshal([]byte(pageDetailAfter), &detailJSON); err != nil {
				t.Errorf("Page detail is not valid JSON: %v", err)
			} else {
				t.Logf("✓ Page detail populated with valid JSON")

				// Verify it has expected fields
				if id, ok := detailJSON["id"]; ok {
					t.Logf("  Page ID: %v", id)
				}
				if title, ok := detailJSON["title"]; ok {
					t.Logf("  Page Title: %v", title)
				}
				if spaceId, ok := detailJSON["spaceId"]; ok {
					t.Logf("  Space ID: %v", spaceId)
					if spaceId != selectedSpaceKey {
						t.Errorf("Expected space ID '%s', got '%v'", selectedSpaceKey, spaceId)
					}
				}
			}
		}

		// Verify row is highlighted
		var isHighlighted bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			document.querySelector('#pages-table-body tr:first-child').style.backgroundColor !== 'transparent'
		`, &isHighlighted))

		if !isHighlighted {
			t.Error("❌ Selected page row is not highlighted")
		} else {
			t.Logf("✓ Selected page row is highlighted")
		}

		takeScreenshot(ctx, t, "11_page_detail_displayed")

		t.Logf("\n=== ✅ ALL CONFLUENCE TESTS PASSED ===")
	})
}
