---
task: "Add comprehensive diagnostic logging for cookie injection flow"
complexity: medium
steps: 4
---

# Plan

## Step 1: Add pre-injection domain diagnostics to enhanced_crawler_executor_auth.go
**Why:** Need to log target URL domain parsing and cookie domain comparison before injection to diagnose domain mismatch issues
**Depends:** none
**Validates:** code_compiles, follows_conventions
**Files:** internal/jobs/processor/enhanced_crawler_executor_auth.go
**Risk:** low
**User decision required:** no

## Step 2: Add post-injection verification and network domain enablement to enhanced_crawler_executor_auth.go
**Why:** Need to enable network domain and verify cookies after injection using network.GetCookies() to confirm persistence
**Depends:** 1
**Validates:** code_compiles, follows_conventions
**Files:** internal/jobs/processor/enhanced_crawler_executor_auth.go
**Risk:** low
**User decision required:** no

## Step 3: Add cookie monitoring before/after navigation in enhanced_crawler_executor.go
**Why:** Need to log what cookies are actually sent with navigation requests to diagnose authentication failures
**Depends:** 2
**Validates:** code_compiles, follows_conventions
**Files:** internal/jobs/processor/enhanced_crawler_executor.go
**Risk:** low
**User decision required:** no

## Step 4: Build and verify compilation
**Why:** Ensure all changes compile successfully and follow project conventions
**Depends:** 1, 2, 3
**Validates:** code_compiles, tests_must_pass, follows_conventions
**Files:** All modified files
**Risk:** low
**User decision required:** no

## User Decision Points
None - this is purely additive diagnostic logging with no functional behavior changes.

## Constraints
- All changes are additive logging only (no functional changes)
- Use 🔐 emoji prefix for all auth-related logs (existing convention)
- Use ChromeDP network API correctly: network.Enable() → network.SetCookie() → network.GetCookies()
- Log detailed cookie attributes (name, domain, path, secure, httpOnly, sameSite)
- Compare injected vs verified cookies to detect mismatches
- Max file size remains under 500 lines (auth file currently 300 lines, executor 808 lines)

## Success Criteria
- Code compiles successfully
- All new logging follows existing conventions
- ChromeDP network API used correctly
- Cookie verification after injection implemented
- Request-time cookie monitoring in renderPageWithChromeDp implemented
- Domain comparison logic logs mismatches
- No functional behavior changes
