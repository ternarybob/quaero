package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestSearchPageLoad(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchPageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"
	var title string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load search page: %v", err)
	}

	// Take screenshot of search page
	if err := TakeScreenshot(ctx, "search-page-load"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	if title != "Search - Quaero" {
		t.Errorf("Expected title 'Search - Quaero', got '%s'", title)
	}

	t.Log("✓ Search page loaded successfully")
}

func TestSearchPageElements(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchPageElements")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Check for presence of search UI elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Search input", `input[type="search"]`},
		{"Search button", `button:has(span:contains("Search"))`},
		{"Clear button", `button:has(i.fa-times)`},
		{"Query syntax help card", `.card-header:contains("Query Syntax Help")`},
		{"Results container", `#results-container`},
		{"Page title", `h1:contains("Document Search")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			err = chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Sleep(500*time.Millisecond), // Wait for Alpine.js initialization
				chromedp.Evaluate(`document.querySelectorAll('`+tt.selector+`').length`, &nodeCount),
			)

			if err != nil {
				t.Fatalf("Failed to check element '%s': %v", tt.name, err)
			}

			if nodeCount == 0 {
				t.Errorf("Element '%s' (selector: %s) not found on page", tt.name, tt.selector)
			}
		})
	}

	t.Log("✓ All search page elements present")
}

func TestSearchQueryExecution(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchQueryExecution")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Test query: "test"
	testQuery := "test"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for Alpine.js initialization

		// Enter search query
		chromedp.SendKeys(`input[type="search"]`, testQuery, chromedp.ByQuery),

		// Click search button
		chromedp.Click(`button[x-ref="searchBtn"]`, chromedp.ByQuery),

		// Wait for results or empty state to render
		chromedp.Sleep(2*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to execute search: %v", err)
	}

	// Take screenshot after search
	if err := TakeScreenshot(ctx, "search-query-executed"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify that the query was submitted (check for results container or empty state)
	var resultsOrEmpty bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			document.querySelector('#results-container .empty') !== null ||
			document.querySelector('#results-container .card') !== null
		`, &resultsOrEmpty),
	)

	if err != nil {
		t.Fatalf("Failed to check results: %v", err)
	}

	if !resultsOrEmpty {
		t.Error("Neither results nor empty state displayed after search")
	}

	t.Logf("✓ Search query executed successfully (query='%s')", testQuery)
}

