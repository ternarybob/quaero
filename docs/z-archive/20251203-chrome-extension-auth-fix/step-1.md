# Step 1: Fix storeAtlassianAuth to generate ID

Model: opus | Status: done

## Done

- Added deterministic ID generation: `fmt.Sprintf("auth:%s:%s", s.serviceName, siteDomain)`
- Set `credentials.ID = credentialsID` before calling StoreCredentials

## Files Changed

- `internal/services/auth/service.go` - Added ID generation in storeAtlassianAuth()

## Build Check

Build: done | Tests: pending
