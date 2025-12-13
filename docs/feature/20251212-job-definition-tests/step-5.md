# Step 5: Create codebase_classify job definition test

## Implementation Details

### Created File
`C:\development\quaero\test\ui\job_definition_codebase_classify_test.go`

### Test Function
`TestJobDefinitionCodebaseClassify(t *testing.T)`

### Configuration
The test uses the following configuration:
- **JobName**: "Codebase Classify" - Matches the name in the TOML file
- **JobDefinitionPath**: "../config/job-definitions/codebase_classify.toml" - Relative path from test/ui/
- **Timeout**: 15 minutes - Reasonable for testing (production TOML defines 4h timeout)
- **RequiredEnvVars**: nil - The rule_classifier agent type doesn't require API keys
- **AllowFailure**: true - Paths in TOML (C:\development\quaero) may not exist in all test environments

### Job Definition Analysis
The codebase_classify.toml defines a three-phase pipeline:
1. **Phase 1 (Parallel)**:
   - `code_map` step: Builds hierarchical code structure map
   - `import_files` step: Imports codebase files as documents
2. **Phase 2 (Sequential)**:
   - `rule_classify_files` step: Rule-based classification (depends on import_files)

The job uses local file paths and doesn't require external API keys, making it suitable for testing with AllowFailure=true to handle environment variations.

### Test Context
- Creates UITestContext with 20 minute overall timeout
- Defers cleanup to ensure proper resource release
- Logs test progression for debugging

### Test Flow
1. Copy job definition TOML to results directory
2. Navigate to Jobs page and take screenshot
3. Trigger the "Codebase Classify" job via UI
4. Monitor job status with 15 minute timeout
5. Allow job failure (paths may not exist)
6. Refresh page and take final screenshot

### Compilation
Verified with `go build ./test/ui/...` - No errors

## Acceptance Criteria Met
- [x] File `test/ui/job_definition_codebase_classify_test.go` exists
- [x] Test function TestJobDefinitionCodebaseClassify defined
- [x] Uses JobDefinitionTestConfig with correct values
- [x] Timeout set to 15 minutes (reasonable for test, not 4h)
- [x] No required env vars (rule_classifier is local)
- [x] AllowFailure set to true (path issues possible)
- [x] Code compiles: `go build ./test/ui/...`

## Next Steps
Task 6: Verification of all job definition tests
