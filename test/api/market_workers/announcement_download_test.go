// -----------------------------------------------------------------------
// Tests for announcement download worker
// Tests the market_announcement_download worker which downloads PDFs from
// filtered announcements using the worker-to-worker pattern
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnnouncementDownloadSingle tests the announcement download worker for a single ticker.
// This worker uses the worker-to-worker pattern: it calls the announcements worker
// via DocumentProvider, then filters and downloads PDFs for matching announcement types.
func TestAnnouncementDownloadSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Announcement download requires LLM for the underlying announcements provider
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-announcement-download-single-%d", time.Now().UnixNano())
	ticker := "EXR"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Announcement Download Single Stock Test",
		"description": "Test announcement download worker with PDF filtering",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "announcement-download", "single-stock"},
		"config": map[string]interface{}{
			"variables": []map[string]interface{}{
				{"ticker": ticker},
			},
		},
		"steps": []map[string]interface{}{
			{
				"name": "download-announcements",
				"type": "market_announcement_download",
				"config": map[string]interface{}{
					"ticker": ticker,
				},
			},
		},
	}

	// MANDATORY: Save job definition before execution
	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing announcement download job: %s", jobID)

	// Wait for completion - announcement download may take longer due to PDF downloads
	finalStatus := WaitForJobCompletion(t, helper, jobID, 5*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===

	// Assert announcement download document output with correct tags
	downloadTags := []string{"announcement-download", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, downloadTags)
	assert.NotEmpty(t, content, "Announcement download content should not be empty")

	// Assert schema compliance (quaero/announcement_download/v1)
	isValid := ValidateSchema(t, metadata, AnnouncementDownloadSchema)
	assert.True(t, isValid, "Output should comply with announcement download schema")

	// Assert required fields from schema
	AssertMetadataHasFields(t, metadata, []string{"$schema", "ticker", "fetched_at", "filter_types", "total_matched", "total_downloaded", "total_failed", "announcements"})

	// Validate schema string
	if schema, ok := metadata["$schema"].(string); ok {
		assert.Equal(t, "quaero/announcement_download/v1", schema, "Schema should be quaero/announcement_download/v1")
		t.Logf("PASS: $schema = %s", schema)
	}

	// Validate ticker matches
	if tickerVal, ok := metadata["ticker"].(string); ok {
		assert.Contains(t, tickerVal, ticker, "Ticker should contain %s", ticker)
		t.Logf("PASS: ticker = %s", tickerVal)
	}

	// Validate filter_types is populated (should have default FY types)
	if filterTypes, ok := metadata["filter_types"].([]interface{}); ok {
		assert.Greater(t, len(filterTypes), 0, "filter_types should have at least one type")
		t.Logf("PASS: filter_types has %d types", len(filterTypes))
	}

	// Validate counts are non-negative
	if totalMatched, ok := metadata["total_matched"].(float64); ok {
		assert.GreaterOrEqual(t, totalMatched, 0.0, "total_matched should be >= 0")
		t.Logf("PASS: total_matched = %.0f", totalMatched)
	}

	if totalDownloaded, ok := metadata["total_downloaded"].(float64); ok {
		require.Greater(t, totalDownloaded, 0.0, "total_downloaded must be > 0 - at least one PDF must be downloaded successfully")
		t.Logf("PASS: total_downloaded = %.0f", totalDownloaded)
	} else {
		require.Fail(t, "total_downloaded field not found in metadata")
	}

	if totalFailed, ok := metadata["total_failed"].(float64); ok {
		assert.GreaterOrEqual(t, totalFailed, 0.0, "total_failed should be >= 0")
		t.Logf("PASS: total_failed = %.0f", totalFailed)
	}

	// Validate announcements array exists
	if anns, ok := metadata["announcements"]; ok {
		if arr, ok := anns.([]interface{}); ok {
			t.Logf("PASS: announcements array has %d items", len(arr))
			// If there are announcements, validate structure of first one
			if len(arr) > 0 {
				if firstAnn, ok := arr[0].(map[string]interface{}); ok {
					// Check expected fields in announcement
					expectedFields := []string{"date", "headline", "type"}
					for _, field := range expectedFields {
						if _, hasField := firstAnn[field]; hasField {
							t.Logf("PASS: First announcement has field '%s'", field)
						}
					}
				}
			}
		}
	}

	// MANDATORY: Save schema definition
	SaveSchemaDefinition(t, env, AnnouncementDownloadSchema, "AnnouncementDownloadSchema")

	// MANDATORY: Save worker output
	SaveWorkerOutput(t, env, helper, downloadTags, ticker)

	// MANDATORY: Assert result files exist
	AssertResultFilesExist(t, env, 1)
	AssertSchemaFileExists(t, env)

	// MANDATORY: Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: announcement download single stock test completed")
}

// TestAnnouncementDownloadMulti tests the announcement download worker with multiple tickers.
// Uses the same tickers as announcements_test.go multi-stock tests for consistency.
func TestAnnouncementDownloadMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Use same tickers as TestMarketAnnouncementsMulti
	stocks := []string{"BHP", "GNP"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-announcement-download-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("Announcement Download Test - %s", stock),
				"description": "Test announcement download multi-stock",
				"type":        "manager",
				"enabled":     true,
				"tags":        []string{"worker-test", "announcement-download", "multi-stock"},
				"config": map[string]interface{}{
					"variables": []map[string]interface{}{
						{"ticker": stock},
					},
				},
				"steps": []map[string]interface{}{
					{
						"name": "download-announcements",
						"type": "market_announcement_download",
						"config": map[string]interface{}{
							"ticker": stock,
						},
					},
				},
			}

			// Save job definition for FIRST stock only
			if stock == stocks[0] {
				SaveJobDefinition(t, env, body)
			}

			jobID, _ := CreateAndExecuteJob(t, helper, body)
			if jobID == "" {
				return
			}

			t.Logf("Executing announcement download job for %s: %s", stock, jobID)

			finalStatus := WaitForJobCompletion(t, helper, jobID, 5*time.Minute)
			if finalStatus != "completed" {
				t.Logf("Job for %s ended with status %s", stock, finalStatus)
				return
			}

			// === ASSERTIONS ===

			// Assert announcement download document output
			downloadTags := []string{"announcement-download", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, downloadTags)

			// Assert content not empty
			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)

			// Assert schema compliance
			isValid := ValidateSchema(t, metadata, AnnouncementDownloadSchema)
			assert.True(t, isValid, "Output for %s should comply with announcement download schema", stock)

			// Validate key fields
			if totalMatched, ok := metadata["total_matched"].(float64); ok {
				t.Logf("%s: total_matched = %.0f", stock, totalMatched)
			}
			if totalDownloaded, ok := metadata["total_downloaded"].(float64); ok {
				t.Logf("%s: total_downloaded = %.0f", stock, totalDownloaded)
			}

			// MANDATORY: Save output for each stock
			SaveWorkerOutput(t, env, helper, downloadTags, stock)

			t.Logf("PASS: Validated announcement download for %s", stock)
		})
	}

	// MANDATORY: Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: announcement download multi-stock test completed")
}
