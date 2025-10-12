# -----------------------------------------------------------------------
# Build Script for Quaero
# -----------------------------------------------------------------------

param (
    [string]$Environment = "dev",
    [string]$Version = "",
    [switch]$Clean,
    [switch]$Test,
    [switch]$Verbose,
    [switch]$Release,
    [switch]$Run
)

<#
.SYNOPSIS
    Build script for Quaero

.DESCRIPTION
    This script builds Quaero for local development and testing.
    Outputs the executable to the project's bin directory.

.PARAMETER Environment
    Target environment for build (dev, staging, prod)

.PARAMETER Version
    Version to embed in the binary (defaults to .version file or git commit hash)

.PARAMETER Clean
    Clean build artifacts before building

.PARAMETER Test
    Run tests before building

.PARAMETER Verbose
    Enable verbose output

.PARAMETER Release
    Build optimized release binary

.PARAMETER Run
    Run the application in a new terminal after successful build

.EXAMPLE
    .\build.ps1
    Build quaero for development

.EXAMPLE
    .\build.ps1 -Release
    Build optimized release version

.EXAMPLE
    .\build.ps1 -Environment prod -Version "1.0.0"
    Build for production with specific version

.EXAMPLE
    .\build.ps1 -Run
    Build and run the application in a new terminal
#>

# Error handling
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# Build configuration
$gitCommit = ""

try {
    $gitCommit = git rev-parse --short HEAD 2>$null
    if (-not $gitCommit) { $gitCommit = "unknown" }
}
catch {
    $gitCommit = "unknown"
}

Write-Host "Quaero Build Script" -ForegroundColor Cyan
Write-Host "===================" -ForegroundColor Cyan

# Setup paths
$scriptDir = $PSScriptRoot
$projectRoot = Split-Path -Parent $scriptDir
$versionFilePath = Join-Path -Path $projectRoot -ChildPath ".version"
$binDir = Join-Path -Path $projectRoot -ChildPath "bin"
$outputPath = Join-Path -Path $binDir -ChildPath "quaero.exe"

Write-Host "Project Root: $projectRoot" -ForegroundColor Gray
Write-Host "Environment: $Environment" -ForegroundColor Gray
Write-Host "Git Commit: $gitCommit" -ForegroundColor Gray

# Handle version file creation and maintenance
$buildTimestamp = Get-Date -Format "MM-dd-HH-mm-ss"

if (-not (Test-Path $versionFilePath)) {
    # Create .version file if it doesn't exist
    $versionContent = @"
version: 0.1.0
build: $buildTimestamp
"@
    Set-Content -Path $versionFilePath -Value $versionContent
    Write-Host "Created .version file with version 0.1.0" -ForegroundColor Green
} else {
    # Read current version and increment patch version
    $versionLines = Get-Content $versionFilePath
    $currentVersion = ""
    $updatedLines = @()

    foreach ($line in $versionLines) {
        if ($line -match '^version:\s*(.+)$') {
            $currentVersion = $matches[1].Trim()

            # Parse version (format: major.minor.patch)
            if ($currentVersion -match '^(\d+)\.(\d+)\.(\d+)$') {
                $major = [int]$matches[1]
                $minor = [int]$matches[2]
                $patch = [int]$matches[3]

                # Increment patch version
                $patch++
                $newVersion = "$major.$minor.$patch"

                $updatedLines += "version: $newVersion"
                Write-Host "Incremented version: $currentVersion -> $newVersion" -ForegroundColor Green
            } else {
                $updatedLines += $line
                Write-Host "Version format not recognized, keeping: $currentVersion" -ForegroundColor Yellow
            }
        } elseif ($line -match '^build:\s*') {
            $updatedLines += "build: $buildTimestamp"
        } else {
            $updatedLines += $line
        }
    }

    Set-Content -Path $versionFilePath -Value $updatedLines
    Write-Host "Updated build timestamp to: $buildTimestamp" -ForegroundColor Green
}

# Read version information from .version file
$versionInfo = @{}
$versionLines = Get-Content $versionFilePath
foreach ($line in $versionLines) {
    if ($line -match '^version:\s*(.+)$') {
        $versionInfo.Version = $matches[1].Trim()
    }
    if ($line -match '^build:\s*(.+)$') {
        $versionInfo.Build = $matches[1].Trim()
    }
}

Write-Host "Using version: $($versionInfo.Version), build: $($versionInfo.Build)" -ForegroundColor Cyan

