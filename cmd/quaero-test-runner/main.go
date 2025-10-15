// -----------------------------------------------------------------------
// Last Modified: Wednesday, 15th October 2025 2:16:41 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type TestSuite struct {
	Name    string
	Path    string
	Command []string
}

type TestResult struct {
	Suite    string
	Success  bool
	Output   string
	Duration time.Duration
}

type TestRunnerConfig struct {
	TestRunner struct {
		TestsDir    string `toml:"tests_dir"`
		OutputDir   string `toml:"output_dir"`
		BuildScript string `toml:"build_script"`
	} `toml:"test_runner"`
	TestServer struct {
		Port int `toml:"port"`
	} `toml:"test_server"`
	Service struct {
		Binary                string `toml:"binary"`
		Config                string `toml:"config"`
		Port                  int    `toml:"port"` // Optional port override
		StartupTimeoutSeconds int    `toml:"startup_timeout_seconds"`
	} `toml:"service"`
}

type ServiceConfig struct {
	Server struct {
		Port int    `toml:"port"`
		Host string `toml:"host"`
	} `toml:"server"`
}

// loadConfig loads the test runner configuration
func loadConfig() (*TestRunnerConfig, error) {
	// Get the directory of the executable
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	// Look for config file in executable directory first
	configPath := filepath.Join(exeDir, "quaero-test-runner.toml")

	// If not found, try current directory
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "quaero-test-runner.toml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TestRunnerConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults if not specified
	if config.TestRunner.TestsDir == "" {
		config.TestRunner.TestsDir = "./test"
	}
	if config.TestRunner.OutputDir == "" {
		config.TestRunner.OutputDir = "./test/results"
	}
	if config.TestRunner.BuildScript == "" {
		if runtime.GOOS == "windows" {
			config.TestRunner.BuildScript = "./scripts/build.ps1"
		} else {
			config.TestRunner.BuildScript = "./scripts/build.sh"
		}
	}

	return &config, nil
}

// loadServiceConfig reads the service configuration from bin/quaero.toml
func loadServiceConfig(configPath string) (*ServiceConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service config: %w", err)
	}

	var config ServiceConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse service config: %w", err)
	}

	return &config, nil
}

// killProcessOnPort kills any process listening on the specified port
func killProcessOnPort(port int) error {
	if runtime.GOOS == "windows" {
		// Use netstat to find process on port, then taskkill
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("$proc = Get-NetTCPConnection -LocalPort %d -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique; if ($proc) { Stop-Process -Id $proc -Force; exit 0 } else { exit 0 }", port))
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Only error if there was a real problem (not just "no process found")
			outputStr := string(output)
			if outputStr != "" && !strings.Contains(outputStr, "Cannot find") {
				return fmt.Errorf("failed to kill process on port %d: %w, output: %s", port, err, outputStr)
			}
		}
		return nil
	} else {
		// Linux/Mac: use lsof and kill
		cmd := exec.Command("sh", "-c", fmt.Sprintf("lsof -ti tcp:%d | xargs kill -9 2>/dev/null || true", port))
		cmd.Run() // Ignore errors - port might not be in use
		return nil
	}
}

