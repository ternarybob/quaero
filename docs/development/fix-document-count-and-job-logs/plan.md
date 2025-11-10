---
task: "Fix document count and job logs issues"
complexity: medium
steps: 5
---

# Plan

## Step 1: Analyze Current Crawler Test Implementation
**Why:** Understand existing test patterns, ChromeDP usage, and verification methods before making changes
**Depends:** none
**Validates:** follows_conventions
**Files:**
- test/ui/crawler_test.go
- test/config/news-crawler.toml
- test/common/setup.go
**Risk:** low

**Actions:**
- Read current crawler_test.go to identify existing patterns
- Verify test environment setup in common/setup.go
- Check news-crawler.toml configuration (max_pages=1, max_depth=1)
- Document current wait/verification patterns used

## Step 2: Add Document Count Validation to Existing Test
**Why:** Ensure crawler respects max_pages=1 configuration and verify exactly 1 document is collected
**Depends:** Step 1
**Validates:** tests_must_pass, follows_conventions
**Files:** test/ui/crawler_test.go
**Risk:** low

**Actions:**
- Modify existing TestNewsCrawlerJobExecution test
- After job completion, verify document count is exactly 1
- Use existing document count check pattern (around line 680)
- Update assertion to fail if count != 1
- Add clear error messages for failures

## Step 3: Add Job Logs Visibility Test
**Why:** Ensure job logs are visible in job details Output tab (currently showing "No logs available")
**Depends:** Step 1
**Validates:** tests_must_pass, follows_conventions
**Files:** test/ui/crawler_test.go
**Risk:** low

**Actions:**
- Add new test function TestCrawlerJobLogsVisibility(t *testing.T)
- Navigate to queue page
- Find and click "News Crawler" job to open details
- Click "Job Details" link/button
- Switch to "Output" tab
- Verify logs are visible and not empty
- Assert no "No logs available" message appears
- Take screenshots at each step

## Step 4: Compile and Test
**Why:** Verify tests compile and can be executed
**Depends:** Steps 2, 3
**Validates:** code_compiles, tests_must_pass
**Files:** test/ui/crawler_test.go
**Risk:** low

**Actions:**
- Run: `go build -o /tmp/test-crawler test/ui/crawler_test.go`
- Fix any compilation errors
- Run: `cd test/ui && go test -v -run TestNewsCrawlerJobExecution`
- Run: `cd test/ui && go test -v -run TestCrawlerJobLogsVisibility`

## Step 5: Validation and Documentation
**Why:** Ensure tests meet requirements and document results
**Depends:** Step 4
**Validates:** tests_must_pass, follows_conventions
**Files:**
- test/ui/crawler_test.go
- docs/fix-document-count-and-job-logs/validation.md
**Risk:** low

**Actions:**
- Verify test fails when document count != 1
- Verify test fails when logs not visible
- Take screenshots showing both failure and success cases
- Document test behavior and edge cases
- Create validation report

## Constraints
- Must use existing test patterns from crawler_test.go
- Tests run in /test/ui directory only
- Use ChromeDP for UI automation
- Follow existing screenshot and logging patterns
- Use test/common/setup.go for environment setup
- Tests must run on port 18085 (test server)
- All screenshots saved to test/results/{suite}-{timestamp}/
- Must handle async UI updates with proper waits

## Success Criteria
- Test fails when document count != 1 (currently shows 34)
- Test fails when logs not visible in Output tab
- Both tests pass when issues are fixed
- Screenshots captured for debugging
- Tests follow existing conventions (ChromeDP patterns, logging, cleanup)
- No regression in existing tests
