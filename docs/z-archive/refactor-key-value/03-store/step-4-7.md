# Steps 4-7: Update Services to Pass KV Storage

**Skill:** @go-coder
**Steps covered:**
- Step 4: Update LLM Service
- Step 5: Update Agent Service
- Step 6: Update Places Service
- Step 7: Update Job Definition Handler

**Pattern:** All services follow the same pattern - add nil as first parameter to ResolveAPIKey() call

---

## Iteration 1

### Agent 2 - Implementation

Updating all service call sites to pass nil for kvStorage parameter. This is intentional because:
1. Services don't need direct KV storage access
2. API keys are resolved at service initialization time (not runtime)
3. At initialization, API keys should come from config (which was already replaced in Phase 2)
4. This maintains clean separation of concerns

**Files being updated:**
1. `internal/services/llm/gemini_service.go:107`
2. `internal/services/agents/service.go:61`
3. `internal/services/places/service.go:37`
4. `internal/handlers/job_definition_handler.go:586`

**Commands run:**
```bash
go build -o bin/quaero.exe ./cmd/quaero
```

**Changes:**
1. Updated service call sites to pass nil for kvStorage:
   - `internal/services/llm/gemini_service.go:108`
   - `internal/services/agents/service.go:62`
   - `internal/services/places/service.go:38`
   - `internal/handlers/job_definition_handler.go:586`

2. Updated manager structs and constructors to accept kvStorage:
   - `internal/jobs/manager/agent_manager.go:17,30,65`
   - `internal/jobs/manager/places_search_manager.go:17,30,76`

3. Updated manager initialization in app.go:
   - `internal/app/app.go:474,480`

**Build result:** ✅ Success - All compilation errors resolved

---

## Iteration 2 - Validation

### Agent 3 - Validator

**Validation steps:**
1. ✅ All service call sites updated correctly
2. ✅ Managers accept KV storage parameter
3. ✅ App.go passes KV storage to managers
4. ✅ Full project compiles successfully
5. ✅ Nil kvStorage used in services (intentional - services use config)
6. ✅ KV storage passed to managers for runtime API key resolution

**Status:** APPROVED - Steps 4-7 complete
