package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPageLayoutConsistency tests that the navbar, footer, and service status are consistent across all pages
func TestPageLayoutConsistency(t *testing.T) {
	t.Log("=== Testing Page Layout Consistency (Navbar, Footer, Service Status) ===")

	// Load test configuration
	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")
	
	// Setup ChromeDP context with options for better UI testing
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // Run in visible mode to see what's happening
		chromedp.Flag("disable-gpu", false),
		chromedp.WindowSize(1280, 720),
	)
	
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	
	// Set a longer timeout for UI operations
	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Define all the navbar menu items and their expected URLs
	navbarItems := []struct {
		name     string
		selector string
		url      string
		pageTitle string
	}{
		{"HOME", `a[href="/"]`, config.ServerURL + "/", "Quaero - Monitoring Dashboard"},
		{"JIRA DATA", `a[href="/jira"]`, config.ServerURL + "/jira", "Quaero - Jira Project Management"},
		{"CONFLUENCE DATA", `a[href="/confluence"]`, config.ServerURL + "/confluence", "Quaero - Confluence Space Management"},
		{"DOCUMENTS", `a[href="/documents"]`, config.ServerURL + "/documents", "Quaero - Document Management"},
		{"CHAT", `a[href="/chat"]`, config.ServerURL + "/chat", "Chat - Quaero"},
		{"SETTINGS", `a[href="/settings"]`, config.ServerURL + "/settings", "Quaero - Settings"},
	}

	// Test each navbar item once
	for i, item := range navbarItems {
		t.Logf("Testing page %d: %s", i+1, item.name)
		
		var pageTitle, navbarStatus, siteSubtitle, footerText string
		var siteTitle map[string]interface{}
		var serviceStatusExists, footerExists bool
		var navbarMenuItems []string
		var themeValidation map[string]interface{}
		
		err := chromedp.Run(ctx,
			// Navigate to the page
			chromedp.Navigate(item.url),
			chromedp.Sleep(2*time.Second),
			
			// Take screenshot for debugging
			chromedp.ActionFunc(func(c context.Context) error {
				takeScreenshot(ctx, t, "navbar_"+strings.ToLower(strings.ReplaceAll(item.name, " ", "_")))
				return nil
			}),
			
			// Verify page loaded correctly
			chromedp.Title(&pageTitle),
			
			// Check navbar status indicator (top-right "ONLINE" text)
			chromedp.Text(`.status-text`, &navbarStatus),
			
			// Check site title structure (must have separate div and small elements with proper styling)
			chromedp.Evaluate(`(() => {
				const titleLink = document.querySelector('nav.top a:first-child');
				if (!titleLink) return 'NO_TITLE_LINK';
				const mainTitle = titleLink.querySelector('div');
				if (!mainTitle) return 'NO_DIV_ELEMENT';
				// Check if the link has proper flex styling for vertical layout
				const linkStyle = window.getComputedStyle(titleLink);
				const hasFlexColumn = linkStyle.display === 'flex' && linkStyle.flexDirection === 'column';
				return {
					text: mainTitle.textContent.trim(),
					hasProperStyling: hasFlexColumn
				};
			})()`, &siteTitle),
			
			chromedp.Evaluate(`(() => {
				const titleLink = document.querySelector('nav.top a:first-child');
				if (!titleLink) return 'NO_TITLE_LINK';
				const subtitle = titleLink.querySelector('small');
				return subtitle ? subtitle.textContent.trim() : 'NO_SMALL_ELEMENT';
			})()`, &siteSubtitle),
			
			// Check if Service Status section exists
			chromedp.Evaluate(`
				Array.from(document.querySelectorAll('h5')).some(h5 => h5.textContent.includes('Service Status'))
			`, &serviceStatusExists),
			
			// Get all navbar menu items to verify they're all present
			chromedp.Evaluate(`Array.from(document.querySelectorAll('nav.top a.min')).map(a => a.textContent.trim())`, &navbarMenuItems),
			
			// Validate theme and layout styling
			chromedp.Evaluate(`(() => {
				const body = document.body;
				const navbar = document.querySelector('nav.top');
				const titleLink = document.querySelector('nav.top a:first-child');
				const statusIndicator = document.querySelector('.status-indicator, .status-text');
				
				if (!body || !navbar || !titleLink) {
					return { error: 'Missing required elements' };
				}
				
				const bodyStyle = window.getComputedStyle(body);
				const navbarStyle = window.getComputedStyle(navbar);
				const titleStyle = window.getComputedStyle(titleLink);
				
				return {
					body: {
						backgroundColor: bodyStyle.backgroundColor,
						color: bodyStyle.color,
						fontFamily: bodyStyle.fontFamily
					},
					navbar: {
						backgroundColor: navbarStyle.backgroundColor,
						color: navbarStyle.color,
						justifyContent: navbarStyle.justifyContent,
						alignItems: navbarStyle.alignItems
					},
					title: {
						position: titleStyle.position,
						left: titleStyle.left,
						float: titleStyle.float,
						textAlign: titleStyle.textAlign,
						fontFamily: titleStyle.fontFamily
					},
					status: {
						position: statusIndicator ? window.getComputedStyle(statusIndicator).position : 'none',
						right: statusIndicator ? window.getComputedStyle(statusIndicator).right : 'none',
						float: statusIndicator ? window.getComputedStyle(statusIndicator).float : 'none'
					},
					isLightTheme: bodyStyle.backgroundColor.includes('255') || bodyStyle.backgroundColor.includes('254') || bodyStyle.backgroundColor.includes('253') || bodyStyle.backgroundColor.includes('252') || bodyStyle.backgroundColor.includes('251') || bodyStyle.backgroundColor.includes('250') || bodyStyle.backgroundColor.includes('white') || bodyStyle.backgroundColor.includes('248, 249, 251'),
					isDarkTheme: bodyStyle.backgroundColor.includes('20, 19, 22') || bodyStyle.backgroundColor === 'rgb(0, 0, 0)' || bodyStyle.backgroundColor.includes('10, 10, 10')
				};
			})()`, &themeValidation),
			
			// Check footer presence and content
			chromedp.Evaluate(`(() => {
				const footer = document.querySelector('nav.bottom, footer');
				return footer !== null;
			})()`, &footerExists),
			
			chromedp.Evaluate(`(() => {
				const footer = document.querySelector('nav.bottom, footer');
				if (!footer) return 'NO_FOOTER';
				const versionElement = footer.querySelector('#footer-version, small');
				return versionElement ? versionElement.textContent.trim() : footer.textContent.trim();
			})()`, &footerText),
		)
		
		require.NoError(t, err, "Failed to test page: %s", item.name)
		
		// Verify page title matches expected
		assert.Contains(t, pageTitle, "Quaero", "Page title should contain 'Quaero' for %s", item.name)
		
		// Verify site title structure and content
		if siteTitle != nil {
			if titleText, ok := siteTitle["text"].(string); ok {
				assert.Equal(t, "QUAERO", titleText, "Site title text should be 'QUAERO' on %s page, got '%s'", item.name, titleText)
			} else {
				t.Errorf("Site title text not found on %s page", item.name)
			}
			
			if hasProperStyling, ok := siteTitle["hasProperStyling"].(bool); ok {
				assert.True(t, hasProperStyling, "Site title should have proper flex column styling on %s page for vertical layout", item.name)
			} else {
				t.Errorf("Site title styling info not found on %s page", item.name)
			}
		} else {
			t.Errorf("Site title structure not found on %s page", item.name)
		}
		
		// Verify site subtitle consistency (DATA COLLECTION SERVICE)
		assert.Equal(t, "DATA COLLECTION SERVICE", siteSubtitle, "Site subtitle should be 'DATA COLLECTION SERVICE' on %s page, got '%s'", item.name, siteSubtitle)
		
		// Verify navbar status shows "ONLINE" consistently on ALL pages
		assert.Equal(t, "ONLINE", navbarStatus, "Navbar status should be 'ONLINE' on %s page for consistency, got '%s'", item.name, navbarStatus)
		
		// Verify Service Status section exists on the page
		assert.True(t, serviceStatusExists, "Service Status section should exist on %s page", item.name)
		
		// Verify Service Logs section exists (except on chat page)
		if item.name != "CHAT" {
			var serviceLogsExists bool
			err := chromedp.Run(ctx,
				chromedp.Evaluate(`document.getElementById('service-logs') !== null`, &serviceLogsExists),
			)
			require.NoError(t, err, "Failed to check service logs on %s page", item.name)
			assert.True(t, serviceLogsExists, "Service Logs section should exist on %s page", item.name)
		}
		
		// Verify all navbar items are present
		expectedNavItems := []string{"HOME", "JIRA DATA", "CONFLUENCE DATA", "DOCUMENTS", "CHAT", "SETTINGS"}
		assert.Equal(t, expectedNavItems, navbarMenuItems, "All navbar items should be present on %s page", item.name)
		
		// Verify footer presence and content
		assert.True(t, footerExists, "Footer should exist on %s page", item.name)
		assert.Contains(t, footerText, "Quaero", "Footer should contain 'Quaero' on %s page, got '%s'", item.name, footerText)
		assert.Contains(t, footerText, "Version", "Footer should contain 'Version' on %s page, got '%s'", item.name, footerText)
		assert.Contains(t, footerText, "Build", "Footer should contain 'Build' on %s page, got '%s'", item.name, footerText)
		
		// Validate theme and layout (CRITICAL - must match reference design)
		if themeValidation != nil {
			// Check for light theme (background should be light/white, not dark)
			if isLight, ok := themeValidation["isLightTheme"].(bool); ok && !isLight {
				t.Errorf("❌ THEME VALIDATION FAILED: %s page is using DARK theme but should use LIGHT theme like reference design", item.name)
				if body, ok := themeValidation["body"].(map[string]interface{}); ok {
					if bgColor, ok := body["backgroundColor"].(string); ok {
						t.Errorf("   Current background color: %s (should be white/light)", bgColor)
					}
				}
			}
			
			// Verify navbar layout positioning
			if title, ok := themeValidation["title"].(map[string]interface{}); ok {
				if fontFamily, ok := title["fontFamily"].(string); ok {
					// Verify monospace font is being used as requested
					if !strings.Contains(strings.ToLower(fontFamily), "courier") && !strings.Contains(strings.ToLower(fontFamily), "monaco") && !strings.Contains(strings.ToLower(fontFamily), "monospace") {
						t.Logf("ℹ️ NOTE: %s page title using font '%s' - monospace preferred", item.name, fontFamily)
					}
				}
			}
			
			t.Logf("🔍 Theme validation for %s: %+v", item.name, themeValidation)
		} else {
			t.Errorf("❌ THEME VALIDATION FAILED: Could not validate theme on %s page", item.name)
		}
		
		titleText := "UNKNOWN"
		stylingOk := false
		if siteTitle != nil {
			if text, ok := siteTitle["text"].(string); ok {
				titleText = text
			}
			if styling, ok := siteTitle["hasProperStyling"].(bool); ok {
				stylingOk = styling
			}
		}
		
		t.Logf("✓ %s page verified: site title='%s' (styling: %v), subtitle='%s', navbar status='%s', service status exists=%v, footer='%s'", 
			item.name, titleText, stylingOk, siteSubtitle, navbarStatus, serviceStatusExists, footerText)
		
		// Test service status table content once (on home page)
		if i == 0 {
			var serviceStatusTable string
			err := chromedp.Run(ctx,
				chromedp.Text(`#service-status-table`, &serviceStatusTable),
			)
			
			require.NoError(t, err, "Failed to read service status table")
			assert.Contains(t, serviceStatusTable, "PARSER SERVICE", "Service status table should contain 'PARSER SERVICE'")
			assert.Contains(t, serviceStatusTable, "DATABASE", "Service status table should contain 'DATABASE'")
			assert.Contains(t, serviceStatusTable, "EXTENSION AUTH", "Service status table should contain 'EXTENSION AUTH'")
			
			t.Logf("✓ Service Status table content verified")
		}
	}
	
	t.Log("\n✅ SUCCESS: All page layout elements (navbar, footer, service status) are consistent across all pages")
}

