package ui

import (
	"testing"
	"time"
)

// TestJobDefinitionNearbyRestaurantsPlaces tests the Places API job definition end-to-end
func TestJobDefinitionNearbyRestaurantsPlaces(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: Nearby Restaurants (Places API) ---")

	config := JobDefinitionTestConfig{
		JobName:           "Nearby Restaurants (Wheelers Hill)",
		JobDefinitionPath: "../config/job-definitions/nearby-restaurants-places.toml",
		Timeout:           5 * time.Minute,
		RequiredEnvVars:   []string{"QUAERO_GOOGLE_PLACES_API_KEY"},
		AllowFailure:      false,
	}

	if err := utc.RunJobDefinitionTest(config); err != nil {
		t.Fatalf("Job definition test failed: %v", err)
	}

	utc.Log("✓ Nearby Restaurants (Places) job definition test completed successfully")
}
