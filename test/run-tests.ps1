#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Run tests for the Quaero project

.DESCRIPTION
    Executes integration tests, UI tests, or all tests with coverage reporting.
    Must be run from the test directory.

.PARAMETER Type
    Type of tests to run: 'unit', 'api', 'integration', 'ui', or 'all' (default: integration)

.PARAMETER Coverage
    Generate coverage report (default: true)

.PARAMETER Verbose
    Enable verbose test output

.EXAMPLE
    ./run-tests.ps1 -Type unit
    ./run-tests.ps1 -Type api
    ./run-tests.ps1 -Type integration
    ./run-tests.ps1 -Type ui
    ./run-tests.ps1 -Type all -Coverage
#>

param(
    [Parameter(Mandatory=$false)]
    [ValidateSet('unit', 'api', 'integration', 'ui', 'all')]
    [string]$Type = 'integration',

    [Parameter(Mandatory=$false)]
    [switch]$Coverage = $true,

    [Parameter(Mandatory=$false)]
    [switch]$VerboseOutput
)

$ErrorActionPreference = "Stop"

# Ensure we're in the test directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

# Create timestamped results directory
$timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$resultsDir = Join-Path -Path $scriptDir -ChildPath "results\$Type-$timestamp"
New-Item -Path $resultsDir -ItemType Directory -Force | Out-Null

# Set environment variables for tests to use
$env:TEST_RUN_DIR = $resultsDir
$env:TEST_SERVER_URL = "http://localhost:8086"

Write-Host "=== Quaero Test Runner ===" -ForegroundColor Cyan
Write-Host "Test Type: $Type" -ForegroundColor Yellow
Write-Host "Results Dir: $resultsDir" -ForegroundColor Cyan
Write-Host ""

# Build the application
Write-Host ""
Write-Host "Building application..." -ForegroundColor Yellow
Set-Location ..
& "./scripts/build.ps1"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}
Set-Location $scriptDir

# Start the test server on port 8086 (avoids conflicts with dev server on 8085)
Write-Host ""
Write-Host "Starting Quaero test server on port 8086..." -ForegroundColor Yellow
$projectRoot = (Get-Item $scriptDir).Parent.FullName
$binDir = Join-Path -Path $projectRoot -ChildPath "bin"
$configPath = Join-Path -Path $binDir -ChildPath "quaero-test.toml"
$exePath = Join-Path -Path $binDir -ChildPath "quaero.exe"

# Start server in hidden window (stays running until explicitly killed)
$startCommand = "cd /d `"$projectRoot`" && `"$exePath`" serve -c `"$configPath`""
$serverProcess = Start-Process cmd -ArgumentList "/k", $startCommand -WindowStyle Hidden -PassThru

# Wait for server to be ready on port 8086
Write-Host "Waiting for server to be ready..." -ForegroundColor Yellow
$maxRetries = 30
$serverReady = $false
for ($i = 0; $i -lt $maxRetries; $i++) {
    # Use curl to check if server is responding (more reliable than Invoke-WebRequest)
    $curlOutput = & curl -s -o nul -w "%{http_code}" http://localhost:8086/ 2>&1 | Out-String
    $curlOutput = $curlOutput.Trim()
    if ($curlOutput -eq "200") {
        $serverReady = $true
        Write-Host "Server is ready on port 8086!" -ForegroundColor Green
        break
    }
    Start-Sleep -Seconds 1
}

if (-not $serverReady) {
    Write-Host "Server did not become ready in time" -ForegroundColor Red
    Stop-Process $serverProcess -Force -ErrorAction SilentlyContinue
    exit 1
}

Write-Host ""

# Build test flags
$testFlags = @()
if ($VerboseOutput) {
    $testFlags += "-v"
}

if ($Coverage) {
    $testFlags += "-coverprofile=coverage.out"
    $testFlags += "-covermode=atomic"
}

# Define output file
$testOutputFile = Join-Path -Path $resultsDir -ChildPath "test-output.log"

