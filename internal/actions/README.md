# Actions Domain

This package contains the **Actions Domain** - user-defined workflows that describe WHAT work should be done.

## Architecture

The Actions Domain is part of the Manager/Worker/Monitor architecture. See `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` for the complete architecture documentation.

### Key Concept

**Actions** define user-facing, editable workflows. They describe:
- What type of job to run (crawler, agent, places_search, etc.)
- What steps to execute (crawl, transform, reindex)
- How to configure each step (URLs, filters, agent prompts)

### Separation of Concerns

| Domain | Location | Purpose |
|--------|----------|---------|
| **Actions** | `internal/actions/` | User-defined workflows (WHAT to do) |
| **Queue** | `internal/queue/` | Job execution and runtime state (HOW to do it) |

## Package Structure

```
internal/actions/
└── definitions/
    └── orchestrator.go    # Routes job steps to appropriate StepManagers
```

### definitions/orchestrator.go

The `JobDefinitionOrchestrator` routes job definition steps to the appropriate StepManager implementations:

- `"crawl"` action → `CrawlerManager` in `internal/queue/managers/`
- `"agent"` action → `AgentManager` in `internal/queue/managers/`
- `"transform"` action → `TransformManager` in `internal/queue/managers/`
- `"reindex"` action → `ReindexManager` in `internal/queue/managers/`
- `"places_search"` action → `PlacesSearchManager` in `internal/queue/managers/`
- `"database_maintenance"` action → `DatabaseMaintenanceManager` in `internal/queue/managers/`

## Data Types

### JobDefinition

The `JobDefinition` model (`internal/models/job_definition.go`) represents a user-defined workflow:

```go
type JobDefinition struct {
    ID          string            // Unique identifier
    Name        string            // Human-readable name
    Type        JobDefinitionType // crawler, ai, places, custom
    Description string            // User description
    Schedule    string            // Cron expression (optional)
    Steps       []JobStep         // Workflow steps to execute
    Enabled     bool              // Whether scheduled execution is enabled
    AuthID      string            // Authentication credentials
}
```

### Important: Job Definition Type vs Queue Job Type

- **Job Definition Type** (`JobDefinitionType`): Uses `"ai"` for AI/agent workflows
- **Queue Job Type** (`QueueJob.Type`): Uses `"agent"` for the actual queue job type

This distinction exists because:
1. Job definitions are user-facing and `"ai"` is more intuitive
2. Queue jobs are internal and `"agent"` is more technically accurate (they are AI-powered agents)

## Usage

The orchestrator is typically invoked when a user triggers a job definition:

```go
// In a handler or service
orchestrator := definitions.NewJobDefinitionOrchestrator(managers, jobManager, logger)
jobID, err := orchestrator.Execute(ctx, jobDefinition)
```

## Related Documentation

- [Manager/Worker Architecture](../../docs/architecture/MANAGER_WORKER_ARCHITECTURE.md)
- [Queue Domain README](../queue/README.md)
- [Agent Framework](../../AGENTS.md)
