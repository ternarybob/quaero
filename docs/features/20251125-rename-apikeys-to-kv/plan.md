# Plan: Rename API Keys to Key Values (kv)

## Dependency Analysis
The "API Keys" section is mis-termed - it actually stores generic key/value pairs used for configuration values like API tokens, secrets, etc. The backend already uses `/api/kv` endpoints, but the UI still labels this as "API Keys".

Dependencies:
- UI changes depend on backend partial mapping being updated
- Test changes depend on UI changes being complete
- All changes can be done in parallel groups after foundation

## Critical Path Flags
None - this is a low-complexity rename operation with no security implications.

## Execution Groups

### Group 1 (Sequential - Foundation)
These must run first, in order:

1. **Rename partial HTML file**
   - Skill: @frontend-developer
   - Files: pages/partials/settings-auth-apikeys.html -> pages/partials/settings-kv.html
   - Complexity: low
   - Critical: no
   - Depends on: none
   - User decision: no

### Group 2 (Parallel - Independent Work)
These can run simultaneously after Group 1:

2a. **Update settings.html navigation**
    - Skill: @frontend-developer
    - Files: pages/settings.html
    - Complexity: low
    - Critical: no
    - Depends on: Step 1
    - Sandbox: worker-a

2b. **Update settings-components.js**
    - Skill: @javascript-pro
    - Files: pages/static/settings-components.js
    - Complexity: medium
    - Critical: no
    - Depends on: Step 1
    - Sandbox: worker-b

2c. **Update page_handler.go partial mapping**
    - Skill: @golang-pro
    - Files: internal/handlers/page_handler.go
    - Complexity: low
    - Critical: no
    - Depends on: Step 1
    - Sandbox: worker-c

### Group 3 (Parallel - Secondary Updates)
These can run after Group 2:

3a. **Update routes.go auth redirect**
    - Skill: @golang-pro
    - Files: internal/server/routes.go
    - Complexity: low
    - Critical: no
    - Depends on: 2c
    - Sandbox: worker-d

3b. **Update settings test expectations**
    - Skill: @test-writer
    - Files: test/ui/settings_test.go
    - Complexity: low
    - Critical: no
    - Depends on: 2a, 2b
    - Sandbox: worker-e

3c. **Update config comments**
    - Skill: @none
    - Files: deployments/local/quaero.toml
    - Complexity: low
    - Critical: no
    - Depends on: none
    - Sandbox: worker-f

### Group 4 (Sequential - Validation)
Runs after Group 3 completes:

4. **Build and test verification**
   - Skill: @test-automator
   - Files: N/A
   - Complexity: low
   - Critical: no
   - Depends on: 3a, 3b, 3c
   - User decision: no

## Parallel Execution Map
```
[Step 1: Rename file] ──┬──> [Step 2a: settings.html] ────┐
                        ├──> [Step 2b: JS components] ─────┼──> [Step 3b: tests]
                        └──> [Step 2c: page_handler.go] ──┼──> [Step 3a: routes.go]
                                                          │
[Step 3c: config comments] ───────────────────────────────┘
                                                          │
                                                          └──> [Step 4: Build & Test]
```

## Naming Conventions

| Old Name | New Name |
|----------|----------|
| auth-apikeys | kv |
| API Keys | Key Values |
| authApiKeys (JS component) | kv |
| apiKeys (JS array) | keyValues |
| settings-auth-apikeys.html | settings-kv.html |

## Success Criteria
- All "API Keys" text in UI replaced with "Key Values"
- Navigation section ID changed from `auth-apikeys` to `kv`
- Partial file renamed from `settings-auth-apikeys.html` to `settings-kv.html`
- Backend routes updated for new partial name
- Tests pass with new naming
- Application builds without errors
