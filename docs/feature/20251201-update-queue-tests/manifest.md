# Feature: Update Queue Tests for Multi-Step Jobs
- Slug: update-queue-tests | Type: feature | Date: 2025-12-01
- Request: "Create/Update the api and ui tests for TestNearbyRestaurantsKeywordsMultiStep: 1) Child jobs run in correct order based on config/dependencies, 2) filter_source_type filters documents correctly, 3) UI shows document count for each child job (remove redundant parent count), 4) UI expands/collapses child jobs on click. Tests only - should fail on current codebase."
- Prior: docs/fix/20251201-child-expand-docs/
