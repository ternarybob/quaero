# Step 1: Implementation

Iteration: 1 | Status: complete

## Changes Made

| File | Action | Description |
|------|--------|-------------|
| `test/ui/job_definition_general_test.go` | modified | Fixed log order assertion to correctly verify latest logs using per-job line numbers |
| `test/ui/job_definition_general_test.go` | modified | Changed screenshot interval from 30s to 15s for better coverage of fast jobs |

## Key Fixes

### 1. Log Order Assertion Fix

**Problem**: The original assertion checked if `lastLine >= totalCount - 100`, assuming line numbers were global (1-3642). But line numbers are **per-job**:
- Orchestration logs: lines 1-27
- Worker job 1: lines 1-1200
- Worker job 2: lines 1-1200
- Worker job 3: lines 1-1200

**Solution**: Changed assertion to verify latest logs by checking:
1. `earlierCount` is high (3500+ means we're showing latest, not earliest)
2. Worker logs have high line numbers (>= 1000, indicating late execution)

```go
// New assertion:
assert.GreaterOrEqual(t, earlierCount, expectedEarlierMin,
    "Should show latest logs (earlierCount should be high)")

// Check for high worker line numbers
hasHighWorkerLines := false
for _, ln := range lineNumbers {
    if ln >= maxWorkerLineExpected { // e.g., >= 1000
        hasHighWorkerLines = true
        break
    }
}
```

### 2. Screenshot Frequency Fix

**Problem**: Jobs completing in ~20 seconds missed 30-second screenshots.

**Solution**: Changed screenshot interval from 30s to 15s:
```go
// Before:
if time.Since(lastScreenshot) >= 30*time.Second {

// After:
if time.Since(lastScreenshot) >= 15*time.Second {
```

## Build & Test

Build: Pass
Tests: Pass (all 3)

| Test | Result | Duration | Notes |
|------|--------|----------|-------|
| TestJobDefinitionHighVolumeLogsWebSocketRefresh | PASS | 33.70s | 1243 logs via WebSocket |
| TestJobDefinitionFastGenerator | PASS | 33.69s | 312 logs (expected 265+) |
| TestJobDefinitionHighVolumeGenerator | PASS | 47.98s | 3643 logs, screenshot at 17s |

## Screenshots Generated

For TestJobDefinitionHighVolumeGenerator:
- `01_jobs_page_high_volume_generator_test.png`
- `02_confirmation_modal_high_volume_generator_test.png`
- `03_status_running.png`
- `04_monitor_progress_15s.png` ← New monitoring screenshot at 15s
- `05_status_completed.png`
- `06_step_expanded_final.png`
- `07_high_volume_generator_completed.png`
- `job_config.json`

## Architecture Compliance (self-check)

- [x] **QUEUE_UI.md - Log Line Numbering**: Logs display per-job line numbers correctly
- [x] **QUEUE_LOGGING.md - Log Retrieval**: API returns newest logs with high earlier count
- [x] **Test Requirements**: Screenshots taken during job execution without page refresh
