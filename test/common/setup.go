// -----------------------------------------------------------------------
// Shared test framework for both UI and API tests
// Last Modified: Wednesday, 5th November 2025 8:09:28 pm
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
	"net/url"
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

// TestLogger provides logging that writes to both t.Log() and a test log file
// This ensures all test output is captured for later analysis
// Log format matches Go's testing framework: "file:line: message"
type TestLogger struct {
	T       *testing.T
	TestLog *os.File
}

// NewTestLogger creates a TestLogger from an environment
func NewTestLogger(t *testing.T, testLog *os.File) *TestLogger {
	return &TestLogger{T: t, TestLog: testLog}
}

// getCallerLocation returns the file:line prefix for the caller
// Skip specifies how many stack frames to skip (2 = caller of Log/Logf)
func getCallerLocation(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???:0"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// getTestFileBaseName walks up the call stack to find the *_test.go file
// and returns its base name without the _test.go suffix.
// Example: "worker_stock_test.go" -> "worker_stock"
// This provides consistent suite naming based on the test file, not function name.
func getTestFileBaseName() string {
	// Walk up the call stack looking for a *_test.go file
	for skip := 1; skip < 20; skip++ {
		_, file, _, ok := runtime.Caller(skip)
		if !ok {
			break
		}

		// Get just the filename without directory
		base := filepath.Base(file)

		// Check if it's a test file
		if strings.HasSuffix(base, "_test.go") {
			// Remove _test.go suffix to get suite name
			suiteName := strings.TrimSuffix(base, "_test.go")
			return suiteName
		}
	}

	// Fallback if no test file found in stack
	return ""
}

// Log writes a message to both the test log file and t.Log()
// Format in file: "    file:line: message" (matches Go test output)
func (l *TestLogger) Log(args ...interface{}) {
	msg := fmt.Sprint(args...)
	if l.TestLog != nil {
		location := getCallerLocation(2) // Skip Log and getCallerLocation
		fmt.Fprintf(l.TestLog, "    %s: %s\n", location, msg)
	}
	l.T.Log(msg)
}

// Logf writes a formatted message to both the test log file and t.Logf()
// Format in file: "    file:line: message" (matches Go test output)
func (l *TestLogger) Logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if l.TestLog != nil {
		location := getCallerLocation(2) // Skip Logf and getCallerLocation
		fmt.Fprintf(l.TestLog, "    %s: %s\n", location, msg)
	}
	l.T.Log(msg)
}

// Error writes an error message to both the test log file and t.Error()
// Format in file: "    file:line: ERROR: message" (matches Go test output)
func (l *TestLogger) Error(args ...interface{}) {
	msg := fmt.Sprint(args...)
	if l.TestLog != nil {
		location := getCallerLocation(2) // Skip Error and getCallerLocation
		fmt.Fprintf(l.TestLog, "    %s: ERROR: %s\n", location, msg)
	}
	l.T.Error(msg)
}

// Errorf writes a formatted error message to both the test log file and t.Errorf()
// Format in file: "    file:line: ERROR: message" (matches Go test output)
func (l *TestLogger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if l.TestLog != nil {
		location := getCallerLocation(2) // Skip Errorf and getCallerLocation
		fmt.Fprintf(l.TestLog, "    %s: ERROR: %s\n", location, msg)
	}
	l.T.Error(msg)
}

// OutputCapture captures stdout/stderr and tees it to a file and original output
type OutputCapture struct {
	buffer       *bytes.Buffer
	originalOut  *os.File
	originalErr  *os.File
	reader       *os.File
	writer       *os.File
	wg           sync.WaitGroup
	testLog      *os.File
	outputFile   *os.File // output.md file for full test output
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

	// ServiceConfigFiles holds the list of service config files to pass to quaero binary
	// First file is base config, subsequent files are overrides
	ServiceConfigFiles []string
}

// TestEnvironment represents a running test environment
type TestEnvironment struct {
	Config         *TestConfig
	Cmd            *exec.Cmd
	ResultsDir     string
	LogFile        *os.File // Service log output
	TestLog        *os.File // Test execution log
	OutputFile     *os.File // output.md file for full test output
	Port           int
	ConfigFilePath string // Path to config file used (for copying to bin/)

	// Output capture for test console
	outputCapture *OutputCapture

	// Environment variables loaded from .env.test file
	EnvVars map[string]string
}

