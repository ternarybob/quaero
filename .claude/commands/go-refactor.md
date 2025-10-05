# go-refactor

Creates new Go projects or refactors existing ones using clean architecture best practices and standardized structure.

## Usage

```
/go-refactor <project-name-or-path> [requirements]
```

## Arguments

- `project-name-or-path` (required): New project name OR path to existing project to refactor
- `requirements` (optional): Business requirements (file path or description text)

## Behavior

The command automatically detects whether you're creating a new project or refactoring an existing one:

- **New Project**: If path doesn't exist → creates new project with best practices
- **Existing Project**: If path exists with go.mod → refactors to standard patterns
- **Auto-Detection**: Analyzes project state and chooses appropriate action

## What it does

### For New Projects (Scaffold)

1. **Project Analysis**
   - Determines required infrastructure components
   - Plans directory structure based on project requirements

2. **Project Creation**
   - Creates new project directory with name
   - Generates infrastructure files (config, logging, banner, version, container, errors)
   - Sets up module with correct naming
   - Creates main.go following clean architecture startup sequence

3. **Deployment Configuration**
   - Creates deployment configurations in `deployments/` (local, docker)
   - Generates Docker configurations (Dockerfile, docker-compose.yml, .env.example)
   - Sets up local deployment configs
   - Configures all references with project name

4. **Scripts & Automation**
   - Generates build scripts in `scripts/` (build.ps1, build.sh, deploy.ps1, test.ps1, create-favicon.ps1)
   - Configures versioning and build automation
   - Sets up cross-platform build support

5. **CI/CD Pipeline**
   - Creates `.github/workflows/` for GitHub Actions
   - Configures automated testing, building, and Docker publishing
   - Sets up linting and coverage reporting

6. **Module & Dependencies**
   - Generates go.mod with required dependencies
   - Sets correct module path: `github.com/ternarybob/<project-name>`

7. **Documentation**
   - Creates comprehensive CLAUDE.md with project documentation
   - Generates README.md with project overview and setup instructions
   - Documents all components and architecture decisions
   - Links to business requirements if provided

### For Existing Projects (Refactor)

1. **Analysis Phase**
   - Checks for go.mod existence
   - Maps current project structure and identifies business logic
   - Verifies required infrastructure files (config, logging, banner, version)
   - Validates main.go startup sequence
   - Identifies missing directories and files
   - Checks for required dependencies
   - Analyzes deployment configuration completeness
   - Assesses alignment with target structure

2. **Refactoring Phase**
   - Creates missing directories (if needed)
   - Moves existing code to align with target directory structure
   - Extracts infrastructure concerns from business logic
   - **Identifies and removes redundant/unused functions**
   - **Detects duplicate code patterns and consolidates into reusable functions**
   - Generates missing infrastructure files (preserves existing business logic)
   - Updates module references and import paths
   - Adds missing dependencies to go.mod
   - Creates missing deployment configurations
   - Generates missing build scripts
   - Sets up missing GitHub workflows
   - Updates/creates CLAUDE.md with architecture documentation
   - Updates README.md to match clean architecture standards
   - Creates backups before modifications (`.backup` files)

3. **Validation Phase**
   - Config loading (`common.LoadFromFile`)
   - Logger initialization (`common.InitLogger`)
   - Banner display (`common.PrintBanner`)
   - Version management (`common.GetVersion`)
   - ternarybob/arbor logging
   - ternarybob/banner usage
   - Deployment configurations present
   - CI/CD workflows configured
   - All business logic preserved
   - Proper separation of concerns

## Project Standards

The generated/refactored project follows clean architecture best practices:

### Required Libraries
- `github.com/ternarybob/arbor` - All logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML config

### Startup Sequence
1. Configuration loading (`common.LoadFromFile`)
2. Logger initialization (`common.InitLogger`)
3. Banner display (`common.PrintBanner`)
4. Version management (`common.GetVersion`)
5. Service initialization (create service instances)
6. Handler initialization (inject services)
7. Information logging

