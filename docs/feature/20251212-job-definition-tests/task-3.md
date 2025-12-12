# Task 3: Create nearby-restaurants-places job definition test

Workdir: ./docs/feature/20251212-job-definition-tests/ | Depends: 1 | Critical: no
Model: sonnet | Skill: go

## Context

This task is part of: Creating job definition test infrastructure for Quaero
Prior tasks completed: Task 1 - Framework helper methods added

## User Intent Addressed

Create test for nearby-restaurants-places job definition (Google Places API) that runs end-to-end with monitoring and screenshots.

## Input State

Files that exist before this task:
- `test/ui/job_framework_test.go` - UITestContext with RunJobDefinitionTest method
- `test/config/job-definitions/nearby-restaurants-places.toml` - Places API job definition

## Output State

Files after this task completes:
- `test/ui/job_definition_nearby_restaurants_places_test.go` - Complete test file for places API job

## Skill Patterns to Apply

### From go/SKILL.md:
- **DO:** Skip tests when required environment not available
- **DO:** Wrap errors with context using %w
- **DO:** Put integration tests in test/ui/
- **DON'T:** Hard-code API keys

## Implementation Steps

1. Create `test/ui/job_definition_nearby_restaurants_places_test.go`
2. Define TestJobDefinitionNearbyRestaurantsPlaces function
3. Configure JobDefinitionTestConfig for nearby-restaurants-places:
   - JobName: "Nearby Restaurants (Wheelers Hill)"
   - JobDefinitionPath: "../config/job-definitions/nearby-restaurants-places.toml"
   - Timeout: 5 minutes (places search is fast)
   - RequiredEnvVars: ["QUAERO_GOOGLE_PLACES_API_KEY"]
   - AllowFailure: false
4. Call RunJobDefinitionTest with config
5. Log success

## Code Specifications

```go
package ui

import (
    "testing"
    "time"
)

// TestJobDefinitionNearbyRestaurantsPlaces tests the Places API job definition end-to-end
func TestJobDefinitionNearbyRestaurantsPlaces(t *testing.T) {
    utc := NewUITestContext(t, 10*time.Minute)
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
```

## Accept Criteria

- [ ] File `test/ui/job_definition_nearby_restaurants_places_test.go` exists
- [ ] Test function TestJobDefinitionNearbyRestaurantsPlaces defined
- [ ] Uses JobDefinitionTestConfig with correct values
- [ ] Requires QUAERO_GOOGLE_PLACES_API_KEY env var
- [ ] Timeout set to 5 minutes
- [ ] Code compiles: `go build ./test/ui/...`

## Handoff

After completion, next task(s): 6 (verification)