# Clean build artifacts if requested
if ($Clean) {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Yellow
    if (Test-Path $binDir) {
        Remove-Item -Path $binDir -Recurse -Force
    }
    if (Test-Path "go.sum") {
        Remove-Item -Path "go.sum" -Force
    }
}

# Create bin directory
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

# Run tests if requested
if ($Test) {
    Write-Host "Running tests..." -ForegroundColor Yellow
    $testScript = Join-Path -Path $projectRoot -ChildPath "test\run-tests.ps1"

    if (Test-Path $testScript) {
        & $testScript -Type all
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Tests failed!" -ForegroundColor Red
            exit 1
        }
    } else {
        go test ./... -v
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Tests failed!" -ForegroundColor Red
            exit 1
        }
    }
    Write-Host "Tests passed!" -ForegroundColor Green
}

# Stop executing process if it's running (graceful shutdown with fallback)
try {
    $processName = "quaero"
    $processes = Get-Process -Name $processName -ErrorAction SilentlyContinue

    if ($processes) {
        Write-Host "Stopping existing Quaero process(es)..." -ForegroundColor Yellow
        
        # Try HTTP shutdown first (most reliable on Windows)
        $httpShutdownSucceeded = $false
        
        # Read port from config
        $configPath = Join-Path -Path $binDir -ChildPath "quaero.toml"
        $serverPort = 8085  # Default
        if (Test-Path $configPath) {
            $configContent = Get-Content $configPath
            foreach ($line in $configContent) {
                if ($line -match '^port\s*=\s*(\d+)') {
                    $serverPort = [int]$matches[1]
                    break
                }
            }
        }
        
        Write-Host "  Attempting HTTP graceful shutdown on port $serverPort..." -ForegroundColor Gray
        
        # Try multiple times with short delays (server might still be starting)
        $maxAttempts = 3
        for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
            try {
                $response = Invoke-WebRequest -Uri "http://localhost:$serverPort/api/shutdown" -Method POST -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
                if ($response.StatusCode -eq 200) {
                    Write-Host "  HTTP shutdown request sent successfully" -ForegroundColor Gray
                    $httpShutdownSucceeded = $true
                    break
                }
            }
            catch {
                if ($attempt -lt $maxAttempts) {
                    Start-Sleep -Milliseconds 500
                } else {
                    Write-Host "  HTTP shutdown not available (server may not be responding)" -ForegroundColor Gray
                }
            }
        }
        
        # Wait for graceful shutdown
        $timeout = if ($httpShutdownSucceeded) { 12 } else { 5 }
        $elapsed = 0
        $checkInterval = 0.5
        
        while ((Get-Process -Name $processName -ErrorAction SilentlyContinue) -and ($elapsed -lt $timeout)) {
            Start-Sleep -Seconds $checkInterval
            $elapsed += $checkInterval
            
            if ($httpShutdownSucceeded -and $elapsed -eq 5) {
                Write-Host "  Still waiting for graceful shutdown..." -ForegroundColor Gray
            }
        }

        # Check if processes exited gracefully
        $remainingProcesses = Get-Process -Name $processName -ErrorAction SilentlyContinue
        
        if ($remainingProcesses) {
            if ($httpShutdownSucceeded) {
                Write-Warning "Process(es) did not exit gracefully within ${timeout}s, forcing termination..."
            }
            Stop-Process -Name $processName -Force -ErrorAction SilentlyContinue
            Start-Sleep -Milliseconds 500
            
            if (Get-Process -Name $processName -ErrorAction SilentlyContinue) {
                Write-Warning "Some processes may still be running"
            } else {
                Write-Host "Process(es) force-stopped" -ForegroundColor Yellow
            }
        } else {
            Write-Host "Process(es) stopped gracefully" -ForegroundColor Green
        }
    } else {
        Write-Host "No Quaero process found running" -ForegroundColor Gray
    }
}
catch {
    Write-Warning "Could not stop Quaero process: $($_.Exception.Message)"
}

