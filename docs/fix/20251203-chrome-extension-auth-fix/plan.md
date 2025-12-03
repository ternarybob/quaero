# Plan: Chrome Extension Auth Fix

Type: fix | Workdir: ./docs/fix/20251203-chrome-extension-auth-fix/

## User Intent (from manifest)

Fix the Chrome extension authentication capture flow that is failing with "credentials ID is required" error. The POST /api/auth endpoint is returning 500 error because the credentials ID is missing from the request. Need to:
1. Trace the authentication flow from Chrome extension to API
2. Identify where credentials_id should come from
3. Implement API tests for the auth capture endpoint
4. Fix the issue and ensure tests pass

## Root Cause Analysis

The error occurs in `internal/storage/badger/auth_storage.go:30`:
```go
if credentials.ID == "" {
    return fmt.Errorf("credentials ID is required")
}
```

The call chain:
1. Chrome extension POSTs to `/api/auth` with auth data
2. `CaptureAuthHandler` calls `authService.UpdateAuth(&authData)`
3. `UpdateAuth` calls `storeAtlassianAuth(authData)`
4. `storeAtlassianAuth` creates `credentials` struct WITHOUT setting ID field
5. `authStorage.StoreCredentials()` fails because `ID == ""`

**Fix**: Generate ID in `storeAtlassianAuth()` using siteDomain or UUID

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Fix storeAtlassianAuth to generate ID | - | yes:authentication | opus |
| 2 | Implement API tests for POST /api/auth | 1 | no | sonnet |
| 3 | Run tests and iterate to pass | 2 | no | sonnet |

## Order

[1] → [2] → [3]
