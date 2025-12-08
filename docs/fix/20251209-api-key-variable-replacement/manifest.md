# Fix: API Key Variable Replacement Intermittent Failure
- Slug: api-key-variable-replacement | Type: fix | Date: 2025-12-09
- Request: "API key validation fails intermittently for {google_gemini_api_key}. The key should be configured in bin/.env and variable replacement should cover. Any configuration request in the code should actively implement the variable replacement strategy using {xxx} pattern. There should be only 1 toml/config service used throughout the codebase - no custom config access/process/function in code files."
- Prior: none

## User Intent
1. Fix the intermittent API key validation failure where `{google_gemini_api_key}` is not resolved
2. Ensure ALL configuration access uses the centralized variable replacement strategy (checking for `{xxx}` pattern and replacing from .env)
3. Consolidate config access to a single service - no duplicate/custom config parsing scattered in code files
4. The config service should be injected/passed into services that need it

## Success Criteria
- [ ] API key variables like `{google_gemini_api_key}` are consistently resolved from .env
- [ ] All code paths that read config values use the centralized variable replacement
- [ ] No custom/duplicate config parsing exists outside the config service
- [ ] ValidateAPIKeys function properly resolves variable placeholders before validation
