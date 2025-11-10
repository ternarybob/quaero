# Plan: Update Chrome Extension for Generic Auth and Quick Crawl

---
task: "Update Chrome extension to support generic auth (any site) and add 'Crawl Current Page' button with configurable defaults"
complexity: medium
steps: 6
---

## Overview
Transform Chrome extension from Atlassian-specific to generic auth capture, add quick crawl functionality with configurable defaults.

## Step 1: Add Crawler Defaults to Config
**Why:** Need global defaults (depth:2, pages:10) for quick crawl feature
**Depends:** none
**Validates:** code_compiles, follows_conventions
**Files:**
- internal/interfaces/config.go
- internal/common/config.go
- quaero.toml (example config)
**Risk:** low
**User decision required:** no

Add crawler configuration section to config structure with sensible defaults for quick crawl operations.

## Step 2: Remove Atlassian/Jira Restrictions from Extension
**Why:** Make auth capture work for any website
**Depends:** none
**Validates:** code_compiles, follows_conventions
**Files:**
- cmd/quaero-chrome-extension/manifest.json (permissions)
- cmd/quaero-chrome-extension/background.js (auth capture logic)
- cmd/quaero-chrome-extension/sidepanel.html (UI text)
- cmd/quaero-chrome-extension/sidepanel.js (UI logic)
**Risk:** low
**User decision required:** no

Remove hardcoded Atlassian domain restrictions, update UI text to be generic.

## Step 3: Create Job Definition Creation/Execute API Endpoint
**Why:** Need backend endpoint to create job definition and execute it immediately
**Depends:** 1
**Validates:** code_compiles, tests_must_pass, follows_conventions
**Files:**
- internal/handlers/crawler_handler.go (new endpoint handler)
- internal/server/routes.go (register route)
- internal/services/crawler/service.go (job creation + execution logic)
**Risk:** medium
**User decision required:** no

Add POST endpoint that creates a job definition file and triggers immediate execution.

## Step 4: Add "Crawl Current Page" Button to Extension UI
**Why:** User-facing feature for quick page crawling
**Depends:** 3
**Validates:** follows_conventions
**Files:**
- cmd/quaero-chrome-extension/sidepanel.html (button UI)
- cmd/quaero-chrome-extension/sidepanel.js (button handler)
**Risk:** low
**User decision required:** no

Add button that captures current URL, cookies, and triggers crawl via new API endpoint.

## Step 5: Integration Testing
**Why:** Verify end-to-end workflow works correctly
**Depends:** 4
**Validates:** code_compiles, tests_must_pass
**Files:**
- test/api/crawler_quick_test.go (API endpoint tests)
**Risk:** low
**User decision required:** no

Test job creation/execution endpoint and verify crawler defaults are applied.

## Step 6: Build and Deploy Extension
**Why:** Deploy updated extension for manual testing
**Depends:** 5
**Validates:** use_build_script, no_root_binaries
**Files:**
- Build via scripts/build.ps1
**Risk:** low
**User decision required:** no

Build project to deploy updated extension to bin/ directory.

## User Decision Points
None - straightforward implementation following existing patterns.

## Constraints
- Must maintain backward compatibility with existing job definitions
- Quick crawl should use config defaults but not modify user's saved job definitions
- Extension must work with any website (no domain restrictions)
- Auth capture should be generic (cookies + tokens, not platform-specific)

## Success Criteria
- Extension captures auth from any website
- "Crawl Current Page" button creates temporary job and executes immediately
- Crawler defaults (depth:2, pages:10) configurable in quaero.toml
- All tests pass
- Extension deploys successfully to bin/
- No breaking changes to existing functionality