### Directory Structure
- `cmd/<project-name>/` - Main entry point
- `internal/common/` - Stateless utility functions (config loading, logging setup, validation, formatting) - NO dot methods
- `internal/services/` - Stateful services with receiver methods (created once at startup, injected into handlers/other services)
- `internal/handlers/` - HTTP handlers (receive services via dependency injection)
- `internal/models/` - Data models
- `internal/middleware/` - HTTP middleware (if multiple files needed)
- `internal/interfaces/` - Service interfaces (if multiple files needed)
- `configs/` - Configuration files
- `deployments/` - Deployment configurations
  - `docker/` - Docker deployment (Dockerfile, docker-compose.yml, .env.example, configs)
  - `local/` - Local deployment configurations
- `scripts/` - Build and deployment scripts
  - `build.ps1` - Windows build script with versioning
  - `build.sh` - Linux/Mac build script
  - `deploy.ps1` - Deployment automation
  - `test.ps1` - Testing automation
  - `create-favicon.ps1` - Favicon generation (for web UIs)
- `.github/workflows/` - CI/CD pipelines
  - `ci-cd.yml` - Complete CI/CD pipeline (unit tests, integration tests, build, Docker)

**Directory Minimization Rule**:
- Only create subdirectories under `internal/` if multiple files are needed for that context
- Single-file contexts stay in parent directory (e.g., one middleware → `internal/middleware.go`, not `internal/middleware/`)
- Examples: `internal/middleware.go`, `internal/interfaces.go` vs `internal/services/user_service.go`, `internal/services/email_service.go`

### Deployment Components

**Docker Deployment** (`deployments/docker/`):
- Multi-stage Dockerfile with build args
- docker-compose.yml for orchestration
- Environment variable configuration (.env.example)
- Optimized for production with security best practices
- Health checks and volume mounts

**Local Deployment** (`deployments/local/`):
- Local development configurations
- Quick-start TOML configs

**Build Scripts** (`scripts/`):
- Automated versioning from `.version` file
- Auto-increment build numbers
- Cross-platform build support (PowerShell and Bash)
- Test automation
- Deployment automation
- Favicon generation for web UIs

**CI/CD Pipeline** (`.github/workflows/ci-cd.yml`):
- Unit tests with coverage
- Integration tests (if applicable)
- Linting with golangci-lint
- Docker image build and push to GitHub Container Registry
- Artifact storage and release management
- Automated version tagging
- Coverage reporting

