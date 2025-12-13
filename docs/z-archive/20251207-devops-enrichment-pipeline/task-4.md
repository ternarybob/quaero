# Task 4: Implement classify_devops action (LLM-based)

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Pass 3 of enrichment: Use LLM to classify each file's role, component, test type, and external dependencies - turning raw structure into DevOps-actionable classifications.

## Do

- Create `internal/jobs/actions/classify_devops.go`
- Craft LLM prompt that:
  - Explains context is for DevOps engineer, not C programmer
  - Includes file path, language, extracted structure (Pass 1 data)
  - Requests JSON response with specific fields
  - Truncates content to ~6000 chars
- Parse JSON response and extract:
  - file_role (header, source, build, test, config, resource)
  - component (logical module name)
  - test_type (unit, integration, hardware, manual, none)
  - test_framework (gtest, catch, cunit, custom, etc.)
  - test_requires (external test requirements)
  - external_deps (hardware, services, databases, SDKs)
  - config_sources (env, file, registry, hardcoded)
- Handle LLM errors with retry and backoff
- Update document metadata with classification data

## Accept

- [ ] LLM prompt crafted for DevOps context
- [ ] Includes Pass 1 extracted structure in prompt
- [ ] Content truncated to ~6000 chars
- [ ] JSON response parsed correctly
- [ ] All classification fields populated
- [ ] Retry with backoff on LLM errors
- [ ] Updates document metadata with `devops.classification` fields
- [ ] Adds "classify_devops" to enrichment_passes
