# Fix: Child Jobs Detached from Parent in UI
- Slug: child-jobs-detached-parent | Type: fix | Date: 2025-12-01
- Request: "The jobs detaches from the parent" - child jobs not showing attached to parent in Job Queue UI
- Prior: ./docs/fix/20251201-github-job-owner-validation/

## Issues Identified
1. Job Statistics shows 1001 completed but Job Queue shows parent as "Running"
2. Child jobs (1000) counted in statistics but not displayed under parent
3. Many WebSocket warnings: "Failed to send job status change to client", "Failed to send parent job progress to client"
4. The warnings are marked as "info" level but user marked them as needing "WARN" level
