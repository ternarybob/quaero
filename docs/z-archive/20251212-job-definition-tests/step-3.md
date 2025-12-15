# Step 3: Create nearby-restaurants-places job definition test

## Implementation Summary

Successfully created `test/ui/job_definition_nearby_restaurants_places_test.go` with the test for the Google Places API job definition.

## Files Created

- `C:\development\quaero\test\ui\job_definition_nearby_restaurants_places_test.go` - Test file for nearby-restaurants-places job

## Implementation Details

### Test Function: TestJobDefinitionNearbyRestaurantsPlaces

The test implements the standard job definition test pattern:

1. **Test Context Setup** (10 minute timeout)
   - Creates UITestContext with NewUITestContext
   - Registers cleanup with defer

2. **Configuration**
   - JobName: "Nearby Restaurants (Wheelers Hill)"
   - JobDefinitionPath: "../config/job-definitions/nearby-restaurants-places.toml"
   - Timeout: 5 minutes (Places API searches are fast)
   - RequiredEnvVars: ["QUAERO_GOOGLE_PLACES_API_KEY"]
   - AllowFailure: false

3. **Execution**
   - Calls utc.RunJobDefinitionTest(config)
   - Automatically handles:
     - Environment variable validation (skips if missing)
     - Job definition copy to results directory
     - Navigation to Jobs page
     - Job triggering via UI
     - Job monitoring until completion
     - Screenshot capture throughout
     - Final state verification

4. **Success Logging**
   - Logs completion message on success

## Test Behavior

### Environment Variable Check
- Requires QUAERO_GOOGLE_PLACES_API_KEY to be set
- Automatically skips test if missing (won't fail)

### Expected Flow
1. Copies nearby-restaurants-places.toml to test results
2. Navigates to /jobs page
3. Takes screenshot of job definition
4. Clicks "Run" button for "Nearby Restaurants (Wheelers Hill)"
5. Confirms in modal dialog
6. Monitors job status on /queue page
7. Takes periodic screenshots during execution
8. Waits for job to reach completed/failed/cancelled status
9. Takes final screenshot after refresh
10. Fails if job status is "failed" (AllowFailure=false)

### Timeout Configuration
- Overall test timeout: 10 minutes
- Job execution timeout: 5 minutes
- Places API searches are typically fast (<1 minute)

## Verification

Compilation verified:
```bash
go build ./test/ui/...
```

Result: Success (no errors)

## Job Definition Details

The test targets the job definition at:
`test/config/job-definitions/nearby-restaurants-places.toml`

Key attributes:
- Job ID: places-nearby-restaurants
- Job Name: "Nearby Restaurants (Wheelers Hill)"
- Job Type: places
- Search Type: nearby_search
- Location: Wheelers Hill (-37.9167, 145.1833)
- Radius: 2000m
- Filter: restaurant type, min rating 3.5
- Max Results: 20

## Accept Criteria Status

- [x] File `test/ui/job_definition_nearby_restaurants_places_test.go` exists
- [x] Test function TestJobDefinitionNearbyRestaurantsPlaces defined
- [x] Uses JobDefinitionTestConfig with correct values
- [x] Requires QUAERO_GOOGLE_PLACES_API_KEY env var
- [x] Timeout set to 5 minutes
- [x] Code compiles: `go build ./test/ui/...`

## Next Steps

Per task-3.md handoff:
- Next task: 6 (verification)
- This test is ready for integration into CI/CD pipeline
- Can be run manually with: `go test -v ./test/ui -run TestJobDefinitionNearbyRestaurantsPlaces`

## Notes

- Test follows established UITestContext pattern from job_framework_test.go
- Uses RunJobDefinitionTest helper for standardized execution
- Integrates with chromedp for browser automation
- Screenshots saved to test results directory for debugging
- Job definition automatically copied to results for audit trail
