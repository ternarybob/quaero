package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// TestErrorToleranceStopAllIntegration tests end-to-end error tolerance with stop_all action
// This test runs the actual crawler with a test HTTP server and verifies:
// - Failures trigger immediate threshold enforcement
// - EventJobFailed is published with correct payload
// - Running and pending children are cancelled
// - Parent job is marked as failed with appropriate error message
func TestErrorToleranceStopAllIntegration(t *testing.T) {
	config, cleanup := LoadTestConfig(t)
	defer cleanup()

	ctx := context.Background()
	app := InitializeTestApp(t, config)
	defer app.Close()

	// Create test HTTP server that returns errors for specific paths
	failCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			// Return 500 error
			http.Error(w, "Test error", http.StatusInternalServerError)
			failCount++
			return
		}
		// Success path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test content</body></html>"))
	}))
	defer server.Close()

	// Event collector to capture EventJobFailed
	var capturedEvents []interfaces.Event
	var eventMutex sync.Mutex

	eventCollector := func(ctx context.Context, event interfaces.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		t.Logf("Captured event: %s with payload: %+v", event.Type, event.Payload)
		return nil
	}

	// Subscribe to EventJobFailed
	app.EventService.Subscribe(interfaces.EventJobFailed, eventCollector)

	// Create job definition with error tolerance (max 2 failures, action: stop_all)
	jobDef := &models.JobDefinition{
		ID:          "test-stop-all-integration",
		Name:        "Stop All Integration Test",
		Type:        models.JobDefinitionTypeCrawler,
		Description: "Integration test for stop_all error tolerance",
		Sources:     []string{"test"},
		Steps: []models.JobStep{
			{
				Name:    "crawl",
				Action:  "crawl",
				Config:  map[string]interface{}{},
				OnError: models.ErrorStrategyContinue,
			},
		},
		Schedule: "",
		Timeout:  "5m",
		Enabled:  true,
		ErrorTolerance: &models.ErrorTolerance{
			MaxChildFailures: 2,
			FailureAction:    "stop_all",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := app.StorageManager.JobDefinitionStorage().SaveJobDefinition(ctx, jobDef)
	if err != nil {
		t.Fatalf("Failed to save job definition: %v", err)
	}

	// Create parent job
	parentJobID := "parent-stop-all-integration"
	parentJob := &models.CrawlJob{
		ID:        parentJobID,
		ParentID:  "",
		JobType:   models.JobTypeParent,
		Name:      "Parent Job Stop All Integration",
		Status:    models.JobStatusPending,
		CreatedAt: time.Now(),
		Progress: models.CrawlProgress{
			TotalURLs:     0,
			CompletedURLs: 0,
			PendingURLs:   0,
			FailedURLs:    0,
		},
	}
	err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
	if err != nil {
		t.Fatalf("Failed to save parent job: %v", err)
	}

	// Enqueue URLs: 3 that will fail, 2 that will succeed
	urls := []string{
		server.URL + "/fail",
		server.URL + "/fail",
		server.URL + "/fail",
		server.URL + "/success1",
		server.URL + "/success2",
	}

	for i, url := range urls {
		msg := &queue.JobMessage{
			ID:              fmt.Sprintf("child-%d", i),
			Type:            "crawler_url",
			URL:             url,
			Depth:           0,
			ParentID:        parentJobID,
			JobDefinitionID: jobDef.ID,
			Config: map[string]interface{}{
				"max_depth":    1,
				"follow_links": false,
			},
		}

		// Create child job record
		childJob := &models.CrawlJob{
			ID:        msg.ID,
			ParentID:  parentJobID,
			JobType:   models.JobTypeCrawlerURL,
			Name:      fmt.Sprintf("Child %d", i),
			Status:    models.JobStatusPending,
			CreatedAt: time.Now(),
		}
		err = app.StorageManager.JobStorage().SaveJob(ctx, childJob)
		if err != nil {
			t.Fatalf("Failed to save child job: %v", err)
		}

		err = app.QueueManager.Enqueue(ctx, msg)
		if err != nil {
			t.Fatalf("Failed to enqueue message %d: %v", i, err)
		}
	}

	// Update parent job progress
	parentJob.Status = models.JobStatusRunning
	parentJob.Progress.TotalURLs = len(urls)
	parentJob.Progress.PendingURLs = len(urls)
	err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
	if err != nil {
		t.Fatalf("Failed to update parent job: %v", err)
	}

	// Wait for job processing and threshold enforcement
	// Poll parent job status until it's failed or timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	parentFailed := false
	for !parentFailed {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for parent job to fail")
		case <-ticker.C:
			jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, parentJobID)
			if err != nil {
				t.Fatalf("Failed to get parent job: %v", err)
			}
			job := jobInterface.(*models.CrawlJob)

			t.Logf("Parent job status: %s, failed URLs: %d", job.Status, job.Progress.FailedURLs)

			if job.Status == models.JobStatusFailed {
				parentFailed = true

				// Verify error message
				if job.Error == "" {
					t.Error("Expected error message on failed parent job")
				}
				if job.Error != "" && !contains(job.Error, "Error tolerance exceeded") {
					t.Errorf("Expected error message to contain 'Error tolerance exceeded', got: %s", job.Error)
				}

				t.Logf("Parent job failed with error: %s", job.Error)
			}
		}
	}

	// Verify EventJobFailed was published
	eventMutex.Lock()
	foundEvent := false
	var failedEventPayload map[string]interface{}
	for _, event := range capturedEvents {
		if event.Type == interfaces.EventJobFailed {
			if payload, ok := event.Payload.(map[string]interface{}); ok {
				if jobID, ok := payload["job_id"].(string); ok && jobID == parentJobID {
					foundEvent = true
					failedEventPayload = payload
					break
				}
			}
		}
	}
	eventMutex.Unlock()

	if !foundEvent {
		t.Error("Expected EventJobFailed to be published for parent job")
	} else {
		// Verify event payload contains threshold fields
		if _, ok := failedEventPayload["failed_children"]; !ok {
			t.Error("Expected 'failed_children' field in EventJobFailed payload")
		}
		if _, ok := failedEventPayload["error_tolerance"]; !ok {
			t.Error("Expected 'error_tolerance' field in EventJobFailed payload")
		}
		if _, ok := failedEventPayload["child_count"]; !ok {
			t.Error("Expected 'child_count' field in EventJobFailed payload")
		}

		t.Logf("EventJobFailed payload: %+v", failedEventPayload)
	}

	// Verify running/pending children were cancelled
	cancelledCount := 0
	stillRunning := 0
	for i := 0; i < len(urls); i++ {
		childID := fmt.Sprintf("child-%d", i)
		jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, childID)
		if err != nil {
			t.Logf("Child job %s not found (may not have been processed): %v", childID, err)
			continue
		}

		child := jobInterface.(*models.CrawlJob)
		t.Logf("Child %s status: %s", childID, child.Status)

		if child.Status == models.JobStatusCancelled {
			cancelledCount++
		}
		if child.Status == models.JobStatusRunning || child.Status == models.JobStatusPending {
			stillRunning++
		}
	}

	t.Logf("Cancelled: %d, Still running/pending: %d", cancelledCount, stillRunning)

	// At least some children should have been cancelled
	if cancelledCount == 0 && stillRunning > 0 {
		t.Error("Expected at least some children to be cancelled after threshold enforcement")
	}

	// Verify at least the expected number of failures occurred
	if failCount < 2 {
		t.Errorf("Expected at least 2 failures to trigger threshold, got %d", failCount)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" &&
		(s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// waitForJobStatus polls a job until it reaches the expected status or times out
func waitForJobStatus(t *testing.T, ctx context.Context, storage interfaces.JobStorage, jobID string, expectedStatus models.JobStatus, timeout time.Duration) *models.CrawlJob {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("Timeout waiting for job %s to reach status %s", jobID, expectedStatus)
			return nil
		case <-ticker.C:
			jobInterface, err := storage.GetJob(ctx, jobID)
			if err != nil {
				continue // Job might not exist yet
			}

			job := jobInterface.(*models.CrawlJob)
			if job.Status == expectedStatus {
				return job
			}
		}
	}
}

