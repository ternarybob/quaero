package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs/types"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// TestErrorToleranceEndToEnd provides comprehensive end-to-end validation
// of error tolerance enforcement using table-driven subtests
func TestErrorToleranceEndToEnd(t *testing.T) {
	tests := []struct {
		name                string
		maxChildFailures    int
		failureAction       string
		failingURLCount     int
		successURLCount     int
		expectParentFailed  bool
		expectChildrenCancelled bool
		expectWarning       bool
		expectEvent         bool
	}{
		{
			name:                "stop_all: threshold exceeded triggers cancellation",
			maxChildFailures:    2,
			failureAction:       "stop_all",
			failingURLCount:     3,
			successURLCount:     2,
			expectParentFailed:  true,
			expectChildrenCancelled: true,
			expectEvent:         true,
		},
		{
			name:                "stop_all: under threshold continues",
			maxChildFailures:    5,
			failureAction:       "stop_all",
			failingURLCount:     2,
			successURLCount:     2,
			expectParentFailed:  false,
			expectChildrenCancelled: false,
			expectEvent:         false,
		},
		{
			name:                "continue: threshold exceeded does not cancel",
			maxChildFailures:    2,
			failureAction:       "continue",
			failingURLCount:     3,
			successURLCount:     2,
			expectParentFailed:  false,
			expectChildrenCancelled: false,
			expectEvent:         false,
		},
		{
			name:                "mark_warning: threshold exceeded sets warning",
			maxChildFailures:    1,
			failureAction:       "mark_warning",
			failingURLCount:     2,
			successURLCount:     1,
			expectParentFailed:  false,
			expectChildrenCancelled: false,
			expectWarning:       true,
			expectEvent:         false,
		},
		{
			name:                "zero threshold: no enforcement",
			maxChildFailures:    0,
			failureAction:       "stop_all",
			failingURLCount:     10,
			successURLCount:     0,
			expectParentFailed:  false,
			expectChildrenCancelled: false,
			expectEvent:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize test environment
			config, cleanup := LoadTestConfig(t)
			defer cleanup()

			ctx := context.Background()
			app := InitializeTestApp(t, config)
			defer app.Close()

			// Create test HTTP server with controllable failures
			server := createTestHTTPServer(t)
			defer server.Close()

			// Create job definition with error tolerance
			jobDef := createJobDefinition(tt.maxChildFailures, tt.failureAction)
			err := app.StorageManager.JobDefinitionStorage().SaveJobDefinition(ctx, jobDef)
			if err != nil {
				t.Fatalf("Failed to save job definition: %v", err)
			}

			// Create parent job
			parentJob := createParentJob(tt.name)
			err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
			if err != nil {
				t.Fatalf("Failed to save parent job: %v", err)
			}

			// Subscribe to EventJobFailed
			eventChan := make(chan interfaces.Event, 10)
			app.EventService.Subscribe(interfaces.EventJobFailed, func(ctx context.Context, event interfaces.Event) error {
				eventChan <- event
				return nil
			})

			// Create crawler job executor
			crawler := createCrawlerJob(app, t)

			// Execute failing URLs
			for i := 0; i < tt.failingURLCount; i++ {
				msg := createJobMessage(parentJob.ID, server.URL+"/fail", i, jobDef.ID)

				// Create child job record
				childJob := createChildJob(msg.ID, parentJob.ID, fmt.Sprintf("Failing Child %d", i))
				if err := app.StorageManager.JobStorage().SaveJob(ctx, childJob); err != nil {
					t.Fatalf("Failed to save child job: %v", err)
				}

				// Execute crawler job
				if err := crawler.Execute(ctx, msg); err != nil {
					t.Logf("Expected failure for URL %d: %v", i, err)
				}

				// Check if parent was failed due to threshold
				jobInterface, _ := app.StorageManager.JobStorage().GetJob(ctx, parentJob.ID)
				if job, ok := jobInterface.(*models.CrawlJob); ok {
					if job.Status == models.JobStatusFailed && tt.expectParentFailed {
						t.Logf("Parent job failed after %d failures as expected", i+1)
						break // Stop processing
					}
				}
			}

			// Execute success URLs
			for i := 0; i < tt.successURLCount; i++ {
				msg := createJobMessage(parentJob.ID, server.URL+"/success", i+tt.failingURLCount, jobDef.ID)

				// Create child job record
				childJob := createChildJob(msg.ID, parentJob.ID, fmt.Sprintf("Success Child %d", i))
				if err := app.StorageManager.JobStorage().SaveJob(ctx, childJob); err != nil {
					t.Fatalf("Failed to save child job: %v", err)
				}

				// Execute crawler job
				if err := crawler.Execute(ctx, msg); err != nil {
					t.Errorf("Unexpected failure for success URL %d: %v", i, err)
				}
			}

			// Poll parent job status
			finalJob := pollParentJobStatus(t, ctx, app, parentJob.ID, 10*time.Second)

			// Verify parent job status
			if tt.expectParentFailed {
				if finalJob.Status != models.JobStatusFailed {
					t.Errorf("Expected parent job to be failed, got %s", finalJob.Status)
				}
				if finalJob.Error == "" {
					t.Error("Expected error message on failed parent job")
				}
				if !strings.Contains(finalJob.Error, "Error tolerance exceeded") {
					t.Errorf("Expected error to contain 'Error tolerance exceeded', got: %s", finalJob.Error)
				}
			} else {
				if finalJob.Status == models.JobStatusFailed {
					t.Errorf("Expected parent job not to be failed, got error: %s", finalJob.Error)
				}
			}

			// Verify warning message
			if tt.expectWarning {
				if finalJob.Error == "" {
					t.Error("Expected warning message in job.Error field")
				}
				if !strings.Contains(strings.ToLower(finalJob.Error), "warning") &&
					!strings.Contains(strings.ToLower(finalJob.Error), "threshold") {
					t.Errorf("Expected warning about threshold, got: %s", finalJob.Error)
				}
			}

			// Verify child cancellation
			if tt.expectChildrenCancelled {
				cancelledCount := countCancelledChildren(t, ctx, app, parentJob.ID)
				if cancelledCount == 0 {
					t.Error("Expected some children to be cancelled")
				}
				t.Logf("Cancelled %d children as expected", cancelledCount)
			} else {
				cancelledCount := countCancelledChildren(t, ctx, app, parentJob.ID)
				if cancelledCount > 0 {
					t.Errorf("Expected no children to be cancelled, but %d were", cancelledCount)
				}
			}

			// Verify EventJobFailed was published
			if tt.expectEvent {
				select {
				case event := <-eventChan:
					validateEventPayload(t, event, parentJob.ID, tt.maxChildFailures)
				case <-time.After(2 * time.Second):
					t.Error("Expected EventJobFailed to be published, but timed out waiting")
				}
			} else {
				select {
				case event := <-eventChan:
					if payload, ok := event.Payload.(map[string]interface{}); ok {
						if jobID, _ := payload["job_id"].(string); jobID == parentJob.ID {
							t.Error("Did not expect EventJobFailed for parent job")
						}
					}
				case <-time.After(500 * time.Millisecond):
					// No event received, as expected
				}
			}
		})
	}
}

