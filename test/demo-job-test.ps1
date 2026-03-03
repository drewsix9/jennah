# Phase 1 Integration Test for Windows

$ErrorActionPreference = "Stop"

Write-Host "=== Phase 1 Integration Test ===" -ForegroundColor Cyan
Write-Host "Testing distributed file processing with demo-job" -ForegroundColor Cyan
Write-Host ""

# Build demo-job if not already built
if (-not (Test-Path "cmd/demo-job/demo-job")) {
    Write-Host "Building demo-job..." -ForegroundColor Yellow
    Push-Location cmd/demo-job
    go build -o demo-job.exe main.go
    Pop-Location
}

# Generate test file (100,000 lines)
Write-Host "Generating test file (100,000 lines)..." -ForegroundColor Yellow
$testFile = "test-integration-input.txt"
$lines = @()
for ($i = 0; $i -lt 100000; $i++) {
    $lines += "Line $i : The quick brown fox jumps over the lazy dog. This is test data for distributed processing."
}
$lines | Out-File -FilePath $testFile -Encoding UTF8
Write-Host "Generated: $testFile" -ForegroundColor Green

$fileSize = (Get-Item $testFile).Length
Write-Host "File size: $fileSize bytes"
Write-Host ""

$test1Pass = $false
$test2Pass = $false
$test3Pass = $false

# Test 1: Single Instance
Write-Host "--- Test 1: Single Instance Mode ---" -ForegroundColor Cyan
$env:INPUT_DATA_PATH = ".\$testFile"
$env:INPUT_DATA_SIZE = $fileSize
$env:BATCH_TASK_INDEX = "0"
$env:BATCH_TASK_COUNT = "1"
$env:OUTPUT_BASE_PATH = ".\output-test-single"
$env:JOB_ID = "integration-test-single"

New-Item -ItemType Directory -Path $env:OUTPUT_BASE_PATH -Force | Out-Null
Write-Host "Running single instance (0/1)..."
try {
    & ".\cmd\demo-job\demo-job.exe" 2>&1 | Out-Null
    
    if (Test-Path "$($env:OUTPUT_BASE_PATH)\instance-0.json") {
        Write-Host "✓ Metrics file created: $($env:OUTPUT_BASE_PATH)\instance-0.json" -ForegroundColor Green
        $metrics = Get-Content "$($env:OUTPUT_BASE_PATH)\instance-0.json" -Raw | ConvertFrom-Json
        Write-Host "  Lines counted: $($metrics.lines_count)"
        Write-Host "  Words counted: $($metrics.words_count)"
        Write-Host "  Throughput: $($metrics.throughput_mb_per_second) MB/s"
        $test1Pass = $true
    } else {
        Write-Host "✗ Metrics file NOT created" -ForegroundColor Red
    }
} catch {
    Write-Host "✗ Test 1 failed: $_" -ForegroundColor Red
}
Write-Host ""

# Test 2: Multi-Instance (2 instances)
Write-Host "--- Test 2: Multi-Instance Mode (2 instances) ---" -ForegroundColor Cyan
$env:OUTPUT_BASE_PATH = ".\output-test-multi"
New-Item -ItemType Directory -Path $env:OUTPUT_BASE_PATH -Force | Out-Null

for ($idx = 0; $idx -lt 2; $idx++) {
    $env:BATCH_TASK_INDEX = $idx
    $env:BATCH_TASK_COUNT = "2"
    Write-Host "Running instance $idx/2..."
    try {
        & ".\cmd\demo-job\demo-job.exe" 2>&1 | Out-Null
    } catch {
        Write-Host "✗ Instance $idx failed: $_" -ForegroundColor Red
    }
}

# Check results
$instance0Exists = Test-Path "$($env:OUTPUT_BASE_PATH)\instance-0.json"
$instance1Exists = Test-Path "$($env:OUTPUT_BASE_PATH)\instance-1.json"

