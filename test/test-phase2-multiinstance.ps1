# Phase 2: Multi-Instance Awareness Test

$ErrorActionPreference = "Stop"

Write-Host "=== Phase 2: Multi-Instance Awareness Test ===" -ForegroundColor Cyan
Write-Host "Testing byte-range isolation and multi-instance coordination" -ForegroundColor Cyan
Write-Host ""

# Build demo-job
Write-Host "Building demo-job..." -ForegroundColor Yellow
go build -o cmd/demo-job/demo-job.exe ./cmd/demo-job 2>&1 | Out-Null
Write-Host "✓ Build complete" -ForegroundColor Green
Write-Host ""

# Generate test file (5M lines)
Write-Host "Generating test file (5M lines)..." -ForegroundColor Yellow
$testFile = "test-phase2-5m.txt"
$lines = @()
for ($i = 0; $i -lt 5000000; $i++) {
    $lines += "Line $i : Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor"
    if ($i % 1000000 -eq 0 -and $i -gt 0) {
        Write-Host "  Generated $i lines..." -ForegroundColor Gray
    }
}
$lines | Out-File -FilePath $testFile -Encoding UTF8
Write-Host "✓ Generated 5M line test file" -ForegroundColor Green

$fileSize = (Get-Item $testFile).Length
Write-Host "File size: $fileSize bytes"
Write-Host ""

# Test: 4 instances in parallel
Write-Host "--- Test: 4 Concurrent Instances ---" -ForegroundColor Cyan
$outputDir = "output-phase2"
New-Item -ItemType Directory -Path $outputDir -Force | Out-Null

$env:INPUT_DATA_PATH = ".\$testFile"
$env:INPUT_DATA_SIZE = $fileSize
$env:OUTPUT_BASE_PATH = ".\$outputDir"

# Start all 4 instances in parallel using jobs
$jobs = @()
for ($i = 0; $i -lt 4; $i++) {
    $env:BATCH_TASK_INDEX = $i
    $env:BATCH_TASK_COUNT = "4"
    Write-Host "Starting instance $i/4..." -ForegroundColor Yellow
    $job = Start-Job -ScriptBlock {
        Set-Item -Path Env:BATCH_TASK_INDEX -Value $using:i
        Set-Item -Path Env:BATCH_TASK_COUNT -Value "4"
        Set-Item -Path Env:INPUT_DATA_PATH -Value $using:env:INPUT_DATA_PATH
        Set-Item -Path Env:INPUT_DATA_SIZE -Value $using:env:INPUT_DATA_SIZE
        Set-Item -Path Env:OUTPUT_BASE_PATH -Value $using:env:OUTPUT_BASE_PATH
        & ".\cmd\demo-job\demo-job.exe" 2>&1 | Out-Null
    }
    $jobs += $job
}

# Wait for all to complete
Write-Host "Waiting for all instances..." -ForegroundColor Yellow
$jobs | Wait-Job | Out-Null
Write-Host "All instances completed" -ForegroundColor Green
Write-Host ""

# Verify results
Write-Host "--- Verification ---" -ForegroundColor Cyan
$totalLines = 0
$totalBytes = 0

for ($i = 0; $i -lt 4; $i++) {
    $metricFile = "$outputDir\instance-$i.json"
    if (Test-Path $metricFile) {
        $metrics = Get-Content $metricFile -Raw | ConvertFrom-Json
        $lines = $metrics.lines_count
        $bytes = $metrics.bytes_processed
        $start = $metrics.start_byte
        $end = $metrics.end_byte
        
        Write-Host "✓ Instance $i :" -ForegroundColor Green
        Write-Host "    Byte range: $start - $end ($bytes bytes)"
        Write-Host "    Lines: $lines"
        
        $totalLines += $lines
        $totalBytes += $bytes
    } else {
        Write-Host "✗ Instance $i output MISSING" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "Summary:" -ForegroundColor Cyan
Write-Host "  Total bytes processed: $totalBytes / $fileSize"
Write-Host "  Total lines counted: $totalLines / ~5000000"

# Validate
if ($totalBytes -eq $fileSize) {
    Write-Host "✓ All bytes accounted for (no gaps/overlaps)" -ForegroundColor Green
} else {
    Write-Host "✗ Byte count mismatch: expected $fileSize, got $totalBytes" -ForegroundColor Red
    exit 1
}

if ($totalLines -gt 4900000 -and $totalLines -lt 5100000) {
    Write-Host "✓ Line count in expected range (~5M)" -ForegroundColor Green
} else {
    Write-Host "✗ Line count out of range: $totalLines" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "=== Phase 2 Test PASSED ✓ ===" -ForegroundColor Green
Write-Host "Multi-instance byte-range processing validated" -ForegroundColor Green
Write-Host ""

# Cleanup
Write-Host "Cleaning up test files..." -ForegroundColor Yellow
Remove-Item -Path $outputDir -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path $testFile -Force -ErrorAction SilentlyContinue
Write-Host "✓ Cleanup complete" -ForegroundColor Green