// TestErrorToleranceNoConfig verifies that jobs without error tolerance config
// do not enforce thresholds
func TestErrorToleranceNoConfig(t *testing.T) {
	config, cleanup := LoadTestConfig(t)
	defer cleanup()

	ctx := context.Background()
	app := InitializeTestApp(t, config)
	defer app.Close()

	// Create test HTTP server
	server := createTestHTTPServer(t)
	defer server.Close()

	// Create job definition WITHOUT error tolerance
	jobDef := &models.JobDefinition{
		ID:          "test-no-config",
		Name:        "No Config Test",
		Type:        models.JobDefinitionTypeCrawler,
		Sources:     []string{"test"},
		Steps:       []models.JobStep{{Name: "crawl", Action: "crawl"}},
		Enabled:     true,
		ErrorTolerance: nil, // No error tolerance configured
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := app.StorageManager.JobDefinitionStorage().SaveJobDefinition(ctx, jobDef)
	if err != nil {
		t.Fatalf("Failed to save job definition: %v", err)
	}

	// Create parent job
	parentJob := createParentJob("no-config-test")
	err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
	if err != nil {
		t.Fatalf("Failed to save parent job: %v", err)
	}

	// Create crawler job executor
	crawler := createCrawlerJob(app, t)

	// Execute many failing URLs (should not trigger enforcement)
	for i := 0; i < 10; i++ {
		msg := createJobMessage(parentJob.ID, server.URL+"/fail", i, jobDef.ID)

		childJob := createChildJob(msg.ID, parentJob.ID, fmt.Sprintf("Failing Child %d", i))
		if err := app.StorageManager.JobStorage().SaveJob(ctx, childJob); err != nil {
			t.Fatalf("Failed to save child job: %v", err)
		}

		if err := crawler.Execute(ctx, msg); err != nil {
			t.Logf("Expected failure for URL %d: %v", i, err)
		}
	}

	// Verify parent job is not failed
	jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, parentJob.ID)
	if err != nil {
		t.Fatalf("Failed to get parent job: %v", err)
	}
	job := jobInterface.(*models.CrawlJob)

	if job.Status == models.JobStatusFailed {
		t.Errorf("Expected parent job not to be failed without config, got error: %s", job.Error)
	}

	// Verify no children were cancelled
	cancelledCount := countCancelledChildren(t, ctx, app, parentJob.ID)
	if cancelledCount > 0 {
		t.Errorf("Expected no children to be cancelled, but %d were", cancelledCount)
	}
}

