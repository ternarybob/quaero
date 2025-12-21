package ui

import (
	"testing"
	"time"
)

// TestJobDefinitionNearbyRestaurantsKeywords tests the multi-step Places + Keywords job definition
func TestJobDefinitionNearbyRestaurantsKeywords(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: Nearby Restaurants + Keywords ---")

	config := JobDefinitionTestConfig{
		JobName:           "Nearby Restaurants + Keywords (Wheelers Hill)",
		JobDefinitionPath: "../config/job-definitions/nearby-restaurants-keywords.toml",
		Timeout:           8 * time.Minute,
		RequiredEnvVars:   []string{"QUAERO_GOOGLE_PLACES_API_KEY", "QUAERO_AGENT_GOOGLE_API_KEY"},
		AllowFailure:      true, // Agent step may hit rate limits
	}

	if err := utc.RunJobDefinitionTest(config); err != nil {
		t.Fatalf("Job definition test failed: %v", err)
	}

	utc.Log("✓ Nearby Restaurants + Keywords job definition test completed successfully")
}
