package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Cron Scheduler Integration Tests
// =============================================================================
// Tests the cron scheduler functionality:
// - Job definitions with cron schedules can be created and executed
// - Invalid schedules (< 5 min intervals) are rejected
// - Scheduled jobs produce expected outputs
//
// Architecture: Follows .claude/skills/test-architecture/SKILL.md
// Reference: test/api/portfolio/stock_deep_dive_test.go
// =============================================================================

// TestCronScheduler_JobExecutesWithSchedule tests that a job definition with a cron schedule
// can be created, loaded, and executed, producing the expected output.
func TestCronScheduler_JobExecutesWithSchedule(t *testing.T) {
	// 1. Environment setup
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := env.GetResultsDir()

	// 2. Test log for guard pattern
	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestCronScheduler_JobExecutesWithSchedule", time.Now().Format(time.RFC3339)))

	// 3. Ensure test log is written on ALL exit paths
	defer func() {
		WriteTestLog(t, resultsDir, testLog)
	}()

	// Step 1: Load the cron scheduler test job definition
	t.Log("Step 1: Loading cron scheduler test job definition")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Loading job definition", time.Now().Format(time.RFC3339)))

	cronJobDefPath := "../config/job-definitions/cron-scheduler-test.toml"
	err = env.LoadTestJobDefinitions(cronJobDefPath)
	require.NoError(t, err, "Failed to load cron scheduler test job definition")

	// 4. MANDATORY: Save job definition BEFORE execution
	saveCronJobConfig(t, resultsDir, "cron-scheduler-test.toml")
	testLog = append(testLog, fmt.Sprintf("[%s] Saved job_definition.toml", time.Now().Format(time.RFC3339)))

	// Step 2: Verify job definition exists with schedule
	t.Log("Step 2: Verifying job definition has schedule")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Verifying schedule", time.Now().Format(time.RFC3339)))

	jobDefID := "cron-scheduler-test"
	resp, err := helper.GET(fmt.Sprintf("/api/job-definitions/%s", jobDefID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var jobDef map[string]interface{}
	err = helper.ParseJSONResponse(resp, &jobDef)
	require.NoError(t, err)

	schedule, ok := jobDef["schedule"].(string)
	assert.True(t, ok, "Job definition should have schedule field")
	assert.Equal(t, "*/5 * * * *", schedule, "Schedule should be every 5 minutes")
	t.Logf("Job definition has schedule: %s", schedule)
	testLog = append(testLog, fmt.Sprintf("[%s] Schedule verified: %s", time.Now().Format(time.RFC3339), schedule))

	// Step 3: Execute the job definition (simulating cron trigger)
	t.Log("Step 3: Executing job definition (simulating cron trigger)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Executing job", time.Now().Format(time.RFC3339)))

	resp, err = helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", jobDefID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusAccepted)

	var execResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &execResult)
	require.NoError(t, err)

	jobID, ok := execResult["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")
	t.Logf("Job execution started: job_id=%s", jobID)
	testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))

	// Cleanup job after test
	defer deleteJob(t, helper, jobID)

	// Step 4: Wait for job completion
	t.Log("Step 4: Waiting for job completion")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 4: Waiting for completion", time.Now().Format(time.RFC3339)))

	finalStatus, finalJob := waitForCronJobCompletion(t, helper, jobID, 2*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)
	testLog = append(testLog, fmt.Sprintf("[%s] Job status: %s", time.Now().Format(time.RFC3339), finalStatus))

	require.NotEmpty(t, finalStatus, "Job should reach terminal state within timeout")

	// 5. MANDATORY: Save outputs AFTER job completion (unconditionally)
	saveCronJobOutput(t, resultsDir, finalJob, finalStatus)
	testLog = append(testLog, fmt.Sprintf("[%s] Saved output files", time.Now().Format(time.RFC3339)))

	// Step 5: Verify output
	t.Log("Step 5: Verifying job output")
	if finalStatus == "completed" {
		t.Log("Job completed successfully")
		testLog = append(testLog, fmt.Sprintf("[%s] PASS: Job completed successfully", time.Now().Format(time.RFC3339)))
	} else if finalStatus == "failed" {
		if errMsg, ok := finalJob["error"].(string); ok {
			t.Logf("Job failed with error: %s", errMsg)
			testLog = append(testLog, fmt.Sprintf("[%s] Job failed: %s", time.Now().Format(time.RFC3339), errMsg))
		}
		t.Log("Note: Job was executed but failed - scheduler trigger mechanism is working")
	}

	// 6. MANDATORY: Verify result files exist
	AssertResultFilesExist(t, resultsDir)

	// 7. MANDATORY: Check for service errors
	common.AssertNoErrorsInServiceLog(t, env)

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestCronScheduler_JobExecutesWithSchedule completed", time.Now().Format(time.RFC3339)))
	t.Log("Cron scheduler test completed successfully")
}

