Quaero Component-Based Refactoring Plan (v8)1. SummaryThis document outlines a comprehensive refactoring of the quaero application. The goal is to evolve the architecture into a more robust, maintainable, and modular system that clearly separates concerns between its core components: Startup & Configuration, UI & API, the Crawling Engine, and the Chat Engine.This refactor is a "greenfield" opportunity: backward compatibility is not required, and the focus is on building a clean foundation for future development, complete with a modern, Go-native testing strategy.2. Guiding PrinciplesSeparation of Concerns: Each major component (Config, UI, Crawler, Chat) will be decoupled and operate independently.Service-Oriented: Logic will be encapsulated in services exposed via interfaces.Startup Injection: Dependencies will be constructed in main.go and passed down to services (e.g., NewMyService(db, logger)).Single User, Transient Data: The database can be wiped and rebuilt at any time, simplifying data management.Required Libraries: Logging and startup banners must use ternarybob/arbor and ternarybob/banner respectively.3. Detailed Refactoring StagesThe plan is broken down into actionable stages, designed to be implemented sequentially.Stage 0: Pre-flight & DependenciesGoal: Prepare the project by updating external dependencies.Instructions:Add Required Libraries:Run go get github.com/ternarybob/arborRun go get github.com/ternarybob/bannerVerify go.mod: Ensure both new dependencies are correctly listed in the go.mod file.Stage 1: Foundational Refactor (Startup, Configuration, and Logging)Goal: Establish a clean application entrypoint, a simplified configuration model, and standardized logging.Instructions:Refactor cmd/quaero/main.go: This file becomes the single orchestrator for application startup.Create a Unified ConfigService: in internal/services/config to manage layered configuration (Defaults -> TOML -> Env -> Flags).Standardize Logging: Delete internal/common/logger.go and banner.go. Integrate ternarybob/arbor and ternarybob/banner, injecting the logger instance into all services from main.go.Stage 2: Crawling Engine OverhaulGoal: Replace the duplicated scrapers with a single, high-performance, configurable crawling engine.Instructions:Create Unified CrawlerService: in internal/services/crawler/service.go. It will manage concurrent crawl jobs using Go routines.Move AuthService: Relocate internal/services/atlassian/auth_service.go to internal/services/auth/service.go.Implement Structured Storage: Enhance internal/storage/sqlite/document_storage.go to handle layered content detail.Deprecate and Remove Old Code: Delete the entire internal/services/atlassian/ directory (after moving auth_service.go) and the specific storage files (jira_storage.go, confluence_storage.go).Stage 3: UI and API RefactorGoal: Decouple the UI from the backend and provide a rich, interactive experience.Instructions:Create a SourceManagement API: Refactor internal/server/routes.go and internal/handlers/ to use a generic /api/sources RESTful endpoint.Build a StatusService: Create internal/services/status/service.go to track and broadcast application state.Refactor the Frontend (pages/): Update the UI to use the new APIs for managing authentication, crawl configurations, and data, and to display real-time progress from WebSocket events.Stage 4: Chat Engine ImplementationGoal: Implement the streaming MCP agent for an intelligent and transparent chat experience.Instructions:Create Agent Toolbox: in internal/agents/tools, define a Tool interface and implement concrete tools (SearchDocumentsTool, etc.) that use the DocumentService.Implement Streaming MCP ChatService: Rewrite internal/services/chat/chat_service.go to orchestrate the agent loop.Update the Chat UI: Update pages/chat.html to handle structured WebSocket events and render the agent's "thought process" in real-time.Stage 5: Comprehensive Testing Strategy with Go-Native HarnessGoal: Build a new, robust test suite from scratch, orchestrated by a Go-native test harness that outputs structured results.Instructions:Archive Existing Tests and Harness:a.  Create a directory named test/archive.b.  Move all existing files and subdirectories from test/api, test/ui, and test/unit into test/archive.c.  Delete the ./test/run-tests.ps1 script.Implement Go-Native Integration Test Fixture (test/main_test.go):Action: Create the file test/main_test.go. This code does not run tests itself; it provides the setup and teardown logic (starting/stopping the server) for all other integration test files in the test/ package. The content should be the same as in v7.Implement New Test Suites with Examples:A. Unit Test Example (ConfigService)Action: Create the file internal/services/config/service_test.go with the same content as in v7.B. API Test Example (Source Management)Action: Create the file test/api/sources_api_test.go with the same content as in v7.C. UI Test Example (Homepage Interaction with chromedp)Goal: Test a core user journey using the Go-native chromedp library.Action: Add the chromedp dependency (go get github.com/chromedp/chromedp) and create the file test/ui/homepage_test.go.// test/ui/homepage_test.go
package test