// extractSuiteName extracts the test suite name from a test name
// Derives lowercase suite name matching test file naming convention
// Example: "TestHomepageLoad" -> "homepage" (from homepage_test.go)
//
//	"HomepageTitle" -> "homepage" (from homepage_test.go)
//	"TestSourcesPageLoad" -> "sources" (from sources_test.go)
//	"TestJobsCreateModal" -> "jobs" (from jobs_test.go)
//	"TestConfig_Something" -> "config" (from config_test.go)
//	"TestJobDefinitionCodebaseClassify" -> "job-codebase" (special handling)
//	"TestJobDefinitionGeneralUIAssertions" -> "job-general" (special handling)
func extractSuiteName(testName string) string {
	// Remove "Test" prefix if present
	remainder := testName
	if strings.HasPrefix(testName, "Test") {
		remainder = testName[4:]
	}

	// Special handling for JobDefinition tests to produce readable names like "job-codebase", "job-general"
	if strings.HasPrefix(remainder, "JobDefinition") {
		// Extract the meaningful part after "JobDefinition"
		afterJobDef := remainder[13:] // len("JobDefinition") = 13

		// Find the first capital letter position in the remainder
		firstCapital := -1
		for i := 0; i < len(afterJobDef); i++ {
			if afterJobDef[i] >= 'A' && afterJobDef[i] <= 'Z' {
				firstCapital = i
				break
			}
		}

		// Extract the first "word" (up to second capital or end)
		var jobType string
		if firstCapital == 0 {
			// Find second capital
			secondCapital := -1
			for i := 1; i < len(afterJobDef); i++ {
				if afterJobDef[i] >= 'A' && afterJobDef[i] <= 'Z' {
					secondCapital = i
					break
				}
			}
			if secondCapital > 0 {
				jobType = strings.ToLower(afterJobDef[:secondCapital])
			} else {
				jobType = strings.ToLower(afterJobDef)
			}
		} else if firstCapital > 0 {
			jobType = strings.ToLower(afterJobDef[:firstCapital])
		} else {
			jobType = strings.ToLower(afterJobDef)
		}

		// Return "job-{type}" format for better readability
		if jobType != "" {
			return "job-" + jobType
		}
		return "job-definition"
	}

	// Special handling for Worker tests to produce readable names like "navexa-worker", "schema-worker"
	// Examples: TestWorkerNavexaPortfolios -> navexa-worker
	//           TestWorkerSchemaIntegration -> schema-worker
	if strings.HasPrefix(remainder, "Worker") {
		afterWorker := remainder[6:] // len("Worker") = 6

		// Find the first capital letter to get the worker type name
		firstCapital := -1
		for i := 0; i < len(afterWorker); i++ {
			if afterWorker[i] >= 'A' && afterWorker[i] <= 'Z' {
				firstCapital = i
				break
			}
		}

		var workerType string
		if firstCapital == 0 {
			// Find second capital to extract first word
			secondCapital := -1
			for i := 1; i < len(afterWorker); i++ {
				if afterWorker[i] >= 'A' && afterWorker[i] <= 'Z' {
					secondCapital = i
					break
				}
			}
			if secondCapital > 0 {
				workerType = strings.ToLower(afterWorker[:secondCapital])
			} else {
				workerType = strings.ToLower(afterWorker)
			}
		} else if firstCapital > 0 {
			workerType = strings.ToLower(afterWorker[:firstCapital])
		} else {
			workerType = strings.ToLower(afterWorker)
		}

		// Return "{type}-worker" format for better readability
		if workerType != "" {
			return workerType + "-worker"
		}
		return "worker"
	}

	// Find all capital letter positions
	var capitals []int
	for i := 0; i < len(remainder); i++ {
		if remainder[i] >= 'A' && remainder[i] <= 'Z' {
			capitals = append(capitals, i)
		}
	}

	var suiteName string
	// If we have at least 2 capitals, take everything up to the second one
	// Example: "HomepageTitle" has capitals at [0, 8]
	//          We want "homepage" (lowercase, up to index 8)
	// Example: "SourcesPageLoad" has capitals at [0, 7, 11]
	//          We want "sources" (lowercase, up to index 7)
	if len(capitals) >= 2 {
		suiteName = strings.ToLower(remainder[:capitals[1]])
	} else {
		// If only one capital or none, return the lowercase name
		suiteName = strings.ToLower(remainder)
	}

	// Trim trailing underscores (from test names like TestConfig_Something)
	return strings.TrimRight(suiteName, "_")
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
	// Format: {suite_name}_{datetime} (underscore matches test file naming convention)
	timestamp := time.Now().Format("20060102-150405")
	suiteDir := filepath.Join(baseDir, fmt.Sprintf("%s_%s", suiteName, timestamp))

	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create suite directory: %w", err)
	}

	// Store for future tests in this suite
	suiteDirectories[suiteName] = suiteDir

	return suiteDir, nil
}

// loadEnvFile loads environment variables from a .env file into a map
// Supports KEY=value and KEY="value" formats
// Ignores comments (lines starting with #) and empty lines
func loadEnvFile(path string) (map[string]string, error) {
	envVars := make(map[string]string)

	data, err := os.ReadFile(path)
	if err != nil {
		// .env file is optional, return empty map if not found
		if os.IsNotExist(err) {
			return envVars, nil
		}
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s", i+1, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		envVars[key] = value
	}

	return envVars, nil
}

