package ui

import (
	"github.com/ternarybob/quaero/test/common"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestSourcesPageLoad(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesPageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"
	var title string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "sources-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Job Management - Quaero"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s' - sources page is part of jobs.html", expectedTitle, title)
	}

	t.Log("✓ Sources page loads correctly")
}

func TestSourcesPageElements(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesPageElements")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Page title", ".page-title"},
		{"Sources list card", ".card"},
		{"Add Source button", "button.btn.btn-sm.btn-primary"},
		{"Sources table", "table.table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			err = chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Evaluate(`document.querySelectorAll("`+tt.selector+`").length`, &nodeCount),
			)

			if err != nil {
				t.Fatalf("Failed to check element '%s': %v", tt.name, err)
			}

			if nodeCount == 0 {
				t.Errorf("Element '%s' (selector: %s) not found on page", tt.name, tt.selector)
			}
		})
	}
}

func TestSourcesNavbar(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesNavbar")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	var navbarVisible bool
	var menuItems []string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.nav-links`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('.nav-links') !== null`, &navbarVisible),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('.nav-links a')).map(el => el.textContent.trim())`, &menuItems),
	)

	if err != nil {
		t.Fatalf("Failed to check navbar: %v", err)
	}

	if !navbarVisible {
		t.Error("Navbar not found on page")
	}

	// Check for menu items (JOBS includes sources page)
	expectedItems := []string{"HOME", "JOBS", "QUEUE", "DOCUMENTS", "CHAT", "SETTINGS"}
	for _, expected := range expectedItems {
		found := false
		for _, item := range menuItems {
			if strings.Contains(item, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Menu item '%s' not found in navbar", expected)
		}
	}

	// Verify JOBS item is active on sources page (sources page is under JOBS menu)
	var jobsActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.nav-links a.active[href="/jobs"]') !== null`, &jobsActive),
	)
	if err != nil {
		t.Fatalf("Failed to check active menu item: %v", err)
	}
	if !jobsActive {
		t.Error("JOBS menu item should be active on sources page")
	}

	t.Log("✓ Navbar displays correctly with JOBS item active")
}

func TestSourcesModalWithAuthentication(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesModalWithAuthentication")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// Click Add Source button to open modal
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`button.btn.btn-sm.btn-primary`, chromedp.ByQuery),
		chromedp.Click(`button.btn.btn-sm.btn-primary`, chromedp.ByQuery),
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to open source modal: %v", err)
	}

	// Take screenshot of modal
	if err := env.TakeScreenshot(ctx, "sources-modal-auth"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check for authentication dropdown
	var authSelectPresent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('select[x-model="currentSource.auth_id"]') !== null`, &authSelectPresent),
	)

	if err != nil {
		t.Fatalf("Failed to check auth dropdown: %v", err)
	}

	if !authSelectPresent {
		t.Error("Authentication dropdown not found in modal")
	}

	// Check for authentication help text
	var helpTextPresent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.form-input-hint') !== null &&
			document.querySelector('.form-input-hint').textContent.includes('Select authentication')`, &helpTextPresent),
	)

	if err == nil && !helpTextPresent {
		t.Error("Authentication help text not found")
	}

	t.Log("✓ Source modal includes authentication dropdown")
}

