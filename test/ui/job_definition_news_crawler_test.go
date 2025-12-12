package ui

import (
	"testing"
	"time"
)

// TestJobDefinitionNewsCrawler tests the News Crawler job definition end-to-end
func TestJobDefinitionNewsCrawler(t *testing.T) {
	utc := NewUITestContext(t, 15*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: News Crawler ---")

	config := JobDefinitionTestConfig{
		JobName:           "News Crawler",
		JobDefinitionPath: "../config/job-definitions/news-crawler.toml",
		Timeout:           10 * time.Minute,
		RequiredEnvVars:   nil, // No API keys needed
		AllowFailure:      false,
	}

	if err := utc.RunJobDefinitionTest(config); err != nil {
		t.Fatalf("Job definition test failed: %v", err)
	}

	utc.Log("✓ News Crawler job definition test completed successfully")
}
