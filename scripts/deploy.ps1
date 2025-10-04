# -----------------------------------------------------------------------
# Deployment Script for Quaero
# -----------------------------------------------------------------------

param (
    [Parameter(Mandatory=$false)]
    [ValidateSet("local", "docker", "production")]
    [string]$Target = "local",

    [Parameter(Mandatory=$false)]
    [string]$ConfigPath = "",

    [switch]$Build,
    [switch]$Stop,
    [switch]$Restart,
    [switch]$Status,
    [switch]$Logs
)

<#
.SYNOPSIS
    Deploy and manage Quaero

.DESCRIPTION
    This script helps deploy and manage the Quaero service
    in different environments (local, Docker, production).

.PARAMETER Target
    Deployment target: local, docker, or production

.PARAMETER ConfigPath
    Path to configuration file (defaults to deployments/<target>/quaero.toml)

.PARAMETER Build
    Build before deploying

.PARAMETER Stop
    Stop the running service

.PARAMETER Restart
    Restart the running service

.PARAMETER Status
    Show service status

.PARAMETER Logs
    Show service logs

.EXAMPLE
    .\deploy.ps1 -Target local
    Deploy to local environment

.EXAMPLE
    .\deploy.ps1 -Target docker -Build
    Build and deploy to Docker

.EXAMPLE
    .\deploy.ps1 -Status
    Show current service status
#>

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# Color output functions
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

# Setup paths
$scriptDir = $PSScriptRoot
$projectRoot = Split-Path -Parent $scriptDir
$binDir = Join-Path -Path $projectRoot -ChildPath "bin"
$executablePath = Join-Path -Path $binDir -ChildPath "quaero.exe"

Write-ColorOutput "Quaero Deployment Script" "Cyan"
Write-ColorOutput "========================" "Cyan"
Write-ColorOutput "Target: $Target" "Gray"

# Determine config path
if (-not $ConfigPath) {
    switch ($Target) {
        "local" {
            $ConfigPath = Join-Path -Path $projectRoot -ChildPath "deployments\local\quaero.toml"
            if (-not (Test-Path $ConfigPath)) {
                $ConfigPath = Join-Path -Path $binDir -ChildPath "quaero.toml"
            }
        }
        "docker" {
            $ConfigPath = Join-Path -Path $projectRoot -ChildPath "deployments\docker\quaero.toml"
        }
        "production" {
            $ConfigPath = Join-Path -Path $projectRoot -ChildPath "deployments\quaero.toml"
        }
    }
}

Write-ColorOutput "Config: $ConfigPath" "Gray"

# Build if requested
if ($Build) {
    Write-ColorOutput "`nBuilding application..." "Yellow"
    $buildScript = Join-Path -Path $scriptDir -ChildPath "build.ps1"

    if ($Target -eq "docker") {
        & $buildScript -Release
    } else {
        & $buildScript
    }

    if ($LASTEXITCODE -ne 0) {
        Write-ColorOutput "Build failed!" "Red"
        exit 1
    }
    Write-ColorOutput "Build completed successfully" "Green"
}

# Get process status
function Get-ServiceStatus {
    $processName = "quaero"
    $process = Get-Process -Name $processName -ErrorAction SilentlyContinue

    if ($process) {
        return @{
            Running = $true
            PID = $process.Id
            StartTime = $process.StartTime
            Memory = [math]::Round($process.WorkingSet64 / 1MB, 2)
        }
    } else {
        return @{Running = $false}
    }
}

# Show status
if ($Status) {
    Write-ColorOutput "`nService Status:" "Cyan"
    $status = Get-ServiceStatus

    if ($status.Running) {
        Write-ColorOutput "Status: RUNNING" "Green"
        Write-ColorOutput "PID: $($status.PID)" "Green"
        Write-ColorOutput "Started: $($status.StartTime)" "Green"
        Write-ColorOutput "Memory: $($status.Memory) MB" "Green"
    } else {
        Write-ColorOutput "Status: STOPPED" "Yellow"
    }

    # Check if Docker containers are running
    if ($Target -eq "docker") {
        Write-ColorOutput "`nDocker Status:" "Cyan"
        try {
            $dockerStatus = docker ps --filter "name=quaero" --format "{{.Status}}"
            if ($dockerStatus) {
                Write-ColorOutput "Docker Container: $dockerStatus" "Green"
            } else {
                Write-ColorOutput "Docker Container: NOT RUNNING" "Yellow"
            }
        } catch {
            Write-ColorOutput "Docker not available" "Yellow"
        }
    }

    exit 0
}

