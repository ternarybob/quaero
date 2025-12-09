# Step 5: Implement build_dependency_graph action

Model: sonnet | Status: ✅

## Done

- Created BuildDependencyGraphAction with graph construction
- Defined DependencyGraph, GraphNode, GraphEdge, ComponentSummary structures
- Implemented edge creation for includes, links, tests, builds
- Added path normalization for cross-platform matching
- Implemented component aggregation with file counts
- Store graph in KV under "devops:dependency_graph"
- Updated DevOpsWorker.handleBuildDependencyGraph to use action

## Files Changed

- `internal/jobs/actions/build_dependency_graph.go` - New action file
- `internal/queue/workers/devops_worker.go` - Updated handler

## Build Check

Build: ✅ (syntax validated) | Tests: ⏭️
