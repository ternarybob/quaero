# Step 1: Fix filter_source_type bug
- Task: task-1.md | Group: 1 | Model: sonnet

## Actions
1. Identified bug: `filter_source_type = "crawler"` but places search creates `source_type = "places"`
2. Changed `filter_source_type` from "crawler" to "places" in job definition

## Files
- `test/config/job-definitions/nearby-resturants-keywords.toml` - line 35: changed filter_source_type

## Decisions
- Direct fix: The filter must match the source_type of documents created by the previous step

## Verify
Compile: ✅ | Tests: ⚙️

## Status: ✅ COMPLETE
