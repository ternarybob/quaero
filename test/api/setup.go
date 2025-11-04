// -----------------------------------------------------------------------
// Last Modified: Tuesday, 4th November 2025 10:16:10 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package api

import (
	"bytes"
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

	"github.com/pelletier/go-toml/v2"
)

// testMainOutput captures the TestMain output for later inclusion in test logs
var testMainOutput bytes.Buffer

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

// TestConfig holds the API test configuration
type TestConfig struct {
	Build struct {
		SourceDir    string `toml:"source_dir"`
		BinaryOutput string `toml:"binary_output"`
		ConfigFile   string `toml:"config_file"`
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
// Example: "TestSourcesPageLoad" -> "TestSources"
//          "TestJobsCreateModal" -> "TestJobs"
//          "TestAuthPageLoad" -> "TestAuth"
func extractSuiteName(testName string) string {
	// Find all capital letter positions after "Test"
	if !strings.HasPrefix(testName, "Test") {
		return testName
	}

	// Remove "Test" prefix
	remainder := testName[4:]

	// Find all capital letter positions
	var capitals []int
	for i := 0; i < len(remainder); i++ {
		if remainder[i] >= 'A' && remainder[i] <= 'Z' {
			capitals = append(capitals, i)
		}
	}

	// If we have at least 2 capitals, take everything up to the second one
	// Example: "SourcesPageLoad" has capitals at [0, 7, 11]
	//          We want "Sources" (up to index 7)
	if len(capitals) >= 2 {
		return "Test" + remainder[:capitals[1]]
	}

	// If only one capital or none, return the whole name
	return testName
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

// LoadTestConfig loads the test configuration from setup.toml
func LoadTestConfig() (*TestConfig, error) {
	configPath := filepath.Join(".", "setup.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TestConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// SetupTestEnvironment starts the Quaero service and prepares the test environment
func SetupTestEnvironment(testName string) (*TestEnvironment, error) {
	config, err := LoadTestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load test config: %w", err)
	}

	// Extract suite name (e.g., "TestSources" from "TestSourcesPageLoad")
	suiteName := extractSuiteName(testName)

	// Get or create suite parent directory: {suite-name}-{datetime}
	suiteDir, err := getOrCreateSuiteDirectory(suiteName, config.Output.ResultsBaseDir)
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
	if testMainOutput.Len() > 0 {
		testLogFile.WriteString("=== TEST MAIN OUTPUT ===\n")
		testLogFile.Write(testMainOutput.Bytes())
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
	if isServiceRunning(config.Service.Host, config.Service.Port) {
		fmt.Fprintf(logFile, "Service already running on %s:%d, attempting graceful shutdown...\n",
			config.Service.Host, config.Service.Port)

		// Attempt graceful shutdown
		if err := shutdownService(config.Service.Host, config.Service.Port, config.Service.ShutdownEndpoint); err != nil {
			logFile.Close()
			testLogFile.Close()
			return nil, fmt.Errorf("failed to shutdown existing service (test cannot continue): %w", err)
		}

		fmt.Fprintf(logFile, "Successfully shutdown existing service\n")

		// Wait for port to be released
		if err := waitForPortRelease(config.Service.Host, config.Service.Port, 10*time.Second); err != nil {
			logFile.Close()
			testLogFile.Close()
			return nil, fmt.Errorf("port not released after shutdown: %w", err)
		}
	}

	// Step 3: Start new service instance
	if err := env.startService(); err != nil {
		logFile.Close()
		testLogFile.Close()
		return nil, fmt.Errorf("failed to start service: %w", err)
	}

	// Step 4: Wait for service to be ready
	if err := env.WaitForServiceReady(); err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("service did not become ready: %w", err)
	}

	return env, nil
}

// isServiceRunning checks if a service is running on the specified host:port
func isServiceRunning(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
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
	// Resolve paths relative to test/api directory
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

	// Build command: go build -o <output> <source_dir>
	cmd := exec.Command("go", "build", "-o", binaryOutput, sourceDir)
	cmd.Stdout = env.LogFile
	cmd.Stderr = env.LogFile

	fmt.Fprintf(env.LogFile, "Building: go build -o %s %s\n", binaryOutput, sourceDir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Build successful: %s\n", binaryOutput)

	// Copy test-config.toml to bin/quaero.toml
	testConfigPath := filepath.Join(".", "test-config.toml")
	binConfigPath := filepath.Join(filepath.Dir(binaryOutput), "quaero.toml")

	if err := env.copyFile(testConfigPath, binConfigPath); err != nil {
		return fmt.Errorf("failed to copy config to bin directory: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Config copied to: %s\n", binConfigPath)

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

	// Start the Quaero service
	fmt.Fprintf(env.LogFile, "Starting service: %s --config %s\n", binaryPath, configPath)
	cmd := exec.Command(binaryPath, "--config", configPath)
	cmd.Stdout = env.LogFile
	cmd.Stderr = env.LogFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	env.Cmd = cmd
	fmt.Fprintf(env.LogFile, "Service started with PID: %d\n", cmd.Process.Pid)

	return nil
}

// WaitForServiceReady polls the service until it responds to health checks
func (env *TestEnvironment) WaitForServiceReady() error {
	timeout := time.Duration(env.Config.Service.StartupTimeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	baseURL := fmt.Sprintf("http://%s:%d", env.Config.Service.Host, env.Config.Service.Port)

	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Fprintf(env.LogFile, "Service ready and responding\n")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("service did not respond within %v", timeout)
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

// GetBaseURL returns the base URL for the service
func (env *TestEnvironment) GetBaseURL() string {
	return fmt.Sprintf("http://%s:%d", env.Config.Service.Host, env.Config.Service.Port)
}

// GetResultsDir returns the results directory for this test run
func (env *TestEnvironment) GetResultsDir() string {
	return env.ResultsDir
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
