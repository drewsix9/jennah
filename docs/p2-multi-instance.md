# Phase 2: Multi-Instance Awareness Implementation

**Status:** Ready to Start  
**Estimated Duration:** 0.5 days  
**Date Started:** March 2, 2026

## Overview

Phase 2 validates that the distributed processing correctly divides work across multiple instances using byte-range isolation. The processor from Phase 1 is already capable of this - Phase 2 focuses on validation and ensuring no gaps or overlaps occur.

## Objectives

✓ Byte-range calculation per instance  
✓ File divided evenly across N instances  
✓ Each instance processes only assigned bytes  
✓ No gaps or overlaps in coverage  
✓ Multi-instance synchronization  
✓ Metrics tracked per instance

## Phase 2 Checklist

### 2.1 Verify Processor Byte Range Logic

**Status:** ✅ Already implemented in Phase 1

The processor (`internal/demo/processor.go`) already:

- Receives `BATCH_TASK_INDEX` and `BATCH_TASK_COUNT`
- Uses `ChunkCalculator` to compute assigned byte range
- Seeks to `StartByte` and reads only `Size()` bytes
- Tracks metrics: `StartByte`, `EndByte`, `BytesProcessed`

**Verify:**

```bash
cd internal/demo
go test -v -run TestChunk
# Should show 6 tests passing ✓
```

### 2.2 Run Multi-Instance Local Test

**Execute the Phase 2 test:**

```powershell
# Windows
powershell -ExecutionPolicy Bypass -File test/test-phase2-multiinstance.ps1

# Linux/macOS
bash test/test-phase2-multiinstance.sh
```

**What it tests:**

- 5M line file, 4 concurrent instances
- Each instance processes different byte range
- Total bytes = file size (validates no gaps)
- Total lines ≈ 5M (validates coverage)

**Expected output:**

```
=== Phase 2: Multi-Instance Awareness Test ===
✓ Instance 0:
    Byte range: 0 - 268435455 (268435456 bytes)
    Lines: 1250000
✓ Instance 1:
    Byte range: 268435456 - 536870911 (268435456 bytes)
    Lines: 1250000
✓ Instance 2:
    Byte range: 536870912 - 805306367 (268435456 bytes)
    Lines: 1250000
✓ Instance 3:
    Byte range: 805306368 - 1073741823 (268435456 bytes)
    Lines: 1250000

Summary:
  Total bytes processed: 1073741824 / 1073741824
  Total lines counted: 5000000 / ~5000000
✓ All bytes accounted for (no gaps/overlaps)
✓ Line count in expected range (~5M)
=== Phase 2 Test PASSED ✓ ===
```

### 2.3 Validate Each Instance

**Check individual instance metrics:**

```bash
# After running test, inspect any remaining output files:
cat output-phase2/instance-0.json | jq '.'

# Expected format:
{
  "instance_id": 0,
  "task_index": 0,
  "task_index_max": 3,
  "start_byte": 0,
  "end_byte": 268435455,
  "bytes_processed": 268435456,
  "lines_count": 1250000,
  "words_count": 12500000,
  "characters_count": 268435456,
  "processing_time_seconds": 2.5,
  "throughput_mb_per_second": 102.4,
  "timestamp": "2026-03-02T12:34:56Z"
}
```

### 2.4 Performance Validation

**Measure speedup:**

Run benchmark test:

```bash
# Single instance (baseline)
time (BATCH_TASK_INDEX=0 BATCH_TASK_COUNT=1 ./cmd/demo-job/demo-job)

# 4 instances (parallel)
time (for i in {0..3}; do (BATCH_TASK_INDEX=$i BATCH_TASK_COUNT=4 ./cmd/demo-job/demo-job &); done; wait)

# Calculate speedup = baseline_time / parallel_time
# Expected: 3-3.5x speedup (due to I/O and parallel overhead)
```

## Success Criteria

- [ ] `test-phase2-multiinstance.ps1` runs without errors
- [ ] All 4 instances complete successfully
- [ ] Total bytes = file size (no gaps/overlaps)
- [ ] Total lines ≈ 5M (documents coverage)
- [ ] Each instance has valid metrics JSON
- [ ] No data corruption (test repeatable)

## Git Workflow for Phase 2

```bash
# Create Phase 2 branch (from main after Phase 1 merge)
git checkout -b feature/r9ight-phase2-multiinstance

# Run test to validate
powershell -ExecutionPolicy Bypass -File test/test-phase2-multiinstance.ps1

# If test passes, commit
git add test/test-phase2-multiinstance.* docs/r9ight/PHASE_2_*.md
git commit -m "feat: Phase 2 - Multi-instance awareness validation

- Add comprehensive multi-instance test with 5M lines
- Validate byte-range isolation across 4 instances
- Verify no gaps or overlaps in coverage
- Measure performance and throughput
- All Phase 2 tests passing"

# Push and create PR
git push origin feature/r9ight-phase2-multiinstance
```

## Next: Phase 3

Once Phase 2 is approved and merged, Phase 3 adds **GCS Integration**:

- Read input files from Google Cloud Storage (`gs://bucket/...`)
- Write metrics to GCS
- Handle network retries and timeouts
- Estimated: 1-2 days

## Troubleshooting

### Issue: Only partial lines counted

**Cause:** Line boundary crossing byte range boundary  
**Expected:** Normal - lines are split across instances  
**Solution:** Both instances handle partial lines correctly (see ProcessReader logic)

### Issue: Total bytes < file size

**Cause:** ChunkCalculator may have off-by-one error  
**Solution:** Check `End_byte` is inclusive (EndByte - StartByte + 1 = size)

### Issue: Timeout or slow performance

**Cause:** Disk I/O contention with 4 parallel processes  
**Solution:** Use SSD test files, increase timeout in test script

## References

- Design: [TASK_1_3_GO_ARCHITECTURE.md](TASK_1_3_GO_ARCHITECTURE.md)
- Implementation: [GATE_2_IMPLEMENTATION_AND_MERGE_GUIDE.md](GATE_2_IMPLEMENTATION_AND_MERGE_GUIDE.md)
- Code: `internal/demo/chunker.go` (byte range logic)