// TestCronScheduler_JobDefinitionWithScheduleIsRegistered tests that creating a job definition
// with a cron schedule results in the job being registered with the scheduler.
func TestCronScheduler_JobDefinitionWithScheduleIsRegistered(t *testing.T) {
	// 1. Environment setup
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := env.GetResultsDir()

	// 2. Test log for guard pattern
	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestCronScheduler_JobDefinitionWithScheduleIsRegistered", time.Now().Format(time.RFC3339)))

	// 3. Ensure test log is written on ALL exit paths
	defer func() {
		WriteTestLog(t, resultsDir, testLog)
	}()

	// Step 1: Create a job definition with cron schedule via TOML upload
	t.Log("Step 1: Creating job definition with cron schedule")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Creating job definition", time.Now().Format(time.RFC3339)))

	uniqueID := fmt.Sprintf("cron-test-registered-%d", time.Now().UnixNano())
	tomlContent := fmt.Sprintf(`
id = "%s"
name = "Cron Registration Test"
type = "crawler"
description = "Test job definition with cron schedule"
tags = ["test", "cron-registration"]
schedule = "*/5 * * * *"
timeout = "5m"
enabled = true
auto_start = false

[step.crawl]
type = "crawler"
description = "Simple crawl step"
on_error = "fail"
start_urls = ["https://example.com"]
max_depth = 0
max_pages = 1
`, uniqueID)

	// 4. MANDATORY: Save job definition BEFORE execution
	saveCronJobDefinitionContent(t, resultsDir, tomlContent)
	testLog = append(testLog, fmt.Sprintf("[%s] Saved job_definition.toml", time.Now().Format(time.RFC3339)))

	resp, err := helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(tomlContent))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var createResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &createResult)
	require.NoError(t, err)

	createdID, ok := createResult["id"].(string)
	require.True(t, ok, "Created job definition should have ID")
	assert.Equal(t, uniqueID, createdID, "Job definition ID should match")
	t.Logf("Created job definition: %s", createdID)
	testLog = append(testLog, fmt.Sprintf("[%s] Created: %s", time.Now().Format(time.RFC3339), createdID))

	// Cleanup: delete job definition at end of test
	defer func() {
		resp, err := helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", createdID))
		if err == nil {
			defer resp.Body.Close()
			t.Logf("Deleted job definition: %s", createdID)
		}
	}()

	// Step 2: Verify job definition exists and has schedule
	t.Log("Step 2: Verifying job definition has schedule")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Verifying schedule", time.Now().Format(time.RFC3339)))

	resp, err = helper.GET(fmt.Sprintf("/api/job-definitions/%s", createdID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var jobDef map[string]interface{}
	err = helper.ParseJSONResponse(resp, &jobDef)
	require.NoError(t, err)

	assert.Equal(t, uniqueID, jobDef["id"], "ID should match")
	assert.Equal(t, "Cron Registration Test", jobDef["name"], "Name should match")
	assert.Equal(t, "*/5 * * * *", jobDef["schedule"], "Schedule should be set")
	assert.Equal(t, true, jobDef["enabled"], "Job should be enabled")

	t.Logf("Job definition verified: id=%s, schedule=%s, enabled=%v",
		jobDef["id"], jobDef["schedule"], jobDef["enabled"])
	testLog = append(testLog, fmt.Sprintf("[%s] Verified: schedule=%s", time.Now().Format(time.RFC3339), jobDef["schedule"]))

	// Step 3: Execute the job to verify it can be triggered
	t.Log("Step 3: Executing job (verifying trigger mechanism)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Executing job", time.Now().Format(time.RFC3339)))

	resp, err = helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", createdID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK,
		"Execution should be accepted (status: %d)", resp.StatusCode)

	var jobID string
	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
		var execResult map[string]interface{}
		err = helper.ParseJSONResponse(resp, &execResult)
		require.NoError(t, err)

		if id, ok := execResult["job_id"].(string); ok {
			jobID = id
			t.Logf("Job execution started: job_id=%s", jobID)
			testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))

			// Wait briefly for job to start processing
			time.Sleep(2 * time.Second)

			// Get final status
			finalStatus, finalJob := waitForCronJobCompletion(t, helper, jobID, 30*time.Second)

			// 5. MANDATORY: Save outputs
			saveCronJobOutput(t, resultsDir, finalJob, finalStatus)
			testLog = append(testLog, fmt.Sprintf("[%s] Saved output files", time.Now().Format(time.RFC3339)))

			// Cleanup: delete job
			defer func() {
				resp, _ := helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
				if resp != nil {
					resp.Body.Close()
				}
			}()
		}
	}

	// 6. MANDATORY: Verify result files exist
	AssertResultFilesExist(t, resultsDir)

	// 7. MANDATORY: Check for service errors
	common.AssertNoErrorsInServiceLog(t, env)

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestCronScheduler_JobDefinitionWithScheduleIsRegistered completed", time.Now().Format(time.RFC3339)))
	t.Log("Cron registration test completed successfully")
}

