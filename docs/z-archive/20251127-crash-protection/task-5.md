# Task 5: Validate Crash Protection and Run Tests

## Metadata
- **ID:** 5
- **Group:** 3
- **Mode:** sequential
- **Skill:** @test-writer
- **Complexity:** medium
- **Model:** claude-sonnet-4-5-20250929
- **Critical:** no
- **Depends:** 2, 3, 4
- **Blocks:** none

## Paths
```yaml
sandbox: /tmp/3agents/task-5/
source: C:/development/quaero/
output: C:/development/quaero/docs/fixes/20251127-crash-protection/
```

## Files to Modify
- None - validation only

## Requirements
Validate that crash protection works and run tests:

1. **Build and verify compilation**:
   ```bash
   cd C:/development/quaero && go build ./...
   ```

2. **Run unit tests for new code**:
   ```bash
   go test ./internal/common/... -v
   ```

3. **Run the existing crash test**:
   ```bash
   cd C:/development/quaero/test/ui && go test -run TestNewsCrawlerCrash -v -timeout 15m
   ```

4. **Manual crash test**:
   - Add temporary panic to verify crash file is created
   - Remove temporary panic after verification

5. **Verify crash file creation**:
   - Check bin/logs/ for crash files after intentional panic
   - Verify file contains all goroutine stacks
   - Verify file contains system info

6. **Document test results**:
   - Record test output
   - Note any failures or issues
   - Provide recommendations

## Acceptance Criteria
- [ ] All code compiles successfully
- [ ] Unit tests pass
- [ ] Crash file is created on intentional panic
- [ ] Crash file contains useful diagnostic info
- [ ] TestNewsCrawlerCrash runs (may take 10+ minutes)
- [ ] Test results documented

## Context
The existing TestNewsCrawlerCrash test monitors the News Crawler job for 10 minutes. If the service crashes, the test will fail with a connection error. If the service stays up, the test passes.

## Dependencies Input
From Tasks 2, 3, 4:
- crash.go module
- Enhanced panic recovery in job_processor.go
- SafeGo wrapper utility
- Event publisher wrappers

## Output for Dependents
- Validation results
- Any remaining issues
- Test coverage report
