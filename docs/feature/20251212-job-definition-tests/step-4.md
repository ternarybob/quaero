# Step 4: Create nearby-restaurants-keywords job definition test

## Status: COMPLETED

## Summary

Created `test/ui/job_definition_nearby_restaurants_keywords_test.go` implementing a comprehensive end-to-end test for the multi-step job definition that combines Google Places API search with AI-powered keyword extraction.

## Implementation Details

### File Created
- **Path**: `C:\development\quaero\test\ui\job_definition_nearby_restaurants_keywords_test.go`
- **Lines of Code**: 28
- **Test Function**: `TestJobDefinitionNearbyRestaurantsKeywords`

### Configuration Values
```go
JobDefinitionTestConfig{
    JobName:           "Nearby Restaurants + Keywords (Wheelers Hill)",
    JobDefinitionPath: "../config/job-definitions/nearby-restaurants-keywords.toml",
    Timeout:           8 * time.Minute,
    RequiredEnvVars:   []string{
        "QUAERO_GOOGLE_PLACES_API_KEY",
        "QUAERO_AGENT_GOOGLE_API_KEY",
    },
    AllowFailure:      true, // Agent step may hit rate limits
}
```

### Key Features

1. **Multi-Step Job Testing**
   - Tests two-step job definition (Places search + Agent keyword extraction)
   - Validates dependency chain (extract_keywords depends on search_nearby_restaurants)

2. **Environment Variable Validation**
   - Requires `QUAERO_GOOGLE_PLACES_API_KEY` for Places API step
   - Requires `QUAERO_AGENT_GOOGLE_API_KEY` for Agent keyword extraction step
   - Skips test gracefully if either is missing

3. **Extended Timeout Handling**
   - 15-minute test context timeout (entire test duration)
   - 8-minute job execution timeout (Places API + Agent processing)
   - Accommodates longer runtime for multi-step execution

4. **Graceful Failure Handling**
   - `AllowFailure: true` prevents test failure on rate limits
   - Agent step may hit Gemini API rate limits during keyword extraction
   - Test validates job execution pipeline without requiring perfect success

### Job Definition Under Test

The test validates `nearby-restaurants-keywords.toml`:
- **Step 1**: `search_nearby_restaurants` - Google Places Nearby Search
  - Location: Wheelers Hill (-37.9167, 145.1833)
  - Radius: 2000m
  - Filter: restaurants with rating >= 3.5
  - Max results: 20

- **Step 2**: `extract_keywords` - AI Keyword Extraction
  - Depends on: search_nearby_restaurants
  - Agent type: keyword_extractor
  - Operation: scan
  - Filter: places source type
  - Max keywords: 10

### Test Execution Flow

1. **Setup Phase**
   - Create UITestContext with 15-minute timeout
   - Validate required environment variables present
   - Skip test if credentials missing

2. **Preparation Phase**
   - Copy job definition TOML to test results directory
   - Navigate to Jobs page
   - Take screenshot of job definitions

3. **Execution Phase**
   - Trigger job via UI button click
   - Confirm execution in modal dialog
   - Monitor job status in queue

4. **Monitoring Phase**
   - Poll job status every 500ms
   - Log status changes (queued -> running -> completed/failed)
   - Take screenshots every 30 seconds
   - Capture final state screenshot

5. **Validation Phase**
   - Verify job reaches terminal state within timeout
   - Accept both success and failure (due to AllowFailure: true)
   - Log completion status

### Compilation Verification

```bash
$ go build ./test/ui/...
# Success - no errors
```

## Acceptance Criteria - ALL MET

- [x] File `test/ui/job_definition_nearby_restaurants_keywords_test.go` exists
- [x] Test function `TestJobDefinitionNearbyRestaurantsKeywords` defined
- [x] Uses `JobDefinitionTestConfig` with correct values
- [x] Requires both `QUAERO_GOOGLE_PLACES_API_KEY` and `QUAERO_AGENT_GOOGLE_API_KEY`
- [x] `AllowFailure` set to `true` (rate limits possible)
- [x] Timeout set to 8 minutes for job execution
- [x] Code compiles: `go build ./test/ui/...` successful

## Testing Notes

### Running the Test

```bash
# Set required environment variables
export QUAERO_GOOGLE_PLACES_API_KEY="your-places-api-key"
export QUAERO_AGENT_GOOGLE_API_KEY="your-gemini-api-key"

# Run the test
go test -v ./test/ui -run TestJobDefinitionNearbyRestaurantsKeywords
```

### Expected Behavior

1. **With Valid Credentials**:
   - Job triggers successfully
   - Step 1 completes: 20 restaurants fetched from Wheelers Hill area
   - Step 2 executes: AI extracts keywords from restaurant data
   - Test passes even if step 2 hits rate limits (AllowFailure: true)

2. **Without Credentials**:
   - Test skips gracefully with message:
     "Skipping test: missing required environment variables: [...]"

3. **Timeout Scenarios**:
   - If job exceeds 8 minutes, test fails with timeout error
   - If context exceeds 15 minutes, entire test fails

### Rate Limit Considerations

The `AllowFailure: true` flag is critical because:
- Gemini API has rate limits (e.g., 15 requests/minute on free tier)
- Keyword extraction for 20 restaurants may exceed limits
- Test validates pipeline execution, not API quota management
- Production jobs would implement retry logic and backoff

## Files Modified

- **Created**: `test/ui/job_definition_nearby_restaurants_keywords_test.go` (28 lines)

## Dependencies

- Requires: `test/ui/job_framework_test.go` (RunJobDefinitionTest method)
- Requires: `test/config/job-definitions/nearby-restaurants-keywords.toml`
- Requires: Google Places API credentials
- Requires: Google Gemini API credentials

## Next Steps

Task 6 will perform verification testing of all job definition tests created in this feature.

## Implementation Timestamp

2025-12-12T13:30:00Z
