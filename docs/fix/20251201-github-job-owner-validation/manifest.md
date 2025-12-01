# Fix: GitHub Job Owner Validation and Status
- Slug: github-job-owner-validation | Type: fix | Date: 2025-12-01
- Request: "1. Does the github connector and jobs need an owner for the repo? 2. This job failed, hence the status should be failed."
- Prior: none

## Issues Identified
1. GitHub Repository Collector job requires 'owner' field in config but validation fails
2. Job shows "Completed" status despite step validation failure - should show "Failed"
