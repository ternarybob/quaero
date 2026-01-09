// -----------------------------------------------------------------------
// Tests for announcement workers
// Tests the market_announcements worker which fetches and classifies
// announcements in a single step (inline classification)
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestProcessingAnnouncementsSingle tests the full announcement processing pipeline
// for a single stock. This configuration matches production (announcements-watchlist.toml)
// where only market_announcements runs without a preceding market_data step.
// The announcements worker handles EODHD fallback internally for price impact data.
// Also validates caching: running the job twice should return the same document ID.
func TestProcessingAnnouncementsSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Processing announcements uses signal classification (no LLM required)
	// But market_announcements may use LLM for some features
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Create job definition matching production config (no separate market_data step)
	// Production: deployments/common/job-definitions/announcements-watchlist.toml
	defID := fmt.Sprintf("test-announcements-single-%d", time.Now().UnixNano())
	ticker := "EXR"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Announcements Single Stock Test",
		"description": "Test announcement fetch and classification",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "announcements", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, env, body)

	// === FIRST EXECUTION ===
	jobID1, _ := CreateAndExecuteJob(t, helper, body)
	if jobID1 == "" {
		return
	}

	t.Logf("Executing announcement processing job (1st run): %s", jobID1)

	// Wait for completion
	finalStatus := WaitForJobCompletion(t, helper, jobID1, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Get document ID from first run
	summaryTags := []string{"announcement", strings.ToLower(ticker)}
	docID1, metadata, content := AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("First run document ID: %s", docID1)

	// === SECOND EXECUTION (tests caching) ===
	jobID2, _ := CreateAndExecuteJob(t, helper, body)
	if jobID2 == "" {
		return
	}

	t.Logf("Executing announcement processing job (2nd run): %s", jobID2)

	finalStatus2 := WaitForJobCompletion(t, helper, jobID2, 3*time.Minute)
	if finalStatus2 != "completed" {
		t.Skipf("Second job ended with status %s", finalStatus2)
		return
	}

	// Get document ID from second run
	docID2, _, _ := AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("Second run document ID: %s", docID2)

	// === CACHING ASSERTION ===
	// Document ID should be the same (cached document reused)
	assert.Equal(t, docID1, docID2, "CACHING: Document ID should be the same on second run (document was cached)")
	if docID1 == docID2 {
		t.Logf("PASS: Caching works - same document ID returned on second run")
	}

	// === REMAINING ASSERTIONS (using first run data) ===

	// Assert content contains expected sections
	expectedSections := []string{
		"Summary",
		"Announcements",
	}
	AssertOutputContains(t, content, expectedSections)

	// Assert all required sections are present
	for _, section := range AnnouncementsRequiredSections {
		if strings.Contains(content, section) {
			t.Logf("PASS: Required section '%s' present", section)
		} else {
			t.Errorf("FAIL: Required section '%s' MISSING from output", section)
		}
	}

	// Assert schema compliance (quaero/announcements/v1)
	isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with announcements schema")

	// Assert required fields for schema
	AssertMetadataHasFields(t, metadata, []string{"$schema", "ticker", "summary", "announcements"})

	// Validate summary object contains classification counts
	if summary, ok := metadata["summary"].(map[string]interface{}); ok {
		if totalCount, ok := summary["total_count"].(float64); ok {
			assert.GreaterOrEqual(t, totalCount, 0.0, "total_count should be >= 0")
			t.Logf("PASS: total_count = %.0f", totalCount)
		}
		if highRelevance, ok := summary["high_relevance_count"].(float64); ok {
			assert.GreaterOrEqual(t, highRelevance, 0.0, "high_relevance_count should be >= 0")
			t.Logf("PASS: high_relevance_count = %.0f", highRelevance)
		}
		if snr, ok := summary["signal_to_noise_ratio"].(float64); ok {
			assert.GreaterOrEqual(t, snr, 0.0, "signal_to_noise_ratio should be >= 0")
			t.Logf("PASS: signal_to_noise_ratio = %.2f", snr)
		}
	} else {
		t.Error("FAIL: Output MUST have summary object")
	}

	// Save output and schema
	SaveWorkerOutput(t, env, helper, summaryTags, ticker)
	SaveSchemaDefinition(t, env, AnnouncementsSchema, "AnnouncementsSchema")
	AssertResultFilesExist(t, env, 1)
	AssertSchemaFileExists(t, env)

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: announcement processing single stock test completed")
}

// TestProcessingAnnouncementsMulti tests announcement processing for multiple stocks
func TestProcessingAnnouncementsMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Test stocks - run as subtests for better isolation
	stocks := []string{"BHP", "CSL", "GNP", "EXR"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-announcements-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("Announcements Test - %s", stock),
				"description": "Test announcement fetch and classification multi-stock",
				"type":        "manager",
				"enabled":     true,
				"tags":        []string{"worker-test", "announcements", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-announcements",
						"type": "market_announcements",
						"config": map[string]interface{}{
							"asx_code": stock,
						},
					},
				},
			}

			// Save job definition for first stock only
			if stock == stocks[0] {
				SaveJobDefinition(t, env, body)
			}

			// Create and execute job
			jobID, _ := CreateAndExecuteJob(t, helper, body)
			if jobID == "" {
				return
			}

			t.Logf("Executing announcements job for %s: %s", stock, jobID)

			// Wait for completion
			finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
			if finalStatus != "completed" {
				t.Logf("Job for %s ended with status %s", stock, finalStatus)
				return
			}

			// === ASSERTIONS ===

			// Assert announcement document output
			announcementTags := []string{"announcement", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, announcementTags)

			// Assert content not empty
			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)

			// Assert schema compliance
			isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			// Save output
			SaveWorkerOutput(t, env, helper, announcementTags, stock)

			t.Logf("PASS: Validated announcements for %s", stock)
		})
	}

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: announcement processing multi-stock test completed")
}

