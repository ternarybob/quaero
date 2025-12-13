// job_types_test.go - Specific job type tests
// Tests individual job types: places, crawler, agent, multi-step

package ui

import (
	"testing"
	"time"
)

// TestPlacesJob tests the Nearby Restaurants (Places API) job
func TestPlacesJob(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	jobName := "Nearby Restaurants (Wheelers Hill)"

	utc.Log("--- Testing Places Job ---")

	// Check for API key
	if utc.Env.EnvVars["QUAERO_GOOGLE_PLACES_API_KEY"] == "" {
		t.Skip("Skipping: QUAERO_GOOGLE_PLACES_API_KEY not set")
	}

	// Trigger and monitor the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	opts := MonitorJobOptions{
		Timeout:              2 * time.Minute,
		ExpectDocuments:      true,
		ValidateAllProcessed: false,
		AllowFailure:         false,
	}
	if err := utc.MonitorJob(jobName, opts); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	utc.Log("✓ Places job completed successfully")
}

// TestNewsCrawlerJob tests the News Crawler job
func TestNewsCrawlerJob(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	jobName := "News Crawler"

	utc.Log("--- Testing News Crawler Job ---")

	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	opts := MonitorJobOptions{
		Timeout:              8 * time.Minute,
		ExpectDocuments:      true,
		ValidateAllProcessed: false,
		AllowFailure:         false,
	}
	if err := utc.MonitorJob(jobName, opts); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	utc.Log("✓ News Crawler job completed successfully")
}

// TestKeywordExtractionJob tests the Keyword Extraction agent job
func TestKeywordExtractionJob(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Keyword Extraction Job ---")

	// Check for API key
	if utc.Env.EnvVars["QUAERO_AGENT_GOOGLE_API_KEY"] == "" {
		t.Skip("Skipping: QUAERO_AGENT_GOOGLE_API_KEY not set")
	}

	// First run Places job to create documents
	placesJobName := "Nearby Restaurants (Wheelers Hill)"
	if err := utc.TriggerJob(placesJobName); err != nil {
		t.Fatalf("Failed to trigger places job: %v", err)
	}

	placesOpts := MonitorJobOptions{
		Timeout:              2 * time.Minute,
		ExpectDocuments:      true,
		ValidateAllProcessed: false,
		AllowFailure:         false,
	}
	if err := utc.MonitorJob(placesJobName, placesOpts); err != nil {
		t.Fatalf("Places job failed: %v", err)
	}
	utc.Log("✓ Places job completed, now running keyword extraction")

	// Run Keyword Extraction on those documents
	agentJobName := "Keyword Extraction"
	if err := utc.TriggerJob(agentJobName); err != nil {
		t.Fatalf("Failed to trigger agent job: %v", err)
	}

	agentOpts := MonitorJobOptions{
		Timeout:              5 * time.Minute,
		ExpectDocuments:      true,
		ValidateAllProcessed: false,
		AllowFailure:         false,
	}
	if err := utc.MonitorJob(agentJobName, agentOpts); err != nil {
		t.Fatalf("Agent job failed: %v", err)
	}

	utc.Log("✓ Keyword Extraction job completed successfully")
}

// TestMultiStepJob tests a multi-step job (Places + Keywords)
func TestMultiStepJob(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	jobName := "Nearby Restaurants + Keywords"

	utc.Log("--- Testing Multi-Step Job ---")

	// Check for required API keys
	if utc.Env.EnvVars["QUAERO_GOOGLE_PLACES_API_KEY"] == "" {
		t.Skip("Skipping: QUAERO_GOOGLE_PLACES_API_KEY not set")
	}
	if utc.Env.EnvVars["QUAERO_AGENT_GOOGLE_API_KEY"] == "" {
		t.Skip("Skipping: QUAERO_AGENT_GOOGLE_API_KEY not set")
	}

	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Multi-step jobs need more time
	opts := MonitorJobOptions{
		Timeout:              8 * time.Minute,
		ExpectDocuments:      true,
		ValidateAllProcessed: false,
		AllowFailure:         true, // Keyword step may fail due to rate limits
	}
	if err := utc.MonitorJob(jobName, opts); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	utc.Log("✓ Multi-step job completed successfully")
}
