# Task 1: Sort Steps by Dependencies
Depends: - | Critical: no | Model: sonnet

## Do
1. Modify `ToJobDefinition()` in `internal/jobs/service.go`
2. After parsing all steps from the map, implement topological sort
3. Steps with no `depends` go first
4. Steps with `depends` must come after their dependencies

## Accept
- [ ] Steps are ordered correctly based on `depends` field
- [ ] Step with `depends = "search_nearby_restaurants"` comes after step `search_nearby_restaurants`
- [ ] Build compiles without errors