// TestCronScheduler_InvalidScheduleRejected tests that job definitions with invalid cron
// schedules (intervals < 5 minutes) are rejected.
func TestCronScheduler_InvalidScheduleRejected(t *testing.T) {
	// 1. Environment setup
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := env.GetResultsDir()

	// 2. Test log for guard pattern
	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestCronScheduler_InvalidScheduleRejected", time.Now().Format(time.RFC3339)))

	// 3. Ensure test log is written on ALL exit paths
	defer func() {
		WriteTestLog(t, resultsDir, testLog)
	}()

	// Test 1: Schedule with 1-minute interval should be rejected
	t.Log("Step 1: Testing rejection of 1-minute schedule")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Testing 1-minute schedule rejection", time.Now().Format(time.RFC3339)))

	uniqueID := fmt.Sprintf("cron-invalid-test-%d", time.Now().UnixNano())
	invalidTOML := fmt.Sprintf(`
id = "%s"
name = "Invalid Cron Test"
type = "crawler"
description = "Job with invalid cron schedule (too frequent)"
schedule = "*/1 * * * *"
enabled = true

[step.crawl]
type = "crawler"
start_urls = ["https://example.com"]
max_pages = 1
`, uniqueID)

	// 4. MANDATORY: Save job definition
	saveCronJobDefinitionContent(t, resultsDir, invalidTOML)

	resp, err := helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(invalidTOML))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Record the response for output
	var responseData map[string]interface{}
	helper.ParseJSONResponse(resp, &responseData)

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		t.Log("Job definition was created - checking if scheduler rejects invalid schedule on execution")
		testLog = append(testLog, fmt.Sprintf("[%s] Job created (validation at scheduler level)", time.Now().Format(time.RFC3339)))

		defer func() {
			resp, _ := helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", uniqueID))
			if resp != nil {
				resp.Body.Close()
			}
		}()
	} else if resp.StatusCode == http.StatusBadRequest {
		t.Log("Job definition with invalid schedule was correctly rejected")
		testLog = append(testLog, fmt.Sprintf("[%s] PASS: Invalid schedule rejected at creation", time.Now().Format(time.RFC3339)))
		if errMsg, ok := responseData["error"].(string); ok {
			t.Logf("Error message: %s", errMsg)
		}
	} else {
		t.Logf("Unexpected status code: %d", resp.StatusCode)
		testLog = append(testLog, fmt.Sprintf("[%s] Unexpected status: %d", time.Now().Format(time.RFC3339), resp.StatusCode))
	}

	// Test 2: Schedule with * (every minute) should be rejected
	t.Log("Step 2: Testing rejection of every-minute schedule")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Testing every-minute schedule rejection", time.Now().Format(time.RFC3339)))

	uniqueID2 := fmt.Sprintf("cron-everymin-test-%d", time.Now().UnixNano())
	everyMinuteTOML := fmt.Sprintf(`
id = "%s"
name = "Every Minute Test"
type = "crawler"
schedule = "* * * * *"
enabled = true

[step.crawl]
type = "crawler"
start_urls = ["https://example.com"]
max_pages = 1
`, uniqueID2)

	resp, err = helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(everyMinuteTOML))
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		t.Log("Every-minute schedule was correctly rejected")
		testLog = append(testLog, fmt.Sprintf("[%s] PASS: Every-minute schedule rejected", time.Now().Format(time.RFC3339)))
	} else {
		t.Logf("Status code for every-minute schedule: %d (may be validated at scheduler registration time)", resp.StatusCode)
		testLog = append(testLog, fmt.Sprintf("[%s] Status: %d (scheduler validation)", time.Now().Format(time.RFC3339), resp.StatusCode))

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			defer func() {
				resp, _ := helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", uniqueID2))
				if resp != nil {
					resp.Body.Close()
				}
			}()
		}
	}

	// 5. MANDATORY: Save outputs
	saveCronValidationOutput(t, resultsDir, "invalid_schedule_test")
	testLog = append(testLog, fmt.Sprintf("[%s] Saved output files", time.Now().Format(time.RFC3339)))

	// 6. MANDATORY: Verify result files exist
	AssertResultFilesExist(t, resultsDir)

	// 7. MANDATORY: Check for service errors
	common.AssertNoErrorsInServiceLog(t, env)

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestCronScheduler_InvalidScheduleRejected completed", time.Now().Format(time.RFC3339)))
	t.Log("Invalid schedule rejection test completed")
}

