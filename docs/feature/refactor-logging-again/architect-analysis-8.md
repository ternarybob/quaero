# ARCHITECT Analysis - Duplicate Logs and Line Number Reset

## Screenshot Analysis
- Top section: Lines 69-84 with correct sequential server line numbers
- Bottom section: Lines 1,1,2,2,3,3... with duplicates and reset to 1

## Root Cause
Logs are being loaded TWICE from different sources:
1. `loadJobTreeData` → calls `fetchStepLogs` (API: `/api/logs`)
2. `connectJobSSE` → calls `sendInitialJobLogs` (SSE initial batch)

Both send the same logs, and the duplicate detection uses:
```javascript
const existingIds = new Set(existingLogs.map(l => l.id || `${l.timestamp}-${l.message}`));
```

If logs don't have an `id` field and have identical timestamp+message, they appear as duplicates in the UI.

## Issue Analysis
1. **Duplicate logs**: Same logs from API and SSE initial load
2. **Line numbers 1,1,2,2**: The logs from both sources have line_number, but since both sets are merged, you see each line number twice
3. **Reset to 1**: Both API and SSE initial load start with line_number=1

## Fix Options

### Option A: Use `line_number` as unique ID (RECOMMENDED)
The server-assigned `line_number` is unique per step. Use it as the duplicate key:
```javascript
const existingIds = new Set(existingLogs.map(l => l.line_number || `${l.timestamp}-${l.message}`));
const uniqueNewLogs = newLogs.filter(l => !existingIds.has(l.line_number || `${l.timestamp}-${l.message}`));
```

### Option B: Skip SSE initial logs if API already loaded
Not recommended - requires coordination between API and SSE.

### Option C: Remove API fetch and rely only on SSE
Not recommended - breaks existing pagination.

## Recommendation
**MODIFY** `handleSSELogs` to use `line_number` as the primary duplicate key.
This leverages the server-assigned unique line numbers we just implemented.
