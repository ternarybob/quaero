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

Write-Host "ternarybob (parent) -> quaero" -ForegroundColor Magenta
Write-Host "Quaero Docker Deployment" -ForegroundColor Cyan
Write-Host "========================" -ForegroundColor Cyan

# Show help
if ($Help) {
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
    exit 0
}

# Show logs
if ($Logs) {
    docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" logs -f
    exit 0
}

# Stop containers
if ($Stop) {
    Write-Host "Stopping Docker containers..." -ForegroundColor Yellow
    docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" down
    Write-Host "Stopped" -ForegroundColor Green
    exit 0
}

# Default action: Rebuild (stop, build, deploy)
Write-Host "Preparing Docker build..." -ForegroundColor Yellow

# Paths
$commonConfig = "$projectRoot\deployments\common"
$dockerConfig = "$projectRoot\deployments\docker\config"
$staging = "$projectRoot\deployments\docker\.docker-staging"

# Clean and create staging
if (Test-Path $staging) { Remove-Item -Path $staging -Recurse -Force }
New-Item -ItemType Directory -Path "$staging\config" -Force | Out-Null
New-Item -ItemType Directory -Path "$staging\job-definitions" -Force | Out-Null
New-Item -ItemType Directory -Path "$staging\job-templates" -Force | Out-Null
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

# Stage job-definitions
Write-Host "  Staging job-definitions..." -ForegroundColor Gray
$jobDefs = Get-ChildItem "$commonConfig\job-definitions\*.toml" -ErrorAction SilentlyContinue
if ($jobDefs) { $jobDefs | ForEach-Object { Copy-Item $_.FullName "$staging\job-definitions\" } }
$dockerJobDefs = Get-ChildItem "$dockerConfig\job-definitions\*.toml" -ErrorAction SilentlyContinue
if ($dockerJobDefs) { $dockerJobDefs | ForEach-Object { Copy-Item $_.FullName "$staging\job-definitions\" -Force } }

# Stage job-templates
Write-Host "  Staging job-templates..." -ForegroundColor Gray
$jobTemplates = Get-ChildItem "$commonConfig\job-templates\*.toml" -ErrorAction SilentlyContinue
if ($jobTemplates) { $jobTemplates | ForEach-Object { Copy-Item $_.FullName "$staging\job-templates\" } }
$dockerJobTemplates = Get-ChildItem "$dockerConfig\job-templates\*.toml" -ErrorAction SilentlyContinue
if ($dockerJobTemplates) { $dockerJobTemplates | ForEach-Object { Copy-Item $_.FullName "$staging\job-templates\" -Force } }

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
if (-not (Get-ChildItem "$staging\job-templates" -ErrorAction SilentlyContinue)) {
    New-Item "$staging\job-templates\.gitkeep" -ItemType File -Force | Out-Null
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

# Build Docker image
Push-Location $projectRoot
docker build `
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

# Check if container is running and stop it
$running = docker ps --filter "name=quaero" --format "{{.Names}}" 2>$null
if ($running) {
    Write-Host "Stopping existing container..." -ForegroundColor Yellow
    docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" down
}

# Start container
Write-Host "Starting Docker container..." -ForegroundColor Yellow
docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" up -d

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nDeployment complete!" -ForegroundColor Green
    Write-Host "Access at: http://localhost:8080" -ForegroundColor Cyan
    docker compose -f "$projectRoot\deployments\docker\docker-compose.yml" ps
} else {
    Write-Host "Failed to start container" -ForegroundColor Red
    exit 1
}
