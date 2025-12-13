# Jobs Domain

This package contains the **Jobs Domain** - user-defined workflow definitions and management.

## Architecture

The Jobs Domain is part of the Manager/Worker/Monitor architecture. See `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` for the complete architecture documentation.

### Key Concept

**Jobs** define user-facing, editable workflows. This domain handles:
- CRUD operations for job definitions (create, read, update, delete)
- Validation of job definitions
- TOML parsing and conversion
- Job definition storage interactions

### Separation of Concerns

| Domain | Location | Purpose |
|--------|----------|---------|
| **Jobs** | `internal/jobs/` | Job definition management (WHAT to do) |
| **Queue** | `internal/queue/` | Job execution and runtime state (HOW to do it) |

**Note:** The `Orchestrator` (execution logic) is in the Queue Domain at `internal/queue/orchestrator.go`.

## Package Structure

```
internal/jobs/
├── README.md    # This file
└── service.go   # Job definition business logic service
```

## Service

The `Service` struct in `service.go` provides business logic for job definitions:

### TOML Parsing

```go
// Parse TOML content into JobDefinitionFile
jobFile, err := jobs.ParseTOML(tomlContent)

// Convert to JobDefinition model
jobDef := jobFile.ToJobDefinition()
```

### TOML Export

```go
// Convert JobDefinition to TOML for download
tomlData, err := service.ConvertToTOML(jobDef)
```

### Validation

```go
// Validate step actions are registered
err := service.ValidateStepActions(jobType, steps)

// Validate runtime dependencies (services, API keys)
service.ValidateRuntimeDependencies(jobDef)

// Validate API keys exist in storage
service.ValidateAPIKeys(jobDef)
```

## Data Types

Job definition models are in `internal/models/` (shared models package):

### JobDefinition

The `JobDefinition` model (`internal/models/job_definition.go`) represents a user-defined workflow:

```go
type JobDefinition struct {
    ID          string            // Unique identifier
    Name        string            // Human-readable name
    Type        JobDefinitionType // crawler, agent, places, custom
    Description string            // User description
    Schedule    string            // Cron expression (optional)
    Steps       []JobStep         // Workflow steps to execute
    Enabled     bool              // Whether scheduled execution is enabled
    AuthID      string            // Authentication credentials
}
```

### Job Definition Types

- **`crawler`**: Web crawling workflows
- **`agent`**: AI-powered document processing with Google Gemini
- **`places`**: Google Places API searches
- **`custom`**: Custom user-defined workflows

## Related Documentation

- [Manager/Worker Architecture](../../docs/architecture/MANAGER_WORKER_ARCHITECTURE.md)
- [Queue Domain README](../queue/README.md)