# Clean up any llama-server processes (spawned by quaero)
try {
    Write-Host "Checking for llama-server processes..." -ForegroundColor Yellow
    $llamaProcesses = Get-Process -Name "llama-server" -ErrorAction SilentlyContinue

    if ($llamaProcesses) {
        Write-Host "  Found $($llamaProcesses.Count) llama-server process(es), stopping..." -ForegroundColor Gray

        foreach ($proc in $llamaProcesses) {
            try {
                $proc.Kill()
                Write-Host "  Stopped llama-server (PID: $($proc.Id))" -ForegroundColor Gray
            }
            catch {
                Write-Warning "  Failed to stop llama-server (PID: $($proc.Id)): $($_.Exception.Message)"
            }
        }

        # Wait briefly for processes to exit
        Start-Sleep -Milliseconds 500

        # Verify cleanup
        $remainingLlama = Get-Process -Name "llama-server" -ErrorAction SilentlyContinue
        if ($remainingLlama) {
            Write-Warning "Some llama-server processes may still be running"
        } else {
            Write-Host "  All llama-server processes stopped successfully" -ForegroundColor Green
        }
    } else {
        Write-Host "  No llama-server processes found" -ForegroundColor Gray
    }
}
catch {
    Write-Warning "Could not check/stop llama-server processes: $($_.Exception.Message)"
}

# Tidy and download dependencies
Write-Host "Tidying dependencies..." -ForegroundColor Yellow
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to tidy dependencies!" -ForegroundColor Red
    exit 1
}

Write-Host "Downloading dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to download dependencies!" -ForegroundColor Red
    exit 1
}

# Build flags
$module = "github.com/ternarybob/quaero/internal/common"
$buildFlags = @(
    "-X", "$module.Version=$($versionInfo.Version)",
    "-X", "$module.Build=$($versionInfo.Build)",
    "-X", "$module.GitCommit=$gitCommit"
)

if ($Release) {
    $buildFlags += @("-w", "-s")  # Strip debug info and symbol table
}

$ldflags = $buildFlags -join " "

# Build command
Write-Host "Building quaero..." -ForegroundColor Yellow

# Disable CGO - using pure Go SQLite (modernc.org/sqlite)
$env:CGO_ENABLED = "0"
if ($Release) {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
}

$buildArgs = @(
    "build"
    "-ldflags=$ldflags"
    "-o", $outputPath
    ".\cmd\quaero"
)

# Change to project root for build
Push-Location $projectRoot

if ($Verbose) {
    $buildArgs += "-v"
}

Write-Host "Build command: go $($buildArgs -join ' ')" -ForegroundColor Gray

& go @buildArgs

# Return to original directory
Pop-Location

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

# Copy configuration file to bin directory
$configSourcePath = Join-Path -Path $projectRoot -ChildPath "deployments\local\quaero.toml"
$configDestPath = Join-Path -Path $binDir -ChildPath "quaero.toml"

if (Test-Path $configSourcePath) {
    if (-not (Test-Path $configDestPath)) {
        Copy-Item -Path $configSourcePath -Destination $configDestPath
        Write-Host "Copied configuration: deployments/local/quaero.toml -> bin/" -ForegroundColor Green
    } else {
        Write-Host "Using existing bin/quaero.toml (preserving customizations)" -ForegroundColor Cyan
    }
}

# Copy Chrome extension to bin directory
$extensionSourcePath = Join-Path -Path $projectRoot -ChildPath "cmd\quaero-chrome-extension"
$extensionDestPath = Join-Path -Path $binDir -ChildPath "quaero-chrome-extension"

if (Test-Path $extensionSourcePath) {
    if (Test-Path $extensionDestPath) {
        Remove-Item -Path $extensionDestPath -Recurse -Force
    }
    Copy-Item -Path $extensionSourcePath -Destination $extensionDestPath -Recurse
    Write-Host "Deployed Chrome extension: cmd/quaero-chrome-extension -> bin/" -ForegroundColor Green
}

# Generate favicon if it doesn't exist
$faviconPath = Join-Path -Path $projectRoot -ChildPath "pages\static\favicon.ico"
if (-not (Test-Path $faviconPath)) {
    Write-Host "Generating favicon..." -ForegroundColor Yellow
    $createFaviconScript = Join-Path -Path $projectRoot -ChildPath "scripts\create-favicon.ps1"
    if (Test-Path $createFaviconScript) {
        & $createFaviconScript
    } else {
        Write-Warning "Favicon script not found: $createFaviconScript"
    }
}

# Copy pages directory to bin
$pagesSourcePath = Join-Path -Path $projectRoot -ChildPath "pages"
$pagesDestPath = Join-Path -Path $binDir -ChildPath "pages"

if (Test-Path $pagesSourcePath) {
    if (Test-Path $pagesDestPath) {
        Remove-Item -Path $pagesDestPath -Recurse -Force
    }
    Copy-Item -Path $pagesSourcePath -Destination $pagesDestPath -Recurse
    Write-Host "Deployed web pages: pages -> bin/" -ForegroundColor Green
}