// Helper functions

// createTestHTTPServer creates a test server that returns 500 for /fail and 200 for everything else
func createTestHTTPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "Test error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body><h1>Test Content</h1><p>Success page</p></body></html>"))
	}))
}

// createJobDefinition creates a job definition with error tolerance config
func createJobDefinition(maxFailures int, action string) *models.JobDefinition {
	return &models.JobDefinition{
		ID:          fmt.Sprintf("test-job-def-%d-%s", maxFailures, action),
		Name:        fmt.Sprintf("Test Job Definition (max=%d, action=%s)", maxFailures, action),
		Type:        models.JobDefinitionTypeCrawler,
		Description: "End-to-end test job definition",
		Sources:     []string{"test"},
		Steps: []models.JobStep{
			{
				Name:    "crawl",
				Action:  "crawl",
				Config:  map[string]interface{}{},
				OnError: models.ErrorStrategyContinue,
			},
		},
		Enabled: true,
		ErrorTolerance: &models.ErrorTolerance{
			MaxChildFailures: maxFailures,
			FailureAction:    action,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// createParentJob creates a parent CrawlJob
func createParentJob(testName string) *models.CrawlJob {
	return &models.CrawlJob{
		ID:        fmt.Sprintf("parent-%s-%d", testName, time.Now().Unix()),
		ParentID:  "",
		JobType:   models.JobTypeParent,
		Name:      fmt.Sprintf("Parent Job: %s", testName),
		Status:    models.JobStatusRunning,
		CreatedAt: time.Now(),
		Progress: models.CrawlProgress{
			TotalURLs:     0,
			CompletedURLs: 0,
			PendingURLs:   0,
			FailedURLs:    0,
		},
	}
}

// createChildJob creates a child CrawlJob record
func createChildJob(id, parentID, name string) *models.CrawlJob {
	return &models.CrawlJob{
		ID:        id,
		ParentID:  parentID,
		JobType:   models.JobTypeCrawlerURL,
		Name:      name,
		Status:    models.JobStatusPending,
		CreatedAt: time.Now(),
	}
}

// createJobMessage creates a JobMessage for crawler execution
func createJobMessage(parentID, url string, index int, jobDefID string) *queue.JobMessage {
	return &queue.JobMessage{
		ID:              fmt.Sprintf("%s-child-%d", parentID, index),
		Type:            "crawler_url",
		URL:             url,
		Depth:           0,
		ParentID:        parentID,
		JobDefinitionID: jobDefID,
		Config: map[string]interface{}{
			"max_depth":      1,
			"follow_links":   false,
			"source_type":    "test",
			"entity_type":    "url",
		},
	}
}

// createCrawlerJob creates a CrawlerJob instance with minimal dependencies
func createCrawlerJob(app *TestApp, t *testing.T) *types.CrawlerJob {
	deps := &types.CrawlerJobDeps{
		DocumentStorage: app.StorageManager.DocumentStorage(),
		JobStorage:      app.StorageManager.JobStorage(),
	}

	baseJob := types.NewBaseJob(
		app.Logger,
		app.LogService,
		app.StorageManager.JobStorage(),
		app.EventService,
		app.QueueManager,
	)

	return types.NewCrawlerJob(baseJob, deps)
}

// pollParentJobStatus polls the parent job status until timeout
func pollParentJobStatus(t *testing.T, ctx context.Context, app *TestApp, jobID string, timeout time.Duration) *models.CrawlJob {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			// Return current state on timeout
			jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, jobID)
			if err != nil {
				t.Fatalf("Timeout waiting for job status, and failed to get job: %v", err)
			}
			return jobInterface.(*models.CrawlJob)
		case <-ticker.C:
			jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, jobID)
			if err != nil {
				continue
			}
			job := jobInterface.(*models.CrawlJob)

			// Return when job reaches a terminal state or has error set
			if job.Status == models.JobStatusFailed ||
				job.Status == models.JobStatusCompleted ||
				job.Error != "" {
				return job
			}
		}
	}
}

