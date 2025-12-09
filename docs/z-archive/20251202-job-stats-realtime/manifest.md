# Fix: Job Statistics Panel Not Aligned with Job Step Status in Real-Time
- Slug: job-stats-realtime | Type: fix | Date: 2025-12-02
- Request: "The step progress reporting is STILL not aligned / updating to the actual. The job loads (page refresh), however does NOT update in real time. The Job statistics is ahead and creates a mismatch. Preference fixing the job step status/statistics. Use the test\ui\queue_test.go -> TestNearbyRestaurantsKeywordsMultiStep add to the test to monitor align between 'Job Statistics' panel and Job Step Status. Note: The job runs and completes however, the UI does NOT update. This is a fail."
- Prior: ./docs/fix/20251202-step-manager-realtime/ (previous fix attempt)

## User Intent
Fix the real-time synchronization between:
1. **Job Statistics panel** (top of page) - shows aggregate counts across all jobs
2. **Job Progress bar** (on each job row) - shows progress of child jobs within a manager job
3. **Step progress** (in step rows) - shows progress of child jobs within each step

The screenshot shows mismatch:
- Job Statistics: 4 pending, 10 running, 2 completed, 7 failed
- Job Progress/Step: 7 pending, 10 running, 2 completed, 2 failed

The UI loads correctly on page refresh but does NOT update in real-time as jobs execute.

## Success Criteria
- [ ] Job Statistics panel updates in real-time as child jobs change status
- [ ] Job Progress bar updates in real-time matching the actual child job counts
- [ ] Step progress updates in real-time matching actual child job counts
- [ ] All three displays show CONSISTENT values during job execution
- [ ] Add test verification to TestNearbyRestaurantsKeywordsMultiStep to monitor alignment
