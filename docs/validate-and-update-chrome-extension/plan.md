---
task: "Validate and update Chrome extension to accurately reflect its capture-auth-and-crawl functionality"
complexity: medium
steps: 4
---

# Plan

## Analysis

The Chrome extension's metadata and documentation are outdated and don't accurately reflect the complete "Capture & Crawl" workflow that was implemented. The current state shows:

**Current Inaccuracies:**
1. **manifest.json** - Description says "Captures authentication data from any website for Quaero" but doesn't mention the immediate crawl functionality
2. **manifest.json** - Title says "Quaero Auth Capture" which is incomplete (should indicate crawl capability)
3. **popup.html** - Has outdated UI with only "Capture Authentication" button and instructions that reference separate web UI for crawling
4. **README.md** - Describes only authentication capture, not the integrated capture-and-crawl workflow
5. **README.md** - Has outdated port references (8080 vs actual 8085)
6. **sidepanel.html** - Instructions say "Check the web UI (localhost:8085) to monitor progress" but don't explain the immediate crawl behavior

**Actual Behavior (from code analysis):**
- Button labeled "Capture & Crawl" in sidepanel.html (line 203)
- Function `captureAndCrawl()` in sidepanel.js (line 123-218):
  1. Captures authentication cookies from current site
  2. Sends auth to `/api/auth` endpoint
  3. Immediately creates and executes crawler job via `/api/job-definitions/quick-crawl`
  4. Shows success message with job ID
- Handler `CreateAndExecuteQuickCrawlHandler` creates job and starts execution immediately

## Step 1: Update manifest.json metadata
**Why:** The manifest metadata should accurately describe what the extension does - both capturing authentication AND initiating crawls
**Depends:** none
**Validates:** manifest_accuracy
**Files:** cmd/quaero-chrome-extension/manifest.json
**Risk:** low
**User decision required:** no

**Changes:**
- Update description to: "Capture authentication and instantly crawl any website with Quaero"
- Update name to: "Quaero Web Crawler"
- Keep version as-is (0.1.0)

## Step 2: Sync popup.html with sidepanel.html functionality
**Why:** The popup.html still has old "Capture Authentication" button while sidepanel.html has the correct "Capture & Crawl" button
**Depends:** Step 1
**Validates:** ui_consistency
**Files:** cmd/quaero-chrome-extension/popup.html, cmd/quaero-chrome-extension/popup.js
**Risk:** low
**User decision required:** no

**Changes:**
- Update button text from "Capture Authentication" to "Capture & Crawl"
- Update instructions to reflect the integrated workflow
- Ensure popup.js has the same captureAndCrawl functionality as sidepanel.js

## Step 3: Update extension README documentation
**Why:** The README needs to accurately describe the complete capture-and-crawl workflow and correct port numbers
**Depends:** Step 2
**Validates:** documentation_accuracy
**Files:** cmd/quaero-chrome-extension/README.md
**Risk:** low
**User decision required:** no

**Changes:**
- Update title to reflect crawler functionality
- Fix port references (8080 → 8085)
- Update usage instructions to explain the integrated workflow
- Add section about the quick crawl functionality
- Update API endpoints section to include `/api/job-definitions/quick-crawl`
- Clarify that clicking the button both captures auth AND starts crawling

## Step 4: Update inline comments and documentation consistency
**Why:** Code comments and any remaining references should consistently describe the capture-and-crawl behavior
**Depends:** Step 3
**Validates:** code_documentation
**Files:** cmd/quaero-chrome-extension/background.js, cmd/quaero-chrome-extension/sidepanel.html
**Risk:** low
**User decision required:** no

**Changes:**
- Update background.js header comment to mention crawling capability
- Ensure sidepanel.html instructions are fully accurate
- Add comments in popup.js if implementing captureAndCrawl function

## User Decision Points
None - all steps are straightforward updates to text/metadata to match existing functionality

## Constraints
- Extension must work in Chrome/Edge
- No breaking changes to existing functionality
- Descriptions must be accurate and user-friendly
- Keep existing version number (0.1.0)
- Maintain compatibility with existing server endpoints

## Success Criteria
- All extension metadata accurately describes the capture-and-crawl behavior
- Extension documentation matches actual implementation
- No functional code changes (only documentation/metadata updates)
- User understands from descriptions that clicking the button will both capture auth AND start crawling immediately
- Port numbers are consistent and correct (8085)