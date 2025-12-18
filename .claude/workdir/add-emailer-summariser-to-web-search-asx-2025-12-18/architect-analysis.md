# Architect Analysis: Add Emailer and Summariser to web-search-asx.toml

## Task Requirements

1. Add a **summariser step** (AI agent) to assess collected web search information and save the summary
2. Add an **emailer step** (final) to send the summary via email
3. Ensure correct step dependencies - NO worker-to-worker communication
4. Each step references saved documents via filters/tags
5. Copy job definition to `deployments/local/job-definitions` and `test/config/job-definitions`
6. Create UI tests that verify each step generates output

---

## Codebase Analysis

### Existing Patterns to EXTEND (NOT Create)

| Need | Existing Code | Pattern Source |
|------|---------------|----------------|
| Multi-step job with depends | `codebase_assess.toml` | Uses `depends = "step_name"` |
| Summary worker type | `codebase_assess.toml:62` | `type = "summary"` with prompt, api_key, filter_tags |
| Email worker type | `docs/architecture/WORKERS.md:460` | `type = "email"` with to, subject, body_from_tag |
| Agent for document generation | `agent-document-generator.toml` | `operation_type = "generate"`, `agent_type = "summary_generator"` |
| Document tagging/filtering | Multiple jobs | `filter_source_type`, `filter_tags` |
| UI test pattern | `job_definition_nearby_restaurants_keywords_test.go` | `RunJobDefinitionTest()` |

### Email Worker Configuration (from WORKERS.md)

```toml
[step.notify]
type = "email"
description = "Send job results via email"

[step.notify.config]
to = "team@example.com"
subject = "Daily Report Complete"
body = "The daily report job has completed successfully."
```

Parameters:
- `to` (required): Recipient email address
- `subject` (optional): Email subject
- `body` (optional): Plain text body
- `body_from_document` (optional): Document ID as body
- `body_from_tag` (optional): Latest document with tag as body

### Summary Worker Configuration (from codebase_assess.toml)

```toml
[step.generate_summary]
type = "summary"
description = "Generate summary document"
depends = "prior_step"
filter_tags = ["source-tag"]
api_key = "{google_gemini_api_key}"
prompt = """Generate a summary..."""
```

---

## Proposed Job Definition Structure

```
┌─────────────────────────┐
│ step.search_asx_gnp     │ (existing)
│ type = "web_search"     │
│ tags: ["web-search", "asx", "stocks", "gnp"]
└──────────┬──────────────┘
           │ saves documents with tags
           ▼
┌─────────────────────────┐
│ step.summarize_results  │ (NEW)
│ type = "summary"        │
│ depends = "search_asx_gnp"
│ filter_tags = ["gnp"]   │
│ Reads: docs from search │
│ Saves: summary document │
│ with tag: "asx-gnp-summary"
└──────────┬──────────────┘
           │ saves summary doc with tag
           ▼
┌─────────────────────────┐
│ step.email_summary      │ (NEW)
│ type = "email"          │
│ depends = "summarize_results"
│ body_from_tag = "asx-gnp-summary"
│ Reads: summary doc by tag
│ Sends: email to recipient
└─────────────────────────┘
```

**Key Design Decisions:**
1. **NO worker-to-worker communication** - Each step reads from storage via tags
2. **Summary step** saves document with unique tag (`asx-gnp-summary`)
3. **Email step** reads summary via `body_from_tag` (per WORKERS.md)
4. Dependencies ensure execution order: search → summarize → email

---

## Anti-Creation Verification

| Item | Action | Justification |
|------|--------|---------------|
| Job definition | MODIFY | Extending existing `web-search-asx.toml` |
| Summary step | EXTEND pattern | Uses existing `type = "summary"` from codebase_assess.toml |
| Email step | EXTEND pattern | Uses existing `type = "email"` from WORKERS.md |
| UI test | EXTEND pattern | Follows `job_definition_nearby_restaurants_keywords_test.go` |

**No new workers or types need to be created** - all patterns exist in codebase.

---

## Implementation Plan

### Step 1: Modify web-search-asx.toml

Add two new steps to the existing job definition:

```toml
# Step 2: Summarize the search results using AI
[step.summarize_results]
type = "summary"
description = "Generate executive summary of ASX:GNP research"
depends = "search_asx_gnp"
on_error = "fail"
filter_tags = ["gnp"]
api_key = "{google_gemini_api_key}"
prompt = """Analyze the collected information about ASX:GNP and create a comprehensive summary:

1. COMPANY OVERVIEW: Brief description of ASX:GNP
2. RECENT NEWS: Key recent developments and announcements
3. FINANCIAL HIGHLIGHTS: Any financial data mentioned
4. MARKET POSITION: Industry context and competitive position
5. KEY RISKS/OPPORTUNITIES: Notable points for investors

Format as a professional investment research brief."""
output_tags = ["asx-gnp-summary"]

# Step 3: Email the summary to recipients
[step.email_summary]
type = "email"
description = "Send ASX:GNP summary via email"
depends = "summarize_results"
on_error = "fail"
to = "{email_recipient}"
subject = "ASX:GNP Analysis Complete"
body_from_tag = "asx-gnp-summary"
```

### Step 2: Copy to Deployment Folders

- `deployments/local/job-definitions/web-search-asx.toml`
- `test/config/job-definitions/web-search-asx.toml`

### Step 3: Create UI Test

Create `test/ui/job_definition_web_search_asx_test.go`:

```go
func TestJobDefinitionWebSearchASX(t *testing.T) {
    // Follow pattern from job_definition_nearby_restaurants_keywords_test.go
    // Verify:
    // 1. Job completes successfully
    // 2. Expected 3 steps: search_asx_gnp, summarize_results, email_summary
    // 3. Each step produces expected output (documents, email)
}
```

---

## Build Verification

Build command: `./scripts/build.sh`

**No code changes required** - only TOML configuration and test files.

---

## Summary

This implementation:
1. **EXTENDS** existing job definition patterns
2. **USES** existing worker types (`summary`, `email`)
3. **NO** new code creation required
4. **FOLLOWS** document-based communication via tags (no worker-to-worker)
5. **ALIGNS** with documented worker interfaces in WORKERS.md

**Files to Modify:** 3 (web-search-asx.toml in 3 locations)
**Files to Create:** 1 (UI test file)
