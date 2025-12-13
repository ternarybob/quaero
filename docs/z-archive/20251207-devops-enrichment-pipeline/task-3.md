# Task 3: Implement analyze_build_system action (regex + LLM)

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Pass 2 of enrichment: Parse build files to understand how the codebase is compiled, what targets exist, and what dependencies are required.

## Do

- Create `internal/jobs/actions/analyze_build_system.go`
- File detection for build files:
  - Makefile*, *.mk
  - CMakeLists.txt, *.cmake
  - *.vcxproj, *.vcxproj.filters
  - configure*, *.sln
- Implement regex extraction for:
  - Makefile targets (pattern: `target:`)
  - CMake targets (add_executable, add_library)
  - Compiler flags (-D, -I, -L, -l patterns)
  - Linked libraries
- Use LLM for complex dependency analysis when regex insufficient
- Update document metadata with build system data

## Accept

- [ ] Detects all build file types
- [ ] Extracts Makefile targets
- [ ] Extracts CMake targets
- [ ] Extracts compiler flags
- [ ] Extracts linked libraries
- [ ] LLM called for complex analysis
- [ ] Updates document metadata with `devops.build_system` fields
- [ ] Adds "analyze_build_system" to enrichment_passes
