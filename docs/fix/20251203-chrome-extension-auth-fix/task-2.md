# Task 2: Implement API tests for POST /api/auth

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Implements API tests for the auth capture endpoint as requested by user.

## Do

1. Create `test/api/auth_test.go` with tests:
   - `TestAuthCapture_Success`: Valid auth data returns 200
   - `TestAuthCapture_InvalidMethod`: GET returns 405
   - `TestAuthCapture_InvalidBody`: Bad JSON returns 400
   - `TestAuthCapture_PersistsCredentials`: After capture, credentials are stored and retrievable

2. Use existing test infrastructure from test/common/setup.go

## Accept

- [ ] auth_test.go created in test/api/
- [ ] Tests compile
- [ ] Tests cover success and error cases