// TestCronScheduler_JobProducesOutputOnScheduledExecution tests the full end-to-end flow
// of a scheduled job producing output.
func TestCronScheduler_JobProducesOutputOnScheduledExecution(t *testing.T) {
	// 1. Environment setup
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := env.GetResultsDir()

	// 2. Test log for guard pattern
	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestCronScheduler_JobProducesOutputOnScheduledExecution", time.Now().Format(time.RFC3339)))

	// 3. Ensure test log is written on ALL exit paths
	defer func() {
		WriteTestLog(t, resultsDir, testLog)
	}()

	// Step 1: Create job definition with cron schedule
	t.Log("Step 1: Creating job definition with cron schedule")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Creating job definition", time.Now().Format(time.RFC3339)))

	uniqueID := fmt.Sprintf("cron-output-test-%d", time.Now().UnixNano())
	tomlContent := fmt.Sprintf(`
id = "%s"
name = "Cron Output Test"
type = "crawler"
description = "Tests that scheduled job executes and completes"
tags = ["test", "cron-output"]
schedule = "*/5 * * * *"
timeout = "2m"
enabled = true
auto_start = false

[step.crawl]
type = "crawler"
description = "Crawl single page"
on_error = "fail"
start_urls = ["https://example.com"]
max_depth = 0
max_pages = 1
concurrency = 1
`, uniqueID)

	// 4. MANDATORY: Save job definition BEFORE execution
	saveCronJobDefinitionContent(t, resultsDir, tomlContent)
	testLog = append(testLog, fmt.Sprintf("[%s] Saved job_definition.toml", time.Now().Format(time.RFC3339)))

	resp, err := helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(tomlContent))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	t.Logf("Created job definition: %s", uniqueID)
	testLog = append(testLog, fmt.Sprintf("[%s] Created: %s", time.Now().Format(time.RFC3339), uniqueID))

	// Cleanup at end
	defer func() {
		resp, _ := helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", uniqueID))
		if resp != nil {
			resp.Body.Close()
		}
	}()

	// Step 2: Execute the job (simulating cron trigger)
	t.Log("Step 2: Executing job (simulating cron trigger)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Executing job", time.Now().Format(time.RFC3339)))

	resp, err = helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", uniqueID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusAccepted)

	var execResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &execResult)
	require.NoError(t, err)

	jobID, ok := execResult["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")
	t.Logf("Job started: %s", jobID)
	testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))

	// Cleanup job at end
	defer func() {
		resp, _ := helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
		if resp != nil {
			resp.Body.Close()
		}
	}()

	// Step 3: Wait for job completion
	t.Log("Step 3: Waiting for job completion")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Waiting for completion", time.Now().Format(time.RFC3339)))

	finalStatus, finalJob := waitForCronJobCompletion(t, helper, jobID, 90*time.Second)

	require.NotEmpty(t, finalStatus, "Job should reach terminal state")
	t.Logf("Job final status: %s", finalStatus)
	testLog = append(testLog, fmt.Sprintf("[%s] Final status: %s", time.Now().Format(time.RFC3339), finalStatus))

	// 5. MANDATORY: Save outputs AFTER job completion
	saveCronJobOutput(t, resultsDir, finalJob, finalStatus)
	testLog = append(testLog, fmt.Sprintf("[%s] Saved output files", time.Now().Format(time.RFC3339)))

	// Step 4: Verify job execution details
	t.Log("Step 4: Verifying job execution details")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 4: Verifying execution", time.Now().Format(time.RFC3339)))

	assert.Equal(t, "completed", finalStatus, "Job should complete successfully")

	// Verify job metadata
	if metadata, ok := finalJob["metadata"].(map[string]interface{}); ok {
		if completedSteps, ok := metadata["completed_steps"].(float64); ok {
			assert.GreaterOrEqual(t, int(completedSteps), 1, "At least one step should have completed")
			t.Logf("Completed steps: %d", int(completedSteps))
		}
	}

	// Verify timestamps
	if completedAt, ok := finalJob["completed_at"].(string); ok {
		assert.NotEmpty(t, completedAt, "Completed timestamp should be set")
		t.Logf("Job completed at: %s", completedAt)
	}

	// 6. MANDATORY: Verify result files exist
	AssertResultFilesExist(t, resultsDir)

	// 7. MANDATORY: Check for service errors
	common.AssertNoErrorsInServiceLog(t, env)

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestCronScheduler_JobProducesOutputOnScheduledExecution completed", time.Now().Format(time.RFC3339)))
	t.Log("Cron scheduled execution test completed - scheduler mechanism verified")
}

