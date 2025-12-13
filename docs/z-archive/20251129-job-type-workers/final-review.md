# Review: Job Type Workers Architecture

**Triggers:** architectural-change
**Files:** 18 (6 new files, 12+ modified files)
**Date:** 2025-11-29
**Reviewer:** Architecture Review (Opus)

---

## Security

**Critical:** None

**Warnings:**
1. Placeholder resolution logs resolved values at DEBUG level - could expose API keys
   - Recommendation: Consider masking sensitive values in debug logs
2. No additional authorization checks in StepWorkers (acceptable for single-tenant)

---

## Architecture

**Breaking:** No breaking changes to API contracts

Full backward compatibility maintained:
- `action` field still parsed via `mapActionToStepType()`
- Legacy StepManager interface preserved
- Dual routing: GenericStepManager (type-based) with fallback to action-based

**Migration:** Clear upgrade path
1. Existing TOML files with `action` continue to work
2. New files should use `type` field
3. Deprecation planned for v4.0

**Design Strengths:**
1. Clean Adapter Pattern - StepWorkers wrap existing managers
2. Type-Safe Routing - StepType enum prevents typos
3. Single Responsibility - Clear separation of concerns
4. Open/Closed Principle - New types require no existing code changes
5. Consistent Interface - All workers implement same 4-method interface

**Design Weaknesses:**
1. Dual Registration Required - Both legacy and new registration needed during transition
2. Adapter Creates New Manager Instance - Should reuse existing instances
3. StepWorker Interface Lacks Execute() - Documentation claims unified interface
4. Incomplete StepType Coverage - 9 types defined, only 6 adapters exist

---

## Code Quality

**Issues:**
1. Missing Unit Tests - No test files in `internal/queue/workers/`
2. CrawlerStepWorker.Validate() returns nil unconditionally
3. AllStepTypes() helper not used for registration validation

**Strengths:**
- Error handling consistent with custom ErrNoWorker type
- Logging appropriate at correct levels
- Thread safety maintained (no concurrent writes after init)

---

## Recommendations

### Required Before Merge
1. Add unit tests for StepWorker adapters
2. Fix duplicate manager instantiation in app.go

### Suggested Improvements
1. Add missing adapters (transform, reindex, database_maintenance)
2. Add startup validation for registered workers
3. Mask sensitive values in placeholder resolution logs
4. Update interface documentation to match implementation

---

## Verdict

**Status:** ⚠️ APPROVED_WITH_NOTES

The architectural refactor is well-designed following clean architecture principles. The Adapter Pattern enables incremental migration while maintaining backward compatibility. Type-safe routing eliminates string-matching errors.

**Technical Debt Created:**
- 3 StepWorker adapters still needed
- Legacy action-based routing to remove in v4.0
- StepWorker interface may need Execute() for full unification

**Risk Assessment:** Low - All changes are additive, backward compatible, and well-isolated.
