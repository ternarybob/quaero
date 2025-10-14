Quaero Component-Based Refactoring Plan (v8)1. SummaryThis document outlines a comprehensive refactoring of the quaero application. The goal is to evolve the architecture into a more robust, maintainable, and modular system that clearly separates concerns between its core components: Startup & Configuration, UI & API, the Crawling Engine, and the Chat Engine.This refactor is a "greenfield" opportunity: backward compatibility is not required, and the focus is on building a clean foundation for future development, complete with a modern, Go-native testing strategy.2. Guiding PrinciplesSeparation of Concerns: Each major component (Config, UI, Crawler, Chat) will be decoupled and operate independently.Service-Oriented: Logic will be encapsulated in services exposed via interfaces.Startup Injection: Dependencies will be constructed in main.go and passed down to services (e.g., NewMyService(db, logger)).Single User, Transient Data: The database can be wiped and rebuilt at any time, simplifying data management.Required Libraries: Logging and startup banners must use ternarybob/arbor and ternarybob/banner respectively.3. Detailed Refactoring StagesThe plan is broken down into actionable stages, designed to be implemented sequentially.Stage 0: Pre-flight & DependenciesGoal: Prepare the project by updating external dependencies.Instructions:Add Required Libraries:Run go get github.com/ternarybob/arborRun go get github.com/ternarybob/bannerVerify go.mod: Ensure both new dependencies are correctly listed in the go.mod file.Stage 1: Foundational Refactor (Startup, Configuration, and Logging)Goal: Establish a clean application entrypoint, a simplified configuration model, and standardized logging.Instructions:Refactor cmd/quaero/main.go: This file becomes the single orchestrator for application startup.Create a Unified ConfigService: in internal/services/config to manage layered configuration (Defaults -> TOML -> Env -> Flags).Standardize Logging: Delete internal/common/logger.go and banner.go. Integrate ternarybob/arbor and ternarybob/banner, injecting the logger instance into all services from main.go.Stage 2: Crawling Engine OverhaulGoal: Replace the duplicated scrapers with a single, high-performance, configurable crawling engine.Instructions:Create Unified CrawlerService: in internal/services/crawler/service.go. It will manage concurrent crawl jobs using Go routines.Move AuthService: Relocate internal/services/atlassian/auth_service.go to internal/services/auth/service.go.Implement Structured Storage: Enhance internal/storage/sqlite/document_storage.go to handle layered content detail.Deprecate and Remove Old Code: Delete the entire internal/services/atlassian/ directory (after moving auth_service.go) and the specific storage files (jira_storage.go, confluence_storage.go).Stage 3: UI and API RefactorGoal: Decouple the UI from the backend and provide a rich, interactive experience.Instructions:Create a SourceManagement API: Refactor internal/server/routes.go and internal/handlers/ to use a generic /api/sources RESTful endpoint.Build a StatusService: Create internal/services/status/service.go to track and broadcast application state.Refactor the Frontend (pages/): Update the UI to use the new APIs for managing authentication, crawl configurations, and data, and to display real-time progress from WebSocket events.Stage 4: Chat Engine ImplementationGoal: Implement the streaming MCP agent for an intelligent and transparent chat experience.Instructions:Create Agent Toolbox: in internal/agents/tools, define a Tool interface and implement concrete tools (SearchDocumentsTool, etc.) that use the DocumentService.Implement Streaming MCP ChatService: Rewrite internal/services/chat/chat_service.go to orchestrate the agent loop.Update the Chat UI: Update pages/chat.html to handle structured WebSocket events and render the agent's "thought process" in real-time.Stage 5: Comprehensive Testing Strategy with Go-Native Harness ✅ COMPLETEDGoal: Build a new, robust test suite from scratch, orchestrated by a Go-native test harness that outputs structured results.Status: COMPLETED - Go-native test infrastructure implemented.Implementation:Archive Existing Tests and Harness: ✅ DONE- Created test/archive directory- Moved old tests into archive- Deleted ./test/run-tests.ps1 PowerShell scriptImplement Go-Native Integration Test Fixture (test/main_test.go):Action: Create the file test/main_test.go. This code does not run tests itself; it provides the setup and teardown logic (starting/stopping the server) for all other integration test files in the test/ package. The content should be the same as in v7.Implement New Test Suites with Examples:A. Unit Test Example (ConfigService)Action: Create the file internal/services/config/service_test.go with the same content as in v7.B. API Test Example (Source Management)Action: Create the file test/api/sources_api_test.go with the same content as in v7.C. UI Test Example (Homepage Interaction with chromedp)Goal: Test a core user journey using the Go-native chromedp library.Action: Add the chromedp dependency (go get github.com/chromedp/chromedp) and create the file test/ui/homepage_test.go.// test/ui/homepage_test.go
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
Implement Go-Native Test Runner (test/run_tests.go): ✅ DONE
Goal: Create a Go program that replaces the PowerShell script entirely, providing a cross-platform way to run all tests and generate structured output.

**Implementation Details:**
- File: test/run_tests.go
- Test suites: API Tests, UI Tests
- Results directory format: ./test/results/{testname}-{datetime}/
- Each test suite creates its own timestamped directory
- Provides summary output with pass/fail status and timing

**Usage:**
```bash
# Run all tests
cd test
go run run_tests.go

# Or run specific suite directly
cd test
go test -v ./api
go test -v ./ui

# Run unit tests (colocated with source)
go test ./internal/...
```

**Current Test Coverage:**
- Unit Tests (62 tests): internal/services/{crawler,search,config,identifiers,metadata,llm/offline,storage/sqlite}
- API Tests: test/api/{sources_api_test.go,chat_api_test.go}
- UI Tests: test/ui/{homepage_test.go,chat_test.go}
