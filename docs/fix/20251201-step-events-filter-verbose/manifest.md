# Fix: Step Events Filter Verbose Logs from UI

- Slug: step-events-filter-verbose | Type: fix | Date: 2025-12-01
- Request: "The events/logging pushed to the github repo job are too verbose. Committing to DB is fine, however not to the UI. The step manager has control, and should filter out anything below INFO - i.e. not send to the UI."
- Prior: docs/feature/20251201-refactor-queue-arch/ (step monitoring architecture)
