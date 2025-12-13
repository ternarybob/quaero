# Step 3: Actual Root Cause - Missing Google Places API Key

**Status**: ✅ COMPLETE
**Root Cause**: Job fails immediately due to missing Google Places API key

---

## Executive Summary

The test timeout is **NOT** caused by jobs running slowly or status updates failing. The job **fails immediately** during execution because the required Google Places API key is missing from the test environment.

**Test Log Evidence**:
```
ERR > Job definition execution failed
      function=github.com/ternarybob/quaero/internal/handlers.(*JobDefinitionHandler).ExecuteJobDefinitionHandler.func1
      job_def_id=places-nearby-restaurants
      error=step search_nearby_restaurants failed: failed to search places: API error: REQUEST_DENIED -
            You must use an API key to authenticate each request to Google Maps Platform APIs
```

---

## Investigation Process

### Step 1: Code Analysis ✅
**Result**: All code is working correctly
- UI displays status correctly (title case "Completed")
- Job execution pipeline is sound
- Status persistence works
- JobProcessor is running
- Test environment starts service correctly

### Step 2: Test Execution 🔍
**Command**: `go test -v ./test/ui/queue_test.go -run TestQueue`

**Result**: Test timed out after 120 seconds
```
setup.go:1270: ✓ Job triggered: Nearby Restaurants (Wheelers Hill)
setup.go:1270: ✓ Job found in queue
setup.go:1270: Waiting for job completion...
queue_test.go:279: Places job monitoring failed: job Nearby Restaurants (Wheelers Hill) did not complete:
                   waiting for function failed: timeout
```

### Step 3: Log Analysis ✅
**File**: `test/results/ui/queue-20251125-080458/TestQueue/service.log`

**Key Findings**:

#### 1. JobProcessor IS Running
```
08:05:00 INF > Job processor goroutine started
      function=github.com/ternarybob/quaero/internal/jobs/worker.(*JobProcessor).processJobs
```

#### 2. Workers ARE Registered
```
08:04:59 INF > Job worker registered job_type=crawler_url
08:04:59 INF > Job worker registered job_type=github_action_log
08:05:00 INF > Job worker registered job_type=agent
```

#### 3. API Key Validation Failed
```
08:05:01 WRN > API key validation failed for job definition
      api_key_name={google_places_api_key}
      error=API key '{google_places_api_key}' not found in environment, KV store, or config
      job_def_id=places-nearby-restaurants
```

#### 4. Job Execution Started
```
08:05:04 INF > Executing job definition
      job_def_id=places-nearby-restaurants
      job_def_type=places
      job_name=Nearby Restaurants (Wheelers Hill)
      step_count=1
```

#### 5. Places API Call Failed
```
08:05:04 ERR > Step execution failed
      step_name=search_nearby_restaurants
      action=places_search
      error=failed to search places: API error: REQUEST_DENIED -
            You must use an API key to authenticate each request to Google Maps Platform APIs
```

#### 6. Job Execution Failed
```
08:05:04 ERR > Job definition execution failed
      job_def_id=places-nearby-restaurants
      error=step search_nearby_restaurants failed: failed to search places: API error: REQUEST_DENIED
```

#### 7. NO Queue Workers Were Invoked
**Expected**: `"Processing job from queue"` log messages
**Actual**: NO such messages found

**Why**: The job failed during the `JobDefinitionOrchestrator` execution (before enqueueing any child jobs to the crawler queue).

---

## Root Cause Explanation

### Architecture Understanding

The "Nearby Restaurants (Wheelers Hill)" job is a **Places job definition**, NOT a crawler job. It uses a different execution path:

#### Places Job Flow:
1. User triggers job via UI
2. `JobDefinitionHandler.ExecuteJobDefinitionHandler()` is called
3. `JobDefinitionOrchestrator.Execute()` creates parent job
4. Step `search_nearby_restaurants` is executed via `PlacesAction`
5. `PlacesService.SearchPlaces()` calls Google Places API
6. **API call fails with REQUEST_DENIED (no API key)**
7. Job status is updated to "failed"
8. No child jobs are enqueued to the crawler queue

#### What DIDN'T Happen:
- ❌ No child crawler jobs were created
- ❌ No messages were sent to the queue
- ❌ JobProcessor workers were never invoked
- ❌ Job did not timeout due to slow execution
- ❌ Status updates are working correctly (job status is "failed")

---

## Why The Test Times Out

