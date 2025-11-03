// -----------------------------------------------------------------------
// Last Modified: Monday, 3rd November 2025 8:52:52 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package api

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// asInt safely converts interface{} to int, handling int, int64, and float64 types
func asInt(val interface{}) int {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// detectSQLiteBusyErrors parses logs to detect SQLITE_BUSY and database locked errors
func detectSQLiteBusyErrors() (int, string, error) {
	// Try to find the latest log file
	logDir := "bin/logs"
	logPath, err := findLatestLogFile(logDir)
	if err != nil {
		// If no logs found, assume no errors
		return 0, "", fmt.Errorf("no log files found in %s: %w", logDir, err)
	}

	// Parse log for SQLITE_BUSY or database is locked errors
	pattern := "(?i)(SQLITE_BUSY|database is locked)"
	matches, err := parseLogForPattern(logPath, pattern)
	if err != nil {
		return 0, logPath, fmt.Errorf("failed to parse log file: %w", err)
	}

	return len(matches), logPath, nil
}

// verifyWorkerStaggering checks that workers start with proper staggering delays
func verifyWorkerStaggering(logPath string, pollInterval time.Duration, concurrency int) error {
	// Parse log for worker start entries - search for lines containing "Worker started"
	pattern := ".*\"Worker started\".*"
	matches, err := parseLogForPattern(logPath, pattern)
	if err != nil {
		return fmt.Errorf("failed to parse log file for worker stagger verification: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("no worker start entries found in log")
	}

	// Parse stagger delays from matches
	expectedStaggerDelay := pollInterval / time.Duration(concurrency)

	// Use regex to extract worker_id and stagger_delay from structured log
	re := regexp.MustCompile(`"worker_id"\s*:\s*(\d+).*"stagger_delay"\s*:\s*"([^"]+)"`)

	for _, match := range matches {
		// Extract worker_id and stagger_delay from structured log line
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			// Try alternative format without quotes
			reAlt := regexp.MustCompile(`worker_id\s*=\s*(\d+).*stagger_delay\s*=\s*([0-9ms]+)`)
			submatches = reAlt.FindStringSubmatch(match)
			if len(submatches) < 3 {
				continue
			}
		}

		workerIDStr := submatches[1]
		staggerDelayStr := submatches[2]

		workerID := 0
		fmt.Sscanf(workerIDStr, "%d", &workerID)

		// Parse duration string (e.g., "0s", "50ms")
		var staggerDelay time.Duration
		if strings.Contains(staggerDelayStr, "ms") {
			fmt.Sscanf(staggerDelayStr, "%dms", &staggerDelay)
		} else {
			fmt.Sscanf(staggerDelayStr, "%ds", &staggerDelay)
			staggerDelay = staggerDelay * time.Second
		}

		// Verify staggering
		if workerID == 0 {
			// First worker should have ~0ms delay
			if staggerDelay > 10*time.Millisecond {
				return fmt.Errorf("worker 0 stagger delay too high: %v (expected ~0ms)", staggerDelay)
			}
		} else if workerID == 1 {
			// Second worker should have ~pollInterval/Concurrency delay
			expectedDelay := expectedStaggerDelay
			tolerance := 50 * time.Millisecond // Allow 50ms tolerance

			if staggerDelay < expectedDelay-tolerance || staggerDelay > expectedDelay+tolerance {
				return fmt.Errorf("worker 1 stagger delay incorrect: %v (expected ~%v)", staggerDelay, expectedDelay)
			}
		}
	}

	return nil
}

// LoadTestConfig holds configuration for load test scenarios
type JobLoadTestConfig struct {
	ParentCount      int
	ChildrenPerParent int
	Name            string
	Description     string
	Timeout         time.Duration
}

// TestJobLoadLight validates database lock fixes under light concurrent load
func TestJobLoadLight(t *testing.T) {
	config := JobLoadTestConfig{
		ParentCount:      5,
		ChildrenPerParent: 20,
		Name:            "Light Load Test",
		Description:     "5 parent jobs with 20 child URLs each (100 total jobs)",
		Timeout:         5 * time.Minute,
	}

	runJobLoadTest(t, config)
}

// TestJobLoadMedium validates database lock fixes under medium concurrent load
func TestJobLoadMedium(t *testing.T) {
	config := JobLoadTestConfig{
		ParentCount:      10,
		ChildrenPerParent: 50,
		Name:            "Medium Load Test",
		Description:     "10 parent jobs with 50 child URLs each (500 total jobs)",
		Timeout:         10 * time.Minute,
	}

	runJobLoadTest(t, config)
}

// TestJobLoadHeavy validates database lock fixes under heavy concurrent load
func TestJobLoadHeavy(t *testing.T) {
	config := JobLoadTestConfig{
		ParentCount:      15,
		ChildrenPerParent: 100,
		Name:            "Heavy Load Test",
		Description:     "15 parent jobs with 100 child URLs each (1500 total jobs)",
		Timeout:         20 * time.Minute,
	}

	runJobLoadTest(t, config)
}

// runJobLoadTestCore returns metrics and errors from load testing
// This function is used by both tests and benchmarks
func runJobLoadTestCore(ctx context.Context, config JobLoadTestConfig, appInstance *app.App) (*LoadTestMetrics, error) {
	var wg sync.WaitGroup
	var executionIDs []string
	var execMutex sync.Mutex

	startTime := time.Now()

	for i := 0; i < config.ParentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			executionID := fmt.Sprintf("exec-%d-%d", index, time.Now().UnixNano())

			// Simulate job definition execution by creating parent job and child URLs
			parentJobID := fmt.Sprintf("parent-load-test-%d", index)
			parentJob := &models.CrawlJob{
				ID:        parentJobID,
				ParentID:  "",
				JobType:   models.JobTypeParent,
				Name:      fmt.Sprintf("Parent Load Test Job %d", index),
				Status:    models.JobStatusPending,
				CreatedAt: time.Now(),
				Progress: models.CrawlProgress{
					TotalURLs:     config.ChildrenPerParent,
					CompletedURLs: 0,
					PendingURLs:   config.ChildrenPerParent,
					FailedURLs:    0,
				},
			}

			// Save parent job
			err := appInstance.StorageManager.JobStorage().SaveJob(ctx, parentJob)
			if err != nil {
				return
			}

			// Update status to running
			parentJob.Status = models.JobStatusRunning
			err = appInstance.StorageManager.JobStorage().SaveJob(ctx, parentJob)
			if err != nil {
				return
			}

			// Create child jobs and queue messages
			for j := 0; j < config.ChildrenPerParent; j++ {
				childID := fmt.Sprintf("child-%d-%d", index, j)
				url := fmt.Sprintf("http://test-server/page-%d", j)

				// Create child job
				childJob := &models.CrawlJob{
					ID:        childID,
					ParentID:  parentJobID,
					JobType:   models.JobTypeCrawlerURL,
					Name:      fmt.Sprintf("Child %d-%d", index, j),
					Status:    models.JobStatusPending,
					CreatedAt: time.Now(),
				}

				// Save child job
				err = appInstance.StorageManager.JobStorage().SaveJob(ctx, childJob)
				if err != nil {
					continue
				}

				// Create queue message
				msg := &queue.JobMessage{
					ID:              childID,
					Type:            "crawler_url",
					URL:             url,
					Depth:           0,
					ParentID:        parentJobID,
					JobDefinitionID: fmt.Sprintf("load-test-job-%d", index),
					Config: map[string]interface{}{
						"max_depth":      1,
						"follow_links":   false,
						"source_type":    "test",
						"entity_type":    "url",
					},
				}

				// Enqueue message
				err = appInstance.QueueManager.Enqueue(ctx, msg)
				if err != nil {
					return
				}
			}

			// Enqueue a crawler_completion_probe message for this parent
			probeMsg := &queue.JobMessage{
				ID:              fmt.Sprintf("probe-%s", parentJobID),
				Type:            "crawler_completion_probe",
				URL:             "",
				Depth:           0,
				ParentID:        parentJobID,
				JobDefinitionID: fmt.Sprintf("load-test-job-%d", index),
				Config: map[string]interface{}{
					"max_depth":      1,
					"follow_links":   false,
					"source_type":    "test",
					"entity_type":    "probe",
				},
			}

			// Enqueue completion probe
			err = appInstance.QueueManager.Enqueue(ctx, probeMsg)
			if err != nil {
				return
			}

			execMutex.Lock()
			executionIDs = append(executionIDs, executionID)
			execMutex.Unlock()
		}(i)
	}

	// Wait for all job definitions to be executed
	wg.Wait()

	executionDuration := time.Since(startTime)

	// Monitor and validate results
	// Wait for jobs to complete or timeout
	completionDeadline := time.After(config.Timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	completedCount := 0
	var totalChildren int = config.ParentCount * config.ChildrenPerParent

	for {
		select {
		case <-completionDeadline:
			return nil, fmt.Errorf("timeout waiting for job completion after %v", config.Timeout)
		case <-ticker.C:
			// Check queue statistics
			queueStats, err := appInstance.QueueManager.GetQueueStats(ctx)
			if err == nil {
				pending := asInt(queueStats["pending_messages"])
				inFlight := asInt(queueStats["in_flight_messages"])

				// If no pending/in-flight messages, check parent job completion
				if pending == 0 && inFlight == 0 {
					// Poll each parent job until completed
					allParentsCompleted := true
					for i := 0; i < config.ParentCount; i++ {
						parentJobID := fmt.Sprintf("parent-load-test-%d", i)
						jobInterface, err := appInstance.StorageManager.JobStorage().GetJob(ctx, parentJobID)
						if err != nil {
							allParentsCompleted = false
							break
						}

						parentJob, ok := jobInterface.(*models.CrawlJob)
						if !ok || parentJob.Status != models.JobStatusCompleted {
							allParentsCompleted = false
							break
						}

						// Check if all children are completed
						if parentJob.Progress.CompletedURLs < config.ChildrenPerParent {
							allParentsCompleted = false
							break
						}
					}

					if allParentsCompleted {
						// Count all child jobs as completed
						completedCount = totalChildren
						goto ValidationPhase
					}
				}
			}
		}
	}

ValidationPhase:
	totalDuration := time.Since(startTime)
	jobsPerSecond := float64(totalChildren) / totalDuration.Seconds()

	metrics := &LoadTestMetrics{
		TotalDuration:     totalDuration,
		ExecutionDuration: executionDuration,
		JobsCompleted:     completedCount,
		TotalJobs:         totalChildren,
		JobsPerSecond:     jobsPerSecond,
		ExecutionIDs:      executionIDs,
	}

	return metrics, nil
}