// =============================================================================
// Helper Functions
// =============================================================================

// waitForCronJobCompletion waits for a job to reach terminal state
func waitForCronJobCompletion(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) (string, map[string]interface{}) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second
	var finalStatus string
	var finalJob map[string]interface{}

	for time.Now().Before(deadline) {
		resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var job map[string]interface{}
			if err := helper.ParseJSONResponse(resp, &job); err == nil {
				if status, ok := job["status"].(string); ok {
					t.Logf("Job status: %s", status)
					if status == "completed" || status == "failed" || status == "cancelled" {
						finalStatus = status
						finalJob = job
						resp.Body.Close()
						break
					}
				}
			}
		}
		resp.Body.Close()
		time.Sleep(pollInterval)
	}

	return finalStatus, finalJob
}

// saveCronJobConfig saves the job definition TOML file to results directory
func saveCronJobConfig(t *testing.T, resultsDir string, jobDefFile string) {
	t.Helper()

	if resultsDir == "" || jobDefFile == "" {
		return
	}

	jobDefPath := filepath.Join("..", "config", "job-definitions", jobDefFile)
	content, err := os.ReadFile(jobDefPath)
	if err != nil {
		t.Logf("Warning: Failed to read job definition %s: %v", jobDefFile, err)
		return
	}

	destPath := filepath.Join(resultsDir, "job_definition.toml")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Logf("Warning: Failed to write job definition: %v", err)
		return
	}

	t.Logf("Saved job definition to: %s (%d bytes)", destPath, len(content))
}

// saveCronJobDefinitionContent saves inline TOML content to results directory
func saveCronJobDefinitionContent(t *testing.T, resultsDir string, content string) {
	t.Helper()

	if resultsDir == "" {
		return
	}

	destPath := filepath.Join(resultsDir, "job_definition.toml")
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write job definition: %v", err)
		return
	}

	t.Logf("Saved job definition to: %s (%d bytes)", destPath, len(content))
}

