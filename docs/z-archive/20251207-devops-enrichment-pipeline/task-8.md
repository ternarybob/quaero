# Task 8: Implement DevOps API handler and endpoints

Depends: 5,6 | Critical: no | Model: sonnet

## Addresses User Intent

Expose enrichment results via REST API so DevOps engineers can query the generated knowledge.

## Do

- Create `internal/handlers/devops_handler.go`
- Implement endpoints:
  - GET /api/devops/summary - Return generated DevOps guide
  - GET /api/devops/components - List components with stats
  - GET /api/devops/graph - Return dependency graph
  - GET /api/devops/platforms - Return platform matrix
  - POST /api/devops/enrich - Trigger enrichment pipeline
- Register routes in router
- Follow existing handler patterns (RequireMethod, WriteJSON, error handling)

## Accept

- [ ] DevOpsHandler created following handler patterns
- [ ] GET /api/devops/summary returns markdown guide (404 if not yet generated)
- [ ] GET /api/devops/components returns component list with file counts
- [ ] GET /api/devops/graph returns JSON graph structure
- [ ] GET /api/devops/platforms returns platform matrix
- [ ] POST /api/devops/enrich triggers job and returns job ID
- [ ] Routes registered in router
- [ ] Proper error handling and status codes
