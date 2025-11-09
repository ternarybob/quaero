# -----------------------------------------------------------------------
# Build Script for Quaero
# -----------------------------------------------------------------------
# Simplified: 2025-11-08
# Removed backward compatibility parameters (-Clean, -Verbose, -Release,
# -ResetDatabase, -Environment, -Version)
# See docs/simplify-build-script/ for migration guide
# -----------------------------------------------------------------------

param (
    [switch]$Run,
    [switch]$Deploy
)

<#
.SYNOPSIS
    Build script for Quaero

.DESCRIPTION
    This script builds Quaero for local development and testing.

    Three operations supported:
    1. Default build (no parameters) - Builds executable silently, no deployment
    2. -Deploy - Builds and deploys all files to bin directory (stops service if running)
    3. -Run - Builds, deploys, and starts application in new terminal

.PARAMETER Deploy
    Deploy all required files to bin directory after building (config, pages, Chrome extension, job definitions)
    Stops any running service before deployment

.PARAMETER Run
    Build, deploy, and run the application in a new terminal
    Automatically triggers deployment before starting the service

.EXAMPLE
    .\build.ps1
    Build quaero executable only (no deployment, silent on success)

.EXAMPLE
    .\build.ps1 -Deploy
    Build and deploy all files to bin directory (stops service if running)

.EXAMPLE
    .\build.ps1 -Run
    Build, deploy, and start the application in a new terminal

.NOTES
    Default build operation does NOT increment version number, only updates build timestamp.
    Version number must be manually incremented in .version file when needed.

    For advanced operations removed in simplification (clean, database reset, etc.),
    see docs/simplify-build-script/migration-guide.md
#>

# Error handling
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# --- Logging Setup ---
$logDir = "$PSScriptRoot/logs"
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir | Out-Null
}
$logFile = "$logDir/build-$(Get-Date -Format 'yyyy-MM-dd-HH-mm-ss').log"

# Function to limit log files to most recent 10
function Limit-LogFiles {
    param(
        [string]$LogDirectory,
        [int]$MaxLogs = 10
    )
    
    $logFiles = Get-ChildItem -Path $LogDirectory -Filter "build-*.log" | Sort-Object CreationTime -Descending
    
    if (@($logFiles).Count -gt $MaxLogs) {
        $filesToDelete = $logFiles | Select-Object -Skip $MaxLogs
        foreach ($file in $filesToDelete) {
            Remove-Item -Path $file.FullName -Force
            Write-Host "Removed old log file: $($file.Name)" -ForegroundColor Gray
        }
    }
}

# Limit old log files before starting transcript
Limit-LogFiles -LogDirectory $logDir -MaxLogs 10

Start-Transcript -Path $logFile -Append

