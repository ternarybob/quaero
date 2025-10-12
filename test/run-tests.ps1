# -----------------------------------------------------------------------
# Last Modified: Wednesday, 8th October 2025 5:34:11 pm
# Modified By: Bob McAllan
# -----------------------------------------------------------------------

#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Run tests for the Quaero project

.DESCRIPTION
    Executes integration tests, UI tests, or all tests with coverage reporting.
    Must be run from the test directory.

.PARAMETER type
    Type of tests to run: 'unit', 'api', 'ui', or 'all' (default: all)

.PARAMETER script
    Filter tests by test function name pattern (case-insensitive, e.g., 'navbar' matches TestNavbar*)
    When used alone, searches all test directories. Combine with -type to limit to specific directory.

.PARAMETER coverage
    Generate coverage report (default: true)

.PARAMETER verboseoutput
    Enable verbose test output

.EXAMPLE
    ./run-tests.ps1
    ./run-tests.ps1 -type unit
    ./run-tests.ps1 -type api
    ./run-tests.ps1 -type ui
    ./run-tests.ps1 -type all -coverage
    ./run-tests.ps1 -script navbar
    ./run-tests.ps1 -type ui -script navbar
#>

param(
    [Parameter(Mandatory=$false)]
    [ValidateSet('unit', 'api', 'ui', 'all')]
    [string]$type = 'all',

    [Parameter(Mandatory=$false)]
    [string]$script = '',

    [Parameter(Mandatory=$false)]
    [switch]$coverage = $true,

    [Parameter(Mandatory=$false)]
    [switch]$verboseoutput
)

$ErrorActionPreference = "Stop"

# Ensure we're in the test directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

# Create timestamped results directory
$timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$testLabel = if ($script) { "$type-$script" } else { $type }
$resultsDir = Join-Path -Path $scriptDir -ChildPath "results\$testLabel-$timestamp"
New-Item -Path $resultsDir -ItemType Directory -Force | Out-Null

# Set environment variables for tests to use (port will be set after reading config)
$env:TEST_RUN_DIR = $resultsDir

Write-Host "=== Quaero Test Runner ===" -ForegroundColor Cyan
Write-Host "Test Type: $type" -ForegroundColor Yellow
if ($script) {
    Write-Host "Script Filter: $script" -ForegroundColor Yellow
}
Write-Host "Results Dir: $resultsDir" -ForegroundColor Cyan
Write-Host ""

# Stop any existing Quaero processes before building (graceful shutdown with fallback)
Write-Host ""
Write-Host "Checking for existing Quaero processes..." -ForegroundColor Yellow
try {
    $processName = "quaero"
    $existingProcesses = Get-Process -Name $processName -ErrorAction SilentlyContinue

    if ($existingProcesses) {
        Write-Host "Stopping existing Quaero process(es)..." -ForegroundColor Yellow
        
        # Try HTTP shutdown first
        $httpShutdownSucceeded = $false
        $projectRoot = (Get-Item $scriptDir).Parent.FullName
        $binDir = Join-Path -Path $projectRoot -ChildPath "bin"
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
                $response = Invoke-WebRequest -Uri "http://localhost:$serverPort/api/shutdown" -Method POST -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop
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
                Write-Warning "Process(es) did not exit gracefully, forcing termination..."
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
        Write-Host "No existing Quaero processes found" -ForegroundColor Gray
    }
}
catch {
    Write-Warning "Could not check/stop Quaero processes: $($_.Exception.Message)"
}

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

# Start the test server using the standard configuration
Write-Host ""
Write-Host "Reading configuration for server port..." -ForegroundColor Yellow
$projectRoot = (Get-Item $scriptDir).Parent.FullName
$binDir = Join-Path -Path $projectRoot -ChildPath "bin"
$configPath = Join-Path -Path $binDir -ChildPath "quaero.toml"
$exePath = Join-Path -Path $binDir -ChildPath "quaero.exe"

# Read port from config file
$serverPort = 8085  # Default port
if (Test-Path $configPath) {
    $configContent = Get-Content $configPath
    foreach ($line in $configContent) {
        if ($line -match '^port\s*=\s*(\d+)') {
            $serverPort = [int]$matches[1]
            break
        }
    }
}

Write-Host "Starting Quaero test server on port $serverPort..." -ForegroundColor Yellow

# Start server in new visible CMD window (like build.ps1 -Run does)
# Using /c so window closes when server stops, making it clear when tests are done
# IMPORTANT: Start from bin directory so database path resolves correctly
$startCommand = "cd /d `"$binDir`" && `"$exePath`" serve -c `"$configPath`""
$serverProcess = Start-Process cmd -ArgumentList "/c", $startCommand -PassThru

# Set the server URL environment variable now that we know the port
$env:TEST_SERVER_URL = "http://localhost:$serverPort"
Write-Host "Test server URL: $env:TEST_SERVER_URL" -ForegroundColor Cyan

