# Plan: Refactor Queue Architecture - Manager/Step/Job Hierarchy

Type: feature | Workdir: ./docs/feature/20251201-refactor-queue-arch/

## Summary

Refactor from flat parent-child to 3-level hierarchy:
- **Manager**: Top-level orchestrator, monitors steps, publishes to UI top panel
- **Step**: Each step becomes a "parent" to its spawned jobs, monitors its children
- **Job**: Work units at single level under their step (jobs can spawn more jobs, but all stay under the step)

## Current vs New Architecture

```
CURRENT:                          NEW:
Parent Job                        Manager (type="manager")
├── Child Job 1                   ├── Step 1 (type="step", parent_id=manager)
├── Child Job 2                   │   ├── Job 1.1 (parent_id=step1)
├── Child Job 3                   │   └── Job 1.2 (parent_id=step1)
└── Child Job N                   └── Step 2 (type="step", parent_id=manager)
                                      ├── Job 2.1 (parent_id=step2)
                                      └── Job 2.2 (parent_id=step2)
```

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Update job models - add JobTypeManager, JobTypeStep, add manager_id field | - | yes:architectural-change | opus |
| 2 | Update Manager.ExecuteJobDefinition - create step jobs, steps monitor their children | 1 | yes:architectural-change | opus |
| 3 | Create StepMonitor - monitors step's children (similar to JobMonitor but per-step) | 1 | yes:architectural-change | opus |
| 4 | Update worker.CreateJobs - return step job ID, children reference step as parent | 2 | no | sonnet |
| 5 | Update event publishing - step publishes to job panel, manager publishes to top panel | 3 | no | sonnet |
| 6 | Update GetJobChildStats - support step-level aggregation | 1 | no | sonnet |
| 7 | Update UI queue.html - display step hierarchy correctly | 5,6 | no | sonnet |
| 8 | Test with github-repo-collector-by-name.toml | 7 | no | sonnet |

## Order

[1] → [2,3] → [4,5,6] → [7] → [8]

## Key Design Decisions

1. **Step as Job**: Steps become actual QueueJob entries with type="step"
2. **Flat under Step**: Jobs spawned by a step OR by other jobs all have parent_id=step_id
3. **Manager monitors Steps**: Manager tracks step completion, not individual jobs
4. **Step monitors Jobs**: Each step monitors its own job children
5. **Event routing**: Jobs publish to Step → Step publishes to UI job panel; Step publishes to Manager → Manager publishes to UI top panel