// TestServiceStatusInteractivity tests that the service status section is interactive
func TestServiceStatusInteractivity(t *testing.T) {
	t.Log("=== Testing Service Status Interactivity ===")
	
	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")
	
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)
	
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	
	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()
	
	var refreshButtonExists bool
	var tableContentBefore, tableContentAfter string
	
	err = chromedp.Run(ctx,
		// Navigate to home page
		chromedp.Navigate(config.ServerURL),
		chromedp.Sleep(2*time.Second),
		
		// Check if refresh button exists
		chromedp.Evaluate(`document.querySelector('button[onclick="refreshServiceStatus()"]') !== null`, &refreshButtonExists),
		
		// Get initial table content
		chromedp.Text(`#service-status-table`, &tableContentBefore),
		
		chromedp.ActionFunc(func(c context.Context) error {
			takeScreenshot(ctx, t, "service_status_before_refresh")
			return nil
		}),
	)
	
	require.NoError(t, err, "Failed to load page and check service status")
	
	// Verify refresh button exists
	assert.True(t, refreshButtonExists, "Service status refresh button should exist")
	
	// Verify initial content
	assert.Contains(t, tableContentBefore, "PARSER SERVICE", "Service status should contain PARSER SERVICE")
	assert.Contains(t, tableContentBefore, "ONLINE", "Service status should contain ONLINE status")
	
	// Test refresh functionality (if button exists)
	if refreshButtonExists {
		err = chromedp.Run(ctx,
			// Click refresh button
			chromedp.Click(`button[onclick="refreshServiceStatus()"]`),
			chromedp.Sleep(3*time.Second), // Wait for refresh
			
			// Get content after refresh
			chromedp.Text(`#service-status-table`, &tableContentAfter),
			
			chromedp.ActionFunc(func(c context.Context) error {
				takeScreenshot(ctx, t, "service_status_after_refresh")
				return nil
			}),
		)
		
		require.NoError(t, err, "Failed to refresh service status")
		
		// Content should still be valid after refresh
		assert.Contains(t, tableContentAfter, "PARSER SERVICE", "Service status should still contain PARSER SERVICE after refresh")
		
		t.Logf("✓ Service status refresh functionality verified")
	}
	
	t.Log("✅ SUCCESS: Service status interactivity verified")
}

// Note: takeScreenshot function is defined in ui_test.go with signature takeScreenshot(ctx context.Context, t *testing.T, name string)