The test at `test/ui/queue_test.go:169` searches for:
```go
statusSelector := fmt.Sprintf(`//span[text()="Completed" or text()="Completed with Errors"]`, jobName)
```

But the job status is `"failed"`, NOT `"completed"`. The test polls for 120 seconds waiting for "Completed" status, but the job has already failed with status "failed".

**The test XPath selector doesn't match "Failed" status, only "Completed" or "Completed with Errors".**

---

## Solution Options

### Option 1: Add Google Places API Key to Test Environment (RECOMMENDED)
**Action**: Configure the test environment with a valid Google Places API key.

**Files to modify**:
- `config/.env.test` - Add `GOOGLE_PLACES_API_KEY=<your-key>`
- OR `config/test-config.toml` - Add API key to config

**Pros**:
- Tests the actual production flow
- Validates API integration
- Tests real-world scenarios

**Cons**:
- Requires Google Cloud Platform account
- API calls cost money (though minimal)
- Tests depend on external service availability

### Option 2: Mock the Places API (ALTERNATIVE)
**Action**: Create a mock Places service for tests that returns fake data.

**Implementation**:
- Add test-specific Places service that doesn't require API key
- Returns hardcoded restaurant data for "Wheelers Hill"
- Only used during testing

**Pros**:
- No external dependencies
- Fast execution
- No API costs
- Reliable (no network issues)

**Cons**:
- Doesn't test actual API integration
- Requires additional mock infrastructure
- May mask real integration issues

### Option 3: Skip Places Job Test (NOT RECOMMENDED)
**Action**: Modify test to use a different job type that doesn't require external APIs.

**Pros**:
- Quick fix
- No API key needed

**Cons**:
- Doesn't test Places functionality
- Reduces test coverage
- Doesn't solve the underlying issue

---

## Recommended Fix

**Priority 1: Add API Key to Test Environment**

1. Create or update `config/.env.test`:
   ```bash
   GOOGLE_PLACES_API_KEY=AIzaSy...your-key-here
   ```

2. Ensure test loads environment variables:
   ```go
   env, err := common.SetupTestEnvironment(t.Name())
   ```
   (This already calls `loadEnvFile()` which loads `.env.test`)

3. Run test again:
   ```bash
   go test -v ./test/ui/queue_test.go -run TestQueue
   ```

4. Verify in service.log:
   - No "API key validation failed" warnings
   - "Step execution failed" should NOT appear
   - "Processing job from queue" messages should appear
   - Job should complete successfully

**Expected Result**: Job executes successfully, creates child crawler jobs, workers process them, and status updates to "completed".

---

## Alternative: Update Test to Handle Failed Jobs

If API key is not available, modify the test to accept "Failed" status as valid completion:

**File**: `test/ui/queue_test.go:169`

**Change**:
```go
// BEFORE:
statusSelector := fmt.Sprintf(`//span[text()="Completed" or text()="Completed with Errors"]`, jobName)

// AFTER:
statusSelector := fmt.Sprintf(`//span[text()="Completed" or text()="Completed with Errors" or text()="Failed"]`, jobName)
```

**Pros**:
- Test passes without API key
- Tests job execution flow and status updates

**Cons**:
- Doesn't test successful job completion
- Masks configuration issues
- Reduces test effectiveness

---

## Summary

✅ **Actual Root Cause Identified**: Missing Google Places API key

✅ **All Code Working Correctly**:
- JobProcessor ✅
- Workers registered ✅
- Job execution flow ✅
- Status updates ✅
- UI display ✅

❌ **Test Configuration Issue**: Missing required API key in test environment

🔧 **Recommended Fix**: Add `GOOGLE_PLACES_API_KEY` to `config/.env.test`

---

## Files Examined

1. ✅ `test/ui/queue_test.go` - Test implementation
2. ✅ `test/results/ui/queue-20251125-080458/TestQueue/service.log` - Service execution logs
3. ✅ `internal/jobs/worker/job_processor.go` - Job processing (confirmed running)
4. ✅ `internal/handlers/job_definition_handler.go` - Job definition execution
5. ✅ `internal/jobs/job_definition_orchestrator.go` - Orchestration logic
6. ✅ `internal/services/places/service.go` - Places API integration

---

## Next Steps

1. Add Google Places API key to test environment
2. Run test again to verify fix
3. Document API key requirement in test README
4. Consider adding API key validation check before running tests
5. Update test documentation with setup instructions

---

## Lessons Learned

1. **Always check logs first** - Would have saved investigation time
2. **Test environment configuration matters** - Missing config can cause timeouts
3. **Different job types use different execution paths** - Places jobs don't use crawler queue
4. **Failed != Timeout** - Job failed immediately, but test timed out waiting for wrong status
5. **API dependencies need test configuration** - External APIs require keys/mocks in tests
