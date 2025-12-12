package ui

import (
	"testing"
	"time"
)

// TestJobDefinitionCodebaseClassify tests the Codebase Classify job definition end-to-end
func TestJobDefinitionCodebaseClassify(t *testing.T) {
	utc := NewUITestContext(t, 20*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: Codebase Classify ---")

	config := JobDefinitionTestConfig{
		JobName:           "Codebase Classify",
		JobDefinitionPath: "../config/job-definitions/codebase_classify.toml",
		Timeout:           15 * time.Minute, // Job has 4h timeout but tests use shorter
		RequiredEnvVars:   nil,              // rule_classifier doesn't need API keys
		AllowFailure:      true,             // May fail if paths don't exist in test env
	}

	if err := utc.RunJobDefinitionTest(config); err != nil {
		t.Fatalf("Job definition test failed: %v", err)
	}

	utc.Log("✓ Codebase Classify job definition test completed successfully")
}
