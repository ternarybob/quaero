# Plan: Code Assessment Performance and Filter Issues

Type: fix | Workdir: ./docs/fix/20251209-code-assessment-perf/

## User Intent (from manifest)
1. Events need buffering - publishing is blocking workers
2. classify_files step is processing ALL files instead of just `category=unknown` ones

## Active Skills
go

## Analysis

### Issue 1: Event Buffering
The event service `Publish` method is already async (fires goroutines per handler). However, WebSocket broadcasts iterate over clients synchronously with mutex locks. For high-throughput scenarios, this could cause contention.

**Fix**: Add a buffered channel for event aggregation to batch events before broadcasting.

### Issue 2: Category Filter Not Working
The `classify_files` step with `filter_category = ["unknown"]` is processing ~906 files instead of just unknown ones.

Investigation findings:
- Filter logic tests pass
- `matchesMetadata` correctly returns false if metadata key doesn't exist
- `matchesMetadata` correctly returns false if value doesn't match

**Root cause hypothesis**: The metadata filter might not be receiving the filter value correctly from TOML config. Need to add logging to debug.

**Alternative hypothesis**: The step completion might be triggered before all child jobs have saved their metadata updates.

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Add debug logging to category filter | - | no | sonnet | go |
| 2 | Add event batching for WebSocket broadcasts | - | no | sonnet | go |
| 3 | Build and verify | 1,2 | no | sonnet | go |

## Order
[1,2] → [3]
