# -----------------------------------------------------------------------
# Deployment Script for Quaero (Docker)
# -----------------------------------------------------------------------
# Builds Docker image and deploys to Docker container
# -----------------------------------------------------------------------

param (
    [switch]$Stop,
    [switch]$Logs,
    [switch]$Status,
    [switch]$Rebuild,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

# Setup paths
$scriptDir = $PSScriptRoot
$projectRoot = Split-Path -Parent $scriptDir

# --- Logging Setup ---
$logDir = "$scriptDir/logs"
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir | Out-Null
}
$logFile = "$logDir/deploy-$(Get-Date -Format 'yyyy-MM-dd-HH-mm-ss').log"

# Function to limit log files to most recent 10
function Limit-DeployLogFiles {
    param(
        [string]$LogDirectory,
        [int]$MaxLogs = 10
    )

    $logFiles = Get-ChildItem -Path $LogDirectory -Filter "deploy-*.log" | Sort-Object CreationTime -Descending

    if (@($logFiles).Count -gt $MaxLogs) {
        $filesToDelete = $logFiles | Select-Object -Skip $MaxLogs
        foreach ($file in $filesToDelete) {
            Remove-Item -Path $file.FullName -Force
            Write-Host "Removed old log file: $($file.Name)" -ForegroundColor Gray
        }
    }
}

# Limit old log files before starting transcript
Limit-DeployLogFiles -LogDirectory $logDir -MaxLogs 10

Start-Transcript -Path $logFile -Append

Write-Host "ternarybob (parent) -> quaero" -ForegroundColor Magenta
Write-Host "Quaero Docker Deployment" -ForegroundColor Cyan
Write-Host "========================" -ForegroundColor Cyan

# Show help (no logging needed)
if ($Help) {
    Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
    Write-Host ""
    Write-Host "Usage: .\deploy.ps1 [-Status] [-Logs] [-Stop] [-Rebuild] [-Help]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -Rebuild   Stop, rebuild and redeploy (default)"
    Write-Host "  -Status    Show Docker container status"
    Write-Host "  -Logs      Follow Docker container logs"
    Write-Host "  -Stop      Stop Docker containers"
    Write-Host "  -Help      Show this help"
    exit 0
}

# Show status
if ($Status) {
    Write-Host "`nDocker Status:" -ForegroundColor Cyan
    docker ps --filter "name=quaero" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
    exit 0
}

# Show logs (no logging needed - interactive)
if ($Logs) {
    Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
    docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" logs -f
    exit 0
}

# Stop containers
if ($Stop) {
    Write-Host "Stopping Docker containers..." -ForegroundColor Yellow
    docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" down
    Write-Host "Stopped" -ForegroundColor Green
    Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
    exit 0
}