// LoadTestConfig loads the test harness configuration from test/config/setup.toml
// Automatically overrides port based on current directory (18085 for UI, 19085 for API)
// Optionally accepts additional config paths to override base config (relative to test/ui or test/api directory)
// Example: LoadTestConfig("../config/quaero-test.toml") - disables agent service
func LoadTestConfig(additionalConfigPaths ...string) (*TestConfig, error) {
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

	// Load test harness config (build, service lifecycle, output)
	harnessConfigFile := "../config/setup.toml"
	harnessData, err := os.ReadFile(harnessConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read harness config %s: %w", harnessConfigFile, err)
	}

	var config TestConfig
	if err := toml.Unmarshal(harnessData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse harness config: %w", err)
	}

	// Build list of service config files (base + overrides)
	serviceConfigFiles := []string{"../config/test-quaero.toml"} // Base config
	serviceConfigFiles = append(serviceConfigFiles, additionalConfigPaths...)

	// Validate all service config files exist and are valid TOML
	for _, serviceConfigFile := range serviceConfigFiles {
		if serviceConfigFile == "" {
			continue
		}

		serviceData, err := os.ReadFile(serviceConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read service config %s: %w", serviceConfigFile, err)
		}

		// Validate service config is valid TOML
		var serviceConfigCheck map[string]any
		if err := toml.Unmarshal(serviceData, &serviceConfigCheck); err != nil {
			return nil, fmt.Errorf("failed to parse service config %s: %w", serviceConfigFile, err)
		}
	}

	// Store config file paths for later use (will be passed to quaero binary)
	config.ServiceConfigFiles = serviceConfigFiles

	// Override port based on test type
	if isAPITest {
		config.Service.Port = 19085 // API tests use port 19085
	}
	// UI tests use default port from harness config (18085)

	return &config, nil
}

// SetupTestEnvironment starts the Quaero service and prepares the test environment
// Optionally accepts a custom config path (relative to test/ui or test/api directory)
// By default, environment variables from .env.test are loaded and passed to the service
func SetupTestEnvironment(testName string, customConfigPath ...string) (*TestEnvironment, error) {
	return setupTestEnvironmentInternal(testName, true, customConfigPath...)
}

// SetupTestEnvironmentWithoutEnv starts the Quaero service without loading .env.test variables
// Use this for tests that don't need environment variable injection
func SetupTestEnvironmentWithoutEnv(testName string, customConfigPath ...string) (*TestEnvironment, error) {
	return setupTestEnvironmentInternal(testName, false, customConfigPath...)
}