func main() {
	fmt.Println("==============================================")
	fmt.Println("Quaero Test Runner")
	fmt.Println("==============================================\n")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Tests Directory: %s\n", config.TestRunner.TestsDir)
	fmt.Printf("  Output Directory: %s\n", config.TestRunner.OutputDir)
	fmt.Printf("  Build Script: %s\n\n", config.TestRunner.BuildScript)

	// Step 0: Start test server for browser validation
	fmt.Printf("STEP 0: Starting test server (port %d)...\n", config.TestServer.Port)
	fmt.Println(strings.Repeat("-", 80))
	testServer := StartTestServer(config.TestServer.Port)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		testServer.Shutdown(ctx)
		fmt.Println("✓ Test server stopped")
	}()

	// Verify test server is ready
	testServerURL := fmt.Sprintf("http://localhost:%d", config.TestServer.Port)
	if err := waitForService(testServerURL, 5*time.Second); err != nil {
		fmt.Printf("ERROR: Test server did not become ready: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Test server ready on %s\n\n", testServerURL)

	// Step 0.5: Check connectivity
	fmt.Println("STEP 0.5: Verifying connectivity...")
	fmt.Println(strings.Repeat("-", 80))
	if err := checkConnectivity(testServerURL); err != nil {
		fmt.Printf("WARNING: Connectivity check failed: %v\n", err)
		fmt.Println("Continuing with tests...\n")
	} else {
		fmt.Println("✓ Connectivity verified\n")
	}

	// Step 1: Read service configuration to determine port
	fmt.Println("STEP 1: Reading service configuration...")
	fmt.Println(strings.Repeat("-", 80))
	serviceConfig, err := loadServiceConfig(config.Service.Config)
	if err != nil {
		fmt.Printf("ERROR: Failed to load service config: %v\n", err)
		os.Exit(1)
	}

	// Determine actual service port (override if specified in test runner config)
	servicePort := serviceConfig.Server.Port
	if config.Service.Port != 0 {
		servicePort = config.Service.Port
		fmt.Printf("Using port override from test runner config: %d\n", servicePort)
	}
	serviceHost := serviceConfig.Server.Host
	if serviceHost == "" {
		serviceHost = "localhost"
	}

	serviceURL := fmt.Sprintf("http://%s:%d", serviceHost, servicePort)

	// Step 2: Check if service is already running
	fmt.Println("\nSTEP 2: Checking for existing service...")
	fmt.Println(strings.Repeat("-", 80))

	var serviceCmd *exec.Cmd
	var needsCleanup bool

	if err := waitForService(serviceURL, 2*time.Second); err == nil {
		// Service is already running!
		fmt.Printf("✓ Service already running on %s\n", serviceURL)
		fmt.Println("✓ Using existing service (will not start new instance)")
		fmt.Println("  Note: Service will NOT be stopped when tests complete\n")
		needsCleanup = false
	} else {
		// Service not running, need to start it
		fmt.Printf("Service not detected on port %d, will start new instance\n", servicePort)

		// Kill any zombie processes on the port
		fmt.Printf("Checking for zombie processes on port %d...\n", servicePort)
		if err := killProcessOnPort(servicePort); err != nil {
			fmt.Printf("WARNING: Failed to kill process on port %d: %v\n", servicePort, err)
		} else {
			fmt.Printf("✓ Port %d is clear\n", servicePort)
		}

		fmt.Println("\nStarting service in visible window...")
		serviceCmd, err = startService(config, servicePort)
		if err != nil {
			fmt.Printf("ERROR: Failed to start service: %v\n", err)
			os.Exit(1)
		}
		needsCleanup = true

		// Wait for service to be ready
		fmt.Println("Waiting for service to be ready...")
		startupTimeout := time.Duration(config.Service.StartupTimeoutSeconds) * time.Second
		if err := waitForService(serviceURL, startupTimeout); err != nil {
			fmt.Printf("ERROR: Service did not become ready: %v\n", err)
			if needsCleanup {
				stopService(serviceCmd)
			}
			os.Exit(1)
		}
		fmt.Printf("✓ Service is ready on %s\n", serviceURL)
		fmt.Println("✓ Service window should be visible\n")
	}

	// Ensure cleanup happens if we started the service
	if needsCleanup {
		defer stopService(serviceCmd)
	}

	// Step 3: Run tests
	fmt.Println("STEP 3: Running tests...")
	fmt.Println(strings.Repeat("-", 80))

	// Define test suites organized by category
	// Use ./ prefix for go test to recognize as relative path
	apiTestPath := filepath.ToSlash(filepath.Join(config.TestRunner.TestsDir, "api"))
	uiTestPath := filepath.ToSlash(filepath.Join(config.TestRunner.TestsDir, "ui"))

	suites := []TestSuite{
		{
			Name:    "API Tests",
			Path:    apiTestPath,
			Command: []string{"go", "test", "-v", "./" + apiTestPath},
		},
		{
			Name:    "UI Tests",
			Path:    uiTestPath,
			Command: []string{"go", "test", "-v", "./" + uiTestPath},
		},
	}

	fmt.Printf("Test results will be saved to: %s/{testname}-{datetime}/\n", config.TestRunner.OutputDir)
	fmt.Printf("UI tests will include screenshots for each navigation\n\n")

	results := make([]TestResult, 0, len(suites))
	allPassed := true

	for _, suite := range suites {
		fmt.Printf("Running %s...\n", suite.Name)
		fmt.Println(strings.Repeat("-", 80))

		result := runTestSuite(suite, config.TestRunner.OutputDir, serviceURL)
		results = append(results, result)

		if result.Success {
			fmt.Printf("✓ %s PASSED (%.2fs)\n\n", suite.Name, result.Duration.Seconds())
		} else {
			fmt.Printf("✗ %s FAILED (%.2fs)\n\n", suite.Name, result.Duration.Seconds())
			allPassed = false
		}
	}

	// Step 4: Cleanup
	if needsCleanup {
		fmt.Println("\nSTEP 4: Stopping service...")
		stopService(serviceCmd)
	} else {
		fmt.Println("\nSTEP 4: Cleanup...")
		fmt.Println("✓ Skipping service stop (using external service)")
	}

	// Print summary
	printSummary(results, allPassed)

	// Exit with appropriate code
	if !allPassed {
		os.Exit(1)
	}
}

