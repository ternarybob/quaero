// -----------------------------------------------------------------------
// Last Modified: Monday, 3rd November 2025 8:50:03 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package api

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"

	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	arbor "github.com/ternarybob/arbor"
	_ "modernc.org/sqlite"
)

// LoadTestConfig loads test configuration with cleanup function
// Creates a temporary database file for test isolation
func LoadTestConfig(t *testing.T) (*common.Config, func()) {
	t.Helper()

	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create test configuration
	config := &common.Config{
		Server: common.ServerConfig{
			Host: "localhost",
			Port: 18085, // Different port for tests
		},
		Storage: common.StorageConfig{
			Type: "sqlite",
			SQLite: common.SQLiteConfig{
				Path:          dbPath,
				EnableFTS5:    true,
				EnableVector:  true,
				EmbeddingDimension: 768,
				CacheSizeMB:   64,
				WALMode:       true,
				BusyTimeoutMS: 10000,
			},
		},
		LLM: common.LLMConfig{
			Mode: "offline",
			Offline: common.OfflineLLMConfig{
				MockMode: true, // Use mock mode for tests
			},
		},
		Queue: common.QueueConfig{
			QueueName:         "test-queue",
			Concurrency:       2,
			PollInterval:      "100ms",
			VisibilityTimeout: "5m",
			MaxReceive:        3,
		},
	}

	// Cleanup function
	cleanup := func() {
		// Database will be automatically cleaned up with temp directory
	}

	return config, cleanup
}

// InitializeTestApp initializes a full application with all services
func InitializeTestApp(t *testing.T, config *common.Config) *app.App {
	t.Helper()

	// Initialize logger
	logger := arbor.NewLogger()

	// Create app instance
	appInstance, err := app.New(config, logger)
	if err != nil {
		t.Fatalf("Failed to initialize test app: %v", err)
	}

	return appInstance
}

// createLoadTestHTTPServer creates a test HTTP server for load testing
func createLoadTestHTTPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "Test error", http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/timeout" {
			time.Sleep(10 * time.Second)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body><h1>Test Content</h1><p>Success page</p><a href='/link1'>Link 1</a><a href='/link2'>Link 2</a></body></html>"))
	}))
}

// createLoadTestJobDefinition generates job definition with specified child URL count
func createLoadTestJobDefinition(id, sourceID string, childCount int) *models.JobDefinition {
	return &models.JobDefinition{
		ID:          id,
		Name:        fmt.Sprintf("Load Test Job (children: %d)", childCount),
		Type:        models.JobDefinitionTypeCrawler,
		Description: "Load test job definition",
		Sources:     []string{sourceID},
		Steps: []models.JobStep{
			{
				Name:    "crawl",
				Action:  "crawl",
				Config: map[string]interface{}{
					"max_depth":      2,
					"follow_links":   true,
					"concurrency":    2,
				},
				OnError: models.ErrorStrategyContinue,
			},
		},
		Schedule: "",
		Timeout:  "10m",
		Enabled:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// getDirectDBConnection opens a direct SQLite connection for queries
func getDirectDBConnection(config *common.Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite", config.Storage.SQLite.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure SQLite for testing
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return db, nil
}

// queryJobCount queries crawl_jobs table for count by status
func queryJobCount(db *sql.DB, status string) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM crawl_jobs WHERE status = ?", status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to query job count: %w", err)
	}
	return count, nil
}

// queryQueueMessageCount queries goqite table for pending messages
func queryQueueMessageCount(db *sql.DB, queueName string) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM goqite WHERE queue = ? AND timeout <= strftime('%Y-%m-%dT%H:%M:%fZ', 'now')", queueName).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to query queue message count: %w", err)
	}
	return count, nil
}

// findLatestLogFile finds the most recent log file in directory
func findLatestLogFile(logDir string) (string, error) {
	globPattern := filepath.Join(logDir, "quaero.*.log")
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return "", fmt.Errorf("failed to find log files: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no log files found in %s", logDir)
	}

	// Find the most recent file
	// Initialize with first file's mod time to handle ties deterministically
	latest := matches[0]
	latestInfo, err := os.Stat(latest)
	if err != nil {
		return "", fmt.Errorf("failed to stat first log file: %w", err)
	}
	latestTime := latestInfo.ModTime()

	// Check remaining files
	for _, match := range matches[1:] {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latest = match
			latestTime = info.ModTime()
		}
	}

	return latest, nil
}

// parseLogForPattern parses log file for regex pattern matches
func parseLogForPattern(logPath string, pattern string) ([]string, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var matches []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return matches, nil
}

// countLogOccurrences counts occurrences of pattern in log file
func countLogOccurrences(logPath string, pattern string) (int, error) {
	matches, err := parseLogForPattern(logPath, pattern)
	if err != nil {
		return 0, err
	}
	return len(matches), nil
}

// waitForJobCompletion polls job until terminal status
func waitForJobCompletion(ctx context.Context, storage interfaces.JobStorage, jobID string, timeout time.Duration) (*models.CrawlJob, error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for job completion")
		case <-ticker.C:
			jobInterface, err := storage.GetJob(ctx, jobID)
			if err != nil {
				continue
			}
			job := jobInterface.(*models.CrawlJob)

			// Return when job reaches a terminal state
			if job.Status == models.JobStatusFailed ||
				job.Status == models.JobStatusCompleted ||
				job.Status == models.JobStatusCancelled {
				return job, nil
			}
		}
	}
}

// validateJobHierarchy validates parent-child relationships
func validateJobHierarchy(ctx context.Context, storage interfaces.JobStorage, parentID string, expectedChildCount int) error {
	opts := &interfaces.JobListOptions{
		ParentID: parentID,
		Limit:    0,
	}

	children, err := storage.ListJobs(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list child jobs: %w", err)
	}

	if len(children) != expectedChildCount {
		return fmt.Errorf("expected %d children, got %d", expectedChildCount, len(children))
	}

	// Verify all children have correct parent_id
	for _, child := range children {
		if child.ParentID != parentID {
			return fmt.Errorf("child job %s has incorrect parent_id: %s", child.ID, child.ParentID)
		}
	}

	return nil
}

// collectQueueMetrics gathers queue statistics
// TODO Phase 8-11: Re-enable when queue manager API is finalized
func collectQueueMetrics(ctx context.Context, queueMgr *queue.Manager) map[string]interface{} {
	// Temporarily disabled during queue refactor - GetQueueStats not available
	_ = ctx      // Suppress unused variable
	_ = queueMgr // Suppress unused variable

	// Return mock data for now (function is unused but kept for future use)
	return map[string]interface{}{
		"queue_length":   0,
		"in_flight":      0,
		"total_messages": 0,
		"queue_name":     "test-queue",
		"concurrency":    2,
	}
}

// createLoadTestSource generates source configuration for load testing
func createLoadTestSource(id, baseURL string) map[string]interface{} {
	return map[string]interface{}{
		"id":   id,
		"type": "test",
		"config": map[string]interface{}{
			"base_url":      baseURL,
			"max_depth":     2,
			"concurrency":   2,
			"follow_links":  true,
			"respect_robots": false,
		},
	}
}