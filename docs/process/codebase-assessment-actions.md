# Codebase Assessment Acceleration Actions

This checklist captures specific, Go-native actions to keep the assessment pipeline fast and predictable, even on large repositories.

## Intake and Filtering
- Narrow extensions to active languages and add negative globs for generated/vendor/output folders (`node_modules`, `vendor`, `dist`, `coverage`, `.cache`, `tmp`).
- Apply `max_files` limits per import step (keep initial scans under ~50 files) and set `max_depth` to skip deep vendor trees.
- Cap `max_file_size` for code ingestion while allowing overrides for a few critical large files.

## Native Analysis Before LLMs
- Generate dependency graphs with native tooling when available (e.g., `go list -deps ./...` or `gopls` package info) and feed the results into graph steps instead of inferring via LLMs.
- Extract build/run/test commands via deterministic parsers for manifest files (Go modules, package.json, Makefile) before invoking agents.
- Hash files during the walk to detect unchanged inputs and reuse cached classifications/summaries.

## Batching and Concurrency
- Process directories in small batches (200–300 files) with worker pools; merge results before synthesis.
- Rate-limit outbound LLM calls and keep batch sizes small enough to avoid context overflows.

## Timing and Observability
- Measure wall-clock timing per step (import, classification, graphing, synthesis) and fail tests when bounds are exceeded.
- Log step start/stop markers with file counts and cache hit ratios to spot slow phases quickly.
- Persist timing histories so repeated runs surface regressions.

## Safety Rails
- Default to skipping non-source assets (media, archives) and add explicit allowlists for intentionally large files.
- Keep `error_tolerance` conservative for assessment jobs so a few bad files do not abort the pipeline.
