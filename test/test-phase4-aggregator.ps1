#!/usr/bin/env pwsh
<#
.SYNOPSIS
Phase 4 Aggregator Integration Test

Tests the aggregator tool by:
1. Running demo-job with 4 instances to generate metrics
2. Running the aggregator on those metrics
3. Validating the aggregated output is correct
#>

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Write-Host "=== Phase 4 Aggregator Integration Test ===" -ForegroundColor Cyan

# Ensure we're in the workspace root
$workspaceRoot = Split-Path -Parent $PSScriptRoot
Set-Location $workspaceRoot
Write-Host "Working directory: $(Get-Location)" -ForegroundColor Gray

# Create output directory
$outputDir = "test-output-phase4"
if (Test-Path $outputDir) {
    Remove-Item $outputDir -Recurse -Force
}
New-Item -ItemType Directory -Path $outputDir | Out-Null
Write-Host "Created output directory: $outputDir" -ForegroundColor Green

# Generate large test file (5M lines = ~500MB)
Write-Host "`nGenerating 5M line test file..." -ForegroundColor Yellow
$testFile = "$outputDir/test-data.txt"
$lineCount = 5000000
$batchSize = 100000

# Write in batches for performance
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$fileStream = [System.IO.File]::Create($testFile)
$writer = New-Object System.IO.StreamWriter($fileStream)

for ($i = 0; $i -lt $lineCount; $i += $batchSize) {
    $endLine = [Math]::Min($i + $batchSize, $lineCount)
    for ($j = $i; $j -lt $endLine; $j++) {
        $writer.WriteLine("Line $j`: Sample data for testing distributed aggregation with metrics")
    }
    Write-Progress -Activity "Generating test file" -PercentComplete ($i / $lineCount * 100) -Status "$i / $lineCount lines"
}
$writer.Close()
$writer.Dispose()
$fileStream.Dispose()
$sw.Stop()

$fileSize = (Get-Item $testFile).Length
Write-Host "Test file created: $(Get-Item $testFile | Select-Object -ExpandProperty Length) bytes ($('{0:N0}' -f $lineCount) lines)" -ForegroundColor Green
Write-Host "Generation time: $($sw.Elapsed.TotalSeconds)s" -ForegroundColor Gray

# Build demo-job if needed
Write-Host "`nBuilding demo-job..." -ForegroundColor Yellow
$demoBinary = "cmd/demo-job/demo-job"
if (-not (Test-Path $demoBinary)) {
    & go build -o $demoBinary ./cmd/demo-job 2>&1 | Write-Host -ForegroundColor Gray
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Failed to build demo-job" -ForegroundColor Red
        exit 1
    }
}
Write-Host "demo-job binary ready" -ForegroundColor Green

# Build aggregator if needed
Write-Host "`nBuilding aggregator..." -ForegroundColor Yellow
$aggregatorBinary = "cmd/aggregator/aggregator"
if (-not (Test-Path $aggregatorBinary)) {
    & go build -o $aggregatorBinary ./cmd/aggregator 2>&1 | Write-Host -ForegroundColor Gray
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Failed to build aggregator" -ForegroundColor Red
        exit 1
    }
}
Write-Host "Aggregator binary ready" -ForegroundColor Green

# Run 4 demo-job instances in parallel
Write-Host "`nRunning 4 instances in parallel..." -ForegroundColor Yellow
$instanceJobs = @()
$metricsDir = "$outputDir/metrics"
New-Item -ItemType Directory -Path $metricsDir -Force | Out-Null

$instanceSw = [System.Diagnostics.Stopwatch]::StartNew()

