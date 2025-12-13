# Task 2: Implement extract_structure action (regex-based)

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Pass 1 of enrichment: Extract #includes, #defines, platform conditionals from C/C++ files without LLM - provides foundation data for all subsequent passes.

## Do

- Create `internal/jobs/actions/extract_structure.go`
- Implement regex patterns for:
  - `#include "..."` (local includes)
  - `#include <...>` (system includes)
  - `#define SYMBOL` (defines)
  - `#ifdef/#ifndef SYMBOL` (conditionals)
  - Platform detection (_WIN32, __linux__, __APPLE__, etc.)
- File detection for C/C++ extensions: .c, .cpp, .cc, .cxx, .h, .hpp, .hxx, .hh
- Update document metadata with extracted data
- Track enrichment pass in `enrichment_passes` array

## Accept

- [ ] Extracts local includes correctly
- [ ] Extracts system includes correctly
- [ ] Extracts defines correctly
- [ ] Extracts conditionals correctly
- [ ] Detects platforms (windows, linux, macos, embedded)
- [ ] Skips non-C/C++ files
- [ ] Updates document metadata with `devops.extracted` fields
- [ ] Adds "extract_structure" to enrichment_passes
- [ ] Idempotent - skips if already processed (unless force flag)
