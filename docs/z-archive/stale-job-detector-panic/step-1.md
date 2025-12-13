# Step 1: Fix GetStaleJobs to avoid IsNil() query

- Task: task-1.md | Group: 1 | Model: sonnet

## Actions
1. Analyzed panic stack trace: `reflect: call of reflect.Value.Interface on zero Value`
2. Identified problematic code in `GetStaleJobs()` using BadgerHold's `IsNil()` query
3. Replaced two-query approach with single query + in-memory filtering

## Files
- `internal/storage/badger/queue_storage.go` - fixed GetStaleJobs function

## Changes
```go
// Before: Two queries, second uses IsNil() which panics
err := s.db.Store().Find(&staleRecords, badgerhold.Where("Status").Eq("running").And("LastHeartbeat").Lt(threshold))
err = s.db.Store().Find(&noHeartbeatRecords, badgerhold.Where("Status").Eq("running").And("LastHeartbeat").IsNil().And("StartedAt").Lt(threshold))

// After: Single query + in-memory filtering
err := s.db.Store().Find(&runningRecords, badgerhold.Where("Status").Eq("running"))
for _, record := range runningRecords {
    if record.LastHeartbeat != nil && record.LastHeartbeat.Before(threshold) {
        staleRecords = append(staleRecords, record)
    } else if record.LastHeartbeat == nil && record.StartedAt != nil && record.StartedAt.Before(threshold) {
        staleRecords = append(staleRecords, record)
    }
}
```

## Decisions
- Used in-memory filtering instead of BadgerHold queries for nil pointer comparisons
- This is safer and avoids reflection-based nil checks that can panic

## Verify
Compile: ✅ | Tests: N/A (runtime fix)

## Status: ✅ COMPLETE
