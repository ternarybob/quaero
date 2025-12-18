# ARCHITECT ANALYSIS

## Task Summary

1. **Emailer credentials protection** - Remove any code protecting email credentials from reset_on_startup (user says it's not working anyway)
2. **Remove ./connectors directory** - Change default connector location to ./ (exe location)
3. **Change connector TOML loading** - Look for single `./connectors.toml` file instead of all TOML files in directory
4. **Create email.toml loading** - New config file for email settings, similar to connectors.toml

## Codebase Analysis

### 1. Emailer Credentials (Requirement 1)

**Current State:**
- `internal/services/mailer/service.go:38` has comment "survives reset_on_startup via variables.toml"
- This is just a comment - no special protection code exists
- Email config is stored in KV storage with `smtp_` prefix keys
- When `reset_on_startup=true`, BadgerDB wipes ALL data including KV storage
- The intended behavior is that users put SMTP settings in `variables/variables.toml`

**Action:** No code needs removal. The comment is informational - it tells users to put credentials in variables.toml. The protection "not working" is expected since there's no protection code. Leave as-is.

### 2. Connector Directory Path (Requirement 2)

**Current State:**
- `internal/common/config.go:213-215`: Default is `./connectors`
  ```go
  Connectors: ConnectorDirConfig{
      Dir: "./connectors", // Default directory for connector files
  }
  ```
- `internal/common/config.go:97-99`: Config struct definition
- `internal/app/app.go:279`: Loads from `a.Config.Connectors.Dir`

**Action:** Change default from `"./connectors"` to `"./"` in config.go

### 3. Connector TOML Loading (Requirement 3)

**Current State:**
- `internal/storage/badger/load_connectors.go:30-178`: `LoadConnectorsFromFiles()` function
- Currently iterates all `.toml` files in directory
- Uses `os.ReadDir(dirPath)` and filters by `.toml` extension

**Action:**
- Change to look for single file `connectors.toml` in the directory path
- Since we're changing dir to `./`, the file will be `./connectors.toml`
- Simplify logic to read single file instead of iterating directory

### 4. Email TOML Loading (Requirement 4)

**Current State:**
- No email.toml loading exists
- `internal/services/mailer/service.go` reads from KV storage only
- Email config keys: `smtp_host`, `smtp_port`, `smtp_username`, `smtp_password`, `smtp_from`, `smtp_from_name`, `smtp_use_tls`

**Action:**
- Create `LoadEmailFromFile()` in `internal/storage/badger/` (following connector pattern)
- Add to StorageManager interface and implementation
- Call from `app.go:initDatabase()` like connectors
- Support variable substitution with `{variable_name}` syntax
- File format: single `[email]` or `[smtp]` section with smtp_* keys

## ANTI-CREATION BIAS CHECK

### Existing Code to Extend
1. **load_connectors.go** - Modify to use single file instead of directory scanning
2. **config.go** - Change default connector path value
3. **manager.go** - Add `LoadEmailFromFile()` method (EXTEND)
4. **app.go** - Add call to load email config (EXTEND)
5. **interfaces/storage.go** - Add `LoadEmailFromFile()` interface method (EXTEND)

### New Files Required
- `internal/storage/badger/load_email.go` - New file following load_connectors.go pattern

**Justification:** New file follows exact pattern from existing `load_connectors.go`. Cannot extend load_connectors.go as it handles different data type (connectors vs email config).

## Implementation Order

1. Modify `internal/common/config.go` - Change connector default path
2. Modify `internal/storage/badger/load_connectors.go` - Single file loading
3. Create `internal/storage/badger/load_email.go` - Email config loading
4. Modify `internal/interfaces/storage.go` - Add interface method
5. Modify `internal/storage/badger/manager.go` - Implement interface method
6. Modify `internal/app/app.go` - Call email loading at startup

## Build Requirement

Platform: Linux (WSL)
Build Command: `./scripts/build.sh`

BUILD FAIL = TASK FAIL (no exceptions)
