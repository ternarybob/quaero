# Step 1: Create Badger-based queue manager to replace goqite

**Skill:** @code-architect
**Files:** `internal/queue/badger_manager.go` (NEW), `internal/interfaces/queue_service.go`

---

## Iteration 1

### Agent 2 - Implementation

Creating a new Badger-based queue manager to replace the goqite SQLite-backed queue. This implementation provides the same interface as the current Manager but uses badgerhold for persistence.

**Key Design Decisions:**
- Message IDs use timestamp-prefixed UUIDs for FIFO ordering: `{timestamp_ns}:{uuid}`
- Messages stored as badgerhold structs with indexed fields for efficient queries
- Visibility timeout implemented using `visible_at` timestamp field with indexed queries
- FIFO ordering maintained through timestamp-prefixed message IDs and SortBy("ID")
- Atomic operations using badgerhold queries and updates
- Redelivery tracking with `receive_count` and `max_receive` limit
- Messages exceeding max_receive are skipped (dead letter handling)

**Changes made:**
- `internal/queue/badger_manager.go`: Created new Badger queue manager with full queue functionality
  - Uses badgerhold.Store instead of raw badger.DB
  - QueueMessage struct with badgerhold tags for indexing
  - Enqueue, Receive, Extend, and Close methods implemented
- `internal/interfaces/queue_service.go`: Updated QueueManager interface to use `string` instead of `goqite.ID`
  - Removed goqite import
  - Updated Extend method signature
  - Added breaking change documentation
- `internal/queue/manager.go`: Temporarily updated Extend signature to match interface (will be deleted in Step 5)

**Commands run:**
```bash
go build -o /tmp/quaero.exe ./cmd/quaero
```

### Agent 3 - Validation
**Skill:** @code-architect

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ No tests applicable (tests will be updated in Step 7)

**Code Quality:**
✅ Follows Go patterns
✅ Matches existing badger storage code style
✅ Proper error handling with wrapped errors
✅ Uses badgerhold idiomatically (Find, Update, Insert, Delete)
✅ Clean interface implementation
✅ Good separation of concerns (queue logic separate from storage)

**Quality Score:** 9/10

**Issues Found:**
None - implementation is clean and follows project patterns

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
Badger queue manager successfully created. The implementation uses badgerhold for clean, indexed queries instead of manual key iteration. FIFO ordering is guaranteed through timestamp-prefixed message IDs. Ready to integrate into app initialization in the next steps.

**→ Continuing to Step 2**
