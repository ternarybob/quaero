# Plan: Job Description Optimization

## Classification
- Type: feature
- Workdir: ./docs/feature/20251130-job-description-optim/

## Analysis

### Dependencies
- Job configuration TOML files across 3 directories
- Queue tests must pass after changes

### Approach
1. Remove deprecated fields (`type`, `job_type`, `source_type`) from all job definitions
2. Convert flat crawler configs to step-based structure where missing
3. Validate with queue tests

### Risks
- Breaking existing job loading if structure is incorrect
- Test failures if jobs are malformed

## Files Analysis

### test/config/job-definitions/ (10 files)

| File | Has Steps | Deprecated Fields | Action |
|------|-----------|-------------------|--------|
| my-custom-crawler.toml | NO | none | Add step, move crawler config |
| news-crawler.toml | NO | type, job_type | Add step, move crawler config, remove deprecated |
| github-actions-collector.toml | YES | type, job_type, source_type | Remove deprecated fields |
| github-repo-collector.toml | YES | type, job_type, source_type | Remove deprecated fields |
| github-repo-collector-batch.toml | YES | type, job_type, source_type | Remove deprecated fields |
| github-repo-collector-by-name.toml | YES | type, job_type, source_type | Remove deprecated fields |
| keyword-extractor-agent.toml | YES | type, job_type | Remove deprecated fields |
| nearby-restaurants-places.toml | YES | type, job_type | Remove deprecated fields |
| test-agent-job.toml | YES | type, job_type | Remove deprecated fields |
| web-search-asx.toml | YES | type, job_type | Remove deprecated fields |

### deployments/local/job-definitions/ (7 files)

| File | Has Steps | Deprecated Fields | Action |
|------|-----------|-------------------|--------|
| agent-document-generator.toml | YES | type, job_type | Remove deprecated fields |
| agent-web-enricher.toml | YES | type, job_type | Remove deprecated fields |
| github-actions-collector.toml | YES | type, job_type, source_type | Remove deprecated fields |
| github-repo-collector.toml | YES | type, job_type, source_type | Remove deprecated fields |
| keyword-extractor-agent.toml | YES | type, job_type | Remove deprecated fields |
| nearby-restaurants-places.toml | YES | type, job_type | Remove deprecated fields |
| news-crawler.toml | YES | type, job_type, source_type | Remove deprecated fields |

### bin/job-definitions/ (7 files)

| File | Has Steps | Deprecated Fields | Action |
|------|-----------|-------------------|--------|
| agent-document-generator.toml | YES | type, job_type | Remove deprecated fields |
| agent-web-enricher.toml | YES | type, job_type | Remove deprecated fields |
| github-repo-collector.toml | YES | type, job_type, source_type | Remove deprecated fields |
| keyword-extractor-agent.toml | YES | type, job_type | Remove deprecated fields |
| nearby-restaurants-places.toml | YES | type, job_type | Remove deprecated fields |
| news-crawler.toml | YES | type, job_type | Remove deprecated fields |
| web-search-asx.toml | YES | type, job_type | Remove deprecated fields |

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Fix test/config crawler jobs missing steps | none | no | medium | sonnet |
| 2 | Remove deprecated fields from test/config jobs | 1 | no | low | sonnet |
| 3 | Remove deprecated fields from deployments/local jobs | none | no | low | sonnet |
| 4 | Remove deprecated fields from bin/job-definitions jobs | none | no | low | sonnet |
| 5 | Run queue tests and validate | 2,3,4 | no | low | sonnet |

## Order
Sequential: [1] -> Concurrent: [2,3,4] -> Sequential: [5]
