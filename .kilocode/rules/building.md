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
