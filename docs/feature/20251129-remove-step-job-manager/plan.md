# Plan: Rename Step/StepWorker to Worker Terminology

## Classification
- Type: feature (refactoring)
- Workdir: ./docs/feature/20251129-remove-step-job-manager/

## Analysis

### Current State
The codebase uses "step" terminology in several places:
1. `StepType` (models/step_type.go) - Enum for worker types (agent, crawler, places_search, etc.)
2. `StepWorker` (interfaces/job_interfaces.go) - Interface for workers that create jobs from job definitions
3. `StepManager` (interfaces/job_interfaces.go) - Unused interface, superseded by StepWorker
4. `JobStep` (models/job_definition.go) - Configuration for a step within a job definition
5. Various methods: `ValidateStep`, `GetType`, etc.

### Problem
The "step" terminology is confusing because:
- Workers are the primary concept (they do work)
- A "step" in a job definition is really just "worker configuration"
- `StepType` is really identifying which worker to use
- The relationship between `JobWorker` and `StepWorker` is unclear

### Proposed Renaming
1. `StepType` → `WorkerType` - Clearly identifies worker types
2. `StepWorker` → `DefinitionWorker` - Workers that handle job definition execution
3. `StepManager` → Remove entirely (unused)
4. `JobStep` → Keep as-is (it's step configuration, not a worker)
5. Update all methods and references

### Risks
- Breaking changes to internal APIs (no external API impact)
- Large number of files to update
- Must ensure build and tests pass after changes

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Rename StepType to WorkerType in models | none | yes:architectural-change | medium | opus |
| 2 | Rename StepWorker to DefinitionWorker interface | 1 | yes:architectural-change | medium | opus |
| 3 | Remove StepManager interface | 2 | no | low | sonnet |
| 4 | Update all worker implementations | 2 | no | medium | sonnet |
| 5 | Update manager.go worker registry | 2,4 | no | medium | sonnet |
| 6 | Update app.go worker registration | 4,5 | no | low | sonnet |
| 7 | Update README and documentation | 1-6 | no | low | sonnet |

## Order
Sequential: [1] → [2] → [3] → Concurrent: [4,5] → [6] → [7] → Review