// countCancelledChildren counts child jobs with cancelled status
func countCancelledChildren(t *testing.T, ctx context.Context, app *TestApp, parentID string) int {
	opts := &interfaces.JobListOptions{
		ParentID: parentID,
		Status:   string(models.JobStatusCancelled),
		Limit:    0,
	}

	children, err := app.StorageManager.JobStorage().ListJobs(ctx, opts)
	if err != nil {
		t.Logf("Failed to list cancelled children: %v", err)
		return 0
	}

	return len(children)
}

// validateEventPayload validates that EventJobFailed contains expected threshold fields
func validateEventPayload(t *testing.T, event interfaces.Event, expectedJobID string, expectedThreshold int) {
	if event.Type != interfaces.EventJobFailed {
		t.Errorf("Expected EventJobFailed, got %s", event.Type)
		return
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		t.Error("Event payload is not a map")
		return
	}

	// Verify job_id
	jobID, ok := payload["job_id"].(string)
	if !ok || jobID != expectedJobID {
		t.Errorf("Expected job_id %s, got %v", expectedJobID, jobID)
	}

	// Verify threshold fields (with alias support)
	// Try primary keys first, then aliases
	failedChildren := getIntField(payload, "failed_children", "child_failure_count")
	errorTolerance := getIntField(payload, "error_tolerance", "threshold")
	childCount := getIntField(payload, "child_count", "")

	if failedChildren == 0 {
		t.Error("Expected 'failed_children' or 'child_failure_count' field in payload")
	}

	if errorTolerance == 0 {
		t.Error("Expected 'error_tolerance' or 'threshold' field in payload")
	}

	if errorTolerance != expectedThreshold {
		t.Errorf("Expected error_tolerance=%d, got %d", expectedThreshold, errorTolerance)
	}

	if childCount == 0 {
		t.Log("Warning: 'child_count' field not present in payload")
	}

	t.Logf("Event payload validated: failed_children=%d, error_tolerance=%d, child_count=%d",
		failedChildren, errorTolerance, childCount)
}

// getIntField tries multiple keys and returns the first found int value
func getIntField(payload map[string]interface{}, primaryKey, aliasKey string) int {
	// Try primary key
	if val, ok := payload[primaryKey]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}

	// Try alias key if provided
	if aliasKey != "" {
		if val, ok := payload[aliasKey]; ok {
			switch v := val.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			}
		}
	}

	return 0
}

// TestErrorToleranceEventAliasKeys validates that event handlers support alias keys
func TestErrorToleranceEventAliasKeys(t *testing.T) {
	config, cleanup := LoadTestConfig(t)
	defer cleanup()

	ctx := context.Background()
	app := InitializeTestApp(t, config)
	defer app.Close()

	// Subscribe to EventJobFailed
	var capturedPayload map[string]interface{}
	var mu sync.Mutex

	app.EventService.Subscribe(interfaces.EventJobFailed, func(ctx context.Context, event interfaces.Event) error {
		mu.Lock()
		defer mu.Unlock()
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			capturedPayload = payload
		}
		return nil
	})

	// Publish test event with both primary and alias keys
	testEvent := interfaces.Event{
		Type: interfaces.EventJobFailed,
		Payload: map[string]interface{}{
			"job_id":             "test-job-123",
			"status":             "failed",
			"error":              "Test error",
			"failed_children":    3,
			"child_failure_count": 3, // Alias key
			"error_tolerance":    2,
			"threshold":          2, // Alias key
			"child_count":        5,
		},
	}

	if err := app.EventService.Publish(ctx, testEvent); err != nil {
		t.Fatalf("Failed to publish test event: %v", err)
	}

	// Wait for event processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if capturedPayload == nil {
		t.Fatal("Event was not captured")
	}

	// Verify both primary and alias keys are present
	if _, ok := capturedPayload["failed_children"]; !ok {
		t.Error("Expected 'failed_children' key in payload")
	}
	if _, ok := capturedPayload["child_failure_count"]; !ok {
		t.Error("Expected 'child_failure_count' alias key in payload")
	}
	if _, ok := capturedPayload["error_tolerance"]; !ok {
		t.Error("Expected 'error_tolerance' key in payload")
	}
	if _, ok := capturedPayload["threshold"]; !ok {
		t.Error("Expected 'threshold' alias key in payload")
	}
	if _, ok := capturedPayload["child_count"]; !ok {
		t.Error("Expected 'child_count' key in payload")
	}

	t.Log("Event payload correctly contains both primary and alias keys")
}
