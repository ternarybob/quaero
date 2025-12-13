# Complete: Job Type Workers Architecture Refactor

## Classification
- Type: feature
- Location: ./docs/feature/20251129-job-type-workers/

## Overview

This refactor implements a type-based worker routing architecture for job execution. The job type has been moved from the parent JobDefinition level into individual steps, enabling support for multiple step types within a single job. A new GenericStepManager routes execution based on `step.Type` to type-defined workers that implement a unified `StepWorker` interface. Full backward compatibility is maintained through action-to-type mapping during TOML parsing.

## Stats
- Tasks: 9
- Files: 30+ (6 new, 24+ modified)
- Models: Planning=opus, Workers=6×sonnet/3×opus, Review=opus

## Tasks

### Task 1: Update JobStep Model (opus)
Created `StepType` enum with 9 step types. Added `Type` and `Description` fields to JobStep struct. Updated validation to require Type field.

### Task 2: Create Generic StepManager (opus)
Implemented `GenericStepManager` with worker registry and type-based routing. Added `StepWorker` interface. Updated Orchestrator with dual routing (new type-based + legacy fallback).

### Task 3: Refactor Workers (sonnet)
Created 6 StepWorker adapters wrapping existing managers: agent, crawler, places_search, web_search, github_repo, github_actions. Registered all workers in app.go.

### Task 4: Update TOML Parsing (sonnet)
Added `type` field parsing with backward compatibility for `action` field. Implemented `mapActionToStepType()` for 13 action values. Added comprehensive validation.

### Task 5: Update Test Configs (sonnet)
Updated 8 test job definitions to use new `type` field format with descriptions.

### Task 6: Execute Tests (sonnet)
All refactor-related tests pass. Fixed 6 mock definitions in jobs_test.go. Identified pre-existing/environmental test issues (unrelated to refactor).

### Task 7: Update Example Configs (sonnet)
Updated 14 job definition files across deployments/local and bin directories.

### Task 8: Update Documentation (sonnet)
Renamed and rewrote architecture documentation to v3.0. Added comprehensive TOML schema, step types reference, and migration guide.

### Task 9: Remove Redundant Code (sonnet)
Conservative cleanup - updated documentation example. Preserved backward compatibility layer.

## Review: ⚠️ APPROVED_WITH_NOTES

**Verdict:** Architecture well-designed following clean principles. Adapter Pattern enables incremental migration. Type-safe routing eliminates string-matching errors.

**Required Actions:**
1. Add unit tests for StepWorker adapters
2. Fix duplicate manager instantiation in app.go

**Technical Debt:**
- 3 StepWorker adapters needed (transform, reindex, database_maintenance)
- Legacy action-based routing to remove in v4.0

## Verify

| Check | Result |
|-------|--------|
| go build | ✅ PASS |
| go test (refactor) | ✅ PASS |
| Backward compat | ✅ PASS |

## Key Deliverables

### New Files
- `internal/models/step_type.go` - StepType enum
- `internal/queue/generic_manager.go` - GenericStepManager
- `internal/queue/workers/agent_step_worker.go`
- `internal/queue/workers/crawler_step_worker.go`
- `internal/queue/workers/places_step_worker.go`
- `internal/queue/workers/web_search_step_worker.go`
- `internal/queue/workers/github_repo_step_worker.go`
- `internal/queue/workers/github_actions_step_worker.go`

### New TOML Format
```toml
[step.{name}]
type = "agent"  # Required: routes to StepWorker
description = "What this step does"
on_error = "fail"
# type-specific config...
```

### Step Types
| Type | Purpose | Child Jobs |
|------|---------|------------|
| agent | AI document processing | Yes |
| crawler | Web crawling | Yes |
| places_search | Google Places API | No |
| web_search | Gemini web search | No |
| github_repo | Repository fetching | Yes |
| github_actions | Actions log fetching | Yes |
| transform | Data transformation | No |
| reindex | Database reindexing | No |
| database_maintenance | DB maintenance | Yes |

## Migration

Existing job definitions continue to work:
- `action` field auto-mapped to `type`
- Warning logged for deprecated usage
- Full deprecation planned for v4.0

New job definitions should use:
- `type` field (required)
- `description` field (recommended)
