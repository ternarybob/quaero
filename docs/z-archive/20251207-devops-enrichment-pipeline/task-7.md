# Task 7: Create devops_enrich job definition

Depends: 2,3,4,5,6 | Critical: no | Model: sonnet

## Addresses User Intent

Orchestrate all 5 passes in the correct sequence as a single job definition that users can trigger.

## Do

- Create `jobs/devops_enrich.toml` job definition file
- Define 5 steps in sequence:
  1. extract_structure (all C/C++ files, parallel)
  2. analyze_build_system (build files only)
  3. classify_devops (all files with Pass 1 metadata, batched)
  4. build_dependency_graph (single aggregation job)
  5. aggregate_devops_summary (single synthesis job)
- Configure dependencies between steps
- Set appropriate error strategies
- Configure filter tags for each step

## Accept

- [ ] Job definition file exists at jobs/devops_enrich.toml
- [ ] All 5 steps defined in correct order
- [ ] Step dependencies configured
- [ ] Error strategies appropriate (continue for regex, fail for aggregation)
- [ ] Filter tags configured for each step
- [ ] Job can be loaded and validated by job service