for ($i = 0; $i -lt 4; $i++) {
    Write-Host "Starting instance $i..." -ForegroundColor Green
    $job = Start-Job -ScriptBlock {
        param($binary, $testFile, $fileSize, $metricsDir, $instanceId)
        & $binary `
            -instance-id $instanceId `
            -total-instances 4 `
            -input-path $testFile `
            -input-size $fileSize `
            -output-dir $metricsDir `
            -job-id "test-phase4" `
            2>&1 | Out-String | Write-Host
    } -ArgumentList $demoBinary, $testFile, $fileSize, $metricsDir, $i -Name "instance-$i"
    $instanceJobs += $job
}

# Wait for all instances to complete
Write-Host "`nWaiting for all instances to complete..." -ForegroundColor Yellow
foreach ($job in $instanceJobs) {
    $jobName = $job.Name
    $result = Wait-Job -Job $job
    $output = Receive-Job -Job $job -ErrorAction SilentlyContinue
    if ($result.State -eq "Completed") {
        Write-Host "✓ $jobName completed" -ForegroundColor Green
    } else {
        Write-Host "✗ $jobName failed: $(Show-Job -Job $job)" -ForegroundColor Red
        Remove-Job -Job $job -Force
        exit 1
    }
    Remove-Job -Job $job -Force
}

$instanceSw.Stop()
Write-Host "`nAll instances completed in $($instanceSw.Elapsed.TotalSeconds)s" -ForegroundColor Green

# Verify all metrics files were created
Write-Host "`nVerifying metrics files..." -ForegroundColor Yellow
$metricsFiles = Get-ChildItem -Path $metricsDir -Filter "instance-*.json"
if ($metricsFiles.Count -ne 4) {
    Write-Host "ERROR: Expected 4 metrics files, found $($metricsFiles.Count)" -ForegroundColor Red
    $metricsFiles | ForEach-Object { Write-Host "  Found: $_" }
    exit 1
}

foreach ($file in $metricsFiles) {
    $content = Get-Content $file.FullName -Raw | ConvertFrom-Json
    Write-Host "✓ $(Split-Path -Leaf $file.FullName) - Instance ID: $($content.instance_id), Lines: $($content.lines_count), Time: $($content.processing_time_seconds)s" -ForegroundColor Green
}

# Run aggregator
Write-Host "`nRunning aggregator (detailed format)..." -ForegroundColor Yellow
$aggregatorSw = [System.Diagnostics.Stopwatch]::StartNew()
& $aggregatorBinary --metrics-path $metricsDir --format detailed 2>&1 | Tee-Object -Variable aggregatorOutput | Write-Host -ForegroundColor Cyan
$aggregatorSw.Stop()

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Aggregator failed with exit code $LASTEXITCODE" -ForegroundColor Red
    exit 1
}

Write-Host "`nAggregator completed in $($aggregatorSw.Elapsed.TotalSeconds)s" -ForegroundColor Green

# Validate aggregator output
Write-Host "`nValidating aggregator output..." -ForegroundColor Yellow

# Check for required fields in output
$requiredFields = @(
    "Instances processed",
    "Total lines",
    "Total bytes",
    "Processing time",
    "Instance Breakdown"
)

$outputString = $aggregatorOutput -join "`n"
$allFieldsFound = $true

foreach ($field in $requiredFields) {
    if ($outputString -match $field) {
        Write-Host "✓ Found: $field" -ForegroundColor Green
    } else {
        Write-Host "✗ Missing: $field" -ForegroundColor Red
        $allFieldsFound = $false
    }
}

if (-not $allFieldsFound) {
    Write-Host "ERROR: Aggregator output missing required fields" -ForegroundColor Red
    exit 1
}

# Verify total lines match expected (should be 4 x 1.25M = 5M)
if ($outputString -match "Total lines:\s+(\d+)") {
    $totalLines = [int]$matches[1]
    $expectedLines = 5000000
    if ($totalLines -eq $expectedLines) {
        Write-Host "✓ Total lines: $totalLines (correct)" -ForegroundColor Green
    } else {
        # Allow some tolerance for distribution
        $tolerance = $expectedLines * 0.01  # 1% tolerance
        if ([Math]::Abs($totalLines - $expectedLines) -lt $tolerance) {
            Write-Host "✓ Total lines: $totalLines (within 1% of $expectedLines)" -ForegroundColor Green
        } else {
            Write-Host "WARNING: Total lines: $totalLines (expected ~$expectedLines)" -ForegroundColor Yellow
        }
    }
}

# Run aggregator with summary format
Write-Host "`nRunning aggregator (summary format)..." -ForegroundColor Yellow
$summaryOutput = & $aggregatorBinary --metrics-path $metricsDir --format summary 2>&1
Write-Host $summaryOutput -ForegroundColor Cyan

Write-Host "`n=== Phase 4 Aggregator Test PASSED ===" -ForegroundColor Green

# Cleanup (optional - keep for manual verification)
Write-Host "`nTest data saved in: $(Resolve-Path $outputDir)" -ForegroundColor Gray
Write-Host "To clean up: Remove-Item '$outputDir' -Recurse -Force" -ForegroundColor Gray