// countChildrenByStatus counts child jobs by their status
func countChildrenByStatus(t *testing.T, ctx context.Context, storage interfaces.JobStorage, parentID string, status models.JobStatus) int {
	opts := &interfaces.JobListOptions{
		ParentID: parentID,
		Status:   string(status),
		Limit:    0,
	}

	children, err := storage.ListJobs(ctx, opts)
	if err != nil {
		t.Logf("Failed to list children with status %s: %v", status, err)
		return 0
	}

	return len(children)
}

// TestErrorToleranceContinueIntegration tests end-to-end error tolerance with continue action
// This test runs the actual crawler with a test HTTP server and verifies:
// - Failures exceed threshold but job continues
// - No children are cancelled
// - Parent job completes successfully
// - No EventJobFailed is published for threshold enforcement
func TestErrorToleranceContinueIntegration(t *testing.T) {
	config, cleanup := LoadTestConfig(t)
	defer cleanup()

	ctx := context.Background()
	app := InitializeTestApp(t, config)
	defer app.Close()

	// Create test HTTP server that returns errors for specific paths
	failCount := 0
	successCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "Test error", http.StatusInternalServerError)
			failCount++
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test content</body></html>"))
		successCount++
	}))
	defer server.Close()

	// Event collector to capture events
	var capturedEvents []interfaces.Event
	var eventMutex sync.Mutex

	eventCollector := func(ctx context.Context, event interfaces.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		t.Logf("Captured event: %s with payload: %+v", event.Type, event.Payload)
		return nil
	}

	// Subscribe to both failed and completed events
	app.EventService.Subscribe(interfaces.EventJobFailed, eventCollector)
	app.EventService.Subscribe(interfaces.EventJobCompleted, eventCollector)

	// Create job definition with error tolerance (max 2 failures, action: continue)
	jobDef := &models.JobDefinition{
		ID:          "test-continue-integration",
		Name:        "Continue Integration Test",
		Type:        models.JobDefinitionTypeCrawler,
		Description: "Integration test for continue error tolerance",
		Sources:     []string{"test"},
		Steps: []models.JobStep{
			{
				Name:    "crawl",
				Action:  "crawl",
				Config:  map[string]interface{}{},
				OnError: models.ErrorStrategyContinue,
			},
		},
		Schedule: "",
		Timeout:  "5m",
		Enabled:  true,
		ErrorTolerance: &models.ErrorTolerance{
			MaxChildFailures: 2,
			FailureAction:    "continue",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := app.StorageManager.JobDefinitionStorage().SaveJobDefinition(ctx, jobDef)
	if err != nil {
		t.Fatalf("Failed to save job definition: %v", err)
	}

	// Create parent job
	parentJobID := "parent-continue-integration"
	parentJob := &models.CrawlJob{
		ID:        parentJobID,
		ParentID:  "",
		JobType:   models.JobTypeParent,
		Name:      "Parent Job Continue Integration",
		Status:    models.JobStatusPending,
		CreatedAt: time.Now(),
		Progress: models.CrawlProgress{
			TotalURLs:     0,
			CompletedURLs: 0,
			PendingURLs:   0,
			FailedURLs:    0,
		},
	}
	err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
	if err != nil {
		t.Fatalf("Failed to save parent job: %v", err)
	}

	// Enqueue URLs: 3 that will fail (exceeding threshold), 2 that will succeed
	urls := []string{
		server.URL + "/fail",
		server.URL + "/fail",
		server.URL + "/fail",
		server.URL + "/success1",
		server.URL + "/success2",
	}

	for i, url := range urls {
		msg := &queue.JobMessage{
			ID:              fmt.Sprintf("child-continue-%d", i),
			Type:            "crawler_url",
			URL:             url,
			Depth:           0,
			ParentID:        parentJobID,
			JobDefinitionID: jobDef.ID,
			Config: map[string]interface{}{
				"max_depth":    1,
				"follow_links": false,
			},
		}

		// Create child job record
		childJob := &models.CrawlJob{
			ID:        msg.ID,
			ParentID:  parentJobID,
			JobType:   models.JobTypeCrawlerURL,
			Name:      fmt.Sprintf("Child Continue %d", i),
			Status:    models.JobStatusPending,
			CreatedAt: time.Now(),
		}
		err = app.StorageManager.JobStorage().SaveJob(ctx, childJob)
		if err != nil {
			t.Fatalf("Failed to save child job: %v", err)
		}

		err = app.QueueManager.Enqueue(ctx, msg)
		if err != nil {
			t.Fatalf("Failed to enqueue message %d: %v", i, err)
		}
	}

	// Update parent job progress
	parentJob.Status = models.JobStatusRunning
	parentJob.Progress.TotalURLs = len(urls)
	parentJob.Progress.PendingURLs = len(urls)
	err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
	if err != nil {
		t.Fatalf("Failed to update parent job: %v", err)
	}

	// Wait for job processing to complete
	// Poll parent job status until it's completed or failed
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	jobFinished := false
	for !jobFinished {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for parent job to complete")
		case <-ticker.C:
			jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, parentJobID)
			if err != nil {
				t.Fatalf("Failed to get parent job: %v", err)
			}
			job := jobInterface.(*models.CrawlJob)

			t.Logf("Parent job status: %s, failed URLs: %d, completed URLs: %d",
				job.Status, job.Progress.FailedURLs, job.Progress.CompletedURLs)

			if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed {
				jobFinished = true

				// Verify job completed successfully (NOT failed)
				if job.Status == models.JobStatusFailed {
					t.Errorf("Expected job to complete successfully with 'continue' action, but it failed with error: %s", job.Error)
				} else {
					t.Logf("Parent job completed successfully as expected")
				}

				// Verify threshold was exceeded but job continued
				if job.Progress.FailedURLs < 2 {
					t.Errorf("Expected at least 2 failures to verify continue behavior, got %d", job.Progress.FailedURLs)
				}
			}
		}
	}

	// Verify no EventJobFailed was published for threshold enforcement
	eventMutex.Lock()
	foundThresholdFailure := false
	for _, event := range capturedEvents {
		if event.Type == interfaces.EventJobFailed {
			if payload, ok := event.Payload.(map[string]interface{}); ok {
				if jobID, ok := payload["job_id"].(string); ok && jobID == parentJobID {
					// Check if this is a threshold-related failure
					if errorMsg, ok := payload["error"].(string); ok && contains(errorMsg, "Error tolerance exceeded") {
						foundThresholdFailure = true
						break
					}
				}
			}
		}
	}
	eventMutex.Unlock()

	if foundThresholdFailure {
		t.Error("Expected no EventJobFailed for threshold enforcement with 'continue' action")
	}

	// Verify no children were cancelled
	cancelledCount := 0
	for i := 0; i < len(urls); i++ {
		childID := fmt.Sprintf("child-continue-%d", i)
		jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, childID)
		if err != nil {
			t.Logf("Child job %s not found (may not have been processed): %v", childID, err)
			continue
		}

		child := jobInterface.(*models.CrawlJob)
		t.Logf("Child %s status: %s", childID, child.Status)

		if child.Status == models.JobStatusCancelled {
			cancelledCount++
		}
	}

	if cancelledCount > 0 {
		t.Errorf("Expected no children to be cancelled with 'continue' action, but %d were cancelled", cancelledCount)
	}

	// Verify failures occurred as expected
	if failCount < 3 {
		t.Errorf("Expected 3 failures to occur, got %d", failCount)
	}
	if successCount < 2 {
		t.Errorf("Expected 2 successes to occur, got %d", successCount)
	}

	t.Logf("Continue test completed: %d failures, %d successes, 0 cancellations", failCount, successCount)
}

