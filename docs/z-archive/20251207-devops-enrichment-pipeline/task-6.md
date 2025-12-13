# Task 6: Implement aggregate_devops_summary action (LLM synthesis)

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Pass 5 of enrichment: Generate a comprehensive DevOps guide from all enriched data - the primary deliverable for the DevOps engineer.

## Do

- Create `internal/jobs/actions/aggregate_devops_summary.go`
- Query all enriched documents and aggregate:
  - Build system overview (targets, toolchains)
  - Platform matrix
  - Component list
  - Test strategy
  - External dependencies
- Craft LLM prompt requesting markdown output covering:
  - Build system overview
  - Toolchain requirements
  - Component architecture
  - Test strategy
  - Platform matrix
  - CI/CD recommendations
  - Common issues
- Store summary in KV under `devops:summary`
- Create searchable document with ID `devops-summary`

## Accept

- [ ] Aggregates all enriched document data
- [ ] LLM prompt requests comprehensive markdown guide
- [ ] Covers all required sections
- [ ] Emphasizes actionable, specific guidance
- [ ] Summary stored in KV as `devops:summary`
- [ ] Document created with ID `devops-summary`
- [ ] Document is searchable
- [ ] Adds "aggregate_devops_summary" to enrichment tracking