if ($instance0Exists -and $instance1Exists) {
    Write-Host "✓ Both metrics files created" -ForegroundColor Green
    
    $metrics0 = Get-Content "$($env:OUTPUT_BASE_PATH)\instance-0.json" -Raw | ConvertFrom-Json
    $metrics1 = Get-Content "$($env:OUTPUT_BASE_PATH)\instance-1.json" -Raw | ConvertFrom-Json
    
    $totalLines = $metrics0.lines_count + $metrics1.lines_count
    Write-Host "  Instance 0: $($metrics0.lines_count) lines"
    Write-Host "  Instance 1: $($metrics1.lines_count) lines"
    Write-Host "  Total: $totalLines lines"
    
    if ($totalLines -eq 100000) {
        Write-Host "✓ Line counts match expected (100,000)" -ForegroundColor Green
        $test2Pass = $true
    } else {
        Write-Host "✗ Line count mismatch: expected 100000, got $totalLines" -ForegroundColor Red
    }
} else {
    Write-Host "✗ Not all metrics files created" -ForegroundColor Red
}
Write-Host ""

# Test 3: Multi-Instance (4 instances)
Write-Host "--- Test 3: Multi-Instance Mode (4 instances) ---" -ForegroundColor Cyan
$env:OUTPUT_BASE_PATH = ".\output-test-4way"
New-Item -ItemType Directory -Path $env:OUTPUT_BASE_PATH -Force | Out-Null

for ($idx = 0; $idx -lt 4; $idx++) {
    $env:BATCH_TASK_INDEX = $idx
    $env:BATCH_TASK_COUNT = "4"
    Write-Host "Running instance $idx/4..."
    try {
        & ".\cmd\demo-job\demo-job.exe" 2>&1 | Out-Null
    } catch {
        Write-Host "✗ Instance $idx failed: $_" -ForegroundColor Red
    }
}

# Check results
$allFilesExist = $true
$totalLines = 0

for ($idx = 0; $idx -lt 4; $idx++) {
    if (-not (Test-Path "$($env:OUTPUT_BASE_PATH)\instance-$idx.json")) {
        $allFilesExist = $false
        break
    }
    $metrics = Get-Content "$($env:OUTPUT_BASE_PATH)\instance-$idx.json" -Raw | ConvertFrom-Json
    $totalLines += $metrics.lines_count
    Write-Host "  Instance $idx : $($metrics.lines_count) lines"
}

if ($allFilesExist -and $totalLines -eq 100000) {
    Write-Host "✓ All 4 instances ran successfully" -ForegroundColor Green
    Write-Host "✓ Total lines: $totalLines (expected 100,000)" -ForegroundColor Green
    $test3Pass = $true
} else {
    Write-Host "✗ Multi-instance test failed" -ForegroundColor Red
}
Write-Host ""

# Summary
Write-Host "=== Test Results ===" -ForegroundColor Cyan
Write-Host "Test 1 (Single Instance):     $(if ($test1Pass) { '✓ PASS' } else { '✗ FAIL' })" -ForegroundColor $(if ($test1Pass) { 'Green' } else { 'Red' })
Write-Host "Test 2 (2 Instances):         $(if ($test2Pass) { '✓ PASS' } else { '✗ FAIL' })" -ForegroundColor $(if ($test2Pass) { 'Green' } else { 'Red' })
Write-Host "Test 3 (4 Instances):         $(if ($test3Pass) { '✓ PASS' } else { '✗ FAIL' })" -ForegroundColor $(if ($test3Pass) { 'Green' } else { 'Red' })
Write-Host ""

# Cleanup
Write-Host "Cleaning up test files..." -ForegroundColor Yellow
Remove-Item -Path $testFile -Force -ErrorAction SilentlyContinue
Remove-Item -Path "output-test-*" -Recurse -Force -ErrorAction SilentlyContinue

if ($test1Pass -and $test2Pass -and $test3Pass) {
    Write-Host "=== Phase 1 Integration Test PASSED ✓ ===" -ForegroundColor Green
    exit 0
} else {
    Write-Host "=== Phase 1 Integration Test FAILED ✗ ===" -ForegroundColor Red
    exit 1
}
