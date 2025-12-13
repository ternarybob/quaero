# Task 12: Write long-running UI tests with progress monitoring

Depends: 7,9 | Critical: no | Model: sonnet

## Addresses User Intent

Verify the full pipeline works end-to-end with realistic data and proper progress tracking.

## Do

- Create `test/ui/devops_enrichment_test.go`
- Follow pattern from `test/ui/queue_test.go` (news crawler long-running test)
- Implement test scenarios:

1. **TestDevOpsEnrichmentPipeline_FullFlow**
   - Import test fixtures via API
   - Run full enrichment pipeline
   - Verify all 5 passes complete
   - Verify metadata on each document
   - Verify graph contains expected edges
   - Verify summary contains expected sections

2. **TestDevOpsEnrichmentPipeline_ProgressMonitoring**
   - Monitor WebSocket progress events
   - Verify parent job tracks child job completion
   - Verify job logs show each pass

3. **TestDevOpsEnrichmentPipeline_IncrementalEnrich**
   - Run enrichment on subset
   - Add more files
   - Run again, verify only new files processed
   - Verify graph updated correctly

4. **TestDevOpsEnrichmentPipeline_LargeCodebase**
   - Generate 1000+ synthetic files
   - Run enrichment with 10 minute timeout
   - Verify completion and resource cleanup

## Accept

- [ ] Test file created at test/ui/devops_enrichment_test.go
- [ ] Uses queue_test.go patterns (polling, screenshots, logging)
- [ ] All 4 test scenarios implemented
- [ ] Progress monitoring via WebSocket/polling
- [ ] Timeout handling appropriate
- [ ] Resource cleanup in defer blocks
- [ ] All UI tests pass
