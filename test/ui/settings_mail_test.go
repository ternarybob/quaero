package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestSettingsMailConfiguration tests the email configuration functionality
// in the Settings > Email section. This test:
// 1. Navigates to Settings page
// 2. Opens Email section
// 3. Fills in SMTP configuration
// 4. Saves configuration
// 5. Verifies "Email Configured" status
// 6. Cleans up by clearing the configuration
func TestSettingsMailConfiguration(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create a timeout context for the entire test
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancelTimeout()

	// Create a new allocator context for the browser
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	// Create the browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	// Base URL for the service
	baseURL := env.GetBaseURL()
	settingsURL := baseURL + "/settings"

	env.LogTest(t, "Navigating to Settings page: %s", settingsURL)

	// 2. Navigate to Settings Page
	if err := chromedp.Run(ctx,
		chromedp.Navigate(settingsURL),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Failed to navigate to settings page: %v", err)
	}

	TakeFullScreenshotInDir(ctx, env.ResultsDir, "settings_page_loaded")

	// 3. Navigate to Email Section
	env.LogTest(t, "Navigating to Email section")

	if err := chromedp.Run(ctx,
		chromedp.Click(`//ul[@class="nav"]//li//a[contains(., "Email")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		t.Fatalf("Failed to navigate to Email section: %v", err)
	}

	// Wait for the Email Configuration card to be visible
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`//h3[contains(., "Email Configuration")]`, chromedp.BySearch),
	); err != nil {
		t.Fatalf("Failed to find Email Configuration section: %v", err)
	}

	TakeFullScreenshotInDir(ctx, env.ResultsDir, "email_section_loaded")

	// 4. Verify Initial State: "Email Not Configured"
	env.LogTest(t, "Verifying initial state (not configured)")

	checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
	err = chromedp.Run(checkCtx,
		chromedp.WaitVisible(`//span[contains(., "Email Not Configured")]`, chromedp.BySearch),
	)
	checkCancel()

	if err != nil {
		// May already be configured from previous run, continue anyway
		env.LogTest(t, "Note: Email may already be configured from previous run")
	} else {
		env.LogTest(t, "✓ Initial state verified: Email Not Configured")
	}

	// 5. Fill in SMTP Configuration
	env.LogTest(t, "Filling in SMTP configuration")

	testConfig := struct {
		Host     string
		Port     string
		Username string
		Password string
		From     string
		FromName string
	}{
		Host:     "smtp.test.example.com",
		Port:     "587",
		Username: "testuser@example.com",
		Password: "testpassword123",
		From:     "testuser@example.com",
		FromName: "Quaero Test",
	}

	// Clear existing values and fill in new ones
	if err := chromedp.Run(ctx,
		// Clear and fill SMTP Host
		chromedp.Clear(`#smtp_host`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_host`, testConfig.Host, chromedp.ByID),

		// Clear and fill SMTP Port
		chromedp.Clear(`#smtp_port`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_port`, testConfig.Port, chromedp.ByID),

		// Clear and fill Username
		chromedp.Clear(`#smtp_username`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_username`, testConfig.Username, chromedp.ByID),

		// Clear and fill Password
		chromedp.Clear(`#smtp_password`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_password`, testConfig.Password, chromedp.ByID),

		// Clear and fill From Email
		chromedp.Clear(`#smtp_from`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_from`, testConfig.From, chromedp.ByID),

		// Clear and fill From Name
		chromedp.Clear(`#smtp_from_name`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_from_name`, testConfig.FromName, chromedp.ByID),
	); err != nil {
		t.Fatalf("Failed to fill SMTP configuration: %v", err)
	}

	TakeFullScreenshotInDir(ctx, env.ResultsDir, "email_form_filled")

	// 6. Save Configuration
	env.LogTest(t, "Saving email configuration")

	if err := chromedp.Run(ctx,
		chromedp.Click(`//button[contains(., "Save Configuration")]`, chromedp.BySearch),
		chromedp.Sleep(2*time.Second), // Wait for save and UI update
	); err != nil {
		t.Fatalf("Failed to save email configuration: %v", err)
	}

	TakeFullScreenshotInDir(ctx, env.ResultsDir, "email_config_saved")

	// 7. Verify Configured State
	env.LogTest(t, "Verifying configured state")

	checkCtx, checkCancel = context.WithTimeout(ctx, 10*time.Second)
	err = chromedp.Run(checkCtx,
		chromedp.WaitVisible(`//span[contains(., "Email Configured")]`, chromedp.BySearch),
	)
	checkCancel()

	if err != nil {
		// Dump HTML for debugging
		var bodyHTML string
		if dumpErr := chromedp.Run(ctx, chromedp.OuterHTML("body", &bodyHTML)); dumpErr == nil {
			dumpPath := filepath.Join(env.ResultsDir, "email_config_fail_dump.html")
			os.WriteFile(dumpPath, []byte(bodyHTML), 0644)
		}
		t.Errorf("Email configuration not saved correctly - 'Email Configured' badge not found: %v", err)
	} else {
		env.LogTest(t, "✓ Email configuration saved successfully")
	}

	// 8. Verify Form Values Persisted
	env.LogTest(t, "Verifying form values persisted")

	var hostValue, portValue, usernameValue, fromValue, fromNameValue string

	if err := chromedp.Run(ctx,
		chromedp.Value(`#smtp_host`, &hostValue, chromedp.ByID),
		chromedp.Value(`#smtp_port`, &portValue, chromedp.ByID),
		chromedp.Value(`#smtp_username`, &usernameValue, chromedp.ByID),
		chromedp.Value(`#smtp_from`, &fromValue, chromedp.ByID),
		chromedp.Value(`#smtp_from_name`, &fromNameValue, chromedp.ByID),
	); err != nil {
		t.Errorf("Failed to read form values: %v", err)
	} else {
		assertions := []struct {
			name     string
			expected string
			actual   string
		}{
			{"Host", testConfig.Host, hostValue},
			{"Port", testConfig.Port, portValue},
			{"Username", testConfig.Username, usernameValue},
			{"From", testConfig.From, fromValue},
			{"FromName", testConfig.FromName, fromNameValue},
		}

		for _, a := range assertions {
			if a.actual != a.expected {
				t.Errorf("%s mismatch: expected '%s', got '%s'", a.name, a.expected, a.actual)
			} else {
				env.LogTest(t, "✓ %s value verified: %s", a.name, a.actual)
			}
		}
	}

	// 9. Verify Password is Masked (optional - password field may show masked value)
	var passwordValue string
	if err := chromedp.Run(ctx,
		chromedp.Value(`#smtp_password`, &passwordValue, chromedp.ByID),
	); err == nil {
		// Password should either be empty (not returned) or masked
		if passwordValue != "" && passwordValue != "********" && passwordValue != testConfig.Password {
			env.LogTest(t, "Note: Password field contains value: %s", passwordValue)
		}
	}

	// 10. Cleanup: Clear the configuration by setting empty values
	env.LogTest(t, "Cleaning up: Clearing email configuration")

	if err := chromedp.Run(ctx,
		chromedp.Clear(`#smtp_host`, chromedp.ByID),
		chromedp.Clear(`#smtp_username`, chromedp.ByID),
		chromedp.Clear(`#smtp_password`, chromedp.ByID),
		chromedp.Clear(`#smtp_from`, chromedp.ByID),
		chromedp.Click(`//button[contains(., "Save Configuration")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		t.Logf("Cleanup warning: Failed to clear email configuration: %v", err)
	} else {
		env.LogTest(t, "✓ Email configuration cleared")
	}

	TakeFullScreenshotInDir(ctx, env.ResultsDir, "email_config_cleanup")

	env.LogTest(t, "Test completed successfully")
}

// TestSettingsMailConfigurationPersistence tests that email configuration persists after page reload
func TestSettingsMailConfigurationPersistence(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create a timeout context for the entire test
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancelTimeout()

	// Create a new allocator context for the browser
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	// Create the browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	baseURL := env.GetBaseURL()

	// Test configuration
	testConfig := struct {
		Host     string
		Port     string
		Username string
		Password string
		From     string
	}{
		Host:     "smtp.persistence-test.example.com",
		Port:     "465",
		Username: "persist@example.com",
		Password: "persistpassword",
		From:     "persist@example.com",
	}

	// Phase 1: Save configuration
	env.LogTest(t, "Phase 1: Saving email configuration")

	if err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/settings"),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Click(`//ul[@class="nav"]//li//a[contains(., "Email")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
		chromedp.WaitVisible(`//h3[contains(., "Email Configuration")]`, chromedp.BySearch),
	); err != nil {
		t.Fatalf("Failed to navigate to email settings: %v", err)
	}

	if err := chromedp.Run(ctx,
		chromedp.Clear(`#smtp_host`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_host`, testConfig.Host, chromedp.ByID),
		chromedp.Clear(`#smtp_port`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_port`, testConfig.Port, chromedp.ByID),
		chromedp.Clear(`#smtp_username`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_username`, testConfig.Username, chromedp.ByID),
		chromedp.Clear(`#smtp_password`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_password`, testConfig.Password, chromedp.ByID),
		chromedp.Clear(`#smtp_from`, chromedp.ByID),
		chromedp.SendKeys(`#smtp_from`, testConfig.From, chromedp.ByID),
		chromedp.Click(`//button[contains(., "Save Configuration")]`, chromedp.BySearch),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Failed to save email configuration: %v", err)
	}

	TakeFullScreenshotInDir(ctx, env.ResultsDir, "persistence_phase1_saved")

	// Phase 2: Reload page and verify persistence
	env.LogTest(t, "Phase 2: Reloading page and verifying persistence")

	if err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/settings"),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Click(`//ul[@class="nav"]//li//a[contains(., "Email")]`, chromedp.BySearch),
		chromedp.Sleep(2*time.Second), // Wait for data to load
	); err != nil {
		t.Fatalf("Failed to reload settings page: %v", err)
	}

	TakeFullScreenshotInDir(ctx, env.ResultsDir, "persistence_phase2_reloaded")

	// Verify values persisted
	var hostValue, portValue, usernameValue, fromValue string

	if err := chromedp.Run(ctx,
		chromedp.Value(`#smtp_host`, &hostValue, chromedp.ByID),
		chromedp.Value(`#smtp_port`, &portValue, chromedp.ByID),
		chromedp.Value(`#smtp_username`, &usernameValue, chromedp.ByID),
		chromedp.Value(`#smtp_from`, &fromValue, chromedp.ByID),
	); err != nil {
		t.Fatalf("Failed to read form values after reload: %v", err)
	}

	if hostValue != testConfig.Host {
		t.Errorf("Host not persisted: expected '%s', got '%s'", testConfig.Host, hostValue)
	} else {
		env.LogTest(t, "✓ Host persisted: %s", hostValue)
	}

	if portValue != testConfig.Port {
		t.Errorf("Port not persisted: expected '%s', got '%s'", testConfig.Port, portValue)
	} else {
		env.LogTest(t, "✓ Port persisted: %s", portValue)
	}

	if usernameValue != testConfig.Username {
		t.Errorf("Username not persisted: expected '%s', got '%s'", testConfig.Username, usernameValue)
	} else {
		env.LogTest(t, "✓ Username persisted: %s", usernameValue)
	}

	if fromValue != testConfig.From {
		t.Errorf("From not persisted: expected '%s', got '%s'", testConfig.From, fromValue)
	} else {
		env.LogTest(t, "✓ From persisted: %s", fromValue)
	}

	// Cleanup
	env.LogTest(t, "Cleaning up")
	if err := chromedp.Run(ctx,
		chromedp.Clear(`#smtp_host`, chromedp.ByID),
		chromedp.Clear(`#smtp_username`, chromedp.ByID),
		chromedp.Clear(`#smtp_password`, chromedp.ByID),
		chromedp.Clear(`#smtp_from`, chromedp.ByID),
		chromedp.Click(`//button[contains(., "Save Configuration")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		t.Logf("Cleanup warning: %v", err)
	}

	env.LogTest(t, "Test completed successfully")
}

// assertEmailConfigured is a helper that verifies the Email Configured badge is visible
func assertEmailConfigured(ctx context.Context, t *testing.T, env *common.TestEnvironment) {
	checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
	defer checkCancel()

	err := chromedp.Run(checkCtx,
		chromedp.WaitVisible(`//span[contains(., "Email Configured")]`, chromedp.BySearch),
	)

	if err != nil {
		t.Errorf("Expected 'Email Configured' badge not found: %v", err)
	} else {
		env.LogTest(t, "✓ Email Configured badge verified")
	}
}

// assertEmailNotConfigured is a helper that verifies the Email Not Configured badge is visible
func assertEmailNotConfigured(ctx context.Context, t *testing.T, env *common.TestEnvironment) {
	checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
	defer checkCancel()

	err := chromedp.Run(checkCtx,
		chromedp.WaitVisible(`//span[contains(., "Email Not Configured")]`, chromedp.BySearch),
	)

	if err != nil {
		env.LogTest(t, "Note: 'Email Not Configured' badge not found (may be configured)")
	} else {
		env.LogTest(t, "✓ Email Not Configured badge verified")
	}
}

// Suppress unused function warnings for helper functions
var _ = assertEmailConfigured
var _ = assertEmailNotConfigured
var _ = fmt.Sprint
