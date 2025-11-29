# Plan: Fix stale job detector panic

## Analysis
The stale job detector loop panics with:
```
panic=reflect: call of reflect.Value.Interface on zero Value
```

This occurs in `GetStaleJobs()` at line 555 in `queue_storage.go`:
```go
badgerhold.Where("Status").Eq("running").And("LastHeartbeat").IsNil().And("StartedAt").Lt(threshold)
```

The `IsNil()` query causes a reflection panic in BadgerHold when comparing nil pointer fields. This is a known issue with BadgerHold's handling of pointer types in queries.

**Approach**: Replace the problematic `IsNil()` query with a two-step approach:
1. First query for running jobs
2. Filter in-memory for jobs where `LastHeartbeat == nil` and `StartedAt < threshold`

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Fix GetStaleJobs to avoid IsNil() query | none | no | low | sonnet |

## Order
Sequential: [1] → Validate
