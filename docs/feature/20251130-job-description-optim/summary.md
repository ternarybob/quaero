# Complete: Job Description Optimization

## Classification
- Type: feature
- Location: ./docs/feature/20251130-job-description-optim/

Refactored all job definition TOML files across three directories to remove deprecated fields (`type`, `job_type`, `source_type`) and ensure all jobs comply with the standard step-based configuration structure. Two crawler jobs that were missing step sections were converted to the new format.

## Stats
Tasks: 5 | Files: 24 | Duration: ~5 min
Models: Planning=opus, Workers=1×opus, Review=N/A (no critical triggers)

## Tasks
- Task 1: Converted 2 crawler jobs (my-custom-crawler.toml, news-crawler.toml) to step-based structure
- Task 2: Removed deprecated fields from 8 test/config job definitions
- Task 3: Removed deprecated fields from 7 deployments/local job definitions
- Task 4: Removed deprecated fields from 7 bin/job-definitions job definitions
- Task 5: Validated all changes with queue tests

## Files Modified

### test/config/job-definitions/ (10 files)
- my-custom-crawler.toml - Added step section
- news-crawler.toml - Added step section, removed deprecated fields
- github-actions-collector.toml - Removed type, job_type, source_type
- github-repo-collector.toml - Removed type, job_type, source_type
- github-repo-collector-batch.toml - Removed type, job_type, source_type
- github-repo-collector-by-name.toml - Removed type, job_type, source_type
- keyword-extractor-agent.toml - Removed type, job_type
- nearby-restaurants-places.toml - Removed type, job_type
- test-agent-job.toml - Removed type, job_type
- web-search-asx.toml - Removed type, job_type

### deployments/local/job-definitions/ (7 files)
- agent-document-generator.toml - Removed type, job_type
- agent-web-enricher.toml - Removed type, job_type
- github-actions-collector.toml - Removed type, job_type, source_type
- github-repo-collector.toml - Removed type, job_type, source_type
- keyword-extractor-agent.toml - Removed type, job_type
- nearby-restaurants-places.toml - Removed type, job_type
- news-crawler.toml - Removed type, job_type, source_type

### bin/job-definitions/ (7 files)
- agent-document-generator.toml - Removed type, job_type
- agent-web-enricher.toml - Removed type, job_type
- github-repo-collector.toml - Removed type, job_type, source_type
- keyword-extractor-agent.toml - Removed type, job_type
- nearby-restaurants-places.toml - Removed type, job_type
- news-crawler.toml - Removed type, job_type
- web-search-asx.toml - Removed type, job_type

## Review: N/A
No critical triggers (security, authentication, crypto, etc.) in this feature.

## Verify
go build: N/A (config files only) | go test: ✅ TestJobManagement_JobQueue passed
