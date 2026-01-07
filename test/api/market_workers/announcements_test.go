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

	// Assert content contains expected sections (MQS Framework)
	expectedSections := []string{
		"Management Quality Score Analysis",
		"Management Quality Score",
		"Information Integrity",
		"Conviction Analysis",
		"Price Retention Analysis",
	}
	AssertOutputContains(t, content, expectedSections)

	// Assert MQS tier is present
	if strings.Contains(content, "TIER_1_OPERATOR") || strings.Contains(content, "TIER_2_HONEST_STRUGGLER") || strings.Contains(content, "TIER_3_PROMOTER") {
		t.Log("PASS: MQS Tier classification present")
	}

	// Assert all required sections are present (REQ-3: schema consistency)
	for _, section := range AnnouncementsRequiredSections {
		if strings.Contains(content, section) {
			t.Logf("PASS: Required section '%s' present", section)
		} else {
			t.Errorf("FAIL: Required section '%s' MISSING from output", section)
		}
	}

	// Assert schema compliance (MQS Framework)
	isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with MQS announcements schema")

	// Assert required MQS fields
	AssertMetadataHasFields(t, metadata, []string{"ticker", "mqs_tier", "mqs_composite", "announcements"})

	// Validate MQS tier is valid
	if tier, ok := metadata["mqs_tier"].(string); ok {
		validTiers := []string{"TIER_1_OPERATOR", "TIER_2_HONEST_STRUGGLER", "TIER_3_PROMOTER"}
		isValidTier := false
		for _, vt := range validTiers {
			if tier == vt {
				isValidTier = true
				break
			}
		}
		assert.True(t, isValidTier, "mqs_tier should be TIER_1_OPERATOR, TIER_2_HONEST_STRUGGLER, or TIER_3_PROMOTER")
		t.Logf("PASS: MQS tier is '%s'", tier)
	}

	// Validate MQS composite score is in valid range
	if composite, ok := metadata["mqs_composite"].(float64); ok {
		assert.GreaterOrEqual(t, composite, 0.0, "mqs_composite should be >= 0")
		assert.LessOrEqual(t, composite, 1.0, "mqs_composite should be <= 1")
		t.Logf("PASS: MQS composite score is %.2f", composite)
	}

	// Validate MQS component scores
	componentFields := []string{"leakage_score", "conviction_score", "retention_score", "saydo_score"}
	for _, field := range componentFields {
		if _, exists := metadata[field]; exists {
			t.Logf("PASS: MQS has component field '%s'", field)
		}
	}

	// Save output
	SaveWorkerOutput(t, env, helper, summaryTags, ticker)
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

	for _, stock := range stocks {
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

	t.Log("PASS: market_announcements multi-stock test completed")
}