// setupTestEnvironmentInternal is the internal implementation that handles environment setup
func setupTestEnvironmentInternal(testName string, includeEnv bool, customConfigPath ...string) (*TestEnvironment, error) {
	config, err := LoadTestConfig(customConfigPath...)
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

	// Get suite name from the test file name (e.g., "worker_stock" from "worker_stock_test.go")
	// This provides consistent naming based on file, not function name variations
	suiteName := getTestFileBaseName()
	if suiteName == "" {
		// Fallback to function-based extraction if file detection fails
		suiteName = extractSuiteName(testName)
	}

	// Get or create suite parent directory: ../../results/{ui|api}/{suite-name}_{datetime}
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

	// Create output.md file for full test output
	outputPath := filepath.Join(resultsDir, "output.md")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		logFile.Close()
		testLogFile.Close()
		return nil, fmt.Errorf("failed to create output.md file: %w", err)
	}

	// Determine config file path for copying to bin/
	configFilePath := "../config/test-config.toml" // Default
	if len(customConfigPath) > 0 && customConfigPath[0] != "" {
		configFilePath = customConfigPath[0]
	}

	env := &TestEnvironment{
		Config:         config,
		ResultsDir:     resultsDir,
		LogFile:        logFile,
		TestLog:        testLogFile,
		OutputFile:     outputFile,
		Port:           config.Service.Port,
		ConfigFilePath: configFilePath,
		EnvVars:        make(map[string]string), // Initialize empty map
	}

	// Load environment variables from .env files (if includeEnv is true)
	// Priority: root .env < test/config/.env.test (later files override earlier)
	// Both files are gitignored and should contain API keys locally
	if includeEnv {
		env.EnvVars = make(map[string]string)

		// First load from root .env file
		rootEnvPath := "../../.env"
		if rootEnvVars, err := loadEnvFile(rootEnvPath); err == nil && len(rootEnvVars) > 0 {
			for k, v := range rootEnvVars {
				if v != "" {
					env.EnvVars[k] = v
				}
			}
			fmt.Fprintf(logFile, "Loaded %d environment variable(s) from %s\n", len(rootEnvVars), rootEnvPath)
			for key := range rootEnvVars {
				fmt.Fprintf(logFile, "  - %s\n", key)
			}
		}

		// Then load from test/config/.env (can override root .env)
		testEnvPath := "../config/.env"
		if testEnvVars, err := loadEnvFile(testEnvPath); err == nil && len(testEnvVars) > 0 {
			for k, v := range testEnvVars {
				if v != "" {
					env.EnvVars[k] = v
				}
			}
			fmt.Fprintf(logFile, "Loaded %d environment variable(s) from %s\n", len(testEnvVars), testEnvPath)
			for key := range testEnvVars {
				fmt.Fprintf(logFile, "  - %s\n", key)
			}
		}

		if len(env.EnvVars) == 0 {
			fmt.Fprintf(logFile, "No API keys loaded - tests requiring API access may fail\n")
		}
	} else {
		fmt.Fprintf(logFile, "Environment variable loading disabled (includeEnv=false)\n")
	}

	// Initialize output capture
	env.outputCapture = NewOutputCapture(testLogFile, outputFile)
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

	// Load .env.test variables into KV store (if includeEnv is true)
	if includeEnv && len(env.EnvVars) > 0 {
		fmt.Fprintf(logFile, "\n=== LOADING ENV VARIABLES INTO KV STORE ===\n")
		if err := env.LoadEnvVariablesIntoKVStore(); err != nil {
			fmt.Fprintf(logFile, "❌ Failed to load env variables into KV store: %v\n", err)
			env.Cleanup()
			return nil, fmt.Errorf("failed to load env variables into KV store: %w", err)
		}
		fmt.Fprintf(logFile, "✓ Env variables loaded into KV store\n")
	}

	// Load test job definitions
	fmt.Fprintf(logFile, "\n=== LOADING TEST JOB DEFINITIONS ===\n")
	if err := env.LoadTestJobDefinitions(); err != nil {
		fmt.Fprintf(logFile, "❌ Failed to load test job definitions: %v\n", err)
		env.Cleanup()
		return nil, fmt.Errorf("failed to load test job definitions: %w", err)
	}
	fmt.Fprintf(logFile, "✓ Test job definitions loaded\n")

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

	// Merge service config files and write to bin/quaero.toml
	binConfigPath := filepath.Join(filepath.Dir(binaryOutput), "quaero.toml")

	if err := env.mergeConfigFiles(binConfigPath); err != nil {
		return fmt.Errorf("failed to merge config files to bin directory: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Config files merged to: %s\n", binConfigPath)
	for i, configPath := range env.Config.ServiceConfigFiles {
		fmt.Fprintf(env.LogFile, "  [%d] %s\n", i+1, configPath)
	}

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

	// Copy Chrome extension to bin/quaero-chrome-extension
	extensionSourcePath, err := filepath.Abs("../../cmd/quaero-chrome-extension")
	if err != nil {
		return fmt.Errorf("failed to resolve extension source path: %w", err)
	}

	extensionDestPath := filepath.Join(binDir, "quaero-chrome-extension")

	// Remove existing extension directory if it exists
	if _, err := os.Stat(extensionDestPath); err == nil {
		if err := os.RemoveAll(extensionDestPath); err != nil {
			return fmt.Errorf("failed to remove existing extension directory: %w", err)
		}
	}

	// Copy extension directory
	if err := env.copyDir(extensionSourcePath, extensionDestPath); err != nil {
		return fmt.Errorf("failed to copy extension directory: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Chrome extension copied from %s to: %s\n", extensionSourcePath, extensionDestPath)

	// Copy job-definitions directory to bin/job-definitions
	jobDefsSourcePath, err := filepath.Abs("../config/job-definitions")
	if err != nil {
		return fmt.Errorf("failed to resolve job-definitions source path: %w", err)
	}

	jobDefsDestPath := filepath.Join(binDir, "job-definitions")

	// Remove existing job-definitions directory if it exists
	if _, err := os.Stat(jobDefsDestPath); err == nil {
		if err := os.RemoveAll(jobDefsDestPath); err != nil {
			return fmt.Errorf("failed to remove existing job-definitions directory: %w", err)
		}
	}

	// Copy job-definitions directory
	if err := env.copyDir(jobDefsSourcePath, jobDefsDestPath); err != nil {
		return fmt.Errorf("failed to copy job-definitions directory: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Job definitions copied from %s to: %s\n", jobDefsSourcePath, jobDefsDestPath)

	// Copy templates directory to bin/templates (for job template worker)
	templatesSourcePath, err := filepath.Abs("../config/templates")
	if err != nil {
		return fmt.Errorf("failed to resolve templates source path: %w", err)
	}

	templatesDestPath := filepath.Join(binDir, "templates")

	// Remove existing templates directory if it exists
	if _, err := os.Stat(templatesDestPath); err == nil {
		if err := os.RemoveAll(templatesDestPath); err != nil {
			return fmt.Errorf("failed to remove existing templates directory: %w", err)
		}
	}

	// Copy templates directory
	if err := env.copyDir(templatesSourcePath, templatesDestPath); err != nil {
		return fmt.Errorf("failed to copy templates directory: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Templates copied from %s to: %s\n", templatesSourcePath, templatesDestPath)

	// Schemas are now embedded in the binary via internal/schemas/embed.go
	// No need to copy external schema files - remove any stale schemas directory
	schemasDestPath := filepath.Join(binDir, "schemas")
	if _, err := os.Stat(schemasDestPath); err == nil {
		if err := os.RemoveAll(schemasDestPath); err != nil {
			return fmt.Errorf("failed to remove stale schemas directory: %w", err)
		}
		fmt.Fprintf(env.LogFile, "Removed stale schemas directory: %s\n", schemasDestPath)
	}

	// Copy variables.toml file to bin/variables.toml (for variables and key/value storage)
	variablesSourcePath, err := filepath.Abs("../config/variables.toml")
	if err != nil {
		return fmt.Errorf("failed to resolve variables source path: %w", err)
	}

	variablesDestPath := filepath.Join(binDir, "variables.toml")

	// Copy variables.toml file
	if err := env.copyFile(variablesSourcePath, variablesDestPath); err != nil {
		return fmt.Errorf("failed to copy variables.toml: %w", err)
	}

	fmt.Fprintf(env.LogFile, "Variables copied from %s to: %s\n", variablesSourcePath, variablesDestPath)

	// Inject real API keys from environment into variables.toml
	// This ensures tests run with actual keys without committing them to git
	variablesFile := variablesDestPath

	type VariableConfig struct {
		Value       string `toml:"value"`
		Description string `toml:"description"`
	}
	var variablesConfig map[string]VariableConfig

	// Read existing variables.toml
	if data, err := os.ReadFile(variablesFile); err == nil {
		if err := toml.Unmarshal(data, &variablesConfig); err != nil {
			fmt.Fprintf(env.LogFile, "Warning: Failed to parse variables.toml: %v\n", err)
			variablesConfig = make(map[string]VariableConfig)
		}
	} else {
		variablesConfig = make(map[string]VariableConfig)
	}

	// Update with environment variables
	// Note: These env vars are loaded from .env.test in NewTestEnvironment
	// .env.test uses lowercase key names matching the placeholder format in config files
	// Priority: Specific keys (google_places_api_key, google_gemini_api_key) > Generic keys (GOOGLE_API_KEY)

	// Load google_gemini_api_key from .env.test
	if key := env.EnvVars["google_gemini_api_key"]; key != "" {
		variablesConfig["google_gemini_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from google_gemini_api_key in .env.test",
		}
		// Also set google_api_key as fallback for generic references
		variablesConfig["google_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from google_gemini_api_key in .env.test",
		}
	}

	// Load google_places_api_key from .env.test
	if key := env.EnvVars["google_places_api_key"]; key != "" {
		variablesConfig["google_places_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from google_places_api_key in .env.test",
		}
		// Also set test-google-places-key for consistency with old format
		variablesConfig["test-google-places-key"] = VariableConfig{
			Value:       key,
			Description: "Injected from google_places_api_key in .env.test",
		}
	}

	// Fallback: Check for legacy GOOGLE_API_KEY format (if neither specific key is set)
	if key := env.EnvVars["GOOGLE_API_KEY"]; key != "" {
		// Only use if specific keys weren't already set
		if _, hasGemini := variablesConfig["google_gemini_api_key"]; !hasGemini {
			variablesConfig["google_gemini_api_key"] = VariableConfig{
				Value:       key,
				Description: "Injected from GOOGLE_API_KEY environment variable (fallback)",
			}
			variablesConfig["google_api_key"] = VariableConfig{
				Value:       key,
				Description: "Injected from GOOGLE_API_KEY environment variable (fallback)",
			}
		}
		if _, hasPlaces := variablesConfig["google_places_api_key"]; !hasPlaces {
			variablesConfig["google_places_api_key"] = VariableConfig{
				Value:       key,
				Description: "Injected from GOOGLE_API_KEY environment variable (fallback)",
			}
			variablesConfig["test-google-places-key"] = VariableConfig{
				Value:       key,
				Description: "Injected from GOOGLE_API_KEY environment variable (fallback)",
			}
		}
	}

	// Override with QUAERO_ prefixed environment variables if set (highest priority)
	if key := env.EnvVars["QUAERO_PLACES_API_KEY"]; key != "" {
		variablesConfig["google_places_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from QUAERO_PLACES_API_KEY environment variable",
		}
		variablesConfig["test-google-places-key"] = VariableConfig{
			Value:       key,
			Description: "Injected from QUAERO_PLACES_API_KEY environment variable",
		}
	}
	if key := env.EnvVars["QUAERO_GEMINI_GOOGLE_API_KEY"]; key != "" {
		variablesConfig["google_gemini_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from QUAERO_GEMINI_GOOGLE_API_KEY environment variable",
		}
	}
	if key := env.EnvVars["QUAERO_AGENT_GOOGLE_API_KEY"]; key != "" {
		variablesConfig["google_gemini_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from QUAERO_AGENT_GOOGLE_API_KEY environment variable",
		}
	}

	// Load EODHD API key from .env.test
	if key := env.EnvVars["eodhd_api_key"]; key != "" {
		variablesConfig["eodhd_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from eodhd_api_key in .env.test",
		}
	}
	// Override with EODHD_API_KEY environment variable if set
	if key := env.EnvVars["EODHD_API_KEY"]; key != "" {
		variablesConfig["eodhd_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from EODHD_API_KEY environment variable",
		}
	}

	// Load Navexa API key from .env.test
	if key := env.EnvVars["navexa_api_key"]; key != "" {
		variablesConfig["navexa_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from navexa_api_key in .env.test",
		}
	}
	// Override with NAVEXA_API_KEY environment variable if set
	if key := env.EnvVars["NAVEXA_API_KEY"]; key != "" {
		variablesConfig["navexa_api_key"] = VariableConfig{
			Value:       key,
			Description: "Injected from NAVEXA_API_KEY environment variable",
		}
	}

	// Write updated variables.toml
	if data, err := toml.Marshal(variablesConfig); err == nil {
		if err := os.WriteFile(variablesFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write updated variables.toml: %w", err)
		}
		fmt.Fprintf(env.LogFile, "Injected API keys into variables.toml\n")
	} else {
		return fmt.Errorf("failed to marshal variables config: %w", err)
	}

	// Copy connectors.toml file to bin/connectors.toml
	connectorsSourcePath, err := filepath.Abs("../config/connectors.toml")
	if err != nil {
		return fmt.Errorf("failed to resolve connectors source path: %w", err)
	}

	connectorsDestPath := filepath.Join(binDir, "connectors.toml")

	// Copy connectors.toml file
	if err := env.copyFile(connectorsSourcePath, connectorsDestPath); err != nil {
		return fmt.Errorf("failed to copy connectors.toml: %w", err)
	}
	fmt.Fprintf(env.LogFile, "Connectors copied from %s to: %s\n", connectorsSourcePath, connectorsDestPath)

	// Copy email.toml file to bin/email.toml
	emailSourcePath, err := filepath.Abs("../config/email.toml")
	if err != nil {
		return fmt.Errorf("failed to resolve email source path: %w", err)
	}

	emailDestPath := filepath.Join(binDir, "email.toml")

	// Copy email.toml file
	if err := env.copyFile(emailSourcePath, emailDestPath); err != nil {
		return fmt.Errorf("failed to copy email.toml: %w", err)
	}
	fmt.Fprintf(env.LogFile, "Email config copied from %s to: %s\n", emailSourcePath, emailDestPath)

	return nil
}

// mergeConfigFiles merges multiple service config files and writes the result to dst
// Later files override earlier files (same behavior as LoadFromFiles)
func (env *TestEnvironment) mergeConfigFiles(dst string) error {
	// Start with empty config map
	mergedConfig := make(map[string]any)

	// Load and merge each config file in order
	for _, configPath := range env.Config.ServiceConfigFiles {
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("failed to resolve config path %s: %w", configPath, err)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to read config file %s: %w", absPath, err)
		}

		// Unmarshal into merged config (later values override earlier ones)
		if err := toml.Unmarshal(data, &mergedConfig); err != nil {
			return fmt.Errorf("failed to parse config file %s: %w", absPath, err)
		}
	}

	// Marshal merged config back to TOML
	mergedData, err := toml.Marshal(mergedConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal merged config: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(dst, mergedData, 0644); err != nil {
		return fmt.Errorf("failed to write merged config: %w", err)
	}

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
	// Override port via environment variable (takes precedence over config file)
	cmd.Env = append(os.Environ(), fmt.Sprintf("QUAERO_SERVER_PORT=%d", env.Config.Service.Port))

	// Note: .env.test variables are loaded into KV store via API after service starts
	// This allows them to override placeholder values in variables.toml
	// See LoadEnvVariablesIntoKVStore() function

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

// Cleanup stops the service, kills browser processes, and closes resources
func (env *TestEnvironment) Cleanup() {
	// Write test completion marker
	if env.TestLog != nil {
		fmt.Fprintf(env.TestLog, "\n=== TEST COMPLETED ===\n")
	}

	// Stop output capture
	if env.outputCapture != nil {
		env.outputCapture.Stop()
	}

	// Stop the service first
	if env.Cmd != nil && env.Cmd.Process != nil {
		fmt.Fprintf(env.LogFile, "Stopping service (PID: %d)...\n", env.Cmd.Process.Pid)
		env.Cmd.Process.Kill()
		env.Cmd.Wait()
		fmt.Fprintf(env.LogFile, "Service stopped\n")
	}

	// Force kill any lingering Chrome processes spawned by tests (Windows specific)
	// This ensures no browser processes leak memory between test runs
	if runtime.GOOS == "windows" {
		env.killLingeringProcesses()
	}

	if env.LogFile != nil {
		env.LogFile.Close()
	}

	// Close test.log and output.md files
	// NOTE: output.md should be written by tests explicitly via saveWorkerOutput()
	// test.log contains execution logs and should NOT be copied to output.md
	if env.TestLog != nil {
		env.TestLog.Sync()
		env.TestLog.Close()
	}
	if env.OutputFile != nil {
		env.OutputFile.Close()
	}
}

// killLingeringProcesses forcefully terminates only test-spawned service processes
// NOTE: We intentionally do NOT kill chrome.exe as this would affect the user's browser.
// chromedp.Cancel() handles browser cleanup; this only handles the quaero service.
func (env *TestEnvironment) killLingeringProcesses() {
	// Only kill the test service, not Chrome (user may have their own browser open)
	// chromedp handles its own browser cleanup via chromedp.Cancel()
	processNames := []string{"quaero.exe"}

	for _, procName := range processNames {
		// Use taskkill /F to forcefully kill processes
		// /F = Force terminate, /IM = Image name
		cmd := exec.Command("taskkill", "/F", "/IM", procName)
		output, err := cmd.CombinedOutput()
		if err == nil {
			if env.LogFile != nil {
				fmt.Fprintf(env.LogFile, "Killed lingering process: %s\n", procName)
			}
		} else if !strings.Contains(string(output), "not found") && !strings.Contains(string(output), "No tasks") {
			// Only log if it's not a "process not found" error
			if env.LogFile != nil {
				fmt.Fprintf(env.LogFile, "Note: Could not kill %s (may not be running): %v\n", procName, err)
			}
		}
	}
}

// GetBaseURL returns the base URL for the service
func (env *TestEnvironment) GetBaseURL() string {
	return fmt.Sprintf("http://%s:%d", env.Config.Service.Host, env.Config.Service.Port)
}

// NewTestLogger creates a TestLogger for the environment
func (env *TestEnvironment) NewTestLogger(t *testing.T) *TestLogger {
	return NewTestLogger(t, env.TestLog)
}

// GetResultsDir returns the results directory for this test run
func (env *TestEnvironment) GetResultsDir() string {
	return env.ResultsDir
}

// GetExtensionPath returns the absolute path to the Chrome extension in bin directory
func (env *TestEnvironment) GetExtensionPath() (string, error) {
	// Get bin directory from binary output path
	binaryOutput, err := filepath.Abs(env.Config.Build.BinaryOutput)
	if err != nil {
		return "", fmt.Errorf("failed to resolve binary output path: %w", err)
	}

	binDir := filepath.Dir(binaryOutput)
	extensionPath := filepath.Join(binDir, "quaero-chrome-extension")

	// Verify extension directory exists
	if _, err := os.Stat(extensionPath); os.IsNotExist(err) {
		return "", fmt.Errorf("extension directory not found: %s", extensionPath)
	}

	return extensionPath, nil
}

// LoadJobDefinitionFile reads a TOML file and uploads it via the job definition upload API
func (env *TestEnvironment) LoadJobDefinitionFile(filePath string) error {
	// Read the TOML file
	tomlBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read job definition file %s: %w", filePath, err)
	}

	// Upload via POST /api/job-definitions/upload
	url := fmt.Sprintf("%s/api/job-definitions/upload", env.GetBaseURL())
	fmt.Fprintf(env.LogFile, "POST %s (Content-Type: text/plain, %d bytes)\n", url, len(tomlBytes))

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(tomlBytes))
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", filePath, err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload job definition file %s: %w", filePath, err)
	}
	defer resp.Body.Close()

	// Check response status (201 Created or 200 OK for updates)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed for %s: status %d, body: %s", filePath, resp.StatusCode, string(bodyBytes))
	}

	// Log success
	fmt.Fprintf(env.LogFile, "✓ Loaded job definition: %s (status: %d)\n", filepath.Base(filePath), resp.StatusCode)
	return nil
}

