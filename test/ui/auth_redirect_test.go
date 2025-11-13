// -----------------------------------------------------------------------
// Auth Redirect Tests
// Verifies /auth redirect behavior including trailing slash and query preservation
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestAuthRedirectBasic verifies /auth redirects to settings with auth accordions
func TestAuthRedirectBasic(t *testing.T) {
	env, err := common.SetupTestEnvironment("AuthRedirectBasic")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test /auth redirect
	resp, err := client.Get(env.GetBaseURL() + "/auth")
	if err != nil {
		t.Fatalf("Failed to request /auth: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect (3xx status)
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		t.Errorf("Expected 3xx redirect status, got %d", resp.StatusCode)
	}

	// Should be 308 Permanent Redirect
	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Errorf("Expected 308 Permanent Redirect, got %d", resp.StatusCode)
	}

	// Check Location header
	location := resp.Header.Get("Location")
	if location != "/settings?a=auth-apikeys%2Cauth-cookies" && location != "/settings?a=auth-apikeys,auth-cookies" {
		t.Errorf("Expected redirect to /settings?a=auth-apikeys,auth-cookies, got %s", location)
	}

	t.Log("✓ /auth redirects correctly with 308 status")
}

// TestAuthRedirectTrailingSlash verifies /auth/ (with trailing slash) redirects correctly
func TestAuthRedirectTrailingSlash(t *testing.T) {
	env, err := common.SetupTestEnvironment("AuthRedirectTrailingSlash")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Test /auth/ redirect (with trailing slash)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	resp, err := client.Get(env.GetBaseURL() + "/auth/")
	if err != nil {
		t.Fatalf("Failed to request /auth/: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect (3xx status)
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		t.Errorf("Expected 3xx redirect status for /auth/, got %d", resp.StatusCode)
	}

	// Should be 308 Permanent Redirect
	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Errorf("Expected 308 Permanent Redirect for /auth/, got %d", resp.StatusCode)
	}

	// Check Location header
	location := resp.Header.Get("Location")
	if location != "/settings?a=auth-apikeys%2Cauth-cookies" && location != "/settings?a=auth-apikeys,auth-cookies" {
		t.Errorf("Expected redirect to /settings?a=auth-apikeys,auth-cookies, got %s", location)
	}

	t.Log("✓ /auth/ (trailing slash) redirects correctly with 308 status")
}

// TestAuthRedirectQueryPreservation verifies existing query parameters are preserved
func TestAuthRedirectQueryPreservation(t *testing.T) {
	env, err := common.SetupTestEnvironment("AuthRedirectQueryPreservation")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Test /auth?foo=bar redirect
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	resp, err := client.Get(env.GetBaseURL() + "/auth?foo=bar")
	if err != nil {
		t.Fatalf("Failed to request /auth?foo=bar: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect
	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Errorf("Expected 308 Permanent Redirect, got %d", resp.StatusCode)
	}

	// Check Location header contains both foo=bar and a=auth-apikeys,auth-cookies
	location := resp.Header.Get("Location")

	// Should contain foo=bar
	if location != "" {
		// Parse as query string to verify both parameters
		if !(strings.Contains(location, "foo=bar") && (strings.Contains(location, "a=auth-apikeys") || strings.Contains(location, "a=auth-apikeys%2Cauth-cookies"))) {
			t.Errorf("Expected redirect to preserve foo=bar and add a=auth-apikeys,auth-cookies, got %s", location)
		}
	} else {
		t.Error("Location header is empty")
	}

	t.Log("✓ Query parameters preserved during redirect")
}

// TestAuthRedirectFollowThrough verifies browser can follow redirect to settings page
func TestAuthRedirectFollowThrough(t *testing.T) {
	env, err := common.SetupTestEnvironment("AuthRedirectFollowThrough")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var finalURL string
	var title string

	// Navigate to /auth and let browser follow redirect
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(env.GetBaseURL()+"/auth"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to follow redirect from /auth: %v", err)
	}

	// Should end up on settings page
	if title != "Settings - Quaero" {
		t.Errorf("Expected title 'Settings - Quaero' after redirect, got '%s'", title)
	}

	// URL should contain settings
	if !strings.Contains(finalURL, "/settings") {
		t.Errorf("Expected final URL to contain /settings, got %s", finalURL)
	}

	// URL should contain accordion parameter
	if !strings.Contains(finalURL, "a=") {
		t.Errorf("Expected final URL to contain accordion parameter, got %s", finalURL)
	}

	t.Log("✓ Browser successfully follows redirect to settings page")
}
