# Test Configuration Files

This directory contains configuration files for automated tests.

## Structure

```
test/config/
├── setup.toml                    # Test harness config (build, service, output)
├── test-quaero.toml              # Base service config for all tests
├── job-definitions/              # Job definitions (copied to bin/job-definitions/)
│   ├── news-crawler.toml         # Example news crawler job
│   ├── my-custom-crawler.toml    # Example custom crawler job
│   └── test-agent-job.toml       # Test agent job (keyword extraction)
└── README.md                     # This file
```

## Configuration Approach

**Simplified multi-config system:**
1. **Base config** (`test-quaero.toml`) - Complete service configuration
2. **Override configs** (`quaero-*.toml`) - Only the settings to change
3. **Priority order** - Later files override earlier files

### Job Definitions: Auto-loaded from Directory

**All job definitions are auto-loaded:**
- Files in `test/config/job-definitions/` are copied to `bin/job-definitions/`
- Service auto-loads all `.toml` and `.json` files from this directory on startup
- Includes: `news-crawler.toml`, `my-custom-crawler.toml`, `test-agent-job.toml`

**Note:** Tests can also load additional job definitions via API using `env.LoadJobDefinitionFile()` if needed for specific test scenarios.

### Test Harness Config (`setup.toml`)

Contains test infrastructure settings:
- Build configuration (source dir, binary output, version file)
- Service lifecycle (startup timeout, ports, shutdown endpoint)
- Output configuration (results directory)

**Loaded automatically** - No need to specify in tests.

### Service Configs

**Base Config (`test-quaero.toml`):**
- Complete Quaero service configuration
- Used by all tests as the foundation
- Mock LLM mode, database reset, debug logging

**Override Configs:**
- `test-quaero-no-variables.toml` - Disables agent service (clears Google API key)

**Job Definitions:**
- `job-definitions/` - Directory of job definitions (copied to `bin/job-definitions/`, auto-loaded by service)
  - `news-crawler.toml` - Example news crawler job
  - `my-custom-crawler.toml` - Example custom crawler job
  - `test-agent-job.toml` - Test agent job (keyword extraction)

## Usage

### In Tests

```go
// Use base config only (test-quaero.toml)
env, err := common.SetupTestEnvironment("TestName")

// Use base + override (test-quaero.toml + test-quaero-no-variables.toml)
env, err := common.SetupTestEnvironment("TestName", "../config/test-quaero-no-variables.toml")

// Use base + multiple overrides (if you create additional override files)
env, err := common.SetupTestEnvironment("TestName",
    "../config/test-quaero-no-variables.toml",
    "../config/custom-override.toml")
```

### Command Line (Main Application)

```bash
# Single config
./bin/quaero.exe -config deployments/local/quaero.toml

# Multiple configs (later files override earlier ones)
./bin/quaero.exe -config base.toml -config override.toml

# Shorthand
./bin/quaero.exe -c base.toml -c override.toml
```

## Priority Order

Configuration values are resolved in this order (highest to lowest priority):

1. **CLI flags** - `--port 8080`, `--host localhost`
2. **Environment variables** - `QUAERO_SERVER_PORT=8080`
3. **Last config file** - `override.toml`
4. **...** - Additional config files
5. **First config file** - `base.toml`
6. **Defaults** - Hardcoded in `internal/common/config.go`

## Examples

### Example 1: Disable Agent Service

**Base config** (`test-quaero.toml`):
```toml
[agent]
google_api_key = ""
model_name = "gemini-3-pro-preview"
```

**Override config** (`test-quaero-no-variables.toml`):
```toml
[agent]
google_api_key = ""  # Explicitly disable
model_name = ""      # Clear model name
```

**Result:** Agent service disabled, job definitions show "Disabled" badge.

### Example 2: Custom Port (Create Your Own Override)

**Base config** (`test-quaero.toml`):
```toml
[server]
port = 18085
host = "localhost"
```

**Create override config** (`custom-port.toml`):
```toml
[server]
port = 19999  # Override port
```

**Usage:**
```go
env, err := common.SetupTestEnvironment("TestName", "../config/custom-port.toml")
```

**Result:** Service runs on port 19999 instead of 18085.

## Benefits

1. **Simplicity** - One base config, small override files
2. **Reusability** - Share base config across all tests
3. **Flexibility** - Easy to add new override scenarios
4. **Maintainability** - Change common settings in one place
5. **Clarity** - Override files show only what's different

## Creating Custom Override Files

To create a custom override config for your tests:

1. **Create a new TOML file** in `test/config/` (e.g., `my-override.toml`)
2. **Add only the settings you want to override:**
   ```toml
   [server]
   port = 19999

   [logging]
   level = "warn"
   ```
3. **Use in your test:**
   ```go
   env, err := common.SetupTestEnvironment("TestName", "../config/my-override.toml")
   ```

The override file will be merged with `test-quaero.toml` automatically.

