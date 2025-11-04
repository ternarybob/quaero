// -----------------------------------------------------------------------
// Shared test framework for both UI and API tests
// Last Modified: Tuesday, 4th November 2025 4:30:00 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pelletier/go-toml/v2"
)

// TestMainOutput captures the TestMain output for later inclusion in test logs
var TestMainOutput bytes.Buffer

// suiteDirectories tracks parent directories for test suites
// Maps suite name (e.g., "TestSources") to parent directory path
var suiteDirectories = make(map[string]string)
var suiteDirectoriesMutex sync.Mutex

// OutputCapture captures stdout/stderr and tees it to a file and original output
type OutputCapture struct {
	buffer       *bytes.Buffer
	originalOut  *os.File
	originalErr  *os.File
	reader       *os.File
	writer       *os.File
	wg           sync.WaitGroup
	testLog      *os.File
	capturing    bool
	captureMutex sync.Mutex
}

// TestConfig holds the UI test configuration
type TestConfig struct {
	Build struct {
		SourceDir    string `toml:"source_dir"`
		BinaryOutput string `toml:"binary_output"`
		ConfigFile   string `toml:"config_file"`
		VersionFile  string `toml:"version_file"`
	} `toml:"build"`
	Service struct {
		StartupTimeoutSeconds int    `toml:"startup_timeout_seconds"`
		Port                  int    `toml:"port"`
		Host                  string `toml:"host"`
		ShutdownEndpoint      string `toml:"shutdown_endpoint"`
	} `toml:"service"`
	Output struct {
		ResultsBaseDir string `toml:"results_base_dir"`
	} `toml:"output"`
}

// TestEnvironment represents a running test environment
type TestEnvironment struct {
	Config     *TestConfig
	Cmd        *exec.Cmd
	ResultsDir string
	LogFile    *os.File // Service log output
	TestLog    *os.File // Test execution log
	Port       int

	// Output capture for test console
	outputCapture *OutputCapture
}

// extractSuiteName extracts the test suite name from a test name
// Derives lowercase suite name matching test file naming convention
// Example: "TestHomepageLoad" -> "homepage" (from homepage_test.go)
//          "HomepageTitle" -> "homepage" (from homepage_test.go)
//          "TestSourcesPageLoad" -> "sources" (from sources_test.go)
//          "TestJobsCreateModal" -> "jobs" (from jobs_test.go)
func extractSuiteName(testName string) string {
	// Remove "Test" prefix if present
	remainder := testName
	if strings.HasPrefix(testName, "Test") {
		remainder = testName[4:]
	}

	// Find all capital letter positions
	var capitals []int
	for i := 0; i < len(remainder); i++ {
		if remainder[i] >= 'A' && remainder[i] <= 'Z' {
			capitals = append(capitals, i)
		}
	}

	// If we have at least 2 capitals, take everything up to the second one
	// Example: "HomepageTitle" has capitals at [0, 8]
	//          We want "homepage" (lowercase, up to index 8)
	// Example: "SourcesPageLoad" has capitals at [0, 7, 11]
	//          We want "sources" (lowercase, up to index 7)
	if len(capitals) >= 2 {
		return strings.ToLower(remainder[:capitals[1]])
	}

	// If only one capital or none, return the lowercase name
	return strings.ToLower(remainder)
}

// getOrCreateSuiteDirectory gets or creates a parent directory for a test suite
// Returns the suite parent directory path
func getOrCreateSuiteDirectory(suiteName string, baseDir string) (string, error) {
	suiteDirectoriesMutex.Lock()
	defer suiteDirectoriesMutex.Unlock()

	// Check if we already have a directory for this suite
	if existingDir, ok := suiteDirectories[suiteName]; ok {
		return existingDir, nil
	}

	// Create new parent directory for this suite
	timestamp := time.Now().Format("20060102-150405")
	suiteDir := filepath.Join(baseDir, fmt.Sprintf("%s-%s", suiteName, timestamp))

	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create suite directory: %w", err)
	}

	// Store for future tests in this suite
	suiteDirectories[suiteName] = suiteDir

	return suiteDir, nil
}