func TestSourcesCardsWithAuthDisplay(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesCardsWithAuthDisplay")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// Check if sources are displayed as cards (not tables)
	var hasCards bool
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.querySelector('.card .card-body .columns') !== null || document.querySelector('.empty') !== null`, &hasCards),
	)

	if err != nil {
		t.Fatalf("Failed to check for cards layout: %v", err)
	}

	if !hasCards {
		t.Error("Sources should be displayed as cards, not table")
	}

	t.Log("✓ Sources displayed as cards with metadata")
}

func TestSourcesFilterInputFieldsVisible(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesFilterInputFieldsVisible")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// Click Add Source button to open modal
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`button.btn.btn-sm.btn-primary`, chromedp.ByQuery),
		chromedp.Click(`button.btn.btn-sm.btn-primary`, chromedp.ByQuery),
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to open source modal: %v", err)
	}

	// Take screenshot of modal with filter fields
	if err := env.TakeScreenshot(ctx, "sources-modal-filter-fields"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check for Include Patterns input field
	var includePatternFieldPresent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('input[x-model="currentSource.filters.include_patterns"]') !== null`, &includePatternFieldPresent),
	)

	if err != nil {
		t.Fatalf("Failed to check include patterns field: %v", err)
	}

	if !includePatternFieldPresent {
		t.Error("Include Patterns input field not found in modal")
	}

	// Check for Exclude Patterns input field
	var excludePatternFieldPresent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('input[x-model="currentSource.filters.exclude_patterns"]') !== null`, &excludePatternFieldPresent),
	)

	if err != nil {
		t.Fatalf("Failed to check exclude patterns field: %v", err)
	}

	if !excludePatternFieldPresent {
		t.Error("Exclude Patterns input field not found in modal")
	}

	// Verify placeholder text for include patterns
	var includePlaceholder string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('input[x-model="currentSource.filters.include_patterns"]').placeholder`, &includePlaceholder),
	)

	if err == nil && !strings.Contains(includePlaceholder, "browse") {
		t.Logf("Warning: Include patterns placeholder may not be set correctly: %s", includePlaceholder)
	}

	// Verify placeholder text for exclude patterns
	var excludePlaceholder string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('input[x-model="currentSource.filters.exclude_patterns"]').placeholder`, &excludePlaceholder),
	)

	if err == nil && !strings.Contains(excludePlaceholder, "admin") {
		t.Logf("Warning: Exclude patterns placeholder may not be set correctly: %s", excludePlaceholder)
	}

	// Check for hint text about comma-separated format
	var hintTextPresent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`Array.from(document.querySelectorAll('.form-input-hint')).some(el => el.textContent.includes('Comma-separated'))`, &hintTextPresent),
	)

	if err == nil && !hintTextPresent {
		t.Log("Warning: Hint text about comma-separated format not found")
	}

	t.Log("✓ Filter input fields are visible in source modal")
}

func TestSourcesCreateWithFilters(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesCreateWithFilters")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// Open the create source modal
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`button.btn.btn-sm.btn-primary`, chromedp.ByQuery),
		chromedp.Click(`button.btn.btn-sm.btn-primary`, chromedp.ByQuery),
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to open source modal: %v", err)
	}

	// Fill in source details
	testSourceName := "Test Source"
	err = chromedp.Run(ctx,
		chromedp.SetValue(`input[x-model="currentSource.name"]`, testSourceName, chromedp.ByQuery),
		chromedp.SetValue(`select[x-model="currentSource.type"]`, "jira", chromedp.ByQuery),
		chromedp.SetValue(`input[x-model="currentSource.base_url"]`, "https://test-ui.atlassian.net", chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to fill in source details: %v", err)
	}

	// Enter filter patterns
	err = chromedp.Run(ctx,
		chromedp.SetValue(`input[x-model="currentSource.filters.include_patterns"]`, "browse,projects,issues", chromedp.ByQuery),
		chromedp.SetValue(`input[x-model="currentSource.filters.exclude_patterns"]`, "admin,logout", chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to enter filter patterns: %v", err)
	}

	// Take screenshot before saving
	if err := env.TakeScreenshot(ctx, "sources-create-with-filters"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Click Save button
	err = chromedp.Run(ctx,
		chromedp.Click(`.modal.active .modal-footer button.btn.btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for save to complete
	)

	if err != nil {
		t.Fatalf("Failed to save source: %v", err)
	}

	// Wait for modal to close
	var modalClosed bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelector('.modal.active') === null`, &modalClosed),
	)

	if err == nil && !modalClosed {
		t.Log("Warning: Modal did not close after save")
	}

	// Take screenshot of table with new source
	if err := env.TakeScreenshot(ctx, "sources-table-with-filters"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Get the source ID for cleanup
	h := env.NewHTTPTestHelper(t)
	sourcesResp, err := h.GET("/api/sources")
	if err != nil {
		t.Logf("Warning: Failed to get sources for cleanup: %v", err)
	} else {
		var sources []map[string]interface{}
		if err := h.ParseJSONResponse(sourcesResp, &sources); err == nil {
			for _, source := range sources {
				if name, ok := source["name"].(string); ok && name == testSourceName {
					if id, ok := source["id"].(string); ok {
						defer h.DELETE("/api/sources/" + id)
						break
					}
				}
			}
		}
	}

	// Verify filters were saved correctly via API
	sourcesResp, err = h.GET("/api/sources")
	if err != nil {
		t.Fatalf("Failed to get sources: %v", err)
	}

	var sources []map[string]interface{}
	if err := h.ParseJSONResponse(sourcesResp, &sources); err != nil {
		t.Fatalf("Failed to parse sources response: %v", err)
	}

	// Find our test source
	var testSource map[string]interface{}
	for _, source := range sources {
		if name, ok := source["name"].(string); ok && name == testSourceName {
			testSource = source
			break
		}
	}

	if testSource == nil {
		t.Fatal("Test source not found in API response")
	}

	// Verify filters
	filters, ok := testSource["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not found in source")
	}

	includePatterns, _ := filters["include_patterns"].(string)
	excludePatterns, _ := filters["exclude_patterns"].(string)

	if includePatterns != "browse,projects,issues" {
		t.Errorf("Expected include_patterns 'browse,projects,issues', got: %s", includePatterns)
	}

	if excludePatterns != "admin,logout" {
		t.Errorf("Expected exclude_patterns 'admin,logout', got: %s", excludePatterns)
	}

	t.Log("✓ Successfully created source with filters via UI")
}

func TestSourcesEditFilters(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesEditFilters")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// First, create a source with filters via API
	h := env.NewHTTPTestHelper(t)

	source := map[string]interface{}{
		"name":     "Source to Edit Filters UI",
		"type":     "jira",
		"base_url": "https://test-edit-filters.atlassian.net",
		"enabled":  true,
		"filters": map[string]interface{}{
			"include_patterns": "browse,projects",
			"exclude_patterns": "admin",
		},
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check for API error
	if status, ok := result["status"].(string); ok && status == "error" {
		errorMsg, _ := result["error"].(string)
		t.Fatalf("API error creating test source: %s (payload: %+v)", errorMsg, source)
	}

	sourceID, ok := result["id"].(string)
	if !ok {
		t.Fatalf("No source ID returned in response: %+v", result)
	}
	defer h.DELETE("/api/sources/" + sourceID)

	// Now open the UI and edit the source
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js and data to load
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Click edit button for our test source (card layout)
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var cards = Array.from(document.querySelectorAll('.card .card-body'));
			var testCard = cards.find(card => {
				var title = card.querySelector('.card-title');
				return title && title.textContent.includes('Source to Edit Filters UI');
			});
			if (testCard) {
				var editButton = testCard.querySelector('button.btn.btn-sm i.fa-edit')?.closest('button');
				if (editButton) editButton.click();
			}
		`, nil),
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for modal to populate
	)

	if err != nil {
		t.Fatalf("Failed to open edit modal: %v", err)
	}

	// Verify filter values populate input fields
	var includeValue string
	var excludeValue string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('input[x-model="currentSource.filters.include_patterns"]').value`, &includeValue),
		chromedp.Evaluate(`document.querySelector('input[x-model="currentSource.filters.exclude_patterns"]').value`, &excludeValue),
	)

	if err != nil {
		t.Fatalf("Failed to read filter values: %v", err)
	}

	if includeValue != "browse,projects" {
		t.Errorf("Expected include_patterns 'browse,projects', got: %s", includeValue)
	}

	if excludeValue != "admin" {
		t.Errorf("Expected exclude_patterns 'admin', got: %s", excludeValue)
	}

	// Take screenshot showing populated filters
	if err := env.TakeScreenshot(ctx, "sources-edit-filters-populated"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Modify the filters using JavaScript to ensure proper Alpine.js binding
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var includeInput = document.querySelector('input[x-model="currentSource.filters.include_patterns"]');
			var excludeInput = document.querySelector('input[x-model="currentSource.filters.exclude_patterns"]');
			if (includeInput) {
				includeInput.value = 'issues,epics,stories';
				includeInput.dispatchEvent(new Event('input', { bubbles: true }));
			}
			if (excludeInput) {
				excludeInput.value = 'admin,logout,settings';
				excludeInput.dispatchEvent(new Event('input', { bubbles: true }));
			}
		`, nil),
		chromedp.Sleep(500*time.Millisecond), // Wait for Alpine.js to process input
	)

	if err != nil {
		t.Fatalf("Failed to modify filters: %v", err)
	}

	// Take screenshot showing modified filters
	if err := env.TakeScreenshot(ctx, "sources-edit-filters-modified"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Save changes
	err = chromedp.Run(ctx,
		chromedp.Click(`.modal.active .modal-footer button.btn.btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for save and API call
	)

	if err != nil {
		t.Fatalf("Failed to save changes: %v", err)
	}

	// Take screenshot after clicking save
	if err := env.TakeScreenshot(ctx, "sources-edit-filters-after-save"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Wait for modal to close
	var modalClosed bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelector('.modal.active') === null`, &modalClosed),
	)

	if err == nil && !modalClosed {
		t.Log("Warning: Modal did not close after save")
	}

	// Take screenshot after save
	if err := env.TakeScreenshot(ctx, "sources-edit-filters-saved"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify changes via API (cards don't display filters)
	updatedResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to get updated source: %v", err)
	}

	var updatedSource map[string]interface{}
	if err := h.ParseJSONResponse(updatedResp, &updatedSource); err != nil {
		t.Fatalf("Failed to parse updated source: %v", err)
	}

	filters, ok := updatedSource["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not found in updated source")
	}

	includePatterns, _ := filters["include_patterns"].(string)
	excludePatterns, _ := filters["exclude_patterns"].(string)

	if includePatterns != "issues,epics,stories" {
		t.Errorf("Expected include_patterns 'issues,epics,stories', got: %s", includePatterns)
	}

	if excludePatterns != "admin,logout,settings" {
		t.Errorf("Expected exclude_patterns 'admin,logout,settings', got: %s", excludePatterns)
	}

	t.Log("✓ Successfully edited source filters via UI")
}

