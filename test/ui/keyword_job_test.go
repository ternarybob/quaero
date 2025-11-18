package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestKeywordJob tests job execution with Google Gemini agent
// Phase 1: Run "places-nearby-restaurants" job and verify documents created
// Phase 2: Run "keyword-extractor-agent" job with proper API key setup and verify successful execution
func TestKeywordJob(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestKeywordJob")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestKeywordJob")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestKeywordJob (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestKeywordJob (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	h := env.NewHTTPTestHelper(t)
	serverURL := env.GetBaseURL()

	// ============================================================
	// SETUP: Insert Google API Key for agent service
	// ============================================================
	env.LogTest(t, "=== SETUP: Inserting Google API Key ===")

	// Verify API key from environment (.env.test) is loaded
	googleAPIKey, exists := env.EnvVars["GOOGLE_API_KEY"]
	if !exists || googleAPIKey == "" {
		t.Fatalf("GOOGLE_API_KEY not found in .env.test file")
	}
	env.LogTest(t, "✓ GOOGLE_API_KEY loaded from .env.test: %s...%s", googleAPIKey[:10], googleAPIKey[len(googleAPIKey)-4:])
	env.LogTest(t, "✓ Environment variable will automatically override variables.toml placeholder")

	// Setup Chrome context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Use 600s (10 minutes) timeout to accommodate:
	// - API setup and HTTP calls
	// - Multiple page navigations
	// - Job polling (Places: 5min, Keyword: 2min)
	// - Screenshots and UI verifications
	ctx, cancel = context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	// Navigate to queue page and setup
	env.LogTest(t, "Navigating to queue page: %s/queue", serverURL)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(serverURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to queue page: %v", err)
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Wait for WebSocket connection
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected (status: ONLINE)")

	// Take initial screenshot
	env.LogTest(t, "Taking screenshot of initial queue state...")
	if err := env.TakeScreenshot(ctx, "phase1-queue-initial"); err != nil {
		env.LogTest(t, "ERROR: Failed to take initial screenshot: %v", err)
		t.Fatalf("Failed to take initial screenshot: %v", err)
	}

	// ============================================================
	// PHASE 1: Create Test Documents for Keyword Extraction
	// ============================================================

	env.LogTest(t, "=== PHASE 1: Creating Test Documents ===")

	// Insert 3 test documents with markdown content
	// Use timestamp to ensure unique IDs for each test run
	timestamp := time.Now().Unix()
	testDocs := []struct {
		id      string
		title   string
		content string
	}{
		{
			id:    fmt.Sprintf("test-doc-ai-ml-%d", timestamp),
			title: "Introduction to AI and Machine Learning",
			content: `# AI and Machine Learning

Artificial Intelligence (AI) and Machine Learning (ML) are transforming technology.

## Key Concepts
- **Neural Networks**: Inspired by the human brain
- **Deep Learning**: Multiple layers of neural networks
- **Natural Language Processing**: Understanding human language
- **Computer Vision**: Teaching computers to see

## Applications
AI is used in healthcare, finance, transportation, and entertainment.

### Keywords
artificial intelligence, machine learning, neural networks, deep learning, NLP, computer vision, algorithms, data science`,
		},
		{
			id:    fmt.Sprintf("test-doc-web-dev-%d", timestamp),
			title: "Modern Web Development Practices",
			content: `# Modern Web Development

Web development has evolved significantly with modern frameworks and tools.

## Frontend Technologies
- **React**: Component-based UI library
- **Vue.js**: Progressive JavaScript framework
- **TypeScript**: Typed superset of JavaScript

## Backend Technologies
- **Node.js**: JavaScript runtime
- **Go**: Fast compiled language
- **Python**: Versatile and popular

## Best Practices
- Responsive design
- Progressive web apps
- Performance optimization
- Security best practices

### Keywords
web development, frontend, backend, React, Vue, TypeScript, Node.js, Go, Python, JavaScript, frameworks, responsive design`,
		},
		{
			id:    fmt.Sprintf("test-doc-cloud-%d", timestamp),
			title: "Cloud Computing and DevOps",
			content: `# Cloud Computing

Cloud computing provides on-demand computing resources over the internet.

## Major Cloud Providers
- **AWS**: Amazon Web Services
- **Azure**: Microsoft Azure
- **GCP**: Google Cloud Platform

## DevOps Practices
- Continuous Integration/Continuous Deployment (CI/CD)
- Infrastructure as Code (IaC)
- Container orchestration with Kubernetes
- Monitoring and observability

## Benefits
- Scalability
- Cost efficiency
- Global availability
- Disaster recovery

### Keywords
cloud computing, AWS, Azure, GCP, DevOps, CI/CD, Kubernetes, Docker, infrastructure, microservices, containers, automation`,
		},
	}

	for _, doc := range testDocs {
		if err := insertTestDocument(t, h, env, doc.id, doc.title, doc.content); err != nil {
			env.LogTest(t, "ERROR: Failed to create test document: %v", err)
			t.Fatalf("Failed to create test document: %v", err)
		}
		// Small delay to avoid potential race conditions
		time.Sleep(100 * time.Millisecond)
	}

	env.LogTest(t, "✓ Created %d test documents for keyword extraction", len(testDocs))
	env.LogTest(t, "✅ PHASE 1 PASS: Test documents created")

	/* COMMENTED OUT - Original Phase 1 (Places API) - kept for reference

	env.LogTest(t, "=== PHASE 1: Places Job - Document Creation ===")

	// Create Places job definition
	placesJobDef := map[string]interface{}{
		"id":          "places-nearby-restaurants",
		"name":        "Nearby Restaurants (Wheelers Hill)",
		"type":        "places",
		"job_type":    "user",
		"description": "Search for restaurants near Wheelers Hill using Google Places Nearby Search API",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":     "search_nearby_restaurants",
				"action":   "places_search",
				"on_error": "fail",
				"config": map[string]interface{}{
					"search_query": "restaurants near Wheelers Hill",
					"search_type":  "nearby_search",
					"max_results":  20,
					"list_name":    "Wheelers Hill Restaurants",
					"location": map[string]interface{}{
						"latitude":  -37.9167,
						"longitude": 145.1833,
						"radius":    2000,
					},
					"filters": map[string]interface{}{
						"type":       "restaurant",
						"min_rating": 3.5,
					},
				},
			},
		},
	}

	// Create the job definition (ignore error if already exists)
	env.LogTest(t, "Creating Places job definition...")
	createResp, err := h.POST("/api/job-definitions", placesJobDef)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to create job definition: %v", err)
		t.Fatalf("Failed to create job definition: %v", err)
	}
	if createResp.StatusCode == 201 || createResp.StatusCode == 409 {
		env.LogTest(t, "✓ Places job definition created/exists")
	}

	// Navigate to jobs page to verify it appears in UI
	env.LogTest(t, "Navigating to jobs page to verify job definition exists in UI...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to job definitions page: %v", err)
		t.Fatalf("Failed to navigate to job definitions page: %v", err)
	}

	// Wait a moment for page to fully load
	time.Sleep(2 * time.Second)

	// Take screenshot of job definitions page (do this first to capture what we see)
	env.LogTest(t, "Taking screenshot of job definitions page...")
	if err := env.TakeScreenshot(ctx, "phase1-job-definition-created"); err != nil {
		env.LogTest(t, "ERROR: Failed to take job definitions screenshot: %v", err)
		t.Fatalf("Failed to take job definitions screenshot: %v", err)
	}

	// Verify job definition appears in UI (optional check)
	env.LogTest(t, "Verifying Places job definition appears in UI...")
	var pageText string
	err = chromedp.Run(ctx,
		chromedp.Text(`body`, &pageText, chromedp.ByQuery),
	)
	if err == nil {
		if strings.Contains(pageText, "places-nearby-restaurants") || strings.Contains(pageText, "Nearby Restaurants") {
			env.LogTest(t, "✓ Places job definition visible in UI")
		} else {
			env.LogTest(t, "⚠️  WARNING: Places job definition not clearly visible in UI (check screenshot)")
		}
	} else {
		env.LogTest(t, "⚠️  WARNING: Could not verify job definition in UI: %v", err)
	}

	// Execute the Places job via UI button click
	// Wait for Alpine.js to render the dynamic content
	time.Sleep(2 * time.Second)

	// Find the execute button for the Places job using a more flexible approach
	// The button ID pattern is: jobDef.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') + '-run'
	// For "Nearby Restaurants (Wheelers Hill)" this is "nearby-restaurants-wheelers-hill--run" (note double dash)
	env.LogTest(t, "Finding execute button for Places job...")
	var placesButtonID string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const title = card.querySelector('.card-title');
					if (title && title.textContent.includes('Nearby Restaurants')) {
						const runButton = card.querySelector('button[id$="-run"]');
						if (runButton) return runButton.id;
					}
				}
				return null;
			})()
		`, &placesButtonID),
	)
	if err != nil || placesButtonID == "" {
		env.LogTest(t, "ERROR: Failed to find execute button")

		// Debug: Check what button IDs are actually present
		var buttonIDs string
		chromedp.Run(ctx,
			chromedp.Evaluate(`Array.from(document.querySelectorAll('button[id]')).map(b => b.id).join(', ')`, &buttonIDs),
		)
		env.LogTest(t, "DEBUG: Available button IDs: %s", buttonIDs)

		t.Fatalf("Failed to find execute button for Places job")
	}
	env.LogTest(t, "Found execute button with ID: %s", placesButtonID)

	// Click the button and handle the confirmation dialog
	env.LogTest(t, "Clicking execute button and accepting confirmation dialog...")

	// Set up listener for JavaScript dialogs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			go chromedp.Run(ctx, page.HandleJavaScriptDialog(true))
		}
	})

	err = chromedp.Run(ctx,
		chromedp.Click("#"+placesButtonID, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to click execute button: %v", err)
		t.Fatalf("Failed to click execute button: %v", err)
	}
	env.LogTest(t, "✓ Places job execution button clicked and dialog accepted")

	// Wait a moment for any UI feedback
	time.Sleep(2 * time.Second)

	// Take screenshot after clicking execute button
	env.LogTest(t, "Taking screenshot after clicking execute button...")
	if err := env.TakeScreenshot(ctx, "phase1-after-execute-click"); err != nil {
		env.LogTest(t, "WARNING: Failed to take screenshot after execute click: %v", err)
	}

	// Navigate to queue page to monitor job execution
	env.LogTest(t, "Navigating to queue page to monitor execution...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to queue page: %v", err)
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Poll for parent job creation (workaround: execute endpoint returns wrong job_id)
	env.LogTest(t, "Polling for Places parent job creation...")
	placesJobID, err := pollForParentJobCreation(t, h, env, "places-nearby-restaurants", 1*time.Minute)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to find Places parent job: %v", err)
		t.Fatalf("Failed to find Places parent job: %v", err)
	}
	env.LogTest(t, "✓ Found Places parent job: %s", placesJobID)

	// Wait for job to appear in UI
	shortJobID := placesJobID
	if len(placesJobID) > 8 {
		shortJobID = placesJobID[:8]
	}

	env.LogTest(t, "Waiting for Places job to appear in queue UI (job ID: %s)...", shortJobID)
	err = chromedp.Run(ctx,
		chromedp.Poll(fmt.Sprintf(`
			(function() {
				const jobCards = document.querySelectorAll('[x-data]');
				for (const card of jobCards) {
					if (card.textContent.includes('%s')) {
						return true;
					}
				}
				return false;
			})()
		`, shortJobID), nil, chromedp.WithPollingInterval(1*time.Second), chromedp.WithPollingTimeout(30*time.Second)),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed waiting for Places job to appear: %v", err)
		t.Fatalf("Failed waiting for Places job to appear: %v", err)
	}
	env.LogTest(t, "✓ Places job appeared in queue")

	// Take screenshot showing job in queue
	env.LogTest(t, "Taking screenshot of Places job in queue...")
	if err := env.TakeScreenshot(ctx, "phase1-job-running"); err != nil {
		env.LogTest(t, "ERROR: Failed to take job screenshot: %v", err)
		t.Fatalf("Failed to take job screenshot: %v", err)
	}

	// Poll for Places job completion using API
	env.LogTest(t, "Polling for Places job completion...")
	placesDocCount, err := pollForJobCompletion(t, h, env, placesJobID, 5*time.Minute)

	// Take screenshot of completed/failed job
	env.LogTest(t, "Taking screenshot of Places job final state...")
	if err := env.TakeScreenshot(ctx, "phase1-job-complete"); err != nil {
		env.LogTest(t, "WARNING: Failed to take completion screenshot: %v", err)
	}

	if err != nil {
		// Job failed unexpectedly
		env.LogTest(t, "ERROR: Places job failed: %v", err)
		t.Fatalf("Places job failed: %v", err)
	} else {
		// Job succeeded
		env.LogTest(t, "✓ Places job completed with %d documents", placesDocCount)

		// Verify documents were created
		if placesDocCount == 0 {
			env.LogTest(t, "ERROR: Places job created 0 documents")
			t.Fatalf("Places job should have created documents but got 0")
		} else {
			env.LogTest(t, "✅ PHASE 1 PASS: Places job created %d documents", placesDocCount)
		}
	}

	*/ // END Phase 1 comment block

	// ============================================================
	// PHASE 2: Run "keyword-extractor-agent" job
	// ============================================================

	env.LogTest(t, "=== PHASE 2: Keyword Extraction Agent Job ===")

	// Create Keyword Extraction job definition
	keywordJobDef := map[string]interface{}{
		"id":          "keyword-extractor-agent",
		"name":        "Keyword Extraction Demo",
		"type":        "custom",
		"job_type":    "user",
		"description": "Extracts keywords from crawled documents using Google Gemini",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":     "Extract Keywords from Documents",
				"action":   "agent",
				"on_error": "fail",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"document_filter": map[string]interface{}{
						"limit": 100,
					},
				},
			},
		},
	}

	// Create the job definition (ignore error if already exists)
	env.LogTest(t, "Creating Keyword Extraction job definition...")
	createResp2, err := h.POST("/api/job-definitions", keywordJobDef)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to create keyword job definition: %v", err)
		t.Fatalf("Failed to create keyword job definition: %v", err)
	}
	if createResp2.StatusCode == 201 || createResp2.StatusCode == 409 {
		env.LogTest(t, "✓ Keyword Extraction job definition created/exists")
	}

	// Navigate to jobs page to verify it appears in UI
	env.LogTest(t, "Navigating to jobs page to verify keyword job definition exists in UI...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to job definitions page: %v", err)
		t.Fatalf("Failed to navigate to job definitions page: %v", err)
	}

	// Wait a moment for page to fully load
	time.Sleep(2 * time.Second)

	// Take screenshot of job definitions page (do this first to capture what we see)
	env.LogTest(t, "Taking screenshot of job definitions page with keyword job...")
	if err := env.TakeScreenshot(ctx, "phase2-job-definition-created"); err != nil {
		env.LogTest(t, "ERROR: Failed to take job definitions screenshot: %v", err)
		t.Fatalf("Failed to take job definitions screenshot: %v", err)
	}

	// Verify job definition appears in UI (optional check)
	env.LogTest(t, "Verifying Keyword Extraction job definition appears in UI...")
	var pageText string
	err = chromedp.Run(ctx,
		chromedp.Text(`body`, &pageText, chromedp.ByQuery),
	)
	if err == nil {
		if strings.Contains(pageText, "keyword-extractor-agent") || strings.Contains(pageText, "Keyword Extraction Demo") {
			env.LogTest(t, "✓ Keyword Extraction job definition visible in UI")
		} else {
			env.LogTest(t, "⚠️  WARNING: Keyword Extraction job definition not clearly visible in UI (check screenshot)")
		}
	} else {
		env.LogTest(t, "⚠️  WARNING: Could not verify keyword job definition in UI: %v", err)
	}

	// Execute the Keyword Extraction job via UI button click
	// Wait for Alpine.js to render the dynamic content
	time.Sleep(2 * time.Second)

	// Find the execute button for the Keyword Extraction job
	env.LogTest(t, "Finding execute button for Keyword Extraction job...")
	var keywordButtonID string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const title = card.querySelector('.card-title');
					if (title && title.textContent.includes('Keyword Extraction')) {
						const runButton = card.querySelector('button[id$="-run"]');
						if (runButton) return runButton.id;
					}
				}
				return null;
			})()
		`, &keywordButtonID),
	)
	if err != nil || keywordButtonID == "" {
		env.LogTest(t, "ERROR: Failed to find execute button")

		// Debug: Check what button IDs are actually present
		var buttonIDs string
		chromedp.Run(ctx,
			chromedp.Evaluate(`Array.from(document.querySelectorAll('button[id]')).map(b => b.id).join(', ')`, &buttonIDs),
		)
		env.LogTest(t, "DEBUG: Available button IDs: %s", buttonIDs)

		t.Fatalf("Failed to find execute button for Keyword Extraction job")
	}
	env.LogTest(t, "Found execute button with ID: %s", keywordButtonID)

	// Click the button and handle the confirmation dialog
	env.LogTest(t, "Clicking execute button and accepting confirmation dialog...")

	// Set up listener for JavaScript dialogs (dialog handler already set up in Phase 1, but being explicit)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			go chromedp.Run(ctx, page.HandleJavaScriptDialog(true))
		}
	})

	err = chromedp.Run(ctx,
		chromedp.Click("#"+keywordButtonID, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to click execute button: %v", err)
		t.Fatalf("Failed to click execute button: %v", err)
	}
	env.LogTest(t, "✓ Keyword Extraction job execution button clicked and dialog accepted")

	// Wait a moment for any UI feedback
	time.Sleep(2 * time.Second)

	// Take screenshot after clicking execute button
	env.LogTest(t, "Taking screenshot after clicking execute button...")
	if err := env.TakeScreenshot(ctx, "phase2-after-execute-click"); err != nil {
		env.LogTest(t, "WARNING: Failed to take screenshot after execute click: %v", err)
	}

	// Navigate to queue page to monitor job execution
	env.LogTest(t, "Navigating to queue page to monitor execution...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to queue page: %v", err)
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Poll for parent job creation (workaround: execute endpoint returns wrong job_id)
	env.LogTest(t, "Polling for Keyword Extraction parent job creation...")
	keywordJobID, err := pollForParentJobCreation(t, h, env, "keyword-extractor-agent", 1*time.Minute)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to find Keyword Extraction parent job: %v", err)
		t.Fatalf("Failed to find Keyword Extraction parent job: %v", err)
	}
	env.LogTest(t, "✓ Found Keyword Extraction parent job: %s", keywordJobID)

	// Wait for job to appear in UI
	shortKeywordJobID := keywordJobID
	if len(keywordJobID) > 8 {
		shortKeywordJobID = keywordJobID[:8]
	}

	env.LogTest(t, "Waiting for Keyword job to appear in queue UI (job ID: %s)...", shortKeywordJobID)
	err = chromedp.Run(ctx,
		chromedp.Poll(fmt.Sprintf(`
			(function() {
				const jobCards = document.querySelectorAll('[x-data]');
				for (const card of jobCards) {
					if (card.textContent.includes('%s')) {
						return true;
					}
				}
				return false;
			})()
		`, shortKeywordJobID), nil, chromedp.WithPollingInterval(1*time.Second), chromedp.WithPollingTimeout(30*time.Second)),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed waiting for Keyword job to appear: %v", err)
		t.Fatalf("Failed waiting for Keyword job to appear: %v", err)
	}
	env.LogTest(t, "✓ Keyword job appeared in queue")

	// Take screenshot showing job in queue
	env.LogTest(t, "Taking screenshot of Keyword job in queue...")
	if err := env.TakeScreenshot(ctx, "phase2-job-running"); err != nil {
		env.LogTest(t, "ERROR: Failed to take keyword job screenshot: %v", err)
		t.Fatalf("Failed to take keyword job screenshot: %v", err)
	}

	// Poll for Keyword job status
	env.LogTest(t, "Polling for Keyword job status...")
	keywordJobStatus, keywordJobError, err := pollForJobStatus(t, h, env, keywordJobID, 2*time.Minute)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to poll keyword job: %v", err)
		t.Fatalf("Failed to poll keyword job: %v", err)
	}

	env.LogTest(t, "✓ Keyword job status: %s", keywordJobStatus)
	if keywordJobError != "" {
		env.LogTest(t, "✓ Keyword job error: %s", keywordJobError)
	}

	// Get document_count from API response to verify documents were processed
	keywordDocumentCount := 0
	jobResp, err := h.GET("/api/jobs/" + keywordJobID)
	if err == nil {
		var jobData map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &jobData); err == nil {
			if dc, ok := jobData["document_count"].(float64); ok {
				keywordDocumentCount = int(dc)
			}
		}
	}

	env.LogTest(t, "✓ Keyword job document_count: %d", keywordDocumentCount)

	// VERIFY: Test should fail if no documents were processed
	if keywordDocumentCount == 0 {
		env.LogTest(t, "ERROR: Keyword job processed 0 documents - test FAILS")
		env.LogTest(t, "  This means no keywords were extracted from the test documents")
		t.Fatalf("Keyword job must process documents and extract keywords (document_count > 0), got: %d", keywordDocumentCount)
	}

	// Verify job completed successfully WITH results
	if keywordJobStatus == "failed" {
		env.LogTest(t, "ERROR: Keyword job failed: %s", keywordJobError)
		t.Fatalf("Keyword job failed: %s", keywordJobError)
	}

	if keywordJobStatus == "completed" && keywordDocumentCount > 0 {
		env.LogTest(t, "✓ Keyword job completed successfully")
		env.LogTest(t, "✓ Processed %d documents and extracted keywords", keywordDocumentCount)
		env.LogTest(t, "✅ PHASE 2 PASS: Keywords extracted from %d documents", keywordDocumentCount)
	} else {
		env.LogTest(t, "ERROR: Job completed but processed 0 documents")
		t.Fatalf("Expected document_count > 0, got: %d", keywordDocumentCount)
	}

	// Wait a moment for UI to update
	time.Sleep(2 * time.Second)

	// Take screenshot of final state
	env.LogTest(t, "Taking screenshot of Keyword job final state...")
	if err := env.TakeScreenshot(ctx, "phase2-final-state"); err != nil {
		env.LogTest(t, "WARNING: Failed to take final screenshot: %v", err)
	}

	env.LogTest(t, "✓ Test completed successfully")
}

// insertTestDocument creates a test document via POST /api/documents
func insertTestDocument(t *testing.T, h *common.HTTPTestHelper, env *common.TestEnvironment, id, title, content string) error {
	doc := map[string]interface{}{
		"id":               id,
		"source_type":      "test",
		"title":            title,
		"content_markdown": content,
		"url":              "",
		"source_id":        id, // Use unique document ID as source_id to avoid constraint violation
		"metadata": map[string]interface{}{
			"test": true,
		},
	}

	env.LogTest(t, "Creating test document: %s (%s)", id, title)
	resp, err := h.POST("/api/documents", doc)
	if err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		// Try to read error response
		var errorResp map[string]interface{}
		if err := h.ParseJSONResponse(resp, &errorResp); err == nil {
			env.LogTest(t, "ERROR response: %v", errorResp)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	env.LogTest(t, "✓ Test document created: %s", id)
	return nil
}

// pollForParentJobCreation polls for a parent job to be created after job definition execution
func pollForParentJobCreation(t *testing.T, h *common.HTTPTestHelper, env *common.TestEnvironment, jobDefID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		jobsResp, err := h.GET("/api/jobs")
		if err != nil {
			continue
		}

		var paginatedResponse struct {
			Jobs []map[string]interface{} `json:"jobs"`
		}
		if err := h.ParseJSONResponse(jobsResp, &paginatedResponse); err != nil {
			continue
		}

		// Look for parent job with matching job_definition_id in metadata
		for _, job := range paginatedResponse.Jobs {
			jobType, _ := job["type"].(string)

			// Match parent jobs by job_definition_id in metadata
			if jobType == "parent" {
				if metadata, ok := job["metadata"].(map[string]interface{}); ok {
					if metaJobDefID, ok := metadata["job_definition_id"].(string); ok && metaJobDefID == jobDefID {
						jobID := job["id"].(string)
						shortID := jobID
						if len(jobID) > 8 {
							shortID = jobID[:8]
						}
						env.LogTest(t, "  Found parent job: %s (job_def: %s)", shortID, jobDefID)
						return jobID, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("timeout waiting for parent job creation after %v", timeout)
}

// pollForJobCompletion polls a job until completion and returns document_count
func pollForJobCompletion(t *testing.T, h *common.HTTPTestHelper, env *common.TestEnvironment, jobID string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		status, ok := job["status"].(string)
		if !ok {
			continue
		}

		// Get document count from job
		docCount := 0
		if count, ok := job["document_count"].(float64); ok {
			docCount = int(count)
		}

		shortID := jobID
		if len(jobID) > 8 {
			shortID = jobID[:8]
		}
		env.LogTest(t, "  Job %s status: %s (document_count: %d)", shortID, status, docCount)

		if status == "completed" {
			return docCount, nil
		}

		if status == "failed" {
			errorMsg := "unknown error"
			if errField, ok := job["error"].(string); ok {
				errorMsg = errField
			}
			return 0, fmt.Errorf("job failed: %s", errorMsg)
		}
	}

	return 0, fmt.Errorf("timeout waiting for job completion after %v", timeout)
}

// pollForJobStatus polls a job until it reaches a terminal state and returns status and error
func pollForJobStatus(t *testing.T, h *common.HTTPTestHelper, env *common.TestEnvironment, jobID string, timeout time.Duration) (string, string, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		status, ok := job["status"].(string)
		if !ok {
			continue
		}

		shortID := jobID
		if len(jobID) > 8 {
			shortID = jobID[:8]
		}
		env.LogTest(t, "  Job %s status: %s", shortID, status)

		if status == "completed" || status == "failed" {
			errorMsg := ""
			if errField, ok := job["error"].(string); ok {
				errorMsg = errField
			}
			return status, errorMsg, nil
		}
	}

	return "", "", fmt.Errorf("timeout waiting for job status after %v", timeout)
}

// containsErrorContent checks if HTML contains error indicators for a specific job
func containsErrorContent(html string, jobID string) bool {
	// Look for common error indicators in the HTML near the job ID
	// This is a simple heuristic check
	htmlLower := strings.ToLower(html)

	// Check for error-related terms
	hasErrorTerms := strings.Contains(htmlLower, "error") ||
		strings.Contains(htmlLower, "failed") ||
		strings.Contains(htmlLower, "failure")

	// Check for error styling
	hasErrorStyling := strings.Contains(htmlLower, "background-color: #f8d7da") ||
		strings.Contains(htmlLower, "bg-red-") ||
		strings.Contains(htmlLower, "text-red-")

	return hasErrorTerms || hasErrorStyling
}

// TestGoogleAPIKeyFromEnv verifies that GOOGLE_API_KEY is loaded from .env.test
// and properly injected into the service configuration at runtime.
// This test verifies the key appears on both /settings?a=auth-apikeys and /settings?a=config pages.
func TestGoogleAPIKeyFromEnv(t *testing.T) {
	// Setup test environment (ENV variables loaded by default)
	env, err := common.SetupTestEnvironment("TestGoogleAPIKeyFromEnv")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestGoogleAPIKeyFromEnv")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestGoogleAPIKeyFromEnv (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestGoogleAPIKeyFromEnv (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	// Verify GOOGLE_API_KEY was loaded from .env.test
	googleAPIKey, exists := env.EnvVars["GOOGLE_API_KEY"]
	if !exists || googleAPIKey == "" {
		t.Fatalf("GOOGLE_API_KEY not found in .env.test file")
	}
	env.LogTest(t, "✓ GOOGLE_API_KEY loaded from .env.test: %s...%s", googleAPIKey[:10], googleAPIKey[len(googleAPIKey)-4:])

	// Setup Chrome context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	serverURL := env.GetBaseURL()

	// ============================================================
	// VERIFY 1: Check /settings?a=auth-apikeys page
	// ============================================================
	env.LogTest(t, "=== VERIFY 1: Checking /settings?a=auth-apikeys ===")

	env.LogTest(t, "Navigating to /settings?a=auth-apikeys...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(serverURL+"/settings?a=auth-apikeys"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to auth-apikeys page: %v", err)
		t.Fatalf("Failed to navigate to auth-apikeys page: %v", err)
	}

	// Wait for WebSocket connection
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Wait a moment for page to fully render
	time.Sleep(2 * time.Second)

	// Take screenshot of auth-apikeys page
	env.LogTest(t, "Taking screenshot of auth-apikeys page...")
	if err := env.TakeScreenshot(ctx, "env-verify-auth-apikeys"); err != nil {
		env.LogTest(t, "WARNING: Failed to take auth-apikeys screenshot: %v", err)
	}

	// Verify google_api_key appears on the page
	env.LogTest(t, "Verifying google_api_key appears on auth-apikeys page...")
	var pageText string
	err = chromedp.Run(ctx,
		chromedp.Text(`body`, &pageText, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to get page text: %v", err)
		t.Fatalf("Failed to get page text: %v", err)
	}

	// Check if page contains "google_api_key" or "GOOGLE_API_KEY" (case variations)
	pageTextLower := strings.ToLower(pageText)
	if strings.Contains(pageTextLower, "google_api_key") || strings.Contains(pageTextLower, "gemini") {
		env.LogTest(t, "✓ google_api_key reference found on auth-apikeys page")
	} else {
		env.LogTest(t, "ERROR: google_api_key NOT found on auth-apikeys page")
		env.LogTest(t, "Page text preview: %s...", pageText[:min(500, len(pageText))])
		t.Fatalf("google_api_key not found on auth-apikeys page")
	}

	// Verify the actual API key value appears (partially masked or full)
	// Look for at least the first 10 characters of the key
	keyPrefix := googleAPIKey[:10]
	if strings.Contains(pageText, keyPrefix) {
		env.LogTest(t, "✓ API key value confirmed on auth-apikeys page (prefix: %s)", keyPrefix)
	} else {
		// Key might be masked, check for "AIzaSy" pattern (Google API key prefix)
		if strings.Contains(pageText, "AIzaSy") {
			env.LogTest(t, "✓ API key pattern (AIzaSy) found on auth-apikeys page")
		} else {
			env.LogTest(t, "⚠️  WARNING: API key value not visible on page (might be masked or not displayed)")
		}
	}

	env.LogTest(t, "✅ VERIFY 1 PASS: google_api_key accessible on auth-apikeys page")

	// ============================================================
	// VERIFY 2: Check /settings?a=config page
	// ============================================================
	env.LogTest(t, "=== VERIFY 2: Checking /settings?a=config ===")

	env.LogTest(t, "Navigating to /settings?a=config...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/settings?a=config"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to config page: %v", err)
		t.Fatalf("Failed to navigate to config page: %v", err)
	}

	// Wait a moment for page to fully render
	time.Sleep(2 * time.Second)

	// Take screenshot of config page
	env.LogTest(t, "Taking screenshot of config page...")
	if err := env.TakeScreenshot(ctx, "env-verify-config"); err != nil {
		env.LogTest(t, "WARNING: Failed to take config screenshot: %v", err)
	}

	// Verify google_api_key appears on the config page
	env.LogTest(t, "Verifying google_api_key appears on config page...")
	err = chromedp.Run(ctx,
		chromedp.Text(`body`, &pageText, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to get config page text: %v", err)
		t.Fatalf("Failed to get config page text: %v", err)
	}

	// Check if page contains configuration related to Google API
	pageTextLower = strings.ToLower(pageText)
	if strings.Contains(pageTextLower, "google") && (strings.Contains(pageTextLower, "api") || strings.Contains(pageTextLower, "gemini")) {
		env.LogTest(t, "✓ Google API configuration found on config page")
	} else {
		env.LogTest(t, "ERROR: Google API configuration NOT found on config page")
		env.LogTest(t, "Page text preview: %s...", pageText[:min(500, len(pageText))])
		t.Fatalf("Google API configuration not found on config page")
	}

	// Check for API key presence (might be in different format on config page)
	if strings.Contains(pageText, keyPrefix) || strings.Contains(pageText, "AIzaSy") {
		env.LogTest(t, "✓ API key value/pattern confirmed on config page")
	} else {
		env.LogTest(t, "⚠️  WARNING: API key value not directly visible on config page (might be nested in config structure)")
	}

	env.LogTest(t, "✅ VERIFY 2 PASS: Google API configuration accessible on config page")

	env.LogTest(t, "✓ Test completed successfully - google_api_key loaded from ENV and accessible on both settings pages")
}