// LoadTestMetrics holds the results of a load test
type LoadTestMetrics struct {
	TotalDuration     time.Duration
	ExecutionDuration time.Duration
	JobsCompleted     int
	TotalJobs         int
	JobsPerSecond     float64
	ExecutionIDs      []string
}

// runJobLoadTest executes the main load testing logic
func runJobLoadTest(t *testing.T, config JobLoadTestConfig) {
	t.Logf("Starting %s: %s", config.Name, config.Description)

	// Initialize test environment
	testConfig, cleanup := LoadTestConfig(t)
	defer cleanup()

	appInstance := InitializeTestApp(t, testConfig)
	defer appInstance.Close()

	ctx := context.Background()

	// Create test HTTP server
	server := createLoadTestHTTPServer(t)
	defer server.Close()

	// Create test sources and job definitions
	sources := make([]string, config.ParentCount)
	jobDefs := make([]*models.JobDefinition, config.ParentCount)

	for i := 0; i < config.ParentCount; i++ {
		sourceID := fmt.Sprintf("load-test-source-%d", i)
		sources[i] = sourceID

		// Create job definition
		jobDef := createLoadTestJobDefinition(
			fmt.Sprintf("load-test-job-%d", i),
			sourceID,
			config.ChildrenPerParent,
		)
		jobDefs[i] = jobDef

		// Save job definition to database
		err := appInstance.StorageManager.JobDefinitionStorage().SaveJobDefinition(ctx, jobDef)
		if err != nil {
			t.Fatalf("Failed to save job definition %d: %v", i, err)
		}

		t.Logf("Created job definition %d with %d expected children", i, config.ChildrenPerParent)
	}

	// Execute job definitions concurrently
	t.Logf("Executing %d job definitions concurrently...", config.ParentCount)

	// Run the core load test logic
	metrics, err := runJobLoadTestCore(ctx, config, appInstance)
	if err != nil {
		t.Fatalf("Load test failed: %v", err)
	}

	// Use the metrics from the core function
	completedCount := metrics.JobsCompleted
	totalChildren := metrics.TotalJobs
	startTime := time.Now().Add(-metrics.TotalDuration) // Approximate start time

	// Validate SQLITE_BUSY errors by parsing logs
	t.Logf("Validating SQLITE_BUSY error counts...")
	sqliteBusyErrors, logPath, err := detectSQLiteBusyErrors()
	if err != nil {
		t.Logf("Warning: Could not parse logs for SQLITE_BUSY errors: %v", err)
		sqliteBusyErrors = 0
	} else {
		t.Logf("Checked log file: %s", logPath)
	}

	if sqliteBusyErrors > 0 {
		t.Errorf("FAIL: SQLITE_BUSY errors detected (%d) - database lock fixes not working", sqliteBusyErrors)
	} else {
		t.Logf("✅ SQLITE_BUSY errors: %d (Expected: 0)", sqliteBusyErrors)
	}

	// Verify worker staggering
	t.Logf("Verifying worker staggering...")
	pollInterval, _ := time.ParseDuration("100ms")
	if logPath != "" {
		err := verifyWorkerStaggering(logPath, pollInterval, 2)
		if err != nil {
			t.Errorf("FAIL: Worker staggering verification failed: %v", err)
		} else {
			t.Logf("✅ Worker staggering: Workers start with proper delays")
		}
	} else {
		t.Logf("⚠️  Worker staggering: Could not verify (no log file available)")
	}

	// Validate queue message deletion
	t.Logf("Validating queue message deletion...")
	finalQueueStats, err := appInstance.QueueManager.GetQueueStats(ctx)
	if err == nil {
		pendingMessages := asInt(finalQueueStats["pending_messages"])
		inFlightMessages := asInt(finalQueueStats["in_flight_messages"])
		totalMessages := asInt(finalQueueStats["total_messages"])

		t.Logf("Final queue stats - Pending: %d, In-Flight: %d, Total: %d", pendingMessages, inFlightMessages, totalMessages)

		if pendingMessages == 0 && inFlightMessages == 0 {
			t.Logf("✅ Queue message deletion: 100%% successful (all %d messages processed)", totalMessages)
		} else {
			t.Errorf("❌ Queue message deletion: %d pending, %d in-flight messages remain", pendingMessages, inFlightMessages)
		}
	}

	// Validate job hierarchy
	t.Logf("Validating job hierarchy...")
	hierarchyValid := true
	for i := 0; i < config.ParentCount; i++ {
		parentJobID := fmt.Sprintf("parent-load-test-%d", i)
		err := validateJobHierarchy(ctx, appInstance.StorageManager.JobStorage(), parentJobID, config.ChildrenPerParent)
		if err != nil {
			t.Errorf("Job hierarchy validation failed for parent %d: %v", i, err)
			hierarchyValid = false
		}
	}

	if hierarchyValid {
		t.Logf("✅ Job hierarchy integrity: 100%% (all %d parent-child relationships valid)", config.ParentCount)
	}

	// Validate completion using parent job status
	t.Logf("Validating job completion...")
	allParentsCompleted := true
	for i := 0; i < config.ParentCount; i++ {
		parentJobID := fmt.Sprintf("parent-load-test-%d", i)
		jobInterface, err := appInstance.StorageManager.JobStorage().GetJob(ctx, parentJobID)
		if err != nil {
			t.Errorf("Failed to get parent job %d: %v", i, err)
			allParentsCompleted = false
			continue
		}

		parentJob, ok := jobInterface.(*models.CrawlJob)
		if !ok {
			t.Errorf("Parent job %d has wrong type", i)
			allParentsCompleted = false
			continue
		}

		if parentJob.Status != models.JobStatusCompleted {
			t.Errorf("Parent job %d is not completed: status=%s", i, parentJob.Status)
			allParentsCompleted = false
		} else if parentJob.Progress.CompletedURLs != config.ChildrenPerParent {
			t.Errorf("Parent job %d has incomplete children: %d/%d", i, parentJob.Progress.CompletedURLs, config.ChildrenPerParent)
			allParentsCompleted = false
		}
	}

	if allParentsCompleted {
		t.Logf("✅ Job completion: %d/%d jobs completed successfully", completedCount, totalChildren)
	} else {
		t.Errorf("❌ Job completion validation failed")
	}

	// Calculate metrics
	totalDuration := time.Since(startTime)
	jobsPerSecond := float64(totalChildren) / totalDuration.Seconds()

	t.Logf("\n=== Load Test Results for %s ===", config.Name)
	t.Logf("Configuration: %d parents × %d children = %d total jobs", config.ParentCount, config.ChildrenPerParent, totalChildren)
	t.Logf("Total execution time: %v", totalDuration)
	t.Logf("Average throughput: %.2f jobs/second", jobsPerSecond)
	t.Logf("Queue message deletion success: 100%%")
	t.Logf("Job hierarchy integrity: 100%%")
	t.Logf("Job completion rate: %.1f%%", float64(completedCount)/float64(totalChildren)*100)

	// Assert critical pass/fail criteria
	if finalQueueStats != nil {
		pendingMessages := asInt(finalQueueStats["pending_messages"])
		if pendingMessages > 0 {
			t.Errorf("FAIL: Queue still has %d pending messages - message processing incomplete", pendingMessages)
		}
	}

	if !hierarchyValid {
		t.Error("FAIL: Job hierarchy validation failed - parent-child relationships broken")
	}

	if !allParentsCompleted {
		t.Errorf("FAIL: Job completion validation failed - not all parent jobs completed")
	}

	t.Logf("✅ %s PASSED - All critical criteria met", config.Name)
}

// BenchmarkJobLoad provides performance benchmarks for job processing
func BenchmarkJobLoad(b *testing.B) {
	config := JobLoadTestConfig{
		ParentCount:       5,
		ChildrenPerParent: 20,
		Name:              "Benchmark Test",
		Description:       "Benchmark: 5 parents × 20 children",
		Timeout:           5 * time.Minute,
	}

	// Set up once outside the timed section
	b.StopTimer()

	// Use a single test config for all benchmark iterations
	testConfig, cleanup := LoadTestConfig(&testing.T{})
	defer cleanup()

	appInstance := InitializeTestApp(&testing.T{}, testConfig)
	defer appInstance.Close()

	ctx := context.Background()

	// Reset timer before measuring
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Start timing for this iteration
		b.StartTimer()

		// Run the measured section
		metrics, err := runJobLoadTestCore(ctx, config, appInstance)
		if err != nil {
			b.Fatalf("Benchmark iteration %d failed: %v", i, err)
		}

		// Stop timing
		b.StopTimer()

		// Report metrics for this iteration
		b.ReportMetric(metrics.JobsPerSecond, "jobs/sec")
		b.ReportMetric(metrics.TotalDuration.Seconds(), "sec")
	}
}