// TestErrorToleranceMarkWarningIntegration tests end-to-end error tolerance with mark_warning action
// This test runs the actual crawler with a test HTTP server and verifies:
// - Failures exceed threshold
// - Job continues and completes successfully
// - job.Error field contains warning message about threshold
// - No children are cancelled
func TestErrorToleranceMarkWarningIntegration(t *testing.T) {
	config, cleanup := LoadTestConfig(t)
	defer cleanup()

	ctx := context.Background()
	app := InitializeTestApp(t, config)
	defer app.Close()

	// Create test HTTP server that returns errors for specific paths
	failCount := 0
	successCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "Test error", http.StatusInternalServerError)
			failCount++
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test content</body></html>"))
		successCount++
	}))
	defer server.Close()

	// Event collector to capture events
	var capturedEvents []interfaces.Event
	var eventMutex sync.Mutex

	eventCollector := func(ctx context.Context, event interfaces.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		t.Logf("Captured event: %s with payload: %+v", event.Type, event.Payload)
		return nil
	}

	// Subscribe to completed events
	app.EventService.Subscribe(interfaces.EventJobCompleted, eventCollector)

	// Create job definition with error tolerance (max 2 failures, action: mark_warning)
	jobDef := &models.JobDefinition{
		ID:          "test-mark-warning-integration",
		Name:        "Mark Warning Integration Test",
		Type:        models.JobDefinitionTypeCrawler,
		Description: "Integration test for mark_warning error tolerance",
		Sources:     []string{"test"},
		Steps: []models.JobStep{
			{
				Name:    "crawl",
				Action:  "crawl",
				Config:  map[string]interface{}{},
				OnError: models.ErrorStrategyContinue,
			},
		},
		Schedule: "",
		Timeout:  "5m",
		Enabled:  true,
		ErrorTolerance: &models.ErrorTolerance{
			MaxChildFailures: 2,
			FailureAction:    "mark_warning",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := app.StorageManager.JobDefinitionStorage().SaveJobDefinition(ctx, jobDef)
	if err != nil {
		t.Fatalf("Failed to save job definition: %v", err)
	}

	// Create parent job
	parentJobID := "parent-mark-warning-integration"
	parentJob := &models.CrawlJob{
		ID:        parentJobID,
		ParentID:  "",
		JobType:   models.JobTypeParent,
		Name:      "Parent Job Mark Warning Integration",
		Status:    models.JobStatusPending,
		CreatedAt: time.Now(),
		Progress: models.CrawlProgress{
			TotalURLs:     0,
			CompletedURLs: 0,
			PendingURLs:   0,
			FailedURLs:    0,
		},
	}
	err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
	if err != nil {
		t.Fatalf("Failed to save parent job: %v", err)
	}

	// Enqueue URLs: 3 that will fail (exceeding threshold), 2 that will succeed
	urls := []string{
		server.URL + "/fail",
		server.URL + "/fail",
		server.URL + "/fail",
		server.URL + "/success1",
		server.URL + "/success2",
	}

	for i, url := range urls {
		msg := &queue.JobMessage{
			ID:              fmt.Sprintf("child-warning-%d", i),
			Type:            "crawler_url",
			URL:             url,
			Depth:           0,
			ParentID:        parentJobID,
			JobDefinitionID: jobDef.ID,
			Config: map[string]interface{}{
				"max_depth":    1,
				"follow_links": false,
			},
		}

		// Create child job record
		childJob := &models.CrawlJob{
			ID:        msg.ID,
			ParentID:  parentJobID,
			JobType:   models.JobTypeCrawlerURL,
			Name:      fmt.Sprintf("Child Warning %d", i),
			Status:    models.JobStatusPending,
			CreatedAt: time.Now(),
		}
		err = app.StorageManager.JobStorage().SaveJob(ctx, childJob)
		if err != nil {
			t.Fatalf("Failed to save child job: %v", err)
		}

		err = app.QueueManager.Enqueue(ctx, msg)
		if err != nil {
			t.Fatalf("Failed to enqueue message %d: %v", i, err)
		}
	}

	// Update parent job progress
	parentJob.Status = models.JobStatusRunning
	parentJob.Progress.TotalURLs = len(urls)
	parentJob.Progress.PendingURLs = len(urls)
	err = app.StorageManager.JobStorage().SaveJob(ctx, parentJob)
	if err != nil {
		t.Fatalf("Failed to update parent job: %v", err)
	}

	// Wait for job processing to complete
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	jobFinished := false
	var finalJob *models.CrawlJob
	for !jobFinished {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for parent job to complete")
		case <-ticker.C:
			jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, parentJobID)
			if err != nil {
				t.Fatalf("Failed to get parent job: %v", err)
			}
			job := jobInterface.(*models.CrawlJob)

			t.Logf("Parent job status: %s, failed URLs: %d, completed URLs: %d, error: %s",
				job.Status, job.Progress.FailedURLs, job.Progress.CompletedURLs, job.Error)

			if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed {
				jobFinished = true
				finalJob = job

				// Verify job completed successfully (NOT failed)
				if job.Status == models.JobStatusFailed {
					t.Errorf("Expected job to complete successfully with 'mark_warning' action, but it failed")
				} else {
					t.Logf("Parent job completed successfully as expected")
				}

				// Verify threshold was exceeded
				if job.Progress.FailedURLs < 2 {
					t.Errorf("Expected at least 2 failures to verify mark_warning behavior, got %d", job.Progress.FailedURLs)
				}
			}
		}
	}

	// Verify job.Error contains warning about threshold
	if finalJob == nil {
		t.Fatal("Final job is nil")
	}

	if finalJob.Error == "" {
		t.Error("Expected job.Error to contain warning message about threshold")
	} else if !contains(finalJob.Error, "warning") && !contains(finalJob.Error, "threshold") && !contains(finalJob.Error, "exceeded") {
		t.Errorf("Expected job.Error to contain warning about threshold, got: %s", finalJob.Error)
	} else {
		t.Logf("Warning message correctly set in job.Error: %s", finalJob.Error)
	}

	// Verify no children were cancelled
	cancelledCount := 0
	for i := 0; i < len(urls); i++ {
		childID := fmt.Sprintf("child-warning-%d", i)
		jobInterface, err := app.StorageManager.JobStorage().GetJob(ctx, childID)
		if err != nil {
			t.Logf("Child job %s not found (may not have been processed): %v", childID, err)
			continue
		}

		child := jobInterface.(*models.CrawlJob)
		t.Logf("Child %s status: %s", childID, child.Status)

		if child.Status == models.JobStatusCancelled {
			cancelledCount++
		}
	}

	if cancelledCount > 0 {
		t.Errorf("Expected no children to be cancelled with 'mark_warning' action, but %d were cancelled", cancelledCount)
	}

	// Verify failures occurred as expected
	if failCount < 3 {
		t.Errorf("Expected 3 failures to occur, got %d", failCount)
	}
	if successCount < 2 {
		t.Errorf("Expected 2 successes to occur, got %d", successCount)
	}

	t.Logf("Mark warning test completed: %d failures, %d successes, 0 cancellations, warning set", failCount, successCount)
}
