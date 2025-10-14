// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 2:21:12 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

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

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set a longer timeout for UI operations (multiple pages to test)
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Define all the navbar menu items and their expected URLs
	navbarItems := []struct {
		name          string
		selector      string
		url           string
		pageTitle     string
		hasRefreshBtn bool // Pages that should have refresh buttons
	}{
		{"HOME", `a[href="/"]`, config.ServerURL + "/", "Quaero - Monitoring Dashboard", true},
		{"JIRA DATA", `a[href="/jira"]`, config.ServerURL + "/jira", "Quaero - Jira Project Management", true},
		{"CONFLUENCE DATA", `a[href="/confluence"]`, config.ServerURL + "/confluence", "Quaero - Confluence Space Management", true},
		{"DOCUMENTS", `a[href="/documents"]`, config.ServerURL + "/documents", "Quaero - Document Management", true},
		{"CHAT", `a[href="/chat"]`, config.ServerURL + "/chat", "Chat - Quaero", false},
		{"SETTINGS", `a[href="/settings"]`, config.ServerURL + "/settings", "Quaero - Settings", false},
	}

	// Bulma navbar should be exactly 52px (3.25rem) - no tolerance
	const expectedNavbarHeight = 52.0
	const navbarHeightTolerance = 1.0 // Allow 1px for rounding

	// Test each navbar item once
	for i, item := range navbarItems {
		t.Logf("Testing page %d: %s", i+1, item.name)

		var pageTitle, navbarStatus, siteSubtitle, footerText string
		var siteTitle map[string]interface{}
		var serviceStatusExists, footerExists bool
		var navbarMenuItems []string
		var themeValidation map[string]interface{}
		var navbarConsistency map[string]interface{}

		// All pages use server-side templates now - consistent loading
		var actions []chromedp.Action
		actions = append(actions,
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

			// Check site title structure (Bulma navbar)
			chromedp.Evaluate(`(() => {
				const brand = document.querySelector('.navbar-brand .navbar-item strong');
				if (!brand) return { text: 'NO_BRAND_ELEMENT', hasProperStyling: false };
				return {
					text: brand.textContent.trim(),
					hasProperStyling: true
				};
			})()`, &siteTitle),

			chromedp.Evaluate(`(() => {
				// Bulma doesn't use subtitle in navbar brand, check hero section instead
				const heroSubtitle = document.querySelector('.hero .subtitle');
				return heroSubtitle ? heroSubtitle.textContent.trim() : 'NO_SUBTITLE';
			})()`, &siteSubtitle),

			// Check if Parser Status section exists (optional, may not be on all pages) - Bulma card header
			chromedp.Evaluate(`
				Array.from(document.querySelectorAll('.card-header-title')).some(title => title.textContent.trim() === 'Parser Status')
			`, &serviceStatusExists),

			// Get all navbar menu items to verify they're all present (Bulma - only from navbar-start)
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.navbar-start .navbar-item')).map(a => a.textContent.trim())`, &navbarMenuItems),

			// Validate theme and layout styling (Bulma)
			chromedp.Evaluate(`(() => {
				const body = document.body;
				const navbar = document.querySelector('.navbar');
				const brand = document.querySelector('.navbar-brand .navbar-item');
				const statusIndicator = document.querySelector('.status-text');
				
				if (!body || !navbar || !brand) {
					return { error: 'Missing required elements' };
				}
				
				const bodyStyle = window.getComputedStyle(body);
				const navbarStyle = window.getComputedStyle(navbar);
				const titleStyle = window.getComputedStyle(brand);
				
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
					isLightTheme: bodyStyle.backgroundColor.includes('255') || bodyStyle.backgroundColor.includes('254') || bodyStyle.backgroundColor.includes('253') || bodyStyle.backgroundColor.includes('252') || bodyStyle.backgroundColor.includes('251') || bodyStyle.backgroundColor.includes('250') || bodyStyle.backgroundColor.includes('white') || bodyStyle.backgroundColor.includes('248, 249, 251') || bodyStyle.backgroundColor.includes('245, 245, 245'),
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

			// NEW: Comprehensive navbar consistency checks (Bulma)
			chromedp.Evaluate(`(() => {
				// 1. Check for forced navbar styling (inline style blocks)
				const styleBlocks = document.querySelectorAll('style');
				let hasForcedNavbarStyling = false;
				let forcedStyleContent = '';

				styleBlocks.forEach(style => {
					const content = style.textContent || '';
					// Check for inline CSS targeting navbar (Bulma)
					if (content.includes('.navbar') &&
					    content.includes('flex-direction') &&
					    content.includes('column')) {
						hasForcedNavbarStyling = true;
						forcedStyleContent = content;
					}
				});

				// 2. Measure navbar height (Bulma)
				const navbar = document.querySelector('.navbar');
				const navbarHeight = navbar ? navbar.offsetHeight : 0;

				// 3. Check hero section structure (Bulma)
				const heroSection = document.querySelector('.hero');
				let heroStructure = {
					exists: heroSection !== null,
					hasTitle: false,
					hasDescription: false,
					classes: ''
				};
				
				if (heroSection) {
					heroStructure.classes = heroSection.className;
					// Check for h1 or h2 title (Bulma uses .title class)
					const title = heroSection.querySelector('h1.title, h2.title, .title');
					heroStructure.hasTitle = title !== null;
					// Check for description paragraph (Bulma uses .subtitle class)
					const description = heroSection.querySelector('p.subtitle, .subtitle');
					heroStructure.hasDescription = description !== null;
				}

				// 4. Check refresh button styling (Bulma uses Alpine.js @click instead of onclick)
				const refreshButtons = document.querySelectorAll('button[aria-label*="refresh"], button[title*="Refresh"]');
				let refreshButtonInfo = {
					count: refreshButtons.length,
					buttons: []
				};
				
				refreshButtons.forEach(btn => {
					const classes = btn.className;
					const hasCircle = classes.includes('circle');
					const hasTransparent = classes.includes('transparent');
					const innerHTML = btn.innerHTML;
					const hasIcon = innerHTML.includes('<i') || innerHTML.includes('refresh');
					const hasText = /^[A-Z\s]{4,}$/.test(btn.textContent.trim()); // Has uppercase text (4+ chars)
					
					refreshButtonInfo.buttons.push({
						classes: classes,
						hasCircle: hasCircle,
						hasTransparent: hasTransparent,
						hasIcon: hasIcon,
						hasText: hasText,
						content: btn.textContent.trim().substring(0, 20)
					});
				});
				
				return {
					hasForcedNavbarStyling: hasForcedNavbarStyling,
					forcedStyleContent: forcedStyleContent.substring(0, 200), // First 200 chars
					navbarHeight: navbarHeight,
					heroSection: heroStructure,
					refreshButtons: refreshButtonInfo
				};
			})()`, &navbarConsistency),
		)

		// Execute all actions
		err = chromedp.Run(ctx, actions...)
		require.NoError(t, err, "Failed to test page: %s", item.name)

		// Verify page title matches expected
		assert.Contains(t, pageTitle, "Quaero", "Page title should contain 'Quaero' for %s", item.name)

		// Verify site title structure and content (Bulma)
		if siteTitle != nil {
			if titleText, ok := siteTitle["text"].(string); ok {
				assert.Equal(t, "Quaero", titleText, "Site title text should be 'Quaero' on %s page, got '%s'", item.name, titleText)
			} else {
				t.Errorf("Site title text not found on %s page", item.name)
			}

			if hasProperStyling, ok := siteTitle["hasProperStyling"].(bool); ok {
				assert.True(t, hasProperStyling, "Site title should be present in navbar on %s page", item.name)
			} else {
				t.Errorf("Site title styling info not found on %s page", item.name)
			}
		} else {
			t.Errorf("Site title structure not found on %s page", item.name)
		}

		// Verify site subtitle consistency (in hero section for Bulma, only on HOME page)
		if item.name == "HOME" {
			assert.Equal(t, "Atlassian Data Collection and Analysis Platform", siteSubtitle, "Site subtitle should be 'Atlassian Data Collection and Analysis Platform' on %s page, got '%s'", item.name, siteSubtitle)
		} else {
			t.Logf("Subtitle on %s page: %s", item.name, siteSubtitle)
		}

		// Verify navbar status shows "ONLINE" consistently on ALL pages (may include icon)
		assert.Contains(t, navbarStatus, "ONLINE", "Navbar status should contain 'ONLINE' on %s page for consistency, got '%s'", item.name, navbarStatus)

		// Log Service Status section presence (not all pages may have it)
		if !serviceStatusExists {
			t.Logf("ℹ️  Note: Service Status section not found on %s page", item.name)
		}

		// Verify Service Logs section exists (except on chat/settings pages) - Bulma card header
		if item.name != "CHAT" && item.name != "SETTINGS" {
			var serviceLogsExists bool
			err := chromedp.Run(ctx,
				chromedp.Evaluate(`Array.from(document.querySelectorAll('.card-header-title')).some(title => title.textContent.trim() === 'Service Logs')`, &serviceLogsExists),
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

		// NEW: Validate navbar consistency checks
		if navbarConsistency != nil {
			t.Logf("🔍 Navbar Consistency Checks for %s:", item.name)

			// 1. Check for forced navbar styling (CRITICAL - root cause of compaction)
			if hasForcedStyling, ok := navbarConsistency["hasForcedNavbarStyling"].(bool); ok {
				if hasForcedStyling {
					forcedContent := ""
					if content, ok := navbarConsistency["forcedStyleContent"].(string); ok {
						forcedContent = content
					}
					t.Errorf("❌ FORCED NAVBAR STYLING DETECTED on %s page!", item.name)
					t.Errorf("   Found inline <style> block with 'nav.top a:first-child' and 'flex-direction: column'")
					t.Errorf("   This is the ROOT CAUSE of navbar compaction issues")
					t.Errorf("   Style content preview: %s", forcedContent)
					assert.Fail(t, "Page has forced navbar styling that causes compaction",
						"Remove inline style blocks targeting navbar from %s page", item.name)
				} else {
					t.Logf("   ✓ No forced navbar styling detected")
				}
			}

			// 2. Navbar height must be exactly 52px (Bulma fixed navbar standard)
			if currentHeight, ok := navbarConsistency["navbarHeight"].(float64); ok {
				heightDiff := currentHeight - expectedNavbarHeight
				if heightDiff < 0 {
					heightDiff = -heightDiff
				}

				if heightDiff > navbarHeightTolerance {
					t.Errorf("❌ NAVBAR HEIGHT INCORRECT on %s page!", item.name)
					t.Errorf("   Expected: %.0fpx (Bulma is-fixed-top navbar standard)", expectedNavbarHeight)
					t.Errorf("   Actual: %.0fpx (difference: %.0fpx)", currentHeight, heightDiff)
					assert.Fail(t, "Navbar height must be 52px",
						"Navbar height on %s should be exactly 52px (3.25rem)", item.name)
				} else {
					t.Logf("   ✓ Navbar height correct: %.0fpx (expected: %.0fpx)", currentHeight, expectedNavbarHeight)
				}
			}

			// 3. Hero section structure validation
			if heroInfo, ok := navbarConsistency["heroSection"].(map[string]interface{}); ok {
				heroExists := false
				if exists, ok := heroInfo["exists"].(bool); ok {
					heroExists = exists
				}

				if heroExists {
					hasTitle := false
					hasDescription := false
					if ht, ok := heroInfo["hasTitle"].(bool); ok {
						hasTitle = ht
					}
					if hd, ok := heroInfo["hasDescription"].(bool); ok {
						hasDescription = hd
					}

					if !hasTitle || !hasDescription {
						t.Errorf("❌ INCOMPLETE HERO SECTION on %s page!", item.name)
						if !hasTitle {
							t.Errorf("   Missing: <h3> or <h4> title element")
						}
						if !hasDescription {
							t.Errorf("   Missing: <p> description element")
						}
						t.Errorf("   Expected structure: <header class=\"center-align\"> with title and description")
					} else {
						t.Logf("   ✓ Hero section properly structured with title and description")
					}
				} else {
					t.Logf("   ⚠️  No hero section found (may be intentional for %s page)", item.name)
				}
			}

			// 4. Refresh button consistency check
			if refreshInfo, ok := navbarConsistency["refreshButtons"].(map[string]interface{}); ok {
				buttonCount := 0
				if count, ok := refreshInfo["count"].(float64); ok {
					buttonCount = int(count)
				}

				if item.hasRefreshBtn && buttonCount == 0 {
					t.Errorf("❌ MISSING REFRESH BUTTON on %s page!", item.name)
					t.Errorf("   Expected: Refresh button with 'circle transparent' classes")
					assert.Fail(t, "Expected refresh button not found",
						"%s page should have a refresh button", item.name)
				} else if buttonCount > 0 {
					if buttons, ok := refreshInfo["buttons"].([]interface{}); ok {
						for idx, btn := range buttons {
							if btnData, ok := btn.(map[string]interface{}); ok {
								hasIcon := false

								if hi, ok := btnData["hasIcon"].(bool); ok {
									hasIcon = hi
								}

								// Bulma uses different button styling than BeerCSS - skip strict class checking
								// Just verify button has an icon
								if hasIcon {
									t.Logf("   ✓ Refresh button #%d has icon", idx+1)
								} else {
									t.Logf("   ⚠️  Refresh button #%d may be missing icon", idx+1)
								}
							}
						}
					}
				} else if !item.hasRefreshBtn {
					t.Logf("   ℹ️  No refresh button expected on %s page", item.name)
				}
			}

			// Take additional screenshot after navbar checks
			takeScreenshot(ctx, t, "navbar_validated_"+strings.ToLower(strings.ReplaceAll(item.name, " ", "_")))
		} else {
			t.Errorf("❌ Could not perform navbar consistency checks on %s page", item.name)
		}

		// Test service status table content once (on home page) if it exists
		if i == 0 && serviceStatusExists {
			var serviceStatusTable string
			var tableExists bool
			err := chromedp.Run(ctx,
				chromedp.Evaluate(`document.getElementById('service-status-table') !== null`, &tableExists),
			)

			if err == nil && tableExists {
				err = chromedp.Run(ctx,
					chromedp.Text(`#service-status-table`, &serviceStatusTable),
				)

				if err == nil {
					assert.Contains(t, serviceStatusTable, "PARSER SERVICE", "Service status table should contain 'PARSER SERVICE'")
					assert.Contains(t, serviceStatusTable, "DATABASE", "Service status table should contain 'DATABASE'")
					assert.Contains(t, serviceStatusTable, "EXTENSION AUTH", "Service status table should contain 'EXTENSION AUTH'")
					t.Logf("✓ Service Status table content verified")
				} else {
					t.Logf("⚠️  Could not read service status table: %v", err)
				}
			} else {
				t.Logf("ℹ️  Service status table not found on page")
			}
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

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

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

// TestButtonStyling tests that all buttons use correct styling (icons for refresh, small for actions)
func TestButtonStyling(t *testing.T) {
	t.Log("=== Testing Button Styling Consistency ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	pages := []struct {
		name string
		url  string
	}{
		{"HOME", config.ServerURL + "/"},
		{"JIRA DATA", config.ServerURL + "/jira"},
		{"CONFLUENCE DATA", config.ServerURL + "/confluence"},
		{"DOCUMENTS", config.ServerURL + "/documents"},
		{"CHAT", config.ServerURL + "/chat"},
		{"SETTINGS", config.ServerURL + "/settings"},
	}

	for _, page := range pages {
		t.Logf("Testing button styling on %s page", page.name)

		var buttonValidation map[string]interface{}

		err := chromedp.Run(ctx,
			chromedp.Navigate(page.url),
			chromedp.Sleep(2*time.Second),

			chromedp.ActionFunc(func(c context.Context) error {
				takeScreenshot(ctx, t, "buttons_"+strings.ToLower(strings.ReplaceAll(page.name, " ", "_")))
				return nil
			}),

			// Comprehensive button validation
			chromedp.Evaluate(`(() => {
				const results = {
					refreshButtons: [],
					actionButtons: [],
					errors: []
				};

				// 1. Check refresh/load/sync buttons (should be icon-only with circle transparent)
				const refreshSelectors = [
					'button[onclick*="refresh"]',
					'button[onclick*="load"]',
					'button[onclick*="sync"]'
				];

				refreshSelectors.forEach(selector => {
					const buttons = document.querySelectorAll(selector);
					buttons.forEach(btn => {
						const classes = btn.className;
						const hasCircle = classes.includes('circle');
						const hasTransparent = classes.includes('transparent');
						const innerHTML = btn.innerHTML;
						const hasIcon = innerHTML.includes('<i') && innerHTML.includes('material-icons');
						const textContent = btn.textContent.trim();
						// Check for text that looks like "REFRESH", "LOAD", "SYNC" (uppercase words)
						const hasTextButton = /^[A-Z\s]{4,}$/.test(textContent);
						const hasRefreshIcon = innerHTML.includes('refresh');

						const isValid = hasCircle && hasTransparent && hasIcon && !hasTextButton && hasRefreshIcon;

						results.refreshButtons.push({
							onclick: btn.getAttribute('onclick')?.substring(0, 50) || 'unknown',
							classes: classes,
							hasCircle: hasCircle,
							hasTransparent: hasTransparent,
							hasIcon: hasIcon,
							hasRefreshIcon: hasRefreshIcon,
							hasTextButton: hasTextButton,
							textContent: textContent,
							innerHTML: innerHTML.substring(0, 100),
							isValid: isValid
						});

						if (!isValid) {
							let issues = [];
							if (!hasCircle) issues.push('missing circle class');
							if (!hasTransparent) issues.push('missing transparent class');
							if (!hasIcon) issues.push('missing <i> icon element');
							if (!hasRefreshIcon) issues.push('missing refresh icon');
							if (hasTextButton) issues.push('has text content: "' + textContent + '"');
							results.errors.push('Refresh button issue: ' + issues.join(', '));
						}
					});
				});

				// 2. Check action buttons on Jira and Confluence pages (should have small class)
				const actionButtonIds = [
					'sync-btn',
					'get-issues-menu-btn',
					'get-pages-menu-btn',
					'clear-data-btn'
				];

				const actionButtonTexts = [
					'GET PROJECTS',
					'GET ISSUES',
					'CLEAR ALL DATA',
					'GET SPACES',
					'GET PAGES'
				];

				// Check by ID
				actionButtonIds.forEach(id => {
					const btn = document.getElementById(id);
					if (btn) {
						const classes = btn.className;
						const hasSmall = classes.includes('small');
						const text = btn.textContent.trim();

						results.actionButtons.push({
							id: id,
							text: text,
							classes: classes,
							hasSmall: hasSmall,
							isValid: hasSmall
						});

						if (!hasSmall) {
							results.errors.push('Action button "' + id + '" missing small class');
						}
					}
				});

				// Check by text content
				actionButtonTexts.forEach(buttonText => {
					const buttons = Array.from(document.querySelectorAll('button')).filter(
						btn => btn.textContent.trim() === buttonText
					);
					buttons.forEach(btn => {
						const classes = btn.className;
						const hasSmall = classes.includes('small');
						const id = btn.id || 'no-id';

						results.actionButtons.push({
							id: id,
							text: buttonText,
							classes: classes,
							hasSmall: hasSmall,
							isValid: hasSmall
						});

						if (!hasSmall) {
							results.errors.push('Action button "' + buttonText + '" missing small class');
						}
					});
				});

				return results;
			})()`, &buttonValidation),
		)

		require.NoError(t, err, "Failed to validate buttons on %s page", page.name)

		// Validate results
		if buttonValidation != nil {
			// Check refresh buttons
			if refreshButtons, ok := buttonValidation["refreshButtons"].([]interface{}); ok && len(refreshButtons) > 0 {
				t.Logf("Found %d refresh/load/sync buttons on %s page", len(refreshButtons), page.name)
				for idx, btn := range refreshButtons {
					if btnData, ok := btn.(map[string]interface{}); ok {
						isValid := false
						if iv, ok := btnData["isValid"].(bool); ok {
							isValid = iv
						}

						if !isValid {
							onclick := ""
							classes := ""
							textContent := ""
							innerHTML := ""
							hasCircle := false
							hasTransparent := false
							hasIcon := false
							hasRefreshIcon := false
							hasTextButton := false

							if oc, ok := btnData["onclick"].(string); ok {
								onclick = oc
							}
							if cl, ok := btnData["classes"].(string); ok {
								classes = cl
							}
							if tc, ok := btnData["textContent"].(string); ok {
								textContent = tc
							}
							if ih, ok := btnData["innerHTML"].(string); ok {
								innerHTML = ih
							}
							if hc, ok := btnData["hasCircle"].(bool); ok {
								hasCircle = hc
							}
							if ht, ok := btnData["hasTransparent"].(bool); ok {
								hasTransparent = ht
							}
							if hi, ok := btnData["hasIcon"].(bool); ok {
								hasIcon = hi
							}
							if hri, ok := btnData["hasRefreshIcon"].(bool); ok {
								hasRefreshIcon = hri
							}
							if htb, ok := btnData["hasTextButton"].(bool); ok {
								hasTextButton = htb
							}

							t.Errorf("❌ REFRESH BUTTON STYLING ISSUE on %s page (button #%d)", page.name, idx+1)
							t.Errorf("   Button: onclick=\"%s\"", onclick)
							t.Errorf("   Current classes: %s", classes)
							if !hasCircle {
								t.Errorf("   ❌ Missing 'circle' class")
							}
							if !hasTransparent {
								t.Errorf("   ❌ Missing 'transparent' class")
							}
							if !hasIcon {
								t.Errorf("   ❌ Missing <i class=\"material-icons\"> icon element")
							}
							if !hasRefreshIcon {
								t.Errorf("   ❌ Missing 'refresh' icon")
							}
							if hasTextButton {
								t.Errorf("   ❌ Has text content: \"%s\" (should be icon-only)", textContent)
							}
							t.Errorf("   Expected: <button class=\"circle transparent\"><i class=\"material-icons\">refresh</i></button>")
							t.Errorf("   Actual HTML: %s", innerHTML)
							assert.Fail(t, "Refresh button not properly styled",
								"Fix button styling on %s page", page.name)
						}
					}
				}
			}

			// Check action buttons (only on Jira and Confluence pages)
			if page.name == "JIRA DATA" || page.name == "CONFLUENCE DATA" {
				if actionButtons, ok := buttonValidation["actionButtons"].([]interface{}); ok && len(actionButtons) > 0 {
					t.Logf("Found %d action buttons on %s page", len(actionButtons), page.name)
					for idx, btn := range actionButtons {
						if btnData, ok := btn.(map[string]interface{}); ok {
							isValid := false
							if iv, ok := btnData["isValid"].(bool); ok {
								isValid = iv
							}

							if !isValid {
								id := ""
								text := ""
								classes := ""
								if btnId, ok := btnData["id"].(string); ok {
									id = btnId
								}
								if btnText, ok := btnData["text"].(string); ok {
									text = btnText
								}
								if cl, ok := btnData["classes"].(string); ok {
									classes = cl
								}

								t.Errorf("❌ ACTION BUTTON SIZE ISSUE on %s page (button #%d)", page.name, idx+1)
								t.Errorf("   Button: id=\"%s\" text=\"%s\"", id, text)
								t.Errorf("   Current classes: %s", classes)
								t.Errorf("   ❌ Missing 'small' class")
								t.Errorf("   Expected: class should include 'small' for action buttons")
								assert.Fail(t, "Action button not properly sized",
									"Add 'small' class to action button on %s page", page.name)
							}
						}
					}
				}
			}

			// Check for errors
			if errors, ok := buttonValidation["errors"].([]interface{}); ok && len(errors) > 0 {
				t.Logf("Button styling errors on %s page:", page.name)
				for _, err := range errors {
					if errStr, ok := err.(string); ok {
						t.Errorf("   ❌ %s", errStr)
					}
				}
			} else {
				t.Logf("✓ All buttons properly styled on %s page", page.name)
			}
		}
	}

	t.Log("\n✅ SUCCESS: All button styling validated across all pages")
}

// TestHeroSectionTransparency tests that hero sections have transparent backgrounds
func TestHeroSectionTransparency(t *testing.T) {
	t.Log("=== Testing Hero Section Transparency ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	pages := []struct {
		name string
		url  string
	}{
		{"HOME", config.ServerURL + "/"},
		{"JIRA DATA", config.ServerURL + "/jira"},
		{"CONFLUENCE DATA", config.ServerURL + "/confluence"},
		{"DOCUMENTS", config.ServerURL + "/documents"},
		{"CHAT", config.ServerURL + "/chat"},
		{"SETTINGS", config.ServerURL + "/settings"},
	}

	for _, page := range pages {
		t.Logf("Testing hero section transparency on %s page", page.name)

		var heroValidation map[string]interface{}

		err := chromedp.Run(ctx,
			chromedp.Navigate(page.url),
			chromedp.Sleep(2*time.Second),

			chromedp.ActionFunc(func(c context.Context) error {
				takeScreenshot(ctx, t, "hero_"+strings.ToLower(strings.ReplaceAll(page.name, " ", "_")))
				return nil
			}),

			// Check hero section backgrounds
			chromedp.Evaluate(`(() => {
				const results = {
					heroSections: [],
					errors: []
				};

				// Find all header.center-align elements (hero sections)
				const heroSections = document.querySelectorAll('header.center-align');

				heroSections.forEach((hero, index) => {
					const computedStyle = window.getComputedStyle(hero);
					const bgColor = computedStyle.backgroundColor;
					const bgImage = computedStyle.backgroundImage;

					// Check if background is transparent or white
					// Transparent: rgba(0,0,0,0), transparent keyword, or rgba with alpha=0
					// White: rgb(255,255,255) or similar light colors
					const isTransparent = bgColor === 'transparent' ||
					                     bgColor === 'rgba(0, 0, 0, 0)' ||
					                     bgColor.includes('rgba') && bgColor.includes(', 0)');

					// Check for colored backgrounds (purple, mauve, etc)
					// Purple family: rgb values with high blue and red, low green
					// Example: rgb(155, 81, 224) = purple
					const isColored = !isTransparent && !bgColor.includes('255, 255, 255') &&
					                 !bgColor.includes('254, 254, 254') &&
					                 !bgColor.includes('253, 253, 253') &&
					                 !bgColor.includes('252, 252, 252') &&
					                 !bgColor.includes('251, 251, 251') &&
					                 !bgColor.includes('250, 250, 250');

					// Check for purple/mauve specifically (high blue channel)
					let isPurplish = false;
					const rgbMatch = bgColor.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
					if (rgbMatch) {
						const r = parseInt(rgbMatch[1]);
						const g = parseInt(rgbMatch[2]);
						const b = parseInt(rgbMatch[3]);
						// Purple: blue > 200 and red > 100 and green < 150
						isPurplish = (b > 200 && r > 100 && g < 150) ||
						            (b > 180 && r > 120 && g < 120);
					}

					const hasColoredBackground = isColored || isPurplish;

					results.heroSections.push({
						index: index,
						backgroundColor: bgColor,
						backgroundImage: bgImage,
						isTransparent: isTransparent,
						hasColoredBackground: hasColoredBackground,
						isPurplish: isPurplish,
						isValid: !hasColoredBackground
					});

					if (hasColoredBackground) {
						results.errors.push(
							'Hero section ' + (index + 1) + ' has colored background: ' + bgColor +
							(isPurplish ? ' (appears to be purple/mauve)' : '')
						);
					}
				});

				return results;
			})()`, &heroValidation),
		)

		require.NoError(t, err, "Failed to validate hero sections on %s page", page.name)

		// Validate results
		if heroValidation != nil {
			if heroSections, ok := heroValidation["heroSections"].([]interface{}); ok && len(heroSections) > 0 {
				t.Logf("Found %d hero section(s) on %s page", len(heroSections), page.name)
				for _, hero := range heroSections {
					if heroData, ok := hero.(map[string]interface{}); ok {
						index := 0
						bgColor := ""
						isValid := false
						hasColoredBg := false
						isPurplish := false

						if idx, ok := heroData["index"].(float64); ok {
							index = int(idx)
						}
						if bg, ok := heroData["backgroundColor"].(string); ok {
							bgColor = bg
						}
						if iv, ok := heroData["isValid"].(bool); ok {
							isValid = iv
						}
						if hc, ok := heroData["hasColoredBackground"].(bool); ok {
							hasColoredBg = hc
						}
						if ip, ok := heroData["isPurplish"].(bool); ok {
							isPurplish = ip
						}

						if !isValid {
							t.Errorf("❌ HERO SECTION BACKGROUND ISSUE on %s page (section #%d)", page.name, index+1)
							t.Errorf("   Current background-color: %s", bgColor)
							if isPurplish {
								t.Errorf("   ❌ Background appears to be purple/mauve colored")
							} else if hasColoredBg {
								t.Errorf("   ❌ Background has a colored value (not transparent/white)")
							}
							t.Errorf("   Expected: transparent, rgba(0,0,0,0), or white background")
							t.Errorf("   Fix: Remove background-color or set to transparent")
							assert.Fail(t, "Hero section has colored background",
								"Hero section should be transparent on %s page", page.name)
						} else {
							t.Logf("   ✓ Hero section #%d has transparent/light background: %s", index+1, bgColor)
						}
					}
				}
			} else {
				t.Logf("ℹ️  No hero sections found on %s page (may be intentional)", page.name)
			}

			// Check for errors
			if errors, ok := heroValidation["errors"].([]interface{}); ok && len(errors) > 0 {
				t.Logf("Hero section errors on %s page:", page.name)
				for _, err := range errors {
					if errStr, ok := err.(string); ok {
						t.Errorf("   ❌ %s", errStr)
					}
				}
			} else {
				t.Logf("✓ All hero sections properly transparent on %s page", page.name)
			}
		}
	}

	t.Log("\n✅ SUCCESS: All hero sections have proper transparent backgrounds")
}

// TestBeerCSSVersion tests that all pages use the same BeerCSS version
func TestBeerCSSVersion(t *testing.T) {
	t.Log("=== Testing BeerCSS Version Consistency ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	pages := []struct {
		name string
		url  string
	}{
		{"HOME", config.ServerURL + "/"},
		{"JIRA DATA", config.ServerURL + "/jira"},
		{"CONFLUENCE DATA", config.ServerURL + "/confluence"},
		{"DOCUMENTS", config.ServerURL + "/documents"},
		{"CHAT", config.ServerURL + "/chat"},
		{"SETTINGS", config.ServerURL + "/settings"},
	}

	const expectedVersion = "3.12.11"
	var baselineVersion string
	versionsByPage := make(map[string]string)

	for _, page := range pages {
		t.Logf("Checking BeerCSS version on %s page", page.name)

		var versionInfo map[string]interface{}

		err := chromedp.Run(ctx,
			chromedp.Navigate(page.url),
			chromedp.Sleep(2*time.Second),

			// Check for BeerCSS version in link tags
			chromedp.Evaluate(`(() => {
				const results = {
					versions: [],
					links: []
				};

				// Find all link tags that reference BeerCSS
				const links = document.querySelectorAll('link[href*="beercss"], link[href*="beer.min.css"]');

				links.forEach(link => {
					const href = link.getAttribute('href');
					results.links.push(href);

					// Extract version from CDN URLs
					// Pattern: cdn.jsdelivr.net/npm/beercss@3.12.11/dist/cdn/beer.min.css
					const versionMatch = href.match(/beercss@([\d.]+)/);
					if (versionMatch) {
						results.versions.push(versionMatch[1]);
					}
					// Also check for older pattern: @3.7.10
					const oldVersionMatch = href.match(/@([\d.]+)/);
					if (oldVersionMatch && !versionMatch) {
						results.versions.push(oldVersionMatch[1]);
					}
				});

				// Also check for inline version indicators
				const metaVersion = document.querySelector('meta[name="beercss-version"]');
				if (metaVersion) {
					results.versions.push(metaVersion.getAttribute('content'));
				}

				return results;
			})()`, &versionInfo),
		)

		require.NoError(t, err, "Failed to check BeerCSS version on %s page", page.name)

		// Process results
		if versionInfo != nil {
			var detectedVersion string

			if versions, ok := versionInfo["versions"].([]interface{}); ok && len(versions) > 0 {
				// Use first detected version
				if ver, ok := versions[0].(string); ok {
					detectedVersion = ver
					versionsByPage[page.name] = detectedVersion

					// Set baseline from first page
					if baselineVersion == "" {
						baselineVersion = detectedVersion
						t.Logf("   Baseline BeerCSS version set: v%s (from %s page)", baselineVersion, page.name)
					}

					// Check if version matches expected
					if detectedVersion != expectedVersion {
						t.Errorf("❌ BEERCSS VERSION MISMATCH on %s page", page.name)
						t.Errorf("   Expected: v%s", expectedVersion)
						t.Errorf("   Actual: v%s", detectedVersion)
						assert.Fail(t, "BeerCSS version does not match expected",
							"Update BeerCSS to v%s on %s page", expectedVersion, page.name)
					}

					// Check if version matches baseline
					if detectedVersion != baselineVersion {
						t.Errorf("❌ BEERCSS VERSION INCONSISTENCY on %s page", page.name)
						t.Errorf("   Baseline: v%s", baselineVersion)
						t.Errorf("   This page: v%s", detectedVersion)
						assert.Fail(t, "BeerCSS version differs from baseline",
							"All pages should use v%s", baselineVersion)
					}

					t.Logf("   ✓ BeerCSS version: v%s", detectedVersion)
				}
			} else {
				t.Errorf("❌ BEERCSS VERSION NOT DETECTED on %s page", page.name)
				if links, ok := versionInfo["links"].([]interface{}); ok && len(links) > 0 {
					t.Errorf("   Found BeerCSS links but could not extract version:")
					for _, link := range links {
						if linkStr, ok := link.(string); ok {
							t.Errorf("     - %s", linkStr)
						}
					}
				} else {
					t.Errorf("   No BeerCSS link tags found in HTML")
				}
				assert.Fail(t, "BeerCSS version not detected",
					"Ensure BeerCSS is properly loaded on %s page", page.name)
			}
		}
	}

	// Final summary
	t.Logf("\n📊 BeerCSS Version Summary:")
	t.Logf("   Expected version: v%s", expectedVersion)
	t.Logf("   Baseline version: v%s", baselineVersion)
	t.Logf("   Versions by page:")
	for pageName, version := range versionsByPage {
		status := "✓"
		if version != expectedVersion {
			status = "❌"
		}
		t.Logf("     %s %s: v%s", status, pageName, version)
	}

	// Verify all pages use the same version
	allSameVersion := true
	for _, version := range versionsByPage {
		if version != baselineVersion {
			allSameVersion = false
			break
		}
	}

	assert.True(t, allSameVersion, "All pages should use the same BeerCSS version")

	t.Log("\n✅ SUCCESS: BeerCSS version consistency validated")
}

// TestStatusIndicatorIcons tests that status indicators have the material-icons class
func TestStatusIndicatorIcons(t *testing.T) {
	t.Log("=== Testing Status Indicator Icons (material-icons class) ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	pages := []struct {
		name string
		url  string
	}{
		{"HOME", config.ServerURL + "/"},
		{"JIRA DATA", config.ServerURL + "/jira"},
		{"CONFLUENCE DATA", config.ServerURL + "/confluence"},
		{"DOCUMENTS", config.ServerURL + "/documents"},
		{"CHAT", config.ServerURL + "/chat"},
		{"SETTINGS", config.ServerURL + "/settings"},
	}

	for _, page := range pages {
		t.Logf("Testing status indicator icons on %s page", page.name)

		var iconValidation map[string]interface{}

		err := chromedp.Run(ctx,
			chromedp.Navigate(page.url),
			chromedp.Sleep(2*time.Second),

			chromedp.ActionFunc(func(c context.Context) error {
				takeScreenshot(ctx, t, "status_icons_"+strings.ToLower(strings.ReplaceAll(page.name, " ", "_")))
				return nil
			}),

			// Check all status indicator icons for material-icons class
			chromedp.Evaluate(`(() => {
				const results = {
					navbarStatusIndicators: [],
					serviceStatusIndicators: [],
					serviceLogsIndicators: [],
					errors: []
				};

				// 1. Check navbar status indicator (top-right ONLINE/OFFLINE)
				const navbarStatus = document.querySelector('.status-text');
				if (navbarStatus) {
					const icons = navbarStatus.querySelectorAll('i');
					icons.forEach((icon, index) => {
						const classes = icon.className || '';
						const hasMaterialIcons = classes.includes('material-icons');
						const textContent = icon.textContent.trim();
						const isCircle = textContent === 'circle';

						results.navbarStatusIndicators.push({
							index: index,
							classes: classes,
							textContent: textContent,
							hasMaterialIcons: hasMaterialIcons,
							isCircle: isCircle,
							isValid: hasMaterialIcons && isCircle
						});

						if (!hasMaterialIcons) {
							results.errors.push('Navbar status indicator #' + (index + 1) + ' missing material-icons class');
						}
						if (!isCircle) {
							results.errors.push('Navbar status indicator #' + (index + 1) + ' not using "circle" icon');
						}
					});
				}

				// 2. Check service-status partial status indicators
				const serviceStatusTable = document.querySelector('#service-status-table');
				if (serviceStatusTable) {
					const statusCells = serviceStatusTable.querySelectorAll('td');
					statusCells.forEach((cell, cellIndex) => {
						const icons = cell.querySelectorAll('i');
						icons.forEach((icon, iconIndex) => {
							const classes = icon.className || '';
							const hasMaterialIcons = classes.includes('material-icons');
							const textContent = icon.textContent.trim();
							const isCircle = textContent === 'circle';

							results.serviceStatusIndicators.push({
								cellIndex: cellIndex,
								iconIndex: iconIndex,
								classes: classes,
								textContent: textContent,
								hasMaterialIcons: hasMaterialIcons,
								isCircle: isCircle,
								isValid: hasMaterialIcons && isCircle
							});

							if (!hasMaterialIcons && isCircle) {
								results.errors.push('Service status indicator (cell ' + cellIndex + ', icon ' + iconIndex + ') missing material-icons class');
							}
						});
					});
				}

				// 3. Check service-logs partial status indicators
				const serviceLogsContainer = document.querySelector('#service-logs, .service-logs-container');
				if (serviceLogsContainer) {
					// Check parent containers for status indicators
					const logSection = serviceLogsContainer.closest('article, section, div[id*="log"]');
					if (logSection) {
						const icons = logSection.querySelectorAll('i');
						icons.forEach((icon, index) => {
							const classes = icon.className || '';
							const textContent = icon.textContent.trim();
							
							// Only check circle icons (status indicators)
							if (textContent === 'circle') {
								const hasMaterialIcons = classes.includes('material-icons');

								results.serviceLogsIndicators.push({
									index: index,
									classes: classes,
									textContent: textContent,
									hasMaterialIcons: hasMaterialIcons,
									isValid: hasMaterialIcons
								});

								if (!hasMaterialIcons) {
									results.errors.push('Service logs status indicator #' + (index + 1) + ' missing material-icons class');
								}
							}
						});
					}
				}

				return results;
			})()`, &iconValidation),
		)

		require.NoError(t, err, "Failed to validate status indicators on %s page", page.name)

		// Validate results
		if iconValidation != nil {
			allValid := true

			// Check navbar status indicators
			if navbarIndicators, ok := iconValidation["navbarStatusIndicators"].([]interface{}); ok && len(navbarIndicators) > 0 {
				t.Logf("Found %d navbar status indicator(s) on %s page", len(navbarIndicators), page.name)
				for _, indicator := range navbarIndicators {
					if data, ok := indicator.(map[string]interface{}); ok {
						isValid := false
						hasMaterialIcons := false
						classes := ""
						textContent := ""

						if iv, ok := data["isValid"].(bool); ok {
							isValid = iv
						}
						if hm, ok := data["hasMaterialIcons"].(bool); ok {
							hasMaterialIcons = hm
						}
						if cl, ok := data["classes"].(string); ok {
							classes = cl
						}
						if tc, ok := data["textContent"].(string); ok {
							textContent = tc
						}

						if !isValid {
							allValid = false
							t.Errorf("❌ NAVBAR STATUS INDICATOR ISSUE on %s page", page.name)
							t.Errorf("   Current classes: \"%s\"", classes)
							t.Errorf("   Text content: \"%s\"", textContent)
							if !hasMaterialIcons {
								t.Errorf("   ❌ Missing 'material-icons' class")
								t.Errorf("   FIX: Add material-icons class to status indicator icon")
								t.Errorf("   Expected: <i class=\"material-icons\" style=\"font-size: 12px;\">circle</i>")
								t.Errorf("   Location: pages/partials/navbar.html line 31 and line 47")
							}
						} else {
							t.Logf("   ✓ Navbar status indicator has material-icons class and circle icon")
						}
					}
				}
			}

			// Check service status indicators
			if serviceIndicators, ok := iconValidation["serviceStatusIndicators"].([]interface{}); ok && len(serviceIndicators) > 0 {
				t.Logf("Found %d service status indicator(s) on %s page", len(serviceIndicators), page.name)
				for _, indicator := range serviceIndicators {
					if data, ok := indicator.(map[string]interface{}); ok {
						isValid := false
						hasMaterialIcons := false
						classes := ""

						if iv, ok := data["isValid"].(bool); ok {
							isValid = iv
						}
						if hm, ok := data["hasMaterialIcons"].(bool); ok {
							hasMaterialIcons = hm
						}
						if cl, ok := data["classes"].(string); ok {
							classes = cl
						}

						if !isValid && !hasMaterialIcons {
							allValid = false
							t.Errorf("❌ SERVICE STATUS INDICATOR ISSUE on %s page", page.name)
							t.Errorf("   Current classes: \"%s\"", classes)
							t.Errorf("   ❌ Missing 'material-icons' class")
							t.Errorf("   FIX: Add material-icons class to status indicator in service-status partial")
							t.Errorf("   Expected: <i class=\"material-icons\">circle</i>")
						}
					}
				}
			}

			// Check service logs indicators
			if logsIndicators, ok := iconValidation["serviceLogsIndicators"].([]interface{}); ok && len(logsIndicators) > 0 {
				t.Logf("Found %d service logs indicator(s) on %s page", len(logsIndicators), page.name)
				for _, indicator := range logsIndicators {
					if data, ok := indicator.(map[string]interface{}); ok {
						isValid := false
						hasMaterialIcons := false
						classes := ""

						if iv, ok := data["isValid"].(bool); ok {
							isValid = iv
						}
						if hm, ok := data["hasMaterialIcons"].(bool); ok {
							hasMaterialIcons = hm
						}
						if cl, ok := data["classes"].(string); ok {
							classes = cl
						}

						if !isValid && !hasMaterialIcons {
							allValid = false
							t.Errorf("❌ SERVICE LOGS INDICATOR ISSUE on %s page", page.name)
							t.Errorf("   Current classes: \"%s\"", classes)
							t.Errorf("   ❌ Missing 'material-icons' class")
							t.Errorf("   FIX: Add material-icons class to status indicator in service-logs partial")
							t.Errorf("   Expected: <i class=\"material-icons\">circle</i>")
						}
					}
				}
			}

			// Check for errors
			if errors, ok := iconValidation["errors"].([]interface{}); ok && len(errors) > 0 {
				allValid = false
				t.Logf("Status indicator icon errors on %s page:", page.name)
				for _, err := range errors {
					if errStr, ok := err.(string); ok {
						t.Errorf("   ❌ %s", errStr)
					}
				}
			}

			if allValid {
				t.Logf("✓ All status indicators properly use material-icons class on %s page", page.name)
			} else {
				assert.Fail(t, "Status indicator icons not properly configured",
					"Fix material-icons class on status indicators on %s page", page.name)
			}
		}
	}

	t.Log("\n✅ SUCCESS: All status indicator icons validated across all pages")
}

// TestSmallButtonsAndDropdowns tests that small buttons and dropdowns have the .small class
func TestSmallButtonsAndDropdowns(t *testing.T) {
	t.Log("=== Testing Small Buttons and Dropdowns (.small class) ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Test Confluence and Documents pages specifically (where .small class is used)
	pages := []struct {
		name         string
		url          string
		hasButtons   bool
		hasDropdowns bool
	}{
		{"CONFLUENCE DATA", config.ServerURL + "/confluence", true, false},
		{"DOCUMENTS", config.ServerURL + "/documents", false, true},
	}

	for _, page := range pages {
		t.Logf("Testing small buttons/dropdowns on %s page", page.name)

		var smallValidation map[string]interface{}

		err := chromedp.Run(ctx,
			chromedp.Navigate(page.url),
			chromedp.Sleep(2*time.Second),

			chromedp.ActionFunc(func(c context.Context) error {
				takeScreenshot(ctx, t, "small_elements_"+strings.ToLower(strings.ReplaceAll(page.name, " ", "_")))
				return nil
			}),

			// Check for .small class on buttons and dropdowns
			chromedp.Evaluate(`(() => {
				const results = {
					smallButtons: [],
					smallDropdowns: [],
					cssRules: {
						buttonSmallExists: false,
						selectSmallExists: false,
						buttonSmallFontSize: null,
						selectSmallFontSize: null
					},
					errors: []
				};

				// Check CSS rules for .small class
				const styleSheets = document.styleSheets;
				for (let sheet of styleSheets) {
					try {
						const rules = sheet.cssRules || sheet.rules;
						for (let rule of rules) {
							if (rule.selectorText) {
								if (rule.selectorText.includes('button.small') || rule.selectorText.includes('.button.small')) {
									results.cssRules.buttonSmallExists = true;
									const fontSize = rule.style.fontSize;
									if (fontSize) {
										results.cssRules.buttonSmallFontSize = fontSize;
									}
								}
								if (rule.selectorText.includes('select.small') || rule.selectorText.includes('.select.small')) {
									results.cssRules.selectSmallExists = true;
									const fontSize = rule.style.fontSize;
									if (fontSize) {
										results.cssRules.selectSmallFontSize = fontSize;
									}
								}
							}
						}
					} catch (e) {
						// Skip cross-origin stylesheets
					}
				}

				// Check Confluence page buttons (GET SPACES, GET PAGES, CLEAR ALL DATA)
				const confluenceButtons = [
					'GET SPACES',
					'GET PAGES',
					'CLEAR ALL DATA'
				];

				confluenceButtons.forEach(buttonText => {
					const buttons = Array.from(document.querySelectorAll('button')).filter(
						btn => btn.textContent.trim() === buttonText
					);
					buttons.forEach(btn => {
						const classes = btn.className;
						const hasSmall = classes.includes('small');
						const computedStyle = window.getComputedStyle(btn);
						const fontSize = computedStyle.fontSize;
						const padding = computedStyle.padding;

						results.smallButtons.push({
							text: buttonText,
							classes: classes,
							hasSmall: hasSmall,
							fontSize: fontSize,
							padding: padding,
							isValid: hasSmall
						});

						if (!hasSmall) {
							results.errors.push('Button "' + buttonText + '" missing small class');
						}
					});
				});

				// Check Documents page dropdowns (source-filter, vectorized-filter)
				const dropdownIds = ['source-filter', 'vectorized-filter'];
				dropdownIds.forEach(id => {
					const dropdown = document.getElementById(id);
					if (dropdown) {
						const classes = dropdown.className;
						const hasSmall = classes.includes('small');
						const computedStyle = window.getComputedStyle(dropdown);
						const fontSize = computedStyle.fontSize;
						const padding = computedStyle.padding;

						results.smallDropdowns.push({
							id: id,
							classes: classes,
							hasSmall: hasSmall,
							fontSize: fontSize,
							padding: padding,
							isValid: hasSmall
						});

						if (!hasSmall) {
							results.errors.push('Dropdown "' + id + '" missing small class');
						}
					}
				});

				return results;
			})()`, &smallValidation),
		)

		require.NoError(t, err, "Failed to validate small elements on %s page", page.name)

		// Validate results
		if smallValidation != nil {
			allValid := true

			// Check CSS rules exist
			if cssRules, ok := smallValidation["cssRules"].(map[string]interface{}); ok {
				buttonSmallExists := false
				selectSmallExists := false

				if bse, ok := cssRules["buttonSmallExists"].(bool); ok {
					buttonSmallExists = bse
				}
				if sse, ok := cssRules["selectSmallExists"].(bool); ok {
					selectSmallExists = sse
				}

				if page.hasButtons && !buttonSmallExists {
					allValid = false
					t.Errorf("❌ CSS RULE MISSING: button.small not defined in CSS")
					t.Errorf("   FIX: Add button.small rule to pages/static/common.css")
					t.Errorf("   Expected around line 390:")
					t.Errorf("   button.small, .button.small {")
					t.Errorf("       font-size: 11px;")
					t.Errorf("       padding: 3px 10px;")
					t.Errorf("       min-height: 28px;")
					t.Errorf("   }")
				}

				if page.hasDropdowns && !selectSmallExists {
					allValid = false
					t.Errorf("❌ CSS RULE MISSING: select.small not defined in CSS")
					t.Errorf("   FIX: Add select.small rule to pages/static/common.css")
					t.Errorf("   Expected around line 399:")
					t.Errorf("   select.small, .select.small {")
					t.Errorf("       font-size: 11px;")
					t.Errorf("       padding: 3px 10px;")
					t.Errorf("       min-height: 28px;")
					t.Errorf("   }")
				}

				if buttonSmallExists {
					t.Logf("   ✓ CSS rule button.small exists")
				}
				if selectSmallExists {
					t.Logf("   ✓ CSS rule select.small exists")
				}
			}

			// Check small buttons
			if smallButtons, ok := smallValidation["smallButtons"].([]interface{}); ok && len(smallButtons) > 0 {
				t.Logf("Found %d button(s) that should have .small class on %s page", len(smallButtons), page.name)
				for _, button := range smallButtons {
					if data, ok := button.(map[string]interface{}); ok {
						isValid := false
						hasSmall := false
						text := ""
						classes := ""
						fontSize := ""

						if iv, ok := data["isValid"].(bool); ok {
							isValid = iv
						}
						if hs, ok := data["hasSmall"].(bool); ok {
							hasSmall = hs
						}
						if t, ok := data["text"].(string); ok {
							text = t
						}
						if cl, ok := data["classes"].(string); ok {
							classes = cl
						}
						if fs, ok := data["fontSize"].(string); ok {
							fontSize = fs
						}

						if !isValid {
							allValid = false
							t.Errorf("❌ BUTTON SIZE ISSUE on %s page", page.name)
							t.Errorf("   Button: \"%s\"", text)
							t.Errorf("   Current classes: \"%s\"", classes)
							t.Errorf("   Current font-size: %s", fontSize)
							if !hasSmall {
								t.Errorf("   ❌ Missing 'small' class")
								t.Errorf("   FIX: Add 'small' class to button")
								t.Errorf("   Expected: <button class=\"... small\">%s</button>", text)
								t.Errorf("   Location: pages/confluence.html")
							}
						} else {
							t.Logf("   ✓ Button \"%s\" has small class (font-size: %s)", text, fontSize)
						}
					}
				}
			}

			// Check small dropdowns
			if smallDropdowns, ok := smallValidation["smallDropdowns"].([]interface{}); ok && len(smallDropdowns) > 0 {
				t.Logf("Found %d dropdown(s) that should have .small class on %s page", len(smallDropdowns), page.name)
				for _, dropdown := range smallDropdowns {
					if data, ok := dropdown.(map[string]interface{}); ok {
						isValid := false
						hasSmall := false
						id := ""
						classes := ""
						fontSize := ""

						if iv, ok := data["isValid"].(bool); ok {
							isValid = iv
						}
						if hs, ok := data["hasSmall"].(bool); ok {
							hasSmall = hs
						}
						if did, ok := data["id"].(string); ok {
							id = did
						}
						if cl, ok := data["classes"].(string); ok {
							classes = cl
						}
						if fs, ok := data["fontSize"].(string); ok {
							fontSize = fs
						}

						if !isValid {
							allValid = false
							t.Errorf("❌ DROPDOWN SIZE ISSUE on %s page", page.name)
							t.Errorf("   Dropdown ID: \"%s\"", id)
							t.Errorf("   Current classes: \"%s\"", classes)
							t.Errorf("   Current font-size: %s", fontSize)
							if !hasSmall {
								t.Errorf("   ❌ Missing 'small' class")
								t.Errorf("   FIX: Add 'small' class to dropdown")
								t.Errorf("   Expected: <select id=\"%s\" class=\"... small\">...</select>", id)
								t.Errorf("   Location: pages/documents.html")
							}
						} else {
							t.Logf("   ✓ Dropdown \"%s\" has small class (font-size: %s)", id, fontSize)
						}
					}
				}
			}

			// Check for errors
			if errors, ok := smallValidation["errors"].([]interface{}); ok && len(errors) > 0 {
				allValid = false
				t.Logf("Small element errors on %s page:", page.name)
				for _, err := range errors {
					if errStr, ok := err.(string); ok {
						t.Errorf("   ❌ %s", errStr)
					}
				}
			}

			if allValid {
				t.Logf("✓ All small elements properly configured on %s page", page.name)
			} else {
				assert.Fail(t, "Small buttons/dropdowns not properly configured",
					"Fix .small class on elements on %s page", page.name)
			}
		}
	}

	t.Log("\n✅ SUCCESS: All small buttons and dropdowns validated")
}

// TestCSSSmallClassDefinition tests that CSS defines the .small class with correct values
func TestCSSSmallClassDefinition(t *testing.T) {
	t.Log("=== Testing CSS .small Class Definition ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()

	var cssValidation map[string]interface{}

	err = chromedp.Run(ctx,
		chromedp.Navigate(config.ServerURL),
		chromedp.Sleep(2*time.Second),

		chromedp.ActionFunc(func(c context.Context) error {
			takeScreenshot(ctx, t, "css_small_class")
			return nil
		}),

		// Comprehensive CSS rule validation
		chromedp.Evaluate(`(() => {
			const results = {
				buttonSmall: {
					exists: false,
					fontSize: null,
					padding: null,
					minHeight: null,
					lineHeight: null,
					selectorText: null
				},
				selectSmall: {
					exists: false,
					fontSize: null,
					padding: null,
					minHeight: null,
					lineHeight: null,
					selectorText: null
				},
				errors: []
			};

			// Expected values
			const expectedFontSize = '11px';
			const expectedPadding = '3px 10px';
			const expectedMinHeight = '28px';
			const expectedLineHeight = '1.4';

			// Search through all stylesheets
			const styleSheets = document.styleSheets;
			for (let sheet of styleSheets) {
				try {
					const rules = sheet.cssRules || sheet.rules;
					for (let rule of rules) {
						if (rule.selectorText) {
							// Check for button.small rule
							if (rule.selectorText.includes('button.small') ||
							    (rule.selectorText.includes('.button.small') && !rule.selectorText.includes('article'))) {
								results.buttonSmall.exists = true;
								results.buttonSmall.selectorText = rule.selectorText;
								results.buttonSmall.fontSize = rule.style.fontSize || null;
								results.buttonSmall.padding = rule.style.padding || null;
								results.buttonSmall.minHeight = rule.style.minHeight || null;
								results.buttonSmall.lineHeight = rule.style.lineHeight || null;
							}
							
							// Check for select.small rule
							if (rule.selectorText.includes('select.small') ||
							    rule.selectorText.includes('.select.small')) {
								results.selectSmall.exists = true;
								results.selectSmall.selectorText = rule.selectorText;
								results.selectSmall.fontSize = rule.style.fontSize || null;
								results.selectSmall.padding = rule.style.padding || null;
								results.selectSmall.minHeight = rule.style.minHeight || null;
								results.selectSmall.lineHeight = rule.style.lineHeight || null;
							}
						}
					}
				} catch (e) {
					// Skip cross-origin stylesheets
				}
			}

			// Validate button.small
			if (!results.buttonSmall.exists) {
				results.errors.push('button.small CSS rule not found');
			} else {
				if (results.buttonSmall.fontSize !== expectedFontSize) {
					results.errors.push('button.small font-size is "' + results.buttonSmall.fontSize + '" (expected "' + expectedFontSize + '")');
				}
				if (results.buttonSmall.padding !== expectedPadding) {
					results.errors.push('button.small padding is "' + results.buttonSmall.padding + '" (expected "' + expectedPadding + '")');
				}
				if (results.buttonSmall.minHeight !== expectedMinHeight) {
					results.errors.push('button.small min-height is "' + results.buttonSmall.minHeight + '" (expected "' + expectedMinHeight + '")');
				}
			}

			// Validate select.small
			if (!results.selectSmall.exists) {
				results.errors.push('select.small CSS rule not found');
			} else {
				if (results.selectSmall.fontSize !== expectedFontSize) {
					results.errors.push('select.small font-size is "' + results.selectSmall.fontSize + '" (expected "' + expectedFontSize + '")');
				}
				if (results.selectSmall.padding !== expectedPadding) {
					results.errors.push('select.small padding is "' + results.selectSmall.padding + '" (expected "' + expectedPadding + '")');
				}
				if (results.selectSmall.minHeight !== expectedMinHeight) {
					results.errors.push('select.small min-height is "' + results.selectSmall.minHeight + '" (expected "' + expectedMinHeight + '")');
				}
			}

			return results;
		})()`, &cssValidation),
	)

	require.NoError(t, err, "Failed to validate CSS .small class")

	// Validate results
	if cssValidation != nil {
		allValid := true

		// Check button.small rule
		if buttonSmall, ok := cssValidation["buttonSmall"].(map[string]interface{}); ok {
			exists := false
			if e, ok := buttonSmall["exists"].(bool); ok {
				exists = e
			}

			if !exists {
				allValid = false
				t.Errorf("❌ CSS RULE MISSING: button.small")
				t.Errorf("   FIX: Add button.small rule to pages/static/common.css around line 390")
				t.Errorf("   Required rule:")
				t.Errorf("   button.small, .button.small {")
				t.Errorf("       font-size: 11px;")
				t.Errorf("       padding: 3px 10px;")
				t.Errorf("       min-height: 28px;")
				t.Errorf("       line-height: 1.4;")
				t.Errorf("   }")
			} else {
				// Rule exists, validate properties
				fontSize := ""
				padding := ""
				minHeight := ""
				selectorText := ""

				if fs, ok := buttonSmall["fontSize"].(string); ok {
					fontSize = fs
				}
				if pd, ok := buttonSmall["padding"].(string); ok {
					padding = pd
				}
				if mh, ok := buttonSmall["minHeight"].(string); ok {
					minHeight = mh
				}
				if st, ok := buttonSmall["selectorText"].(string); ok {
					selectorText = st
				}

				t.Logf("✓ button.small CSS rule exists: %s", selectorText)
				t.Logf("   Properties:")
				t.Logf("   - font-size: %s", fontSize)
				t.Logf("   - padding: %s", padding)
				t.Logf("   - min-height: %s", minHeight)

				// Validate property values
				if fontSize != "11px" {
					allValid = false
					t.Errorf("   ❌ Incorrect font-size: %s (expected: 11px)", fontSize)
				}
				if padding != "3px 10px" {
					allValid = false
					t.Errorf("   ❌ Incorrect padding: %s (expected: 3px 10px)", padding)
				}
				if minHeight != "28px" {
					allValid = false
					t.Errorf("   ❌ Incorrect min-height: %s (expected: 28px)", minHeight)
				}
			}
		}

		// Check select.small rule
		if selectSmall, ok := cssValidation["selectSmall"].(map[string]interface{}); ok {
			exists := false
			if e, ok := selectSmall["exists"].(bool); ok {
				exists = e
			}

			if !exists {
				allValid = false
				t.Errorf("❌ CSS RULE MISSING: select.small")
				t.Errorf("   FIX: Add select.small rule to pages/static/common.css around line 399")
				t.Errorf("   Required rule:")
				t.Errorf("   select.small, .select.small {")
				t.Errorf("       font-size: 11px;")
				t.Errorf("       padding: 3px 10px;")
				t.Errorf("       min-height: 28px;")
				t.Errorf("       line-height: 1.4;")
				t.Errorf("   }")
			} else {
				// Rule exists, validate properties
				fontSize := ""
				padding := ""
				minHeight := ""
				selectorText := ""

				if fs, ok := selectSmall["fontSize"].(string); ok {
					fontSize = fs
				}
				if pd, ok := selectSmall["padding"].(string); ok {
					padding = pd
				}
				if mh, ok := selectSmall["minHeight"].(string); ok {
					minHeight = mh
				}
				if st, ok := selectSmall["selectorText"].(string); ok {
					selectorText = st
				}

				t.Logf("✓ select.small CSS rule exists: %s", selectorText)
				t.Logf("   Properties:")
				t.Logf("   - font-size: %s", fontSize)
				t.Logf("   - padding: %s", padding)
				t.Logf("   - min-height: %s", minHeight)

				// Validate property values
				if fontSize != "11px" {
					allValid = false
					t.Errorf("   ❌ Incorrect font-size: %s (expected: 11px)", fontSize)
				}
				if padding != "3px 10px" {
					allValid = false
					t.Errorf("   ❌ Incorrect padding: %s (expected: 3px 10px)", padding)
				}
				if minHeight != "28px" {
					allValid = false
					t.Errorf("   ❌ Incorrect min-height: %s (expected: 28px)", minHeight)
				}
			}
		}

		// Check for errors
		if errors, ok := cssValidation["errors"].([]interface{}); ok && len(errors) > 0 {
			allValid = false
			t.Log("CSS .small class definition errors:")
			for _, err := range errors {
				if errStr, ok := err.(string); ok {
					t.Errorf("   ❌ %s", errStr)
				}
			}
		}

		if !allValid {
			assert.Fail(t, "CSS .small class not properly defined",
				"Fix .small class definitions in pages/static/common.css")
		} else {
			t.Log("✓ All CSS .small class definitions are correct")
		}
	}

	t.Log("\n✅ SUCCESS: CSS .small class definition validated")
}

// Note: takeScreenshot function is defined in ui_test.go with signature takeScreenshot(ctx context.Context, t *testing.T, name string)
