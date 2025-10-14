package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

func main() {
	fmt.Println("==============================================")
	fmt.Println("Quaero Test Runner")
	fmt.Println("==============================================\n")

	// Define test suites
	suites := []TestSuite{
		{
			Name:    "API Tests",
			Path:    "./api",
			Command: []string{"go", "test", "-v", "-coverprofile=coverage.out", "./api"},
		},
		{
			Name:    "UI Tests",
			Path:    "./ui",
			Command: []string{"go", "test", "-v", "./ui"},
		},
	}

	// Create results directory
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	resultsDir := filepath.Join("results", fmt.Sprintf("run-%s", timestamp))
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		fmt.Printf("ERROR: Failed to create results directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Test results will be saved to: %s\n\n", resultsDir)

	// Run all test suites
	results := make([]TestResult, 0, len(suites))
	allPassed := true

	for _, suite := range suites {
		fmt.Printf("Running %s...\n", suite.Name)
		fmt.Println(strings.Repeat("-", 80))

		result := runTestSuite(suite, resultsDir)
		results = append(results, result)

		if result.Success {
			fmt.Printf("✓ %s PASSED (%.2fs)\n\n", suite.Name, result.Duration.Seconds())
		} else {
			fmt.Printf("✗ %s FAILED (%.2fs)\n\n", suite.Name, result.Duration.Seconds())
			allPassed = false
		}
	}

	// Print summary
	printSummary(results, allPassed)

	// Exit with appropriate code
	if !allPassed {
		os.Exit(1)
	}
}

func runTestSuite(suite TestSuite, resultsDir string) TestResult {
	startTime := time.Now()

	// Run the test command
	cmd := exec.Command(suite.Command[0], suite.Command[1:]...)
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Save output to file
	outputFile := filepath.Join(resultsDir, fmt.Sprintf("%s.log", sanitizeFilename(suite.Name)))
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
