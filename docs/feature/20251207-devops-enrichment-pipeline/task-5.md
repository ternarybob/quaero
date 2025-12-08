# Task 5: Implement build_dependency_graph action

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Pass 4 of enrichment: Aggregate include/link relationships across all documents into a queryable dependency graph stored in KV storage.

## Do

- Create `internal/jobs/actions/build_dependency_graph.go`
- Query all documents with Pass 1 metadata
- Build graph structure with:
  - Nodes: file paths
  - Edges with types: includes, links, tests, builds
- Normalize file paths for consistent matching
- Handle circular includes gracefully
- Store graph in KV storage under `devops:dependency_graph`
- Compute component-level aggregations

## Accept

- [ ] Queries all documents with devops metadata
- [ ] Creates nodes for each file
- [ ] Creates edges for include relationships
- [ ] Creates edges for link relationships
- [ ] Edge types properly categorized
- [ ] Paths normalized
- [ ] Circular includes handled
- [ ] Graph stored in KV as JSON under `devops:dependency_graph`
- [ ] Component aggregation computed
- [ ] Adds "build_dependency_graph" to enrichment tracking
