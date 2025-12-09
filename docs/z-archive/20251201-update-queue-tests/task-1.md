# Task 1: Add Test for Child Job Execution Order
Depends: - | Critical: no | Model: sonnet

## Context
The job definition `nearby-restaurants-keywords.toml` defines:
- `search_nearby_restaurants` (no dependencies)
- `extract_keywords` (depends = "search_nearby_restaurants")

The test needs to verify that:
1. Child jobs are created in the correct dependency order
2. Step 1 (places_search) runs before Step 2 (agent)

## Do
1. Add a new sub-test to verify child job execution order
2. After job completes, check that the child jobs were created in order
3. Verify that the places_search child completed before agent child started
4. Use Alpine.js data to extract child job metadata including created_at timestamps

## Accept
- [ ] Test verifies child jobs have correct execution order
- [ ] Test checks that dependent steps ran after their dependencies
- [ ] Test should FAIL if execution order is wrong
