package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestratorWorkerDisplay(t *testing.T) {
	utc := NewUITestContext(t, 5*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Orchestrator Worker UI ---")

	jobName := "Fact Check: Flat Earth Claim" // Matches TOML name

	// 1. Submit/Ensure Job Exists (using helper)
	// For now, we rely on the file system loading 'orchestrator-fact-check.toml'
	// OR we POST it dynamically here if hot-reloading is supported.

	// 2. Trigger Job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job %s: %v", jobName, err)
	}

	// 3. Navigate to Queue
	err = utc.Navigate(utc.QueueURL)
	require.NoError(t, err)

	// 4. Assert Job in Tree (Business Logic: It must exist and be of type Orchestrator)
	var jobType string
	err = chromedp.Run(utc.Ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				// Find card with title
				const titles = Array.from(document.querySelectorAll('.card-title'));
				const card = titles.find(el => el.textContent.includes('%s'))?.closest('.card');
				if (!card) return "NOT_FOUND";
				
				// Return type badge text
				return card.querySelector('.label-primary')?.textContent || "UNKNOWN";
			})()
		`, jobName), &jobType),
	)
	require.NoError(t, err)

	// Expect failure here until implemented
	assert.Contains(t, jobType, "orchestrator", "Job type should be visible in UI")

	// 5. Expand and Check Execution Flow (Business Logic: "Thinking" logs)
	// ... (Click expand)
	// ... (Check logs for "Step 1: Search for scientific consensus")

	// 6. Check Final Output (Business Logic: Structured verdict)
	// ... (Click 'View Output')
	// ... (Assert text contains "verdict = FALSE")
}