# Run tests based on type
switch ($Type) {
    'unit' {
        Write-Host "Running unit tests..." -ForegroundColor Green
        go test $testFlags ./unit/... 2>&1 | Tee-Object -FilePath $testOutputFile
        $testResult = $LASTEXITCODE
    }
    'api' {
        Write-Host "Running API tests..." -ForegroundColor Green
        go test $testFlags ./api/... 2>&1 | Tee-Object -FilePath $testOutputFile
        $testResult = $LASTEXITCODE
    }
    'integration' {
        Write-Host "Running integration tests..." -ForegroundColor Green
        go test $testFlags ./integration/... 2>&1 | Tee-Object -FilePath $testOutputFile
        $testResult = $LASTEXITCODE
    }
    'ui' {
        Write-Host "Running UI tests..." -ForegroundColor Green
        go test $testFlags ./ui/... 2>&1 | Tee-Object -FilePath $testOutputFile
        $testResult = $LASTEXITCODE
    }
    'all' {
        Write-Host "Running all tests..." -ForegroundColor Green
        go test $testFlags ./... 2>&1 | Tee-Object -FilePath $testOutputFile
        $testResult = $LASTEXITCODE
    }
}

# Display coverage if generated
if ($Coverage -and (Test-Path "coverage.out")) {
    # Copy coverage to results directory
    $coverageFile = Join-Path -Path $resultsDir -ChildPath "coverage.out"
    Copy-Item "coverage.out" $coverageFile -Force

    # Check if coverage file has actual data (more than just "mode: atomic")
    $coverageSize = (Get-Item "coverage.out").Length
    if ($coverageSize -gt 20) {
        Write-Host ""
        Write-Host "=== Coverage Report ===" -ForegroundColor Cyan

        # Display summary
        go tool cover -func=coverage.out | Select-Object -Last 1

        # Generate HTML coverage report
        $coverageHTML = Join-Path -Path $resultsDir -ChildPath "coverage.html"
        go tool cover -html=coverage.out -o $coverageHTML 2>$null

        Write-Host ""
        Write-Host "Coverage files saved to:" -ForegroundColor Yellow
        Write-Host "  Text: $coverageFile" -ForegroundColor Gray
        if (Test-Path $coverageHTML) {
            Write-Host "  HTML: $coverageHTML" -ForegroundColor Gray
        }
    } else {
        Write-Host ""
        Write-Host "No coverage data generated (tests run via HTTP, not directly)" -ForegroundColor Gray
        Write-Host "Coverage file saved to: $coverageFile" -ForegroundColor Gray
    }
}

# Stop the server
Write-Host ""
Write-Host "Stopping Quaero server..." -ForegroundColor Yellow
Stop-Process $serverProcess -Force -ErrorAction SilentlyContinue
Write-Host "Server stopped" -ForegroundColor Green

Write-Host ""
Write-Host "=== Tests Complete ===" -ForegroundColor Green
Write-Host "Results saved to: $resultsDir" -ForegroundColor Cyan

# List test artifacts
if (Test-Path $testOutputFile) {
    Write-Host "  Test output: test-output.log" -ForegroundColor Gray
}

$screenshots = @(Get-ChildItem -Path $resultsDir -Filter "*.png" -ErrorAction SilentlyContinue)
if ($screenshots.Count -gt 0) {
    Write-Host "  Screenshots: $($screenshots.Count)" -ForegroundColor Gray
}

$coverageOut = Join-Path -Path $resultsDir -ChildPath "coverage.out"
if (Test-Path $coverageOut) {
    $coverageSize = (Get-Item $coverageOut).Length
    if ($coverageSize -gt 20) {
        $coverageHTML = Join-Path -Path $resultsDir -ChildPath "coverage.html"
        if (Test-Path $coverageHTML) {
            Write-Host "  Coverage: coverage.out, coverage.html" -ForegroundColor Gray
        } else {
            Write-Host "  Coverage: coverage.out" -ForegroundColor Gray
        }
    }
}

# Exit with test result
if ($testResult -ne 0) {
    Write-Host ""
    Write-Host "Tests failed!" -ForegroundColor Red
    exit 1
}

exit 0
