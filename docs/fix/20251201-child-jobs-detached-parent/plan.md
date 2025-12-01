# Plan: Child Jobs Statistics/Display Issues
Type: fix | Workdir: ./docs/fix/20251201-child-jobs-detached-parent/

## Analysis from Screenshots
1. **Statistics mismatch**: Job Statistics shows 1001 completed (parent + all children), but Queue shows 1 parent as "Running" - this is BY DESIGN (stats count all jobs, UI shows only parents)
2. **WebSocket failures**: "Failed to send..." messages appear in Service Logs - these ARE correctly logged at WARN level in code
3. **Job works correctly**: The GitHub Repository Collector job ran successfully, creating 1000 child jobs

## Findings
- The statistics/UI behavior is intentional - statistics count ALL jobs, UI displays parent jobs only
- WebSocket failures are logged correctly with `.Warn()` level in websocket.go
- The job configuration in bin/job-definitions/github-repo-collector.toml is correct

## No Code Changes Required
The observed behavior is working as designed:
1. Statistics panel = total of all jobs (parents + children)
2. Job Queue UI = parent jobs only (children shown when expanded)
3. WebSocket warnings are already at WARN level in code

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Document the expected behavior and verify no actual bug exists | - | no | sonnet |

## Order
[1]