// buildApplication runs the build script
func buildApplication(config *TestRunnerConfig) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Run PowerShell script
		cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", config.TestRunner.BuildScript)
	} else {
		cmd = exec.Command("bash", config.TestRunner.BuildScript)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// startService starts the quaero service
func startService(config *TestRunnerConfig, servicePort int) (*exec.Cmd, error) {
	// Get absolute paths
	exePath, err := filepath.Abs(config.Service.Binary)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve binary path: %w", err)
	}

	configPath, err := filepath.Abs(config.Service.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	binDir := filepath.Dir(exePath)

	// Build command args
	args := []string{"-c", configPath}
	if config.Service.Port != 0 {
		// Port override specified in test runner config
		args = append(args, "--port", fmt.Sprintf("%d", servicePort))
	}

	// Run service directly from bin directory
	cmd := exec.Command(exePath, args...)
	cmd.Dir = binDir

	// For Windows, create a new console window
	if runtime.GOOS == "windows" {
		// CREATE_NEW_CONSOLE flag (0x00000010)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: 0x00000010,
		}
		fmt.Println("  ✓ Service starting in new console window...")
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start service: %w", err)
	}

	// Give it a moment to actually start
	time.Sleep(3 * time.Second)

	return cmd, nil
}

// stopService stops the quaero service
func stopService(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	fmt.Println("Stopping service...")

	if runtime.GOOS == "windows" {
		// Kill all quaero processes on Windows
		exec.Command("taskkill", "/F", "/IM", "quaero.exe").Run()
	} else {
		cmd.Process.Kill()
	}

	// Give it a moment to clean up
	time.Sleep(1 * time.Second)

	fmt.Println("✓ Service stopped")
}

// checkConnectivity verifies network connectivity
func checkConnectivity(testServerURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}

	// Test local connectivity (test server)
	resp, err := client.Get(testServerURL + "/status")
	if err != nil {
		return fmt.Errorf("local connectivity failed: %w", err)
	}
	resp.Body.Close()

	// Test internet connectivity (optional - don't fail if this doesn't work)
	resp2, err := client.Get("http://www.google.com")
	if err != nil {
		fmt.Printf("  ⚠ Internet connectivity check failed (non-critical): %v\n", err)
	} else {
		resp2.Body.Close()
		fmt.Println("  ✓ Internet connectivity OK")
	}

	fmt.Println("  ✓ Local connectivity OK")
	return nil
}

// waitForService waits for the service to become ready
func waitForService(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		// For test server, use /status endpoint
		checkURL := url + "/api/status"
		if strings.Contains(url, "3333") {
			checkURL = url + "/status"
		}

		resp, err := client.Get(checkURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("service did not become ready within %v", timeout)
}

func runTestSuite(suite TestSuite, outputDir string, serviceURL string) TestResult {
	startTime := time.Now()
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	// Create results directory structure: {output_dir}/{testname}-{datetime}/
	suiteDir := filepath.Join(outputDir, fmt.Sprintf("%s-%s", sanitizeFilename(suite.Name), timestamp))
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		fmt.Printf("ERROR: Failed to create suite directory: %v\n", err)
	}

	// Convert to absolute path for environment variable
	absSuiteDir, err := filepath.Abs(suiteDir)
	if err != nil {
		fmt.Printf("ERROR: Failed to resolve absolute path: %v\n", err)
		absSuiteDir = suiteDir
	}

	// Create screenshots subdirectory for UI tests
	if strings.Contains(strings.ToLower(suite.Name), "ui") {
		screenshotDir := filepath.Join(absSuiteDir, "screenshots")
		if err := os.MkdirAll(screenshotDir, 0755); err != nil {
			fmt.Printf("ERROR: Failed to create screenshots directory: %v\n", err)
		}
	}

	// Run the test command with environment variables
	cmd := exec.Command(suite.Command[0], suite.Command[1:]...)
	cmd.Dir = "."

	// Pass environment variables to test process with ABSOLUTE paths
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TEST_RESULTS_DIR=%s", absSuiteDir),
		fmt.Sprintf("TEST_SERVER_URL=%s", serviceURL),
	)

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Save output to test.log in the suite directory
	outputFile := filepath.Join(suiteDir, "test.log")
	os.WriteFile(outputFile, output, 0644)

	// Determine success
	success := err == nil

	return TestResult{
		Suite:    suite.Name,
		Success:  success,
		Output:   string(output),
		Duration: duration,
	}
}

func printSummary(results []TestResult, allPassed bool) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 80))

	totalDuration := time.Duration(0)
	passed := 0
	failed := 0

	for _, result := range results {
		status := "PASS"
		if !result.Success {
			status = "FAIL"
			failed++
		} else {
			passed++
		}

		fmt.Printf("%-30s %s (%.2fs)\n", result.Suite, status, result.Duration.Seconds())
		totalDuration += result.Duration
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total: %d passed, %d failed (%.2fs)\n", passed, failed, totalDuration.Seconds())

	if allPassed {
		fmt.Println("\n✓ ALL TESTS PASSED")
	} else {
		fmt.Println("\n✗ SOME TESTS FAILED")
	}
}

func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscores
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		":", "_",
	)
	return strings.ToLower(replacer.Replace(name))
}