// LoadEnvVariablesIntoKVStore loads environment variables from .env.test into the KV store via API
// This allows .env.test variables to override placeholder values in variables.toml
func (env *TestEnvironment) LoadEnvVariablesIntoKVStore() error {
	if len(env.EnvVars) == 0 {
		return nil
	}

	baseURL := fmt.Sprintf("http://%s:%d", env.Config.Service.Host, env.Config.Service.Port)
	client := &http.Client{Timeout: 10 * time.Second}

	loadedCount := 0
	for key, value := range env.EnvVars {
		// Upsert variable via PUT /api/kv/{key}
		url := fmt.Sprintf("%s/api/kv/%s", baseURL, url.PathEscape(key))

		reqBody := map[string]string{
			"value":       value,
			"description": "Loaded from .env.test",
		}

		jsonBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request for key %s: %w", key, err)
		}

		req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBytes))
		if err != nil {
			return fmt.Errorf("failed to create request for key %s: %w", key, err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to upsert key %s: %w", key, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to upsert key %s (status %d): %s", key, resp.StatusCode, string(body))
		}

		fmt.Fprintf(env.LogFile, "  ✓ Loaded variable: %s\n", key)
		loadedCount++
	}

	fmt.Fprintf(env.LogFile, "Loaded %d variable(s) into KV store\n", loadedCount)
	return nil
}