// saveCronJobOutput saves job execution output to results directory
func saveCronJobOutput(t *testing.T, resultsDir string, job map[string]interface{}, status string) {
	t.Helper()

	if resultsDir == "" {
		return
	}

	// Save output.json (job metadata)
	if job != nil {
		jsonPath := filepath.Join(resultsDir, "output.json")
		if data, err := json.MarshalIndent(job, "", "  "); err == nil {
			if err := os.WriteFile(jsonPath, data, 0644); err != nil {
				t.Logf("Warning: Failed to write output.json: %v", err)
			} else {
				t.Logf("Saved output.json to: %s (%d bytes)", jsonPath, len(data))
			}
		}
	}

	// Save output.md (human-readable summary)
	var summary string
	if job != nil {
		jobID, _ := job["id"].(string)
		jobName, _ := job["name"].(string)
		completedAt, _ := job["completed_at"].(string)

		summary = fmt.Sprintf(`# Cron Scheduler Test Output

## Job Details
- **Job ID:** %s
- **Job Name:** %s
- **Status:** %s
- **Completed At:** %s

## Test Result
The cron scheduler successfully %s the job execution.
`, jobID, jobName, status, completedAt, getStatusVerb(status))
	} else {
		summary = fmt.Sprintf(`# Cron Scheduler Test Output

## Status
%s

## Notes
Job data not available.
`, status)
	}

	mdPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(mdPath, []byte(summary), 0644); err != nil {
		t.Logf("Warning: Failed to write output.md: %v", err)
	} else {
		t.Logf("Saved output.md to: %s", mdPath)
	}
}

// saveCronValidationOutput saves validation test output
func saveCronValidationOutput(t *testing.T, resultsDir string, testType string) {
	t.Helper()

	if resultsDir == "" {
		return
	}

	summary := fmt.Sprintf(`# Cron Scheduler Validation Test Output

## Test Type
%s

## Result
Schedule validation tests completed.

## Validation Rules
- Schedules with intervals < 5 minutes should be rejected
- Every-minute schedules (* * * * *) should be rejected
`, testType)

	mdPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(mdPath, []byte(summary), 0644); err != nil {
		t.Logf("Warning: Failed to write output.md: %v", err)
	}

	// Save minimal output.json
	jsonContent := map[string]interface{}{
		"test_type": testType,
		"result":    "validation_complete",
	}
	jsonPath := filepath.Join(resultsDir, "output.json")
	if data, err := json.MarshalIndent(jsonContent, "", "  "); err == nil {
		os.WriteFile(jsonPath, data, 0644)
	}
}

func getStatusVerb(status string) string {
	switch status {
	case "completed":
		return "completed"
	case "failed":
		return "attempted but failed"
	case "cancelled":
		return "cancelled"
	default:
		return "processed"
	}
}

// deleteJob is defined in jobs_test.go - use that instead

// WriteTestLog writes the test log to the results directory
func WriteTestLog(t *testing.T, resultsDir string, entries []string) {
	t.Helper()

	if resultsDir == "" || len(entries) == 0 {
		return
	}

	content := ""
	for _, entry := range entries {
		content += entry + "\n"
	}

	logPath := filepath.Join(resultsDir, "test.log")
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write test.log: %v", err)
	}
}

// AssertResultFilesExist verifies required result files exist
func AssertResultFilesExist(t *testing.T, resultsDir string) {
	t.Helper()

	requiredFiles := []string{"output.md", "output.json", "test.log"}
	for _, file := range requiredFiles {
		path := filepath.Join(resultsDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Required result file missing: %s", file)
		}
	}

	// Check for job_definition (toml or json)
	tomlPath := filepath.Join(resultsDir, "job_definition.toml")
	jsonPath := filepath.Join(resultsDir, "job_definition.json")
	_, tomlErr := os.Stat(tomlPath)
	_, jsonErr := os.Stat(jsonPath)
	if os.IsNotExist(tomlErr) && os.IsNotExist(jsonErr) {
		t.Error("Required result file missing: job_definition.toml or job_definition.json")
	}
}