import (
	"context"
	"log"
	"testing"
	"time"

	"[github.com/chromedp/chromedp](https://github.com/chromedp/chromedp)"
	"[github.com/stretchr/testify/require](https://github.com/stretchr/testify/require)"
)

// TestHomepage_TitleAndInteraction_Chromedp verifies that the homepage loads and a key
// interaction (like navigating to the settings page) works using chromedp.
func TestHomepage_TitleAndInteraction_Chromedp(t *testing.T) {
	// ARRANGE
	// Create a context for the browser interaction.
	// The serverURL is the global variable from our TestMain fixture.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create a new browser instance.
	allocCtx, cancel := chromedp.NewContext(ctx, chromedp.WithLogf(log.Printf))
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// ACT & ASSERT
	var title string
	err := chromedp.Run(taskCtx,
		// 1. Navigate to the application's homepage.
		chromedp.Navigate(serverURL),

		// 2. ASSERT: Check the page title.
		// Pass Requirement 1: The homepage title must be "Quaero".
		chromedp.Title(&title),
		chromedp.ActionFunc(func(c context.Context) error {
			require.Equal(t, "Quaero", title, "Page title should be 'Quaero'")
			return nil
		}),

		// 3. ACT: Simulate a user clicking the 'Settings' link in the navbar.
		// This assumes a navbar link with the text "Settings".
		chromedp.Click("nav >> text=Settings", chromedp.NodeVisible),

		// 4. ASSERT: Verify navigation by waiting for a unique element
		// on the settings page to become visible.
		// Pass Requirement 2: After clicking 'Settings', an H1 element with the text
		// "Application Settings" must be visible.
		chromedp.WaitVisible(`h1:has-text("Application Settings")`),
	)

	require.NoError(t, err, "Chromedp task sequence failed")
}
Implement Go-Native Test Runner (test/run_tests.go):Goal: Create a Go program that replaces the PowerShell script entirely, providing a cross-platform way to run all tests and generate structured output.Action: Create the file test/run_tests.go.// test/run_tests.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// main orchestrates the entire test suite, running different test types
// and directing their output to the results directory.
func main() {
	fmt.Println("Starting Quaero test runner...")
	resultsDir := filepath.Join(".", "results")

	// Create the results directory, ignoring error if it already exists.
	_ = os.Mkdir(resultsDir, 0755)

	// Test categories: path to test and the output file name.
	testTasks := []struct {
		name     string
		path     string
		outFile  string
	}{
		{"Unit Tests", "../internal/...", "unit_tests.json"},
		{"API Tests", "./api/...", "api_tests.json"},
		{"UI Tests", "./ui/...", "ui_tests.json"},
	}

	allPassed := true
	for _, task := range testTasks {
		fmt.Printf("\n--- Running %s ---\n", task.name)
		outputFile := filepath.Join(resultsDir, task.outFile)

		// The command `go test -v -json ./path/to/tests` runs the tests and outputs results as a JSON stream.
		cmd := exec.Command("go", "test", "-v", "-json", task.path)

		// Open the output file for writing.
		out, err := os.Create(outputFile)
		if err != nil {
			fmt.Printf("Error creating output file %s: %v\n", outputFile, err)
			allPassed = false
			continue
		}
		defer out.Close()

		// Pipe the command's stdout to both the console and the output file.
		cmd.Stdout = out // Direct output to the file
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("!!! %s failed. See %s for details.\n", task.name, outputFile)
			allPassed = false
		} else {
			fmt.Printf("+++ %s passed. Results saved to %s.\n", task.name, outputFile)
		}
	}

	if !allPassed {
		fmt.Println("\n!!! One or more test suites failed.")
		os.Exit(1)
	}

	fmt.Println("\n+++ All test suites passed successfully.")
}