// LoadTestJobDefinitions loads job definition files for tests
// Accepts variadic list of job definition file paths (relative to test/ui or test/api directory)
// Example: env.LoadTestJobDefinitions("../config/test-agent-job.toml")
func (env *TestEnvironment) LoadTestJobDefinitions(jobDefPaths ...string) error {
	if len(jobDefPaths) == 0 {
		// No job definitions to load
		return nil
	}

	fmt.Fprintf(env.LogFile, "\n=== LOADING TEST JOB DEFINITIONS ===\n")

	// Load each job definition file
	for _, configPath := range jobDefPaths {
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("failed to resolve path for job definition %s: %w", configPath, err)
		}

		if err := env.LoadJobDefinitionFile(absPath); err != nil {
			return fmt.Errorf("failed to load job definition %s: %w", configPath, err)
		}

		fmt.Fprintf(env.LogFile, "✓ Loaded job definition: %s\n", configPath)
	}

	fmt.Fprintf(env.LogFile, "✓ All test job definitions loaded successfully\n")
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
	TestLog *os.File // Test log file for capturing output
}

// Log writes a message to both the test log file and t.Log()
// Format in file: "    file:line: message" (matches Go test output)
func (h *HTTPTestHelper) Log(args ...interface{}) {
	msg := fmt.Sprint(args...)
	if h.TestLog != nil {
		location := getCallerLocation(2) // Skip Log and getCallerLocation
		fmt.Fprintf(h.TestLog, "    %s: %s\n", location, msg)
	}
	h.T.Log(msg)
}

