// -----------------------------------------------------------------------
// Last Modified: Tuesday, 4th November 2025 8:53:50 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

func TestAuthPageLoad(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("AuthPageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/auth"
	var title string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for page to fully load
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load auth page: %v", err)
	}

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "auth-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Job Management - Quaero"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s' - routing issue: /auth should serve jobs.html", expectedTitle, title)
	}

	// Verify we're on the jobs page with auth section by checking for "Job Management" heading
	var hasJobManagement bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.body.textContent.includes('Job Management')`, &hasJobManagement),
	)
	if err != nil {
		t.Fatalf("Failed to check page content: %v", err)
	}

	if !hasJobManagement {
		t.Error("Page does not contain 'Job Management' - wrong page loaded (check routes.go)")
	}

	// Verify auth section exists
	var hasAuthSection bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.body.textContent.includes('Authentication')`, &hasAuthSection),
	)
	if err != nil {
		t.Fatalf("Failed to check auth section: %v", err)
	}

	if !hasAuthSection {
		t.Error("Page does not contain 'Authentication' section")
	}

	t.Log("✓ Auth page (jobs.html with auth section) loads correctly")
}

func TestAuthPageElements(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("AuthPageElements")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/auth"

	// Check for presence of key elements on jobs.html (auth section)
	tests := []struct {
		name     string
		selector string
	}{
		{"Page title", ".page-title"},
		{"Authentication card", ".card"},
		{"Refresh button", "button[title='Refresh Authentications']"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			// Use double quotes for JavaScript string to avoid escaping issues with selectors containing single quotes
			evalJS := fmt.Sprintf(`document.querySelectorAll("%s").length`, strings.ReplaceAll(tt.selector, `"`, `\"`))
			err := chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Evaluate(evalJS, &nodeCount),
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

func TestAuthNavbar(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("AuthNavbar")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/auth"

	var navbarVisible bool
	var menuItems []string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Brief wait for page to settle
		chromedp.Evaluate(`document.querySelector('nav') !== null`, &navbarVisible),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('nav a')).map(el => el.textContent.trim())`, &menuItems),
	)

	if err != nil {
		t.Fatalf("Failed to check navbar: %v", err)
	}

	if !navbarVisible {
		t.Error("Navbar not found on page")
	}

	// Check for expected menu items (auth page is under JOBS menu)
	expectedItems := []string{"HOME", "JOBS", "QUEUE", "DOCUMENTS", "SEARCH", "CHAT", "SETTINGS"}
	for _, expected := range expectedItems {
		found := false
		for _, item := range menuItems {
			// Case insensitive comparison and allow partial matches
			if strings.Contains(strings.ToUpper(item), expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Menu item '%s' not found in navbar. Found items: %v", expected, menuItems)
		}
	}

	// Verify JOBS item is active on auth page (auth is grouped under jobs menu)
	var jobsActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('nav a[href="/jobs"].active') !== null`, &jobsActive),
	)
	if err != nil {
		t.Fatalf("Failed to check active menu item: %v", err)
	}
	if !jobsActive {
		t.Logf("Warning: JOBS menu item may not be marked as active on auth page (this is non-critical)")
	}

	t.Log("✓ Navbar displays correctly with expected menu items")
}

func TestAuthCookieInjection(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("AuthCookieInjection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create test authentication data
	authData := map[string]interface{}{
		"baseUrl":   "https://test.atlassian.net",
		"userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0",
		"timestamp": time.Now().Unix(),
		"cookies": []map[string]interface{}{
			{
				"name":     "cloud.session.token",
				"value":    "test-session-token-12345",
				"domain":   ".atlassian.net",
				"path":     "/",
				"expires":  time.Now().Add(24 * time.Hour).Unix(),
				"secure":   true,
				"httpOnly": true,
				"sameSite": "None",
			},
			{
				"name":     "JSESSIONID",
				"value":    "test-jsessionid-67890",
				"domain":   ".atlassian.net",
				"path":     "/",
				"expires":  time.Now().Add(24 * time.Hour).Unix(),
				"secure":   true,
				"httpOnly": true,
				"sameSite": "Lax",
			},
			{
				"name":     "atl.xsrf.token",
				"value":    "test-xsrf-token-abcdef",
				"domain":   ".atlassian.net",
				"path":     "/",
				"expires":  time.Now().Add(24 * time.Hour).Unix(),
				"secure":   true,
				"httpOnly": false,
				"sameSite": "Lax",
			},
		},
		"tokens": map[string]string{
			"cloudId":  "test-cloud-id-xyz123",
			"atlToken": "test-atl-token-abc456",
		},
	}

	// Post auth data to server (simulating Chrome extension)
	err = postAuthData(env.GetBaseURL(), authData)
	if err != nil {
		t.Fatalf("Failed to post auth data: %v", err)
	}

	t.Log("✓ Test cookies posted to server")

	// Navigate to auth page and verify
	url := env.GetBaseURL() + "/auth"
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for any async updates
	)

	if err != nil {
		t.Fatalf("Failed to load auth page: %v", err)
	}

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "auth-page-with-cookies"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify auth status via API
	authStatus, err := getAuthStatus(env.GetBaseURL())
	if err != nil {
		t.Fatalf("Failed to get auth status: %v", err)
	}

	if !authStatus {
		t.Error("Expected authenticated status to be true after posting test cookies")
	}

	t.Log("✓ Authentication cookies injected and verified successfully")
}

// Helper function to post authentication data to the server
func postAuthData(baseURL string, authData map[string]interface{}) error {
	jsonData, err := json.Marshal(authData)
	if err != nil {
		return fmt.Errorf("failed to marshal auth data: %w", err)
	}

	resp, err := http.Post(baseURL+"/api/auth", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to post auth data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if status, ok := result["status"].(string); !ok || status != "success" {
		return fmt.Errorf("expected success status, got %v", result["status"])
	}

	return nil
}

// Helper function to get authentication status from the server
func getAuthStatus(baseURL string) (bool, error) {
	resp, err := http.Get(baseURL + "/api/auth/status")
	if err != nil {
		return false, fmt.Errorf("failed to get auth status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	authenticated, ok := status["authenticated"].(bool)
	if !ok {
		return false, fmt.Errorf("response missing or invalid 'authenticated' field")
	}

	return authenticated, nil
}
