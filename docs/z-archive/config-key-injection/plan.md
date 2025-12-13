# Plan: Dynamic Key Injection in Config with Pub/Sub Events

## Problem Statement

Currently, the Google Places API key is hardcoded in `quaero.toml`:
```toml
[places_api]
api_key = "AIzaSyCwXVa0E5aCDmCg9FlhPeX8ct83E9EADFg"
```

Keys should be referenced using variables (e.g., `{google-places-key}`) and dynamically injected at runtime from the KV storage. When users modify keys through the UI, the config service needs to refresh its cache via pub/sub events.

## Current State Analysis

1. **Key Storage**: Keys are loaded from `bin/keys/example-keys.toml` into KV storage
2. **Config Loading**: `LoadFromFiles()` already performs `{key-name}` replacement using `ReplaceInStruct()` (config.go:331-346)
3. **Config Handler**: Returns raw config struct without re-applying key injection (config_handler.go:40)
4. **Event System**: Robust pub/sub system exists (event_service.go) but no config-specific events

## Goals

1. Config should use `{key-name}` placeholders instead of hardcoded values
2. Config handler should inject keys dynamically when serving config to requestors
3. Add pub/sub event for key updates to invalidate config cache
4. Ensure backward compatibility (nil kvStorage still works)

## Steps

1. **Add EventKeyUpdated to event system**
   - Skill: @code-architect
   - Files: `internal/interfaces/event_service.go`
   - User decision: no
   - Define new EventKeyUpdated constant with payload structure for key change notifications

2. **Update config.toml to use key placeholders**
   - Skill: @none
   - Files: `bin/quaero.toml`
   - User decision: no
   - Replace hardcoded `api_key = "AIza..."` with `api_key = "{google-places-key}"`

3. **Create ConfigService with caching and event subscription**
   - Skill: @code-architect
   - Files: `internal/services/config/config_service.go` (new)
   - User decision: no
   - Create service that:
     - Caches config with injected keys
     - Subscribes to EventKeyUpdated
     - Invalidates cache and re-injects keys on key updates
     - Provides GetConfig() method for handlers

4. **Implement key injection in ConfigService.GetConfig()**
   - Skill: @go-coder
   - Files: `internal/services/config/config_service.go`
   - User decision: no
   - Deep clone config struct, apply ReplaceInStruct with latest KV values

5. **Update ConfigHandler to use ConfigService**
   - Skill: @go-coder
   - Files: `internal/handlers/config_handler.go`
   - User decision: no
   - Inject ConfigService dependency, use GetConfig() instead of raw config

6. **Add ConfigService to dependency injection**
   - Skill: @go-coder
   - Files: `cmd/quaero/main.go`
   - User decision: no
   - Wire ConfigService into main.go with event service and KV storage

7. **Publish EventKeyUpdated when keys change**
   - Skill: @go-coder
   - Files: `internal/handlers/auth_config_handler.go` (or wherever KV updates happen)
   - User decision: no
   - Publish event after successful KV.Set() operations

8. **Add tests for ConfigService**
   - Skill: @test-writer
   - Files: `internal/services/config/config_service_test.go` (new)
   - User decision: no
   - Test caching, event handling, key injection, nil kvStorage handling

9. **Update API tests to verify dynamic injection**
   - Skill: @test-writer
   - Files: `test/api/config_api_test.go`
   - User decision: no
   - Verify config endpoint returns injected keys after KV updates

## Success Criteria

- Config files use `{key-name}` placeholders instead of hardcoded secrets
- ConfigHandler returns dynamically injected values from KV storage
- Key updates trigger EventKeyUpdated and invalidate config cache
- Tests verify end-to-end key injection and cache invalidation
- Backward compatible: works with nil kvStorage (skips replacement gracefully)
- No breaking changes to existing API contracts

## Technical Notes

- Use `common.ReplaceInStruct()` for key injection (already exists)
- Event payload should include: `key_name`, `old_value`, `new_value`, `timestamp`
- Cache invalidation should be thread-safe (use sync.RWMutex)
- Deep clone config before injection to avoid mutating shared state