// Logf writes a formatted message to both the test log file and t.Logf()
// Format in file: "    file:line: message" (matches Go test output)
func (h *HTTPTestHelper) Logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if h.TestLog != nil {
		location := getCallerLocation(2) // Skip Logf and getCallerLocation
		fmt.Fprintf(h.TestLog, "    %s: %s\n", location, msg)
	}
	if h.T != nil {
		h.T.Log(msg)
	}
}

// NewHTTPTestHelper creates a new HTTP test helper with the env's base URL
func (env *TestEnvironment) NewHTTPTestHelper(t *testing.T) *HTTPTestHelper {
	return &HTTPTestHelper{
		BaseURL: env.GetBaseURL(),
		Client:  &http.Client{Timeout: 60 * time.Second},
		T:       t,
		TestLog: env.TestLog,
	}
}

// NewHTTPTestHelperWithTimeout creates a new HTTP test helper with custom timeout
func (env *TestEnvironment) NewHTTPTestHelperWithTimeout(t *testing.T, timeout time.Duration) *HTTPTestHelper {
	return &HTTPTestHelper{
		BaseURL: env.GetBaseURL(),
		Client:  &http.Client{Timeout: timeout},
		T:       t,
		TestLog: env.TestLog,
	}
}

// GET makes a GET request and returns the response
func (h *HTTPTestHelper) GET(path string) (*http.Response, error) {
	url := h.BaseURL + path
	h.Logf("GET %s", url)
	return h.Client.Get(url)
}

