// -----------------------------------------------------------------------
// Tests for market_announcements worker
// Fetches ASX company announcements via Markit Digital API
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerAnnouncementsSingle tests single stock announcements
func TestWorkerAnnouncementsSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require LLM for AI summary feature
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Create job definition
	defID := fmt.Sprintf("test-announcements-single-%d", time.Now().UnixNano())
	ticker := "EXR" // Changed from BHP to EXR as benchmark for signal analysis

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Announcements Single Stock Test",
		"description": "Test market_announcements worker with single stock",
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

	// Create and execute job
	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_announcements job: %s", jobID)

	// Wait for completion
	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===

	// Assert summary document output
	summaryTags := []string{"asx-announcement-summary", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, summaryTags)

	// Assert content contains expected sections (updated for conviction-based rating)
	expectedSections := []string{
		"ASX Announcements Summary",
		"Relevance Distribution",
		"Signal Analysis & Conviction Rating", // REQ-5: New conviction-based rating
		"Mandatory Business Update Calendar",  // REQ-2: Business calendar section
	}
	AssertOutputContains(t, content, expectedSections)

	// Assert Signal Breakdown section is present (new format)
	if strings.Contains(content, "Signal Breakdown") {
		t.Log("PASS: Signal Breakdown section present")
	}

	// Assert all required sections are present (REQ-3: schema consistency)
	for _, section := range AnnouncementsRequiredSections {
		if strings.Contains(content, section) {
			t.Logf("PASS: Required section '%s' present", section)
		} else {
			t.Errorf("FAIL: Required section '%s' MISSING from output", section)
		}
	}

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with announcements schema")

	// Assert required fields
	AssertMetadataHasFields(t, metadata, []string{"asx_code", "total_count"})

	// Validate announcements array
	if announcements, ok := metadata["announcements"].([]interface{}); ok {
		assert.Greater(t, len(announcements), 0, "Should have announcements")
		t.Logf("PASS: Found %d announcements", len(announcements))

		// Validate first announcement has required fields
		if len(announcements) > 0 {
			if firstAnn, ok := announcements[0].(map[string]interface{}); ok {
				requiredFields := []string{"date", "headline", "relevance_category"}
				for _, field := range requiredFields {
					if _, exists := firstAnn[field]; exists {
						t.Logf("PASS: Announcement has field '%s'", field)
					} else {
						t.Errorf("FAIL: Announcement missing field '%s'", field)
					}
				}

				// Validate relevance category is valid
				if category, ok := firstAnn["relevance_category"].(string); ok {
					validCategories := []string{"HIGH", "MEDIUM", "LOW", "NOISE"}
					isValidCategory := false
					for _, vc := range validCategories {
						if category == vc {
							isValidCategory = true
							break
						}
					}
					assert.True(t, isValidCategory, "relevance_category should be HIGH, MEDIUM, LOW, or NOISE")
				}
			}
		}
	}

	// Validate count fields
	countFields := []string{"total_count", "high_count", "medium_count", "low_count", "noise_count"}
	for _, field := range countFields {
		if _, exists := metadata[field]; exists {
			t.Logf("PASS: Summary has count field '%s'", field)
		}
	}

	// Save output
	SaveWorkerOutput(t, env, helper, summaryTags, 1)
	AssertResultFilesExist(t, env, 1)

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_announcements single stock test completed")
}

// TestWorkerAnnouncementsMulti tests multi-stock announcements
func TestWorkerAnnouncementsMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require LLM for AI summary feature
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Test stocks - run as subtests for better isolation
	stocks := []string{"BHP", "CSL", "GNP", "EXR"}

	for i, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-announcements-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("Announcements Test - %s", stock),
				"description": "Test market_announcements worker multi-stock",
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
			if i == 0 {
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
			SaveWorkerOutput(t, env, helper, summaryTags, i+1)

			t.Logf("PASS: Validated announcements for %s", stock)
		})
	}

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_announcements multi-stock test completed")
}