try {

# ========== HELPER FUNCTIONS ==========

# Function to get server port from config file
function Get-ServerPort {
    param(
        [string]$BinDirectory
    )

    $configPath = Join-Path -Path $BinDirectory -ChildPath "quaero.toml"
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

    return $serverPort
}

# Function to stop Quaero service gracefully
function Stop-QuaeroService {
    param(
        [int]$Port
    )

    try {
        $processName = "quaero"
        $processes = Get-Process -Name $processName -ErrorAction SilentlyContinue

        if ($processes) {
            Write-Host "Stopping existing Quaero process(es)..." -ForegroundColor Yellow

            # Try HTTP shutdown first
            $httpShutdownSucceeded = $false

            Write-Host "  Attempting HTTP graceful shutdown on port $Port..." -ForegroundColor Gray

            # Try multiple times with short delays
            $maxAttempts = 3
            for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
                try {
                    $response = Invoke-WebRequest -Uri "http://localhost:$Port/api/shutdown" -Method POST -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
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
}

# Function to stop all llama-server processes
function Stop-LlamaServers {
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
}

# Function to deploy files to bin directory
function Deploy-Files {
    param(
        [string]$ProjectRoot,
        [string]$BinDirectory
    )

    # Deploy configuration file (only if not exists)
    $configSourcePath = Join-Path -Path $ProjectRoot -ChildPath "deployments\local\quaero.toml"
    $configDestPath = Join-Path -Path $BinDirectory -ChildPath "quaero.toml"

    if (Test-Path $configSourcePath) {
        if (-not (Test-Path $configDestPath)) {
            Copy-Item -Path $configSourcePath -Destination $configDestPath
        }
    }

    # Deploy project README to bin root
    $projectReadmePath = Join-Path -Path $ProjectRoot -ChildPath "README.md"
    if (Test-Path $projectReadmePath) {
        $binReadmePath = Join-Path -Path $BinDirectory -ChildPath "README.md"
        Copy-Item -Path $projectReadmePath -Destination $binReadmePath -Force
    }

    # Deploy Chrome extension
    $extensionSourcePath = Join-Path -Path $ProjectRoot -ChildPath "cmd\quaero-chrome-extension"
    $extensionDestPath = Join-Path -Path $BinDirectory -ChildPath "quaero-chrome-extension"

    if (Test-Path $extensionSourcePath) {
        if (Test-Path $extensionDestPath) {
            Remove-Item -Path $extensionDestPath -Recurse -Force
        }
        Copy-Item -Path $extensionSourcePath -Destination $extensionDestPath -Recurse
    }

    # Deploy MCP server documentation
    $mcpSourcePath = Join-Path -Path $ProjectRoot -ChildPath "cmd\quaero-mcp"
    $mcpDestPath = Join-Path -Path $BinDirectory -ChildPath "quaero-mcp"

    if (Test-Path $mcpSourcePath) {
        # MCP directory and executable already created during build
        # Copy MCP-specific README as the directory's README.md
        $mcpReadmePath = Join-Path -Path $mcpSourcePath -ChildPath "README.md"
        if (Test-Path $mcpReadmePath) {
            $destReadme = Join-Path -Path $mcpDestPath -ChildPath "README.md"
            Copy-Item -Path $mcpReadmePath -Destination $destReadme -Force
        }
    }

    # Deploy pages directory
    $pagesSourcePath = Join-Path -Path $ProjectRoot -ChildPath "pages"
    $pagesDestPath = Join-Path -Path $BinDirectory -ChildPath "pages"

    if (Test-Path $pagesSourcePath) {
        if (Test-Path $pagesDestPath) {
            Remove-Item -Path $pagesDestPath -Recurse -Force
        }
        Copy-Item -Path $pagesSourcePath -Destination $pagesDestPath -Recurse
    }

    # Deploy job-definitions directory (only new files, no override)
    $jobDefsSourcePath = Join-Path -Path $ProjectRoot -ChildPath "deployments\local\job-definitions"
    $jobDefsDestPath = Join-Path -Path $BinDirectory -ChildPath "job-definitions"

    if (Test-Path $jobDefsSourcePath) {
        if (-not (Test-Path $jobDefsDestPath)) {
            New-Item -ItemType Directory -Path $jobDefsDestPath -Force | Out-Null
        }

        # Copy files without overriding existing ones
        $sourceFiles = Get-ChildItem -Path $jobDefsSourcePath -File
        $copiedCount = 0
        $skippedCount = 0

        foreach ($file in $sourceFiles) {
            $destFile = Join-Path -Path $jobDefsDestPath -ChildPath $file.Name
            if (-not (Test-Path $destFile)) {
                Copy-Item -Path $file.FullName -Destination $destFile
                $copiedCount++
            } else {
                $skippedCount++
            }
        }

    }
}

# ========== END HELPER FUNCTIONS ==========

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
    # Read current version and update ONLY build timestamp (no version increment)
    $versionLines = Get-Content $versionFilePath
    $updatedLines = @()

    foreach ($line in $versionLines) {
        if ($line -match '^version:\s*(.+)$') {
            # Keep version as-is
            $updatedLines += $line
        } elseif ($line -match '^build:\s*') {
            # Update build timestamp
            $updatedLines += "build: $buildTimestamp"
        } else {
            $updatedLines += $line
        }
    }

    Set-Content -Path $versionFilePath -Value $updatedLines
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

# Create bin directory
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

# Stop services if running (using helper functions)
$serverPort = Get-ServerPort -BinDirectory $binDir
Stop-QuaeroService -Port $serverPort
Stop-LlamaServers

# Tidy and download dependencies
Write-Host "Tidying dependencies..." -ForegroundColor Yellow
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to tidy dependencies!" -ForegroundColor Red
    Stop-Transcript
    exit 1
}

Write-Host "Downloading dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to download dependencies!" -ForegroundColor Red
    Stop-Transcript
    exit 1
}

# Build flags (standard build - no conditional logic)
$module = "github.com/ternarybob/quaero/internal/common"
$ldflags = "-X $module.Version=$($versionInfo.Version) -X $module.Build=$($versionInfo.Build) -X $module.GitCommit=$gitCommit"

# Build the Go application
Write-Host "Building quaero..." -ForegroundColor Yellow

$buildArgs = @(
    "build"
    "-ldflags=$ldflags"
    "-o", $outputPath
    ".\cmd\quaero"
)

# Change to project root for build
Push-Location $projectRoot

Write-Host "Build command: go $($buildArgs -join ' ')" -ForegroundColor Gray

& go @buildArgs

# Return to original directory
Pop-Location

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    Stop-Transcript
    exit 1
}

# Verify executable was created
if (-not (Test-Path $outputPath)) {
    Write-Error "Build completed but executable not found: $outputPath"
    Stop-Transcript
    exit 1
}

# Build MCP server
Write-Host "Building quaero-mcp..." -ForegroundColor Yellow

# Create MCP directory if it doesn't exist
$mcpDir = Join-Path -Path $binDir -ChildPath "quaero-mcp"
if (-not (Test-Path $mcpDir)) {
    New-Item -ItemType Directory -Path $mcpDir | Out-Null
}

$mcpOutputPath = Join-Path -Path $mcpDir -ChildPath "quaero-mcp.exe"

$mcpBuildArgs = @(
    "build"
    "-ldflags=$ldflags"
    "-o", $mcpOutputPath
    ".\cmd\quaero-mcp"
)

# Change to project root for build
Push-Location $projectRoot

Write-Host "Build command: go $($mcpBuildArgs -join ' ')" -ForegroundColor Gray

& go @mcpBuildArgs

# Return to original directory
Pop-Location

if ($LASTEXITCODE -ne 0) {
    Write-Host "MCP server build failed!" -ForegroundColor Red
    Stop-Transcript
    exit 1
}

# Verify MCP executable was created
if (-not (Test-Path $mcpOutputPath)) {
    Write-Error "MCP build completed but executable not found: $mcpOutputPath"
    Stop-Transcript
    exit 1
}

Write-Host "MCP server built successfully: $mcpOutputPath" -ForegroundColor Green

# Handle deployment and execution based on parameters
if ($Run -or $Deploy) {
    # Deploy files to bin directory
    Deploy-Files -ProjectRoot $projectRoot -BinDirectory $binDir

    if ($Run) {
        # Start application in new terminal
        Write-Host "`n==== Starting Application ====" -ForegroundColor Yellow

        $configPath = Join-Path -Path $binDir -ChildPath "quaero.toml"
        $startCommand = "cd /d `"$binDir`" && `"$outputPath`" -c `"$configPath`""

        Start-Process cmd -ArgumentList "/c", $startCommand

        Write-Host "Application started in new terminal window" -ForegroundColor Green
        Write-Host "Command: quaero.exe -c quaero.toml" -ForegroundColor Cyan
        Write-Host "Config: bin\quaero.toml" -ForegroundColor Gray
        Write-Host "Press Ctrl+C in the server window to stop gracefully" -ForegroundColor Yellow
        Write-Host "Check bin\logs\ for application logs" -ForegroundColor Yellow
    }
}

} finally {
    # Ensure transcript is stopped in all cases (success, error, or early exit)
    # Suppress errors if transcript wasn't started or already stopped
    try {
        Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
    } catch {
        # Silently ignore errors from Stop-Transcript
    }
}
