# Complete: UI Events LLM Indicator and Remove Queue Stats

Type: fix | Tasks: 3 | Files: 1

## User Request
"1. Workers making LLM calls need to indicate in events whether it's an LLM call or local. 2. Remove step queue assessment stats - inaccurate and shows wrong information."

## Result
Removed all inaccurate queue stats displays from the Queue UI:
- **Job-level**: Removed "Progress: X pending, Y running, Z completed, W failed" from parent job cards
- **Step-level**: Removed the same stats from step cards
- **Dead code**: Removed unused stepProgress computation

The LLM indicator ("AI:" vs "Rule:" prefix in events) was already implemented and verified working.

## Skills Used
go, frontend

## Validation: ✅ MATCHES

## Review: N/A (no critical triggers)

## Verify
Build: ✅ | Tests: ⏭️ (UI change only)

## Files Changed
- `pages/queue.html` - Removed 50 lines (2 insertions, 50 deletions)
  - Lines 191-198: Step progress stats template
  - Lines 471-485: Job progress display block
  - Lines 2545-2568: stepProgress computation
  - Line 2583: `progress: stepProgress` property