# Stop service
if ($Stop -or $Restart) {
    Write-ColorOutput "`nStopping service..." "Yellow"

    if ($Target -eq "docker") {
        # Stop Docker containers
        try {
            docker-compose -f "$projectRoot\deployments\docker\docker-compose.yml" down
            Write-ColorOutput "Docker containers stopped" "Green"
        } catch {
            Write-ColorOutput "Failed to stop Docker containers" "Red"
        }
    } else {
        # Stop local process
        $processName = "quaero"
        $process = Get-Process -Name $processName -ErrorAction SilentlyContinue

        if ($process) {
            Stop-Process -Name $processName -Force
            Start-Sleep -Seconds 2
            Write-ColorOutput "Service stopped" "Green"
        } else {
            Write-ColorOutput "Service not running" "Yellow"
        }
    }

    if (-not $Restart) {
        exit 0
    }
}

# Show logs
if ($Logs) {
    Write-ColorOutput "`nService Logs:" "Cyan"

    if ($Target -eq "docker") {
        docker-compose -f "$projectRoot\deployments\docker\docker-compose.yml" logs -f
    } else {
        $logPath = Join-Path -Path $projectRoot -ChildPath "logs\quaero.log"
        if (Test-Path $logPath) {
            Get-Content -Path $logPath -Tail 50 -Wait
        } else {
            Write-ColorOutput "Log file not found: $logPath" "Yellow"
            Write-ColorOutput "Check console output or configure file logging" "Gray"
        }
    }

    exit 0
}

# Deploy/Start service
Write-ColorOutput "`nDeploying service..." "Yellow"

if ($Target -eq "docker") {
    # Docker deployment
    Write-ColorOutput "Starting Docker containers..." "Yellow"

    $dockerComposeFile = Join-Path -Path $projectRoot -ChildPath "deployments\docker\docker-compose.yml"

    if (-not (Test-Path $dockerComposeFile)) {
        Write-ColorOutput "docker-compose.yml not found: $dockerComposeFile" "Red"
        exit 1
    }

    docker-compose -f $dockerComposeFile up -d

    if ($LASTEXITCODE -eq 0) {
        Write-ColorOutput "Docker containers started successfully" "Green"
        Start-Sleep -Seconds 2
        docker-compose -f $dockerComposeFile ps
    } else {
        Write-ColorOutput "Failed to start Docker containers" "Red"
        exit 1
    }
} else {
    # Local deployment
    if (-not (Test-Path $executablePath)) {
        Write-ColorOutput "Executable not found: $executablePath" "Red"
        Write-ColorOutput "Run with -Build flag to build first" "Yellow"
        exit 1
    }

    if (-not (Test-Path $ConfigPath)) {
        Write-ColorOutput "Config file not found: $ConfigPath" "Red"
        exit 1
    }

    Write-ColorOutput "Starting service..." "Yellow"

    # Start in new window for local development
    $startArgs = @{
        FilePath = $executablePath
        ArgumentList = @("serve", "-c", $ConfigPath)
        WorkingDirectory = $projectRoot
        PassThru = $true
    }

    if ($Target -eq "production") {
        # Production: run as background process
        $process = Start-Process @startArgs -WindowStyle Hidden
    } else {
        # Local: run in new window
        $process = Start-Process @startArgs
    }

    Start-Sleep -Seconds 2

    $status = Get-ServiceStatus
    if ($status.Running) {
        Write-ColorOutput "Service started successfully" "Green"
        Write-ColorOutput "PID: $($status.PID)" "Green"
        Write-ColorOutput "Config: $ConfigPath" "Green"
    } else {
        Write-ColorOutput "Failed to start service" "Red"
        exit 1
    }
}

Write-ColorOutput "`nDeployment completed!" "Green"

if ($Target -eq "local") {
    Write-ColorOutput "`nAccess the web interface at: http://localhost:8080" "Cyan"
    Write-ColorOutput "Available commands:" "Yellow"
    Write-ColorOutput "  quaero serve   - Start web server and API" "White"
    Write-ColorOutput "  quaero collect - Run data collection" "White"
    Write-ColorOutput "  quaero query   - Execute search query" "White"
    Write-ColorOutput "  quaero version - Show version info" "White"
}
