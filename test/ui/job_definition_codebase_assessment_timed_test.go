package ui

import (
	"testing"
	"time"
)

// TestJobDefinitionCodebaseAssessmentTimed runs the timed assessment job definition
// and enforces passable timing for each major step.
func TestJobDefinitionCodebaseAssessmentTimed(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	jobName := "Codebase Assessment (Timed)"
	jobDefPath := "../config/job-definitions/codebase_assessment_timed.toml"

	recordTiming := func(name string, start time.Time) time.Duration {
		duration := time.Since(start)
		utc.Log("%s completed in %v", name, duration.Round(time.Millisecond))
		return duration
	}

	assertTiming := func(name string, duration, limit time.Duration) {
		if duration > limit {
			t.Fatalf("%s exceeded limit: %v > %v", name, duration, limit)
		}
		utc.Log("✓ %s within limit (%v <= %v)", name, duration.Round(time.Millisecond), limit)
	}

	// Copy job definition for reference
	start := time.Now()
	if err := utc.CopyJobDefinitionToResults(jobDefPath); err != nil {
		t.Fatalf("Failed to copy job definition: %v", err)
	}
	copyDuration := recordTiming("Copy job definition", start)

	// Navigate to Jobs page and capture initial state
	start = time.Now()
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}
	if err := utc.Screenshot("codebase_assessment_timed_job_definition"); err != nil {
		t.Fatalf("Failed to capture job definition screenshot: %v", err)
	}
	navigationDuration := recordTiming("Navigate to Jobs page", start)

	// Trigger the job
	start = time.Now()
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}
	triggerDuration := recordTiming("Trigger job", start)

	// Monitor the job until completion
	monitorOpts := MonitorJobOptions{
		Timeout:              MaxJobTestTimeout,
		ExpectDocuments:      false,
		ValidateAllProcessed: false,
		AllowFailure:         false,
	}

	start = time.Now()
	if err := utc.MonitorJob(jobName, monitorOpts); err != nil {
		t.Fatalf("Failed to monitor job: %v", err)
	}
	monitorDuration := recordTiming("Monitor job", start)

	// Final refresh and screenshot
	start = time.Now()
	if err := utc.RefreshAndScreenshot("codebase_assessment_timed_final_state"); err != nil {
		t.Fatalf("Failed to refresh and screenshot: %v", err)
	}
	finalizeDuration := recordTiming("Finalize job run", start)

	// Timing assertions
	assertTiming("Definition copy", copyDuration, 15*time.Second)
	assertTiming("Jobs navigation", navigationDuration, 20*time.Second)
	assertTiming("Job trigger", triggerDuration, 10*time.Second)
	assertTiming("Job monitoring", monitorDuration, 8*time.Minute)
	assertTiming("Finalization", finalizeDuration, 30*time.Second)

	utc.Log("✓ Timed assessment job definition completed successfully")
}