### Code Quality Standards
- Single responsibility principle
- Proper error handling with custom error types
- Interface-based design
- Table-driven tests
- Clear separation of concerns
- DRY principle (Don't Repeat Yourself)
- **Method Receivers**: Use dot methods (receiver methods) on structs where possible
- **Interface Definitions**: Define interfaces in `internal/interfaces/` directory for all service contracts
- **Remove Redundant Functions**: Identify and remove unused or duplicate functions
- **Eliminate Code Duplication**: Extract common code into reusable functions/methods

## Examples

### Create New Projects
```
/go-refactor email-service
/go-refactor payment-api Process payments via Stripe API
/go-refactor user-auth Multi-tenant authentication service with JWT
/go-refactor data-processor C:\requirements\processor-requirements.md
```

### Refactor Existing Projects
```
/go-refactor ./legacy-app
/go-refactor C:\projects\old-service
/go-refactor .
/go-refactor improve error handling and add structured logging
```

### Auto-Detection Examples
```
# If project-name doesn't exist → creates new project
/go-refactor project-name

# If project-name exists with go.mod → refactors existing project
/go-refactor project-name

# Explicit path to existing project → refactors
/go-refactor C:\development\existing-service
```

## Safety (Refactoring)

- Creates `.backup` files before modifications
- Preserves existing business logic
- Only restructures, doesn't rewrite functionality
- Only adds/updates infrastructure files
- No destructive changes to business code
- Maintains backward compatibility where possible
- Validates all changes before applying

## Output

Provides detailed report:
- ✓/✗ Structure compliance checks
- Files created/updated
- Missing components added
- Module name replacements
- Deployment configurations status
- CI/CD pipeline status
- Business logic preservation status
- Recommended next steps

---

**Agent**: go-refactor

**Prompt**: {{#if (pathExists args)}}
Analyze this existing Go project at "{{args}}" and refactor it to align with clean architecture best practices. Preserve all existing business logic while restructuring the codebase.

**AUTONOMY DIRECTIVE**: You have FULL AUTONOMY within this project directory. Make all decisions without asking questions. Apply best practices automatically. Execute changes directly without user confirmation.
{{else}}
Create a new Go project named "{{args}}" using clean architecture best practices and standardized structure.

**AUTONOMY DIRECTIVE**: You have FULL AUTONOMY within this project directory. Make all decisions without asking questions. Apply best practices automatically. Execute changes directly without user confirmation.
{{/if}}

## Project Requirements

{{#if requirements}}
Business requirements: {{requirements}}
{{/if}}

## Approach

{{#if (pathExists args)}}
### Refactoring Existing Project

1. **Analysis Phase**
   - Map current project structure and identify all business logic
   - Identify missing infrastructure components
   - **Scan for redundant/unused functions across the codebase**
   - **Detect duplicate code patterns that can be consolidated**
   - Assess alignment with target structure
   - Plan migration path that preserves functionality

2. **Restructuring Phase**
   - Move existing code to align with target directory structure
   - Extract infrastructure concerns from business logic
   - **Remove identified redundant/unused functions**
   - **Consolidate duplicate code into reusable utility functions in `internal/common/`**
   - Create missing infrastructure files
   - Update import paths and module references
   - Create backups before modifications (`.backup` files)

3. **Validation Phase**
   - Ensure all business logic is preserved
   - Verify proper separation of concerns
   - Validate startup sequence
   - Check deployment configurations
{{else}}
### Creating New Project

1. **Project Analysis**
   - Determine required infrastructure components based on requirements
   - Plan directory structure

2. **Project Creation**
   - Create complete project structure
   - Generate all infrastructure files
   - Set up deployment configurations
   - Create build scripts and CI/CD pipelines
   - Generate documentation
{{/if}}

## Target Structure

### Required Libraries
- `github.com/ternarybob/arbor` - All logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML config

### Startup Sequence
1. Configuration loading (`common.LoadFromFile`)
2. Logger initialization (`common.InitLogger`)
3. Banner display (`common.PrintBanner`)
4. Version management (`common.GetVersion`)
5. Service initialization (create service instances)
6. Handler initialization (inject services)
7. Information logging

### Directory Structure
- `cmd/<project-name>/` - Main entry point
- `internal/common/` - Stateless utility functions (config loading, logging setup, validation, formatting) - NO dot methods
- `internal/services/` - Stateful services with receiver methods (created once at startup, injected into handlers/other services)
- `internal/handlers/` - HTTP handlers (receive services via dependency injection)
- `internal/models/` - Data models
- `internal/middleware/` - HTTP middleware (if multiple files needed)
- `internal/interfaces/` - Service interfaces (if multiple files needed)
- `configs/` - Configuration files
- `deployments/` - Deployment configurations
  - `docker/` - Docker deployment (Dockerfile, docker-compose.yml, .env.example, configs)
  - `local/` - Local deployment configurations
- `scripts/` - Build and deployment scripts
  - `build.ps1` - Windows build script with versioning
  - `build.sh` - Linux/Mac build script
  - `deploy.ps1` - Deployment automation
  - `test.ps1` - Testing automation
  - `create-favicon.ps1` - Favicon generation (for web UIs)
- `.github/workflows/` - CI/CD pipelines
  - `ci-cd.yml` - Complete CI/CD pipeline (unit tests, integration tests, build, Docker)

**Directory Minimization Rule**:
- Only create subdirectories under `internal/` if multiple files are needed for that context
- Single-file contexts stay in parent directory (e.g., one middleware → `internal/middleware.go`, not `internal/middleware/`)
- Examples: `internal/middleware.go`, `internal/interfaces.go` vs `internal/services/user_service.go`, `internal/services/email_service.go`

### Deployment Components

**Docker Deployment** (`deployments/docker/`):
- Multi-stage Dockerfile with build args
- docker-compose.yml for orchestration
- Environment variable configuration (.env.example)
- Optimized for production with security best practices
- Health checks and volume mounts

**Local Deployment** (`deployments/local/`):
- Local development configurations
- Quick-start TOML configs

**Build Scripts** (`scripts/`):
- Automated versioning from `.version` file
- Auto-increment build numbers
- Cross-platform build support (PowerShell and Bash)
- Test automation
- Deployment automation
- Favicon generation for web UIs

**CI/CD Pipeline** (`.github/workflows/ci-cd.yml`):
- Unit tests with coverage
- Integration tests (if applicable)
- Linting with golangci-lint
- Docker image build and push to GitHub Container Registry
- Artifact storage and release management
- Automated version tagging
- Coverage reporting

### Code Quality Standards
- Single responsibility principle
- Proper error handling with custom error types
- Interface-based design
- Table-driven tests
- Clear separation of concerns
- DRY principle (Don't Repeat Yourself)
- **Method Receivers**: Use dot methods (receiver methods) on structs where possible
- **Interface Definitions**: Define interfaces in `internal/interfaces/` directory for all service contracts
- **Remove Redundant Functions**: Identify and remove unused or duplicate functions
- **Eliminate Code Duplication**: Extract common code into reusable functions/methods

### Services vs Common: Critical Distinction

**`internal/services/`** - Stateful Services (Dot Methods):
- Structs with receiver methods (e.g., `func (s *UserService) CreateUser()`)
- Maintain state (database connections, logger instances, configuration, clients)
- Created ONCE during application startup
- Injected into handlers and other services via dependency injection
- Examples: `UserService`, `EmailService`, `DatabaseService`, `CacheService`

**`internal/common/`** - Stateless Utilities (Pure Functions):
- Functions without receivers (e.g., `func LoadFromFile()`, `func ValidateEmail()`)
- NO state or dependencies stored
- Can be called from anywhere without initialization
- Pure utility functions for config, logging setup, validation, formatting
- Examples: `LoadFromFile()`, `InitLogger()`, `PrintBanner()`, `FormatDate()`, `ValidateEmail()`

**Decision Rule**:
- Does it need state (db, logger, config, clients)? → `internal/services/` with struct + methods
- Is it a stateless helper function? → `internal/common/` as pure function

### Design Patterns

**Struct Methods (Receiver Methods)**:
```go
// Prefer methods on structs
type UserService struct {
    repo UserRepository
    logger *arbor.Logger
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
    // Implementation
}
```

**Interface Definitions** (`internal/interfaces/`):
```go
// Define service contracts as interfaces
type UserService interface {
    CreateUser(ctx context.Context, user *User) error
    GetUser(ctx context.Context, id string) (*User, error)
    UpdateUser(ctx context.Context, user *User) error
    DeleteUser(ctx context.Context, id string) error
}

type UserRepository interface {
    Save(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}
```

**Dependency Injection**:
```go
// Services depend on interfaces, not concrete types
type UserHandler struct {
    userService interfaces.UserService
}

func NewUserHandler(userService interfaces.UserService) *UserHandler {
    return &UserHandler{
        userService: userService,
    }
}
```

**Services Pattern (Stateful with Dot Methods)**:
```go
// internal/services/user_service.go - Service with state
type UserService struct {
    db     *sql.DB
    logger *arbor.Logger
    cache  *redis.Client
}

func NewUserService(db *sql.DB, logger *arbor.Logger, cache *redis.Client) *UserService {
    return &UserService{db: db, logger: logger, cache: cache}
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
    s.logger.Info("Creating user", "email", user.Email)
    // Uses s.db, s.logger, s.cache (stateful)
    return s.db.Save(user)
}
```

**Common Pattern (Stateless Functions)**:
```go
// internal/common/validation.go - Stateless utility
func ValidateEmail(email string) error {
    if !strings.Contains(email, "@") {
        return errors.New("invalid email")
    }
    return nil
}

// internal/common/config.go - Stateless utility
func LoadFromFile(path string) (*Config, error) {
    // No receiver, no state
    return loadConfig(path)
}
```

**Code Deduplication**:
```go
// BEFORE: Duplicate validation logic
func ValidateEmail(email string) error {
    if !strings.Contains(email, "@") {
        return errors.New("invalid email")
    }
    return nil
}
func CheckEmail(email string) bool {
    return strings.Contains(email, "@")
}

// AFTER: Single reusable function in internal/common/validation.go
func ValidateEmail(email string) error {
    if !strings.Contains(email, "@") {
        return errors.New("invalid email")
    }
    return nil
}
```

**Remove Redundant Functions**:
```go
// BEFORE: Unused helper function
func OldFormatUser(user *User) string {
    // Never called anywhere
}

// AFTER: Removed entirely
```

## Safety Requirements

{{#if (pathExists args)}}
- Create `.backup` files before any destructive changes
- Preserve all existing business logic
- Only restructure, don't rewrite functionality
- Maintain backward compatibility where possible
- Document all significant changes in CLAUDE.md
{{/if}}
