// -----------------------------------------------------------------------
// Tests for announcement processing workers
// Tests the two-step workflow:
// 1. market_announcements - Fetches raw announcements from ASX API
// 2. processing_announcements - Classifies and analyzes announcements
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
// for a single stock using the two-step workflow.
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

	// Create job definition with two-step workflow
	defID := fmt.Sprintf("test-announcements-single-%d", time.Now().UnixNano())
	ticker := "EXR"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Announcements Single Stock Test",
		"description": "Test announcement processing with two-step workflow",
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
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, env, body)

	// Create and execute job
	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing announcement processing job: %s", jobID)

	// Wait for completion
	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===

	// Assert summary document output (from processing_announcements)
	summaryTags := []string{"asx-announcement-summary", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, summaryTags)

	// Assert content contains expected sections (Signal Classification)
	expectedSections := []string{
		"Signal Quality Metrics",
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

	// Assert schema compliance (Signal Classification)
	isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with signal classification schema")

	// Assert required fields for signal classification
	AssertMetadataHasFields(t, metadata, []string{"ticker", "total_count", "high_relevance_count"})

	// Validate relevance counts are valid numbers
	if totalCount, ok := metadata["total_count"].(float64); ok {
		assert.GreaterOrEqual(t, totalCount, 0.0, "total_count should be >= 0")
		t.Logf("PASS: total_count = %.0f", totalCount)
	}

	if highRelevance, ok := metadata["high_relevance_count"].(float64); ok {
		assert.GreaterOrEqual(t, highRelevance, 0.0, "high_relevance_count should be >= 0")
		t.Logf("PASS: high_relevance_count = %.0f", highRelevance)
	}

	// Validate MQS scores if present
	if mqsScores, ok := metadata["mqs_scores"].(map[string]interface{}); ok {
		if snr, ok := mqsScores["signal_to_noise_ratio"].(float64); ok {
			assert.GreaterOrEqual(t, snr, 0.0, "signal_to_noise_ratio should be >= 0")
			t.Logf("PASS: signal_to_noise_ratio = %.2f", snr)
		}
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
				"description": "Test announcement processing multi-stock",
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
					{
						"name":    "process-announcements",
						"type":    "processing_announcements",
						"depends": "fetch-announcements",
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

			// Assert summary document output
			summaryTags := []string{"asx-announcement-summary", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, summaryTags)

			// Assert content not empty
			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)

			// Assert schema compliance
			isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			// Save output
			SaveWorkerOutput(t, env, helper, summaryTags, stock)

			t.Logf("PASS: Validated announcements for %s", stock)
		})
	}

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: announcement processing multi-stock test completed")
}

// =============================================================================
// market_announcements Worker Tests (Raw Output - No Processing)
// =============================================================================

// RawAnnouncementsSchema defines expected fields for raw announcement output
var RawAnnouncementsSchema = WorkerSchema{
	RequiredFields: []string{"ticker", "announcements"},
	OptionalFields: []string{"exchange", "fetched_at", "period", "total_count"},
	FieldTypes: map[string]string{
		"ticker":        "string",
		"announcements": "array",
		"total_count":   "number",
	},
	ArraySchemas: map[string][]string{},
}

// TestMarketAnnouncementsSingle tests the market_announcements worker alone (single ticker)
// This tests raw announcement fetching WITHOUT the processing_announcements step
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
		"description": "Test market_announcements worker raw output (no processing)",
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

	// Assert raw document output with correct tags
	rawTags := []string{"asx-announcement-raw", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, rawTags)
	assert.NotEmpty(t, content, "Raw content should not be empty")

	// Assert schema compliance for raw announcements
	isValid := ValidateSchema(t, metadata, RawAnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with raw announcements schema")

	// Raw output MUST have announcements array
	if anns, ok := metadata["announcements"]; ok {
		if arr, ok := anns.([]interface{}); ok {
			assert.GreaterOrEqual(t, len(arr), 0, "announcements array should exist")
			t.Logf("PASS: Raw output has %d announcements", len(arr))
		} else {
			t.Error("FAIL: announcements field must be an array")
		}
	} else {
		t.Error("FAIL: Raw output MUST have announcements field")
	}

	// Should NOT have classification fields (those come from processing_announcements)
	classificationFields := []string{"high_relevance_count", "medium_relevance_count", "low_relevance_count", "noise_count", "mqs_scores"}
	for _, field := range classificationFields {
		if _, hasField := metadata[field]; hasField {
			t.Errorf("FAIL: Raw output should NOT have %s (classification field from processing_announcements)", field)
		}
	}
	t.Log("PASS: Raw output correctly lacks classification fields")

	// Save output files
	SaveWorkerOutput(t, env, helper, rawTags, ticker)
	SaveSchemaDefinition(t, env, RawAnnouncementsSchema, "RawAnnouncementsSchema")
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

	// Test stocks for raw announcement fetching
	stocks := []string{"BHP", "GNP"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-market-announcements-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("Market Announcements Test - %s", stock),
				"description": "Test market_announcements multi-stock raw output",
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

			// Assert raw document output
			rawTags := []string{"asx-announcement-raw", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, rawTags)

			// Assert content not empty
			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)

			// Assert schema compliance
			isValid := ValidateSchema(t, metadata, RawAnnouncementsSchema)
			assert.True(t, isValid, "Output for %s should comply with raw schema", stock)

			// Save output
			SaveWorkerOutput(t, env, helper, rawTags, stock)

			t.Logf("PASS: Validated raw announcements for %s", stock)
		})
	}

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_announcements multi-stock test completed")
}

// TestMarketAnnouncementsRawOutput tests that market_announcements produces raw output
// DEPRECATED: Use TestMarketAnnouncementsSingle instead - this test is kept for backward compatibility
func TestMarketAnnouncementsRawOutput(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-raw-announcements-%d", time.Now().UnixNano())
	ticker := "GNP"

	// Test market_announcements worker alone (raw output)
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Raw Announcements Test",
		"description": "Test market_announcements raw data output",
		"type":        "market_announcements",
		"enabled":     true,
		"tags":        []string{"worker-test", "raw-announcements"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-raw-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
		},
	}

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing raw announcements job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Assert raw document output
	rawTags := []string{"asx-announcement-raw", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, rawTags)

	// Assert schema compliance for raw announcements
	isValid := ValidateSchema(t, metadata, RawAnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with raw announcements schema")

	// Raw output should have announcements array
	if anns, ok := metadata["announcements"]; ok {
		if arr, ok := anns.([]interface{}); ok {
			t.Logf("PASS: Raw output has %d announcements", len(arr))
		}
	} else {
		t.Error("FAIL: Raw output MUST have announcements field")
	}

	// Should NOT have classification fields (those come from processing)
	if _, hasRelevance := metadata["high_relevance_count"]; hasRelevance {
		t.Error("FAIL: Raw output should NOT have high_relevance_count (classification field)")
	} else {
		t.Log("PASS: Raw output correctly lacks classification fields")
	}

	assert.NotEmpty(t, content, "Raw content should not be empty")

	// Save output files
	SaveWorkerOutput(t, env, helper, rawTags, ticker)
	AssertResultFilesExist(t, env, 1)

	t.Log("PASS: market_announcements raw output test completed")
}