// POST makes a POST request with JSON body
func (h *HTTPTestHelper) POST(path string, body interface{}) (*http.Response, error) {
	url := h.BaseURL + path
	h.Logf("POST %s", url)

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

// POSTBody makes a POST request with raw byte content and specified content type
func (h *HTTPTestHelper) POSTBody(path string, contentType string, body []byte) (*http.Response, error) {
	url := h.BaseURL + path
	h.Logf("POST %s (Content-Type: %s, %d bytes)", url, contentType, len(body))

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)

	return h.Client.Do(req)
}

// PUT makes a PUT request with JSON body
func (h *HTTPTestHelper) PUT(path string, body interface{}) (*http.Response, error) {
	url := h.BaseURL + path
	h.Logf("PUT %s", url)

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
	h.Logf("DELETE %s", url)

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

	h.Logf("Response body: %s", string(body))

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
func NewOutputCapture(testLog *os.File, outputFile *os.File) *OutputCapture {
	return &OutputCapture{
		buffer:      &bytes.Buffer{},
		originalOut: os.Stdout,
		originalErr: os.Stderr,
		testLog:     testLog,
		outputFile:  outputFile,
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
		// Tee to buffer, original output, test log, and output.md
		writers := []io.Writer{oc.buffer, oc.originalOut, oc.testLog}
		if oc.outputFile != nil {
			writers = append(writers, oc.outputFile)
		}
		mw := io.MultiWriter(writers...)
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
