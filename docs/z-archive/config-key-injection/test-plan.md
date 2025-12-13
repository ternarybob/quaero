# Test Plan: Dynamic Key Injection in Config

## Coverage Needed

### Unit Tests (ConfigService)
- Step 3: ConfigService caching - Test: `TestConfigService_Caching` - Exists: **NO**
- Step 3: ConfigService event handling - Test: `TestConfigService_EventInvalidation` - Exists: **NO**
- Step 4: Key injection - Test: `TestConfigService_KeyInjection` - Exists: **NO**
- Step 3: Nil kvStorage handling - Test: `TestConfigService_NilKVStorage` - Exists: **NO**
- Step 3: Thread-safe concurrent access - Test: `TestConfigService_ConcurrentAccess` - Exists: **NO**

### Integration Tests (API)
- Step 5: Config endpoint returns injected keys - Test: `TestConfigEndpoint_DynamicKeyInjection` - Exists: **NO**
- Step 7: Key update triggers cache refresh - Test: `TestConfigEndpoint_KeyUpdateRefresh` - Exists: **NO**

### Existing Tests
- ✅ `TestConfigEndpoint` - Basic config endpoint validation (already exists)

## Tests to Create

### Unit Tests
1. **`internal/services/config/config_service_test.go`**
   - TestConfigService_Caching - Verify cache hit/miss behavior
   - TestConfigService_EventInvalidation - Verify EventKeyUpdated invalidates cache
   - TestConfigService_KeyInjection - Verify {key-name} replacement works
   - TestConfigService_NilKVStorage - Verify graceful degradation
   - TestConfigService_ConcurrentAccess - Verify thread-safety

### Integration Tests
2. **`test/api/config_dynamic_injection_test.go`** (new file)
   - TestConfigEndpoint_DynamicKeyInjection - Verify /api/config returns injected keys
   - TestConfigEndpoint_KeyUpdateRefresh - Verify key update triggers config refresh

## Test Execution Order

1. Create unit tests for ConfigService
2. Create integration tests for API
3. Run unit tests: `cd internal/services/config && go test -v`
4. Run integration tests: `cd test/api && go test -v -run TestConfigEndpoint_Dynamic`
5. Generate test report