try {

# Default action: Rebuild (stop, build, deploy)
Write-Host "Preparing Docker build..." -ForegroundColor Yellow

# Stop existing container and clean up Docker resources
Write-Host "Stopping existing containers..." -ForegroundColor Yellow
docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" down -v 2>$null

Write-Host "Pruning Docker resources..." -ForegroundColor Yellow
docker container prune -f 2>$null
docker image prune -f 2>$null
docker volume prune -f 2>$null

# Remove old quaero image to force fresh build
$oldImage = docker images -q quaero:latest 2>$null
if ($oldImage) {
    Write-Host "Removing old quaero:latest image..." -ForegroundColor Yellow
    docker rmi quaero:latest -f 2>$null
}

# Paths
$commonConfig = "$projectRoot\deployments\common"
$dockerConfig = "$projectRoot\deployments\docker\config"
$staging = "$projectRoot\deployments\docker\.docker-staging"

# Clean and create staging
if (Test-Path $staging) { Remove-Item -Path $staging -Recurse -Force }
New-Item -ItemType Directory -Path "$staging\config" -Force | Out-Null
New-Item -ItemType Directory -Path "$staging\job-definitions" -Force | Out-Null
New-Item -ItemType Directory -Path "$staging\templates" -Force | Out-Null
New-Item -ItemType Directory -Path "$staging\docs" -Force | Out-Null

# Stage configs
Write-Host "  Staging configuration..." -ForegroundColor Gray
if (Test-Path "$commonConfig\connectors.toml") { Copy-Item "$commonConfig\connectors.toml" "$staging\config\" }
if (Test-Path "$commonConfig\email.toml") { Copy-Item "$commonConfig\email.toml" "$staging\config\" }
# Use docker-specific config template from common, or fall back to docker config dir
if (Test-Path "$commonConfig\quaero.docker.toml") {
    Copy-Item "$commonConfig\quaero.docker.toml" "$staging\config\quaero.toml"
} elseif (Test-Path "$dockerConfig\quaero.toml") {
    Copy-Item "$dockerConfig\quaero.toml" "$staging\config\"
} else {
    Write-Host "ERROR: No quaero.toml found for Docker deployment!" -ForegroundColor Red
    Write-Host "Expected at: $commonConfig\quaero.docker.toml or $dockerConfig\quaero.toml"
    exit 1
}
if (Test-Path "$dockerConfig\connectors.toml") { Copy-Item "$dockerConfig\connectors.toml" "$staging\config\" -Force }
if (Test-Path "$dockerConfig\email.toml") { Copy-Item "$dockerConfig\email.toml" "$staging\config\" -Force }
if (Test-Path "$dockerConfig\variables.toml") { Copy-Item "$dockerConfig\variables.toml" "$staging\config\" }
if (Test-Path "$dockerConfig\.env") { Copy-Item "$dockerConfig\.env" "$staging\config\" }

# Stage job-definitions (Docker uses ONLY job-definitions-docker, not common)
Write-Host "  Staging job-definitions from: $commonConfig\job-definitions-docker\" -ForegroundColor Gray
$dockerJobDefs = Get-ChildItem "$commonConfig\job-definitions-docker\*.toml" -ErrorAction SilentlyContinue
if ($dockerJobDefs) {
    Write-Host "  Found $($dockerJobDefs.Count) job definition(s):" -ForegroundColor Gray
    $dockerJobDefs | ForEach-Object {
        Write-Host "    - $($_.Name)" -ForegroundColor Cyan
        Copy-Item $_.FullName "$staging\job-definitions\"
    }
} else {
    Write-Host "  WARNING: No job definitions found in job-definitions-docker!" -ForegroundColor Yellow
}

# Stage templates
Write-Host "  Staging templates..." -ForegroundColor Gray
$templates = Get-ChildItem "$commonConfig\templates\*.toml" -ErrorAction SilentlyContinue
if ($templates) { $templates | ForEach-Object { Copy-Item $_.FullName "$staging\templates\" } }
$dockerTemplates = Get-ChildItem "$dockerConfig\templates\*.toml" -ErrorAction SilentlyContinue
if ($dockerTemplates) { $dockerTemplates | ForEach-Object { Copy-Item $_.FullName "$staging\templates\" -Force } }

# Stage docs
Write-Host "  Staging documentation..." -ForegroundColor Gray
if (Test-Path "$projectRoot\README.md") { Copy-Item "$projectRoot\README.md" "$staging\docs\" }
$archDocs = Get-ChildItem "$projectRoot\docs\architecture\*.md" -ErrorAction SilentlyContinue
if ($archDocs) { $archDocs | ForEach-Object { Copy-Item $_.FullName "$staging\docs\" } }

# Ensure directories are not empty (Docker COPY fails on empty dirs)
if (-not (Get-ChildItem "$staging\config" -ErrorAction SilentlyContinue)) {
    New-Item "$staging\config\.gitkeep" -ItemType File -Force | Out-Null
}
if (-not (Get-ChildItem "$staging\job-definitions" -ErrorAction SilentlyContinue)) {
    New-Item "$staging\job-definitions\.gitkeep" -ItemType File -Force | Out-Null
}
if (-not (Get-ChildItem "$staging\templates" -ErrorAction SilentlyContinue)) {
    New-Item "$staging\templates\.gitkeep" -ItemType File -Force | Out-Null
}

# Get version info
$version = "dev"
$build = "unknown"
$versionFile = "$projectRoot\.version"
if (Test-Path $versionFile) {
    Get-Content $versionFile | ForEach-Object {
        if ($_ -match '^version:\s*(.+)$') { $version = $matches[1].Trim() }
        if ($_ -match '^build:\s*(.+)$') { $build = $matches[1].Trim() }
    }
}
$gitCommit = git rev-parse --short HEAD 2>$null
if (-not $gitCommit) { $gitCommit = "unknown" }

Write-Host "Building Docker image..." -ForegroundColor Yellow
Write-Host "  Version: $version, Build: $build, Commit: $gitCommit" -ForegroundColor Gray

# Build Docker image (--no-cache ensures staged configs are always picked up)
Push-Location $projectRoot
docker build `
    --no-cache `
    --build-arg VERSION=$version `
    --build-arg BUILD=$build `
    --build-arg GIT_COMMIT=$gitCommit `
    -t quaero:latest `
    -f deployments/docker/Dockerfile `
    .
$buildResult = $LASTEXITCODE
Pop-Location

# Cleanup staging
Remove-Item -Path $staging -Recurse -Force

if ($buildResult -ne 0) {
    Write-Host "Docker build failed!" -ForegroundColor Red
    exit 1
}
Write-Host "Docker image built" -ForegroundColor Green

# Start container
Write-Host "Starting Docker container..." -ForegroundColor Yellow
docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" up -d

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nDeployment complete!" -ForegroundColor Green
    Write-Host "Access at: http://localhost:9000" -ForegroundColor Cyan
    docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" ps
} else {
    Write-Host "Failed to start container" -ForegroundColor Red
    Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
    exit 1
}

} finally {
    # Ensure transcript is stopped in all cases
    try {
        Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
    } catch {
        # Silently ignore errors from Stop-Transcript
    }
}