# Copy MCP client to bin
$mcpSourcePath = Join-Path -Path $projectRoot -ChildPath "mcp-client"
$mcpDestPath = Join-Path -Path $binDir -ChildPath "mcp-client"

if (Test-Path $mcpSourcePath) {
    if (Test-Path $mcpDestPath) {
        Remove-Item -Path $mcpDestPath -Recurse -Force
    }
    Copy-Item -Path $mcpSourcePath -Destination $mcpDestPath -Recurse
    Write-Host "Deployed MCP client: mcp-client -> bin/" -ForegroundColor Green

    # Generate MCP configuration files
    $proxyPath = Join-Path -Path $mcpDestPath -ChildPath "proxy.js"
    $proxyPath = $proxyPath -replace '\\', '/'

    # LM Studio configuration
    $lmStudioConfig = @{
        mcpServers = @{
            quaero = @{
                command = "node"
                args = @($proxyPath)
                env = @{
                    QUAERO_URL = "http://localhost:8085"
                }
            }
        }
    } | ConvertTo-Json -Depth 10

    $lmStudioConfigPath = Join-Path -Path $mcpDestPath -ChildPath "lmstudio-config.json"
    $lmStudioConfig | Set-Content -Path $lmStudioConfigPath -Encoding UTF8

    # Claude Desktop configuration
    $claudeConfig = @{
        mcpServers = @{
            quaero = @{
                command = "node"
                args = @($proxyPath)
                env = @{
                    QUAERO_URL = "http://localhost:8085"
                }
            }
        }
    } | ConvertTo-Json -Depth 10

    $claudeConfigPath = Join-Path -Path $mcpDestPath -ChildPath "claude-desktop-config.json"
    $claudeConfig | Set-Content -Path $claudeConfigPath -Encoding UTF8

    Write-Host "Generated MCP configurations:" -ForegroundColor Green
    Write-Host "  - lmstudio-config.json" -ForegroundColor Gray
    Write-Host "  - claude-desktop-config.json" -ForegroundColor Gray
}

# Verify executable was created
if (-not (Test-Path $outputPath)) {
    Write-Error "Build completed but executable not found: $outputPath"
    exit 1
}

# Get file info for binary
$fileInfo = Get-Item $outputPath
$fileSizeMB = [math]::Round($fileInfo.Length / 1MB, 2)

Write-Host "`n==== Build Summary ====" -ForegroundColor Cyan
Write-Host "Status: SUCCESS" -ForegroundColor Green
Write-Host "Environment: $Environment" -ForegroundColor Green
Write-Host "Version: $($versionInfo.Version)" -ForegroundColor Green
Write-Host "Build: $($versionInfo.Build)" -ForegroundColor Green
Write-Host "Output: $outputPath ($fileSizeMB MB)" -ForegroundColor Green
Write-Host "Build Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Green

if ($Test) {
    Write-Host "Tests: EXECUTED" -ForegroundColor Green
}

if ($Clean) {
    Write-Host "Clean: EXECUTED" -ForegroundColor Green
}

Write-Host "`nBuild completed successfully!" -ForegroundColor Green
Write-Host "Executable: $outputPath" -ForegroundColor Cyan

# Run application if -Run flag is set
if ($Run) {
    Write-Host "`n==== Starting Application ====" -ForegroundColor Yellow

    # Use bin config (already deployed from deployments/local/)
    $configPath = Join-Path -Path $binDir -ChildPath "quaero.toml"

    # Start in a new terminal window with serve command
    # Use /k to KEEP window open so Ctrl+C signal propagates correctly
    # This allows proper graceful shutdown via Ctrl+C
    $startCommand = "cd /d `"$binDir`" && `"$outputPath`" serve -c `"$configPath`""

    Start-Process cmd -ArgumentList "/k", $startCommand

    Write-Host "Application started in new terminal window" -ForegroundColor Green
    Write-Host "Command: quaero.exe serve -c quaero.toml" -ForegroundColor Cyan
    Write-Host "Config: bin\quaero.toml" -ForegroundColor Gray
    Write-Host "Press Ctrl+C in the server window to stop gracefully" -ForegroundColor Yellow
    Write-Host "Check bin\logs\ for application logs" -ForegroundColor Yellow
} else {
    Write-Host "`nTo run with local config:" -ForegroundColor Yellow
    Write-Host "./bin/quaero.exe serve -c quaero.toml" -ForegroundColor White
}
