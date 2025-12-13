# Plan: Update Queue Tests for Multi-Step Jobs
Type: feature | Workdir: ./docs/feature/20251201-update-queue-tests/

## Overview
Update `TestNearbyRestaurantsKeywordsMultiStep` to add comprehensive tests for:
1. Child jobs execution order (based on dependencies)
2. filter_source_type filtering verification
3. Child job document counts in UI
4. Expand/collapse child jobs functionality

These tests should FAIL on the current codebase since:
- Parent document count is showing 24 instead of 20 (double-counting bug)
- Child expand may not be working properly
- Child document counts may not be populated

## Job Definition Analysis
From `test/config/job-definitions/nearby-restaurants-keywords.toml`:
- Step 1: `search_nearby_restaurants` (type: places_search) - creates 20 documents
- Step 2: `extract_keywords` (type: agent) - depends on step 1, filters by source_type="places"

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Add test for child job execution order | - | no | sonnet |
| 2 | Add test for filter_source_type filtering | 1 | no | sonnet |
| 3 | Add test for child job document counts in UI | 2 | no | sonnet |
| 4 | Add test for expand/collapse children | 3 | no | sonnet |
| 5 | Run tests and verify they fail | 4 | no | sonnet |

## Order
[1] → [2] → [3] → [4] → [5]

## Key Files
- `test/ui/queue_test.go` - Main test file (lines 1305-1433)
- `test/config/job-definitions/nearby-restaurants-keywords.toml` - Job definition