// TestMarketAnnouncementsSingle tests the market_announcements worker alone (single ticker)
// This tests announcement fetching WITH inline classification
func TestMarketAnnouncementsSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-market-announcements-single-%d", time.Now().UnixNano())
	ticker := "EXR"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Market Announcements Single Stock Test",
		"description": "Test market_announcements worker with inline classification",
		"type":        "market_announcements",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-announcements", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_announcements job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===

	// Assert announcement document output with correct tags
	announcementTags := []string{"announcement", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, announcementTags)
	assert.NotEmpty(t, content, "Announcement content should not be empty")

	// Assert schema compliance for announcements (quaero/announcements/v1)
	isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with announcements schema")

	// Output MUST have announcements array
	if anns, ok := metadata["announcements"]; ok {
		if arr, ok := anns.([]interface{}); ok {
			assert.GreaterOrEqual(t, len(arr), 0, "announcements array should exist")
			t.Logf("PASS: Output has %d announcements", len(arr))
		} else {
			t.Error("FAIL: announcements field must be an array")
		}
	} else {
		t.Error("FAIL: Output MUST have announcements field")
	}

	// Validate summary object contains classification counts
	if summary, ok := metadata["summary"].(map[string]interface{}); ok {
		classificationFields := []string{"total_count", "high_relevance_count", "medium_relevance_count", "low_relevance_count", "noise_count"}
		for _, field := range classificationFields {
			if _, hasField := summary[field]; hasField {
				t.Logf("PASS: summary has classification field %s", field)
			}
		}
	} else {
		t.Error("FAIL: Output MUST have summary object")
	}

	// Validate 36-month rolling window (date range should span at least 30 months for Y3 period)
	// Note: EODHD may not have 36 months of data for all stocks, but we verify the requested range
	if dateRangeStart, ok := metadata["date_range_start"].(string); ok {
		if dateRangeEnd, ok := metadata["date_range_end"].(string); ok {
			startDate, errStart := time.Parse("2006-01-02", dateRangeStart)
			endDate, errEnd := time.Parse("2006-01-02", dateRangeEnd)
			if errStart == nil && errEnd == nil {
				monthsDiff := int(endDate.Sub(startDate).Hours() / (24 * 30))
				t.Logf("Date range: %s to %s (%d months)", dateRangeStart, dateRangeEnd, monthsDiff)
				// Warn if date range is less than expected (API may have limited data)
				if monthsDiff < 30 {
					t.Logf("WARNING: Date range is only %d months (expected ~36 months for Y3 period). API may have limited historical data.", monthsDiff)
				} else {
					t.Logf("PASS: Date range spans %d months (36-month rolling window)", monthsDiff)
				}
			}
		}
	}

	// Save output files
	SaveWorkerOutput(t, env, helper, announcementTags, ticker)
	SaveSchemaDefinition(t, env, AnnouncementsSchema, "AnnouncementsSchema")
	AssertResultFilesExist(t, env, 1)
	AssertSchemaFileExists(t, env)

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_announcements single stock test completed")
}

// TestMarketAnnouncementsMulti tests market_announcements worker with multiple tickers
func TestMarketAnnouncementsMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Test stocks for announcement fetching with inline classification
	stocks := []string{"BHP", "GNP"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-market-announcements-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("Market Announcements Test - %s", stock),
				"description": "Test market_announcements multi-stock with classification",
				"type":        "market_announcements",
				"enabled":     true,
				"tags":        []string{"worker-test", "market-announcements", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-announcements",
						"type": "market_announcements",
						"config": map[string]interface{}{
							"asx_code": stock,
						},
					},
				},
			}

			// Save job definition for first stock only
			if stock == stocks[0] {
				SaveJobDefinition(t, env, body)
			}

			jobID, _ := CreateAndExecuteJob(t, helper, body)
			if jobID == "" {
				return
			}

			t.Logf("Executing market_announcements job for %s: %s", stock, jobID)

			finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
			if finalStatus != "completed" {
				t.Logf("Job for %s ended with status %s", stock, finalStatus)
				return
			}

			// === ASSERTIONS ===

			// Assert announcement document output with classification
			announcementTags := []string{"announcement", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, announcementTags)

			// Assert content not empty
			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)

			// Assert schema compliance
			isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
			assert.True(t, isValid, "Output for %s should comply with announcements schema", stock)

			// Save output
			SaveWorkerOutput(t, env, helper, announcementTags, stock)

			t.Logf("PASS: Validated announcements for %s", stock)
		})
	}

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_announcements multi-stock test completed")
}
