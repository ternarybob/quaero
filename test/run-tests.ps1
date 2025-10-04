#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Run tests for the Quaero project

.DESCRIPTION
    Executes unit tests, integration tests, or all tests with coverage reporting.
    Must be run from the test directory.

.PARAMETER Type
    Type of tests to run: 'unit', 'integration', or 'all' (default: all)

.PARAMETER Coverage
    Generate coverage report (default: true)

.PARAMETER Verbose
    Enable verbose test output

.EXAMPLE
    ./run-tests.ps1 -Type unit
    ./run-tests.ps1 -Type all -Coverage
#>

param(
    [Parameter(Mandatory=$false)]
    [ValidateSet('unit', 'integration', 'all')]
    [string]$Type = 'all',

    [Parameter(Mandatory=$false)]
    [switch]$Coverage = $true,

    [Parameter(Mandatory=$false)]
    [switch]$VerboseOutput
)

$ErrorActionPreference = "Stop"

# Ensure we're in the test directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

Write-Host "=== Quaero Test Runner ===" -ForegroundColor Cyan
Write-Host "Test Type: $Type" -ForegroundColor Yellow
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

# Run tests based on type
switch ($Type) {
    'unit' {
        Write-Host "Running unit tests..." -ForegroundColor Green
        go test $testFlags ./unit/...
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Unit tests failed!" -ForegroundColor Red
            exit 1
        }
    }
    'integration' {
        Write-Host "Running integration tests..." -ForegroundColor Green
        go test $testFlags ./integration/...
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Integration tests failed!" -ForegroundColor Red
            exit 1
        }
    }
    'all' {
        Write-Host "Running all tests..." -ForegroundColor Green
        go test $testFlags ./...
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Tests failed!" -ForegroundColor Red
            exit 1
        }
    }
}

# Display coverage if generated
if ($Coverage -and (Test-Path "coverage.out")) {
    Write-Host ""
    Write-Host "=== Coverage Report ===" -ForegroundColor Cyan
    go tool cover -func=coverage.out | Select-Object -Last 1

    Write-Host ""
    Write-Host "Full coverage report: coverage.out" -ForegroundColor Yellow
    Write-Host "To view HTML coverage: go tool cover -html=coverage.out" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== Tests Complete ===" -ForegroundColor Green
exit 0