// LoadTestConfig loads the test configuration from test/config/setup.toml
// Automatically overrides port based on current directory (18085 for UI, 19085 for API)
func LoadTestConfig() (*TestConfig, error) {
	// Determine test type based on working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	isUITest := strings.Contains(cwd, "test\\ui") || strings.Contains(cwd, "test/ui")
	isAPITest := strings.Contains(cwd, "test\\api") || strings.Contains(cwd, "test/api")

	if !isUITest && !isAPITest {
		return nil, fmt.Errorf("LoadTestConfig must be called from test/ui or test/api directory, current: %s", cwd)
	}

	// Load shared config file
	configFile := "../config/setup.toml"
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	var config TestConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override port based on test type
	if isAPITest {
		config.Service.Port = 19085 // API tests use port 19085
	}
	// UI tests use default port from config (18085)

	return &config, nil
}

// SetupTestEnvironment starts the Quaero service and prepares the test environment
func SetupTestEnvironment(testName string) (*TestEnvironment, error) {
	config, err := LoadTestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load test config: %w", err)
	}

	// Determine test type (ui or api) for results directory organization
	cwd, _ := os.Getwd()
	var testType string
	if strings.Contains(cwd, "test\\ui") || strings.Contains(cwd, "test/ui") {
		testType = "ui"
	} else {
		testType = "api"
	}

	// Create results base directory with test type: ../../results/{ui|api}/
	resultsBaseDir := filepath.Join(config.Output.ResultsBaseDir, testType)

	// Extract suite name (e.g., "homepage" from "TestHomepageLoad")
	suiteName := extractSuiteName(testName)

	// Get or create suite parent directory: ../../results/{ui|api}/{suite-name}-{datetime}
	suiteDir, err := getOrCreateSuiteDirectory(suiteName, resultsBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create suite directory: %w", err)
	}

	// Create test-specific subdirectory under suite directory
	resultsDir := filepath.Join(suiteDir, testName)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create test directory: %w", err)
	}

	// Create service log file for this test run
	logPath := filepath.Join(resultsDir, "service.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create service log file: %w", err)
	}

	// Create test output log file
	testLogPath := filepath.Join(resultsDir, "test.log")
	testLogFile, err := os.Create(testLogPath)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to create test log file: %w", err)
	}

	env := &TestEnvironment{
		Config:     config,
		ResultsDir: resultsDir,
		LogFile:    logFile,
		TestLog:    testLogFile,
		Port:       config.Service.Port,
	}

	// Initialize output capture
	env.outputCapture = NewOutputCapture(testLogFile)
	env.outputCapture.Start()

	// Write TestMain output to test log
	if TestMainOutput.Len() > 0 {
		testLogFile.WriteString("=== TEST MAIN OUTPUT ===\n")
		testLogFile.Write(TestMainOutput.Bytes())
		testLogFile.WriteString("========================\n\n")
	}

	// Step 1: Build the application
	fmt.Fprintf(logFile, "Building application...\n")
	if err := env.buildService(); err != nil {
		logFile.Close()
		testLogFile.Close()
		return nil, fmt.Errorf("failed to build service: %w", err)
	}
	fmt.Fprintf(logFile, "Build completed successfully\n")

	// Step 2: Check if service is already running on configured port
	fmt.Fprintf(logFile, "\n=== SERVICE PORT CHECK ===\n")
	fmt.Fprintf(logFile, "Checking if service is already running on %s:%d...\n",
		config.Service.Host, config.Service.Port)

	if isServiceRunning(config.Service.Host, config.Service.Port) {
		fmt.Fprintf(logFile, "⚠️  EXISTING SERVICE DETECTED on %s:%d\n",
			config.Service.Host, config.Service.Port)
		fmt.Fprintf(logFile, "Attempting graceful shutdown via %s...\n",
			config.Service.ShutdownEndpoint)

		// Attempt graceful shutdown
		if err := shutdownService(config.Service.Host, config.Service.Port, config.Service.ShutdownEndpoint); err != nil {
			fmt.Fprintf(logFile, "❌ Failed to shutdown existing service: %v\n", err)
			fmt.Fprintf(logFile, "Test cannot continue with port in use\n")
			logFile.Close()
			testLogFile.Close()
			return nil, fmt.Errorf("failed to shutdown existing service (test cannot continue): %w", err)
		}

		fmt.Fprintf(logFile, "✓ Shutdown request sent successfully\n")

		// Wait for port to be released
		fmt.Fprintf(logFile, "Waiting for port %d to be released (timeout: 10s)...\n", config.Service.Port)
		if err := waitForPortRelease(config.Service.Host, config.Service.Port, 10*time.Second); err != nil {
			fmt.Fprintf(logFile, "❌ Port %d not released after shutdown: %v\n", config.Service.Port, err)
			logFile.Close()
			testLogFile.Close()
			return nil, fmt.Errorf("port not released after shutdown: %w", err)
		}
		fmt.Fprintf(logFile, "✓ Port %d released and available for test service\n", config.Service.Port)
	} else {
		fmt.Fprintf(logFile, "✓ Port %d is available (no existing service detected)\n", config.Service.Port)
	}

	// Step 3: Start new service instance
	fmt.Fprintf(logFile, "\n=== STARTING TEST SERVICE ===\n")
	fmt.Fprintf(logFile, "Starting new test service instance...\n")
	fmt.Fprintf(logFile, "  Host: %s\n", config.Service.Host)
	fmt.Fprintf(logFile, "  Port: %d\n", config.Service.Port)
	fmt.Fprintf(logFile, "  URL:  http://%s:%d\n", config.Service.Host, config.Service.Port)

	if err := env.startService(); err != nil {
		fmt.Fprintf(logFile, "❌ Failed to start test service: %v\n", err)
		logFile.Close()
		testLogFile.Close()
		return nil, fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Fprintf(logFile, "✓ Test service process started (PID: %d)\n", env.Cmd.Process.Pid)

	// Step 4: Wait for service to be ready
	fmt.Fprintf(logFile, "\n=== WAITING FOR SERVICE READY ===\n")
	fmt.Fprintf(logFile, "Polling service health endpoint (timeout: %ds)...\n",
		config.Service.StartupTimeoutSeconds)

	if err := env.WaitForServiceReady(); err != nil {
		fmt.Fprintf(logFile, "❌ Service did not become ready: %v\n", err)
		env.Cleanup()
		return nil, fmt.Errorf("service did not become ready: %w", err)
	}

	fmt.Fprintf(logFile, "\n=== SERVICE READY ===\n")
	fmt.Fprintf(logFile, "✓ Service is ready and accepting connections\n")
	fmt.Fprintf(logFile, "✓ Service URL: http://%s:%d\n", config.Service.Host, config.Service.Port)
	fmt.Fprintf(logFile, "✓ Test can proceed\n\n")

	return env, nil
}

