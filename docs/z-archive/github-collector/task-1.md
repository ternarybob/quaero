# Task 1: Add New Job/Source Type Constants

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/models/job_model.go` - Add job type constants
- `internal/models/document.go` - Add source type constants

## Requirements

### In job_model.go
Add new job type constants for GitHub operations:
```go
const (
    // Existing constants...
    JobTypeGitHubRepoFile   = "github_repo_file"   // Process single repo file
    JobTypeGitHubActionLog  = "github_action_log"  // Already exists, verify
)
```

### In document.go
Add/verify source type constants:
```go
const (
    // Existing constants...
    SourceTypeGitHubRepo      = "github_repo"       // Repository content
    SourceTypeGitHubActionLog = "github_action_log" // Action log (verify exists)
)
```

## Acceptance
- [ ] JobTypeGitHubRepoFile constant defined
- [ ] SourceTypeGitHubRepo constant defined
- [ ] Existing github_action_log constants verified
- [ ] Compiles
- [ ] Tests pass
