# Task 1: Fix storeAtlassianAuth to generate ID

Depends: - | Critical: yes:authentication | Model: opus

## Addresses User Intent

Fixes the root cause of "credentials ID is required" error by generating a deterministic ID for credentials.

## Do

1. In `internal/services/auth/service.go` function `storeAtlassianAuth`:
   - Generate ID using `fmt.Sprintf("auth:%s:%s", s.serviceName, siteDomain)` for deterministic, unique ID
   - This allows upsert behavior (same site domain updates existing credentials)

2. Set the ID field before calling `authStorage.StoreCredentials()`

## Accept

- [ ] credentials.ID is set before StoreCredentials is called
- [ ] ID format is deterministic: `auth:{serviceName}:{siteDomain}`
- [ ] Build compiles without errors
