# Fix: Chrome Extension Auth Capture Failure

- Slug: chrome-extension-auth-fix | Type: fix | Date: 2025-12-03
- Request: "The chrome extension is failing. Failed to update authentication error='credentials ID is required'. Trace through the chrome extension path, implement API tests, execute and iterate to pass."
- Prior: none

## User Intent

Fix the Chrome extension authentication capture flow that is failing with "credentials ID is required" error. The POST /api/auth endpoint is returning 500 error because the credentials ID is missing from the request. Need to:
1. Trace the authentication flow from Chrome extension to API
2. Identify where credentials_id should come from
3. Implement API tests for the auth capture endpoint
4. Fix the issue and ensure tests pass

## Success Criteria

- [ ] Identify root cause of "credentials ID is required" error
- [ ] API tests for POST /api/auth endpoint implemented in test/api/
- [ ] Chrome extension auth capture flow works without error
- [ ] All tests pass