// isServiceRunning checks if a service is running on the specified host:port
func isServiceRunning(host string, port int) bool {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// shutdownService attempts to gracefully shutdown the service via its shutdown endpoint
func shutdownService(host string, port int, shutdownEndpoint string) error {
	url := fmt.Sprintf("http://%s:%d%s", host, port, shutdownEndpoint)
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create shutdown request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("shutdown request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("shutdown returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// waitForPortRelease waits for a port to be released
func waitForPortRelease(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isServiceRunning(host, port) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("port %d not released after %v", port, timeout)
}

// buildService builds the Quaero application using go build
func (env *TestEnvironment) buildService() error {
	// Resolve paths relative to test/ui directory
	sourceDir, err := filepath.Abs(env.Config.Build.SourceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve source directory: %w", err)
	}

	binaryOutput, err := filepath.Abs(env.Config.Build.BinaryOutput)
	if err != nil {
		return fmt.Errorf("failed to resolve binary output path: %w", err)
	}

	// Add platform-specific extension
	if runtime.GOOS == "windows" {
		binaryOutput += ".exe"
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(binaryOutput)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Read version information from .version file
	versionFile, err := filepath.Abs(env.Config.Build.VersionFile)
	if err != nil {
		return fmt.Errorf("failed to resolve version file path: %w", err)
	}

	versionInfo := map[string]string{
		"Version": "unknown",
		"Build":   "unknown",
	}

	if data, err := os.ReadFile(versionFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "version:") {
				versionInfo["Version"] = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
			} else if strings.HasPrefix(line, "build:") {
				versionInfo["Build"] = strings.TrimSpace(strings.TrimPrefix(line, "build:"))
			}
		}
		fmt.Fprintf(env.LogFile, "Version info: %s (build: %s)\n", versionInfo["Version"], versionInfo["Build"])
	} else {
		fmt.Fprintf(env.LogFile, "Warning: Could not read version file, using defaults: %v\n", err)
	}

	// Build ldflags to inject version information
	module := "github.com/ternarybob/quaero/internal/common"
	ldflags := fmt.Sprintf("-X %s.Version=%s -X %s.Build=%s -X %s.GitCommit=test",
		module, versionInfo["Version"],
		module, versionInfo["Build"],
		module)

	// Build command: go build -ldflags="..." -o <output> <source_dir>
	cmd := exec.Command("go", "build", "-ldflags="+ldflags, "-o", binaryOutput, sourceDir)
	cmd.Stdout = env.LogFile
	cmd.Stderr = env.LogFile
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0") // Disable CGO like production build

	fmt.Fprintf(env.LogFile, "Building: go build -ldflags=\"%s\" -o %s %s\n", ldflags, binaryOutput, sourceDir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Build successful: %s\n", binaryOutput)

	// Copy test-config.toml to bin/quaero.toml
	testConfigPath := "../config/test-config.toml"
	binConfigPath := filepath.Join(filepath.Dir(binaryOutput), "quaero.toml")

	if err := env.copyFile(testConfigPath, binConfigPath); err != nil {
		return fmt.Errorf("failed to copy config to bin directory: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Config copied from %s to: %s\n", testConfigPath, binConfigPath)

	// Copy pages directory to bin/pages
	pagesSourcePath, err := filepath.Abs("../../pages")
	if err != nil {
		return fmt.Errorf("failed to resolve pages source path: %w", err)
	}

	binDir := filepath.Dir(binaryOutput)
	pagesDestPath := filepath.Join(binDir, "pages")

	// Remove existing pages directory if it exists
	if _, err := os.Stat(pagesDestPath); err == nil {
		if err := os.RemoveAll(pagesDestPath); err != nil {
			return fmt.Errorf("failed to remove existing pages directory: %w", err)
		}
	}

	// Copy pages directory
	if err := env.copyDir(pagesSourcePath, pagesDestPath); err != nil {
		return fmt.Errorf("failed to copy pages directory: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Pages copied from %s to: %s\n", pagesSourcePath, pagesDestPath)

	return nil
}

// copyFile copies a file from src to dst
func (env *TestEnvironment) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory from src to dst
func (env *TestEnvironment) copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := env.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := env.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// startService starts the Quaero service
func (env *TestEnvironment) startService() error {
	// Resolve binary path (from build output)
	binaryPath, err := filepath.Abs(env.Config.Build.BinaryOutput)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	// Add platform-specific extension
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Resolve config path
	configPath, err := filepath.Abs(env.Config.Build.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("service binary does not exist: %s", binaryPath)
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("service config does not exist: %s", configPath)
	}

	// Get the bin directory (where binary and config are located)
	binDir := filepath.Dir(binaryPath)

	// Start the Quaero service
	fmt.Fprintf(env.LogFile, "\n--- Service Startup Details ---\n")
	fmt.Fprintf(env.LogFile, "Command:     %s --config %s\n", binaryPath, configPath)
	fmt.Fprintf(env.LogFile, "Working Dir: %s\n", binDir)
	fmt.Fprintf(env.LogFile, "Listen URL:  http://%s:%d\n",
		env.Config.Service.Host, env.Config.Service.Port)
	fmt.Fprintf(env.LogFile, "Data Path:   %s\n", filepath.Join(binDir, "data"))
	fmt.Fprintf(env.LogFile, "-------------------------------\n\n")

	cmd := exec.Command(binaryPath, "--config", configPath)
	cmd.Dir = binDir // Set working directory to bin/ so data path resolves to bin/data/
	cmd.Stdout = env.LogFile
	cmd.Stderr = env.LogFile

	fmt.Fprintf(env.LogFile, "Starting service process...\n")
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(env.LogFile, "❌ Failed to start process: %v\n", err)
		return fmt.Errorf("failed to start service process: %w", err)
	}

	env.Cmd = cmd
	fmt.Fprintf(env.LogFile, "✓ Service process started successfully\n")
	fmt.Fprintf(env.LogFile, "  PID: %d\n", cmd.Process.Pid)
	fmt.Fprintf(env.LogFile, "  Port: %d\n", env.Config.Service.Port)
	fmt.Fprintf(env.LogFile, "\n--- Service Output Begins Below ---\n")

	return nil
}

// WaitForServiceReady polls the service until it responds to health checks
func (env *TestEnvironment) WaitForServiceReady() error {
	timeout := time.Duration(env.Config.Service.StartupTimeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	baseURL := fmt.Sprintf("http://%s:%d", env.Config.Service.Host, env.Config.Service.Port)

	attemptCount := 0
	lastLogTime := time.Now()

	for time.Now().Before(deadline) {
		attemptCount++
		resp, err := client.Get(baseURL + "/")

		// Log progress every 5 seconds
		if time.Since(lastLogTime) >= 5*time.Second {
			elapsed := time.Since(deadline.Add(-timeout)).Seconds()
			fmt.Fprintf(env.LogFile, "  Still waiting... (attempt %d, elapsed: %.1fs)\n",
				attemptCount, elapsed)
			lastLogTime = time.Now()
		}

		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			elapsed := time.Since(deadline.Add(-timeout)).Seconds()
			fmt.Fprintf(env.LogFile, "✓ Service ready and responding (took %.1fs, %d attempts)\n",
				elapsed, attemptCount)
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("service did not respond within %v (after %d attempts)", timeout, attemptCount)
}

// WaitForWebSocketConnection waits for the WebSocket to connect and status to show ONLINE
// This should be called after navigating to a page that includes the navbar
func (env *TestEnvironment) WaitForWebSocketConnection(ctx context.Context, timeoutSeconds int) error {
	timeout := time.Duration(timeoutSeconds) * time.Second
	startTime := time.Now()

	fmt.Fprintf(env.LogFile, "Waiting for WebSocket connection (status: ONLINE)...\n")

	// Wait for the status indicator to show "ONLINE"
	err := chromedp.Run(ctx,
		chromedp.WaitVisible(`.status-text`, chromedp.ByQuery),
		chromedp.Poll(`document.querySelector('.status-text')?.textContent === 'ONLINE'`,
			nil,
			chromedp.WithPollingTimeout(timeout),
			chromedp.WithPollingInterval(100*time.Millisecond),
		),
	)

	if err != nil {
		elapsed := time.Since(startTime).Seconds()
		fmt.Fprintf(env.LogFile, "✗ WebSocket did not connect within %.1fs: %v\n", elapsed, err)
		return fmt.Errorf("WebSocket connection timeout after %.1fs: %w", elapsed, err)
	}

	elapsed := time.Since(startTime).Seconds()
	fmt.Fprintf(env.LogFile, "✓ WebSocket connected (status: ONLINE) in %.1fs\n", elapsed)
	return nil
}

// Cleanup stops the service and closes resources
func (env *TestEnvironment) Cleanup() {
	// Write test completion marker
	if env.TestLog != nil {
		fmt.Fprintf(env.TestLog, "\n=== TEST COMPLETED ===\n")
	}

	// Stop output capture
	if env.outputCapture != nil {
		env.outputCapture.Stop()
	}

	if env.Cmd != nil && env.Cmd.Process != nil {
		fmt.Fprintf(env.LogFile, "Stopping service (PID: %d)...\n", env.Cmd.Process.Pid)
		env.Cmd.Process.Kill()
		env.Cmd.Wait()
		fmt.Fprintf(env.LogFile, "Service stopped\n")
	}
	if env.LogFile != nil {
		env.LogFile.Close()
	}
	if env.TestLog != nil {
		env.TestLog.Close()
	}
}

// GetScreenshotPath returns the path for saving a screenshot
func (env *TestEnvironment) GetScreenshotPath(name string) string {
	filename := fmt.Sprintf("%s.png", name)
	return filepath.Join(env.ResultsDir, filename)
}

// GetBaseURL returns the base URL for the service
func (env *TestEnvironment) GetBaseURL() string {
	return fmt.Sprintf("http://%s:%d", env.Config.Service.Host, env.Config.Service.Port)
}

// GetResultsDir returns the results directory for this test run
func (env *TestEnvironment) GetResultsDir() string {
	return env.ResultsDir
}

// TakeScreenshot captures a screenshot using chromedp and saves it to the test results directory
func (env *TestEnvironment) TakeScreenshot(ctx context.Context, name string) error {
	screenshotPath := env.GetScreenshotPath(name)

	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}

	if err := os.WriteFile(screenshotPath, buf, 0644); err != nil {
		return fmt.Errorf("failed to save screenshot: %w", err)
	}

	return nil
}

// LogTest writes a message to both the test log file and the test output (via t.Log)
func (env *TestEnvironment) LogTest(t *testing.T, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	logMsg := fmt.Sprintf("[%s] %s\n", timestamp, msg)

	// Write to test log file
	if env.TestLog != nil {
		env.TestLog.WriteString(logMsg)
	}

	// Also log to test output (appears in console and go test output)
	t.Log(msg)
}

// HTTPTestHelper provides helper methods for HTTP testing
type HTTPTestHelper struct {
	BaseURL string
	Client  *http.Client
	T       *testing.T
}

// NewHTTPTestHelper creates a new HTTP test helper with the env's base URL
func (env *TestEnvironment) NewHTTPTestHelper(t *testing.T) *HTTPTestHelper {
	return &HTTPTestHelper{
		BaseURL: env.GetBaseURL(),
		Client:  &http.Client{Timeout: 60 * time.Second},
		T:       t,
	}
}

// NewHTTPTestHelperWithTimeout creates a new HTTP test helper with custom timeout
func (env *TestEnvironment) NewHTTPTestHelperWithTimeout(t *testing.T, timeout time.Duration) *HTTPTestHelper {
	return &HTTPTestHelper{
		BaseURL: env.GetBaseURL(),
		Client:  &http.Client{Timeout: timeout},
		T:       t,
	}
}

// GET makes a GET request and returns the response
func (h *HTTPTestHelper) GET(path string) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("GET %s", url)
	return h.Client.Get(url)
}

// POST makes a POST request with JSON body
func (h *HTTPTestHelper) POST(path string, body interface{}) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("POST %s", url)

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest(http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return h.Client.Do(req)
}

// PUT makes a PUT request with JSON body
func (h *HTTPTestHelper) PUT(path string, body interface{}) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("PUT %s", url)

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return h.Client.Do(req)
}

// DELETE makes a DELETE request
func (h *HTTPTestHelper) DELETE(path string) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("DELETE %s", url)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}

	return h.Client.Do(req)
}

// AssertStatusCode verifies the response status code
func (h *HTTPTestHelper) AssertStatusCode(resp *http.Response, expected int) {
	if resp.StatusCode != expected {
		h.T.Errorf("Expected status code %d, got %d", expected, resp.StatusCode)
	}
}

// ParseJSONResponse parses JSON response into target
func (h *HTTPTestHelper) ParseJSONResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	h.T.Logf("Response body: %s", string(body))

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return nil
}

