# Task 9: Create C/C++ test fixtures

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Provide realistic C/C++ project structure for testing the enrichment pipeline end-to-end.

## Do

- Create `test/fixtures/cpp_project/` directory
- Create files:
  - Makefile with multiple targets (all, test, clean)
  - CMakeLists.txt (optional, for CMake testing)
  - src/main.cpp with includes and platform detection
  - src/utils.cpp with utility functions
  - include/utils.h header file
  - src/platform_win.cpp with Windows-specific code (#ifdef _WIN32)
  - src/platform_linux.cpp with Linux-specific code (#ifdef __linux__)
  - tests/test_main.cpp with test framework includes (gtest pattern)
- Include realistic patterns:
  - Local and system includes
  - Multiple #defines
  - Platform conditionals
  - Test framework references

## Accept

- [ ] Directory structure created at test/fixtures/cpp_project/
- [ ] Makefile has multiple targets
- [ ] main.cpp includes local and system headers
- [ ] Platform-specific files have appropriate #ifdef guards
- [ ] Test file has test framework includes
- [ ] Files contain realistic C/C++ patterns
- [ ] At least 8 source files total