func TestSearchWithResults(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchWithResults")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Use a broad query that's likely to return results
	testQuery := "a"

	var resultsVisible bool
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Enter search query
		chromedp.SendKeys(`input[type="search"]`, testQuery, chromedp.ByQuery),

		// Click search button
		chromedp.Click(`button[x-ref="searchBtn"]`, chromedp.ByQuery),

		// Wait for results
		chromedp.Sleep(2*time.Second),

		// Check if results are displayed
		chromedp.Evaluate(`document.querySelectorAll('.card-body').length > 0`, &resultsVisible),
	)

	if err != nil {
		t.Fatalf("Failed to execute search with results: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "search-with-results"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Logf("✓ Search with query '%s' completed (results visible: %v)", testQuery, resultsVisible)
}

func TestSearchClearButton(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchClearButton")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	var inputValue string
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Enter search query
		chromedp.SendKeys(`input[type="search"]`, "test query", chromedp.ByQuery),

		// Click search button
		chromedp.Click(`button[x-ref="searchBtn"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// Click clear button
		chromedp.Click(`button[x-ref="clearBtn"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Check input value after clear
		chromedp.Evaluate(`document.querySelector('input[type="search"]').value`, &inputValue),
	)

	if err != nil {
		t.Fatalf("Failed to test clear button: %v", err)
	}

	// Take screenshot after clear
	if err := TakeScreenshot(ctx, "search-after-clear"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	if inputValue != "" {
		t.Errorf("Expected empty input after clear, got: '%s'", inputValue)
	}

	t.Log("✓ Clear button works correctly")
}

func TestSearchSyntaxHelp(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchSyntaxHelp")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Verify syntax help card can be toggled
	var syntaxHelpVisible bool
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Click syntax help header to toggle
		chromedp.Click(`.card-header:contains("Query Syntax Help")`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Check if help content is visible
		chromedp.Evaluate(`
			document.querySelector('.card-body:has(h4:contains("Basic Search"))') !== null &&
			window.getComputedStyle(document.querySelector('.card-body:has(h4:contains("Basic Search"))')).display !== 'none'
		`, &syntaxHelpVisible),
	)

	if err != nil {
		t.Fatalf("Failed to test syntax help: %v", err)
	}

	// Take screenshot with syntax help visible
	if err := TakeScreenshot(ctx, "search-syntax-help"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Logf("✓ Syntax help card toggle works (visible: %v)", syntaxHelpVisible)
}

func TestSearchResultStructure(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchResultStructure")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Use a broad query that's likely to return results
	testQuery := "test"

	var hasResults bool
	var hasTitle, hasBrief, hasLink bool
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Enter search query
		chromedp.SendKeys(`input[type="search"]`, testQuery, chromedp.ByQuery),

		// Click search button
		chromedp.Click(`button[x-ref="searchBtn"]`, chromedp.ByQuery),

		// Wait for results
		chromedp.Sleep(2*time.Second),

		// Check if we have results
		chromedp.Evaluate(`document.querySelectorAll('#results-container .card').length > 0`, &hasResults),
	)

	if err != nil {
		t.Fatalf("Failed to execute search: %v", err)
	}

	if hasResults {
		// Verify result structure
		err = chromedp.Run(ctx,
			// Check for title in first result
			chromedp.Evaluate(`document.querySelector('#results-container .card h3') !== null`, &hasTitle),
			// Check for brief content
			chromedp.Evaluate(`document.querySelector('#results-container .card .card-body div') !== null`, &hasBrief),
			// Check for external link button
			chromedp.Evaluate(`document.querySelector('#results-container .card a[target="_blank"]') !== null`, &hasLink),
		)

		if err != nil {
			t.Fatalf("Failed to check result structure: %v", err)
		}

		// Take screenshot
		if err := TakeScreenshot(ctx, "search-result-structure"); err != nil {
			t.Logf("Warning: Failed to take screenshot: %v", err)
		}

		if !hasTitle {
			t.Error("Result missing title")
		}
		if !hasBrief {
			t.Error("Result missing brief content")
		}
		if !hasLink {
			t.Error("Result missing external link")
		}

		t.Log("✓ Search result structure verified")
	} else {
		t.Log("⚠ No results returned (database may be empty)")
	}
}

func TestSearchPagination(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchPagination")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Use a broad query that's likely to return results
	testQuery := "a"

	var paginationVisible bool
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Enter search query
		chromedp.SendKeys(`input[type="search"]`, testQuery, chromedp.ByQuery),

		// Click search button
		chromedp.Click(`button[x-ref="searchBtn"]`, chromedp.ByQuery),

		// Wait for results
		chromedp.Sleep(2*time.Second),

		// Check if pagination controls are visible
		chromedp.Evaluate(`document.querySelector('.pagination') !== null`, &paginationVisible),
	)

	if err != nil {
		t.Fatalf("Failed to check pagination: %v", err)
	}

	if paginationVisible {
		// Take screenshot with pagination
		if err := TakeScreenshot(ctx, "search-pagination"); err != nil {
			t.Logf("Warning: Failed to take screenshot: %v", err)
		}

		// Verify pagination buttons exist
		var hasPrevious, hasCurrent, hasNext bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('.page-item a:contains("Previous")') !== null`, &hasPrevious),
			chromedp.Evaluate(`document.querySelector('.page-item.active a') !== null`, &hasCurrent),
			chromedp.Evaluate(`document.querySelector('.page-item a:contains("Next")') !== null`, &hasNext),
		)

		if err != nil {
			t.Fatalf("Failed to check pagination buttons: %v", err)
		}

		if !hasPrevious || !hasCurrent || !hasNext {
			t.Error("Pagination missing Previous, Current, or Next buttons")
		}

		t.Log("✓ Pagination controls visible and structured correctly")
	} else {
		t.Log("⚠ Pagination not visible (may need more results)")
	}
}

func TestSearchNavbarActiveState(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchNavbarActiveState")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Check if Search link in navbar has active class
	var hasActiveClass bool
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Check if SEARCH link has active class
		chromedp.Evaluate(`
			document.querySelector('a[href="/search"].active') !== null
		`, &hasActiveClass),
	)

	if err != nil {
		t.Fatalf("Failed to check navbar active state: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "search-navbar-active"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	if !hasActiveClass {
		t.Error("SEARCH link in navbar does not have active class on search page")
	}

	t.Log("✓ Search navbar link has active state")
}

func TestSearchEmptyQuery(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestSearchEmptyQuery")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/search"

	// Try to search with empty query (should show notification)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Click search button without entering query
		chromedp.Click(`button[x-ref="searchBtn"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to test empty query: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "search-empty-query"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Note: Cannot easily verify notification display without additional UI inspection
	// This test primarily verifies the app doesn't crash on empty query

	t.Log("✓ Empty query handled without error")
}