// AssertJSONField checks if a JSON field has the expected value
func (h *HTTPTestHelper) AssertJSONField(resp *http.Response, field string, expected interface{}) {
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		h.T.Fatalf("Failed to parse JSON: %v", err)
	}

	actual, ok := result[field]
	if !ok {
		h.T.Errorf("Field '%s' not found in response", field)
		return
	}

	if actual != expected {
		h.T.Errorf("Field '%s': expected %v, got %v", field, expected, actual)
	}
}

// NewOutputCapture creates a new output capturer
func NewOutputCapture(testLog *os.File) *OutputCapture {
	return &OutputCapture{
		buffer:      &bytes.Buffer{},
		originalOut: os.Stdout,
		originalErr: os.Stderr,
		testLog:     testLog,
		capturing:   false,
	}
}

// Start begins capturing stdout/stderr
func (oc *OutputCapture) Start() {
	oc.captureMutex.Lock()
	defer oc.captureMutex.Unlock()

	if oc.capturing {
		return
	}

	// Create pipe for capturing output
	r, w, err := os.Pipe()
	if err != nil {
		return // Silently fail if pipe creation fails
	}

	oc.reader = r
	oc.writer = w
	oc.capturing = true

	// Start copying in background
	oc.wg.Add(1)
	go func() {
		defer oc.wg.Done()
		// Tee to buffer, original output, and test log
		mw := io.MultiWriter(oc.buffer, oc.originalOut, oc.testLog)
		io.Copy(mw, oc.reader)
	}()

	// Redirect stdout/stderr to our pipe
	os.Stdout = oc.writer
	os.Stderr = oc.writer
}

// Stop restores stdout/stderr and returns captured output
func (oc *OutputCapture) Stop() string {
	oc.captureMutex.Lock()
	defer oc.captureMutex.Unlock()

	if !oc.capturing {
		return oc.buffer.String()
	}

	// Restore original stdout/stderr
	os.Stdout = oc.originalOut
	os.Stderr = oc.originalErr

	// Close writer to signal end of capture
	if oc.writer != nil {
		oc.writer.Close()
	}

	// Wait for copying to finish
	oc.wg.Wait()

	oc.capturing = false
	return oc.buffer.String()
}
