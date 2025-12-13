# Fix: Step Status Mismatch - Failed Steps Show as Completed
- Slug: step-status-mismatch | Type: fix | Date: 2025-12-09
- Request: "The complete solution is failing, individual steps are NOT reporting as failure, and should."
- Prior: none

## User Intent
When a step fails (e.g., "worker init failed: no documents found matching tags"), the UI should show that step as **Failed** (red), not **Completed** (green). Currently:
- Step 5/9 `generate_index` shows "Completed" in green, but events show `[ERR] Init failed` and `Status changed: failed`
- Step 8/9 `build_graph` shows "Completed" in green, but had warnings
- Step 9/9 `generate_map` shows "Completed" in green, but events show `[ERR] Init failed` and `Status changed: failed`

The parent job correctly shows "Failed (3 failed)" but the individual step badges show incorrect "Completed" status.

## Success Criteria
- [x] Steps that fail during init or execution show "Failed" status badge in UI (red)
- [x] Steps that complete successfully show "Completed" status badge in UI (green)
- [x] The step status badge matches the actual step status from events/logs