# Wait for server to be ready
Write-Host "Waiting for server to be ready..." -ForegroundColor Yellow
$maxRetries = 30
$serverReady = $false
for ($i = 0; $i -lt $maxRetries; $i++) {
    # Use curl.exe to check if server is responding (curl is aliased to Invoke-WebRequest in PowerShell)
    $curlOutput = & curl.exe -s -o nul -w "%{http_code}" "http://localhost:$serverPort/" 2>&1 | Out-String
    $curlOutput = $curlOutput.Trim()
    if ($curlOutput -eq "200") {
        $serverReady = $true
        Write-Host "Server is ready on port $serverPort!" -ForegroundColor Green
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
if ($verboseoutput) {
    $testFlags += "-v"
}

if ($coverage) {
    $testFlags += "-coverprofile=coverage.out"
    $testFlags += "-covermode=atomic"
}

# Add script pattern filter if specified (case-insensitive)
if ($script) {
    $testFlags += "-run"
    $testFlags += "(?i)$script"
}

# Define output file
$testOutputFile = Join-Path -Path $resultsDir -ChildPath "test-output.log"

# Determine test path based on script parameter
if ($script -and $type -eq 'all') {
    # When script is specified with default type, search all directories
    $testPath = "./..."
    $testDescription = "tests matching '$script'"
} else {
    # Use type to determine path
    switch ($type) {
        'unit' { $testPath = "./unit/..."; $testDescription = "unit tests" }
        'api' { $testPath = "./api/..."; $testDescription = "API tests" }
        'ui' { $testPath = "./ui/..."; $testDescription = "UI tests" }
        'all' { $testPath = "./..."; $testDescription = "all tests" }
    }
    if ($script) {
        $testDescription += " matching '$script'"
    }
}

# Run tests
Write-Host "Running $testDescription..." -ForegroundColor Green
go test $testFlags $testPath 2>&1 | Tee-Object -FilePath $testOutputFile
$testResult = $LASTEXITCODE

# Display coverage if generated
if ($coverage -and (Test-Path "coverage.out")) {
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

# Stop the server and any quaero processes (graceful shutdown with fallback)
Write-Host ""
Write-Host "Stopping Quaero server..." -ForegroundColor Yellow

# Try graceful shutdown first
try {
    if ($serverProcess -and !$serverProcess.HasExited) {
        Write-Host "  Attempting HTTP graceful shutdown..." -ForegroundColor Gray
        
        # Use HTTP shutdown endpoint
        $httpShutdownSucceeded = $false
        try {
            $response = Invoke-WebRequest -Uri $env:TEST_SERVER_URL/api/shutdown -Method POST -TimeoutSec 2 -UseBasicParsing -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                Write-Host "  HTTP shutdown request sent successfully" -ForegroundColor Gray
                $httpShutdownSucceeded = $true
            }
        }
        catch {
            Write-Host "  HTTP shutdown failed, will force termination" -ForegroundColor Gray
        }
        
        # Wait for graceful shutdown
        $timeout = if ($httpShutdownSucceeded) { 12 } else { 2 }
        $elapsed = 0
        $checkInterval = 0.5
        
        while (!$serverProcess.HasExited -and ($elapsed -lt $timeout)) {
            Start-Sleep -Seconds $checkInterval
            $elapsed += $checkInterval
            $serverProcess.Refresh()
        }
        
        if ($serverProcess.HasExited) {
            Write-Host "Server stopped gracefully" -ForegroundColor Green
        } else {
            Write-Warning "Server did not exit gracefully, forcing termination..."
            Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
            Write-Host "Server force-stopped" -ForegroundColor Yellow
        }
    }
}
catch {
    Write-Warning "Could not stop server by PID: $($_.Exception.Message)"
}

# Belt-and-suspenders: ensure all quaero processes are stopped
try {
    Start-Sleep -Milliseconds 500
    $remainingProcesses = Get-Process -Name "quaero" -ErrorAction SilentlyContinue
    
    if ($remainingProcesses) {
        Write-Host "Stopping remaining quaero processes by name..." -ForegroundColor Yellow
        
        # Try HTTP shutdown for remaining processes
        foreach ($proc in $remainingProcesses) {
            try {
                Invoke-WebRequest -Uri $env:TEST_SERVER_URL/api/shutdown -Method POST -TimeoutSec 1 -UseBasicParsing -ErrorAction SilentlyContinue | Out-Null
            }
            catch {
                # Fallback to force kill
                Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
            }
        }
        
        Start-Sleep -Seconds 2
        $stillRunning = Get-Process -Name "quaero" -ErrorAction SilentlyContinue
        
        if ($stillRunning) {
            Write-Warning "Forcing remaining processes..."
            Stop-Process -Name "quaero" -Force -ErrorAction SilentlyContinue
            Start-Sleep -Milliseconds 500
        }
        
        $finalCheck = Get-Process -Name "quaero" -ErrorAction SilentlyContinue
        if ($finalCheck) {
            Write-Warning "Some quaero processes may still be running"
        } else {
            Write-Host "All quaero processes stopped" -ForegroundColor Green
        }
    }
}
catch {
    Write-Warning "Could not verify all processes stopped: $($_.Exception.Message)"
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
