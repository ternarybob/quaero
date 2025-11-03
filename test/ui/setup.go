package ui

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pelletier/go-toml/v2"
)

// TestConfig holds the UI test configuration
type TestConfig struct {
	Service struct {
		Binary               string `toml:"binary"`
		Config               string `toml:"config"`
		StartupTimeoutSeconds int    `toml:"startup_timeout_seconds"`
		Port                 int    `toml:"port"`
		Host                 string `toml:"host"`
		ShutdownEndpoint     string `toml:"shutdown_endpoint"`
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
	LogFile    *os.File  // Service log output
	TestLog    *os.File  // Test execution log
	Port       int
}

// LoadTestConfig loads the test configuration from config.toml
func LoadTestConfig() (*TestConfig, error) {
	configPath := filepath.Join(".", "config.toml")
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

	// Create test-specific results directory: {test-name}-{datetime}
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(config.Output.ResultsBaseDir, fmt.Sprintf("%s-%s", testName, timestamp))
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create results directory: %w", err)
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

	// Step 1: Check if service is already running on configured port
	if isServiceRunning(config.Service.Host, config.Service.Port) {
		fmt.Fprintf(logFile, "Service already running on %s:%d, attempting graceful shutdown...\n",
			config.Service.Host, config.Service.Port)

		// Attempt graceful shutdown
		if err := shutdownService(config.Service.Host, config.Service.Port, config.Service.ShutdownEndpoint); err != nil {
			logFile.Close()
			return nil, fmt.Errorf("failed to shutdown existing service (test cannot continue): %w", err)
		}

		fmt.Fprintf(logFile, "Successfully shutdown existing service\n")

		// Wait for port to be released
		if err := waitForPortRelease(config.Service.Host, config.Service.Port, 10*time.Second); err != nil {
			logFile.Close()
			return nil, fmt.Errorf("port not released after shutdown: %w", err)
		}
	}

	// Step 2: Start new service instance
	if err := env.startService(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to start service: %w", err)
	}

	// Step 3: Wait for service to be ready
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

// startService starts the Quaero service
func (env *TestEnvironment) startService() error {
	// Resolve binary and config paths
	binaryPath, err := filepath.Abs(env.Config.Service.Binary)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	configPath, err := filepath.Abs(env.Config.Service.Config)
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