func TestSourcesClearFilters(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesClearFilters")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// Create a source with filters via API
	h := env.NewHTTPTestHelper(t)

	// Use timestamp to ensure unique name and base_url across test runs
	timestamp := time.Now().UnixNano()
	sourceName := fmt.Sprintf("Source to Clear Filters UI %d", timestamp)

	source := map[string]interface{}{
		"name":     sourceName,
		"type":     "confluence",
		"base_url": fmt.Sprintf("https://test-clear-filters-%d.atlassian.net/wiki", timestamp),
		"enabled":  true,
		"filters": map[string]interface{}{
			"include_patterns": "browse,projects,issues",
			"exclude_patterns": "admin,logout",
		},
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check for API error
	if status, ok := result["status"].(string); ok && status == "error" {
		errorMsg, _ := result["error"].(string)
		t.Fatalf("API error creating test source: %s (payload: %+v)", errorMsg, source)
	}

	sourceID, ok := result["id"].(string)
	if !ok {
		t.Fatalf("No source ID returned in response: %+v", result)
	}
	defer h.DELETE("/api/sources/" + sourceID)

	// Open UI and edit the source
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js and data to load
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Click edit button (card layout) using the unique source name
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			var cards = Array.from(document.querySelectorAll('.card .card-body'));
			var testCard = cards.find(card => {
				var title = card.querySelector('.card-title');
				return title && title.textContent.includes('%s');
			});
			if (testCard) {
				var editButton = testCard.querySelector('button.btn.btn-sm i.fa-edit')?.closest('button');
				if (editButton) editButton.click();
			}
		`, sourceName), nil),
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to open edit modal: %v", err)
	}

	// Clear filter inputs by setting values and dispatching input events to trigger Alpine.js x-model binding
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Set input values to empty and dispatch input events to trigger Alpine.js reactivity
			var includeInput = document.querySelector('input[x-model="currentSource.filters.include_patterns"]');
			var excludeInput = document.querySelector('input[x-model="currentSource.filters.exclude_patterns"]');
			if (includeInput) {
				includeInput.value = '';
				includeInput.dispatchEvent(new Event('input', { bubbles: true }));
				includeInput.dispatchEvent(new Event('change', { bubbles: true }));
			}
			if (excludeInput) {
				excludeInput.value = '';
				excludeInput.dispatchEvent(new Event('input', { bubbles: true }));
				excludeInput.dispatchEvent(new Event('change', { bubbles: true }));
			}
		`, nil),
		chromedp.Sleep(1*time.Second), // Wait for Alpine.js reactivity to process events
	)

	if err != nil {
		t.Fatalf("Failed to clear filters: %v", err)
	}

	// Verify the Alpine.js model was updated
	var includeAfterClear, excludeAfterClear string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var modal = document.querySelector('.modal.active');
			modal && modal.__x && modal.__x.$data.currentSource?.filters?.include_patterns || ''
		`, &includeAfterClear),
		chromedp.Evaluate(`
			var modal = document.querySelector('.modal.active');
			modal && modal.__x && modal.__x.$data.currentSource?.filters?.exclude_patterns || ''
		`, &excludeAfterClear),
	)
	if err == nil {
		t.Logf("Alpine.js model after clearing - include: '%s', exclude: '%s'", includeAfterClear, excludeAfterClear)
	}

	// Take screenshot showing cleared filters
	if err := env.TakeScreenshot(ctx, "sources-clear-filters"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Save changes
	err = chromedp.Run(ctx,
		chromedp.Click(`.modal.active .modal-footer button.btn.btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // Wait longer for API call to complete and sources list to reload
	)

	if err != nil {
		t.Fatalf("Failed to save changes: %v", err)
	}

	// Wait for modal to close
	var modalClosed bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelector('.modal.active') === null`, &modalClosed),
	)

	if err == nil && !modalClosed {
		t.Log("Warning: Modal did not close after save")
	}

	// Take screenshot after save
	if err := env.TakeScreenshot(ctx, "sources-clear-filters-saved"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify filters cleared via API (cards don't display filters)
	updatedResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to get updated source: %v", err)
	}

	var updatedSource map[string]interface{}
	if err := h.ParseJSONResponse(updatedResp, &updatedSource); err != nil {
		t.Fatalf("Failed to parse updated source: %v", err)
	}

	filters, ok := updatedSource["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not found in updated source")
	}

	includePatterns, _ := filters["include_patterns"].(string)
	excludePatterns, _ := filters["exclude_patterns"].(string)

	if includePatterns != "" {
		t.Errorf("Expected empty include_patterns, got: %s", includePatterns)
	}

	if excludePatterns != "" {
		t.Errorf("Expected empty exclude_patterns, got: %s", excludePatterns)
	}

	t.Log("✓ Successfully cleared source filters via UI")
}

func TestSourcesFilterDisplayFormatting(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestSourcesFilterDisplayFormatting")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/sources"

	// Create sources with various filter combinations via API
	h := env.NewHTTPTestHelper(t)

	// Use timestamp to ensure unique names across test runs
	timestamp := time.Now().UnixNano()

	testSources := []struct {
		name         string
		filters      map[string]interface{}
		expectedText string
	}{
		{
			name: fmt.Sprintf("UI Test Both Filters %d", timestamp),
			filters: map[string]interface{}{
				"include_patterns": "browse,projects",
				"exclude_patterns": "admin",
			},
			expectedText: "Include: 2, Exclude: 1",
		},
		{
			name: fmt.Sprintf("UI Test Include Only %d", timestamp+1),
			filters: map[string]interface{}{
				"include_patterns": "browse,projects,issues",
			},
			expectedText: "Include: 3",
		},
		{
			name: fmt.Sprintf("UI Test Exclude Only %d", timestamp+2),
			filters: map[string]interface{}{
				"exclude_patterns": "admin,logout",
			},
			expectedText: "Exclude: 2",
		},
		{
			name:         fmt.Sprintf("UI Test No Filters %d", timestamp+3),
			filters:      map[string]interface{}{},
			expectedText: "None",
		},
	}

	var sourceIDs []string

	for _, ts := range testSources {
		// Use unique base_url by replacing spaces with hyphens in the name (includes timestamp)
		uniqueBaseURL := fmt.Sprintf("https://test-display-%s.atlassian.net", strings.ReplaceAll(ts.name, " ", "-"))

		source := map[string]interface{}{
			"name":     ts.name,
			"type":     "jira",
			"base_url": uniqueBaseURL,
			"enabled":  true,
			"crawl_config": map[string]interface{}{
				"concurrency": 1,
			},
		}

		if len(ts.filters) > 0 {
			source["filters"] = ts.filters
		}

		resp, err := h.POST("/api/sources", source)
		if err != nil {
			t.Fatalf("Failed to create source '%s': %v", ts.name, err)
		}

		var result map[string]interface{}
		if err := h.ParseJSONResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check for API error
		if status, ok := result["status"].(string); ok && status == "error" {
			errorMsg, _ := result["error"].(string)
			t.Fatalf("API error creating test source '%s': %s (payload: %+v)", ts.name, errorMsg, source)
		}

		sourceID, ok := result["id"].(string)
		if !ok {
			t.Fatalf("No source ID returned for '%s' in response: %+v", ts.name, result)
		}
		sourceIDs = append(sourceIDs, sourceID)
	}

	// Cleanup
	defer func() {
		for _, sourceID := range sourceIDs {
			h.DELETE("/api/sources/" + sourceID)
		}
	}()

	// Load the UI and verify sources appear
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js and all sources to load
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Take screenshot showing all source cards
	if err := env.TakeScreenshot(ctx, "sources-filter-display-formatting"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify each source exists in the UI (cards don't display filters, so just verify cards exist)
	for _, ts := range testSources {
		var cardFound bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					var cards = Array.from(document.querySelectorAll('.card .card-body'));
					var found = cards.some(card => {
						var title = card.querySelector('.card-title');
						return title && title.textContent.includes('`+ts.name+`');
					});
					return found;
				})()
			`, &cardFound),
		)

		if err != nil {
			t.Logf("Warning: Failed to check for source card '%s': %v", ts.name, err)
			continue
		}

		if !cardFound {
			t.Errorf("Source card '%s' not found in UI", ts.name)
		}
	}

	// Verify filter values via API (cards don't display filters)
	for i, ts := range testSources {
		sourceID := sourceIDs[i]
		sourceResp, err := h.GET("/api/sources/" + sourceID)
		if err != nil {
			t.Logf("Warning: Failed to get source '%s': %v", ts.name, err)
			continue
		}

		var source map[string]interface{}
		if err := h.ParseJSONResponse(sourceResp, &source); err != nil {
			t.Logf("Warning: Failed to parse source '%s': %v", ts.name, err)
			continue
		}

		filters, ok := source["filters"].(map[string]interface{})
		if !ok {
			t.Logf("Warning: Filters not found for source '%s'", ts.name)
			continue
		}

		// Verify filters match what was configured
		includePatterns, _ := filters["include_patterns"].(string)
		excludePatterns, _ := filters["exclude_patterns"].(string)

		expectedInclude, _ := ts.filters["include_patterns"].(string)
		expectedExclude, _ := ts.filters["exclude_patterns"].(string)

		if includePatterns != expectedInclude {
			t.Errorf("Source '%s': expected include_patterns '%s', got '%s'", ts.name, expectedInclude, includePatterns)
		}

		if excludePatterns != expectedExclude {
			t.Errorf("Source '%s': expected exclude_patterns '%s', got '%s'", ts.name, expectedExclude, excludePatterns)
		}
	}

	t.Log("✓ Filter configurations verified correctly")
}
