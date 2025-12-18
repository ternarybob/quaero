# Summary: Add Emailer and Summariser to web-search-asx.toml

## Task Completed Successfully

### Changes Made

#### 1. Job Definition Updates (3 files)

**bin/job-definitions/web-search-asx.toml** - Added 2 new steps:

```toml
# Step 2: Summarize the search results using AI
[step.summarize_results]
type = "summary"
description = "Generate executive summary of ASX:GNP research"
depends = "search_asx_gnp"
on_error = "fail"
filter_tags = ["gnp"]
api_key = "{google_gemini_api_key}"
prompt = """..."""
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

**deployments/local/job-definitions/web-search-asx.toml** - Created (copy)
**test/config/job-definitions/web-search-asx.toml** - Updated

#### 2. UI Test Created

**test/ui/job_definition_web_search_asx_test.go**
- Tests job completion
- Verifies 3 expected steps exist
- Verifies each step completes successfully
- Verifies each step generates output

### Architecture Compliance

| Requirement | Implementation |
|-------------|----------------|
| NO worker-to-worker communication | Steps use document tags for data passing |
| Step dependencies correct | search_asx_gnp -> summarize_results -> email_summary |
| Each step references saved docs | filter_tags, output_tags, body_from_tag |
| EXTEND > MODIFY > CREATE | Extended existing summary and email patterns |

### Job Execution Flow

```
┌─────────────────────────┐
│ step.search_asx_gnp     │
│ type = "web_search"     │
│ Saves: docs tagged "gnp"│
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│ step.summarize_results  │
│ type = "summary"        │
│ Reads: filter_tags=gnp  │
│ Saves: asx-gnp-summary  │
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│ step.email_summary      │
│ type = "email"          │
│ Reads: body_from_tag    │
│ Sends: email            │
└─────────────────────────┘
```

### Build Verification

- Main build: PASS
- MCP server: PASS
- UI test compilation: PASS

### Prerequisites for Running

1. **Google Gemini API Key**: Set in Settings > Key-Value Store as `google_gemini_api_key`
2. **Email Recipient**: Set in Settings > Key-Value Store as `email_recipient`
3. **SMTP Config**: Configure in `deployments/local/email.toml` or via Settings > Email

### Testing Instructions

1. Start Quaero:
   ```bash
   cd bin && ./quaero.exe
   ```

2. Configure API keys in Settings > Key-Value Store:
   - `google_gemini_api_key` - Your Gemini API key
   - `email_recipient` - Destination email address

3. Configure SMTP in Settings > Email

4. Run the job from Jobs page: "Web Search: ASX:GNP Company Info"

5. Run UI test:
   ```bash
   go test -v ./test/ui -run TestJobDefinitionWebSearchASX -timeout 10m
   ```
