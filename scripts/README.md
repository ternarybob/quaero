# Quaero Build and Deployment Scripts

This directory contains scripts for building, testing, and deploying Quaero.

## Available Scripts

### build.ps1 (Windows) / build.sh (Linux/Mac)

Build the Quaero application.

**Usage:**
```powershell
# Windows
.\scripts\build.ps1 [options]

# Linux/Mac
./scripts/build.sh [options]
```

**Options:**
- `-Environment <env>` - Target environment (dev, staging, prod)
- `-Version <ver>` - Version to embed in binary
- `-Clean` - Clean build artifacts before building
- `-Test` - Run tests before building
- `-Verbose` - Enable verbose output
- `-Release` - Build optimized release binary
- `-Run` - Run the application after successful build (Windows only)

**Examples:**
```powershell
# Basic build
.\scripts\build.ps1

# Clean build with tests
.\scripts\build.ps1 -Clean -Test

# Release build
.\scripts\build.ps1 -Release

# Build and run
.\scripts\build.ps1 -Run
```

### deploy.ps1 (Windows)

Deploy and manage the Quaero service.

**Usage:**
```powershell
.\scripts\deploy.ps1 -Target <target> [options]
```

**Targets:**
- `local` - Local development (default)
- `docker` - Docker container
- `production` - Production deployment

**Options:**
- `-ConfigPath <path>` - Custom config file path
- `-Build` - Build before deploying
- `-Stop` - Stop the running service
- `-Restart` - Restart the running service
- `-Status` - Show service status
- `-Logs` - Show service logs

**Examples:**
```powershell
# Deploy locally
.\scripts\deploy.ps1 -Target local

# Build and deploy to Docker
.\scripts\deploy.ps1 -Target docker -Build

# Check service status
.\scripts\deploy.ps1 -Status

# Stop service
.\scripts\deploy.ps1 -Stop

# Restart service
.\scripts\deploy.ps1 -Restart

# View logs
.\scripts\deploy.ps1 -Logs
```

## Build Output

Built artifacts are placed in the `bin/` directory:

```
bin/
├── quaero.exe       # Main executable (Windows)
├── quaero           # Main executable (Linux/Mac)
└── quaero.toml      # Configuration file (copied from deployments/)
```

## Version Management

The scripts automatically manage versioning using the `.version` file in the project root:

```
version: 0.1.0
build: 10-04-16-30-15
```

- **version**: Semantic version (auto-incremented on each build)
- **build**: Build timestamp

## Common Workflows

### Development

```powershell
# Build and run locally
.\scripts\build.ps1 -Run

# Or build and deploy
.\scripts\build.ps1
.\scripts\deploy.ps1 -Target local
```

### Testing

```powershell
# Build with tests
.\scripts\build.ps1 -Test

# Or run tests separately
.\test\run-tests.ps1 -Type all
```

### Release

```powershell
# Clean release build
.\scripts\build.ps1 -Clean -Release -Test

# Deploy to production
.\scripts\deploy.ps1 -Target production -Build
```

### Docker

```powershell
# Build and deploy to Docker
.\scripts\deploy.ps1 -Target docker -Build

# Check Docker status
.\scripts\deploy.ps1 -Target docker -Status

# View Docker logs
.\scripts\deploy.ps1 -Target docker -Logs
```

## Troubleshooting

### Build Fails

1. Ensure Go is installed and in PATH
2. Run `go mod tidy` to fix dependencies
3. Try a clean build: `.\scripts\build.ps1 -Clean`

### Tests Fail

1. Check test output for specific failures
2. Ensure test dependencies are installed
3. Run tests with verbose output: `.\scripts\build.ps1 -Test -Verbose`

### Deployment Issues

1. Verify executable exists: `ls bin/`
2. Check config file is valid
3. Ensure port 8080 is not in use
4. Check logs: `.\scripts\deploy.ps1 -Logs`

## See Also

- [Build Guide](../docs/BUILD.md) - Detailed build documentation
- [Deployment Guide](../deployments/README.md) - Deployment configurations
- [Testing Guide](../test/README.md) - Testing documentation
