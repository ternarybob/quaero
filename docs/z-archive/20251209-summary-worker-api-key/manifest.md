# Fix: Summary Worker Missing API Key in Codebase Assessment

- Slug: summary-worker-api-key | Type: fix | Date: 2025-12-09
- Request: "Screenshot shows Codebase Assessment Pipeline failing with 'worker init failed: api_key is required for summary'"
- Prior: none

## User Intent

Fix the Codebase Assessment Pipeline so the summary steps (generate_index, generate_summary, generate_map) have access to the Gemini API key. The error occurs because these steps don't have `api_key` configured but the SummaryWorker requires it.

## Success Criteria

- [ ] Codebase Assessment Pipeline can complete all 9 steps without API key errors
- [ ] Summary steps have api_key configured using the variable reference pattern `{google_gemini_api_key}`
- [ ] Existing .env configuration is sufficient (no new keys needed)
