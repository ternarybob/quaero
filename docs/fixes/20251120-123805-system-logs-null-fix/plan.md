# Plan: Fix System Logs Endpoint Returning Null

## Problem
- System logs endpoint `/api/system/logs/content` returns null
- UI shows "No logs found matching criteria"
- Log files exist in `bin/logs/` but handler looks in `logs/`
- Path mismatch between logger configuration (execDir/logs) and handler (hardcoded "logs")

## Root Cause
- `system_logs_handler.go:102` uses hardcoded `filepath.Join("logs", filename)`
- Logger creates logs in `{execDir}/logs` which is `bin/logs/` when running from bin
- Handler should use the same logs directory as the logger, or get it from the arbor service

## Steps

1. **Investigate arbor service log directory configuration**
   - Skill: @none
   - Files: Look at arbor service to see if it provides log directory path
   - User decision: no

2. **Fix handler to use correct log directory path**
   - Skill: @go-coder
   - Files: `internal/handlers/system_logs_handler.go`
   - User decision: no
   - Action: Either use arbor service method or get log directory from config/logger

3. **Verify fix works with local test**
   - Skill: @go-coder
   - Files: `internal/handlers/system_logs_handler.go`
   - User decision: no
   - Action: Build and test endpoint manually

## Success Criteria
- Endpoint returns log entries (not null)
- UI displays logs correctly
- Code uses clean approach without custom path handling
- Uses arbor service if it provides directory path, otherwise matches logger's path logic
