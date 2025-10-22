package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
)

func TestSourcesPageLoad(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"
	var title string

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "sources-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Source Management - Quaero"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}

	t.Log("✓ Sources page loads correctly")
}

func TestSourcesPageElements(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

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
			err := chromedp.Run(ctx,
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
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	var navbarVisible bool
	var menuItems []string

	err := chromedp.Run(ctx,
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
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Click Add Source button to open modal
	err := chromedp.Run(ctx,
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
	if err := TakeScreenshot(ctx, "sources-modal-auth"); err != nil {
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

func TestSourcesTableWithAuthColumn(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Check if the table has authentication column
	var hasAuthColumn bool
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`table.table`, chromedp.ByQuery),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('th')).some(th => th.textContent.includes('AUTHENTICATION'))`, &hasAuthColumn),
	)

	if err != nil {
		t.Fatalf("Failed to check table headers: %v", err)
	}

	if !hasAuthColumn {
		t.Error("Authentication column not found in sources table")
	}

	// Check column count (should be 7 with authentication)
	var columnCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('thead tr th').length`, &columnCount),
	)

	if err == nil && columnCount != 7 {
		t.Errorf("Expected 7 columns in table, got %d", columnCount)
	}

	t.Log("✓ Sources table includes authentication column")
}

func TestSourcesFilterInputFieldsVisible(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Click Add Source button to open modal
	err := chromedp.Run(ctx,
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
	if err := TakeScreenshot(ctx, "sources-modal-filter-fields"); err != nil {
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
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Open the create source modal
	err := chromedp.Run(ctx,
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
	testSourceName := "Test Source with Filters UI"
	err = chromedp.Run(ctx,
		chromedp.SetValue(`input[x-model="currentSource.name"]`, testSourceName, chromedp.ByQuery),
		chromedp.SetValue(`select[x-model="currentSource.type"]`, "jira", chromedp.ByQuery),
		chromedp.SetValue(`input[x-model="currentSource.base_url"]`, "https://test-filters-ui.atlassian.net", chromedp.ByQuery),
		chromedp.SetValue(`input[x-model="currentSource.auth_domain"]`, "test-filters-ui.atlassian.net", chromedp.ByQuery),
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
	if err := TakeScreenshot(ctx, "sources-create-with-filters"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Click Save button
	err = chromedp.Run(ctx,
		chromedp.Click(`button.btn.btn-primary[x-on\\:click="saveSource()"]`, chromedp.ByQuery),
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
	if err := TakeScreenshot(ctx, "sources-table-with-filters"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify filter display in table (should show "Include: 3, Exclude: 2")
	var filterDisplayText string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var rows = Array.from(document.querySelectorAll('tbody tr'));
			var testRow = rows.find(row => row.textContent.includes('`+testSourceName+`'));
			if (testRow) {
				var cells = Array.from(testRow.querySelectorAll('td'));
				// Find the FILTERS column (should be one of the cells)
				for (var cell of cells) {
					if (cell.textContent.includes('Include:') || cell.textContent.includes('Exclude:')) {
						return cell.textContent.trim();
					}
				}
			}
			return '';
		`, &filterDisplayText),
	)

	if err != nil {
		t.Fatalf("Failed to check filter display: %v", err)
	}

	if !strings.Contains(filterDisplayText, "Include: 3") {
		t.Errorf("Expected filter display to contain 'Include: 3', got: %s", filterDisplayText)
	}

	if !strings.Contains(filterDisplayText, "Exclude: 2") {
		t.Errorf("Expected filter display to contain 'Exclude: 2', got: %s", filterDisplayText)
	}

	t.Log("✓ Successfully created source with filters via UI")
}

func TestSourcesEditFilters(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// First, create a source with filters via API
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	source := map[string]interface{}{
		"name":        "Source to Edit Filters UI",
		"type":        "jira",
		"base_url":    "https://test-edit-filters.atlassian.net",
		"auth_domain": "test-edit-filters.atlassian.net",
		"enabled":     true,
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

	sourceID := result["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// Now open the UI and edit the source
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`tbody`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for data to load
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Click edit button for our test source
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var rows = Array.from(document.querySelectorAll('tbody tr'));
			var testRow = rows.find(row => row.textContent.includes('Source to Edit Filters UI'));
			if (testRow) {
				var editButton = testRow.querySelector('button.btn.btn-sm i.fa-edit')?.closest('button');
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
	if err := TakeScreenshot(ctx, "sources-edit-filters-populated"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Modify the filters
	err = chromedp.Run(ctx,
		chromedp.SetValue(`input[x-model="currentSource.filters.include_patterns"]`, "issues,epics,stories", chromedp.ByQuery),
		chromedp.SetValue(`input[x-model="currentSource.filters.exclude_patterns"]`, "admin,logout,settings", chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to modify filters: %v", err)
	}

	// Take screenshot showing modified filters
	if err := TakeScreenshot(ctx, "sources-edit-filters-modified"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Save changes
	err = chromedp.Run(ctx,
		chromedp.Click(`button.btn.btn-primary[x-on\\:click="saveSource()"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for save
	)

	if err != nil {
		t.Fatalf("Failed to save changes: %v", err)
	}

	// Verify changes reflected in table
	var filterDisplayText string
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			var rows = Array.from(document.querySelectorAll('tbody tr'));
			var testRow = rows.find(row => row.textContent.includes('Source to Edit Filters UI'));
			if (testRow) {
				var cells = Array.from(testRow.querySelectorAll('td'));
				for (var cell of cells) {
					if (cell.textContent.includes('Include:') || cell.textContent.includes('Exclude:')) {
						return cell.textContent.trim();
					}
				}
			}
			return '';
		`, &filterDisplayText),
	)

	if err != nil {
		t.Fatalf("Failed to check updated filter display: %v", err)
	}

	if !strings.Contains(filterDisplayText, "Include: 3") {
		t.Errorf("Expected updated filter display to contain 'Include: 3', got: %s", filterDisplayText)
	}

	if !strings.Contains(filterDisplayText, "Exclude: 3") {
		t.Errorf("Expected updated filter display to contain 'Exclude: 3', got: %s", filterDisplayText)
	}

	t.Log("✓ Successfully edited source filters via UI")
}

func TestSourcesClearFilters(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Create a source with filters via API
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	source := map[string]interface{}{
		"name":        "Source to Clear Filters UI",
		"type":        "confluence",
		"base_url":    "https://test-clear-filters.atlassian.net/wiki",
		"auth_domain": "test-clear-filters.atlassian.net",
		"enabled":     true,
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

	sourceID := result["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// Open UI and edit the source
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`tbody`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Click edit button
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var rows = Array.from(document.querySelectorAll('tbody tr'));
			var testRow = rows.find(row => row.textContent.includes('Source to Clear Filters UI'));
			if (testRow) {
				var editButton = testRow.querySelector('button.btn.btn-sm i.fa-edit')?.closest('button');
				if (editButton) editButton.click();
			}
		`, nil),
		chromedp.WaitVisible(`.modal.is-active`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to open edit modal: %v", err)
	}

	// Clear filter inputs
	err = chromedp.Run(ctx,
		chromedp.SetValue(`input[x-model="currentSource.filters.include_patterns"]`, "", chromedp.ByQuery),
		chromedp.SetValue(`input[x-model="currentSource.filters.exclude_patterns"]`, "", chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to clear filters: %v", err)
	}

	// Take screenshot showing cleared filters
	if err := TakeScreenshot(ctx, "sources-clear-filters"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Save changes
	err = chromedp.Run(ctx,
		chromedp.Click(`button.btn.btn-primary[x-on\\:click="saveSource()"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to save changes: %v", err)
	}

	// Verify table shows "None" for filters
	var filterDisplayText string
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			var rows = Array.from(document.querySelectorAll('tbody tr'));
			var testRow = rows.find(row => row.textContent.includes('Source to Clear Filters UI'));
			if (testRow) {
				var cells = Array.from(testRow.querySelectorAll('td'));
				for (var cell of cells) {
					var text = cell.textContent.trim();
					if (text === 'None' || text.includes('Include:') || text.includes('Exclude:')) {
						return text;
					}
				}
			}
			return '';
		`, &filterDisplayText),
	)

	if err != nil {
		t.Fatalf("Failed to check filter display: %v", err)
	}

	if filterDisplayText != "None" {
		t.Logf("Warning: Expected 'None' for cleared filters, got: %s", filterDisplayText)
	}

	t.Log("✓ Successfully cleared source filters via UI")
}

func TestSourcesFilterDisplayFormatting(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Create sources with various filter combinations via API
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	testSources := []struct {
		name         string
		filters      map[string]interface{}
		expectedText string
	}{
		{
			name: "UI Test Both Filters",
			filters: map[string]interface{}{
				"include_patterns": "browse,projects",
				"exclude_patterns": "admin",
			},
			expectedText: "Include: 2, Exclude: 1",
		},
		{
			name: "UI Test Include Only",
			filters: map[string]interface{}{
				"include_patterns": "browse,projects,issues",
			},
			expectedText: "Include: 3",
		},
		{
			name: "UI Test Exclude Only",
			filters: map[string]interface{}{
				"exclude_patterns": "admin,logout",
			},
			expectedText: "Exclude: 2",
		},
		{
			name:         "UI Test No Filters",
			filters:      map[string]interface{}{},
			expectedText: "None",
		},
	}

	var sourceIDs []string

	for _, ts := range testSources {
		source := map[string]interface{}{
			"name":        ts.name,
			"type":        "jira",
			"base_url":    "https://test-display-" + strings.ReplaceAll(ts.name, " ", "-") + ".atlassian.net",
			"auth_domain": "test-display.atlassian.net",
			"enabled":     true,
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

		sourceIDs = append(sourceIDs, result["id"].(string))
	}

	// Cleanup
	defer func() {
		for _, sourceID := range sourceIDs {
			h.DELETE("/api/sources/" + sourceID)
		}
	}()

	// Load the UI and verify filter display
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`tbody`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for all sources to load
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Take screenshot of table with various filter displays
	if err := TakeScreenshot(ctx, "sources-filter-display-formatting"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify each source's filter display
	for _, ts := range testSources {
		var filterDisplayText string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				var rows = Array.from(document.querySelectorAll('tbody tr'));
				var testRow = rows.find(row => row.textContent.includes('`+ts.name+`'));
				if (testRow) {
					var cells = Array.from(testRow.querySelectorAll('td'));
					for (var cell of cells) {
						var text = cell.textContent.trim();
						if (text === 'None' || text.includes('Include:') || text.includes('Exclude:')) {
							return text;
						}
					}
				}
				return 'NOT FOUND';
			`, &filterDisplayText),
		)

		if err != nil {
			t.Logf("Warning: Failed to check filter display for '%s': %v", ts.name, err)
			continue
		}

		// Normalize whitespace for comparison
		normalizedDisplay := strings.Join(strings.Fields(filterDisplayText), " ")
		normalizedExpected := strings.Join(strings.Fields(ts.expectedText), " ")

		if normalizedDisplay != normalizedExpected {
			t.Errorf("Source '%s': expected filter display '%s', got: '%s'", ts.name, ts.expectedText, filterDisplayText)
		}
	}

	t.Log("✓ Filter display formatting is correct")
}
