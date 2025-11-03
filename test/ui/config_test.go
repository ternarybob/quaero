package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestConfigPageLoad(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestConfigPageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/config"
	var title string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load config page: %v", err)
	}

	// Take screenshot of config page
	if err := TakeScreenshot(ctx, "config-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Quaero - Configuration"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}

	t.Log("✓ Config page loads correctly")
}

func TestConfigHeroSection(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestConfigHeroSection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/config"

	var heroVisible bool
	var heroTitle string
	var heroSubtitle string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`section.hero`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('section.hero') !== null`, &heroVisible),
		chromedp.Text(`section.hero .title`, &heroTitle, chromedp.ByQuery),
		chromedp.Text(`section.hero .subtitle`, &heroSubtitle, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to check hero section: %v", err)
	}

	if !heroVisible {
		t.Error("Hero section not found on page")
	}

	if !strings.Contains(heroTitle, "Configuration") {
		t.Errorf("Expected hero title to contain 'Configuration', got: %s", heroTitle)
	}

	if heroSubtitle == "" {
		t.Error("Hero subtitle is empty")
	}

	t.Log("✓ Hero section displays correctly")
}

func TestConfigNavbar(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestConfigNavbar")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/config"

	// Check navbar and all menu items
	var navbarVisible bool
	var menuItems []string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`nav.navbar`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('nav.navbar') !== null`, &navbarVisible),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('.navbar-item')).map(el => el.textContent.trim())`, &menuItems),
	)

	if err != nil {
		t.Fatalf("Failed to check navbar: %v", err)
	}

	if !navbarVisible {
		t.Error("Navbar not found on page")
	}

	// Check for expected menu items
	expectedItems := []string{"Quaero", "HOME", "SOURCES", "JOBS", "DOCUMENTS", "CHAT", "SETTINGS"}
	for _, expected := range expectedItems {
		found := false
		for _, item := range menuItems {
			if strings.Contains(item, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Menu item '%s' not found in navbar. Found: %v", expected, menuItems)
		}
	}

	t.Log("✓ Navbar displays correct menu items")
}

func TestConfigServiceStatus(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestConfigServiceStatus")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/config"

	// Wait for Alpine.js to initialize and load config
	var serviceStatus string
	var version string
	var build string
	var port string
	var host string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#service-status`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for fetch to complete
		chromedp.Text(`#service-status`, &serviceStatus, chromedp.ByQuery),
		chromedp.Text(`#config-version`, &version, chromedp.ByQuery),
		chromedp.Text(`#config-build`, &build, chromedp.ByQuery),
		chromedp.Text(`#config-port`, &port, chromedp.ByQuery),
		chromedp.Text(`#config-host`, &host, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to check service status: %v", err)
	}

	// Check service is online
	if !strings.Contains(serviceStatus, "Online") {
		t.Errorf("Expected service status to contain 'Online', got: %s", serviceStatus)
	}

	// Check version is displayed (may be "unknown" if .version file not found)
	if version == "" {
		t.Error("Version field is empty")
	}
	t.Logf("Version: %s", version)

	// Check build is displayed
	if build == "" {
		t.Error("Build field is empty")
	}
	t.Logf("Build: %s", build)

	// Check port matches expected (test environment uses port 18085)
	if !strings.Contains(port, "18085") {
		t.Errorf("Expected port to contain '18085', got: %s", port)
	}
	t.Logf("Port: %s", port)

	// Check host is displayed
	if host == "" {
		t.Error("Host field is empty")
	}
	t.Logf("Host: %s", host)

	t.Log("✓ Service status displays correctly")
}

func TestConfigDisplay(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestConfigDisplay")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/config"

	// Check that config is displayed
	var configText string
	var configVisible bool

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#config-display`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for fetch to complete
		chromedp.Text(`#config-display`, &configText, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('#config-display') !== null`, &configVisible),
	)

	if err != nil {
		t.Fatalf("Failed to check config display: %v", err)
	}

	if !configVisible {
		t.Error("Config display element not found")
	}

	// Check config contains expected fields
	expectedFields := []string{"Server", "Port", "LLM", "Storage"}
	for _, field := range expectedFields {
		if !strings.Contains(configText, field) {
			t.Errorf("Config display missing expected field '%s'", field)
		}
	}

	// Take screenshot of config display
	if err := TakeScreenshot(ctx, "config-display"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Log("✓ Config display shows highlighted configuration")
}

func TestConfigServiceLogs(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestConfigServiceLogs")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/config"

	var logsVisible bool
	var logsHeaderText string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.log-container`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('.log-container') !== null`, &logsVisible),
		chromedp.Text(`.card-header-title`, &logsHeaderText, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to check service logs: %v", err)
	}

	if !logsVisible {
		t.Error("Service logs container not found")
	}

	// Take screenshot of service logs
	if err := TakeScreenshot(ctx, "config-logs"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Log("✓ Service logs section displays correctly")
}

func TestConfigFooter(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestConfigFooter")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/config"

	var footerVisible bool
	var footerText string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`footer.footer`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for version fetch
		chromedp.Evaluate(`document.querySelector('footer.footer') !== null`, &footerVisible),
		chromedp.Text(`footer.footer`, &footerText, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to check footer: %v", err)
	}

	if !footerVisible {
		t.Error("Footer not found on page")
	}

	// Check footer contains "Quaero"
	if !strings.Contains(footerText, "Quaero") {
		t.Errorf("Expected footer to contain 'Quaero', got: %s", footerText)
	}

	t.Logf("Footer text: %s", footerText)
	t.Log("✓ Footer displays correctly")
}
